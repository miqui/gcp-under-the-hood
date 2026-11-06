// Command psds publishes and consumes Pub/Sub messages and deliberately
// misbehaves to demonstrate delivery semantics. Config via env only.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"time"

	"cloud.google.com/go/pubsub"

	"github.com/example/gcp-under-the-hood/pubsubdelivery/internal/logic"
)

func projectID() string {
	for _, k := range []string{"GCP_PROJECT_ID", "TF_VAR_project_id"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	log.Fatal("set GCP_PROJECT_ID (or TF_VAR_project_id)")
	return ""
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: psds publish|subscribe [flags]")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client, err := pubsub.NewClient(ctx, projectID())
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	switch os.Args[1] {
	case "publish":
		publish(ctx, client, os.Args[2:])
	case "subscribe":
		subscribe(ctx, client, os.Args[2:])
	default:
		log.Fatalf("unknown command %q", os.Args[1])
	}
}

func publish(ctx context.Context, c *pubsub.Client, args []string) {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	topic := fs.String("topic", "psds-events", "topic ID")
	n := fs.Int("n", 10, "messages to publish")
	ordered := fs.Bool("ordered", false, "use ordering keys")
	keys := fs.Int("keys", 3, "number of distinct ordering keys")
	_ = fs.Parse(args)

	t := c.Topic(*topic)
	t.EnableMessageOrdering = *ordered
	defer t.Stop()

	seq := map[string]int64{}
	var results []*pubsub.PublishResult
	for i := 0; i < *n; i++ {
		key := logic.OrderingKey(i, *keys)
		seq[key]++
		msg := &pubsub.Message{
			Data:       []byte(fmt.Sprintf("payload-%d", i)),
			Attributes: map[string]string{"key": key, "seq": strconv.FormatInt(seq[key], 10)},
		}
		if *ordered {
			msg.OrderingKey = key
		}
		results = append(results, t.Publish(ctx, msg))
	}
	for i, r := range results {
		id, err := r.Get(ctx)
		if err != nil {
			log.Fatalf("publish %d: %v", i, err)
		}
		log.Printf("published #%d server message ID %s", i, id)
	}
}

func subscribe(ctx context.Context, c *pubsub.Client, args []string) {
	fs := flag.NewFlagSet("subscribe", flag.ExitOnError)
	subID := fs.String("sub", "psds-pull", "subscription ID")
	mode := fs.String("mode", "ack", "ack|stall|nack|poison")
	failUntil := fs.Int("fail-until", 2, "misbehave on attempts <= this")
	ackDeadline := fs.Duration("ack-deadline", 10*time.Second, "must match subscription ack deadline (for stall duration)")
	extend := fs.Bool("extend", false, "let the client library extend leases (MaxExtension) while stalling")
	eod := fs.Bool("eod", false, "exactly-once subscription: use AckWithResult")
	ordered := fs.Bool("ordered", false, "verify per-key ordering")
	runFor := fs.Duration("run-for", 2*time.Minute, "how long to receive")
	_ = fs.Parse(args)

	sub := c.Subscription(*subID)
	if *extend {
		sub.ReceiveSettings.MaxExtension = 10 * time.Minute
	} else {
		// Disable automatic lease extension so the ack deadline is really enforced.
		sub.ReceiveSettings.MaxExtension = -1
	}
	sub.ReceiveSettings.NumGoroutines = 1

	dups := logic.NewDuplicateTracker()
	order := logic.NewOrderChecker()

	rctx, cancel := context.WithTimeout(ctx, *runFor)
	defer cancel()
	err := sub.Receive(rctx, func(_ context.Context, m *pubsub.Message) {
		count, dup := dups.Observe(m.ID)
		// DeliveryAttempt is only set when a dead-letter policy exists; fall back to local count.
		attempt := count
		if m.DeliveryAttempt != nil {
			attempt = *m.DeliveryAttempt
		}
		log.Printf("recv id=%s attempt=%d local_count=%d dup=%v key=%q attrs=%v", m.ID, attempt, count, dup, m.OrderingKey, m.Attributes)

		if *ordered && m.OrderingKey != "" {
			if s, perr := strconv.ParseInt(m.Attributes["seq"], 10, 64); perr == nil {
				if oerr := order.Check(m.OrderingKey, s); oerr != nil {
					log.Printf("ORDER: %v", oerr)
				}
			}
		}

		switch logic.ChooseOutcome(*mode, attempt, *failUntil) {
		case logic.Stall:
			d := logic.StallDuration(*ackDeadline)
			log.Printf("  stalling %v (ack deadline %v, extension=%v)", d, *ackDeadline, *extend)
			time.Sleep(d)
			finish(ctx, m, *eod)
		case logic.Nack:
			log.Printf("  nack")
			m.Nack()
		default:
			finish(ctx, m, *eod)
		}
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("duplicates (id -> times seen): %v", dups.Duplicates())
}

func finish(ctx context.Context, m *pubsub.Message, eod bool) {
	if eod {
		res := m.AckWithResult()
		status, err := res.Get(ctx)
		log.Printf("  ack result status=%v err=%v", status, err)
		return
	}
	m.Ack()
	log.Printf("  ack sent (best effort: no confirmation on non-EOD subscriptions)")
}

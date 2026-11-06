// dbapp connects to Cloud SQL Postgres through the Cloud SQL Go connector
// using IAM database authentication (no password) over private IP.
// Env: INSTANCE_CONNECTION_NAME, DB_USER (IAM user, SA email minus
// ".gserviceaccount.com"), DB_NAME.
package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"database/sql"
	"example.com/gcp-under-the-hood/datatier/internal/logic"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var vals [3]string
	for i, k := range []string{"INSTANCE_CONNECTION_NAME", "DB_USER", "DB_NAME"} {
		v, err := logic.Env(os.Getenv, k)
		if err != nil {
			log.Fatal(err)
		}
		vals[i] = v
	}
	// WithIAMAuthN: connector fetches an OAuth token for the SA and sends it
	// as the Postgres password; it also gets an ephemeral client cert from the
	// sqladmin API and does mTLS on port 3307. WithPrivateIP: use the PSA address.
	d, err := cloudsqlconn.NewDialer(ctx, cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()))
	if err != nil {
		log.Fatal(err)
	}
	defer d.Close()

	cfg, err := pgx.ParseConfig(fmt.Sprintf("user=%s database=%s sslmode=disable", vals[1], vals[2]))
	if err != nil {
		log.Fatal(err)
	}
	cfg.DialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
		return d.Dial(ctx, vals[0])
	}
	db := sql.OpenDB(stdlib.GetConnector(*cfg))
	defer db.Close()

	var user, addr string
	var inRecovery bool
	err = db.QueryRowContext(ctx,
		`SELECT current_user, host(inet_server_addr()), pg_is_in_recovery()`).Scan(&user, &addr, &inRecovery)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("current_user=%s server_ip=%s in_recovery=%v\n", user, addr, inRecovery)
}

// Command bqlab loads sample data and compares dry-run bytes billed.
//
//	bqlab load     # seed lab tables from a public dataset (billed, small)
//	bqlab compare  # dry-run scenarios (free)
//	bqlab dryrun "SELECT ..."
//	bqlab predict  # print scenarios without hitting the API (offline)
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/gcp-under-the-hood/bqinternals/internal/bq"
	"github.com/gcp-under-the-hood/bqinternals/internal/cost"
	"github.com/gcp-under-the-hood/bqinternals/internal/queries"
)

func env(keys ...string) string {
	for _, k := range keys {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return ""
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: bqlab load|compare|dryrun <sql>|predict")
		os.Exit(2)
	}
	project := env("GCP_PROJECT_ID", "TF_VAR_project_id")
	location := env("BQ_LOCATION", "GCP_REGION", "TF_VAR_region")
	if location == "" {
		location = "US"
	}
	dataset := env("BQ_DATASET")
	if dataset == "" {
		dataset = "bq_internals_lab"
	}

	flat, part, clust := mustTables(project, dataset)
	if os.Args[1] == "predict" {
		for _, s := range queries.Scenarios(flat, part, clust) {
			fmt.Printf("%-45s %s\n", s.Label, s.SQL)
		}
		return
	}
	if project == "" {
		fatal("set GCP_PROJECT_ID (or TF_VAR_project_id)")
	}
	token := os.Getenv("GCP_ACCESS_TOKEN")
	if token == "" {
		fatal("set GCP_ACCESS_TOKEN, e.g. export GCP_ACCESS_TOKEN=$(gcloud auth print-access-token)")
	}
	c := &bq.Client{Project: project, Token: token}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	switch os.Args[1] {
	case "load":
		for _, t := range []string{flat, part, clust} {
			n, err := c.Exec(ctx, queries.LoadSQL(t, 200000), location)
			if err != nil {
				fatal(err.Error())
			}
			fmt.Printf("loaded %d rows into %s\n", n, t)
		}
	case "compare":
		var rs []cost.Result
		for _, s := range queries.Scenarios(flat, part, clust) {
			d, err := c.DryRun(ctx, s.SQL, location)
			if err != nil {
				fatal(s.Label + ": " + err.Error())
			}
			rs = append(rs, cost.Result{Label: s.Label, Processed: d.TotalBytesProcessed, Billed: cost.BilledBytes(d.TotalBytesProcessed, d.CacheHit)})
		}
		fmt.Printf("%-45s %12s %12s %10s %7s\n", "scenario", "processed", "billed", "USD", "%base")
		for _, r := range cost.Compare(rs) {
			fmt.Printf("%-45s %12s %12s %10.6f %6.1f%%\n", r.Label, cost.Human(r.Processed), cost.Human(r.Billed), r.USD, r.PctOfBase)
		}
		fmt.Println("cheapest:", cost.Cheapest(rs))
	case "dryrun":
		if len(os.Args) < 3 {
			fatal("dryrun needs a SQL argument")
		}
		d, err := c.DryRun(ctx, os.Args[2], location)
		if err != nil {
			fatal(err.Error())
		}
		b := cost.BilledBytes(d.TotalBytesProcessed, d.CacheHit)
		fmt.Printf("processed=%s billed=%s usd=%.6f\n", cost.Human(d.TotalBytesProcessed), cost.Human(b), cost.OnDemandUSD(b))
	default:
		fatal("unknown command " + os.Args[1])
	}
}

func mustTables(project, dataset string) (string, string, string) {
	p := project
	if p == "" {
		p = "PROJECT_ID"
	}
	out := make([]string, 3)
	for i, n := range []string{"trips_flat", "trips_part", "trips_part_clust"} {
		t, err := queries.Table(p, dataset, n)
		if err != nil {
			fatal(err.Error())
		}
		out[i] = t
	}
	return out[0], out[1], out[2]
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, "error:", msg)
	os.Exit(1)
}

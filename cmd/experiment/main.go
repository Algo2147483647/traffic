package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"traffic/internal/algorithms/experiments"
	"traffic/internal/app"
	model "traffic/internal/infrastructure/road"
	"traffic/internal/operations/taxi/dispatch"
	sim "traffic/internal/simulation"
)

func main() {
	config := flag.String("config", "example/Map2/road.json", "Map file")
	policies := flag.String("algorithms", "nearest,fifo,greedy,hungarian,batch,score", "Comma-separated policy IDs")
	seeds := flag.String("seeds", "71429,71430,71431", "Paired random seeds")
	horizon := flag.Float64("duration", 600, "Simulation seconds")
	fleet := flag.Int("fleet", 16, "Fleet size")
	demand := flag.Float64("demand", 8, "Orders per minute")
	output := flag.String("output", "taxi-experiment", "Output prefix")
	dir := flag.String("dispatch-dir", "experiments/dispatch", "Worker directory")
	flag.Parse()
	fail := func(e error) { fmt.Fprintln(os.Stderr, e); os.Exit(1) }
	if e := dispatch.LoadDirectory(*dir); e != nil {
		fail(e)
	}
	c, e := model.LoadConfig(*config)
	if e != nil {
		fail(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		fail(e)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	results := []experiments.Result[*sim.TaxiView]{}
	for _, seed := range strings.Split(*seeds, ",") {
		n, e := strconv.ParseInt(strings.TrimSpace(seed), 10, 64)
		if e != nil {
			fail(e)
		}
		for _, id := range strings.Split(*policies, ",") {
			cfg := sim.DefaultTaxiConfig()
			cfg.Seed = n
			cfg.Fleet = *fleet
			cfg.Demand = *demand
			cfg.Algorithm = strings.TrimSpace(id)
			r, e := experiments.Run(ctx, experiments.Specification{Algorithm: cfg.Algorithm, Seed: cfg.Seed, Horizon: *horizon}, func() (experiments.Session[*sim.TaxiView], error) { return app.NewTaxiExperiment(g, cfg) })
			if e != nil {
				fail(e)
			}
			results = append(results, r)
			fmt.Fprintf(os.Stderr, "%s seed=%d: %s, trips=%d\n", id, n, r.Status, r.Taxi.Completed)
			if ctx.Err() != nil {
				break
			}
		}
		if ctx.Err() != nil {
			break
		}
	}
	mapBytes, _ := os.ReadFile(*config)
	sum := sha256.Sum256(mapBytes)
	exe, _ := os.Executable()
	engineBytes, _ := os.ReadFile(exe)
	engineHash := sha256.Sum256(engineBytes)
	report := map[string]any{"schema_version": 1, "map_sha256": hex.EncodeToString(sum[:]), "engine_sha256": hex.EncodeToString(engineHash[:]), "candidate_policy": "all eligible taxis for oldest 64 waiting orders", "eta_model": "free-flow-v1; actual movement observes queues and signals", "warmup_s": 0, "results": results}
	data, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		fail(e)
	}
	if e = os.MkdirAll(filepath.Dir(*output), 0755); e != nil {
		fail(e)
	}
	if e = os.WriteFile(*output+".json", data, 0644); e != nil {
		fail(e)
	}
	f, e := os.Create(*output + ".csv")
	if e != nil {
		fail(e)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	_ = w.Write([]string{"algorithm", "seed", "status", "simulation_s", "created", "picked", "completed", "cancelled", "open_at_horizon", "mean_picked_wait_s", "failures", "wall_s"})
	for _, r := range results {
		t := r.Taxi
		_ = w.Write([]string{r.Algorithm, fmt.Sprint(r.Seed), r.Status, fmt.Sprint(r.ActualTime), fmt.Sprint(t.Created), fmt.Sprint(t.Picked), fmt.Sprint(t.Completed), fmt.Sprint(t.Cancelled), fmt.Sprint(t.Created - t.Completed - t.Cancelled), fmt.Sprint(t.MeanWait), fmt.Sprint(t.Failures), fmt.Sprint(r.WallSeconds)})
	}
	w.Flush()
	if e = w.Error(); e != nil {
		fail(e)
	}
}

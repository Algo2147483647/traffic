package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"time"

	"traffic/internal/algorithms"
	"traffic/internal/dispatch"
	model "traffic/internal/infrastructure/road"
	sim "traffic/internal/road"
	"traffic/internal/server"
)

func main() {
	if err := run(); err != nil {
		slog.Error("traffic stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	config := flag.String("config", "configs/meridian-city.json", "City JSON file")
	presetDir := flag.String("preset-dir", "configs/presets", "Additional city presets (empty disables discovery)")
	addr := flag.String("addr", "127.0.0.1:8080", "HTTP listen address")
	desktop := flag.Bool("desktop", false, "Private desktop service; exit when parent stdin closes")
	policyDir := flag.String("dispatch-dir", "experiments/dispatch", "Local experimental policy manifest directory")
	roadPolicy := flag.String("road-policy", "baseline", "Registered road policy profile")
	roadParameters := flag.String("road-parameters", "{}", "Numeric policy parameters as JSON")
	algorithmDir := flag.String("algorithm-dir", "experiments/algorithms", "Route and signal worker manifests")
	flag.Parse()
	if err := algorithms.LoadDirectory(*algorithmDir); err != nil {
		return err
	}
	var policyParameters map[string]float64
	if err := json.Unmarshal([]byte(*roadParameters), &policyParameters); err != nil {
		return err
	}
	if err := dispatch.LoadDirectory(*policyDir); err != nil {
		return err
	}
	token := os.Getenv("TRAFFIC_DESKTOP_TOKEN")
	if *desktop {
		if len(token) < 32 {
			return fmt.Errorf("desktop session token is missing")
		}
		*addr = "127.0.0.1:0"
	}
	city, err := model.LoadConfig(*config)
	if err != nil {
		return err
	}
	graph, err := model.BuildGraph(city)
	if err != nil {
		return err
	}
	presets := []server.Preset{{ID: "preset-1", Graph: graph}}
	if *presetDir != "" {
		entries, err := model.LoadPresetDirectory(*presetDir)
		if os.IsNotExist(err) {
			err = nil
		}
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.ID == "preset-1" {
				return fmt.Errorf("reserved preset ID: %s", entry.ID)
			}
			g, err := model.BuildGraph(entry.Config)
			if err != nil {
				return fmt.Errorf("preset %s: %w", entry.ID, err)
			}
			presets = append(presets, server.Preset{ID: entry.ID, Graph: g})
		}
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *desktop {
		go func() { _, _ = io.Copy(io.Discard, os.Stdin); cancel() }()
	}
	simulation, err := sim.NewWithPolicy(graph, *roadPolicy, policyParameters)
	if err != nil {
		return err
	}
	defer simulation.Close()
	api := server.NewWithPresets(simulation, slog.Default(), "preset-1", presets)
	defer api.Close()
	handler := api.Handler()
	if *desktop {
		handler = privateHandler(handler, token)
	}
	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		return err
	}
	defer listener.Close()
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	go simulation.Run(ctx)
	errors := make(chan error, 1)
	go func() { errors <- httpServer.Serve(listener) }()
	if *desktop {
		if err := json.NewEncoder(os.Stdout).Encode(map[string]string{"url": "http://" + listener.Addr().String()}); err != nil {
			return err
		}
	} else {
		slog.Info("Traffic ready", "url", "http://"+listener.Addr().String())
	}
	select {
	case <-ctx.Done():
		api.Close()
		shutdown, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		return httpServer.Shutdown(shutdown)
	case err := <-errors:
		return err
	}
}

func privateHandler(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Traffic-Token")), []byte(token)) != 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

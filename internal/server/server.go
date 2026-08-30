package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"traffic/internal/sim"
)

//go:embed web/*
var webFiles embed.FS

type Server struct {
	simulation *sim.Simulation
	logger     *slog.Logger
	handler    http.Handler
	shutdown   chan struct{}
	closeOnce  sync.Once
}

func New(simulation *sim.Simulation, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{simulation: simulation, logger: logger, shutdown: make(chan struct{})}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("GET /api/city", server.city)
	mux.HandleFunc("GET /api/snapshot", server.snapshot)
	mux.HandleFunc("GET /api/stream", server.stream)
	mux.HandleFunc("POST /api/control", server.control)
	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		panic(err)
	}
	mux.Handle("/", http.FileServer(http.FS(static)))
	server.handler = server.withMiddleware(mux)
	return server
}

func (s *Server) Handler() http.Handler { return s.handler }

// Close ends long-lived event streams before the HTTP server shuts down.
func (s *Server) Close() { s.closeOnce.Do(func() { close(s.shutdown) }) }

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("Access-Control-Allow-Origin", "*")
		if r.Method == http.MethodOptions {
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
		if r.URL.Path != "/api/stream" {
			s.logger.Debug("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(started))
		}
	})
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "time": s.simulation.Snapshot().Time})
}

func (s *Server) city(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.simulation.City())
}

func (s *Server) snapshot(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, s.simulation.Snapshot())
}

func (s *Server) stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming is unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	fps := s.simulation.City().Simulation.SnapshotFPS
	interval := time.Second / time.Duration(fps)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	writeEvent := func(name string, payload any) error {
		data, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, data)
		if err == nil {
			flusher.Flush()
		}
		return err
	}
	if err := writeEvent("city", s.simulation.City()); err != nil {
		return
	}
	if err := writeEvent("snapshot", s.simulation.Snapshot()); err != nil {
		return
	}
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.shutdown:
			return
		case <-ticker.C:
			if err := writeEvent("snapshot", s.simulation.Snapshot()); err != nil {
				return
			}
		}
	}
}

type controlRequest struct {
	Action     string  `json:"action"`
	TimeScale  float64 `json:"time_scale,omitempty"`
	Population int     `json:"population,omitempty"`
	SpawnRate  float64 `json:"spawn_rate_per_second,omitempty"`
	MaxPeople  int     `json:"max_population,omitempty"`
	Delta      float64 `json:"delta,omitempty"`
	SafeGap    float64 `json:"safe_gap,omitempty"`
}

func (s *Server) control(w http.ResponseWriter, r *http.Request) {
	var request controlRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	switch request.Action {
	case "pause":
		s.simulation.SetPaused(true)
	case "resume":
		s.simulation.SetPaused(false)
	case "step":
		s.simulation.Step(request.Delta)
	case "reset":
		s.simulation.Reset(request.Population)
	case "set_time_scale":
		if request.TimeScale <= 0 || request.TimeScale > 8 {
			s.writeError(w, http.StatusBadRequest, "time_scale must be in (0, 8]")
			return
		}
		s.simulation.SetTimeScale(request.TimeScale)
	case "set_load":
		if request.Population < 1 || request.Population > 5000 {
			s.writeError(w, http.StatusBadRequest, "population must be in [1, 5000]")
			return
		}
		if request.MaxPeople < request.Population || request.MaxPeople > 5000 {
			s.writeError(w, http.StatusBadRequest, "max_population must be between population and 5000")
			return
		}
		if request.SpawnRate < 0 || request.SpawnRate > 100 {
			s.writeError(w, http.StatusBadRequest, "spawn_rate_per_second must be in [0, 100]")
			return
		}
		s.simulation.SetLoad(request.Population, request.SpawnRate, request.MaxPeople)
	case "set_spacing":
		if request.SafeGap < 2 || request.SafeGap > 200 {
			s.writeError(w, http.StatusBadRequest, "safe_gap must be in [2, 200]")
			return
		}
		s.simulation.SetSpacing(request.SafeGap)
	default:
		s.writeError(w, http.StatusBadRequest, "unknown action; use pause, resume, step, reset, set_time_scale, set_load, or set_spacing")
		return
	}
	s.writeJSON(w, http.StatusOK, s.simulation.Snapshot())
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, map[string]string{"error": message, "status": strconv.Itoa(status)})
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Error("write JSON response", "error", err)
	}
}

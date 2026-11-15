package server

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"
	"traffic/frontend"

	"traffic/internal/algorithms"
	"traffic/internal/foundation"
	model "traffic/internal/infrastructure/road"
	"traffic/internal/operations/demand"
	"traffic/internal/operations/metro"
	"traffic/internal/operations/taxi/dispatch"
	sim "traffic/internal/simulation"
)

var webFiles = frontend.Files

type Server struct {
	simulation *sim.Simulation
	logger     *slog.Logger
	handler    http.Handler
	shutdown   chan struct{}
	closeOnce  sync.Once
	presetMu   sync.RWMutex
	presets    map[string]*model.Graph
	presetList []presetInfo
	current    string
	revision   uint64
}

type Preset struct {
	ID    string
	Graph *model.Graph
}

type presetInfo struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	Intersections int    `json:"intersections"`
	DirectedWays  int    `json:"directed_ways"`
}

func New(simulation *sim.Simulation, logger *slog.Logger) *Server {
	city := simulation.City()
	return newServer(simulation, logger, "current", nil, []presetInfo{{
		ID: "current", Name: city.Name, Description: city.Description,
		Intersections: len(city.Intersections), DirectedWays: len(city.Ways),
	}})
}

// NewWithPresets exposes validated graphs as switchable map presets while one
// Simulation instance continues to serve the existing stream connections.
func NewWithPresets(simulation *sim.Simulation, logger *slog.Logger, current string, presets []Preset) *Server {
	graphs := make(map[string]*model.Graph, len(presets))
	infos := make([]presetInfo, 0, len(presets))
	for _, preset := range presets {
		if preset.ID == "" || preset.Graph == nil {
			continue
		}
		if _, duplicate := graphs[preset.ID]; duplicate {
			panic("duplicate map preset: " + preset.ID)
		}
		graphs[preset.ID] = preset.Graph
		infos = append(infos, presetInfo{
			ID: preset.ID, Name: preset.Graph.Name, Description: preset.Graph.Description,
			Intersections: len(preset.Graph.Intersections), DirectedWays: len(preset.Graph.Ways),
		})
	}
	if len(infos) == 0 {
		return New(simulation, logger)
	}
	if _, ok := graphs[current]; !ok {
		current = infos[0].ID
	}
	return newServer(simulation, logger, current, graphs, infos)
}

func newServer(simulation *sim.Simulation, logger *slog.Logger, current string, presets map[string]*model.Graph, presetList []presetInfo) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := &Server{simulation: simulation, logger: logger, shutdown: make(chan struct{}), presets: presets, presetList: presetList, current: current}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("GET /api/algorithms/workers", func(w http.ResponseWriter, r *http.Request) { server.writeJSON(w, 200, algorithms.List()) })
	mux.HandleFunc("POST /api/commands", func(w http.ResponseWriter, r *http.Request) {
		var e foundation.Envelope[sim.Command]
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
		d.DisallowUnknownFields()
		if err := d.Decode(&e); err != nil {
			server.writeError(w, 400, "invalid command envelope")
			return
		}
		seq, err := server.simulation.QueueCommand(e)
		if err != nil {
			server.writeError(w, 400, err.Error())
			return
		}
		server.writeJSON(w, 202, map[string]any{"sequence": seq})
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) { server.writeJSON(w, 200, server.simulation.Events()) })
	mux.HandleFunc("GET /api/algorithms", func(w http.ResponseWriter, r *http.Request) {
		server.writeJSON(w, 200, map[string]any{"road": sim.PolicyCatalog(), "dispatch": dispatch.List()})
	})
	mux.HandleFunc("GET /api/dispatch/algorithms", func(w http.ResponseWriter, r *http.Request) { server.writeJSON(w, 200, dispatch.List()) })
	mux.HandleFunc("GET /api/city", server.city)
	mux.HandleFunc("GET /api/presets", server.presetOptions)
	mux.HandleFunc("GET /api/snapshot", server.snapshot)
	mux.HandleFunc("GET /api/stream", server.stream)
	mux.HandleFunc("POST /api/control", server.control)
	static, err := fs.Sub(webFiles, "dist")
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

func (s *Server) presetOptions(w http.ResponseWriter, _ *http.Request) {
	s.presetMu.RLock()
	defer s.presetMu.RUnlock()
	s.writeJSON(w, http.StatusOK, map[string]any{
		"current_preset_id": s.current,
		"presets":           append([]presetInfo(nil), s.presetList...),
	})
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
	s.presetMu.RLock()
	revision := s.revision
	s.presetMu.RUnlock()
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
			s.presetMu.RLock()
			currentRevision := s.revision
			s.presetMu.RUnlock()
			if currentRevision != revision {
				if err := writeEvent("city", s.simulation.City()); err != nil {
					return
				}
				revision = currentRevision
			}
			if err := writeEvent("snapshot", s.simulation.Snapshot()); err != nil {
				return
			}
		}
	}
}

type controlRequest struct {
	Metro      metro.Config        `json:"metro,omitempty"`
	Demand     demand.Config       `json:"demand,omitempty"`
	Algorithms sim.AlgorithmConfig `json:"algorithms,omitempty"`
	WalkSpeed  float64             `json:"walk_speed_mps,omitempty"`
	Mode       string              `json:"mode,omitempty"`
	Taxi       sim.TaxiConfig      `json:"taxi,omitempty"`
	Action     string              `json:"action"`
	TimeScale  float64             `json:"time_scale,omitempty"`
	Population int                 `json:"population,omitempty"`
	SpawnRate  float64             `json:"spawn_rate_per_second,omitempty"`
	MaxPeople  int                 `json:"max_population,omitempty"`
	Delta      float64             `json:"delta,omitempty"`
	SafeGap    float64             `json:"safe_gap,omitempty"`
	PresetID   string              `json:"preset_id,omitempty"`
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
	case "set_metro":
		if err := s.simulation.Execute(sim.Command{Kind: "set_metro", Metro: request.Metro}); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "set_taxi_config":
		if err := s.simulation.Execute(sim.Command{Kind: "set_taxi_config", Taxi: request.Taxi}); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "set_demand":
		if err := s.simulation.Execute(sim.Command{Kind: "set_demand", Demand: request.Demand}); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "set_algorithms":
		if err := s.simulation.Execute(sim.Command{Kind: "set_algorithms", Algorithms: request.Algorithms}); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "set_walk_speed":
		if err := s.simulation.Execute(sim.Command{Kind: "set_walk_speed", Value: request.WalkSpeed}); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "set_taxi_load":
		if err := s.simulation.SetTaxiLoad(request.Taxi.Fleet, request.Taxi.Demand); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}

	case "set_mode":
		if err := s.simulation.SetMode(request.Mode, request.Taxi); err != nil {
			s.writeError(w, 400, err.Error())
			return
		}
	case "pause":
		s.simulation.SetPaused(true)
	case "resume":
		s.simulation.SetPaused(false)
	case "step":
		s.simulation.Step(request.Delta)
	case "reset", "set_time_scale", "set_load", "set_spacing":
		c := sim.Command{Kind: request.Action, Population: request.Population, MaxPeople: request.MaxPeople}
		switch request.Action {
		case "set_time_scale":
			c.Value = request.TimeScale
		case "set_load":
			c.Value = request.SpawnRate
		case "set_spacing":
			c.Value = request.SafeGap
		}
		if err := s.simulation.Execute(c); err != nil {
			s.writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	case "set_preset":
		s.presetMu.RLock()
		graph, ok := s.presets[request.PresetID]
		s.presetMu.RUnlock()
		if !ok {
			s.writeError(w, http.StatusBadRequest, "unknown preset_id")
			return
		}
		s.simulation.ReplaceGraph(graph)
		s.presetMu.Lock()
		s.current = request.PresetID
		s.revision++
		s.presetMu.Unlock()
	default:
		s.writeError(w, http.StatusBadRequest, "unknown action; use pause, resume, step, reset, set_time_scale, set_load, set_spacing, or set_preset")
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

package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"traffic/internal/model"
	"traffic/internal/sim"
)

func testHandler(t *testing.T) http.Handler {
	t.Helper()
	graph, err := model.BuildGraph(model.CityConfig{
		Name: "API City", Bounds: model.Bounds{MaxX: 100, MaxY: 100},
		Simulation:    model.SimulationConfig{People: 2},
		Intersections: []model.IntersectionConfig{{ID: "A", Position: model.Vec2{X: 10, Y: 50}}, {ID: "B", Position: model.Vec2{X: 90, Y: 50}}},
		Ways:          []model.WayConfig{{ID: "AB", From: "A", To: "B", Lanes: 2}},
	})
	if err != nil {
		t.Fatal(err)
	}
	return New(sim.New(graph), slog.New(slog.NewTextHandler(io.Discard, nil))).Handler()
}

func TestCityAndStaticEndpoints(t *testing.T) {
	handler := testHandler(t)
	for _, path := range []string{"/", "/styles.css", "/api/city", "/api/snapshot", "/api/health"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Errorf("GET %s = %d", path, response.Code)
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/api/city", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var city sim.CityView
	if err := json.Unmarshal(response.Body.Bytes(), &city); err != nil {
		t.Fatal(err)
	}
	if city.DrivingSide != model.DrivingSideRight || city.MovementRules.ThroughToleranceDegrees != 40 {
		t.Fatalf("city API omitted traffic conventions: side=%q rules=%+v", city.DrivingSide, city.MovementRules)
	}
}

func TestControlPause(t *testing.T) {
	handler := testHandler(t)
	body := bytes.NewBufferString(`{"action":"pause"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/control", body)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST control = %d: %s", response.Code, response.Body.String())
	}
	var snapshot sim.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if !snapshot.Paused {
		t.Fatal("simulation was not paused")
	}
}

func TestControlTrafficLoad(t *testing.T) {
	handler := testHandler(t)
	body := bytes.NewBufferString(`{"action":"set_load","population":80,"spawn_rate_per_second":3.5,"max_population":200}`)
	request := httptest.NewRequest(http.MethodPost, "/api/control", body)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST load control = %d: %s", response.Code, response.Body.String())
	}
	var snapshot sim.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Load.Population != 80 || snapshot.Load.SpawnRate != 3.5 || snapshot.Load.MaxPopulation != 200 {
		t.Fatalf("unexpected load: %+v", snapshot.Load)
	}
}

func TestControlVehicleSpacing(t *testing.T) {
	handler := testHandler(t)
	body := bytes.NewBufferString(`{"action":"set_spacing","safe_gap":42}`)
	request := httptest.NewRequest(http.MethodPost, "/api/control", body)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST spacing control = %d: %s", response.Code, response.Body.String())
	}
	var snapshot sim.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Spacing.SafeGap != 42 {
		t.Fatalf("safe gap = %v, want 42", snapshot.Spacing.SafeGap)
	}
}

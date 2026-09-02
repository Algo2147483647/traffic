package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
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
	for _, path := range []string{"/", "/styles.css", "/app.js", "/api/city", "/api/presets", "/api/snapshot", "/api/health"} {
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
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	html := response.Body.String()
	if !strings.Contains(html, `<option value="10">10×</option>`) || !strings.Contains(html, `id="safe-gap-control" type="range" min="4" max="80" step="2" value="4"`) ||
		!strings.Contains(html, `id="map-preset"`) || !strings.Contains(html, `id="compute-rate"`) ||
		!strings.Contains(html, `id="population-control" type="range" min="20" max="10000"`) ||
		!strings.Contains(html, `id="max-population-control" type="range" min="100" max="10000"`) {
		t.Fatal("dashboard does not expose map presets, 10x rate, 4m spacing, 10,000 people, and compute frequency")
	}
	request = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if script := response.Body.String(); !strings.Contains(script, "bezierCurveTo") || !strings.Contains(script, "updateComputeRate") {
		t.Fatal("dashboard script does not draw Bezier roads or measure compute frequency")
	}
}

func TestMapPresetSwitch(t *testing.T) {
	build := func(name string, people int) *model.Graph {
		t.Helper()
		graph, err := model.BuildGraph(model.CityConfig{
			Name: name, Bounds: model.Bounds{MaxX: 100, MaxY: 100}, Simulation: model.SimulationConfig{People: people},
			Intersections: []model.IntersectionConfig{{ID: "A", Position: model.Vec2{X: 10, Y: 50}}, {ID: "B", Position: model.Vec2{X: 90, Y: 50}}},
			Ways:          []model.WayConfig{{ID: "AB", From: "A", To: "B", Lanes: 2}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return graph
	}
	first, second := build("Preset One", 2), build("Preset Two", 5)
	simulation := sim.New(first)
	handler := NewWithPresets(simulation, slog.New(slog.NewTextHandler(io.Discard, nil)), "preset-1", []Preset{
		{ID: "preset-1", Graph: first}, {ID: "preset-2", Graph: second},
	}).Handler()

	request := httptest.NewRequest(http.MethodGet, "/api/presets", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	var options struct {
		Current string       `json:"current_preset_id"`
		Presets []presetInfo `json:"presets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &options); err != nil {
		t.Fatal(err)
	}
	if options.Current != "preset-1" || len(options.Presets) != 2 {
		t.Fatalf("unexpected preset options: %+v", options)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/control", bytes.NewBufferString(`{"action":"set_preset","preset_id":"preset-2"}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST preset switch = %d: %s", response.Code, response.Body.String())
	}
	if city := simulation.City(); city.Name != "Preset Two" {
		t.Fatalf("city after preset switch = %q", city.Name)
	}
	if snapshot := simulation.Snapshot(); snapshot.City != "Preset Two" || len(snapshot.People) != 5 {
		t.Fatalf("snapshot was not reset for preset two: city=%q people=%d", snapshot.City, len(snapshot.People))
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

func TestControlTrafficLoadAllowsTenThousand(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/control", bytes.NewBufferString(`{"action":"set_load","population":2,"spawn_rate_per_second":0,"max_population":10000}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST 10,000 population cap = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/control", bytes.NewBufferString(`{"action":"set_load","population":10001,"spawn_rate_per_second":0,"max_population":10001}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST population above 10,000 = %d, want %d", response.Code, http.StatusBadRequest)
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

func TestControlSupportsTenTimesSpeed(t *testing.T) {
	handler := testHandler(t)
	body := bytes.NewBufferString(`{"action":"set_time_scale","time_scale":10}`)
	request := httptest.NewRequest(http.MethodPost, "/api/control", body)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST 10x time scale = %d: %s", response.Code, response.Body.String())
	}
	var snapshot sim.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.TimeScale != 10 {
		t.Fatalf("time scale = %v, want 10", snapshot.TimeScale)
	}

	body = bytes.NewBufferString(`{"action":"set_time_scale","time_scale":10.1}`)
	request = httptest.NewRequest(http.MethodPost, "/api/control", body)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST time scale above 10x = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

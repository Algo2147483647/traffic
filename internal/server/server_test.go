package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	model "traffic/internal/infrastructure/road"
	sim "traffic/internal/simulation"
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
	for _, path := range []string{"/", "/api/city", "/api/presets", "/api/snapshot", "/api/health"} {
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
	if !strings.Contains(html, `id="root"`) {
		t.Fatal("missing React mount point")
	}
	assets := regexp.MustCompile(`(?:src|href)="(/assets/[^" ]+)"`).FindAllStringSubmatch(html, -1)
	if len(assets) < 2 {
		t.Fatal("missing production JavaScript and stylesheet")
	}
	for _, asset := range assets {
		response = httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, asset[1], nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("GET %s = %d, empty=%v", asset[1], response.Code, response.Body.Len() == 0)
		}
		if strings.HasSuffix(asset[1], ".js") && !strings.Contains(response.Body.String(), "bezierCurveTo") {
			t.Fatal("missing Canvas renderer in production bundle")
		}
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

func TestControlTrafficLoadAllowsTypedCapacity(t *testing.T) {
	handler := testHandler(t)
	request := httptest.NewRequest(http.MethodPost, "/api/control", bytes.NewBufferString(`{"action":"set_load","population":2,"spawn_rate_per_second":0,"max_population":15000}`))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("POST 10,000 population cap = %d: %s", response.Code, response.Body.String())
	}

	request = httptest.NewRequest(http.MethodPost, "/api/control", bytes.NewBufferString(`{"action":"set_load","population":2,"spawn_rate_per_second":0,"max_population":1}`))
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST capacity below population = %d, want %d", response.Code, http.StatusBadRequest)
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

func TestControlSupportsTypedSpeed(t *testing.T) {
	handler := testHandler(t)
	body := bytes.NewBufferString(`{"action":"set_time_scale","time_scale":12.5}`)
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
	if snapshot.TimeScale != 12.5 {
		t.Fatalf("time scale = %v, want 10", snapshot.TimeScale)
	}

	body = bytes.NewBufferString(`{"action":"set_time_scale","time_scale":0}`)
	request = httptest.NewRequest(http.MethodPost, "/api/control", body)
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("POST nonpositive time scale = %d, want %d", response.Code, http.StatusBadRequest)
	}
}

func TestScheduledCommandEndpoint(t *testing.T) {
	h := testHandler(t)
	get := httptest.NewRecorder()
	h.ServeHTTP(get, httptest.NewRequest("GET", "/api/snapshot", nil))
	var snap sim.Snapshot
	if err := json.Unmarshal(get.Body.Bytes(), &snap); err != nil {
		t.Fatal(err)
	}
	payload, _ := json.Marshal(map[string]any{"stamp": snap.Kernel, "due_tick": snap.Tick, "payload": map[string]any{"kind": "set_spacing", "value": 7}})
	response := httptest.NewRecorder()
	h.ServeHTTP(response, httptest.NewRequest("POST", "/api/commands", bytes.NewReader(payload)))
	if response.Code != 202 {
		t.Fatal(response.Body.String())
	}
	apply := httptest.NewRecorder()
	h.ServeHTTP(apply, httptest.NewRequest("POST", "/api/control", strings.NewReader(`{"action":"step","delta":0.05}`)))
	if err := json.Unmarshal(apply.Body.Bytes(), &snap); err != nil || snap.Spacing.SafeGap != 7 {
		t.Fatal(apply.Body.String())
	}
	reset := httptest.NewRecorder()
	h.ServeHTTP(reset, httptest.NewRequest("POST", "/api/control", strings.NewReader(`{"action":"reset"}`)))
	stale := httptest.NewRecorder()
	h.ServeHTTP(stale, httptest.NewRequest("POST", "/api/commands", bytes.NewReader(payload)))
	if stale.Code != 400 {
		t.Fatal("stale command accepted")
	}
}

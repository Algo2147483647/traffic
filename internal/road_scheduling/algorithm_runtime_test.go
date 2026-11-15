package roadscheduling

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"traffic/internal/algorithms"
	model "traffic/internal/infrastructure/road"
	interchange "traffic/internal/road_scheduling/routing"
)

func TestAlgorithmWorkerHelper(t *testing.T) {
	if len(os.Args) < 2 || !strings.HasPrefix(os.Args[len(os.Args)-1], "worker:") {
		return
	}
	mode := strings.TrimPrefix(os.Args[len(os.Args)-1], "worker:")
	if mode == "timeout" {
		time.Sleep(5 * time.Second)
		os.Exit(0)
	}
	if mode == "crash" {
		os.Exit(3)
	}
	if mode == "malformed" {
		fmt.Print("invalid json")
		os.Exit(0)
	}
	var in algorithms.Request
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	r := algorithms.Response{Version: in.Version, Algorithm: in.Algorithm, AlgorithmVersion: in.AlgorithmVersion, Capability: in.Capability, Stamp: in.Stamp, Tick: in.Tick, Request: in.Request}
	if mode == "stale" {
		r.Stamp.Run++
	}
	if in.Capability == "route" {
		var data struct {
			Tasks []routeTask `json:"tasks"`
		}
		_ = json.Unmarshal(in.Payload, &data)
		out := []routeResult{}
		for _, task := range data.Tasks {
			steps := task.Options[len(task.Options)-1].Steps
			if mode == "invalid" {
				steps[0].WayID = "missing"
			}
			out = append(out, routeResult{task.Entity, steps})
		}
		r.Payload, _ = json.Marshal(map[string]any{"routes": out})
	} else {
		var data struct {
			Tasks []signalTask `json:"tasks"`
		}
		_ = json.Unmarshal(in.Payload, &data)
		out := []signalResult{}
		for _, task := range data.Tasks {
			idx := (task.Current + 1) % len(task.Phases)
			green := task.Phases[idx].Min
			if mode == "invalid" {
				green = -1
			}
			out = append(out, signalResult{task.Intersection, idx, green})
		}
		r.Payload, _ = json.Marshal(map[string]any{"signals": out})
	}
	_ = json.NewEncoder(os.Stdout).Encode(r)
	os.Exit(0)
}
func registerAlgorithmHelper(t *testing.T, cap, mode string) string {
	t.Helper()
	id := t.Name() + "/" + cap + "/" + mode
	timeout := 1500
	if mode == "timeout" {
		timeout = 50
	}
	err := algorithms.Register(algorithms.Manifest{ID: id, Capability: cap, AlgorithmVersion: "test", ContractVersion: 1, Command: []string{os.Args[0], "-test.run=^TestAlgorithmWorkerHelper$", "--", "worker:" + mode}, Timeout: timeout})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func finishAlgorithms(t *testing.T, s *Simulation) {
	t.Helper()
	deadline := time.Now().Add(6 * time.Second)
	for !s.advanceAlgorithmsLocked() {
		if time.Now().After(deadline) {
			t.Fatal("worker did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
func workerGraph(t *testing.T) *model.Graph {
	t.Helper()
	c, e := model.LoadConfig("../testdata/signal-network/road.json")
	if e != nil {
		t.Fatal(e)
	}
	c.Simulation.People = 30
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	return g
}
func TestExternalRouteWorkerChangesPhysicalRoute(t *testing.T) {
	s := New(workerGraph(t))
	defer s.Close()
	for _, p := range s.people {
		p.state = "driving"
		p.wayID = p.route[0]
	}
	var task routeTask
	var chosen *personState
	for k := 0; k < 10 && chosen == nil; k++ {
		for _, candidate := range s.algorithmRoutes() {
			if len(candidate.Options) > 1 {
				for _, p := range s.people {
					if p.id == candidate.Entity {
						task = candidate
						chosen = p
						break
					}
				}
			}
		}
	}
	if chosen == nil {
		t.Fatal("fixture has no alternative route")
	}
	s.people = []*personState{chosen}
	s.algorithm.cursor = 0
	chosen.route, chosen.routeLanes, chosen.routeEntryLanes = task.Options[0].Arrays()
	chosen.routeIndex = 0
	want, _, _ := task.Options[len(task.Options)-1].Arrays()
	id := registerAlgorithmHelper(t, "route", "valid")
	if e := s.setAlgorithms(AlgorithmConfig{Route: AlgorithmChoice{ID: id}}); e != nil {
		t.Fatal(e)
	}
	finishAlgorithms(t, s)
	if s.algorithm.view.Applied != 1 || !reflect.DeepEqual(chosen.route, want) {
		t.Fatalf("external route not applied: %+v; want %v got %v", s.algorithm.view, want, chosen.route)
	}
	if len(chosen.routeEntryLanes) != len(chosen.route) {
		t.Fatal("lane path lost")
	}
}
func TestExternalSignalWorkerRespectsClearance(t *testing.T) {
	s := New(workerGraph(t))
	defer s.Close()
	id := registerAlgorithmHelper(t, "signal", "valid")
	_ = s.setAlgorithms(AlgorithmConfig{Signal: AlgorithmChoice{ID: id}})
	finishAlgorithms(t, s)
	if s.algorithm.view.Applied != 1 {
		t.Fatal(s.algorithm.view)
	}
	for node, r := range s.signals {
		control := s.graph.Intersections[node].TrafficControl
		if len(control.Phases) < 2 {
			continue
		}
		current, want := r.PhaseIndex, r.externalPhase
		r.Stage = "amber"
		r.StageElapsed = 0
		r.LastServedPhase = current
		s.advanceSignalLocked(control, r, .01)
		if r.Stage != "amber" || r.PhaseIndex != current {
			t.Fatal("external schedule bypassed amber")
		}
		r.Stage = "all_red"
		r.StageElapsed = 0
		duration := control.Phases[current].AllRedSeconds
		if duration > .01 {
			s.advanceSignalLocked(control, r, .01)
			if r.Stage != "all_red" {
				t.Fatal("bypassed all-red")
			}
		}
		r.StageElapsed = 0
		s.advanceSignalLocked(control, r, duration)
		if r.Stage != "green" || r.PhaseIndex != want {
			t.Fatal("proposal did not select actual phase")
		}
		return
	}
	t.Fatal("no signal fixture")
}
func TestAlgorithmFailureUsesNativeFallback(t *testing.T) {
	for _, mode := range []string{"timeout", "crash", "malformed", "stale", "invalid"} {
		t.Run(mode, func(t *testing.T) {
			s := New(workerGraph(t))
			defer s.Close()
			id := registerAlgorithmHelper(t, "signal", mode)
			_ = s.setAlgorithms(AlgorithmConfig{Signal: AlgorithmChoice{ID: id}})
			finishAlgorithms(t, s)
			if s.algorithm.view.Fallbacks != 1 || s.clock.Paused {
				t.Fatal(s.algorithm.view)
			}
			for _, r := range s.signals {
				if r.externalUntil != 0 {
					t.Fatal("invalid result applied")
				}
			}
			s.Step(.05)
			if s.clock.Tick != 1 {
				t.Fatal("fallback did not advance simulation")
			}
		})
	}
}

func TestRouteValidationAllowsDestinationInsideEntryConnector(t *testing.T) {
	s := New(workerGraph(t))
	defer s.Close()
	for _, p := range s.people {
		p.state = "driving"
		p.wayID = p.route[0]
	}
	tasks := s.algorithmRoutes()
	if len(tasks) == 0 {
		t.Fatal("missing route fixture")
	}
	task := tasks[0]
	steps := append([]interchange.Step(nil), task.Options[0].Steps...)
	if len(steps) < 2 {
		t.Fatal("short fixture")
	}
	steps[len(steps)-1].ExitLane = steps[len(steps)-1].EntryLane
	task.DestinationAlong = 1
	if !s.validRoute(task, steps) {
		t.Fatal("valid endpoint before nominal entry was rejected")
	}
}

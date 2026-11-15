package dispatch

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func fixture() Input {
	return Input{Version: 1, Epoch: 2, Request: 3, Vehicles: []Vehicle{{ID: 1}, {ID: 2}}, Orders: []Order{{ID: 10}, {ID: 20}}, Candidates: []Candidate{{1, 10, 1, 10}, {2, 10, 2, 20}, {1, 20, 2, 20}, {2, 20, 100, 1000}}}
}
func TestBaselinesAndOptimalAssignment(t *testing.T) {
	in := fixture()
	for _, m := range List() {
		p, _ := New(m.ID)
		r, e := p.Decide(context.Background(), in)
		if e != nil {
			t.Fatal(e)
		}
		if e = Validate(in, r); e != nil {
			t.Fatal(m.ID, e)
		}
		if len(r.Assignments) != 2 {
			t.Fatal("missing assignment")
		}
		if m.ID == "hungarian" || m.ID == "batch" {
			for _, a := range r.Assignments {
				if a.VehicleID == 1 && a.OrderID != 20 {
					t.Fatal("not optimal")
				}
			}
		}
	}
}
func TestRejectBadProposals(t *testing.T) {
	in := fixture()
	for _, p := range []Proposal{{Version: 1, Epoch: 2, Request: 4}, {Version: 1, Epoch: 2, Request: 3, Assignments: []Pair{{1, 10}, {1, 20}}}, {Version: 1, Epoch: 2, Request: 3, Assignments: []Pair{{99, 10}}}} {
		if Validate(in, p) == nil {
			t.Fatal("bad proposal accepted")
		}
	}
	in.Candidates = nil
	p := optimal(in)
	if len(p) != 0 {
		t.Fatal("unreachable pair matched")
	}
}
func TestWorkerHelper(t *testing.T) {
	if os.Getenv("DISPATCH_TEST_HELPER") == "" {
		return
	}
	if os.Getenv("DISPATCH_TEST_HELPER") == "sleep" {
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	var in Input
	_ = json.NewDecoder(os.Stdin).Decode(&in)
	_ = json.NewEncoder(os.Stdout).Encode(Empty(in))
	os.Exit(0)
}
func TestWorkerProtocolAndTimeout(t *testing.T) {
	exe, _ := os.Executable()
	t.Setenv("DISPATCH_TEST_HELPER", "echo")
	w := worker{Manifest{Command: []string{exe, "-test.run=^TestWorkerHelper$"}, Timeout: 2000}}
	if _, e := w.Decide(context.Background(), fixture()); e != nil {
		t.Fatal(e)
	}
	t.Setenv("DISPATCH_TEST_HELPER", "sleep")
	w.m.Timeout = 100
	start := time.Now()
	if _, e := w.Decide(context.Background(), fixture()); e == nil {
		t.Fatal("timeout not enforced")
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("worker did not terminate")
	}
}

type mutatingPolicy struct{}

func (mutatingPolicy) Decide(_ context.Context, in Input) (Proposal, error) {
	in.Candidates[0].VehicleID = 99
	in.Vehicles[0].ID = 99
	in.Parameters["x"] = 123
	panic("test")
}
func TestNativeInputIsolationAndPanic(t *testing.T) {
	in := fixture()
	in.Parameters = map[string]float64{"x": 1}
	if _, e := Invoke(context.Background(), mutatingPolicy{}, in); e == nil {
		t.Fatal("panic escaped")
	}
	if in.Candidates[0].VehicleID != 1 || in.Vehicles[0].ID != 1 || in.Parameters["x"] != 1 {
		t.Fatal("policy mutated host snapshot")
	}
}

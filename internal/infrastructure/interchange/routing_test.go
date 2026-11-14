package interchange

import (
	"math"
	"math/rand"
	"reflect"
	"testing"

	model "traffic/internal/infrastructure/road"
)

func routingFixture(t *testing.T) (*model.Graph, *Planner) {
	t.Helper()
	c, err := model.LoadConfig("../../../example/Map2/road.json")
	if err != nil {
		t.Fatal(err)
	}
	g, err := model.BuildGraph(c)
	if err != nil {
		t.Fatal(err)
	}
	return g, NewPlanner(g)
}

func checkRoute(t *testing.T, p *Planner, req Request, r Route) {
	t.Helper()
	if len(r.Steps) == 0 || r.Steps[0].WayID != req.Start || r.Steps[len(r.Steps)-1].WayID != req.Finish {
		t.Fatalf("invalid endpoints: %+v", r)
	}
	if req.StartLane >= 0 && r.Steps[0].EntryLane != req.StartLane {
		t.Fatal("changed initial lane")
	}
	cost := 0.0
	for i, s := range r.Steps {
		w := p.g.Ways[s.WayID]
		if s.EntryLane < 0 || s.EntryLane >= w.Lanes || s.ExitLane < 0 || s.ExitLane >= w.Lanes {
			t.Fatal("invalid lane", s)
		}
		if s.EntryLane != s.ExitLane && (req.CanChange == nil || !req.CanChange(w, s.EntryLane, s.ExitLane, i == 0)) {
			t.Fatal("unplanned lane jump", s)
		}
		if i > 0 {
			c := w.Length / math.Max(1, w.SpeedLimit)
			if req.Cost != nil {
				c = req.Cost(w)
			}
			cost += c
		}
		if i+1 == len(r.Steps) {
			continue
		}
		next := r.Steps[i+1]
		if w.To != p.g.Ways[next.WayID].From {
			t.Fatal("disconnected roads")
		}
		m, ok := p.g.Intersections[w.To].TrafficControl.Movement(s.WayID, next.WayID, s.ExitLane)
		valid := false
		for _, c := range m.ConnectionsFrom(s.ExitLane) {
			valid = valid || c.ToLane == next.EntryLane
		}
		if !ok || !valid {
			t.Fatalf("illegal connector: %+v -> %+v", s, next)
		}
	}
	if math.Abs(cost-r.Cost) > 1e-6 {
		t.Fatal("transfer cost differs from expansion", cost, r.Cost)
	}
	for _, leg := range r.Legs {
		if leg.First < 0 || leg.Last >= len(r.Steps) || leg.Last <= leg.First {
			t.Fatal("invalid expansion range", leg)
		}
		for i := leg.First; i < leg.Last; i++ {
			if p.owner[p.g.Ways[r.Steps[i].WayID].To] != leg.Interchange {
				t.Fatal("local search escaped its macro node", leg)
			}
		}
	}
}

// The uncontracted search is an exact reference over the same legal lane graph.
// Compare reachability and cost, not tie-dependent road IDs.
func TestHierarchyMatchesDetailedLaneSearch(t *testing.T) {
	g, p := routingFixture(t)
	ways := g.SortedWays()
	rng := rand.New(rand.NewSource(813))
	var requests []Request
	for _, a := range ways {
		if !a.TripAccess {
			continue
		}
		for _, b := range ways {
			if b.TripAccess {
				requests = append(requests, Request{Start: a.ID, Finish: b.ID, StartLane: -1})
			}
		}
	}
	for i := 0; i < 250; i++ {
		a, b := ways[rng.Intn(len(ways))], ways[rng.Intn(len(ways))]
		r := Request{Start: a.ID, Finish: b.ID, StartLane: rng.Intn(a.Lanes)}
		if i%3 == 0 {
			r.CanChange = func(w *model.Way, a, b int, initial bool) bool { return w.Length > 300 }
		}
		if i%4 == 0 && len(g.Outgoing[a.To]) > 0 {
			r.BlockedFirst = g.Outgoing[a.To][0].ID
		}
		if i%5 == 0 {
			r.Cost = func(w *model.Way) float64 {
				if w.Role == "loop_ramp" {
					return w.Length * 20
				}
				return w.Length
			}
		}
		requests = append(requests, r)
	}
	macroTrips, reduced := 0, 0
	for _, req := range requests {
		r, ok := p.Route(req)
		flat, found := p.route(req, false)
		if ok != found || (ok && math.Abs(r.Cost-flat.Cost) > 1e-6) {
			t.Fatalf("hierarchy/reference mismatch %+v: %v %.5f / %v %.5f", req, ok, r.Cost, found, flat.Cost)
		}
		if !ok {
			continue
		}
		checkRoute(t, p, req, r)
		if len(r.Legs) > 0 {
			macroTrips++
			if r.MacroSettled < flat.MacroSettled {
				reduced++
			}
		}
	}
	if macroTrips < 100 || reduced < 100 {
		t.Fatal("search did not contract internal roads", macroTrips, reduced)
	}
	t.Logf("%d queries checked; %d macro routes; %d with fewer outer settled states", len(requests), macroTrips, reduced)
}

func TestHierarchyEndpointsCyclesAndCosts(t *testing.T) {
	g, p := routingFixture(t)
	for _, group := range g.Interchanges {
		for _, movement := range group.Routes {
			ids := movement.WayIDs
			for _, pair := range [][2]string{{ids[0], ids[len(ids)-1]}, {ids[1], ids[len(ids)-2]}, {ids[1], ids[1]}} {
				req := Request{Start: pair[0], Finish: pair[1], StartLane: -1, CanChange: func(*model.Way, int, int, bool) bool { return true }}
				r, ok := p.Route(req)
				if !ok {
					t.Fatal("missing movement or internal endpoint cycle", group.ID, pair)
				}
				checkRoute(t, p, req, r)
				if len(r.Legs) == 0 || len(r.Steps) < 2 {
					t.Fatal("missing actual macro expansion", r)
				}
				again, _ := p.Route(req)
				if !reflect.DeepEqual(r, again) {
					t.Fatal("nondeterministic route")
				}
			}
		}
	}
	start := g.SortedWays()[0]
	r, ok := p.Route(Request{Start: start.ID, Finish: start.ID, StartLane: 0, AllowDirect: true})
	if !ok || len(r.Steps) != 1 || r.Cost != 0 {
		t.Fatal("direct route must not make a cycle")
	}
	for _, req := range []Request{{Start: "missing", Finish: start.ID}, {Start: start.ID, Finish: start.ID, StartLane: start.Lanes}, {Start: start.ID, Finish: start.ID, Cost: func(*model.Way) float64 { return math.NaN() }}} {
		if _, ok := p.Route(req); ok {
			t.Fatal("invalid request accepted")
		}
	}
	// Costs must be fresh on every query, including paths hidden inside macro arcs.
	ids := g.Interchanges[0].Routes[0].WayIDs
	req := Request{Start: ids[0], Finish: ids[len(ids)-1], StartLane: -1}
	base, ok := p.Route(req)
	if !ok {
		t.Fatal("missing base route")
	}
	req.Cost = func(w *model.Way) float64 { return 7 * w.Length / math.Max(1, w.SpeedLimit) }
	changed, ok := p.Route(req)
	if !ok || math.Abs(changed.Cost-7*base.Cost) > 1e-6 {
		t.Fatal("stale macro costs")
	}
}

func TestMacroTransferRequiresRealLaneConnection(t *testing.T) {
	// The internal road receives lane 0 but can only exit from lane 1. A
	// port-only/all-to-all graph would incorrectly declare this always reachable.
	g := &model.Graph{Interchanges: []model.Interchange{{ID: "X"}}, Ways: map[string]*model.Way{}, Intersections: map[string]*model.Intersection{}, Outgoing: map[string][]*model.Way{}}
	for _, id := range []string{"A", "in", "out", "B"} {
		g.Intersections[id] = &model.Intersection{ID: id}
	}
	g.Intersections["in"].Interchange = "X"
	g.Intersections["out"].Interchange = "X"
	for _, w := range []*model.Way{{ID: "entry", From: "A", To: "in", Lanes: 1, Length: 100, SpeedLimit: 10}, {ID: "inside", From: "in", To: "out", Lanes: 2, Length: 300, SpeedLimit: 10}, {ID: "exit", From: "out", To: "B", Lanes: 1, Length: 100, SpeedLimit: 10}} {
		g.Ways[w.ID] = w
		g.Outgoing[w.From] = append(g.Outgoing[w.From], w)
	}
	for i, node := range []string{"in", "out"} {
		from, to, lane := "entry", "inside", 0
		if i == 1 {
			from, to, lane = "inside", "exit", 1
		}
		g.Intersections[node].TrafficControl = model.TrafficControl{Phases: []model.TrafficPhaseConfig{{AllowedMovements: []model.TrafficMovementConfig{{FromWayID: from, ToWayID: to, FromLanes: []int{lane}, ToLanes: []int{0}, LaneConnections: []model.LaneConnectionConfig{{FromLane: lane, ToLane: 0}}}}}}}
	}
	p := NewPlanner(g)
	req := Request{Start: "entry", Finish: "exit", StartLane: 0}
	if _, ok := p.Route(req); ok {
		t.Fatal("invented a lane connection")
	}
	req.CanChange = func(w *model.Way, from, to int, initial bool) bool { return w.Length >= 200 }
	r, ok := p.Route(req)
	if !ok {
		t.Fatal("legal lane preparation was not expanded")
	}
	checkRoute(t, p, req, r)
	if len(r.Legs) != 1 || len(r.Steps) != 3 || r.Steps[1].EntryLane != 0 || r.Steps[1].ExitLane != 1 {
		t.Fatal("entry and exit lanes were collapsed", r)
	}
	req.BlockedFirst = "inside"
	if _, ok := p.Route(req); ok {
		t.Fatal("blocked first ramp still used")
	}
	req.BlockedFirst = ""
	req.CanChange = func(*model.Way, int, int, bool) bool { return false }
	if _, ok := p.Route(req); ok {
		t.Fatal("unsafe preparation accepted")
	}
}

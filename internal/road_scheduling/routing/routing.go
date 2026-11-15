package routing

import (
	"container/heap"
	"math"
	"sort"

	model "traffic/internal/infrastructure/road"
)

// Step distinguishes the receiving connector from the lane needed at the next
// exit. A difference requires a physical lane change on this road, not a jump.
type Step struct {
	WayID     string `json:"way_id"`
	EntryLane int    `json:"entry_lane"`
	ExitLane  int    `json:"exit_lane"`
}

// Leg identifies an actual macro traversal and its inclusive expanded indices.
// EndpointInside is true when the trip starts or ends within the interchange.
type Leg struct {
	Interchange    string `json:"interchange"`
	EntryPort      string `json:"entry_port"`
	ExitPort       string `json:"exit_port"`
	First          int    `json:"first"`
	Last           int    `json:"last"`
	EndpointInside bool   `json:"endpoint_inside"`
}

type Route struct {
	Steps           []Step  `json:"steps"`
	Legs            []Leg   `json:"interchanges"`
	Cost            float64 `json:"cost"`
	MacroSettled    int     `json:"macro_settled"`
	InternalSettled int     `json:"internal_settled"`
}

func (r Route) Arrays() (ways []string, exits []int, entries []int) {
	for _, step := range r.Steps {
		ways = append(ways, step.WayID)
		exits = append(exits, step.ExitLane)
		entries = append(entries, step.EntryLane)
	}
	return
}

type Request struct {
	Start, Finish string
	StartLane     int // -1 permits any initial lane for a newly entering vehicle.
	BlockedFirst  string
	AllowDirect   bool
	Cost          func(*model.Way) float64
	// CanChange may authorize a lane change only when preparation distance is
	// available. The executor must still check gaps and perform the maneuver.
	CanChange func(way *model.Way, entry, exit int, initial bool) bool
}

type state struct {
	way      string
	lane     int
	departed bool // keeps a return to the initial road distinct from zero travel.
}
type edge struct {
	to   state
	exit int
}
type arc struct {
	to    state
	cost  float64
	edges []edge
	group string
}
type predecessor struct {
	from state
	arc  arc
}

// Planner caches topology only. Costs and local transfer paths are computed from
// one request snapshot so congestion/policy changes never reuse stale weights.
// The outer search visits ordinary roads and directional interchange boundaries;
// the local search alone visits the interchange's internal lane graph.
type Planner struct {
	g           *model.Graph
	owner       map[string]string
	connections map[state][]edge
}

func NewPlanner(g *model.Graph) *Planner {
	p := &Planner{g: g, owner: map[string]string{}, connections: map[state][]edge{}}
	groups := map[string]bool{}
	for _, group := range g.Interchanges {
		groups[group.ID] = true
	}
	for id, n := range g.Intersections {
		if groups[n.Interchange] {
			p.owner[id] = n.Interchange
		}
	}
	for _, w := range g.SortedWays() {
		n := g.Intersections[w.To]
		out := append([]*model.Way(nil), g.Outgoing[w.To]...)
		sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
		for lane := 0; lane < w.Lanes; lane++ {
			key := state{way: w.ID, lane: lane}
			for _, next := range out {
				m, ok := n.TrafficControl.Movement(w.ID, next.ID, lane)
				if !ok {
					continue
				}
				for _, c := range m.ConnectionsFrom(lane) {
					p.connections[key] = append(p.connections[key], edge{state{next.ID, c.ToLane, true}, lane})
				}
			}
		}
	}
	return p
}

type query struct {
	p        *Planner
	req      Request
	costs    map[string]float64
	internal int
}

func (p *Planner) Route(req Request) (Route, bool) { return p.route(req, true) }

func (p *Planner) route(req Request, contract bool) (Route, bool) {
	start, end := p.g.Ways[req.Start], p.g.Ways[req.Finish]
	if start == nil || end == nil || req.StartLane < -1 || req.StartLane >= start.Lanes {
		return Route{}, false
	}
	q := query{p: p, req: req, costs: map[string]float64{}}
	for _, w := range p.g.SortedWays() {
		cost := w.Length / math.Max(1, w.SpeedLimit)
		if req.Cost != nil {
			cost = req.Cost(w)
		}
		if cost <= 0 || math.IsNaN(cost) || math.IsInf(cost, 0) {
			return Route{}, false
		}
		q.costs[w.ID] = cost
	}
	dist := map[state]float64{}
	prev := map[state]predecessor{}
	queue := priorityQueue{}
	for lane := 0; lane < start.Lanes; lane++ {
		if req.StartLane >= 0 && lane != req.StartLane {
			continue
		}
		s := state{way: start.ID, lane: lane}
		dist[s] = 0
		heap.Push(&queue, item{s, 0})
	}
	settled := map[state]bool{}
	for queue.Len() > 0 {
		it := heap.Pop(&queue).(item)
		cur := it.s
		if settled[cur] || it.cost > dist[cur]+1e-9 {
			continue
		}
		settled[cur] = true
		if q.goal(cur) {
			var reversed []predecessor
			for cursor := cur; ; {
				v, ok := prev[cursor]
				if !ok {
					break
				}
				reversed = append(reversed, v)
				cursor = v.from
			}
			root := cur
			if len(reversed) > 0 {
				root = reversed[len(reversed)-1].from
			}
			r := Route{Cost: it.cost, MacroSettled: len(settled), InternalSettled: q.internal, Steps: []Step{{root.way, root.lane, root.lane}}}
			for i := len(reversed) - 1; i >= 0; i-- {
				a := reversed[i].arc
				first := len(r.Steps) - 1
				for _, e := range a.edges {
					r.Steps[len(r.Steps)-1].ExitLane = e.exit
					r.Steps = append(r.Steps, Step{e.to.way, e.to.lane, e.to.lane})
				}
				if a.group != "" {
					entry, exit := p.g.Ways[r.Steps[first].WayID], p.g.Ways[r.Steps[len(r.Steps)-1].WayID]
					r.Legs = append(r.Legs, Leg{a.group, entry.To, exit.From, first, len(r.Steps) - 1, p.owner[entry.From] == a.group || p.owner[exit.To] == a.group})
				}
			}
			return r, true
		}
		group := p.owner[p.g.Ways[cur.way].To]
		var arcs []arc
		if contract && group != "" {
			arcs = q.transfers(cur, group)
		} else {
			for _, e := range q.neighbors(cur) {
				arcs = append(arcs, arc{e.to, q.costs[e.to.way], []edge{e}, ""})
			}
		}
		for _, a := range arcs {
			candidate := it.cost + a.cost
			if old, ok := dist[a.to]; !settled[a.to] && (!ok || candidate < old-1e-9) {
				dist[a.to] = candidate
				prev[a.to] = predecessor{cur, a}
				heap.Push(&queue, item{a.to, candidate})
			}
		}
	}
	return Route{}, false
}

func (q *query) goal(s state) bool { return s.way == q.req.Finish && (s.departed || q.req.AllowDirect) }

func (q *query) neighbors(s state) []edge {
	w := q.p.g.Ways[s.way]
	var out []edge
	for exit := 0; exit < w.Lanes; exit++ {
		if exit != s.lane && (q.req.CanChange == nil || !q.req.CanChange(w, s.lane, exit, !s.departed)) {
			continue
		}
		for _, e := range q.p.connections[state{way: s.way, lane: exit}] {
			if !s.departed && e.to.way == q.req.BlockedFirst {
				continue
			}
			out = append(out, e)
		}
	}
	return out
}

// transfers computes exact lane-sensitive ingress-to-egress costs for one macro
// node. It stops on leaving this node, never walking through another interchange.
// A destination inside the node is an additional temporary exit for this query.
func (q *query) transfers(start state, group string) []arc {
	dist := map[state]float64{start: 0}
	prev := map[state]predecessor{}
	queue := priorityQueue{{start, 0}}
	settled := map[state]bool{}
	var out []arc
	for queue.Len() > 0 {
		it := heap.Pop(&queue).(item)
		cur := it.s
		if settled[cur] || it.cost > dist[cur]+1e-9 {
			continue
		}
		settled[cur] = true
		q.internal++
		if cur != start && (q.goal(cur) || q.p.owner[q.p.g.Ways[cur.way].To] != group) {
			var reversed []edge
			for cursor := cur; cursor != start; {
				v := prev[cursor]
				reversed = append(reversed, v.arc.edges[0])
				cursor = v.from
			}
			edges := make([]edge, len(reversed))
			for i, e := range reversed {
				edges[len(edges)-1-i] = e
			}
			out = append(out, arc{cur, it.cost, edges, group})
			continue
		}
		for _, e := range q.neighbors(cur) {
			candidate := it.cost + q.costs[e.to.way]
			if old, ok := dist[e.to]; !settled[e.to] && (!ok || candidate < old-1e-9) {
				dist[e.to] = candidate
				prev[e.to] = predecessor{cur, arc{edges: []edge{e}}}
				heap.Push(&queue, item{e.to, candidate})
			}
		}
	}
	return out
}

type item struct {
	s    state
	cost float64
}
type priorityQueue []item

func (q priorityQueue) Len() int { return len(q) }
func (q priorityQueue) Less(i, j int) bool {
	if q[i].cost != q[j].cost {
		return q[i].cost < q[j].cost
	}
	a, b := q[i].s, q[j].s
	if a.way != b.way {
		return a.way < b.way
	}
	if a.lane != b.lane {
		return a.lane < b.lane
	}
	return !a.departed && b.departed
}
func (q priorityQueue) Swap(i, j int) { q[i], q[j] = q[j], q[i] }
func (q *priorityQueue) Push(v any)   { *q = append(*q, v.(item)) }
func (q *priorityQueue) Pop() any     { old := *q; v := old[len(old)-1]; *q = old[:len(old)-1]; return v }

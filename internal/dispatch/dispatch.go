package dispatch

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

const Version = 1

type Vehicle struct {
	ID     int     `json:"id"`
	Idle   float64 `json:"idle_seconds"`
	Income float64 `json:"income"`
}
type Order struct {
	ID       int     `json:"id"`
	Created  float64 `json:"created_s"`
	Deadline float64 `json:"deadline_s"`
}
type Candidate struct {
	VehicleID int     `json:"vehicle_id"`
	OrderID   int     `json:"order_id"`
	ETA       float64 `json:"pickup_eta_s"`
	Distance  float64 `json:"pickup_distance_m"`
}
type Input struct {
	Version    int                `json:"contract_version"`
	Epoch      uint64             `json:"epoch"`
	Request    uint64             `json:"request_id"`
	Time       float64            `json:"sim_time_s"`
	Seed       int64              `json:"seed"`
	Vehicles   []Vehicle          `json:"vehicles"`
	Orders     []Order            `json:"orders"`
	Candidates []Candidate        `json:"candidates"`
	Parameters map[string]float64 `json:"parameters,omitempty"`
}
type Pair struct {
	VehicleID int `json:"vehicle_id"`
	OrderID   int `json:"order_id"`
}
type Proposal struct {
	Version     int    `json:"contract_version"`
	Epoch       uint64 `json:"epoch"`
	Request     uint64 `json:"request_id"`
	Assignments []Pair `json:"assignments"`
}
type Policy interface {
	Decide(context.Context, Input) (Proposal, error)
}
type Factory func() Policy
type Metadata struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

var mu sync.RWMutex
var registry = map[string]struct {
	Metadata
	New Factory
}{}

func Register(m Metadata, f Factory) error {
	mu.Lock()
	defer mu.Unlock()
	if m.ID == "" || f == nil {
		return fmt.Errorf("invalid policy registration")
	}
	if _, ok := registry[m.ID]; ok {
		return fmt.Errorf("duplicate policy %s", m.ID)
	}
	registry[m.ID] = struct {
		Metadata
		New Factory
	}{m, f}
	return nil
}
func New(id string) (Policy, error) {
	mu.RLock()
	defer mu.RUnlock()
	v, ok := registry[id]
	if !ok {
		return nil, fmt.Errorf("unknown dispatch policy %s", id)
	}
	return v.New(), nil
}
func List() []Metadata {
	mu.RLock()
	defer mu.RUnlock()
	v := []Metadata{}
	for _, p := range registry {
		v = append(v, p.Metadata)
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}
func Validate(in Input, p Proposal) error {
	if p.Version != Version || p.Epoch != in.Epoch || p.Request != in.Request {
		return fmt.Errorf("stale or incompatible proposal")
	}
	allowed := map[Pair]bool{}
	for _, c := range in.Candidates {
		allowed[Pair{c.VehicleID, c.OrderID}] = true
	}
	vs, os := map[int]bool{}, map[int]bool{}
	for _, a := range p.Assignments {
		if !allowed[a] || vs[a.VehicleID] || os[a.OrderID] {
			return fmt.Errorf("duplicate or forbidden assignment")
		}
		vs[a.VehicleID] = true
		os[a.OrderID] = true
	}
	return nil
}
func Empty(in Input) Proposal {
	return Proposal{Version: Version, Epoch: in.Epoch, Request: in.Request, Assignments: []Pair{}}
}

type builtin string

func init() {
	for _, id := range []string{"nearest", "fifo", "greedy", "hungarian", "batch", "score"} {
		id := id
		_ = Register(Metadata{id, map[string]string{"nearest": "Nearest ETA", "fifo": "FIFO", "greedy": "Global greedy", "hungarian": "Hungarian", "batch": "Batch Hungarian", "score": "Weighted score"}[id], "native"}, func() Policy { return builtin(id) })
	}
}
func (b builtin) Decide(ctx context.Context, in Input) (Proposal, error) {
	p := Empty(in)
	if b == "hungarian" || b == "batch" {
		p.Assignments = optimal(in)
		return p, nil
	}
	cs := append([]Candidate(nil), in.Candidates...)
	orders := map[int]Order{}
	vehicles := map[int]Vehicle{}
	for _, o := range in.Orders {
		orders[o.ID] = o
	}
	for _, v := range in.Vehicles {
		vehicles[v.ID] = v
	}
	score := func(c Candidate) float64 {
		w := in.Parameters["wait_weight"]
		if w == 0 {
			w = .25
		}
		return c.ETA - w*(in.Time-orders[c.OrderID].Created) - .12*vehicles[c.VehicleID].Idle + .08*vehicles[c.VehicleID].Income
	}
	sort.SliceStable(cs, func(i, j int) bool {
		a, z := cs[i], cs[j]
		if b == "nearest" || b == "fifo" {
			if orders[a.OrderID].Created != orders[z.OrderID].Created {
				return orders[a.OrderID].Created < orders[z.OrderID].Created
			}
			if a.OrderID != z.OrderID {
				return a.OrderID < z.OrderID
			}
		}
		if b == "fifo" && vehicles[a.VehicleID].Idle != vehicles[z.VehicleID].Idle {
			return vehicles[a.VehicleID].Idle > vehicles[z.VehicleID].Idle
		}
		av, zv := a.ETA, z.ETA
		if b == "score" {
			av, zv = score(a), score(z)
		}
		if av != zv {
			return av < zv
		}
		if a.OrderID != z.OrderID {
			return a.OrderID < z.OrderID
		}
		return a.VehicleID < z.VehicleID
	})
	vs, os := map[int]bool{}, map[int]bool{}
	for _, c := range cs {
		if err := ctx.Err(); err != nil {
			return p, err
		}
		if !vs[c.VehicleID] && !os[c.OrderID] {
			p.Assignments = append(p.Assignments, Pair{c.VehicleID, c.OrderID})
			vs[c.VehicleID] = true
			os[c.OrderID] = true
		}
	}
	return p, nil
}

// Rectangular shortest augmenting path assignment; private dummy columns allow unmatched orders.
func optimal(in Input) []Pair {
	n := len(in.Orders)
	m := len(in.Vehicles) + n
	if n == 0 {
		return []Pair{}
	}
	cost := make([][]float64, n+1)
	oi, vi := map[int]int{}, map[int]int{}
	for i, o := range in.Orders {
		oi[o.ID] = i + 1
	}
	for j, v := range in.Vehicles {
		vi[v.ID] = j + 1
	}
	for i := 1; i <= n; i++ {
		cost[i] = make([]float64, m+1)
		for j := 1; j <= m; j++ {
			cost[i][j] = 1e12
			if j > len(in.Vehicles) {
				cost[i][j] = 1e9
			}
		}
	}
	for _, c := range in.Candidates {
		cost[oi[c.OrderID]][vi[c.VehicleID]] = c.ETA
	}
	u, v := make([]float64, n+1), make([]float64, m+1)
	p, way := make([]int, m+1), make([]int, m+1)
	for i := 1; i <= n; i++ {
		p[0] = i
		j0 := 0
		mins := make([]float64, m+1)
		used := make([]bool, m+1)
		for j := 1; j <= m; j++ {
			mins[j] = 1e30
		}
		for {
			used[j0] = true
			i0 := p[j0]
			delta := 1e30
			j1 := 0
			for j := 1; j <= m; j++ {
				if !used[j] {
					cur := cost[i0][j] - u[i0] - v[j]
					if cur < mins[j] {
						mins[j] = cur
						way[j] = j0
					}
					if mins[j] < delta {
						delta = mins[j]
						j1 = j
					}
				}
			}
			for j := 0; j <= m; j++ {
				if used[j] {
					u[p[j]] += delta
					v[j] -= delta
				} else {
					mins[j] -= delta
				}
			}
			j0 = j1
			if p[j0] == 0 {
				break
			}
		}
		for {
			j1 := way[j0]
			p[j0] = p[j1]
			j0 = j1
			if j0 == 0 {
				break
			}
		}
	}
	out := []Pair{}
	for j := 1; j <= len(in.Vehicles); j++ {
		if p[j] > 0 && cost[p[j]][j] < 1e9 {
			out = append(out, Pair{in.Vehicles[j-1].ID, in.Orders[p[j]-1].ID})
		}
	}
	return out
}

// Invoke contains policy panics without granting a plugin any state mutation rights.
func Invoke(ctx context.Context, p Policy, in Input) (out Proposal, err error) {
	defer func() {
		if r := recover(); r != nil {
			out = Empty(in)
			err = fmt.Errorf("policy panic: %v", r)
		}
	}()
	copyInput := in
	copyInput.Vehicles = append([]Vehicle(nil), in.Vehicles...)
	copyInput.Orders = append([]Order(nil), in.Orders...)
	copyInput.Candidates = append([]Candidate(nil), in.Candidates...)
	copyInput.Parameters = make(map[string]float64, len(in.Parameters))
	for k, v := range in.Parameters {
		copyInput.Parameters[k] = v
	}
	return p.Decide(ctx, copyInput)
}

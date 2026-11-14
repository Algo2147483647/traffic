package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"traffic/internal/algorithms"
	"traffic/internal/infrastructure/interchange"
)

type AlgorithmChoice struct {
	ID         string             `json:"id"`
	Parameters map[string]float64 `json:"parameters"`
}
type AlgorithmConfig struct {
	Route  AlgorithmChoice `json:"route"`
	Signal AlgorithmChoice `json:"signal"`
}
type AlgorithmView struct {
	Config    AlgorithmConfig `json:"config"`
	Pending   bool            `json:"pending"`
	Applied   int             `json:"applied"`
	Fallbacks int             `json:"fallbacks"`
	LastError string          `json:"last_error"`
}
type routeTask struct {
	Entity           int                 `json:"entity"`
	Start            string              `json:"start"`
	Finish           string              `json:"finish"`
	Lane             int                 `json:"lane"`
	Along            float64             `json:"along"`
	DestinationAlong float64             `json:"destination_along"`
	Options          []interchange.Route `json:"options"`
}
type routeResult struct {
	Entity int                `json:"entity"`
	Steps  []interchange.Step `json:"steps"`
}
type signalTask struct {
	Intersection string         `json:"intersection"`
	Current      int            `json:"current"`
	Stage        string         `json:"stage"`
	Phases       []signalOption `json:"phases"`
}
type signalOption struct {
	Index    int     `json:"index"`
	Min      float64 `json:"min_green_s"`
	Max      float64 `json:"max_green_s"`
	Pressure float64 `json:"pressure"`
}
type signalResult struct {
	Intersection string  `json:"intersection"`
	Phase        int     `json:"phase"`
	Green        float64 `json:"green_s"`
}
type algorithmResult struct {
	input    algorithms.Request
	response algorithms.Response
	err      error
	routes   []routeTask
	signals  []signalTask
}
type algorithmRuntime struct {
	view      AlgorithmView
	cancel    context.CancelFunc
	pending   chan algorithmResult
	remaining int
	next      uint64
	request   uint64
	cursor    int
}

func (s *Simulation) cancelAlgorithms() {
	if s.algorithm.cancel != nil {
		s.algorithm.cancel()
	}
	s.algorithm.pending = nil
	s.algorithm.remaining = 0
	s.algorithm.view.Pending = false
	s.algorithm.next = 0
}
func (s *Simulation) setAlgorithms(c AlgorithmConfig) error {
	for cap, choice := range map[string]AlgorithmChoice{"route": c.Route, "signal": c.Signal} {
		if choice.ID != "" && choice.ID != "native" {
			m, ok := algorithms.Lookup(choice.ID)
			if !ok || m.Capability != cap {
				return fmt.Errorf("unknown %s algorithm", cap)
			}
		}
		for _, v := range choice.Parameters {
			if math.IsNaN(v) || math.IsInf(v, 0) {
				return fmt.Errorf("nonfinite parameter")
			}
		}
	}
	s.cancelAlgorithms()
	for _, r := range s.signals {
		r.externalUntil = 0
	}
	s.algorithm.view.Config = c
	return nil
}
func (s *Simulation) algorithmView() AlgorithmView {
	v := s.algorithm.view
	b, _ := json.Marshal(v.Config)
	v.Config = AlgorithmConfig{}
	_ = json.Unmarshal(b, &v.Config)
	return v
}
func (s *Simulation) advanceAlgorithmsLocked() bool {
	a := &s.algorithm
	if a.pending != nil {
		for a.remaining > 0 {
			select {
			case result := <-a.pending:
				s.commitAlgorithm(result)
				a.remaining--
			default:
				return false
			}
		}
		a.cancel()
		a.cancel = nil
		a.pending = nil
		a.view.Pending = false
		a.next = s.clock.Tick + 100
		return true
	}
	if s.clock.Tick < a.next {
		return true
	}
	type work struct {
		cap     string
		choice  AlgorithmChoice
		routes  []routeTask
		signals []signalTask
		payload any
	}
	jobs := []work{}
	if c := a.view.Config.Route; c.ID != "" && c.ID != "native" {
		tasks := s.algorithmRoutes()
		if len(tasks) > 0 {
			jobs = append(jobs, work{cap: "route", choice: c, routes: tasks, payload: map[string]any{"tasks": tasks, "ways": s.graph.SortedWays(), "intersections": s.graph.SortedIntersections()}})
		}
	}
	if c := a.view.Config.Signal; c.ID != "" && c.ID != "native" {
		tasks := s.algorithmSignals()
		if len(tasks) > 0 {
			jobs = append(jobs, work{cap: "signal", choice: c, signals: tasks, payload: map[string]any{"tasks": tasks}})
		}
	}
	if len(jobs) == 0 {
		a.next = s.clock.Tick + 100
		return true
	}
	ctx, cancel := context.WithCancel(context.Background())
	a.cancel = cancel
	a.pending = make(chan algorithmResult, len(jobs))
	a.remaining = len(jobs)
	a.view.Pending = true
	for _, j := range jobs {
		a.request++
		m, _ := algorithms.Lookup(j.choice.ID)
		body, _ := json.Marshal(j.payload)
		if j.choice.Parameters == nil {
			j.choice.Parameters = map[string]float64{}
		}
		in := algorithms.Request{Version: 1, Algorithm: m.ID, AlgorithmVersion: m.AlgorithmVersion, Capability: j.cap, Stamp: s.stamp, Tick: s.clock.Tick, Request: a.request, Seed: s.graph.Simulation.Seed, Parameters: j.choice.Parameters, Payload: body}
		ch := a.pending
		go func(w work, in algorithms.Request) {
			r, e := algorithms.Invoke(ctx, in)
			ch <- algorithmResult{in, r, e, w.routes, w.signals}
		}(j, in)
	}
	return false
}
func (s *Simulation) algorithmRoutes() []routeTask {
	tasks := []routeTask{}
	n := len(s.people)
	if n == 0 {
		return tasks
	}
	for i := 0; i < n && len(tasks) < 8; i++ {
		p := s.people[(s.algorithm.cursor+i)%n]
		if (p.state != "driving" && p.state != "waiting") || p.laneChanging || p.routeIndex+1 >= len(p.route) {
			continue
		}
		start, finish := p.wayID, p.route[len(p.route)-1]
		base, ok := s.congestionRoute(start, finish, "", p.lane, p.along)
		if !ok {
			continue
		}
		task := routeTask{Entity: p.id, Start: start, Finish: finish, Lane: p.lane, Along: p.along, DestinationAlong: p.destinationAlong, Options: []interchange.Route{base}}
		if len(base.Steps) > 1 {
			if alt, ok := s.congestionRoute(start, finish, base.Steps[1].WayID, p.lane, p.along); ok {
				task.Options = append(task.Options, alt)
			}
		}
		validOptions := make([]interchange.Route, 0, len(task.Options))
		for _, option := range task.Options {
			if s.validRoute(task, option.Steps) {
				validOptions = append(validOptions, option)
			}
		}
		task.Options = validOptions
		if len(task.Options) > 0 {
			tasks = append(tasks, task)
		}
	}
	s.algorithm.cursor = (s.algorithm.cursor + 8) % n
	return tasks
}
func (s *Simulation) algorithmSignals() []signalTask {
	q := s.signalQueuesLocked()
	out := []signalTask{}
	ids := []string{}
	for id := range s.signals {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		r := s.signals[id]
		control := s.graph.Intersections[id].TrafficControl
		d := (MaxPressureScheduler{}).Evaluate(control, q, r.LastServedPhase)
		task := signalTask{Intersection: id, Current: r.PhaseIndex, Stage: r.Stage}
		for i, p := range control.Phases {
			min, max := p.MinGreenSeconds, p.MaxGreenSeconds
			if min <= 0 {
				min = p.DurationSeconds
			}
			if max < min {
				max = p.DurationSeconds
			}
			task.Phases = append(task.Phases, signalOption{i, min, max, d.Phases[i].Pressure})
		}
		out = append(out, task)
	}
	return out
}
func (s *Simulation) validRoute(task routeTask, steps []interchange.Step) bool {
	if len(steps) == 0 || len(steps) > len(s.graph.Ways)*2 || steps[0].WayID != task.Start || steps[len(steps)-1].WayID != task.Finish || steps[0].EntryLane != task.Lane {
		return false
	}
	for i, st := range steps {
		w := s.graph.Ways[st.WayID]
		if w == nil || st.EntryLane < 0 || st.EntryLane >= w.Lanes || st.ExitLane < 0 || st.ExitLane >= w.Lanes {
			return false
		}
		begin := math.Min(s.graph.Simulation.Intersection, w.Length)
		if i == 0 {
			begin = task.Along
		}
		end := s.stopLine(w)
		if i == len(steps)-1 {
			end = task.DestinationAlong
			if i > 0 {
				begin = math.Min(begin, end)
			}
		}
		needed := float64(absInt(st.EntryLane-st.ExitLane)) * (w.SpeedLimit*laneChangeBaseDuration + s.minimumVehicleSpacing())
		if end-begin+1e-6 < needed {
			return false
		}
		if i+1 < len(steps) {
			next := steps[i+1]
			nw := s.graph.Ways[next.WayID]
			if nw == nil || w.To != nw.From {
				return false
			}
			m, ok := s.graph.Intersections[w.To].TrafficControl.Movement(w.ID, nw.ID, st.ExitLane)
			if !ok || !laneIn(m.TargetLanes(st.ExitLane), next.EntryLane) {
				return false
			}
		}
	}
	return true
}
func (s *Simulation) commitAlgorithm(result algorithmResult) {
	a := &s.algorithm
	err := result.err
	if err == nil && (result.input.Stamp != s.stamp || result.input.Tick != s.clock.Tick) {
		err = fmt.Errorf("stale algorithm result")
	}
	if err == nil && result.input.Capability == "route" {
		var out struct {
			Routes []routeResult `json:"routes"`
		}
		err = algorithms.Decode(result.response.Payload, &out)
		if err == nil && len(out.Routes) != len(result.routes) {
			err = fmt.Errorf("incomplete route response")
		}
		tasks := map[int]routeTask{}
		people := map[int]*personState{}
		for _, x := range result.routes {
			tasks[x.Entity] = x
		}
		for _, p := range s.people {
			people[p.id] = p
		}
		seen := map[int]bool{}
		if err == nil {
			for _, r := range out.Routes {
				task, ok := tasks[r.Entity]
				p := people[r.Entity]
				if !ok || seen[r.Entity] || p == nil || p.wayID != task.Start || p.along != task.Along || !s.validRoute(task, r.Steps) {
					err = fmt.Errorf("invalid lane path for %d", r.Entity)
					break
				}
				seen[r.Entity] = true
			}
		}
		if err == nil {
			for _, r := range out.Routes {
				p := people[r.Entity]
				plan := interchange.Route{Steps: r.Steps}
				p.route, p.routeLanes, p.routeEntryLanes = plan.Arrays()
				p.routeIndex = 0
				delete(s.reservations, p.id)
			}
		}
	}
	if err == nil && result.input.Capability == "signal" {
		var out struct {
			Signals []signalResult `json:"signals"`
		}
		err = algorithms.Decode(result.response.Payload, &out)
		if err == nil && len(out.Signals) != len(result.signals) {
			err = fmt.Errorf("incomplete signal response")
		}
		tasks := map[string]signalTask{}
		for _, x := range result.signals {
			tasks[x.Intersection] = x
		}
		seen := map[string]bool{}
		if err == nil {
			for _, v := range out.Signals {
				t, ok := tasks[v.Intersection]
				if !ok || seen[v.Intersection] || v.Phase < 0 || v.Phase >= len(t.Phases) || math.IsNaN(v.Green) || math.IsInf(v.Green, 0) {
					err = fmt.Errorf("invalid phase proposal")
					break
				}
				p := t.Phases[v.Phase]
				if v.Green < p.Min || v.Green > p.Max {
					err = fmt.Errorf("green duration outside bounds")
					break
				}
				seen[v.Intersection] = true
			}
		}
		if err == nil {
			for _, v := range out.Signals {
				r := s.signals[v.Intersection]
				r.externalPhase = v.Phase
				r.externalGreen = v.Green
				r.externalUntil = s.clock.Tick + 200
			}
		}
	}
	if err != nil {
		if result.input.Capability == "signal" {
			for _, r := range s.signals {
				r.externalUntil = 0
			}
		}
		if result.input.Capability == "route" && result.input.Stamp == s.stamp && result.input.Tick == s.clock.Tick {
			for _, task := range result.routes {
				if len(task.Options) == 0 {
					continue
				}
				for _, p := range s.people {
					if p.id == task.Entity && p.wayID == task.Start && p.along == task.Along && s.validRoute(task, task.Options[0].Steps) {
						p.route, p.routeLanes, p.routeEntryLanes = task.Options[0].Arrays()
						p.routeIndex = 0
						delete(s.reservations, p.id)
					}
				}
			}
		}
		a.view.Fallbacks++
		a.view.LastError = err.Error()
		s.journal.Record(s.clock.Time, "algorithm", 0, result.input.Capability+":native fallback: "+err.Error())
	} else {
		a.view.Applied++
		a.view.LastError = ""
		s.journal.Record(s.clock.Time, "algorithm", 0, result.input.Capability+":applied")
	}
}

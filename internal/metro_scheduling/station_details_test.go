package metroscheduling

import (
	"math"
	"testing"
	track "traffic/internal/infrastructure/metro"
)

func TestExchangeDetailsMatchLiveQueues(t *testing.T) {
	l, err := track.LoadLayout("../../example/Map1/metro.json")
	if err != nil {
		t.Fatal(err)
	}
	c := l.Defaults
	c.Demand = 0
	r := NewNetwork(l, c)
	r.UseSharedDemand()
	x := l.Transfers[0]
	a, b := r.globalStation(x.A), r.globalStation(x.B)
	if !r.AddJourney(a, b, 17) || !r.AddJourney(b, r.globalStation([2]int{3, 1}), 18) {
		t.Fatal("admission")
	}
	v := r.Snapshot()
	detail := v.StationDetails[0]
	if detail.Transferring != 1 || len(detail.PassengerIDs) != 2 || len(detail.Platforms) != 2 {
		t.Fatalf("detail %+v", detail)
	}
	waiting := 0
	for _, p := range detail.Platforms {
		line, local := r.stationOwner(p.Station)
		for _, d := range p.Directions {
			if d.Waiting != len(line.queues[local][directionIndex(d.Direction)]) {
				t.Fatal("wrong directional queue")
			}
			waiting += d.Waiting
			if d.ETA == nil || math.IsNaN(*d.ETA) || *d.ETA < 0 {
				t.Fatalf("bad ETA %+v", d)
			}
		}
	}
	if waiting != 1 {
		t.Fatal("walking counted as waiting", waiting)
	}
	for _, line := range r.lines {
		line.config.Enabled = false
	}
	v = r.Snapshot()
	for _, d := range v.StationDetails[0].Platforms[0].Directions {
		if d.ETA != nil || d.Status != "paused" {
			t.Fatal("paused forecast")
		}
	}
}
func TestArrivalForecastProgressAndDirection(t *testing.T) {
	l, _ := track.LoadLayout("../../example/Map1/metro.json")
	r := NewNetwork(l, l.Defaults).lines[3]
	tr := r.trains[0]
	tr.Station = 0
	tr.Next = 1
	tr.Direction = 1
	tr.State = "moving"
	tr.Authority = r.lengths[0]
	tr.Distance = 100
	tr.Speed = 20
	a, _, ok := r.arrivalEstimate(tr, 1, 1)
	if !ok || a <= 0 {
		t.Fatal("missing approaching arrival")
	}
	tr.Distance += 100
	b, _, _ := r.arrivalEstimate(tr, 1, 1)
	if b >= a {
		t.Fatal("arrival did not approach")
	}
	reverse, _, ok := r.arrivalEstimate(tr, 1, -1)
	if ok {
		t.Fatal("ring train cannot reverse", reverse)
	}
	tr.State = "dwelling"
	tr.Station = 1
	c, status, ok := r.arrivalEstimate(tr, 1, 1)
	if !ok || c != 0 || status != "at platform" {
		t.Fatal("docked train")
	}
}

func TestTurningArrivalUsesRemainingLoop(t *testing.T) {
	l, _ := track.LoadLayout("../../example/Map1/metro.json")
	r := NewNetwork(l, l.Defaults).lines[0]
	tr := r.trains[0]
	tr.State = "turning"
	tr.Authority = 200
	tr.Distance = 50
	eta, _, ok := r.arrivalEstimate(tr, tr.Station, tr.Direction)
	if !ok || math.Abs(eta-150/r.spec.LoopSpeed) > 1e-6 {
		t.Fatal("terminal loop forecast", eta)
	}
}

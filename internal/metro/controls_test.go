package metro

import "testing"

func TestControlsAffectMotionAndKeepAuthority(t *testing.T) {
	c := DefaultConfig()
	c.Demand = 0
	c.Dwell = 5
	c.Headway = 30
	a, b := New([]Point{{0, 0}, {2000, 0}, {4000, 0}}, c), New([]Point{{0, 0}, {2000, 0}, {4000, 0}}, c)
	for i := 0; i < 800; i++ {
		a.Advance(.05)
		b.Advance(.05)
	}
	var moving *train
	for _, v := range b.trains {
		if v.State == "moving" && v.Speed > 1 {
			moving = v
			break
		}
	}
	if moving == nil {
		t.Fatal("fixture has no moving train")
	}
	before := moving.Speed
	if e := b.ApplyControls([]Control{{Train: moving.ID, Speed: 0, DepartAfter: b.time}}); e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 40; i++ {
		a.Advance(.05)
		b.Advance(.05)
	}
	if moving.Speed >= before {
		t.Fatal("desired control did not brake actual train")
	}
	if moving.Distance > moving.Authority {
		t.Fatal("authority exceeded")
	}
	b.ClearControls()
	if moving.controlUntil != 0 {
		t.Fatal("native selection retained override")
	}
}
func TestDepartureProposalCannotSkipDwellOrGrantTrack(t *testing.T) {
	c := DefaultConfig()
	c.Demand = 0
	c.Dwell = 5
	r := New([]Point{{0, 0}, {2000, 0}, {4000, 0}}, c)
	target := r.trains[0]
	if e := r.ApplyControls([]Control{{Train: target.ID, Speed: 10, DepartAfter: 0}}); e != nil {
		t.Fatal(e)
	}
	r.Advance(.05)
	if target.State == "moving" {
		t.Fatal("worker bypassed dwell")
	}
	// Invalid batches must not commit the valid prefix.
	if e := r.ApplyControls([]Control{{Train: target.ID, Speed: 1, DepartAfter: r.time}, {Train: -1, Speed: 1, DepartAfter: r.time}}); e == nil {
		t.Fatal("unknown train accepted")
	}
	if target.control.Speed != 10 {
		t.Fatal("partially committed invalid batch")
	}
}

func TestLateSharedMetroPassengersComplete(t *testing.T) {
	l, e := LoadLayout("../../example/Map2/metro.json")
	if e != nil {
		t.Fatal(e)
	}
	c := l.Defaults
	c.Demand = 0
	r := NewNetwork(l, c)
	r.UseSharedDemand()
	for i := 0; i < 30000; i++ {
		r.Advance(.05)
	}
	c.Headway = 60
	c.Dwell = 5
	if e := r.Configure(c); e != nil {
		t.Fatal(e)
	}
	for id, pair := range r.ServedPairs() {
		if !r.AddJourney(pair[0], pair[1], id+1) {
			t.Fatal("admission failed")
		}
	}
	for i := 0; i < 30000; i++ {
		r.Advance(.05)
	}
	if len(r.DrainCompletedJourneys()) != len(r.ServedPairs()) || len(r.ActiveJourneys()) != 0 {
		t.Fatalf("late passengers stuck: completed=%d trains=%+v", r.Snapshot().Completed, r.Snapshot().Trains)
	}
}

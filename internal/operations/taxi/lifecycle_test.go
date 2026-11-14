package taxi

import "testing"

func TestAcceptedTaskRemainsExclusiveThroughDwell(t *testing.T) {
	v := NewVehicle(1, 0)
	order := &Order{ID: 1, State: "Waiting"}
	next := &Order{ID: 2, State: "Waiting"}
	if !v.Assign(order, 0, 10) {
		t.Fatal("assignment")
	}
	locked := func() {
		t.Helper()
		if v.Assign(next, 1, 0) {
			t.Fatal("busy vehicle reassigned")
		}
	}
	locked()
	if !v.Pickup(order, 10, 20) {
		t.Fatal("pickup")
	}
	locked()
	if v.DwellReady(29) || !v.DwellReady(30) {
		t.Fatal("pickup dwell")
	}
	if !v.Depart(order, 30) {
		t.Fatal("depart")
	}
	locked()
	v.Account(30, 1000)
	if !v.Dropoff(60, 20) {
		t.Fatal("dropoff")
	}
	locked()
	if v.Complete(order, 79) || !v.Complete(order, 80) {
		t.Fatal("dropoff dwell")
	}
	if v.Completed != 1 || v.OccupiedDistance != 1000 || v.Income != 5.2 {
		t.Fatalf("accounting: %+v", v)
	}
	if !v.Assign(next, 81, 0) {
		t.Fatal("not released")
	}
}

func TestCancellationCannotRemovePassenger(t *testing.T) {
	for _, state := range []string{"Boarding", "InService", "Completed"} {
		o := &Order{State: state}
		if Cancel(o) || o.State != state {
			t.Fatal(state)
		}
	}
}

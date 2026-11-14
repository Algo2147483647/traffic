package foundation

import "testing"

func TestQueueOrderingAndRevisions(t *testing.T) {
	var q Queue[string]
	stamp := Stamp{Run: 1, World: 4, Topology: 2}
	for _, e := range []Envelope[string]{
		{Stamp: stamp, Due: 3, Entity: 2, Payload: "later"},
		{Stamp: stamp, Due: 2, Entity: 2, Payload: "second"},
		{Stamp: stamp, Due: 2, Entity: 1, Payload: "first"},
		{Stamp: stamp, Due: 2, Priority: -1, Payload: "priority"},
	} {
		q.Push(e)
	}
	a := q.Drain(2)
	if len(a) != 3 || a[0].Payload != "priority" || a[1].Payload != "first" || a[2].Payload != "second" {
		t.Fatal(a)
	}
	if len(q.Drain(3)) != 1 || len(q.Drain(4)) != 0 {
		t.Fatal("queue drain")
	}
	e := Envelope[string]{Stamp: stamp, StrictWorld: true}
	if !e.Valid(stamp) || e.Valid(Stamp{Run: 1, World: 5, Topology: 2}) || e.Valid(Stamp{Run: 2, World: 4, Topology: 2}) || e.Valid(Stamp{Run: 1, World: 4, Topology: 3}) {
		t.Fatal("revision validation")
	}
}

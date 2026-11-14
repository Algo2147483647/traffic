package foundation

import "testing"

func TestJournalOrderingAndIsolation(t *testing.T) {
	var j Journal
	for i := 0; i < 2100; i++ {
		j.Record(0, "order", i, "Waiting")
	}
	a := j.Events()
	if len(a) != 2048 || a[0].Sequence != 53 {
		t.Fatal("journal order/retention")
	}
	a[0].State = "changed"
	if j.Events()[0].State != "Waiting" {
		t.Fatal("journal exposed live state")
	}
}

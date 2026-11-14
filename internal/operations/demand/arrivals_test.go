package demand

import "testing"

func TestArrivalsPartitionAndCapacity(t *testing.T) {
	var a, b Arrivals
	count := 0
	for i := 0; i < 200; i++ {
		count += a.Advance(.5, .05, 100)
	}
	if count != 5 || b.Advance(.5, 10, 100) != count {
		t.Fatal("step partition changed demand")
	}
	if a.Advance(100, 1, 0) != 0 || a.Advance(1, 1, 1) != 1 {
		t.Fatal("capacity did not suspend arrivals")
	}
}

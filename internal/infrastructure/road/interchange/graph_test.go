package interchange

import (
	"testing"
	model "traffic/internal/infrastructure/road"
)

func TestMacroPorts(t *testing.T) {
	c, e := model.LoadConfig("../../../../example/Map2/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := model.BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	nodes := Build(g)
	if len(nodes) != 5 {
		t.Fatal("macro graph missing interchanges")
	}
	for _, n := range nodes {
		expected := 6
		if n.ID == "C" {
			expected = 8
		}
		if len(n.Ports) != expected || len(n.InternalWays) == 0 {
			t.Fatal("missing directional ports", n.ID, len(n.Ports))
		}
	}
}

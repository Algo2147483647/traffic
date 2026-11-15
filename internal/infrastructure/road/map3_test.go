package model

import (
	"math"
	"strings"
	"testing"
)

func TestMap3GeometryAndReachability(t *testing.T) {
	c, e := LoadConfig("../../../example/Map3/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	if len(g.Interchanges) != 1 || g.Interchanges[0].Center.X != 0 || g.Interchanges[0].Center.Y != 0 {
		t.Fatal("cloverleaf origin")
	}
	neighbors := map[string]map[string]bool{}
	for _, w := range g.Ways {
		if neighbors[w.From] == nil {
			neighbors[w.From] = map[string]bool{}
		}
		neighbors[w.From][w.To] = true
		if w.RoadClass == "local" {
			if w.Lanes != 1 {
				t.Fatal("grid lane count")
			}
			for _, id := range []string{w.From, w.To} {
				p := g.Intersections[id].Position
				if math.Hypot(p.X, p.Y) >= 2000 {
					t.Fatal("grid touched ring")
				}
			}
		}
		if strings.HasSuffix(w.From, "_4000") && strings.HasSuffix(w.To, "_4000") {
			if w.Lanes != 4 {
				t.Fatal("outer ring lane count")
			}
		}
	}
	for id := range g.Intersections {
		if strings.HasPrefix(id, "G_") && len(neighbors[id]) < 2 {
			t.Fatal("grid dead end", id)
		}
	}
	for start := range g.Intersections {
		seen := map[string]bool{start: true}
		q := []string{start}
		for len(q) > 0 {
			a := q[0]
			q = q[1:]
			for b := range neighbors[a] {
				if !seen[b] {
					seen[b] = true
					q = append(q, b)
				}
			}
		}
		if len(seen) != len(g.Intersections) {
			t.Fatal("disconnected network", start)
		}
	}
}

func TestMap3CornerEntriesAndUndergroundSeparation(t *testing.T) {
	c, e := LoadConfig("../../../example/Map3/road.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := BuildGraph(c)
	if e != nil {
		t.Fatal(e)
	}
	corners, tunnels := 0, 0
	for id, n := range g.Intersections {
		if strings.HasPrefix(id, "RingCorner_") {
			corners++
			if len(g.Incoming[id]) != 4 || len(g.Outgoing[id]) != 4 {
				t.Fatal("corner did not join two grid legs and ring")
			}
		}
		if strings.HasPrefix(id, "T_") && n.TrafficControl.Mode != "flow" {
			t.Fatal("tunnel portal has signals")
		}
	}
	for _, w := range g.Ways {
		if w.Structure == "tunnel" {
			tunnels++
			if w.StartLayer != -1 || w.EndLayer != -1 || w.StartElevation != -5 || w.EndElevation != -5 {
				t.Fatal("wrong tunnel layer")
			}
		}
	}
	if corners != 4 || tunnels != 8 {
		t.Fatal(corners, tunnels)
	}
	for i, l := range c.Metro.Lines {
		if l.Layer != -10-i {
			t.Fatal("metro floor")
		}
	}
}

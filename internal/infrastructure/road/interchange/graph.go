package interchange

import (
	"sort"
	model "traffic/internal/infrastructure/road"
)

type Port struct {
	NodeID   string   `json:"node_id"`
	Incoming []string `json:"incoming"`
	Outgoing []string `json:"outgoing"`
}
type Node struct {
	ID           string   `json:"id"`
	Ports        []Port   `json:"ports"`
	InternalWays []string `json:"internal_ways"`
}

// Build is the macro-node projection. It retains the micrograph for exact execution.
func Build(g *model.Graph) []Node {
	out := []Node{}
	for _, group := range g.Interchanges {
		n := Node{ID: group.ID, Ports: []Port{}, InternalWays: []string{}}
		for _, w := range g.SortedWays() {
			if g.Intersections[w.From].Interchange == group.ID && g.Intersections[w.To].Interchange == group.ID {
				n.InternalWays = append(n.InternalWays, w.ID)
			}
		}
		for _, junction := range g.SortedIntersections() {
			if junction.Interchange != group.ID {
				continue
			}
			p := Port{NodeID: junction.ID, Incoming: []string{}, Outgoing: []string{}}
			for _, w := range g.Incoming[junction.ID] {
				if g.Intersections[w.From].Interchange != group.ID {
					p.Incoming = append(p.Incoming, w.ID)
				}
			}
			for _, w := range g.Outgoing[junction.ID] {
				if g.Intersections[w.To].Interchange != group.ID {
					p.Outgoing = append(p.Outgoing, w.ID)
				}
			}
			if len(p.Incoming)+len(p.Outgoing) > 0 {
				sort.Strings(p.Incoming)
				sort.Strings(p.Outgoing)
				n.Ports = append(n.Ports, p)
			}
		}
		out = append(out, n)
	}
	return out
}

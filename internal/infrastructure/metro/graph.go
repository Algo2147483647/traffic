package metro

import "fmt"

type Station struct {
	ID       string
	Capacity int
}
type Section struct {
	ID, From, To string
	Capacity     int
}
type Graph struct {
	Stations []Station
	Sections []Section
}

func (g Graph) Validate() error {
	seen := map[string]bool{}
	for _, s := range g.Stations {
		if s.ID == "" || seen[s.ID] || s.Capacity < 1 {
			return fmt.Errorf("invalid station")
		}
		seen[s.ID] = true
	}
	edges := map[string]bool{}
	for _, s := range g.Sections {
		if s.ID == "" || edges[s.ID] || !seen[s.From] || !seen[s.To] || s.Capacity < 1 {
			return fmt.Errorf("invalid section")
		}
		edges[s.ID] = true
	}
	return nil
}

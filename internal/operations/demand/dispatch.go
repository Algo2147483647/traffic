package demand

import "fmt"

// Context is the world-owned operational ledger, shared with its admission adapters.
type Context struct{ Stream *Stream }

func (d *Stream) Dispatch(now float64, stops int, external bool, pairs [][2]int, metro func(int, int, int) bool, road func(*Journey) bool) {
	d.Generate(now, stops)
	for _, j := range d.Journeys {
		if j.State != "queued" {
			continue
		}
		if !j.Prepared {
			j.DepartAt = now
			if j.Mode == "metro" {
				if len(pairs) == 0 {
					continue
				}
				pair := pairs[d.Choose(len(pairs))]
				j.Origin, j.Destination = pair[0], pair[1]
			}
			j.Prepared = true
		}
		if external && !j.Ready || j.DepartAt > now {
			continue
		}
		if j.Mode == "metro" {
			if len(pairs) > 0 && metro(j.Origin, j.Destination, j.ID) {
				j.Entity = j.ID
				j.State = "active"
			}
			continue
		}
		road(j)
	}
	d.Prune()
}
func (d *Stream) Audit(linked map[int]int) Summary {
	summary := d.Summary()
	for _, j := range d.Journeys {
		expected := 0
		if j.State == "active" {
			expected = 1
		}
		if linked[j.ID] != expected {
			summary.Errors = append(summary.Errors, fmt.Sprintf("journey %d has %d executor tasks, expected %d", j.ID, linked[j.ID], expected))
		}
		delete(linked, j.ID)
	}
	if len(linked) > 0 {
		summary.Errors = append(summary.Errors, "orphan executor tasks")
	}
	summary.Conserved = summary.Conserved && len(summary.Errors) == 0
	return summary
}

package metroscheduling

import (
	"fmt"
)

func (r *Runtime) networkView() View {
	v := View{Config: r.config, Time: r.time, Layer: -1, ServicePlanners: ServicePlannerIDs()}
	if r.layout != nil {
		v.HighestLayer = r.layout.HighestLayer
	}
	v.CrossingOwner = r.lines[0].gate.owner
	for _, line := range r.lines {
		if line.service != nil {
			v.Services = append(v.Services, line.serviceView())
		}
		x := line.Snapshot()
		offset := len(v.Stations)
		trainOffset := line.lineID * 100
		for _, s := range x.Stations {
			s.ID += offset
			v.Stations = append(v.Stations, s)
		}
		for _, t := range x.Trains {
			t.ID += trainOffset
			t.Station += offset
			t.Next += offset
			v.Trains = append(v.Trains, t)
		}
		for _, b := range x.Blocks {
			b.ID = fmt.Sprintf("L%d.%s", line.lineID+1, b.ID)
			b.From += offset
			b.To += offset
			if b.Owner != 0 {
				b.Owner += trainOffset
			}
			if b.Occupied != 0 {
				b.Occupied += trainOffset
			}
			v.Blocks = append(v.Blocks, b)
		}
		for _, route := range x.Routes {
			route.Station += offset
			if route.Owner != 0 {
				route.Owner += trainOffset
			}
			v.Routes = append(v.Routes, route)
		}
		for _, l := range x.Lines {
			for i := range l.Stations {
				l.Stations[i] += offset
			}
			v.Lines = append(v.Lines, l)
		}
		v.Generated += x.Generated
		v.Waiting += x.Waiting
		v.Onboard += x.Onboard
		v.Completed += x.Completed
		v.Departures += x.Departures
	}
	r.transferView(&v)
	r.stationDetails(&v)
	return v
}

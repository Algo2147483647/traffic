package model

import "math"

type Projection struct {
	Position Vec2
	Distance float64
	Along    float64
	Tangent  Vec2
	Segment  int
}

// Project returns the closest point on the directed way.
func (way *Way) Project(point Vec2) Projection {
	best := Projection{Distance: math.Inf(1), Tangent: Vec2{X: 1}}
	for i := 0; i < len(way.Points)-1; i++ {
		start, end := way.Points[i], way.Points[i+1]
		line := end.Sub(start)
		lengthSquared := line.X*line.X + line.Y*line.Y
		t := ((point.X-start.X)*line.X + (point.Y-start.Y)*line.Y) / lengthSquared
		t = math.Max(0, math.Min(1, t))
		projected := start.Add(line.Mul(t))
		d := distance(point, projected)
		if d < best.Distance {
			best = Projection{
				Position: projected, Distance: d,
				Along:   way.cumulative[i] + line.Length()*t,
				Tangent: line.Normalize(), Segment: i,
			}
		}
	}
	return best
}

// Sample returns the lane-center position and tangent at a distance along a way.
func (way *Way) Sample(along float64, lane int) (Vec2, Vec2) {
	return way.SampleLane(along, float64(lane))
}

// SampleLane accepts a fractional lane index so simulations can render a
// continuous lateral transition between discrete lane centers.
func (way *Way) SampleLane(along, lane float64) (Vec2, Vec2) {
	lane = math.Max(0, math.Min(float64(way.Lanes-1), lane))
	along = math.Max(0, math.Min(way.Length, along))
	segmentIndex := len(way.Points) - 2
	for i := 0; i < len(way.cumulative)-1; i++ {
		if along <= way.cumulative[i+1] {
			segmentIndex = i
			break
		}
	}
	start, end := way.Points[segmentIndex], way.Points[segmentIndex+1]
	line := end.Sub(start)
	length := line.Length()
	t := 0.0
	if length > 0 {
		t = (along - way.cumulative[segmentIndex]) / length
	}
	t = math.Max(0, math.Min(1, t))
	tangent := line.Normalize()
	normal := Vec2{X: -tangent.Y, Y: tangent.X}
	position := start.Add(line.Mul(t)).Add(normal.Mul(way.CarriagewayOffset + lanePositionOffset(lane, way.Lanes, way.LaneWidth)))
	return position, tangent
}

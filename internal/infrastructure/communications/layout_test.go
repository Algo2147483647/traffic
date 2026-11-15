package communications

import (
	"math"
	"testing"
)

func TestMap1HexDeployment(t *testing.T) {
	l, e := Load("../../../example/Map1/communications.json")
	if e != nil {
		t.Fatal(e)
	}
	d := l.Deployment
	if d == nil || d.Radius != 256 || len(l.Sites) != 700 {
		t.Fatal("wrong honeycomb deployment")
	}
	nearest := math.Inf(1)
	for _, s := range l.Sites[1:] {
		nearest = math.Min(nearest, math.Hypot(s.Position.X-l.Sites[0].Position.X, s.Position.Y-l.Sites[0].Position.Y))
	}
	if math.Abs(nearest-math.Sqrt(3)*256) > 1e-6 {
		t.Fatal("wrong nearest-site spacing", nearest)
	}
	for xi := 0; xi <= 82; xi++ {
		for yi := 0; yi <= 82; yi++ {
			x := d.MinX + (d.MaxX-d.MinX)*float64(xi)/82
			y := d.MinY + (d.MaxY-d.MinY)*float64(yi)/82
			distance := math.Inf(1)
			for _, s := range l.Sites {
				distance = math.Min(distance, math.Hypot(s.Position.X-x, s.Position.Y-y))
			}
			if distance > 256+1e-6 {
				t.Fatalf("coverage hole at %.1f,%.1f: %.2f", x, y, distance)
			}
		}
	}
	l.Sites[0].Position.X++
	if l.Validate() == nil {
		t.Fatal("off-lattice site accepted")
	}
}

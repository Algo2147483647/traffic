package communicationscheduling

import (
	"math"
	"reflect"
	"testing"
	"traffic/internal/foundation"
	infra "traffic/internal/infrastructure/communications"
)

func fixture() *infra.Layout {
	return &infra.Layout{Schema: 1, CoordinateSystem: "local-meters-x-east-y-south-z-up", Profile: infra.Profile{Model: "log-distance", Version: "1", ReferenceDistance: 10, PathLossExponent: 2, ReceiverHeight: 1.5, Sensitivity: 1e-12, Temperature: 290}, Sites: []infra.Site{{ID: "A", MastHeight: 1.5, CoverageRadius: 1000, Sectors: []infra.Sector{{ID: "A1", Frequency: 1e9, Bandwidth: 1e6, Power: 1, HorizontalWidth: 65, VerticalWidth: 15, Attenuation: 30}}}}}
}
func TestAnalyticLinkBudgetAndGain(t *testing.T) {
	l := fixture()
	s, c := l.Sites[0], l.Sites[0].Sectors[0]
	u := Pose{Position: infra.Position{X: 100, Y: 0, Z: 1.5}}
	a, ok := LinkBudget(l.Profile, s, c, u)
	if !ok {
		t.Fatal("missing link")
	}
	expected := math.Pow(299792458/(4*math.Pi*100*1e9), 2)
	if math.Abs(a.Received-expected) > expected*1e-12 {
		t.Fatal("Friis fixture", a.Received, expected)
	}
	u.Position.X = 200
	b, _ := LinkBudget(l.Profile, s, c, u)
	if math.Abs(10*math.Log10(a.Received/b.Received)-6.020599913) > 1e-8 {
		t.Fatal("inverse square law")
	}
	u.Position.X = -100
	b, _ = LinkBudget(l.Profile, s, c, u)
	if math.Abs(a.AntennaGain-b.AntennaGain-30) > 1e-8 {
		t.Fatal("front/back antenna pattern")
	}
	l.Profile.VehicleLoss = 10
	u.Position.X = 100
	u.IndoorVehicle = true
	b, _ = LinkBudget(l.Profile, s, c, u)
	if math.Abs(a.Received/b.Received-10) > 1e-9 {
		t.Fatal("vehicle penetration")
	}
	if math.Abs(a.Noise-1.380649e-23*290*1e6) > 1e-20 {
		t.Fatal("thermal noise")
	}
}
func TestDeterministicAssociationUnsupportedAndConservation(t *testing.T) {
	l := fixture()
	b := l.Sites[0]
	b.ID = "B"
	b.Position.X = 200
	b.Sectors = append([]infra.Sector(nil), b.Sectors...)
	b.Sectors[0].ID = "B1"
	b.Sectors[0].Azimuth = 180
	l.Sites = append(l.Sites, b)
	poses := []Pose{{ID: "one", Position: infra.Position{X: 50, Z: 1.5}}, {ID: "two", Position: infra.Position{X: 150, Z: 1.5}}, {ID: "underground", Position: infra.Position{X: 50, Z: -50}, Underground: true}, {ID: "far", Position: infra.Position{X: 1e6, Z: 1.5}}}
	stamp := foundation.Stamp{Run: 2, World: 3, Topology: 1}
	a := Observe(l, stamp, .05, poses)
	bview := Observe(l, stamp, .05, poses)
	if !reflect.DeepEqual(a, bview) {
		t.Fatal("read changed radio state")
	}
	if a.TimeUS != 50000 || a.Connected != 2 || a.Unsupported != 1 || a.Outage != 1 {
		t.Fatalf("accounting %+v", a)
	}
	for _, u := range a.Terminals {
		if u.ID == "one" && u.Serving != "A1" || u.ID == "two" && u.Serving != "B1" {
			t.Fatal("strongest-cell association", u)
		}
	}
	count := 0
	for _, c := range a.Cells {
		count += c.Associated
	}
	if count != a.Connected || a.Connected+a.Unsupported+a.Outage != len(poses) {
		t.Fatal("terminal conservation")
	}
	poses[0].Position.X = 150
	c := Observe(l, stamp, .1, poses)
	for _, u := range c.Terminals {
		if u.ID == "one" && u.Serving != "B1" {
			t.Fatal("moving terminal did not reselect")
		}
	}
}
func TestInvalidLayoutsRejected(t *testing.T) {
	l := fixture()
	if e := l.Validate(); e != nil {
		t.Fatal(e)
	}
	l.Sites[0].Sectors = append(l.Sites[0].Sectors, l.Sites[0].Sectors[0])
	if l.Validate() == nil {
		t.Fatal("duplicate cells accepted")
	}
	l = fixture()
	l.Profile.NoiseFigure = math.NaN()
	if l.Validate() == nil {
		t.Fatal("NaN accepted")
	}
}

func TestNominalRadiusAndSpatialIndex(t *testing.T) {
	l := fixture()
	l.Sites[0].CoverageRadius = 256
	inside := Pose{ID: "inside", Position: infra.Position{X: 256, Z: 1.5}}
	outside := inside
	outside.ID = "outside"
	outside.Position.X = 256.001
	if _, ok := LinkBudget(l.Profile, l.Sites[0], l.Sites[0].Sectors[0], inside); !ok {
		t.Fatal("radius boundary excluded")
	}
	if _, ok := LinkBudget(l.Profile, l.Sites[0], l.Sites[0].Sectors[0], outside); ok {
		t.Fatal("range cap ignored")
	}
	view := Observe(l, foundation.Stamp{}, 0, []Pose{inside, outside})
	if view.Connected != 1 || view.Outage != 1 {
		t.Fatal("spatial radius classification", view)
	}
}

// Package communications owns radio facilities and the versioned baseline profile.
package communications

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}
type Sector struct {
	ID              string  `json:"id"`
	Azimuth         float64 `json:"azimuth_deg"`
	Tilt            float64 `json:"downtilt_deg"`
	HorizontalWidth float64 `json:"horizontal_beamwidth_deg"`
	VerticalWidth   float64 `json:"vertical_beamwidth_deg"`
	Gain            float64 `json:"peak_gain_db"`
	Attenuation     float64 `json:"max_attenuation_db"`
	Frequency       float64 `json:"frequency_hz"`
	Bandwidth       float64 `json:"bandwidth_hz"`
	Power           float64 `json:"transmit_power_w"`
}
type Site struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Position       Position `json:"position_m"` // Ground/roof reference, not antenna height.
	MastHeight     float64  `json:"mast_height_m"`
	CoverageRadius float64  `json:"coverage_radius_m"` // Nominal horizontal service limit, not a guarantee of received power.
	Sectors        []Sector `json:"sectors"`
}
type Profile struct {
	Model             string  `json:"model"`
	Version           string  `json:"version"`
	ReferenceDistance float64 `json:"reference_distance_m"`
	PathLossExponent  float64 `json:"path_loss_exponent"`
	ReceiverHeight    float64 `json:"receiver_height_m"`
	ReceiverGain      float64 `json:"receiver_gain_db"`
	VehicleLoss       float64 `json:"vehicle_penetration_loss_db"`
	Sensitivity       float64 `json:"receiver_sensitivity_w"`
	NoiseFigure       float64 `json:"noise_figure_db"`
	Temperature       float64 `json:"temperature_k"`
}
type Deployment struct {
	Pattern string   `json:"pattern"`
	Radius  float64  `json:"radius_m"`
	Origin  Position `json:"origin_m"`
	MinX    float64  `json:"min_x_m"`
	MinY    float64  `json:"min_y_m"`
	MaxX    float64  `json:"max_x_m"`
	MaxY    float64  `json:"max_y_m"`
}
type Traffic struct {
 Interval float64 `json:"interval_s"`
 OfferedRate float64 `json:"offered_bps_per_terminal"`
 QueueLimit float64 `json:"queue_limit_bits"`
 Efficiency float64 `json:"efficiency_factor"`
 MaxEfficiency float64 `json:"max_spectral_efficiency"`
 Gap float64 `json:"sinr_gap_linear"`
}
type Layout struct {
 Traffic *Traffic `json:"traffic,omitempty"`
	Deployment       *Deployment `json:"deployment,omitempty"`
	Schema           int         `json:"schema_version"`
	CoordinateSystem string      `json:"coordinate_system"`
	Profile          Profile     `json:"profile"`
	Sites            []Site      `json:"sites"`
}

func Load(path string) (*Layout, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var l Layout
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if e = d.Decode(&l); e != nil {
		return nil, e
	}
	if e = l.Validate(); e != nil {
		return nil, e
	}
	return &l, nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }
func (l *Layout) Validate() error {
	if l.Schema != 1 || l.CoordinateSystem != "local-meters-x-east-y-south-z-up" {
		return fmt.Errorf("unsupported radio schema or coordinate system")
	}
	if t:=l.Traffic;t!=nil {
 for _,v:=range []float64{t.Interval,t.QueueLimit,t.Efficiency,t.MaxEfficiency,t.Gap}{if !finite(v)||v<=0{return fmt.Errorf("invalid radio traffic parameter")}}
 if !finite(t.OfferedRate)||t.OfferedRate<0||t.Interval<.05||math.Abs(t.Interval/.05-math.Round(t.Interval/.05))>1e-7||t.Efficiency>1||t.Gap<1{return fmt.Errorf("invalid traffic rate, slot interval or efficiency")}
 }
 p := l.Profile
	if p.Model != "log-distance" || p.Version != "1" {
		return fmt.Errorf("unsupported radio model/version")
	}
	for _, v := range []float64{p.ReferenceDistance, p.PathLossExponent, p.ReceiverHeight, p.Sensitivity, p.Temperature} {
		if !finite(v) || v <= 0 {
			return fmt.Errorf("radio physical parameters must be finite and positive")
		}
	}
	for _, v := range []float64{p.ReceiverGain, p.VehicleLoss, p.NoiseFigure} {
		if !finite(v) {
			return fmt.Errorf("nonfinite radio gain/loss")
		}
	}
	if p.VehicleLoss < 0 || p.NoiseFigure < 0 {
		return fmt.Errorf("radio losses cannot be negative")
	}
	if d := l.Deployment; d != nil {
		if d.Pattern != "hex-pointy" || !finite(d.Radius) || d.Radius <= 0 || d.MaxX <= d.MinX || d.MaxY <= d.MinY {
			return fmt.Errorf("invalid hex deployment")
		}
		for _, v := range []float64{d.Origin.X, d.Origin.Y, d.Origin.Z, d.MinX, d.MinY, d.MaxX, d.MaxY} {
			if !finite(v) {
				return fmt.Errorf("nonfinite deployment bounds")
			}
		}
	}
	sites, cells := map[string]bool{}, map[string]bool{}
	for _, s := range l.Sites {
		if s.ID == "" || sites[s.ID] || len(s.Sectors) == 0 {
			return fmt.Errorf("invalid or duplicate site %q", s.ID)
		}
		sites[s.ID] = true
		if d := l.Deployment; d != nil {
			row := (s.Position.Y - d.Origin.Y) / (1.5 * d.Radius)
			col := (s.Position.X-d.Origin.X)/(math.Sqrt(3)*d.Radius) - row/2
			if math.Abs(s.CoverageRadius-d.Radius) > 1e-6 || math.Abs(row-math.Round(row)) > 1e-6 || math.Abs(col-math.Round(col)) > 1e-6 {
				return fmt.Errorf("site %s is outside the declared hex lattice", s.ID)
			}
		}
		for _, v := range []float64{s.Position.X, s.Position.Y, s.Position.Z, s.MastHeight, s.CoverageRadius} {
			if !finite(v) {
				return fmt.Errorf("nonfinite site geometry")
			}
		}
		if s.MastHeight < 0 || s.CoverageRadius <= 0 {
			return fmt.Errorf("invalid mast or display radius")
		}
		for _, c := range s.Sectors {
			if c.ID == "" || cells[c.ID] {
				return fmt.Errorf("duplicate or empty cell ID")
			}
			cells[c.ID] = true
			for _, v := range []float64{c.Frequency, c.Bandwidth, c.Power, c.HorizontalWidth, c.VerticalWidth} {
				if !finite(v) || v <= 0 {
					return fmt.Errorf("invalid carrier or beamwidth")
				}
			}
			for _, v := range []float64{c.Azimuth, c.Tilt, c.Gain, c.Attenuation} {
				if !finite(v) {
					return fmt.Errorf("nonfinite antenna pattern")
				}
			}
			if c.HorizontalWidth > 360 || c.VerticalWidth > 180 || c.Attenuation < 0 || c.Bandwidth > c.Frequency || p.ReferenceDistance < 10*299792458/c.Frequency {
				return fmt.Errorf("invalid antenna pattern or reference distance")
			}
		}
	}
	return nil
}

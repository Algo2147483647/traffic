// Package communicationscheduling implements the explicitly simplified radio observer.
// It has no road ownership, packet engine, NR slots or hidden random state.
package communicationscheduling

import (
	"math"
	"sort"
	"traffic/internal/foundation"
	infra "traffic/internal/infrastructure/communications"
)

type Pose struct {
	ID            string         `json:"id"`
	Kind          string         `json:"kind"`
	EntityID      int            `json:"entity_id"`
	Carrier       string         `json:"carrier,omitempty"`
	Position      infra.Position `json:"position_m"`
	Speed         float64        `json:"speed_mps"`
	Heading       float64        `json:"heading_rad"`
	IndoorVehicle bool           `json:"in_vehicle"`
	Underground   bool           `json:"underground"`
}
type Link struct {
	Site        string  `json:"site_id"`
	Cell        string  `json:"cell_id"`
	Distance    float64 `json:"distance_m"`
	PathLoss    float64 `json:"path_loss_db"`
	AntennaGain float64 `json:"antenna_gain_db"`
	Received    float64 `json:"received_power_w"`
	Noise       float64 `json:"noise_power_w"`
	SNR         float64 `json:"snr_db"` // Thermal-noise-only SNR, never SINR or capacity.
}
type Terminal struct {
 Interference float64 `json:"interference_w"`
 SINR *float64 `json:"sinr_db"`
 Allocated float64 `json:"allocated_bandwidth_hz"`
 BandStart float64 `json:"band_start_hz"`
 Throughput float64 `json:"throughput_bps"`
 Delivered float64 `json:"delivered_bits"`
 Queued float64 `json:"queued_bits"`
 Offered float64 `json:"offered_bits"`
 Dropped float64 `json:"dropped_bits"`
	Pose
	Status     string `json:"status"`
	Serving    string `json:"serving_cell,omitempty"`
	Neighbours []Link `json:"neighbours"`
}
type Cell struct {
 Bandwidth float64 `json:"bandwidth_hz"`
 Allocated float64 `json:"allocated_bandwidth_hz"`
 Utilization float64 `json:"resource_utilization"`
 Throughput float64 `json:"throughput_bps"`
 Delivered float64 `json:"delivered_bits"`
	ID         string `json:"id"`
	Site       string `json:"site_id"`
	Associated int    `json:"associated"`
}
type View struct {
 Interval float64 `json:"interval_s"`
 Throughput float64 `json:"throughput_bps"`
 AverageThroughput float64 `json:"average_throughput_bps"`
 Utilization float64 `json:"resource_utilization"`
 Offered float64 `json:"offered_bits"`
 Delivered float64 `json:"delivered_bits"`
 Queued float64 `json:"queued_bits"`
 Dropped float64 `json:"dropped_bits"`
 Retired float64 `json:"retired_bits"`
 Conserved bool `json:"conserved"`
	Schema      int              `json:"schema_version"`
	Model       string           `json:"model"`
	Version     string           `json:"model_version"`
	Fidelity    string           `json:"fidelity"`
	Stamp       foundation.Stamp `json:"stamp"`
	TimeUS      int64            `json:"time_us"`
	Connected   int              `json:"connected"`
	Outage      int              `json:"outage"`
	Unsupported int              `json:"unsupported"`
	Cells       []Cell           `json:"cells"`
	Terminals   []Terminal       `json:"terminals"`
}

func DBm(w float64) float64     { return 10*math.Log10(w) + 30 }
func angular(a float64) float64 { return math.Remainder(a, 360) }

// LinkBudget uses Friis at d0 followed by the configured log-distance exponent.
// The separable parabolic pattern is a baseline assumption, not a 3GPP implementation.
func LinkBudget(p infra.Profile, s infra.Site, c infra.Sector, u Pose) (Link, bool) {
	if math.Hypot(u.Position.X-s.Position.X,u.Position.Y-s.Position.Y)>s.CoverageRadius{return Link{},false}
 return Propagation(p,s,c,u)
}
// Propagation deliberately ignores association radius: distant co-channel emitters
// still interfere. All configured active sites participate, without distance truncation.
func Propagation(p infra.Profile,s infra.Site,c infra.Sector,u Pose)(Link,bool){
 dx, dy, dz := u.Position.X-s.Position.X, u.Position.Y-s.Position.Y, u.Position.Z-(s.Position.Z+s.MastHeight)
	horizontal := math.Hypot(dx, dy)
	d := math.Hypot(horizontal, dz)
	if d < p.ReferenceDistance || u.Underground {
		return Link{}, false
	}
	az := math.Atan2(dy, dx) * 180 / math.Pi
	tilt := -math.Atan2(dz, horizontal) * 180 / math.Pi
	loss := math.Min(c.Attenuation, 12*math.Pow(angular(az-c.Azimuth)/c.HorizontalWidth, 2)+12*math.Pow((tilt-c.Tilt)/c.VerticalWidth, 2))
	gain := c.Gain - loss
	pl := 20*math.Log10(4*math.Pi*p.ReferenceDistance*c.Frequency/299792458) + 10*p.PathLossExponent*math.Log10(d/p.ReferenceDistance)
	penetration := 0.0
	if u.IndoorVehicle {
		penetration = p.VehicleLoss
	}
	received := c.Power * math.Pow(10, (gain+p.ReceiverGain-pl-penetration)/10)
	noise := 1.380649e-23 * p.Temperature * c.Bandwidth * math.Pow(10, p.NoiseFigure/10)
	return Link{s.ID, c.ID, d, pl, gain, received, noise, 10 * math.Log10(received/noise)}, true
}

// Observe is deterministic and stateless at a committed mobility boundary. Repeated
// reads cannot change association history or advance time. No sub-tick radio fidelity is claimed.
func Observe(l *infra.Layout, stamp foundation.Stamp, time float64, poses []Pose) *View {
	if l == nil {
		return nil
	}
	v := &View{Schema: 1, Model: l.Profile.Model, Version: l.Profile.Version, Fidelity: "simplified-outdoor-link-budget", Stamp: stamp, TimeUS: int64(math.Round(time * 1e6)), Cells: []Cell{}, Terminals: []Terminal{}}
	index := map[string]int{}
	for _, s := range l.Sites {
		for _, c := range s.Sectors {
			index[c.ID] = len(v.Cells)
			v.Cells = append(v.Cells, Cell{ID: c.ID, Site: s.ID, Bandwidth:c.Bandwidth})
		}
	}
	// Buckets bound the search to nearby sites; costs do not grow as UE x all sites.
	bucketSize := 1.0
	for _, site := range l.Sites {
		bucketSize = math.Max(bucketSize, site.CoverageRadius)
	}
	buckets := map[[2]int][]int{}
	key := func(x, y float64) [2]int {
		return [2]int{int(math.Floor(x / bucketSize)), int(math.Floor(y / bucketSize))}
	}
	for i, site := range l.Sites {
		k := key(site.Position.X, site.Position.Y)
		buckets[k] = append(buckets[k], i)
	}
	for _, u := range poses {
		t := Terminal{Pose: u, Status: "outage", Neighbours: []Link{}}
		near := false
		k := key(u.Position.X, u.Position.Y)
		if !u.Underground {
			for dx := -1; dx <= 1; dx++ {
				for dy := -1; dy <= 1; dy++ {
					for _, i := range buckets[[2]int{k[0] + dx, k[1] + dy}] {
						site := l.Sites[i]
						horizontal := math.Hypot(u.Position.X-site.Position.X, u.Position.Y-site.Position.Y)
						if horizontal > site.CoverageRadius {
							continue
						}
						near = near || math.Hypot(horizontal, u.Position.Z-site.Position.Z-site.MastHeight) < l.Profile.ReferenceDistance
						for _, cell := range site.Sectors {
							if x, ok := LinkBudget(l.Profile, site, cell, u); ok {
								t.Neighbours = append(t.Neighbours, x)
							}
						}
					}
				}
			}
		}
		sort.Slice(t.Neighbours, func(i, j int) bool {
			a, b := t.Neighbours[i], t.Neighbours[j]
			if a.Received == b.Received {
				return a.Cell < b.Cell
			}
			return a.Received > b.Received
		})
		if u.Underground {
			t.Status = "unsupported-underground"
			v.Unsupported++
		} else if len(t.Neighbours) == 0 && near {
			t.Status = "unsupported-distance"
			v.Unsupported++
		} else if len(t.Neighbours) > 0 && t.Neighbours[0].Received >= l.Profile.Sensitivity {
			t.Status = "connected"
			t.Serving = t.Neighbours[0].Cell
			v.Cells[index[t.Serving]].Associated++
			v.Connected++
		} else {
			v.Outage++
		}
		if len(t.Neighbours) > 3 {
			t.Neighbours = t.Neighbours[:3]
		}
		v.Terminals = append(v.Terminals, t)
	}
	sort.Slice(v.Terminals, func(i, j int) bool { return v.Terminals[i].ID < v.Terminals[j].ID })
	return v
}

package model

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// Vec2 is a point or vector in screen-oriented world coordinates: X increases
// to the right and Y increases downward. This matches the canvas renderer and
// makes a positive signed turn angle a visual/rightward turn.
type Vec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v Vec2) Add(other Vec2) Vec2 { return Vec2{X: v.X + other.X, Y: v.Y + other.Y} }
func (v Vec2) Sub(other Vec2) Vec2 { return Vec2{X: v.X - other.X, Y: v.Y - other.Y} }
func (v Vec2) Mul(s float64) Vec2  { return Vec2{X: v.X * s, Y: v.Y * s} }
func (v Vec2) Length() float64     { return math.Hypot(v.X, v.Y) }

func (v Vec2) Normalize() Vec2 {
	if length := v.Length(); length > 0 {
		return v.Mul(1 / length)
	}
	return Vec2{X: 1}
}

// Matrix3 is a row-major homogeneous 2D transform. It maps local coordinates
// into world coordinates.
type Matrix3 [3][3]float64

func Transform(position, tangent Vec2) Matrix3 {
	t := tangent.Normalize()
	return Matrix3{
		{t.X, -t.Y, position.X},
		{t.Y, t.X, position.Y},
		{0, 0, 1},
	}
}

func Translation(position Vec2) Matrix3 {
	return Matrix3{{1, 0, position.X}, {0, 1, position.Y}, {0, 0, 1}}
}

func RelativeTo(origin Vec2, world Matrix3) Matrix3 {
	world[0][2] -= origin.X
	world[1][2] -= origin.Y
	return world
}

type Bounds struct {
	MinX float64 `json:"min_x"`
	MinY float64 `json:"min_y"`
	MaxX float64 `json:"max_x"`
	MaxY float64 `json:"max_y"`
}

type SimulationConfig struct {
	People       int     `json:"people"`
	SpawnRate    float64 `json:"spawn_rate_per_second"`
	MaxPeople    int     `json:"max_people"`
	TickMS       int     `json:"tick_ms"`
	WalkSpeed    float64 `json:"walk_speed"`
	DriveSpeed   float64 `json:"drive_speed"`
	SafeGap      float64 `json:"safe_gap"`
	SpawnMargin  float64 `json:"spawn_margin"`
	Seed         int64   `json:"seed"`
	SnapshotFPS  int     `json:"snapshot_fps"`
	VehicleSize  float64 `json:"vehicle_size"`
	Intersection float64 `json:"intersection_radius"`
	ArrivalDwell float64 `json:"arrival_dwell_seconds"`
}

type TrafficPhaseConfig struct {
	ID               string                  `json:"id"`
	Name             string                  `json:"name"`
	DurationSeconds  float64                 `json:"duration_seconds"`
	MinGreenSeconds  float64                 `json:"min_green_seconds,omitempty"`
	MaxGreenSeconds  float64                 `json:"max_green_seconds,omitempty"`
	AmberSeconds     float64                 `json:"amber_seconds,omitempty"`
	AllRedSeconds    float64                 `json:"all_red_seconds,omitempty"`
	DemandScore      float64                 `json:"demand_score,omitempty"`
	FlowRatio        float64                 `json:"critical_flow_ratio,omitempty"`
	AllowedIncoming  []string                `json:"allowed_incoming_way_ids,omitempty"`
	AllowedMovements []TrafficMovementConfig `json:"allowed_movements,omitempty"`
}

// LaneConnectionConfig is one traversable lane-centre connector through an
// intersection. Lane indices are zero based from the driver's left to right in
// the direction of travel.
type LaneConnectionConfig struct {
	FromLane int `json:"from_lane"`
	ToLane   int `json:"to_lane"`
}

// TrafficMovementConfig is a protected movement from one directed approach to
// one directed exit. FromLanes and ToLanes are convenient unions used by the
// signal controller; LaneConnections is the authoritative lane-to-lane
// topology used by routing and vehicle turns.
type TrafficMovementConfig struct {
	FromWayID       string                 `json:"from_way_id"`
	ToWayID         string                 `json:"to_way_id"`
	FromLanes       []int                  `json:"from_lanes"`
	ToLanes         []int                  `json:"to_lanes"`
	LaneConnections []LaneConnectionConfig `json:"lane_connections"`
	Turn            string                 `json:"turn"`
	DemandScore     float64                `json:"demand_score,omitempty"`
}

type TrafficControlConfig struct {
	Mode          string               `json:"mode,omitempty"`
	OffsetSeconds float64              `json:"offset_seconds"`
	Phases        []TrafficPhaseConfig `json:"phases"`
}

type IntersectionConfig struct {
	Layer       int                  `json:"layer,omitempty"`
	Elevation   float64              `json:"elevation,omitempty"`
	Interchange string               `json:"interchange,omitempty"`
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Position    Vec2                 `json:"position"`
	Rotation    float64              `json:"rotation_degrees,omitempty"`
	Traffic     TrafficControlConfig `json:"traffic_control,omitempty"`
}

type WayConfig struct {
	Structure  string  `json:"structure,omitempty"`
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Lanes      int     `json:"lanes"`
	LaneWidth  float64 `json:"lane_width"`
	SpeedLimit float64 `json:"speed_limit"`
	RoadClass  string  `json:"road_class,omitempty"`
	Demand     float64 `json:"demand_weight,omitempty"`
	Points     []Vec2  `json:"points,omitempty"`
}

const (
	DrivingSideRight = "right"
	DrivingSideLeft  = "left"
)

// MovementRulesConfig controls the geometry-only classification used when a
// city does not provide an explicit signal movement plan.
type MovementRulesConfig struct {
	ThroughToleranceDegrees float64 `json:"through_tolerance_degrees"`
	UTurnToleranceDegrees   float64 `json:"u_turn_tolerance_degrees"`
}

// CityConfig is the JSON input contract for the simulator.
type CityConfig struct {
	Name          string               `json:"name"`
	Description   string               `json:"description"`
	DrivingSide   string               `json:"driving_side"`
	MovementRules MovementRulesConfig  `json:"movement_rules"`
	Bounds        Bounds               `json:"bounds"`
	Simulation    SimulationConfig     `json:"simulation"`
	Intersections []IntersectionConfig `json:"intersections"`
	Ways          []WayConfig          `json:"ways"`
}

func LoadConfig(path string) (CityConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CityConfig{}, fmt.Errorf("read city config: %w", err)
	}
	var cfg CityConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return CityConfig{}, fmt.Errorf("decode city config: %w", err)
	}
	return cfg, nil
}

type SubSegment struct {
	ID        string  `json:"id"`
	Lane      int     `json:"lane"`
	Start     Vec2    `json:"start"`
	End       Vec2    `json:"end"`
	Length    float64 `json:"length"`
	Transform Matrix3 `json:"transform"`
}

type Segment struct {
	ID          string       `json:"id"`
	WayID       string       `json:"way_id"`
	Index       int          `json:"index"`
	Start       Vec2         `json:"start"`
	End         Vec2         `json:"end"`
	Length      float64      `json:"length"`
	Transform   Matrix3      `json:"transform"`
	SubSegments []SubSegment `json:"sub_segments"`
}

type Approach struct {
	WayID       string       `json:"way_id"`
	Direction   string       `json:"direction"`
	Lanes       int          `json:"lanes"`
	Transform   Matrix3      `json:"transform"`
	SubSegments []SubSegment `json:"sub_segments"`
}

type Intersection struct {
	Layer          int            `json:"layer"`
	Elevation      float64        `json:"elevation"`
	Interchange    string         `json:"interchange,omitempty"`
	ID             string         `json:"id"`
	Name           string         `json:"name"`
	Position       Vec2           `json:"position"`
	Transform      Matrix3        `json:"transform"`
	Approaches     []Approach     `json:"approaches"`
	TrafficControl TrafficControl `json:"traffic_control"`
}

type TrafficControl struct {
	Mode          string               `json:"mode,omitempty"`
	OffsetSeconds float64              `json:"offset_seconds"`
	CycleSeconds  float64              `json:"cycle_seconds"`
	Phases        []TrafficPhaseConfig `json:"phases"`
	Generated     bool                 `json:"generated"`
	PlanningBasis string               `json:"planning_basis"`
	TimingMethod  string               `json:"timing_method"`
	Adaptive      bool                 `json:"adaptive"`
	LostTime      float64              `json:"lost_time_seconds"`
	VolumeRatio   float64              `json:"design_volume_ratio"`
	// MaxRedSeconds bounds phase starvation in generated max-pressure plans.
	// Explicit fixed-time plans leave it at zero because their configured cycle
	// already provides the red-time bound.
	MaxRedSeconds float64 `json:"max_red_seconds"`
}

type Way struct {
	Structure      string  `json:"structure"`
	StartElevation float64 `json:"start_elevation"`
	EndElevation   float64 `json:"end_elevation"`
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	From           string  `json:"from"`
	To             string  `json:"to"`
	Lanes          int     `json:"lanes"`
	LaneWidth      float64 `json:"lane_width"`
	// CarriagewayOffset separates opposite directed ways while keeping every
	// lane transform relative to the configured road centre line.
	CarriagewayOffset float64   `json:"carriageway_offset"`
	SpeedLimit        float64   `json:"speed_limit"`
	RoadClass         string    `json:"road_class"`
	Demand            float64   `json:"demand_weight"`
	Weight            int       `json:"weight"`
	Length            float64   `json:"length"`
	Points            []Vec2    `json:"points"`
	Segments          []Segment `json:"segments"`
	cumulative        []float64
}

type Edge struct {
	WayID  string `json:"way_id"`
	From   string `json:"from"`
	To     string `json:"to"`
	Weight int    `json:"weight"`
}

type Graph struct {
	Name          string                   `json:"name"`
	Description   string                   `json:"description"`
	DrivingSide   string                   `json:"driving_side"`
	MovementRules MovementRulesConfig      `json:"movement_rules"`
	Bounds        Bounds                   `json:"bounds"`
	Simulation    SimulationConfig         `json:"simulation"`
	Intersections map[string]*Intersection `json:"intersections"`
	Ways          map[string]*Way          `json:"ways"`
	Edges         []Edge                   `json:"edges"`
	Outgoing      map[string][]*Way        `json:"-"`
	Incoming      map[string][]*Way        `json:"-"`
}

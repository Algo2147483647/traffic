package metro

import geometry "traffic/internal/foundation/geometry"

type Point = geometry.Point
type CarPose = geometry.Pose
type StationView struct {
	Elevation float64 `json:"elevation_m"`
	Transfer  string  `json:"transfer,omitempty"`
	Layer     int     `json:"layer"`
	ID        int     `json:"id"`
	Name      string  `json:"name"`
	Position  Point   `json:"position"`
	Waiting   int     `json:"waiting"`
}
type Passenger struct {
	NetworkID   int
	JourneyID   int
	Car         int
	ID          int
	Destination int
}
type TrainView struct {
	Elevation     float64   `json:"elevation_m"`
	Layer         int       `json:"layer"`
	CarPoses      []CarPose `json:"car_poses,omitempty"`
	CarLength     float64   `json:"car_length_m"`
	CarWidth      float64   `json:"car_width_m"`
	Line          int       `json:"line"`
	Cars          []int     `json:"cars"`
	CarCapacities []int     `json:"car_capacities"`
	Heading       float64   `json:"heading"`
	ID            int       `json:"id"`
	Station       int       `json:"station"`
	Next          int       `json:"next"`
	Direction     int       `json:"direction"`
	State         string    `json:"state"`
	Position      Point     `json:"position"`
	Speed         float64   `json:"speed_mps"`
	Length        float64   `json:"length_m"`
	Capacity      int       `json:"capacity"`
	Passengers    int       `json:"passengers"`
	Departure     float64   `json:"departure_s"`
	Delay         float64   `json:"delay_s"`
	Distance      float64   `json:"distance_m"`
	Authority     float64   `json:"authority_m"`
}
type Block struct {
	ID       string  `json:"id"`
	From     int     `json:"from"`
	To       int     `json:"to"`
	Start    float64 `json:"start_m"`
	End      float64 `json:"end_m"`
	Owner    int     `json:"reserved_by"`
	Occupied int     `json:"occupied_by"`
}
type Route struct {
	Station int `json:"station"`
	Owner   int `json:"owner"`
}
type LineView struct {
	Elevation     float64      `json:"elevation_m"`
	StopChainages [][2]float64 `json:"stop_chainages,omitempty"`
	ReverseTrack  []Point      `json:"reverse_track,omitempty"`
	Circular      bool         `json:"circular"`
	Layer         int          `json:"layer"`
	Track         []Point      `json:"track"`
	TunnelWidth   float64      `json:"tunnel_inner_diameter_m"`
	TunnelOuter   float64      `json:"tunnel_outer_diameter_m"`
	ID            int          `json:"id"`
	Name          string       `json:"name"`
	Stations      []int        `json:"stations"`
}
type View struct {
	Services           []ServiceView    `json:"services,omitempty"`
	ServicePlanners    []string         `json:"service_planners,omitempty"`
	StationDetails     []StationDetail  `json:"station_details,omitempty"`
	HighestLayer       int              `json:"highest_layer"`
	Transferring       int              `json:"transferring"`
	TransfersCompleted int              `json:"transfers_completed"`
	Interchanges       []TransferSite   `json:"interchanges,omitempty"`
	Journeys           []NetworkJourney `json:"journeys,omitempty"`
	CrossingOwner      int              `json:"crossing_owner"`
	Lines              []LineView       `json:"lines"`
	Layer              int              `json:"layer"`
	Config             Config           `json:"config"`
	Time               float64          `json:"time"`
	Stations           []StationView    `json:"stations"`
	Trains             []TrainView      `json:"trains"`
	Blocks             []Block          `json:"blocks"`
	Routes             []Route          `json:"routes"`
	Generated          int              `json:"generated"`
	Waiting            int              `json:"waiting"`
	Onboard            int              `json:"onboard"`
	Completed          int              `json:"completed"`
	Departures         int              `json:"departures"`
}

type PlatformDirection struct {
	Direction int      `json:"direction"`
	Toward    int      `json:"toward"`
	Waiting   int      `json:"waiting"`
	Train     int      `json:"train"`
	ETA       *float64 `json:"eta_seconds"`
	Status    string   `json:"status"`
}
type PlatformDetail struct {
	Elevation  float64             `json:"elevation_m"`
	Station    int                 `json:"station"`
	Line       int                 `json:"line"`
	Layer      int                 `json:"layer"`
	Directions []PlatformDirection `json:"directions"`
}
type StationDetail struct {
	ID           string           `json:"id"`
	Transferring int              `json:"transferring"`
	Platforms    []PlatformDetail `json:"platforms"`
	PassengerIDs []int            `json:"passenger_ids"`
}

type ServicePlan struct {
	Line      int     `json:"line"`
	Name      string  `json:"name"`
	Fleet     int     `json:"fleet"`
	Headway   float64 `json:"headway_s"`
	Requested float64 `json:"requested_headway_s"`
	Cycle     float64 `json:"cycle_s"`
	Minimum   float64 `json:"minimum_headway_s"`
	Limit     int     `json:"circulation_limit"`
}

type ServiceTrip struct {
	ID        int     `json:"id"`
	Direction int     `json:"direction"`
	Origin    int     `json:"origin"`
	Planned   float64 `json:"planned_s"`
	Actual    float64 `json:"actual_s"`
	Finished  float64 `json:"finished_s"`
	Train     int     `json:"train"`
	State     string  `json:"state"`
}
type ServiceView struct {
	Plan      ServicePlan   `json:"plan"`
	Active    int           `json:"active"`
	Depot     int           `json:"depot"`
	Retiring  int           `json:"retiring"`
	Withdrawn int           `json:"withdrawn"`
	Departed  int           `json:"departed"`
	Completed int           `json:"completed"`
	Trips     []ServiceTrip `json:"trips"`
	Depots    []int         `json:"depots"`
}

type TransferSite struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Stations [2]int  `json:"stations"`
	Position Point   `json:"position"`
	Seconds  float64 `json:"walk_seconds"`
}

type Control struct {
	Train       int     `json:"train"`
	Speed       float64 `json:"speed_mps"`
	DepartAfter float64 `json:"depart_after_s"`
}

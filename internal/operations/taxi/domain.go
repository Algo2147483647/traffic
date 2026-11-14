package taxi

import model "traffic/internal/infrastructure/road"

type Config struct {
	Fleet      int                `json:"fleet"`
	Demand     float64            `json:"orders_per_minute"`
	Algorithm  string             `json:"algorithm"`
	Seed       int64              `json:"seed"`
	MaxWait    float64            `json:"max_wait_s"`
	Dwell      float64            `json:"dwell_s"`
	Interval   float64            `json:"dispatch_interval_s"`
	Parameters map[string]float64 `json:"parameters,omitempty"`
}
type Stop struct {
	ID       string     `json:"id"`
	WayID    string     `json:"way_id"`
	Along    float64    `json:"along"`
	Position model.Vec2 `json:"position"`
}
type Order struct {
	ID          int     `json:"id"`
	Pickup      string  `json:"pickup"`
	Destination string  `json:"destination"`
	State       string  `json:"state"`
	VehicleID   int     `json:"vehicle_id"`
	Created     float64 `json:"created_s"`
	Assigned    float64 `json:"assigned_s"`
	Picked      float64 `json:"picked_s"`
	Completed   float64 `json:"completed_s"`
	ETA         float64 `json:"pickup_eta_s"`
}

// Accepting an assignment makes the vehicle unavailable until completion/cancellation.
func CanAssign(vehicleState, orderState string) bool {
	return vehicleState == "Idle" && orderState == "Waiting"
}
func Eligible(state string, retiring, turning, changing bool) bool {
	return state == "Idle" && !retiring && !turning && !changing
}

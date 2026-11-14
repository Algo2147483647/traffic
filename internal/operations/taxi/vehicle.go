package taxi

import "math"

// Vehicle is operational state only. Physical position and route belong to road.
type Vehicle struct {
	Retiring                                 bool    `json:"retiring"`
	ID                                       int     `json:"id"`
	State                                    string  `json:"state"`
	OrderID                                  int     `json:"order_id"`
	Completed                                int     `json:"completed"`
	Income                                   float64 `json:"income"`
	Occupied                                 float64 `json:"occupied_s"`
	EmptyDistance                            float64 `json:"empty_distance_m"`
	OccupiedDistance                         float64 `json:"occupied_distance_m"`
	tripDistanceStart, idleSince, dwellUntil float64
}

func NewVehicle(id int, now float64) Vehicle { return Vehicle{ID: id, State: "Idle", idleSince: now} }
func (v *Vehicle) Assign(o *Order, now, eta float64) bool {
	return !v.Retiring && Assign(&v.State, &v.OrderID, o, v.ID, now, eta)
}
func (v *Vehicle) Depart(o *Order, now float64) bool {
	return o != nil && o.ID == v.OrderID && o.VehicleID == v.ID && v.DwellReady(now) && Depart(&v.State, o)
}
func (v *Vehicle) IdleTime(now float64) float64 { return now - v.idleSince }
func (v *Vehicle) MarkIdle(now float64)         { v.idleSince = math.Min(v.idleSince, now) }
func (v *Vehicle) Release(now float64)          { v.State = "Idle"; v.OrderID = 0; v.idleSince = now }
func (v *Vehicle) Pickup(o *Order, now, dwell float64) bool {
	if o == nil || v.OrderID != o.ID || o.VehicleID != v.ID || !Pickup(&v.State, o, now) {
		return false
	}
	v.tripDistanceStart = v.OccupiedDistance
	v.dwellUntil = now + dwell
	return true
}
func (v *Vehicle) Dropoff(now, dwell float64) bool {
	if !Dropoff(&v.State) {
		return false
	}
	v.dwellUntil = now + dwell
	return true
}
func (v *Vehicle) DwellReady(now float64) bool {
	return (v.State == "Boarding" || v.State == "Alighting") && now >= v.dwellUntil
}
func (v *Vehicle) Complete(o *Order, now float64) bool {
	if o == nil || o.VehicleID != v.ID || !v.DwellReady(now) || !Complete(&v.State, &v.OrderID, o, now) {
		return false
	}
	v.Completed++
	v.Income += 3.4 + 1.8*(v.OccupiedDistance-v.tripDistanceStart)/1000
	v.idleSince = now
	return true
}
func (v *Vehicle) Account(dt, distance float64) {
	if v.State == "WithPassenger" {
		v.Occupied += dt
		v.OccupiedDistance += distance
	} else {
		v.EmptyDistance += distance
	}
}

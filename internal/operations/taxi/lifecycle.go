package taxi

// Lifecycle owns operational transitions. The road adapter supplies arrivals
// and successful route admission; it retains sole ownership of physical state.
func Assign(state *string, task *int, o *Order, vehicle int, now, eta float64) bool {
	if o == nil || !CanAssign(*state, o.State) || *task != 0 {
		return false
	}
	o.State = "Assigned"
	o.VehicleID = vehicle
	o.Assigned = now
	o.ETA = eta
	*state = "ToPickup"
	*task = o.ID
	return true
}
func Pickup(state *string, o *Order, now float64) bool {
	if o == nil || *state != "ToPickup" || o.State != "Assigned" {
		return false
	}
	*state = "Boarding"
	o.State = "Boarding"
	o.Picked = now
	return true
}
func Depart(state *string, o *Order) bool {
	if o == nil || *state != "Boarding" || o.State != "Boarding" {
		return false
	}
	*state = "WithPassenger"
	o.State = "InService"
	return true
}
func Dropoff(state *string) bool {
	if *state != "WithPassenger" {
		return false
	}
	*state = "Alighting"
	return true
}
func Complete(state *string, task *int, o *Order, now float64) bool {
	if o == nil || *state != "Alighting" || *task != o.ID || o.State != "InService" {
		return false
	}
	o.State = "Completed"
	o.Completed = now
	*state = "Idle"
	*task = 0
	return true
}
func Cancel(o *Order) bool {
	if o == nil || (o.State != "Waiting" && o.State != "Assigned") {
		return false
	}
	o.State = "Cancelled"
	return true
}

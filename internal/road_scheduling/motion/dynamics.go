package motion

import "math"

const (
	VehicleAcceleration = 2.0
	ComfortableBraking  = 3.0
	EmergencyBraking    = 8.0
	FollowingTime       = 1.5
)

// FollowingSpeed is a desired speed, not an instantaneous speed assignment.
// At equilibrium, net clearance is standstill clearance plus speed * headway.
func FollowingSpeed(speed, leaderSpeed, gap, dt float64) float64 {
	return math.Max(0, leaderSpeed+(gap-speed*FollowingTime)/FollowingTime)
}

// StoppingSpeed includes the next integration interval in the stopping envelope.
func StoppingSpeed(distance, leaderSpeed, braking, dt float64) float64 {
	return math.Max(0, math.Sqrt(braking*braking*dt*dt+leaderSpeed*leaderSpeed+2*braking*math.Max(0, distance))-braking*dt)
}

func AdvanceSpeed(speed, desired, safety, dt float64) float64 {
	v := math.Min(speed+VehicleAcceleration*dt, math.Max(speed-ComfortableBraking*dt, desired))
	// Host safety wins over comfort, including an infeasible externally inserted gap.
	return math.Max(0, math.Min(v, safety))
}

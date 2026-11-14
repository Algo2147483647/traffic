package sim

import "math"

const (
	vehicleAcceleration = 2.0
	comfortableBraking  = 3.0
	emergencyBraking    = 8.0
	followingTime       = 1.5
)

// followingSpeed is a desired speed, not an instantaneous speed assignment.
// At equilibrium, net clearance is standstill clearance plus speed * headway.
func followingSpeed(speed, leaderSpeed, gap, dt float64) float64 {
	return math.Max(0, leaderSpeed+(gap-speed*followingTime)/followingTime)
}

// stoppingSpeed includes the next integration interval in the stopping envelope.
func stoppingSpeed(distance, leaderSpeed, braking, dt float64) float64 {
	return math.Max(0, math.Sqrt(braking*braking*dt*dt+leaderSpeed*leaderSpeed+2*braking*math.Max(0, distance))-braking*dt)
}

func advanceSpeed(speed, desired, safety, dt float64) float64 {
	v := math.Min(speed+vehicleAcceleration*dt, math.Max(speed-comfortableBraking*dt, desired))
	// Host safety wins over comfort, including an infeasible externally inserted gap.
	return math.Max(0, math.Min(v, safety))
}

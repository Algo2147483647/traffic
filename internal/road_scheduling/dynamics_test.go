package roadscheduling

import (
	"math"
	"testing"
	"traffic/internal/road_scheduling/motion"
)

func TestAccelerationAndComfortableBraking(t *testing.T) {
	v := 0.0
	for i := 0; i < 100; i++ {
		v = motion.AdvanceSpeed(v, 20, math.Inf(1), .05)
	}
	if math.Abs(v-10) > 1e-8 {
		t.Fatalf("five-second acceleration: %v", v)
	}
	v = motion.AdvanceSpeed(v, 0, math.Inf(1), .05)
	if math.Abs(v-9.85) > 1e-8 {
		t.Fatalf("normal braking: %v", v)
	}
}

func TestHeadwayEquilibriumAndEmergencyQueue(t *testing.T) {
	const dt = .05
	for _, speed := range []float64{5, 15, 25} {
		if got := motion.FollowingSpeed(speed, speed, speed*motion.FollowingTime, dt); math.Abs(got-speed) > 1e-8 {
			t.Fatal(got)
		}
	}
	// A leader brakes at the emergency limit from a steady 1.5-second headway.
	lx, fx, lv, fv := 100.0, 60.0, 25.0, 25.0
	for i := 0; i < 600; i++ {
		gap := lx - fx - 2.5
		desired := motion.FollowingSpeed(fv, lv, gap, dt)
		safety := math.Min(gap/dt, motion.StoppingSpeed(gap, lv, motion.EmergencyBraking, dt))
		next := motion.AdvanceSpeed(fv, desired, safety, dt)
		if next-fv > motion.VehicleAcceleration*dt+1e-8 {
			t.Fatal("acceleration exceeded")
		}
		if fv-next > motion.EmergencyBraking*dt+1e-6 {
			t.Fatalf("emergency deceleration exceeded: %v", (fv-next)/dt)
		}
		lv = math.Max(0, lv-motion.EmergencyBraking*dt)
		lx += lv * dt
		fx += next * dt
		fv = next
		if lx-fx < 2.5-1e-8 {
			t.Fatalf("overlap at step %d", i)
		}
	}
	if fv > .01 {
		t.Fatalf("queue failed to stop: %v", fv)
	}
}

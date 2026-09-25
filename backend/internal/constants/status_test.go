package constants

import "testing"

func TestVesselCallTransitionGraph(t *testing.T) {
	if !CanTransition(VesselCallTransitions, "planned", "approach") {
		t.Fatalf("expected planned -> approach transition to be allowed")
	}
	if CanTransition(VesselCallTransitions, "planned", "unknown") {
		t.Fatal("unknown status must never be accepted")
	}
}

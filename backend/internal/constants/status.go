package constants

// Shared status values are mirrored in frontend/src/types/status.ts. Keeping
// the lists explicit makes state-machine drift visible during code review.

type CallState string

const (
	CallStatePlanned  CallState = "planned"
	CallStateApproach CallState = "approach"
	CallStateMoored   CallState = "moored"
	CallStateDeparted CallState = "departed"
)

var AllCallState = []string{"planned", "approach", "moored", "departed"}

type ClearanceState string

const (
	ClearanceStatePending    ClearanceState = "pending"
	ClearanceStateCleared    ClearanceState = "cleared"
	ClearanceStateRestricted ClearanceState = "restricted"
	ClearanceStateExpired    ClearanceState = "expired"
)

var AllClearanceState = []string{"pending", "cleared", "restricted", "expired"}

var VesselCallTransitions = map[string]map[string]bool{
	"planned":  {"approach": true, "moored": true},
	"approach": {"moored": true, "departed": true, "planned": true},
	"moored":   {"departed": true, "approach": true},
	"departed": {"moored": true},
}

var MooringPlanTransitions = map[string]map[string]bool{
	"draft":      {"review": true, "approved": true},
	"review":     {"approved": true, "superseded": true, "draft": true},
	"approved":   {"superseded": true, "review": true},
	"superseded": {"approved": true},
}

var WeatherWindowTransitions = map[string]map[string]bool{
	"forecast":   {"safe": true, "restricted": true},
	"safe":       {"restricted": true, "expired": true, "forecast": true},
	"restricted": {"expired": true, "safe": true},
	"expired":    {"restricted": true},
}

var SafetyClearanceTransitions = map[string]map[string]bool{
	"pending":    {"cleared": true, "restricted": true, "expired": true},
	"cleared":    {"restricted": true, "expired": true, "pending": true},
	"restricted": {"expired": true, "cleared": true, "pending": true},
	"expired":    {"restricted": true, "pending": true},
}

func CanTransition(graph map[string]map[string]bool, from, to string) bool {
	targets, exists := graph[from]
	return exists && targets[to]
}

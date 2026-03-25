package collector

const (
	workflowPhasePending   = "Pending"
	workflowPhaseRunning   = "Running"
	workflowPhaseSucceeded = "Succeeded"
	workflowPhaseFailed    = "Failed"
	workflowPhaseError     = "Error"
)

var workflowPhases = []string{
	workflowPhasePending,
	workflowPhaseRunning,
	workflowPhaseSucceeded,
	workflowPhaseFailed,
	workflowPhaseError,
}

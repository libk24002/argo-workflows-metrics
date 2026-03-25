package collector

import "testing"

func TestDiffNamespaces(t *testing.T) {
	previous := map[string]struct{}{
		"team-a": {},
		"team-b": {},
	}
	current := map[string]struct{}{
		"team-b": {},
		"team-c": {},
	}

	stale := diffNamespaces(previous, current)
	if len(stale) != 1 {
		t.Fatalf("expected 1 stale namespace, got %d", len(stale))
	}
	if _, ok := stale["team-a"]; !ok {
		t.Fatalf("expected team-a to be stale")
	}
}

func TestDiffNamespacePhases(t *testing.T) {
	previous := map[string]map[string]struct{}{
		"team-a": {
			workflowPhaseRunning:   {},
			workflowPhaseSucceeded: {},
		},
		"team-b": {
			workflowPhaseFailed: {},
		},
	}
	current := map[string]map[string]struct{}{
		"team-a": {
			workflowPhaseRunning: {},
		},
	}

	stale := diffNamespacePhases(previous, current)
	if len(stale) != 2 {
		t.Fatalf("expected stale phases for two namespaces, got %d", len(stale))
	}
	if _, ok := stale["team-a"][workflowPhaseSucceeded]; !ok {
		t.Fatalf("expected team-a/succeeded to be stale")
	}
	if _, ok := stale["team-b"][workflowPhaseFailed]; !ok {
		t.Fatalf("expected team-b/failed to be stale")
	}
}

func TestWorkflowPhasesContainsSucceeded(t *testing.T) {
	found := false
	for _, phase := range workflowPhases {
		if phase == workflowPhaseSucceeded {
			found = true
			break
		}
	}

	if !found {
		t.Fatalf("expected workflow phases to include %q", workflowPhaseSucceeded)
	}
}

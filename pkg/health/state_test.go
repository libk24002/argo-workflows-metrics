package health

import (
	"testing"
	"time"
)

func TestStateReadinessRequiresInformerSync(t *testing.T) {
	state := NewState(0, 30*time.Minute)

	ready, reason := state.IsReady(time.Now())
	if ready {
		t.Fatalf("expected not ready before sync")
	}
	if reason != "workflow informer not synced" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	state.MarkWorkflowSynced()
	ready, reason = state.IsReady(time.Now())
	if ready {
		t.Fatalf("expected not ready when pod informer is unsynced")
	}
	if reason != "pod informer not synced" {
		t.Fatalf("unexpected reason: %s", reason)
	}

	state.MarkPodSynced()
	ready, reason = state.IsReady(time.Now())
	if !ready {
		t.Fatalf("expected ready after informer sync, got reason: %s", reason)
	}
}

func TestStateReadinessStaleEvent(t *testing.T) {
	state := NewState(0, 10*time.Minute)
	state.MarkWorkflowSynced()
	state.MarkPodSynced()

	state.mu.Lock()
	state.lastWorkflowEvt = time.Now().Add(-11 * time.Minute)
	state.lastPodEvt = time.Now().Add(-11 * time.Minute)
	state.mu.Unlock()

	ready, reason := state.IsReady(time.Now())
	if ready {
		t.Fatalf("expected not ready for stale events")
	}
	if reason == "" {
		t.Fatalf("expected non-empty stale reason")
	}
}

func TestStateLivenessShutdown(t *testing.T) {
	state := NewState(0, 10*time.Minute)

	alive, reason := state.IsLive(time.Now())
	if !alive {
		t.Fatalf("expected alive before shutdown, got: %s", reason)
	}

	state.MarkShuttingDown()
	alive, reason = state.IsLive(time.Now())
	if alive {
		t.Fatalf("expected unhealthy after shutdown")
	}
	if reason != "shutting down" {
		t.Fatalf("unexpected shutdown reason: %s", reason)
	}
}

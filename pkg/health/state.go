package health

import (
	"fmt"
	"sync"
	"time"

	"github.com/conti/argo-workflows-metrics/pkg/metrics"
)

const (
	workflowInformer = "workflow"
	podInformer      = "pod"
)

type Snapshot struct {
	WorkflowSynced  bool
	PodSynced       bool
	LastWorkflowEvt time.Time
	LastPodEvt      time.Time
	LeaderElect     bool
	IsLeader        bool
	ShuttingDown    bool
}

type State struct {
	mu              sync.RWMutex
	startedAt       time.Time
	startupGrace    time.Duration
	staleThreshold  time.Duration
	workflowSynced  bool
	podSynced       bool
	lastWorkflowEvt time.Time
	lastPodEvt      time.Time
	leaderElect     bool
	isLeader        bool
	shuttingDown    bool
}

func NewState(startupGrace, staleThreshold time.Duration, leaderElect bool) *State {
	s := &State{
		startedAt:      time.Now(),
		startupGrace:   startupGrace,
		staleThreshold: staleThreshold,
		leaderElect:    leaderElect,
	}

	metrics.ExporterInformerSynced.WithLabelValues(workflowInformer).Set(0)
	metrics.ExporterInformerSynced.WithLabelValues(podInformer).Set(0)
	metrics.ExporterLastEventTimestamp.WithLabelValues(workflowInformer).Set(0)
	metrics.ExporterLastEventTimestamp.WithLabelValues(podInformer).Set(0)
	metrics.ExporterShuttingDown.Set(0)
	metrics.ExporterReadiness.Set(0)
	metrics.ExporterLiveness.Set(1)
	metrics.ExporterIsLeader.Set(0)

	return s
}

func (s *State) MarkLeader(elected bool) {
	s.mu.Lock()
	s.isLeader = elected
	s.mu.Unlock()

	if elected {
		metrics.ExporterIsLeader.Set(1)
		metrics.ExporterLeaderTransitionsTotal.WithLabelValues("leader").Inc()
		return
	}
	metrics.ExporterIsLeader.Set(0)
	metrics.ExporterLeaderTransitionsTotal.WithLabelValues("follower").Inc()
}

func (s *State) MarkWorkflowSynced() {
	s.mu.Lock()
	s.workflowSynced = true
	s.mu.Unlock()
	metrics.ExporterInformerSynced.WithLabelValues(workflowInformer).Set(1)
}

func (s *State) MarkPodSynced() {
	s.mu.Lock()
	s.podSynced = true
	s.mu.Unlock()
	metrics.ExporterInformerSynced.WithLabelValues(podInformer).Set(1)
}

func (s *State) MarkWorkflowEvent() {
	now := time.Now()
	s.mu.Lock()
	s.lastWorkflowEvt = now
	s.mu.Unlock()
	metrics.ExporterLastEventTimestamp.WithLabelValues(workflowInformer).Set(float64(now.Unix()))
}

func (s *State) MarkPodEvent() {
	now := time.Now()
	s.mu.Lock()
	s.lastPodEvt = now
	s.mu.Unlock()
	metrics.ExporterLastEventTimestamp.WithLabelValues(podInformer).Set(float64(now.Unix()))
}

func (s *State) MarkShuttingDown() {
	s.mu.Lock()
	s.shuttingDown = true
	s.mu.Unlock()
	metrics.ExporterShuttingDown.Set(1)
}

func (s *State) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return Snapshot{
		WorkflowSynced:  s.workflowSynced,
		PodSynced:       s.podSynced,
		LastWorkflowEvt: s.lastWorkflowEvt,
		LastPodEvt:      s.lastPodEvt,
		LeaderElect:     s.leaderElect,
		IsLeader:        s.isLeader,
		ShuttingDown:    s.shuttingDown,
	}
}

func (s *State) IsLive(now time.Time) (bool, string) {
	_ = now
	snapshot := s.Snapshot()
	if snapshot.ShuttingDown {
		metrics.ExporterLiveness.Set(0)
		return false, "shutting down"
	}
	metrics.ExporterLiveness.Set(1)
	return true, "alive"
}

func (s *State) IsReady(now time.Time) (bool, string) {
	snapshot := s.Snapshot()

	if snapshot.ShuttingDown {
		metrics.ExporterReadiness.Set(0)
		return false, "shutting down"
	}

	if snapshot.LeaderElect && !snapshot.IsLeader {
		metrics.ExporterReadiness.Set(0)
		return false, "not leader"
	}

	if !snapshot.WorkflowSynced {
		metrics.ExporterReadiness.Set(0)
		return false, "workflow informer not synced"
	}

	if !snapshot.PodSynced {
		metrics.ExporterReadiness.Set(0)
		return false, "pod informer not synced"
	}

	if s.staleThreshold <= 0 {
		metrics.ExporterReadiness.Set(1)
		return true, "ready"
	}

	if now.Sub(s.startedAt) < s.startupGrace {
		metrics.ExporterReadiness.Set(1)
		return true, "ready (startup grace)"
	}

	lastEvent := maxTime(snapshot.LastWorkflowEvt, snapshot.LastPodEvt)
	if lastEvent.IsZero() {
		metrics.ExporterReadiness.Set(1)
		return true, "ready (no events yet)"
	}

	idleFor := now.Sub(lastEvent)
	if idleFor > s.staleThreshold {
		metrics.ExporterReadiness.Set(0)
		return false, fmt.Sprintf("no workflow/pod events for %s", idleFor.Round(time.Second))
	}

	metrics.ExporterReadiness.Set(1)
	return true, "ready"
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

package scheduler

import (
	"context"
	"errors"
	"testing"

	"infinite-experiment/politburo/internal/metrics"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

type testJob struct {
	name string
	err  error
}

func (j testJob) Name() string              { return j.name }
func (j testJob) Run(context.Context) error { return j.err }

func TestRegisterRejectsDuplicateNames(t *testing.T) {
	s := New(metrics.NewRegistry())
	t.Cleanup(s.Stop)
	if err := s.Register(testJob{name: "example"}, "0 * * * * *"); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := s.Register(testJob{name: "example"}, "0 * * * * *"); err == nil {
		t.Fatal("duplicate Register() error = nil")
	}
}

func TestRegisterReturnsInvalidSchedule(t *testing.T) {
	s := New(metrics.NewRegistry())
	t.Cleanup(s.Stop)
	if err := s.Register(testJob{name: "example"}, "invalid"); err == nil {
		t.Fatal("Register() error = nil, want invalid schedule error")
	}
}

func TestRunRecordsSuccessMetrics(t *testing.T) {
	registry := metrics.NewRegistry()
	s := New(registry)
	t.Cleanup(s.Stop)

	s.run(testJob{name: "example"})

	if got := testutil.ToFloat64(registry.JobRuns.WithLabelValues("example", "success")); got != 1 {
		t.Fatalf("successful runs = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registry.JobRunning.WithLabelValues("example")); got != 0 {
		t.Fatalf("running = %v, want 0", got)
	}
	if got := testutil.ToFloat64(registry.JobLastSuccess.WithLabelValues("example")); got <= 0 {
		t.Fatalf("last success timestamp = %v, want positive value", got)
	}
}

func TestRunRecordsFailureMetrics(t *testing.T) {
	registry := metrics.NewRegistry()
	s := New(registry)
	t.Cleanup(s.Stop)

	s.run(testJob{name: "example", err: errors.New("failed")})

	if got := testutil.ToFloat64(registry.JobRuns.WithLabelValues("example", "error")); got != 1 {
		t.Fatalf("failed runs = %v, want 1", got)
	}
	if got := testutil.ToFloat64(registry.JobLastSuccess.WithLabelValues("example")); got != 0 {
		t.Fatalf("last success timestamp = %v, want 0", got)
	}
}

package scheduler

import (
	"context"
	"testing"
)

type testJob struct{ name string }

func (j testJob) Name() string              { return j.name }
func (j testJob) Run(context.Context) error { return nil }

func TestRegisterRejectsDuplicateNames(t *testing.T) {
	s := New()
	t.Cleanup(s.Stop)
	if err := s.Register(testJob{name: "example"}, "0 * * * * *"); err != nil {
		t.Fatalf("first Register() error = %v", err)
	}
	if err := s.Register(testJob{name: "example"}, "0 * * * * *"); err == nil {
		t.Fatal("duplicate Register() error = nil")
	}
}

func TestRegisterReturnsInvalidSchedule(t *testing.T) {
	s := New()
	t.Cleanup(s.Stop)
	if err := s.Register(testJob{name: "example"}, "invalid"); err == nil {
		t.Fatal("Register() error = nil, want invalid schedule error")
	}
}

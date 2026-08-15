package flights

import "testing"

func TestContainsFoldTreatsBlankQueryAsMatch(t *testing.T) {
	if !ContainsFold("Swiss 39 Heavy", "") || !ContainsFold("Swiss 39 Heavy", "  ") {
		t.Fatal("blank query should match")
	}
}

func TestContainsFoldIsCaseInsensitiveSubstring(t *testing.T) {
	if !ContainsFold("Swiss 39 Heavy", "swiss") || ContainsFold("Swiss 39 Heavy", "lufthansa") {
		t.Fatal("expected case-insensitive substring match")
	}
}

func TestUsernameReturnsEmptyWhenNil(t *testing.T) {
	if Username(Flight{}) != "" {
		t.Fatal("nil username should be empty")
	}
	name := "Hantder"
	if Username(Flight{Username: &name}) != "Hantder" {
		t.Fatal("expected stored username")
	}
}

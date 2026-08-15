package ui

import "testing"

func TestEmbeddedTemplatesParse(t *testing.T) {
	if _, err := NewRenderer(); err != nil {
		t.Fatalf("NewRenderer() error = %v", err)
	}
}

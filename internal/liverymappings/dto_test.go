package liverymappings

import "testing"

func TestNormalizeCreateMappingRequest(t *testing.T) {
	req := createMappingRequest{
		FieldType:        " aircraft ",
		SourceValue:      " A320 ",
		TargetValue:      " B738 ",
		ConflictStrategy: " skip ",
	}

	normalizeCreateMappingRequest(&req)

	if req.FieldType != "aircraft" {
		t.Fatalf("FieldType = %q, want %q", req.FieldType, "aircraft")
	}
	if req.SourceValue != "A320" {
		t.Fatalf("SourceValue = %q, want %q", req.SourceValue, "A320")
	}
	if req.TargetValue != "B738" {
		t.Fatalf("TargetValue = %q, want %q", req.TargetValue, "B738")
	}
	if req.ConflictStrategy != "skip" {
		t.Fatalf("ConflictStrategy = %q, want %q", req.ConflictStrategy, "skip")
	}
}

func TestIsSupportedFieldType(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		want      bool
	}{
		{name: "aircraft", fieldType: "aircraft", want: true},
		{name: "airline", fieldType: "airline", want: true},
		{name: "invalid", fieldType: "flight_number", want: false},
		{name: "empty", fieldType: "", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isSupportedFieldType(tc.fieldType); got != tc.want {
				t.Fatalf("isSupportedFieldType(%q) = %t, want %t", tc.fieldType, got, tc.want)
			}
		})
	}
}

func TestUniqueStrings(t *testing.T) {
	got := uniqueStrings([]string{"", "A320", "A320", "B738", "", "B738", "E190"})
	want := []string{"A320", "B738", "E190"}

	if len(got) != len(want) {
		t.Fatalf("len(uniqueStrings) = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("uniqueStrings[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

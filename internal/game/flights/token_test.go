package flights

import "testing"

func TestMarkerTokenRoundTrip(t *testing.T) {
	tokens := NewTokens([]byte("0123456789abcdef0123456789abcdef"))
	encoded, err := tokens.Encode(MarkerToken{FlightID: "flight-1", ServerID: "casual"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if encoded == "flight-1" {
		t.Fatal("token must not be the raw flight id")
	}
	decoded, err := tokens.Decode(encoded)
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if decoded.FlightID != "flight-1" || decoded.ServerID != "casual" {
		t.Fatalf("decoded = %#v", decoded)
	}
}

func TestMarkerTokenRejectsTampering(t *testing.T) {
	tokens := NewTokens([]byte("0123456789abcdef0123456789abcdef"))
	encoded, err := tokens.Encode(MarkerToken{FlightID: "flight-1", ServerID: "casual"})
	if err != nil {
		t.Fatalf("Encode() error = %v", err)
	}
	if _, err := tokens.Decode(encoded + "x"); err != ErrInvalidFlightToken {
		t.Fatalf("error = %v, want ErrInvalidFlightToken", err)
	}
	if _, err := tokens.Decode("not-a-token"); err != ErrInvalidFlightToken {
		t.Fatalf("error = %v, want ErrInvalidFlightToken", err)
	}
}

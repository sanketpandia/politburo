package flights

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

var ErrInvalidFlightToken = errors.New("invalid flight token")

// MarkerToken is the encrypted map marker identity. ServerID is included so a
// later detail lookup can find the right cache key without a query param.
type MarkerToken struct {
	FlightID string `json:"f"`
	ServerID string `json:"s"`
}

type Tokens struct {
	secret []byte
}

func NewTokens(secret []byte) *Tokens {
	return &Tokens{secret: secret}
}

func (t *Tokens) Encode(token MarkerToken) (string, error) {
	if t == nil || len(t.secret) != 32 {
		return "", fmt.Errorf("flight token secret must be 32 bytes")
	}
	if token.FlightID == "" || token.ServerID == "" {
		return "", fmt.Errorf("flight token requires flightId and serverId")
	}
	payload, err := json.Marshal(token)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(t.secret)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := gcm.Seal(nonce, nonce, payload, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func (t *Tokens) Decode(raw string) (MarkerToken, error) {
	var token MarkerToken
	if t == nil || len(t.secret) != 32 {
		return token, ErrInvalidFlightToken
	}
	sealed, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return token, ErrInvalidFlightToken
	}
	block, err := aes.NewCipher(t.secret)
	if err != nil {
		return token, ErrInvalidFlightToken
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return token, ErrInvalidFlightToken
	}
	nonceSize := gcm.NonceSize()
	if len(sealed) < nonceSize {
		return token, ErrInvalidFlightToken
	}
	payload, err := gcm.Open(nil, sealed[:nonceSize], sealed[nonceSize:], nil)
	if err != nil {
		return token, ErrInvalidFlightToken
	}
	if err := json.Unmarshal(payload, &token); err != nil || token.FlightID == "" || token.ServerID == "" {
		return MarkerToken{}, ErrInvalidFlightToken
	}
	return token, nil
}

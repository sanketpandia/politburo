package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"infinite-experiment/politburo/internal/cache"
)

const TicketTTL = 10 * time.Minute

var ErrInvalidTicket = errors.New("invalid or expired login ticket")

type LoginTicket struct {
	UserID          string `json:"user_id"`
	DiscordUserID   string `json:"discord_user_id"`
	DiscordServerID string `json:"discord_server_id,omitempty"`
	Username        string `json:"username,omitempty"`
	RedirectTo      string `json:"redirect_to"`
}

type ticketCache interface {
	SetJSON(context.Context, string, any, time.Duration) error
	GetDelJSON(context.Context, string, any) error
}

type Tickets struct {
	cache  ticketCache
	secret []byte
}

func NewTickets(store ticketCache, secret []byte) *Tickets {
	return &Tickets{cache: store, secret: secret}
}

func (t *Tickets) Issue(ctx context.Context, ticket LoginTicket) (string, error) {
	ticketID := make([]byte, 32)
	if _, err := rand.Read(ticketID); err != nil {
		return "", fmt.Errorf("generate login ticket: %w", err)
	}
	token, err := encryptTicketID(t.secret, ticketID)
	if err != nil {
		return "", err
	}
	if err := t.cache.SetJSON(ctx, cache.KeyLoginTicket(hex.EncodeToString(ticketID)), ticket, TicketTTL); err != nil {
		return "", err
	}
	return token, nil
}

func (t *Tickets) Consume(ctx context.Context, token string) (*LoginTicket, error) {
	ticketID, err := decryptTicketID(t.secret, token)
	if err != nil {
		return nil, ErrInvalidTicket
	}
	var ticket LoginTicket
	if err := t.cache.GetDelJSON(ctx, cache.KeyLoginTicket(hex.EncodeToString(ticketID)), &ticket); err != nil {
		if errors.Is(err, cache.ErrMiss) {
			return nil, ErrInvalidTicket
		}
		return nil, err
	}
	return &ticket, nil
}

func encryptTicketID(secret, ticketID []byte) (string, error) {
	block, err := aes.NewCipher(secret)
	if err != nil {
		return "", fmt.Errorf("signed link cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("signed link gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("signed link nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, ticketID, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

func decryptTicketID(secret []byte, token string) ([]byte, error) {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(secret)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(raw) < nonceSize {
		return nil, ErrInvalidTicket
	}
	plaintext, err := gcm.Open(nil, raw[:nonceSize], raw[nonceSize:], nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

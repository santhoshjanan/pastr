package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/santhoshjanan/pastr/internal/store"
)

const SessionTTL = 30 * 24 * time.Hour

// NewToken returns 32 random bytes, URL-safe encoded.
func NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// HashToken is what gets stored, so a copy of the database cannot be replayed as a cookie.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type Sessions struct {
	Store *store.Store
	Now   func() time.Time
}

func (s Sessions) Start(ctx context.Context, userID int64) (string, error) {
	token, err := NewToken()
	if err != nil {
		return "", err
	}
	csrf, err := NewToken()
	if err != nil {
		return "", err
	}
	now := s.Now()
	err = s.Store.CreateSession(ctx, store.Session{
		TokenHash: HashToken(token), UserID: userID, CSRFToken: csrf,
		ExpiresAt: now.Add(SessionTTL), CreatedAt: now,
	})
	return token, err
}

// Lookup returns store.ErrNotFound for unknown, expired and disabled-user sessions alike.
func (s Sessions) Lookup(ctx context.Context, token string) (store.Session, store.User, error) {
	sess, err := s.Store.SessionByTokenHash(ctx, HashToken(token), s.Now())
	if err != nil {
		return store.Session{}, store.User{}, err
	}
	u, err := s.Store.UserByID(ctx, sess.UserID)
	if err != nil {
		return store.Session{}, store.User{}, err
	}
	if u.Disabled {
		return store.Session{}, store.User{}, store.ErrNotFound
	}
	return sess, u, nil
}

func (s Sessions) End(ctx context.Context, token string) error {
	return s.Store.DeleteSession(ctx, HashToken(token))
}

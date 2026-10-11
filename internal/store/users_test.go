package store

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

var t0 = time.Unix(1_800_000_000, 0).UTC()

func TestUsers(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	u, err := s.CreateUser(ctx, "ana", "hash", true, t0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateUser(ctx, "ana", "hash2", false, t0); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate username: got %v, want ErrConflict", err)
	}
	got, err := s.UserByUsername(ctx, "ana")
	if err != nil || got.ID != u.ID || !got.IsAdmin || !got.CreatedAt.Equal(t0) {
		t.Errorf("UserByUsername = %+v, %v", got, err)
	}
	if _, err := s.UserByUsername(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown user: got %v, want ErrNotFound", err)
	}
	if err := s.UpdateProfile(ctx, u.ID, "Ana", "https://x.com/ana"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePassword(ctx, u.ID, "new"); err != nil {
		t.Fatal(err)
	}
	got, _ = s.UserByID(ctx, u.ID)
	if got.DisplayName != "Ana" || got.DisplayLink != "https://x.com/ana" || got.PasswordHash != "new" {
		t.Errorf("profile not saved: %+v", got)
	}
	if n, _ := s.CountUsers(ctx); n != 1 {
		t.Errorf("CountUsers = %d", n)
	}
	if n, _ := s.CountEnabledAdmins(ctx); n != 1 {
		t.Errorf("CountEnabledAdmins = %d", n)
	}
}

func TestSessions(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "ana", "hash", false, t0)
	sess := Session{TokenHash: "h1", UserID: u.ID, CSRFToken: "c", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0}
	if err := s.CreateSession(ctx, sess); err != nil {
		t.Fatal(err)
	}
	if got, err := s.SessionByTokenHash(ctx, "h1", t0); err != nil || got.CSRFToken != "c" {
		t.Errorf("lookup = %+v, %v", got, err)
	}
	if _, err := s.SessionByTokenHash(ctx, "h1", t0.Add(2*time.Hour)); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session: got %v, want ErrNotFound", err)
	}
	if n, _ := s.DeleteExpiredSessions(ctx, t0.Add(2*time.Hour)); n != 1 {
		t.Errorf("swept %d sessions, want 1", n)
	}
}

func TestDisableDeletesSessions(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "ana", "hash", false, t0)
	s.CreateSession(ctx, Session{TokenHash: "h1", UserID: u.ID, CSRFToken: "c", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0})
	if err := s.SetDisabled(ctx, u.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByTokenHash(ctx, "h1", t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("session survived disabling: %v", err)
	}
}

func TestSignupSettingDefaultsOff(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	if on, err := s.SignupsEnabled(ctx); err != nil || on {
		t.Errorf("default = %v, %v; want false", on, err)
	}
	s.SetSignupsEnabled(ctx, true)
	if on, _ := s.SignupsEnabled(ctx); !on {
		t.Error("setting did not persist")
	}
}

func TestReopenKeepsData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, _ := Open(path)
	s.CreateUser(context.Background(), "ana", "hash", false, t0)
	s.Close()
	s2, err := Open(path)
	if err != nil {
		t.Fatalf("reopen (migrations must be idempotent): %v", err)
	}
	defer s2.Close()
	if n, _ := s2.CountUsers(context.Background()); n != 1 {
		t.Errorf("CountUsers after reopen = %d", n)
	}
}

func TestPasswordChangeEndsSessions(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	u, _ := s.CreateUser(ctx, "ana", "hash", false, t0)
	s.CreateSession(ctx, Session{TokenHash: "stolen", UserID: u.ID, CSRFToken: "c", ExpiresAt: t0.Add(time.Hour), CreatedAt: t0})
	if err := s.UpdatePassword(ctx, u.ID, "new"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SessionByTokenHash(ctx, "stolen", t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("a session survived the password change: %v", err)
	}
}

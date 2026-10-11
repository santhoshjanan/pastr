package main

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/config"
	"github.com/santhoshjanan/pastr/internal/store"
)

var now = time.Unix(1_800_000_000, 0).UTC()

func open(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestBootstrapAdmin(t *testing.T) {
	ctx := context.Background()

	if err := bootstrapAdmin(ctx, config.Config{}, open(t), now); err == nil {
		t.Error("empty database with no admin variables must refuse to start")
	}
	if err := bootstrapAdmin(ctx, config.Config{AdminUsername: "root", AdminPassword: "short"}, open(t), now); err == nil {
		t.Error("weak admin password accepted")
	}

	st := open(t)
	cfg := config.Config{AdminUsername: "Root", AdminPassword: "a long enough password"}
	if err := bootstrapAdmin(ctx, cfg, st, now); err != nil {
		t.Fatal(err)
	}
	u, err := st.UserByUsername(ctx, "root")
	if err != nil || !u.IsAdmin {
		t.Fatalf("admin not created: %+v, %v", u, err)
	}
	// Second start: users exist, so the variables are ignored, even if changed or removed.
	if err := bootstrapAdmin(ctx, config.Config{}, st, now); err != nil {
		t.Errorf("restart without admin variables failed: %v", err)
	}
	if n, _ := st.CountUsers(ctx); n != 1 {
		t.Errorf("users = %d, want 1", n)
	}
}

func TestSweep(t *testing.T) {
	ctx, st := context.Background(), open(t)
	u, _ := st.CreateUser(ctx, "ana", "h", false, now)
	past := now.Add(-time.Hour)
	st.CreatePaste(ctx, store.Paste{Slug: "aaaaaaaaaa", UserID: u.ID, Title: "t", BodyHTML: "b", RevealMode: "tap", ExpiresAt: &past, CreatedAt: now, UpdatedAt: now})
	st.CreateSession(ctx, store.Session{TokenHash: "h", UserID: u.ID, CSRFToken: "c", ExpiresAt: past, CreatedAt: past})
	sweep(ctx, st, now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if _, err := st.PasteBySlug(ctx, "aaaaaaaaaa"); err != store.ErrNotFound {
		t.Errorf("expired paste survived the sweep: %v", err)
	}
	if n, _ := st.DeleteExpiredSessions(ctx, now); n != 0 {
		t.Error("expired session survived the sweep")
	}
}

func TestHealthURL(t *testing.T) {
	for addr, want := range map[string]string{
		":8080":          "http://127.0.0.1:8080/healthz",
		"0.0.0.0:9000":   "http://127.0.0.1:9000/healthz",
		"127.0.0.1:8080": "http://127.0.0.1:8080/healthz",
		"192.168.1.5:80": "http://192.168.1.5:80/healthz",
		"[::]:8080":      "http://127.0.0.1:8080/healthz",
	} {
		if got := healthURL(addr); got != want {
			t.Errorf("healthURL(%q) = %q, want %q", addr, got, want)
		}
	}
}

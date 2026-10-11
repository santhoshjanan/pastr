package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/santhoshjanan/pastr/internal/store"
)

func TestPasswordHashing(t *testing.T) {
	h, err := HashPassword("correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(h, "$argon2id$v=19$") {
		t.Errorf("unexpected encoding: %s", h)
	}
	if !VerifyPassword(h, "correct horse battery") {
		t.Error("correct password rejected")
	}
	if VerifyPassword(h, "wrong password!") {
		t.Error("wrong password accepted")
	}
	h2, _ := HashPassword("correct horse battery")
	if h == h2 {
		t.Error("two hashes of one password are identical; salt is not random")
	}
	for _, bad := range []string{"", "garbage", "$argon2id$v=19$m=1,t=1,p=1$!!!$!!!"} {
		if VerifyPassword(bad, "x") {
			t.Errorf("malformed hash %q verified", bad)
		}
	}
}

func TestValidation(t *testing.T) {
	if err := ValidatePassword("short"); err == nil {
		t.Error("9-or-fewer character password accepted")
	}
	if err := ValidatePassword("ten chars!"); err != nil {
		t.Errorf("10-character password rejected: %v", err)
	}
	if u, err := ValidateUsername("  Ana_K-9 "); err != nil || u != "ana_k-9" {
		t.Errorf("ValidateUsername = %q, %v", u, err)
	}
	for _, bad := range []string{"ab", strings.Repeat("a", 33), "has space", "émile", "a/b"} {
		if _, err := ValidateUsername(bad); err == nil {
			t.Errorf("username %q accepted", bad)
		}
	}
}

func TestSessions(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ctx := context.Background()
	now := time.Unix(1_800_000_000, 0).UTC()
	u, _ := st.CreateUser(ctx, "ana", "hash", false, now)
	ss := Sessions{Store: st, Now: func() time.Time { return now }}

	token, err := ss.Start(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(token) < 40 {
		t.Errorf("token too short: %q", token)
	}
	sess, got, err := ss.Lookup(ctx, token)
	if err != nil || got.ID != u.ID || sess.CSRFToken == "" || sess.CSRFToken == token {
		t.Fatalf("Lookup = %+v, %+v, %v", sess, got, err)
	}
	if _, err := st.SessionByTokenHash(ctx, token, now); !errors.Is(err, store.ErrNotFound) {
		t.Error("the raw token is stored; only its hash may be")
	}
	if _, _, err := ss.Lookup(ctx, "not-a-token"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("bogus token: %v", err)
	}
	now = now.Add(SessionTTL + time.Second)
	if _, _, err := ss.Lookup(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("expired session still valid: %v", err)
	}
	now = now.Add(-SessionTTL)
	if err := ss.End(ctx, token); err != nil {
		t.Fatal(err)
	}
	if _, _, err := ss.Lookup(ctx, token); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("ended session still valid: %v", err)
	}
}

func TestThrottle(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	th := NewThrottle(5, 15*time.Minute, func() time.Time { return now })
	for i := 0; i < 5; i++ {
		if !th.Allowed("k") {
			t.Fatalf("blocked after %d failures", i)
		}
		th.Fail("k")
	}
	if th.Allowed("k") {
		t.Error("still allowed after 5 failures")
	}
	if !th.Allowed("other") {
		t.Error("unrelated key blocked")
	}
	now = now.Add(16 * time.Minute)
	if !th.Allowed("k") {
		t.Error("still blocked after the window passed")
	}
	th.Fail("k")
	th.Reset("k")
	if !th.Allowed("k") {
		t.Error("Reset did not clear failures")
	}
}

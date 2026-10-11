package auth

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
		if !th.Attempt("k") {
			t.Fatalf("blocked on attempt %d", i+1)
		}
	}
	if th.Attempt("k") {
		t.Error("6th attempt allowed")
	}
	if !th.Attempt("other") {
		t.Error("unrelated key blocked")
	}
	now = now.Add(16 * time.Minute)
	if !th.Attempt("k") {
		t.Error("still blocked after the window passed")
	}
	th.Reset("k")
	for i := 0; i < 4; i++ {
		th.Attempt("k")
	}
	th.Release("k") // a success hands its slot back
	first, second, third := th.Attempt("k"), th.Attempt("k"), th.Attempt("k")
	if !first || !second || third {
		t.Errorf("after Release, attempts = %v %v %v; want exactly two more slots", first, second, third)
	}
}

func TestThrottleIsAtomicUnderConcurrency(t *testing.T) {
	th := NewThrottle(5, time.Minute, time.Now)
	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if th.Attempt("k") {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 5 {
		t.Errorf("%d parallel attempts allowed, want 5", granted.Load())
	}
}

func TestThrottleMemoryIsBounded(t *testing.T) {
	th := NewThrottle(5, time.Hour, time.Now)
	for i := 0; i < maxThrottleKeys*2; i++ {
		th.Attempt("flood-" + strconv.Itoa(i))
	}
	if n := len(th.attempts); n > maxThrottleKeys {
		t.Errorf("throttle holds %d keys, cap is %d", n, maxThrottleKeys)
	}
}

func TestWeakOrBrokenStoredHashesAreRejected(t *testing.T) {
	salt, key := "c29tZXNhbHRzb21lc2FsdA", "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE"
	for _, h := range []string{
		"$argon2id$v=19$m=65536,t=0,p=2$" + salt + "$" + key, // t=0 would panic inside argon2
		"$argon2id$v=19$m=8,t=1,p=1$" + salt + "$" + key,     // far below a safe cost
		"$argon2id$v=19$m=65536,t=3,p=2$$" + key,             // empty salt
		"$argon2id$v=19$m=65536,t=3,p=2$" + salt + "$MA",     // 1-byte key
	} {
		if VerifyPassword(h, "anything") {
			t.Errorf("accepted %s", h)
		}
	}
	h, _ := HashPassword("correct horse battery")
	if VerifyPassword(h, strings.Repeat("a", 5000)) {
		t.Error("over-long password verified")
	}
}

func TestPasswordLengthCountsCharacters(t *testing.T) {
	if err := ValidatePassword("🎬🎬🎬🎬"); err == nil {
		t.Error("4 emoji (16 bytes) accepted as a 10-character password")
	}
}

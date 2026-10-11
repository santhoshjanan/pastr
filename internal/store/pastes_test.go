package store

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func intp(n int) *int { return &n }

func seedPaste(t *testing.T, s *Store, slug string, mod func(*Paste)) Paste {
	t.Helper()
	ctx := context.Background()
	u, err := s.UserByUsername(ctx, "ana")
	if errors.Is(err, ErrNotFound) {
		u, err = s.CreateUser(ctx, "ana", "hash", false, t0)
	}
	if err != nil {
		t.Fatal(err)
	}
	p := Paste{Slug: slug, UserID: u.ID, Title: "Ending", BodyHTML: "<div>x</div>", RevealMode: "tap", CreatedAt: t0, UpdatedAt: t0}
	if mod != nil {
		mod(&p)
	}
	if err := s.CreatePaste(ctx, p); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPasteRoundTrip(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	exp := t0.Add(time.Hour)
	p := seedPaste(t, s, "aaaaaaaaaa", func(p *Paste) { p.ExpiresAt = &exp; p.MaxViews = intp(3) })
	if err := s.CreatePaste(ctx, p); !errors.Is(err, ErrConflict) {
		t.Errorf("slug collision: got %v, want ErrConflict", err)
	}
	got, err := s.PasteBySlug(ctx, p.Slug)
	if err != nil || got.Title != "Ending" || *got.MaxViews != 3 || !got.ExpiresAt.Equal(exp) {
		t.Errorf("PasteBySlug = %+v, %v", got, err)
	}
	got.Title, got.MaxViews, got.ExpiresAt, got.UpdatedAt = "New", nil, nil, t0.Add(time.Minute)
	if err := s.UpdatePaste(ctx, got); err != nil {
		t.Fatal(err)
	}
	got, _ = s.PasteBySlug(ctx, p.Slug)
	if got.Title != "New" || got.MaxViews != nil || got.ExpiresAt != nil {
		t.Errorf("update not applied: %+v", got)
	}
	if err := s.DeletePaste(ctx, p.Slug, p.UserID+1); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete by non-owner: got %v, want ErrNotFound", err)
	}
	if err := s.DeletePaste(ctx, p.Slug, p.UserID); err != nil {
		t.Fatal(err)
	}
}

func TestReadPasteHidesUnavailable(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	exp := t0.Add(time.Hour)
	p := seedPaste(t, s, "bbbbbbbbbb", func(p *Paste) { p.ExpiresAt = &exp })
	s.UpdateProfile(ctx, p.UserID, "Ana", "https://x.com/ana")

	v, err := s.ReadPaste(ctx, p.Slug, t0)
	if err != nil || v.AuthorName != "Ana" || v.AuthorLink != "https://x.com/ana" {
		t.Fatalf("ReadPaste = %+v, %v", v, err)
	}
	if _, err := s.ReadPaste(ctx, p.Slug, exp); !errors.Is(err, ErrNotFound) {
		t.Errorf("at expiry instant: got %v, want ErrNotFound", err)
	}
	s.SetDisabled(ctx, p.UserID, true)
	if _, err := s.ReadPaste(ctx, p.Slug, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("owner disabled: got %v, want ErrNotFound", err)
	}
	if _, err := s.ReadPaste(ctx, "zzzzzzzzzz", t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown slug: got %v, want ErrNotFound", err)
	}
}

func TestConsumeViewIsAtomic(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	p := seedPaste(t, s, "cccccccccc", func(p *Paste) { p.MaxViews = intp(5) })

	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, err := s.ConsumeView(ctx, p.Slug, t0)
			switch {
			case err == nil:
				if v.BodyHTML == "" {
					t.Error("granted view has no body")
				}
				granted.Add(1)
			case !errors.Is(err, ErrNotFound):
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 5 {
		t.Errorf("granted %d views, want exactly 5", granted.Load())
	}
	// The last view removes the row in the same transaction: nothing is left to clean up.
	if _, err := s.PasteBySlug(ctx, p.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("burned paste still present: %v", err)
	}
}

func TestConsumeViewUnlimitedAndLoweredLimit(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	p := seedPaste(t, s, "dddddddddd", nil)
	for i := 1; i <= 3; i++ {
		v, err := s.ConsumeView(ctx, p.Slug, t0)
		if err != nil || v.ViewCount != i {
			t.Fatalf("view %d: count %d, err %v", i, v.ViewCount, err)
		}
	}
	// Owner lowers the limit below the current count: the next read must be refused.
	got, _ := s.PasteBySlug(ctx, p.Slug)
	got.MaxViews = intp(2)
	s.UpdatePaste(ctx, got)
	if _, err := s.ConsumeView(ctx, p.Slug, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("view granted past a lowered limit: %v", err)
	}
	if _, err := s.ReadPaste(ctx, p.Slug, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("exhausted paste readable: %v", err)
	}
	// It can no longer be read, so the sweep removes it even though it never expires.
	if n, _ := s.DeleteExpiredPastes(ctx, t0); n != 1 {
		t.Errorf("sweep removed %d exhausted pastes, want 1", n)
	}
}

func TestConsumeViewRespectsExpiryAndDisabledOwner(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	exp := t0.Add(time.Hour)
	p := seedPaste(t, s, "gggggggggg", func(p *Paste) { p.ExpiresAt = &exp })
	if _, err := s.ConsumeView(ctx, p.Slug, exp); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired paste consumed: %v", err)
	}
	s.SetDisabled(ctx, p.UserID, true)
	if _, err := s.ConsumeView(ctx, p.Slug, t0); !errors.Is(err, ErrNotFound) {
		t.Errorf("disabled owner's paste consumed: %v", err)
	}
	if got, _ := s.PasteBySlug(ctx, p.Slug); got.ViewCount != 0 {
		t.Errorf("refused reads were counted: %d", got.ViewCount)
	}
}

func TestListAndSweep(t *testing.T) {
	s, ctx := newTestStore(t), context.Background()
	past, future := t0.Add(-time.Hour), t0.Add(time.Hour)
	live := seedPaste(t, s, "eeeeeeeeee", func(p *Paste) { p.ExpiresAt = &future })
	seedPaste(t, s, "ffffffffff", func(p *Paste) { p.ExpiresAt = &past })

	list, err := s.ListPastesByUser(ctx, live.UserID, t0)
	if err != nil || len(list) != 1 || list[0].Slug != live.Slug {
		t.Errorf("list = %+v, %v; want only the live paste", list, err)
	}
	if n, _ := s.DeleteExpiredPastes(ctx, t0); n != 1 {
		t.Errorf("swept %d pastes, want 1", n)
	}
}

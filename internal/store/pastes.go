package store

import (
	"context"
	"database/sql"
	"time"
)

type Paste struct {
	Slug       string
	UserID     int64
	Title      string
	BodyHTML   string
	RevealMode string
	ExpiresAt  *time.Time
	MaxViews   *int
	ViewCount  int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// PasteView is a paste as the public sees it, with the author's optional byline.
type PasteView struct {
	Paste
	AuthorName string
	AuthorLink string
}

const pasteCols = `p.slug, p.user_id, p.title, p.body_html, p.reveal_mode, p.expires_at, p.max_views, p.view_count, p.created_at, p.updated_at`

func scanPaste(row scanner, extra ...any) (Paste, error) {
	var p Paste
	var exp, maxViews sql.NullInt64
	var created, updated int64
	dest := append([]any{&p.Slug, &p.UserID, &p.Title, &p.BodyHTML, &p.RevealMode, &exp, &maxViews, &p.ViewCount, &created, &updated}, extra...)
	if err := row.Scan(dest...); err != nil {
		return p, mapErr(err)
	}
	if exp.Valid {
		t := time.Unix(exp.Int64, 0).UTC()
		p.ExpiresAt = &t
	}
	if maxViews.Valid {
		n := int(maxViews.Int64)
		p.MaxViews = &n
	}
	p.CreatedAt, p.UpdatedAt = time.Unix(created, 0).UTC(), time.Unix(updated, 0).UTC()
	return p, nil
}

func nullTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.Unix()
}

func nullInt(n *int) any {
	if n == nil {
		return nil
	}
	return *n
}

func (s *Store) CreatePaste(ctx context.Context, p Paste) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pastes (slug, user_id, title, body_html, reveal_mode, expires_at, max_views, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.Slug, p.UserID, p.Title, p.BodyHTML, p.RevealMode, nullTime(p.ExpiresAt), nullInt(p.MaxViews), p.CreatedAt.Unix(), p.UpdatedAt.Unix())
	return mapErr(err)
}

func (s *Store) PasteBySlug(ctx context.Context, slug string) (Paste, error) {
	return scanPaste(s.db.QueryRowContext(ctx, `SELECT `+pasteCols+` FROM pastes p WHERE p.slug = ?`, slug))
}

// ReadPaste is the public lookup. Anything a reader must not see is reported as ErrNotFound.
func (s *Store) ReadPaste(ctx context.Context, slug string, now time.Time) (PasteView, error) {
	var v PasteView
	p, err := scanPaste(s.db.QueryRowContext(ctx,
		`SELECT `+pasteCols+`, u.display_name, u.display_link
		 FROM pastes p JOIN users u ON u.id = p.user_id
		 WHERE p.slug = ? AND u.disabled = 0
		   AND (p.expires_at IS NULL OR p.expires_at > ?)
		   AND (p.max_views IS NULL OR p.view_count < p.max_views)`,
		slug, now.Unix()), &v.AuthorName, &v.AuthorLink)
	v.Paste = p
	return v, err
}

func (s *Store) ListPastesByUser(ctx context.Context, userID int64, now time.Time) ([]Paste, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+pasteCols+` FROM pastes p
		 WHERE p.user_id = ? AND (p.expires_at IS NULL OR p.expires_at > ?)
		 ORDER BY p.created_at DESC, p.slug`, userID, now.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Paste
	for rows.Next() {
		p, err := scanPaste(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UpdatePaste(ctx context.Context, p Paste) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE pastes SET title = ?, body_html = ?, reveal_mode = ?, expires_at = ?, max_views = ?, updated_at = ?
		 WHERE slug = ? AND user_id = ?`,
		p.Title, p.BodyHTML, p.RevealMode, nullTime(p.ExpiresAt), nullInt(p.MaxViews), p.UpdatedAt.Unix(), p.Slug, p.UserID)
	return affected(res, err)
}

func (s *Store) DeletePaste(ctx context.Context, slug string, userID int64) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pastes WHERE slug = ? AND user_id = ?`, slug, userID)
	return affected(res, err)
}

// ConsumeView counts one view and returns the paste, or ErrNotFound when the paste
// is expired, out of views, or owned by a disabled user. The check, the increment and
// the removal of a paste that has just used its last view happen in one transaction,
// so concurrent readers cannot overshoot the limit and a burned body is not left behind.
func (s *Store) ConsumeView(ctx context.Context, slug string, now time.Time) (PasteView, error) {
	var v PasteView
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return v, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx,
		`UPDATE pastes SET view_count = view_count + 1
		 WHERE slug = ?
		   AND (expires_at IS NULL OR expires_at > ?)
		   AND (max_views IS NULL OR view_count < max_views)
		   AND user_id IN (SELECT id FROM users WHERE disabled = 0)`, slug, now.Unix())
	if err != nil {
		return v, err
	}
	if n, err := res.RowsAffected(); err != nil {
		return v, err
	} else if n == 0 {
		return v, ErrNotFound
	}
	p, err := scanPaste(tx.QueryRowContext(ctx,
		`SELECT `+pasteCols+`, u.display_name, u.display_link
		 FROM pastes p JOIN users u ON u.id = p.user_id WHERE p.slug = ?`, slug), &v.AuthorName, &v.AuthorLink)
	if err != nil {
		return v, err
	}
	v.Paste = p
	if p.MaxViews != nil && p.ViewCount >= *p.MaxViews {
		if _, err := tx.ExecContext(ctx, `DELETE FROM pastes WHERE slug = ?`, slug); err != nil {
			return v, err
		}
	}
	return v, tx.Commit()
}

// DeleteExpiredPastes removes pastes nobody can read any more: past their expiry,
// or at their view limit (which an owner can cause by lowering the limit).
func (s *Store) DeleteExpiredPastes(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM pastes
		 WHERE (expires_at IS NOT NULL AND expires_at <= ?)
		    OR (max_views IS NOT NULL AND view_count >= max_views)`, now.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func affected(res sql.Result, err error) error {
	if err != nil {
		return mapErr(err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

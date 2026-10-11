package store

import (
	"context"
	"time"
)

type User struct {
	ID           int64
	Username     string
	PasswordHash string
	DisplayName  string
	DisplayLink  string
	IsAdmin      bool
	Disabled     bool
	CreatedAt    time.Time
}

type Session struct {
	TokenHash string
	UserID    int64
	CSRFToken string
	ExpiresAt time.Time
	CreatedAt time.Time
}

const userCols = `id, username, password_hash, display_name, display_link, is_admin, disabled, created_at`

type scanner interface{ Scan(dest ...any) error }

func scanUser(row scanner) (User, error) {
	var u User
	var created int64
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.DisplayLink, &u.IsAdmin, &u.Disabled, &created)
	u.CreatedAt = time.Unix(created, 0).UTC()
	return u, mapErr(err)
}

func (s *Store) CreateUser(ctx context.Context, username, hash string, isAdmin bool, now time.Time) (User, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO users (username, password_hash, is_admin, created_at) VALUES (?, ?, ?, ?)`,
		username, hash, isAdmin, now.Unix())
	if err != nil {
		return User{}, mapErr(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return User{}, err
	}
	return s.UserByID(ctx, id)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE username = ?`, username))
}

func (s *Store) UserByID(ctx context.Context, id int64) (User, error) {
	return scanUser(s.db.QueryRowContext(ctx, `SELECT `+userCols+` FROM users WHERE id = ?`, id))
}

func (s *Store) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+userCols+` FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

func (s *Store) CountEnabledAdmins(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE is_admin = 1 AND disabled = 0`).Scan(&n)
	return n, err
}

func (s *Store) UpdateProfile(ctx context.Context, id int64, name, link string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET display_name = ?, display_link = ? WHERE id = ?`, name, link, id)
	return err
}

func (s *Store) UpdatePassword(ctx context.Context, id int64, hash string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
	return err
}

// SetDisabled flips the flag and, when disabling, ends every session the user has.
func (s *Store) SetDisabled(ctx context.Context, id int64, disabled bool) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE users SET disabled = ? WHERE id = ?`, disabled, id); err != nil {
		return err
	}
	if disabled {
		if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) CreateSession(ctx context.Context, x Session) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, csrf_token, expires_at, created_at) VALUES (?, ?, ?, ?, ?)`,
		x.TokenHash, x.UserID, x.CSRFToken, x.ExpiresAt.Unix(), x.CreatedAt.Unix())
	return mapErr(err)
}

func (s *Store) SessionByTokenHash(ctx context.Context, hash string, now time.Time) (Session, error) {
	var x Session
	var exp, created int64
	err := s.db.QueryRowContext(ctx,
		`SELECT token_hash, user_id, csrf_token, expires_at, created_at FROM sessions WHERE token_hash = ? AND expires_at > ?`,
		hash, now.Unix()).Scan(&x.TokenHash, &x.UserID, &x.CSRFToken, &exp, &created)
	x.ExpiresAt, x.CreatedAt = time.Unix(exp, 0).UTC(), time.Unix(created, 0).UTC()
	return x, mapErr(err)
}

func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, hash)
	return err
}

func (s *Store) DeleteUserSessions(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now.Unix())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) SignupsEnabled(ctx context.Context) (bool, error) {
	var v string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = 'signups_enabled'`).Scan(&v)
	if mapErr(err) == ErrNotFound {
		return false, nil
	}
	return v == "true", err
}

func (s *Store) SetSignupsEnabled(ctx context.Context, on bool) error {
	v := "false"
	if on {
		v = "true"
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings (key, value) VALUES ('signups_enabled', ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value`, v)
	return err
}

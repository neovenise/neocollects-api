package auth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type User struct {
	ID        string
	Username  string
	CreatedAt time.Time
}

var errNotFound = errors.New("auth: not found")

type Repository struct{ db DBTX }

func NewRepository(db DBTX) *Repository { return &Repository{db: db} }

func wrap(op string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("auth: %s: %w", op, err)
}

// CreateUser adds an account.
func (r *Repository) CreateUser(ctx context.Context, username, passwordHash string) (User, error) {
	var u User
	err := r.db.QueryRow(ctx, `
		INSERT INTO users (username, password_hash) VALUES ($1, $2)
		ON CONFLICT DO NOTHING
		RETURNING id::text, username, created_at`, username, passwordHash).Scan(&u.ID, &u.Username, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, errUsernameTaken
	}
	return u, wrap("create user", err)
}

// UserByUsername returns an account and its password hash.
func (r *Repository) UserByUsername(ctx context.Context, username string) (User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRow(ctx, `SELECT id::text, username, created_at, password_hash FROM users WHERE lower(username) = lower($1)`, username).
		Scan(&u.ID, &u.Username, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", errNotFound
	}
	return u, hash, wrap("user by username", err)
}

// UserByID returns an account and its password hash.
func (r *Repository) UserByID(ctx context.Context, id string) (User, string, error) {
	var u User
	var hash string
	err := r.db.QueryRow(ctx, `SELECT id::text, username, created_at, password_hash FROM users WHERE id = $1`, id).
		Scan(&u.ID, &u.Username, &u.CreatedAt, &hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, "", errNotFound
	}
	return u, hash, wrap("user by id", err)
}

// SetPassword replaces an account's password hash.
func (r *Repository) SetPassword(ctx context.Context, userID, hash string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, userID, hash)
	return wrap("set password", err)
}

// DeleteUser removes an account and, by cascade, its data.
func (r *Repository) DeleteUser(ctx context.Context, userID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	return wrap("delete user", err)
}

// CreateSession starts a session for one device.
func (r *Repository) CreateSession(ctx context.Context, userID string) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `INSERT INTO sessions (user_id) VALUES ($1) RETURNING id::text`, userID).Scan(&id)
	return id, wrap("create session", err)
}

// AddTokens stores the hashes of a new token pair.
func (r *Repository) AddTokens(ctx context.Context, sessionID string, refreshHash []byte, refreshExpires time.Time, accessHash []byte, accessExpires time.Time) error {
	if _, err := r.db.Exec(ctx, `INSERT INTO refresh_tokens (token_hash, session_id, expires_at) VALUES ($1, $2, $3)`,
		refreshHash, sessionID, refreshExpires); err != nil {
		return wrap("add refresh token", err)
	}
	_, err := r.db.Exec(ctx, `INSERT INTO access_tokens (token_hash, session_id, expires_at) VALUES ($1, $2, $3)`,
		accessHash, sessionID, accessExpires)
	return wrap("add access token", err)
}

type RefreshState int

const (
	RefreshUsed RefreshState = iota

	RefreshUnknown

	RefreshExpired

	RefreshReplayed
)

// ConsumeRefresh marks a refresh token as used.
func (r *Repository) ConsumeRefresh(ctx context.Context, hash []byte, now time.Time) (state RefreshState, sessionID, userID string, err error) {
	err = r.db.QueryRow(ctx, `
		UPDATE refresh_tokens t SET used_at = $2 FROM sessions s
		WHERE t.token_hash = $1 AND t.used_at IS NULL AND t.expires_at > $2 AND s.id = t.session_id
		RETURNING s.id::text, s.user_id::text`, hash, now).Scan(&sessionID, &userID)
	if err == nil {
		return RefreshUsed, sessionID, userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return RefreshUnknown, "", "", wrap("consume refresh token", err)
	}

	var usedAt *time.Time
	var expiresAt time.Time
	err = r.db.QueryRow(ctx, `
		SELECT t.used_at, t.expires_at, s.id::text, s.user_id::text
		FROM refresh_tokens t JOIN sessions s ON s.id = t.session_id WHERE t.token_hash = $1`, hash).
		Scan(&usedAt, &expiresAt, &sessionID, &userID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return RefreshUnknown, "", "", nil
	case err != nil:
		return RefreshUnknown, "", "", wrap("explain refresh token", err)
	case usedAt != nil:
		return RefreshReplayed, sessionID, userID, nil
	default:
		return RefreshExpired, sessionID, userID, nil
	}
}

// SessionOfRefresh returns the session a refresh token belongs to.
func (r *Repository) SessionOfRefresh(ctx context.Context, hash []byte) (string, error) {
	var id string
	err := r.db.QueryRow(ctx, `SELECT session_id::text FROM refresh_tokens WHERE token_hash = $1`, hash).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", errNotFound
	}
	return id, wrap("session of refresh token", err)
}

type AccessInfo struct {
	UserID    string
	SessionID string
	Expired   bool
}

// AccessSession returns the session of an access token.
func (r *Repository) AccessSession(ctx context.Context, hash []byte, now time.Time) (AccessInfo, error) {
	var info AccessInfo
	var expires time.Time
	err := r.db.QueryRow(ctx, `
		SELECT s.user_id::text, s.id::text, a.expires_at
		FROM access_tokens a JOIN sessions s ON s.id = a.session_id WHERE a.token_hash = $1`, hash).
		Scan(&info.UserID, &info.SessionID, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return AccessInfo{}, errNotFound
	}
	info.Expired = !expires.After(now)
	return info, wrap("access token", err)
}

// EndSession deletes a session and its tokens.
func (r *Repository) EndSession(ctx context.Context, sessionID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE id = $1`, sessionID)
	return wrap("end session", err)
}

// EndOtherSessions signs the user out everywhere but one session.
func (r *Repository) EndOtherSessions(ctx context.Context, userID, keepSessionID string) error {
	_, err := r.db.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1 AND ($2 = '' OR id::text <> $2)`, userID, keepSessionID)
	return wrap("end other sessions", err)
}

// PurgeExpired deletes tokens that can no longer be used.
func (r *Repository) PurgeExpired(ctx context.Context, now time.Time) error {
	if _, err := r.db.Exec(ctx, `DELETE FROM access_tokens WHERE expires_at < $1`, now); err != nil {
		return wrap("purge access tokens", err)
	}
	_, err := r.db.Exec(ctx, `DELETE FROM refresh_tokens WHERE expires_at < $1 OR (used_at IS NOT NULL AND used_at < $2)`, now, now.Add(-24*time.Hour))
	return wrap("purge refresh tokens", err)
}

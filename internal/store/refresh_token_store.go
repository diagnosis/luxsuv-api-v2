package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"net"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrTokenInvalid     = errors.New("token expired or revoked")
	ErrRotateRace       = errors.New("rotate failed due to race/invalid state")
	defaultRotateWindow = 7 * 24 * time.Hour
)

type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Hash      string
	IssuedAt  time.Time
	ExpiresAt time.Time
	RevokedAt sql.NullTime
	UserAgent sql.NullString
	IP        net.IP
}

type RefreshStore interface {
	// Create generates a new random plain token, stores its hash, and returns (plain, record).
	Create(ctx context.Context, userID uuid.UUID, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, RefreshToken, error)
	// Rotate revokes the old token row and creates a new one atomically, returning (plain, record).
	Rotate(ctx context.Context, oldID uuid.UUID, userID uuid.UUID, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, RefreshToken, error)

	FindByHash(ctx context.Context, hash string) (*RefreshToken, error)
	Revoke(ctx context.Context, id uuid.UUID, when time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	RevokeByHash(ctx context.Context, hash string) error
	DeleteExpired(ctx context.Context) (int64, error)
}
type PostgresRefreshTokenStore struct {
	pool *pgxpool.Pool
}

func NewPostgresRefreshTokenStore(pool *pgxpool.Pool) *PostgresRefreshTokenStore {
	return &PostgresRefreshTokenStore{pool: pool}
}

//Helpers

func generatePlain(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
func hashHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func ipToNullable(ip net.IP) any {
	if ip == nil {
		return nil
	}
	return ip.String()
}

func toNullString(s string) sql.NullString {
	if strings.TrimSpace(s) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func (s *PostgresRefreshTokenStore) insert(ctx context.Context, t *RefreshToken) error {
	const q = `
		INSERT INTO auth_refresh_tokens
			(user_id, token_hash, issued_at, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6::inet)
		RETURNING id, issued_at, ip;
	`
	var ipStr sql.NullString
	if err := s.pool.QueryRow(ctx, q,
		t.UserID, t.Hash, t.IssuedAt.UTC(), t.ExpiresAt.UTC(), t.UserAgent, ipToNullable(t.IP),
	).Scan(&t.ID, &t.IssuedAt, &ipStr); err != nil {
		return err
	}
	if ipStr.Valid {
		t.IP = net.ParseIP(ipStr.String)
	}
	return nil
}

func (s *PostgresRefreshTokenStore) Create(ctx context.Context, userID uuid.UUID, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, RefreshToken, error) {
	if ttl <= 0 {
		ttl = defaultRotateWindow
	}
	plain, err := generatePlain(32)
	if err != nil {
		return "", RefreshToken{}, err
	}
	rec := RefreshToken{
		UserID:    userID,
		Hash:      hashHex(plain),
		IssuedAt:  now.UTC(),
		ExpiresAt: now.UTC().Add(ttl),
		UserAgent: toNullString(ua),
		IP:        ip,
	}
	if err := s.insert(ctx, &rec); err != nil {
		return "", RefreshToken{}, err
	}
	return plain, rec, nil
}

func (s *PostgresRefreshTokenStore) Rotate(ctx context.Context, oldID uuid.UUID, userID uuid.UUID, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, RefreshToken, error) {
	if ttl <= 0 {
		ttl = defaultRotateWindow
	}

	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return "", RefreshToken{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Lock old token row and verify it's still valid
	var expires time.Time
	var revoked sql.NullTime
	if err := tx.QueryRow(ctx, `
		SELECT expires_at, revoked_at
		FROM auth_refresh_tokens
		WHERE id = $1 AND user_id = $2
		FOR UPDATE
	`, oldID, userID).Scan(&expires, &revoked); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", RefreshToken{}, ErrNotFound
		}
		return "", RefreshToken{}, err
	}

	nowUTC := now.UTC()
	if nowUTC.After(expires) || (revoked.Valid && !revoked.Time.IsZero()) {
		return "", RefreshToken{}, ErrTokenInvalid
	}

	// Revoke old
	tag, err := tx.Exec(ctx, `
  UPDATE auth_refresh_tokens
  SET revoked_at = $2
  WHERE id = $1 AND revoked_at IS NULL
`, oldID, nowUTC)
	if err != nil {
		return "", RefreshToken{}, err
	}
	if tag.RowsAffected() == 0 {
		// Shouldn’t normally happen because you hold the row lock and
		// just validated revoked_at is NULL, but if it does, treat as a race/invalid state.
		return "", RefreshToken{}, ErrRotateRace
	}

	// Create new
	plain, err := generatePlain(32)
	if err != nil {
		return "", RefreshToken{}, err
	}
	newHash := hashHex(plain)
	newExpires := nowUTC.Add(ttl)

	var rec RefreshToken
	var ipStr sql.NullString
	if err := tx.QueryRow(ctx, `
		INSERT INTO auth_refresh_tokens (user_id, token_hash, issued_at, expires_at, user_agent, ip)
		VALUES ($1, $2, $3, $4, $5, $6::inet)
		RETURNING id, user_id, token_hash, issued_at, expires_at, revoked_at, user_agent, ip
	`, userID, newHash, nowUTC, newExpires, toNullString(ua), ipToNullable(ip)).
		Scan(&rec.ID, &rec.UserID, &rec.Hash, &rec.IssuedAt, &rec.ExpiresAt, &rec.RevokedAt, &rec.UserAgent, &ipStr); err != nil {
		return "", RefreshToken{}, err
	}
	if ipStr.Valid {
		rec.IP = net.ParseIP(ipStr.String)
	}

	if err := tx.Commit(ctx); err != nil {
		return "", RefreshToken{}, err
	}
	return plain, rec, nil
}

func (s *PostgresRefreshTokenStore) FindByHash(ctx context.Context, hash string) (*RefreshToken, error) {
	const q = `
		SELECT id, user_id, token_hash, issued_at, expires_at, revoked_at, user_agent, ip
		FROM auth_refresh_tokens
		WHERE token_hash = $1
		LIMIT 1;
	`
	var t RefreshToken
	var ipStr sql.NullString
	if err := s.pool.QueryRow(ctx, q, hash).Scan(
		&t.ID, &t.UserID, &t.Hash, &t.IssuedAt, &t.ExpiresAt, &t.RevokedAt, &t.UserAgent, &ipStr,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if ipStr.Valid {
		t.IP = net.ParseIP(ipStr.String)
	}
	return &t, nil
}

func (s *PostgresRefreshTokenStore) Revoke(ctx context.Context, id uuid.UUID, when time.Time) error {
	ct, err := s.pool.Exec(ctx, `UPDATE auth_refresh_tokens SET revoked_at = $2 WHERE id = $1 AND revoked_at IS NULL`, id, when.UTC())
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresRefreshTokenStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE auth_refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

func (s *PostgresRefreshTokenStore) RevokeByHash(ctx context.Context, hash string) error {
	ct, err := s.pool.Exec(ctx, `UPDATE auth_refresh_tokens SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
func (s *PostgresRefreshTokenStore) DeleteExpired(ctx context.Context) (int64, error) {
	ct, err := s.pool.Exec(ctx, `DELETE FROM auth_refresh_tokens WHERE expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return ct.RowsAffected(), nil
}

var _ RefreshStore = (*PostgresRefreshTokenStore)(nil)

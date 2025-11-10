package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AVPurpose string

const (
	PurposeRiderConfirm  AVPurpose = "rider_confirm"
	PurposeDriverConfirm AVPurpose = "driver_confirm"
	PurposePasswordReset AVPurpose = "password_reset"
)

var (
	ErrInvalidToken = errors.New("invalid, used, or expired token")
)

type AuthVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Purpose   AVPurpose
	TokenHash string
	IssuedAt  time.Time
	ExpiresAt time.Time
	UsedAt    sql.NullTime
	UserAgent sql.NullString
	IP        net.IP
}

type AuthVerificationStore interface {
	// Create generates a plain token, stores its hash, returns (plain, record).
	Create(ctx context.Context, userID uuid.UUID, purpose AVPurpose, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, AuthVerificationToken, error)

	// Read helpers
	FindByHash(ctx context.Context, hash string) (*AuthVerificationToken, error)

	// Mutations
	ValidateAndConsume(ctx context.Context, hash string, purpose AVPurpose, now time.Time) (*AuthVerificationToken, error)
	Revoke(ctx context.Context, id uuid.UUID, when time.Time) error
	RevokeAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}

type PostgresAuthVerificationStore struct {
	Pool *pgxpool.Pool
}

func NewAuthVerificationStore(pool *pgxpool.Pool) *PostgresAuthVerificationStore {
	return &PostgresAuthVerificationStore{pool}
}

// --- internal insert ---

func (p *PostgresAuthVerificationStore) insert(ctx context.Context, at *AuthVerificationToken) error {
	if at.UserID == uuid.Nil {
		return fmt.Errorf("user_id required")
	}
	if at.Purpose == "" {
		return fmt.Errorf("purpose required")
	}
	if at.TokenHash == "" {
		return fmt.Errorf("token_hash required")
	}
	if at.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at required")
	}

	const q = `
		INSERT INTO auth_verification_tokens
		  (user_id, purpose, token_hash, issued_at, expires_at, user_agent, ip)
		VALUES
		  ($1, $2, $3, $4, $5, $6, $7::inet)
		RETURNING id
	`

	var ua any
	if at.UserAgent.Valid {
		ua = at.UserAgent.String
	}
	var ip any
	if at.IP != nil {
		ip = at.IP.String()
	}

	return p.Pool.
		QueryRow(ctx, q,
			at.UserID, string(at.Purpose), at.TokenHash, at.IssuedAt.UTC(), at.ExpiresAt.UTC(), ua, ip,
		).
		Scan(&at.ID)
}

// --- Create ---

var defaultAuthTokenExpiry = 24 * time.Hour

func (p *PostgresAuthVerificationStore) Create(ctx context.Context, userID uuid.UUID, purpose AVPurpose, ua string, ip net.IP, ttl time.Duration, now time.Time) (string, AuthVerificationToken, error) {
	if ttl <= 0 {
		ttl = defaultAuthTokenExpiry
	}
	plain, err := generatePlain(32)
	if err != nil {
		return "", AuthVerificationToken{}, err
	}
	rec := AuthVerificationToken{
		UserID:    userID,
		Purpose:   purpose,
		TokenHash: hashHex(plain),
		IssuedAt:  now.UTC(),
		ExpiresAt: now.UTC().Add(ttl),
		UserAgent: toNullString(ua),
		IP:        ip,
	}
	if err := p.insert(ctx, &rec); err != nil {
		return "", AuthVerificationToken{}, fmt.Errorf("insert verification token: %w", err)
	}
	return plain, rec, nil
}

// --- FindByHash ---

func (p *PostgresAuthVerificationStore) FindByHash(ctx context.Context, hash string) (*AuthVerificationToken, error) {
	const q = `
		SELECT id, user_id, purpose, token_hash, issued_at, expires_at, used_at, user_agent, ip
		FROM auth_verification_tokens
		WHERE token_hash = $1
		LIMIT 1
	`
	var at AuthVerificationToken
	var ipStr sql.NullString

	if err := p.Pool.QueryRow(ctx, q, hash).Scan(
		&at.ID, &at.UserID, &at.Purpose, &at.TokenHash,
		&at.IssuedAt, &at.ExpiresAt, &at.UsedAt, &at.UserAgent, &ipStr,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if ipStr.Valid && ipStr.String != "" {
		at.IP = net.ParseIP(ipStr.String)
	}
	return &at, nil
}

// --- ValidateAndConsume (atomic, prevents replay) ---

func (p *PostgresAuthVerificationStore) ValidateAndConsume(ctx context.Context, raw string, purpose AVPurpose, now time.Time) (*AuthVerificationToken, error) {
	hash := hashHex(raw)
	const q = `
		WITH consumed AS (
		  UPDATE auth_verification_tokens
		     SET used_at = $3
		   WHERE token_hash = $1
		     AND purpose    = $2
		     AND used_at    IS NULL
		     AND expires_at > $3
		RETURNING id, user_id, purpose, token_hash, issued_at, expires_at, used_at, user_agent, ip
		)
		SELECT id, user_id, purpose, token_hash, issued_at, expires_at, used_at, user_agent, ip
		  FROM consumed
	`

	var at AuthVerificationToken
	var ipStr sql.NullString

	if err := p.Pool.QueryRow(ctx, q, hash, string(purpose), now.UTC()).Scan(
		&at.ID, &at.UserID, &at.Purpose, &at.TokenHash,
		&at.IssuedAt, &at.ExpiresAt, &at.UsedAt, &at.UserAgent, &ipStr,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInvalidToken // wrong hash, wrong purpose, already used, or expired
		}
		return nil, err
	}
	if ipStr.Valid && ipStr.String != "" {
		at.IP = net.ParseIP(ipStr.String)
	}
	return &at, nil
}

// --- Revoke / RevokeAll / DeleteExpired ---

func (p *PostgresAuthVerificationStore) Revoke(ctx context.Context, id uuid.UUID, when time.Time) error {
	tag, err := p.Pool.Exec(ctx,
		`UPDATE auth_verification_tokens SET used_at = $2 WHERE id = $1 AND used_at IS NULL`,
		id, when.UTC(),
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresAuthVerificationStore) RevokeAllForUser(ctx context.Context, userID uuid.UUID) error {
	_, err := p.Pool.Exec(ctx,
		`UPDATE auth_verification_tokens SET used_at = now() WHERE user_id = $1 AND used_at IS NULL`,
		userID,
	)
	return err
}

func (p *PostgresAuthVerificationStore) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	tag, err := p.Pool.Exec(ctx,
		`DELETE FROM auth_verification_tokens WHERE expires_at < $1`,
		now.UTC(),
	)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

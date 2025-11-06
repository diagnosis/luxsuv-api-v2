package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	Role         *string
	IsActive     bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type UserStore interface {
	CreateUser(ctx context.Context, u *User) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, newHash string) error
	ActivateUser(ctx context.Context, id uuid.UUID) error
	DeactivateUser(ctx context.Context, id uuid.UUID) error
}

type PostgresUserStore struct {
	pool *pgxpool.Pool
}

func NewPostgresUserStore(pool *pgxpool.Pool) *PostgresUserStore {
	return &PostgresUserStore{pool: pool}
}

var (
	ErrDuplicateEmail = errors.New("email already exists")
	ErrNotFound       = errors.New("not found")
)

func normalizeEmail(e string) string { return strings.ToLower(strings.TrimSpace(e)) }

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// CreateUser inserts user; default role -> 'rider' if nil or "".
func (p *PostgresUserStore) CreateUser(ctx context.Context, u *User) (*User, error) {
	const q = `
INSERT INTO users (email, password_hash, role, is_active, created_at, updated_at)
VALUES (lower(btrim($1)), $2, COALESCE(NULLIF($3, ''), 'rider'), false, now(), now())
RETURNING id, email, password_hash, role, is_active, created_at, updated_at;
`
	var out User
	var roleStr string
	// u.Role may be nil; pass nil or *u.Role safely:
	var roleArg any
	if u.Role == nil {
		roleArg = nil
	} else {
		roleArg = *u.Role
	}

	if err := p.pool.QueryRow(ctx, q, normalizeEmail(u.Email), u.PasswordHash, roleArg).Scan(
		&out.ID, &out.Email, &out.PasswordHash, &roleStr,
		&out.IsActive, &out.CreatedAt, &out.UpdatedAt,
	); err != nil {
		if isUniqueViolation(err) {
			return nil, ErrDuplicateEmail
		}
		return nil, err
	}
	out.Role = &roleStr
	return &out, nil
}

func (p *PostgresUserStore) GetByEmail(ctx context.Context, email string) (*User, error) {
	const q = `
SELECT id, email, password_hash, role, is_active, created_at, updated_at
FROM users
WHERE lower(btrim(email)) = lower(btrim($1))
LIMIT 1;
`
	var u User
	var roleStr string
	if err := p.pool.QueryRow(ctx, q, normalizeEmail(email)).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &roleStr, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Role = &roleStr
	return &u, nil
}

func (p *PostgresUserStore) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	const q = `
SELECT id, email, password_hash, role, is_active, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;
`
	var u User
	var roleStr string
	if err := p.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &roleStr, &u.IsActive, &u.CreatedAt, &u.UpdatedAt,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	u.Role = &roleStr
	return &u, nil
}

func (p *PostgresUserStore) UpdatePassword(ctx context.Context, id uuid.UUID, newHash string) error {
	const q = `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1;`
	tag, err := p.pool.Exec(ctx, q, id, newHash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresUserStore) ActivateUser(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET is_active = true, updated_at = now() WHERE id = $1;`
	tag, err := p.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresUserStore) DeactivateUser(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE users SET is_active = false, updated_at = now() WHERE id = $1;`
	tag, err := p.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

var _ UserStore = (*PostgresUserStore)(nil)

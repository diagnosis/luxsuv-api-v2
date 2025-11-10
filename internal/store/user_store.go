package store

import (
	"context"
	"errors"
	"fmt"
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
	IsVerified   bool
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
	SetVerified(ctx context.Context, id uuid.UUID, verified bool) error
	VerifyEmailExists(ctx context.Context, email string) (bool, error)
	ListByStatus(ctx context.Context, verified, active *bool, role *string, limit, offset int) ([]User, error)
	CreateUserWithRole(ctx context.Context, email, passwordHash string, role string) (uuid.UUID, error)
	SetUserRole(ctx context.Context, userID uuid.UUID, role string) error
	SetUserFlags(ctx context.Context, userID uuid.UUID, verified, active *bool) error
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

// CreateUser inserts a new user.
// - role: if nil/empty => defaults to 'rider' (enum cast is done in SQL)
// - is_verified/is_active: both start false; flip in flows as needed
func (p *PostgresUserStore) CreateUser(ctx context.Context, u *User) (*User, error) {
	if u.Role != nil {
		switch strings.ToLower(strings.TrimSpace(*u.Role)) {
		case "rider", "driver":
			// ok
		case "":
			u.Role = nil
		default:
			return nil, fmt.Errorf("invalid role for this endpoint")
		}
	}
	const q = `
INSERT INTO users (
  email,
  password_hash,
  role,
  is_verified,
  is_active,
  created_at,
  updated_at
)
VALUES (
  lower(btrim($1)),
  $2,
  COALESCE(NULLIF($3, '')::user_role, 'rider'::user_role),
  false,
  false,
  now(),
  now()
)
RETURNING id, email, password_hash, role, is_verified, is_active, created_at, updated_at;
`

	var roleArg any
	if u.Role == nil {
		roleArg = nil // NULL -> COALESCE to 'rider'
	} else {
		roleArg = *u.Role
	}

	var out User
	var roleStr string
	if err := p.pool.QueryRow(ctx, q, normalizeEmail(u.Email), u.PasswordHash, roleArg).Scan(
		&out.ID,
		&out.Email,
		&out.PasswordHash,
		&roleStr,
		&out.IsVerified,
		&out.IsActive,
		&out.CreatedAt,
		&out.UpdatedAt,
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
SELECT id, email, password_hash, role, is_verified, is_active, created_at, updated_at
FROM users
WHERE lower(btrim(email)) = lower(btrim($1))
LIMIT 1;
`
	var u User
	var roleStr string
	if err := p.pool.QueryRow(ctx, q, normalizeEmail(email)).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&roleStr,
		&u.IsVerified,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
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
SELECT id, email, password_hash, role, is_verified, is_active, created_at, updated_at
FROM users
WHERE id = $1
LIMIT 1;
`
	var u User
	var roleStr string
	if err := p.pool.QueryRow(ctx, q, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&roleStr,
		&u.IsVerified,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
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
	const q = `
UPDATE users
SET password_hash = $2, updated_at = now()
WHERE id = $1;
`
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
	const q = `
UPDATE users
SET is_active = true, updated_at = now()
WHERE id = $1;
`
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
	const q = `
UPDATE users
SET is_active = false, updated_at = now()
WHERE id = $1;
`
	tag, err := p.pool.Exec(ctx, q, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresUserStore) SetVerified(ctx context.Context, id uuid.UUID, verified bool) error {
	const q = `
UPDATE users
SET is_verified = $2, updated_at = now()
WHERE id = $1;
`
	tag, err := p.pool.Exec(ctx, q, id, verified)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresUserStore) VerifyEmailExists(ctx context.Context, email string) (bool, error) {
	const q = `
SELECT EXISTS(
  SELECT 1 FROM users WHERE lower(btrim(email)) = lower(btrim($1))
);
`
	var exists bool
	if err := p.pool.QueryRow(ctx, q, normalizeEmail(email)).Scan(&exists); err != nil {
		return false, err
	}
	return exists, nil
}

// (Optional)
func (p *PostgresUserStore) ListByStatus(ctx context.Context, verified, active *bool, role *string, limit, offset int) ([]User, error) {
	q := `
SELECT id, email, password_hash, role, is_verified, is_active, created_at, updated_at
FROM users
WHERE ($1::bool IS NULL OR is_verified = $1)
  AND ($2::bool IS NULL OR is_active   = $2)
  AND ($3::user_role IS NULL OR role   = $3::user_role)
ORDER BY created_at DESC
LIMIT $4 OFFSET $5;
`
	var roleCast *string
	if role != nil && strings.TrimSpace(*role) != "" {
		r := strings.ToLower(strings.TrimSpace(*role))
		roleCast = &r
	}
	rows, err := p.pool.Query(ctx, q, verified, active, roleCast, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []User
	for rows.Next() {
		var u User
		var roleStr string
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &roleStr, &u.IsVerified, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.Role = &roleStr
		out = append(out, u)
	}
	return out, rows.Err()
}

//super-admin-methods

func (p *PostgresUserStore) CreateUserWithRole(ctx context.Context, email, passwordHash string, role string) (uuid.UUID, error) {
	const q = `SELECT create_user_with_role($1,$2,$3::user_role);`
	var id uuid.UUID
	if err := p.pool.QueryRow(ctx, q, normalizeEmail(email), passwordHash, role).Scan(&id); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (p *PostgresUserStore) SetUserRole(ctx context.Context, userID uuid.UUID, role string) error {
	const q = `SELECT set_user_role($1,$2::user_role);`
	_, err := p.pool.Exec(ctx, q, userID, role)
	return err
}

func (p *PostgresUserStore) SetUserFlags(ctx context.Context, userID uuid.UUID, verified, active *bool) error {
	const q = `SELECT set_user_flags($1,$2,$3);`
	// pass nils through with *bool
	var v, a any
	if verified != nil {
		v = *verified
	} else {
		v = nil
	}
	if active != nil {
		a = *active
	} else {
		a = nil
	}
	_, err := p.pool.Exec(ctx, q, userID, v, a)
	return err
}

var _ UserStore = (*PostgresUserStore)(nil)

package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type DriverAppStatus string

const (
	DriverPending  DriverAppStatus = "pending"
	DriverApproved DriverAppStatus = "approved"
	DriverRejected DriverAppStatus = "rejected"
)

type DriverApplication struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Status    DriverAppStatus
	Notes     sql.NullString
	CreatedAt time.Time
	UpdatedAt time.Time
}

type DriverApplicationStore interface {
	// Create a new application (user_id is UNIQUE; one application per user).
	Create(ctx context.Context, userID uuid.UUID, notes string, now time.Time) (*DriverApplication, error)

	// Reads
	GetByID(ctx context.Context, id uuid.UUID) (*DriverApplication, error)
	GetByUserID(ctx context.Context, userID uuid.UUID) (*DriverApplication, error)

	// Lists (admin)
	List(ctx context.Context, status *DriverAppStatus, limit, offset int) ([]DriverApplication, error)
	Count(ctx context.Context, status *DriverAppStatus) (int64, error)

	// Mutations
	UpdateNotes(ctx context.Context, id uuid.UUID, notes string) error
	UpdateStatus(ctx context.Context, id uuid.UUID, newStatus DriverAppStatus) error

	// Delete (rare, but handy for cleanup/tests)
	Delete(ctx context.Context, id uuid.UUID) error
}

type PostgresDriverApplicationStore struct {
	pool *pgxpool.Pool
}

func NewDriverApplicationStore(pool *pgxpool.Pool) *PostgresDriverApplicationStore {
	return &PostgresDriverApplicationStore{pool: pool}
}

// -------- Create --------

func (s *PostgresDriverApplicationStore) Create(ctx context.Context, userID uuid.UUID, notes string, now time.Time) (*DriverApplication, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("user_id required")
	}
	const q = `
		INSERT INTO driver_applications (user_id, status, notes, created_at, updated_at)
		VALUES ($1, 'pending', NULLIF($2, ''), $3, $3)
		RETURNING id, user_id, status, notes, created_at, updated_at
	`
	var da DriverApplication
	if err := s.pool.QueryRow(ctx, q, userID, notes, now.UTC()).
		Scan(&da.ID, &da.UserID, &da.Status, &da.Notes, &da.CreatedAt, &da.UpdatedAt); err != nil {
		return nil, err
	}
	return &da, nil
}

// -------- Reads --------

func (s *PostgresDriverApplicationStore) GetByID(ctx context.Context, id uuid.UUID) (*DriverApplication, error) {
	const q = `
		SELECT id, user_id, status, notes, created_at, updated_at
		FROM driver_applications
		WHERE id = $1
	`
	var da DriverApplication
	if err := s.pool.QueryRow(ctx, q, id).
		Scan(&da.ID, &da.UserID, &da.Status, &da.Notes, &da.CreatedAt, &da.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &da, nil
}

func (s *PostgresDriverApplicationStore) GetByUserID(ctx context.Context, userID uuid.UUID) (*DriverApplication, error) {
	const q = `
		SELECT id, user_id, status, notes, created_at, updated_at
		FROM driver_applications
		WHERE user_id = $1
	`
	var da DriverApplication
	if err := s.pool.QueryRow(ctx, q, userID).
		Scan(&da.ID, &da.UserID, &da.Status, &da.Notes, &da.CreatedAt, &da.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &da, nil
}

// -------- Lists --------

func (s *PostgresDriverApplicationStore) List(ctx context.Context, status *DriverAppStatus, limit, offset int) ([]DriverApplication, error) {
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	base := `
		SELECT id, user_id, status, notes, created_at, updated_at
		FROM driver_applications
	`
	var rows pgx.Rows
	var err error

	if status != nil {
		q := base + ` WHERE status = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err = s.pool.Query(ctx, q, string(*status), limit, offset)
	} else {
		q := base + ` ORDER BY created_at DESC LIMIT $1 OFFSET $2`
		rows, err = s.pool.Query(ctx, q, limit, offset)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DriverApplication
	for rows.Next() {
		var da DriverApplication
		if err := rows.Scan(&da.ID, &da.UserID, &da.Status, &da.Notes, &da.CreatedAt, &da.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, da)
	}
	return out, rows.Err()
}

func (s *PostgresDriverApplicationStore) Count(ctx context.Context, status *DriverAppStatus) (int64, error) {
	const base = `SELECT COUNT(*) FROM driver_applications`
	var n int64
	var err error
	if status != nil {
		err = s.pool.QueryRow(ctx, base+` WHERE status = $1`, string(*status)).Scan(&n)
	} else {
		err = s.pool.QueryRow(ctx, base).Scan(&n)
	}
	return n, err
}

// -------- Mutations --------

func (s *PostgresDriverApplicationStore) UpdateNotes(ctx context.Context, id uuid.UUID, notes string) error {
	const q = `
		UPDATE driver_applications
		   SET notes = NULLIF($2,''), updated_at = now()
		 WHERE id = $1
	`
	tag, err := s.pool.Exec(ctx, q, id, notes)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresDriverApplicationStore) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus DriverAppStatus) error {
	switch newStatus {
	case DriverPending, DriverApproved, DriverRejected:
	default:
		return fmt.Errorf("invalid status: %s", newStatus)
	}

	const q = `
		UPDATE driver_applications
		   SET status = $2, updated_at = now()
		 WHERE id = $1
	`
	tag, err := s.pool.Exec(ctx, q, id, string(newStatus))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// -------- Delete --------

func (s *PostgresDriverApplicationStore) Delete(ctx context.Context, id uuid.UUID) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM driver_applications WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

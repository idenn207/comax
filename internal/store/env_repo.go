package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// EnvRepo persists Environments scoped to a Project.
type EnvRepo struct{ db DBTX }

// NewEnvRepo wraps db.
func NewEnvRepo(db DBTX) *EnvRepo { return &EnvRepo{db: db} }

// Create inserts a new environment under projectID. inheritsFrom is the
// name of a sibling env or "" for none. Returns ErrConflict if the same
// (project, name) pair already exists.
func (r *EnvRepo) Create(ctx context.Context, projectID int64, name, inheritsFrom string) (Environment, error) {
	now := nowUnix()
	var inheritsArg any
	if inheritsFrom != "" {
		inheritsArg = inheritsFrom
	}
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO environments (project_id, name, inherits_from, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		projectID, name, inheritsArg, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Environment{}, fmt.Errorf("create env %q in project %d: %w", name, projectID, ErrConflict)
		}
		return Environment{}, fmt.Errorf("create env %q in project %d: %w", name, projectID, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Environment{}, fmt.Errorf("create env %q in project %d: %w", name, projectID, err)
	}
	return Environment{
		ID:           id,
		ProjectID:    projectID,
		Name:         name,
		InheritsFrom: inheritsFrom,
		CreatedAt:    unixSeconds(now),
		UpdatedAt:    unixSeconds(now),
	}, nil
}

// ByID returns the env with the given primary key, or ErrNotFound.
// Used by the reference resolver which walks the inherits_from chain
// and only has IDs in hand once it has descended one level.
func (r *EnvRepo) ByID(ctx context.Context, id int64) (Environment, error) {
	var (
		e                    Environment
		inherits             sql.NullString
		createdAt, updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, inherits_from, created_at, updated_at
		   FROM environments
		  WHERE id = ?`,
		id,
	).Scan(&e.ID, &e.ProjectID, &e.Name, &inherits, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Environment{}, fmt.Errorf("env id=%d: %w", id, ErrNotFound)
	}
	if err != nil {
		return Environment{}, fmt.Errorf("lookup env id=%d: %w", id, err)
	}
	e.InheritsFrom = inherits.String
	e.CreatedAt = unixSeconds(createdAt)
	e.UpdatedAt = unixSeconds(updatedAt)
	return e, nil
}

// ByName returns the env named name within projectID, or ErrNotFound.
func (r *EnvRepo) ByName(ctx context.Context, projectID int64, name string) (Environment, error) {
	var (
		e                    Environment
		inherits             sql.NullString
		createdAt, updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, project_id, name, inherits_from, created_at, updated_at
		   FROM environments
		  WHERE project_id = ? AND name = ?`,
		projectID, name,
	).Scan(&e.ID, &e.ProjectID, &e.Name, &inherits, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Environment{}, fmt.Errorf("env %q in project %d: %w", name, projectID, ErrNotFound)
	}
	if err != nil {
		return Environment{}, fmt.Errorf("lookup env %q in project %d: %w", name, projectID, err)
	}
	e.InheritsFrom = inherits.String
	e.CreatedAt = unixSeconds(createdAt)
	e.UpdatedAt = unixSeconds(updatedAt)
	return e, nil
}

// ListByProject returns every environment in projectID, ordered by name.
func (r *EnvRepo) ListByProject(ctx context.Context, projectID int64) ([]Environment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, project_id, name, inherits_from, created_at, updated_at
		   FROM environments
		  WHERE project_id = ?
		  ORDER BY name`,
		projectID,
	)
	if err != nil {
		return nil, fmt.Errorf("list envs for project %d: %w", projectID, err)
	}
	defer rows.Close()

	var out []Environment
	for rows.Next() {
		var (
			e                    Environment
			inherits             sql.NullString
			createdAt, updatedAt int64
		)
		if err := rows.Scan(&e.ID, &e.ProjectID, &e.Name, &inherits, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan env: %w", err)
		}
		e.InheritsFrom = inherits.String
		e.CreatedAt = unixSeconds(createdAt)
		e.UpdatedAt = unixSeconds(updatedAt)
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate envs: %w", err)
	}
	return out, nil
}

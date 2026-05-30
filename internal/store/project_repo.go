package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ProjectRepo persists Projects.
type ProjectRepo struct{ db DBTX }

// NewProjectRepo wraps db. Pass a *sql.DB for autocommit semantics or a
// *sql.Tx to join the caller's transaction.
func NewProjectRepo(db DBTX) *ProjectRepo { return &ProjectRepo{db: db} }

// Create inserts a project with the given name and returns it. Returns
// ErrConflict if name already exists.
func (r *ProjectRepo) Create(ctx context.Context, name string) (Project, error) {
	now := nowUnix()
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO projects (name, created_at, updated_at) VALUES (?, ?, ?)`,
		name, now, now,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return Project{}, fmt.Errorf("create project %q: %w", name, ErrConflict)
		}
		return Project{}, fmt.Errorf("create project %q: %w", name, err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Project{}, fmt.Errorf("create project %q: %w", name, err)
	}
	return Project{
		ID:        id,
		Name:      name,
		CreatedAt: unixSeconds(now),
		UpdatedAt: unixSeconds(now),
	}, nil
}

// ByName returns the project named name, or ErrNotFound.
func (r *ProjectRepo) ByName(ctx context.Context, name string) (Project, error) {
	var (
		p                    Project
		createdAt, updatedAt int64
	)
	err := r.db.QueryRowContext(ctx,
		`SELECT id, name, created_at, updated_at FROM projects WHERE name = ?`,
		name,
	).Scan(&p.ID, &p.Name, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, fmt.Errorf("project %q: %w", name, ErrNotFound)
	}
	if err != nil {
		return Project{}, fmt.Errorf("lookup project %q: %w", name, err)
	}
	p.CreatedAt = unixSeconds(createdAt)
	p.UpdatedAt = unixSeconds(updatedAt)
	return p, nil
}

// List returns every project, ordered by name for stable output.
func (r *ProjectRepo) List(ctx context.Context) ([]Project, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, created_at, updated_at FROM projects ORDER BY name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}
	defer rows.Close()

	var out []Project
	for rows.Next() {
		var (
			p                    Project
			createdAt, updatedAt int64
		)
		if err := rows.Scan(&p.ID, &p.Name, &createdAt, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		p.CreatedAt = unixSeconds(createdAt)
		p.UpdatedAt = unixSeconds(updatedAt)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return out, nil
}

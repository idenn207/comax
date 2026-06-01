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

// ProjectWithEnvCount is the list-only projection that pairs a project
// with the number of environments under it. Embeds Project so callers
// can still reach .Name / .CreatedAt directly without a second hop.
type ProjectWithEnvCount struct {
	Project
	EnvCount int64
}

// ListWithEnvCounts returns every project plus its env count in one
// round-trip. LEFT JOIN (not INNER) so a project with zero environments
// still surfaces — the dashboard treats "0 configs" as a real state, not
// a row to hide.
func (r *ProjectRepo) ListWithEnvCounts(ctx context.Context) ([]ProjectWithEnvCount, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT p.id, p.name, p.created_at, p.updated_at, COUNT(e.id)
		 FROM projects p
		 LEFT JOIN environments e ON e.project_id = p.id
		 GROUP BY p.id
		 ORDER BY p.name`,
	)
	if err != nil {
		return nil, fmt.Errorf("list projects with env counts: %w", err)
	}
	defer rows.Close()

	var out []ProjectWithEnvCount
	for rows.Next() {
		var (
			entry                ProjectWithEnvCount
			createdAt, updatedAt int64
		)
		if err := rows.Scan(&entry.ID, &entry.Name, &createdAt, &updatedAt, &entry.EnvCount); err != nil {
			return nil, fmt.Errorf("scan project: %w", err)
		}
		entry.CreatedAt = unixSeconds(createdAt)
		entry.UpdatedAt = unixSeconds(updatedAt)
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate projects: %w", err)
	}
	return out, nil
}

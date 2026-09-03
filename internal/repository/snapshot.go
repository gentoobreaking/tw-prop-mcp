package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"tw-prop-mcp/internal/domain"
	"tw-prop-mcp/internal/repository/db"
)

// SnapshotRepository defines persistence operations for dataset_snapshot.
type SnapshotRepository interface {
	Create(ctx context.Context, arg CreateSnapshotParams) (domain.DatasetSnapshot, error)
	GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error)
	List(ctx context.Context) ([]domain.DatasetSnapshot, error)
	UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error
	Lock(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// CreateSnapshotParams mirrors required fields for creation.
type CreateSnapshotParams struct {
	Source        string
	SourceVersion string
	FileName      string
	FileSHA256    string
	RecordCount   int64
	Status        domain.SnapshotStatus
	SchemaVersion string
}

// DBTX is the minimal interface for pgx operations (matches db.DBTX).
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
}

type snapshotRepository struct {
	queries *db.Queries
	db      DBTX
}

// NewSnapshotRepository creates a repository backed by pgx + sqlc.
func NewSnapshotRepository(dbt DBTX) SnapshotRepository {
	return &snapshotRepository{
		queries: db.New(dbt),
		db:      dbt,
	}
}

// Create inserts a new snapshot row.
func (r *snapshotRepository) Create(ctx context.Context, arg CreateSnapshotParams) (domain.DatasetSnapshot, error) {
	if !domain.IsValidStatus(arg.Status) {
		return domain.DatasetSnapshot{}, fmt.Errorf("invalid status: %s", arg.Status)
	}
	row, err := r.queries.CreateSnapshot(ctx, db.CreateSnapshotParams{
		Source:        arg.Source,
		SourceVersion: arg.SourceVersion,
		FileName:      arg.FileName,
		FileSha256:    arg.FileSHA256,
		RecordCount:   arg.RecordCount,
		Status:        string(arg.Status),
		SchemaVersion: arg.SchemaVersion,
	})
	if err != nil {
		return domain.DatasetSnapshot{}, err
	}
	return toDomain(row), nil
}

// GetByID fetches a snapshot by UUID string.
func (r *snapshotRepository) GetByID(ctx context.Context, id string) (domain.DatasetSnapshot, error) {
	uid, err := parseUUID(id)
	if err != nil {
		return domain.DatasetSnapshot{}, err
	}
	row, err := r.queries.GetSnapshotByID(ctx, uid)
	if err != nil {
		return domain.DatasetSnapshot{}, err
	}
	return toDomain(row), nil
}

// List returns all snapshots ordered by downloaded_at DESC.
func (r *snapshotRepository) List(ctx context.Context) ([]domain.DatasetSnapshot, error) {
	rows, err := r.queries.ListSnapshots(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.DatasetSnapshot, len(rows))
	for i, v := range rows {
		out[i] = toDomain(v)
	}
	return out, nil
}

// UpdateStatus transitions status via state machine and DB update.
// It reads current status, validates CanTransition, then executes UPDATE.
func (r *snapshotRepository) UpdateStatus(ctx context.Context, id string, to domain.SnapshotStatus) error {
	if !domain.IsValidStatus(to) {
		return fmt.Errorf("invalid target status: %s", to)
	}
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	// Read current to validate state machine.
	cur, err := r.queries.GetSnapshotByID(ctx, uid)
	if err != nil {
		return err
	}
	from := domain.SnapshotStatus(cur.Status)
	if !domain.CanTransition(from, to) {
		return fmt.Errorf("illegal transition %s -> %s", from, to)
	}

	// Handle IMPORTING: set import_started_at as well.
	var tag pgconn.CommandTag
	if to == domain.SnapshotStatusImporting {
		tag, err = r.db.Exec(ctx, `UPDATE dataset_snapshot SET status=$2, import_started_at=NOW() WHERE id=$1`, uid, string(to))
	} else if to == domain.SnapshotStatusFailed {
		tag, err = r.db.Exec(ctx, `UPDATE dataset_snapshot SET status=$2 WHERE id=$1`, uid, string(to))
	} else {
		// Generic: LOCKED is handled via Lock(), but allow it here too with import_completed_at.
		tag, err = r.db.Exec(ctx, `UPDATE dataset_snapshot SET status=$2, import_completed_at=NOW() WHERE id=$1`, uid, string(to))
	}
	if err != nil {
		// DB trigger for LOCKED will surface as "snapshot locked: ..."
		if strings.Contains(err.Error(), "snapshot locked") {
			return fmt.Errorf("snapshot locked: %w", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("snapshot not found or locked: %s", id)
	}
	return nil
}

// Lock transitions IMPORTING -> LOCKED atomically, checking rowCount.
// It mirrors: UPDATE status='LOCKED' WHERE status='IMPORTING'.
func (r *snapshotRepository) Lock(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `UPDATE dataset_snapshot SET status='LOCKED', import_completed_at=NOW() WHERE id=$1 AND status='IMPORTING'`, uid)
	if err != nil {
		if strings.Contains(err.Error(), "snapshot locked") {
			return fmt.Errorf("snapshot locked: %w", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		// Distinguish: check if snapshot exists and its status.
		cur, qErr := r.queries.GetSnapshotByID(ctx, uid)
		if qErr != nil {
			return fmt.Errorf("lock failed: snapshot not found %s: %w", id, qErr)
		}
		return fmt.Errorf("lock failed: snapshot %s status is %s, expected IMPORTING", id, cur.Status)
	}
	return nil
}

// Delete removes a snapshot; DB trigger blocks if status='LOCKED'.
func (r *snapshotRepository) Delete(ctx context.Context, id string) error {
	uid, err := parseUUID(id)
	if err != nil {
		return err
	}
	tag, err := r.db.Exec(ctx, `DELETE FROM dataset_snapshot WHERE id=$1`, uid)
	if err != nil {
		if strings.Contains(err.Error(), "snapshot locked") {
			return fmt.Errorf("snapshot locked: %w", err)
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("snapshot not found: %s", id)
	}
	return nil
}

func parseUUID(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q: %w", s, err)
	}
	if !u.Valid {
		return pgtype.UUID{}, fmt.Errorf("invalid uuid %q", s)
	}
	return u, nil
}

func toDomain(row db.DatasetSnapshot) domain.DatasetSnapshot {
	ds := domain.DatasetSnapshot{
		Source:        row.Source,
		SourceVersion: row.SourceVersion,
		FileName:      row.FileName,
		FileSHA256:    row.FileSha256,
		RecordCount:   row.RecordCount,
		Status:        domain.SnapshotStatus(row.Status),
		SchemaVersion: row.SchemaVersion,
	}
	if row.ID.Valid {
		ds.ID = uuidToString(row.ID)
	}
	if row.DownloadedAt.Valid {
		ds.DownloadedAt = row.DownloadedAt.Time
	}
	if row.PublishedAt.Valid {
		t := row.PublishedAt.Time
		ds.PublishedAt = &t
	}
	if row.ImportStartedAt.Valid {
		t := row.ImportStartedAt.Time
		ds.ImportStartedAt = &t
	}
	if row.ImportCompletedAt.Valid {
		t := row.ImportCompletedAt.Time
		ds.ImportCompletedAt = &t
	}
	if row.CreatedAt.Valid {
		ds.CreatedAt = row.CreatedAt.Time
	}
	return ds
}
func uuidToString(u pgtype.UUID) string {
	id := uuid.UUID(u.Bytes)
	return id.String()
}

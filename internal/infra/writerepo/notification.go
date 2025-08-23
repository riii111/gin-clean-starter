package writerepo

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	"gin-clean-starter/internal/infra/sqlc"
	"gin-clean-starter/internal/usecase/queries"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type NotificationQueries interface {
	CreateNotificationJob(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateNotificationJobParams) error
	GetPendingNotificationJobs(ctx context.Context, db sqlc.DBTX, limit int32) ([]sqlc.NotificationJobs, error)
	UpdateNotificationJobStatus(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateNotificationJobStatusParams) error
}

type NotificationRepository struct {
	queries NotificationQueries
	db      sqlc.DBTX
}

func NewNotificationRepository(queries *sqlc.Queries, db sqlc.DBTX) *NotificationRepository {
	return &NotificationRepository{
		queries: queries,
		db:      db,
	}
}

func (r *NotificationRepository) CreateJob(ctx context.Context, tx sqlc.DBTX, kind, topic string, payload []byte, runAt time.Time) error {
	params := sqlc.CreateNotificationJobParams{
		Kind:    kind,
		Topic:   topic,
		Payload: payload,
		RunAt:   pgtype.Timestamptz{Time: runAt, Valid: true},
		Status:  "queued",
	}

	err := r.queries.CreateNotificationJob(ctx, tx, params)
	if err != nil {
		return infra.WrapRepoErr("failed to create notification job", err)
	}

	return nil
}

func (r *NotificationRepository) GetPendingJobs(ctx context.Context, limit int32) ([]*queries.NotificationJobView, error) {
	rows, err := r.queries.GetPendingNotificationJobs(ctx, r.db, limit)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get pending notification jobs", err)
	}

	result := make([]*queries.NotificationJobView, len(rows))
	for i, row := range rows {
		result[i] = toNotificationJobViewFromRow(row)
	}

	return result, nil
}

func (r *NotificationRepository) UpdateJobStatus(ctx context.Context, tx sqlc.DBTX, jobID uuid.UUID, status string, lastError *string) error {
	params := sqlc.UpdateNotificationJobStatusParams{
		ID:     jobID,
		Status: status,
	}

	if lastError != nil {
		params.LastError = pgtype.Text{String: *lastError, Valid: true}
	} else {
		params.LastError = pgtype.Text{Valid: false}
	}

	err := r.queries.UpdateNotificationJobStatus(ctx, tx, params)
	if err != nil {
		return infra.WrapRepoErr("failed to update notification job status", err)
	}

	return nil
}

func toNotificationJobViewFromRow(row sqlc.NotificationJobs) *queries.NotificationJobView {
	rm := &queries.NotificationJobView{
		ID:        row.ID,
		Kind:      row.Kind,
		Topic:     row.Topic,
		Payload:   row.Payload,
		RunAt:     row.RunAt.Time,
		Attempts:  row.Attempts,
		Status:    row.Status,
		CreatedAt: row.CreatedAt.Time,
		UpdatedAt: row.UpdatedAt.Time,
	}

	if row.LastError.Valid {
		rm.LastError = &row.LastError.String
	}

	return rm
}

package repository

import (
	"context"
	"time"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

type NotificationWriteQueries interface {
	CreateNotificationJob(ctx context.Context, db sqlc.DBTX, arg sqlc.CreateNotificationJobParams) error
	UpdateNotificationJobStatus(ctx context.Context, db sqlc.DBTX, arg sqlc.UpdateNotificationJobStatusParams) error
}

type NotificationRepository struct {
	queries NotificationWriteQueries
	db      sqlc.DBTX
}

func NewNotificationRepository(queries NotificationWriteQueries, db sqlc.DBTX) *NotificationRepository {
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

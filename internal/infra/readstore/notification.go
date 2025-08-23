package readstore

import (
	"context"

	"gin-clean-starter/internal/infra"
	sqlc "gin-clean-starter/internal/infra/sqlc/generated"
	"gin-clean-starter/internal/usecase/queries"
)

type NotificationReadQueries interface {
	GetPendingNotificationJobs(ctx context.Context, db sqlc.DBTX, limit int32) ([]sqlc.NotificationJobs, error)
}

type NotificationReadStore struct {
	queries NotificationReadQueries
	db      sqlc.DBTX
}

func NewNotificationReadStore(queries *sqlc.Queries, db sqlc.DBTX) *NotificationReadStore {
	return &NotificationReadStore{
		queries: queries,
		db:      db,
	}
}

func (s *NotificationReadStore) GetPendingJobs(ctx context.Context, limit int32) ([]*queries.NotificationJobView, error) {
	rows, err := s.queries.GetPendingNotificationJobs(ctx, s.db, limit)
	if err != nil {
		return nil, infra.WrapRepoErr("failed to get pending notification jobs", err)
	}

	result := make([]*queries.NotificationJobView, len(rows))
	for i, row := range rows {
		result[i] = toNotificationJobViewFromRow(row)
	}

	return result, nil
}

func toNotificationJobViewFromRow(row sqlc.NotificationJobs) *queries.NotificationJobView {
	view := &queries.NotificationJobView{
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
		view.LastError = &row.LastError.String
	}

	return view
}

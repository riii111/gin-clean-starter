package pgconv

import (
	"database/sql"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrInvalidFloat64Value = errors.New("invalid float64 value in pgtype.Float8")

func UUIDPtrFromPgtype(pu pgtype.UUID) *uuid.UUID {
	if !pu.Valid {
		return nil
	}
	id := uuid.UUID(pu.Bytes)
	return &id
}

func StringPtrFromPgtype(pt pgtype.Text) *string {
	if !pt.Valid {
		return nil
	}
	return &pt.String
}

func TimeFromPgtype(pt pgtype.Timestamptz) time.Time {
	return pt.Time
}

func Float64PtrFromPgtype(pf pgtype.Float8) (*float64, error) {
	if !pf.Valid {
		return nil, nil
	}

	value, err := pf.Float64Value()
	if err != nil {
		return nil, ErrInvalidFloat64Value
	}

	return &value.Float64, nil
}

func Float64PtrFromNumeric(pn pgtype.Numeric) (*float64, error) {
	if !pn.Valid {
		return nil, nil
	}

	value, err := pn.Float64Value()
	if err != nil {
		return nil, ErrInvalidFloat64Value
	}

	return &value.Float64, nil
}

func Int32PtrFromPgtype(pi pgtype.Int4) *int32 {
	if !pi.Valid {
		return nil
	}
	return &pi.Int32
}

func UUIDToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func UUIDPtrToPgtype(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func StringToPgtype(s string) pgtype.Text {
	return pgtype.Text{String: s, Valid: true}
}

func StringPtrToPgtype(s *string) pgtype.Text {
	if s == nil {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: *s, Valid: true}
}

func TimeToPgtype(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// IsNoRows checks if the error is a "no rows" error from either sql or pgx
func IsNoRows(err error) bool {
	return errors.Is(err, sql.ErrNoRows) || errors.Is(err, pgx.ErrNoRows)
}

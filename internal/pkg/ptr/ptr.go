package ptr

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func Int32FromPgtype(pi pgtype.Int4) *int32 {
	if !pi.Valid {
		return nil
	}
	return &pi.Int32
}

func Float64FromPgtype(pn pgtype.Numeric) (*float64, error) {
	if !pn.Valid {
		return nil, nil
	}

	value, err := pn.Float64Value()
	if err != nil {
		return nil, err
	}

	return &value.Float64, nil
}

func TimeFromPgtype(pt pgtype.Timestamptz) *time.Time {
	if !pt.Valid {
		return nil
	}
	return &pt.Time
}

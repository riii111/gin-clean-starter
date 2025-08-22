package infra

import (
	"errors"

	"gin-clean-starter/internal/pkg/errs"

	"github.com/jackc/pgx/v5/pgconn"
)

type RepositoryErrorKind string

type RepositoryError struct {
	Kind RepositoryErrorKind
	msg  string
	err  error // wrapped low-level error
}

func (e RepositoryError) Error() string {
	if e.err != nil {
		return string(e.Kind) + ": " + e.msg + ": " + e.err.Error()
	}
	return string(e.Kind) + ": " + e.msg
}

func (e RepositoryError) Unwrap() error {
	return e.err
}

func WrapRepoErr(msg string, err error, kinds ...RepositoryErrorKind) error {
	var kind RepositoryErrorKind
	if len(kinds) > 0 {
		kind = kinds[0] // Use specified kind
	} else {
		kind = classifyPgErr(err) // Auto-classify if not specified
	}

	if err != nil {
		err = errs.Wrap(err, msg)
	}

	return RepositoryError{Kind: kind, msg: msg, err: err}
}

func IsKind(err error, kind RepositoryErrorKind) bool {
	var e RepositoryError
	if errors.As(err, &e) {
		return e.Kind == kind
	}
	return false
}

const (
	KindNotFound           RepositoryErrorKind = "NOT_FOUND"
	KindDBFailure          RepositoryErrorKind = "DB_FAILURE"
	KindDuplicateKey       RepositoryErrorKind = "DUPLICATE_KEY"
	KindForeignKeyViolated RepositoryErrorKind = "FOREIGN_KEY_VIOLATED"
	KindConflict           RepositoryErrorKind = "CONFLICT"
)

func classifyPgErr(err error) RepositoryErrorKind {
	pgErr := &pgconn.PgError{}
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return KindDuplicateKey
		case "23503": // foreign_key_violation
			return KindForeignKeyViolated
		default:
			return KindDBFailure
		}
	}
	return KindDBFailure
}

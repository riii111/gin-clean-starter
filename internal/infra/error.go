package infra

import (
	"errors"
	"log/slog"

	"gin-clean-starter/internal/pkg/errs"
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

func WrapRepoErr(slogger *slog.Logger, kind RepositoryErrorKind, msg string, err error) error {
	logArgs := []any{
		slog.String("kind", string(kind)),
	}

	slogger.Error("Repository error: "+msg, logArgs...)

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

// Infrastructure-specific error kinds
const (
	KindNotFound           RepositoryErrorKind = "NOT_FOUND"
	KindDBFailure          RepositoryErrorKind = "DB_FAILURE"
	KindDuplicateKey       RepositoryErrorKind = "DUPLICATE_KEY"
	KindForeignKeyViolated RepositoryErrorKind = "FOREIGN_KEY_VIOLATED"
)

package errs

import (
	"fmt"
	"strings"

	cr "github.com/cockroachdb/errors"
)

func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return cr.Wrap(err, msg)
}

func New(msg string) error {
	return cr.New(msg)
}

func Mark(err error, markErr error) error {
	if err == nil {
		return markErr
	}
	return cr.Mark(err, markErr)
}

func ExtractStackLines(err error, maxLines int) []string {
	if err == nil {
		return nil
	}
	s := fmt.Sprintf("%+v", err)
	lines := strings.Split(s, "\n")
	if maxLines > 0 && len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	return lines
}

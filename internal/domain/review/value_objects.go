package review

import "strings"

const MaxCommentLength = 1000

type Rating struct {
	value int
}

func NewRating(v int) (Rating, error) {
	if v < 1 || v > 5 {
		return Rating{}, ErrInvalidRating
	}
	return Rating{value: v}, nil
}

func (r Rating) Value() int { return r.value }

type Comment struct {
	text string
}

func NewComment(s string) (Comment, error) {
	t := strings.TrimSpace(s)
	if t == "" {
		return Comment{}, ErrEmptyComment
	}
	if len(t) > MaxCommentLength {
		return Comment{}, ErrCommentTooLong
	}
	return Comment{text: t}, nil
}

func (c Comment) String() string { return c.text }

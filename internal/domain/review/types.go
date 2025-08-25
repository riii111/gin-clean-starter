package review

import "errors"

var (
    ErrInvalidRating           = errors.New("rating must be between 1 and 5")
    ErrEmptyComment            = errors.New("comment cannot be empty")
    ErrCommentTooLong          = errors.New("comment exceeds maximum length")

    ErrReservationNotEligible  = errors.New("reservation is not eligible for review")
    ErrReviewAlreadyExists     = errors.New("review already exists for this reservation")
)


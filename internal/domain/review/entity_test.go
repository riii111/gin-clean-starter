//go:build unit

package review_test

import (
	"testing"
	"time"

	"gin-clean-starter/internal/domain/review"
	"gin-clean-starter/tests/common/builder"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCase struct {
	name   string
	mutate func(*builder.ReviewBuilder)
	errIs  error
}

func TestReview(t *testing.T) {
	t.Run("basic success case", func(t *testing.T) {
		actual, err := builder.NewReviewBuilder().BuildDomain()
		require.NoError(t, err)
		require.NotNil(t, actual)

		assert.NotEqual(t, uuid.Nil, actual.ID())
		assert.False(t, actual.CreatedAt().IsZero())
		assert.False(t, actual.UpdatedAt().IsZero())
		assert.Equal(t, actual.CreatedAt(), actual.UpdatedAt())
		assert.Equal(t, 5, actual.Rating().Value())
		assert.Equal(t, "Excellent service!", actual.Comment().String())
	})

	t.Run("rating validation", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name:   "below minimum rating",
				mutate: func(b *builder.ReviewBuilder) { b.WithRating(0) },
				errIs:  review.ErrInvalidRating,
			},
			{
				name:   "minimum valid rating",
				mutate: func(b *builder.ReviewBuilder) { b.WithRating(1) },
			},
			{
				name:   "maximum valid rating",
				mutate: func(b *builder.ReviewBuilder) { b.WithRating(5) },
			},
			{
				name:   "above maximum rating",
				mutate: func(b *builder.ReviewBuilder) { b.WithRating(6) },
				errIs:  review.ErrInvalidRating,
			},
			{
				name:   "negative rating",
				mutate: func(b *builder.ReviewBuilder) { b.WithRating(-1) },
				errIs:  review.ErrInvalidRating,
			},
		})
	})

	t.Run("comment validation", func(t *testing.T) {
		runCases(t, []testCase{
			{
				name:   "minimum length comment",
				mutate: func(b *builder.ReviewBuilder) { b.WithComment("a") },
			},
			{
				name: "maximum length comment",
				mutate: func(b *builder.ReviewBuilder) {
					longComment := make([]byte, review.MaxCommentLength)
					for i := range longComment {
						longComment[i] = 'a'
					}
					b.WithComment(string(longComment))
				},
			},
			{
				name:   "empty comment",
				mutate: func(b *builder.ReviewBuilder) { b.WithComment("") },
				errIs:  review.ErrEmptyComment,
			},
			{
				name:   "whitespace only comment",
				mutate: func(b *builder.ReviewBuilder) { b.WithComment("   ") },
				errIs:  review.ErrEmptyComment,
			},
			{
				name: "comment exceeds maximum length",
				mutate: func(b *builder.ReviewBuilder) {
					longComment := make([]byte, review.MaxCommentLength+1)
					for i := range longComment {
						longComment[i] = 'a'
					}
					b.WithComment(string(longComment))
				},
				errIs: review.ErrCommentTooLong,
			},
		})
	})

	t.Run("comment trimming", func(t *testing.T) {
		userID := uuid.New()
		resourceID := uuid.New()
		reservationID := uuid.New()
		now := time.Now()

		review, err := review.NewReview(uuid.Nil, userID, resourceID, reservationID, 4, "  Trimmed comment  ", now)
		require.NoError(t, err)
		require.NotNil(t, review)

		assert.Equal(t, "Trimmed comment", review.Comment().String())
	})

	t.Run("UUID uniqueness", func(t *testing.T) {
		userID := uuid.New()
		resourceID := uuid.New()
		reservationID := uuid.New()
		now := time.Now()

		review1, err1 := review.NewReview(uuid.Nil, userID, resourceID, reservationID, 5, "Great!", now)
		review2, err2 := review.NewReview(uuid.Nil, userID, resourceID, reservationID, 5, "Great!", now)

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NotNil(t, review1)
		require.NotNil(t, review2)

		assert.NotEqual(t, review1.ID(), review2.ID())
	})
}

func runCases(t *testing.T, cases []testCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual, err := builder.NewReviewBuilder().With(c.mutate).BuildDomain()

			if c.errIs == nil {
				require.NotNil(t, actual)
				require.NoError(t, err)
			} else {
				require.Nil(t, actual)
				require.Error(t, err)
				require.ErrorIs(t, err, c.errIs)
			}
		})
	}
}

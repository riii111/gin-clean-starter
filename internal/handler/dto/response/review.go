package response

import (
	"gin-clean-starter/internal/usecase/queries"
)

type ReviewResponse struct {
	ID            string `json:"id"`
	UserID        string `json:"user_id"`
	UserEmail     string `json:"user_email"`
	ResourceID    string `json:"resource_id"`
	ResourceName  string `json:"resource_name"`
	ReservationID string `json:"reservation_id"`
	Rating        int32  `json:"rating"`
	Comment       string `json:"comment"`
	CreatedAt     int64  `json:"created_at"`
	UpdatedAt     int64  `json:"updated_at"`
}

func FromReviewView(v *queries.ReviewView) *ReviewResponse {
	return &ReviewResponse{
		ID:            v.ID.String(),
		UserID:        v.UserID.String(),
		UserEmail:     v.UserEmail,
		ResourceID:    v.ResourceID.String(),
		ResourceName:  v.ResourceName,
		ReservationID: v.ReservationID.String(),
		Rating:        v.Rating,
		Comment:       v.Comment,
		CreatedAt:     v.CreatedAt.Unix(),
		UpdatedAt:     v.UpdatedAt.Unix(),
	}
}

type ReviewListItemResponse struct {
	ID        string `json:"id"`
	UserEmail string `json:"user_email"`
	Rating    int32  `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt int64  `json:"created_at"`
}

func FromReviewList(items []*queries.ReviewListItem) []*ReviewListItemResponse {
	res := make([]*ReviewListItemResponse, len(items))
	for i, it := range items {
		res[i] = &ReviewListItemResponse{
			ID:        it.ID.String(),
			UserEmail: it.UserEmail,
			Rating:    it.Rating,
			Comment:   it.Comment,
			CreatedAt: it.CreatedAt.Unix(),
		}
	}
	return res
}

type ResourceRatingStatsResponse struct {
	ResourceID    string  `json:"resource_id"`
	TotalReviews  int32   `json:"total_reviews"`
	AverageRating float64 `json:"average_rating"`
	Rating1Count  int32   `json:"rating_1_count"`
	Rating2Count  int32   `json:"rating_2_count"`
	Rating3Count  int32   `json:"rating_3_count"`
	Rating4Count  int32   `json:"rating_4_count"`
	Rating5Count  int32   `json:"rating_5_count"`
	UpdatedAt     int64   `json:"updated_at"`
}

func FromResourceRatingStats(s *queries.ResourceRatingStats) *ResourceRatingStatsResponse {
	return &ResourceRatingStatsResponse{
		ResourceID:    s.ResourceID.String(),
		TotalReviews:  s.TotalReviews,
		AverageRating: s.AverageRating,
		Rating1Count:  s.Rating1Count,
		Rating2Count:  s.Rating2Count,
		Rating3Count:  s.Rating3Count,
		Rating4Count:  s.Rating4Count,
		Rating5Count:  s.Rating5Count,
		UpdatedAt:     s.UpdatedAt.Unix(),
	}
}

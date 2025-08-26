package response

import (
	"gin-clean-starter/internal/usecase/queries"
)

type ReviewResponse struct {
	ID            string `json:"id"`
	UserID        string `json:"userId"`
	UserEmail     string `json:"userEmail"`
	ResourceID    string `json:"resourceId"`
	ResourceName  string `json:"resourceName"`
	ReservationID string `json:"reservationId"`
	Rating        int32  `json:"rating"`
	Comment       string `json:"comment"`
	CreatedAt     int64  `json:"createdAt"`
	UpdatedAt     int64  `json:"updatedAt"`
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
	UserEmail string `json:"userEmail"`
	Rating    int32  `json:"rating"`
	Comment   string `json:"comment"`
	CreatedAt int64  `json:"createdAt"`
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
	ResourceID    string  `json:"resourceId"`
	TotalReviews  int32   `json:"totalReviews"`
	AverageRating float64 `json:"averageRating"`
	Rating1Count  int32   `json:"rating1Count"`
	Rating2Count  int32   `json:"rating2Count"`
	Rating3Count  int32   `json:"rating3Count"`
	Rating4Count  int32   `json:"rating4Count"`
	Rating5Count  int32   `json:"rating5Count"`
	UpdatedAt     int64   `json:"updatedAt"`
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

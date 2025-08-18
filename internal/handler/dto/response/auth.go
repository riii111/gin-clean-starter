package response

import "gin-clean-starter/internal/usecase/readmodel"

type LoginResponse struct {
	AccessToken string                      `json:"access_token"`
	User        *readmodel.AuthorizedUserRM `json:"user"`
}

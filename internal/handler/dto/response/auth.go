package response

import "gin-clean-starter/internal/usecase/readmodel"

type LoginResponse struct {
	User *readmodel.AuthorizedUserRM `json:"user"`
}

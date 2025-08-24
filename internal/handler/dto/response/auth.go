package response

import "gin-clean-starter/internal/usecase/queries"

type LoginResponse struct {
	User *queries.AuthorizedUserView `json:"user"`
}

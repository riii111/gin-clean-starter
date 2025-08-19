package readmodel

import (
	"github.com/google/uuid"
)

type AuthorizedUserRM struct {
	ID        uuid.UUID  `json:"id"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	CompanyID *uuid.UUID `json:"company_id,omitempty"`
	IsActive  bool       `json:"is_active"`
}

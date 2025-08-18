package user

type Role string

const (
	RoleViewer   Role = "viewer"
	RoleOperator Role = "operator"
	RoleAdmin    Role = "admin"
)

func (r Role) String() string {
	return string(r)
}

func (r Role) IsValid() bool {
	switch r {
	case RoleViewer, RoleOperator, RoleAdmin:
		return true
	default:
		return false
	}
}

func NewRole(s string) (Role, error) {
	role := Role(s)
	if !role.IsValid() {
		return "", ErrInvalidRole
	}
	return role, nil
}

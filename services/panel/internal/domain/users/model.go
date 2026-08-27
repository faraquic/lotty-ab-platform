package users

import "time"

type Role string

const (
	RoleAdmin        Role = "admin"
	RoleExperimenter Role = "experimenter"
	RoleApprover     Role = "approver"
	RoleViewer       Role = "viewer"
)

func AllRoles() []Role {
	return []Role{RoleAdmin, RoleExperimenter, RoleApprover, RoleViewer}
}

func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleExperimenter, RoleApprover, RoleViewer:
		return true
	default:
		return false
	}
}

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Role         Role
	AvatarURL    string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

package auth

// Role represents a user role in the system.
type Role string

const (
	RoleAdmin Role = "admin"
	RoleWrite Role = "write"
	RoleRead  Role = "read"
)

// HasPermission checks if the given role has permission for the specified action.
func HasPermission(role Role, action string) bool {
	switch role {
	case RoleAdmin:
		return true
	case RoleWrite:
		return action == "spawn" || action == "write" || action == "resize" || action == "attach"
	case RoleRead:
		return action == "attach" || action == "heartbeat"
	default:
		return false
	}
}

// CanManageSession returns true if the role can manage (kill) sessions.
func CanManageSession(role Role) bool {
	return role == RoleAdmin
}

// CanCreateInvite returns true if the role can create invite tokens.
func CanCreateInvite(role Role) bool {
	return role == RoleAdmin
}

package auth

import (
	"crypto/subtle"
)

// VerifyAdminPassword performs constant-time comparison of the provided password against the expected admin password.
func VerifyAdminPassword(provided, expected string) bool {
	if expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

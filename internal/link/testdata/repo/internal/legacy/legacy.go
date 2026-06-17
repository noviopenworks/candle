package legacy

// ValidateToken is a same-named symbol in a DIFFERENT package than auth.
// Export disambiguation must not pick this one for the auth export.
func ValidateToken(token string) bool {
	return false
}

package web

import "strings"

// These pure validators are shared by registration and account settings so the
// same rules apply everywhere. Each returns an error message, or "" when valid.

func validateUsername(username string) string {
	if len(username) < 3 || len(username) > 32 {
		return "Username must be 3-32 characters"
	}
	return ""
}

func validateEmail(email string) string {
	if !strings.Contains(email, "@") {
		return "Enter a valid email"
	}
	return ""
}

func validatePassword(password string) string {
	if len(password) < 10 {
		return "Password must be at least 10 characters"
	}
	return ""
}

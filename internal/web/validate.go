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

// validateAllianceTag accepts a short, uppercase-friendly abbreviation made of
// letters, digits and hyphens (reference tags look like [ThF-], [Linux]).
func validateAllianceTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if len(tag) < 2 || len(tag) > 8 {
		return "Tag must be 2-8 characters"
	}
	for _, r := range tag {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-'
		if !ok {
			return "Tag may only contain letters, digits and hyphens"
		}
	}
	return ""
}

func validateAllianceName(name string) string {
	name = strings.TrimSpace(name)
	if len(name) < 3 || len(name) > 64 {
		return "Alliance name must be 3-64 characters"
	}
	return ""
}

func validatePost(title, body string) string {
	if len(strings.TrimSpace(title)) < 3 || len(title) > 120 {
		return "Title must be 3-120 characters"
	}
	if strings.TrimSpace(body) == "" {
		return "Post body cannot be empty"
	}
	return ""
}

// validateDiplomacyMessage bounds the optional message that accompanies a NAP or
// war declaration.
func validateDiplomacyMessage(message string) string {
	if len(message) > 500 {
		return "Message must be at most 500 characters"
	}
	return ""
}

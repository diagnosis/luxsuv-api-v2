package helper

import (
	"net/mail"
	"strings"
)

func IsValidEmail(email string) bool {
	email = strings.TrimSpace(email)
	if len(email) == 0 {
		return false
	}
	_, err := mail.ParseAddress(email)
	return err == nil
}

func IsValidPassword(password string) bool {
	return len(strings.TrimSpace(password)) >= 8
}

func SanitizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

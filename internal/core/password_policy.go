package core

import (
	"errors"
	"regexp"
)

var (
	ErrPasswordTooShort    = errors.New("Password must be at least 8 characters")
	ErrPasswordPolicy      = errors.New("Password must contain at least one uppercase letter, one lowercase letter, one number, and one special character")
	passwordUppercaseRegex = regexp.MustCompile(`[A-Z]`)
	passwordLowercaseRegex = regexp.MustCompile(`[a-z]`)
	passwordNumberRegex    = regexp.MustCompile(`[0-9]`)
	passwordSpecialRegex   = regexp.MustCompile(`[^A-Za-z0-9[:space:]]`)
)

func ValidatePasswordPolicy(password string) error {
	if len(password) < 8 {
		return ErrPasswordTooShort
	}

	if !passwordUppercaseRegex.MatchString(password) ||
		!passwordLowercaseRegex.MatchString(password) ||
		!passwordNumberRegex.MatchString(password) ||
		!passwordSpecialRegex.MatchString(password) {
		return ErrPasswordPolicy
	}

	return nil
}

package user

import (
	"errors"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"
)

const MinPasswordLength = 8

var (
	ErrInvalidEmail       = errors.New("user: invalid email")
	ErrWeakPassword       = errors.New("user: password is too short")
	ErrEmailTaken         = errors.New("user: email already registered")
	ErrInvalidCredentials = errors.New("user: invalid credentials")
	ErrNotFound           = errors.New("user: not found")
)

type User struct {
	ID           string
	Email        string
	PasswordHash string
	CreatedAt    time.Time
}

type Credentials struct {
	Email    string
	Password string
}

type Session struct {
	Token string
}

func NormalizeEmail(raw string) (string, error) {
	email := strings.ToLower(strings.TrimSpace(raw))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email {
		return "", ErrInvalidEmail
	}
	return email, nil
}

func validatePassword(plain string) error {
	if utf8.RuneCountInString(plain) < MinPasswordLength {
		return ErrWeakPassword
	}
	return nil
}

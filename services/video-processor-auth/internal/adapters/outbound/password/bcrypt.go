package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

type Bcrypt struct {
	Cost int
}

func (b Bcrypt) Hash(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), b.Cost)
	if err != nil {
		return "", fmt.Errorf("bcrypt: %w", err)
	}
	return string(hash), nil
}

func (Bcrypt) Compare(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

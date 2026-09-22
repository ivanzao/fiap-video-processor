package user

import (
	"context"
	"errors"
	"fmt"
)

type Service struct {
	repo   Repository
	hasher PasswordHasher
	tokens TokenIssuer
	clock  Clock
	ids    IDGenerator
}

func NewService(repo Repository, hasher PasswordHasher, tokens TokenIssuer, clock Clock, ids IDGenerator) *Service {
	return &Service{repo: repo, hasher: hasher, tokens: tokens, clock: clock, ids: ids}
}

func (s *Service) SignUp(ctx context.Context, creds Credentials) (User, error) {
	email, err := NormalizeEmail(creds.Email)
	if err != nil {
		return User{}, err
	}
	if err := validatePassword(creds.Password); err != nil {
		return User{}, err
	}
	hash, err := s.hasher.Hash(creds.Password)
	if err != nil {
		return User{}, fmt.Errorf("hash password: %w", err)
	}
	u := User{ID: s.ids.NewID(), Email: email, PasswordHash: hash, CreatedAt: s.clock.Now().UTC()}
	if err := s.repo.Create(ctx, u); err != nil {
		return User{}, err
	}
	return u, nil
}

func (s *Service) Login(ctx context.Context, creds Credentials) (Session, error) {
	email, err := NormalizeEmail(creds.Email)
	if err != nil {
		return Session{}, ErrInvalidCredentials
	}
	u, err := s.repo.FindByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return Session{}, ErrInvalidCredentials
	}
	if err != nil {
		return Session{}, err
	}
	if !s.hasher.Compare(u.PasswordHash, creds.Password) {
		return Session{}, ErrInvalidCredentials
	}
	token, err := s.tokens.Issue(u)
	if err != nil {
		return Session{}, fmt.Errorf("issue token: %w", err)
	}
	return Session{Token: token}, nil
}

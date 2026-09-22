package user_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/domain/user"
)

type fakeRepo struct {
	byEmail map[string]user.User
	err     error
}

func (f *fakeRepo) Create(_ context.Context, u user.User) error {
	if f.err != nil {
		return f.err
	}
	if _, ok := f.byEmail[u.Email]; ok {
		return user.ErrEmailTaken
	}
	f.byEmail[u.Email] = u
	return nil
}

func (f *fakeRepo) FindByEmail(_ context.Context, email string) (user.User, error) {
	if f.err != nil {
		return user.User{}, f.err
	}
	u, ok := f.byEmail[email]
	if !ok {
		return user.User{}, user.ErrNotFound
	}
	return u, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(plain string) (string, error) { return "hashed:" + plain, nil }
func (fakeHasher) Compare(hash, plain string) bool   { return hash == "hashed:"+plain }

type fakeTokens struct{ err error }

func (f fakeTokens) Issue(u user.User) (string, error) { return "token-for-" + u.ID, f.err }

type fixedClock struct{ now time.Time }

func (c fixedClock) Now() time.Time { return c.now }

type fixedIDs struct{ id string }

func (f fixedIDs) NewID() string { return f.id }

var now = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

func newService(repo *fakeRepo) *user.Service {
	return user.NewService(repo, fakeHasher{}, fakeTokens{}, fixedClock{now}, fixedIDs{"usr-1"})
}

func TestSignUpStoresNormalizedEmailAndHashedPassword(t *testing.T) {
	repo := &fakeRepo{byEmail: map[string]user.User{}}

	got, err := newService(repo).SignUp(t.Context(), user.Credentials{Email: "  Ana@Example.com ", Password: "correct horse"})

	require.NoError(t, err)
	assert.Equal(t, user.User{ID: "usr-1", Email: "ana@example.com", PasswordHash: "hashed:correct horse", CreatedAt: now}, got)
	assert.Equal(t, got, repo.byEmail["ana@example.com"])
}

func TestSignUpRejectsInvalidInput(t *testing.T) {
	cases := map[string]struct {
		creds user.Credentials
		want  error
	}{
		"malformed email":    {user.Credentials{Email: "not-an-email", Password: "correct horse"}, user.ErrInvalidEmail},
		"email with name":    {user.Credentials{Email: "Ana <ana@example.com>", Password: "correct horse"}, user.ErrInvalidEmail},
		"short password":     {user.Credentials{Email: "ana@example.com", Password: "1234567"}, user.ErrWeakPassword},
		"already registered": {user.Credentials{Email: "taken@example.com", Password: "correct horse"}, user.ErrEmailTaken},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			repo := &fakeRepo{byEmail: map[string]user.User{"taken@example.com": {ID: "usr-0"}}}
			_, err := newService(repo).SignUp(t.Context(), tc.creds)
			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestLoginIssuesTokenForValidCredentials(t *testing.T) {
	repo := &fakeRepo{byEmail: map[string]user.User{"ana@example.com": {ID: "usr-1", Email: "ana@example.com", PasswordHash: "hashed:correct horse"}}}

	got, err := newService(repo).Login(t.Context(), user.Credentials{Email: "ANA@example.com", Password: "correct horse"})

	require.NoError(t, err)
	assert.Equal(t, user.Session{Token: "token-for-usr-1"}, got)
}

func TestLoginHidesWhetherEmailExists(t *testing.T) {
	repo := &fakeRepo{byEmail: map[string]user.User{"ana@example.com": {ID: "usr-1", PasswordHash: "hashed:correct horse"}}}
	svc := newService(repo)

	_, unknownErr := svc.Login(t.Context(), user.Credentials{Email: "nobody@example.com", Password: "correct horse"})
	_, wrongErr := svc.Login(t.Context(), user.Credentials{Email: "ana@example.com", Password: "wrong"})
	_, malformedErr := svc.Login(t.Context(), user.Credentials{Email: "garbage", Password: "correct horse"})

	assert.ErrorIs(t, unknownErr, user.ErrInvalidCredentials)
	assert.ErrorIs(t, wrongErr, user.ErrInvalidCredentials)
	assert.ErrorIs(t, malformedErr, user.ErrInvalidCredentials)
}

func TestLoginPropagatesRepositoryFailures(t *testing.T) {
	boom := errors.New("db down")
	_, err := newService(&fakeRepo{err: boom}).Login(t.Context(), user.Credentials{Email: "ana@example.com", Password: "correct horse"})
	assert.ErrorIs(t, err, boom)
}

func TestLoginWrapsTokenIssuerFailures(t *testing.T) {
	repo := &fakeRepo{byEmail: map[string]user.User{"ana@example.com": {ID: "usr-1", PasswordHash: "hashed:correct horse"}}}
	boom := errors.New("no key")
	svc := user.NewService(repo, fakeHasher{}, fakeTokens{err: boom}, fixedClock{now}, fixedIDs{"usr-1"})
	_, err := svc.Login(t.Context(), user.Credentials{Email: "ana@example.com", Password: "correct horse"})
	assert.ErrorIs(t, err, boom)
}

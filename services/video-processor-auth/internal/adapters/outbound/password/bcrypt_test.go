package password_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-auth/internal/adapters/outbound/password"
)

func TestHashIsSaltedAndVerifiable(t *testing.T) {
	hasher := password.Bcrypt{Cost: bcrypt.MinCost}

	first, err := hasher.Hash("correct horse")
	require.NoError(t, err)
	second, err := hasher.Hash("correct horse")
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
	assert.True(t, hasher.Compare(first, "correct horse"))
	assert.False(t, hasher.Compare(first, "wrong"))
	assert.False(t, hasher.Compare("not a hash", "correct horse"))
}

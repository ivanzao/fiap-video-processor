package workspace_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/workspace"
)

func TestNewCreatesADirectoryPerRequestAndCleanupRemovesIt(t *testing.T) {
	root := t.TempDir()

	dir, cleanup, err := workspace.NewTemp(root).New("req-1")

	require.NoError(t, err)
	assert.Equal(t, root, filepath.Dir(dir))
	assert.Contains(t, filepath.Base(dir), "processing-req-1-")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "frame.png"), []byte("x"), 0o600))

	cleanup()

	assert.NoDirExists(t, dir)
}

func TestNewFailsWhenTheRootIsMissing(t *testing.T) {
	_, cleanup, err := workspace.NewTemp(filepath.Join(t.TempDir(), "missing")).New("req-1")

	require.Error(t, err)
	assert.Nil(t, cleanup)
	assert.Contains(t, err.Error(), "workspace:")
}

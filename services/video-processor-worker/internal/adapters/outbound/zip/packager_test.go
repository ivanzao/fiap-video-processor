package zip_test

import (
	archive "archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/zip"
)

func TestPackWritesEveryFileOfTheDirectoryInNameOrder(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"frame_0002.png", "frame_0001.png", "frame_0003.png"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600))
	}
	zipPath := filepath.Join(t.TempDir(), "frames.zip")

	require.NoError(t, zip.Packager{}.Pack(t.Context(), dir, zipPath))

	r, err := archive.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	var names []string
	for _, f := range r.File {
		names = append(names, f.Name)
	}
	assert.Equal(t, []string{"frame_0001.png", "frame_0002.png", "frame_0003.png"}, names)
	rc, err := r.File[0].Open()
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	content := make([]byte, 32)
	n, _ := rc.Read(content)
	assert.Equal(t, "frame_0001.png", string(content[:n]))
}

func TestPackOfEmptyDirectoryProducesAnEmptyArchive(t *testing.T) {
	zipPath := filepath.Join(t.TempDir(), "frames.zip")

	require.NoError(t, zip.Packager{}.Pack(t.Context(), t.TempDir(), zipPath))

	r, err := archive.OpenReader(zipPath)
	require.NoError(t, err)
	defer func() { _ = r.Close() }()
	assert.Empty(t, r.File)
}

package ffmpeg_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/adapters/outbound/ffmpeg"
	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

func requireFFmpeg(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg not installed")
	}
	return bin
}

func syntheticVideo(t *testing.T, bin string, seconds int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "synthetic.mp4")
	cmd := exec.Command(bin, "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=10",
		"-t", strconv.Itoa(seconds), "-pix_fmt", "yuv420p", "-y", path)
	require.NoError(t, cmd.Run())
	return path
}

func TestExtractProducesOneFramePerSecondAtFrameRateOne(t *testing.T) {
	bin := requireFFmpeg(t)
	video := syntheticVideo(t, bin, 3)
	out := t.TempDir()

	frames, err := ffmpeg.NewExtractor(bin, 1).Extract(t.Context(), video, out, 1)

	require.NoError(t, err)
	assert.Equal(t, 3, frames)
	files, _ := filepath.Glob(filepath.Join(out, "frame_*.png"))
	assert.Len(t, files, 3)
}

func TestExtractClassifiesGarbageInputAsUnprocessable(t *testing.T) {
	bin := requireFFmpeg(t)
	garbage := filepath.Join(t.TempDir(), "garbage.mp4")
	require.NoError(t, os.WriteFile(garbage, []byte("definitely not a video"), 0o600))

	_, err := ffmpeg.NewExtractor(bin, 1).Extract(t.Context(), garbage, t.TempDir(), 1)

	var unprocessable *video.UnprocessableError
	require.ErrorAs(t, err, &unprocessable)
	assert.NotEmpty(t, unprocessable.Reason)
}

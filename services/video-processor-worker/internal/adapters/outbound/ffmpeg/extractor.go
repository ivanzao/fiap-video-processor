package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type Extractor struct {
	binary  string
	threads int
}

func NewExtractor(binary string, threads int) *Extractor {
	return &Extractor{binary: binary, threads: threads}
}

func (e *Extractor) Extract(ctx context.Context, videoPath, outDir string, frameRate int) (int, error) {
	cmd := exec.CommandContext(ctx, e.binary,
		"-hide_banner", "-loglevel", "error", "-nostdin",
		"-threads", fmt.Sprint(e.threads),
		"-i", videoPath,
		"-vf", fmt.Sprintf("fps=%d", frameRate),
		"-y", filepath.Join(outDir, "frame_%04d.png"),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		var exitErr *exec.ExitError
		if isExitError(err, &exitErr) {
			return 0, &video.UnprocessableError{Reason: firstLine(stderr.String())}
		}
		return 0, fmt.Errorf("ffmpeg: %w", err)
	}
	frames, err := filepath.Glob(filepath.Join(outDir, "frame_*.png"))
	if err != nil {
		return 0, fmt.Errorf("ffmpeg: list frames: %w", err)
	}
	if len(frames) == 0 {
		return 0, &video.UnprocessableError{Reason: "no frames could be extracted"}
	}
	return len(frames), nil
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if s == "" {
		return "ffmpeg rejected the input"
	}
	return s
}

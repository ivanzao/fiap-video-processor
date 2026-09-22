package ffmpeg

import (
	"errors"
	"os/exec"
)

func isExitError(err error, target **exec.ExitError) bool {
	return errors.As(err, target)
}

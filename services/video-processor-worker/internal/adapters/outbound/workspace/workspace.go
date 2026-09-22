package workspace

import (
	"fmt"
	"os"
)

type Temp struct {
	root string
}

func NewTemp(root string) Temp {
	return Temp{root: root}
}

func (t Temp) New(requestID string) (string, func(), error) {
	dir, err := os.MkdirTemp(t.root, "processing-"+requestID+"-")
	if err != nil {
		return "", nil, fmt.Errorf("workspace: %w", err)
	}
	return dir, func() { _ = os.RemoveAll(dir) }, nil
}

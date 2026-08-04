package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

// inProject resolves p against projectDir and verifies the result stays
// inside the project. ponytail: symlink escape is not followed; a later
// pass can EvalSymlinks before the prefix check.
func inProject(projectDir, p string) (string, error) {
	if p == "" {
		return "", fmt.Errorf("empty path")
	}
	abs := p
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(projectDir, p)
	}
	abs = filepath.Clean(abs)
	proj := filepath.Clean(projectDir)
	if abs != proj && !strings.HasPrefix(abs, proj+"/") {
		return "", fmt.Errorf("path %q is outside the project directory %q", p, projectDir)
	}
	return abs, nil
}

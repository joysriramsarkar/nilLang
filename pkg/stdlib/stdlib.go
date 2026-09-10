package stdlib

import (
	"embed"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

//go:embed modules
var FS embed.FS

// Read returns a bundled standard-library module.
// The public import namespace is std/<module>.
func Read(module string) (string, error) {
	module = strings.TrimPrefix(module, "std/")
	module = strings.TrimPrefix(module, "/")
	module = strings.TrimSuffix(module, ".nil")
	name := path.Join("modules", module+".nil")
	data, err := fs.ReadFile(FS, name)
	if err != nil {
		return "", fmt.Errorf("standard module %q not found: %w", module, err)
	}
	return string(data), nil
}

func Exists(module string) bool {
	_, err := Read(module)
	return err == nil
}

//go:build !windows

package stdlib

import "github.com/joysriramsarkar/nilLang/compiler/object"

func runWin32NotesGUI(args []object.Object) (object.Object, error) {
	return object.FALSE, nil
}

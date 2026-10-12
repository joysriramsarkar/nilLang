package gpu

import "fmt"

func errNotInitialized() error { return fmt.Errorf("renderer not initialized") }

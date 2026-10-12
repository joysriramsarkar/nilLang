package gpu

import "os"

// lookupBackendEnv reads NIL_<NAME> as 1/true (available) or 0/false.
func lookupBackendEnv(name string) (bool, bool) {
	v, ok := os.LookupEnv(name)
	if !ok {
		return false, false
	}
	switch v {
	case "1", "true", "TRUE", "yes":
		return true, true
	default:
		return false, true
	}
}

package ffi

import (
	"fmt"
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	for _, a := range os.Args {
		if a == "-short" || a == "--short" || a == "-test.short=true" {
			fmt.Println("ffi: skipping long-running suite under -short")
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

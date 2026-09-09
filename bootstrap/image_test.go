package bootstrap

import (
	"bytes"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCompilerImageReachesFixedPoint(t *testing.T) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate bootstrap directory")
	}

	stageB, err := BuildSeed(filepath.Dir(currentFile))
	if err != nil {
		t.Fatal(err)
	}
	stageC, err := Rebuild(stageB)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(stageB, stageC) {
		t.Fatal("self-hosted compiler did not reach a byte-for-byte fixed point")
	}
}

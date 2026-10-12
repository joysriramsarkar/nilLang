package buildcache

import (
	"errors"
	"testing"
)

func TestSecondBuildHitsCache(t *testing.T) {
	c := New(t.TempDir())
	calls := 0
	compile := func(src string) ([]byte, error) {
		calls++
		return []byte("BYTECODE:" + src), nil
	}
	extra := map[string][]byte{"lib.nil": []byte("helper")}

	first, hit, err := c.Build("println(1);", extra, compile)
	if err != nil || hit || calls != 1 {
		t.Fatalf("first build must miss: hit=%v calls=%d err=%v", hit, calls, err)
	}
	second, hit, err := c.Build("println(1);", extra, compile)
	if err != nil || !hit || calls != 1 {
		t.Fatalf("second build must hit without recompiling: hit=%v calls=%d err=%v", hit, calls, err)
	}
	if string(first) != string(second) {
		t.Fatal("cached bytes differ from compiled bytes")
	}
	if h, m := c.Stats(); h != 1 || m != 1 {
		t.Errorf("stats: hits=%d misses=%d, want 1/1", h, m)
	}
}

func TestChangedSourceMisses(t *testing.T) {
	c := New(t.TempDir())
	compile := func(src string) ([]byte, error) { return []byte(src), nil }
	if _, hit, _ := c.Build("a", nil, compile); hit {
		t.Fatal("first build must miss")
	}
	if _, hit, _ := c.Build("b", nil, compile); hit {
		t.Fatal("changed source must miss")
	}
}

func TestKeyIgnoresMapOrder(t *testing.T) {
	a := KeyOf("src", map[string][]byte{"x": []byte("1"), "y": []byte("2")})
	b := KeyOf("src", map[string][]byte{"y": []byte("2"), "x": []byte("1")})
	if a != b {
		t.Error("key must not depend on map iteration order")
	}
}

func TestCompileErrorNotCached(t *testing.T) {
	c := New(t.TempDir())
	boom := errors.New("boom")
	compile := func(src string) ([]byte, error) { return nil, boom }
	if _, _, err := c.Build("src", nil, compile); err != boom {
		t.Fatalf("expected compile error, got %v", err)
	}
	if _, ok := c.Lookup(KeyOf("src", nil)); ok {
		t.Error("failed compilations must not be cached")
	}
}

func TestDiskPersistenceAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	c1 := New(dir)
	compile := func(src string) ([]byte, error) { return []byte("DATA"), nil }
	if _, _, err := c1.Build("src", nil, compile); err != nil {
		t.Fatal(err)
	}
	c2 := New(dir) // fresh memory, same disk
	calls := 0
	compile2 := func(src string) ([]byte, error) { calls++; return []byte("DATA"), nil }
	if _, hit, err := c2.Build("src", nil, compile2); err != nil || !hit || calls != 0 {
		t.Fatalf("disk entry must serve a new instance: hit=%v calls=%d err=%v", hit, calls, err)
	}
}

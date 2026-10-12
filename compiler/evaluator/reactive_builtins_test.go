package evaluator

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/object"
)

func evalReactive(t *testing.T, src string) object.Object {
	t.Helper()
	ResetReactiveGraph()
	return testEval(src)
}

func TestNilangSignalGetSet(t *testing.T) {
	out := evalReactive(t, `let s = signal(10); sset(s, 25); sget(s);`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 25 {
		t.Fatalf("want 25, got %v", out)
	}
}

func TestNilangComputedMemoized(t *testing.T) {
	out := evalReactive(t, `
let s = signal(3);
let runs = signal(0);
let d = computed(fn() { sset(runs, sget(runs) + 1); return sget(s) * 2; });
let a = cget(d);
let b = cget(d);
a + b + sget(runs);
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 13 { // 6+6+1
		t.Fatalf("want 13 (6+6+1 run), got %v", out)
	}
}

func TestNilangComputedInvalidates(t *testing.T) {
	out := evalReactive(t, `
let s = signal(3);
let d = computed(fn() { return sget(s) * 2; });
let before = cget(d);
sset(s, 10);
before + cget(d);
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 26 { // 6+20
		t.Fatalf("want 26, got %v", out)
	}
}

func TestNilangEffectRerunsAndDisposes(t *testing.T) {
	out := evalReactive(t, `
let s = signal(1);
let seen = reactiveArray();
let e = effect(fn() { ra_push(seen, sget(s)); });
sset(s, 2);
sset(s, 3);
dispose(e);
sset(s, 4);
ra_len(seen) * 100 + ra_get(seen, 2);
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 303 { // len 3, last seen 3
		t.Fatalf("want 303, got %v", out)
	}
}

func TestNilangBatchGroups(t *testing.T) {
	out := evalReactive(t, `
let s = signal(0);
let n = signal(0);
let e = effect(fn() { sset(n, sget(n) + 1); let _ = sget(s); });
let before = sget(n);
batch(fn() { sset(s, 1); sset(s, 2); sset(s, 3); });
sget(n) - before;
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 1 {
		t.Fatalf("batched effect must run once, got %v", out)
	}
}

func TestNilangReactiveArrayOps(t *testing.T) {
	out := evalReactive(t, `
let a = reactiveArray(1, 2, 3);
ra_push(a, 4);
ra_set(a, 0, 9);
ra_len(a) * 100 + ra_get(a, 0) * 10 + ra_get(a, 3);
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 400+90+4 {
		t.Fatalf("want 494, got %v", out)
	}
}

func TestNilangStoreOps(t *testing.T) {
	out := evalReactive(t, `
let st = store({"k": 1});
store_set(st, "k", 41);
store_get(st, "k") + 1;
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 42 {
		t.Fatalf("want 42, got %v", out)
	}
}

func TestNilangSignalVersionMonotonic(t *testing.T) {
	out := evalReactive(t, `
let s = signal(0);
let v0 = sver(s);
sset(s, 1);
sset(s, 2);
sver(s) - v0;
`)
	iv, ok := out.(*object.Integer)
	if !ok || iv.Value != 2 {
		t.Fatalf("want version delta 2, got %v", out)
	}
}

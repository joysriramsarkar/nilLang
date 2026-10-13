package typecheck

// TestAnyOrigins — P0.3 (docs/CORE_STABILIZATION.md): every place the checker
// answers `Any` must belong to a recorded class. The three classes are:
//
//  1. Narrowed away — the value carried a precise type and the checker now
//     reports it (Result/Optional payloads, Array element types).
//  2. Explicit dynamic intent — a boundary where dynamic is the contract:
//     host FFI (`native`), host UI (`emit`), tensor builtins (no Tensor type
//     exists yet), unannotated function returns.
//  3. Unresolved inference — the checker honestly has nothing to go on: empty
//     or heterogeneous literals, index expressions, unknown members.
//
// A new `Any` that fits none of these is a regression: extend this table, do
// not let it slide. Tensor builtins stay `Any` until a Tensor type exists;
// `push` checks the receiver but not the element, because the run time
// accepts heterogeneous pushes and the checker must not refuse what runs.

import (
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/types"
)

func checkSource(t *testing.T, src string) (*Checker, bool) {
	t.Helper()
	p := parseProgram(t, src)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		t.Fatalf("parse errors for %q: %v", src, p.Errors())
	}
	c := NewChecker()
	return c, c.CheckProgram(prog)
}

func probeVar(t *testing.T, c *Checker, src string) types.Type {
	t.Helper()
	typ, ok := c.currentScope.GetVar("probe")
	if !ok {
		t.Fatalf("source %q: no `probe` binding", src)
	}
	return typ
}

func isIntArray(typ types.Type) bool {
	g, ok := typ.(*types.GenericType)
	if !ok || (g.Base != "Array" && g.Base != "List") || len(g.Parameters) != 1 {
		return false
	}
	return g.Parameters[0].Equals(types.Int)
}

func TestSequenceBuiltinsNarrow(t *testing.T) {
	intCases := []struct{ name, src string }{
		{"first of literal", `let probe = first([1, 2, 3]);`},
		{"last of literal", `let probe = last([1, 2, 3]);`},
		{"first of inferred var", `let xs = [1, 2]; let probe = first(xs);`},
		{"last of annotated var", `let xs: Array<Int> = [1]; let probe = last(xs);`},
		{"unwrap of Ok", `let probe = unwrap(Ok(5));`},
	}
	for _, tc := range intCases {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := checkSource(t, tc.src)
			if !ok {
				t.Fatalf("valid source rejected: %v", c.Diagnostics)
			}
			if got := probeVar(t, c, tc.src); !got.Equals(types.Int) {
				t.Fatalf("got %s, want Int", got.String())
			}
		})
	}

	t.Run("rest preserves array", func(t *testing.T) {
		const src = `let probe = rest([1, 2]);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !isIntArray(got) {
			t.Fatalf("got %s, want array of Int", got.String())
		}
	})

	t.Run("push preserves array", func(t *testing.T) {
		const src = `let r = [1]; let probe = push(r, 2);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !isIntArray(got) {
			t.Fatalf("got %s, want array of Int", got.String())
		}
	})

	t.Run("push on empty stays dynamic", func(t *testing.T) {
		const src = `let r = []; let probe = push(r, 1);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		got := probeVar(t, c, src)
		if g, isArr := got.(*types.GenericType); !isArr || (g.Base != "Array" && g.Base != "List") {
			t.Fatalf("got %s, want an array type", got.String())
		}
	})

	t.Run("Some infers payload", func(t *testing.T) {
		const src = `let probe = Some(5);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		opt, isOpt := probeVar(t, c, src).(*types.OptionalType)
		if !isOpt || !opt.Base.Equals(types.Int) {
			t.Fatalf("got %s, want ?Int", probeVar(t, c, src).String())
		}
	})
}

func TestPermittedAnyOrigins(t *testing.T) {
	t.Run("empty array literal", func(t *testing.T) {
		const src = `let probe = [];`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		got := probeVar(t, c, src)
		if g, isArr := got.(*types.GenericType); !isArr || !g.Parameters[0].Equals(types.Any) {
			t.Fatalf("got %s, want array of Any", got.String())
		}
	})

	t.Run("heterogeneous array literal", func(t *testing.T) {
		const src = `let probe = [1, "a"];`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		got := probeVar(t, c, src)
		if g, isArr := got.(*types.GenericType); !isArr || !g.Parameters[0].Equals(types.Any) {
			t.Fatalf("got %s, want array of Any", got.String())
		}
	})

	t.Run("empty hash literal", func(t *testing.T) {
		const src = `let probe = {};`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); got.String() != "Hash<Any, Any>" {
			t.Fatalf("got %s, want Hash<Any, Any>", got.String())
		}
	})

	t.Run("None carries nothing", func(t *testing.T) {
		const src = `let probe = None();`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		opt, isOpt := probeVar(t, c, src).(*types.OptionalType)
		if !isOpt || !opt.Base.Equals(types.Any) {
			t.Fatalf("got %s, want ?Any", probeVar(t, c, src).String())
		}
	})

	t.Run("Ok error side stays Any", func(t *testing.T) {
		const src = `let probe = Ok(5);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		got := probeVar(t, c, src)
		g, isRes := got.(*types.GenericType)
		if !isRes || g.Base != "Result" || !g.Parameters[0].Equals(types.Int) || !g.Parameters[1].Equals(types.Any) {
			t.Fatalf("got %s, want Result<Int, Any>", got.String())
		}
	})

	t.Run("index expression is untyped", func(t *testing.T) {
		const src = `let a = [1, 2]; let probe = a[0];`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !got.Equals(types.Any) {
			t.Fatalf("got %s, want Any", got.String())
		}
	})

	t.Run("unannotated function returns Any", func(t *testing.T) {
		const src = `fn f() { return 1; }; let probe = f();`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !got.Equals(types.Any) {
			t.Fatalf("got %s, want Any", got.String())
		}
	})

	t.Run("native FFI boundary", func(t *testing.T) {
		const src = `let probe = native("x");`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !got.Equals(types.Any) {
			t.Fatalf("got %s, want Any", got.String())
		}
	})

	t.Run("tensor builtin pending Tensor type", func(t *testing.T) {
		const src = `let probe = tensorSum([1, 2]);`
		c, ok := checkSource(t, src)
		if !ok {
			t.Fatalf("valid source rejected: %v", c.Diagnostics)
		}
		if got := probeVar(t, c, src); !got.Equals(types.Any) {
			t.Fatalf("got %s, want Any", got.String())
		}
	})
}

func TestSequenceBuiltinsRejectNonArray(t *testing.T) {
	for _, tc := range []struct{ name, src string }{
		{"first of Int", `first(5);`},
		{"last of String", `last("s");`},
		{"rest of Int", `rest(5);`},
		{"push on Int", `push(5, 1);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, ok := checkSource(t, tc.src)
			if ok {
				t.Fatal("non-array receiver was accepted")
			}
			for _, d := range c.Diagnostics {
				if d.Code == CodeNotNarrowable {
					return
				}
			}
			t.Fatalf("expected %s, diagnostics=%v", CodeNotNarrowable, c.Diagnostics)
		})
	}

	t.Run("undefined receiver is E0102 not E0112", func(t *testing.T) {
		const src = `first(nosuchvar);`
		c, ok := checkSource(t, src)
		if ok {
			t.Fatal("undefined receiver was accepted")
		}
		var sawE0102, sawE0112 bool
		for _, d := range c.Diagnostics {
			sawE0102 = sawE0102 || d.Code == "E0102"
			sawE0112 = sawE0112 || d.Code == CodeNotNarrowable
		}
		if !sawE0102 || sawE0112 {
			t.Fatalf("want E0102 without E0112, diagnostics=%v", c.Diagnostics)
		}
	})

	t.Run("valid array call stays clean", func(t *testing.T) {
		const src = `let xs = [1]; first(xs);`
		if c, ok := checkSource(t, src); !ok {
			t.Fatalf("valid call rejected: %v", c.Diagnostics)
		}
	})
}

package tests

import (
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
)

// Traits and implementations.
//
// # What is being tested
//
// `impl` lowers to ordinary functions and a rewritten call, so a trait reaches
// both engines through exactly the path a plain function takes. Every
// behavioural case therefore runs in both, and every rejection case is checked
// for the diagnostic that names it — a program that is refused for the wrong
// reason passes a test that only checks "it was refused".
//
// # What is deliberately not here
//
// Static methods (`Type.new()`) and generic traits. A static method needs a
// namespace form the language has no syntax for, and generics need a type system
// that can constrain them; both are L2 work.

// TestTraitMethodCall covers the shapes a method call takes.
func TestTraitMethodCall(t *testing.T) {
	cases := []struct {
		name   string
		source string
		want   string
	}{
		{
			name: "a method returns its value",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				let d = Dog { name: "Rex" };
				let result = d.greet();`,
			want: "Woof",
		},
		{
			name: "two types implement one trait differently",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				struct Cat { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				impl Greeter for Cat { fn greet(self) -> String { return "Meow"; } }
				let result = Dog { name: "Rex" }.greet() + "/" + Cat { name: "Tom" }.greet();`,
			want: "Woof/Meow",
		},
		{
			name: "self names the receiver",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof " + self.name; } }
				let result = Dog { name: "Rex" }.greet();`,
			want: "Woof Rex",
		},
		{
			name: "a method may take arguments",
			source: `trait Greeter { fn greet(self, loud: String) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self, loud: String) -> String { return loud + " " + self.name; } }
				let result = Dog { name: "Rex" }.greet("Hi");`,
			want: "Hi Rex",
		},
		{
			name: "a trait may declare several methods",
			source: `trait Sized { fn width(self) -> Int; fn area(self) -> Int }
				struct Box { w: Int, h: Int }
				impl Sized for Box {
					fn width(self) -> Int { return self.w; }
					fn area(self) -> Int { return self.w * self.h; }
				}
				let b = Box { w: 3, h: 4 };
				let result = "\(b.width())x\(b.area())";`,
			want: "3x12",
		},
		{
			name: "a method may call another method on the receiver",
			source: `trait Chain { fn first(self) -> Int; fn twice(self) -> Int }
				struct N { v: Int }
				impl Chain for N {
					fn first(self) -> Int { return self.v; }
					fn twice(self) -> Int { return self.first() * 2; }
				}
				let result = N { v: 5 }.twice();`,
			want: "10",
		},
		{
			name: "an impl may be written before its trait",
			source: `struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				trait Greeter { fn greet(self) -> String }
				let result = Dog { name: "Rex" }.greet();`,
			want: "Woof",
		},
		{
			name: "a method result may be used in an expression",
			source: `trait Sized { fn area(self) -> Int }
				struct Box { w: Int, h: Int }
				impl Sized for Box { fn area(self) -> Int { return self.w * self.h; } }
				let b = Box { w: 2, h: 3 };
				let total = 0;
				for box in [b, b] { total = total + box.area(); }
				let result = total;`,
			want: "12",
		},
		{
			name: "a trait declaration alone produces no value",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				let result = 1;`,
			want: "1",
		},
		{
			name: "self is an ordinary name outside a method",
			source: `let self = 7;
				let result = self;`,
			want: "7",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := evalSource(t, tc.source)
			if !res.OK {
				t.Fatalf("source was rejected: %s", res.ErrSummary())
			}
			for label, out := range map[string]string{
				"evaluator": mustEvaluatorEval(t, res.Program, "result"),
				"VM":        mustVMEval(t, res.Program, "result"),
			} {
				if out != tc.want {
					t.Errorf("%s produced %q, want %q", label, out, tc.want)
				}
			}
		})
	}
}

// TestTraitErrors covers the rejections, by the code they report.
//
// Every case names the code because "it was refused" is a much weaker statement
// than "it was refused for the reason a developer would need to hear".
func TestTraitErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		code   string
		inText string
	}{
		{
			name: "an impl must provide every declared method",
			source: `trait Greeter { fn greet(self) -> String; fn farewell(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }`,
			code:   "E0601",
			inText: "farewell",
		},
		{
			name: "a method must take the parameters the trait declares",
			source: `trait Greeter { fn greet(self, loud: String) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }`,
			code: "E0601",
		},
		{
			name: "an impl may not add a method the trait does not declare",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog {
					fn greet(self) -> String { return "Woof"; }
					fn fly(self) -> String { return "no"; }
				}`,
			code:   "E0602",
			inText: "fly",
		},
		{
			name: "a type has one impl",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				impl Greeter for Dog { fn greet(self) -> String { return "Wuff"; } }`,
			code:   "E0602",
			inText: "already implements",
		},
		{
			name:   "a trait may not declare the same method twice",
			source: `trait Greeter { fn greet(self) -> String; fn greet(self) -> String }`,
			code:   "E0601",
			inText: "twice",
		},
		{
			name: "the impl must name a declared trait",
			source: `struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }`,
			code:   "E0603",
			inText: "not a declared trait",
		},
		{
			name: "a call needs an impl on the receiver's type",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				let d = Dog { name: "Rex" };
				let result = d.greet();`,
			code:   "E0603",
			inText: "no `impl` block",
		},
		{
			name: "a call must name a method the impl provides",
			source: `trait Greeter { fn greet(self) -> String }
				struct Dog { name: String }
				impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
				let d = Dog { name: "Rex" };
				let result = d.fly();`,
			code:   "E0603",
			inText: "does not provide",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "traits.nil"})
			if res.OK {
				t.Fatal("expected the program to be rejected")
			}
			joined := ""
			for _, d := range res.Diagnostics {
				joined += d.Code + " " + d.Message + "\n"
			}
			if !strings.Contains(joined, tc.code) {
				t.Fatalf("expected %s, got:\n%s", tc.code, joined)
			}
			if tc.inText != "" && !strings.Contains(joined, tc.inText) {
				t.Fatalf("expected the message to mention %q, got:\n%s", tc.inText, joined)
			}
		})
	}
}

// TestTraitSyntaxErrors covers the forms the parser refuses outright, before any
// type is known.
func TestTraitSyntaxErrors(t *testing.T) {
	cases := []struct {
		name   string
		source string
		inText string
	}{
		{"impl without for", `impl Greeter Dog { fn greet(self) -> String { return "x"; } }`, "for"},
		{"trait method with a body", `trait Greeter { fn greet(self) -> String { return "x"; } }`, "has a body"},
		{"impl method with no body", `trait Greeter { fn greet(self) -> String } impl Greeter for Dog { fn greet(self) -> String }`, "no body"},
		{"a statement in a trait body", `trait Greeter { let x = 1; }`, "only `fn` signatures"},
		{"an empty trait", `trait Greeter { }`, "declares no methods"},
		{"a duplicate method in one impl", `trait Greeter { fn greet(self) -> String }
			struct Dog { name: String }
			impl Greeter for Dog {
				fn greet(self) -> String { return "a"; }
				fn greet(self) -> String { return "b"; }
			}`, "twice"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := frontend.Check(tc.source, frontend.Options{Filename: "traits.nil"})
			if len(res.ParseErrors) == 0 && res.OK {
				t.Fatal("expected the source to be rejected")
			}
			joined := strings.Join(res.ParseErrors, "\n")
			for _, d := range res.Diagnostics {
				joined += "\n" + d.Message
			}
			if !strings.Contains(joined, tc.inText) {
				t.Fatalf("expected a message mentioning %q, got:\n%s", tc.inText, joined)
			}
		})
	}
}

// TestMethodNameDoesNotLeak pins the one guarantee the mangling buys.
//
// The generated name is not a name a developer can write, so a hand-written
// function cannot shadow an implementation and be silently used in its place.
// A test that only checked the value would pass even if the mangling were
// ordinary text.
func TestMethodNameDoesNotLeak(t *testing.T) {
	source := `trait Greeter { fn greet(self) -> String }
		struct Dog { name: String }
		impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
		fn Dog.greet(self) { return "not the impl"; }
		let result = Dog { name: "Rex" }.greet();`

	res := evalSource(t, source)
	if res.OK {
		// If a `Dog.greet` declaration turns out to be legal, the important thing
		// is still that it is not what the call resolves to.
		if got := mustEvaluatorEval(t, res.Program, "result"); got != "Woof" {
			t.Fatalf("call resolved to the hand-written function: %s", got)
		}
		if got := mustVMEval(t, res.Program, "result"); got != "Woof" {
			t.Fatalf("call resolved to the hand-written function: %s", got)
		}
		return
	}

	joined := ""
	for _, d := range res.Diagnostics {
		joined += d.Message + "\n"
	}
	if !strings.Contains(joined, "expected next token to be IDENT") &&
		!strings.Contains(joined, "prefix parse function") {
		t.Fatalf("rejected for the wrong reason:\n%s", joined)
	}
}

// TestTraitLoweringIsTransparent pins that a method call becomes an ordinary
// call. If the lowering left anything trait-shaped in the tree, a backend that
// knows nothing about traits would be the first thing to break — silently.
func TestTraitLoweringIsTransparent(t *testing.T) {
	source := `trait Greeter { fn greet(self) -> String }
		struct Dog { name: String }
		impl Greeter for Dog { fn greet(self) -> String { return "Woof"; } }
		let d = Dog { name: "Rex" };
		let result = d.greet();`

	res := evalSource(t, source)
	if !res.OK {
		t.Fatalf("source was rejected: %s", res.ErrSummary())
	}
	if res.Lowered == nil {
		t.Fatal("no lowered program")
	}

	var sawImpl, sawTrait ast.Node
	ast.Inspect(res.Lowered, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.ImplDecl:
			sawImpl = node
		case *ast.TraitDecl:
			sawTrait = node
		}
		return true
	})

	if sawImpl != nil {
		t.Error("an `impl` block survived lowering; backends do not understand it")
	}
	if sawTrait != nil {
		t.Error("a `trait` declaration survived lowering; backends do not understand it")
	}
}

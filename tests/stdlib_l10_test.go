package tests

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/ast"
	"github.com/joysriramsarkar/nilLang/compiler/compiler"
	"github.com/joysriramsarkar/nilLang/compiler/evaluator"
	"github.com/joysriramsarkar/nilLang/compiler/frontend"
	"github.com/joysriramsarkar/nilLang/compiler/object"
	"github.com/joysriramsarkar/nilLang/compiler/vm"
	pkgcompiler "github.com/joysriramsarkar/nilLang/pkg/compiler"
)

// L10 conformance: every new or extended standard-library module must agree
// across the tree-walking evaluator and the bytecode VM.
//
// # Why one fixture for all of these
//
// A stdlib module is a program like any other once imported: the evaluator
// runs its lowered form and the VM runs an inlined compilation of the same
// source. If the two engines disagree on a module, the same application
// behaves differently under `nil run` and `nil run -vm`, which is exactly the
// class of bug the dual-engine suite exists to catch.

type stdlibFixture struct {
	dir     string
	global  string
	program *ast.Program
}

func newStdlibFixture(t *testing.T, global, entry string) *stdlibFixture {
	t.Helper()

	dir := t.TempDir()
	res := frontend.Check(entry, frontend.Options{Filename: "main.nil", SourceDir: dir})
	if len(res.ParseErrors) > 0 {
		t.Fatalf("parse errors: %v", res.ParseErrors)
	}
	if !res.OK {
		msgs := make([]string, 0, len(res.Diagnostics))
		for _, d := range res.Diagnostics {
			msgs = append(msgs, d.Message)
		}
		t.Fatalf("type errors: %s", strings.Join(msgs, "; "))
	}

	program := res.Program
	if res.Lowered != nil {
		program = res.Lowered
	}
	return &stdlibFixture{dir: dir, global: global, program: program}
}

func (f *stdlibFixture) run(t *testing.T, engine string) (string, error) {
	t.Helper()

	switch engine {
	case "evaluator":
		evaluator.ClearModuleCache()
		evaluator.PushScriptDir(f.dir)
		defer evaluator.PopScriptDir()

		env := object.NewEnvironment()
		for builtinName, builtin := range evaluator.Builtins {
			env.Set(builtinName, builtin)
		}
		if evaluated := evaluator.Eval(f.program, env); evaluated != nil {
			if errObj, ok := evaluated.(*object.Error); ok {
				return "", fmt.Errorf("%s", errObj.Inspect())
			}
		}
		bound, ok := env.Get(f.global)
		if !ok || bound == nil {
			return "", fmt.Errorf("global %q not found", f.global)
		}
		return bound.Inspect(), nil

	case "VM":
		comp := compiler.New()
		pkgcompiler.ConfigureCompiler(comp, f.dir)
		if err := comp.Compile(f.program); err != nil {
			return "", err
		}
		bytecode := comp.Bytecode()

		slot := -1
		for i, name := range bytecode.GlobalNames {
			if name == f.global {
				slot = i
				break
			}
		}
		if slot < 0 {
			return "", fmt.Errorf("global %q not found; slots: %v", f.global, bytecode.GlobalNames)
		}
		machine := vm.New(bytecode)
		if err := machine.Run(); err != nil {
			return "", err
		}
		bound := machine.GetGlobal(slot)
		if bound == nil {
			return "", fmt.Errorf("global %q (slot %d) is unbound after Run", f.global, slot)
		}
		return bound.Inspect(), nil

	default:
		t.Fatalf("unknown engine %q", engine)
		return "", nil
	}
}

// checkBothEngines runs entry in both engines and requires the named global
// to inspect to want in each.
func checkBothEngines(t *testing.T, entry, global, want string) {
	t.Helper()

	f := newStdlibFixture(t, global, entry)
	for _, engine := range []string{"evaluator", "VM"} {
		out, err := f.run(t, engine)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		if out != want {
			t.Errorf("%s produced %q, want %q", engine, out, want)
		}
	}
}

func TestStdlibCoreExtended(t *testing.T) {
	checkBothEngines(t, `
import "std/core" as core;
let doubled = core.mapResult(core.ok(3), fn(x) { return x * 2; });
let chained = core.andThen(doubled, fn(x) { return core.ok(x + 4); });
let kept = core.mapResult(core.err("boom"), fn(x) { return x * 2; });
let conv = core.okOr(core.some(7), "missing");
let convNone = core.okOr(core.none(), "missing");
let piped = core.pipeline(3, [fn(x) { return x + 1; }, fn(x) { return x * 10; }]);
let clamped = core.clamp(150, 0, 100);
let swapped = core.swap(1, 2);
let result = str(unwrap(chained)) + "|" + str(isErr(kept)) + "|" + str(unwrap(conv)) + "|" + str(isErr(convNone)) + "|" + str(piped) + "|" + str(clamped) + "|" + str(swapped[0]) + str(swapped[1]);
`, "result", "10|true|7|true|40|100|21")
}

func TestStdlibCoreOptionHelpers(t *testing.T) {
	checkBothEngines(t, `
import "std/core" as core;
let mapped = core.mapOption(core.some(5), fn(x) { return x + 1; });
let stayed = core.mapOption(core.none(), fn(x) { return x + 1; });
let dflt = core.unwrapOrDefault(core.none(), "অতিথি");
let exp = core.expectOption(core.some("রহিম"), "হারানো উচিত নয়");
let expr = core.expectResult(core.ok(9), "হারানো উচিত নয়");
let empty1 = core.isEmptyValue(null);
let empty2 = core.isEmptyValue("");
let empty3 = core.isEmptyValue([1]);
let result = str(unwrap(mapped)) + "|" + str(isNone(stayed)) + "|" + dflt + "|" + exp + "|" + str(expr) + "|" + str(empty1) + str(empty2) + str(empty3);
`, "result", "6|true|অতিথি|রহিম|9|truetruefalse")
}

func TestStdlibIOPath(t *testing.T) {
	checkBothEngines(t, `
import "std/io/path" as path;
let j = path.joinPath("docs", "book/প্রথম.md");
let b = path.basename("a/b/c.txt");
let d = path.dirname("a/b/c.txt");
let e = path.extName("archive.tar.gz");
let w = path.withoutExt("a/b/c.txt");
let abs1 = path.isAbsolute("/x/y");
let abs2 = path.isAbsolute("x/y");
let all = path.joinAll(["a", "b", "c"]);
let result = j + "|" + b + "|" + d + "|" + e + "|" + w + "|" + str(abs1) + str(abs2) + "|" + all;
`, "result", "docs/book/প্রথম.md|c.txt|a/b|.gz|a/b/c|truefalse|a/b/c")
}

func TestStdlibIOLines(t *testing.T) {
	dir := t.TempDir()
	file := filepath.ToSlash(filepath.Join(dir, "lines.txt"))
	copyDst := filepath.ToSlash(filepath.Join(dir, "copy.txt"))
	entry := fmt.Sprintf(`
import "std/io" as io;
io.writeLines(%q, ["শূন্য", "থেকে", "নীলাং"]);
io.appendText(%q, "\nচতুর্থ");
let lines = io.readLines(%q);
io.copyText(%q, %q);
let back = io.readText(%q);
let result = join(lines, "+") + "|" + str(len(lines)) + "|" + back;
`, file, file, file, file, copyDst, copyDst)
	f := newStdlibFixture(t, "result", entry)
	for _, engine := range []string{"evaluator", "VM"} {
		out, err := f.run(t, engine)
		if err != nil {
			t.Fatalf("%s: %v", engine, err)
		}
		want := "শূন্য+থেকে+নীলাং+চতুর্থ|4|শূন্য\nথেকে\nনীলাং\nচতুর্থ"
		if out != want {
			t.Errorf("%s produced %q, want %q", engine, out, want)
		}
	}
}

func TestStdlibIOJSONCSV(t *testing.T) {
	dir := t.TempDir()
	jfile := filepath.ToSlash(filepath.Join(dir, "data.json"))
	cfile := filepath.ToSlash(filepath.Join(dir, "data.csv"))
	checkBothEngines(t, fmt.Sprintf(`
import "std/io" as io;
io.writeJson(%q, {"ভাষা": "নীলাং", "সংস্করণ": 2});
let back = io.readJson(%q);
io.writeCsv(%q, [["a", "b"], ["c", "d"]]);
let rows = io.readCsv(%q);
let result = back["ভাষা"] + str(back["সংস্করণ"]) + "|" + rows[0][0] + rows[1][1] + str(len(rows));
`, jfile, jfile, cfile, cfile), "result", "নীলাং2|ad2")
}

func TestStdlibURL(t *testing.T) {
	checkBothEngines(t, `
import "std/net/url" as url;
let u = "https://nilang.org:8080/docs/শুরু?lang=bn&n=2#অংশ";
let rt = url.decode(url.encode("শূন্য থেকে নীলাং"));
let q = url.buildQuery({"b": "2", "a": "1"});
let v = url.queryValue("a=1&b=2", "b");
let missing = url.queryValue("a=1", "z");
let full = url.withQuery("https://x.org/s", {"k": "v v"});
let result = url.scheme(u) + "|" + url.host(u) + "|" + url.path(u) + "|" + url.query(u) + "|" + url.fragment(u) + "|" + rt + "|" + q + "|" + v + "|" + str(missing) + "|" + full;
`, "result", "https|nilang.org:8080|/docs/শুরু|lang=bn&n=2|অংশ|শূন্য থেকে নীলাং|a=1&b=2|2|null|https://x.org/s?k=v+v")
}

func TestStdlibCryptoBasics(t *testing.T) {
	checkBothEngines(t, `
import "std/crypto" as crypto;
let h = crypto.sha256("test-data");
let h512 = crypto.sha512("test-data");
let sig = crypto.hmacSha256("my-secret", "payload");
let okv = crypto.hmacVerify("my-secret", "payload", sig);
let bad = crypto.hmacVerify("other", "payload", sig);
let r = crypto.randomBytes(32);
let k32 = crypto.randomKey32();
let n12 = crypto.randomNonce12();
let sealed = crypto.aesEncrypt(k32, n12, "গোপন বার্তা");
let opened = crypto.aesDecrypt(k32, n12, sealed);
let kp = crypto.edKeypair();
let signature = crypto.edSign(kp["private_key"], "চুক্তি v1");
let valid = crypto.edVerify(kp["public_key"], "চুক্তি v1", signature);
let invalid = crypto.edVerify(kp["public_key"], "জাল", signature);
let b64 = crypto.base64Encode("hello");
let result = str(len(h)) + str(len(h512)) + str(okv) + str(bad) + str(len(r)) + str(len(k32)) + str(len(n12)) + opened + str(valid) + str(invalid) + crypto.base64Decode(b64);
`, "result", "64128truefalse444416গোপন বার্তাtruefalsehello")
}

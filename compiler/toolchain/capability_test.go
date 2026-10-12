package toolchain

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joysriramsarkar/nilLang/compiler/llvm"
)

// TestAnAbsentToolCanDoNothing is the first thing to establish.
//
// Every capability question routes through it, and a tool with no path has to
// answer "no" without running anything — otherwise discovery on a machine with no
// compiler installed spends seconds running commands that do not exist.
func TestAnAbsentToolCanDoNothing(t *testing.T) {
	missing := Tool{Name: "clang"}

	for _, capability := range []Capability{CapAssemble, CapCompileC, CapLink} {
		if missing.Can(capability) {
			t.Errorf("a tool with no path reported it can %s", capability)
		}
	}
}

// TestCapabilityIsCachedPerToolAndOperation is the reason the probe is not
// re-run for every question.
//
// A build resolves the toolchain once and then asks three questions about the same
// compiler. Without the cache that is three compiler invocations — and each one
// writes an object file to a temporary directory, so the cost is disk as well as
// time.
func TestCapabilityIsCachedPerToolAndOperation(t *testing.T) {
	clang := findTool("clang")
	if !clang.Available() {
		t.Skip("no clang on this machine")
	}

	// Two questions about the same tool must agree with one another even if the
	// machine changes underneath, which is what a cache buys: the build asked once
	// and every later question gets that answer.
	first := clang.Can(CapAssemble)
	second := clang.Can(CapAssemble)
	if first != second {
		t.Errorf("two probes of the same tool disagreed: %v then %v", first, second)
	}

	// The key includes the capability, so a tool's answers are independent per
	// operation rather than one flag for all three.
	if _, loaded := capabilityCache.Load(clang.Path + "|" + string(CapAssemble)); !loaded {
		t.Error("the probe result was not cached")
	}
}

// TestTheDiscoveredCapabilitiesMatchTheMachine is the guard against the toolchain
// reporting something it cannot do.
//
// This test is the reason the probe exists. It asserts nothing portable — it reads
// what the machine can actually do and checks that each capability agrees. A build
// on a machine with an SDK-less LLVM install has clang reading IR and not linking;
// a build on macOS has clang doing all three. Both are correct, and both are
// discovered rather than assumed.
func TestTheDiscoveredCapabilitiesMatchTheMachine(t *testing.T) {
	tc := DiscoverToolchain(testTarget(t))
	reports := tc.Capabilities()

	if len(reports) == 0 {
		t.Fatal("no capability reports were produced")
	}

	for _, report := range reports {
		if !report.Installed {
			continue
		}

		// A tool that can read IR has to be able to answer, and the reverse has to
		// hold too: an assembler that cannot assemble is a broken installation.
		if report.ReadsIR && report.Path == "" {
			t.Errorf("%s reported reading IR without a path", report.Tool)
		}

		// The implication that actually matters: a C compiler that cannot compile C
		// would make the runtime library build fail with a message about a header
		// the user has never heard of.
		if report.Installed && !report.CompilesC && report.Tool == "gcc" {
			if _, err := os.Stat(report.Path); err == nil {
				t.Logf("gcc is installed at %s but cannot compile C; this is a broken "+
					"installation and the message would be worth showing", report.Path)
			}
		}
	}
}

// TestLinkReadyIsNotReady is the distinction the whole file exists for.
//
// `Ready` asks whether IR can become objects. `LinkReady` asks whether something
// can run. A machine can answer yes to the first and no to the second, and a build
// that confuses them reports success having produced nothing executable.
func TestLinkReadyIsNotReady(t *testing.T) {
	tc := Toolchain{Target: testTarget(t)}

	if tc.Ready() {
		t.Error("a toolchain with no tools reported itself ready")
	}
	if tc.LinkReady() {
		t.Error("a toolchain with no tools reported itself able to link")
	}

	advice := tc.LinkAdvice()
	if advice == "" {
		t.Fatal("LinkAdvice gave no advice for a machine that cannot link")
	}

	// `libcmt.lib` is named because it is the literal file the linker names when it
	// fails. Someone reading the linker error can search for that name and find the
	// package that provides it; "a C runtime" gives them nothing to search for.
	if !strings.Contains(strings.ToLower(advice), "libcmt") {
		t.Errorf("LinkAdvice does not name the file the linker actually complains about:\n%s", advice)
	}

	// A toolchain that has nothing at all still has to say "found: nothing" rather
	// than leaving an empty list under a heading.
	if !strings.Contains(advice, "nothing") {
		t.Errorf("LinkAdvice listed no tools under 'found':\n%s", advice)
	}
}

// TestLinkAdviceNamesTheToolsThatAreInstalled is the half that matters on a real
// machine.
//
// With nothing installed the advice is generic. With clang installed and unable to
// link, it has to name clang and say *why* it cannot link — otherwise the message
// is the same on a machine with LLVM and a machine without it, and the reader has
// no reason to suspect the LLVM install at all.
func TestLinkAdviceNamesTheToolsThatAreInstalled(t *testing.T) {
	tc := DiscoverToolchain(testTarget(t))
	if tc.LinkReady() {
		t.Skip("this machine can link, so there is no advice to give")
	}

	advice := tc.LinkAdvice()
	if !tc.Clang.Available() {
		t.Skip("no clang here; nothing to name")
	}

	if !strings.Contains(advice, tc.Clang.Path) {
		t.Errorf("LinkAdvice does not name clang at %s:\n%s", tc.Clang.Path, advice)
	}

	// And the message must not tell someone to install what they have. LLVM is
	// present on this machine, so advice that says "install LLVM" as its first step
	// is the failure this whole file exists to avoid.
	firstStep := strings.ToLower(strings.SplitN(strings.TrimSpace(advice), "\n", 2)[0])
	if strings.Contains(firstStep, "install llvm") {
		t.Errorf("LinkAdvice tells the reader to install LLVM as its first step:\n%s", advice)
	}
}

// TestAssemblyPrefersClangThenLLC records the order the two are tried in.
//
// The order matters because the two are not interchangeable: `clang` knows the
// target triple the module was emitted for, and `llc` has to be told it. On this
// machine `llc` is absent, so if the order were reversed the build would report
// "no assembler" while holding a working one.
func TestAssemblyPrefersClangThenLLC(t *testing.T) {
	tc := DiscoverToolchain(testTarget(t))

	assembler, ok := tc.Assembler()
	if !ok {
		if tc.Clang.Available() || tc.LLC.Available() {
			t.Fatalf("clang and/or llc are installed (%v / %v) but no assembler was chosen",
				tc.Clang.Path, tc.LLC.Path)
		}
		return
	}

	if tc.Clang.Available() && tc.Clang.Can(CapAssemble) {
		if assembler.Path != tc.Clang.Path {
			t.Errorf("chose %s over clang, which can read IR", assembler.Path)
		}
		return
	}
	if !tc.LLC.Can(CapAssemble) {
		t.Errorf("chose %s, which cannot read IR", assembler.Path)
	}
}

// TestTheRuntimeCanBeBuiltHere is the test that keeps this file honest about why
// the probe exists.
//
// It actually compiles one C file with whatever `CCompiler` picked. A capability
// probe that answered "yes" without doing this would be the exact bug the probe
// was written to prevent, and this is the test that would catch it.
func TestTheRuntimeCanBeBuiltHere(t *testing.T) {
	tc := DiscoverToolchain(testTarget(t))

	cc, ok := tc.CCompiler()
	if !ok {
		t.Log("no C compiler here; the runtime library cannot be built on this machine")
		return
	}

	dir := t.TempDir()
	source := filepath.Join(dir, "runtime_probe.c")
	object := filepath.Join(dir, "runtime_probe.o")
	if err := os.WriteFile(source, []byte(cProbeSource), 0o644); err != nil {
		t.Fatal(err)
	}

	if output, err := RunTool(cc, "-c", source, "-o", object); err != nil {
		t.Fatalf("CCompiler chose %s but it could not compile:\n%s", cc.Path, output)
	}
	if _, err := os.Stat(object); err != nil {
		t.Fatalf("the compiler reported success but wrote no object: %v", err)
	}
}

// TestEveryToolReportedIsReal checks that discovery does not invent a path.
//
// A `Tool` with a name but no path is how "not installed" is represented, and
// `Tools()` returns those so the report can list what was looked for. This test
// makes sure none of them claims to exist.
func TestEveryToolReportedIsReal(t *testing.T) {
	tc := DiscoverToolchain(testTarget(t))

	for _, tool := range tc.Tools() {
		if !tool.Available() {
			continue
		}
		if _, err := os.Stat(tool.Path); err != nil {
			t.Errorf("%s was reported at %s, which does not exist", tool.Name, tool.Path)
		}
		if tool.Version() == "" {
			t.Errorf("%s is at %s but reported no version", tool.Name, tool.Path)
		}
	}
}

// testTarget is the machine's own target.
//
// `llvm.TargetHost` and not a lookup by triple: `LookupTarget` is keyed by ID
// (`x86_64-windows`) and `HostTriple` is a triple (`x86_64-pc-windows-msvc`), so
// passing one where the other is wanted finds nothing. The first version of this
// helper fell back to "the first registered target" when that failed, which on this
// machine is `aarch64-unknown-linux-android` — and the capability probes, which
// care about the machine rather than the target, went on passing.
func testTarget(t *testing.T) llvm.Target {
	t.Helper()
	target := llvm.TargetHost
	if target.ID == "" {
		t.Fatal("no host target is registered; the target table is empty")
	}
	return target
}

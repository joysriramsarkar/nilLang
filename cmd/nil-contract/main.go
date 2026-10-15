// Command nil-contract generates and verifies the Onuron ecosystem contract
// bundle.
//
//	nil-contract generate   rewrite generated files and the bundle index
//	nil-contract verify     check checksums and drift against the Go source
//	nil-contract show FILE  print a contract file
//	nil-contract validate FILE.nilax
//	                        validate a .nilax package against the contract
//
// generate and verify are two halves of one rule: the generated files are a
// projection of the compiler's requirement table, so a projection that has
// fallen behind is a defect, not a stale document.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/contract"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	root, args := parseRoot(os.Args[1:])
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}

	var err error
	switch args[0] {
	case "generate":
		err = cmdGenerate(root)
	case "verify":
		err = cmdVerify(root)
	case "show":
		err = cmdShow(root, args[1:])
	case "validate":
		err = cmdValidate(root, args[1:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "nil-contract: unknown command %q\n\n", args[0])
		usage()
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "nil-contract:", err)
		os.Exit(1)
	}
}

// parseRoot pulls a leading `-root DIR` (or `-root=DIR`) out of the argument
// list so every subcommand sees the same root without re-parsing global flags.
//
// Dispatch happens before flag parsing would otherwise be convenient, and
// threading a *flag.FlagSet through four commands to carry one string is more
// machinery than the string is worth.
func parseRoot(args []string) (string, []string) {
	root := ""
	rest := make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "-root" || args[i] == "--root":
			if i+1 < len(args) {
				root = args[i+1]
				i++
			}
		case strings.HasPrefix(args[i], "-root="):
			root = strings.TrimPrefix(args[i], "-root=")
		case strings.HasPrefix(args[i], "--root="):
			root = strings.TrimPrefix(args[i], "--root=")
		default:
			rest = append(rest, args[i])
		}
	}

	if root == "" {
		root = defaultRoot()
	}
	return root, rest
}

func usage() {
	fmt.Fprint(os.Stderr, `nil-contract — the Onuron ecosystem contract bundle

  nil-contract [-root DIR] generate        rewrite generated files and bundle.json
  nil-contract [-root DIR] verify          check checksums and drift against Go source
  nil-contract [-root DIR] show FILE       print a contract file
  nil-contract validate [-trust] FILE      validate a .nilax package

The root defaults to the repository containing this command's module.
`)
}

func defaultRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for d := dir; ; {
		if _, err := os.Stat(filepath.Join(d, "contracts", "bundle.json")); err == nil {
			return d
		}
		parent := filepath.Dir(d)
		if parent == d {
			return dir
		}
		d = parent
	}
}

func contractsDir(root string) string { return filepath.Join(root, "contracts") }

// cmdGenerate rewrites the generated contract files and the bundle index.
//
// It writes generated files first and the index last, so an interrupted run
// leaves a bundle whose index no longer matches — which verify reports, rather
// than a bundle that looks fine and is not.
func cmdGenerate(root string) error {
	dir := contractsDir(root)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("no contracts directory at %s (use -root)", dir)
	}

	reqs, err := contract.BuildRequirements()
	if err != nil {
		return err
	}
	if err := writeIfChanged(filepath.Join(dir, contract.PathRequirements), reqs); err != nil {
		return err
	}

	vectors, err := contract.BuildSigningVectors()
	if err != nil {
		return err
	}
	if err := writeIfChanged(filepath.Join(dir, contract.PathVectors), vectors); err != nil {
		return err
	}

	entries, err := scanContractDir(dir)
	if err != nil {
		return err
	}
	index, err := contract.BuildIndex(entries)
	if err != nil {
		return err
	}
	if err := writeIfChanged(filepath.Join(dir, contract.PathIndex), index); err != nil {
		return err
	}

	fmt.Printf("generated %s, %s and %s (%d files indexed)\n",
		contract.PathRequirements, contract.PathVectors, contract.PathIndex, len(entries))
	return nil
}

// cmdVerify checks the bundle against itself and against the Go source.
//
// Both checks matter. A checksum mismatch means a file was edited without the
// index being regenerated; a drift mismatch means the index is current but the
// compiler moved. The second is the one that lets a contract quietly stop
// describing the thing it exists to describe.
func cmdVerify(root string) error {
	dir := contractsDir(root)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("no contracts directory at %s (use -root)", dir)
	}

	var problems []string

	entries, err := scanContractDir(dir)
	if err != nil {
		return err
	}
	indexBytes, err := contract.BuildIndex(entries)
	if err != nil {
		return err
	}
	onDisk, err := os.ReadFile(filepath.Join(dir, contract.PathIndex))
	if err != nil {
		return err
	}
	if string(onDisk) != string(indexBytes) {
		problems = append(problems, fmt.Sprintf("%s is stale: run `nil-contract generate`", contract.PathIndex))
	}

	reqs, err := contract.BuildRequirements()
	if err != nil {
		return err
	}
	onDiskReqs, err := os.ReadFile(filepath.Join(dir, contract.PathRequirements))
	if err != nil {
		return err
	}
	if string(onDiskReqs) != string(reqs) {
		problems = append(problems, fmt.Sprintf("%s does not match compiler/capability: run `nil-contract generate`", contract.PathRequirements))
	}

	vectors, err := contract.BuildSigningVectors()
	if err != nil {
		return err
	}
	onDiskVectors, err := os.ReadFile(filepath.Join(dir, contract.PathVectors))
	if err != nil {
		return err
	}
	if string(onDiskVectors) != string(vectors) {
		problems = append(problems, fmt.Sprintf("%s does not match pkg/signing: run `nil-contract generate`", contract.PathVectors))
	}

	if len(problems) > 0 {
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, "  -", p)
		}
		return fmt.Errorf("contract bundle is out of date")
	}

	fmt.Printf("contract bundle %s verified: %d files, checksums and drift both clean\n",
		contract.BundleVersion, len(entries))
	return nil
}

func cmdShow(root string, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("show needs a file path, for example `nil-contract show capabilities/vocabulary.json`")
	}
	data, err := os.ReadFile(filepath.Join(contractsDir(root), filepath.FromSlash(args[0])))
	if err != nil {
		return err
	}
	_, err = os.Stdout.Write(data)
	return err
}

func cmdValidate(root string, args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	var trust bool
	var abi string
	fs.BoolVar(&trust, "trust", false, "require a trusted signature")
	fs.StringVar(&abi, "runtime-abi", "", "ABI this verifier implements")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return fmt.Errorf("validate needs a .nilax path")
	}

	set, err := contract.Load()
	if err != nil {
		return err
	}

	failed := false
	for _, pkg := range fs.Args() {
		manifest, entries, err := contract.OpenZipManifest(pkg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", pkg, err)
			failed = true
			continue
		}
		digest, _ := contract.FileDigest(pkg)
		opts := contract.ValidateOptions{
			Archive: contract.FileArchive(entries),
		}
		if trust {
			opts.Trust = contract.TrustRequired
		}
		if abi != "" {
			opts.RuntimeABI = abi
		}
		vs := set.Validate(manifest, opts)
		if vs.OrNil() != nil {
			fmt.Printf("%s (%s)\n", pkg, strings.TrimSpace(digest))
			for _, v := range vs {
				fmt.Printf("  %s\n", v.Error())
			}
			failed = true
			continue
		}
		fmt.Printf("%s: OK (%s, %d entries, format_version %d)\n",
			pkg, digest[:16], len(entries), manifest.FormatVersion)
	}
	if failed {
		return fmt.Errorf("one or more packages failed contract validation")
	}
	return nil
}

func writeIfChanged(path string, data []byte) error {
	if existing, err := os.ReadFile(path); err == nil && string(existing) == string(data) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// generatedFiles are the contract files `generate` owns.
//
// They are listed rather than inferred from a filename pattern so that a new
// hand-written document is not silently overwritten by the next generation, and
// a new generated document is not quietly treated as hand-written.
var generatedFiles = map[string]bool{
	contract.PathRequirements: true,
	contract.PathVectors:      true,
}

// scanContractDir reads every JSON contract file and produces its index entry.
//
// The status is read from each document rather than hard-coded, because a file
// whose declared status disagrees with the index is exactly the kind of drift a
// consumer relies on the index to rule out.
func scanContractDir(dir string) ([]contract.IndexEntry, error) {
	var entries []contract.IndexEntry

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == contract.PathIndex {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var head struct {
			Status string `json:"status"`
		}
		if err := json.Unmarshal(data, &head); err != nil {
			return fmt.Errorf("%s: not a JSON object with a status: %w", rel, err)
		}
		if head.Status == "" {
			return fmt.Errorf("%s: contract files must declare a status of stable, proposed or draft", rel)
		}

		sum := sha256.Sum256(data)
		entries = append(entries, contract.IndexEntry{
			Path:      rel,
			Status:    head.Status,
			SHA256:    hex.EncodeToString(sum[:]),
			Bytes:     len(data),
			Generated: generatedFiles[rel],
		})
		return nil
	})
	if err != nil {
		return nil, err
	}

	contract.SortEntries(entries)
	return entries, nil
}
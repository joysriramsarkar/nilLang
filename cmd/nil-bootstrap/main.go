package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joysriramsarkar/nilLang/bootstrap"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "build":
		err = build(argsOrDefault(2, "bootstrap"), argsOrDefault(3, filepath.Join("build", "nil-compiler.json")))
	case "rebuild":
		err = rebuild(argsOrDefault(2, filepath.Join("build", "nil-compiler.json")), argsOrDefault(3, filepath.Join("build", "nil-compiler.next.json")))
	case "verify":
		err = verify(argsOrDefault(2, "bootstrap"))
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nil-bootstrap:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage:")
	fmt.Fprintln(os.Stderr, "  nil-bootstrap build [source-dir] [output]")
	fmt.Fprintln(os.Stderr, "  nil-bootstrap rebuild [input] [output]")
	fmt.Fprintln(os.Stderr, "  nil-bootstrap verify [source-dir]")
}

func argsOrDefault(index int, fallback string) string {
	if len(os.Args) > index {
		return os.Args[index]
	}
	return fallback
}

func build(sourceDirectory, output string) error {
	image, err := bootstrap.BuildSeed(sourceDirectory)
	if err != nil {
		return err
	}
	if err := writeImage(output, image); err != nil {
		return err
	}
	fmt.Printf("built self-hosted Nil compiler image: %s\n", output)
	return nil
}

func rebuild(input, output string) error {
	image, err := os.ReadFile(input)
	if err != nil {
		return fmt.Errorf("read %s: %w", input, err)
	}
	rebuilt, err := bootstrap.Rebuild(image)
	if err != nil {
		return err
	}
	if err := writeImage(output, rebuilt); err != nil {
		return err
	}
	fmt.Printf("rebuilt self-hosted Nil compiler image: %s\n", output)
	return nil
}

func verify(sourceDirectory string) error {
	stageB, err := bootstrap.BuildSeed(sourceDirectory)
	if err != nil {
		return err
	}
	stageC, err := bootstrap.Rebuild(stageB)
	if err != nil {
		return err
	}
	if !bytes.Equal(stageB, stageC) {
		return fmt.Errorf("stage B and stage C compiler images differ")
	}
	fmt.Println("verified: stage B and stage C compiler images are byte-identical")
	return nil
}

func writeImage(path string, image []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(path, image, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

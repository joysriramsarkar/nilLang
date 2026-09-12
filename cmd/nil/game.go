package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/config"
	"github.com/joysriramsarkar/nilLang/pkg/game"
)

func cmdGame() {
	if len(os.Args) < 3 {
		printGameUsage()
		return
	}

	switch os.Args[2] {
	case "init", "create":
		cmdGameInit(os.Args[3:])
	case "doctor", "check":
		cmdGameDoctor(os.Args[3:])
	case "engines", "targets":
		cmdGameEngines()
	case "help", "-h", "--help":
		printGameUsage()
	default:
		fmt.Fprintf(os.Stderr, "unknown game command: %s\n\n", os.Args[2])
		printGameUsage()
		os.Exit(2)
	}
}

func cmdGameInit(args []string) {
	options := game.DefaultOptions("my-nilang-game")
	var selectedEngines []string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--engine" || arg == "--engines":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--engine needs a value")
				os.Exit(2)
			}
			i++
			selectedEngines = append(selectedEngines, args[i])
		case strings.HasPrefix(arg, "--engine="):
			selectedEngines = append(selectedEngines, strings.TrimPrefix(arg, "--engine="))
		case strings.HasPrefix(arg, "--engines="):
			selectedEngines = append(selectedEngines, strings.TrimPrefix(arg, "--engines="))
		case arg == "--author":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "--author needs a value")
				os.Exit(2)
			}
			i++
			options.Author = args[i]
		case strings.HasPrefix(arg, "--author="):
			options.Author = strings.TrimPrefix(arg, "--author=")
		case !strings.HasPrefix(arg, "-") && options.Name == "my-nilang-game":
			options.Name = arg
		default:
			fmt.Fprintf(os.Stderr, "unknown game init option: %s\n", arg)
			os.Exit(2)
		}
	}

	if len(selectedEngines) > 0 {
		options.Engines = selectedEngines
	}

	if err := game.ValidateProjectName(options.Name); err != nil {
		fmt.Fprintf(os.Stderr, "invalid game project name: %v\n", err)
		os.Exit(2)
	}

	written, err := game.WriteScaffold(options.Name, options)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game scaffold failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Nilang game project created: %s\n", options.Name)
	fmt.Printf("Files written: %d\n", len(written))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", options.Name)
	fmt.Println("  nil game doctor")
	fmt.Println("  nil build android")
}

func cmdGameDoctor(args []string) {
	projectDir := "."
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		projectDir = args[0]
	}

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot resolve project path: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.LoadConfig(absDir)
	if err != nil {
		if len(args) == 0 {
			fmt.Println("ℹ️  বর্তমান ডিরেক্টরিতে কোনো nil.json পাওয়া যায়নি।")
			fmt.Println("ব্যবহার: nil game doctor [game-project-dir]")
			fmt.Println("উদাহরণ: nil game doctor my-nilang-game")
			return
		}
		fmt.Fprintf(os.Stderr, "cannot load nil.json: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(absDir); err != nil {
		fmt.Fprintf(os.Stderr, "configuration error: %v\n", err)
		os.Exit(1)
	}

	report := game.CheckProject(cfg, absDir)
	fmt.Printf("Nilang game readiness: %s\n", cfg.Name)
	for _, check := range report.Checks {
		mark := "OK"
		if !check.OK {
			mark = "FIX"
		}
		fmt.Printf("  [%s] %-20s %s\n", mark, check.Name, check.Detail)
	}
	if !report.Ready {
		fmt.Fprintln(os.Stderr, "Game project is not ready yet.")
		os.Exit(1)
	}
	fmt.Println("Game project is ready for Android APK and engine bundle workflows.")
}

func cmdGameEngines() {
	fmt.Println("Supported game adapters:")
	for _, target := range game.KnownEngines() {
		fmt.Printf("  %-8s %s\n", target.ID, target.Name)
		fmt.Printf("           bundle: %s\n", target.BundleDestination)
	}
}

func printGameUsage() {
	fmt.Println("Usage:")
	fmt.Println("  nil game init [name] [--engine android,godot,unity,unreal|all]")
	fmt.Println("  nil game doctor [project-dir]")
	fmt.Println("  nil game engines")
}

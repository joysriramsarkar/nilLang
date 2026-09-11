package game

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joysriramsarkar/nilLang/pkg/capability"
	"github.com/joysriramsarkar/nilLang/pkg/config"
)

const ProfileID = "game"

const (
	EngineAndroid EngineID = "android"
	EngineGodot   EngineID = "godot"
	EngineUnity   EngineID = "unity"
	EngineUnreal  EngineID = "unreal"
)

type EngineID string

type EngineTarget struct {
	ID                EngineID
	Name              string
	Directory         string
	BundleDestination string
	RuntimeLoader     string
}

type ProjectOptions struct {
	Name    string
	Version string
	Author  string
	Entry   string
	Engines []string
}

type Check struct {
	Name   string
	OK     bool
	Detail string
}

type ReadinessReport struct {
	Ready  bool
	Checks []Check
}

var DefaultCapabilities = []string{"GPU", "Audio", "Filesystem", "Network", "Process", "Sensors", "AI", "Crypto"}
var DefaultBuildTargets = []string{"android", "windows", "linux"}

var engineOrder = []EngineID{EngineAndroid, EngineGodot, EngineUnity, EngineUnreal}

var engineRegistry = map[EngineID]EngineTarget{
	EngineAndroid: {
		ID:                EngineAndroid,
		Name:              "Android APK",
		Directory:         "engine/android",
		BundleDestination: "assets/app.nilax",
		RuntimeLoader:     "Nilang Android runner APK",
	},
	EngineGodot: {
		ID:                EngineGodot,
		Name:              "Godot 4 GDExtension",
		Directory:         "engine/godot",
		BundleDestination: "res://nilang/build/app.nilax",
		RuntimeLoader:     "Godot native extension descriptor",
	},
	EngineUnity: {
		ID:                EngineUnity,
		Name:              "Unity Package",
		Directory:         "engine/unity",
		BundleDestination: "Assets/StreamingAssets/nilang/app.nilax",
		RuntimeLoader:     "Unity C# behaviour and native plugin boundary",
	},
	EngineUnreal: {
		ID:                EngineUnreal,
		Name:              "Unreal Engine Plugin",
		Directory:         "engine/unreal",
		BundleDestination: "Content/Nilang/app.nilax",
		RuntimeLoader:     "Unreal module and plugin descriptor",
	},
}

func DefaultOptions(name string) ProjectOptions {
	if strings.TrimSpace(name) == "" {
		name = "my-nilang-game"
	}
	return ProjectOptions{
		Name:    name,
		Version: "0.1.0",
		Author:  "Joysriram Sarkar",
		Entry:   "src/main.nil",
		Engines: []string{"all"},
	}
}

func KnownEngines() []EngineTarget {
	targets := make([]EngineTarget, 0, len(engineOrder))
	for _, id := range engineOrder {
		targets = append(targets, engineRegistry[id])
	}
	return targets
}

func ResolveEngine(id string) (EngineTarget, bool) {
	normalized := EngineID(strings.ToLower(strings.TrimSpace(id)))
	target, ok := engineRegistry[normalized]
	return target, ok
}

func ResolveEngines(ids []string) ([]EngineTarget, error) {
	if len(ids) == 0 {
		ids = []string{"all"}
	}

	seen := map[EngineID]bool{}
	var targets []EngineTarget
	for _, raw := range ids {
		for _, piece := range strings.Split(raw, ",") {
			id := strings.ToLower(strings.TrimSpace(piece))
			if id == "" {
				continue
			}
			if id == "all" {
				for _, target := range KnownEngines() {
					if !seen[target.ID] {
						seen[target.ID] = true
						targets = append(targets, target)
					}
				}
				continue
			}
			target, ok := ResolveEngine(id)
			if !ok {
				return nil, fmt.Errorf("unknown game engine %q (valid: android, godot, unity, unreal, all)", piece)
			}
			if !seen[target.ID] {
				seen[target.ID] = true
				targets = append(targets, target)
			}
		}
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("no game engines selected")
	}
	return targets, nil
}

func ValidateProjectName(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}
	for i, ch := range name {
		if ch == '-' || ch == '_' {
			continue
		}
		if ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' {
			if i == 0 && ch >= '0' && ch <= '9' {
				return fmt.Errorf("project name cannot start with a number")
			}
			continue
		}
		return fmt.Errorf("project name can only contain letters, numbers, hyphen, and underscore")
	}
	return nil
}

func ScaffoldFiles(options ProjectOptions) (map[string]string, error) {
	if err := ValidateProjectName(options.Name); err != nil {
		return nil, err
	}
	if options.Version == "" {
		options.Version = "0.1.0"
	}
	if options.Author == "" {
		options.Author = "Joysriram Sarkar"
	}
	if options.Entry == "" {
		options.Entry = "src/main.nil"
	}

	targets, err := ResolveEngines(options.Engines)
	if err != nil {
		return nil, err
	}

	engineIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		engineIDs = append(engineIDs, string(target.ID))
	}

	cfg := config.ProjectConfig{
		Name:         options.Name,
		Version:      options.Version,
		Author:       options.Author,
		Description:  "Nilang real-time game logic package",
		Entry:        options.Entry,
		Profile:      ProfileID,
		Capabilities: append([]string(nil), DefaultCapabilities...),
		Targets:      append([]string(nil), DefaultBuildTargets...),
		Resources:    []string{"resources/*"},
		Build: config.BuildConfig{
			OutputDir: "build",
			Optimize:  true,
			Debug:     true,
		},
		Metadata: map[string]string{
			"engine_adapters": strings.Join(engineIDs, ","),
			"runtime_bundle":  "nilax",
		},
		Scripts: map[string]string{
			"dev":          "nil run src/main.nil",
			"build":        "nil build android",
			"bundle":       "nil build",
			"game:doctor":  "nil game doctor",
			"game:android": "nil build android",
		},
	}
	configBytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, err
	}

	gameConfig := map[string]any{
		"name":               options.Name,
		"profile":            ProfileID,
		"tick_rate":          60,
		"bundle":             "build/" + options.Name + "-" + options.Version + ".nilax",
		"engines":            engineIDs,
		"entry_points":       []string{"game_start", "game_update", "game_event"},
		"android_apk_target": "build/" + options.Name + ".apk",
	}
	gameConfigBytes, err := json.MarshalIndent(gameConfig, "", "  ")
	if err != nil {
		return nil, err
	}

	files := map[string]string{
		"nil.json":                   string(configBytes) + "\n",
		options.Entry:                sourceTemplate(options.Name),
		"resources/nilang.game.json": string(gameConfigBytes) + "\n",
		"engine/README.md":           engineReadme(options.Name, targets),
		".gitignore":                 gitignoreTemplate(),
	}

	for _, target := range targets {
		for rel, content := range adapterFiles(options.Name, target) {
			files[rel] = content
		}
	}

	return files, nil
}

func WriteScaffold(root string, options ProjectOptions) ([]string, error) {
	files, err := ScaffoldFiles(options)
	if err != nil {
		return nil, err
	}

	paths := make([]string, 0, len(files))
	for rel := range files {
		paths = append(paths, rel)
	}
	sort.Strings(paths)

	for _, rel := range paths {
		fullPath := filepath.Join(root, rel)
		if _, err := os.Stat(fullPath); err == nil {
			return nil, fmt.Errorf("refusing to overwrite existing file: %s", rel)
		}
	}

	written := make([]string, 0, len(paths))
	for _, rel := range paths {
		fullPath := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return written, err
		}
		if err := os.WriteFile(fullPath, []byte(files[rel]), 0644); err != nil {
			return written, err
		}
		written = append(written, rel)
	}

	return written, nil
}

func CheckProject(cfg *config.ProjectConfig, projectDir string) ReadinessReport {
	report := ReadinessReport{Ready: true}
	add := func(name string, ok bool, detail string) {
		report.Checks = append(report.Checks, Check{Name: name, OK: ok, Detail: detail})
		if !ok {
			report.Ready = false
		}
	}

	add("game profile", strings.EqualFold(cfg.Profile, ProfileID), "nil.json profile should be game")

	entryPath := cfg.GetEntryPath(projectDir)
	_, entryErr := os.Stat(entryPath)
	add("entry source", entryErr == nil, "entry file: "+cfg.Entry)

	capResult, capErr := cfg.ValidateCapabilities()
	add("capability matrix", capErr == nil && capResult != nil && capResult.Valid, capabilityDetail(capResult, capErr))

	requiredCaps := []string{"GPU", "Audio", "Filesystem"}
	for _, capName := range requiredCaps {
		add("capability "+capName, containsFold(cfg.Capabilities, capName), "required for portable game runtime")
	}

	add("android target", containsFold(cfg.Targets, "android"), "needed for direct APK export")
	add("desktop target", containsAnyFold(cfg.Targets, []string{"windows", "linux", "onuron", "macos", "darwin"}), "needed for editor-side game logic testing")

	adapterNames := selectedAdapters(cfg)
	if len(adapterNames) == 0 {
		add("engine adapters", false, "metadata engine_adapters should list android, godot, unity, or unreal")
		return report
	}

	for _, adapter := range adapterNames {
		target, ok := ResolveEngine(adapter)
		if !ok {
			add("adapter "+adapter, false, "unknown adapter")
			continue
		}
		info, err := os.Stat(filepath.Join(projectDir, target.Directory))
		add("adapter "+string(target.ID), err == nil && info.IsDir(), target.Directory)
	}

	return report
}

func selectedAdapters(cfg *config.ProjectConfig) []string {
	if cfg.Metadata == nil {
		return nil
	}
	value := strings.TrimSpace(cfg.Metadata["engine_adapters"])
	if value == "" {
		return nil
	}
	var adapters []string
	for _, piece := range strings.Split(value, ",") {
		if trimmed := strings.TrimSpace(piece); trimmed != "" {
			adapters = append(adapters, trimmed)
		}
	}
	return adapters
}

func capabilityDetail(result *capability.VerificationResult, err error) string {
	if err != nil {
		return err.Error()
	}
	if result == nil {
		return "capability verification did not return a result"
	}
	if result.Valid {
		return "declared capabilities match the game profile"
	}
	return strings.Join(result.Violations, "; ")
}

func containsFold(values []string, target string) bool {
	for _, value := range values {
		if strings.EqualFold(value, target) {
			return true
		}
	}
	return false
}

func containsAnyFold(values []string, targets []string) bool {
	for _, target := range targets {
		if containsFold(values, target) {
			return true
		}
	}
	return false
}

func sourceTemplate(name string) string {
	return fmt.Sprintf(`// %s - Nilang portable game logic
// Build direct Android APKs with: nil build android
// Build engine bundles for Godot, Unity, and Unreal with: nil build

let gameName = "%s";
let targetFPS = 60;
let frame = 0;
let playerX = 0;
let playerY = 0;
let velocityX = 2;
let velocityY = 1;

puts("Nilang game boot: \(gameName)");
puts("Target FPS: \(targetFPS)");
puts("Adapters: Android APK, Godot, Unity, Unreal");

while (frame < 8) {
    let playerX = playerX + velocityX;
    let playerY = playerY + velocityY;
    puts("frame=\(frame) player=(\(playerX), \(playerY))");
    let frame = frame + 1;
}

puts("Game loop smoke test complete.");
`, name, name)
}

func engineReadme(name string, targets []EngineTarget) string {
	var b strings.Builder
	b.WriteString("# Nilang game engine adapters\n\n")
	b.WriteString("Project: " + name + "\n\n")
	b.WriteString("Build the portable Nilang bundle first:\n\n")
	b.WriteString("    nil build\n\n")
	b.WriteString("For a direct Android APK:\n\n")
	b.WriteString("    nil build android\n\n")
	b.WriteString("Selected adapters:\n\n")
	for _, target := range targets {
		b.WriteString("- " + target.Name + ": copy the bundle to " + target.BundleDestination + "\n")
	}
	b.WriteString("\nRun readiness checks with:\n\n")
	b.WriteString("    nil game doctor\n")
	return b.String()
}

func adapterFiles(name string, target EngineTarget) map[string]string {
	switch target.ID {
	case EngineAndroid:
		return map[string]string{
			"engine/android/README.md": androidReadme(name),
		}
	case EngineGodot:
		return map[string]string{
			"engine/godot/README.md":          godotReadme(name),
			"engine/godot/nilang.gdextension": godotExtensionTemplate(),
			"engine/godot/nilang_bridge.gd":   godotBridgeScript(),
			"engine/godot/bin/.gitkeep":       "",
			"engine/godot/nilang/.gitkeep":    "",
		}
	case EngineUnity:
		return map[string]string{
			"engine/unity/package.json":                    unityPackageTemplate(name),
			"engine/unity/Runtime/NilangBehaviour.cs":      unityBehaviourTemplate(),
			"engine/unity/Runtime/NilangBridge.cs":         unityBridgeTemplate(),
			"engine/unity/Plugins/.gitkeep":                "",
			"engine/unity/StreamingAssets/nilang/.gitkeep": "",
		}
	case EngineUnreal:
		return map[string]string{
			"engine/unreal/NilLang.uplugin":                          unrealPluginTemplate(),
			"engine/unreal/Source/NilLang/NilLang.Build.cs":          unrealBuildTemplate(),
			"engine/unreal/Source/NilLang/Public/NilLangBridge.h":    unrealBridgeHeader(),
			"engine/unreal/Source/NilLang/Private/NilLangBridge.cpp": unrealBridgeSource(),
			"engine/unreal/Content/Nilang/.gitkeep":                  "",
		}
	default:
		return map[string]string{}
	}
}

func androidReadme(name string) string {
	return "# Android APK\n\n" +
		"Build and install a debug APK:\n\n" +
		"    nil build android\n" +
		"    adb install build/" + name + ".apk\n\n" +
		"The generated APK carries assets/app.nil and assets/app.nilax so the same game logic can run on device.\n"
}

func godotReadme(name string) string {
	return "# Godot adapter\n\n" +
		"Copy build/" + name + "-0.1.0.nilax to res://nilang/build/app.nilax.\n\n" +
		"Attach nilang_bridge.gd to an autoload or root node. The descriptor is ready for a native GDExtension binary named libnilang_godot in engine/godot/bin.\n"
}

func godotExtensionTemplate() string {
	return `[configuration]
entry_symbol = "nilang_library_init"
compatibility_minimum = "4.2"

[libraries]
linux.debug.x86_64 = "res://addons/nilang/bin/libnilang_godot.so"
windows.debug.x86_64 = "res://addons/nilang/bin/nilang_godot.dll"
macos.debug = "res://addons/nilang/bin/libnilang_godot.dylib"
android.debug.arm64 = "res://addons/nilang/bin/libnilang_godot.android.arm64.so"
`
}

func godotBridgeScript() string {
	return `extends Node

@export var bundle_path := "res://nilang/build/app.nilax"

func _ready():
	print("Nilang bundle ready: " + bundle_path)

func _process(delta):
	pass
`
}

func unityPackageTemplate(name string) string {
	packageName := strings.ToLower(strings.ReplaceAll(name, "_", "-"))
	return fmt.Sprintf(`{
  "name": "org.nilang.%s",
  "version": "0.1.0",
  "displayName": "Nilang Game Runtime",
  "description": "Unity bridge files for loading a Nilang .nilax game bundle.",
  "unity": "2021.3",
  "author": {
    "name": "Nilang"
  }
}
`, packageName)
}

func unityBehaviourTemplate() string {
	return `using UnityEngine;

namespace Nilang.Unity
{
    public sealed class NilangBehaviour : MonoBehaviour
    {
        [SerializeField] private TextAsset bundle;

        private void Start()
        {
            var bundleName = bundle != null ? bundle.name : "StreamingAssets/nilang/app.nilax";
            Debug.Log("Nilang bundle ready: " + bundleName);
        }

        private void Update()
        {
            NilangBridge.Tick(Time.deltaTime);
        }
    }
}
`
}

func unityBridgeTemplate() string {
	return `using System.Runtime.InteropServices;

namespace Nilang.Unity
{
    public static class NilangBridge
    {
        public static void Tick(float deltaSeconds)
        {
            // Native plugin entry point is intentionally optional while authoring in the editor.
        }
    }
}
`
}

func unrealPluginTemplate() string {
	return `{
  "FileVersion": 3,
  "Version": 1,
  "VersionName": "0.1.0",
  "FriendlyName": "Nilang Runtime",
  "Description": "Loads Nilang .nilax game logic bundles.",
  "Category": "Scripting",
  "Modules": [
    {
      "Name": "NilLang",
      "Type": "Runtime",
      "LoadingPhase": "Default"
    }
  ]
}
`
}

func unrealBuildTemplate() string {
	return `using UnrealBuildTool;

public class NilLang : ModuleRules
{
    public NilLang(ReadOnlyTargetRules Target) : base(Target)
    {
        PCHUsage = PCHUsageMode.UseExplicitOrSharedPCHs;
        PublicDependencyModuleNames.AddRange(new string[] { "Core", "CoreUObject", "Engine" });
    }
}
`
}

func unrealBridgeHeader() string {
	return `#pragma once

#include "CoreMinimal.h"

class FNilangBridge
{
public:
    static void Tick(float DeltaSeconds);
};
`
}

func unrealBridgeSource() string {
	return `#include "NilLangBridge.h"

void FNilangBridge::Tick(float DeltaSeconds)
{
    // Native Nilang runtime loading is wired here by engine builds.
}
`
}

func gitignoreTemplate() string {
	return `# Nilang build output
build/
*.nilax

# Engine build caches
Library/
Temp/
Binaries/
Intermediate/
.godot/

# OS
.DS_Store
Thumbs.db
`
}

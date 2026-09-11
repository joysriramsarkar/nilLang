# Game Engine Readiness

Nilang is now prepared as a portable game-logic layer for Android APK builds and for engine-hosted workflows in Godot, Unity, and Unreal.

## What is ready

- `profile: "game"` with GPU, audio, filesystem asset access, network, sensors, AI, crypto, and process capabilities.
- Direct Android packaging through `nil build android`, embedding both source and `.nilax` bundle assets.
- Portable `.nilax` bundles for editor and engine workflows.
- `nil game init` for creating a game project with Android, Godot, Unity, and Unreal adapter folders.
- `nil game doctor` for checking that a game project has the right profile, capabilities, Android target, desktop/editor target, and adapter folders.
- `examples/game-starter` as a ready reference project.

## Start a new game

    nil game init my-game --engine all
    cd my-game
    nil game doctor
    nil build android

Use narrower engine output when needed:

    nil game init my-unity-game --engine unity,android
    nil game init my-godot-game --engine godot
    nil game init my-unreal-game --engine unreal

## Android

Build:

    nil build android

Install:

    adb install build/my-game.apk

The generated APK includes:

- `assets/app.nil`
- `assets/app.nilax`
- files from the project `resources` directory

If Android SDK tools are present, the APK is zipaligned and debug-signed. Without them, Nilang still emits an unsigned APK suitable for manual signing.

## Godot

Use the generated `engine/godot` folder as an addon seed. Copy the game bundle to:

    res://nilang/build/app.nilax

The `nilang.gdextension` descriptor reserves the native extension names for desktop and Android builds.

## Unity

Use the generated `engine/unity` folder as a Unity package seed. Copy the bundle to:

    Assets/StreamingAssets/nilang/app.nilax

`NilangBehaviour.cs` gives the scene-level update hook. `NilangBridge.cs` is the stable boundary for native runtime calls.

## Unreal

Use the generated `engine/unreal` folder as an Unreal plugin seed. Copy the bundle to:

    Content/Nilang/app.nilax

The generated module contains the public bridge header and runtime module descriptor expected by Unreal projects.

## Boundary

Nilang is prepared for game logic, simulation, data transforms, and engine integration scaffolding. It is not claiming to replace Godot, Unity, or Unreal editors, renderers, physics, audio mixers, importers, or asset cookers. For production engines, wire the generated bridge stubs to native engine plugins and keep `.nilax` as the portable game-logic artifact.

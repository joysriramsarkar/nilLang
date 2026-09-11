# Nilang game engine adapters

Project: my-nilang-game

Build the portable Nilang bundle first:

    nil build

For a direct Android APK:

    nil build android

Selected adapters:

- Android APK: copy the bundle to assets/app.nilax
- Godot 4 GDExtension: copy the bundle to res://nilang/build/app.nilax
- Unity Package: copy the bundle to Assets/StreamingAssets/nilang/app.nilax
- Unreal Engine Plugin: copy the bundle to Content/Nilang/app.nilax

Run readiness checks with:

    nil game doctor

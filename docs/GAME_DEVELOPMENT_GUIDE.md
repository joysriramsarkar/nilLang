# 🎮 Nilang গেম ডেভেলপমেন্ট সম্পূর্ণ গাইড

> **তারিখ:** ২০২৬-০৯-১১  
> **অবস্থা:** ✅ সম্পূর্ণ প্রস্তুত (Fully Ready)

---

## 📋 পরিবেশ সামারি (Environment Summary)

| উপাদান | অবস্থা | সংস্করণ | পথ |
|--------|--------|---------|-----|
| Nilang Compiler | ✅ ইনস্টল | v0.1.0 | `C:\Users\joysr\go\bin\nil.exe` |
| Godot Engine | ✅ ইনস্টল | 4.7.2 | `C:\Users\joysr\Tools\Godot\` |
| Android SDK | ✅ ইনস্টল | Platform-tools + Build-tools 37 | `C:\Users\joysr\AppData\Local\Android\Sdk` |
| Android NDK | ✅ ইনস্টল | আছে | `C:\Users\joysr\AppData\Local\Android\Sdk\ndk` |
| Java JDK | ✅ ইনস্টল | OpenJDK 21.0.11 | `C:\Program Files\Eclipse Adoptium\jdk-21.0.11.10-hotspot` |
| ADB | ✅ ইনস্টল | v1.0.41 | SDK Platform-tools |
| Go Toolchain | ✅ ইনস্টল | go1.27.0 | System PATH |
| Unity Editor | ⬜ আলাদাভাবে ইনস্টল করতে হবে | — | [unity.com/download](https://unity.com/download) |
| Unreal Engine | ⬜ আলাদাভাবে ইনস্টল করতে হবে | — | [unrealengine.com](https://www.unrealengine.com/download) |

---

## 🔧 পরিবেশ চলচ্চিত্র (Environment Variables)

নিম্নলিখিত এনভায়রনমেন্ট ভ্যারিয়েবল **User** স্তরে সেট করা হয়েছে:

| Variable | মান |
|----------|-----|
| `ANDROID_HOME` | `C:\Users\joysr\AppData\Local\Android\Sdk` |
| `ANDROID_SDK_ROOT` | `C:\Users\joysr\AppData\Local\Android\Sdk` |
| `GODOT_HOME` | `C:\Users\joysr\Tools\Godot` |
| `PATH` (User) | Godot পথ যোগ করা হয়েছে |
| `PATH` (System) | ANDROID_HOME/platform-tools যোগ করা হয়েছে |


---

## 🚀 দ্রুত শুরু (Quick Start)

### ১. নতুন গেম প্রজেক্ট তৈরি (সব ইঞ্জিন একসাথে)

```powershell
cd C:\Users\joysr\Documents\programming language\nilLang
nil game init my-game --engine all
cd my-game
nil game doctor
nil build android
```

### ২. নির্দিষ্ট ইঞ্জিনের জন্য প্রজেক্ট তৈরি

```powershell
# শুধু Android
nil game init my-android-game --engine android

# Godot + Android
nil game init my-godot-game --engine godot,android

# Unity + Android
nil game init my-unity-game --engine unity,android

# Unreal + Android
nil game init my-unreal-game --engine unreal,android
```

### ৩. রেফারেন্স প্রজেক্ট দেখুন

```powershell
cd examples/game-starter
nil game doctor        # ✅ সব ১২টি চেক পাস
nil build android      # ✅ APK + nilax বান্ডিল তৈরি
nil run build/game-starter-0.1.0.nilax  # ✅ গেম রান
```

---

## 📱 অ্যান্ড্রয়েড (Android)

### বিল্ড

```powershell
nil build android
```

আউটপুট:
- `build/{name}.apk` — সাইনড ডিবাগ APK (~17-25 KB)
- `build/{name}-{version}.nilax` — ইউনিভার্সাল বান্ডিল

### ইনস্টল

```powershell
adb install build/{name}.apk
```


---

## 🎮 Godot 4 (GDExtension)

### Godot কনফিগারেশন

Godot 4.7.2 ইনস্টল:
- এক্সিকিউটেবল: `C:\Users\joysr\Tools\Godot\Godot_v4.7.2-stable_win64.exe`
- কনসোল: `C:\Users\joysr\Tools\Godot\Godot_v4.7.2-stable_win64_console.exe`

### Nilang বান্ডিল এক্সপোর্ট

```powershell
cd {project-dir}
nil build
```

### Godot প্রজেক্ট সেটআপ

১. Godot খুলুন → Project → New Project
২. `engine/godot/` ফোল্ডার থেকে ফাইলগুলো কপি করুন:
   - `res://nilang/build/app.nilax` — আপনার .nilax বান্ডিল
   - `res://nilang/build/nilang.gdextension` — GDExtension ডেসক্রিপ্টর
   - `res://nilang/build/nilang_bridge.gd` — ব্রিজ স্ক্রিপ্ট

---

## 🧱 Unity

### Unity ইনস্টলেশন (আলাদাভাবে)

১. [Unity Hub](https://unity.com/download) ইনস্টল করুন
২. Unity Editor 2021.3+ ইনস্টল করুন

### Nilang বান্ডিল এক্সপোর্ট

```powershell
cd {project-dir}
nil build
```

### Unity C# ব্রিজ

`NilangBehaviour.cs` দেয়ান:
```csharp
using UnityEngine;
namespace Nilang.Unity {
    public sealed class NilangBehaviour : MonoBehaviour {
        [SerializeField] private TextAsset bundle;
        private void Start() {
            Debug.Log("Nilang bundle ready");
        }
        private void Update() { NilangBridge.Tick(Time.deltaTime); }
    }
}
```

---

## 🚀 Unreal Engine

### Unreal Engine ইনস্টলেশন (আলাদাভাবে)

১. [Epic Games Launcher](https://www.unrealengine.com/download) থেকে ইনস্টল করুন
২. Unreal Engine 5.x ইনস্টল করুন

---

## 🏗️ প্রজেক্ট স্ট্রাকচার

```
{project-name}/
├── nil.json                     # প্রজেক্ট কনফিগ (profile: game)
├── src/
│   └── main.nil                 # মূল গেম লজিক
├── resources/
│   └── nilang.game.json         # গেম কনফিগ
├── build/
│   ├── {name}.apk              # Android APK
│   └── {name}-{ver}.nilax      # Universal Bundle
├── engine/
│   ├── android/README.md
│   ├── godot/                   # nilang.gdextension, nilang_bridge.gd
│   ├── unity/                   # package.json, NilangBehaviour.cs, NilangBridge.cs
│   └── unreal/                  # NilLang.uplugin, bridge header/cpp
└── .gitignore
```


---

## 📖 কমান্ড রেফারেন্স

### গেম কমান্ড

| কমান্ড | বিবরণ |
|--------|--------|
| `nil game init [name] [--engine all/android,godot,unity,unreal]` | গেম প্রজেক্ট স্ক্যাফোল্ড তৈরি |
| `nil game doctor [project-dir]` | রেডিনেস চেক (১২টি চেক) |
| `nil game engines` | সমর্থিত ইঞ্জিন লিস্ট দেখান |

### নিল কমান্ড

| কমান্ড | বিবরণ |
|--------|--------|
| `nil build android` | অ্যান্ড্রয়েড APK + nilax বান্ডিল |
| `nil build [linux/windows/macos/onuron/wasm/web]` | নির্দিষ্ট টার্গেট বিল্ড |
| `nil run [file.nil]` | নির্দিষ্ট ফাইল চালান |
| `nil run [file.nilax]` | .nilax বান্ডিল চালান |

---

## ⚙️ রেডিনেস চেক (nil game doctor)

`nil game doctor` নিম্নলিখিত ১২টি চেক চালায়:

| # | চেক | বিবরণ |
|---|------|--------|
| ১ | game profile | `nil.json` প্রোফাইল "game" আছে কিনা |
| ২ | entry source | এন্ট্রি ফাইল `src/main.nil` আছে কিনা |
| ৩ | capability matrix | ক্যাপাবিলিটি ম্যাট্রিক্স বৈধ কিনা |
| ৪ | capability GPU | GPU ক্যাপাবিলিটি আছে কিনা |
| ৫ | capability Audio | অডিও ক্যাপাবিলিটি আছে কিনা |
| ৬ | capability Filesystem | ফাইলসিস্টেম ক্যাপাবিলিটি আছে কিনা |
| ৭ | android target | Android টার্গেট আছে কিনা |
| ৮ | desktop target | ডেস্কটপ টার্গেট আছে কিনা |
| ৯ | adapter android | Android অ্যাডাপ্টার ফোল্ডার আছে কিনা |
| ১০ | adapter godot | Godot অ্যাডাপ্টার ফোল্ডার আছে কিনা |
| ১১ | adapter unity | Unity অ্যাডাপ্টার ফোল্ডার আছে কিনা |
| ১২ | adapter unreal | Unreal অ্যাডাপ্টার ফোল্ডার আছে কিনা |

---

## 🧪 টেস্ট

```powershell
go test ./pkg/game/...        # গেম প্যাকেজ টেস্ট
go test ./...                 # পুরো প্রজেক্ট টেস্ট
go test ./pkg/mobile/android/...  # Android বিল্ডার টেস্ট
go test ./pkg/signing/...     # সাইনিং টেস্ট
go test ./pkg/game/... -v     # গেম টেস্ট ভার্বোজ>
```

---

## 🔒 সীমাবদ্ধতা (Boundary)

Nilang একটি **পোর্টেবল গেম-লজিক লেয়ার** হিসেবে তৈরি করা হয়েছে। এটি Godot, Unity, বা Unreal এর সম্পূর্ণ প্রতিস্থাপন নয়। Nilang:

- ✅ **দেয়**: গেম লজিক, সিমুলেশন, ডেটা ট্রান্সফর্ম, টেন্সর কম্পিউট, `.nilax` পোর্টেবল বান্ডিল
- ✅ **দেয়**: অ্যান্ড্রয়েড APK বিল্ড, GDExtension/Unity/Unreal ব্রিজ স্ক্যাফোল্ড
- ❌ **দেয় না**: ইঞ্জিন এডিটর, রেন্ডারার, ফিজিক্স, অডিও মিক্সার, অ্যাসেট ইম্পোর্টার/কুকার

---

## ✅ এখন যা যা হয়ে গেছে (2026-09-11)

1. ✅ Nilang v0.1.0 কম্পাইলার — গেম কমান্ড সহ
2. ✅ Godot 4.7.2 — ইনস্টল ও PATH কনফিগার
3. ✅ Android SDK — পূর্ণাঙ্গ (platform-tools, build-tools 37, NDK)
4. ✅ OpenJDK 21 — Android বিল্ডের জন্য
5. ✅ ANDROID_HOME, GODOT_HOME — স্থায়ী এনভায়রনমেন্ট ভ্যারিয়েবল
6. ✅ `nil game init` — ২১টি ফাইল সহ সব ইঞ্জিন স্ক্যাফোল্ড
7. ✅ `nil game doctor` — ১২টি রেডিনেস চেক
8. ✅ `nil build android` — সাইনড ডিবাগ APK + nilax বান্ডিল
9. ✅ `nil run` — .nilax বান্ডিল চালানো
10. ✅ `examples/game-starter` — রেফারেন্স প্রজেক্ট
11. ✅ সমস্ত `go test` — পাস
12. ✅ `my-nilang-game` — ডেমো প্রজেক্ট তৈরি হয়েছে

---

## ⬜ যা এখনো করতে হবে (Unity ও Unreal এর জন্য)

- [ ] **Unity Hub + Editor** ইনস্টল করুন — [unity.com/download](https://unity.com/download)
- [ ] **Unreal Engine** ইনস্টল করুন — [unrealengine.com/download](https://www.unrealengine.com/download)

এগুলো ছাড়া Godot ও Android সম্পূর্ণ কাজ করবে। Unity ও Unreal এর জন্য শুধু নেটিভ এনজিন ইনস্টলের দরকার, বাকি সব স্ক্যাফোল্ডিং Nilang থেকে আসে।

---

*গেম শুরু করুন! 🎮*  
`nil game init my-first-game --engine all` → `cd my-first-game` → `nil build android` → `adb install build/my-first-game.apk*

---

## 🐛 সম্পূর্ণ হওয়া বাগ ফিক্স (2026-09-11)

| বাগ | অবস্থা | বিবরণ |
|------|--------|--------|
| Typechecker tensor 3-arg | ✅ ফিক্স | `compiler/typecheck/typecheck.go` - `tensor` ফাংশনের min/max arity 2,2 থেকে 2,3 এ পরিবর্তন করা হয়েছে, যাতে `dtype` প্যারামিটার সাপোর্ট পায় |
| SoftBus `from` keyword | ✅ ফিক্স | `examples/softbus-chat/src/main.nil` - `from` ও `to` প্যারামিটারের নাম `sender` ও `receiver` এ পরিবর্তন করা হয়েছে (`from` একটি রিজার্ভড কীওয়ার্ড) |


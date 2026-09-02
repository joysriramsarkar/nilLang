# 🌟 Nilang (নীলাং) Programming Language

[![License: AGPL-3.0](https://img.shields.io/badge/License-AGPL_3.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Onuron%20OS%20%7C%20Linux%20%7C%20Windows-blue)](#)
[![Ecosystem](https://img.shields.io/badge/Ecosystem-Alap%20Framework-005A9C)](#)

> **"A modern, concurrent, and declarative programming language built specifically for the Alap Framework and Onuron OS."**

নীলাং (Nilang) হলো একটি শক্তিশালী, আধুনিক এবং ডিক্লারেটিভ প্রোগ্রামিং ভাষা যা **Alap Framework** এবং **Onuron OS** ইকোসিস্টেমের জন্য তৃণমূল থেকে তৈরি করা হয়েছে। এটি গো-এর সরলতা, রাস্টের পারফরম্যান্স ও নিরাপত্তা এবং ArkTS-এর মতো ডিক্লারেটিভ UI প্যারাডাইমের মেলবন্ধন।

---

## 🏛️ মূল বৈশিষ্ট্য (Core Highlights)

- 🎨 **Alap Declarative UI & Component Model**: `component`, `state`, `render`, `emit` কীওয়ার্ড সহ রিঅ্যাক্টিভ UI স্টেট ম্যানেজমেন্ট।
- 🎬 **60 FPS Animation Engine**: ৩০টিরও বেশি বিল্ট-ইন ইজিং কার্ভ (Bounce, Elastic, Cubic, Back, Quad ইত্যাদি) এবং টাইমলাইন কি-ফ্রেমিং।
- ⚡ **Vulkan & OpenGL GPU Renderer Pipeline**: জিপিইউ-অ্যাক্সিলারেটেড ভেক্টর, শেডার এবং ব্যাচড রেন্ডারিং।
- 📡 **Distributed SoftBus Protocol**: Onuron OS ডিভাইসের মধ্যে জিরো-কনফিগ ল্যান ডিসকভারি, RPC মেসেজিং এবং ফাইল ট্রান্সফার।
- 📦 **.nilax Application Bundle & nilpkg Package Manager**: জিপ-ভিত্তিক কম্প্রেসড অ্যাপ বান্ডিল, সিগনেচার ভেরিফিকেশন এবং মাইক্রোসার্ভিস রেজিস্ট্রি সার্ভার।
- 🔐 **Ed25519 Package Signing (`nilkey`)**: ক্রিপ্টোগ্রাফিক কি-পেয়ার জেনারেশন, এনক্রিপ্টেড কি-স্টোর এবং প্যাকেজ সাইনিং।
- 🛠️ **Dual Execution Engine**:
  - **Tree-Walking Interpreter**: তাৎক্ষণিক ডেভেলপমেন্ট ও REPL-এর জন্য।
  - **Bytecode Compiler + Stack VM**: হাই-পারফরম্যান্স প্রোডাকশন এক্সিকিউশনের জন্য।

---

## 📂 রিপোজিটরি আর্কিটেকচার (Architecture)

```text
nilLang/
├── cmd/
│   ├── nil/              # অল-ইন-ওয়ান CLI (build, run, repl, init, fmt, render, clean)
│   ├── nilc/             # পিওর বাইটকোড কম্পাইলার ও ডিসঅ্যাসেম্বলার
│   ├── nilpkg/           # প্যাকেজ ম্যানেজার ক্লায়েন্ট
│   ├── nilpkg-server/    # প্যাকেজ রেজিস্ট্রি ও ড্যাশবোর্ড সার্ভার
│   ├── nilkey/           # Ed25519 ক্রিপ্টোগ্রাফিক কী ম্যানেজমেন্ট টুল
│   └── softbusd/         # ডিস্ট্রিবিউটেড সফটবাস নেটওয়ার্ক ডেমন
├── compiler/
│   ├── token/            # লেক্সিক্যাল টোকেন সংজ্ঞা
│   ├── lexer/            # UTF-8 স্ক্যানার, স্ট্রিং ইন্টারপোলেশন \(...)
│   ├── ast/              # নোড হায়ারার্কি ও অ্যাবস্ট্রাক্ট সিনট্যাক্স ট্রি
│   ├── parser/           # Pratt রিকার্সিভ ডিসেন্ট পার্সার
│   ├── object/           # রানটাইম অবজেক্ট মডেল ও এনভায়রনমেন্ট
│   ├── evaluator/        # ট্রি-ওয়াকিং ইন্টারপ্রেটার ও স্ট্যান্ডার্ড বিল্ট-ইন
│   ├── code/             # বাইটকোড অপকোড ও ডিসঅ্যাসেম্বলার
│   ├── compiler/         # AST থেকে বাইটকোড কম্পাইলার ও সিম্বল টেবিল
│   └── vm/               # হাই-স্পিড স্ট্যাক-ভিত্তিক ভার্চুয়াল মেশিন
├── pkg/
│   ├── alap/             # Alap ডিক্লারেটিভ UI ফ্রেমওয়ার্ক ও ANSI/HTML রেন্ডারার
│   ├── animation/        # ৩০+ ইজিং ফাংশন, কি-ফ্রেম ট্র্যাক, টাইমলাইন ইঞ্জিন
│   ├── gpu/              # ভলকান/ওপেনজিএল জিপিইউ ব্যাকএন্ড ও শেডার পাইপলাইন
│   ├── bundle/           # .nilax অ্যাপ বান্ডিল বিল্ডার ও রিডার
│   ├── config/           # nil.json প্রজেক্ট কনফিগারেশন ম্যানেজার
│   ├── nilpkg/           # ডিপেনডেন্সি রেজলভার, ভেরিফায়ার ও ইনস্টলার
│   ├── registry/         # এইচটিটিপি রেজিস্ট্রি স্টোরেজ ও এপিআই
│   ├── signing/          # Ed25519 ডিজিটাল সাইনিং ও কি-স্টোর
│   └── softbus/          # Onuron SoftBus পিয়ার ডিসকভারি ও আরপিসি
├── runtime/
│   └── vm/
│       ├── bridge_cgo.go    # Onuron OS / Linux Rust FFI ব্রিজ
│       ├── bridge_nocgo.go  # ক্রস-প্ল্যাটফর্ম পিওর গো ফলব্যাক ব্রিজ
│       └── native/          # হাই-পারফরম্যান্স নেটিভ রাস্ট লাইব্রেরি
├── examples/
│   ├── hello-onuron/     # সম্পূর্ণ .nilax বান্ডিল প্রজেক্ট উদাহরণ
│   ├── ui-counter/       # Alap UI স্টেট ও কাউন্টার উদাহরণ
│   ├── animation-demo/   # জিপিইউ ইজিং অ্যানিমেশন উদাহরণ
│   └── softbus-chat/     # সফটবাস ডিস্ট্রিবিউটেড মেসেজিং উদাহরণ
├── syntaxes/
│   └── nilang.tmLanguage.json  # VS Code ও টেক্সটমেট সিনট্যাক্স হাইলাইটিং
└── .github/
    └── languages.yml     # গিটহাব লিঙ্গুইস্ট ভাষা স্বীকৃতি
```

---

## 🚀 দ্রুত শুরু (Quick Start)

### ১. বাইনারি কম্পাইল করুন

সবগুলো বাইনারি (`nil`, `nilc`, `nilpkg`, `nilpkg-server`, `nilkey`, `softbusd`) একবারে কম্পাইল করার জন্য:

#### অপশন ১: লোকাল `bin/` ফোল্ডারে বিল্ড (এক কমান্ডেই সব)
```bash
# সব CLI টুল এক কমান্ডে তৈরি করুন
go build -o bin/ ./cmd/...

# অথবা উইন্ডোজে সরাসরি বিল্ড স্ক্রিপ্ট চালান:
.\build.bat

# অথবা Linux/macOS-এ:
make
```

#### অপশন ২ (প্রস্তাবিত): গ্লোবাল ইনস্টলেশন
যাতে টার্মিনালের যেকোনো জায়গা থেকে সরাসরি `nil`, `nilc` ইত্যাদি রান করতে পারেন:
```bash
go install ./cmd/...
```
*(এটি সরাসরি Go-এর বিন ডিরেক্টরিতে সব টুল ইনস্টল করে দেবে, ফলে কোনো পাথ উল্লেখ ছাড়াই সরাসরি কমান্ডগুলো কাজ করবে)*

---

### ২. প্রথম নীলাং প্রোগ্রাম চালান

**যদি `go install` (গ্লোবাল) করে থাকেন:**
```bash
# সরাসরি স্ক্রিপ্ট রান করুন (ট্রি-ওয়াকিং ইন্টারপ্রেটার)
nil run examples/hello-onuron/src/main.nil

# হাই-স্পিড বাইটকোড ভার্চুয়াল মেশিনে রান করুন
nil run examples/hello-onuron/src/main.nil -vm

# ইন্টারঅ্যাক্টিভ REPL চালু করুন
nil repl
```

**অথবা লোকাল `bin/` থেকে চালাতে:**
```bash
# Windows (PowerShell / CMD):
.\bin\nil.exe run examples/hello-onuron/src/main.nil
.\bin\nil.exe run examples/hello-onuron/src/main.nil -vm
.\bin\nil.exe repl

# Linux / macOS:
./bin/nil run examples/hello-onuron/src/main.nil
./bin/nil run examples/hello-onuron/src/main.nil -vm
./bin/nil repl
```

---

## 💻 ভাষা ও সিনট্যাক্স (Language Syntax)

### ভ্যারিয়েবল ও টাইপস (Variables & Types)

```nil
let message = "নমস্কার, Onuron OS!";
let count = 42;
let pi = 3.14159;
let isActive = true;
let fruits = ["আম", "জাম", "লিচু"];
let user = {"name": "জয়শ্রীরাম", "role": "Developer"};
```

### স্ট্রিং ইন্টারপোলেশন (String Interpolation)

```nil
let author = "Joysriram Sarkar";
let release = "1.0.0";
puts("Nilang v\(release) created by \(author)!");
```

### ফাংশন ও ক্লোজার (Functions & Closures)

```nil
let makeMultiplier = fn(factor) {
    return fn(n) {
        return n * factor;
    };
};

let double = makeMultiplier(2);
puts("5 * 2 = \(double(5))"); // 10
```

### লুপ ও কন্ট্রোল ফ্লো (Loops & Control Flow)

```nil
let i = 0;
while (i < 5) {
    puts("Iteration: \(i)");
    let i = i + 1;
}
```

---

## 📦 প্রজেক্ট বিল্ড ও .nilax বান্ডিল

একটি নীলাং প্রজেক্টের রুটে থাকে `nil.json`:

```json
{
  "name": "my-app",
  "version": "1.0.0",
  "author": "Your Name",
  "entry": "src/main.nil",
  "targets": ["onuron", "linux"]
}
```

প্রজেক্ট বিল্ড করতে:

```bash
nil build
```

এটি আপনার সোর্স কোডকে বাইটকোডে কম্পাইল করে সব রিসোর্স ও ম্যানিফেস্ট সহ `build/my-app-1.0.0.nilax` বান্ডিল তৈরি করবে।

---

## 🔐 প্যাকেজ সাইনিং (`nilkey`)

Ed25519 ক্রিপ্টোগ্রাফিক কী জেনারেট এবং বান্ডিল সাইন করুন:

```bash
# ডেভেলপার কি-পেয়ার তৈরি করুন
nilkey generate -name="Joyshriram" -email="joy@onuron.org" -password="secretpassword"

# .nilax প্যাকেজ সাইন করুন
nilkey sign build/hello-onuron-1.0.0.nilax -password="secretpassword"

# সাইন ভেরিফাই করুন
nilkey verify build/hello-onuron-1.0.0.nilax
```

---

## 🛒 প্যাকেজ ম্যানেজার (`nilpkg`) & রেজিস্ট্রি

```bash
# প্যাকেজ ভেরিফাই করুন
nilpkg verify build/hello-onuron-1.0.0.nilax

# লোকাল প্যাকেজ ইনস্টল করুন
nilpkg install build/hello-onuron-1.0.0.nilax

# ইনস্টলড প্যাকেজ তালিকা
nilpkg list

# রেজিস্ট্রি সার্ভার চালু করুন (পোর্ট ৮০৮০)
nilpkg-server
```

---

## 🎨 Alap Declarative UI & 60 FPS Animation

Alap UI কম্পোনেন্ট ANSI কনসোল অথবা ব্রাউজার প্রিভিউতে রেন্ডার করতে:

```bash
nil render
```

আউটপুট প্রিভিউ হিসেবে `build/preview.html` জেনারেট হবে।

---

## 📡 Onuron SoftBus ডিস্ট্রিবিউটেড মেসেজিং

লোকাল নেটওয়ার্কে SoftBus ডেমন চালু করুন:

```bash
softbusd -id="onuron-pc-01" -port=9000
```

সফটবাস ডেমন পিয়ার ডিসকভারি, ফাইল ট্রান্সফার এবং আরপিসি রিকোয়েস্ট পরিচালনা করবে।

---

## 🧪 টেস্ট সুইট চালানো (Running Tests)

সম্পূর্ণ রিপোজিটরির সব টেস্ট চালান:

```bash
go test ./...
```

সবগুলো মডিউল (Lexer, Parser, Evaluator, VM, Bundle, Signing, Animation) ১০০% টেস্ট পাস করে।

---

## 📜 লাইসেন্স (License)

Nilang Programming Language is licensed under the [GNU Affero General Public License v3.0 (AGPL-3.0)](LICENSE).
Created with passion by Joysriram Sarkar for **Alap Framework** & **Onuron OS**.

# নীলাং ভাষাকে Android, iOS ও HarmonyOS-এর পূর্ণাঙ্গ মোবাইল অ্যাপ প্ল্যাটফর্মে উন্নীত করার রূপরেখা

**ধরন:** দীর্ঘমেয়াদি প্রযুক্তিগত নকশা, ইঞ্জিনিয়ারিং রোডম্যাপ ও গ্রহণযোগ্যতা-পরীক্ষার পরিকল্পনা  
**প্রধান লক্ষ্য:** একটি নীলাং সোর্স প্রজেক্ট থেকে Android, iOS এবং HarmonyOS-এর জন্য বাস্তব, ইনস্টলযোগ্য, পরীক্ষা-যোগ্য ও রিলিজ-উপযোগী অ্যাপ তৈরি করা।  
**প্রস্তাবিত নীতি:** ভাষার মূল অংশে স্থিতিশীলতা; অ্যাপ-লজিক ও UI-র মধ্যে স্পষ্ট সীমা; প্ল্যাটফর্ম-নির্দিষ্ট কাজের জন্য নির্দিষ্ট ব্যাকএন্ড; দাবির বদলে পরীক্ষা দিয়ে সক্ষমতা প্রমাণ।

---

## ১. নির্বাহী সারাংশ: আসলে কী তৈরি করতে হবে

নীলাংকে “যে কোনো আধুনিক ফোনের জন্য অ্যাপ বানানোর ভাষা” করতে হলে শুধু নতুন কীওয়ার্ড, আরও বেশি বিল্ট-ইন ফাংশন, অথবা `nil build android`-এর মতো একটি কমান্ড যোগ করলেই হবে না। সম্পূর্ণ ব্যবস্থাটিকে কয়েকটি স্তরে ভাগ করতে হবে:

1. **ভাষা ও কম্পাইলার:** সোর্স কোডের নির্দিষ্ট অর্থ, টাইপ, ত্রুটি, মডিউল, লাইব্রেরি ও সংস্করণের নিয়ম।
2. **রানটাইম:** নীলাং প্রোগ্রাম চালানো, মেমরি ও অবজেক্টের জীবনচক্র, এক্সেপশন, অ্যাসিঙ্ক কাজ, নেটওয়ার্ক ও নিরাপদ API অ্যাক্সেস।
3. **মোবাইল অ্যাপ মডেল:** অ্যাপের এন্ট্রি পয়েন্ট, স্ক্রিন, স্টেট, ব্যাকগ্রাউন্ড কাজ, অনুমতি, ডিপ-লিংক, নোটিফিকেশন, সংরক্ষণ ও জীবনচক্রের নিয়ম।
4. **UI ফ্রেমওয়ার্ক:** এক ঘোষণামূলক UI মডেল থেকে প্ল্যাটফর্ম-উপযোগী UI তৈরি; লেআউট, ফর্ম, নেভিগেশন, অ্যানিমেশন, অ্যাক্সেসিবিলিটি ও ইনপুট।
5. **প্ল্যাটফর্ম অ্যাডাপ্টার:** Android, iOS ও HarmonyOS-এর নেটিভ API-তে নিয়ন্ত্রিত প্রবেশাধিকার।
6. **বিল্ড ও প্যাকেজিং:** প্রজেক্ট কনফিগারেশন থেকে নেটিভ প্রজেক্ট, রিসোর্স, সাইনিং, ডিবাগ বিল্ড ও রিলিজ প্যাকেজ।
7. **ডেভেলপার টুলিং:** formatter, language server, editor integration, debugger, profiler, diagnostics, test runner ও emulator/device workflow।
8. **গুণমান প্রমাণ:** unit test, conformance test, real-device test, security review, compatibility matrix ও reproducible builds।

এখানে সবচেয়ে জরুরি পার্থক্য হলো **source portability**, **logic portability**, **UI portability**, **native capability parity** এবং **distribution readiness** এক জিনিস নয়। একটি `.nil` ফাইল সব প্ল্যাটফর্মে parse হতে পারলেই অ্যাপটি সব ফোনে চলবে না। একই business logic তিনটি প্ল্যাটফর্মে চললেও UI নেটিভ না-ও হতে পারে। একই UI দেখা গেলেও push notification, background task, permission, camera, Bluetooth বা store publishing অসম্পূর্ণ থাকতে পারে।

### প্রস্তাবিত চূড়ান্ত অভিজ্ঞতা

```text
my-app/
├── nil.json
├── src/
│   ├── main.nil
│   ├── app.nil
│   ├── screens/
│   ├── models/
│   ├── services/
│   └── shared/
├── resources/
│   ├── images/
│   ├── fonts/
│   ├── icons/
│   └── locales/
├── tests/
└── platforms/                 # ঐচ্ছিক native extension ও override

# প্রত্যাশিত developer workflow
nil check
nil test
nil run --target android
nil build android --debug
nil device install android
nil device logs android
nil test --target android --device
nil build ios --release
nil build harmony --release
```

উপরের কমান্ডগুলো **প্রস্তাবিত CLI চুক্তি**, বর্তমান রেপোজিটরিতে সব কটি ইতিমধ্যে রয়েছে—এমন দাবি নয়। CLI-এর নাম চাইলে বদলাতে পারো; মূল কথা হলো কমান্ডগুলোর আচরণ স্পষ্ট, পুনরুৎপাদনযোগ্য ও পরীক্ষিত হতে হবে।

### সফলতার বাস্তব মাপকাঠি

প্রথম লক্ষ্য “সমস্ত ফোনে সব ফিচার” নয়। প্রথম লক্ষ্য হবে তিনটি প্ল্যাটফর্মের জন্য একই ছোট অ্যাপের নির্ভরযোগ্য vertical slice: একটি স্ক্রিন, টেক্সট, বাটন, স্টেট, স্থানীয় ডেটা সংরক্ষণ, নেভিগেশন, প্ল্যাটফর্মের Back/দিকনির্দেশ, অ্যাপ বন্ধ-খোলার পর স্টেট পুনরুদ্ধার, ডিবাগ লগ, এবং স্বাক্ষরিত পরীক্ষামূলক প্যাকেজ। এই ক্ষুদ্র অ্যাপ তিনটি বাস্তব পরিবেশে পরীক্ষা না করা পর্যন্ত বড় API তালিকা তৈরি করা উচিত নয়।

---

## ২. বর্তমান নীলাং: কোথা থেকে শুরু করবে

প্রকাশ্য README অনুযায়ী নীলাংয়ের বর্তমান স্থাপত্যে Go-ভিত্তিক টুলিং, tree-walking interpreter, bytecode compiler ও stack VM, HIR/MIR, diagnostics, WebAssembly backend, `.nilax` bundle, package manager/signing, Alap declarative UI এবং SoftBus-এর মতো উপাদানের কথা রয়েছে। বর্তমান `go.mod`-এ Go module ব্যবহার করা হয়েছে। এগুলো ভালো ভিত্তি, কিন্তু মোবাইল প্ল্যাটফর্মের সম্পূর্ণতা প্রমাণ করে না।

### কী আছে বলে নথিতে উল্লেখ আছে

- lexer/parser/AST এবং ভাষার execution path;
- interpreter এবং bytecode VM;
- HIR/MIR-এ রূপান্তরের সরঞ্জাম;
- WebAssembly জেনারেটর;
- `.nilax` bundle reader/builder;
- package manager, signing ও verification;
- Alap UI এবং browser preview workflow;
- Android game-build workflow-এর বর্ণনা;
- Godot, Unity ও Unreal-এর adapter scaffolding-এর উল্লেখ।

### কী আলাদাভাবে যাচাই করতে হবে

নথিতে কোনো ফিচারের বর্ণনা থাকা মানে নিম্নলিখিত সব কটি যাচাই হয়ে গেছে—এটা ধরে নেওয়া যাবে না:

- নির্দিষ্ট সোর্স ফাইলে ফিচারটি বাস্তবায়িত আছে কি না;
- compiler ও interpreter একই semantics অনুসরণ করে কি না;
- বাস্তব ফোনে ইনস্টলযোগ্য প্যাকেজ তৈরি হয় কি না;
- সেই প্যাকেজে সঠিক ABI, runtime, manifest, resources ও signatures আছে কি না;
- পরিষ্কার পরিবেশে build পুনরুৎপাদন করা যায় কি না;
- runtime crash, lifecycle, permissions ও memory pressure সামলাতে পারে কি না;
- release mode-এ debug mode-এর মতোই সঠিক ফল পাওয়া যায় কি না;
- API বাস্তব OS সংস্করণে কাজ করে কি না;
- UI accessibility, keyboard, rotation, font scaling ও localization ঠিক থাকে কি না;
- iOS এবং HarmonyOS-এর জন্য আলাদা integration পরীক্ষা আছে কি না।

README-তে Android APK-র কথা থাকলেও সেটিকে এখনই পূর্ণাঙ্গ সাধারণ Android অ্যাপ-কম্পাইলার হিসেবে ধরে নেওয়া উচিত নয়। গেম-সংক্রান্ত নথিতে Android package-এর ভিতরে `assets/app.nil` ও `assets/app.nilax` থাকার কথা বলা হয়েছে; SDK tools না থাকলে unsigned APK-ও তৈরি হতে পারে। একই নথি game-engine integration-এর ক্ষেত্রে generated bridge-কে seed/stub হিসেবে ব্যাখ্যা করে। এটি উপযোগী prototype/scaffolding হতে পারে, কিন্তু release-ready product-এর মানদণ্ড আলাদা।

### প্রথম কাজ: capability inventory

নতুন ফিচার যোগ করার আগে একটি `docs/platform-capabilities.md` তৈরি করো। প্রতিটি capability-এর পাশে নিচের অবস্থা লিখবে:

- `spec-only`: নকশা/ডকুমেন্ট আছে, বাস্তবায়ন নেই;
- `prototype`: demo চলে, কিন্তু stability/security contract নেই;
- `implemented`: code আছে, unit test আছে;
- `integration-tested`: emulator/device-এ পরীক্ষা হয়েছে;
- `release-verified`: release build, signing, lifecycle, error handling ও documentation যাচাই হয়েছে;
- `unsupported`: ইচ্ছাকৃতভাবে এই সংস্করণে নেই।

উদাহরণ:

| Capability | Language/runtime | Android | iOS | HarmonyOS | প্রমাণ |
|---|---|---|---|---|---|
| string/number/basic collection | implemented? যাচাই করতে হবে | n/a | n/a | n/a | unit tests |
| bytecode VM | README-তে উল্লেখ | mobile runtime verify | mobile runtime verify | mobile runtime verify | runtime tests |
| declarative Button/Text | browser preview উল্লেখ | native/device test প্রয়োজন | test প্রয়োজন | test প্রয়োজন | UI integration test |
| local persistent store | actual API audit প্রয়োজন | test প্রয়োজন | test প্রয়োজন | test প্রয়োজন | restart/device test |
| camera permission | API audit প্রয়োজন | unsupported until tested | unsupported until tested | unsupported until tested | physical-device test |
| signed release package | signing component উল্লেখ | signing audit | signing workflow প্রয়োজন | signing workflow প্রয়োজন | clean CI build |

প্রশ্নবোধক অবস্থা অনুমান করে “সম্পূর্ণ” লিখবে না। কোড, টেস্ট ও artefact না দেখে সেটিকে যাচাইহীন বলবে। এটি দুর্বলতা স্বীকার নয়; একটি serious language project-এর জন্য এটাই পেশাদার পদ্ধতি।

---

## ৩. “পূর্ণাঙ্গ” শব্দের সংজ্ঞা নির্দিষ্ট করো

একটি ভাষা কখনো সব ধরনের অ্যাপ, সব OS API, সব হার্ডওয়্যার, সব OS সংস্করণ ও সব অ্যাপ-স্টোরের বৈশিষ্ট্য একসঙ্গে সমর্থন করতে পারে না। “যেকোনো আধুনিক মোবাইল”কে তাই একটি যাচাইযোগ্য target promise-এ নামাতে হবে।

### ৩.১ প্রাথমিক সমর্থনের সীমানা

প্রথম public mobile release-এর জন্য নির্দিষ্ট করে লিখবে:

- Android-এর কোন ন্যূনতম API level সমর্থিত;
- কোন CPU ABI সমর্থিত—যেমন 64-bit ARM; emulator-এর জন্য প্রয়োজন হলে x86-64;
- iOS-এর কোন minimum deployment target ও device architecture;
- HarmonyOS-এর কোন generation/API version এবং কোন package model;
- কোন feature সাধারণভাবে সমর্থিত, কোনটি platform-specific extension চায়;
- কোন feature background restrictions বা store policy-র কারণে সীমাবদ্ধ;
- কোন feature unsupported হলে compile-time warning/error দেবে।

চলমান OS version-গুলো দ্রুত বদলায়; প্রতিটি major release-এর আগে official SDK matrix আপডেট করতে হবে। একটি স্থির সংখ্যা চিরকালের জন্য ধরে নেওয়া ঠিক নয়।

### ৩.২ পাঁচটি আলাদা স্তরের প্রস্তুতি

**স্তর A — Parse portability:** একই সোর্স কোড তিন প্ল্যাটফর্মে parse ও type-check হয়।  
**স্তর B — Runtime portability:** standard language behavior একই থাকে; সংখ্যা, string, collections, errors, async semantics একরকম।  
**স্তর C — Logic portability:** business logic, validation, data model, networking policy ও state transition ভাগ করা যায়।  
**স্তর D — App/UI portability:** screen tree, navigation, input ও layout-এর একই declaration থেকে উপযুক্ত platform UI তৈরি হয়।  
**স্তর E — Product portability:** permissions, lifecycle, signing, store package, crash diagnostics, accessibility ও device testing সহ সম্পূর্ণ বিতরণ-যোগ্য অ্যাপ তৈরি হয়।

প্রতিটি স্তরের pass criteria আলাদা থাকবে। একটি project-কে `mobile-ready` বলার আগে অন্তত কোন স্তর পর্যন্ত ready তা দেখাতে হবে।

### ৩.৩ “একই কোড” বনাম “একই অভিজ্ঞতা”

একই UI source থেকে iOS, Android ও HarmonyOS-এর অভিজ্ঞতা পুরোপুরি এক হবে—এটি লক্ষ্য হওয়া উচিত নয়। তিন প্ল্যাটফর্মের Back/gesture, dialog, keyboard, status bar, navigation, permission prompt ও accessibility-এর আচরণে পার্থক্য আছে। নীলাংয়ের কাজ হবে একটি অভিন্ন app intent বজায় রেখে সঠিক native convention অনুসরণ করা। ব্যবহারকারীকে Android-এর UI-কে iOS-এ জোর করে বসিয়ে দেওয়া বা উল্টোটা করা ঠিক হবে না।

---

## ৪. স্থাপত্যগত নির্বাচন: নীলাং কীভাবে native app তৈরি করবে

তিনটি সাধারণ পন্থা আছে। নীলাংয়ের জন্য একটি পন্থাকে মূল পথ এবং অন্য দুটিকে ভবিষ্যৎ বিকল্প হিসেবে নির্ধারণ করা উচিত।

### ৪.১ পন্থা A: WebView ভিত্তিক UI

নীলাং UI-কে HTML/CSS/JavaScript-এ রূপান্তর করে অ্যাপের ভিতরে WebView-তে দেখানো। নেটিভ wrapper ফোনের camera, file system, notification ইত্যাদির সঙ্গে bridge দেয়।

**সুবিধা:**
- বর্তমানে browser preview বা HTML renderer থাকলে প্রথম demo দ্রুত করা যায়;
- layout, typography ও styling-এর জন্য পরিচিত web technology ব্যবহার করা যায়;
- UI logic দ্রুত iterate করা যায়;
- কিছু feature PWA হিসেবেও পুনর্ব্যবহারযোগ্য হতে পারে।

**অসুবিধা:**
- UI accessibility, platform feel, keyboard behavior, performance ও memory management-এর অনেক কাজ থেকেই যায়;
- bridge-এর নিরাপত্তা ভুল হলে web content নেটিভ ক্ষমতা অপব্যবহার করতে পারে;
- platform-native component parity নিশ্চিত করা কঠিন;
- WebView-এর engine version এবং OS-এর আচরণের উপর নির্ভরতা বাড়ে;
- background processing, push notifications, sharing, media, system navigation ও store review-এর জন্য native integration তবু প্রয়োজন।

**ব্যবহার:** প্রথম demonstrator, internal tools, content-first app, অথবা দ্রুত prototype। এটিকে নীলাংয়ের একমাত্র বা চূড়ান্ত UI strategy করা উচিত নয়।

### ৪.২ পন্থা B: নিজস্ব scene graph ও renderer

Alap একটি নিজস্ব widget tree/scene graph তৈরি করবে, যেটি নিজস্ব 2D renderer বা graphics API-র মাধ্যমে আঁকা হবে। Flutter-জাতীয় পদ্ধতির মতো একই visual output সব প্ল্যাটফর্মে বেশি নিয়ন্ত্রণযোগ্য হতে পারে।

**সুবিধা:**
- layout, animation ও rendering-এ উচ্চ নিয়ন্ত্রণ;
- একই UI design একাধিক platform-এ তুলনামূলকভাবে consistent;
- game-like UI বা custom visual tool-এ উপকারী;
- widget system পুরোপুরি নিজের মতো করে বানানো যায়।

**অসুবিধা:**
- native text input, IME composition, selection handles, accessibility tree, screen reader, dynamic type, context menus, drag-and-drop, autofill, system gestures ও focus system পুনর্নির্মাণ করতে হয়;
- GPU backend, text shaping, glyph rasterization, font fallback, clipping, DPI scaling, color space, touch dispatch ও frame scheduling তৈরি করা কঠিন;
- Vulkan/Metal/Direct3D/অন্যান্য backend-এর কাজ বিশাল;
- performance শুধু “GPU আছে” বললেই নিশ্চিত হয় না; profiling ও real-device benchmark দরকার।

**ব্যবহার:** পরবর্তী পর্যায়ের বিশেষ UI, game UI বা high-control rendering। প্রথম মোবাইল release-এর বাধ্যতামূলক নির্ভরতা বানিও না।

### ৪.৩ পন্থা C: Native UI adapter + shared Nilang logic — সুপারিশকৃত

নীলাং app-logic, data model, validation, state machine এবং declarative UI description দেয়। তারপর platform backend সেই description-কে native widget ও lifecycle-এ মানচিত্রায়িত করে। Android backend Jetpack Compose বা View-ভিত্তিক host adapter তৈরি করতে পারে; iOS backend SwiftUI/UIKit wrapper; HarmonyOS backend ArkUI/ArkTS host layer। কম্পাইলার প্রথম পর্যায়ে সরাসরি machine code জেনারেট করতে বাধ্য নয়; সে intermediate app/UI representation থেকে native project source ও wrapper generate করতে পারে।

**সুবিধা:**
- native accessibility এবং platform convention বজায় রাখার সুযোগ;
- প্রতিটি OS-এর অফিসিয়াল SDK-তে প্রয়োজনীয় ক্ষমতা পাওয়া;
- business logic ভাগ করা যায়, কিন্তু platform-dependent behavior আলাদা রাখা যায়;
- প্রথমে code generation, পরে প্রয়োজন অনুযায়ী embedded runtime/AOT পথে যাওয়া যায়;
- compiler-এর কাজের সীমানা স্পষ্ট থাকে।

**অসুবিধা:**
- তিনটি adapter maintain করতে হবে;
- UI component contract ও state/event mapping খুব ভালোভাবে নকশা করতে হবে;
- build system-এ platform SDK dependency থাকবে;
- platform-specific code লেখার একটি পরিষ্কার escape hatch রাখতে হবে।

**চূড়ান্ত সুপারিশ:** নীলাংয়ের প্রথম বাস্তব multi-platform product-এর জন্য পন্থা C বেছে নাও। প্রয়োজন হলে prototype-এ WebView fallback রাখো। নিজস্ব GPU renderer-কে পৃথক, ঐচ্ছিক প্রকল্প হিসেবে বিকশিত করো।

### ৪.৪ প্রস্তাবিত architecture diagram

```text
            .nil source + nil.json + resources
                         |
                  Lexer / Parser
                         |
                     Typed AST
                         |
                 HIR / semantic model
                         |
              Mobile App IR + UI IR
                         |
       +-----------------+------------------+
       |                 |                  |
 Android backend      iOS backend      Harmony backend
       |                 |                  |
 Kotlin/Compose     Swift/SwiftUI      ArkTS/ArkUI host
 + Nilang runtime   + Nilang runtime   + Nilang bridge/runtime
       |                 |                  |
 APK/AAB           Xcode archive/IPA    HAP/App package
       |                 |                  |
 Android device       iPhone/iPad       HarmonyOS device
```

এই diagram-এ “Kotlin/Compose”, “Swift/SwiftUI” বা “ArkTS/ArkUI” নীলাংয়ের ভাষা হয়ে যাচ্ছে না। এগুলো target-specific output বা host integration। নীলাং ডেভেলপার সাধারণত `.nil`-এ কাজ করবে; generated source দেখতে ও debug করতে পারবে; প্রয়োজনে explicit native extension যোগ করতে পারবে।

## ৫. নীলাং ভাষার core specification শক্ত করো

মোবাইল ব্যাকএন্ড তৈরির আগে ভাষার semantics স্থির করতে হবে। একটি interpreter-এ আজ যে আচরণ হয়, Android build-এ কাল ভিন্ন ফল হলে developer-রা ভাষার উপর আস্থা হারাবে। তাই syntax feature বাড়ানোর চেয়ে compiler/interpreter/runtime consistency-কে অগ্রাধিকার দাও।

### ৫.১ একটি versioned language specification

`docs/spec/`-এ একটি versioned specification রাখো, যেটি codebase-এর সঙ্গে একই release-এ version পাবে। অন্তত এই বিষয়গুলো নির্দিষ্ট হবে:

- identifiers, Unicode normalization এবং case-sensitivity;
- keywords, comments, newline/semicolon policy;
- numeric literal, integer overflow ও floating-point semantics;
- string encoding, interpolation, escaping এবং invalid Unicode handling;
- variable binding, mutability ও scope;
- function, closure, return, recursion ও call semantics;
- arrays, maps, tuples/records এবং iteration order;
- error/exception/Result pattern;
- type inference, generics, union/ADT এবং nullability;
- module/import visibility ও circular-dependency policy;
- async task, cancellation, timeout এবং scheduler semantics;
- FFI/host-call rules;
- reflection, serialization ও ABI compatibility;
- standard library guarantees;
- compiler flags এবং optimization levels-এর আচরণ।

Specification ছাড়া compiler উন্নত হতে পারে, কিন্তু language platform স্থিতিশীল হয় না। README-তে থাকা সব ভাষাগত feature-এর জন্য specification ও test case থাকা উচিত।

### ৫.২ Compiler এবং interpreter-এর এক semantics

নিচের ছোট প্রোগ্রাম interpreter, bytecode VM, WASM target এবং ভবিষ্যতের mobile runtime-এ একই ফল দেয় কি না, সেটা একটি corpus দিয়ে পরীক্ষা করো:

```nil
let amount = 10;
let taxRate = 0.05;
let total = amount + amount * taxRate;
puts("total=\(total)");
```

এটি খুব সহজ উদাহরণ; বাস্তব conformance corpus-এ closures, nested collections, generic functions, error propagation, async, string Unicode, integer edge cases, recursion, equality, sort order এবং module import থাকতে হবে। প্রতিটি case-এর source, expected output, expected error এবং target list থাকবে।

**Golden test** শুধু snapshot রাখবে না; test failure হলে কোন semantic rule ভেঙেছে সেটিও জানাবে। Test corpus-এর সংস্করণ release-এর সঙ্গে বাঁধা থাকবে।

### ৫.৩ Type system: strictness এবং usability-এর ভারসাম্য

মোবাইল অ্যাপের জন্য static typing বিশেষ কাজে লাগে, কারণ permission response, network error, lifecycle event এবং asynchronous state-এর ভুল আগে ধরা যায়। তবে overly complex type system single maintainer-এর প্রকল্পে অনেক compiler complexity তৈরি করবে।

প্রথমে স্থিতিশীল করো:

- `Int`, `Float/Double`, `Bool`, `String`, `Bytes`, `Unit`;
- typed list/map, immutable record/struct;
- optional values বা `Option<T>`;
- typed error/`Result<T, E>`;
- function types ও closures;
- generics-এর একটি predictable subset;
- enums/ADT এবং exhaustive pattern matching;
- module boundary-তে public API type checking।

এরপর প্রয়োজন হলে interfaces/traits, effect types, actor model বা ownership-সদৃশ discipline যোগ করো। Generics, union, effect, ADT ইত্যাদির নাম README-তে থাকা মানেই তাদের পুরো semantic interaction সহজ বা সম্পূর্ণ—এমন ধরে নেওয়া যাবে না। প্রতিটি feature-এর interaction table বানাও: generics+async, generics+FFI, optional+serialization, union+pattern matching, error+native callback ইত্যাদি।

### ৫.৪ Nullability ও error path

মোবাইল অ্যাপে সাধারণ ভুল হয়: network timeout, permission denied, file missing, app background-এ যাওয়া, native API error এবং user cancellation। এগুলোকে silent `nil` বা empty string দিয়ে প্রকাশ করো না।

```nil
let result = await http.get(url);
match result {
    Ok(response) => render(response.body),
    Err(NetworkError.Timeout) => showMessage("সময় শেষ হয়েছে"),
    Err(NetworkError.Offline) => showMessage("ইন্টারনেট সংযোগ নেই"),
    Err(other) => logError(other)
}
```

উপরের syntax কেবল প্রস্তাবিত উদাহরণ; বাস্তব নীলাং grammar-এ `match` বা `Ok/Err` যেভাবে থাকবে, specification-এ তা সংজ্ঞায়িত করতে হবে। মূল নীতি হলো error type-এ অন্তত `code`, `message`, `retryable`, `platformDetails` এবং প্রয়োজনে underlying cause থাকবে। Sensitive credential বা personal data-কে error message-এ ছাপানো যাবে না।

### ৫.৫ Async, concurrency এবং cancellation

মোবাইল অ্যাপের সব network, database, file, camera বা platform callback synchronousভাবে main UI thread-এ চালানো উচিত নয়। নীলাংয়ে asynchronous task-এর জন্য সুস্পষ্ট policy প্রয়োজন:

- `async/await`-এর semantics;
- parent-child task relationship;
- cancellation propagation;
- timeout/deadline;
- error propagation;
- UI/main-thread dispatch;
- background task-এর জীবনকাল;
- app background/terminate হলে কাজের কী হবে;
- task leakage ও unhandled error detection।

শুরুতে একটি structured concurrency model বেছে নাও। যেকোনো জায়গা থেকে unbounded background thread/task তৈরি করা সহজ করলে battery drain এবং lifecycle bug বাড়বে। UI screen-এর মালিকানাধীন কাজ screen ছেড়ে গেলে cancel করতে পারবে; durable upload বা scheduled work আলাদা background-work API ব্যবহার করবে।

### ৫.৬ Memory management

প্রথম mobile release-এর জন্য memory ব্যবস্থাপনা অত্যন্ত গুরুত্বপূর্ণ। সম্ভাব্য বিকল্প:

1. **Tracing garbage collector:** implement করা তুলনামূলকভাবে সহজ, কিন্তু pause, memory spike, mobile constraints এবং native object lifetime সামলাতে হবে।
2. **Reference counting:** deterministic cleanup পাওয়া যায়, কিন্তু cycles এবং cross-language references-এর নকশা কঠিন।
3. **Host-managed objects plus VM heap:** নীলাংয়ের object heap আলাদা থাকবে; native objects opaque handles হিসেবে থাকবে এবং explicit release/weak-reference rules পাবে।
4. **AOT ownership/borrow model:** শক্তিশালী হলেও language design ও compiler burden অনেক বেশি।

প্রস্তাব: বর্তমান VM-এর সঙ্গে সামঞ্জস্যপূর্ণ একটি নির্দিষ্ট heap strategy স্থির করো; native UI object-কে নীলাং heap-এর সাধারণ object মনে করো না। `NativeHandle` lifecycle, finalization, explicit close/dispose এবং leak diagnostics তৈরি করো। প্রথম দিন থেকেই native callback ধরে থাকা closure leak test করো।

### ৫.৭ ABI এবং FFI contract

Nilang runtime বা generated native bridge-এর জন্য platform-independent C ABI-সদৃশ একটি সীমিত boundary বিবেচনা করতে পারো। উদাহরণ:

```c
// ধারণাগত interface; final ABI spec নয়
NilEngine* nil_engine_create(const NilConfig* config);
NilResult nil_engine_load(NilEngine*, const uint8_t* source, size_t length);
NilValue nil_engine_call(NilEngine*, const char* module, const char* function,
                         const NilValue* args, size_t argc);
void nil_value_release(NilEngine*, NilValue);
void nil_engine_destroy(NilEngine*);
```

ABI-তে স্থির integer width, string/byte buffer ownership, error representation, symbol versioning, thread-safety, reentrancy ও shutdown rules থাকতে হবে। Swift, JNI এবং Harmony native bridge এই contract-কে সরাসরি বা wrapper-এর মাধ্যমে ব্যবহার করতে পারে। ABI-কে “যে কোনো Go internal struct” দিয়ে প্রকাশ কোরো না; internal struct বদলালে ABI ভেঙে যাবে।

### ৫.৮ Modules ও dependency boundaries

প্রকল্পের module graph স্পষ্ট হওয়া দরকার:

- `stdlib.core`: string, number, collection, result, formatting;
- `stdlib.async`: tasks, timeout, cancellation;
- `stdlib.net`: HTTP ও URL primitives;
- `stdlib.data`: JSON, CSV, schema/serialization;
- `stdlib.fs`: capability-guarded file access;
- `stdlib.mobile`: app lifecycle ও platform abstraction;
- `alap.ui`: UI tree, state, events, layout contract;
- `platform.android`, `platform.ios`, `platform.harmony`: native integration;
- `testing`: test harness ও fake services।

`core` module যেন `android.*` বা `ark.*`-এর উপর নির্ভর না করে। Dependency graph একমুখী হবে; compiler বা platform plugin-কে language runtime-এর internal implementation-এর সঙ্গে সরাসরি জড়িয়ে ফেলবে না।

---

## ৬. নীলাং Mobile Application Model

একটি মোবাইল অ্যাপ কেবল `main()` ফাংশন চালানো প্রক্রিয়া নয়। OS app process বন্ধ করে দিতে পারে, screen ঘোরাতে পারে, permission revoke হতে পারে, user deep link দিয়ে নির্দিষ্ট screen-এ ঢুকতে পারে, notification থেকে app চালু হতে পারে। তাই নীলাংয়ের জন্য একটি app lifecycle specification অপরিহার্য।

### ৬.১ অ্যাপের entry point

প্রস্তাবিত ধারণা:

```nil
app MyApp {
    id: "org.onuron.example"
    version: "0.1.0"
    minimum: {
        android: "<প্রকল্পে নির্ধারিত API level>"
        ios: "<নির্ধারিত deployment target>"
        harmony: "<নির্ধারিত API version>"
    }

    launch => HomeScreen

    permissions {
        network
        notifications: optional
    }
}
```

এটি illustration; syntax চূড়ান্ত নয়। Manifest-এর তথ্য, API version ও native project config-এর source of truth একটাই হওয়া উচিত। একই permission যদি `nil.json`, `.nil` এবং generated platform project-এ তিন জায়গায় আলাদা করে লিখতে হয়, configuration drift হবে।

### ৬.২ Application lifecycle event

একটি platform-neutral API-তে অন্তত নিচের ধারণাগুলো থাকবে:

- `launch`: প্রথম launch বা cold start;
- `resume`: foreground-এ ফিরে আসা;
- `pause`: user অন্য app-এ গেলে বা screen আড়াল হলে;
- `stateSave`: পুনরুদ্ধারের জন্য state সংরক্ষণ;
- `terminateExpected`: OS যদি সুযোগ দেয়, shutdown signal;
- `deepLink(url)`: external URL/route থেকে প্রবেশ;
- `notificationAction(action)`: notification-এ user-এর ক্রিয়া;
- `permissionChanged(permission, status)`: permission status পরিবর্তন;
- `memoryWarning(level)`: memory pressure-এর ইঙ্গিত, যেখানে OS অনুমতি দেয়।

কোনো callback সব platform-এ সমানভাবে নিশ্চিত হবে না। SDK event model অনুযায়ী adapter event-কে common event-এ map করবে এবং unsupported event-এর জন্য documented fallback থাকবে। Termination-এর আগে callback চলবেই—এমন নিশ্চয়তা দেবে না। গুরুত্বপূর্ণ data নিয়মিত commit করতে হবে, shutdown event-এর উপর নির্ভর নয়।

### ৬.৩ State persistence

UI state, domain state এবং durable data আলাদা করতে হবে:

- **UI state:** selected tab, expanded section, temporary form state;
- **domain state:** logged-in session-এর domain information, cart, document state;
- **durable data:** database, encrypted credentials, file বা server-side source of truth।

প্রস্তাবিত state flow একমুখী হবে:

```text
User event -> Action -> State transition -> New state -> Render
```

UI widget-কে নিজেই database update, permission request এবং network call করতে দিলে test ও lifecycle management কঠিন হয়। `ViewModel` নাম ব্যবহার করতেই হবে এমন নয়; মূল কথা হলো state owner এবং side-effect boundary স্পষ্ট হওয়া। Android-এর official architecture guide-ও UI ও data layer আলাদা রাখা, lifecycle-aware state management এবং adaptive UI-কে গুরুত্ব দেয়: https://developer.android.com/topic/architecture

### ৬.৪ Configuration ও app manifest

`nil.json`-কে অপ্রয়োজনীয়ভাবে সবকিছুর mega-config বানিও না। প্রস্তাবিত ক্ষেত্রগুলো:

```json
{
  "name": "nilang-notes",
  "version": "0.1.0",
  "entry": "src/main.nil",
  "targets": ["android", "ios", "harmony"],
  "appId": "org.onuron.nilangnotes",
  "permissions": ["network"],
  "resources": "resources",
  "nativeExtensions": [],
  "build": {
    "minify": false,
    "debugSymbols": true
  }
}
```

এটি একটি নমুনা schema, এখনই সমর্থিত configuration নয়। Schema version বাধ্যতামূলক করো; invalid field-এ স্পষ্ট error দাও; unknown field silently ignore করলে typo-র কারণে security setting অনুপস্থিত থাকতে পারে। Config validate করবে `nil check` এবং build শুরু হওয়ার আগে diagnostics দেবে।

### ৬.৫ Navigation ও deep linking

একটি portable navigation model নির্ধারণ করো:

- route identity ও typed parameters;
- push/pop/replace/reset semantics;
- Android back button/gesture;
- iOS navigation stack ও universal link/deep link;
- HarmonyOS route/navigation behavior;
- app cold start-এ route restoration;
- unknown route ও malformed URL-এর fallback;
- authentication-required route-এর redirect policy।

Native navigation-এর সব খুঁটিনাটি এক API-তে ঢোকানোর চেষ্টা কোরো না। Common layer route intent জানাবে; platform adapter native behavior বজায় রাখবে।

---

## ৭. Alap UI: ঘোষণামূলক মডেলকে বাস্তব মোবাইল UI-তে রূপান্তর

নীলাংয়ের README-তে `component`, `state`, `render`, `on`, `build`, `emit`-সহ declarative/event-driven UI এবং browser preview-এর কথা আছে। এই মডেল কাজে লাগতে পারে, কিন্তু browser HTML output আর native mobile widget output আলাদা backend। তাই Alap-এর portable UI contract সংজ্ঞায়িত করো।

### ৭.১ UI intermediate representation (UI IR)

UI syntax সরাসরি HTML string-এ নামিয়ে না দিয়ে একটি typed component tree বা UI IR তৈরি করো। প্রতিটি node-এ থাকবে:

- stable identity/key;
- component type;
- typed properties;
- accessibility label/role/hint;
- children;
- event bindings;
- state bindings;
- layout constraints;
- theme tokens;
- resource references।

উদাহরণ:

```nil
component CounterScreen {
    state count: Int = 0

    render {
        Column(spacing: 12) {
            Text("গণনা: \(count)")
                .accessibilityLabel("বর্তমান গণনা \(count)")

            Button("বাড়াও") {
                count = count + 1
            }
        }
    }
}
```

এখানে `Column`, `Text`, `Button`, `accessibilityLabel` ইত্যাদি প্রস্তাবিত UI API-র উদাহরণ। বর্তমান Alap API-র syntax ও semantics যাচাই করে চূড়ান্ত করো। মূল বিষয় হলো UI-কে string template হিসেবে নয়, semantic tree হিসেবে উপস্থাপন করা।

### ৭.২ প্রথম component set ছোট রাখো

প্রথমে component parity সম্পূর্ণ করতে ৮০টি অসম্পূর্ণ widget বানিও না। প্রথম release-এ নিচেরগুলো ঠিকমতো করো:

- `Text`, `Icon`, `Image`;
- `Button`, `TextField`, `SecureField`;
- `Column`, `Row`, `Box`, `ScrollView`, `Spacer`;
- `List`, `ListItem`, `Card`, `Divider`;
- `Checkbox`, `Switch`, `Radio`, `Slider`;
- `ProgressIndicator`, `ActivityIndicator`;
- `Dialog`, `Alert`, `Sheet`;
- `AppBar/NavigationBar`, `TabBar`;
- `Form`, `EmptyState`, `ErrorState`;
- `SafeArea`, `Insets`, `ResponsiveContainer`।

প্রতিটি component-এর property types, default behavior, focus handling, keyboard interaction, accessibility, disabled state, localization এবং test fixture থাকবে। `Button` যদি এক platform-এ double event দেয় আর অন্য platform-এ event drop করে, তা component complete নয়।

### ৭.৩ Layout system

ওয়েবের pixel unit সরাসরি ফোনে ব্যবহার করলে density, system font scale ও display size-এর পার্থক্য তৈরি হবে। UI layout-এর জন্য platform-neutral logical unit ব্যবহার করো, তারপর backend device scale factor-এ map করবে।

সমর্থন করতে হবে:

- constraints-based/flex layouts;
- intrinsic text sizing;
- safe-area/inset;
- min/max width/height;
- wrap, overflow, ellipsis;
- dynamic font scale;
- portrait/landscape;
- tablet/foldable/adaptive window sizes;
- right-to-left layouts;
- keyboard open/close resize/pan behavior;
- scroll state restoration;
- accessibility traversal order।

“একটি fixed 360×800 canvas”কে সব ফোনের layout ধরে নিও না। Android-এ tablet/foldable layout এবং app window পরিবর্তনের কথা official guidance-এও বলা আছে: https://developer.android.com/topic/architecture

### ৭.৪ UI state ও rerender policy

একটি state পরিবর্তনে পুরো অ্যাপের সমস্ত UI পুনর্নির্মাণ করলে ছোট demo-তে সমস্যা দেখা নাও দিতে পারে, কিন্তু বড় তালিকা বা input-heavy screen-এ performance সমস্যা হবে। UI IR-এ stable keys ও dependency tracking রাখো। প্রাথমিকভাবে manual optimized diff না হলেও চলবে, কিন্তু architecture যেন future incremental rendering আটকায় না।

একটি UI transaction-এর নিয়ম নির্ধারণ করো:

1. event dispatch;
2. handler execution;
3. state mutation/immutable state replacement;
4. affected node invalidation;
5. layout pass;
6. paint/native update;
7. accessibility state synchronization।

একটি event-এর সময় sync exception হলে app crash করবে, নাকি error boundary-তে যাবে—এটির policy লিখে রাখো। Production mode-এ সব error নীরবে গিলে ফেললে debugging অসম্ভব হবে।

### ৭.৫ Native component, custom component ও escape hatch

প্রতিটি visual element নিজের মতো আঁকার প্রয়োজন নেই। সাধারণ form/control native widget-এ map করো; custom chart/visualization-এর জন্য custom rendering API দাও। developer যেন নিজের নেটিভ UI অংশ embed করতে পারে:

```nil
NativeView(platform: "ios", module: "MyCameraPreview")
NativeView(platform: "android", module: "CameraPreview")
NativeView(platform: "harmony", module: "CameraPreview")
```

এটি conceptual form; final API হিসেবে অনুমোদিত নয়। সবচেয়ে জরুরি হলো extension-এর type/signature ও fallback strategy documented থাকা। Platform-specific component অনুপস্থিত থাকলে unsupported platform-এ compile-time warning বা স্পষ্ট fallback দেখাতে হবে; নীরবে ফাঁকা UI নয়।

### ৭.৬ Accessibility ও localization শুরু থেকেই

Accessibility পরে যোগ করার feature নয়। UI component contract-এ role, label, hint, value, enabled/selected state, traversal grouping এবং focus behavior রাখো। Screen reader-এর নাম উচ্চারণ, text scaling, contrast, reduced-motion preference এবং keyboard/assistive device interaction টেস্ট করো।

Localization-এর জন্য user-facing string source code-এ সরাসরি ছড়িয়ে না রেখে resource catalog/translation key ব্যবহার করো। বাংলার জন্য Unicode normalization, combining marks, বাংলা সংখ্যা/তারিখ format, line break, complex script shaping এবং font fallback পরীক্ষা করো। একই সঙ্গে English, Hindi বা অন্য script-এ layout ভাঙে কি না দেখো।

### ৭.৭ Animation

বর্তমান animation engine-এ easing বা timeline থাকলে সেটি UI IR-এর সঙ্গে সংযুক্ত করা যায়। কিন্তু animation API-তে frame callback-এর ভিতরে arbitrary heavy work চালানোর অনুমতি দেবে না। স্পষ্ট করে দাও:

- duration ও easing;
- interrupt/cancel behavior;
- reduced motion;
- app background-এ animation pause হবে কি না;
- layout animation বনাম transform animation;
- compositor/GPU acceleration যেখানে সমর্থিত;
- frame-drop measurement ও test।

60 FPS-কে guarantee হিসেবে লিখো না, যদি low-end device, thermal throttling, battery saver ও complex layout-এ তা পরিমাপ করে নিশ্চিত না করা হয়। লক্ষ্য হিসেবে performance budget নির্ধারণ করো; benchmark ছাড়া marketing claim নয়।

## ৮. Mobile standard library: app-এর বাস্তব প্রয়োজনের API

নীলাংয়ে standard library বাড়াতে হবে, কিন্তু platform SDK-র প্রতিটি function-এর নাম ও signature হুবহু নকল করলেই ভালো abstraction হয় না। আগে portable API নির্ধারণ করো, তারপর platform adapter দিয়ে native implementation দাও। API-তে capability, failure, permission এবং lifecycle স্পষ্ট থাকবে।

### ৮.১ Networking

প্রথম পর্যায়ে HTTP/HTTPS client, request/response, headers, URL parsing, JSON, timeout, cancellation, upload/download progress এবং retry policy দরকার।

API-তে থাকা উচিত:

- request method ও body;
- typed header collection;
- timeout ও cancellation token;
- TLS certificate validation-এর default secure behavior;
- response status, headers ও bounded body reader;
- streaming response যেখানে সত্যিই দরকার;
- upload/download progress callback;
- network reachability-কে নিশ্চয়তা না ধরে hint হিসেবে ব্যবহার;
- retry policy-তে idempotency, backoff ও jitter;
- proxy ও custom trust configuration-এর সীমিত, স্পষ্ট ব্যবস্থা;
- sensitive header/password-কে default log থেকে বাদ রাখা।

যেমন `http.get()` একটি raw response ফেরত দেওয়ার পাশাপাশি transport failure ও HTTP status failure আলাদা করে প্রকাশ করবে। `404` সাধারণত connection failure নয়; `401`, `429`, `500` এবং TLS failure আলাদা করে handle করতে হবে। Certificate validation disable করার global switch সাধারণ অ্যাপ API-তে প্রকাশ করা যাবে না।

### ৮.২ JSON, serialization এবং schema evolution

JSON encode/decode-এর সঙ্গে type validation, invalid field handling, optional property, unknown-property policy ও versioned schema দরকার। Mobile app-এ server payload পরিবর্তিত হয়; unknown field সবসময় error করলে forward compatibility ভাঙতে পারে, আবার ভুল type নীরবে default করলে data corruption হতে পারে। API-তে এই policy explicit করতে হবে।

- `decode<T>`-এ structured decode error;
- unknown field ignore/reject/retain policy;
- date/time format;
- integer range validation;
- Unicode handling;
- enum unknown-value fallback;
- migration-friendly serialized models;
- large payload-এ memory bound।

### ৮.৩ Local storage এবং database

প্রথমে key-value preferences, file/resource access এবং SQLite-ধরনের local database adapter যুক্ত করা যেতে পারে। তবে storage API-তে security boundary স্পষ্ট হওয়া উচিত:

- app-private storage বনাম user-visible documents;
- cache বনাম durable data;
- transactional database;
- migration/versioning;
- encryption key storage-এর platform adapter;
- backup/restore behavior;
- scoped permissions;
- file picker ও share sheet;
- atomic file replacement;
- corruption recovery ও disk-full error।

Password বা authentication token plaintext preferences-এ রাখার API-কে সহজ default বানিও না। Secret store-এর platform implementation ও fallback behavior আলাদা specification পাবে। Sensitive data encryption নিয়ে বাস্তব security review করাতে হবে; নিজে cryptographic primitive উদ্ভাবন করবে না।

### ৮.৪ Device capabilities

Camera, photo library, microphone, location, contacts, Bluetooth, NFC, sensors, biometrics, calendar, health data ও background location সবই একই স্তরের feature নয়। এগুলোকে capability API হিসেবে ভাগ করো। উদাহরণস্বরূপ:

```nil
let status = await permissions.status(.camera)
if status == .granted {
    let photo = await camera.capture()
} else {
    showPermissionExplanation(.camera)
}
```

এটি illustrative syntax। Permission API-র মূল নিয়ম:

- permission status check করা এবং permission request করা আলাদা;
- request-এর আগে user-facing rationale দেখানোর পথ থাকতে হবে;
- denial, restricted, limited, granted ও unavailable status পার্থক্য করতে হবে;
- permission পরে revoke হলে error path থাকবে;
- API ব্যবহার না করলে permission manifest-এ অকারণে ঘোষণা হবে না;
- app-এ ব্যবহৃত purpose/description strings platform metadata-তে তৈরি হবে;
- test-এ permission grant এবং deny উভয় scenario থাকবে।

সব প্ল্যাটফর্মে capability থাকলেও একই permission semantics নেই। Portable API-তে common denominator থাকবে; অতিরিক্ত platform capability-এর জন্য explicit extension লাগবে।

### ৮.৫ Notifications

Local notification এবং remote push notification আলাদা API ও lifecycle ব্যবহার করবে। প্রথমে local notification, schedule, cancel, action, payload decode এবং notification tap থেকে route restoration implement করো। Push notification পরে যোগ করা যায়, কারণ এর জন্য server credentials, platform provider configuration, signing/entitlements, app lifecycle ও store privacy policy লাগে।

API design-এ:

- notification permission state;
- channel/category identifier যেখানে প্রযোজ্য;
- title/body/localization key;
- schedule ও timezone semantics;
- action identifier;
- foreground presentation policy;
- payload size ও data sensitivity;
- user notification settings change;
- duplicate delivery/idempotency।

### ৮.৬ Clipboard, share, file picker এবং external links

এগুলো সাধারণ app-এ প্রায়ই দরকার হয় এবং native feel-এর জন্য গুরুত্বপূর্ণ। API-গুলো asynchronous হবে, user cancellation স্বাভাবিক ফল হিসেবে বিবেচিত হবে, এবং কোনো external activity/extension খুললে app state পুনরুদ্ধার করতে পারবে। File URI-কে সাধারণ permanent file path ধরে নেবে না; security-scoped access প্রয়োজন হতে পারে।

### ৮.৭ Time, locale, keyboard ও text

Date/time API-তে timezone database, UTC timestamp, locale formatting, calendar system এবং daylight-saving rule আলাদা রাখো। User locale বদলালে app restart ছাড়াই UI refresh করার ব্যবস্থা রাখো। Text input-এ IME composition, autocorrect, password/autofill, selection, multiline, submit action এবং hardware keyboard support platform adapter-এর দায়িত্ব। বাংলা ও Indic script-এর composition ও cursor behavior test করা বিশেষ গুরুত্বপূর্ণ।

### ৮.৮ Biometric, camera, media ও sensor

এই API-গুলো প্রথম vertical slice-এর বাইরে রাখা ভালো। কারণ permission/entitlement, native resource lifecycle, camera session, media file format, device variation এবং platform-specific policy কাজের পরিমাণ বাড়ায়। তবে API boundary এখন থেকেই স্থির করা যায়:

- result/error type;
- cancellation;
- resource cleanup;
- background transition;
- privacy purpose;
- capability query;
- mocked provider for tests;
- unsupported target result।

একটি API শুধু stub করে `return true` দিলে সেটা implement করা নয়। Stub-এর জন্য `UnsupportedCapability` ফেরত দেবে এবং logs-এ স্পষ্ট করবে।

---

## ৯. তিনটি প্ল্যাটফর্মের আলাদা backend পরিকল্পনা

একই নীলাং source সব platform-এ চলতে পারে, কিন্তু build pipeline এক হবে না। SDK, UI, package, signing, capabilities ও store delivery আলাদা থাকবে। প্রতিটি backend-কে আলাদা project এবং আলাদা acceptance criteria হিসেবে ভাবো।

## ৯.১ Android backend

Android-কে প্রথম target হিসেবে নেওয়া বাস্তবসম্মত, কারণ APK/device workflow দ্রুত পরীক্ষা করা যায় এবং তোমার বর্তমান নথিতে Android game build আছে। তবে game bundle packaging-কে সাধারণ app target-এর সমতুল্য ধরে নেওয়া যাবে না।

### Build output-এর ধাপ

প্রথম পর্যায়ে `nil build android --debug` একটি minimal Android project তৈরি করবে। নির্ভরযোগ্যতার জন্য নিজে সব APK binary structure হাতে assemble করার বদলে official Android SDK/Gradle tooling orchestration করো। Generated project-এ থাকবে:

- `AndroidManifest.xml`;
- launcher Activity/host;
- generated `res/` ও assets;
- `lib/`-এ runtime native libraries যেখানে প্রয়োজন;
- generated Kotlin/Java bridge;
- Gradle build files ও pinned plugin versions;
- debug/release build variants;
- ProGuard/R8 বা equivalent shrink/minify policy;
- test target ও package metadata।

### UI integration

প্রথম UI backend হিসেবে Jetpack Compose-কে বিবেচনা করা যায়, কারণ Alap declarative semantics থেকে Compose-like composable host generate করা পরিষ্কার হতে পারে। বিকল্প হিসেবে Views ব্যবহার করা যায়। দুটিকে প্রথম release-এ একসঙ্গে support করার প্রয়োজন নেই। Backend interface-কে এমনভাবে করো যাতে ভবিষ্যতে দ্বিতীয় host যোগ করা সম্ভব হয়।

### Runtime integration-এর বিকল্প

1. **Generated Kotlin/Java source:** নীলাং app logic-এর কিছু অংশ Kotlin/Java-তে generate করা। Native tooling-এর সঙ্গে মেলে, কিন্তু language feature-to-target mapping বড় কাজ।
2. **Embedded bytecode VM:** APK-এর মধ্যে compact runtime রেখে `.nilax` bytecode execute করা। Build সহজ হতে পারে, কিন্তু interpreter/VM performance, JIT/AOT policy, debugging, app startup ও security যাচাই করতে হবে।
3. **Native shared library:** Go runtime বা অন্য runtime native library হিসেবে package করে JNI-এর মাধ্যমে host-এর সঙ্গে কথা বলা। ABI, cross compilation, native crash ও memory boundary সামলাতে হবে।

প্রথম MVP-তে যে model সবচেয়ে কম ঝুঁকিপূর্ণ তা বেছে নাও। Android NDK native C/C++ library ও JNI integration-এর জন্য official toolchain সরবরাহ করে: https://developer.android.com/ndk/guides/concepts. NDK-কে ব্যবহার করার কারণ থাকা দরকার; নেটওয়ার্ক বা সাধারণ UI API-র জন্য অকারণে native code দরকার নেই।

### APK, AAB এবং signing

Debug APK স্থানীয় test-এর জন্য উপকারী। Distribution-এর জন্য release signing, package identity, versioning ও store pipeline আলাদা করে যাচাই করবে। Google Play distribution-এ App Signing/Upload Key workflow আছে; build system যেন developer-কে key file বা password source code-এ রাখতে উৎসাহ না দেয়। Official guidance: https://developer.android.com/studio/publish/app-signing

প্রাথমিক acceptance checklist:

- clean machine/CI-তে debug APK build;
- `adb install` ও `adb uninstall`;
- launch, pause, resume, process death ও state restoration;
- back button/system gestures;
- keyboard, screen rotation, font scaling;
- permission grant/deny/revoke;
- network offline/timeout;
- release build এবং signing verification;
- supported ABIs-এ runtime load;
- crash logs ও symbol information;
- generated package-এ secret বা unintended source file নেই।

## ৯.২ iOS backend

iOS backend-কে “Android build-এর আরেকটা target flag” হিসেবে দেখা যাবে না। Xcode project, signing, app capabilities, provisioning এবং Apple distribution workflow আছে। Apple-এর official documentation অনুযায়ী Xcode-ই Apple platform app build, test, optimize ও submission workflow-এর কেন্দ্র; App Store Connect-এ build upload-ও Xcode/সমর্থিত Apple tooling-ভিত্তিক। দেখো: https://developer.apple.com/documentation/xcode এবং https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds

### Host structure

Generated iOS project-এ থাকবে:

- Xcode project/workspace;
- Swift/SwiftUI বা UIKit entry point;
- Nilang runtime framework/static library অথবা generated logic;
- resources/assets catalog;
- app bundle identifier ও deployment target;
- `Info.plist` keys;
- entitlements;
- privacy usage descriptions;
- Swift Package/other pinned dependency integration যেখানে দরকার;
- unit/UI test target;
- debug/release configuration।

### iOS-এর জন্য বাস্তব পথ

একটি সম্ভাব্য first implementation হলো SwiftUI host app, যেখানে নীলাং runtime একটি framework বা C-compatible layer থেকে expose হয়। Swift-এর C/C++ interoperability সম্পর্কে Apple-এর documentation আছে: https://developer.apple.com/documentation/swift/callingapisacrosslanguageboundaries. তবে Go runtime-কে iOS target-এ integrate করা, framework তৈরি করা, simulator/device architectures, Objective-C/Swift bridge এবং App Store runtime constraints আলাদা করে পরীক্ষা করতে হবে। এটিকে এক লাইন cross-compile দিয়ে সমাধান হয়েছে ধরে নেওয়া যাবে না।

**গুরুত্বপূর্ণ পরিবেশগত সীমাবদ্ধতা:** iOS simulator/device build, signing ও official distribution-এর জন্য Apple toolchain দরকার হয়; সাধারণ Windows PC-তে Android-এর মতো পূর্ণ native iOS build workflow ধরে নেওয়া যাবে না। তাই তোমার Windows laptop-এ compiler design, code generation ও Android work করা সম্ভব হলেও iOS release validation-এর জন্য বাস্তব Mac access—নিজের, ধার করা বা নির্ভরযোগ্য CI runner—রোডম্যাপে রাখতে হবে। Kotlin Multiplatform-এর নিজস্ব guide-ও iOS application চালানোর কিছু কাজের জন্য macOS প্রয়োজনীয় বলে উল্লেখ করে: https://kotlinlang.org/docs/multiplatform/multiplatform-integrate-in-existing-app.html

### iOS API-এর বিশেষ ক্ষেত্র

- application lifecycle এবং scene lifecycle;
- safe area ও system navigation;
- permission usage descriptions;
- keychain/secure credential storage;
- notification authorization ও capabilities;
- background task limitations;
- deep/universal link;
- file provider/document picker;
- share sheet;
- privacy manifest ও third-party SDK declarations যেখানে প্রযোজ্য;
- signing, provisioning, archive/export এবং TestFlight/App Store pipeline।

iOS-এ unsupported entitlement বা ভুল privacy purpose declaration build/review-কে বাধা দিতে পারে। Manifest থেকে সব capability স্বয়ংক্রিয়ভাবে যোগ করা উচিত নয়; developer যে capability ব্যবহার করে কেবল সেটিই প্রয়োজন অনুযায়ী configure হবে। Release checklist-এ generated entitlements diff দেখাও।

## ৯.৩ HarmonyOS backend

HarmonyOS NEXT-কে Android-সামঞ্জস্যপূর্ণ আরেকটি target ধরে নেওয়া ভুল হতে পারে। HarmonyOS NEXT-এর official developer platform-এ ArkTS, ArkUI, DevEco Studio, ArkCompiler এবং আলাদা SDK/service kits উল্লেখ করা হয়েছে: https://developer.huawei.com/consumer/en/harmonyos/develop/. ArkTS TypeScript-ধাঁচের syntax ব্যবহার করে, কিন্তু stricter static typing ও HarmonyOS-specific restrictions আছে: https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/arkts-get-started-V5

### প্রথম HarmonyOS backend কী করবে

প্রথম implementation-এ সম্পূর্ণ native code generation-এর বদলে DevEco Studio-তে build হতে পারে এমন generated app project তৈরির দিকে মনোযোগ দাও:

- ArkTS entry/Ability lifecycle host;
- ArkUI components-এর wrapper;
- Nilang logic runtime বা generated logic integration;
- resources ও module metadata;
- declared permissions/capabilities;
- build configuration;
- HAP/app package generation;
- debug deploy ও logging;
- supported API-version matrix।

Official HarmonyOS build docs-এ source/resources/libraries compile, HAP বা app pack packaging এবং signing-সহ build path বর্ণিত আছে: https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V2/build_overview-0000001055075201-V2. তোমার toolchain-কে সেই official pipeline invoke করতে হতে পারে; HAP structure নিজে হাতে তৈরি করে official signing/package validation এড়িয়ে যাওয়া উচিত নয়।

### HarmonyOS integration-এর সিদ্ধান্ত

দুইটি সম্ভাব্য architecture তুলনা করো:

1. **ArkTS host + bytecode/logic bridge:** UI ও lifecycle ArkUI-তে থাকবে; নীলাং logic host-এর সঙ্গে controlled bridge-এর মাধ্যমে কথা বলবে।
2. **Native shared library/runtime:** platform-supported native interfaces দিয়ে VM expose করা হবে; ArkTS host প্রয়োজনীয় native module-কে call করবে।

কোনটি সঠিক হবে তা তোমার নির্দিষ্ট SDK/API version, available NDK/native ABI এবং compiler/runtime-এর compatibility পরীক্ষা করে স্থির করো। HarmonyOS-এ Android APK ইনস্টল হবে—এমন অনুমান করবে না। HAP ও App Pack/Harmony package workflow স্পষ্ট করে implement ও test করতে হবে।

### HarmonyOS acceptance tests

- DevEco Studio/toolchain version pinned;
- debug HAP/build artefact generation;
- official emulator বা supported real device-এ installation;
- Ability lifecycle ও UI restoration;
- ArkUI event থেকে Nilang logic এবং ফিরতি state update;
- permission denial এবং capability absence;
- screen density, font scaling, orientation/window changes;
- signing ও release package validation;
- runtime unload/crash handling;
- app distribution workflow-এর documented output।

### HarmonyOS-এ API parity

Android-এর `Intent`-সদৃশ ধারণা থাকলেই সব platform একই semantics পায় না। natively exposed API-র উপর platform adapter থাকবে; common library কেবল portable intent প্রকাশ করবে। কোনো capability HarmonyOS-এর নির্দিষ্ট SDK-তে পাওয়া না গেলে `UnsupportedCapability` বা alternative flow থাকবে, ভুয়ো success নয়।

---

## ১০. Runtime deployment model: interpreter, bytecode VM, source generation না AOT?

এটি নীলাংয়ের সবচেয়ে বড় technical decision-গুলোর একটি। mobile app-এর ভিতরে language runtime কীভাবে চলবে তা স্থির না করলে UI, package size, performance, debugging, store policy ও portability-এর কাজ দ্বিগুণ হবে।

### ১০.১ Option 1 — Bundle-সহ embedded interpreter/VM

`.nilax`-এর ভিতরে bytecode/metadata/resources থাকবে; app-এর মধ্যে runtime VM তা চালাবে।

**সুবিধা:**
- একই VM একাধিক target-এ ভাগ করা সম্ভব;
- code generation-এর কাজ কম হতে পারে;
- `.nilax` portable logic artifact হিসেবে কাজে লাগে;
- interpreter ও VM-এর বর্তমান কাজ পুনর্ব্যবহার করা সম্ভব।

**ঝুঁকি:**
- startup time, binary size ও memory overhead;
- VM-কে প্রতিটি target architecture-এ build ও test করতে হবে;
- debugging/runtime stack trace source mapping দরকার;
- dynamic loading/update policy platform store rules-এর সঙ্গে মিলিয়ে দেখতে হবে;
- unsafe FFI বা filesystem access sandbox ভাঙতে পারে;
- native UI callbacks এবং garbage collection-এর interaction জটিল।

### ১০.২ Option 2 — Native source code generation

Typed AST/HIR থেকে Kotlin/Swift/ArkTS-এর মতো target source generate করা হবে; official platform compiler চূড়ান্ত app binary তৈরি করবে।

**সুবিধা:**
- platform toolchain-এর native build, optimizer, debugger ও UI ecosystem ব্যবহার;
- runtime VM ছোট করা বা এড়ানোর সম্ভাবনা;
- source-level native debugging সহজ হতে পারে;
- static type check-এ platform compiler সাহায্য করতে পারে।

**ঝুঁকি:**
- Nilang-এর প্রতিটি language feature-এর target-language mapping দরকার;
- generated code জটিল হলে diagnostics user-এর জন্য দুর্বোধ্য হবে;
- language semantic parity বজায় রাখা কঠিন;
- তিনটি target-এর codegen ও escaping আলাদা maintenance burden;
- ArkTS subset, Swift concurrency বা Kotlin language semantics-এর সঙ্গে mismatch হতে পারে।

### ১০.৩ Option 3 — Shared native runtime library

VM/runtime এক বা একাধিক native library হিসেবে compile হবে; platform host layer JNI, Swift/C ABI বা Harmony native boundary দিয়ে runtime চালাবে।

**সুবিধা:**
- core logic ভাগ করা যেতে পারে;
- source code generation-এর চাপ কম;
- pure logic-এর জন্য VM reuse সম্ভব;
- host native UI বজায় রাখে।

**ঝুঁকি:**
- Go cross-compilation, cgo, C ABI, platform linker, symbol visibility ও native crash handling জটিল;
- Swift/Kotlin/ArkTS object-কে runtime VM-এর lifetime-এর সঙ্গে মিলাতে হবে;
- FFI call ঘন ঘন হলে overhead বাড়তে পারে;
- native dependency-এর ABI/version management দরকার।

### ১০.৪ Option 4 — WebAssembly runtime

WASM target থাকলে common logic WASM-এ compile করে browser/WebView-like environment বা embeddable WASM runtime-এ চালানো যায়। এটি portable computation-এর জন্য কার্যকর হতে পারে। তবে WASM একাই iOS/Android/Harmony native app-এর সব UI, lifecycle, permission, background execution ও native service সমস্যার সমাধান করে না। WASM-কে একটি logic backend হিসেবে দেখো, পুরো mobile platform হিসেবে নয়।

### ১০.৫ প্রস্তাবিত সিদ্ধান্ত

প্রথম ছয় মাসে চারটি runtime strategy একসঙ্গে implement কোরো না। বর্তমান codebase ও test maturity audit করে একটি MVP route বেছে নাও। সম্ভাব্য ধাপ:

- interpreter/VM path-কে language reference implementation হিসেবে রাখো;
- native UI host-কে পরিষ্কার bridge API দিয়ে যুক্ত করো;
- Android-এ embedded VM বা generated logic-এর একটি পদ্ধতি real device-এ benchmark করো;
- একই bridge contract-এর iOS/Harmony prototype তৈরি করো;
- performance, APK/package size, startup, memory ও debugging তুলনা করে সিদ্ধান্ত lock করো;
- future AOT/codegen-কে roadmap-এ রাখো, MVP blocker করো না।

Decision record (ADR) লিখবে: কী বেছে নিলে, কোন বিকল্প বাতিল করলে, কী মাপজোখের ভিত্তিতে, এবং ভবিষ্যতে কোন evidence এলে সিদ্ধান্ত পরিবর্তন করবে।

## ১১. Build system ও package pipeline

ভালো mobile compiler-এর প্রধান বৈশিষ্ট্য শুধু code compile করা নয়; developer-কে reproducible, inspectable এবং signed output দেওয়া। নীলাংয়ের CLI-তে `nil build` থাকলে তার অর্থ নির্দিষ্ট করতে হবে: default target কী, কোন SDK খুঁজবে, কোন artifact তৈরি করবে, signing key কোথা থেকে নেবে, failure হলে কোন ধাপ ব্যর্থ হয়েছে, এবং কোন cache পুনর্ব্যবহার হয়েছে।

### ১১.১ Build pipeline-এর ধাপ

একটি standard pipeline এমন হতে পারে:

1. **Discover:** `nil.json`, source roots, resources, selected target এবং toolchain versions শনাক্ত করা।
2. **Validate config:** schema, app ID, minimum OS, permissions, resource paths এবং target compatibility check।
3. **Resolve dependencies:** manifest/lockfile-ভিত্তিক package resolution এবং integrity verification।
4. **Compile/check:** syntax, type check, static diagnostics ও target-specific capability checks।
5. **Lower to IR:** common semantic IR এবং mobile app/UI IR তৈরি।
6. **Generate native host:** platform-specific source, project configuration, resource references, manifest/plist/module metadata তৈরি।
7. **Build runtime:** প্রয়োজন হলে VM/native library target ABI-তে build করা।
8. **Package assets:** icons, fonts, locales, images, `.nilax`/bytecode, metadata এবং native binaries যোগ করা।
9. **Invoke platform toolchain:** official SDK/Gradle, Xcode tooling অথবা DevEco/Harmony build tools।
10. **Validate artifact:** package structure, app identifier, permissions, signing status, ABI, min OS এবং debug-symbol presence পরীক্ষা।
11. **Emit report:** output path, versions, hash, signing mode, warnings, unsupported features এবং reproducibility metadata।

এই ধাপগুলো module হিসেবে আলাদা রাখবে। সব কাজ এক বিশাল `build()` function-এ লিখলে error isolation, unit testing ও নতুন target যোগ করা কঠিন হবে।

### ১১.২ CLI-এর প্রস্তাবিত contract

```text
nil check [project] [--target android|ios|harmony]
nil test [project] [--target ...]
nil run <file-or-project> [--engine interpreter|vm]
nil build <target> [--debug|--release]
nil doctor [--target ...]
nil devices [--target ...]
nil install <target> [--device <id>]
nil logs <target> [--device <id>]
nil clean
nil explain <diagnostic-id>
```

CLI-র exact syntax current command conventions-এর সঙ্গে মিলিয়ে নেবে। এখানে গুরুত্বপূর্ণ নিয়মগুলো হলো:

- `nil check` build-এর আগে actionable diagnostics দেয়;
- `nil doctor` SDK/toolchain presence ও version যাচাই করে;
- `nil build --release` কখনো অজান্তে debug বা unsigned package-কে release বলে না;
- explicit flag ছাড়া private signing credential generate/store করা হয় না;
- build output deterministic এবং machine-readable JSON report-ও দিতে পারে;
- toolchain missing হলে স্পষ্ট installation guide দেয়;
- command exit code CI-তে নির্ভরযোগ্য হয়;
- generated sources output directory-তে থাকে এবং inspect করা যায়।

### ১১.৩ Platform toolchain invocation

একটি নিজস্ব compiler যেন Android Gradle/SDK, Xcode এবং Harmony DevEco build system-এর সব দায়িত্ব নকল করার চেষ্টা না করে। নীলাংয়ের কাজ হবে project generate, code/IR validate, target configuration map এবং official toolchain invoke করা। Native build toolchain তার নিজস্ব package format, signing, linker, platform metadata ও compatibility checks-এর মালিক থাকবে। এতে compatibility ও store submission-এর ঝুঁকি কমে।

একই সঙ্গে version pinning দরকার: Gradle plugin, Android SDK, Xcode, Swift compiler, Harmony SDK ও DevEco plugins-এর tested version table রাখো। “latest version দিয়ে হয়তো চলবে” CI policy নয়। Toolchain update একটি explicit dependency update হবে, automated smoke test-সহ।

### ১১.৪ Reproducible builds ও caches

একই source, lockfile, toolchain এবং build config থেকে ফলাফল যতটা সম্ভব একই হওয়া উচিত। Build metadata-তে রাখো:

- Nilang compiler version ও commit;
- language spec version;
- dependency lockfile hash;
- target SDK/NDK version;
- host toolchain versions;
- build flags;
- resource manifest/hash;
- signing mode (key content নয়);
- debug-symbol mapping version।

Build cache key-তে target SDK, compiler version, feature flag, dependencies এবং generated source inputs থাকতে হবে। শুধু source filename দেখে cache ব্যবহার করলে stale native output তৈরি হবে। Cache corruption বা missing entry হলে safe rebuild হবে; cache integrity ছাড়া executable code trust করা যাবে না।

### ১১.৫ Artifact model

প্রতিটি target-এর জন্য artefact type ও intended use documentation রাখো:

- `.nil`: human-authored source;
- `.nilbc` বা সমজাতীয়: versioned bytecode (যদি ব্যবহার করো);
- `.nilax`: portable Nilang application bundle—এর অর্থ ও সীমা specification-এ স্থির করতে হবে;
- Android debug APK;
- Android release APK/AAB, প্রকল্পের distribution strategy অনুযায়ী;
- iOS build/archive/IPA export, actual tooling workflow অনুযায়ী;
- Harmony HAP/App package;
- debug symbols ও source maps;
- SBOM/dependency report, release workflow-এ যেখানে প্রয়োজন।

`.nilax`-কে কোনোভাবেই নিজে থেকে Android APK, IPA বা HAP-এর বিকল্প বলবে না। এটি app logic/asset bundle হতে পারে; platform installable package আলাদা।

### ১১.৬ Bundle format-এর স্থায়িত্ব

ZIP-ভিত্তিক archive হলে ZIP ফাইলের উপর শুধু নির্ভর করলে হবে না। Bundle header-এ থাকবে:

- magic bytes ও format version;
- Nilang language/runtime ABI version;
- application ID ও version;
- entry point;
- resource manifest;
- bytecode/asset digests;
- target constraints;
- capability requirements;
- signature metadata;
- optional debug metadata reference।

Parser-এ malicious archive handling, path traversal (`../`), duplicate path, decompression bomb, oversized file, invalid UTF-8 manifest, checksum mismatch ও unknown version-এর tests থাকতে হবে। `zip-slip`-সদৃশ path exploit ঠেকাতে extracted path normalized destination directory-এর ভিতরে আছে কি না validate করতে হবে। Bundle verifier “signature valid” বলার আগে signer trust policy/identity-ও যাচাই করবে; signature valid হওয়া মানেই publisher trusted নয়।

### ১১.৭ Signing ও key lifecycle

`nilkey` বা Ed25519 package signing tool থাকলে সেটা Nilang bundle authenticity কাজে লাগাতে পারে। কিন্তু platform signing keys ও store account credentials-এর জীবনচক্র আলাদা। Android app signing, Apple code signing/provisioning এবং Harmony signing আলাদা policy অনুসরণ করে। একটির key দিয়ে অন্যটির app sign করা যায়—এমন ধারণা করবে না।

- development key এবং release key আলাদা;
- private key repository-তে commit নয়;
- passphrase log/env dump-এ প্রকাশ নয়;
- CI secret vault ব্যবহার;
- key rotation ও compromise procedure document;
- signature verification-এর positive ও negative tests;
- tampered bundle/modified resource detection;
- release artefact checksum এবং provenance;
- signing failure-এ fail closed।

### ১১.৮ Project template ও migration

`nil init`-এ template বেছে নিতে পারবে: `console`, `mobile`, `mobile-library`, `web`, `game`, `service`। `mobile` template যেন অকারণে SoftBus daemon, database server, AI oracle, game engine বা backend microservice যোগ না করে। নতুন প্রকল্পের dependency footprint ছোট রাখো।

Project migration tool (যেমন `nil migrate`) পরে প্রয়োজন হতে পারে। Manifest schema v1 থেকে v2-তে গেলে compiler স্পষ্ট migration proposal দেবে; পুরোনো config-এর meaning নীরবে বদলাবে না।

---

## ১২. Capability security ও permission model

মোবাইল অ্যাপের ভাষা হিসেবে নীলাংকে নিরাপত্তার দিক থেকে default-deny model নেওয়া উচিত। একটি `.nil` file যদি স্বয়ংক্রিয়ভাবে filesystem, network, camera, contacts, background execution এবং subprocess access পায়, তবে package signing থাকলেও তার ক্ষমতা সীমাহীন থাকবে।

### ১২.১ Compile-time capability declaration

App config বা source manifest-এ explicit capabilities থাকবে, যেমন:

- `network`;
- `local-storage`;
- `documents-picker`;
- `camera`;
- `microphone`;
- `location-foreground`;
- `location-background`;
- `notifications`;
- `bluetooth`;
- `biometrics`।

Capability declaration-এর অর্থ সরাসরি OS permission granted নয়। এটি বোঝাবে app capability ব্যবহার করতে চায়, এবং generator-এর উচিত উপযুক্ত platform metadata emit করা। Runtime-এ actual permission check ও request তবু লাগবে। Camera না-ব্যবহার করা app-এ camera permission যোগ হওয়া উচিত নয়।

### ১২.২ Capability check এবং sandbox

একটি file access API শুধু `read(path)` নয়; path কোন storage root-এ, app-private নাকি user-selected, access lifetime কতক্ষণ এবং permission কী—এসব জানাতে হবে। একইভাবে network access-এ allowed endpoint policy লাগতে পারে enterprise/sandbox profile-এর জন্য।

প্রস্তাবিত model:

- `CapabilitySet` runtime context-এর অংশ;
- standard library capability token ছাড়া restricted operation করতে পারবে না;
- native bridge প্রতিটি sensitive call-এ capability validate করবে;
- capability denial typed error হবে;
- host injection/FFI path দিয়ে sandbox bypass করার test থাকবে;
- dev mode-এ overly broad capability declaration warning;
- release build-এ শুধু প্রয়োজনীয় capability package-এ রাখা।

Compile-time check যথেষ্ট নয়; malicious/incompatible bundle runtime-এও validate করতে হবে। Runtime API-র ভিতরে check না করে UI-তে permission hide করলেই নিরাপত্তা হয় না।

### ১২.৩ Unsafe code এবং native extension

Native extension দরকার হবে, কারণ কোনো app একদিন এমন SDK চাইতে পারে যা common API-তে নেই। কিন্তু native escape hatch-কে স্পষ্ট boundary-তে রাখো:

- extension-এর manifest-এ platforms ও permissions;
- declared exported methods/types;
- versioned ABI;
- trusted dependency identity;
- restricted capability profile;
- crash/error boundary;
- async callback contract;
- compile-time error যদি target extension অনুপস্থিত;
- extension-এর নিজস্ব integration tests।

Native extension-এর ভিতরের code নীলাং sandbox সম্পূর্ণভাবে নিয়ন্ত্রণ করতে পারবে না—এই সীমা documentation-এ স্পষ্ট করবে।

### ১২.৪ Package security

Package manager-এর কাজ শুধু download/install নয়। Supply-chain security এর মধ্যে dependency pinning, integrity, publisher, signing key, provenance, vulnerability report, yanked release handling এবং package permissions review থাকবে।

- floating `latest` dependency release build-এ অনুপযুক্ত;
- lockfile commit করা উচিত;
- registry TLS verification বাধ্যতামূলক;
- archive extraction safe;
- duplicate package name/typosquatting-এর বিরুদ্ধে namespace policy;
- package install-এর আগে requested capability দেখানো;
- local path dependency-এর build provenance recorded;
- dependency update-এ automated tests;
- release-এ dependency inventory/SBOM export option।

### ১২.৫ Secrets ও personal data

API key, password, refresh token, payment data বা personal records log-এ প্রকাশ করা যাবে না। `puts()` বা generic logger production data-কে অকারণে expose করতে পারে। Logger-এ sensitivity level এবং redaction utilities থাকবে; তবু developer-কে sensitive value logging না করার নিয়ম শেখাতে হবে। Encryption-এর জন্য standard OS crypto APIs বা well-reviewed libraries ব্যবহার করো। নিজস্ব crypto algorithm বা “encrypted” নামে custom reversible encoding তৈরি করো না।

### ১২.৬ Threat model

প্রতিটি release-এর আগে অন্তত এই attacker scenarios বিবেচনা করো:

1. malicious `.nilax` bundle;
2. tampered resource/bytecode;
3. compromised package registry/dependency;
4. malicious native extension;
5. crafted deep link বা intent;
6. path traversal এবং archive bomb;
7. unexpected network response;
8. permission revoke/process death race condition;
9. leaked debug server/secrets;
10. crash logs-এ personal data leak;
11. stale cache থেকে পুরোনো executable code;
12. signing key compromise।

Threat model একটি ২-পৃষ্ঠার document হলেও উপকারী, যদি প্রতিটি risk-এর mitigation ও test থাকে। Security claim “safe by design” হিসেবে লিখবে না যতক্ষণ না threat model, code review ও test evidence আছে।

---

## ১৩. Editor, LSP, diagnostics ও developer experience

একটি ভাষা ব্যবহারযোগ্য হয় তখনই, যখন লেখার সময় সঠিক error, autocomplete, navigation ও debugging পাওয়া যায়। নীলাংয়ের mobile backend যতই উন্নত হোক, diagnostics দুর্বোধ্য হলে নতুন developer platform ছেড়ে দেবে।

### ১৩.১ Language Server Protocol

একটি `nil-language-server` (বা CLI-এর অন্তর্নির্মিত LSP mode) পরিকল্পনা করো:

- syntax diagnostics;
- type errors ও related information;
- hover/type information;
- completion/import suggestions;
- go to definition;
- find references;
- rename symbol;
- document symbols/workspace symbols;
- code actions/quick fixes;
- formatting;
- semantic highlighting;
- project-aware target capability diagnostics;
- generated native code mapping।

LSP যেন compiler-এর আলাদা, নকল semantic engine না হয়ে একই frontend library reuse করে। নয়তো editor-এ “valid” দেখাবে, কিন্তু build fail করবে।

### ১৩.২ Diagnostics format

প্রতিটি diagnostic-এ থাকতে হবে:

- stable code, যেমন `NIL-TYPE-0042`;
- severity;
- source span/related span;
- plain-language explanation;
- cause এবং suggested fix;
- platform/target context;
- docs URL বা local documentation anchor;
- machine-readable JSON form।

খারাপ diagnostic: `invalid value`। ভালো diagnostic: `camera.capture()` requires the `camera` capability, but the project has not declared it. Declare the capability in `nil.json`, or remove the call for targets that do not support it.


Diagnostics-এর code স্থিতিশীল থাকলে editor, CI ও documentation একই error-এর সঙ্গে সংযোগ রাখতে পারবে। Compiler-এর internal stack trace সাধারণ user-এর error message নয়; debugging mode-এ অতিরিক্ত details দেয়া যেতে পারে।

### ১৩.৩ Formatter ও lint

একটি canonical formatter রাখো যাতে source code diff ছোট ও consistent হয়। `nil fmt --check` CI-তে formatting violation জানাবে; `nil fmt` file পরিবর্তন করবে। Formatter যেন comments, Bengali string literal, Unicode identifiers এবং embedded data নষ্ট না করে।

Linter rule-গুলো severity-সহ configurable হবে:

- unused variable/import;
- unreachable code;
- unhandled Result/error;
- blocking call in UI/main context;
- unbounded retry;
- missing cancellation;
- missing accessibility label;
- broad permission/capability not used;
- target-specific API without guard;
- hard-coded secret suspicion;
- oversized resource;
- deprecated API;
- unsafe native extension;
- unsupported UI component on selected target।

সব lint rule-কে compile error বানিও না। Style issue warning হতে পারে; unsafe capability বা impossible target call hard error হওয়া উচিত। Policy project config-এ explicit হবে।

### ১৩.৪ Debugger ও source mapping

যদি app VM-এ চলে, stack trace-এ `.nil`-এর file/line/function দেখাতে হবে—VM bytecode instruction offset নয়। Compiler source map তৈরি করবে। Optimized build-এ inline/constant folding-এর কারণে exact line mapping জটিল হয়, তাই original span metadata রাখা দরকার।

Debugger-এর MVP feature:

- breakpoint;
- step over/into/out;
- stack frames;
- local variables/watch values;
- exception breakpoint;
- async task list;
- event trace;
- runtime logs;
- native bridge call trace;
- source-to-bytecode/host-code mapping।

সবকিছু একসঙ্গে না পারলে প্রথমে deterministic stack trace, source locations এবং structured logging করো। তারপর remote debugging। Mobile debug server default release build-এ চালু থাকবে না। Developer যদি network debugging enable করে, local-only binding, authentication, timeout এবং explicit opt-in চাইবে।

### ১৩.৫ Profiler ও diagnostics bundle

`nil doctor` এবং `nil profile` এক জিনিস নয়। `doctor` build/toolchain setup; `profile` runtime performance। পরে profiler দিতে পারো:

- startup phase timings;
- VM/bridge call counts;
- memory allocations;
- GC pauses, যদি GC থাকে;
- UI render/layout times;
- frame drops;
- network request durations;
- storage transaction timings;
- task cancellation leaks;
- app package-size report।

Profiler-এর ডেটা app-এর sensitive content প্রকাশ করতে পারে; redaction ও explicit capture policy দরকার। Crash report দিলে user consent এবং privacy controls বিবেচনা করবে।

### ১৩.৬ Editor integration

প্রথমে VS Code extension-এর জন্য syntax highlighting, snippets, language server, project task ও debug configuration তৈরি করা যথেষ্ট। IntelliJ/Neovim/অন্যান্য editor পরে যোগ করা যায়। Syntax highlight rules-এর সঙ্গে language spec মিলিয়ে রাখো; keyword বদলালে extension test update হবে। Generated Kotlin, Swift ও ArkTS file developer দেখতে পারবে এবং চাইলে native IDE-তে inspect করতে পারবে।

---

## ১৪. Testing strategy: ভাষা থেকে বাস্তব ফোন পর্যন্ত

ভাষা ও mobile framework-এর পরীক্ষা একধরনের নয়। unit test পাস করলেই native app ঠিক আছে—এমন নয়; একটি device-এ app চাললেই সব supported OS version ঠিক—এমনও নয়। আলাদা test layer রাখো, যাতে failure কোথায় হয়েছে বোঝা যায়।

### ১৪.১ Test pyramid

**Layer 1 — Compiler/language unit tests:** lexer, parser, AST, type checker, IR lowering, codegen, diagnostics ও formatter। দ্রুত চলবে, প্রতিটি commit-এ সম্ভব হলে সব test।

**Layer 2 — Semantic conformance tests:** একই `.nil` program interpreter, VM, WASM এবং future mobile backend-এ চালিয়ে expected result তুলনা। Bytecode serialization/version mismatch, Unicode, numeric edge cases এবং error semantics-এর test এখানেই।

**Layer 3 — Standard library tests:** fake HTTP server, temporary storage, database in-memory mode, cancellation, serialization ও locale formatting। Network test যেন বাস্তব internet-এর উপর অনির্ভরশীল থাকে।

**Layer 4 — Host adapter tests:** mock Android/iOS/Harmony service-এর সাহায্যে UI event, native API bridge, permission state এবং lifecycle mapping।

**Layer 5 — Generated project tests:** clean checkout থেকে platform project generate, platform build tool run, artefact validate। Generated manifest/plist/Gradle/ArkTS configuration-এর snapshot বা structural test থাকতে পারে।

**Layer 6 — Emulator/simulator tests:** UI smoke test, navigation, keyboard, rotation/window change, app relaunch, process recreation ও permissions।

**Layer 7 — Real-device tests:** অন্তত প্রতিটি platform-এর একটি supported physical device দিয়ে camera/notification/biometric/keyboard/lifecycle-এর বাস্তব behavior যাচাই। Emulator device-specific API বা hardware behavior সম্পূর্ণ প্রমাণ করতে পারে না।

**Layer 8 — Release and store pipeline test:** signed release artifact, metadata validation, package install/update, crash logs, version migration এবং internal/beta distribution workflow।

### ১৪.২ Test corpus ও golden fixtures

প্রতিটি ভাষাগত feature-এর জন্য program fixture রাখো। উদাহরণ:

```text
tests/conformance/
├── literals/
├── arithmetic/
├── strings-unicode/
├── closures/
├── types-generics/
├── errors-results/
├── async-cancel/
├── module-imports/
├── serialization/
└── native-bridge/
```

একটি fixture-এ থাকতে পারে source, expected stdout, expected result, expected diagnostic code, supported engines এবং bug reference। নতুন regression fix করলে একটি test যোগ করো; bug শুধু হাতে device-এ ঠিক করে ছেড়ে দিও না।

### ১৪.৩ Cross-platform parity suite

একটি shared UI test app থাকবে: counter, login form validation (real credential ছাড়াই), list scroll, settings, local persistence, deep link route, permission explanation ও error state। Android, iOS এবং HarmonyOS target-এ একই semantic test specification চালানো হবে, কিন্তু screenshot pixel-identical হতে বাধ্য নয়। Native typography ও system control-এ পার্থক্য থাকতে পারে। পরিবর্তে যাচাই করো:

- একই action একই logical state তৈরি করে;
- screen reader-এর কাছে semantic roles থাকে;
- user input হারায় না;
- lifecycle-এর পর state policy অনুযায়ী restore হয়;
- navigation/back behavior platform convention মেনে চলে;
- denied permission-এ crash হয় না;
- unavailable feature-এ graceful error হয়;
- no-network অবস্থায় UI freeze হয় না।

### ১৪.৪ Device matrix

একটি বাস্তব device matrix-এ প্রতিটি target-এর জন্য minimum supported OS, বর্তমান supported OS, low-memory device, ভিন্ন screen size/density, locale এবং architecture বিবেচনা করো। একসঙ্গে শত device লাগবে না; বরং representative coverage নির্বাচন করো।

প্রস্তাবিত test dimensions:

- OS/API version: minimum, latest stable, প্রয়োজনে beta smoke test;
- architecture: supported physical ARM64 এবং emulator architecture;
- form factor: ছোট ফোন, বড় ফোন/tablet/foldable যেখানে প্রাসঙ্গিক;
- language: Bengali, English, লম্বা text ও RTL smoke test;
- font scaling: default ও বড় scale;
- theme: light/dark, high contrast যেখানে platform support করে;
- network: offline, slow, timeout, TLS failure;
- power: battery saver/background restriction;
- permissions: granted, denied, limited/restricted, revoked;
- storage: low disk, missing file, corrupted record;
- update: existing app data সহ নতুন version install।

### ১৪.৫ Snapshot test-এর সীমা

Screenshot test উপকারী, কিন্তু native UI-তে OS/font version, rendering antialiasing, localization ও system inset অনুযায়ী image বদলাতে পারে। Pixel-perfect snapshot-কে একমাত্র quality metric করো না। Structural UI assertion, accessibility tree, action/state behavior এবং screenshot diff একসঙ্গে ব্যবহার করো। Stable visual fixture-এর জন্য screen size, OS, font scale ও theme pin করতে হবে।

### ১৪.৬ Performance budget

“Fast” বা “60 FPS” একটি slogan নয়। প্রতিটি test device class-এ target metric ঠিক করো:

- cold start time;
- warm start time;
- first usable screen;
- bundle/runtime initialization;
- memory after launch;
- peak memory for list screen;
- idle battery/network activity;
- UI response after tap;
- scrolling frame drop;
- package size;
- app install/upgrade duration;
- crash-free session rate, beta scale যথেষ্ট হলে।

প্রথমে baseline মাপো। তারপর প্রতি release-এ regression threshold দাও। একেবারে zero overhead বা সব ফোনে 60 FPS-এর দাবি বাস্তবসম্মত নয়।

---

## ১৫. CI/CD, repository discipline এবং releases

নতুন platform backend এলে regression-এর সংখ্যা বাড়ে। তাই manual testing-এর উপর নির্ভর না করে GitHub Actions বা অন্য CI-তে reproducible pipeline গঠন করো।

### ১৫.১ প্রতিটি pull request-এর ন্যূনতম CI

- format/check;
- `go test ./...` এবং project-specific test commands;
- static analysis/vet/lint;
- compiler conformance corpus;
- race test যেখানে runtime concurrency ব্যবহার করে;
- fuzz test-এর bounded run;
- `nil check` ও project template smoke test;
- dependency/licence/security scan;
- bundle verifier tests;
- generated-source golden tests।

Android SDK runner-এ debug build ও basic emulator test যোগ করো। iOS/Harmony build-এর জন্য প্রয়োজনীয় hosted macOS/compatible runner এবং DevEco environment বাস্তবে configure না করা পর্যন্ত তাদের CI green badge দিও না। “CI-তে Android passed” থেকে “all mobile targets passed” সিদ্ধান্ত টানা যাবে না।

### ১৫.২ Nightly/weekly CI

- longer fuzzing;
- full conformance across engines;
- package-size report;
- memory/leak tests;
- full Android emulator matrix;
- available iOS simulator tests;
- available Harmony emulator/device tests;
- release-build reproducibility check;
- dependency update PR tests।

Nightly workflow ব্যর্থ হলে issue তৈরি হতে পারে, কিন্তু automated update কোনো signing secret বা production release নিজে থেকে পরিবর্তন করবে না।

### ১৫.৩ Release train

Semantic versioning ব্যবহার করার কথা বিবেচনা করো, তবে language runtime, compiler, bytecode format, standard library এবং mobile SDK-র versioning সম্পর্ক নথিভুক্ত হওয়া চাই। উদাহরণস্বরূপ:

- language semantic major/minor;
- compiler version;
- `.nilax` format version;
- VM/runtime ABI version;
- platform backend version;
- standard library API version।

এই সব version একে অপরের সঙ্গে সম্পূর্ণ স্বাধীন না হলেও চলবে, কিন্তু compatibility rules পরিষ্কার না হলে নতুন compiler পুরোনো bundle চালাবে কি না বোঝা যাবে না। Stable release-এর আগে release candidate-এ conformance corpus ও device matrix চালাও।

### ১৫.৪ Bug report template

Issue template-এ জিজ্ঞেস করো:

- নীলাং/CLI/compiler version;
- OS/platform version;
- selected target;
- minimal source reproducer;
- expected behavior;
- actual behavior;
- `nil doctor` output (secret scrubbed);
- debug logs/crash trace;
- generated project/toolchain versions;
- reproducer interpreter, VM না native target-এ ব্যর্থ;
- permission/lifecycle condition।

User-এর ব্যক্তিগত data, signing key বা production API secret issue-তে দিতে বলবে না। Bug report সরলীকরণে সাহায্য করতে `nil report --redact` utility পরে যোগ করা যায়।

### ১৫.৫ Open-source contribution process

তুমি একা শুরু করলেও coding rules, architecture decision records (ADR), contribution guide, issue labels এবং reviewer checklist রাখো। Feature PR-এ অন্তত spec, tests, error handling, docs, platform behavior এবং backward-compatibility note থাকবে। `main` branch-এর status green এবং release tagging reproducible রাখো।

---

## ১৬. প্রস্তাবিত repository structure

বর্তমান repo structure সম্পূর্ণ বদলে ফেলার প্রয়োজন নেই। নতুন mobile components-কে স্পষ্ট boundary-তে যোগ করো। উদাহরণ:

```text
nilLang/
├── cmd/
│   ├── nil/
│   ├── nilc/
│   ├── nil-runner/
│   ├── nilpkg/
│   └── nilkey/
├── compiler/
│   ├── lexer/
│   ├── parser/
│   ├── ast/
│   ├── types/
│   ├── typecheck/
│   ├── hir/
│   ├── mir/
│   ├── mobileir/             # প্রস্তাবিত: app semantics/IR
│   ├── uiir/                 # প্রস্তাবিত: typed UI tree
│   ├── codegen/
│   │   ├── android/
│   │   ├── ios/
│   │   └── harmony/
│   └── diagnostics/
├── runtime/
│   ├── vm/
│   ├── abi/
│   ├── mobilehost/
│   └── platform/
│       ├── android/
│       ├── ios/
│       └── harmony/
├── pkg/
│   ├── stdlib/
│   ├── alap/
│   ├── mobile/
│   ├── bundle/
│   ├── signing/
│   └── capability/
├── templates/
│   ├── mobile-common/
│   ├── android-host/
│   ├── ios-host/
│   └── harmony-host/
├── examples/
│   ├── hello-mobile/
│   ├── counter/
│   ├── notes-local/
│   └── native-extension-demo/
├── tests/
│   ├── conformance/
│   ├── integration/
│   ├── bundle-security/
│   └── mobile-parity/
├── docs/
│   ├── spec/
│   ├── platform-matrix.md
│   ├── mobile-architecture.md
│   ├── security-model.md
│   ├── native-extension-guide.md
│   └── release-checklist.md
└── .github/workflows/
```

এটি প্রস্তাবিত logical map, প্রত্যেক directory এখনই তৈরি করতে হবে না। বর্তমান structure-এ যেসব module ইতিমধ্যে একই দায়িত্ব নেয়, নতুন duplicate package না করে সেগুলো ব্যবহার করো। Directory naming-এর চেয়ে dependency boundaries ও package ownership বেশি গুরুত্বপূর্ণ।

### ১৬.১ Dependency graph-এর নিয়ম

- `compiler/` platform SDK import করবে না;
- `uiir/` Android/Swift/ArkTS-specific native types জানবে না;
- `codegen/android` কেবল Android backend-এর responsibilities জানবে;
- `runtime/abi` language internal object layout সরাসরি expose করবে না;
- `stdlib/core` platform package-এর উপর নির্ভর করবে না;
- `native extension` project-এর সীমা ও trust boundary পার হলে versioned API ব্যবহার করবে;
- `cmd/nil` package build orchestration করবে, language semantics নিজে implement করবে না।

### ১৬.২ Native templates update policy

Generated template থেকে developer project তৈরি হয়ে গেলে template change সেই project-কে স্বয়ংক্রিয়ভাবে overwrite করবে না। Template version record করো। `nil upgrade` migration suggestions দেখাবে; user-এর native customization মুছে দেবে না। Generated file এবং developer-editable file আলাদা mark করা যায়, যেমন header comment, manifest marker বা generator metadata দিয়ে।

---

## ১৭. বাস্তবায়নের পর্যায়ভিত্তিক রোডম্যাপ

নীলাংকে mobile-ready করার জন্য একটি giant milestone নয়, বরং নির্দিষ্ট exit criteria-সহ ছোট release train দরকার। নিচের সময়সীমাগুলো পরিকল্পনার আনুমানিক ধারণা; একজন part-time maintainer বা অনির্দিষ্ট toolchain access থাকলে সময় অনেক বাড়তে পারে। সময়ের চেয়ে completion criteria গুরুত্বপূর্ণ। একটি phase-এর acceptance test পূরণ না হলে পরের phase-এ যাওয়ার আগে blocker ঠিক করো।

## Phase 0 — Audit ও product boundary

**লক্ষ্য:** কী আছে, কী নেই, কোন architecture বেছে নেওয়া হচ্ছে তার evidence-based inventory তৈরি।

কাজ:

1. `README.md`-তে দাবি করা প্রত্যেক feature-এর code path চিহ্নিত করা।
2. `go test ./...` চালিয়ে বর্তমান test baseline নথিভুক্ত করা।
3. CLI command-গুলোর বাস্তব output ও failure behavior যাচাই করা।
4. বর্তমান `.nilax` format, bytecode version, VM/runtime dependency audit করা।
5. `nil build android`-এর generated APK debug/unsigned/signed অবস্থায় কী করতে পারে, real device-এ পরীক্ষা করা।
6. বর্তমান Alap UI-এর AST/IR/output model বোঝা—শুধু browser preview নাকি semantic tree আছে।
7. Android, iOS এবং HarmonyOS-এর expected minimum support range লিখে রাখা।
8. `docs/platform-capabilities.md`, `docs/mobile-architecture.md` এবং ADR তৈরি।
9. “পূর্ণাঙ্গ mobile support”-এর সংজ্ঞা ও MVP scope অনুমোদন।

**সমাপ্তির শর্ত:** প্রতিটি দাবি `implemented`, `prototype`, `untested` বা `not implemented` হিসেবে evidence-সহ mark করা। Architecture decision record-এ native UI adapter approach কেন নেওয়া হচ্ছে তা লেখা। Existing baseline test reproducible।

**এই পর্যায়ে যা করবে না:** নতুন UI widget-এর লম্বা তালিকা, GPU renderer, custom AOT compiler, নতুন package registry feature।

## Phase 1 — Language semantics ও reproducibility

**লক্ষ্য:** একই program interpreter/VM-এ একই semantics দিচ্ছে এবং compiler error debug করা যায়।

কাজ:

1. language spec versioning চালু করা।
2. formatter, conformance fixtures ও stable diagnostic codes তৈরি।
3. runtime/bytecode compatibility policy নির্ধারণ।
4. error handling ও typed Result semantics স্থিতিশীল করা।
5. Unicode/string/number edge-case tests যোগ।
6. bundle format version ও integrity-check tests লেখা।
7. `nil check`, `nil test`, `nil doctor`-এর CLI contract স্থির করা।
8. dependency lockfile ও reproducible build metadata যোগ।

**সমাপ্তির শর্ত:** test corpus-এর প্রতিটি program deterministic expected behavior দেয়; malformed source-এ panic নয়, diagnostics আসে; corrupted bundle safeভাবে reject হয়; build report target/toolchain/version জানায়।

## Phase 2 — Minimal mobile runtime host

**লক্ষ্য:** native host app থেকে Nilang runtime initialize, execute, event dispatch ও safely shutdown করা।

কাজ:

1. runtime ABI বা embedding interface স্থির করা;
2. `create/load/call/dispatch/dispose` lifecycle design;
3. string/bytes/number/object conversion rules;
4. native call error mapping;
5. background thread এবং main UI thread-এর স্পষ্ট নিয়ম;
6. callback cancellation ও cleanup;
7. runtime shutdown ও process interruption simulation;
8. source stack trace/source mapping;
9. minimal hello/counter host integration;
10. VM startup/memory profiling।

**সমাপ্তির শর্ত:** একটি native host app Nilang code চালায়, host থেকে event পাঠায়, Nilang state update করে, ফলাফল UI-তে ফেরে, error stack trace-এ `.nil` file/line দেখা যায় এবং app পুনরায় খুললে lifecycle crash হয় না।

## Phase 3 — Android vertical slice

**লক্ষ্য:** নীলাং সোর্স থেকে installable debug Android app তৈরি ও বাস্তব ফোনে চালানো।

MVP features:

- `nil init --template mobile`;
- Text/Button/Column/Row/TextField;
- `state` ও event binding;
- screen navigation;
- local key-value storage বা একেবারে ছোট database;
- permission status/request abstraction;
- deep link-এর minimal handling;
- Android back/navigation semantics;
- generated Gradle project;
- debug APK build, install, logs ও uninstall;
- debug-symbol/source mapping;
- emulator smoke test।

**সমাপ্তির শর্ত:** clean checkout থেকে `nil build android --debug` চালিয়ে package তৈরি, `adb install` দিয়ে supported physical phone-এ install ও launch, basic interaction, state persistence, app process recreate, permission denial এবং error route test পাস করে। SDK অনুপস্থিত থাকলে clear diagnostics আসে। Debug APK-কে release বলে উপস্থাপন করা হয় না।

## Phase 4 — Alap UI core + common mobile APIs

**লক্ষ্য:** Android demo-কে প্রকৃত reusable framework-এ রূপ দেওয়া।

কাজ:

1. typed UI IR;
2. stable component identity;
3. layout constraints ও safe area;
4. TextField composition/keyboard behavior;
5. accessibility roles/labels;
6. theme tokens ও dark mode;
7. responsive/adaptive layout;
8. HTTP client, JSON এবং cancellation;
9. local database/storage contract;
10. localization catalog;
11. component-level test harness;
12. performance baseline ও package-size report।

**সমাপ্তির শর্ত:** একই sample app-এ UI state, form, navigation, database ও network behavior deterministic test দিয়ে যাচাই হয়; UI-এর accessibility smoke test পাস করে; scroll/input-এ crash বা unbounded memory growth নেই।

## Phase 5 — iOS proof of concept

**লক্ষ্য:** একটি minimal Nilang mobile app official Apple toolchain দিয়ে build ও iOS simulator/physical device-এ চালানো।

কাজ:

1. macOS/Xcode access ব্যবস্থা;
2. SwiftUI/UIKit host template;
3. runtime ABI/framework integration;
4. generated assets ও app metadata;
5. lifecycle, scene events ও state restoration;
6. native UI component mapping;
7. Xcode debug build ও simulator launch;
8. signing/provisioning requirements documentation;
9. TestFlight/internal testing workflow-এর prototype;
10. iOS-specific `doctor` checks।

**সমাপ্তির শর্ত:** Apple toolchain থেকে তৈরি, valid signing configuration-সহ installable development build-এ একই counter/notes demo চলে; source trace ও native error mapping কাজ করে; iOS-only feature না থাকলেও common app gracefulভাবে চলে। শুধু `.ipa` extension-সহ ফাইল তৈরি হওয়াকে success ধরা হবে না।

## Phase 6 — HarmonyOS proof of concept

**লক্ষ্য:** ArkTS/ArkUI host-এ একই logical app চালানো, supported SDK path-এ HAP package তৈরি।

কাজ:

1. official HarmonyOS/DevEco target version নির্ধারণ;
2. generated ArkTS host ও ArkUI mapping;
3. Nilang VM/logic bridge;
4. entry Ability/lifecycle event mapping;
5. resource/manifest/permission generation;
6. HAP build/install/debug;
7. runtime error logging;
8. API parity matrix;
9. unsupported features graceful handling;
10. signing/release pipeline documentation।

**সমাপ্তির শর্ত:** official toolchain দিয়ে HAP তৈরি, supported emulator বা বাস্তব device-এ install, UI event থেকে Nilang logic execute, state update, navigation ও error handling test পাস করে। Android package-কে Harmony package হিসেবে পুনরায় নামিয়ে success ঘোষণা করা যাবে না।

## Phase 7 — Security, stability ও release readiness

**লক্ষ্য:** prototype-কে beta-quality platform-এ উন্নীত করা।

কাজ:

- capability sandbox;
- dependency lockfile ও signature verification;
- archive security tests;
- release signing separation;
- crash reporting ও secret redaction;
- lifecycle/process death tests;
- app upgrade/data migration;
- accessibility audit;
- minimum/maximum OS matrix;
- performance budget;
- CI reproducibility;
- native extension API versioning;
- docs and migration guide।

**সমাপ্তির শর্ত:** কোনো critical security test failing নয়; release artefact signature/metadata validation পাস; minimum supported OS-এ smoke test; upgrade ও rollback/data migration test; known limitations প্রকাশিত।

## Phase 8 — Ecosystem ও advanced capability

Core mobile app support স্থিতিশীল হলে ধাপে ধাপে যোগ করো:

- notification/push;
- camera/photo/video;
- location/Bluetooth/NFC;
- secure storage/biometric;
- background tasks;
- app widget/extensions;
- in-app purchase যেখানে লক্ষ্য product-এর দরকার;
- deep links/universal links;
- platform-specific accessibility extensions;
- app distribution/analytics integrations;
- compiler optimization/AOT;
- hot reload, যেখানে নিরাপদ ও বাস্তবসম্মত;
- package registry policy ও verified publisher।

এই features-কে “mobile support complete” হওয়ার পূর্বশর্ত হিসেবে একবারে যোগ কোরো না। আগে supported baseline stable; পরে feature set বাড়বে।

---

## ১৮. একক ডেভেলপার হিসেবে কাজের অগ্রাধিকার

একজন ডেভেলপার হিসেবে তোমার সময়, অর্থ, hardware এবং testing access সীমিত হতে পারে। সেক্ষেত্রে সবচেয়ে কার্যকর strategy হলো **একটি vertical slice, তারপর একটি নির্দিষ্ট platform-এ production-quality baseline, এরপর দ্বিতীয় ও তৃতীয় platform**। একসঙ্গে তিন backend-এর অর্ধেক implementation করলে কোনোটি বাস্তব app developer ব্যবহার করতে পারবে না।

### ১৮.১ Priority P0 — না করলে mobile app হবে না

- language/runtime semantics consistency;
- `.nilax` format/version validation;
- manifest schema;
- native host/runtime bridge;
- native UI adapter-এর minimal set;
- Android APK debug build ও real-device testing;
- lifecycle এবং state restoration;
- error reporting/source mapping;
- reproducible build এবং docs।

### ১৮.২ Priority P1 — app usable করতে প্রয়োজন

- navigation;
- form/input/keyboard;
- local data persistence;
- HTTP/JSON;
- permission API;
- accessibility basics;
- responsive layout;
- release signing workflow;
- CI integration tests;
- iOS ও Harmony minimal host prototype।

### ১৮.৩ Priority P2 — product maturity

- robust dependency management;
- complete debug tooling/LSP;
- comprehensive localization;
- crash analytics;
- profiler;
- secure storage;
- deep linking;
- notification handling;
- visual test tooling;
- package-size optimization।

### ১৮.৪ Priority P3 — পরে করলেও চলে

- নিজের graphics engine-কে সব UI-র default করা;
- full AOT optimizer;
- all-OS widgets/extensions;
- universal hot reload;
- complex distributed SoftBus integration into every mobile app;
- large package registry marketplace;
- game engine replacements;
- AI code generation as compiler requirement;
- custom cryptographic algorithms;
- every device capability at once।

### ১৮.৫ সপ্তাহের পরিকল্পনা কীভাবে রাখবে

তোমার কাজকে “আজ camera API, কাল new type system, পরশু iOS build” করে ছড়িয়ে দিও না। ১–২ সপ্তাহের milestone ধরো:

- একটি issue = একটি observable behavior;
- test লিখে তারপর implementation;
- একবারে একটি vertical slice;
- কাজ শেষে short demo video বা reproducible command;
- README/docs update;
- bug list ও risk register update;
- পরবর্তী milestone-এর blocker বাছাই।

প্রতিটি session শেষে লিখে রাখো: কী বদলাল, কোন test চালালে, কী ব্যর্থ, পরের একমাত্র কাজ কী। এতে দীর্ঘ প্রকল্পে context-switching কমে।


## ১৯. Reference project: “Nilang Notes” দিয়ে পুরো stack পরীক্ষা

Mobile framework তৈরির জন্য একটি সাধারণ, ছোট, কিন্তু যথেষ্ট বাস্তব sample app দরকার। Counter একা যথেষ্ট নয়; শুধু counter দিয়ে network, local persistence, form validation, navigation, permission, lifecycle বা data migration পরীক্ষা করা যায় না। তবে প্রথম vertical slice-এ counter, পরে notes app যোগ করতে পারো। একই example তিন platform-এ build/test হবে।

### ১৯.১ কেন Notes app?

Notes app-এ কয়েকটি গুরুত্বপূর্ণ capability স্বাভাবিকভাবে পরীক্ষা করা যায়:

- list screen এবং detail/editor screen;
- text input ও keyboard/IME;
- insert/edit/delete;
- persistent storage;
- navigation ও back behavior;
- empty/loading/error state;
- process death-পরবর্তী persistence;
- search/filter;
- accessibility labels;
- localization;
- upgrade/migration;
- optional share/export extension।

প্রথম release-এ cloud sync বা login যোগ করা দরকার নেই। Offline-first notes app দিয়ে local app model প্রমাণ করো। পরে HTTP sync যোগ করলে network/error/cancellation model পরীক্ষা হবে।

### ১৯.২ প্রস্তাবিত structure

```text
examples/nilang-notes/
├── nil.json
├── src/
│   ├── main.nil
│   ├── app.nil
│   ├── model/note.nil
│   ├── data/note_repository.nil
│   ├── state/notes_state.nil
│   ├── screens/notes_list.nil
│   ├── screens/note_editor.nil
│   └── shared/validation.nil
├── resources/
│   ├── locales/bn.json
│   ├── locales/en.json
│   └── icons/
├── tests/
│   ├── note_model_test.nil
│   ├── repository_test.nil
│   └── navigation_test.nil
└── platforms/
    ├── android/             # manual native extension, যদি থাকে
    ├── ios/
    └── harmony/
```

এই directory tree একটি target-state example। File-by-file implementation না থাকলে generated tree-কে live feature বলে দেখাবে না।

### ১৯.৩ Model এবং repository contract

```nil
record Note {
    id: String
    title: String
    body: String
    createdAt: DateTime
    updatedAt: DateTime
}

interface NoteRepository {
    fn list(): Result<List<Note>, StorageError>
    fn get(id: String): Result<Option<Note>, StorageError>
    fn save(note: Note): Result<Note, StorageError>
    fn delete(id: String): Result<Unit, StorageError>
}
```

এটি conceptual API। নীলাংয়ের actual record/interface/generic syntax-এর সঙ্গে মিলিয়ে নিতে হবে। গুরুত্বপূর্ণ হলো UI যেন সরাসরি database handle-এর সঙ্গে বাঁধা না থাকে। Test-এ in-memory fake repository ব্যবহার করা যাবে; mobile build-এ SQLite adapter বা অন্য persistent backend থাকবে।

### ১৯.৪ State transition model

```nil
enum NotesAction {
    Load
    Create
    Open(id: String)
    Save(note: Note)
    Delete(id: String)
    Search(query: String)
}

record NotesState {
    notes: List<Note>
    query: String
    loading: Bool
    error: Option<AppError>
}
```

এগুলো illustrative data model। Actions-কে pure state transition এবং repository side-effect-এর মধ্যে ভাগ করতে হবে। `Save` সফল হলে state আপডেট, ব্যর্থ হলে user-facing error এবং retry path থাকবে। UI button handler database exception গিলে ফেলবে না।

### ১৯.৫ Declarative screen-এর ধারণা

```nil
component NotesListScreen {
    state notesState: NotesState = NotesState.empty()

    onAppear {
        dispatch(NotesAction.Load)
    }

    render {
        Scaffold {
            TopBar(title: tr("notes.title"))

            if notesState.loading {
                ProgressIndicator()
            } else if notesState.notes.isEmpty {
                EmptyState(
                    title: tr("notes.empty.title"),
                    message: tr("notes.empty.message")
                )
            } else {
                List(items: visibleNotes(notesState)) { note in
                    NoteListItem(
                        title: note.title,
                        subtitle: note.updatedAt.format()
                    ) onClick {
                        navigate("notes/editor", { "id": note.id })
                    }
                }
            }

            FloatingActionButton(
                label: tr("notes.create"),
                icon: "plus"
            ) {
                navigate("notes/editor", { "mode": "create" })
            }
        }
    }
}
```

এই code snippet API design বোঝানোর জন্য; এতে ব্যবহৃত সব component/keyword বর্তমান Nilang-এ আছে বলে ধরে নেওয়া যাবে না। Compiler diagnostics যদি বলে `onAppear` বা `navigate` unknown, তাহলে actual syntax অনুযায়ী উদাহরণ সংশোধন করতে হবে—সিনট্যাক্স কল্পনা করে README-তে “supported” বলা যাবে না।

### ১৯.৬ UI এবং storage-এর মধ্যে contract

তিন প্ল্যাটফর্মের জন্য নিচের আচরণ একই থাকা দরকার:

1. App launch হলে persisted notes load হবে;
2. empty list হলে empty state দেখা যাবে;
3. নতুন note তৈরি করলে editor screen খুলবে;
4. Save-এ validation হবে;
5. title/body input keyboard থেকে commit হবে;
6. save success হলে notes list আপডেট হবে;
7. app force-stop/reopen হলে note হারাবে না;
8. delete confirmation-এ cancel করলে note থাকবে;
9. storage failure হলে actionable error হবে;
10. large list-এ UI responsiveness বজায় থাকবে।

### ১৯.৭ Test scenario-গুলোর নমুনা

```text
Test: create_note_persists_after_restart
Given: empty persistent store
When: user creates a note and saves it
And: the app process ends and a new process starts
Then: the note exists in the list
And: its title and body are preserved

Test: storage_failure_does_not_fake_success
Given: repository save returns DiskFull
When: user taps Save
Then: editor shows a storage error
And: note is not marked as saved
And: the user can retry after freeing space

Test: missing_capability_is_reported
Given: selected platform does not implement optional export capability
When: user invokes Export
Then: the app presents a clear unavailable message
And: no success toast is shown
```

এগুলো test specification; বাস্তব harness-এর syntax অনুসারে unit/integration/UI test implement করবে।

### ১৯.৮ Sample app দিয়ে platform parity যাচাই

Android, iOS এবং HarmonyOS-এ screenshot একেবারে pixel-identical করতে হবে না। তবে same user intent, একই data integrity, একই error semantics এবং equivalent accessibility পাওয়া দরকার। UI native control-এর কারণে visual differences acceptable হতে পারে; data loss, duplicate click, missing button label বা permission crash acceptable নয়।

এই Notes app-এর জন্য platform-specific test notes রাখো: keyboard behavior, safe area, back/gesture, app resume, file/database persistence path, locale rendering, signing/install/update। এতে abstraction-এর ভুলগুলো দ্রুত দেখা যাবে।

---

## ২০. Mobile SDK abstraction-এর contract ও API versioning

Native API দ্রুত পরিবর্তিত হয়। নীলাংয়ে platform API-গুলোকে সরাসরি compiler keyword বানালে প্রতিটি SDK change language release-কে প্রভাবিত করবে। তাই versioned adapter contract দরকার।

### ২০.১ Common API বনাম platform extension

**Common API** হবে সেই operation, যার অর্থ তিন প্ল্যাটফর্মে মোটামুটি এক:

- HTTP request;
- app-private storage;
- common text input;
- simple navigation intent;
- permission status request;
- local notification scheduling-এর common subset;
- clipboard read/write permission model-সহ;
- selected file access;
- app lifecycle events;
- JSON/date/locale primitives।

**Platform-specific API** হবে যেখানে OS-এর behavior আলাদা বা feature unavailable:

- Android intents/foreground services/widgets;
- iOS entitlements/extensions/background modes;
- HarmonyOS Ability-specific services, ArkUI-specific components বা platform-specific device services।

Portable API-কে নিচু পর্যায়ের সব platform detail দিয়ে ভারী করবে না; দরকার হলে `platform.android.foo()`-এর মতো namespaced extension থাকবে। কিন্তু common API-র নামে কোনো platform-এর unique behavior লুকিয়ে দেবে না।

### ২০.২ Capability matrix data model

Compiler-কে জানাতে হবে selected target-এ কোন feature available। এই তথ্য code-এ scattered `if android` string condition দিয়ে না রেখে machine-readable manifest বা backend registry-তে রাখো।

```text
Capability: camera.capture
  Android: available from configured API floor
  iOS: available with camera usage description + runtime permission
  Harmony: SDK-specific availability; not yet release-verified
  Common fallback: CapabilityUnavailable
```

এটি শুধু উদাহরণ; নির্দিষ্ট API floor official SDK documentation দিয়ে নির্ধারণ করবে। Tooling `nil doctor --target harmony`-এ জানাতে পারবে কোন SDK/API version প্রয়োজন এবং feature release-verified কি না।

### ২০.৩ API stability level

প্রতিটি API-কে stability label দাও:

- `experimental`: পরিবর্তন হতে পারে;
- `preview`: demo/feedback-এর জন্য;
- `stable`: documented compatibility contract;
- `deprecated`: replacement ও removal timeline আছে;
- `unsupported`: target-এ ইচ্ছাকৃতভাবে নেই।

Stable API-তে breaking change হলে migration guide ও version bump লাগবে। Compile-time warning-এর সঙ্গে replacement-এর code example দাও। Silent behavior change এড়িয়ে চলবে।

### ২০.৪ API availability checking

Source যদি camera API ব্যবহার করে কিন্তু target-এ তার implementation না থাকে, compiler-এর উচিত build-time warning/error দেওয়া। Runtime fallback তখনই গ্রহণযোগ্য, যখন source code unavailable capability handle করে:

```nil
match device.capabilities.camera {
    Supported => openCameraFlow(),
    PermissionRequired => requestCameraPermission(),
    Unsupported => showMessage(tr("camera.unavailable"))
}
```

এই pattern illustrative। `Supported`-এর অর্থ permission already granted—এমন নয়; hardware/service availability আর permission state আলাদা ধারণা। API design-এ এই পার্থক্য খুব গুরুত্বপূর্ণ।

### ২০.৫ Stable common behavior-এর test

প্রতিটি common API-র অন্তত একটি contract test থাকবে, যা fake provider এবং বাস্তব platform provider দুটিতে চালানো সম্ভব। উদাহরণ: `Storage.save()` successful হলে subsequent `load()` value ফেরাবে; save error হলে store-এর existing value corrupt হবে না। `http.get()` cancellation হলে task completion state নির্ধারিত থাকবে। UI API event একবার পাঠালে একবারই dispatch হবে।

---

## ২১. যে ভুলগুলো নীলাংয়ের mobile roadmap-কে বিপদে ফেলতে পারে

### ভুল ১ — সব feature একসঙ্গে শুরু করা

AI Oracle, SoftBus, GPU renderer, package registry, game engine integration, server framework এবং mobile UI সবকিছু একসঙ্গে polish করলে কোনো একটি মূল vertical slice production-quality হবে না। অগ্রাধিকার হবে language/runtime correctness, এক app-এর build-install-run-debug loop, তারপর cross-platform expansion।

### ভুল ২ — demo মানেই production support

`nil build android` যদি একটি ZIP/APK-সদৃশ artifact emit করে, তাতে signature, manifest, ABI, resource packaging, lifecycle, network permission, native libraries এবং store compliance ঠিক আছে প্রমাণ হয় না। Feature status-এ `prototype` আর `release-verified` আলাদা করো।

### ভুল ৩ — নিজস্ব graphics engine-কে প্রথম বাধ্যতামূলক UI engine করা

Text shaping, accessibility, IME, font fallback, semantics tree, native gestures, window insets এবং OS component behavior-এর কাজ graphics API-তে drawing করার চেয়ে অনেক বড় হতে পারে। প্রথমে native host adapter; custom renderer পরে।

### ভুল ৪ — iOS ও HarmonyOS-কে শুধু অন্য CPU architecture ভাবা

একটি binary ARM64-এ compile হওয়া মানেই iOS বা HarmonyOS app তৈরি নয়। Package model, host app lifecycle, SDK, signing, native API, permission model এবং distribution workflow আলাদা।

### ভুল ৫ — Platform difference লুকিয়ে রাখা

সব API-কে “cross-platform” বলে expose করলে subtle mismatch জমবে। Common subset তৈরি করো; unique feature platform namespace-এ রাখো; unsupported behavior explicit করো।

### ভুল ৬ — FFI-কে implementation detail মনে করা

Cross-language boundary-তে buffer ownership, thread, cancellation, exceptions, reference lifetime এবং ABI versioning পরিষ্কার না হলে intermittent crash তৈরি হবে। FFI contract লিখে tests না করা পর্যন্ত native bridge production-ready নয়।

### ভুল ৭ — permission prompt-কে API call-এর সঙ্গে এক করে দেখা

Permission prompt user deny করতে পারেন; status পরে বদলাতে পারে; permission prompt-ই app flow-এর অংশ। `camera.capture()` এমনভাবে design করো না যেন permission নেই মানেই function নিশ্চয়ই সফলভাবে prompt করে দেবে। Permission state, request এবং actual operation আলাদা।

### ভুল ৮ — UI state সবসময় memory-তে থাকবে ধরে নেওয়া

OS process kill করতে পারে; app দীর্ঘসময় background-এ থাকতে পারে। গুরুত্বপূর্ণ data event-handler-এর memory-তে রেখে দিলে হারাতে পারে। Persistent domain data আর ephemeral UI state আলাদা করো।

### ভুল ৯ — এক API error-কে empty response দিয়ে ঢেকে ফেলা

Network failure, 404, malformed JSON, permission denial এবং unsupported feature আলাদা error। সব ব্যর্থতায় empty list ফেরালে UI success দেখাতে পারে অথচ data হারিয়ে যায়। Typed errors ও retryability দরকার।

### ভুল ১০ — test শুধুমাত্র নিজের ফোনে চালানো

একটি নির্দিষ্ট Android ফোনে অ্যাপ চলা তিন platform-এর readiness প্রমাণ নয়। Minimum OS, emulator, different screen size, permission denial, app upgrade ও release signing test দরকার। কোনো platform testable না হলে unsupported/experimental হিসেবে ঘোষণা করো।

### ভুল ১১ — generator-এর সব output একবারে immutable করে দেওয়া

Native developer generated project-এ custom capability যোগ করতে চাইবে। Generated files আর user-editable files-এর boundary এবং `nil upgrade` migration policy না থাকলে প্রতিটি regeneration custom code মুছে দেবে।

### ভুল ১২ — private key বা secret source/config-এ রাখা

Signing password, API secret বা registry token committed file-এ থাকলে repository compromise মারাত্মক হতে পারে। CI secret vault, local keychain/secure storage, ignored config এবং secret-scanning test ব্যবহার করো।

### ভুল ১৩ — compatibility version ছাড়াই bytecode বিতরণ

পুরোনো `.nilax` নতুন VM-এ চালানো যাবে কি না নির্দিষ্ট না থাকলে update-এর পর app load fail হতে পারে। Format version, VM compatibility range ও migration strategy আবশ্যক।

### ভুল ১৪ — LSP/diagnostics পরে করব বলা

যদি developer প্রতি compile-এ opaque stack trace দেখে, language adoption কমবে। Early source spans, diagnostics, formatter ও basic debugger রাখো। Rich IDE features পরে আসতে পারে।

### ভুল ১৫ — সময়সীমার আগে marketing promise

“যে কোনো ফোন”, “নেটিভ পারফরম্যান্স”, “60 FPS”, “সব ফিচার সমান”—এসব দাবি কেবল verified test matrix-এর উপর ভিত্তি করে করবে। Documentation-এ feature status, known limitations এবং tested SDK/device versions স্পষ্ট থাকবে।

---

## ২২. প্রথম ৩০ দিনের পরিকল্পনা: কোডে নামানোর বাস্তব ক্রম

এই ৩০ দিনের পরিকল্পনা ধরে নেওয়া হয়েছে যে লক্ষ্য হলো Android-এর জন্য একটি প্রথম vertical slice এবং পরে iOS/HarmonyOS-এর জন্য একই architecture ব্যবহার করার ভিত্তি। সময়সীমা guaranteed delivery নয়; কাজগুলো ছোট করে সাজানোর একটি উপায়। কোনো কাজ ১ দিনে শেষ না হলে test বাদ দিয়ে এগিয়ে যাবে না।

### দিন ১–৩: baseline সংগ্রহ

- বর্তমান branch-এর clean checkout;
- `go test ./...` এবং repo-তে নথিভুক্ত build commands চালানো;
- failing tests, missing SDK, stale doc ও environment assumptions নথিভুক্ত করা;
- README-র feature list থেকে capability inventory তৈরি;
- current Android APK artifact real device/emulator-এ installable কি না যাচাই;
- `docs/mobile-architecture.md`-তে current-vs-proposed diagram লেখা।

**Deliverable:** baseline report, feature status table, blocker list। এই ধাপে code refactor না করলেও চলবে; আগে জানা দরকার কী সত্যিই কাজ করে।

### দিন ৪–৬: MVP contract lock

- MVP sample app হিসেবে Counter → Notes app নির্বাচন;
- native UI adapter approach-এর ADR লিখে রাখা;
- প্রথম target হিসেবে Android নির্ধারণ;
- minimum supported SDK এবং tested device নির্ধারণ;
- proposed `nil.json` schema তৈরি;
- UI IR-এর প্রথম ৫ component-এর contract লেখা;
- runtime host bridge-এর method list ও ownership rules লেখা।

**Deliverable:** design documents ও sample project contract। iOS/Harmony target ভবিষ্যৎ phase হলেও তাদের high-level boundaries এখনই নির্ধারিত থাকবে, যাতে Android solution পরে impossible abstraction তৈরি না করে।

### দিন ৭–১০: App host scaffold

- `nil init --template mobile`-এর minimal output;
- generated Android host project;
- manifest/app ID/version/resources configuration;
- runtime initialize/shutdown;
- Nilang entry point invoke;
- structured logs ও error path;
- `nil doctor --target android`-এর SDK check।

**Deliverable:** নীলাং সোর্সের “Hello Mobile” থেকে buildable host project। প্রথমেই fully declarative UI না হলেও চলবে; কিন্তু runtime invocation বাস্তব হতে হবে।

### দিন ১১–১৫: প্রথম native UI events

- Text/Button/Column-এর native mapping;
- state initialization;
- button event dispatch;
- Nilang থেকে state ফেরত;
- source-location diagnostics;
- one-screen counter sample;
- interpreter/VM semantics check;
- debug install/run/log workflow।

**Deliverable:** counter app বাস্তব ফোনে চলে এবং Button tap করলে সঠিক state আপডেট হয়। UI demo-র source code, command এবং known limitation commit-এ থাকবে।

### দিন ১৬–২০: Lifecycle ও persistence

- app pause/resume event mapping;
- process kill/restore tests;
- local storage repository interface;
- create/edit note minimal flow;
- typed storage errors;
- back/navigation behavior;
- keyboard/IME test;
- accessibility label ও font scaling smoke test।

**Deliverable:** নোট সংরক্ষণ হয় এবং app process পুনরায় শুরু হলেও data থাকে। শুধু memory variable-এ value রেখে acceptance পাস হয়েছে বলা যাবে না।

### দিন ২১–২৪: Package quality

- bundle manifest version;
- resource packaging ও path validation;
- debug signing/install validation;
- package inspection command/report;
- corrupted archive test;
- release build mode-এ unsupported capability fail;
- dependency/toolchain versions report;
- build output directory cleanup/rebuild test।

**Deliverable:** package structure verified; unsigned/debug/release status আলাদা দেখানো হয়; CI-তে build failure reproducible।

### দিন ২৫–২৭: Test automation ও docs

- compiler conformance fixtures;
- UI parity test specification;
- generated project golden test;
- GitHub Actions PR workflow;
- `docs/mobile-quickstart.md`;
- `docs/platform-capabilities.md`;
- `docs/troubleshooting.md`;
- known limitations section।

**Deliverable:** নতুন contributor-ও documented command দিয়ে app build/run করতে পারে, এবং test failure থেকে root cause বের করার পথ পায়।

### দিন ২৮–৩০: Review ও পরের milestone

- clean Windows environment-এ build পুনরাবৃত্তি;
- সম্ভব হলে দ্বিতীয় Android device বা emulator test;
- source/build/runtime performance measure;
- package size baseline;
- crash/lifecycle bug list;
- iOS backend-এর জন্য Mac/toolchain access plan;
- HarmonyOS backend-এর জন্য DevEco/toolchain availability plan;
- ৩০ দিনের retrospective ও পরবর্তী issue backlog।

**Deliverable:** ছোট কিন্তু verified mobile prototype, এবং পরবর্তী phase-এর evidence-based backlog। Prototype-কে “Android/iOS/Harmony পূর্ণ সমর্থিত” বলা হবে না।

### এই ৩০ দিনের মধ্যে যে কাজগুলো স্থগিত থাকবে

- push notification এবং server-side push provider;
- camera/video/location/Bluetooth/NFC;
- native GPU renderer;
- AOT optimizer;
- all-platform code generation;
- marketplace/registry expansion;
- AI-driven automatic code changes;
- elaborate theme/component library।

এগুলো অপ্রয়োজনীয় নয়; কিন্তু মূল vertical slice এবং test loop প্রমাণের আগে করলে core architecture বারবার বদলাতে হতে পারে।

---

## ২৩. Release acceptance checklist: কখন একটি target সমর্থিত বলা যাবে?

প্রতিটি platform-এর জন্য আলাদা release checklist থাকবে। নিচের তালিকা চেকবক্সসহ issue template বা `docs/release-checklist.md`-এ রূপান্তর করতে পারো।

### ২৩.১ Language/runtime

- [ ] language specification version ঘোষিত;
- [ ] compiler ও VM semantics conformance tests পাস;
- [ ] error behavior ও source locations পাওয়া যায়;
- [ ] runtime ABI/version recorded;
- [ ] memory/callback cleanup test হয়েছে;
- [ ] async cancellation ও exception path test হয়েছে;
- [ ] malformed source, bytecode ও bundle safeভাবে reject হয়;
- [ ] `.nilax` format version ও compatibility policy লেখা;
- [ ] release optimization build semantic conformance বজায় রাখে।

### ২৩.২ Generated native project

- [ ] clean checkout থেকে generator output তৈরি হয়;
- [ ] platform-specific project official toolchain দিয়ে build হয়;
- [ ] app identity, version ও minimum OS ঠিক;
- [ ] required assets/font/locales যুক্ত হয়;
- [ ] no accidental source/secret included;
- [ ] supported ABI/architecture package-এ আছে;
- [ ] debug/release configuration আলাদা;
- [ ] generated project native IDE-তে খুলে inspect করা যায়;
- [ ] regenerated project developer-owned files overwrite করে না।

### ২৩.৩ UI/runtime behavior

- [ ] Text/Button/Input/Scroll/List basic components কাজ করে;
- [ ] event একবারে একবার dispatch হয়;
- [ ] navigation/back behavior documented;
- [ ] keyboard open/close-এ form usability থাকে;
- [ ] app background/resume crash হয় না;
- [ ] process recreation-এর পর durable state ঠিক থাকে;
- [ ] screen reader labels/roles পাওয়া যায়;
- [ ] font scaling ও localized strings layout ভাঙে না;
- [ ] unsupported UI/API gracefulভাবে জানায়;
- [ ] long list ও large text-এ memory leak দেখা যায় না।

### ২৩.৪ System services ও permissions

- [ ] capability declaration থেকে manifest/metadata ঠিক তৈরি হয়;
- [ ] runtime permission denied case test হয়েছে;
- [ ] permission revoked হলে safe error হয়;
- [ ] service unavailable হলে false success হয় না;
- [ ] cancellation এবং timeout path test হয়েছে;
- [ ] file access path sandbox মেনে চলে;
- [ ] secrets/logging redaction পরীক্ষা হয়েছে;
- [ ] platform-specific feature explicitভাবে documented।

### ২৩.৫ Distribution ও signing

- [ ] debug artefact installable;
- [ ] release artefact official toolchain থেকে তৈরি;
- [ ] release signature/metadata validation পাস;
- [ ] key material source/artefact/log-এ নেই;
- [ ] app update existing data preserve বা migrate করে;
- [ ] install/uninstall/update smoke tests আছে;
- [ ] store/distribution-specific configuration documented;
- [ ] build metadata/reproducibility report তৈরি হয়;
- [ ] known limitations ও minimum/maximum tested versions প্রকাশিত।

### ২৩.৬ Documentation

- [ ] first project tutorial working;
- [ ] native extension guide;
- [ ] permission/capability guide;
- [ ] debugging/troubleshooting guide;
- [ ] build environment/version requirements;
- [ ] examples compile in CI;
- [ ] unsupported features list;
- [ ] migration/compatibility policy;
- [ ] security contact/bug reporting instructions;
- [ ] examples কোন SDK/device-এ পরীক্ষা হয়েছে তা লেখা।

### ২৩.৭ Release label-এর প্রস্তাব

`Experimental` বলতে build/toolchain বা API বদলাতে পারে, এবং কিছু feature অসম্পূর্ণ। `Preview` বলতে মূল workflow চলে কিন্তু সীমিত device/feature coverage। `Beta` বলতে প্রধান use case কার্যকর, known limitations প্রকাশিত, test matrix আছে। `Stable` বলতে published acceptance checklist পূরণ, backward compatibility policy, release pipeline ও security process আছে। এই শব্দগুলোকে marketing label না বানিয়ে operational rule করো।

---

## ২৪. Native bridge-এর নকশা: boundary, event loop ও value conversion

Native bridge হলো নীলাং runtime এবং platform UI/framework-এর মধ্যে চুক্তি। এই boundary-তে ভুল হলে ত্রুটি sporadic হয়: button handler কখনো দুইবার চলে, callback screen destroy হওয়ার পরও আসে, native object free হয়ে গেছে অথচ VM handle ধরে রাখে, অথবা UI update ভুল thread-এ ঘটে। তাই bridge-কে একটি প্রথম-শ্রেণির API হিসেবে design করো।

### ২৪.১ Bridge-কে ছোট রাখো

প্রথম bridge-এ ২০টি আলাদা API method দরকার নেই। প্রাথমিক contract হতে পারে:

- `engineCreate(config)`;
- `engineLoad(bundle)`;
- `dispatchEvent(componentId, eventType, payload)`;
- `subscribeState(listener)`;
- `drainErrors()`;
- `cancelTask(taskId)`;
- `enginePause()` / `engineResume()`;
- `engineDispose()`।

বাস্তব API নাম ও signature নীলাংয়ের language/runtime model অনুযায়ী নির্ধারণ করবে। উদ্দেশ্য হলো platform host যেন VM-এর internal AST/heap-এর সব details জানে না। UI IR এবং event payload bridge-এর contract।

### ২৪.২ Thread policy

প্রতিটি bridge function-এর thread rule লিখে রাখো:

- runtime creation background thread-এ করা যায় কি না;
- UI state delivery main thread-এ হবে কি না;
- event dispatch synchronous না asynchronous;
- host callback reentrant হতে পারে কি না;
- `engineDispose()` outstanding callback wait করবে, নাকি cancel করবে;
- callback-এর ভিতর আবার `dispatchEvent()` ডাকলে কী semantics;
- app background-এ গেলে scheduler কী করবে।

একটি safe default হতে পারে: UI events host-এর main thread থেকে accepted হলেও Nilang logic নির্দিষ্ট runtime executor-এ চলে; UI mutation result main thread-এ ফেরে। তবে বাস্তব সিদ্ধান্ত performance ও runtime support দিয়ে যাচাই করতে হবে। Main thread-এ blocking database/network কাজ নিষিদ্ধ বা lint warning হওয়া উচিত।

### ২৪.৩ Value conversion

Common values-এর mapping নির্ধারণ করো:

| Nilang value | Native bridge form | নিয়ম |
|---|---|---|
| `Bool` | platform boolean | এক-এক mapping |
| integer/float | fixed-width numeric contract | overflow/precision documented |
| `String` | UTF-8 buffer/host string | encoding ও lifetime স্পষ্ট |
| `Bytes` | copied/shared byte buffer | ownership/reference count স্পষ্ট |
| `List<T>` | typed sequence | maximum size/recursive depth policy |
| `Record` | schema-tagged map/object | field versioning |
| `Option<T>` | tagged optional value | null/missing আলাদা হতে পারে |
| `Result<T,E>` | tagged success/error | exception boundary অতিক্রম নয় |
| function/closure | opaque callable handle | release/retain rules |
| native object | opaque `NativeHandle` | direct pointer expose নয় |

JSON serialization-কে সব bridge communication-এর একমাত্র উপায় করলে implementation সহজ হতে পারে, কিন্তু large data, binary values, performance এবং type safety সীমিত হবে। প্রথম prototype-এ JSON bridge acceptable; production design-এ structured type mapping ও measurement দরকার।

### ২৪.৪ Callback lifetime ও cancellation

প্রতিটি callback handle-এর lifecycle লিখবে: কে তৈরি করে, কে owner, কখন release হয় এবং owner screen dispose হলে কী হয়। Native UI event যেন stale screen ID-তে event dispatch না করে। Async network response ফেরার আগে route change হয়ে গেলে response পুরোনো UI-তে apply হবে কি না নির্ধারণ করো।

Task ID বা cancellation token ব্যবহার করতে পারো; cancel-এর পর task completed না cancelled হিসেবে report করবে সেটাও consistent হতে হবে। কিছু native API cancellation সমর্থন না করলে cancellation request best-effort হতে পারে—এই সীমা docs-এ লিখবে।

### ২৪.৫ Error boundary

Native exception/NSException/Java exception/C++ exception বা host crash Nilang exception হিসেবে স্বাভাবিকভাবে catch করা যাবে—এমন অনুমান করবে না। Bridge-এ error conversion policy থাকবে:

- predictable API errors typed result হিসেবে ফেরত;
- native exception controlled boundary-তে captured, যেখানে platform support করে;
- fatal process-level fault normal error নয়;
- developer-mode stack trace detailed;
- production user message privacy-safe;
- native error code, Nilang call site ও operation ID logs-এ correlate করা যায়;
- redaction policy সর্বত্র প্রয়োগ হয়।

### ২৪.৬ ABI stability

Bridge API versioned হওয়া উচিত। Runtime 1.x-এ `NilValue` layout বদলালে generated host crashes তৈরি হতে পারে। Public ABI-তে version/size fields, explicit enum tags এবং forward-compatible extension pattern বিবেচনা করো। Unknown method/version পেলে app স্পষ্টভাবে fail করবে; memory-এর arbitrary interpretation নয়।

### ২৪.৭ Native bridge conformance tests

- source string-এ বাংলা/emoji passed round-trip;
- null/missing/empty string distinction;
- huge integer overflow behavior;
- callback called after dispose;
- duplicate event dispatch;
- event during pause/resume;
- task cancellation race;
- native host throws/returns failure;
- malformed payload;
- wrong ABI version;
- memory handle double-release;
- thread violation detector;
- high-frequency event flood;
- runtime close while task pending।

এই test-গুলো Android, iOS ও Harmony host integration-এ সমতুল্য semantics নিশ্চিত করবে।

---

## ২৫. Documentation strategy এবং developer onboarding

একটি নতুন language/platform-কে অন্য ভাষার বিকল্প হিসেবে ব্যবহারযোগ্য হতে হলে documentation-কে source code-এর মতো version ও test করতে হবে। শুধু বড় README যথেষ্ট নয়; নতুন developer-কে “Hello World” থেকে real app পর্যন্ত যেতে আলাদা guide দরকার।

### ২৫.১ Documentation information architecture

`docs/`-এ নিচের রকম navigation রাখতে পারো:

1. **Start Here:** installation, `nil doctor`, প্রথম প্রোগ্রাম, project creation।
2. **Language Guide:** syntax, types, modules, errors, async, package import।
3. **Alap UI Guide:** components, state, layouts, navigation, accessibility, themes।
4. **Mobile App Guide:** lifecycle, permissions, storage, network, resources, target selection।
5. **Platform Guides:** Android build, iOS/Xcode build, HarmonyOS/DevEco build।
6. **Native Extension Guide:** native host integration, ABI, feature fallback, sample extension।
7. **Tooling:** formatter, LSP, debugger, profiler, logs, diagnostics।
8. **Security:** capabilities, package signing, secrets, dependency verification।
9. **Compatibility:** version matrix, API availability, bundle format, migration।
10. **Troubleshooting:** common toolchain errors, package install errors, runtime crashes।
11. **Contributor Guide:** architecture, test commands, coding rules, release process।

### ২৫.২ Tutorial-কে end-to-end রাখো

প্রথম mobile tutorial-এ শুধু component syntax দেখাবে না। User যেন এগুলো করতে পারে:

- compiler install/build;
- `nil doctor` চালানো;
- mobile project scaffold;
- একটি Text এবং Button যুক্ত করা;
- target নির্বাচন;
- debug package build;
- emulator/device-এ install;
- log দেখা;
- test চালানো;
- release build-এর পার্থক্য বোঝা;
- common failure diagnose করা।

Tutorial-এ exact toolchain versions এবং tested date রাখবে। কোনো command আর কাজ না করলে CI tutorial test fail করবে বা release process-এ documentation review আবশ্যক হবে।

### ২৫.৩ Example gallery

Example ছোট ও একক ধারণাকেন্দ্রিক রাখো:

- `hello-console`;
- `hello-mobile`;
- `counter`;
- `form-validation`;
- `notes-local`;
- `http-json`;
- `permission-camera` (শুধু বাস্তব implementation হলে);
- `deep-link`;
- `local-notification`;
- `native-extension`;
- `accessible-list`;
- `bengali-localization`;
- `app-lifecycle`;
- `database-migration`;
- `unsupported-feature-fallback`।

সব উদাহরণের README-তে status badge বা plain text দেখাবে: target, SDK version, real-device-tested কি না, known limitation। Stub capability-এর example-এ sample purpose স্পষ্ট হবে।

### ২৫.৪ API docs generation

Source annotations/metadata থেকে API reference generate করা যায়। Manual docs-এর সঙ্গে public API signature mismatch না হয়, CI-তে doc generation diff/check রাখো। Stable API-র জন্য parameter descriptions, error cases, target availability এবং example code থাকতে হবে। Example compilation tests থাকলে obsolete documentation দ্রুত ধরা যায়।

### ২৫.৫ Learning curve কমানো

Nilang-এর syntax পরিচিত ভাষার মতো দেখাতে পারে, কিন্তু শুধু syntax familiar হলেই learning curve কমে না। App lifecycle, permission model, async semantics, layout, state persistence এবং native toolchain আলাদা শেখাতে হবে। প্রতিটি concept-এর জন্য ছোট mental model, diagram, runnable example এবং “কখন ব্যবহার করবেন না” অংশ রাখো। Developer-কে platform SDK-এর সবকিছু শিখতেই হবে—এমন নয়; তবে platform-specific API-র প্রয়োজন হলে extension guide ব্যবহার করার দক্ষতা থাকতে হবে।

---

## ২৬. Reference technologies থেকে কী শিখবে—আর কী অন্ধভাবে নকল করবে না

Cross-platform platform-গুলোতে কী কাজ করেছে তা অধ্যয়ন করা দরকার, কিন্তু লক্ষ্য হওয়া উচিত architecture trade-off শেখা, feature checklist নকল করা নয়।

### Android native architecture

Android documentation lifecycle, UI/data layer separation, adaptive layout এবং system-managed process behavior-কে গুরুত্ব দেয়। Nilang app model-এও UI view-কে data ownership-এর একমাত্র স্থান করা উচিত নয়। Reference: https://developer.android.com/topic/architecture

### Apple Xcode ও native app workflow

Xcode project structure, build, simulator, profiling, signing এবং distribution-এর কেন্দ্রীয় role পালন করে। Nilang iOS backend-এর উচিত Xcode প্রকল্পকে opaque binary package না ধরে inspectable generated project হিসেবে তৈরি করা। Reference: https://developer.apple.com/documentation/xcode

### HarmonyOS ArkTS/ArkUI

HarmonyOS NEXT-এ ArkTS এবং ArkUI কেন্দ্রীয় প্রযুক্তি; natively hosted UI ও lifecycle-এর জন্য ArkUI host adapter গুরুত্বপূর্ণ। Nilang-এর লক্ষ্য হওয়া উচিত common logic reuse, অথচ Harmony-specific ability ও system services-এর access বন্ধ না করা। References: https://developer.huawei.com/consumer/en/harmonyos/develop/ এবং https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/arkts-get-started-V5

### Kotlin Multiplatform-এর shared-logic ধারণা

Kotlin Multiplatform-এর documentation দেখায় যে common logic share করে UI platform-native রাখা সম্ভব; shared UI বা native UI—দুটিই architecture choice। নীলাংয়ের জন্য এই distinction গুরুত্বপূর্ণ: source sharing মানেই সব UI implementation এক হতে হবে না। Reference: https://kotlinlang.org/docs/multiplatform/kmp-overview.html

### WebAssembly

WASM-এর portability নীলাংয়ের logic module-এর জন্য উপকারী হতে পারে, কিন্তু platform-specific app lifecycle, UI/accessibility, permissions, signing এবং store packaging তার একমাত্র দায়িত্ব নয়। WASM output-কে একটি backend হিসেবে দেখো; mobile native app-এর একমাত্র সমাধান নয়।

### কী নকল না করাই ভালো

- নিজস্ব renderer বানানোর জন্য renderer architecture অন্ধভাবে অনুকরণ নয়;
- এমন macro/plugin system তৈরি নয় যার নিরাপত্তা ও compatibility model নেই;
- runtime optimization আগে, measured bottleneck পরে;
- every platform-specific API-কে common denominator-এ গলিয়ে ফেলা নয়;
- custom package registry থাকা মানেই secure supply chain নয়;
- AI oracle বা code generation থাকা মানেই compiler correctness নয়।

Reference project থেকে শেখা মানে তাদের system contract বোঝা, তাদের complexity একই পর্যায়ে কপি করা নয়।

---

## ২৭. Architecture Decision Record (ADR) template

বড় প্রকল্পে ভুল সিদ্ধান্ত শুধু code bug নয়; এটি কয়েক মাসের কাজকে ভুল দিকে নিয়ে যেতে পারে। তাই গুরুত্বপূর্ণ architecture সিদ্ধান্ত ছোট, version-controlled document-এ লিখে রাখো। ভবিষ্যতে অন্য contributor এলে সে একই debate আবার শুরু করতে হবে না।

একটি ADR-এর কাঠামো:

```text
# ADR-0001: Mobile UI backend strategy

Status: Accepted / Proposed / Superseded
Date: YYYY-MM-DD
Owner: project maintainer

## Context
কোন সমস্যা বা constraint সিদ্ধান্তটি প্রয়োজন করেছে?

## Decision
কী বেছে নেওয়া হচ্ছে এবং কী নয়?

## Alternatives
কোন বিকল্প বিবেচনা করা হয়েছে?

## Evidence
কোন prototype, benchmark, compatibility test বা official document সিদ্ধান্তটিকে সমর্থন করেছে?

## Consequences
কী সহজ হলো, কী কঠিন হলো, কোন নতুন risk তৈরি হলো?

## Revisit condition
কোন measurable evidence এলে সিদ্ধান্তটি পুনর্বিবেচনা করা হবে?
```

### ২৭.১ প্রথম ADR-গুলো

1. `ADR-0001`: Nilang mobile UI strategy—native adapters বনাম WebView বনাম own renderer।
2. `ADR-0002`: mobile runtime model—embedded VM বনাম source code generation বনাম native runtime library।
3. `ADR-0003`: runtime ABI ও native handles।
4. `ADR-0004`: `.nilax` versioning/signature format।
5. `ADR-0005`: common API বনাম platform namespaces।
6. `ADR-0006`: UI state/event model ও main-thread policy।
7. `ADR-0007`: Android build orchestration ও signing।
8. `ADR-0008`: iOS build integration ও Mac/CI requirement।
9. `ADR-0009`: HarmonyOS ArkTS/ArkUI host integration।
10. `ADR-0010`: capability security model।

প্রতিটি ADR পাঁচটি বাক্যেও হতে পারে; উদ্দেশ্য লম্বা essay নয়, traceability। সিদ্ধান্ত বদলালে পুরোনো ADR edit করে ইতিহাস মুছে না ফেলে নতুন ADR লিখবে।

---

## ২৮. Measurement framework: উন্নতি কীভাবে মাপবে

“নীলাং আগের থেকে উন্নত হয়েছে কি না” প্রশ্নের উত্তর commit সংখ্যা, file count বা README-র badge দিয়ে দেওয়া উচিত নয়। Platform readiness-কে observable metric দিয়ে মাপো।

### ২৮.১ Compiler quality

- supported language spec feature-এর অনুপাতে conformance pass rate;
- compiler/interpreter/VM result mismatch সংখ্যা;
- diagnostic quality regression;
- crash/panic on invalid source count;
- fuzzing-discovered bugs;
- clean build success rate;
- source mapping coverage;
- compile time on representative projects।

### ২৮.২ Runtime quality

- startup time;
- peak memory;
- crash rate;
- uncaught exception count;
- pending task/callback leak count;
- cancellation latency;
- bridge call throughput;
- bytecode compatibility test results;
- fatal native crash reproduction quality।

### ২৮.৩ UI framework quality

- core component integration test pass rate;
- screen reader semantic coverage;
- keyboard/IME defects;
- adaptive layout test pass rate;
- UI event duplication/loss count;
- frame-drop baseline for agreed sample screen;
- memory behavior for long lists;
- localization and text scaling issues।

### ২৮.৪ Build/distribution quality

- clean checkout থেকে build success;
- SDK/toolchain missing diagnostics quality;
- invalid manifest rejection;
- release signing validation success;
- generated project reproducibility;
- artifact size by target/ABI;
- dependency scan findings;
- app upgrade/data migration pass rate;
- package verification failure detection।

### ২৮.৫ Ecosystem quality

- official example projects CI-তে compile হয় কি না;
- tutorial freshness;
- bug report reproduction success;
- native extension compatibility tests;
- stable API migration burden;
- issue resolution time (team বড় হলে);
- release notes-এ limitations কতটা স্পষ্ট।

Metric target বাস্তব baseline থেকে নির্ধারণ করো। Baseline নেই এমন জায়গায় কল্পিত threshold ঘোষণা করো না। প্রথম release-এ লক্ষ্য হবে measurement চালু করা, তারপর regression threshold স্থির করা।

---

## ২৯. তিন platform-এর feature matrix: common baseline ও parity

নিচের matrix ভবিষ্যৎ implementation planning-এর জন্য। এতে `planned` বা `needs proof` মানে বাস্তবে ইতিমধ্যে চালু আছে নয়। বর্তমান repo-র README/গেম-নথিতে সাধারণ iOS/Harmony application workflow প্রতিষ্ঠিত নয়; তাই target support নিশ্চিত না হওয়া পর্যন্ত status `not verified` থাকবে।

| Feature | Android MVP | iOS MVP | HarmonyOS MVP | পরবর্তী ধাপ |
|---|---|---|---|---|
| Hello screen + Text/Button | প্রথম প্রমাণ | native host proof | native host proof | component parity |
| Nilang logic runtime | VM/bridge verify | framework/bridge verify | ArkTS/native bridge verify | runtime optimization |
| UI state/event | integration test | integration test | integration test | predictable state restoration |
| Basic navigation | Android back | iOS navigation stack | Harmony route behavior | deep links |
| Local persistence | app-private storage | app container storage | app-private storage | database migration |
| HTTP/JSON | common contract | common contract | common contract | uploads/streaming/retry |
| Permissions | Android runtime permission | usage descriptions + authorization | Harmony API-specific permission | capability audit |
| Accessibility | TalkBack audit | VoiceOver audit | platform screen reader audit | advanced focus semantics |
| Debug symbol/source maps | VM/source map | Xcode/bridge mapping | ArkTS/native mapping | integrated debugger |
| Debug install | APK + adb | Xcode/simulator/device | HAP/dev tool/device | one CLI workflow |
| Release signing | Android signing | Apple signing/provisioning | Harmony signing | store/beta pipeline |
| Package format | APK/AAB workflow | archive/IPA workflow | HAP/App package | release provenance |
| Deep links | Android app links | universal/custom links | SDK-specific links | auth route protection |
| Notifications | local first | local first | local first | remote push |
| Camera | after baseline | after baseline | after baseline | media capture |
| Background jobs | Android policy | iOS policy | Harmony policy | durable work abstraction |
| Native extensions | Kotlin/Java/JNI path | Swift/Obj-C/C boundary | ArkTS/native module path | versioned plugin SDK |
| Real-device evidence | mandatory | mandatory | mandatory | supported-device matrix |

এই matrix-এর উদ্দেশ্য তিন platform-কে একরকম বানানো নয়। এটি বোঝায় common behavior কোথায়, native difference কোথায়, আর test evidence কোথায় লাগবে।

---

## ৩০. Developer experience-এর ১০টি অপরিহার্য নিয়ম

1. **কমান্ড আগে predictably কাজ করবে, marketing পরে।** CLI command-এর output, exit code, build artefact ও failure reason স্থির হতে হবে।
2. **Examples compile হতে হবে।** README-তে snippet থাকলে CI-তে grammar/syntax অন্তত validate হবে অথবা স্পষ্টভাবে pseudocode label থাকবে।
3. **Native project inspectable হতে হবে।** Generated project black box নয়; developer native IDE-তে খুলে debug করতে পারবে।
4. **Unsupported feature গোপন থাকবে না।** Build warning/error হবে; success-looking stub নয়।
5. **একটি common API মানে common semantics।** একই নামের API তিন target-এ তিনরকম আচরণ করলে documentation-এ পার্থক্য স্পষ্ট থাকবে।
6. **Security default-deny।** Sensitive capability declaration, runtime permission check ও package signing আলাদা দায়িত্ব।
7. **Errors actionable হবে।** কী ব্যর্থ হয়েছে, কেন, কোন target/toolchain চাই, পরের পদক্ষেপ কী—সব বলা হবে।
8. **Backward compatibility লিখিত থাকবে।** `.nilax`, runtime ABI, stable APIs ও config migrations versioned।
9. **Performance measured হবে।** “fast” বা “native performance” শব্দের পেছনে benchmark ও device details থাকবে।
10. **Real device evidence ছাড়া platform stable নয়।** Emulator test দরকার, কিন্তু camera, notification, keyboard ও lifecycle-এর device behavior-ও পরীক্ষা করতে হবে।

---

## ৩১. প্রথম mobile milestone-এর প্রস্তাবিত issue backlog

নিচের backlog-কে GitHub Issues-এ পৃথক task-এ ভাঙতে পারো। এটি বর্তমান issue tracker audit নয়; প্রস্তাবিত engineering breakdown।

### Epic A — Mobile app spec

- [ ] `nil.json` mobile schema v1 লিখে tests যোগ।
- [ ] supported target/API matrix document।
- [ ] lifecycle event contract define।
- [ ] permission/capability contract define।
- [ ] mobile UI IR contract define।

### Epic B — Runtime embed API

- [ ] runtime create/load/execute/dispose API prototype।
- [ ] value ownership + native handle policy।
- [ ] event dispatch and callback lifecycle।
- [ ] source-mapped runtime error output।
- [ ] pending task cancellation tests।

### Epic C — Android MVP

- [ ] native Android host template build।
- [ ] Nilang runtime packaged/loaded।
- [ ] Text/Button/Column native mapping।
- [ ] counter sample app end-to-end।
- [ ] debug install/log command।
- [ ] lifecycle + state restoration integration tests।
- [ ] release signing configuration/validation।

### Epic D — Alap mobile UI

- [ ] typed UI tree/IR।
- [ ] stable component key policy।
- [ ] TextField/IME semantics।
- [ ] accessibility role/label contract।
- [ ] safe-area/density/font-scale model।
- [ ] core component contract tests।

### Epic E — iOS prototype

- [ ] Mac/Xcode build runner available and pinned।
- [ ] SwiftUI host template builds in simulator।
- [ ] runtime bridge loads and invokes Nilang function।
- [ ] state/event round-trip test।
- [ ] signing/provisioning preflight diagnostics।

### Epic F — HarmonyOS prototype

- [ ] DevEco/SDK version pinned on available runner।
- [ ] ArkTS/ArkUI host template builds HAP।
- [ ] bridge dispatches event into Nilang runtime।
- [ ] state/event round-trip test।
- [ ] release signing/package checks documented।

### Epic G — Supply chain & quality

- [ ] `.nilax` archive path traversal tests।
- [ ] lockfile/dependency integrity report।
- [ ] source/secret redaction tests।
- [ ] CI conformance suite।
- [ ] release acceptance checklist implementation।

এই তালিকার Epic A, B এবং C-র প্রথম অংশ শেষ হওয়ার আগে Epic F-এ বড় বিনিয়োগ কোরো না। তবে Harmony/iOS-কে একেবারে শেষে ভাবার বদলে initial architecture-এ their constraints documented রাখো; এতে Android-only design পরে ভাঙতে হবে না।

---

## ৩২. সিদ্ধান্ত: নীলাংয়ের জন্য কোন জিনিসগুলো “এখন”, “পরে” এবং “দীর্ঘমেয়াদে”

### এখন — প্রথম ১–৩ মাস

- বর্তমান implementation status যাচাই;
- language/runtime conformance;
- stable mobile manifest ও UI IR design;
- runtime embedding bridge;
- Android debug APK build/install/run/log workflow;
- Text/Button/state/navigation/persistence-এর vertical slice;
- source-mapped errors;
- first real-device test ও CI automation।

### পরের ৩–৬ মাস, baseline কাজের উপর নির্ভর করে

- Android release build quality;
- Alap core component set;
- networking/JSON/database capabilities;
- accessibility/localization/layout tests;
- iOS native host proof of concept;
- HarmonyOS native host proof of concept;
- SDK versions/tooling integration ও known limitation docs।

### এরপর ৬–১২+ মাস, বাস্তব কাজ ও access-এর উপর নির্ভর করে

- তিন target-এর feature parity roadmap;
- signing/release pipelines;
- device/test matrix expansion;
- native extension SDK;
- debugger/profiler/LSP improvements;
- optimized runtime/AOT benchmark-driven work;
- notification, camera, location, media ও background capabilities;
- package supply-chain security এবং stable API policy।

সময়সীমা কোনো প্রতিশ্রুতি নয়; বিশেষত iOS ও HarmonyOS-এর SDK, signing account, build runner এবং real hardware access না থাকলে সেই target-এর verification এগোবে না। কিন্তু dependency ও architecture আজ থেকেই সাজালে পরে কাজের পুনরাবৃত্তি কমবে।

---

## ৩৩. পরিভাষার সংক্ষিপ্ত অভিধান

- **AST:** Abstract Syntax Tree—source program-এর syntax tree।
- **HIR/MIR:** compiler-এর intermediate representation-এর স্তর, যেখানে উচ্চস্তরের syntax ধীরে ধীরে নিচুস্তরের operation-এ রূপান্তরিত হয়।
- **AOT:** Ahead-of-Time compilation—runtime-এ চালানোর আগে native code-এ compile করা।
- **ABI:** Application Binary Interface—compiled components-এর মধ্যে calling convention, type layout, symbol এবং ownership-এর contract।
- **FFI:** Foreign Function Interface—এক ভাষা থেকে অন্য ভাষার function/API call করার পদ্ধতি।
- **JNI:** Java Native Interface—Android/JVM code থেকে native code call করার interface।
- **UI IR:** UI tree-এর platform-neutral intermediate representation।
- **Native adapter/host:** প্রতিটি platform-এর নিজস্ব UI/lifecycle/API-এর সঙ্গে Nilang-কে যুক্ত করা code।
- **HAP:** HarmonyOS application/module package-এর একটি package format; target SDK ও release workflow অনুযায়ী toolchain দিয়ে তৈরি হয়।
- **APK/AAB:** Android app package এবং Google Play distribution-এর app bundle format; দুটির ব্যবহার এক নয়।
- **IPA/archive:** Apple platform build/distribution-এর package/archive workflow-এর অংশ; Xcode ও signing configuration অনুযায়ী তৈরি হয়।
- **Capability:** app কোন protected operation করতে চায়, তার ঘোষিত অধিকার/সীমা; capability declared হলেই user permission granted হয় না।
- **Conformance test:** language specification-এ নির্ধারিত আচরণ compiler/runtime বজায় রাখছে কি না যাচাই।
- **Vertical slice:** ছোট কিন্তু end-to-end কাজ—source থেকে build, install, run, test ও debug পর্যন্ত।
- **Reproducible build:** pinned inputs ও toolchain-এ পুনরায় build করলে যাচাইযোগ্যভাবে একই বা সমমানের output পাওয়া।
- **ADR:** Architecture Decision Record—কেন একটি architecture choice নেওয়া হয়েছে তার version-controlled নথি।
- **Parity:** একই capability-এর documented, testable equivalent behavior; সব UI pixel বা low-level API এক হওয়া নয়।

---

## ৩৪. চূড়ান্ত সারাংশ

নীলাংকে multi-platform mobile development-এর জন্য প্রস্তুত করার সবচেয়ে শক্তিশালী পথ হলো ভাষা, runtime, UI এবং platform build-কে আলাদা contract-সহ একসঙ্গে কাজ করানো। বর্তমান repo-র Go compiler/runtime, bytecode VM, HIR/MIR, bundle এবং Alap UI-এর উপর দাঁড়িয়ে কাজ শুরু করা যায়; কিন্তু iOS/Harmony support বা release-grade mobile experience-এর দাবি করার আগে নির্দিষ্ট backend, official toolchain integration এবং real-device evidence দরকার।

**সর্বোচ্চ অগ্রাধিকার:**

1. বর্তমান সক্ষমতার audit ও conformance test;
2. language semantics/runtime ABI স্থিতিশীল করা;
3. typed UI IR এবং lifecycle/capability model design;
4. Android-এ build → install → run → debug → persistence vertical slice;
5. common native bridge-এর contract এবং source-mapped diagnostics;
6. iOS/Xcode host এবং HarmonyOS/ArkTS-ArkUI host-এর ছোট proof of concept;
7. real-device test, signing, security, accessibility ও release checklist;
8. evidence পাওয়া গেলে ধাপে ধাপে আরও mobile APIs এবং optimization।

নীলাংয়ের বড় শক্তি হতে পারে নিজের ecosystem-এর জন্য একটি পরিষ্কার, portable app model—যেখানে developer এক ভাষায় logic এবং UI intent লিখবে, কিন্তু অপারেটিং সিস্টেমের বাস্তব সুবিধা ও নিয়ম হারাবে না। সেই লক্ষ্য অর্জনের জন্য তিনটি native platform backend থাকা কোনো ব্যর্থতা নয়; বরং একই ভাষা দিয়ে ভিন্ন platform-এর শক্তি ব্যবহার করার এটাই সবচেয়ে বাস্তবসম্মত স্থাপত্য।


## ৩৫. Compiler backend-এ কাজের প্রযুক্তিগত ক্রম

Mobile backend যোগ করার সময় compiler frontend-কে বারবার platform-specific করে ফেলবে না। ভাষার syntax ও semantics এক জায়গায় থাকবে; target lowering ও output generation আলাদা module-এ থাকবে। এর জন্য প্রতিটি compiler phase-এর input/output contract লিখে রাখো।

### ৩৫.১ Frontend contract

Frontend-এর দায়িত্ব source parse করা, name resolution, type checking, diagnostics এবং semantic metadata তৈরি। Android-এর permission বা Swift-এর type syntax এখানে hard-code করা উচিত নয়। Frontend-এর output-এ থাকবে typed program model, module graph, symbol table, source spans এবং capability usage summary।

### ৩৫.২ Target selection ও feature resolution

Build শুরুতে compiler নির্বাচিত target এবং backend capability table load করবে। একটি source program যে API/feature ব্যবহার করেছে তার set হিসাব করে backend-এর supported set-এর সঙ্গে তুলনা করবে।

```text
program capability set:
  [network, storage, camera]

selected target: harmony
backend support:
  network = stable
  storage = stable
  camera = experimental

result:
  build allowed only if project permits experimental APIs;
  otherwise emit a blocking diagnostic.
```

এই output conceptual। Backend capability table version-controlled ও testable হওয়া দরকার। `experimental` status build-এ defaultভাবে silently accept করা উচিত নয়। Project `allowExperimental` flag explicitভাবে সেট করলে warning/report-এ সেটি থাকবে।

### ৩৫.৩ IR boundary

`HIR/MIR` general-purpose language execution-এর কাজ করবে; `MobileAppIR` app lifecycle, routes, permissions, resource references এবং platform configuration-এর মতো mobile concepts প্রকাশ করবে; `UIIR` visual tree ও event binding প্রকাশ করবে। একই ধারণাকে HIR, MobileAppIR এবং generated source—তিন জায়গায় আলাদা semantics দিয়ে duplicate করো না। Lowering-এর প্রতিটি ধাপে invariant test থাকবে।

### ৩৫.৪ Native code generation-এর নিয়ম

Generated source মানুষের পড়ার উপযোগী হবে। Compiler generated filename, stable symbol naming, source comments ও mapping data তৈরি করবে। User code-এর identifier-কে naive string replacement দিয়ে platform language-এর code-এ বসালে escaping/injection bug তৈরি হতে পারে; typed AST/structured template generation ব্যবহার করো। String literal, resource path, package name, route name এবং annotation value সঠিকভাবে escape করতে হবে।

Generated project-এর native dependency version ও plugin version pinned থাকবে। Compiler-এ native source code string concatenate করে এক বিশাল file বানানোর পরিবর্তে ছোট, deterministic generator functions রাখো, এবং representative output golden tests দিয়ে compare করো। Native template update হলে generated-source diff সহজে inspect করা যাবে।

### ৩৫.৫ Diagnostics mapping

Compiler-এর internal representation থেকে error হলে original `.nil` span ধরে রাখবে। Generated Kotlin/Swift/ArkTS compiler error-এর সঙ্গে generated filename/line-কে original `.nil` source map-এর সঙ্গে map করার ব্যবস্থা রাখো। সব native error source line-এ map করা সম্ভব না হলে generated file ও host target-এর error দেখাবে; মিথ্যা source location বানাবে না। Diagnostic-এ “generated host file is invalid” বলার পাশাপাশি raw tool output debug details হিসেবে যুক্ত করা যেতে পারে।

### ৩৫.৬ Dependency এবং resource resolution

Resources-কে string literal path দিয়ে target-specific filesystem-এর উপর নির্ভরশীল করে তুলবে না। Project-relative asset ID/manifest দিয়ে resolve করো। একই image Android asset, iOS asset catalog ও Harmony resource directory-তে ভিন্ন path পেলেও Nilang code resource identifier ব্যবহার করবে। Resource missing, case mismatch, unsupported format বা duplicate identifier build-time error হবে। Font fallback এবং locale resource missing-এর policy নির্ধারিত থাকবে।

### ৩৫.৭ Incremental build

প্রথম release-এ build speed নিয়ে অতিরিক্ত কাজ প্রয়োজন নেই; correctness আগে। তবে architecture-এ incremental build-এর পথ রাখো:

- source unit hash;
- module dependency graph;
- target/config/toolchain version hash;
- generated code cache key;
- resource hash;
- dependency lockfile hash;
- stale artifact invalidation;
- build report-এ cache hit/miss।

Cache miss হলে rebuild; cache corruption হলে safe invalidation। Cross-project cached generated code-এর মধ্যে user-specific path বা secret ঢুকবে না।

### ৩৫.৮ Compiler plugin বা code macro-এর সীমা

Mobile target তৈরি করার জন্য unrestricted compile-time code execution বা arbitrary shell command চালানোর সুযোগ প্রথম থেকেই দিও না। Plugin system দরকার হলে তার trust model, sandbox, dependency versioning, deterministic output এবং reproducibility contract আলাদা করতে হবে। শুরুতে platform adapter-গুলো compiler-এর trusted built-in backend হিসেবে যথেষ্ট।

---

## ৩৬. চূড়ান্ত implementation review-এর প্রশ্ন

প্রতিটি বড় PR বা release review-এর আগে নিজেকে নিচের প্রশ্নগুলো করবে:

1. এই feature-এর contract কোথায় লেখা আছে?
2. Interpreter ও VM-এ আচরণ একই কি?
3. UI/runtime error হলে developer actionable diagnostic পাবে কি?
4. Android, iOS ও HarmonyOS-এর target difference কোথায় প্রকাশিত?
5. Unsupported feature compile-time-এ ধরা সম্ভব কি?
6. Permission request আর permission-granted state আলাদা কি?
7. Async task cancel এবং lifecycle dispose হলে callback leak হয় কি?
8. Generated project reproducible কি এবং native IDE-তে inspect করা যায় কি?
9. `.nilax`/dependency integrity validate হয় কি?
10. Test real device/emulator-এ হয়েছে, নাকি শুধু unit test?
11. Release build signed/validated কি, নাকি একটি unsigned artifact?
12. Build report-এ SDK/toolchain/ABI/version metadata আছে কি?
13. Accessibility, keyboard, font scale ও localization টেস্ট হয়েছে কি?
14. Existing stable API-র compatibility কীভাবে বজায় থাকবে?
15. Documentation এবং example একই version-এর সঙ্গে আপডেট হয়েছে কি?

যদি কোনো প্রশ্নের উত্তর “কাজ করবে বোধহয়” হয়, সেটি `verified` status পাওয়ার আগে পরীক্ষা দরকার। এই নিয়ম নীলাংয়ের গতি কমানোর জন্য নয়; ভুল abstraction-এর উপর মাসের পর মাস feature বানানোর খরচ কমানোর জন্য।

---

## ৩৭. শেষ কর্মপরিকল্পনা এক পাতায়

**প্রথম কাজ:** GitHub issue বা project board-এ একটি `Mobile Platform MVP` epic তৈরি করো। প্রথমে status audit, spec, runtime bridge, Android host scaffold এবং one-screen app-এর কাজ ভাগ করো।

**প্রথম deliverable:** বাস্তব Android ফোনে চলা একটি ছোট Nilang app, যার source থেকে debug package build হয়, button event নীলাং logic চালায়, error source line-এ দেখা যায় এবং app পুনরায় চালু করলে persisted data থাকে।

**দ্বিতীয় deliverable:** একই app logic ও UI intent-এর জন্য iOS-এ Xcode project এবং HarmonyOS-এ ArkTS/ArkUI host project তৈরি হয়; প্রতিটি official toolchain দিয়ে build ও install হয়। তখনই তিন প্ল্যাটফর্মের shared architecture বাস্তব প্রমাণ পাবে।

**তৃতীয় deliverable:** release signing, capabilities, permissions, accessibility, test matrix, package verification ও documentation-সহ প্রথম beta release।

এটাই সবচেয়ে বাস্তব ভিত্তি। নীলাংয়ের নিজস্ব ভাষাগত বৈশিষ্ট্য, Alap UI, bundle format, WASM, game integration এবং package tools সবই এই লক্ষ্যে ভবিষ্যতে মূল্য যোগ করতে পারে। কিন্তু mobile platform-এর জন্য প্রথম সাফল্য হবে একটি অ্যাপকে সত্যিই তৈরি, ইনস্টল, চালানো, ডিবাগ, আপডেট এবং নিরাপদভাবে বিতরণ করতে পারা—নতুন feature-এর সংখ্যা নয়।

## ৩৮. সিদ্ধান্ত নেওয়ার নিয়ম: কোন কাজটি পরের কাজ হবে?

Mobile platform-এর কাজের তালিকা সবসময় বড় হবে; তাই “সবচেয়ে আকর্ষণীয় feature” বেছে নেওয়া ঠিক নয়। পরের কাজটি বাছাই করার সময় চারটি প্রশ্ন ব্যবহার করো। প্রথমত, এই কাজটি কি source-to-device vertical slice-এর বাধা সরায়? দ্বিতীয়ত, এটি কি language semantics, runtime stability বা security risk কমায়? তৃতীয়ত, এটি কি একাধিক পরবর্তী feature-এর ভিত্তি? চতুর্থত, ফলাফল কি একটি নির্দিষ্ট test বা demo দিয়ে প্রমাণ করা যাবে? কোনো কাজ যদি এই প্রশ্নগুলোর একটিও পূরণ না করে, তবে সেটা অপেক্ষা করতে পারে।

একটি সহজ priority scoring ব্যবহার করা যায়: `Priority = (Impact × Confidence × Reuse) / Cost`। এখানে সব value আনুমানিক; সূত্রটি বৈজ্ঞানিক মাপ নয়, আলোচনা গোছানোর উপায়। Runtime bridge-এর কাজের reuse বেশি, কারণ Android, iOS ও HarmonyOS—তিনটিই সেটির উপর নির্ভর করতে পারে। নতুন easing curve-এর reuse তুলনামূলকভাবে কম, কারণ app package তৈরি বা lifecycle debugging তার উপর নির্ভর করে না। তাই bridge আগে, animation polish পরে।

### Go/No-Go gate

- **Gate 1 — Language:** conformance tests পাস না করলে নতুন backend feature freeze।
- **Gate 2 — Android prototype:** real-device vertical slice না থাকলে iOS/Harmony-তে API proliferation নয়; আগে native bridge ও UI IR validate।
- **Gate 3 — Cross-platform architecture:** iOS এবং Harmony host-এ একই core logic অন্তত একটি করে real build/run না করলে common abstraction সফল প্রমাণিত নয়।
- **Gate 4 — Beta:** security, permissions, lifecycle, update এবং signed package tests পাস না করলে `stable` label নয়।
- **Gate 5 — Stable API:** migration guide, compatibility promise ও documented limitations প্রকাশ না হলে external developers-কে breaking change-এর ঝুঁকিতে ফেলো না।

এই gate-গুলো কোনো নির্দিষ্ট মাসে বাধ্যতামূলকভাবে পার হতে হবে না। কোনো gate আটকে থাকলে তার কারণ নথিভুক্ত করো, scope কমাও, অথবা target status-কে experimental রাখো। অন্য target-এর progress দেখানোর জন্য অসম্পূর্ণ target-কে ready ঘোষণা করবে না।

### শেষ কথা

নীলাংয়ের ভবিষ্যৎ সাফল্য সবচেয়ে বেশি নির্ভর করবে তুমি কত দ্রুত একটি নির্ভরযোগ্য end-to-end workflow প্রমাণ করতে পারো তার উপর: `.nil` লিখলে compiler অর্থ বুঝবে, UI intent native host-এ রূপান্তরিত হবে, runtime নিরাপদভাবে কাজ করবে, real phone-এ app চলবে, সমস্যা হলে source-level diagnostics পাওয়া যাবে, এবং release package official toolchain দিয়ে তৈরি ও যাচাই করা যাবে। এই workflow স্থির হলে নতুন API, widget, engine এবং optimization যোগ করা অনেক সহজ হবে। workflow না থাকলে প্রতিটি feature আলাদা experiment হয়ে থাকবে।

## ৩৯. Public support statement কীভাবে লিখবে

যখন কোনো target বাস্তবে প্রস্তুত হবে, README-তে “সব ফোন support করে” না লিখে একটি ছোট, নির্ভুল support statement দেবে। উদাহরণস্বরূপ:

> Nilang Mobile Preview বর্তমানে নির্দিষ্ট Android SDK range এবং পরীক্ষিত ARM64 ডিভাইসে debug build ও মৌলিক UI/state/storage workflow সমর্থন করে। iOS এবং HarmonyOS backend উন্নয়নাধীন; build support দাবি করার আগে সংশ্লিষ্ট official toolchain, simulator/real-device installation, lifecycle, signing এবং regression tests যাচাই করা হবে। সমর্থিত API ও known limitations-এর বর্তমান তালিকা `docs/platform-capabilities.md`-এ আছে।

এটি একটি উদাহরণ, বর্তমান পণ্যের জন্য সরাসরি প্রকাশযোগ্য status নয়। বাস্তবে যে target/SDK/device-এ পরীক্ষা হয়েছে, সেই তথ্য বসাতে হবে। iOS এবং HarmonyOS-এর backend যখন release-verified হবে তখন statement আপডেট করো, কিন্তু পুরোনো version-এর limitation ইতিহাস থেকে মুছে ফেলো না। User যদি কোনো unsupported API দিয়ে app বানান, compiler আগেই সতর্ক করলে platform-টির উপর বিশ্বাস বাড়ে।

প্রতিটি release tag-এর সঙ্গে release notes, build-tool version matrix, conformance summary, tested-device matrix এবং security/known-issues summary attach করা যায়। এই নথিগুলো project-এর technical maturity-র প্রমাণ; README-র feature badge-এর চেয়ে এদের ওজন বেশি।

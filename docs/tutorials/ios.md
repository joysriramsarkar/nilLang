# iOS অ্যাপ টিউটোরিয়াল — একই নীলাং কোড, iPhone-এ

Android টিউটোরিয়াল শেষ করে থাকলে `src/main.nil` আগের মতোই আছে — ব্যবসায়িক
যুক্তি (`std/mobile`, `std/pos`, `std/text/bn`) প্ল্যাটফর্ম বদলালেও একই থাকে।
সময়: ~৩০ মিনিট (macOS + Xcode দরকার শুধু সাইনিং/IPA-এর জন্য)।

## পূর্বশর্ত (copy-paste)

```sh
nil version
xcodebuild -version   # macOS-এ
```

## ধাপ ১ — একই প্রজেক্ট ব্যবহার করো

```sh
cd amar-app
nil check src/main.nil
```

`src/main.nil` বদলানোর দরকার নেই — এটাই শেয়ার্ড-লজিক পদ্ধতির মূল কথা।

## ধাপ ২ — iOS টার্গেট বিল্ড

```sh
nil build ios
```

সফল হলে `dist/ios/`-এ XCFramework + Xcode প্রজেক্ট তৈরি হয়:

```sh
open dist/ios/AmarApp.xcodeproj
```

Xcode-এ Team বেছে Run চাপো (simulator বা ডিভাইস)।

## ধাপ ৩ — lifecycle যাচাই

App Delegate ব্রিজ `onCreate/onResume/onPause` ইভেন্ট পাঠায়। লগে দেখো:

```
lifecycle=resumed foreground=true
```

না দেখা গেলে `mobile.appState(app)` print করে অবস্থা দেখো।

## ধাপ ৪ — শেয়ার-শিট

```nil
import "std/mobile" as mobile;

let শেয়ার = mobile.shareLink("https://nilang.org", "নীলাং");
println(str(শেয়ার["shared"])); // true
```

## সমস্যা হলে

| লক্ষণ | করণীয় |
|---|---|
| Linux-এ `.ipa` চাই | ক্রস-কম্পাইল হয়; সাইনিং/IPA-তে macOS লাগে |
| SwiftUI preview ফাঁকা | NilangActivity-এর বদলে Swift ব্রিজ টার্গেট দেখো |

পরের ধাপ: `docs/tutorials/harmony.md`।

# শূন্য থেকে নীলাং

বাংলায় নীলাং প্রোগ্রামিং ভাষা শেখার ধাপে-ধাপে বই। কোনো পূর্ব-অভিজ্ঞতা ধরে নেওয়া
হয়নি — শুধু একটি কম্পিউটার আর কৌতূহল থাকলেই চলবে।

## কীভাবে পড়বে

- ক্রমানুসারে পড়ো: প্রতিটি অধ্যায় আগেরটির ওপর দাঁড়ায়।
- প্রতিটি উদাহরণ চালিয়ে দেখো: `nil run উদাহরণ.nil`।
- আটকে গেলে `nil doc <নাম>` দিয়ে বিল্ট-ইন/মডিউলের বিবরণ দেখো।

## সূচি

| অধ্যায় | শিরোনাম | অবস্থা |
|---|---|---|
| ১ | [প্রথম প্রোগ্রাম](অধ্যায়-১-প্রথম-প্রোগ্রাম.md) — ইনস্টল, `nil run`, print | ✅ পূর্ণ |
| ২ | [চলক, ফাংশন ও নিয়ন্ত্রণ](অধ্যায়-২-চলক-ফাংশন.md) — let/fn/if/while/for | ✅ পূর্ণ |
| ৩ | [মডিউল ও স্ট্যান্ডার্ড লাইব্রেরি](অধ্যায়-৩-মডিউল-stdlib.md) — import, std, POS দোকান | ✅ পূর্ণ |
| ৪ | টাইপ, struct, enum ও match | outline |
| ৫ | ভুল সামলানো: Result, Optional, testing | outline |
| ৬ | টাস্ক ও চ্যানেল: একসাথে অনেক কাজ | outline |
| ৭ | প্রথম মোবাইল অ্যাপ (Android) | outline |
| ৮ | টুলচেইন: fmt, check, doc, build | outline |

## Outline (upcoming অধ্যায়)

### ৪. টাইপ, struct, enum ও match
- `struct User { name: String, age: Int }`, struct literal, field access।
- `enum`, পেলোডসহ ভ্যারিয়েন্ট, `match` + guard।
- টাইপ অ্যানোটেশন `-> Int`, `?T`, জেনেরিক্সের ধারণা।

### ৫. ভুল সামলানো
- `Ok`/`Err`, `andThen`, `unwrapOr`; `Some`/`None`, `?.` ও `??`।
- `std/testing`: assert, mock, benchmark, golden।

### ৬. টাস্ক ও চ্যানেল
- `task`/`await`, `std/concurrency/pool`-এ bounded map।
- Channel দিয়ে producer/consumer; mutex কখন লাগে।

### ৭. প্রথম মোবাইল অ্যাপ
- `nil build android` দিয়ে APK; lifecycle; deep link; notify।
- iOS/Harmony পথের ধারণা (docs/tutorials দেখো)।

### ৮. টুলচেইন
- `nil fmt`, `nil check`, `nil doc --json`, `nil build` টার্গেট।
- VS Code এক্সটেনশন ও ডিবাগার।

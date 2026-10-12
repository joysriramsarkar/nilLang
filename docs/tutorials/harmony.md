# HarmonyOS অ্যাপ টিউটোরিয়াল — ArkTS + NAPI ব্রিজ

একই `src/main.nil` HarmonyOS-এ চলে; UI ArkUI, যুক্তি নীলাং। সময়: ~৩০ মিনিট।

## পূর্বশর্ত (copy-paste)

```sh
nil version
node --version   # NAPI টুলচেইনের জন্য
```

## ধাপ ১ — প্রজেক্ট যাচাই

```sh
cd amar-app
nil check src/main.nil
```

## ধাপ ২ — .hap বিল্ড

```sh
nil build harmony
```

সফল হলে `dist/harmony/`-এ NAPI ব্রিজ + ArkTS অ্যাপ তৈরি হয়। DevEco Studio-তে
খোলো ও emulator-এ চালাও।

## ধাপ ৩ — ArkTS থেকে ডাকা (copy-paste)

```ts
// entry/src/main/ets/pages/Index.arkts
import nilang from '@nilang/runtime';

@Entry
@Component
struct Index {
  @State total: string = '';

  aboutToAppear() {
    const receipt = nilang.call('pos.checkout', []);
    this.total = receipt.total;
  }

  build() {
    Column() {
      Text(`মোট: ${this.total}`)
    }
  }
}
```

## ধাপ ৪ — deep link (Harmony want)

```nil
import "std/mobile" as mobile;

let রুট = mobile.linkRoute("nilang://shop/product/42");
let কোড = mobile.parseLink("nilang://shop/product/42?ref=home")["query"];
```

## সমস্যা হলে

| লক্ষণ | করণীয় |
|---|---|
| OHOS NDK নেই | ArkTS সোর্স + NAPI কোড SDK ছাড়াই রিভিউ করা যায় |
| `.hap` সাইন হয় না | DevEco-তে debug সার্টিফিকেট বানাও |

মূল নীতি তিন প্ল্যাটফর্মেই এক: **নেটিভ UI অ্যাডাপ্টার + শেয়ার্ড নীলাং লজিক**।

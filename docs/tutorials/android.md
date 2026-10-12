# Android অ্যাপ টিউটোরিয়াল — নীলাং দিয়ে প্রথম APK

এই টিউটোরিয়ালের শেষে তোমার নীলাং অ্যাপ নিজের ফোনে চলবে। সময়: ~৩০ মিনিট।

## পূর্বশর্ত (copy-paste)

```sh
nil version
java -version        # JDK 17+
echo $ANDROID_HOME   # Android SDK path থাকতে হবে
```

## ধাপ ১ — প্রজেক্ট তৈরি

```sh
nil init amar-app --profile mobile
cd amar-app
```

`src/main.nil` খোলো ও লেখো:

```nil
import "std/mobile" as mobile;

let app = mobile.new("আমার অ্যাপ");
mobile.onResume(app);

let দাম = 150;
mobile.notify("মোট: " + str(দাম));
```

## ধাপ ২ — ডেস্কটপে যাচাই

```sh
nil run src/main.nil
nil check src/main.nil
```

দুটোই সবুজ হতে হবে। `mobile.notify` ডেস্কটপে সারি-ভুক্ত হয় (ক্র্যাশ নয়)।

## ধাপ ৩ — APK বিল্ড

```sh
nil build android
```

সফল হলে `dist/amar-app.apk` তৈরি হয়। USB-ডিবাগিং চালু ফোনে ইনস্টল করো:

```sh
adb install -r dist/amar-app.apk
```

## ধাপ ৪ — lifecycle + deep link যোগ করো

```nil
import "std/mobile" as mobile;

let app = mobile.new("আমার অ্যাপ");
mobile.onCreate(app);
mobile.onResume(app);

let পথ = mobile.linkRoute("nilang://shop/product/42");
println(পথ); // product
```

আবার ধাপ ২–৩ চালাও।

## সমস্যা হলে

| লক্ষণ | করণীয় |
|---|---|
| `ANDROID_HOME not set` | SDK path export করো |
| APK > 25MB | `nil build android --release` (AOT core) |
| notify দেখা যায় না | Android 13+: runtime permission দাও |

পরের ধাপ: `docs/tutorials/ios.md` (একই `src/main.nil` পুনঃব্যবহার করে)।

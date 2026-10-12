# অধ্যায় ৩ — মডিউল ও স্ট্যান্ডার্ড লাইব্রেরি

## ৩.১ `import` কেন

বড় প্রোগ্রাম অনেক ফাইলে ভাগ হয়। `import` অন্য ফাইলের জিনিস আনে:

```nil
import "std/text/bn" as bn;

println(bn.toBnDigits("2026")); // ২০২৬
```

`"std/..."` দিয়ে শুরু মানেই বিল্ট-ইন স্ট্যান্ডার্ড লাইব্রেরি; `./` দিয়ে শুরু
মানে পাশের ফাইল (`import "./দোকান.nil" as দোকান;`)।

## ৩.২ টাকার হিসাব (float ছাড়া)

টাকায় কখনো float ব্যবহার কোরো না — `std/money` fixed-point decimal দেয়:

```nil
import "std/money" as money;

let দাম = money.ofMajor(150, "BDT");   // ১৫০ টাকা
let কর = money.mulQty(দাম, 150, 1000); // ১৫% কর
let মোট = unwrap(money.add(দাম, কর));
println(money.format(মোট, "৳"));       // ৳172.50
```

`money.add` `Ok(...)` দেয় (ভুল মুদ্রা হলে `Err`) — তাই `unwrap` লাগে।

## ৩.৩ ছোট POS দোকান

`std/pos` দিয়ে পুরো বিক্রয়-হিসাব:

```nil
import "std/pos" as pos;

let তালিকা = pos.newCatalog();
pos.addItem(তালিকা, "চা", 20, "BDT");
pos.addItem(তালিকা, "কফি", 50, "BDT");

let কার্ট = pos.newCart("BDT");
unwrap(pos.addToCart(তালিকা, কার্ট, "চা", 2));
unwrap(pos.addToCart(তালিকা, কার্ট, "কফি", 1));

let রসিদ = pos.checkout(কার্ট, 10, 5); // ১০% ছাড়, ৫% কর
println(pos.formatReceipt(কার্ট, রসিদ, "৳"));
```

আউটপুট:

```
===== রসিদ =====
চা x2 @ ৳20.00
কফি x1 @ ৳50.00
------------------------
সাবটোটাল: ৳90.00
ছাড়: ৳9.00
কর: ৳4.05
মোট: ৳85.05
```

## ৩.৪ বাংলা লেখা ও সময়

```nil
import "std/text/bn" as bn;
import "std/time" as সময়;

println(bn.banglaYearText(2026, 10, 12)); // ১৪৩৩ বঙ্গাব্দ
println(সময়.format(0, "YYYY-MM-DD"));      // 1970-01-01
```

## ৩.৫ নিজের মডিউল

`সহায়ক.nil`:

```nil
let দ্বিগুণ = fn(x) { return x * 2; };
```

`মূল.nil`:

```nil
import "./সহায়ক.nil" as সহায়ক;
println(str(সহায়ক.দ্বিগুণ(21))); // 42
```

`export` না লিখলে সবকিছু export হয়; একটি `export` লিখলেই শুধু চিহ্নিতগুলো
বাইরে যায় (`private` ভেতরে থাকে)।

## ৩.৬ অনুশীলনী

1. `std/collections`-এর `map`/`filter` দিয়ে দামের তালিকা থেকে ১০০-এর বড়গুলো বাছো।
2. `std/data/migrate` দিয়ে `books` টেবিলের মাইগ্রেশন লেখো।
3. `nil doc std/pos` চালিয়ে মডিউল-রেফারেন্স পড়ো।

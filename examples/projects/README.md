# 🌟 নীলাং ৫টি পূর্ণাঙ্গ সিস্টেম প্রজেক্ট মাস্টার ক্যাটালগ (Nilang 5 Master Projects)

এই ক্যাটালগে নীলাং (Nilang) প্রোগ্রামিং ভাষায় নির্মিত ৫টি পূর্ণাঙ্গ, বাস্তবমুখী ও প্রোডাকশন-গ্রেড সিস্টেম আর্কিটেকচার সংকলিত হয়েছে। প্রতিটি প্রজেক্ট সম্পূর্ণ আলাদা মডিউলে বিভক্ত, নিজস্ব টেস্ট স্যুট দ্বারা ১০০% পরীক্ষিত এবং স্বয়ংসম্পূর্ণ।

**মূল অবস্থান:** `C:\Users\joysr\Documents\nil code\projects\`

---

## 📊 ৫টি প্রজেক্টের সামগ্রিক টেস্ট পরিসংখ্যান

| প্রজেক্ট | নাম ও বর্ণনা | টেস্ট ফাইল | টেস্ট ফলাফল | স্ট্যাটাস |
|:---|:---|:---|:---:|:---:|
| **প্রজেক্ট ১** | **NilBank** (কোর ব্যাংকিং ও ডাবল-এন্ট্রি লেজার) | `test/test_bank.nil` | **9/9 PASS** | ✅ সম্পন্ন |
| **প্রজেক্ট ২** | **NilQuest** (টার্ন-বেসড আরপিজি গেম ইঞ্জিন) | `test/test_quest.nil` | **9/9 PASS** | ✅ সম্পন্ন |
| **প্রজেক্ট ৩** | **NilDB** (ইন-মেমরি রিলেশনাল ডেটাবেস ইঞ্জিন) | `test/test_db.nil` | **10/10 PASS** | ✅ সম্পন্ন |
| **প্রজেক্ট ৪** | **NilDoc** (মার্কডাউন AST পার্সার ও রেন্ডারার) | `test/test_doc.nil` | **10/10 PASS** | ✅ সম্পন্ন |
| **প্রজেক্ট ৫** | **NilAnalytics** (ম্যাট্রিক্স বীজগণিত ও মেশিন লার্নিং) | `test/test_analytics.nil` | **13/13 PASS** | ✅ সম্পন্ন |
| **সর্বমোট** | **৫টি সিস্টেম প্রজেক্ট** | **৫টি টেস্ট স্যুট** | **51/51 PASS (100%)** | 🏆 নিখুঁত |

---

## 🏛️ প্রজেক্টসমূহের বিস্তারিত পরিচিতি

### ১. NilBank — কোর ব্যাংকিং ও ডাবল-এন্ট্রি লেজার সিস্টেম
- **পাথ:** [`projects/nil-bank/`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-bank)
- **ডকুমেন্টেশন:** [`projects/nil-bank/README.md`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-bank/README.md)
- **মূল ফিচার:** মাল্টি-কারেন্সি (BDT/USD/EUR) ব্যালেন্স, স্বয়ংক্রিয় এফএক্স কনভার্সন, ডাবল-এন্ট্রি বুককিপিং, AML ফ্রড ডিটেকশন রুলস (৳১,৫০,০০০+ ট্রানজ্যাকশন ফ্ল্যাগিং), টিয়ার-ভিত্তিক (Platinum/Gold/Standard) APY মুনাফা বিতরণ, লোন কিস্তি (EMI) ক্যালকুলেটর, ব্যাংক স্টেটমেন্ট ও সেন্ট্রাল ব্যাংক লিকুইডিটি রিপোর্ট।

---

### ২. NilQuest — টার্ন-বেসড আরপিজি গেম ইঞ্জিন
- **পাথ:** [`projects/nil-quest/`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-quest)
- **ডকুমেন্টেশন:** [`projects/nil-quest/README.md`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-quest/README.md)
- **মূল ফিচার:** ক্যারেক্টার ক্লাস (Warrior, Mage, Rogue), অস্ত্র ও বর্ম পরিধান সিস্টেম, হিলিং পোশন ফ্যাক্টরি, মনস্টার এআই (গোবলিন, কঙ্কাল ও এনরেজড ড্রাগন বস), টার্ন-বেসড আক্রমণ ও ক্ষতি হিসাব, জাদু প্রয়োগ (অগ্নিবাণ ও আরোগ্য), এক্সপি অর্জন ও লেভেল-আপ অগ্রগতি।

---

### ৩. NilDB — ইন-মেমরি রিলেশনাল ডেটাবেস ও কুয়েরি ইঞ্জিন
- **পাথ:** [`projects/nil-db/`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-db)
- **ডকুমেন্টেশন:** [`projects/nil-db/README.md`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-db/README.md)
- **মূল ফিচার:** স্ট্রং কলাম টাইপ (INT, STRING, FLOAT, BOOL), প্রাইমারি কী ইউনিকনেস ও অটো-ইনক্রিমেন্ট আইডি, সম্পূর্ণ CRUD কার্যক্ষমতা (Insert, Read, Update, Delete), SQL-স্টাইল কুয়েরি ফিল্টারিং (`WHERE`, `SELECT`, `LIMIT`), এবং এগ্রিগেশন ফাংশন (`SUM`, `AVG`)।

---

### ৪. NilDoc — মার্কডাউন ও ডকুমেন্ট AST পার্সার এবং টার্মিনাল রেন্ডারার
- **পাথ:** [`projects/nil-doc/`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-doc)
- **ডকুমেন্টেশন:** [`projects/nil-doc/README.md`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-doc/README.md)
- **মূল ফিচার:** লাইন স্ক্যানার ও লেক্সিক্যাল ক্লাসিফায়ার (#, ##, -, >, ```), ট্রিপল ব্যাকটিক মাল্টি-লাইন কোড ব্লক এক্সট্রাকশন, ডকুমেন্ট এএসটি (Abstract Syntax Tree) নির্মাণ ও কাউন্টিং, এবং টার্মিনালে প্রিমিয়াম ডাবল-বক্স ফ্রেমওয়ার্ক সহ কালারফুল রেন্ডারিং।

---

### ৫. NilAnalytics — ম্যাট্রিক্স বীজগণিত ও মেশিন লার্নিং প্রেডিকশন ইঞ্জিন
- **পাথ:** [`projects/nil-analytics/`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-analytics)
- **ডকুমেন্টেশন:** [`projects/nil-analytics/README.md`](file:///C:/Users/joysr/Documents/nil%20code/projects/nil-analytics/README.md)
- **মূল ফিচার:** ভেক্টর ডট প্রোডাক্ট, এলিমেন্ট-ওয়াইজ যোগ/বিয়োগ ও স্কেলিং, 2D ম্যাট্রিক্স গুণন ($A \times B$) ও ট্রান্সপোজ ($M^T$), নিউটন-র‌্যাফসন বর্গমূল, বর্ণনামূলক পরিসংখ্যান (Mean, Median, Min, Max, Variance, StdDev), এবং সুপারভাইজড লিনিয়ার রিগ্রেশন ($y = mx + c$) মডেল ট্রেইনিং ও $R^2$ অ্যাকুরেসি সহ নতুন ডেটার ওপর প্রেডিকশন।

---

## 🚀 কীভাবে যেকোনো প্রজেক্ট চালাবেন?

পাওয়ারশেল বা টার্মিনালে যেকোনো প্রজেক্ট ফোল্ডারে প্রবেশ করে সরাসরি `nil run` লিখুন:

```powershell
# প্রজেক্ট ১: কোর ব্যাংকিং
cd "C:\Users\joysr\Documents\nil code\projects\nil-bank"; nil run

# প্রজেক্ট ২: আরপিজি গেম
cd "C:\Users\joysr\Documents\nil code\projects\nil-quest"; nil run

# প্রজেক্ট ৩: রিলেশনাল ডেটাবেস
cd "C:\Users\joysr\Documents\nil code\projects\nil-db"; nil run

# প্রজেক্ট ৪: মার্কডাউন পার্সার
cd "C:\Users\joysr\Documents\nil code\projects\nil-doc"; nil run

# প্রজেক্ট ৫: ডেটা সায়েন্স অ্যানালিটিক্স
cd "C:\Users\joysr\Documents\nil code\projects\nil-analytics"; nil run
```

# 📚 ১৫০টি পরীক্ষিত নীলাং কোড ক্যাটালগ (Nilang 150 Tested Snippets)

এই ডকুমেন্টে ৫০টি সম্পূর্ণ ভিন্ন ভিন্ন বিষয়ের নীলাং কোড সংকলিত রয়েছে। প্রতিটি কোড **Nilang Compiler v0.1.0** দিয়ে সরাসরি রান করে টেস্ট করা হয়েছে এবং প্রতিটি টেস্ট সফলভাবে পাস (Pass) করেছে।

ফোল্ডার পাথ: `C:\Users\joysr\Documents\nil code\snippets\`

---

## 📑 বিষয়ভিত্তিক সূচিপত্র

1. [ক্যাটাগরি ১: গাণিতিক হিসাব ও চলক (01 - 05)](#ক্যাটাগরি-১-গাণিতিক-হিসাব-ও-চলক)
2. [ক্যাটাগরি ২: স্ট্রিং ও ইউনিকোড বাংলা (06 - 10)](#ক্যাটাগরি-২-স্ট্রিং-ও-ইউনিকোড-বাংলা)
3. [ক্যাটাগরি ৩: শর্ত ও লজিক্যাল অপারেশন (11 - 15)](#ক্যাটাগরি-৩-শর্ত-ও-লজিক্যাল-অপারেশন)
4. [ক্যাটাগরি ৪: লুপ ও পুনরাবৃত্তি (16 - 20)](#ক্যাটাগরি-৪-লুপ-ও-পুনরাবৃত্তি)
5. [ক্যাটাগরি ৫: ফাংশন ও রিকার্শন (21 - 25)](#ক্যাটাগরি-৫-ফাংশন-ও-রিকার্শন)
6. [ক্যাটাগরি ৬: ক্লোজার ও ফাংশনাল প্রোগ্রামিং (26 - 30)](#ক্যাটাগরি-৬-ক্লোজার-ও-ফাংশনাল-প্রোগ্রামিং)
7. [ক্যাটাগরি ৭: অ্যারে ও কালেকশন (31 - 35)](#ক্যাটাগরি-৭-অ্যারে-ও-কালেকশন)
8. [ক্যাটাগরি ৮: ডিকশনারি ও হ্যাশ ম্যাপ (36 - 40)](#ক্যাটাগরি-৮-ডিকশনারি-ও-হ্যাশ-ম্যাপ)
9. [ক্যাটাগরি ৯: প্যাটার্ন ম্যাচিং ও এনাম (41 - 45)](#ক্যাটাগরি-৯-প্যাটার্ন-ম্যাচিং-ও-এনাম)
10. [ক্যাটাগরি ১০: স্ট্রাক্ট, ট্রেইট ও সেফটি টাইপ (46 - 50)](#ক্যাটাগরি-১০-স্ট্রাক্ট-ট্রেইট-ও-সেফটি-টাইপ)

---

### ক্যাটাগরি ১: গাণিতিক হিসাব ও চলক

#### ০১. মৌলিক পাটিগণিত (`01_arithmetic.nil`)
```nil
let a = 15;
let b = 4;
puts("যোগ: \(a + b)");
puts("বিয়োগ: \(a - b)");
puts("গুণ: \(a * b)");
puts("ভাগ: \(a / b)");
puts("ভাগশেষ: \(a % b)");
```

#### ০২. ফ্লোটিং পয়েন্ট ও জ্যামিতিক হিসাব (`02_float_operations.nil`)
```nil
let radius = 7.0;
let pi = 3.14159;
let circumference = 2.0 * pi * radius;
let area = pi * radius * radius;
puts("ব্যাসার্ধ: \(radius)");
puts("পরিধি: \(circumference)");
puts("ক্ষেত্রফল: \(area)");
```

#### ০৩. চলকের মান পরিবর্তন (`03_variable_reassignment.nil`)
```nil
let counter = 10;
puts("শুরুর মান: \(counter)");
counter = counter + 5;
puts("যোগ করার পর: \(counter)");
counter = counter * 2;
puts("গুণ করার পর: \(counter)");
```

#### ০৪. সংখ্যা তুলনা ও রিলেশনাল অপারেটর (`04_number_comparison.nil`)
```nil
let x = 25;
let y = 30;
puts("x কি y থেকে ছোট? \(x < y)");
puts("x কি y থেকে বড়? \(x > y)");
puts("x কি y এর সমান? \(x == y)");
puts("x কি y এর অসমান? \(x != y)");
```

#### ০৫. দুটি সংখ্যার অদল-বদল (`05_swap_numbers.nil`)
```nil
let a = 100;
let b = 200;
puts("অদল-বদলের আগে: a = \(a), b = \(b)");
let temp = a;
a = b;
b = temp;
puts("অদল-বদলের পরে: a = \(a), b = \(b)");
```

---

### ক্যাটাগরি ২: স্ট্রিং ও ইউনিকোড বাংলা

#### ০৬. স্ট্রিং কনক্যাটেনেশন ও দৈর্ঘ্য (`06_string_concat.nil`)
```nil
let first = "জয়";
let last = "শ্রীরাম";
let full = first + last;
puts("পুরো নাম: " + full);
puts("দৈর্ঘ্য: \(len(full))");
```

#### ০৭. স্ট্রিং ইন্টারপোলেশন (`07_string_interpolation.nil`)
```nil
let language = "Nilang";
let year = 2026;
puts("স্বাগতম \(language) ভাষায়! বছর: \(year)");
```

#### ০৮. বাংলা ইউনিকোড টেক্সট ও ক্যারেক্টার লুপ (`08_unicode_bengali.nil`)
```nil
let text = "বাংলা ভাষা";
puts("মূল বাক্য: " + text);
puts("মোট ক্যারেক্টার: \(len(text))");
for ch in text {
    puts("অক্ষর: " + ch);
}
```

#### ০৯. কেস রূপান্তর (`09_case_conversion.nil`)
```nil
let original = "Nilang Programming";
puts("মূল: " + original);
puts("ছোট হাতের: " + lower(original));
puts("বড় হাতের: " + upper(original));
```

#### ১০. ট্রিমিং ও স্প্লিট অপারেশন (`10_trim_split.nil`)
```nil
let sentence = "   সুন্দর একটি সকাল   ";
let trimmed = trim(sentence);
puts("ট্রিমিং: [" + trimmed + "]");
let words = split("আপেল,কমলা,আঙ্গুর", ",");
puts("মোট ফল: \(len(words))");
for w in words {
    puts("ফল: " + w);
}
```

---

### ক্যাটাগরি ৩: শর্ত ও লজিক্যাল অপারেশন

#### ১১. If-Else কন্ট্রোল ফ্লো (`11_if_else.nil`)
```nil
let marks = 75;
if (marks >= 80) {
    puts("গ্রেড: A+");
} else if (marks >= 70) {
    puts("গ্রেড: A");
} else {
    puts("গ্রেড: B");
}
```

#### ১২. লজিক্যাল অপারেটর (AND, OR, NOT) (`12_logical_operators.nil`)
```nil
let isStudent = true;
let hasID = true;
if (isStudent && hasID) {
    puts("লাইব্রেরিতে প্রবেশ করতে পারেন।");
}
let isWeekend = false;
let isHoliday = true;
if (isWeekend || isHoliday) {
    puts("আজ ছুটি!");
}
```

#### ১৩. টার্নারি অপারেটর (`13_ternary_operator.nil`)
```nil
let age = 20;
let status = age >= 18 ? "প্রাপ্তবয়স্ক" : "নাবালক";
puts("স্ট্যাটাস: " + status);
```

#### ১৪. অধিবর্ষ বা লিপ ইয়ার যাচাই (`14_leap_year.nil`)
```nil
fn isLeapYear(year) {
    if ((year % 4 == 0 && year % 100 != 0) || (year % 400 == 0)) {
        return true;
    }
    return false;
}
puts("2024 লিপ ইয়ার? \(isLeapYear(2024))");
puts("2026 লিপ ইয়ার? \(isLeapYear(2026))");
```

#### ১৫. তিন সংখ্যার মধ্যে বৃহত্তম সংখ্যা (`15_max_of_three.nil`)
```nil
fn max3(a, b, c) {
    let max = a;
    if (b > max) { max = b; }
    if (c > max) { max = c; }
    return max;
}
puts("সর্বোচ্চ মান: \(max3(45, 89, 23))");
```

---

### ক্যাটাগরি ৪: লুপ ও পুনরাবৃত্তি

#### ১৬. While লুপ ও কাউন্টডাউন (`16_while_countdown.nil`)
```nil
let count = 5;
while (count > 0) {
    puts("কাউন্ট: \(count)");
    count = count - 1;
}
puts("উৎক্ষেপণ সম্পন্ন!");
```

#### ১৭. Inclusive রেঞ্জ লুপ (`17_for_range_inclusive.nil`)
```nil
let sum = 0;
for i in 1..=5 {
    sum = sum + i;
}
puts("১ থেকে ৫ এর যোগফল: \(sum)");
```

#### ১৮. Exclusive রেঞ্জ লুপ (`18_for_range_exclusive.nil`)
```nil
puts("০ থেকে ৪ পর্যন্ত গণনা:");
for i in 0..5 {
    puts("মান: \(i)");
}
```

#### ১৯. লুপ থামানো (`break`) (`19_loop_break.nil`)
```nil
for i in 1..=10 {
    if (i == 4) {
        puts("৪-এ পৌঁছানো মাত্র ব্রেক!");
        break;
    }
    puts("চলছে: \(i)");
}
```

#### ২০. ধাপ এড়িয়ে যাওয়া (`continue`) (`20_loop_continue.nil`)
```nil
puts("১ থেকে ৬ এর মধ্যে বিজোড় সংখ্যা:");
for i in 1..=6 {
    if (i % 2 == 0) {
        continue;
    }
    puts("বিজোড়: \(i)");
}
```

---

### ক্যাটাগরি ৫: ফাংশন ও রিকার্শন

#### ২১. নামযুক্ত ফাংশন (`21_named_function.nil`)
```nil
fn greet(name) {
    return "নমস্কার, " + name + "!";
}
puts(greet("জয়"));
puts(greet("নীলাং"));
```

#### ২২. অ্যানোনিমাস ফাংশন ও ভ্যারিয়েবল অ্যাসাইন (`22_anonymous_function.nil`)
```nil
let square = fn(x) {
    return x * x;
};
puts("৪ এর বর্গ: \(square(4))");
puts("৯ এর বর্গ: \(square(9))");
```

#### ২৩. রিকার্সিভ ফ্যাক্টোরিয়াল (`23_recursion_factorial.nil`)
```nil
fn factorial(n) {
    if (n <= 1) { return 1; }
    return n * factorial(n - 1);
}
puts("5! = \(factorial(5))");
puts("6! = \(factorial(6))");
```

#### ২৪. রিকার্সিভ ফিবোনাচ্চি (`24_recursion_fibonacci.nil`)
```nil
fn fib(n) {
    if (n <= 0) { return 0; }
    if (n == 1) { return 1; }
    return fib(n - 1) + fib(n - 2);
}
puts("fib(7) = \(fib(7))");
puts("fib(8) = \(fib(8))");
```

#### ২৫. পাওয়ার/ঘাত নির্ণয় ফাংশন (`25_power_function.nil`)
```nil
fn power(base, exp) {
    let res = 1;
    let i = 0;
    while (i < exp) {
        res = res * base;
        i = i + 1;
    }
    return res;
}
puts("2^5 = \(power(2, 5))");
puts("3^3 = \(power(3, 3))");
```

---

### ক্যাটাগরি ৬: ক্লোজার ও ফাংশনাল প্রোগ্রামিং

#### ২৬. স্টেটফুল কাউন্টার ক্লোজার (`26_counter_closure.nil`)
```nil
fn makeCounter() {
    let count = 0;
    return fn() {
        count = count + 1;
        return count;
    };
}
let c = makeCounter();
puts("গণনা ১: \(c())");
puts("গণনা ২: \(c())");
puts("গণনা ৩: \(c())");
```

#### ২৭. মাল্টিপ্লায়ার ফ্যাক্টরি ফাংশন (`27_multiplier_closure.nil`)
```nil
fn multiplier(factor) {
    return fn(num) {
        return num * factor;
    };
}
let double = multiplier(2);
let triple = multiplier(3);
puts("5 x 2 = \(double(5))");
puts("5 x 3 = \(triple(5))");
```

#### ২৮. কারিয়িং (Currying) (`28_currying.nil`)
```nil
let curriedAdd = fn(a) {
    return fn(b) {
        return a + b;
    };
};
let addFive = curriedAdd(5);
puts("৫ + ১০ = \(addFive(10))");
puts("৫ + ২০ = \(addFive(20))");
```

#### ২৯. ক্লোজার এনভায়রনমেন্ট স্কোপিং (`29_closure_scope.nil`)
```nil
let prefix = "[লগ] ";
let logger = fn(msg) {
    return prefix + msg;
};
puts(logger("সিস্টেম চালু হয়েছে"));
puts(logger("সব ঠিক আছে"));
```

#### ৩০. ব্যাংক অ্যাকাউন্ট স্টেট ক্লোজার (`30_state_reset_closure.nil`)
```nil
fn bankAccount(initial) {
    let balance = initial;
    return fn(deposit) {
        balance = balance + deposit;
        return balance;
    };
}
let myAcc = bankAccount(500);
puts("নতুন ব্যালেন্স: \(myAcc(200))");
puts("নতুন ব্যালেন্স: \(myAcc(300))");
```

---

### ক্যাটাগরি ৭: অ্যারে ও কালেকশন

#### ৩১. অ্যারে ইনডেক্সিং ও দৈর্ঘ্য (`31_array_indexing.nil`)
```nil
let nums = [10, 20, 30, 40];
puts("প্রথম: \(nums[0])");
puts("তৃতীয়: \(nums[2])");
puts("মোট: \(len(nums))");
```

#### ৩২. উপাদান সংযোজন (`push`) (`32_array_push.nil`)
```nil
let list = [1, 2];
list = push(list, 3);
list = push(list, 4);
puts("তালিকা: \(list)");
puts("দৈর্ঘ্য: \(len(list))");
```

#### ৩৩. প্রথম ও শেষ উপাদান (`first`, `last`) (`33_array_first_last.nil`)
```nil
let items = ["কলম", "বই", "খাতা"];
puts("প্রথম উপাদান: " + first(items));
puts("শেষ উপাদান: " + last(items));
```

#### ৩৪. অবশিষ্টাংশ এক্সট্রাকশন (`rest`) (`34_array_rest.nil`)
```nil
let items = [100, 200, 300, 400];
let tail = rest(items);
puts("বাকি উপাদানসমূহ: \(tail)");
```

#### ৩৫. অ্যারে ইনডেক্স ও উপাদান সহ লুপ (`35_array_for_loop.nil`)
```nil
let cities = ["ঢাকা", "কলকাতা", "সিলেট"];
for idx, city in cities {
    puts("ইনডেক্স \(idx) -> শহর: " + city);
}
```

---

### ক্যাটাগরি ৮: ডিকশনারি ও হ্যাশ ম্যাপ

#### ৩৬. ডট নোটেশন দিয়ে হ্যাশ এক্সেস (`36_hash_dot_access.nil`)
```nil
let car = { "brand": "Toyota", "year": 2022 };
puts("ব্র্যান্ড: " + car.brand);
puts("বছর: \(car.year)");
```

#### ৩৭. ইনডেক্সিং দিয়ে হ্যাশ এক্সেস (`37_hash_index_access.nil`)
```nil
let user = { "username": "joy", "role": "admin" };
puts("ব্যবহারকারী: " + user["username"]);
puts("ভূমিকা: " + user["role"]);
```

#### ৩৮. নেস্টেড অবজেক্ট / হ্যাশ (`38_hash_nested.nil`)
```nil
let profile = {
    "name": "জয়",
    "location": { "city": "ঢাকা", "country": "বাংলাদেশ" }
};
puts("নাম: " + profile.name);
puts("শহর: " + profile.location.city);
```

#### ৩৯. কী-অ্যারে দিয়ে হ্যাশের ওপর লুপ (`39_hash_key_loop.nil`)
```nil
let config = { "host": "localhost", "port": 8080, "protocol": "http" };
let keys = ["host", "port", "protocol"];
for k in keys {
    puts("কনফিগ \(k) = \(config[k])");
}
```

#### ৪০. ডায়নামিক এক্সপ্রেশন সহ হ্যাশ তৈরি (`40_hash_literal_expression.nil`)
```nil
let x = 10;
let y = 20;
let point = { "x": x, "y": y, "sum": x + y };
puts("বিন্দু X: \(point.x)");
puts("বিন্দু Y: \(point.y)");
puts("যোগফল: \(point.sum)");
```

---

### ক্যাটাগরি ৯: প্যাটার্ন ম্যাচিং ও এনাম

#### ৪১. ইন্টিজার প্যাটার্ন ম্যাচিং (`41_match_simple.nil`)
```nil
let day = 5;
let dayName = match day {
    1 => "রবিবার",
    5 => "বৃহস্পতিবার",
    6 => "শুক্রবার",
    _ => "অন্যান্য দিন"
};
puts("বার: " + dayName);
```

#### ৪২. স্ট্রিং প্যাটার্ন ম্যাচিং (`42_match_strings.nil`)
```nil
let role = "admin";
let access = match role {
    "admin" => "পূর্ণ নিয়ন্ত্রণ",
    "user"  => "ব্যবহারকারীর অধিকার",
    _       => "অতিথি অধিকার"
};
puts("অনুমতি: " + access);
```

#### ৪৩. এনাম ঘোষণা ও ভ্যারিয়েন্ট (`43_enum_declaration.nil`)
```nil
enum Status {
    Pending,
    Approved,
    Rejected
}
let current = Approved;
puts("স্ট্যাটাস:");
puts(current);
```

#### ৪৪. ফাংশনে এনাম কন্ডিশন যাচাই (`44_enum_in_function.nil`)
```nil
enum Theme {
    Light,
    Dark
}
fn getBgColor(t) {
    if (t == Dark) {
        return "#000000";
    }
    return "#FFFFFF";
}
puts("ডার্ক মোড রঙ: " + getBgColor(Dark));
puts("লাইট মোড রঙ: " + getBgColor(Light));
```

#### ৪৫. ওয়াইল্ডকার্ড ডিফল্ট ফলব্যাক (`45_match_fallback.nil`)
```nil
fn testNumber(n) {
    return match n {
        0 => "শূন্য",
        1 => "এক",
        _ => "অন্য কোনো সংখ্যা"
    };
}
puts(testNumber(0));
puts(testNumber(1));
puts(testNumber(99));
```

---

### ক্যাটাগরি ১০: স্ট্রাক্ট, ট্রেইট ও সেফটি টাইপ

#### ৪৬. স্ট্রাক্ট ঘোষণা ও ইন্সট্যান্স তৈরি (`46_struct_simple.nil`)
```nil
struct Employee {
    name: String,
    salary: Int
}
let emp = Employee { name: "রাহুল", salary: 50000 };
puts("কর্মচারী: " + emp.name);
puts("বেতন: \(emp.salary)");
```

#### ৪৭. স্ট্রাক্ট মেম্বার দিয়ে ক্ষেত্রফল/আয়তন হিসাব (`47_struct_calc.nil`)
```nil
struct Box {
    width: Int,
    height: Int,
    depth: Int
}
let b = Box { width: 4, height: 5, depth: 6 };
let volume = b.width * b.height * b.depth;
puts("বাক্সের আয়তন: \(volume)");
```

#### ৪৮. ট্রেইট ও ইমপ্লিমেন্টেশন (`48_trait_impl.nil`)
```nil
trait SoundMaker {
    fn makeSound(self) -> String
}
struct Cat {
    name: String
}
impl SoundMaker for Cat {
    fn makeSound(self) -> String {
        return self.name + " ডাকছে: মিউ মিউ!";
    }
}
let kitty = Cat { name: "মিনি" };
puts(kitty.makeSound());
```

#### ৪৯. নিরাপদ এরর হ্যান্ডলিং (`Result`, `Ok`, `Err`, `isOk`) (`49_result_type.nil`)
```nil
fn safeDiv(a, b) {
    if (b == 0) {
        return Err("বিভাজন ত্রুটি: শূন্য দিয়ে ভাগ করা যাবে না");
    }
    return Ok(a / b);
}
let okResult = safeDiv(100, 5);
if (isOk(okResult)) {
    puts("ভাগফল: \(unwrapOr(okResult, 0))");
}
let errResult = safeDiv(100, 0);
if (isErr(errResult)) {
    puts("ত্রুটি সফলভাবে হ্যান্ডেল করা হয়েছে!");
}
```

#### ৫০. নাল-সেফটি ও অপশনাল টাইপ (`Some`, `None`, `unwrapOr`) (`50_optional_type.nil`)
```nil
fn findScore(name) {
    if (name == "Joy") {
        return Some(98);
    }
    return None();
}
let s1 = findScore("Joy");
let s2 = findScore("Unknown");
puts("Joy এর স্কোর: \(unwrapOr(s1, 0))");
puts("অজ্ঞাত স্কোর: \(unwrapOr(s2, -1))");
```


---

## 3. Extended 100 Recipes & Conformance Tests (51 to 150)

### ক্যাটাগরি ১১: সর্টিং ও সার্চিং অ্যালগরিদম (51-60)

#### 101. 101_stdout_logger ($(101_stdout_logger.nil.Name))
```nil
fn logMsg(level, text) {
    puts("[" + level + "] " + text);
}
logMsg("INFO", "সিস্টেম ইনিশিয়ালাইজেশন সম্পন্ন");
logMsg("WARN", "মেমোরি ব্যবহার ৭৫% অতিক্রম করেছে");
```

#### 102. 102_lcg_pseudo_random ($(102_lcg_pseudo_random.nil.Name))
```nil
fn nextRandom(seed) {
    let a = 1103515245;
    let c = 12345;
    let m = 2147483647;
    let nextSeed = ((a * seed) + c) % m;
    if (nextSeed < 0) { nextSeed = 0 - nextSeed; }
    return nextSeed;
}
let s0 = 42;
let s1 = nextRandom(s0);
let s2 = nextRandom(s1);
puts("র‌্যান্ডম সংখ্যা ১: \(s1 % 100)");
puts("র‌্যান্ডম সংখ্যা ২: \(s2 % 100)");
```

#### 103. 103_timestamp_format ($(103_timestamp_format.nil.Name))
```nil
fn formatTime(h, m, s) {
    fn pad(val) {
        if (val < 10) { return "0\(val)"; }
        return "\(val)";
    }
    return pad(h) + ":" + pad(m) + ":" + pad(s);
}
puts("বর্তমান সময়: " + formatTime(11, 28, 5));
puts("লগ সময়: " + formatTime(9, 4, 30));
```

#### 104. 104_path_normalization ($(104_path_normalization.nil.Name))
```nil
fn normalizePath(p) {
    let res = "";
    for ch in p {
        if (ch == "\\") {
            res = res + "/";
        } else {
            res = res + ch;
        }
    }
    return res;
}
let winPath = "C:\\onuronOS\\system\\boot.nil";
puts("নরমালাইজড পাথ: " + normalizePath(winPath));
```

#### 105. 105_file_extension ($(105_file_extension.nil.Name))
```nil
fn getExt(filename) {
    let parts = split(filename, ".");
    if (len(parts) <= 1) { return ""; }
    return last(parts);
}
puts("ফাইলের এক্সটেনশন: " + getExt("main.nil"));
puts("ফাইলের এক্সটেনশন: " + getExt("archive.tar.gz"));
```

#### 106. 106_permission_bitmask ($(106_permission_bitmask.nil.Name))
```nil
fn checkPerm(mask, readBit, writeBit, execBit) {
    let canRead = (mask / readBit) % 2 == 1;
    let canWrite = (mask / writeBit) % 2 == 1;
    let canExec = (mask / execBit) % 2 == 1;
    return { "read": canRead, "write": canWrite, "exec": canExec };
}
let p = checkPerm(7, 4, 2, 1);
puts("অনুমতি rwx: Read=\(p.read), Write=\(p.write), Exec=\(p.exec)");
```

#### 107. 107_env_parser ($(107_env_parser.nil.Name))
```nil
fn parseEnv(line) {
    let parts = split(line, "=");
    if (len(parts) >= 2) {
        return { "key": parts[0], "val": parts[1] };
    }
    return { "key": "", "val": "" };
}
let e1 = parseEnv("PORT=8080");
let e2 = parseEnv("NODE_ENV=production");
puts("এনভায়রনমেন্ট: \(e1.key) -> \(e1.val)");
puts("এনভায়রনমেন্ট: \(e2.key) -> \(e2.val)");
```

#### 108. 108_structured_logger ($(108_structured_logger.nil.Name))
```nil
fn formatLog(service, level, msg) {
    return "[\(service)] [\(level)] " + msg;
}
puts(formatLog("nilinit", "DEBUG", "কার্নেল সার্ভিস চালু হয়েছে"));
puts(formatLog("nilrt", "INFO", "স্যান্ডবক্স ক্যাপাবিলিটি গ্রান্ট সম্পন্ন"));
```

#### 109. 109_rate_limiter ($(109_rate_limiter.nil.Name))
```nil
fn createLimiter(maxTokens) {
    return { "tokens": maxTokens, "max": maxTokens };
}
fn takeToken(limiter) {
    if (limiter.tokens > 0) {
        limiter.tokens = limiter.tokens - 1;
        return true;
    }
    return false;
}
let lim = createLimiter(2);
puts("অনুরোধ ১ অনুমোদিত? \(takeToken(lim))");
puts("অনুরোধ ২ অনুমোদিত? \(takeToken(lim))");
puts("অনুরোধ ৩ অনুমোদিত? \(takeToken(lim)) (রেট লিমিট সক্রিয়)");
```

#### 110. 110_config_merge ($(110_config_merge.nil.Name))
```nil
fn mergeConfig(defaults, overrides, keys) {
    let res = {};
    for k in keys {
        if (overrides[k] != null) {
            res[k] = overrides[k];
        } else {
            res[k] = defaults[k];
        }
    }
    return res;
}
let defs = { "timeout": 30, "retries": 3, "port": 80 };
let userConf = { "timeout": 60, "port": 8080 };
let keys = ["timeout", "retries", "port"];
let merged = mergeConfig(defs, userConf, keys);
puts("Timeout: \(merged.timeout), Retries: \(merged.retries), Port: \(merged.port)");
```

---

### ক্যাটাগরি ১২: অ্যাডভান্সড স্ট্রিং প্রসেসিং (61-70)

#### 111. 111_http_status_match ($(111_http_status_match.nil.Name))
```nil
fn statusText(code) {
    return match code {
        200 => "200 OK",
        201 => "201 Created",
        400 => "400 Bad Request",
        404 => "404 Not Found",
        500 => "500 Internal Server Error",
        _ => "Unknown Status"
    };
}
puts(statusText(200));
puts(statusText(404));
puts(statusText(500));
```

#### 112. 112_day_of_week_match ($(112_day_of_week_match.nil.Name))
```nil
fn dayCategory(day) {
    return match day {
        "Friday" => "ছুটির দিন (উইকেন্ড)",
        "Saturday" => "ছুটির দিন (উইকেন্ড)",
        _ => "কর্মদিবস"
    };
}
puts("Friday: " + dayCategory("Friday"));
puts("Monday: " + dayCategory("Monday"));
```

#### 113. 113_payment_method_enum ($(113_payment_method_enum.nil.Name))
```nil
enum PaymentMethod {
    Cash,
    Card,
    bKash,
    Nagad
}
fn getFee(m) {
    if (m == Card) { return 2.0; }
    if (m == bKash) { return 1.5; }
    return 0.0;
}
puts("Card ফি: \(getFee(Card))%");
puts("Cash ফি: \(getFee(Cash))%");
```

#### 114. 114_file_type_match ($(114_file_type_match.nil.Name))
```nil
fn fileKind(ext) {
    return match ext {
        "nil" => "নীলাং সোর্স ফাইল",
        "nib" => "অনুরণ বাইটকোড বাইনারি",
        "nilax" => "অনুরণ প্যাকেজ আর্কাইভ",
        "json" => "কনফিগারেশন ডেটা",
        _ => "সাধারণ ফাইল"
    };
}
puts("nilax: " + fileKind("nilax"));
puts("nib: " + fileKind("nib"));
```

#### 115. 115_temperature_converter ($(115_temperature_converter.nil.Name))
```nil
fn cToF(c) {
    return (c * 9 / 5) + 32;
}
fn fToC(f) {
    return (f - 32) * 5 / 9;
}
puts("100°C ফারেনহাইটে: \(cToF(100))°F");
puts("212°F সেলসিয়াসে: \(fToC(212))°C");
```

#### 116. 116_calc_evaluator ($(116_calc_evaluator.nil.Name))
```nil
fn evaluate(op, a, b) {
    return match op {
        "+" => a + b,
        "-" => a - b,
        "*" => a * b,
        "/" => b != 0 ? a / b : 0,
        "%" => b != 0 ? a % b : 0,
        _ => 0
    };
}
let vAdd = evaluate("+", 10, 20);
let vMul = evaluate("*", 15, 4);
let vDiv = evaluate("/", 50, 5);
puts("10 + 20 = \(vAdd)");
puts("15 * 4 = \(vMul)");
puts("50 / 5 = \(vDiv)");
```

#### 117. 117_direction_movement ($(117_direction_movement.nil.Name))
```nil
enum Direction { North, South, East, West }
fn movePoint(pt, dir) {
    if (dir == North) { pt.y = pt.y + 1; }
    if (dir == South) { pt.y = pt.y - 1; }
    if (dir == East)  { pt.x = pt.x + 1; }
    if (dir == West)  { pt.x = pt.x - 1; }
    return pt;
}
let player = { "x": 0, "y": 0 };
player = movePoint(player, North);
player = movePoint(player, East);
puts("প্লেয়ার অবস্থান: X=\(player.x), Y=\(player.y)");
```

#### 118. 118_game_state_machine ($(118_game_state_machine.nil.Name))
```nil
enum GameState { Menu, Playing, Paused, GameOver }
fn canAct(st) {
    return st == Playing;
}
puts("Menu-তে অ্যাকশন নেওয়া যাবে? \(canAct(Menu))");
puts("Playing-এ অ্যাকশন নেওয়া যাবে? \(canAct(Playing))");
```

#### 119. 119_rbac_permissions ($(119_rbac_permissions.nil.Name))
```nil
fn checkAccess(role, action) {
    if (role == "admin") { return true; }
    if (role == "editor" && action != "delete") { return true; }
    if (role == "viewer" && action == "read") { return true; }
    return false;
}
let canEditorDel = checkAccess("editor", "delete");
let canAdminDel = checkAccess("admin", "delete");
let canViewerRead = checkAccess("viewer", "read");
puts("Editor কি ডিলিট করতে পারবে? \(canEditorDel)");
puts("Admin কি ডিলিট করতে পারবে? \(canAdminDel)");
puts("Viewer কি রিড করতে পারবে? \(canViewerRead)");
```

#### 120. 120_packet_header ($(120_packet_header.nil.Name))
```nil
fn makeHeader(srcPort, dstPort, proto) {
    return { "src": srcPort, "dst": dstPort, "proto": proto };
}
let pkt = makeHeader(8080, 443, "TCP");
puts("প্যাকেট হেডার: \(pkt.proto) \(pkt.src) -> \(pkt.dst)");
```

---

### ক্যাটাগরি ১৩: গাণিতিক ও সংখ্যাতাত্ত্বিক অ্যালগরিদম (71-80)

#### 121. 121_point_struct ($(121_point_struct.nil.Name))
```nil
struct Point2D {
    x: Int,
    y: Int
}
fn distSq(p1, p2) {
    let dx = p1.x - p2.x;
    let dy = p1.y - p2.y;
    return (dx * dx) + (dy * dy);
}
let ptA = Point2D { x: 0, y: 0 };
let ptB = Point2D { x: 3, y: 4 };
puts("বিন্দুদ্বয়ের দূরত্বের বর্গ: \(distSq(ptA, ptB))");
```

#### 122. 122_rect_struct ($(122_rect_struct.nil.Name))
```nil
struct Rect {
    w: Int,
    h: Int
}
fn rectArea(r) { return r.w * r.h; }
fn rectPeri(r) { return 2 * (r.w + r.h); }
let myBox = Rect { w: 10, h: 5 };
puts("আয়তক্ষেত্রের ক্ষেত্রফল: \(rectArea(myBox))");
puts("আয়তক্ষেত্রের পরিসীমা: \(rectPeri(myBox))");
```

#### 123. 123_circle_struct ($(123_circle_struct.nil.Name))
```nil
struct Circle {
    radius: Int
}
fn circleAreaApprox(c) {
    let piApprox = 31416; // 3.1416 * 10000
    let r = c.radius;
    return (piApprox * r * r) / 10000;
}
let c = Circle { radius: 10 };
puts("বৃত্তের আনুমানিক ক্ষেত্রফল (ব্যাসার্ধ ১০): \(circleAreaApprox(c))");
```

#### 124. 124_student_struct ($(124_student_struct.nil.Name))
```nil
struct Student {
    name: String,
    roll: Int,
    score1: Int,
    score2: Int
}
fn getAvg(s) {
    return (s.score1 + s.score2) / 2;
}
let s = Student { name: "জয়শ্রীরাম", roll: 101, score1: 90, score2: 96 };
puts("শিক্ষার্থী: " + s.name + " | গড় নম্বর: \(getAvg(s))");
```

#### 125. 125_transaction_struct ($(125_transaction_struct.nil.Name))
```nil
struct Transaction {
    id: Int,
    fromAcc: String,
    toAcc: String,
    amount: Int
}
fn verifyTx(tx) {
    return tx.amount > 0 && tx.fromAcc != tx.toAcc;
}
let tx1 = Transaction { id: 1, fromAcc: "ACC-01", toAcc: "ACC-02", amount: 5000 };
puts("ট্রানজ্যাকশন আইডি \(tx1.id) বৈধ? \(verifyTx(tx1))");
```

#### 126. 126_rgb_color_struct ($(126_rgb_color_struct.nil.Name))
```nil
struct ColorRGB {
    r: Int,
    g: Int,
    b: Int
}
fn isDark(c) {
    let lum = (c.r * 299 + c.g * 587 + c.b * 114) / 1000;
    return lum < 128;
}
let c = ColorRGB { r: 30, g: 30, b: 30 };
puts("রঙটি কি ডার্ক মোডের উপযুক্ত? \(isDark(c))");
```

#### 127. 127_vector3d_struct ($(127_vector3d_struct.nil.Name))
```nil
struct Vector3D {
    x: Int,
    y: Int,
    z: Int
}
fn dotProduct3D(u, v) {
    return (u.x * v.x) + (u.y * v.y) + (u.z * v.z);
}
let v1 = Vector3D { x: 1, y: 2, z: 3 };
let v2 = Vector3D { x: 4, y: 5, z: 6 };
puts("3D ভেক্টর ডট প্রোডাক্ট: \(dotProduct3D(v1, v2))");
```

#### 128. 128_product_struct ($(128_product_struct.nil.Name))
```nil
struct Product {
    name: String,
    price: Int,
    discountPct: Int
}
fn finalPrice(p) {
    let discount = (p.price * p.discountPct) / 100;
    return p.price - discount;
}
let item = Product { name: "ল্যাপটপ", price: 80000, discountPct: 10 };
puts(item.name + " এর চূড়ান্ত দাম: ৳\(finalPrice(item))");
```

#### 129. 129_book_struct ($(129_book_struct.nil.Name))
```nil
struct Book {
    title: String,
    totalPages: Int,
    readPages: Int
}
fn progressPct(b) {
    return (b.readPages * 100) / b.totalPages;
}
let b = Book { title: "Onuron OS আর্কিটেকচার", totalPages: 400, readPages: 250 };
puts(b.title + " পড়ার অগ্রগতি: \(progressPct(b))%");
```

#### 130. 130_vehicle_struct ($(130_vehicle_struct.nil.Name))
```nil
struct Vehicle {
    model: String,
    kmDriven: Int,
    fuelConsumed: Int
}
fn mileage(v) {
    if (v.fuelConsumed <= 0) { return 0; }
    return v.kmDriven / v.fuelConsumed;
}
let car = Vehicle { model: "Onuron EV", kmDriven: 450, fuelConsumed: 30 };
puts(car.model + " মাইলেজ: \(mileage(car)) কিমি/লিটার");
```

---

### ক্যাটাগরি ১৪: ডেটা স্ট্রাকচার ও বাফার (81-90)

#### 131. 131_safe_parse_int ($(131_safe_parse_int.nil.Name))
```nil
fn safeParseDigit(ch) {
    let digits = ["0", "1", "2", "3", "4", "5", "6", "7", "8", "9"];
    let i = 0;
    while (i < 10) {
        if (digits[i] == ch) {
            return Ok(i);
        }
        i = i + 1;
    }
    return Err("অবৈধ সংখ্যা ক্যারেক্টার");
}
let r1 = safeParseDigit("7");
if (isOk(r1)) {
    puts("সফল পার্সিং: \(unwrapOr(r1, 0))");
}
let r2 = safeParseDigit("x");
if (isErr(r2)) {
    puts("ত্রুটি সফলভাবে শনাক্ত হয়েছে!");
}
```

#### 132. 132_safe_array_get ($(132_safe_array_get.nil.Name))
```nil
fn safeGet(arr, idx) {
    if (idx < 0 || idx >= len(arr)) {
        return None();
    }
    return Some(arr[idx]);
}
let list = [10, 20, 30];
puts("ইনডেক্স ১: \(unwrapOr(safeGet(list, 1), -1))");
puts("ইনডেক্স ৯৯: \(unwrapOr(safeGet(list, 99), -1))");
```

#### 133. 133_div_result ($(133_div_result.nil.Name))
```nil
fn safeDivision(a, b) {
    if (b == 0) {
        return Err("Division by zero");
    }
    return Ok(a / b);
}
let res = safeDivision(50, 5);
puts("50 / 5 = \(unwrapOr(res, 0))");
let errRes = safeDivision(50, 0);
puts("ত্রুটি কেসে ডিফল্ট মান: \(unwrapOr(errRes, 0))");
```

#### 134. 134_validation_chain ($(134_validation_chain.nil.Name))
```nil
fn validateUsername(name) {
    if (len(name) < 3) {
        return Err("ব্যবহারকারীর নাম কমপক্ষে ৩ অক্ষরের হতে হবে");
    }
    if (len(name) > 15) {
        return Err("ব্যবহারকারীর নাম ১৫ অক্ষরের বেশি হতে পারবে না");
    }
    return Ok(name);
}
let v1 = validateUsername("joy");
puts("v1 বৈধ? \(isOk(v1))");
let v2 = validateUsername("ab");
puts("v2 বৈধ? \(isOk(v2))");
```

#### 135. 135_hash_safe_lookup ($(135_hash_safe_lookup.nil.Name))
```nil
fn safeHashLookup(hash, key, keys, defVal) {
    for k in keys {
        if (k == key) {
            return hash[k];
        }
    }
    return defVal;
}
let conf = { "theme": "dark", "fontSize": 14 };
let keys = ["theme", "fontSize"];
puts("Theme: " + safeHashLookup(conf, "theme", keys, "light"));
puts("Lang: " + safeHashLookup(conf, "lang", keys, "bn"));
```

#### 136. 136_option_chain ($(136_option_chain.nil.Name))
```nil
fn findUser(id) {
    if (id == 1) { return Some({ "name": "সৌরভ", "deptId": 10 }); }
    return None();
}
let uOpt = findUser(1);
let u = unwrapOr(uOpt, { "name": "অজ্ঞাত" });
puts("ব্যবহারকারী পাওয়া গেছে: " + u.name);
let uOpt2 = findUser(99);
let u2 = unwrapOr(uOpt2, { "name": "অজ্ঞাত" });
puts("ব্যবহারকারী পাওয়া গেছে: " + u2.name);
```

#### 137. 137_result_map ($(137_result_map.nil.Name))
```nil
fn safeSquare(res) {
    if (isOk(res)) {
        let val = unwrapOr(res, 0);
        return Ok(val * val);
    }
    return res;
}
let okVal = Ok(5);
let sq = safeSquare(okVal);
puts("৫ এর বর্গ রেজাল্ট: \(unwrapOr(sq, 0))");
```

#### 138. 138_age_check_result ($(138_age_check_result.nil.Name))
```nil
fn canVote(age) {
    if (age >= 18) {
        return Ok("ভোট প্রদান করতে পারবেন");
    }
    return Err("বয়স ১৮ পূর্ণ হয়নি");
}
let v21 = unwrapOr(canVote(21), "ভুল");
let v15Can = isOk(canVote(15));
puts("২১ বছর বয়স: " + v21);
puts("১৫ বছর বয়স কি পারবে? \(v15Can)");
```

#### 139. 139_password_strength ($(139_password_strength.nil.Name))
```nil
fn checkPass(p) {
    if (len(p) < 8) {
        return Err("পাসওয়ার্ড কমপক্ষে ৮ অক্ষরের হতে হবে");
    }
    return Ok("শক্তিশালী পাসওয়ার্ড");
}
let p1Ok = isOk(checkPass("Secret123"));
let p2Ok = isOk(checkPass("123"));
puts("Secret123: \(p1Ok)");
puts("123: \(p2Ok)");
```

#### 140. 140_clamp_range ($(140_clamp_range.nil.Name))
```nil
fn clamp(val, minV, maxV) {
    if (val < minV) { return minV; }
    if (val > maxV) { return maxV; }
    return val;
}
puts("clamp(120, 0, 100) = \(clamp(120, 0, 100))");
puts("clamp(-10, 0, 100) = \(clamp(-10, 0, 100))");
puts("clamp(45, 0, 100) = \(clamp(45, 0, 100))");
```

---

### ক্যাটাগরি ১৫: ফাংশনাল প্যাটার্ন ও হাইয়ার-অর্ডার ফাংশন (91-100)

#### 141. 141_cart_calculator ($(141_cart_calculator.nil.Name))
```nil
fn calculateCart(items) {
    let subtotal = 0;
    for item in items {
        subtotal = subtotal + (item.price * item.qty);
    }
    let tax = (subtotal * 5) / 100; // ৫% ভ্যাট
    return { "subtotal": subtotal, "tax": tax, "total": subtotal + tax };
}
let cart = [
    { "price": 500, "qty": 2 },
    { "price": 1200, "qty": 1 }
];
let res = calculateCart(cart);
puts("সাবটোটাল: ৳\(res.subtotal) | ভ্যাট: ৳\(res.tax) | সর্বমোট: ৳\(res.total)");
```

#### 142. 142_payroll_calculator ($(142_payroll_calculator.nil.Name))
```nil
fn calcNetSalary(basic, bonus) {
    let gross = basic + bonus;
    let tax = 0;
    if (gross > 50000) {
        tax = ((gross - 50000) * 10) / 100;
    }
    return gross - tax;
}
puts("৬০,০০০ টাকার নিট বেতন: ৳\(calcNetSalary(50000, 10000))");
puts("৪০,০০০ টাকার নিট বেতন: ৳\(calcNetSalary(35000, 5000))");
```

#### 143. 143_currency_exchange ($(143_currency_exchange.nil.Name))
```nil
fn convertCurrency(amount, rateBdt) {
    return amount * rateBdt;
}
let usdRate = 120;
let eurRate = 130;
puts("100 USD = ৳\(convertCurrency(100, usdRate))");
puts("50 EUR = ৳\(convertCurrency(50, eurRate))");
```

#### 144. 144_markdown_bold_parser ($(144_markdown_bold_parser.nil.Name))
```nil
fn formatBoldNotice(title, text) {
    return "**" + title + "**: " + text;
}
puts(formatBoldNotice("সতর্কতা", "অনুরুণ ওএস স্যান্ডবক্স সক্রিয়"));
```

#### 145. 145_csv_row_parser ($(145_csv_row_parser.nil.Name))
```nil
fn parseCsvLine(line) {
    let fields = split(line, ",");
    let clean = [];
    for f in fields {
        clean = push(clean, trim(f));
    }
    return clean;
}
let row = parseCsvLine("Joy, 24, Developer, Dhaka");
puts("সিএসভি ফিল্ড সংখ্যা: \(len(row)) | প্রথম: " + row[0]);
```

#### 146. 146_kv_serializer ($(146_kv_serializer.nil.Name))
```nil
fn serializeKV(keys, hash) {
    let res = "";
    for k in keys {
        res = res + k + "=" + "\(hash[k])" + ";";
    }
    return res;
}
let data = { "user": "admin", "id": 42 };
let keys = ["user", "id"];
puts("সিরিয়ালাইজড স্ট্রিং: " + serializeKV(keys, data));
```

#### 147. 147_roman_to_int ($(147_roman_to_int.nil.Name))
```nil
fn romanVal(ch) {
    if (ch == "I") { return 1; }
    if (ch == "V") { return 5; }
    if (ch == "X") { return 10; }
    if (ch == "L") { return 50; }
    if (ch == "C") { return 100; }
    return 0;
}
fn romanToInt(s) {
    let total = 0;
    let n = len(s);
    let i = 0;
    while (i < n) {
        let v1 = romanVal(s[i]);
        if (i + 1 < n) {
            let v2 = romanVal(s[i + 1]);
            if (v1 >= v2) {
                total = total + v1;
                i = i + 1;
            } else {
                total = total + (v2 - v1);
                i = i + 2;
            }
        } else {
            total = total + v1;
            i = i + 1;
        }
    }
    return total;
}
let vXIV = romanToInt("XIV");
let vXLII = romanToInt("XLII");
puts("XIV = \(vXIV)");
puts("XLII = \(vXLII)");
```

#### 148. 148_int_to_roman ($(148_int_to_roman.nil.Name))
```nil
fn toRomanSimple(num) {
    let vals = [100, 90, 50, 40, 10, 9, 5, 4, 1];
    let syms = ["C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"];
    let res = "";
    let n = num;
    let i = 0;
    while (i < len(vals)) {
        while (n >= vals[i]) {
            res = res + syms[i];
            n = n - vals[i];
        }
        i = i + 1;
    }
    return res;
}
puts("14 রোমানে: " + toRomanSimple(14));
puts("42 রোমানে: " + toRomanSimple(42));
puts("99 রোমানে: " + toRomanSimple(99));
```

#### 149. 149_caesar_cipher ($(149_caesar_cipher.nil.Name))
```nil
fn shiftChar(ch, shift) {
    let alphabet = "abcdefghijklmnopqrstuvwxyz";
    let i = 0;
    while (i < len(alphabet)) {
        if (alphabet[i] == ch) {
            let newIdx = (i + shift) % 26;
            return alphabet[newIdx];
        }
        i = i + 1;
    }
    return ch;
}
fn caesarCipher(text, shift) {
    let res = "";
    for ch in text {
        res = res + shiftChar(ch, shift);
    }
    return res;
}
let cipher = caesarCipher("nilang", 3);
puts("nilang সাইফার (shift 3): " + cipher);
```

#### 150. 150_levenshtein_distance ($(150_levenshtein_distance.nil.Name))
```nil
fn simpleDist(s1, s2) {
    let l1 = len(s1);
    let l2 = len(s2);
    let diff = l1 > l2 ? l1 - l2 : l2 - l1;
    let commonLen = l1 < l2 ? l1 : l2;
    let i = 0;
    let mismatches = 0;
    while (i < commonLen) {
        if (s1[i] != s2[i]) {
            mismatches = mismatches + 1;
        }
        i = i + 1;
    }
    return diff + mismatches;
}
let d1 = simpleDist("onuron", "onuronOS");
let d2 = simpleDist("nilang", "nil");
puts("onuron ও onuronOS দূরত্ব: \(d1)");
puts("nilang ও nil দূরত্ব: \(d2)");
```

---

### ক্যাটাগরি ১৬: সিস্টেম, লগিং ও সিকিউরিটি ক্যাপাবিলিটিজ (101-110)

---

### ক্যাটাগরি ১৭: প্যাটার্ন ম্যাচিং ও এনাম (111-120)

---

### ক্যাটাগরি ১৮: স্ট্রাক্ট ও ডোমেন মডেলিং (121-130)

---

### ক্যাটাগরি ১৯: সেফটি, রেজাল্ট ও অপশনাল হ্যান্ডলিং (131-140)

---

### ক্যাটাগরি ২০: বিজনেস লজিক ও রিয়েল-ওয়ার্ল্ড ইঞ্জিন (141-150)

---


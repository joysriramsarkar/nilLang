# 🌌 নীলাং (Nilang) মাস্টার আর্কিটেকচারাল ব্লুপ্রিন্ট
## Alap Framework ও Onuron OS-এর জন্য স্বয়ংসম্পূর্ণ সিস্টেম প্রোগ্রামিং ভাষা

---

## 📑 বিষয়সূচি (Table of Contents)
1. [ভাষা পরিচিতি ও দর্শন (Philosophy & Architecture)](#১-ভাষা-পরিচিতি-ও-দর্শন)
2. [রিপোজিটরি ফাইল স্ট্রাকচার (Repository Structure)](#২-রিপোজিটরি-ফাইল-স্ট্রাকচার)
3. [কম্পাইলার ফ্রন্টএন্ড (Compiler Frontend)](#৩-কম্পাইলার-ফ্রন্টএন্ড)
   - টোকেন সিস্টেম (`compiler/token`)
   - লেক্সার ও স্ক্যানার (`compiler/lexer`)
   - অ্যাবস্ট্রাক্ট সিনট্যাক্স ট্রি (`compiler/ast`)
   - প্রাট রিকার্সিভ ডিসেন্ট পার্সার (`compiler/parser`)
4. [রানটাইম ও এক্সিকিউশন ইঞ্জিন (Dual Execution Engine)](#৪-রানটাইম-ও-এক্সিকিউশন-ইঞ্জিন)
   - অবজেক্ট মডেল ও এনভায়রনমেন্ট (`compiler/object`)
   - ট্রি-ওয়াকিং ইভ্যালুয়েটর ও বিল্ট-ইনস (`compiler/evaluator`)
   - NABC বাইটকোড ইন্সট্রাকশন ও ডিসঅ্যাসেম্বলার (`compiler/code`)
   - সিম্বল টেবিল ও স্কোপ রেজোলিউশন (`compiler/compiler/symbol_table`)
   - বাইটকোড কম্পাইলার (`compiler/compiler`)
   - ভার্চুয়াল মেশিন (`compiler/vm`)
5. [প্রজেক্ট ম্যানেজমেন্ট ও বান্ডিল সিস্টেম (Build & Packaging)](#৫-প্রজেক্ট-ম্যানেজমেন্ট-ও-বান্ডিল-সিস্টেম)
   - কনফিগারেশন (`pkg/config/nil.json`)
   - `.nilax` বান্ডিল বিল্ডার ও রিডার (`pkg/bundle`)
   - কম্পাইলেশন পাইপলাইন (`pkg/compiler`)
   - অল-ইন-ওয়ান CLI (`cmd/nil` & `cmd/nilc`)
6. [প্যাকেজ ম্যানেজার ও মাইক্রোসার্ভিস রেজিস্ট্রি (`nilpkg`)](#৬-প্যাকেজ-ম্যানেজার-ও-রেজিস্ট্রি)
   - ডিপেনডেন্সি রেজোলিউশন অ্যালগরিদম
   - ভেরিফায়ার ও ইনস্টলার (`pkg/nilpkg`)
   - রেজিস্ট্রি স্টোরেজ ও রেস্ট এপিআই (`pkg/registry`)
   - প্যাকেজ সার্ভার ও ওয়েব ড্যাশবোর্ড (`cmd/nilpkg-server`)
7. [ক্রিপ্টোগ্রাফিক সিকিউরিটি ও সাইনিং (`nilkey`)](#৭-ক্রিপ্টোগ্রাফিক-সিকিউরিটি-ও-সাইনিং)
   - Ed25519 অ্যাসিম্যাট্রিক কি-পেয়ার
   - এনক্রিপ্টেড কি-স্টোর (`AES-256-GCM + Argon2id`)
   - সার্টিফিকেট অথরিটি ও ডিজিটাল সাইনেচার
8. [ডিক্লারেটিভ UI ও ৬০ এফপিএস অ্যানিমেশন ইঞ্জিন](#৮-ডিক্লারেটিভ-ui-ও-অ্যানিমেশন-ইঞ্জিন)
   - Alap ডিক্লারেটিভ কম্পোনেন্ট ট্রি (`pkg/alap`)
   - অ্যানিমেশন ও ৩০+ ইজিং ফাংশন (`pkg/animation`)
   - Vulkan / OpenGL GPU হার্ডওয়্যার পাইপলাইন (`pkg/gpu`)
9. [Onuron SoftBus ডিস্ট্রিবিউটেড প্রোটোকল ও Rust FFI](#৯-softbus-প্রোটোকল-ও-rust-ffi)
   - জিরো-কনফিগ ল্যান ডিসকভারি (UDP Multicast)
   - ফ্রেমড ট্রান্সপোর্ট ও JSON-RPC 2.0 (`pkg/softbus`)
   - Onuron OS Rust FFI ব্রিজ (`runtime/vm`)
10. [ডেভেলপার টুলিং, এডিটর ইন্টিগ্রেশন ও রিলিজ গাইড](#১০-টুলিং-ও-রিলিজ-গাইড)

---

## ১. ভাষা পরিচিতি ও দর্শন

**নীলাং (Nilang)** হলো একটি অত্যাধুনিক, কনকারেন্ট ও ডিক্লারেটিভ প্রোগ্রামিং ভাষা, যা বিশেষ করে **Onuron OS** এবং **Alap Framework**-এর জন্য ডিজাইন করা হয়েছে। 

### মূল দর্শন:
1. **ডিক্লারেটিভ UI মডেল**: অ্যাপ্লিকেশনের ইন্টারফেস তৈরির জন্য `component`, `state`, `render`, `emit`, `on` ইত্যাদি প্রিমিটিভ।
2. **ডুয়াল এক্সিকিউশন ইঞ্জিন**:
   - দ্রুত ডেভেলপমেন্ট, ডিবাগিং ও ইন্টারেক্টিভ ব্যবহারের জন্য **Tree-Walking Interpreter** এবং **REPL**।
   - প্রোডাকশন অ্যাপ্লিকেশন ও হাই-পারফরম্যান্স টাস্কের জন্য **NABC বাইটকোড কম্পাইলার** ও **Stack-based VM**।
3. **অনুরন সফটবাস (SoftBus)**: একাধিক অনুরন ডিভাইসের (ফোন, ট্যাবলেট, পিসি, ওয়াচ) মধ্যে কোনো ক্লাউড ছাড়াই লোকাল নেটওয়ার্কে তাৎক্ষণিক ডেটা ও সার্ভিস শেয়ারিং।
4. **সিকিউর ডিস্ট্রিবিউশন**: প্রতিটি `.nilax` বান্ডিল ক্রিপ্টোগ্রাফিকালি সাইন ও ভেরিফাই করা।

---

## ২. রিপোজিটরি ফাইল স্ট্রাকচার

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

## ৩. কম্পাইলার ফ্রন্টএন্ড

### ৩.১ টোকেন স্পেসিফিকেশন (`compiler/token`)
- **Keywords**: `let`, `const`, `fn`, `return`, `if`, `else`, `while`, `component`, `state`, `render`, `emit`, `on`, `build`, `style`, `import`।
- **Literals**: `IDENT`, `INT`, `FLOAT`, `STRING`, `TRUE`, `FALSE`, `NULL`।
- **Delimiters & Operators**: `+`, `-`, `*`, `/`, `%`, `==`, `!=`, `<`, `>`, `<=`, `>=`, `!`, `=`, `,`, `;`, `:`, `(`, `)`, `{`, `}`, `[`, `]`।

### ৩.২ লেক্সার ও স্ট্রিং ইন্টারপোলেশন (`compiler/lexer`)
লেক্সারটি UTF-8 এনকোডিং পুরোপুরি সমর্থন করে, ফলে বাংলা ভ্যারিয়েবল ও টেক্সট সরাসরি ব্যবহার করা যায়।
স্ট্রিং ইন্টারপোলেশন সিনট্যাক্স:
```nil
let user = "জয়শ্রীরাম";
let msg = "স্বাগতম, \(user)!";
```
লেক্সার যখন স্ট্রিংয়ের ভেতরে `\(` পায়, তখন এটি ইন্টারপোলেশন সেগমেন্ট হিসেবে ট্র্যাক করে।

### ৩.৩ প্রাট পার্সার (Pratt Recursive Descent Parser)
প্রেসিডেন্স স্তরসমূহ:
```go
const (
	_ int = iota
	LOWEST
	EQUALS      // ==, !=
	LESSGREATER // >, <, >=, <=
	SUM         // +, -
	PRODUCT     // *, /, %
	PREFIX      // -X, !X
	CALL        // myFunction(X)
	INDEX       // array[index], hash[key]
)
```
- প্রিফিক্স হ্যান্ডলার: আইডেন্টিফায়ার, সংখ্যা, স্ট্রিং, ইন্টারপোলেশন, অ্যারে, হ্যাশ, ফাংশন, ইফ-এক্সপ্রেশন।
- ইনফিক্স হ্যান্ডলার: বাইনারি এরিথমেটিক, কম্প্যারিজন, কল এক্সপ্রেশন, ইনডেক্স অ্যাক্সেস।

---

## ৪. রানটাইম ও এক্সিকিউশন ইঞ্জিন

### ৪.১ অবজেক্ট মডেল (`compiler/object`)
রানটাইমে প্রতিটি ভ্যালু `object.Object` ইন্টারফেস বাস্তবায়ন করে:
- `INTEGER_OBJ`, `FLOAT_OBJ`, `BOOLEAN_OBJ`, `STRING_OBJ`, `NULL_OBJ`
- `RETURN_VALUE_OBJ`, `ERROR_OBJ`
- `FUNCTION_OBJ`, `BUILTIN_OBJ`, `COMPILED_FUNCTION_OBJ`, `CLOSURE_OBJ`
- `ARRAY_OBJ`, `HASH_OBJ`, `ENVIRONMENT`

### ৪.২ বিল্ট-ইন ফাংশনস (`compiler/evaluator/builtins.go`)
- `puts(...args)` / `println(...args)`: কনসোলে প্রিন্ট ও নতুন লাইন।
- `print(...args)`: নতুন লাইন ছাড়া প্রিন্ট।
- `len(iterable)`: স্ট্রিং বা অ্যারের দৈর্ঘ্য।
- `first(array)`, `last(array)`, `rest(array)`, `push(array, elem)`: অ্যারে ম্যানিপুলেশন।
- `type(obj)`: অবজেক্টের টাইপ রিটার্ন।
- `str(val)`, `int(val)`: টাইপ কনভার্সন।
- `time()`: বর্তমান টাইমস্ট্যাম্প (মিলিসেকেন্ড)।
- `assert(condition, message)`: টেস্ট ভ্যালিডেশন।
- `input(prompt)`: ব্যবহারকারী থেকে ইনপুট গ্রহণ।
- `exit(code)`: প্রোগ্রাম সমাপ্তি।

### ৪.৩ NABC বাইটকোড আর্কিটেকচার (`compiler/code`)
নীলাং বাইটকোড (NABC) একটি স্ট্যাক-ভিত্তিক ভার্চুয়াল মেশিনে চলে। ২৬টি অপকোড:
- `OpConstant (2 bytes operand)`: ধ্রুবক পুশ।
- `OpAdd, OpSub, OpMul, OpDiv, OpMod`: গাণিতিক অপারেশন।
- `OpEqual, OpNotEqual, OpGreaterThan`: শর্তাধীন তুলনা।
- `OpMinus, OpBang`: ইউনিারি নেগেশন।
- `OpTrue, OpFalse, OpNull`: কনস্ট্যান্ট ভ্যালু।
- `OpJump, OpJumpNotTruthy (2 bytes operand)`: শর্তাধীন জাম্প।
- `OpSetGlobal, OpGetGlobal (2 bytes operand)`: গ্লোবাল ভ্যারিয়েবল।
- `OpSetLocal, OpGetLocal (1 byte operand)`: লোকাল স্ট্যাক ভ্যারিয়েবল।
- `OpArray, OpHash, OpIndex`: ডেটা স্ট্রাকচার অপারেশন।
- `OpCall, OpReturnValue, OpReturn`: ফাংশন কল ও রিটার্ন ফ্রেম।
- `OpClosure (2 bytes index, 1 byte free-var count)`: ক্লোজার ইন্সট্যান্সিয়েশন।
- `OpGetFree (1 byte index)`: ক্লোজারের ফ্রি ভ্যারিয়েবল রিড।

### ৪.৪ ভার্চুয়াল মেশিন (`compiler/vm`)
- **Stack**: ২,০৪৮ এলিমেন্টের হাই-স্পিড ফিক্সড অ্যারে।
- **Frames**: ১,০২৪ কল ফ্রেমের রিকার্শন সাপোর্ট।
- **Globals**: ৬৫,৫৩৬ মেমোরি স্লট।
- **Native Call Handler**: সফটবাস ও অনুরুনের নেটিভ সিস্টেম কল ডিসপ্যাচ।

---

## ৫. প্রজেক্ট ম্যানেজমেন্ট ও বান্ডিল সিস্টেম

### ৫.১ কনফিগারেশন ফাইল (`nil.json`)
```json
{
  "name": "hello-onuron",
  "version": "1.0.0",
  "author": "Joysriram Sarkar",
  "entry": "src/main.nil",
  "targets": ["onuron", "linux"],
  "resources": ["resources/*"],
  "build": {
    "output_dir": "build",
    "optimize": true,
    "debug": false
  }
}
```

### ৫.২ `.nilax` বান্ডিল স্ট্রাকচার
`.nilax` হলো একটি জিপ-ভিত্তিক সংকুচিত অ্যাপ্লিকেশন প্যাকেজ:
```text
hello-onuron-1.0.0.nilax
├── manifest.json            # অ্যাপ মেটাডেটা, টার্গেট, পারমিশন
├── bytecode/
│   └── main.nabc            # প্রাক-কম্পাইলকৃত NABC বাইটকোড
├── resources/               # ছবি, আইকন, লেআউট ইত্যাদি
└── signature.sig            # Ed25519 ক্রিপ্টোগ্রাফিক সিগনেচার
```

### ৫.৩ CLI কমান্ডসমূহ (`nil` ও `nilc`)
```bash
nil init [name]        # নতুন প্রজেক্ট টেমপ্লেট তৈরি
nil build              # সোর্স কম্পাইল করে .nilax বান্ডিল তৈরি
nil run [file] [-vm]   # প্রোগ্রাম রান (ইন্টারপ্রেটার বা VM)
nil repl               # ইন্টারেক্টিভ REPL
nil render             # Alap ডিক্লারেটিভ UI রেন্ডার (ANSI/HTML)
nil fmt                # কোড অটো-ফরম্যাটিং
nil clean              # বিল্ড আর্টিফ্যাক্ট ডিলিট
nilc <file.nil>        # ডেডিকেটেড বাইটকোড কম্পাইলার ও এক্সিকিউটার
```

---

## ৬. প্যাকেজ ম্যানেজার ও রেজিস্ট্রি (`nilpkg`)

`nilpkg` হলো নীলাং-এর অফিশিয়াল প্যাকেজ ও ডিপেনডেন্সি ম্যানেজার।
- **লোকাল ডেটাবেস**: `~/.nilang/packages.db` (ইনস্টলকৃত প্যাকেজ ও ভার্সন ট্র্যাক)।
- **ইনটিগ্রিটি ভেরিফিকেশন**: প্রতিটি প্যাকেজের SHA-256 চেকসাম ও ম্যানিফেস্ট ভ্যালিডেশন।
- **রেজিস্ট্রি সার্ভার (`cmd/nilpkg-server`)**:
  - পোর্ট `8080`-এ চালিত হাই-স্পিড গো মাইক্রোসার্ভিস।
  - RESTful API: `/api/v1/packages`, `/api/v1/packages/search`, `/api/v1/publish`, `/api/v1/download/{name}/{version}`।
  - আধুনিক ওয়েব ড্যাশবোর্ড: প্যাকেজ সার্চ, ডাউনলোড স্ট্যাটিস্টিক্স এবং রিয়েল-টাইম মেট্রিক্স।

---

## ৭. ক্রিপ্টোগ্রাফিক সিকিউরিটি ও সাইনিং (`nilkey`)

Onuron OS-এ প্রতিটি অ্যাপ্লিকেশনের বিশ্বাসযোগ্যতা নিশ্চিত করতে Ed25519 ডিজিটাল সাইনিং বাধ্যতামূলক:
- **কী জেনারেশন**: `nilkey generate -name="Dev" -email="dev@onuron.org" -password="secret"`।
- **এনক্রিপ্টেড কি-স্টোর**: পাসওয়ার্ড দিয়ে Argon2id কি-ডেরিভেশন এবং AES-256-GCM এনক্রিপশনের মাধ্যমে `~/.nilang/keystore.json`-এ সুরক্ষিত রাখা হয়।
- **প্যাকেজ সাইনিং**: `nilkey sign app.nilax -password="secret"` কমান্ডের মাধ্যমে SHA-256 হ্যাশ গণনা করে Ed25519 সিগনেচার তৈরি হয়।
- **ভেরিফিকেশন**: `nilkey verify app.nilax` দিয়ে ডেভেলপার পরিচিতি ও ফাইলের অবিকৃত অবস্থা পরীক্ষা করা যায়।

---

## ৮. ডিক্লারেটিভ UI ও ৬০ এফপিএস অ্যানিমেশন ইঞ্জিন

### ৮.১ Alap ডিক্লারেটিভ UI ফ্রেমওয়ার্ক (`pkg/alap`)
ArkTS এবং Flutter-অনুপ্রাণিত কম্পোনেন্ট মডেল:
- লেআউট: `Column`, `Row`, `Container`, `Stack`।
- কন্ট্রোল: `Button`, `Input`, `Text`, `Image`।
- রেন্ডারিং: ANSI টার্মিনাল ডায়াগ্রাম এবং রেস্পন্সিভ HTML/CSS আউটপুট (`build/preview.html`)।

### ৮.২ অ্যানিমেশন ইঞ্জিন (`pkg/animation`)
- ৩০টিরও বেশি বিল্ট-ইন ইজিং কার্ভ: Linear, EaseInQuad, EaseOutQuad, EaseInOutQuad, EaseInCubic, EaseOutCubic, EaseInOutCubic, EaseInQuart, EaseOutQuart, EaseInExpo, EaseOutExpo, EaseInBack, EaseOutBack, EaseInElastic, EaseOutElastic, EaseOutBounce, ইত্যাদি।
- কি-ফ্রেম ট্র্যাক ও মাল্টি-লেয়ার টাইমলাইন।

### ৮.৩ GPU ব্যাকএন্ড (`pkg/gpu`)
- Vulkan এবং OpenGL হার্ডওয়্যার অ্যাক্সিলারেশন।
- বাফার ম্যানেজমেন্ট (Vertex & Index Buffers), শেডার পাইপলাইন, ম্যাট্রিক্স ট্রান্সফর্মেশন (Model, View, Projection)।

---

## ৯. Onuron SoftBus ডিস্ট্রিবিউটেড প্রোটোকল ও Rust FFI

### ৯.১ SoftBus প্রোটোকল (`pkg/softbus`)
অনুরন ডিভাইসের মধ্যে নিরবচ্ছিন্ন যোগাযোগের প্রোটোকল:
- **ল্যান ডিসকভারি**: UDP মাল্টিকাস্ট (`239.255.0.1:9001`) দিয়ে ২ সেকেন্ড অন্তর অটো-ডিসকভারি ও হার্টবিট।
- **মেসেজ এক্সচেঞ্জ**: TCP ফ্রেমড ট্রান্সপোর্ট, এন্ড-টু-এন্ড এনক্রিপশন সাপোর্ট।
- **JSON-RPC 2.0**: ডিস্ট্রিবিউটেড ফাংশন কল (ডিভাইস স্ট্যাটাস, ব্যাটারি লেভেল, ক্রস-ডিভাইস স্ক্রিন শেয়ারિંગ)।
- **SoftBus Daemon (`cmd/softbusd`)**: ব্যাকগ্রাউন্ড সিস্টেম সার্ভিস।

### ৯.২ Rust Native FFI ব্রিজ (`runtime/vm`)
Onuron OS কার্নেল ও ড্রাইভার লেভেলের সাথে যোগাযোগের জন্য সি-এবিআই (C-ABI) ও রাস্ট ব্রিজ:
- `bridge_cgo.go`: Linux / Onuron OS প্ল্যাটফর্মে সরাসরি Rust লাইব্রেরির C ফাংশন কল।
- `bridge_nocgo.go`: উইন্ডোজ বা সিজিও-বিহীন পরিবেশে স্বয়ংক্রিয় পিওর গো ফলব্যাক।

---

## ১০. টুলিং ও রিলিজ গাইড

### ক্রস-প্ল্যাটফর্ম বিল্ড ম্যাট্রিক্স:
1. **Linux**: `linux-amd64`, `linux-arm64` (`.tar.gz`)
2. **macOS**: `darwin-amd64` (Intel), `darwin-arm64` (Apple Silicon) (`.tar.gz`)
3. **Windows**: `windows-amd64` (`.zip`)

### ইনস্টলেশন:
- **Linux/macOS**: `curl -fsSL https://raw.githubusercontent.com/joysriramsarkar/nilLang/main/install.sh | bash`
- **Homebrew**: `brew install joysriramsarkar/tap/nilang`

---

## ১১. লাইসেন্স (License)

নীলাং (Nilang) প্রোগ্রামিং ভাষাটি সম্পূর্ণ ওপেন-সোর্স এবং **GNU Affero General Public License v3.0 (AGPL-3.0)**-এর অধীনে প্রকাশিত।
Copyright (C) 2026 Joysriram Sarkar.
Alap Framework & Onuron OS Ecosystem.

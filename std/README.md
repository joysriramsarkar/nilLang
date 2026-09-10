# Nilang Standard Library (`std`)

The official standard library for Nilang (`nilLang`), organized according to `দরকার.md` and `মান্য-লাইব্রেরি.md`.

## Modules Overview

### 1. Core & Base Types
- **`std/core`**: Identity, default values, clamp, repeat, swap, and Result (`ok`, `err`), Optional (`some`, `none`), and Error (`error(code, message)`) helpers.

### 2. Financial Core & Math
- **`std/math`**: Mathematical functions (`abs`, `min`, `max`, `clamp`, `sign`, `isEven`, `isOdd`, `gcd`, `lcm`, `factorial`, `powInt`, `fibonacci`, `average`), constants `PI` and `E`, and re-exports for Decimal and Money.
- **`std/math/decimal`**: Exact fixed-point arithmetic (`new`, `alignScales`, `add`, `sub`, `mul`, `div`, `toString`, `fromInt`, `fromMinor`). Eliminates `float64` floating point inaccuracies in monetary transactions.
- **`std/money`**: Currency-aware financial structures (`new`, `ofMinor`, `ofMajor`, `add`, `sub`, `mulQty`, `format`).

### 3. Collections & Strings
- **`std/collections`**: Functional array operations (`map`, `filter`, `reduce`, `find`, `any`, `all`, `count`, `sum`, `product`, `take`, `drop`, `range`, `enumerate`, `contains`, `indexOf`, `reverse`, etc.).
- **`std/strings`**: String utilities (`length`, `startsWith`, `endsWith`, `includes`, `capitalize`, `lowercase`, `uppercase`, `normalized`, `reverse`, `repeat`, `replaceAll`, `lines`, `words`, `joinLines`).

### 4. Database & ORM Layer
- **`std/db/query_builder`**: Fluent SQL query construction (`select`, `where`, `limit`, `order`, `buildSql`, `execute`).
- **`std/db`**: Database connection management (`connect`), SQL queries (`query`), command execution (`exec`), and transactions (`transaction`).

### 5. Networking & HTTP
- **`std/net/http/router`**: Web router with route dispatching (`new`, `get`, `post`, `handle`) and response constructors (`response`, `responseJson`, `responseText`).
- **`std/net/http`** & **`std/http`**: HTTP client (`get`, `post`) and server router helpers.
- **`std/net/url`**: URL path, host, and scheme parsing.
- **`std/net/socket`**: Low-level TCP socket client contract.

### 6. Data & Serialization
- **`std/data/json`** & **`std/json`**: High-performance JSON `encode` and `decode` bridged to native Go serializer.
- **`std/data/base64`** & **`std/base64`**: Base64 encoding and decoding.
- **`std/data/csv`**: Line and document CSV parsing and row encoding.

### 7. Cryptography & Security
- **`std/crypto/hash`**: SHA-256 and SHA-512 cryptographic hashing.
- **`std/crypto/hmac`**: HMAC-SHA256 signature verification.
- **`std/crypto/random`**: CSPRNG byte generation using `crypto/rand`.
- **`std/crypto`**: Combined cryptography convenience module.

### 8. Time & Concurrency
- **`std/time`**: Timestamps (`now`), thread sleep (`sleep`), and dates (`today`).
- **`std/time/instant`**, **`std/time/duration`**, **`std/time/date`**: High-precision time utilities.
- **`std/concurrency/task`**: Task spawning and awaiting.
- **`std/concurrency/channel`**: Channel creation, sending, and receiving.

### 9. I/O & Filesystem
- **`std/io`**: Console and file writing helpers.
- **`std/fs`**: File reading, writing, presence checks, and directory traversal.
- **`std/process`**: Command execution and exit code handling.
- **`std/testing`**: Assertion helpers (`assertTrue`, `assertFalse`, `assertEqual`, `assertNotEqual`, `assertNull`, `assertNotNull`, `assertContains`).

## Usage Examples

```nil
import "std/math/decimal" as decimal;
import "std/money" as money;
import "std/data/json" as json;
import "std/crypto" as crypto;

// Financial calculations
let price = money.ofMajor(150, "BDT");
let tax = money.mulQty(price, 150, 1000); // 15% tax
let total = money.add(price, tax);

// JSON & Crypto
let payload = {"order_id": 1001, "total": money.format(total.unwrap(), "৳")};
let encoded = json.encode(payload);
let signature = crypto.sha256(encoded);
```

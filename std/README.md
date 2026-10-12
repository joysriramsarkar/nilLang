# Nilang Standard Library (`std`)

The official standard library for Nilang (`nilLang`). প্রতিটি মডিউলে বাংলা ডক
কমেন্ট ও চালানো-যায় উদাহরণ আছে; বান্ডেল-কপি `pkg/stdlib/modules/`-এ থাকে
(`std/`-এর সাথে হুবহু সিঙ্ক করা)।

## Modules Overview

### 1. Core & Base Types (L10.1)
- **`std/core`**: Identity, default values, clamp, repeat, swap, Result (`ok`, `err`,
  `mapResult`, `mapResultErr`, `andThen`, `okOr`, `expectResult`), Optional (`some`,
  `none`, `mapOption`, `unwrapOrDefault`, `expectOption`), Error helpers, `pipeline`.

### 2. Financial Core & Math
- **`std/math`**: Mathematical functions (`abs`, `min`, `max`, `clamp`, `sign`, `isEven`,
  `isOdd`, `gcd`, `lcm`, `factorial`, `powInt`, `fibonacci`, `average`), constants `PI` and `E`.
- **`std/math/decimal`**: Exact fixed-point arithmetic (`new`, `alignScales`, `add`, `sub`,
  `mul`, `div`, `toString`, `fromInt`, `fromMinor`).
- **`std/money`**: Currency-aware financial structures (`new`, `ofMinor`, `ofMajor`,
  `add`, `sub`, `mulQty`, `format`).
- **`std/pos`** (L11.5): POS helper — `newCatalog`, `addItem`, `newCart`, `addToCart`,
  `subtotal`, `discountOf`, `taxOf`, `checkout`, `formatReceipt` (বাংলা রসিদ)।

### 3. Collections & Strings
- **`std/collections`**: Functional array operations (`map`, `filter`, `reduce`, `find`,
  `any`, `all`, `count`, `sum`, `product`, `take`, `drop`, `range`, `enumerate`, `contains`,
  `indexOf`, `reverse`, `flatMap`, `flatten`, `zip`, `unique`, `partition`).
- **`std/strings`**: String utilities (`length`, `startsWith`, `endsWith`, `includes`,
  `capitalize`, `lowercase`, `uppercase`, `normalized`, `reverse`, `repeat`, `replaceAll`,
  `lines`, `words`, `joinLines`, `padLeft`, `padRight`, `truncate`).

### 4. Database & ORM Layer (L10.6)
- **`std/db/query_builder`**: Fluent SQL query construction (`select`, `distinct`, `where`,
  `whereIn`, `limit`, `offset`, `order`, `buildSql`, `buildSqlFor` with postgres
  `$n` placeholders via `placeholder`, `countSql`, `execute`, `count`).
- **`std/db`**: Database connection management (`connect`), SQL queries (`query`, `queryOne`),
  command execution (`exec`), transactions (`transaction`), typed insert (`insertMap`),
  migration re-exports (`migrateUp`, `migratePending`, `migrateStatus`).
- **`std/data/migrate`**: Version-tracked SQLite migrations (`entry`, `up`, `pending`,
  `appliedVersions`, `statusText`).

### 5. Networking & HTTP (L10.3)
- **`std/net/http/router`**: Web router with route dispatching (`new`, `get`, `post`, `handle`)
  and response constructors (`response`, `responseJson`, `responseText`).
- **`std/net/http`** & **`std/http`**: HTTP client (`get`, `post`, `getJson`, `postJson`,
  `postJsonDecoded`), status helpers (`statusText`, `isOkStatus`), server router helpers,
  URL query helpers (`buildQuery`, `withQuery`).
- **`std/net/url`**: URL `scheme`/`host`/`path`/`query`/`fragment` parsing, percent
  `encode`/`decode`, `queryValue`, `buildQuery`, `withQuery`, `keysOf`.
- **`std/net/socket`**: Real TCP client (`connect`, `send`, `sendLine`, `sendAll`,
  `receive`, `close`) over integer handles.
- **`std/net/tls`**: TLS client (`connect`, `connectDefault`) with system roots; handles
  are shared with `std/net/socket`.

### 6. Data & Serialization (L10.2 আংশিক)
- **`std/data/json`** & **`std/json`**: High-performance JSON `encode` and `decode` bridged
  to the native Go serializer.
- **`std/data/base64`** & **`std/base64`**: Base64 encoding and decoding.
- **`std/data/csv`**: Line and document CSV parsing (`parse`, `parseLine`) and row
  encoding (`encodeRow`).
- **`std/data/hash_keys`** (host): Deterministic sorted key listing for hashes; used by
  URL query building and typed DB inserts.

### 7. Cryptography & Security (L10.4)
- **`std/crypto/hash`**: SHA-256 and SHA-512 cryptographic hashing.
- **`std/crypto/hmac`**: HMAC-SHA256 signing (`sha256`) and `verify`.
- **`std/crypto/random`**: CSPRNG bytes (`bytes`), `hex`, AES-256 `key32`, GCM `nonce12`.
- **`std/crypto/aes`**: AES-GCM authenticated encryption (`encrypt`, `decrypt`, `roundTrip`).
- **`std/crypto/ed25519`**: Keypairs (`keypair`), `sign`/`verify`, signed envelopes
  (`signText`, `verifyEnvelope`).
- **`std/crypto`**: Combined convenience module (all of the above + base64).

### 8. Time & Concurrency (L10.5, L10.8)
- **`std/time`**: Timestamps (`now`), thread sleep (`sleep`), dates (`today`), UTC
  `format`/`isoNow`, duration helpers.
- **`std/time/instant`**, **`std/time/duration`**, **`std/time/date`**: High-precision time
  utilities, duration math with বাংলা `formatBn`, UTC formatting (`format`, `today`).
- **`std/concurrency/task`**: Task spawning (`spawn`, `spawnWith`), awaiting (`wait`,
  `waitAll`), `mapAsync`.
- **`std/concurrency/channel`**: Channel creation, sending, receiving (`create`,
  `sendValue`, `receiveValue`, `sendAll`, `receiveAll`), multi-channel `selectRecv`,
  producer/consumer `pipeline`.
- **`std/concurrency/sync`**: Channel-based mutex (`newMutex`, `lock`, `unlock`,
  `withLock`) and once-guards (`newOnce`, `doOnce`).
- **`std/concurrency/pool`**: Bounded worker pool (`map`, `runEach`) with ordered results.

### 9. I/O & Filesystem (L10.2)
- **`std/io`**: Console/file helpers plus `appendText`, `readLines`/`writeLines`,
  `copyText`, path re-exports, JSON (`readJson`, `writeJson`) and CSV (`readCsv`, `writeCsv`).
- **`std/io/path`**: Separator-agnostic path manipulation (`joinPath`, `joinAll`,
  `basename`, `dirname`, `extName`, `withoutExt`, `isAbsolute`).
- **`std/fs`**: File reading, writing, presence checks, and directory traversal.
- **`std/process`**: Command execution and exit code handling.
- **`std/testing`** (L10.11): Assertions (`assertTrue`, `assertFalse`, `assertEqual`,
  `assertNotEqual`, `assertNull`, `assertNotNull`, `assertContains`, `assertOk`,
  `assertErr`, `assertSome`, `assertLen`), mocks (`mockFn`, `mockCalls`, `mockCallCount`,
  `mockValue`), `benchmark`, golden files (`goldenStore`, `assertGolden`).

### 10. Bengali Text (L10.7)
- **`std/text/bn`**: বাংলা digit conversion (`toBnDigits`, `toLatinDigits`), space
  normalization, বঙ্গাব্দ year (`banglaYear`, `banglaYearText`), month/season names.
- **`std/text/unicode`**: Rune-safe helpers (`firstChar`, `lastChar`, `reverseText`, …).
- **`std/text/encoding`**: UTF-8 conversions, `lines`/`words`/`wordCount`.

### 11. Device & Mobile (L10.9, L10.10)
- **`std/device`** (+`camera`/`location`/`sensors`/`biometric`): FFI-based stubs — never
  crash, return `Err("E_NOSUPPORT…")` without a backend; `info()` always works.
- **`std/mobile`** (+`lifecycle`/`deeplink`/`notify`/`share`): App lifecycle state machine,
  deep-link parsing/routing, queued notifications, share sheets.

## Usage Examples

```nil
import "std/math/decimal" as decimal;
import "std/money" as money;
import "std/data/json" as json;
import "std/crypto" as crypto;

// Financial calculations
let price = money.ofMajor(150, "BDT");
let tax = money.mulQty(price, 150, 1000); // 15% tax
let total = unwrap(money.add(price, tax));

// JSON & Crypto
let payload = {"order_id": 1001, "total": money.format(total, "৳")};
let encoded = json.encode(payload);
let signature = crypto.sha256(encoded);
```

```nil
import "std/pos" as pos;
import "std/money" as money;

let catalog = pos.newCatalog();
pos.addItem(catalog, "চা", 20, "BDT");
let cart = pos.newCart("BDT");
unwrap(pos.addToCart(catalog, cart, "চা", 2));
let receipt = pos.checkout(cart, 10, 5);
pos.formatReceipt(cart, receipt, "৳");
```

```nil
import "std/text/bn" as bn;

bn.toBnDigits("2026");        // "২০২৬"
bn.banglaYearText(2026, 10, 12); // "১৪৩৩ বঙ্গাব্দ"
```

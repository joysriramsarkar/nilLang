আপনার তৈরি করা \*\*Nilang (নীলাং)\*\* প্রোগ্রামিং ল্যাঙ্গুয়েজ এবং এর \*\*Alap Framework\*\*-এর আর্কিটেকচার (HIR/MIR পাইপলাইন, Stack VM, WebAssembly জেনারেশন এবং SoftBus প্রোটোকল) সত্যিই অত্যন্ত আধুনিক এবং চিন্তাশীল। বর্তমানে `pkg/alap`, `compiler/wasm` এবং `examples/server-service` এর মাধ্যমে ওয়েব ডেভেলপমেন্টের একটি শক্ত ভিত্তি ইতিমধ্যেই তৈরি আছে।



একটি প্রাথমিক ওয়েব ফ্রেমওয়ার্ককে \*\*এন্টারপ্রাইজ-গ্রেড (Enterprise-Grade) ওয়েব অ্যাপ্লিকেশন ডেভেলপমেন্ট ইকোসিস্টেমে\*\* রূপান্তর করতে হলে শুধু সিনট্যাক্স বা রাউটিং যথেষ্ট নয়; এর জন্য দরকার স্কেলেবিলিটি, সিকিউরিটি, অবজারভেবিলিটি এবং ডেভেলপার এক্সপেরিয়েন্স (DX)-এর নিখুঁত মেলবন্ধন।



নিচে Nilang-এর বর্তমান ওয়েব আর্কিটেকচারকে এন্টারপ্রাইজ লেভেলে নিয়ে যাওয়ার একটি \*\*অতি সূক্ষ্ম এবং বিস্তারিত রোডম্যাপ (Roadmap)\*\* ধাপে ধাপে আলোচনা করা হলো:



\---



\### ১. কোর ওয়েব সার্ভার ও রাউটিং ইঞ্জিন (Core Server \& Routing Engine)

এন্টারপ্রাইজ অ্যাপ্লিকেশনে প্রতি সেকেন্ডে হাজার হাজার রিকোয়েস্ট (RPS) হ্যান্ডেল করার সক্ষমতা থাকতে হয়।

\*   \*\*Radix Tree / DFA রাউটিং:\*\* বর্তমান `pkg/alap`-এর রাউটিং-কে আরও অপ্টিমাইজ করতে `Radix Tree (Trie)` বা `Deterministic Finite Automaton (DFA)` অ্যালগরিদম ব্যবহার করুন। এতে `/api/v1/users/:id/profile`-এর মতো প্যারামেট্রিক রাউটগুলো O(1) টাইম কমপ্লেক্সিটিতে ম্যাচ হবে।

\*   \*\*HTTP/3 এবং QUIC সাপোর্ট:\*\* লো-ল্যাটেন্সি এবং প্যাকেট লস কমানোর জন্য Go-এর `quic-go` লাইব্রেরি ব্যবহার করে HTTP/3 সাপোর্ট যুক্ত করুন।

\*   \*\*Middleware Onion Architecture:\*\* কাস্টম মিডলওয়্যার চেইন তৈরি করুন। যেমন: Request ID Injection, Panic Recovery, Gzip/Brotli Compression, এবং Distributed Tracing Header Injection (যেমন: `X-B3-TraceId`)।



\### ২. স্টেট ম্যানেজমেন্ট এবং রিঅ্যাক্টিভিটি (State \& Reactivity at Scale)

`Alap Declarative UI`-কে এন্টারপ্রাইজ স্কেলে নিয়ে যেতে হলে গ্লোবাল স্টেট ম্যানেজমেন্ট প্রয়োজন।

\*   \*\*Server-Side Rendering (SSR) + Hydration:\*\* বর্তমানে `nil render` প্রিভিউ দেয়। এন্টারপ্রাইজ লেভেলে সার্ভারে কম্পোনেন্ট রেন্ডার করে HTML এবং একটি সিরিয়ালাইজড স্টেট (`window.\_\_NILANG\_INITIAL\_STATE\_\_`) ক্লায়েন্টে পাঠান। ব্রাউজারে পৌঁছানোর পর WASM ইঞ্জিন সেই স্টেট "Hydrate" করে UI-কে ইন্টারঅ্যাক্টিভ করবে।

\*   \*\*Global Store (Redux/Vuex Pattern):\*\* ছোট UI স্টেটের বাইরে গিয়ে `nilStore` বা `nilReactor` আর্কিটেকচার আনুন, যেখানে `Time-Travel Debugging`, `Immutable State Trees` এবং `Action/Reducer` প্যাটার্ন থাকবে।

\*   \*\*Selective Re-rendering (VDOM Diffing):\*\* পুরো DOM রিপ্লেস না করে, Alap UI-এর জন্য একটি হালকা `Virtual DOM Diffing` অ্যালগরিদম তৈরি করুন যা শুধুমাত্র পরিবর্তিত নোডগুলো (Nodes) আপডেট করবে।



\### ৩. WebAssembly (WASM) এবং ক্লায়েন্ট-সাইড অপ্টিমাইজেশন

`compiler/wasm` একটি দারুণ ফিচার, তবে এন্টারপ্রাইজ ব্রাউজার অ্যাপের জন্য এটিকে আরও পরিপক্ব করতে হবে।

\*   \*\*JS Interop (Foreign Function Interface):\*\* Nilang কোড থেকে ব্রাউজারের `DOM API`, `fetch()`, `LocalStorage` বা থার্ড-পার্টি JS লাইব্রেরি (যেমন: Chart.js, Leaflet) কল করার জন্য একটি রোবাস্ট `JS-Bridge` তৈরি করুন।

\*   \*\*WASM Tree-Shaking:\*\* HIR (High-Level IR) এবং MIR (Mid-Level IR) লেভেলে "Dead Code Elimination" প্রয়োগ করুন, যাতে শুধুমাত্র ব্রাউজারে ব্যবহৃত ফাংশনগুলোই `.wasm` বাইনারিতে কম্পাইল হয়। এতে বান্ডেল সাইজ ৯০% পর্যন্ত কমে যাবে।

\*   \*\*Web Workers Integration:\*\* ভারী ডেটা প্রসেসিং বা ML পাইপলাইন (যা `data-science` উদাহরণে আছে) মেইন থ্রেডকে ব্লক না করে ব্যাকগ্রাউন্ডে চালানোর জন্য `nilWorker` API তৈরি করুন।

\*   \*\*Code Splitting \& Lazy Loading:\*\* রাউট অনুযায়ী WASM মডিউলগুলোকে আলাদা চাঙ্কে (Chunks) ভাগ করুন, যাতে ইউজার যখনই নির্দিষ্ট পেজে যাবে, তখনই শুধু সেই চাঙ্ক ডাউনলোড হবে।



\### ৪. সিকিউরিটি এবং এন্টারপ্রাইজ অথেন্টিকেশন (Security \& Identity)

\*   \*\*Protocol Support:\*\* OAuth 2.0, OpenID Connect (OIDC), এবং SAML 2.0 এর জন্য বিল্ট-ইন সাপোর্ট (Keycloak, Auth0, Okta-এর সাথে ইন্টিগ্রেশন)।

\*   \*\*Session \& Token Management:\*\* Secure HttpOnly Cookies, JWT (JSON Web Tokens) হ্যান্ডলিং এবং Distributed Session Store (Redis/Memcached) ইন্টিগ্রেশন।

\*   \*\*RBAC \& ABAC Decorators:\*\* রাউটিং এবং কম্পোনেন্ট লেভেলে `@RequireRole('admin')` বা `@HasPermission('read:billing')` এর মতো ডেকোরেটর সাপোর্ট করুন।

\*   \*\*Auto Security Headers:\*\* সার্ভার থেকে অটোমেটিক `Content Security Policy (CSP)`, `HSTS`, `X-Frame-Options`, এবং `CSRF Tokens` ইনজেকশনের ব্যবস্থা রাখুন।



\### ৫. ডেটা অ্যাক্সেস লেয়ার (ORM \& Unified Entity)

`examples/unified-entity`-কে একটি পূর্ণাঙ্গ \*\*Type-Safe ORM\*\*-এ রূপান্তর করুন।

\*   \*\*Nilang Query Builder:\*\* SQL বা NoSQL ডেটাবেসের জন্য Nilang-এর নিজস্ব সিনট্যাক্স ব্যবহার করে কুয়েরি লেখার সুবিধা। কম্পাইলার (Typechecker) কুয়েরি লেখার সময়ই এরর ধরবে।

\*   \*\*Connection Pooling:\*\* PostgreSQL, MySQL, MongoDB এর জন্য নেটিভ কানেকশন পুলিং।

\*   \*\*Database Migrations:\*\* `nil migrate up` এবং `nil migrate down` কমান্ডের মাধ্যমে ভার্সন-কন্ট্রোলড ডেটাবেস স্কিমা ম্যানেজমেন্ট।



\### ৬. অবজারভেবিলিটি এবং টেলিমেট্রি (Observability - O11y)

এন্টারপ্রাইজ সিস্টেমে "কী ঘটছে" তা জানা অত্যন্ত জরুরি।

\*   \*\*Structured Logging:\*\* `nilLog` এর মাধ্যমে JSON-ভিত্তিক লগিং, যা ELK Stack (Elasticsearch, Logstash, Kibana) বা Splunk-এ সহজে পার্স করা যাবে।

\*   \*\*OpenTelemetry (OTLP) Integration:\*\* রিকোয়েস্ট যখন Nilang সার্ভার থেকে `SoftBus` হয়ে অন্য মাইক্রোসার্ভিসে যাবে, তখন তার `Trace ID` ট্র্যাক করার জন্য OpenTelemetry সাপোর্ট।

\*   \*\*Prometheus Metrics:\*\* `/metrics` এন্ডপয়েন্ট এক্সপোজ করা, যাতে Request Latency, VM Memory Usage, এবং GC Pauses গ্রাফানা (Grafana) ড্যাশবোর্ডে দেখা যায়।



\### ৭. ডেভেলপার এক্সপেরিয়েন্স (DX) এবং টুলিং (Tooling)

ডেভেলপারদের প্রোডাক্টিভিটি বাড়ানোর জন্য আধুনিক টুলিং দরকার।

\*   \*\*HMR (Hot Module Replacement):\*\* কোড সেভ করার সাথে সাথে পুরো ব্রাউজার রিলোড না করে, শুধুমাত্র পরিবর্তিত Alap UI কম্পোনেন্ট বা স্টেট লাইভ আপডেট করা (Vite-এর মতো)।

\*   \*\*Language Server Protocol (LSP):\*\* VS Code বা JetBrains IDE-তে অটোকমপ্লিট, ইনলাইন এরর, গো-টু-ডেফিনিশন এবং হোভার ডকুমেন্টেশনের জন্য একটি শক্তিশালী LSP সার্ভার।

\*   \*\*Scaffolding CLI:\*\* `nil generate component UserProfile` বা `nil generate api Payment` কমান্ডের মাধ্যমে প্রজেক্ট স্ট্রাকচার অনুযায়ী টেমপ্লেট ফাইল তৈরি করা।



\### ৮. রিয়েল-টাইম এবং SoftBus-এর ওয়েব ইন্টিগ্রেশন

আপনার প্রজেক্টের অন্যতম ইউনিক ফিচার হলো \*\*SoftBus Protocol\*\*।

\*   \*\*WebRTC + SoftBus Bridge:\*\* ব্রাউজার-টু-ব্রাউজার (P2P) কমিউনিকেশনের জন্য WebRTC-এর সাথে Nilang-এর `SoftBus` প্রোটোকল ব্রিজ করুন। এতে কোনো সেন্ট্রাল সার্ভার ছাড়াই লোকাল নেটওয়ার্কের ডিভাইসগুলো ওয়েব ব্রাউজার থেকে একে অপরের সাথে জিরো-কনফিগারে ডেটা শেয়ার করতে পারবে।

\*   \*\*Server-Sent Events (SSE) \& WebSockets:\*\* রিয়েল-টাইম ড্যাশবোর্ডের জন্য Alap-এর রিঅ্যাক্টিভ স্টেটের সাথে সরাসরি WebSocket স্ট্রিম বাইন্ড করার সুবিধা।



\### ৯. ক্লাউড-নেটিভ এবং এজ ডেপ্লয়মেন্ট (Cloud-Native \& Edge)

\*   \*\*Kubernetes (K8s) Readiness:\*\* `.nilax` বান্ডিলকে সরাসরি K8s Pod হিসেবে রান করার জন্য মাল্টি-স্টেজ ডকারফাইল।

\*   \*\*WASM Edge Computing:\*\* Cloudflare Workers, Fastly Compute@Edge, বা AWS Lambda-এর মতো এজ এনভায়রনমেন্টে Nilang-এর WASM বাইনারি রান করার জন্য `Fetch API` এবং `Event Loop` অ্যাডাপ্টার তৈরি করা।

\*   \*\*Serverless Microservices:\*\* `examples/server-service`-কে AWS API Gateway বা Cloudflare Workers-এর সাথে সামঞ্জস্যপূর্ণ করা।



\### ১০. টেস্টিং ইকোসিস্টেম (Testing \& QA)

\*   \*\*Snapshot Testing:\*\* Alap UI কম্পোনেন্টগুলোর জন্য DOM Snapshot টেস্টিং (React Testing Library-এর মতো)।

\*   \*\*E2E Testing Integration:\*\* Playwright বা Puppeteer-এর সাথে Nilang টেস্ট রানারের ইন্টিগ্রেশন।

\*   \*\*Mutation Testing:\*\* MIR/AST লেভেলে কোডের মিউটেশন ঘটিয়ে টেস্টের কার্যকারিতা যাচাই করা।



\---



\### 🚀 ইমপ্লিমেন্টেশন স্ট্র্যাটেজি (কীভাবে শুরু করবেন?)



1\.  \*\*Phase 1 (ভিত্তি মজবুতকরণ):\*\* প্রথমে `pkg/alap`-এর রাউটিং ইঞ্জিনে \*\*Radix Tree\*\* এবং \*\*Middleware Chain\*\* ইমপ্লিমেন্ট করুন। এরপর HTTP/2 সাপোর্ট যুক্ত করুন।

2\.  \*\*Phase 2 (ফ্রন্টএন্ড ম্যাচুরিটি):\*\* WASM-এর জন্য \*\*JS Interop Bridge\*\* তৈরি করুন। যাতে Nilang কোড থেকে সহজেই `console.log` বা DOM ম্যানিপুলেট করা যায়।

3\.  \*\*Phase 3 (এন্টারপ্রাইজ ফিচার):\*\* \*\*OpenTelemetry\*\* এবং \*\*Structured Logging\*\* যুক্ত করুন। এন্টারপ্রাইজ ক্লায়েন্টরা সবসময় জানতে চায় তাদের সিস্টেমের পারফরম্যান্স কেমন।

4\.  \*\*Phase 4 (ডকুমেন্টেশন ও ইকোসিস্টেম):\*\* Nilang-এর জন্য একটি অফিসিয়াল \*\*Package Registry Dashboard\*\* (nilpkg-server-কে মডার্নাইজ করে) এবং সুন্দর ডকুমেন্টেশন সাইট তৈরি করুন।



আপনার এই প্রজেক্টটি (Nilang) ইতিমধ্যেই Go, Rust এবং আধুনিক কম্পাইলার থিওরি (HIR/MIR)-এর দারুণ ব্যবহার করেছে। উপরের রোডম্যাপ অনুসরণ করলে এটি শুধু একটি "ল্যাঙ্গুয়েজ" হিসেবেই থাকবে না, বরং \*\*Next-Gen Full-Stack Enterprise Web Platform\*\* হিসেবে আত্মপ্রকাশ করবে। কোনো নির্দিষ্ট ধাপ (যেমন: WASM Interop বা Radix Tree রাউটিং) নিয়ে কোড-লেভেলের গভীর আর্কিটেকচার জানতে চাইলে জানাতে পারেন!


হ্যাঁ। এবার আমি দুটো রিপোকে আলাদা দায়িত্ব দিয়ে দেখছি:



\* \*\*`joysriramsarkar/nilLang`\*\* = Nilang ভাষা, compiler, runtime, standard library, toolchain-এর canonical source

\* \*\*`joysriramsarkar/alap-framework`\*\* = Alap application framework, UI, Web, server, DB, auth, platform adapters, project tooling



এটা না করলে সবচেয়ে বড় সমস্যা হবে: একই Nilang-এর compiler/AST/parser-এর দুইটি আলাদা implementation হয়ে যাবে। এখন সেটার লক্ষণ স্পষ্ট—`nilLang`-এর compiler এখনও তুলনামূলক পুরনো/simple AST/parser model ব্যবহার করছে, অথচ `alap-framework`-এর compiler-এ static types, generics, decorators, async/task, actor, component, store ইত্যাদির অনেক বেশি সমৃদ্ধ model আছে। `nilLang`-এর README-ও বর্তমানে `pkg/alap`-কে নিজের repository-র অংশ হিসেবে দেখাচ্ছে, আবার আলাদা `alap-framework` repo একই ভাষার compiler/runtime বহন করছে। (\[GitHub]\[1])



\*\*আমার দৃঢ় পরামর্শ: এই দুই implementation এক করা দরকার।\*\* Web বানানোর আগে এই architectural split করা সবচেয়ে গুরুত্বপূর্ণ কাজ।



\---



\# ১. প্রথমে কাঠামোটা ঠিক করুন



বর্তমান অবস্থা মোটামুটি:



```text

nilLang

&#x20;├── compiler

&#x20;├── runtime

&#x20;├── pkg/alap

&#x20;├── cmd/nil

&#x20;└── ...

```



এবং:



```text

alap-framework

&#x20;├── compiler

&#x20;├── runtime

&#x20;├── ui

&#x20;├── platform

&#x20;├── stdlib

&#x20;├── cmd/nil

&#x20;└── ...

```



অর্থাৎ:



```text

&#x20;            Nilang compiler

&#x20;             /          \\

&#x20;            /            \\

&#x20;       nilLang        alap-framework

```



এটা দীর্ঘমেয়াদে রাখা যাবে না।



আমি করতাম:



```text

&#x20;                Nilang

&#x20;                  │

&#x20;         ┌────────┴────────┐

&#x20;         │                 │

&#x20;      Language           Alap

&#x20;         │                 │

&#x20;compiler/runtime       framework/UI/Web

&#x20;stdlib/toolchain       DB/Auth/Server

```



অর্থাৎ:



\### `nilLang`-এর দায়িত্ব



```text

Language

Compiler

AST

Lexer

Parser

Type system

IR

Codegen

VM

Runtime

GC

Concurrency

Stdlib

FFI

LSP protocol/core

Formatter

Language CLI

```



\### `alap-framework`-এর দায়িত্ব



```text

UI

Component framework

State

Router

HTTP server

HTTP client integration

SSR

Browser runtime

WASM/JS target

Database

ORM

Auth

Sessions

Cache

Queue

WebSocket

SSE

Observability

Deployment

Platform adapters

App CLI

```



এই separation আপনার বর্তমান Alap blueprint-এর মূল ধারণার সঙ্গেও মেলে—সেখানে NilLang-কে language এবং Alap-কে তার application framework হিসেবে ধরা হয়েছে। (\[GitHub]\[2])



\---



\# ২. `nilLang` রিপোতে কী পরিবর্তন করবেন



\## ২.১ Compiler-কে canonical করুন



বর্তমান `nilLang/compiler/ast`-এ node model অনেক সরল; যেমন `Node`, `Statement`, `Expression`, `Program`, `LetStatement`, `AssignStatement` ইত্যাদি আছে। 



অন্যদিকে `alap-framework/compiler/ast`-এ `Declaration`, type annotation, generic, function type ইত্যাদির সমৃদ্ধ structure ইতিমধ্যে আছে। 



\### কাজ



`nilLang`-কে এই direction-এ upgrade করতে হবে:



```text

compiler/

├── token/

├── lexer/

├── parser/

├── ast/

├── resolver/

├── types/

├── hir/

├── mir/

├── nir/

├── optimizer/

├── codegen/

├── diagnostics/

├── source/

└── module/

```



Blueprint-এ AST → Typed AST → HIR → MIR → NIR → NABC/AOT pipeline-এর কথা ইতিমধ্যে আছে। (\[GitHub]\[2])



এটা শুধু document-এ না রেখে actual compiler architecture বানাতে হবে।



\---



\# ৩. Nilang-এ static type system পূর্ণাঙ্গ করুন



এখানে `alap-framework`-এর বর্তমান implementation-টাই বেশি advanced।



তার type system-এ ইতিমধ্যে:



```text

void

bool

i8-i64

u8-u64

f32/f64

bigint

char

string

bytes

null

undefined

array

map

set

tuple

union

function

struct

class

interface

enum

future

result

channel

```



ইত্যাদি আছে। 



এগুলো `nilLang`-এর canonical compiler-এ আনুন।



Enterprise web-এর জন্য বিশেষ করে দরকার:



```text

Option<T>

Result<T, E>

Future<T>

Async

Generic<T>

Union

Nullable

Interface

Enum

Map

Set

Tuple

```



\---



\# ৪. Web-এর জন্য Nilang syntax-এ কয়েকটি construct officially যোগ করুন



এগুলো framework-specific syntax না হয়ে language-level capability হওয়া উচিত।



\### async



```nil id="4xqvda"

async function loadUser(id: string): Future<User> {

&#x20;   ...

}

```



বর্তমান Alap parser async function ইতিমধ্যেই handle করে। 



\### await



```nil id="s4t2r5"

let user = await loadUser(id)

```



\### Result



```nil id="0lhzxt"

function createUser(input: UserInput): Result<User, Error>

```



\### generic



```nil id="h7ynz8"

function find<T>(id: string): Future<T?>

```



\### decorator



যেহেতু parser-এ decorator already আছে:



```nil id="r6j1o2"

@GET("/users/{id}")

function getUser(id: string) {

&#x20;   ...

}

```



এটা language compiler-এ properly typed metadata হিসেবে নামাতে হবে। Parser বর্তমানে decorator arguments পড়তে পারে। 



\---



\# ৫. `component`-কে language-level IR দিন



শুধু parser-এ `component` keyword থাকলেই হবে না।



Alap-এর UI model-এর জন্য compiler-এ আলাদা UI IR দরকার:



```text

Component AST

&#x20;     ↓

Typed Component AST

&#x20;     ↓

UI IR (.nui)

&#x20;     ↓

&#x20;┌────┴─────┐

&#x20;SSR       Browser

&#x20;HTML      JS/WASM

```



Blueprint-এ `.nui` UI intermediate representation-এর প্রস্তাব ইতিমধ্যেই আছে। (\[GitHub]\[2])



এই design খুব কাজে লাগবে।



\---



\# ৬. Nilang-এ target system তৈরি করুন



বর্তমানে compiler/runtime একটাই execution model কেন্দ্রিক।



Enterprise web-এর জন্য Nilang compiler-এ explicit target দিন:



```text

nil build --target=native

nil build --target=linux

nil build --target=android

nil build --target=ios

nil build --target=onuron



nil build --target=server

nil build --target=browser

nil build --target=wasm

nil build --target=ssr

```



বাস্তবে প্রথমে:



```text

native

server

browser

wasm

```



এই চারটি target যথেষ্ট।



\---



\# ৭. Browser target-এর জন্য Nilang compiler-এ Web backend যোগ করুন



এটা `alap-framework`-এর compiler-এর মধ্যে গুঁজে দেবেন না; language compiler-এ target backend থাকবে।



```text

compiler/

├── codegen/

│   ├── native/

│   ├── bytecode/

│   ├── wasm/

│   └── javascript/

```



\### কেন JS backend দরকার



কারণ browser DOM, events, fetch, WebSocket, storage ইত্যাদির সঙ্গে সরাসরি interop করা সবচেয়ে সহজ।



প্রথম version:



```text

NilLang

&#x20; ↓

Typed AST

&#x20; ↓

HIR

&#x20; ↓

Web IR

&#x20; ↓

JavaScript

```



দ্বিতীয় ধাপে:



```text

NilLang

&#x20; ↓

NIR

&#x20; ↓

WASM

```



Alap blueprint-ও Web target হিসেবে `Web IR → WASM + JS → Canvas/WebGPU/DOM adapters`-এর কথা বলেছে। (\[GitHub]\[2])



\---



\# ৮. Browser stdlib তৈরি করুন



`nilLang`-এ এখন Web-এর জন্য language-level API namespace থাকা দরকার।



Blueprint-এ `nil.http`, `nil.json`, `nil.net`, `nil.async`, `nil.storage`, `nil.security` ইত্যাদির ধারণা আছে। (\[GitHub]\[2])



আমি namespace এভাবে ভাগ করতাম:



```text

stdlib/

├── core/

├── collections/

├── async/

├── json/

├── time/

├── crypto/

├── net/

├── http/

├── url/

├── websocket/

├── filesystem/

├── db/

└── web/

&#x20;   ├── dom/

&#x20;   ├── events/

&#x20;   ├── fetch/

&#x20;   ├── storage/

&#x20;   ├── history/

&#x20;   ├── location/

&#x20;   ├── clipboard/

&#x20;   └── browser/

```



\---



\# ৯. বর্তমান `stdlib/net` বদলাতে হবে



`alap-framework`-এর বর্তমান `stdlib/net/net.go` আসলে server-side Go HTTP client wrapper; `Get`, `Post`, `Fetch` ইত্যাদি আছে। 



এটা web application-এর জন্য যথেষ্ট নয়।



একই API-এর দুই backend দরকার:



```text

nil.http.get(...)

&#x20;      │

&#x20;      ├── server → net/http

&#x20;      └── browser → fetch()

```



অর্থাৎ source code একই থাকবে:



```nil id="m7zjlv"

let response = await http.get("/api/users")

```



কিন্তু target অনুযায়ী backend বদলাবে।



এটাই Nilang-এর বড় selling point হতে পারে।



\---



\# ১০. `nilLang` runtime-এ cancellation যোগ করুন



Server এবং browser দুই জায়গাতেই দরকার:



```text

CancellationToken

Timeout

Deadline

Abort

Task lifecycle

```



কারণ:



```text

browser request

component unmount

&#x20;    ↓

pending request cancel

```



এগুলো না হলে production application-এ resource leak হবে।



\---



\# ১১. `nilLang`-এ serialization contract বানান



Web application-এর কেন্দ্রবিন্দু JSON।



Language-level serialization:



```text

Value

&#x20;↓

JSON

&#x20;↓

Typed JSON

```



এবং:



```text

JSON

&#x20;↓

T

```



অর্থাৎ:



```nil id="g2ktw7"

let user = response.json<User>()

```



এখানে compiler/runtime ideally type metadata ব্যবহার করবে।



এই feature UI এবং API দুটোতেই কাজে লাগবে।



\---



\# ১২. `nilLang`-এর CLI আর `alap-framework`-এর CLI আলাদা করুন



এখন দুটো repository-তেই `nil` CLI আছে। `alap-framework`-এর CLI ইতিমধ্যেই `init`, `run`, `build`, `check`, `fmt`, `test`, `clean` handle করে। 



এটা future-এ এমন হওয়া উচিত:



\### Nilang CLI



```text

nilc

nilfmt

nills

nilrun

```



মূল language toolchain।



\### Alap CLI



```text

nil

```



application framework tool।



যেমন:



```bash

nil create myapp --template web

nil dev

nil build web

nil build server

nil preview

nil test

nil db migrate

nil db seed

nil generate api

nil deploy

```



অর্থাৎ `nil` হলো Alap developer experience।



\---



\# ১৩. এখন `alap-framework` repo-তে আসি



এখানে বড় পরিবর্তন হবে।



বর্তমান repo-র architecture-এ compiler/runtime/platform/UI/stdlib/package manager ইতিমধ্যে আছে। (\[GitHub]\[3])



Web যোগ করার জন্য আমি top-level structure করতাম:



```text

alap-framework/

│

├── framework/

│   ├── core/

│   ├── component/

│   ├── state/

│   ├── router/

│   └── lifecycle/

│

├── ui/

│   ├── core/

│   ├── widgets/

│   ├── layout/

│   ├── theme/

│   ├── animation/

│   ├── accessibility/

│   └── forms/

│

├── web/

│   ├── http/

│   ├── server/

│   ├── router/

│   ├── middleware/

│   ├── request/

│   ├── response/

│   ├── cookies/

│   ├── sessions/

│   ├── static/

│   ├── ssr/

│   ├── websocket/

│   ├── sse/

│   ├── csrf/

│   ├── cors/

│   └── security/

│

├── browser/

│   ├── runtime/

│   ├── dom/

│   ├── events/

│   ├── hydration/

│   ├── router/

│   └── fetch/

│

├── data/

│   ├── db/

│   ├── orm/

│   ├── migration/

│   ├── repository/

│   └── transaction/

│

├── auth/

│   ├── session/

│   ├── oidc/

│   ├── oauth/

│   ├── jwt/

│   ├── rbac/

│   ├── csrf/

│   └── password/

│

├── cache/

├── queue/

├── observability/

├── deploy/

└── templates/

```



\---



\# ১৪. `alap-framework`-এর সবচেয়ে প্রথম নতুন package: `web`



\## HTTP server



```go

type Server struct {

&#x20;   Addr        string

&#x20;   Handler     Handler

&#x20;   Middleware  \[]Middleware

}

```



কিন্তু standard `net/http`-এর ওপর সরাসরি application logic না বসিয়ে abstraction দিন:



```text

Incoming HTTP

&#x20;     ↓

Alap Request

&#x20;     ↓

Middleware

&#x20;     ↓

Router

&#x20;     ↓

Controller/Handler

&#x20;     ↓

Alap Response

```



\---



\# ১৫. Router-কে enterprise-grade করুন



বর্তমান `nilLang/pkg/alap/routing`-এর route abstraction আছে, কিন্তু সেটা আলাদা monolithic package হিসেবে পড়ে আছে। `alap-framework`-এ নতুন canonical router হওয়া উচিত।



Capabilities:



```text

GET

POST

PUT

PATCH

DELETE

OPTIONS

HEAD

```



এর সঙ্গে:



```text

/static/\*

/users/{id}

/posts/{slug}

/api/{version}/users/{id}

```



আর constraints:



```text

{id:int}

{id:uuid}

{slug:string}

```



তারপর route groups:



```nil

api "/api/v1" {

&#x20;   use auth



&#x20;   GET "/users" -> users.list

&#x20;   POST "/users" -> users.create

}

```



এটা শুধু convenience নয়; enterprise codebase-এ route organization-এর জন্য জরুরি।



\---



\# ১৬. Middleware system পুরোপুরি standardize করুন



Pipeline:



```text

Request

&#x20;↓

Recover

&#x20;↓

Request ID

&#x20;↓

Logger

&#x20;↓

Metrics

&#x20;↓

Tracing

&#x20;↓

CORS

&#x20;↓

Rate Limit

&#x20;↓

Security Headers

&#x20;↓

Session/Auth

&#x20;↓

CSRF

&#x20;↓

Router

```



Application developer শুধু:



```nil

app.use(auth)

app.use(rateLimit)

```



লিখবে।



\---



\# ১৭. `Context`-কে framework-এর কেন্দ্র বানান



প্রতি request-এ:



```text

ctx.request

ctx.response

ctx.params

ctx.query

ctx.headers

ctx.cookies

ctx.session

ctx.user

ctx.state

ctx.trace

ctx.logger

ctx.abort

```



এর ফলে framework-এর সব subsystem এক context-এর মাধ্যমে যুক্ত হবে।



\---



\# ১৮. SSR subsystem বানান



বর্তমান repo-তে UI engine আছে, কিন্তু Web SSR আলাদা subsystem নয়। UI folder-এ engine/layout/render/state/theme/widgets রয়েছে। (\[GitHub]\[4])



এখানে:



```text

ui tree

&#x20;  ↓

SSR renderer

&#x20;  ↓

HTML

```



দরকার।



API:



```go

RenderPage(component, context)

```



Output:



```text

<!doctype html>

<html>

<head>...</head>

<body>...</body>

</html>

```



কিন্তু security rule:



```text

Text        → escaped

Attribute   → escaped

RawHTML     → explicitly unsafe

URL         → validated

```



\---



\# ১৯. Hydration তৈরি করুন



SSR-এর পরে:



```text

HTML from server

&#x20;      ↓

Browser

&#x20;      ↓

Alap runtime loads

&#x20;      ↓

hydrate()

&#x20;      ↓

interactive components

```



এখানে server এবং browser-এর UI tree একই হওয়া দরকার।



এই কারণেই আমি UI IR-কে language/compiler level-এ রাখতে বলছি।



\---



\# ২০. Browser runtime তৈরি করুন



বর্তমান Alap UI engine-এর structure native application-এর দিকে বেশি oriented। (\[GitHub]\[4])



Web-এর জন্য আলাদা:



```text

browser/runtime

```



এখানে:



```text

Component Registry

State Store

Scheduler

DOM Renderer

Event Delegation

Router

Hydration

Effects

```



থাকবে।



\---



\# ২১. Event system বদলাতে হবে



বর্তমান declarative syntax-এ `onClick => {}` ধরনের handler আছে। 



এখন browser backend-এ:



```text

onClick

onInput

onChange

onSubmit

onKeyDown

onFocus

onBlur

onPointerDown

onPointerMove

onPointerUp

```



কে actual DOM listener-এ নামাতে হবে।



একটা global event delegation system দিলে performance ভালো হবে:



```text

document

&#x20; ↓

single delegated listener

&#x20; ↓

component id

&#x20; ↓

Alap event dispatcher

&#x20; ↓

NilLang callback

```



\---



\# ২২. Reactive state-এর proper implementation



বর্তমান Alap blueprint-এ UI `build()`-কে pure-ish রাখার কথা এবং side effects event/lifecycle/tasks-এ রাখার কথা আছে। (\[GitHub]\[2])



এটাই follow করুন।



State model:



```text

Signal

Computed

Effect

Store

Transaction

Batch

```



উদাহরণ:



```nil

state count: i32 = 0



computed doubled = count \* 2



effect {

&#x20;   log(doubled)

}

```



প্রতি state change-এ পুরো page rerender নয়।



বরং:



```text

state changed

&#x20;   ↓

dependency graph

&#x20;   ↓

affected components

&#x20;   ↓

minimal DOM patch

```



\---



\# ২৩. Router-কে browser + server দু জায়গায় একই API দিন



এটা খুব গুরুত্বপূর্ণ।



```text

Alap Router

```



দুটি backend:



```text

Server Router

Browser Router

```



Source:



```nil

route "/products/{id}" -> ProductPage

```



Server:



```text

HTTP request

```



Browser:



```text

history.pushState()

```



একই route definition।



\---



\# ২৪. `data` package-কে বাস্তব DB system বানান



বর্তমান blueprint `nil.db`-এর জন্য SQLite/Postgres/MySQL/remote API abstraction প্রস্তাব করছে। (\[GitHub]\[2])



কিন্তু enterprise web-এর জন্য প্রথম priority:



```text

PostgreSQL

```



তারপর:



```text

SQLite

MySQL

```



Architecture:



```text

alap/data/

├── db/

├── query/

├── orm/

├── transaction/

├── migration/

├── schema/

└── repository/

```



\---



\# ২৫. Entity system-কে ORM-এ পরিণত করুন



যে abstraction `nilLang/pkg/alap/entity`-এ শুরু হয়েছে, সেটা এবার framework-level architecture পাবে।



```nil

entity User {

&#x20;   id: UUID

&#x20;   name: String

&#x20;   email: Email unique

}

```



এর থেকে generate:



```text

PostgreSQL schema

Migration

Go/Nil repository

Type model

API schema

Validation

OpenAPI

```



অর্থাৎ:



```text

Entity

&#x20;↓

Schema compiler

&#x20;├── SQL

&#x20;├── API

&#x20;├── validation

&#x20;├── client model

&#x20;└── docs

```



\---



\# ২৬. Query builder দিন



ORM হলেও raw SQL পুরো নিষিদ্ধ করবেন না।



দুটি পথ:



```nil

User.find(id)

User.where("age > ?", 18)

```



এবং advanced:



```nil

db.query(

&#x20;   "SELECT ...",

&#x20;   \[arg1, arg2]

)

```



কিন্তু parameter binding বাধ্যতামূলক।



\---



\# ২৭. Transaction API



Enterprise application-এ অপরিহার্য:



```nil

db.transaction {

&#x20;   user.save()

&#x20;   order.save()

&#x20;   payment.save()

}

```



Failure:



```text

commit

```



না হলে automatic rollback।



\---



\# ২৮. Authentication + authorization



`AuthMiddleware`-এর বর্তমান সরল token comparison production auth নয়। (\[GitHub]\[2])



`alap-framework/auth`-এ:



```text

Password hashing

Session

Secure cookie

JWT

OIDC

OAuth2

MFA hooks

RBAC

ABAC

Tenant

API key

```



প্রথম release:



```text

session + secure cookie

password hashing

RBAC

CSRF

OIDC

```



এই পাঁচটি আগে।



\---



\# ২৯. Multi-tenancy



Enterprise-level হওয়ার জন্য এটা এখন থেকেই design-এ ঢোকান।



```text

request

&#x20;↓

tenant resolution

&#x20;↓

tenant context

&#x20;↓

repository

&#x20;↓

DB

```



যেমন:



```text

tenant\_id

```



প্রতি tenant-scoped table-এ enforce করা।



Application developer যেন ভুল করে tenant boundary bypass করতে না পারে।



\---



\# ৩০. WebSocket + SSE



`websocket()` blueprint-এ আছে, কিন্তু actual framework implementation লাগবে। (\[GitHub]\[2])



\### WebSocket



```nil

ws "/chat" {

&#x20;   onConnect(...)

&#x20;   onMessage(...)

&#x20;   onClose(...)

}

```



\### SSE



```nil

sse "/events" {

&#x20;   stream(...)

}

```



Dashboard, chat, notifications, monitoring-এর জন্য দরকার।



\---



\# ৩১. Background jobs



Enterprise application শুধু request-response নয়।



```text

alap/queue

├── job

├── worker

├── retry

├── backoff

├── dead-letter

└── scheduler

```



API:



```nil

queue.dispatch(SendWelcomeEmail(user))

```



Redis দিয়ে শুরু করা যায়।



\---



\# ৩২. Cache



Process-local cache রাখবেন, কিন্তু adapter abstraction করুন:



```text

Cache

&#x20;├── memory

&#x20;├── redis

&#x20;└── none

```



API:



```nil

cache.get("user:" + id)

cache.set("user:" + id, user, ttl)

```



\---



\# ৩৩. Observability



`alap-framework`-এ dedicated:



```text

observability/

├── logging/

├── metrics/

├── tracing/

├── health/

└── profiling/

```



Default request span:



```text

HTTP request

&#x20;  ↓

router span

&#x20;  ↓

DB span

&#x20;  ↓

external HTTP span

&#x20;  ↓

response

```



এটা ভবিষ্যৎ enterprise debugging-এর জন্য বিশাল সুবিধা।



\---



\# ৩৪. Security headers default করুন



Default:



```text

Content-Security-Policy

Strict-Transport-Security

X-Content-Type-Options

Referrer-Policy

Permissions-Policy

Frame restrictions

```



এবং secure cookie defaults।



Framework-এর কাজ হবে নিরাপদ default দেওয়া—developer-কে প্রতিটি security option manual configure করতে বাধ্য করা নয়।



\---



\# ৩৫. Static asset pipeline



Web app-এর জন্য:



```text

CSS

JS

images

fonts

icons

```



pipeline:



```text

source assets

&#x20;↓

hash

&#x20;↓

compress

&#x20;↓

manifest

&#x20;↓

cache-control

```



যেমন:



```text

app.css

&#x20;→

app.91f2a.css

```



\---



\# ৩৬. OpenAPI generation



এটা খুব মূল্যবান feature হতে পারে।



Route + request/response types থেকে:



```text

OpenAPI 3.x

```



generate হবে।



তারপর:



```bash

nil api docs

```



এবং:



```text

swagger/openapi

```



দিয়ে external consumers API বুঝতে পারবে।



\---



\# ৩৭. Typed API client generator



একই schema থেকে:



```text

NilLang client

TypeScript client

Kotlin client

Swift client

```



generate করা সম্ভব।



এখানে আপনার framework-এর entity/schema systemের পূর্ণ সুবিধা নেওয়া যাবে।



\---



\# ৩৮. `alap-framework/cmd/nil`-এ web workflow যোগ করুন



বর্তমান CLI architecture-এ project creation, run, build, check, format, test, package installation already আছে। 



নতুন command:



```bash

nil create myapp --template web

nil dev

nil build web

nil build server

nil preview

nil serve

nil db generate

nil db migrate

nil db rollback

nil api generate

nil routes

nil doctor

```



সবচেয়ে গুরুত্বপূর্ণ:



```bash

nil dev

```



এটা ideally:



```text

compiler watch

\+

SSR server

\+

browser build

\+

asset watcher

\+

hot reload

```



একসঙ্গে চালাবে।



\---



\# ৩৯. Project template বদলান



বর্তমান template `alap.yaml`-কে web-capable করুন। CLI template-এ বর্তমান entry, permissions, targets এবং dependencies already আছে। 



আমি:



```yaml

name: myapp

version: 0.1.0



type: web



entry:

&#x20; server: src/server.nil

&#x20; client: src/client.nil



web:

&#x20; ssr: true

&#x20; hydration: true

&#x20; router: true



database:

&#x20; driver: postgres



targets:

&#x20; server:

&#x20;   os: linux

&#x20;   arch: amd64



&#x20; browser:

&#x20;   target: web



dependencies: {}

```



জাতীয় manifest করতাম।



\---



\# ৪০. Web application structure



`nil create app --template web` চালালে:



```text

myapp/

├── alap.yaml

├── src/

│   ├── server.nil

│   ├── client.nil

│   ├── routes.nil

│   ├── components/

│   ├── pages/

│   ├── models/

│   ├── services/

│   ├── repositories/

│   └── middleware/

├── public/

├── assets/

├── migrations/

├── tests/

└── build/

```



\---



\# ৪১. Nilang language syntax এমন হওয়া উচিত



আমি খুব বেশি framework magic না করে পরিষ্কার syntax রাখতাম।



উদাহরণ:



```nil

import { App, Page, Button, Text } from "alap/web"

import { User } from "./models/user"



route GET "/" {

&#x20;   return Page {

&#x20;       Text("Hello Nilang")

&#x20;   }

}

```



API:



```nil

route GET "/api/users" {

&#x20;   let users = await User.all()

&#x20;   return json(users)

}

```



Protected API:



```nil

@auth

@role("admin")

route POST "/api/users" {

&#x20;   ...

}

```



Component:



```nil

@Component

component Counter {

&#x20;   @State count: i32 = 0



&#x20;   build() {

&#x20;       Column {

&#x20;           Text(count.toString())



&#x20;           Button("+") {

&#x20;               count += 1

&#x20;           }

&#x20;       }

&#x20;   }

}

```



এগুলো \*\*proposed syntax\*\*, বর্তমান Nilang syntax বলে দাবি করছি না। তবে এগুলো বর্তমান Alap parser-এর component/decorator/state/build ধারণার সঙ্গে সামঞ্জস্যপূর্ণ। 



\---



\# ৪২. Browser code এবং server code একই language-এ থাকবে



এটাই পুরো পরিকল্পনার সবচেয়ে গুরুত্বপূর্ণ ফল।



```text

&#x20;             Nilang source

&#x20;                   │

&#x20;       ┌───────────┴───────────┐

&#x20;       │                       │

&#x20;    Server                   Browser

&#x20;       │                       │

&#x20;     native                    JS/WASM

&#x20;       │                       │

&#x20;     Alap SSR               Alap runtime

&#x20;       │                       │

&#x20;      HTML                    DOM

```



একই:



```text

types

models

validation

business logic

API schema

```



দুই target-এ share করা যাবে।



\---



\# ৪৩. কিন্তু server আর browser capability আলাদা রাখতে হবে



এটা খুব গুরুত্বপূর্ণ।



Browser-এ:



```text

filesystem ✗

database ✗

process ✗

raw TCP ✗

```



Server-এ:



```text

filesystem ✓

database ✓

process ✓

network ✓

```



Compiler capability system দিয়ে enforce করুন:



```text

@server

@browser

@shared

```



অথবা module capability metadata।



এটা ভুল করে security boundary ভাঙা ঠেকাবে।



\---



\# ৪৪. Shared code model



আমি ৩ ধরনের module রাখতাম:



```text

shared/

server/

client/

```



উদাহরণ:



```text

src/

├── shared/

│   ├── User.nil

│   └── validation.nil

├── server/

│   ├── db.nil

│   └── auth.nil

└── client/

&#x20;   ├── App.nil

&#x20;   └── pages.nil

```



Compiler automatically capability check করবে।



\---



\# ৪৫. বর্তমান দুটি repo-র code migration-এর বাস্তব পথ



এটা সবচেয়ে গুরুত্বপূর্ণ অংশ।



\## Phase 1 — `nilLang`



প্রথমে:



```text

nilLang/compiler

```



কে canonical করুন।



`alap-framework/compiler` থেকে advanced pieces port করুন:



```text

AST

type checker

parser features

HIR

formatter

LSP concepts

```



এবং version দিন:



```text

NilLang 0.2

```



\---



\# ৪৬. Phase 2 — `alap-framework`



`alap-framework` থেকে compiler ownership কমিয়ে দিন।



শেষে:



```text

alap-framework/compiler

```



প্রায় থাকবেই না, অথবা খুব thin framework-specific lowering layer থাকবে।



তার বদলে:



```text

alap-framework

&#x20;  ↓

depends on

&#x20;  ↓

nilLang compiler SDK

```



\---



\# ৪৭. `pkg/alap` কী করবেন?



বর্তমান `nilLang`-এ `pkg/alap`-এর:



```text

ai

core

data

entity

onuron

routing

server

state

ui

```



অংশগুলো আছে। (\[GitHub]\[5])



এগুলোর framework-specific অংশ `alap-framework`-এ স্থানান্তর করাই পরিষ্কার।



\### `nilLang`-এ রাখুন



```text

compiler

runtime

stdlib

language tooling

FFI

serialization

net/http primitives

```



\### `alap-framework`-এ রাখুন



```text

entity

server framework

routing

component

UI

state

web

auth

ORM

SSR

browser

platform integration

```



\---



\# ৪৮. তবে সবকিছু সরিয়ে দিলে চলবে না



একটা গুরুত্বপূর্ণ distinction:



```text

Language primitive

```



আর



```text

Framework abstraction

```



আলাদা।



উদাহরণ:



\### Nilang



```nil

let x = await http.get(url)

```



এটা language/std API।



\### Alap



```nil

route GET "/users"

```



এটা framework abstraction।



\### Nilang



```nil

Future<T>

Result<T,E>

```



language।



\### Alap



```nil

@app

@middleware

@route

```



framework।



\---



\# ৪৯. নতুন web target-এর জন্য তিনটি runtime লাগবে



\### ১. Server runtime



```text

NilLang VM/native

\+

Alap server

```



\### ২. Browser runtime



```text

JS/WASM

\+

Alap browser runtime

```



\### ৩. SSR runtime



```text

Server runtime

\+

UI renderer

```



এই তিনটি একসঙ্গে না করলে Full-stack Nilang হবে না।



\---



\# ৫০. শেষ architecture



শেষে আমি এই architecture-টাই চাইতাম:



```text

&#x20;                       Nilang

&#x20;                          │

&#x20;               ┌──────────┴──────────┐

&#x20;               │                     │

&#x20;            Language               Alap

&#x20;               │                     │

&#x20;      ┌────────┼────────┐      ┌─────┼─────┐

&#x20;      │        │        │      │     │     │

&#x20;     AST      IR      Runtime   UI   Web   Data

&#x20;      │        │        │      │     │     │

&#x20;   Parser    NIR      VM       SSR  HTTP   ORM

&#x20;   Types     WASM     GC       CSR  WS     DB

&#x20;   HIR       Native   Async    DOM  SSE    Cache

&#x20;      │                         │

&#x20;      └──────────────┬──────────┘

&#x20;                     │

&#x20;                Application

&#x20;                     │

&#x20;       ┌─────────────┼─────────────┐

&#x20;       │             │             │

&#x20;     Server        Browser       Mobile

```



\---



\# ৫১. কাজের priority আমি এভাবে দেব



| Priority | Repo    | কাজ                              |

| -------- | ------- | -------------------------------- |

| P0       | দুটো    | compiler duplication বন্ধ        |

| P0       | nilLang | canonical AST/parser/type system |

| P0       | nilLang | HIR/MIR/NIR pipeline             |

| P0       | nilLang | target architecture              |

| P0       | Alap    | framework ownership separation   |

| P1       | nilLang | async/await/result/future        |

| P1       | nilLang | JSON/HTTP/Web APIs               |

| P1       | Alap    | HTTP server                      |

| P1       | Alap    | router                           |

| P1       | Alap    | middleware                       |

| P1       | Alap    | UI runtime                       |

| P1       | Alap    | browser renderer                 |

| P1       | Alap    | SSR                              |

| P2       | nilLang | JS backend                       |

| P2       | nilLang | WASM backend                     |

| P2       | Alap    | hydration                        |

| P2       | Alap    | WebSocket/SSE                    |

| P2       | Alap    | PostgreSQL                       |

| P2       | Alap    | ORM/entity                       |

| P2       | Alap    | auth                             |

| P2       | Alap    | sessions                         |

| P3       | Alap    | Redis/cache                      |

| P3       | Alap    | queue/jobs                       |

| P3       | Alap    | OpenAPI                          |

| P3       | Alap    | observability                    |

| P3       | Alap    | multi-tenancy                    |

| P4       | Alap    | Kubernetes/deployment tooling    |

| P4       | Alap    | enterprise security/policy       |



\---



\# ৫২. সবচেয়ে গুরুত্বপূর্ণ তিনটি architectural সিদ্ধান্ত



\*\*প্রথম:\*\* `nilLang`-ই একমাত্র Nilang compiler-এর মালিক হবে। আলাদা `alap-framework/compiler` রাখা উচিত নয়। বর্তমান দুই repo-র compiler implementation-এর divergence ইতিমধ্যেই দেখা যাচ্ছে। 



\*\*দ্বিতীয়:\*\* `Alap` হবে language-এর ওপর নির্মিত framework, compiler নয়। বর্তমানে Alap repo-তে compiler/runtime/UI/platform সব একসঙ্গে আছে; এটাকে framework-facing অংশে নামিয়ে আনতে হবে। (\[GitHub]\[3])



\*\*তৃতীয়:\*\* Web-কে “HTML preview” হিসেবে নয়, \*\*first-class compilation target\*\* হিসেবে ধরতে হবে। আপনার blueprint-এ Web/PWA এখন Phase 2/3 এবং optional বলা আছে; enterprise web লক্ষ্য করলে এটাকে optional রাখা যাবে না। Web IR → JS/WASM → DOM/SSR architecture-কে roadmap-এর মূল target করতে হবে। (\[GitHub]\[2])



এখনকার repo অবস্থার ওপর ভিত্তি করে সবচেয়ে বড় ভুল হবে সরাসরি `web/` folder বানিয়ে কিছু HTTP endpoint যোগ করা। তাতে demo web app হবে, কিন্তু Nilang-এর জন্য scalable web platform হবে না। আগে \*\*compiler ownership + IR + capability model\*\*, তারপর \*\*Alap server/browser/SSR\*\*, তারপর \*\*DB/auth/enterprise\*\*—এই ক্রমটাই সঠিক।



\[1]: https://github.com/joysriramsarkar/nilLang "GitHub - joysriramsarkar/nilLang: Cross-platform mobile programming language · GitHub"

\[2]: https://github.com/joysriramsarkar/alap-framework/blob/master/Alap\_Framework\_NilLang\_Blueprint.md "alap-framework/Alap\_Framework\_NilLang\_Blueprint.md at master · joysriramsarkar/alap-framework · GitHub"

\[3]: https://github.com/joysriramsarkar/alap-framework "GitHub - joysriramsarkar/alap-framework: cross-os app building framework for nilOS, Android, IOS, linux · GitHub"

\[4]: https://github.com/joysriramsarkar/alap-framework/tree/master/ui "alap-framework/ui at master · joysriramsarkar/alap-framework · GitHub"

\[5]: https://github.com/joysriramsarkar/nilLang/tree/main/pkg/alap "nilLang/pkg/alap at main · joysriramsarkar/nilLang · GitHub"





হ্যাঁ। বর্তমান `main` branch দেখে বলছি—Nilang-এর web capability একেবারে শূন্য নয়; বরং একটা ভালো \*\*full-stack web framework-এর skeleton ইতিমধ্যেই আছে\*\*। কিন্তু এখনো এটাকে React/Next.js/Django/Laravel-এর সমতুল্য পূর্ণ web platform বলা যাবে না। বিশেষ করে browser-side runtime এবং production server stack-টাই সবচেয়ে বড় অসম্পূর্ণ অংশ। (\[GitHub]\[1])



\## ১. এখন Nilang-এর web অংশে আসলে কী আছে



বর্তমান `Alap`-এর মধ্যে যথেষ্ট ভালো কিছু foundation আছে:



`pkg/alap/routing`-এ route, method, path parameter, request context এবং dispatcher আছে। `GET/POST/PUT/DELETE` route এবং `{id}`/`:id` ধরনের parameter matching করা যায়। 



`pkg/alap/server`-এ service, endpoint, middleware chain, request context, JSON serialization এবং in-memory cache আছে। অর্থাৎ API/backend architecture-এর একটি প্রাথমিক skeleton তৈরি হয়েছে। 



`pkg/alap/entity` আরও গুরুত্বপূর্ণ। সেখানে UUID, String, Email, Int, Float, Bool, Markdown, Date, Relation ইত্যাদি field type, validation, PostgreSQL-oriented SQL DDL generation, REST endpoint specification এবং web-এর জন্য TypeScript interface generation আছে। 



UI layer-এ `Page`, `Navigation`, `Card`, `Table`, `Form`, `Dashboard` আছে এবং এগুলো HTML render করতে পারে। 



আর low-level `nilui` abstraction-এ click/change/keypress/resize/hover/focus event-এর ধারণাও আছে। 



CLI-তেও সরাসরি web profile এবং WASM target-এর কথা আছে:



```text

nil init my-web-app --profile web

nil build wasm

nil render

```



তবে এগুলো CLI-তে ঘোষিত capability; এগুলোকে এখনই পূর্ণ browser application runtime ধরে নেওয়া যাবে না। 



\---



\# ২. সবচেয়ে গুরুত্বপূর্ণ সমস্যা: বর্তমান HTML renderer ≠ Web Framework



এটা খুব পরিষ্কারভাবে আলাদা করা দরকার।



বর্তমান `RenderToHTML()` HTML string তৈরি করে। যেমন `Button` শেষ পর্যন্ত সাধারণ HTML `<button>` হয়ে যায়, `Input` সাধারণ `<input>` হয়। 



কিন্তু browser-এ:



```text

click

&#x20;  ↓

Nilang function

&#x20;  ↓

state change

&#x20;  ↓

component re-render

```



এই lifecycle এখনো প্রকৃত browser runtime হিসেবে তৈরি হয়নি।



বরং বর্তমান component code-এ `SetState()` করলে শুধু re-render trigger-এর log করা হচ্ছে:



```text

\[Alap] Re-render triggered for ...

```



এবং UI component-এর event callback Go-side object-এ আছে; HTML renderer সেই callback-কে browser JavaScript event হিসেবে wire করে না। 



অর্থাৎ এখনকার architecture:



```text

Nilang/Alap

&#x20;   ↓

UI Tree

&#x20;   ↓

HTML String

&#x20;   ↓

Browser

```



Enterprise-level architecture হওয়া উচিত:



```text

&#x20;                   ┌──────── Browser ────────┐

&#x20;                   │ DOM / WASM / JS bridge  │

&#x20;                   │ Event + State + Router  │

&#x20;                   └───────────┬─────────────┘

&#x20;                               │

&#x20;                        HTTP / WS / SSE

&#x20;                               │

&#x20;                   ┌───────────▼─────────────┐

&#x20;                   │      Nilang Server      │

&#x20;                   │ Router / Middleware     │

&#x20;                   │ Auth / API / SSR        │

&#x20;                   └───────────┬─────────────┘

&#x20;                               │

&#x20;                   ┌───────────▼─────────────┐

&#x20;                   │ Alap Data / ORM / Cache  │

&#x20;                   └───────────┬─────────────┘

&#x20;                               │

&#x20;                      PostgreSQL / Redis

```



\---



\# ৩. আমি Nilang-এর জন্য যে web architecture বানাতাম



আমি এটাকে চারটি স্তরে ভাগ করতাম।



\### স্তর A — `Alap Web UI`



Browser-এর জন্য:



```text

component

state

props

render

event

route

```



উদাহরণ হিসেবে ভবিষ্যৎ Nilang syntax এমন হতে পারে:



```nil

component Counter {

&#x20;   state count = 0;



&#x20;   render {

&#x20;       Column {

&#x20;           Text("Count: \\(count)")



&#x20;           Button("−") {

&#x20;               count = count - 1;

&#x20;           }



&#x20;           Button("+") {

&#x20;               count = count + 1;

&#x20;           }

&#x20;       }

&#x20;   }

}

```



এটা \*\*বর্তমান language syntax বলে দাবি করছি না\*\*; এটা আমি Nilang-এর existing declarative direction ধরে proposed web syntax হিসেবে দেখাচ্ছি।



Browser-এ এটাকে compile করতে হবে:



```text

Nilang

&#x20;↓

AST

&#x20;↓

UI IR

&#x20;↓

WASM / Browser Runtime

&#x20;↓

DOM

```



অথবা শুরুতে:



```text

Nilang

&#x20;↓

JavaScript

&#x20;↓

DOM

```



বাস্তবসম্মতভাবে প্রথম implementation-এর জন্য JS backend অনেক সহজ হবে। WASM পরে performance-critical অংশে ব্যবহার করা যায়।



\---



\# ৪. স্তর B — Nilang Full-Stack Server



বর্তমান `routing` এবং `server` code এখানে খুব ভালো starting point। `Service`, `Endpoint`, middleware chain এবং `Context` ইতিমধ্যে আছে। 



এখন যা করতে হবে:



```text

pkg/alap/web/

├── router

├── http

├── request

├── response

├── middleware

├── session

├── cookie

├── websocket

├── sse

├── static

├── template

└── server

```



তখন ব্যবহারটা এমন হতে পারে:



```nil

web App("shop")



App.get("/", home)

App.get("/products/{id}", product)

App.post("/api/orders", createOrder)



App.listen(":8080")

```



এবং handler:



```nil

fn createOrder(ctx) {

&#x20;   let data = ctx.json();



&#x20;   ...

&#x20;   

&#x20;   return json({

&#x20;       "ok": true

&#x20;   });

}

```



কিন্তু একটা গুরুত্বপূর্ণ কাজ আগে করতে হবে: বর্তমানে `server.Service` নিজে বাস্তব TCP/HTTP listener চালায় না; `HandleRequest()` মূলত HTTP-style request নিয়ে internally route dispatch করে এবং JSON return করে। 



অতএব পরের milestone হবে:



```text

net.Listener

&#x20;   ↓

HTTP server

&#x20;   ↓

Request parsing

&#x20;   ↓

Alap Context

&#x20;   ↓

Router

&#x20;   ↓

Middleware

&#x20;   ↓

Handler

&#x20;   ↓

Response

```



\---



\# ৫. স্তর C — Database + ORM



এখানে Nilang ইতিমধ্যে একটি অসাধারণ দিক খুলে রেখেছে।



`Entity` থেকে:



```text

Entity definition

&#x20;      ↓

Validation

&#x20;      ↓

SQL schema

&#x20;      ↓

REST specification

&#x20;      ↓

Client model

```



এই architecture-টাই enterprise framework-এর backbone হতে পারে। 



যেমন:



```nil

entity User {

&#x20;   id: UUID primary

&#x20;   name: String required

&#x20;   email: Email required unique

&#x20;   createdAt: Date

}



entity Order {

&#x20;   id: UUID primary

&#x20;   user: Relation<User> required

&#x20;   total: Float required

}

```



তারপর:



```bash

nil db migrate

nil db generate

nil db seed

```



Framework নিজে করতে পারে:



```text

Entity

&#x20;↓

Migration

&#x20;↓

PostgreSQL schema

&#x20;↓

Repository / Query layer

&#x20;↓

REST / GraphQL API

&#x20;↓

Client model

```



\### কিন্তু এখানে বর্তমান বড় ঘাটতি



বর্তমান `Entity.GenerateSQL()` schema বানাতে পারে, কিন্তু সেটা নিজে database persistence layer নয়। 



Enterprise-এর জন্য দরকার:



```text

Database driver

Connection pool

Transaction

Prepared statement

Query builder

Migration

Rollback

Index

Foreign key

Pagination

Optimistic locking

```



আর database হিসেবে প্রথমে \*\*PostgreSQL\*\* নিলে সবচেয়ে যুক্তিযুক্ত।



\---



\# ৬. স্তর D — SSR + SPA দুটোই



এটাই Nilang-এর জন্য সবচেয়ে শক্তিশালী architecture হতে পারে।



একটা `.nil` application থেকে দুই ধরনের output:



\### SSR



```text

Request

&#x20;↓

Nilang Server

&#x20;↓

Component Tree

&#x20;↓

HTML

&#x20;↓

Browser

```



\### Interactive SPA



```text

Initial HTML

&#x20;↓

Hydration

&#x20;↓

Nilang browser runtime

&#x20;↓

Stateful UI

```



অর্থাৎ Next.js-এর মতো ধারণা:



```text

Static rendering

SSR

CSR

Hydration

API

WebSocket

```



সব একই ecosystem-এ।



\---



\# ৭. বর্তমান UI layer-কে আরও শক্তিশালী করতে হবে



বর্তমান UI component library-তে `Page`, `Navigation`, `Card`, `Table`, `Form`, `Dashboard` আছে। 



এর ওপর আমি এভাবে library বানাতাম:



```text

alap/web

├── Button

├── Input

├── Select

├── Checkbox

├── Radio

├── Form

├── Modal

├── Drawer

├── Tabs

├── Table

├── DataGrid

├── Pagination

├── Chart

├── Dropdown

├── Toast

├── Alert

├── DatePicker

├── FileUpload

├── Layout

├── Grid

└── Responsive

```



এবং component API:



```text

props

state

events

slots

children

effects

lifecycle

accessibility

```



বিশেষ করে enterprise-এর জন্য:



```text

keyboard navigation

ARIA

focus management

screen reader support

responsive layout

dark/light theme

i18n

RTL

```



অত্যন্ত গুরুত্বপূর্ণ।



\---



\# ৮. Authentication ছাড়া enterprise web সম্ভব নয়



বর্তমান `AuthMiddleware()` একটি নির্দিষ্ট bearer token-এর সঙ্গে `Authorization` header মিলিয়ে দেখে। এটি demo-level authentication, production identity system নয়। (\[GitHub]\[2])



এটাকে বদলে করতে হবে:



```text

alap/auth

├── session

├── cookie

├── password

├── password\_hash

├── MFA

├── OAuth2

├── OIDC

├── JWT

├── CSRF

├── RBAC

├── ABAC

├── API keys

└── service identity

```



তারপর:



```nil

route "/admin" {

&#x20;   auth required

&#x20;   role "admin"



&#x20;   ...

}

```



Enterprise-এ শুধু authentication নয়:



```text

Identity

Authentication

Authorization

Tenant isolation

Audit log

Session management

Credential rotation

```



সব লাগবে।



\---



\# ৯. Current cache-টাও enterprise-ready নয়



বর্তমান server cache:



```go

cache map\[string]cacheEntry

```



অর্থাৎ process-local memory cache। 



একটি single-process application-এর জন্য ঠিক আছে।



কিন্তু:



```text

Server A

Server B

Server C

```



হলে প্রত্যেকটির cache আলাদা।



তখন:



```text

Alap Cache API

&#x20;    ↓

┌───────────────┐

│ Local Memory  │

│ Redis         │

│ Memcached     │

└───────────────┘

```



করতে হবে।



\---



\# ১০. Rate limiting-ও এখন শুধু metadata



`Endpoint`-এ:



```go

RateLimit int

```



আছে। কিন্তু বর্তমান code-এ এটা বাস্তব rate-limiter হিসেবে কাজ করছে না। 



Enterprise version-এ:



```text

Global rate limit

Per-IP

Per-user

Per-token

Per-route

Burst

Sliding window

Token bucket

Distributed Redis limiter

```



দরকার।



\---



\# ১১. Observability যোগ করতে হবে



বর্তমানে logging middleware execution time নেয়, কিন্তু বাস্তব metrics system নেই। 



Enterprise stack:



```text

logs

metrics

traces

profiles

health checks

readiness

liveness

audit events

```



এবং ideally:



```text

OpenTelemetry

Prometheus

structured JSON logs

```



সাপোর্ট।



\---



\# ১২. Security-এর একটা বড় কাজ এখনই দরকার



বর্তমান HTML renderer-এ string values সরাসরি HTML-এর মধ্যে বসানো হচ্ছে। যেমন title, text, attributes ইত্যাদি সরাসরি output হচ্ছে। 



এখানে escaping/sanitization না থাকলে untrusted data দিয়ে XSS-এর ঝুঁকি তৈরি হতে পারে।



তাই framework-level rule হওয়া উচিত:



```text

Text → HTML escaped

Attribute → attribute escaped

RawHTML → explicitly unsafe

Markdown → sanitized

URL → validated

```



এটা শুরুতেই ঠিক করা দরকার। পরে ecosystem তৈরি হওয়ার পর এই ধরনের ভুল বদলানো অত্যন্ত কঠিন।



\---



\# ১৩. Enterprise application architecture



শেষ পর্যন্ত আমি Nilang application-এর default structure এভাবে রাখতাম:



```text

my-enterprise-app/

│

├── app.nil

├── nil.json

│

├── routes/

│   ├── web.nil

│   └── api.nil

│

├── pages/

│   ├── home.nil

│   ├── login.nil

│   └── dashboard.nil

│

├── components/

│   ├── navbar.nil

│   ├── table.nil

│   └── form.nil

│

├── models/

│   ├── user.nil

│   └── order.nil

│

├── services/

│   ├── auth.nil

│   ├── payment.nil

│   └── email.nil

│

├── repositories/

│   ├── user.nil

│   └── order.nil

│

├── middleware/

│   ├── auth.nil

│   ├── cors.nil

│   └── rate\_limit.nil

│

├── jobs/

│   ├── email.nil

│   └── cleanup.nil

│

├── migrations/

│

├── tests/

│

├── public/

│   ├── images/

│   ├── fonts/

│   └── static/

│

└── config/

&#x20;   ├── development.nil

&#x20;   ├── production.nil

&#x20;   └── test.nil

```



এতে language এবং framework দুটোই scalable হবে।



\---



\# ১৪. Basic → Enterprise roadmap



আমি Nilang-এ web development এই sequence-এ করতাম।



| Stage | কী বানানো হবে             | বর্তমান অবস্থা                 |

| ----- | ------------------------- | ------------------------------ |

| 1     | Static HTML page          | ✅ আছে                          |

| 2     | Component library         | ✅ foundation আছে               |

| 3     | Server routing            | ✅ foundation আছে               |

| 4     | API endpoints             | ✅ foundation আছে               |

| 5     | Forms                     | ✅ UI আছে, server binding দরকার |

| 6     | Browser events            | ⚠️ দরকার                       |

| 7     | Reactive browser state    | ⚠️ দরকার                       |

| 8     | JS/WASM browser runtime   | ⚠️ বড় কাজ                      |

| 9     | SSR                       | ⚠️ দরকার                       |

| 10    | Hydration                 | ⚠️ দরকার                       |

| 11    | PostgreSQL ORM            | ❌ বড় কাজ                       |

| 12    | Migration system          | ❌ দরকার                        |

| 13    | Authentication            | ⚠️ demo-level আছে              |

| 14    | Sessions/Cookies/OIDC     | ❌ দরকার                        |

| 15    | WebSocket/SSE             | ❌ দরকার                        |

| 16    | Redis/cache               | ⚠️ local cache আছে             |

| 17    | Queue/jobs                | ❌ দরকার                        |

| 18    | File/object storage       | ❌ দরকার                        |

| 19    | RBAC/tenancy              | ❌ দরকার                        |

| 20    | Observability             | ⚠️ খুব basic                   |

| 21    | Security hardening        | ❌ বড় কাজ                       |

| 22    | Production deployment     | ⚠️ শুরু করা যায়                |

| 23    | Kubernetes/multi-instance | ❌ framework integration দরকার  |

| 24    | Enterprise SDK/tooling    | ❌ দরকার                        |



এই table-টাই আমার কাছে Nilang Web-এর বাস্তব roadmap।



\---



\# ১৫. কিন্তু একটা আরও ভালো design সম্ভব



আমি Nilang-কে শুধু “আরেকটা web framework” করতাম না।



এটাকে বানাতাম:



```text

&#x20;            Nilang

&#x20;              │

&#x20;      ┌───────┼────────┐

&#x20;      │       │        │

&#x20;     Web    Mobile   Desktop

&#x20;      │       │        │

&#x20;      └───────┼────────┘

&#x20;              │

&#x20;             Alap

&#x20;              │

&#x20;    ┌─────────┼──────────┐

&#x20;    │         │          │

&#x20;   UI       Data       Network

&#x20;    │         │          │

&#x20;   DOM      SQL       HTTP/WS

&#x20;   WASM     Cache      RPC

```



অর্থাৎ \*\*এক language + এক component model + এক data model + এক application architecture\*\*।



এটা Nilang-এর existing direction-এর সঙ্গে অনেক বেশি সামঞ্জস্যপূর্ণ। কারণ বর্তমানে Alap-এ UI, routing, server, state, entity, data, AI এবং Onuron adapter—সবকিছুকেই একই framework-এর অংশ করা হয়েছে। 



\---



\# ১৬. সবচেয়ে জরুরি: কোন কাজটা আগে করবেন



আমার মতে এখনই PostgreSQL, Kubernetes, microservices দিয়ে শুরু করা ভুল হবে।



প্রথমে এই চারটি জিনিস সম্পূর্ণ করুন:



```text

1\. Browser Runtime

2\. HTTP Server

3\. Reactive State + Events

4\. Database Layer

```



তারপর:



```text

SSR

Authentication

WebSocket

Cache

Jobs

Observability

Security

```



তারপর enterprise deployment।



কারণ browser runtime না থাকলে Alap-এর web UI মূলত HTML generator হয়, আর HTTP listener না থাকলে server package পূর্ণ web server হয় না। বর্তমানে এই দুই জায়গাতেই সবচেয়ে বেশি architectural work বাকি। 



\## আমার মতে Nilang-এর ultimate web stack



```text

&#x20;                 Nilang

&#x20;                    │

&#x20;             ┌──────▼──────┐

&#x20;             │     Alap    │

&#x20;             └──────┬──────┘

&#x20;                    │

&#x20;      ┌─────────────┼─────────────┐

&#x20;      │             │             │

&#x20;     Web            API          Data

&#x20;      │             │             │

&#x20;  SSR/CSR         REST/WS       ORM

&#x20;  DOM/WASM        Auth          SQL

&#x20;  Router          Middleware    Cache

&#x20;  State           Rate Limit    Queue

&#x20;      │             │             │

&#x20;      └─────────────┼─────────────┘

&#x20;                    │

&#x20;             PostgreSQL/Redis

&#x20;                    │

&#x20;              Linux/Docker

&#x20;                    │

&#x20;            Kubernetes/Cloud

```



\*\*এই architecture বানাতে পারলে Nilang দিয়ে blog বা landing page নয়—SaaS, ERP, CRM, e-commerce, admin platform, enterprise dashboard, internal business system—সবই বানানো সম্ভব হবে।\*\*



আর সবচেয়ে ভালো ব্যাপার হলো, আপনার বর্তমান repo-তে তার বেশ কিছু foundation ইতিমধ্যে আছে: routing, server abstraction, entities, SQL generation, REST specification, HTML UI rendering, state/UI primitives, package manager এবং WASM target-এর ধারণা। এখন এগুলোকে আলাদা আলাদা demo না রেখে একটি \*\*সুসংহত `Alap Web` platform\*\*-এ পরিণত করাই আসল কাজ। 



পরের ধাপে সবচেয়ে কার্যকর কাজ হবে \*\*বর্তমান `nilLang` repo-র জন্য সম্পূর্ণ `Alap Web 1.0` specification বানানো—directory structure, Nilang syntax, browser runtime, SSR, HTTP API, ORM, auth, WebSocket এবং enterprise architecture মিলিয়ে, কোন file-এ কী code যোগ করতে হবে সেটাও নির্দিষ্ট করে।\*\*



\[1]: https://github.com/joysriramsarkar/nilLang "GitHub - joysriramsarkar/nilLang: Cross-platform mobile programming language · GitHub"

\[2]: https://github.com/joysriramsarkar/nilLang/blob/main/pkg/alap/server/server.go "nilLang/pkg/alap/server/server.go at main · joysriramsarkar/nilLang · GitHub"


# Alap Framework + NilLang দিয়ে POS App — Readiness Assessment ও নীল নকশা

> দুটো রিপোই (`joysriramsarkar/alap-framework`, `joysriramsarkar/nilLang`) সরাসরি ক্লোন করে কোড পড়ে এই রিপোর্ট বানানো হয়েছে (সর্বশেষ কমিট: nilLang — ৫ সেপ্টেম্বর ২০২৬)। নিচে ফাইল/ফাংশন-লেভেল রেফারেন্স দেওয়া আছে, যাতে যাচাই করে নিতে পারেন।

---

## ১. সংক্ষিপ্ত উত্তর

**না, এখনই এই দুটো দিয়ে `pos-app`-এর মতো একটা প্রোডাকশন অ্যাপ বানানো যাবে না।** কম্পাইলার/রানটাইম ইঞ্জিনিয়ারিং অংশ সত্যিই চমৎকার এবং কার্যকর (lexer → parser → typecheck → HIR → MIR → bytecode VM → WASM, LSP, REPL — সবই বাস্তবে কাজ করে)। কিন্তু **অ্যাপ্লিকেশন-লেভেলের ক্ষমতাগুলো** — ডাটাবেস, HTTP সার্ভার, রিয়েল UI রেন্ডারিং, মডিউল/ইম্পোর্ট সিস্টেম — এখনো `.nil` ভাষার সাথে **সংযুক্তই (wired) হয়নি**। যা README/example-এ "কাজ করছে" বলে দেখানো হয়েছে, তার অনেকটাই আসলে `puts()` দিয়ে ছাপানো বর্ণনামূলক সিমুলেশন, বাস্তব এক্সিকিউশন নয়।

---

## ২. যা সত্যিই কাজ করে (verified)

| অংশ | প্রমাণ | মন্তব্য |
|---|---|---|
| Lexer/Parser/AST | `compiler/lexer`, `compiler/parser`, `compiler/ast` (nilLang) | বাস্তব, টেস্ট সহ |
| স্ট্যাটিক টাইপচেক | `compiler/typecheck/typecheck.go` | কাজ করে, টেস্ট আছে |
| Tree-walking Interpreter | `compiler/evaluator/evaluator.go` | `nil run` দিয়ে চলে |
| Bytecode Compiler + VM | `compiler/compiler`, `compiler/vm` | `nil run -vm` |
| HIR/MIR/WASM ব্যাকএন্ড | `compiler/hir`, `compiler/mir`, `compiler/wasm` | গতকালই (৫ সেপ্টেম্বর) যোগ হয়েছে |
| LSP সার্ভার | `cmd/nills` (alap-framework), `compiler/oracle` (nilLang) | এডিটর সাপোর্টের ভিত্তি আছে |
| বেসিক ভাষা: variable, function, closure, loop, string interpolation | `compiler/evaluator/builtins.go` (মোট ~৩৪টি বিল্ট-ইন: `len, puts, str, split, join, readFile, writeFile, exec, time...`) | সীমিত কিন্তু বাস্তব |
| প্যাকেজ সাইনিং/বান্ডলিং টুল | `pkg/signing`, `pkg/bundle`, `cmd/nilkey` | Ed25519 সাইনিং, `.nilax` বান্ডল ফরম্যাট বাস্তব কোড |
| SoftBus (LAN discovery/RPC) | `pkg/softbus/*` | Go-লেভেলে বাস্তবায়িত, কিন্তু `.nil` থেকে ডাকা যায় না এখনো |

---

## ৩. যা এখনো কাজ করে না — মূল ফাঁক (gaps)

### ৩.১ `.nil` কোড থেকে ইম্পোর্ট/মডিউল সিস্টেমই নেই
`LANGUAGE_SPEC.md`-এ `import` কীওয়ার্ড তালিকাভুক্ত (লাইন ৩৬), কিন্তু:
- `compiler/ast/ast.go`-তে কোনো `ImportDecl` নোড নেই
- `compiler/parser/parser.go`-তে import পার্স করার কোনো কেস নেই
- `compiler/evaluator/evaluator.go`-র মূল `switch` এ ইম্পোর্ট হ্যান্ডলিং নেই

মানে: `stdlib/net`, `data/orm`, `pkg/alap/entity`, `pkg/alap/server` — এসব Go প্যাকেজ যতই সমৃদ্ধ হোক, কোনো `.nil` স্ক্রিপ্ট এগুলো **ডাকতেই পারবে না**।

### ৩.২ Declarative UI (`component`) বাস্তবে রেন্ডার করে না
README-তে "Alap Declarative UI & 60 FPS Animation" আছে, কিন্তু evaluator-এ:
```go
case *ast.ComponentLiteral:
    return &object.String{Value: fmt.Sprintf("Component<%s>", node.Name.Value)}
```
একটা `component` ব্লক এক্সিকিউট করলে শুধু `"Component<নাম>"` স্ট্রিং ফেরত আসে — কোনো state binding, reconciliation, বা আসল রেন্ডারিং হয় না। GPU রেন্ডারার (`pkg/gpu`), animation engine (`pkg/animation`), state reconciler (`pkg/alap/state/reconciler.go`) — এগুলো Go-তে লেখা আছে, কিন্তু ভাষার সাথে যুক্ত নয়।

### ৩.৩ "সার্ভার" ও "ডাটাবেস" উদাহরণগুলো আসলে সিমুলেশন
`examples/server-service/src/main.nil` এবং `examples/unified-entity/src/main.nil` পড়লে দেখা যায় — এগুলো real HTTP listen বা real SQL execute করে না; শুধু `puts()` দিয়ে "কী হতো" তা বর্ণনা করে (যেমন `puts("➜ GET /api/users/101")` তারপর হার্ডকোড করা ম্যাপ থেকে ভ্যালু বের করে ছাপায়)। `data/orm/orm.go`-র `QueryBuilder.ToSQL()` বাস্তব SQL স্ট্রিং বানাতে পারে, `pkg/alap/entity/entity.go`-র `GenerateSQL()` বাস্তব DDL বানাতে পারে — কিন্তু এগুলো এখনো কোনো actual PostgreSQL/SQLite ড্রাইভারের সাথে সংযুক্ত না (driver/exec কোড কোথাও নেই), আর `.nil` থেকে অ্যাক্সেসযোগ্যও না (৩.১ দেখুন)।

### ৩.৪ দুটো রিপো একই নাম নিয়ে সমান্তরালভাবে এগোচ্ছে
`alap-framework` আর `nilLang` — দুটোই নিজেদের "Alap Framework + NilLang"-এর মূল ঘর দাবি করে, দুটোতেই `cmd/nil`, `cmd/nilc` আছে, কিন্তু ভেতরের কোড আলাদা (যেমন `alap-framework`-এ কাজ করা `net`/`orm` stdlib আছে যা `nilLang`-এ নেই, আবার `nilLang`-এ আছে HIR/MIR/WASM/Oracle যা `alap-framework`-এ নেই)। কোনটা canonical, সেটা repo দুটোতে স্পষ্ট না — এটা প্রথমেই ঠিক করা দরকার, নইলে দুই জায়গায় ডুপ্লিকেট কাজ চলতেই থাকবে।

---

## ৪. `pos-app`-এর সাথে ফারাক — কী কী লাগবে

| `pos-app`-এ যা আছে | NilLang/Alap-এ বর্তমান অবস্থা |
|---|---|
| PostgreSQL + Prisma-স্টাইল কোয়েরি | `orm.QueryBuilder` শুধু SQL স্ট্রিং বানায়, execute করে না; কোনো DB ড্রাইভার বাইন্ডিং নেই |
| ৫৫টা API রুট, auth middleware | `pkg/alap/server`, `pkg/alap/routing` Go-তে বাস্তব, কিন্তু `.nil` থেকে ডিফাইন করা যায় না |
| React/Next.js UI, shadcn চার্ট | `component` কীওয়ার্ড শুধু placeholder স্ট্রিং দেয়; কোনো রেন্ডার-টু-স্ক্রিন পাইপলাইন `.nil` স্তরে নেই |
| Capacitor + SQLite অফলাইন সিঙ্ক | SoftBus (LAN P2P) আছে Go-তে, কিন্তু cloud sync/offline-first স্তর অনুপস্থিত |
| Decimal.js নির্ভুল টাকা হিসাব | NilLang-এর টাইপ সিস্টেমে `Float`/`Int` আছে, arbitrary-precision decimal টাইপ নেই |
| Android/iOS বিল্ড (Capacitor) | `platform/android`, `platform/ios` অ্যাডাপ্টার স্ক্যাফোল্ড আছে (`alap-framework`), বাস্তবে বিল্ড-টেস্টেড কিনা অনিশ্চিত |

---

## ৫. প্রস্তুত হওয়ার জন্য ধাপে ধাপে নীল নকশা

### Phase 0 — একটাকে canonical ধরুন
দুই রিপো একসাথে না রেখে, `nilLang`-কে (এটাই বেশি এগিয়ে — HIR/MIR/WASM/VM/LSP আছে) মূল রিপো ধরে `alap-framework`-এর কাজের অংশগুলো (`stdlib/net`, `data/orm`, প্রুভেন auth.go) সেখানে migrate করুন। নইলে দুই জায়গায় সমান্তরাল, বিরোধপূর্ণ development চলবে।

### Phase 1 — Import/Module সিস্টেম বাস্তবায়ন
এটাই সবচেয়ে জরুরি ব্লকার। দরকার:
1. `ast.ImportDecl` নোড + parser কেস
2. Evaluator/Compiler-এ native Go প্যাকেজ রেজিস্ট্রি (যেমন `registerNative("net", netPkg)`) যাতে `import "net"` করলে Go-তে লেখা `stdlib/net` এক্সপোজ হয়
3. `.nil` থেকে ব্যবহারযোগ্য namespace syntax: `net.get(url)`, `db.query(sql, args)`

### Phase 2 — Data স্তর বাস্তব করুন
- `data/orm/orm.go`-র `QueryBuilder`-কে বাস্তব ড্রাইভারের (`database/sql` + `lib/pq`/`pgx`, বা SQLite জন্য `mattn/go-sqlite3`) সাথে যুক্ত করুন — `Exec()`/`Query()` মেথড যোগ করুন যা সত্যিই DB-তে যায়
- `pkg/alap/entity/entity.go`-র `GenerateSQL()`-এর পাশে migration runner (alap-framework-এ `data/migration/migration.go` আছে, সেটা কাজে লাগান) যোগ করুন
- Decimal/Money টাইপ (টাকার হিসাবের জন্য, `pos-app`-এ যেমন `decimal.js`)

### Phase 3 — HTTP সার্ভার স্তর বাস্তব করুন
`pkg/alap/server/server.go` + `pkg/alap/routing/routing.go` ইতিমধ্যে বেশ পূর্ণাঙ্গ (middleware, rate-limit, cache) — এগুলোকে `net/http.ListenAndServe`-এর সাথে সত্যিই বাইন্ড করে দিন, আর `import` সিস্টেম দিয়ে `.nil` থেকে route ডিফাইন করার সিনট্যাক্স দিন:
```nil
import "server";

let app = server.new("pos-api");
app.get("/products", fn(req) { return db.query("SELECT * FROM products"); });
app.listen(3000);
```

### Phase 4 — Declarative UI সত্যিকার করুন
`ComponentLiteral` evaluate করে placeholder string দেওয়ার বদলে:
- বাস্তব component tree বানান (props/state/children সহ)
- `pkg/alap/state/reconciler.go`-র সাথে যুক্ত করুন যাতে `state` পরিবর্তনে `render` আবার চলে
- কমপক্ষে একটা target-এ (Linux desktop বা web/WASM, যেহেতু WASM ব্যাকএন্ড এখন আছে) সত্যিকার পিক্সেলে আঁকুন

### Phase 5 — POS ডোমেইন মডেল ডিজাইন (এই ধাপে এসে আসল অ্যাপ শুরু)
`lakhan-bhandar-pos`-এর স্কিমা থেকে entity গুলো `pkg/alap/entity` সিনট্যাক্সে (Phase 1-4 হয়ে গেলে) এভাবে লিখতে পারবেন:
```nil
entity Product {
    id: UUID primary,
    name: String required,
    sku: String unique,
    costPrice: Decimal,
    sellPrice: Decimal,
    stock: Int
}

entity Sale {
    id: UUID primary,
    customer: Customer relation,
    items: [SaleItem],
    totalAmount: Decimal,
    amountPaid: Decimal,
    dueAmount: Decimal,
    createdAt: Date
}

entity SaleItem {
    id: UUID primary,
    sale: Sale relation,
    product: Product relation,
    qty: Int,
    costPriceAtSale: Decimal   // pos-app-এর মতো WAC স্ন্যাপশট
}
```
এটা থেকে auto: DDL, CRUD REST route, TypeScript-স্টাইল ক্লায়েন্ট মডেল — যেটা `unified-entity` উদাহরণ ইতিমধ্যে *কল্পনা* করে দেখিয়েছে, শুধু বাস্তবায়িত করা বাকি।

### Phase 6 — অফলাইন-ফার্স্ট সিঙ্ক ও প্যাকেজিং
- `pkg/softbus` LAN discovery ইতিমধ্যে আছে — local network-এ multi-counter sync-এর জন্য ভিত্তি হতে পারে
- cloud sync (Supabase-এর বদলে নিজস্ব `nilpkg-server`-স্টাইল রেজিস্ট্রি, অথবা সহজ REST push/pull) যোগ করা লাগবে
- Android বিল্ড টার্গেট (`platform/android`, `pkg/mobile/android`) দিয়ে বাস্তব APK বানিয়ে টেস্ট করুন — এখনো "বিল্ড-টেস্টেড" প্রমাণ পাইনি

### Phase 7 — প্যারিটি চেকলিস্ট
`pos-app`-এর ইতিমধ্যে সমাধান হওয়া জিনিসগুলোর (৫৫ API রুট auth কভারেজ, ৮৯/২৮৯ টেস্ট পাস, GST/tax gap, return/refund flow, atomic UNNEST batch) বিপরীতে NilLang সংস্করণকে ধাপে ধাপে মেলান — নাহলে ফিচার-প্যারিটি ছাড়াই মাইগ্রেশনের ঝুঁকি থাকবে।


**না — এখনই `pos-app` স্তরের প্রোডাকশন POS বানানোর জন্য প্রস্তুত নয়।** ভাষা ও টুলচেইন ডোমেইন লজিক ডেমোর জন্য যথেষ্ট; দোকানের অফলাইন-ফার্স্ট, লেজার, প্রিন্ট, বারকোড POS-এর জন্য নয়।

আজকের [pos-app-nilLang](https://github.com/joysriramsarkar/pos-app-nilLang) সেটাই দেখায়: মডেল/সার্ভিস ফাইল আছে, কিন্তু `main.nil` সেগুলো ইমপোর্ট করে না — হার্ডকোডেড `puts()` ডেমো।

---

## সোজা উত্তর

| স্তর | অবস্থা | `pos-app` তুলনায় |
|---|---|---|
| NilLang কোর (`let`/`fn`/অ্যারে/হ্যাশ/REPL/VM) | কাজ করে | যথেষ্ট |
| Alap UI (Go উইজেট ট্রি) | স্ক্যাফোল্ড | ১০টা প্রিমিটিভ; লিস্ট/ডায়ালগ/ট্যাব নেই |
| ডেটা | স্টব | SQL স্ট্রিং বিল্ডার, আসল SQLite/Postgres ড্রাইভার নেই |
| টাকা | ভাসমান বিন্দু | `decimal.js` নেই; `round2` আসলে নো-অপ |
| অফলাইন সিঙ্ক | নেই | IndexedDB + আইডেম্পোটেন্সি কিউ নেই |
| প্রিন্ট/PDF/বারকোড/ক্যামেরা | নেই | Capacitor স্তর নেই |
| RBAC/অথ | Go HMAC JWT | Nilang থেকে কলযোগ্য নয়; পাসওয়ার্ড চেক হয় না |
| i18n | নেই | `next-intl` সমতুল্য নেই |

স্পেক বলে *One Way To Do It*, কিন্তু রিপোতে দুই ব্যাকরণ চলছে:

- [nilLang](https://github.com/joysriramsarkar/nilLang): `let x = 1;`, `fn()`, `puts()`
- [alap-framework](https://github.com/joysriramsarkar/alap-framework) উদাহরণ: `function`, `struct`, `component`, `.toString()`

POS পোর্টের আগে **এক ক্যানোনিকাল সিনট্যাক্স** লক করতে হবে। নিচের নকশা [LANGUAGE_SPEC](https://github.com/joysriramsarkar/nilLang/blob/main/docs/spec/LANGUAGE_SPEC.md) অনুসরণ করে: `let`/`fn`/`struct`/`component`।

---

## কেন `pos-app` এখন পোর্ট হয় না

[pos-app](https://github.com/joysriramsarkar/pos-app) যে জিনিসগুলোর ওপর দাঁড়ায়, সেগুলো Alap/Nilang-এ এখন নেই:

1. **Decimal টাকা** — Prisma `Decimal` + `decimal.js`। IEEE float দিয়ে ৳১.১ + ৳২.২ ভাঙবে।
2. **অ্যাটমিক স্টক + লেজার** — `UNNEST` + `WHERE current_stock >= qty`। ORM `Tx` শুধু ফ্ল্যাগ ফ্লিপ করে।
3. **অফলাইন কিউ** — IndexedDB, আইডেম্পোটেন্সি কি, `/api/sync`।
4. **মাল্টি-ট্যাব কার্ট** — Zustand `processingTabIds`।
5. **থার্মাল/A4 প্রিন্ট + PDF শেয়ার**।
6. **বারকোড** — কীবোর্ড-ওয়েজ + ML Kit।
7. **EN/BN UI + রসিদ ভাষা আলাদা**।
8. **WAC খরচ স্ন্যাপশট** লাভ রিপোর্টের জন্য।

`alap/data` এখন CSV/রিগ্রেশন — দোকানের DB নয়। `alap/entity` DDL/REST স্পেক জেনারেট করে, রো পড়ায় না।

---

## নীল নকশা — POS কে চার স্তরে ভাঙো

```text
┌─────────────────────────────────────────────────────────┐
│  pos-shell (Alap UI)                                    │
│  Cart · Catalog · Parties · Reports · Settings          │
├─────────────────────────────────────────────────────────┤
│  pos-domain (খাঁটি Nilang, প্ল্যাটফর্মহীন)               │
│  Money · Cart · Sale · Ledger · Stock · RBAC            │
├─────────────────────────────────────────────────────────┤
│  pos-ports (ইন্টারফেস)                                  │
│  Store · Clock · Id · Printer · Scanner · Sync          │
├─────────────────────────────────────────────────────────┤
│  pos-adapters                                           │
│  SQLite/Onuron · SoftBus · Camera · Thermal · FileKV    │
└─────────────────────────────────────────────────────────┘
```

নিয়ম: **ডোমেইন কোনো `puts`, ফাইল, HTTP, উইজেট ছুঁয়বে না।** আজকের `main.nil` সেই নিয়ম ভাঙে।

### টার্গেট ট্রি

```text
pos-app-nilLang/
├── nil.json
├── resources/i18n/{bn,en}.json
├── src/
│   ├── main.nil
│   ├── app.nil
│   ├── domain/
│   │   ├── money.nil
│   │   ├── ids.nil
│   │   ├── product.nil
│   │   ├── party.nil
│   │   ├── cart.nil
│   │   ├── sale.nil
│   │   ├── ledger.nil
│   │   ├── stock.nil
│   │   └── rbac.nil
│   ├── ports/
│   │   ├── store.nil
│   │   ├── printer.nil
│   │   ├── scanner.nil
│   │   └── sync.nil
│   ├── adapters/
│   │   ├── memory_store.nil    # ফেজ ১
│   │   ├── sqlite_store.nil    # ফেজ ২
│   │   └── file_queue.nil      # ফেজ ৪
│   ├── services/
│   │   ├── checkout.nil
│   │   ├── inventory.nil
│   │   ├── parties.nil
│   │   └── reports.nil
│   └── ui/
│       ├── shell.nil
│       ├── pos_page.nil
│       ├── cart_panel.nil
│       ├── catalog_grid.nil
│       ├── pay_sheet.nil
│       └── receipt.nil
└── tests/
    ├── money_test.nil
    ├── checkout_test.nil
    └── ledger_test.nil
```

`nil.json` এখন `Database` ক্যাপাবিলিটি ঘোষণা করে, ইমপ্লিমেন্ট করে না। ফেজ ২ পর্যন্ত `Filesystem` + ইন-মেমোরি স্টোর রাখো।

---

## ফেজ ০ — ভাষা/ফ্রেমওয়ার্ক গেট (POS-এর আগে)

এগুলো না হলে পোর্ট থামবে:

| গেট | কেন |
|---|---|
| **এক সিনট্যাক্স** | দুই রিপো মিলিয়ে `import`/`export` চালু |
| **মডিউল লোডার** | `main.nil` যেন `src/domain/money.nil` টানে |
| **`Money` টাইপ** | ক্ষুদ্রতম একক (পয়সা) `i64`; ফ্লোট নিষেধ |
| **`Result<T,E>` রানটাইমে** | স্পেক আছে, ইভ্যালুয়েটরে নেই |
| **SQLite FFI** | `alap/db` আসল কানেকশন + ট্রানজ্যাকশন |
| **List / Dialog / TabBar** | POS শেলের ন্যূনতম উইজেট |
| **টেস্ট রানার** | `nil test` ডোমেইন অ্যাসার্ট চালায় |

`money.nil`-এর বর্তমান `round2`:

```nil
return (val * factor) / factor;  // কিছু করে না
```

এটা দিয়ে বিল কাটা যাবে না।

---

## ফেজ ১ — ডোমেইন কোর (এখনই লেখা যায়)

ইন-মেমোরি, UI ছাড়া। লক্ষ্য: `nil run` এ একটা চেকআউট প্রুফ, কিন্তু **ফাংশনগুলো টেস্টযোগ্য**।

### ১. টাকা — পয়সায় `i64`

```nil
// src/domain/money.nil
struct Money {
    minor: Int  // ৳12.50 → 1250
}

let zero = fn() {
    return { "minor": 0 };
};

let ofMinor = fn(n) {
    return { "minor": n };
};

let ofMajor = fn(major, minorPart) {
    return { "minor": (major * 100) + minorPart };
};

let add = fn(a, b) {
    return { "minor": a["minor"] + b["minor"] };
};

let sub = fn(a, b) {
    return { "minor": a["minor"] - b["minor"] };
};

let mulQty = fn(unit, qtyMinor) {
    // qtyMinor: 2.5 কেজি → 2500 যদি qtyScale=1000
    return { "minor": (unit["minor"] * qtyMinor) / 1000 };
};

let format = fn(m, symbol) {
    let n = m["minor"];
    let neg = n < 0;
    if (neg) { let n = 0 - n; }
    let maj = n / 100;
    let min = n % 100;
    let pad = min;
    if (min < 10) { let pad = "0" + min; }
    let s = symbol + str(maj) + "." + pad;
    if (neg) { return "-" + s; }
    return s;
};
```

কোনো `Float` নেই। পরিমাণ (`qty`) আলাদা স্কেল: পিস = ১, কেজি = ১০০০।

### ২. কার্ট + ভ্যালিডেশন

```nil
// src/domain/cart.nil
let addLine = fn(cart, product, qty) {
    if (product["active"] == false) {
        return { "ok": false, "err": "INACTIVE_PRODUCT" };
    }
    if (qty <= 0) {
        return { "ok": false, "err": "BAD_QTY" };
    }
    let line = {
        "productId": product["id"],
        "name": product["name"],
        "unitPrice": product["selling"],
        "costAtSale": product["wac"],
        "qty": qty,
        "lineTotal": mulQty(product["selling"], qty)
    };
    return { "ok": true, "cart": push(cart, line) };
};

let totals = fn(lines, discount, tax) {
    let sub = zero();
    let i = 0;
    while (i < len(lines)) {
        let sub = add(sub, lines[i]["lineTotal"]);
        let i = i + 1;
    }
    let afterDisc = sub(sub, discount);
    let grand = add(afterDisc, tax);
    return {
        "subtotal": sub,
        "discount": discount,
        "tax": tax,
        "grand": grand
    };
};
```

### ৩. চেকআউট — এক ট্রানজ্যাকশন, তিন সাইড ইফেক্ট

`pos-app` যা করে, হুবহু সেই ক্রম:

1. স্টক যথেষ্ট কি না (`current >= qty`)
2. সেল + সেল-আইটেম লিখো (দাম/খরচ স্ন্যাপশট)
3. স্টক কাটো + `StockHistory`
4. কাস্টমার লেজার: due / prepaid / cash+UPI স্প্লিট
5. অডিট লগ

```nil
// src/services/checkout.nil
let checkout = fn(store, clock, ids, cmd) {
    // cmd: { items, customerId, pay: {cash, upi, prepaid, due}, discount, tax, cashierId }
    let stockErr = store["assertStock"](cmd["items"]);
    if (stockErr != null) {
        return { "ok": false, "err": stockErr };
    }

    let t = totals(cmd["items"], cmd["discount"], cmd["tax"]);
    let paid = add(add(cmd["pay"]["cash"], cmd["pay"]["upi"]), cmd["pay"]["prepaid"]);
    let due = sub(t["grand"], paid);
    if (due["minor"] < 0) {
        // উদ্বৃত্ত → প্রিপেইড বা খুচরা
        let change = { "minor": 0 - due["minor"] };
        let due = zero();
    }

    let saleId = ids["sale"]();
    let invoice = ids["invoice"](clock["now"]());

    let tx = store["begin"]();
    tx["insertSale"](saleId, invoice, cmd, t, paid, due);
    tx["deductStock"](cmd["items"], saleId);
    tx["applyLedger"](cmd["customerId"], due, cmd["pay"]["prepaid"], saleId);
    tx["audit"]("SALE_CREATE", saleId, cmd["cashierId"]);
    let committed = tx["commit"]();
    if (committed["ok"] == false) {
        return { "ok": false, "err": committed["err"] };
    }
    return { "ok": true, "saleId": saleId, "invoice": invoice, "change": change };
};
```

ফেজ ১-এ `store` = মেমোরি ম্যাপ। সিগনেচার ফেজ ২-এ বদলাবে না।

### ৪. লেজার নিয়ম (অপরিবর্তনীয়)

| ঘটনা | due | prepaid |
|---|---|---|
| বাকি সেল | +grand−paid | ০ |
| প্রিপেইড দিয়ে সেল | ০ | −paid |
| বাকি আদায় | −amount | ০ |
| প্রিপেইড টপআপ | ০ | +amount |
| প্রিপেইড উত্তোলন | ০ | −amount |
| খুচরা প্রিপেইডে | ০ | +change |

প্রতিটি এন্ট্রিতে `balanceAfter`। রানিং টোটাল এন্ট্রি ছাড়া আপডেট নয়।

### ৫. WAC স্টক

কেনায়:

```text
newWac = (oldQty*oldWac + buyQty*buyPrice) / (oldQty+buyQty)
```

সব হিসাব `Money.minor` ও `qtyScale`-এ। সেলের সময় `costAtSale = wac` স্ন্যাপশট — পরে কেনার দাম বদলালে পুরনো লাভ নষ্ট হবে না। `pos-app` এটা করে; বর্তমান Nilang পোর্ট করে না।

### ৬. RBAC — পারমিশন কোড, রোল নয়

`pos-app` এর মতো কোড: `sales.create`, `products.update`, `reports.view`। সার্ভিসের প্রথম লাইন:

```nil
let requirePerm = fn(user, code) {
    if (hasPermission(user["permissions"], code) == false) {
        return { "ok": false, "err": "FORBIDDEN" };
    }
    return { "ok": true };
};
```

`authenticateUser` পাসওয়ার্ড ছাড়ে — ফেজ ২-এ `alap` `HashPassword`/`CheckPassword` Nilang-এ এক্সপোজ করতে হবে।

---

## ফেজ ২ — পার্সিস্টেন্স

`alap/entity` দিয়ে স্কিমা জেনারেট করো, তারপর SQLite/Onuron স্টোরে চালাও। Prisma মডেলগুলো ১:১ নামে রাখো যাতে পরে ডেটা মাইগ্রেট হয়:

`Product`, `Category`, `StockHistory`, `Customer`, `LedgerEntry`, `Sale`, `SaleItem`, `SaleReturn`, `Supplier`, `Purchase`, `PurchaseItem`, `SyncQueue`, `User`, `Permission`, `RolePermission`, `Expense`, `AuditLog`, `Setting`

অর্থ কলাম সব `INTEGER` (minor units)। `FLOAT`/`DOUBLE` নিষেধ — `pos-app`-এর `cleanup-floating-point-errors.sql` সেই শিক্ষা।

স্টোর পোর্ট:

```nil
struct Store {
    begin: fn() -> Tx
}

struct Tx {
    insertSale: fn(...)
    deductStock: fn(...)   // WHERE stock >= qty
    applyLedger: fn(...)
    commit: fn() -> Result
    rollback: fn()
}
```

কমিট ব্যর্থ হলে পুরো সেল উধাও — আংশিক স্টক কাটা যাবে না।

---

## ফেজ ৩ — Alap POS শেল

উইজেট গেট না হওয়া পর্যন্ত `nil render` প্রিভিউ। শেল লেআউট `pos-app` `src/app/pos/` থেকে:

```text
┌──────────────┬─────────────────────┬──────────────────┐
│ Sidebar      │ Catalog + search    │ Cart tabs        │
│ POS          │ barcode input       │ lines            │
│ Stock        │ category chips      │ totals           │
│ Parties      │ product tiles       │ Pay / Hold / New │
│ Reports      │                     │                  │
│ Settings     │                     │                  │
└──────────────┴─────────────────────┴──────────────────┘
```

```nil
component PosPage {
    state query: String = ""
    state tabId: String = "t1"
    state cart: Hash = {}

    build() {
        Row {
            NavRail()
            CatalogPane(query, onScan, onAdd)
            CartPanel(tabId, cart, onPay)
        }
    }
}
```

যে উইজেট এখন আছে: `Column`, `Row`, `Text`, `Button`, `TextInput`, `Image`, `Card`, `Divider`, `Spacer`, `Switch`। POS-এর জন্য যোগ করতেই হবে: **`List`/`LazyList`, `Dialog`/`Sheet`, `TabBar`, `ScrollView`**। ভার্চুয়ালাইজড স্টক লিস্ট (`react-virtuoso` সমতুল্য) ছাড়া বড় ক্যাটালগ ধীর হবে।

পেমেন্ট শিট: Cash / UPI / Mixed / Due / Prepaid — `pos-app` এর স্প্লিট হুবহু।

---

## ফেজ ৪ — ডিভাইস ও অফলাইন

`pos-app` যে পোর্টগুলো আশা করে:

| পোর্ট | Onuron/Alap ম্যাপিং | নোট |
|---|---|---|
| `Printer` | ৫৮/৮০মিমি + A4 লেআউট → সিস্টেম প্রিন্ট | আগে টেক্সট রসিদ, পরে PDF |
| `Scanner` | কীবোর্ড-ওয়েজ + ক্যামেরা ক্যাপাবিলিটি | ফেজ ৩-এ শুধু টেক্সট ইনপুট |
| `Clock`/`Id` | ইনভয়েস সিরিয়াল, CUID | অফলাইনে লোকাল জেনারেট |
| `Sync` | SoftBus বা HTTP | `SyncQueue` + idempotency key |
| `I18n` | `t("pay.cash")` | UI ভাষা ≠ রসিদ ভাষা |
| `Settings` | কারেন্সি সিম্বল সেটিংস থেকে | `৳` হার্ডকোড নয় |

অফলাইন কিউ: প্রতিটি মিউটেশন `{ idempotencyKey, entityType, payload }`। রিপ্লে ইডেম্পোটেন্ট। আর্থিক আপডেট লেজার-ইনক্রিমেন্টাল — `pos-app` `src/lib/offline/ARCHITECTURE.md` কপি করো, নতুন প্রোটোকল উদ্ভাবন নয়।

---

## ফেজ ৫ — প্যারিটি চেকলিস্ট

`pos-app` README-এর ফিচার, পোর্ট শেষ না হওয়া পর্যন্ত:

- [ ] অফলাইন-ফার্স্ট সিঙ্ক
- [ ] EN/BN UI + আলাদা রসিদ ভাষা + বাংলা অঙ্ক
- [ ] বারকোড (কীবোর্ড + ক্যামেরা)
- [ ] মাল্টি-ট্যাব কার্ট, ট্যাবপ্রতি চেকআউট লক
- [ ] আংশিক/বাকি, প্রিপেইড, Cash+UPI, খুচরা→প্রিপেইড
- [ ] থার্মাল + A4/A5, শেয়ার সবসময় PDF
- [ ] WAC, বাল্ক স্টক, স্টক হিস্টরি
- [ ] কাস্টমার/সাপ্লায়ার লেজার, বাকি আদায়, প্রিপেইড উত্তোলন
- [ ] ড্যাশবোর্ড, ক্যাশ/UPI মিল, খরচ
- [ ] অডিট লগ
- [ ] ADMIN / MANAGER / CASHIER / VIEWER প্রতি API/সার্ভিসে

হ্যাঁ। এবার আমি এটাকে **একটি বাস্তব engineering plan** হিসেবে দিচ্ছি—যাতে আপনি চাইলে সরাসরি `alap-framework`, `nilLang` এবং নতুন `pos-nil` repository ধরে কাজ শুরু করতে পারেন।

একটা সতর্কতা আগে: নিচের NilLang code-গুলো **target API/design**, বর্তমান compiler-এ প্রতিটি syntax হুবহু এখনই compile করবে—এ দাবি করছি না। কারণ বর্তমান repository-তে architecture অনেকখানি এগোলেও POS-level framework API এখনো পুরোপুরি বাস্তবায়িত নয়। বিশেষ করে ORM এখনও query-builder/transaction skeleton পর্যায়ে।

---

# ১. আমরা শেষ পর্যন্ত কী বানাব

লক্ষ্য:

```text
                           POS APP
                              │
                    লিখবে শুধু NilLang-এ
                              │
                              ▼
                        Alap Framework
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
       UI                   Domain                 Data
        │                     │                     │
   Components             Services              Repositories
   State                  Money                 Local DB
   Router                 Validation            Remote DB
   Forms                  Permissions           Sync
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │
                        Alap Runtime
                              │
                    Platform Abstraction
                              │
             ┌────────────────┼─────────────────┐
             │                │                 │
          Android           Linux            Onuron
             │
       Native APIs
```

একটি POS developer-এর ideally এই code লিখলেই যথেষ্ট হওয়া উচিত:

```text
let cart = POS.createCart()

cart.add(product)
cart.add(product2)

let result = await cart.checkout(
    payment: Cash(500)
)

print(result.receipt)
```

ভিতরে Android, Linux বা Onuron-এর native implementation কী হচ্ছে সেটা application developer জানবে না।

---

# ২. তিনটি repository-এর দায়িত্ব আলাদা করুন

এখানে boundary পরিষ্কার না করলে ecosystem জটিল হয়ে যাবে।

## `nilLang`

এখানে থাকবে:

```text
lexer
parser
AST
type system
type checker
HIR
MIR
code generation
VM
language tooling
standard language semantics
```

এটি হবে **language**।

---

## `alap-framework`

এখানে থাকবে:

```text
UI
runtime
data
network
storage
router
auth
permissions
platform abstraction
barcode
camera
print
pdf
sync
package manager
build system
```

এটি হবে **application framework**।

বর্তমান repository structure ইতিমধ্যে compiler/runtime/ui/platform/stdlib/data/package-manager আলাদা করে রেখেছে, তাই এই separation-এর ভিত্তি আছে।

---

## `pos-nil`

এখানে শুধু:

```text
models
business logic
pages
components
reports
configuration
```

অর্থাৎ POS framework-এর consumer হবে।

---

# ৩. প্রথমে `alap-framework`-এর নতুন architecture

বর্তমান structure-কে পুরো ভাঙবেন না।

বরং:

```text
alap-framework/
│
├── abi/
├── cmd/
├── compiler/
├── data/
├── pkg/
├── platform/
├── runtime/
├── stdlib/
├── ui/
│
├── framework/
│   ├── app/
│   ├── auth/
│   ├── barcode/
│   ├── forms/
│   ├── http/
│   ├── money/
│   ├── pdf/
│   ├── permissions/
│   ├── print/
│   ├── router/
│   ├── storage/
│   ├── sync/
│   └── validation/
│
└── sdk/
    ├── web/
    ├── android/
    ├── linux/
    ├── onuron/
    └── ios/
```

### মূল principle

`framework/*` হবে **developer-facing API**।

`runtime/*`, `platform/*`, `ui/*` হবে **implementation**।

---

# ৪. Dependency direction

এখানে dependency direction খুব কঠোর রাখবেন।

```text
Application
   ↓
Alap SDK
   ↓
Framework API
   ↓
Runtime
   ↓
Platform Adapter
   ↓
OS
```

কখনো:

```text
POS → Android Kotlin API
```

হবে না।

বরং:

```text
POS → alap.barcode
                 ↓
              Android
```

---

# ৫. `alap.core`

প্রথমে foundational types ঠিক করুন।

নতুন:

```text
framework/core/
```

অথবা stdlib-এর সঙ্গে carefully ভাগ করে:

```text
stdlib/
framework/
```

আমি আলাদা রাখব।

কারণ:

```text
stdlib = language/runtime primitives
framework = application primitives
```

---

# ৬. Result system

POS-এর জন্য exception-driven application architecture করবেন না।

NilLang-এর intended design অনুযায়ী `Result<T,E>` হওয়া উচিত। Blueprint-এও এই model আছে।

Target:

```text
type Result<T, E> =
    | Ok(T)
    | Err(E)
```

ব্যবহার:

```text
let result: Result<Sale, SaleError> =
    await pos.checkout(request)
```

---

# ৭. `Option`

```text
type Option<T> =
    | Some(T)
    | None
```

যেমন:

```text
let customer: Customer? = ...
```

---

# ৮. Money subsystem

নতুন:

```text
framework/money/
├── money.nil
├── decimal.nil
├── currency.nil
├── tax.nil
└── rounding.nil
```

API:

```text
type Currency = {
    code: string
    exponent: i32
}
```

```text
type Money = {
    amount: Decimal
    currency: Currency
}
```

Factory:

```text
money("100.50", "INR")
```

Operations:

```text
let a = money("100", "INR")
let b = money("20.50", "INR")

let total = a + b
```

Strict rule:

```text
Money + Money
```

শুধু একই currency হলে valid।

---

# ৯. Decimal

POS-এর হিসাব:

```text
price × quantity
```

এখানে floating point error চলবে না।

তাই:

```text
Decimal
```

হবে arbitrary বা fixed precision implementation।

NilLang compiler এই type-কে special native type হিসেবে optimize করতে পারে।

---

# ১০. Database architecture-এর নতুন design

বর্তমান:

```text
data/
    migration/
    orm/
```

আছে।

এটাকে করুন:

```text
data/
├── driver/
│   ├── postgres/
│   ├── sqlite/
│   ├── mysql/
│   └── memory/
│
├── orm/
│   ├── model.go
│   ├── query.go
│   ├── insert.go
│   ├── update.go
│   ├── delete.go
│   ├── transaction.go
│   └── relation.go
│
├── migration/
├── schema/
└── pool/
```

---

# ১১. Database abstraction

NilLang side:

```text
database ShopDB {
    driver = "postgres"
}
```

or:

```text
database LocalDB {
    driver = "sqlite"
}
```

---

# ১২. Entity system

Target syntax:

```text
entity Product {

    id: UUID

    name: string
    sku: string
    barcode: string?

    price: Money
    cost: Money

    stock: Decimal

    active: bool

    createdAt: DateTime
    updatedAt: DateTime
}
```

Compiler এখানে metadata generate করবে।

যেমন:

```text
Product.__schema
Product.__table
Product.__fields
```

---

# ১৩. কেন compiler-integrated entity ভালো

এতে:

```text
db.Product.where(...)
```

runtime reflection-এর ওপর পুরোপুরি নির্ভর করতে হবে না।

Compiler জানবে:

```text
Product.name → string
Product.price → Money
Product.stock → Decimal
```

ফলে:

```text
product.prcie
```

লিখলে compile-time error।

---

# ১৪. Query DSL

Target:

```text
let products =
    await db.products
        .where(p => p.active == true)
        .where(p => p.stock > 0)
        .orderBy(p => p.name)
        .all()
```

Compiler ideally এটাকে lower করবে:

```text
Typed Query AST
        ↓
SQL Query IR
        ↓
Driver
```

---

# ১৫. SQL injection বন্ধ করার architecture

এইটা খুব গুরুত্বপূর্ণ।

Application developer:

```text
where(p => p.name == search)
```

লিখবে।

Compiler/runtime internally করবে:

```sql
WHERE name = $1
```

আর:

```text
search
```

হবে bound parameter।

String concatenation-based SQL generation নিষিদ্ধ করুন।

---

# ১৬. Transaction

এইটা নতুনভাবে implement করতে হবে।

বর্তমান `Tx.Commit()` বাস্তব database transaction করছে না; skeleton implementation মাত্র।

Target API:

```text
await db.transaction(async tx => {

    let sale =
        await tx.sales.create(...)

    await tx.stock.decrease(...)

    await tx.payments.create(...)

})
```

Internal:

```text
BEGIN
↓
statement
↓
statement
↓
statement
↓
COMMIT
```

Failure:

```text
ROLLBACK
```

---

# ১৭. Repository abstraction

Application layer যেন ORM জানে না।

```text
interface ProductRepository {

    get(id: UUID): Future<Product?>

    findByBarcode(
        barcode: string
    ): Future<Product?>

    search(
        query: string
    ): Future<Product[]>
}
```

Implementation:

```text
PostgresProductRepository
LocalProductRepository
CachedProductRepository
```

---

# ১৮. Local database

POS-এর জন্য local database mandatory।

Android:

```text
SQLite
```

Linux:

```text
SQLite
```

Onuron:

```text
SQLite
```

Web:

```text
IndexedDB
```

তবে application API একই থাকবে:

```text
alap.local.db
```

---

# ১৯. Offline architecture

এখানে একটা গুরুত্বপূর্ণ design decision:

**offline cache আর offline business database এক জিনিস নয়।**

POS-এর local database-এ প্রয়োজনীয় data-এর একটি working set থাকতে হবে।

```text
Remote PostgreSQL
       │
       │ sync
       ▼
Local SQLite / IndexedDB
       │
       ▼
POS application
```

---

# ২০. Sync subsystem

নতুন:

```text
framework/sync/
├── operation.nil
├── queue.nil
├── engine.nil
├── conflict.nil
├── cursor.nil
└── protocol.nil
```

প্রতিটি mutation:

```text
SyncOperation {
    id: UUID
    mutationId: UUID

    entity: string
    action: string

    payload: JSON

    createdAt: DateTime
    status: SyncStatus
}
```

---

# ২১. Sync API

Application:

```text
await sync.enqueue(
    mutation
)
```

Framework:

```text
offline:
    local commit
    queue mutation

online:
    local commit
    send mutation

reconnect:
    replay queue
```

---

# ২২. Idempotency

প্রতিটি sale-এ:

```text
mutationId
```

থাকবে।

উদাহরণ:

```text
mutationId =
    "018ef..."
```

Server database-এ:

```text
UNIQUE(mutationId)
```

দেবেন।

একই request দ্বিতীয়বার এলে existing result ফিরবে।

---

# ২৩. Conflict strategy

POS-এর জন্য generic:

```text
last-write-wins
```

সব জায়গায় ব্যবহার করবেন না।

কারণ:

```text
stock
ledger
payment
```

এগুলোর conflict আলাদা।

### Product

```text
last-write-wins
```

### Stock

```text
transactional / reject-and-reconcile
```

### Ledger

```text
append-only
```

### Sale

```text
immutable
```

এটাই অনেক বেশি নিরাপদ।

---

# ২৪. Ledger system

POS-এর financial data overwrite করা যাবে না।

নতুন:

```text
framework/ledger/
```

ধারণা:

```text
LedgerEntry {
    id
    account
    amount
    direction
    referenceType
    referenceId
    createdAt
}
```

Sale:

```text
Sale
 ↓
LedgerEntries
```

Due:

```text
Due
 ↓
LedgerEntries
```

Prepayment:

```text
Prepayment
 ↓
LedgerEntries
```

---

# ২৫. Stock system

Stock-এর জন্য:

```text
StockMovement
```

কে source of truth করুন।

```text
Purchase +100
Sale -2
Return +1
Damage -3
Adjustment +5
```

তখন:

```text
current stock
```

হিসাবযোগ্য।

আর POS-এর মতো WAC costing-এর জন্য stock layer-এ cost snapshot রাখা যাবে। `pos-app`-এ WAC এবং sale-time cost snapshot ব্যবহৃত হচ্ছে।

---

# ২৬. Auth subsystem

```text
framework/auth/
├── auth.nil
├── session.nil
├── identity.nil
└── token.nil
```

API:

```text
let user = await auth.login(
    username,
    password
)
```

তারপর:

```text
auth.currentUser()
```

---

# ২৭. Permission system

```text
permission Sales.Create
permission Sales.Void

permission Inventory.Read
permission Inventory.Update

permission Reports.View
```

Function:

```text
@requires(Sales.Create)
async function checkout(...) {
    ...
}
```

Server side-এও permission enforce হবে।

শুধু UI-তে button hide করলেই security হবে না।

`pos-app`-এর বর্তমান architecture-ও প্রতিটি API route-এ permission checking করে।

---

# ২৮. Audit system

```text
audit.log({
    action: "sale.created",
    actor: user.id,
    entity: sale.id
})
```

আর framework critical mutation-এর জন্য automatic hooks দিতে পারে:

```text
beforeCreate
afterCreate
beforeUpdate
afterUpdate
```

---

# ২৯. Router

নতুন:

```text
framework/router/
```

API:

```text
router {

    route("/", DashboardPage)

    route("/pos", POSPage)

    route("/inventory", InventoryPage)

    route("/customers", CustomerPage)

    route("/reports", ReportsPage)
}
```

Guard:

```text
protected("/reports")
```

Permission guard:

```text
protected(
    "/inventory",
    permission: Inventory.Read
)
```

---

# ৩০. Forms

POS-এর জন্য:

```text
framework/forms/
```

Target:

```text
form ProductForm {

    field name: string {
        required = true
        minLength = 2
    }

    field sku: string {
        required = true
    }

    field price: Money {
        required = true
        min = money("0", "INR")
    }
}
```

Submit:

```text
let result = await form.submit()
```

---

# ৩১. Barcode abstraction

নতুন:

```text
framework/barcode/
```

Application API:

```text
barcode.scan()
```

Continuous scanning:

```text
scanner.start()

scanner.onDetected {
    code =>
        store.addBarcode(code)
}
```

Platform implementations:

```text
Android → ML Kit / camera
iOS → native camera
Linux → keyboard wedge / HID
Onuron → native camera service
```

`pos-app` ইতিমধ্যে Android-এ ML Kit এবং desktop-এ keyboard-wedge scanner ব্যবহার করছে।

---

# ৩২. Printer abstraction

```text
framework/print/
```

Interface:

```text
interface Printer {

    print(
        document: PrintDocument
    ): Future<Result<void, PrintError>>
}
```

Target:

```text
printer.print(
    receipt,
    paper: Thermal80
)
```

---

# ৩৩. PDF

```text
framework/pdf/
```

API:

```text
let pdf =
    await document.toPDF()
```

তারপর:

```text
await share(pdf)
```

---

# ৩৪. UI subsystem

বর্তমান Alap-এর UI architecture ইতিমধ্যে:

```text
animation
engine
layout
render
state
theme
widgets
```

এভাবে ভাগ করা আছে।

এটা রাখুন।

কিন্তু framework-level widget API আরও disciplined করুন।

---

# ৩৫. Widget hierarchy

```text
View
├── Text
├── Image
├── Icon
├── Button
├── Input
├── Checkbox
├── Switch
├── List
├── Grid
├── ScrollView
├── Dialog
└── Navigation
```

Layout:

```text
Container
Row
Column
Stack
Grid
Spacer
Divider
```

---

# ৩৬. Reactive state

Blueprint-এ state/computed store-এর ধারণা আছে।

এই model আরও strict করুন:

```text
state cart: Cart
```

Computed:

```text
computed total: Money {
    return cart.total
}
```

Mutation:

```text
cart.add(product)
```

State update হলে dependent UI automatically rerender হবে।

---

# ৩৭. POS UI hierarchy

```text
POSPage
│
├── AppShell
│
├── TopBar
│   ├── StoreName
│   ├── UserMenu
│   └── ConnectivityIndicator
│
├── Main
│   │
│   ├── CatalogPanel
│   │   ├── SearchBar
│   │   ├── CategoryTabs
│   │   └── ProductGrid
│   │
│   └── CartPanel
│       ├── CartItems
│       ├── CustomerSelector
│       ├── Discount
│       ├── Tax
│       └── CheckoutButton
│
└── BottomBar
```

---

# ৩৮. Cart model

```text
entity CartItem {

    id: UUID

    productId: UUID

    quantity: Decimal

    unitPrice: Money
    unitCost: Money

    discount: Money
    tax: Money
}
```

`unitPrice` এবং `unitCost` snapshot করবেন।

Product-এর বর্তমান price পরে বদলালেও historical sale বদলাবে না।

---

# ৩৯. Sale model

```text
entity Sale {

    id: UUID

    invoiceNumber: string

    customerId: UUID?

    subtotal: Money
    discount: Money
    tax: Money
    total: Money

    status: SaleStatus

    mutationId: UUID

    createdAt: DateTime
}
```

Sale item:

```text
entity SaleItem {

    id: UUID

    saleId: UUID
    productId: UUID

    quantity: Decimal

    unitPrice: Money
    unitCost: Money

    discount: Money
    tax: Money
    total: Money
}
```

---

# ৪০. Payment model

```text
enum PaymentMethod {
    Cash
    UPI
    Card
    Credit
}
```

```text
entity Payment {

    id: UUID

    saleId: UUID

    method: PaymentMethod

    amount: Money

    reference: string?

    createdAt: DateTime
}
```

Split payment:

```text
Cash 300
+
UPI 250
```

দুইটি Payment record।

---

# ৪১. Checkout request

```text
type CheckoutRequest = {

    cart: CartSnapshot

    customerId: UUID?

    payments: PaymentRequest[]

    discount: Money

}
```

Service:

```text
checkout(
    request: CheckoutRequest
)
```

---

# ৪২. Checkout implementation

এটাই POS-এর হৃদয়।

```text
async function checkout(
    request: CheckoutRequest
): Future<Result<Sale, SaleError>> {

    validateCart(request.cart)

    validatePayments(request.payments)

    return await db.transaction(async tx => {

        let sale =
            await sales.create(tx, request)

        await inventory.consume(
            tx,
            sale.items
        )

        await payments.record(
            tx,
            sale.id,
            request.payments
        )

        await ledger.recordSale(
            tx,
            sale
        )

        await audit.record(
            tx,
            "sale.created",
            sale.id
        )

        return Ok(sale)
    })
}
```

---

# ৪৩. Offline checkout

Offline mode-এও একই service call করবেন।

```text
checkout()
```

ভিতরে framework decide করবে:

```text
online
  ↓
remote/local transactional strategy

offline
  ↓
local transaction
+
sync mutation
```

Application code-এ:

```text
if offline ...
```

লিখতে হবে না—এটাই framework-এর কাজ।

---

# ৪৪. `pos-nil` repository

এখন নতুন repository:

```text
pos-nil/
```

structure:

```text
pos-nil/
│
├── alap.yaml
│
├── src/
│   ├── main.nil
│   │
│   ├── app/
│   │   └── App.nil
│   │
│   ├── models/
│   │   ├── Product.nil
│   │   ├── Category.nil
│   │   ├── Customer.nil
│   │   ├── Sale.nil
│   │   ├── SaleItem.nil
│   │   ├── Payment.nil
│   │   └── Ledger.nil
│   │
│   ├── repositories/
│   │   ├── ProductRepository.nil
│   │   ├── SaleRepository.nil
│   │   └── CustomerRepository.nil
│   │
│   ├── services/
│   │   ├── POSService.nil
│   │   ├── InventoryService.nil
│   │   ├── PaymentService.nil
│   │   └── ReportService.nil
│   │
│   ├── stores/
│   │   ├── POSStore.nil
│   │   └── AppStore.nil
│   │
│   ├── pages/
│   │   ├── LoginPage.nil
│   │   ├── POSPage.nil
│   │   ├── InventoryPage.nil
│   │   ├── CustomerPage.nil
│   │   └── ReportsPage.nil
│   │
│   └── components/
│       ├── ProductCard.nil
│       ├── ProductGrid.nil
│       ├── CartPanel.nil
│       ├── CheckoutPanel.nil
│       └── Receipt.nil
│
├── db/
│   ├── schema.nil
│   └── migrations/
│
├── assets/
│
└── native/
    └── ...
```

---

# ৪৫. `alap.yaml`

Target:

```yaml
name: nil-pos
version: 0.1.0

app:
  id: org.onuron.nilpos
  title: Nil POS

entry:
  source: src/main.nil

platforms:
  android: true
  linux: true
  onuron: true
  ios: true
  web: true

database:
  remote: postgres
  local: sqlite

permissions:
  - camera
  - storage
  - bluetooth
  - network
  - printer
```

---

# ৪৬. `main.nil`

```text
import alap.app
import app.App

function main() {
    App.run()
}
```

---

# ৪৭. `App.nil`

```text
app NilPOS {

    window {
        title = "Nil POS"
    }

    build() {

        AuthRouter {

            login = LoginPage()

            authenticated = POSShell()
        }
    }
}
```

---

# ৪৮. `POSStore`

```text
store POSStore {

    products: Product[] = []
    cart: Cart = Cart.empty()
    customer: Customer?
    search: string = ""

    computed filteredProducts: Product[] {

        return products.filter(
            p => p.name
                .toLower()
                .contains(search.toLower())
        )
    }

    async loadProducts() {

        products =
            await productRepository
                .search(search)
    }

    addProduct(product: Product) {

        cart.add(product)
    }

    removeItem(itemId: UUID) {

        cart.remove(itemId)
    }

    async checkout(
        payments: PaymentRequest[]
    ) {

        return await posService.checkout({
            cart: cart.snapshot(),
            customerId: customer?.id,
            payments: payments
        })
    }
}
```

---

# ৪৯. ProductCard

```text
component ProductCard {

    prop product: Product
    prop onSelect: (Product) => void

    build() {

        Card {

            Column {

                Image(product.image)

                Text(product.name)

                Text(
                    product.price.format()
                )

                Text(
                    "Stock: "
                    + product.stock.toString()
                )

                Button("Add") {

                    onClick =>
                        onSelect(product)
                }
            }
        }
    }
}
```

---

# ৫০. POSPage

```text
component POSPage {

    store = useStore<POSStore>()

    onAppear {

        task {
            await store.loadProducts()
        }
    }

    build() {

        Row {

            Column(weight: 2) {

                SearchInput(
                    value: store.search,
                    onChange: value => {
                        store.search = value
                    }
                )

                ProductGrid(
                    products:
                        store.filteredProducts,

                    onSelect:
                        product =>
                            store.addProduct(product)
                )
            }

            CartPanel(
                cart: store.cart
            )
        }
    }
}
```

---

# ৫১. Checkout UI

```text
component CheckoutPanel {

    prop cart: Cart

    state paymentMethod:
        PaymentMethod = .cash

    state amountTendered:
        Money

    computed change:
        Money {

        return amountTendered
            - cart.total
    }

    build() {

        Column {

            Text(
                "Total: "
                + cart.total.format()
            )

            PaymentMethodSelector(
                value: paymentMethod
            )

            MoneyInput(
                value: amountTendered
            )

            Text(
                "Change: "
                + change.format()
            )

            Button("Complete Sale") {

                onClick =>
                    checkout()
            }
        }
    }
}
```

---

# ৫২. Barcode-first checkout

আরও ভালো UX:

```text
POSPage
```

এখানেই global scanner listener।

```text
scanner.onDetected {

    code => {

        task {

            let product =
                await store.findBarcode(code)

            if product != null {
                store.addProduct(product)
            }
        }
    }
}
```

এতে cashier:

```text
scan
scan
scan
scan
Pay
```

করতে পারবে।

---

# ৫৩. Inventory page

```text
component InventoryPage {

    build() {

        Column {

            Toolbar {
                title = "Inventory"

                Button("Add Stock") {
                    onClick => openStockEntry()
                }
            }

            LazyTable {

                columns = [
                    "Name",
                    "SKU",
                    "Stock",
                    "Cost",
                    "Price"
                ]

                rows = inventory.rows
            }
        }
    }
}
```

---

# ৫৪. Reports

প্রথম version:

```text
Daily Sales
Sales by Product
Sales by Category
Stock Report
Profit Report
Customer Due
Expenses
Cash/UPI reconciliation
```

এগুলো framework-এর generic chart/report engine দিয়ে render করা যাবে।

---

# ৫৫. Database migration DSL

NilLang-এ:

```text
migration "001_initial" {

    create Product {
        id UUID primary
        name string
        sku string unique
        price Decimal
        stock Decimal
    }

}
```

পরের migration:

```text
migration "002_add_barcode" {

    alter Product {

        add barcode string nullable
    }
}
```

CLI:

```text
nil db generate
nil db migrate
nil db rollback
```

---

# ৫৬. Backend strategy

এখানে একটি গুরুত্বপূর্ণ সিদ্ধান্ত আছে।

আমি POS-এর initial version-এ **NilLang backend এবং NilLang frontend একই language-এ** রাখতাম।

```text
pos/
├── client/
└── server/
```

দুটিই:

```text
NilLang
```

এবং একই models:

```text
shared/
```

ব্যবহার করবে।

Architecture:

```text
                    shared/
                       │
         ┌─────────────┴──────────────┐
         │                            │
       client                       server
         │                            │
       Alap UI                   Alap Server
         │                            │
       Local DB                   PostgreSQL
         │                            │
         └──────────── API ───────────┘
```

এতে TypeScript/Next.js-এর মতো frontend/backend দুই ভাষার problem থাকবে না।

---

# ৫৭. API layer

Target:

```text
api ProductAPI {

    get(id: UUID): Product?

    search(query: string): Product[]

    create(input: CreateProduct):
        Product

    update(id: UUID, input: UpdateProduct):
        Product
}
```

Framework automatically route বানাবে:

```text
GET    /api/products
POST   /api/products
PATCH  /api/products/:id
```

---

# ৫৮. Typed RPC আরও ভালো

REST API লিখতে developer-কে manually serialization করতে না দিয়ে:

```text
service POSAPI {

    checkout(
        request: CheckoutRequest
    ): Result<Sale, SaleError>

}
```

Compiler generate করবে:

```text
client stub
server handler
serialization
validation
auth hook
```

এটা Alap-এর বড় শক্তি হতে পারে।

---

# ৫৯. JSON

NilLang-এ:

```text
let json = encode(product)
```

এবং:

```text
let product =
    decode<Product>(json)
```

Typed decode হলে invalid payload compile-time নয়, runtime validation error হবে।

---

# ৬০. Web target

বর্তমান web adapter আছে, কিন্তু এটাকে production-grade করা দরকার। বর্তমান generated runtime মূলত boot/hydration/event skeleton এবং WASM placeholder লিখছে।

আমি চাই:

```text
NilLang
   ↓
NIR
   ↓
WASM
   ↓
Alap JS host
   ↓
DOM / Canvas / WebGPU
```

তবে প্রথম POS milestone-এর জন্য Web-কে blocker করবেন না।

---

# ৬১. Android target

Android adapter-এর skeleton ইতিমধ্যেই Android Studio/Gradle project, JNI/NDK এবং NilRT bytecode loading-এর architecture করছে।

তাই:

```text
nil build android
```

কে target করুন।

Output:

```text
build/android/
    settings.gradle.kts
    build.gradle.kts
    app/
    ...
```

তারপর:

```text
./gradlew assembleDebug
```

---

# ৬২. কিন্তু Android adapter-এ পরে যা যোগ হবে

বর্তমান generic UI renderer পর্যাপ্ত নয়।

Native bridge API:

```text
alap_native_init()
alap_native_run()
alap_native_render()
alap_native_event()
alap_native_barcode()
alap_native_print()
alap_native_share()
alap_native_file()
```

একটা single giant JNI function বানাবেন না।

---

# ৬৩. ABI design

বর্তমান Alap-এ stable C ABI boundary-এর ধারণা রয়েছে।

সেটাকে formalize করুন:

```text
abi/
├── nilabi.h
├── nil_value.h
├── nil_string.h
├── nil_error.h
├── nil_context.h
├── nil_ui.h
└── nil_platform.h
```

Application-level native bridge সবকিছুর জন্য এই ABI ব্যবহার করবে।

---

# ৬৪. Capability model

Security-এর জন্য:

```text
camera
microphone
network
filesystem
bluetooth
printer
contacts
location
```

এগুলো capability হিসেবে।

```text
capability camera
capability printer
```

Application manifest:

```yaml
permissions:
  - camera
  - printer
```

Runtime:

```text
camera.scan()
```

→ capability check

---

# ৬৫. POS-এর জন্য exact permission model

```text
Product.Read
Product.Create
Product.Update
Product.Delete

Inventory.Read
Inventory.Update

Sales.Create
Sales.Void

Customer.Read
Customer.Update

Reports.View

Settings.Update

User.Manage
```

Roles:

```text
Admin
Manager
Cashier
Viewer
```

`pos-app`-এও Admin/Manager/Cashier/Viewer role model এবং per-permission API checking আছে।

---

# ৬৬. First milestone কী হবে?

**পুরো POS নয়।**

প্রথম target:

# `Nil POS Alpha 0`

শুধু:

```text
Product
Category
Local DB
Search
Cart
Cash checkout
Receipt
```

Flow:

```text
start app
   ↓
load local products
   ↓
search
   ↓
add to cart
   ↓
checkout
   ↓
transaction
   ↓
update local stock
   ↓
receipt
```

এটা যদি সত্যি NilLang-এ চলে, framework-এর core architecture validated।

---

# ৬৭. দ্বিতীয় milestone

```text
Nil POS Beta 1
```

যোগ হবে:

```text
barcode
customer
due
UPI
split payment
inventory
purchase
stock history
```

---

# ৬৮. তৃতীয় milestone

```text
Nil POS Beta 2
```

যোগ হবে:

```text
remote PostgreSQL
auth
RBAC
audit
sync
multi-device
```

---

# ৬৯. চতুর্থ milestone

```text
Nil POS 1.0
```

যোগ হবে:

```text
thermal print
PDF
reports
expenses
backup
restore
notifications
A4/A5
localization
Android release
Linux release
Onuron release
```

---

# ৭০. এখন `alap-framework`-এ কোন কাজ আগে করবেন?

আমি priority-টা একেবারে নির্দিষ্ট করে দিচ্ছি:

```text
01  Result / Option / Error
02  Decimal / Money
03  UUID / DateTime
04  Persistent local DB
05  Real transaction engine
06  ORM
07  Migration
08  HTTP client
09  Typed API/RPC
10  Router
11  Reactive state stabilization
12  Form + validation
13  Auth/session
14  Permission/RBAC
15  Audit log
16  Sync engine
17  Barcode abstraction
18  Printer abstraction
19  PDF
20  Share/files
21  Android native services
22  Linux services
23  Onuron services
24  Web/WASM runtime
25  POS domain package
```

---

# ৭১. কোন repository-তে কোন কাজ হবে?

### `nilLang`

```text
compiler/
    types/
    typecheck/
    ast/
    hir/
    mir/
    wasm/
    compiler/
    vm/
```

মূল কাজ:

```text
Result<T,E>
Option<T>
async/await
generics
decorators/attributes
entity metadata
typed query expressions
serialization metadata
```

---

### `alap-framework`

```text
framework/
    money/
    db/
    auth/
    router/
    sync/
    print/
    barcode/
    pdf/
    forms/
    validation/
    permissions/
    audit/
```

---

### `pos-nil`

```text
business logic
UI
models
repositories
services
reports
```

---

# ৭২. সবচেয়ে গুরুত্বপূর্ণ compiler feature: annotations

POS framework clean করতে decorator/attribute system খুব দরকার।

যেমন:

```text
@entity
type Product = {
    ...
}
```

```text
@table("products")
entity Product {
    ...
}
```

```text
@requires(Inventory.Update)
function updateStock(...) {
    ...
}
```

```text
@api
function checkout(...) {
    ...
}
```

Compiler metadata generate করবে।

---

# ৭৩. Query expressions compiler-এ নিতে চাই

এই:

```text
db.products
    .where(p => p.price > minPrice)
```

runtime-এ arbitrary function execute করবে না।

বরং compiler বুঝবে:

```text
Field(Product.price)
GT
Parameter(minPrice)
```

এর ফলে:

```text
NilLang expression
       ↓
Query AST
       ↓
SQL / IndexedDB query / local query
```

এই architecture অত্যন্ত গুরুত্বপূর্ণ।

---

# ৭৪. একই query সব platform-এ

Developer:

```text
products
    .where(p => p.active)
    .all()
```

Backend:

```text
PostgreSQL SQL
```

Android:

```text
SQLite
```

Web:

```text
IndexedDB query
```

কিন্তু API একই।

এটাই Alap-এর আসল cross-platform value।

---

# ৭৫. Shared model serialization

ধরুন:

```text
entity Product
```

compiler থেকে generate হবে:

```text
Product
ProductJSONCodec
ProductSchema
ProductDBMapping
ProductValidation
```

একটি source থেকে।

এতে boilerplate প্রচুর কমবে।

---

# ৭৬. POS-এর জন্য generic package বানানোর পরে

তারপর আপনি:

```text
alap add pos
```

দিয়ে domain primitives পেতে পারেন।

যেমন:

```text
import alap.pos
```

তারপর:

```text
Cart
Sale
SaleItem
Payment
Inventory
Ledger
Receipt
```

তৈরি।

তখন POS application অনেক ছোট হয়ে যাবে।

---

# ৭৭. কিন্তু `alap.pos` খুব তাড়াতাড়ি বানাবেন না

এইটা গুরুত্বপূর্ণ।

প্রথমে generic framework:

```text
db
sync
money
auth
print
barcode
```

ঠিক করুন।

তারপর:

```text
alap.pos
```

বানান।

নইলে POS-specific workaround framework-এর design নষ্ট করবে।

---

# ৭৮. Testing strategy

আপনাকে তিন স্তরে test করতে হবে।

## Language tests

```text
compiler tests
type checker tests
runtime tests
```

## Framework tests

```text
database tests
transaction tests
sync tests
money tests
auth tests
UI tests
```

## Application tests

```text
checkout tests
inventory tests
payment tests
report tests
```

---

# ৭৯. POS-এর critical test case

অবশ্যই automated test:

```text
Product stock = 10
Sale quantity = 2

→ stock = 8
```

Split payment:

```text
total = 500
cash = 300
upi = 200

→ paid = 500
→ due = 0
```

Overpayment:

```text
total = 500
cash = 600

→ change = 100
```

Offline:

```text
internet = OFF

checkout

→ sale locally committed
→ sync queued
```

Reconnect:

```text
internet = ON

→ mutation uploaded
→ queue acknowledged
```

Duplicate:

```text
same mutationId twice

→ one sale only
```

Concurrent stock:

```text
stock = 1
terminal A sells 1
terminal B sells 1

→ one succeeds
→ one fails
```

এগুলো না হলে POS production-ready নয়।

---

# ৮০. আপনার বর্তমান `pos-app`-কে কীভাবে ব্যবহার করবেন

`pos-app` delete বা rewrite করবেন না।

এটাকে বানান:

# Reference Implementation

অর্থাৎ:

```text
pos-app/
```

থেকে বের করবেন:

```text
Feature
↓
Business rule
↓
Current implementation
↓
Abstract framework requirement
↓
Alap API
↓
NilLang implementation
↓
New pos-nil implementation
```

উদাহরণ:

```text
pos-app
IndexedDB sync
       ↓
alap.sync
       ↓
pos-nil sync
```

আবার:

```text
pos-app
decimal.js
       ↓
alap.money
       ↓
pos-nil Money
```

আবার:

```text
pos-app
Capacitor ML Kit
       ↓
alap.barcode
       ↓
pos-nil scanner
```

এটাই সবচেয়ে কার্যকর migration strategy।

---

# ৮১. Migration কখন করবেন না

এই মুহূর্তে:

```text
pos-app React/Next.js
       ↓
automatically convert
       ↓
NilLang
```

করার চেষ্টা করবেন না।

কারণ translation-এর চেয়ে বড় সমস্যা হলো:

```text
framework capability gap
```

আগে Alap capability তৈরি করুন।

---

# ৮২. প্রথম actual vertical slice

আমি development-এর প্রথম সত্যিকারের target হিসেবে এটা নিতাম:

```text
Product
   ↓
Local SQLite
   ↓
ProductRepository
   ↓
POSStore
   ↓
ProductGrid
   ↓
Cart
   ↓
Cash Checkout
   ↓
Transaction
   ↓
Receipt
```

এটাকে বলুন:

# `Vertical Slice 01`

একটি feature end-to-end।

---

# ৮৩. তারপর Vertical Slice 02

```text
Barcode
 ↓
ProductRepository
 ↓
Cart
 ↓
Checkout
 ↓
Stock
 ↓
Receipt
```

---

# ৮৪. তারপর Vertical Slice 03

```text
Offline
 ↓
Sale
 ↓
SyncQueue
 ↓
Server
 ↓
PostgreSQL
 ↓
Ack
```

---

# ৮৫. তারপর Vertical Slice 04

```text
Login
 ↓
Session
 ↓
Role
 ↓
Permission
 ↓
POS
 ↓
Audit
```

---

# ৮৬. Build system-এর desired experience

শেষ পর্যন্ত developer:

```bash
nil init pos
cd pos

nil add alap.ui
nil add alap.db
nil add alap.auth
nil add alap.sync
nil add alap.pos
```

তারপর:

```bash
nil dev
```

Android:

```bash
nil build android
```

Linux:

```bash
nil build linux
```

Onuron:

```bash
nil build onuron
```

iOS:

```bash
nil build ios
```

Web:

```bash
nil build web
```

Alap-এর বর্তমান CLI-তে init/run/build/test/fmt/pm-এর direction ইতিমধ্যেই আছে।

---

# ৮৭. আমার মতে সবচেয়ে বড় architectural improvement

বর্তমান Alap-এ অনেক subsystem **নাম এবং directory হিসেবে উপস্থিত**।

এখন লক্ষ্য হওয়া উচিত:

```text
directory exists
        ↓
code exists
        ↓
unit test
        ↓
integration test
        ↓
real platform implementation
        ↓
sample application
        ↓
production application
```

অর্থাৎ:

> **Feature presence নয়, vertical completion।**

---

# ৮৮. “Done” বলতে কী বোঝাবেন?

উদাহরণ:

### `alap.db` done

শুধু:

```text
data/orm/
```

থাকলেই done নয়।

Done মানে:

```text
create
read
update
delete
query
relations
transactions
rollback
migration
pool
SQLite
Postgres
tests
```

সব আছে।

### `alap.print` done

শুধু:

```text
printer.Print()
```

থাকলেই done নয়।

Done:

```text
Android
Linux
Onuron
thermal
A4
PDF
error handling
tests
```

---

# ৮৯. বর্তমান repo-এর জন্য আমার সবচেয়ে জরুরি নির্দেশনা

একসঙ্গে সব feature implement করবেন না।

প্রথম branch:

```text
feat/application-core
```

এর মধ্যে:

```text
Result
Option
Money
Decimal
UUID
DateTime
Validation
```

তারপর:

```text
feat/database-v1
```

তারপর:

```text
feat/local-storage
```

তারপর:

```text
feat/router-state
```

তারপর:

```text
feat/auth-rbac
```

তারপর:

```text
feat/offline-sync
```

এরপর POS।

---

# ৯০. Final blueprint

পুরো ecosystem-এর final shape:

```text
                         ┌─────────────────────┐
                         │      NilLang        │
                         │                     │
                         │ Syntax              │
                         │ Type System         │
                         │ Compiler            │
                         │ HIR/MIR             │
                         │ VM / WASM           │
                         └──────────┬──────────┘
                                    │
                                    ▼
                         ┌─────────────────────┐
                         │        ALAP         │
                         │                     │
                         │ UI                  │
                         │ State               │
                         │ Router              │
                         │ Forms               │
                         │ DB                  │
                         │ ORM                 │
                         │ HTTP                │
                         │ Auth                │
                         │ RBAC                │
                         │ Sync                │
                         │ Money               │
                         │ Barcode             │
                         │ PDF                 │
                         │ Print               │
                         │ Files               │
                         │ Share               │
                         └──────────┬──────────┘
                                    │
              ┌─────────────────────┼─────────────────────┐
              │                     │                     │
              ▼                     ▼                     ▼
          Android                 Linux                Onuron
              │                     │                     │
          JNI/NDK               Wayland/X11          Wayland/Vulkan
              │                     │                     │
              └─────────────────────┼─────────────────────┘
                                    │
                                    ▼
                            ┌──────────────┐
                            │  POS-NIL     │
                            │              │
                            │ Product      │
                            │ Inventory    │
                            │ Cart         │
                            │ Checkout     │
                            │ Customer     │
                            │ Payment      │
                            │ Ledger       │
                            │ Reports      │
                            └──────────────┘
```

এটাই আমি আপনার জন্য **সঠিক দীর্ঘমেয়াদি নীলনকশা** মনে করি।

সবচেয়ে গুরুত্বপূর্ণ কথা: **এখন আপনার কাজ POS app লেখা নয়; প্রথমে Alap-কে POS লিখতে সক্ষম করা।** আর সেটার সবচেয়ে বড় তিনটি blocker হলো **real database/transaction layer, offline-sync layer, এবং native service abstraction**। বর্তমান repository-র ORM code ও platform adapters দেখে এই gap-টাই সবচেয়ে স্পষ্ট।

আপনার জন্য practical next move হবে `alap-framework`-এর **Phase 1 implementation শুরু করা: `Money + Decimal + Result + SQLite + ORM + Transaction`**। এগুলো ঠিক হয়ে গেলে আমরা প্রথম ছোট `pos-nil` vertical slice—**Product → Cart → Cash Checkout → Receipt**—বাস্তবে নীলাং code-এ নামাতে পারব।


হ্যাঁ—**লক্ষ্য হিসেবে NilLang + Alap দিয়ে `pos-app`-এর মতো অ্যাপ বানানো সম্ভব**, কিন্তু **আজকের বর্তমান রিপোজিটরি অবস্থাকে আমি “একেবারে প্রস্তুত production framework” বলব না**। বরং বলব:

> **ভাষা + compiler + VM + UI + platform adapter-এর skeleton যথেষ্ট দূর এগিয়েছে; কিন্তু POS-এর মতো বাস্তব application বানানোর জন্য framework-এর কয়েকটি critical capability এখনো পূর্ণাঙ্গ ও end-to-end wired করতে হবে।**

আমি রিপোজিটরিগুলো সরাসরি দেখে এই সিদ্ধান্তে এসেছি। Alap-এ compiler, VM, UI, runtime, platform adapters এবং stdlib-এর কাঠামো আছে; Android adapter-ও আছে।

অন্যদিকে `pos-app` একটি ছোট demo নয়। এটি offline-first POS, IndexedDB sync, PostgreSQL/Prisma, authentication/RBAC, inventory, accounting-style ledger, printing/PDF, barcode, Android native integration, localization এবং বহু API নিয়ে তৈরি।

---

# ১. এখনকার অবস্থাটা বাস্তবে কোথায়?

আমি তিনটি repository-কে এভাবে দেখি:

| অংশ                     | বর্তমান অবস্থা                                | POS-এর জন্য                   |
| ----------------------- | --------------------------------------------- | ----------------------------- |
| NilLang syntax/compiler | ভালো ভিত্তি                                   | ✅ যথেষ্ট ভিত্তি               |
| Static typing           | আছে                                           | ✅                             |
| Bytecode + VM           | আছে                                           | ✅                             |
| Declarative UI model    | আছে                                           | ✅ ভিত্তি                      |
| UI widgets/layout/state | কাঠামো আছে                                    | 🟡 গভীরতা বাড়াতে হবে          |
| Android adapter         | আছে                                           | 🟡 production hardening দরকার |
| Web adapter             | আছে                                           | 🔴 বর্তমানে placeholder-level |
| Network                 | stdlib/structure আছে                          | 🟡                            |
| JSON                    | আছে                                           | ✅ ভিত্তি                      |
| Database                | ORM skeleton আছে                              | 🔴 POS-এর জন্য যথেষ্ট নয়      |
| Transactions            | API আছে, কিন্তু বাস্তব DB transaction নয়      | 🔴                            |
| Offline local DB        | নেই/অপূর্ণ                                    | 🔴                            |
| Sync engine             | নেই                                           | 🔴                            |
| Auth/RBAC               | framework-level production implementation নেই | 🔴                            |
| PDF/printing            | POS-এর মতো complete subsystem নেই             | 🔴                            |
| Barcode/camera          | native abstraction দরকার                      | 🔴                            |
| Money/Decimal           | first-class money type দরকার                  | 🔴                            |
| Form validation         | দরকার                                         | 🟡                            |
| Routing/navigation      | ভিত্তি আছে                                    | 🟡                            |
| Packaging               | আছে                                           | ✅ ভিত্তি                      |
| Package manager         | আছে                                           | ✅ ভিত্তি                      |
| LSP/dev tooling         | আছে                                           | ✅/🟡                          |

সবচেয়ে গুরুত্বপূর্ণ প্রমাণ হচ্ছে database layer। Alap-এর `data/orm/orm.go` এখন `SELECT ... WHERE ... LIMIT` SQL string বানানোর QueryBuilder এবং একটি খুবই সরল `Transaction()` abstraction দেখাচ্ছে; বাস্তব DB connection, query execution, rollback/commit semantics সেখানে নেই।

অর্থাৎ **“ORM directory আছে” মানেই Prisma-এর সমতুল্য ORM তৈরি হয়ে গেছে নয়।**

একই সমস্যা Web adapter-এও আছে। সেখানে `alap.wasm` লেখা হলেও browser runtime-এর বড় অংশ placeholder-style JS (`console.log`, simple event binding, hydration skeleton)।

---

# ২. `pos-app` আসলে কী ধরনের জিনিস?

আপনার POS app-এর README দেখলে পরিষ্কার যে এটি মূলত এই architecture:

```text
                    POS APPLICATION
                           │
             ┌─────────────┴─────────────┐
             │                           │
         Frontend                    Backend
             │                           │
      Next.js + React               Next API
             │                           │
       Zustand state                 Prisma
             │                           │
      IndexedDB/offline           PostgreSQL
             │
      Capacitor Android
             │
       Native Plugins
```

এখানে শুধু UI নেই।

এতে আছে:

```text
UI
↓
application state
↓
business rules
↓
financial calculations
↓
local persistence
↓
offline operation queue
↓
synchronization
↓
server API
↓
database transactions
↓
audit trail
```

এটাই আপনার Alap-কে eventually করতে হবে।

`pos-app`-এর README-তেই offline sync/idempotency, atomic stock update, RBAC, audit log, WAC inventory costing, split payment, due ledger, prepaid balance, PDF/thermal printing, barcode ইত্যাদি উল্লেখ আছে।

---

# ৩. তাই Alap-এর আসল লক্ষ্য কী হওয়া উচিত?

আমি Alap-কে শুধু:

> “NilLang দিয়ে UI বানানোর framework”

বানাতাম না।

আমি এটাকে বানাতাম:

> **Application Operating Layer**

অর্থাৎ:

```text
                 NilLang Application
                         │
                         ▼
                    ALAP SDK
                         │
       ┌─────────────────┼──────────────────┐
       │                 │                  │
      UI              Data               System
       │                 │                  │
       ▼                 ▼                  ▼
 Rendering          Persistence          Camera
 State              Database             Barcode
 Routing            Sync                 Print
 Forms              Cache                Files
 Animation          Transactions         Share
 Accessibility      Auth                 Network
       │                 │                  │
       └─────────────────┼──────────────────┘
                         ▼
                  Platform Adapter
          ┌──────────┬──────────┬─────────┐
        Android    Linux      Onuron    iOS
```

এটাই দীর্ঘমেয়াদে Alap-কে সত্যিকারের framework বানাবে।

---

# ৪. POS বানানোর জন্য আমি Alap-এ একটি বিশেষ “Business App Stack” যোগ করতাম

এটি অত্যন্ত গুরুত্বপূর্ণ।

আপনার framework-এ generic primitive থাকবে, কিন্তু POS-এর মতো app দ্রুত বানানোর জন্য higher-level packages থাকবে।

যেমন:

```text
alap.core
alap.ui
alap.router
alap.forms
alap.data
alap.db
alap.sync
alap.auth
alap.permissions
alap.storage
alap.network
alap.print
alap.barcode
alap.pdf
alap.locale
alap.money
alap.audit
alap.test
```

তারপর application code এমন হবে:

```text
import alap.ui
import alap.data
import alap.db
import alap.money
import alap.print
import alap.barcode
```

এটাই হবে আসল power.

---

# ৫. সবচেয়ে গুরুত্বপূর্ণ: Money type

POS application-এ `float64` ব্যবহার করা যাবে না।

আমি NilLang-এর standard/business library-তে:

```text
Money
Decimal
Currency
Tax
Percentage
```

first-class type করতাম।

যেমন:

```text
let price: Money = money("125.50", "INR")
let qty: Decimal = decimal("2.5")

let subtotal = price * qty
```

আর:

```text
subtotal + tax - discount
```

সবকিছু deterministic decimal arithmetic-এ চলবে।

---

# ৬. Database architecture

এখানে Alap-এর সবচেয়ে বেশি কাজ বাকি।

বর্তমান ORM skeleton-কে production database subsystem-এ রূপান্তর করতে হবে।

আমি API এমন করতাম:

```text
database ShopDB {
    provider = postgres
}
```

Model:

```text
entity Product {
    id: UUID
    name: string
    sku: string unique
    barcode: string?
    price: Money
    cost: Money
    stock: Decimal
    active: bool

    createdAt: DateTime
    updatedAt: DateTime
}
```

Query:

```text
let products = await db.Product
    .where(p => p.active == true)
    .orderBy(p => p.name)
    .take(100)
    .all()
```

Insert:

```text
let product = await db.Product.create({
    name: "Rice",
    sku: "RICE001",
    price: money("65", "INR"),
    cost: money("55", "INR"),
    stock: decimal("100")
})
```

Update:

```text
await db.Product.update(product.id, {
    price: money("70", "INR")
})
```

---

# ৭. Transaction অবশ্যই সত্যিকারের transaction হতে হবে

বর্তমান:

```text
Transaction(fn)
```

ধারণাটি আছে, কিন্তু database transaction semantics বাস্তবে নেই।

POS-এ sale করার সময়:

```text
BEGIN

create Sale
create SaleItems
decrease Stock
create Payment
create LedgerEntry
create AuditLog

COMMIT
```

যেকোনো ধাপে failure হলে:

```text
ROLLBACK
```

NilLang API:

```text
await db.transaction(async tx => {
    let sale = await tx.sales.create(...)
    await tx.stock.decrease(...)
    await tx.payments.create(...)
    await tx.ledger.create(...)
})
```

এটি framework-এর core feature হওয়া উচিত।

---

# ৮. Offline-first architecture

এটাই `pos-app`-এর অন্যতম কঠিন অংশ।

আমি Alap-এ first-class:

```text
LocalDB
SyncQueue
SyncEngine
ConflictResolver
Connectivity
MutationLog
```

দিতাম।

Architecture:

```text
              UI
               │
               ▼
        Application Service
               │
       ┌───────┴────────┐
       │                │
   Local DB          Sync Queue
       │                │
       └───────┬────────┘
               │
          Connectivity
               │
               ▼
          Sync Engine
               │
               ▼
             API
               │
               ▼
           Server DB
```

Sale করার সময় internet না থাকলেও:

```text
Sale created locally
↓
stock updated locally
↓
receipt generated
↓
sync queue entry created
↓
user continues working
```

নেট ফিরলে:

```text
queue
 ↓
idempotency key
 ↓
server
 ↓
ack
 ↓
local queue marked synced
```

---

# ৯. Idempotency first-class feature হওয়া উচিত

POS-এ সবচেয়ে ভয়ংকর সমস্যা:

```text
user presses Pay
↓
request sent
↓
network timeout
↓
user presses Pay again
```

ফলে দুইটি sale তৈরি হয়ে গেল।

তাই:

```text
mutationId: UUID
```

প্রতিটি financial operation-এ থাকবে।

```text
Sale {
    id
    mutationId
}
```

Server:

```text
if mutationId already processed:
    return existing result
```

এটি `alap.sync`-এর core primitive হওয়া উচিত।

---

# ১০. POS-এর domain model

আমি শুরুতেই এই entities বানাতাম:

```text
User
Role
Permission

Product
Category
Unit
Barcode

Supplier
Customer

Stock
StockMovement
StockPurchase

Sale
SaleItem
Payment

LedgerEntry
Due
Prepayment

Expense
CashSession

Invoice
InvoiceTemplate

AuditLog

SyncOperation
Device
```

সম্পর্ক:

```text
Customer
   │
   ├── Sales
   ├── Payments
   ├── Due Ledger
   └── Prepayment

Product
   │
   ├── Stock
   ├── StockMovement
   └── SaleItems

Sale
 ├── SaleItems
 ├── Payments
 ├── LedgerEntries
 └── Invoice
```

---

# ১১. State management NilLang-এর জন্য

আপনার বর্তমান UI design অনুযায়ী state model already declarative দিকের দিকে যাচ্ছে। Blueprint-এ `state`, computed state এবং external store-এর ধারণা আছে।

POS-এ:

```text
store POSStore {
    cart: Cart
    customer: Customer?
    payment: PaymentState
    processing: bool

    addProduct(product: Product) {
        ...
    }

    removeItem(id: UUID) {
        ...
    }

    checkout(): Future<Result<Sale, SaleError>> {
        ...
    }
}
```

তারপর UI:

```text
component POSScreen {

    build() {

        Row {

            ProductCatalog()

            CartPanel(cart)

            CheckoutPanel(payment)
        }
    }
}
```

এই separation খুব গুরুত্বপূর্ণ:

```text
UI ≠ business logic
```

---

# ১২. POS screen-এর NilLang design কেমন হবে?

ধরুন ভবিষ্যৎ Alap API:

```text
component ProductCard {
    prop product: Product
    prop onSelect: (Product) => void

    build() {
        Card {
            Column {
                Text(product.name)
                Text(product.price.format())
                Text("Stock: " + product.stock.toString())

                Button("Add") {
                    onClick => onSelect(product)
                }
            }
        }
    }
}
```

Catalog:

```text
component ProductGrid {

    prop products: Product[]
    prop onSelect: (Product) => void

    build() {

        LazyGrid(columns: 4) {

            for product in products {

                ProductCard(
                    product: product,
                    onSelect: onSelect
                )
            }
        }
    }
}
```

Cart:

```text
component CartPanel {

    prop cart: Cart

    build() {

        Column {

            Text("Cart")

            for item in cart.items {

                Row {

                    Text(item.product.name)

                    QuantityStepper(
                        value: item.quantity,
                        onChange: q => cart.setQuantity(item.id, q)
                    )

                    Text(item.total.format())
                }
            }

            Divider()

            Text("Subtotal: " + cart.subtotal.format())
            Text("Discount: " + cart.discount.format())
            Text("Tax: " + cart.tax.format())
            Text("Total: " + cart.total.format())
        }
    }
}
```

---

# ১৩. Checkout পুরোপুরি business service হবে

UI থেকে database সরাসরি call করাবেন না।

ভুল:

```text
Button {
    onClick => db.sales.create(...)
}
```

সঠিক:

```text
Button {
    onClick => pos.checkout()
}
```

তারপর:

```text
class POSService {

    async checkout(
        request: CheckoutRequest
    ): Future<Result<Sale, SaleError>> {

        return await db.transaction(async tx => {

            let sale = await createSale(tx, request)

            await reduceInventory(tx, sale)

            await recordPayments(tx, sale)

            await updateLedger(tx, sale)

            await audit(tx, sale)

            return sale
        })
    }
}
```

UI business logic জানবেই না transaction-এর ভিতরে কী হচ্ছে।

---

# ১৪. Barcode

`pos-app`-এ desktop keyboard-wedge scanner এবং Android camera/ML Kit আছে।

Alap-এর API হওয়া উচিত:

```text
barcode.onScan {
    code =>
        pos.findProductByBarcode(code)
}
```

Camera:

```text
let scanner = BarcodeScanner()

await scanner.start()

scanner.onDetected {
    barcode =>
        handleBarcode(barcode)
}
```

Platform implementation:

```text
Android → ML Kit
iOS     → native scanner
Linux   → keyboard wedge / USB HID
Onuron  → native camera/barcode service
```

Application code এগুলো জানবে না।

---

# ১৫. Printing

এটিও একটি আলাদা framework subsystem হওয়া উচিত:

```text
alap.print
alap.pdf
```

API:

```text
let invoice = Invoice(
    number: sale.invoiceNumber,
    items: sale.items,
    total: sale.total
)

await printer.print(
    invoice,
    paper: .thermal80
)
```

PDF:

```text
let pdf = await invoice.toPDF()

await share(pdf)
```

Output targets:

```text
58mm thermal
80mm thermal
A4
A5
PDF
system printer
share sheet
```

`pos-app`-এর বর্তমান implementation-ও thermal, A4/A5, Android PrintManager, PDF share ইত্যাদি আলাদা করে handle করে।

---

# ১৬. Authentication

Alap-এ:

```text
auth.login()
auth.logout()
auth.currentUser()
```

এর সঙ্গে:

```text
@requires("sales.create")
function createSale(...) { ... }
```

অথবা:

```text
permission Sales.Create
permission Inventory.Update
permission Reports.View
```

তারপর:

```text
role Admin {
    Sales.*
    Inventory.*
    Reports.*
}

role Cashier {
    Sales.Create
    Sales.View
}
```

এতে developer-কে নিজে নিজে RBAC লেখার প্রয়োজন কমে যাবে।

---

# ১৭. Audit log framework-level হওয়া উচিত

POS-এর মতো app-এ:

```text
who
what
when
where
before
after
device
requestId
```

এসব দরকার।

তাই:

```text
audit.record(
    action: "sale.created",
    entity: sale.id,
    actor: user.id
)
```

এবং critical database operation framework নিজেই audit hook করতে পারবে।

---

# ১৮. Reporting system

আপনার বর্তমান `pos-app`-এ dashboard/report subsystem আছে।

Alap-এ generic query/report API করা যেতে পারে:

```text
report SalesByDay {

    dimension date
    measure totalSales = sum(sale.total)
    measure transactions = count(sale.id)
}
```

তারপর:

```text
Chart(SalesByDay)
```

অর্থাৎ framework শুধু app বানাবে না—**business app-এর common patterns-ও standardize করবে।**

---

# ১৯. POS project-এর recommended directory

আমি এমন project structure নিতাম:

```text
pos-nil/
│
├── alap.yaml
│
├── src/
│   ├── main.nil
│   │
│   ├── app/
│   │   └── PosApp.nil
│   │
│   ├── pages/
│   │   ├── LoginPage.nil
│   │   ├── POSPage.nil
│   │   ├── InventoryPage.nil
│   │   ├── CustomersPage.nil
│   │   ├── ReportsPage.nil
│   │   └── SettingsPage.nil
│   │
│   ├── components/
│   │   ├── ProductCard.nil
│   │   ├── ProductGrid.nil
│   │   ├── CartPanel.nil
│   │   ├── CheckoutPanel.nil
│   │   ├── BarcodeInput.nil
│   │   └── ReceiptPreview.nil
│   │
│   ├── models/
│   │   ├── Product.nil
│   │   ├── Sale.nil
│   │   ├── Customer.nil
│   │   ├── Payment.nil
│   │   └── Ledger.nil
│   │
│   ├── stores/
│   │   ├── POSStore.nil
│   │   ├── InventoryStore.nil
│   │   └── SessionStore.nil
│   │
│   ├── services/
│   │   ├── POSService.nil
│   │   ├── InventoryService.nil
│   │   ├── CustomerService.nil
│   │   └── ReportService.nil
│   │
│   ├── repositories/
│   │   ├── ProductRepository.nil
│   │   ├── SaleRepository.nil
│   │   └── CustomerRepository.nil
│   │
│   ├── sync/
│   │   ├── SyncEngine.nil
│   │   └── ConflictResolver.nil
│   │
│   └── reports/
│       ├── SalesReport.nil
│       ├── StockReport.nil
│       └── ProfitReport.nil
│
├── db/
│   ├── schema.nil
│   └── migrations/
│
├── assets/
│   ├── icons/
│   ├── images/
│   ├── fonts/
│   └── receipts/
│
└── native/
    ├── android/
    ├── ios/
    ├── linux/
    └── onuron/
```

---

# ২০. `main.nil`

Conceptually:

```text
import alap.ui
import alap.router
import alap.auth
import app.PosApp

function main() {

    AlapApp.run(
        app: PosApp()
    )
}
```

---

# ২১. `PosApp.nil`

```text
app PosApp {

    window {
        title = "Nil POS"
    }

    build() {

        Router {

            route("/login", LoginPage())
            route("/pos", POSPage())
            route("/inventory", InventoryPage())
            route("/customers", CustomersPage())
            route("/reports", ReportsPage())
            route("/settings", SettingsPage())
        }
    }
}
```

---

# ২২. POS page

```text
component POSPage {

    let store = useStore<POSStore>()

    onAppear {
        task {
            await store.loadCatalog()
        }
    }

    build() {

        Row {

            Column(weight: 2) {

                SearchInput(
                    value: store.search,
                    onChange: value => store.search = value
                )

                ProductGrid(
                    products: store.filteredProducts,
                    onSelect: product =>
                        store.addToCart(product)
                )
            }

            CartPanel(
                cart: store.cart
            )
        }
    }
}
```

এটাই হবে আপনার framework-এর ideal developer experience।

---

# ২৩. `checkout()` এর real flow

একজন user:

```text
Barcode scan
      ↓
Product lookup
      ↓
Add to cart
      ↓
Quantity
      ↓
Discount
      ↓
Tax
      ↓
Total
      ↓
Payment
      ↓
Transaction
      ↓
Stock deduction
      ↓
Ledger
      ↓
Invoice
      ↓
Print
      ↓
Sync
```

Framework architecture:

```text
Scanner
   ↓
POSStore
   ↓
POSService
   ↓
TransactionManager
   ├── SaleRepository
   ├── StockRepository
   ├── PaymentRepository
   ├── LedgerRepository
   └── AuditRepository
   ↓
Local commit
   ↓
SyncQueue
   ↓
Remote server
```

---

# ২৪. এখন আসল প্রশ্ন: বর্তমান Alap দিয়ে আজই কি এটা লিখে ফেলা যাবে?

**পূর্ণ `pos-app` parity-তে — না।**

**একটি ছোট POS prototype — প্রায় অবশ্যই লক্ষ্য করা উচিত।**

যেমন প্রথম milestone:

```text
Product list
+
Search
+
Cart
+
Quantity
+
Subtotal
+
Cash payment
+
Local persistence
+
Receipt
```

এগুলো হলে আপনি প্রমাণ করবেন:

```text
NilLang
  ↓
Compiler
  ↓
VM
  ↓
Alap UI
  ↓
State
  ↓
Storage
  ↓
Business logic
```

এই পুরো chain সত্যি কাজ করছে।

---

# ২৫. আমি development order এভাবে নিতাম

## Phase 1 — “Hello Business App”

আগে language/framework দিয়ে:

```text
Todo
Notes
Expense Tracker
Inventory
```

বানান।

কারণ এগুলো দিয়ে আপনি পরীক্ষা করবেন:

```text
UI
state
navigation
forms
storage
CRUD
```

---

## Phase 2 — Data layer

এখন:

```text
alap.db
alap.orm
alap.local
alap.sync
```

সম্পূর্ণ করুন।

এটি সবচেয়ে গুরুত্বপূর্ণ phase।

---

## Phase 3 — Business primitives

তারপর:

```text
Money
Decimal
Date
Time
UUID
Validation
Result
Error
```

---

## Phase 4 — Device APIs

তারপর:

```text
Camera
Barcode
File
Share
Print
PDF
Network
SecureStorage
Notifications
```

---

## Phase 5 — Auth

```text
Authentication
Session
RBAC
Permissions
Audit
```

---

## Phase 6 — POS framework package

তারপর:

```text
alap.business
alap.pos
```

এর মধ্যে:

```text
Cart
Product
Inventory
Sale
Payment
Ledger
Invoice
```

---

# ২৬. তারপর POS app বানানো হবে

এখন application developer-এর কাছে কাজ কমে যাবে:

```text
Product model
Sale model
POSStore
POSService
Pages
Components
```

এটাই ideal।

---

# ২৭. Android architecture

এখানে আপনার বর্তমান Alap-এর একটি ভালো foundation ইতিমধ্যেই আছে।

Android adapter project structure generate করছে, JNI/NDK bridge এবং NABC runtime loading-এর ধারণাও আছে।

Architecture:

```text
                 POS.nil
                   │
                 nilc
                   │
                 NABC
                   │
            ┌──────┴──────┐
            │             │
        Alap UI        NilRT
            │             │
            └──────┬──────┘
                   │
                JNI ABI
                   │
             Android native
                   │
        ┌──────────┼──────────┐
        │          │          │
      Camera    Printer    Files
```

এখানে একটা খুব গুরুত্বপূর্ণ architectural rule:

**Android-specific API NilLang application-এর ভেতরে ছড়িয়ে দেবেন না।**

বরং:

```text
alap.barcode.scan()
alap.print()
alap.share()
```

এই generic API ব্যবহার করবেন।

---

# ২৮. Web target সম্পর্কে আমার কঠিন মত

বর্তমান web adapter-কে আমি এখনও production web framework বলব না।

কারণ adapter-এর generated runtime-এ UI execution/reconciliation-এর বাস্তব engine-এর পরিবর্তে skeleton implementation দেখা যাচ্ছে, এবং `.wasm` লেখার জায়গাটিও placeholder-like।

তাই **প্রথমে Android/Linux/Onuron native target শক্ত করুন।**

তারপর Web/WASM।

এতে architecture-ও পরিষ্কার থাকবে।

---

# ২৯. আরও গুরুত্বপূর্ণ: আপনি Next.js-এর clone বানাবেন না

এটা ভুল দিক হবে:

```text
Alap = Next.js rewritten in NilLang
```

বরং:

```text
Alap = application runtime
```

যার মধ্যে:

```text
UI
+
state
+
routing
+
data
+
native API
+
storage
+
sync
+
build
```

সব integrated।

এটাই আপনার ecosystem-এর আসল advantage হতে পারে।

---

# ৩০. Final target architecture

দীর্ঘমেয়াদে আমি পুরো system-কে এমন করতাম:

```text
                         NILANG SOURCE
                              │
                              ▼
                         ALAP COMPILER
                              │
                ┌─────────────┼─────────────┐
                │             │             │
              Logic          UI           Data
                │             │             │
                └─────────────┼─────────────┘
                              │
                           NIR/NABC
                              │
                         NIL RUNTIME
                              │
        ┌─────────────────────┼─────────────────────┐
        │                     │                     │
     ALAP UI               ALAP DATA            ALAP SYS
        │                     │                     │
    Rendering             DB/LocalDB             Camera
    Layout                Sync                   Barcode
    State                 Cache                  Printer
    Forms                 Network                Files
    Router                Transaction            Share
        │                     │                     │
        └─────────────────────┼─────────────────────┘
                              │
                      PLATFORM ADAPTER
                              │
          ┌───────────┬───────┼────────┬───────────┐
          │           │       │        │           │
       Android      Linux   Onuron    iOS         Web
```

আর POS:

```text
                         POS APP
                            │
         ┌──────────────────┼───────────────────┐
         │                  │                   │
        UI                Domain              Data
         │                  │                   │
   POS Dashboard        Sale Service        Local DB
   Inventory            Payment             Remote DB
   Checkout             Inventory           Sync
   Reports              Ledger              Cache
         │                  │                   │
         └──────────────────┼───────────────────┘
                            │
                       ALAP PLATFORM
```

---

# ৩১. সবচেয়ে বাস্তব কাজের তালিকা

আপনার এখন **আরেকটা demo widget বানানো নয়**। এখন দরকার framework-কে application-grade করা।

আমি priority এমন রাখব:

```text
P0
├── compiler/runtime correctness
├── real DB abstraction
├── real transactions
├── local persistent storage
├── HTTP client/server
├── Result/Error
├── Decimal/Money
└── reactive UI correctness

P1
├── Router
├── Forms + validation
├── Auth
├── RBAC
├── Audit
├── Sync engine
├── idempotency
└── migration system

P2
├── Barcode
├── Camera
├── PDF
├── Printing
├── Share
├── File APIs
└── Secure storage

P3
├── POS-specific packages
├── reporting
├── charts
├── business templates
└── developer tooling

P4
├── Web/WASM production backend
├── AOT
├── optimization
└── ecosystem/package registry maturity
```

---

# ৩২. Bottom line

আপনার repositories-এর বর্তমান architecture দেখে আমার মূল্যায়ন:

**NilLang:**
ভাষা হিসেবে এখন যথেষ্ট বড় foundation তৈরি হয়েছে—compiler, parser, type system, HIR/MIR, VM, package ecosystem, runtime ইত্যাদির স্পষ্ট architecture আছে।

**Alap:**
একটি framework-এর structural skeleton ইতিমধ্যেই বেশ ভালো—UI, runtime, platform adapters, stdlib, data এবং tooling-এর আলাদা স্তর আছে।

**কিন্তু:**
আজকের অবস্থায় এটি `pos-app`-এর মতো production-grade business application framework নয়। বিশেষ করে real persistence/transactions, offline sync, business-grade data model, money/decimal, printing, barcode, auth/RBAC এবং production Web runtime এখনো শক্ত করতে হবে। ORM-এর বর্তমান code-ই এই gap-এর সবচেয়ে পরিষ্কার উদাহরণ।

**তবে architecture-এর দিক থেকে আপনি ভুল পথে নেই।** বরং এখনকার সবচেয়ে বুদ্ধিমান কাজ হবে `pos-app`-কে আলাদা করে NilLang-এ port করার আগে **Alap-কে এমন অবস্থায় নিয়ে যাওয়া যাতে POS application-এর business code সত্যিই NilLang-এ লেখা যায় এবং native platform code framework-এর নিচে লুকিয়ে থাকে।**

আর একটা জিনিস বিশেষভাবে বলব: `pos-app`-এর **feature-by-feature clone** করার বদলে এর architecture-কে reference implementation হিসেবে নিয়ে **Alap Business Application Stack** বানালে Alap-এর মূল্য অনেক বেশি হবে।

এই ভিত্তিতে পরের ধাপ হওয়া উচিত **“Alap v0.x → POS-ready SDK”**-এর জন্য repository-level implementation blueprint—অর্থাৎ কোন নতুন directory/file/package তৈরি হবে, NilLang-এ exact API কেমন হবে, `alap-framework`-এর কোন existing অংশ বদলাতে হবে, এবং শেষে প্রথম `pos.nil` prototype কীভাবে compile হয়ে Android APK-তে যাবে—এগুলো একেবারে code-level নীলনকশা।

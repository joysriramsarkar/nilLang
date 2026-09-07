হ্যাঁ—এবার লক্ষ্যটা বদলাতে হবে। **“NilLang দিয়ে একটা POS demo বানানো” নয়; “TypeScript + Next.js + Node.js দিয়ে যে পূর্ণাঙ্গ POS বানাবি, Alap + NilLang-এ তার সমান product বানানো”**—এটাই সঠিক target।

তোর বর্তমান `nilLang` repo-তে ৬ সেপ্টেম্বর ২০২৬ পর্যন্ত compiler, HIR/MIR, WASM, Entity, ORM/data, Money/Decimal, routing, web-dev CLI ইত্যাদি যোগ হয়েছে; সর্বশেষ commits-ও `pos-app` এবং formatting-এর দিকে গেছে। কিন্তু এগুলো এখনও একটি **production POS platform-এর সমগ্র contract পূরণ করে না**।

## প্রথমে একটি গুরুত্বপূর্ণ কথা

**TypeScript/Next.js POS = NilLang/Alap POS** বলতে আমি “একই code” বোঝাচ্ছি না।

আমি বোঝাচ্ছি:

```text
                         SAME PRODUCT CONTRACT

 TypeScript + Next.js + Node.js              NilLang + Alap
 ─────────────────────────────              ─────────────────────
 UI                                          UI
 Routing                                     Routing
 State                                       State
 Forms                                       Forms
 Validation                                  Validation
 API                                         API
 Authentication                              Authentication
 Database                                    Database
 Transactions                                Transactions
 Inventory                                   Inventory
 POS cart                                    POS cart
 Checkout                                    Checkout
 Payment                                     Payment
 Receipt                                     Receipt
 Reports                                     Reports
 Bengali                                     Bengali
 Offline                                     Offline
 Printing                                    Printing
 Keyboard                                    Keyboard
 Permissions                                 Permissions
 Audit                                       Audit
 Realtime                                    Realtime
```

অর্থাৎ **একটা POS-এর যেসব observable behaviour, business rule এবং user interaction আছে—দুই implementation-এ সেগুলো একই হতে হবে।**

---

# ১. সবচেয়ে আগে Alap-কে “UI framework” থেকে “Application Platform” করতে হবে

তোর Alap-এ এখন component/state/render জাতীয় ভিত্তি আছে। কিন্তু POS-এর জন্য শুধু declarative component যথেষ্ট নয়।

Alap-এর architecture আমি এভাবে চাই:

```text
Alap
├── UI Runtime
│   ├── Component
│   ├── State
│   ├── Event
│   ├── Form
│   ├── Table
│   ├── Modal
│   ├── Drawer
│   ├── Select
│   ├── Combobox
│   ├── DatePicker
│   ├── Tabs
│   ├── Toast
│   ├── Tooltip
│   ├── Pagination
│   └── VirtualList
│
├── Application Runtime
│   ├── Router
│   ├── Navigation
│   ├── Session
│   ├── Cache
│   ├── Async
│   ├── Validation
│   ├── Forms
│   ├── i18n
│   └── Permissions
│
├── Server Runtime
│   ├── HTTP
│   ├── API
│   ├── Middleware
│   ├── Auth
│   ├── Sessions
│   ├── WebSocket
│   └── Jobs
│
├── Data Runtime
│   ├── ORM
│   ├── Query Builder
│   ├── Transactions
│   ├── Migration
│   ├── Relations
│   ├── SQLite
│   ├── PostgreSQL
│   └── Sync
│
├── Device Runtime
│   ├── Printer
│   ├── Barcode Scanner
│   ├── Cash Drawer
│   ├── Keyboard
│   ├── USB
│   ├── Bluetooth
│   └── Camera
│
└── POS Runtime
    ├── Product
    ├── Inventory
    ├── Cart
    ├── Sale
    ├── Payment
    ├── Refund
    ├── Customer
    ├── Supplier
    ├── Purchase
    ├── Tax
    ├── Discount
    ├── Shift
    ├── Register
    ├── Receipt
    ├── Report
    └── Audit
```

**এই layer-গুলো Alap-এর first-class primitives হওয়া দরকার।**

---

# ২. NilLang syntax-কে POS বানানোর উপযোগী করতে হবে

বর্তমান ভাষা দিয়ে imperative কাজ করা যায়, কিন্তু POS-এর code যেন TypeScript-এর মতো boilerplate-heavy না হয়।

আমি NilLang-এ এই ধরনের abstraction চাই:

```nil
entity Product {
    id: uuid primary
    sku: string unique
    name: string
    price: money
    stock: decimal
    category: relation Category
}
```

এটা শুধু struct নয়।

একই declaration থেকে:

```text
Database schema
+
Validation
+
CRUD
+
API
+
Client model
+
Form metadata
+
Table metadata
+
Search metadata
+
Serialization
```

generate হবে।

অর্থাৎ:

```nil
entity Product
```

হবে **single source of truth**।

তোর repo-তে Entity generator এখন SQL, REST এবং client model পর্যন্ত করেছে—এটাই সঠিক দিক; এখন এটাকে অনেক গভীরে নিতে হবে।

---

# ৩. Entity system-কে পূর্ণাঙ্গ করতে হবে

বর্তমান:

```text
Field
Entity
Relation
GenerateSQL
GenerateRESTEndpoints
GenerateClientModel
Validate
```

থেকে যেতে হবে:

```text
Entity
 ├── schema
 ├── validation
 ├── defaults
 ├── indexes
 ├── unique constraints
 ├── foreign keys
 ├── cascade rules
 ├── soft delete
 ├── timestamps
 ├── computed fields
 ├── lifecycle hooks
 ├── audit fields
 ├── permissions
 ├── serialization
 ├── filtering
 ├── sorting
 ├── pagination
 ├── search
 └── API contract
```

যেমন:

```nil
entity Product {
    id: uuid primary
    sku: string unique indexed
    name: string required searchable
    price: money required
    stock: decimal default 0
    active: bool default true

    created_at: datetime auto
    updated_at: datetime auto
}
```

তারপর:

```nil
Product.find(...)
Product.where(...)
Product.order(...)
Product.paginate(...)
Product.create(...)
Product.update(...)
Product.delete(...)
```

---

# ৪. ORM-কে সত্যিকারের ORM বানাতে হবে

POS-এ ORM শুধু CRUD করলে চলবে না।

প্রয়োজন:

```nil
transaction {
    sale = Sale.create(...)
    SaleItem.create(...)
    Product.update(...)
    Payment.create(...)
}
```

একটাও step fail করলে:

```text
ROLLBACK
```

সবকিছু rollback।

### অবশ্যই লাগবে

* transactions
* nested transactions / savepoint
* prepared statement
* parameter binding
* connection pool
* timeout
* retry
* deadlock handling
* optimistic locking
* pessimistic locking
* eager loading
* lazy loading
* joins
* aggregates
* group by
* raw SQL escape hatch
* migrations
* seed
* indexes
* constraints

---

# ৫. Money system-এ float পুরোপুরি সরাতে হবে

এটা POS-এর জন্য অত্যন্ত গুরুত্বপূর্ণ।

বর্তমান Money system আছে—এটা ভালো। কিন্তু `float64`-ভিত্তিক constructor/multiplication production financial core-এ রাখা যাবে না।

অর্থাৎ এ ধরনের API:

```text
Money.Mul(float64)
NewMoneyFromMajor(float64)
```

শেষ পর্যন্ত core calculation থেকে সরাতে হবে।

বরং:

```nil
money
decimal
quantity
percentage
tax
discount
```

সব fixed-point / Decimal ভিত্তিক হবে।

উদাহরণ:

```nil
subtotal = price * quantity
discount = subtotal * discount_rate
tax = taxable_amount * tax_rate
total = subtotal - discount + tax
```

এগুলোর প্রতিটি calculation deterministic হতে হবে।

---

# ৬. Quantity-কে আলাদা numeric type করতে হবে

POS শুধু integer quantity নয়।

```text
1 item
2 item
0.5 kg
1.250 litre
3.75 meter
```

তাই:

```nil
quantity
```

কে money থেকে আলাদা semantics দিতে হবে।

যেমন:

```nil
quantity: decimal
unit_price: money
```

এবং:

```nil
line_total = unit_price * quantity
```

compiler যেন type mismatch ধরে।

---

# ৭. Tax engine চাই

POS-এর অন্যতম core module।

```text
Tax
TaxRate
TaxCategory
TaxRule
Inclusive tax
Exclusive tax
Compound tax
Exemption
Rounding
```

যেমন:

```nil
tax = Tax.calculate(
    amount,
    rate,
    inclusive: false
)
```

একই codebase-এ দেশ/রাজ্য/পণ্যের ধরন অনুযায়ী tax rule বদলানো যাবে।

---

# ৮. Discount engine চাই

শুধু:

```text
price - discount
```

না।

লাগবে:

```text
percentage discount
fixed discount
item discount
cart discount
category discount
customer discount
coupon
buy X get Y
tier pricing
time-based promotion
```

এগুলো business-rule engine হিসেবে বানাতে হবে।

---

# ৯. POS Cart-কে first-class runtime object করতে হবে

এটা অত্যন্ত গুরুত্বপূর্ণ।

```nil
cart {
    items
    subtotal
    discount
    tax
    total
}
```

কিন্তু আরও:

```text
addItem
removeItem
changeQuantity
applyDiscount
removeDiscount
hold
resume
clear
calculate
```

এবং state update reactive হতে হবে।

উদাহরণ:

```nil
cart.add(product, quantity: 2)

cart.total
```

বদলালেই UI-এর relevant অংশ automatically rerender হবে।

---

# ১০. Reactive state system অনেক শক্তিশালী করতে হবে

POS-এ এরকম dependency থাকবে:

```text
Product search
      ↓
Selected product
      ↓
Cart
      ↓
Subtotal
      ↓
Discount
      ↓
Tax
      ↓
Grand Total
      ↓
Payment due
      ↓
Change
```

এখানে React-এর মতো granular reactivity দরকার।

Alap-এ থাকতে হবে:

```nil
state
computed
derived
watch
effect
async state
resource
cache
```

যেমন:

```nil
computed total = cart.subtotal + cart.tax - cart.discount
```

`cart` বদলালে total নিজে update হবে।

---

# ১১. Async programming model ঠিক করতে হবে

Next/Node application-এ async হলো মূল বিষয়।

NilLang-এ থাকতে হবে:

```nil
async function loadProducts()
await Product.find(...)
```

এবং:

```nil
try {
    await saveSale()
} catch error {
    ...
}
```

এর সঙ্গে:

```text
Promise/Future
Cancellation
Timeout
Retry
Parallel execution
Race handling
Task
Scheduler
```

প্রয়োজন।

---

# ১২. Form system বানাতে হবে

POS UI-এর অর্ধেকই form।

Alap-এ first-class:

```text
Form
Field
Input
Select
Combobox
Autocomplete
Checkbox
Radio
Date
Time
Number
MoneyInput
BarcodeInput
```

এবং:

```nil
form ProductForm {
    sku required
    name required minLength 2
    price money required
}
```

Validation client এবং server দুই জায়গাতেই একই definition থেকে হবে।

---

# ১৩. Table/DataGrid না থাকলে POS অসম্ভব

Full POS-এ লাগবে:

```text
DataGrid
Virtual scrolling
Column resize
Column reorder
Sorting
Filtering
Multi-select
Keyboard navigation
Inline edit
Pagination
Sticky header
Frozen column
Empty state
Loading state
Error state
```

এটা অত্যন্ত polished হতে হবে।

কারণ:

```text
Sales
Products
Customers
Inventory
Purchases
Suppliers
Reports
```

সবই table-heavy।

---

# ১৪. Combobox + instant search অত্যন্ত শক্তিশালী করতে হবে

Cashier-এর workflow:

```text
barcode scan
→ product
→ cart
```

অথবা:

```text
search "rice"
→ results
→ click
→ cart
```

UI-তে milliseconds-level interaction দরকার।

তাই Alap UI runtime-এ:

```text
debounced search
cancel previous request
keyboard selection
highlight match
virtualized result
barcode input
```

support দরকার।

---

# ১৫. Router-কে Next.js-level application router-এর দিকে নিতে হবে

শুধু route registration যথেষ্ট নয়।

লাগবে:

```text
nested routes
dynamic routes
route params
query params
layouts
loading
error
not-found
guards
redirect
navigation state
prefetch
data loading
```

উদাহরণ:

```text
/
 /login
 /pos
 /products
 /products/:id
 /inventory
 /sales
 /sales/:id
 /customers
 /suppliers
 /reports
 /settings
```

---

# ১৬. Middleware architecture চাই

Node ecosystem-এর মতো:

```text
request
 ↓
middleware
 ↓
auth
 ↓
permission
 ↓
validation
 ↓
handler
 ↓
response
```

NilLang-এ declarative করা যায়:

```nil
middleware auth {
    require session
}

middleware manager {
    require role "manager"
}
```

---

# ১৭. Authentication + Authorization সম্পূর্ণ করতে হবে

POS-এ login যথেষ্ট নয়।

লাগবে:

```text
User
Role
Permission
Session
Refresh token
Password hashing
MFA-ready architecture
Device/session management
Logout
Session expiry
```

Permissions:

```text
sale.create
sale.refund
product.edit
inventory.adjust
report.view
user.manage
settings.manage
```

এবং UI ও API দুই জায়গায় enforce হবে।

---

# ১৮. Audit log বাধ্যতামূলক

কে কী করল:

```text
User
Action
Entity
Entity ID
Before
After
Timestamp
Device
IP
Reason
```

উদাহরণ:

```text
Rahim
Refund Sale #10234
₹500
09:42 AM
```

POS-এর জন্য এটা optional luxury নয়।

---

# ১৯. Bengali support-কে শুধু Unicode support ভাবা যাবে না

“বাংলা সাপোর্ট” মানে:

```text
Unicode
UTF-8
Bengali fonts
Bengali locale
Translation
Plural rules
Number formatting
Currency formatting
Date formatting
Time formatting
Calendar formatting
Input methods
RTL-ready architecture
```

যদিও বাংলা LTR।

Alap i18n system:

```nil
t("sale.total")
```

আর locale:

```text
bn-BD
bn-IN
en-IN
en-US
```

Translation:

```text
sale.total = "মোট"
sale.pay = "পরিশোধ"
sale.change = "ফেরত"
```

---

# ২০. বাংলা সংখ্যা formatting-ও চাই

যেমন:

```text
৳ 1,250.00
```

বা locale অনুযায়ী:

```text
₹ ১,২৫০.০০
```

এগুলো formatting layer-এ controlled হওয়া চাই।

কোথাও manually string concatenate করা যাবে না।

---

# ২১. UI Design System বানাতে হবে

এটাই “সুন্দর UI”-এর ভিত্তি।

Alap-এর মধ্যে first-class:

```text
Theme
Color tokens
Typography
Spacing
Radius
Shadow
Motion
Icon
Breakpoint
Density
Dark mode
```

POS-এর জন্য বিশেষ:

```text
compact mode
touch mode
keyboard mode
large-screen mode
mobile mode
```

অর্থাৎ একই application:

```text
Desktop cashier
Tablet cashier
Mobile manager
```

সবখানেই usable।

---

# ২২. Keyboard-first interaction লাগবে

POS operator mouse দিয়ে সব করবে না।

যেমন:

```text
F2 → Search
F4 → Payment
F6 → Hold
F8 → Customer
Esc → Close
Enter → Confirm
Delete → Remove item
↑ ↓ → navigate
Ctrl+P → print
```

Framework-level keyboard shortcut system চাই:

```nil
shortcut "F4" {
    open payment
}
```

এটাই Alap-কে POS-ready করবে।

---

# ২৩. Focus management ঠিক করতে হবে

POS-এ barcode scanner সাধারণ keyboard-এর মতো input পাঠাতে পারে।

তাই:

```text
focus
blur
restore focus
trap focus
dialog focus
scanner focus
```

framework-এর built-in behaviour হওয়া উচিত।

---

# ২৪. Barcode subsystem চাই

কমপক্ষে:

```text
USB scanner
Bluetooth scanner
Camera scanner
EAN-13
EAN-8
UPC
Code128
QR
```

Device abstraction:

```nil
barcode.onScan {
    product = Product.findByBarcode(value)
}
```

---

# ২৫. Printer abstraction চাই

POS-এর ক্ষেত্রে PDF generate করলেই শেষ নয়।

লাগবে:

```text
Thermal printer
80mm
58mm
USB
Network
Bluetooth
ESC/POS
```

Framework API:

```nil
printer.receipt.print(receipt)
```

আর receipt template আলাদা।

---

# ২৬. Cash drawer support

Payment complete হলে:

```text
print receipt
+
open drawer
```

একই transaction flow-এর অংশ।

Alap device API:

```nil
cashDrawer.open()
```

---

# ২৭. Offline-first architecture করতে হবে

এটাই TypeScript/Next implementation-এর সঙ্গে সত্যিকারের parity আনার অন্যতম বড় জায়গা।

দোকানে internet চলে গেলে POS বন্ধ হওয়া যাবে না।

Architecture:

```text
UI
 ↓
Local State
 ↓
Local Database
 ↓
Sync Engine
 ↓
Server
```

অর্থাৎ:

```text
sale locally committed
↓
queue
↓
internet returns
↓
sync
```

---

# ২৮. Local database প্রথমে SQLite

তোর POS target-এর জন্য:

```text
SQLite
```

অবশ্যই first-class target কর।

Schema:

```text
products
inventory
customers
sales
sale_items
payments
users
shifts
registers
audit_logs
sync_queue
```

তারপর PostgreSQL server backend।

---

# ২৯. Sync engine আলাদা subsystem হবে

```text
local mutation
 ↓
sync queue
 ↓
server
 ↓
ack
 ↓
remove queue item
```

Conflict handling:

```text
version
timestamp
device id
operation id
```

অবশ্যই লাগবে।

---

# ৩০. POS transaction model নির্দিষ্ট করতে হবে

এটা:

```text
Cart
 ↓
Checkout
 ↓
Payment
 ↓
Sale
 ↓
Inventory decrement
 ↓
Receipt
```

একটি atomic business operation হতে হবে।

যেমন:

```text
BEGIN

create sale
create sale items
create payment
decrement stock
record audit
create receipt record

COMMIT
```

failure:

```text
ROLLBACK
```

---

# ৩১. Payment architecture আলাদা করতে হবে

```text
Cash
Card
UPI
Wallet
Mixed payment
Split payment
Refund
Partial refund
```

Payment provider abstraction:

```nil
payment.process(...)
```

কিন্তু provider implementation আলাদা।

---

# ৩২. Shift/Register system চাই

একটা প্রকৃত POS-এ:

```text
Register
Shift
Opening cash
Cash in
Cash out
Closing cash
Expected cash
Actual cash
Variance
```

এসব না থাকলে application POS হলেও complete retail POS হয় না।

---

# ৩৩. Inventory engine চাই

কমপক্ষে:

```text
stock on hand
stock reserved
stock available
stock adjustment
stock movement
stock transfer
low stock
reorder level
opening stock
purchase
sale
return
```

Stock history:

```text
+100 purchase
-2 sale
-1 damage
+5 adjustment
```

---

# ৩৪. Product catalogue পূর্ণাঙ্গ করতে হবে

```text
Product
Variant
SKU
Barcode
Category
Brand
Unit
Cost
Price
Tax
Discount
Image
Stock
Supplier
```

Variant support:

```text
T-Shirt
 ├── Small
 ├── Medium
 └── Large
```

---

# ৩৫. Customer subsystem

```text
Customer
Phone
Email
Address
Loyalty
Credit
Purchase history
Returns
```

এবং search instant হতে হবে।

---

# ৩৬. Supplier + Purchase subsystem

```text
Supplier
Purchase Order
Goods Received
Purchase Invoice
Supplier Payment
Purchase Return
```

---

# ৩৭. Returns/refunds ঠিকভাবে implement করতে হবে

POS-এর আসল কঠিন অংশগুলোর একটি।

```text
full refund
partial refund
item refund
quantity refund
payment refund
stock return
exchange
reason
authorization
```

---

# ৩৮. Reporting engine চাই

কমপক্ষে:

```text
Daily sales
Hourly sales
Product sales
Category sales
Cashier sales
Payment breakdown
Tax
Discount
Profit
Inventory
Low stock
Refund
Void
```

Query/API দুই স্তরেই reusable হতে হবে।

---

# ৩৯. Dashboard chart subsystem চাই

Alap UI-তে:

```text
Line chart
Bar chart
Pie/donut
KPI cards
Table
Trend
```

তারপর:

```nil
chart SalesByDay {
    ...
}
```

---

# ৪০. File/image upload system চাই

Products-এ image লাগবে।

অতএব:

```text
File
Upload
Multipart
Storage
Resize
Thumbnail
Cache
Delete
```

এবং local/server storage abstraction।

---

# ৪১. Next.js-এর মতো SSR/CSR distinction-এর সমতুল্য architecture চাই

Alap web target-এ শুধু client rendering করলে চলবে না।

প্রয়োজন অনুযায়ী:

```text
server-rendered page
client-reactive component
server action
API endpoint
static page
```

architecture থাকতে হবে।

---

# ৪২. NilLang compiler-এর কাজ এখন সবচেয়ে গুরুত্বপূর্ণ

তুই যতই framework বানাস, compiler যদি production-grade না হয়, সবকিছু কাগজে থাকবে।

তোর current compiler tree:

```text
lexer
parser
AST
types
typecheck
HIR
MIR
compiler
VM
WASM
```

এখন এটাকে একটানা validated pipeline করতে হবে:

```text
.nil
 ↓
Lexer
 ↓
Parser
 ↓
AST
 ↓
Name resolution
 ↓
Type checking
 ↓
Lowering
 ↓
HIR
 ↓
MIR
 ↓
Optimization
 ↓
Backend
 ├── Native
 ├── JVM? [প্রয়োজনে পরে]
 ├── WASM
 └── Bytecode/VM
```

সবচেয়ে বড় কাজ:

**প্রতিটি stage-এর মধ্যে একই semantics বজায় রাখা।**

---

# ৪৩. Type system শক্ত করতে হবে

POS-এর জন্য অন্তত:

```text
bool
int
decimal
money
string
datetime
date
duration
uuid
bytes
list<T>
map<K,V>
optional<T>
result<T,E>
entity
relation
```

এবং:

```text
generic
interface
trait
enum
union
pattern matching
```

---

# ৪৪. Error handling production-grade হওয়া চাই

শুধু generic exception না।

```nil
result SaleReceipt =
    checkout(cart)
```

এবং:

```nil
match result {
    Ok(receipt) => ...
    Err(PaymentFailed(error)) => ...
    Err(StockChanged(error)) => ...
}
```

POS-এর business errors type-safe হওয়া দরকার।

---

# ৪৫. Concurrency model-ও ঠিক করতে হবে

তোর ভাষার “modern concurrent” লক্ষ্য আছে।

POS backend-এ:

```text
requests
sync workers
background jobs
printing
notifications
inventory updates
```

concurrently চলবে।

কিন্তু database mutation safe হতে হবে।

---

# ৪৬. Background job runtime চাই

```text
daily report
sync
backup
cleanup
receipt retry
low stock notification
```

এর জন্য:

```nil
job DailyReport {
    ...
}
```

---

# ৪৭. Cache layer চাই

```text
memory cache
local cache
HTTP cache
database cache
```

কিন্তু cache invalidation deterministic হতে হবে।

---

# ৪৮. Networking abstraction চাই

```nil
http.get(...)
http.post(...)
websocket(...)
```

এবং:

```text
request
response
headers
cookies
multipart
stream
timeout
retry
```

---

# ৪৯. API client generation চাই

Entity থেকেই:

```text
server API
+
typed client
+
validation
```

generate হবে।

এটা NilLang/Alap-এর বড় competitive advantage হতে পারে।

---

# ৫০. Serialization system চাই

```text
JSON
binary
form-data
query parameters
```

type-aware serializer:

```nil
Money
DateTime
Decimal
UUID
Optional
Entity
```

সব properly serialize করবে।

---

# ৫১. Environment/config system চাই

```text
development
test
production
```

যেমন:

```text
DATABASE_URL
APP_ENV
PORT
SECRET_KEY
PRINTER
```

NilLang/Alap-এ standard config API চাই।

---

# ৫২. Secrets management

Source code-এ:

```text
password
API key
secret
token
```

রাখা যাবে না।

Runtime secret injection লাগবে।

---

# ৫৩. Testing framework লাগবে

TypeScript-এর Jest/Vitest style equivalent দরকার।

NilLang-এ:

```nil
test "cart total" {
    ...
}
```

তার সঙ্গে:

```text
unit test
integration test
database test
API test
UI test
snapshot test
property test
```

---

# ৫৪. End-to-end testing

যে POS application বানাবি সেখানে পুরো workflow machine-testable হতে হবে:

```text
login
→ search product
→ add cart
→ discount
→ checkout
→ cash
→ sale saved
→ stock reduced
→ receipt generated
```

---

# ৫৫. UI snapshot/visual regression system চাই

কারণ তুই বলেছিস:

> “হুবহু দেখতে কার্যকর পিওএস অ্যাপের মতো”

তাই visual regression অত্যন্ত দরকার।

```text
render page
↓
screenshot
↓
compare
↓
detect visual regression
```

---

# ৫৬. Accessibility

Production UI-তে:

```text
keyboard
focus
screen reader
contrast
labels
ARIA-equivalent semantics
```

প্রয়োজন।

---

# ৫৭. Responsive layout engine যথেষ্ট শক্তিশালী করতে হবে

```text
desktop
tablet
mobile
```

এবং:

```text
grid
flex
stack
split panel
sidebar
bottom sheet
```

first-class component হওয়া উচিত।

---

# ৫৮. Animation runtime “60 FPS” লেখা থাকলেই হবে না

বাস্তবে benchmark চাই।

```text
1000 rows
10000 products
100 cart updates
rapid typing
modal opening
large table scrolling
```

সব benchmark করতে হবে।

---

# ৫৯. Performance profiling

NilLang/Alap-এ:

```text
nil profile
```

আছে—এটাকে বাস্তব profiling system বানাতে হবে।

Measure:

```text
startup
compile
render
database query
API latency
memory
GC
frame time
```

---

# ৬০. Dev server-কে Next.js-এর মতো developer experience দিতে হবে

বর্তমান `nil dev`, web live server/hot reload এগিয়েছে।

এখন চাই:

```text
nil dev
```

চালালেই:

```text
watch
compile
incremental build
HMR
browser reload
error overlay
source mapping
terminal diagnostics
```

---

# ৬১. Error message-কে অসাধারণ করতে হবে

যেমন:

```text
error[NL2304]:

Expected Money but got Float64

  18 | total = price * quantity
                     ^^^^^^^^

price: Money
quantity: Float64

Hint:
Convert quantity to Decimal or use Money × Decimal.
```

Compiler error এমন হতে হবে যাতে নতুন developer বুঝতে পারে।

---

# ৬২. Debugger চাই

```text
breakpoint
step over
step into
watch
stack
variables
```

ভবিষ্যতে VS Code extension-এর জন্য DAP-level integration করা উচিত।

---

# ৬৩. Package manager বাস্তব করতে হবে

বর্তমান `nilpkg` concept আছে।

এখন চাই:

```text
registry
versioning
dependency resolution
lock file
integrity
signature
cache
offline packages
```

---

# ৬৪. Framework SDK আলাদা করা উচিত

আমি architecture-এ এভাবে ভাগ করতাম:

```text
nilLang
    ↓
Alap Core
    ↓
Alap UI
    ↓
Alap Web
    ↓
Alap Server
    ↓
Alap Data
    ↓
Alap Device
    ↓
Alap POS
```

যাতে language আর POS logic এক জিনিস না হয়ে যায়।

---

# ৬৫. সবচেয়ে গুরুত্বপূর্ণ—POS app-কে framework-এর integration test বানাও

এখানে তোর জন্য আসল পথটা আছে।

একটা **reference TypeScript POS** বানাবি।

তার feature list হবে canonical specification।

তারপর:

```text
Reference POS
      ↓
Behaviour specification
      ↓
Alap implementation
      ↓
NilLang implementation
```

---

# ৬৬. দুইটা implementation পাশাপাশি রাখতে হবে

Repository structure:

```text
pos/
├── reference/
│   └── typescript-next-node/
│
├── nilang/
│   └── alap-pos/
│
├── shared-spec/
│   ├── entities/
│   ├── workflows/
│   ├── validation/
│   ├── fixtures/
│   └── scenarios/
│
└── conformance/
    ├── api/
    ├── database/
    ├── business/
    └── ui/
```

এটা করলে:

**“TypeScript POS works, NilLang POS almost works”** ধরনের আত্মপ্রবঞ্চনা থাকবে না।

---

# ৬৭. Behaviour parity test বানাতে হবে

উদাহরণ:

```text
Scenario: cash sale

Given product A = 100
quantity = 2
discount = 10
tax = 5

When checkout with cash 200

Then:
subtotal = 200
discount = 10
tax = ...
total = ...
change = ...
inventory -= 2
sale exists
payment exists
receipt exists
```

এই একই test:

```text
TypeScript
```

এবং:

```text
NilLang
```

দুই implementation-এ run হবে।

---

# ৬৮. “একই UI” অর্জনের বাস্তব পদ্ধতি

তুই TypeScript POS-এর UI আগে pixel-level polish করবি।

ধর:

```text
Sidebar
Topbar
Product grid
Cart
Payment panel
Modal
Tables
Forms
Dashboard
```

তারপর প্রতিটি screen-এর জন্য:

```text
layout spec
spacing spec
typography spec
interaction spec
state spec
```

বানাবি।

তারপর Alap-এ একই screen rebuild।

---

# ৬৯. UI component parity matrix বানাতে হবে

যেমন:

| TypeScript/Next | Alap              |
| --------------- | ----------------- |
| Button          | Alap Button       |
| Input           | Alap Input        |
| Select          | Alap Select       |
| Combobox        | Alap Combobox     |
| Dialog          | Alap Dialog       |
| Drawer          | Alap Drawer       |
| Table           | Alap DataGrid     |
| Toast           | Alap Toast        |
| Form            | Alap Form         |
| Tabs            | Alap Tabs         |
| Sidebar         | Alap Sidebar      |
| Chart           | Alap Chart        |
| Modal           | Alap Modal        |
| Date Picker     | Alap DatePicker   |
| Barcode input   | Alap BarcodeInput |

**একটার replacement না থাকলে POS development থামবে।**

---

# ৭০. POS-specific Alap components আলাদা package কর

আমি করতাম:

```text
pkg/alap/pos/
├── cart
├── cashier
├── checkout
├── payment
├── receipt
├── barcode
├── inventory
├── product
├── customer
├── register
├── shift
└── reporting
```

এগুলো generic UI-এর উপর build হবে।

---

# ৭১. NilLang syntax-এর লক্ষ্য

শেষে developer experience যেন এমন হয়:

```nil
page POS {
    layout split {
        ProductSearch()
        Cart()
    }

    on barcode.scan(code) {
        cart.add(Product.byBarcode(code))
    }

    on checkout {
        sale.checkout()
    }
}
```

এটা ideally এমন কাজ করবে:

```text
UI
state
events
routing
API
DB
```

সব framework runtime handle করবে।

---

# ৭২. কিন্তু business logic language-এ explicit থাকবে

Framework magic অতিরিক্ত করা যাবে না।

যেমন:

```nil
service CheckoutService {

    checkout(cart, payment) -> Result<Sale, CheckoutError> {
        transaction {
            ...
        }
    }
}
```

এটা maintainable।

---

# ৭৩. “Generated magic” এবং “custom code” আলাদা রাখতে হবে

Entity declaration:

```nil
entity Product {...}
```

থেকে generated code হবে।

কিন্তু:

```nil
service PricingService {...}
```

মানুষ লিখবে।

এতে framework flexible থাকবে।

---

# ৭৪. POS-এর domain model আগে lock করতে হবে

আমি canonical domain এভাবে ধরতাম:

```text
Organization
Store
Register
Shift

User
Role
Permission

Product
Category
Brand
Unit
ProductVariant

Inventory
StockMovement
Warehouse

Customer
Supplier

Purchase
PurchaseItem

Sale
SaleItem
Payment
Refund
RefundItem

Discount
Tax

Receipt

AuditLog

SyncJob
```

এগুলো ছাড়া UI আগে polished করলে পরে architecture ভাঙতে হবে।

---

# ৭৫. যে জিনিসগুলো এখনই বন্ধ/পিছিয়ে রাখবি

এই পর্যায়ে নতুন করে আর:

```text
আরও AI feature
আরও exotic compiler feature
আরও OS abstraction
আরও experimental distributed system
আরও rendering feature
```

যোগ করা উচিত নয়—যতক্ষণ না:

```text
DB
+
API
+
UI
+
POS
+
Offline
+
Printer
+
Testing
```

বাস্তবে কাজ করে।

তোর repository-তে AI Compiler Oracle, HIR/MIR, WASM ইত্যাদি ইতিমধ্যে ঢুকে গেছে; এখন **breadth নয়, integration depth** দরকার।

---

# ৭৬. আমি কাজটাকে এই ৮টা milestone-এ ভাগ করতাম

### M1 — Language Core

```text
type system
async
error handling
generics
pattern matching
collections
datetime
decimal
money
```

### M2 — Application Core

```text
router
state
form
validation
i18n
HTTP
API
auth
```

### M3 — Data Core

```text
SQLite
PostgreSQL
ORM
transactions
relations
migration
query builder
```

### M4 — UI Core

```text
DataGrid
Form
Modal
Drawer
Combobox
DatePicker
Charts
Keyboard
Responsive
Theme
```

### M5 — Device Core

```text
barcode
printer
cash drawer
camera
USB/Bluetooth
```

### M6 — POS Domain

```text
products
inventory
cart
sales
payment
customer
supplier
purchase
tax
discount
shift
register
refund
receipt
reports
```

### M7 — Production

```text
offline
sync
audit
permissions
backup
logging
monitoring
testing
security
```

### M8 — Reference Parity

```text
TypeScript POS
          ↕
Conformance suite
          ↕
NilLang/Alap POS
```

---

# ৭৭. আর সবচেয়ে গুরুত্বপূর্ণ সিদ্ধান্ত

**Reference application আগে বানাবি, framework পরে তার gaps পূরণ করবে।**

মানে:

### ভুল পথ

```text
আগে Alap-এ ২০০ feature
তারপর POS
```

### সঠিক পথ

```text
Real POS requirement
        ↓
একটা workflow
        ↓
Alap/NilLang-এ প্রয়োজনীয় primitive
        ↓
Implement
        ↓
Test
        ↓
পরের workflow
```

---

# ৭৮. প্রথম vertical slice ঠিক কেমন হবে

আমি প্রথমেই পুরো POS বানাতে যাব না।

প্রথম production-grade slice:

```text
LOGIN
  ↓
PRODUCT SEARCH
  ↓
BARCODE SCAN
  ↓
ADD TO CART
  ↓
CHANGE QUANTITY
  ↓
DISCOUNT
  ↓
TAX
  ↓
TOTAL
  ↓
CASH PAYMENT
  ↓
CHANGE
  ↓
ATOMIC SALE TRANSACTION
  ↓
STOCK DECREMENT
  ↓
RECEIPT
  ↓
PRINT
  ↓
AUDIT LOG
```

এটা **TypeScript version এবং NilLang version দুইটাতেই 100% কাজ করাতে হবে**।

তারপর:

```text
refund
purchase
inventory
customer
reports
offline sync
```

---

# ৭৯. শেষ target architecture

শেষে আমি তোর ecosystem-কে এভাবে দেখতে চাই:

```text
                    NILANG
                       │
             ┌─────────┴─────────┐
             │                   │
         Compiler             Runtime
             │                   │
        HIR → MIR         ┌───────┴────────┐
             │            │                │
        Native/WASM      Alap             VM
                          │
        ┌─────────────────┼──────────────────┐
        │                 │                  │
      Alap UI         Alap Server        Alap Data
        │                 │                  │
        ├── Forms         ├── HTTP           ├── SQLite
        ├── Tables        ├── Auth           ├── PostgreSQL
        ├── Charts        ├── WebSocket      ├── ORM
        ├── Modal         ├── Jobs           ├── Migration
        └── Theme         └── API            └── Sync
                          │
                     Alap Device
                          │
              ┌───────────┼───────────┐
              │           │           │
           Barcode     Printer     Cash Drawer
                          │
                       Alap POS
                          │
       ┌──────────────────┼───────────────────┐
       │                  │                   │
    Sales              Inventory          Reporting
       │                  │                   │
       └──────────────────┼───────────────────┘
                          │
                   Production POS
```

## তাহলে “TypeScript POS = NilLang Alap POS” সত্যি কীভাবে প্রমাণ হবে?

শুধু screenshot দিয়ে নয়।

এই ৫টা জিনিস একসঙ্গে pass করতে হবে:

```text
1. Visual parity
2. Interaction parity
3. Business-logic parity
4. Data/API parity
5. Performance/reliability parity
```

তখন বলতে পারবি:

> **এই POS-এর reference implementation TypeScript/Next.js/Node.js-এ আছে; একই product contract NilLang + Alap-এও পূর্ণভাবে implement করা হয়েছে।**

আর তখন NilLang আর “toy language” থাকবে না।
**তখন NilLang + Alap নিজেই একটি application platform হবে।**

তোর বর্তমান `nilLang`-এর অবস্থান দেখে আমার মতে **সবচেয়ে জরুরি নতুন feature এখন আর “আরেকটা compiler feature” নয়—Alap-এর মধ্যে `UI + state + data + transaction + device + POS domain`-এর production integration শেষ করা।** সর্বশেষ `pos-app` commit-টাও এই দিকেই গেছে।

পরের বাস্তব কাজ হিসেবে আমি চাইলে এই লক্ষ্য ধরে **`nilLang` repo-র জন্য একটি সম্পূর্ণ “TypeScript/Next.js POS → NilLang/Alap parity specification”** বানিয়ে দিতে পারি—ফাইল/ডিরেক্টরি, NilLang syntax, Alap API, database schema, UI component তালিকা, compiler changes এবং implementation order সহ।


অবশ্যই। এবার এটাকে **আলাপ-ভিত্তিক POS বানানোর সম্পূর্ণ engineering specification** হিসেবে ধরছি—শুধু feature list নয়, **কোন layer-এ কী পরিবর্তন, কোন ফাইল/প্যাকেজ লাগবে, NilLang syntax কেমন হবে, এবং কোন ক্রমে implement করলে TypeScript/Next.js/Node POS-এর সঙ্গে parity পাওয়া যাবে**—সব।

# NilLang + Alap POS — Production Parity Specification

Reference target:

[বর্তমান NilLang repository](https://github.com/joysriramsarkar/nilLang?utm_source=chatgpt.com)

বর্তমান repo-তে ইতিমধ্যে Entity, data/ORM/migration/Money, routing, compiler-এর HIR/MIR/WASM pipeline এবং POS-oriented CLI scaffolding এসেছে। তাই আমরা একেবারে শূন্য থেকে শুরু করছি না। এখন লক্ষ্য হলো এগুলোকে **একটি coherent runtime-এর মধ্যে জোড়া দেওয়া**।

---

# ১. চূড়ান্ত architecture

```text
nilLang/
│
├── cmd/
│   ├── nil/
│   ├── nilc/
│   ├── nil-runner/
│   ├── nilpkg/
│   ├── nilkey/
│   └── softbusd/
│
├── compiler/
│   ├── token/
│   ├── lexer/
│   ├── ast/
│   ├── parser/
│   ├── types/
│   ├── typecheck/
│   ├── diagnostics/
│   ├── hir/
│   ├── mir/
│   ├── optimizer/
│   ├── compiler/
│   ├── code/
│   ├── vm/
│   └── wasm/
│
├── runtime/
│   ├── async/
│   ├── errors/
│   ├── collections/
│   ├── datetime/
│   ├── decimal/
│   ├── money/
│   ├── serialization/
│   ├── io/
│   └── process/
│
├── pkg/
│   └── alap/
│       ├── core/
│       ├── ui/
│       ├── state/
│       ├── forms/
│       ├── routing/
│       ├── data/
│       ├── entity/
│       ├── server/
│       ├── security/
│       ├── realtime/
│       ├── i18n/
│       ├── device/
│       ├── storage/
│       ├── sync/
│       ├── jobs/
│       ├── testing/
│       └── pos/
│
├── templates/
│   ├── web/
│   └── pos/
│
├── examples/
│   └── pos/
│
└── docs/
    ├── language/
    ├── alap/
    ├── pos/
    └── conformance/
```

---

# ২. প্রথম বড় পরিবর্তন: Alap Core

বর্তমান component/state ভিত্তিকে expand করে:

```text
Alap Core
├── Component
├── Props
├── State
├── Computed
├── Effect
├── Event
├── Context
├── Lifecycle
├── Async
├── Resource
├── Error
└── Dependency Injection
```

## NilLang API

```nil
component ProductCard {
    prop product: Product

    render {
        Card {
            Text(product.name)
            MoneyText(product.price)

            Button("যোগ করুন") {
                emit addToCart(product)
            }
        }
    }
}
```

এখানে `component`, `state`, `render`, `emit` যেন language-level বা framework-level first-class construct হয়।

---

# ৩. Reactive state engine

React-এর state-এর equivalent শুধু `state` keyword দিয়ে শেষ নয়।

প্রয়োজন:

```nil
state cart: Cart
state search: string = ""

computed subtotal = cart.subtotal
computed total = cart.total

effect {
    if cart.changed {
        saveDraft()
    }
}
```

Runtime-এ dependency graph:

```text
cart.items
   ↓
subtotal
   ↓
discount
   ↓
tax
   ↓
total
   ↓
paymentDue
   ↓
change
```

শুধু যেটা পরিবর্তিত হয়েছে সেটাই rerender হবে।

---

# ৪. Component lifecycle

```nil
component POS {
    on mount {
        loadProducts()
    }

    on unmount {
        cancelRequests()
    }

    on focus {
        focusBarcode()
    }
}
```

Runtime:

```text
mount
update
render
effect
focus
blur
unmount
```

---

# ৫. UI primitive library

Alap-এর মধ্যে প্রথম-class components:

```text
Button
IconButton
Input
NumberInput
MoneyInput
SearchInput
BarcodeInput

Select
Combobox
Autocomplete
Checkbox
Radio
Switch

Form
FormField
ValidationMessage

Table
DataGrid
VirtualList
Pagination

Card
Panel
Stack
Grid
SplitPane

Modal
Dialog
Drawer
Popover
Tooltip

Tabs
Accordion
Menu
Dropdown

Toast
Alert
Banner

DatePicker
TimePicker
DateTimePicker

Image
Avatar
Badge
Progress
Spinner

Chart
KPI
StatCard
```

---

# ৬. POS-specific UI components

তারপর:

```text
POSLayout
ProductSearch
ProductGrid
ProductCard
Cart
CartItem
CartSummary

PaymentPanel
PaymentMethod
CashPayment
CardPayment
UPIPayment
SplitPayment

CustomerPicker
DiscountEditor
TaxSummary

ReceiptPreview
ReceiptTemplate

RegisterStatus
ShiftPanel
CashDrawerControl

BarcodeScanner
```

---

# ৭. Design system

এক জায়গায়:

```nil
theme POS {
    typography ...
    spacing ...
    radius ...
    density ...
    motion ...
}
```

Support:

```text
light
dark
high-contrast
compact
comfortable
touch
keyboard
```

---

# ৮. Layout system

Next.js/Tailwind-এর উপর নির্ভর না করে Alap-এর নিজের layout primitives:

```nil
layout POS {
    sidebar width: 240px

    main {
        ProductArea()
    }

    aside width: 380px {
        Cart()
    }
}
```

Responsive:

```nil
when width < 900 {
    Cart -> Drawer
}
```

---

# ৯. Form system

```nil
form ProductForm {
    sku: string required
    name: string required minLength 2
    price: money required min 0
    stock: decimal min 0
}
```

একই schema:

```text
UI validation
+
server validation
+
database validation
+
API validation
```

---

# ১০. Validation engine

Built-in:

```text
required
min
max
minLength
maxLength
email
regex
uuid
url
numeric
integer
decimal
money
date
custom
```

Custom:

```nil
validate sku {
    unique Product.sku
}
```

---

# ১১. Entity system-এর নতুন পূর্ণ syntax

তোর বর্তমান Entity system-এর উপর:

```nil
entity Product {
    id: uuid primary

    sku: string {
        required
        unique
        indexed
        searchable
    }

    name: string {
        required
        searchable
    }

    description: string?
    category: Category

    cost: money
    price: money

    stock: quantity
    unit: Unit

    active: bool = true

    created_at: datetime auto
    updated_at: datetime auto
}
```

এটা generate করবে:

```text
SQL
ORM
REST
JSON
Type model
Forms
Tables
Validation
Search
API docs
```

---

# ১২. Relations

```nil
entity Sale {
    customer: Customer?
    items: SaleItem[]
    payments: Payment[]
}
```

এখানে compiler/type system বুঝবে:

```text
Customer?
SaleItem[]
Payment[]
```

---

# ১৩. ORM API

```nil
Product.find(id)

Product.findBy(sku: "ABC123")

Product.where(active: true)

Product
    .where(category: categoryID)
    .order(name: asc)
    .paginate(page: 1, size: 50)
```

CRUD:

```nil
product = Product.create({...})

product.update({...})

product.delete()
```

---

# ১৪. Query builder

প্রয়োজন:

```nil
Product
    .select(...)
    .where(...)
    .join(...)
    .groupBy(...)
    .having(...)
    .order(...)
    .limit(...)
    .offset(...)
```

Prepared statements বাধ্যতামূলক।

---

# ১৫. Transaction API

সবচেয়ে গুরুত্বপূর্ণ:

```nil
transaction {
    sale = Sale.create(data)

    for item in cart.items {
        SaleItem.create(...)
        Inventory.decrease(...)
    }

    Payment.create(...)
}
```

যেকোনো error:

```text
ROLLBACK
```

সব operation atomic।

---

# ১৬. Database targets

প্রথম:

```text
SQLite
```

তারপর:

```text
PostgreSQL
```

Architecture:

```text
Alap Data API
       │
       ├── SQLite Driver
       └── PostgreSQL Driver
```

Application code driver-specific হবে না।

---

# ১৭. Migration system

```nil
migration "create_products" {
    up {
        ...
    }

    down {
        ...
    }
}
```

CLI:

```text
nil db migrate
nil db rollback
nil db status
nil db create
```

তোর বর্তমান CLI-র `db migrate|rollback` ভিত্তিটা এখানেই expand করতে হবে।

---

# ১৮. Money + Decimal redesign

বর্তমান `Money` ভালো ভিত্তি।

কিন্তু final API:

```nil
price: money
quantity: decimal
rate: decimal
```

Operations:

```nil
subtotal = price * quantity
discount = subtotal * rate
tax = taxable * taxRate
total = subtotal - discount + tax
```

**Financial calculation-এর core path-এ float64 থাকবে না।**

---

# ১৯. POS domain schema

এটাই canonical schema:

```text
Organization
Store
Register
Shift

User
Role
Permission

Category
Brand
Unit
Product
ProductVariant

Inventory
StockMovement
Warehouse

Customer

Supplier
Purchase
PurchaseItem

Sale
SaleItem
Payment
Refund
RefundItem

Tax
Discount
Promotion

Receipt

AuditLog

SyncOperation
```

---

# ২০. Sale model

```nil
entity Sale {
    id: uuid primary
    invoice_number: string unique

    register: Register
    cashier: User
    customer: Customer?

    subtotal: money
    discount: money
    tax: money
    total: money

    status: SaleStatus

    items: SaleItem[]
    payments: Payment[]

    created_at: datetime auto
}
```

---

# ২১. Sale state machine

```text
Draft
  ↓
PendingPayment
  ↓
Paid
  ↓
Completed
```

Alternative:

```text
Cancelled
Refunded
PartiallyRefunded
```

Invalid transitions compiler/runtime reject করবে।

---

# ২২. Cart

```nil
cart.add(product, quantity: 2)

cart.remove(item)

cart.setQuantity(item, 3)

cart.applyDiscount(...)

cart.subtotal
cart.tax
cart.total
```

Cart হবে reactive state object।

---

# ২৩. Checkout service

```nil
service Checkout {

    execute(cart, payments) -> Result<Receipt, CheckoutError> {

        transaction {
            validateStock(cart)
            calculate(cart)
            createSale(cart)
            createPayments(payments)
            decrementInventory(cart)
            createAudit()
            createReceipt()
        }
    }
}
```

এটাই POS-এর heart।

---

# ২৪. Payment model

```nil
enum PaymentMethod {
    Cash
    Card
    UPI
    Wallet
    Other
}
```

একাধিক payment:

```nil
payments [
    cash: 500,
    upi: 250
]
```

---

# ২৫. Cash calculation

```nil
due = sale.total

received = cashReceived

change = received - due
```

Negative হলে payment incomplete।

---

# ২৬. Inventory transaction

Sale সফল হলে:

```text
StockMovement:
    type = SALE
    quantity = -2
```

Purchase:

```text
quantity = +100
```

Refund:

```text
quantity = +2
```

Manual adjustment:

```text
quantity = +/-N
```

Stock কখনো সরাসরি mutate না করে movement ledger-এর মাধ্যমে পরিবর্তন করা ভালো।

---

# ২৭. Barcode

API:

```nil
barcode.onScan(code) {
    product = Product.findByBarcode(code)

    if product != null {
        cart.add(product)
    } else {
        notify("পণ্য পাওয়া যায়নি")
    }
}
```

প্রথমে keyboard-emulation scanner support কর।

তারপর:

```text
USB
Bluetooth
Camera
```

---

# ২৮. Printer

```nil
receipt.print()
```

Device abstraction:

```text
Printer
 ├── ESC/POS
 ├── Network
 ├── USB
 └── Bluetooth
```

Receipt width:

```text
58mm
80mm
```

---

# ২৯. Cash drawer

```nil
cashDrawer.open()
```

Checkout success-এর পর configurable hook:

```nil
on sale.completed {
    printer.print(receipt)
    cashDrawer.open()
}
```

---

# ৩০. Offline architecture

এটা আলাদা করে বানাতে হবে:

```text
Alap POS UI
      ↓
Application State
      ↓
Local Repository
      ↓
SQLite
      ↓
Sync Queue
      ↓
Server API
      ↓
PostgreSQL
```

Internet নেই:

```text
Sale → SQLite → Sync Queue
```

Internet ফিরে এলে:

```text
Sync Queue → Server
```

---

# ৩১. Sync protocol

প্রতিটি mutation-এর:

```text
operation_id
device_id
entity_id
entity_type
operation
version
timestamp
payload
```

থাকবে।

Server idempotent হতে হবে।

একই operation দুবার পাঠালেও duplicate sale হবে না।

---

# ৩২. Authentication

```nil
login(username, password)
logout()
currentUser()
```

Session:

```text
access
refresh
expiry
device
```

Password hashing server runtime-এর দায়িত্ব।

---

# ৩৩. RBAC

```nil
role Cashier {
    allow sale.create
    allow customer.read
}

role Manager {
    allow sale.*
    allow inventory.*
    allow report.*
}

role Admin {
    allow *
}
```

---

# ৩৪. UI permission

যদি:

```text
sale.refund
```

permission না থাকে:

```nil
if can("sale.refund") {
    RefundButton()
}
```

কিন্তু **শুধু UI hide করা যাবে না**।

API/service layer-এও permission check হবে।

---

# ৩৫. Audit

```nil
audit.record {
    action: "sale.refund"
    entity: sale.id
    reason: reason
}
```

Before/after snapshot রাখার ব্যবস্থা থাকবে।

---

# ৩৬. i18n

```nil
Text(t("sale.total"))
```

Translation:

```text
locales/
├── bn-BD/
│   └── pos.json
├── bn-IN/
│   └── pos.json
└── en-IN/
    └── pos.json
```

---

# ৩৭. Bengali formatting

একটি central locale service:

```nil
format.money(amount)
format.number(quantity)
format.date(date)
format.time(time)
```

তাহলে UI-তে:

```text
৳ 1,250.00
```

এর formatting manually করতে হবে না।

---

# ৩৮. Routing

```text
/login

/pos

/products
/products/:id

/inventory
/inventory/movements

/sales
/sales/:id

/purchases
/purchases/:id

/customers
/suppliers

/reports

/settings
```

Nested layout ও route guards লাগবে।

---

# ৩৯. API

Entity থেকে automatic CRUD হলেও POS business operation আলাদা explicit service API হবে।

যেমন:

```text
GET    /api/products
GET    /api/products/:id

POST   /api/sales
POST   /api/sales/:id/refund

POST   /api/checkout

GET    /api/reports/daily-sales
```

---

# ৪০. Realtime

যদি দুই cashier একই inventory ব্যবহার করে:

```text
Cashier A sells Product X
        ↓
Server
        ↓
Cashier B
        ↓
stock update
```

WebSocket/SSE abstraction লাগবে।

---

# ৪১. Notification system

```nil
notify.success("বিক্রয় সম্পন্ন হয়েছে")
notify.error("স্টক যথেষ্ট নেই")
notify.warning("স্টক কমে এসেছে")
```

Toast runtime-level component।

---

# ৪২. Keyboard system

```nil
shortcut "F2" {
    focus(productSearch)
}

shortcut "F4" {
    open(payment)
}

shortcut "F6" {
    holdCart()
}

shortcut "ESC" {
    closeCurrentDialog()
}
```

Global + scoped shortcut দুটোই চাই।

---

# ৪৩. Focus system

```nil
focus(barcodeInput)
restoreFocus()
trapFocus(dialog)
```

Cashier workflow-এর জন্য অত্যন্ত গুরুত্বপূর্ণ।

---

# ৪৪. DataGrid

এটাকে খুব গুরুত্ব দে।

Required:

```text
virtualization
sorting
filtering
pagination
column resize
column visibility
keyboard navigation
selection
inline edit
loading
empty
error
```

হাজার হাজার product render করেও UI sluggish হওয়া চলবে না।

---

# ৪৫. Search engine

Product search-এর জন্য:

```text
SKU
Barcode
Name
Category
Brand
```

prefix + fuzzy search support করা যায়।

প্রথমে database indexed search যথেষ্ট।

---

# ৪৬. Reporting

Query layer-এর উপর:

```text
SalesReport
ProductReport
InventoryReport
PaymentReport
TaxReport
CashierReport
ProfitReport
```

একই report data:

```text
Dashboard
Table
Chart
Export
```

সবখানে ব্যবহার হবে।

---

# ৪৭. Export

কমপক্ষে:

```text
CSV
JSON
PDF
Print
```

Export API framework-level হওয়া ভালো।

---

# ৪৮. Error architecture

```nil
Result<T, E>
```

Business errors:

```text
ProductNotFound
InsufficientStock
InvalidQuantity
PaymentInsufficient
PaymentFailed
Unauthorized
Conflict
DatabaseError
SyncConflict
```

---

# ৪৯. Async

NilLang-এ:

```nil
async function loadProducts() {
    return await Product.query(...)
}
```

এবং cancellation:

```nil
task.cancel()
```

Search-এর ক্ষেত্রে:

```text
query A
query B
query C
```

A/B cancel করে শুধু C-এর result ব্যবহার করা যাবে।

---

# ৫০. Background jobs

```nil
job Sync {
    sync.pending()
}

job DailyBackup {
    backup.database()
}
```

---

# ৫১. Testing

NilLang test syntax:

```nil
test "cash checkout" {
    product = fixture.product(price: 100)

    cart.add(product, 2)

    result = checkout(cart, cash: 250)

    assert result.total == 200
    assert result.change == 50
}
```

---

# ৫২. Conformance test

এটাই TypeScript parity-এর মূল ব্যবস্থা।

```text
conformance/
├── checkout/
├── discount/
├── tax/
├── inventory/
├── refund/
├── payment/
└── authentication/
```

প্রতিটি scenario:

```text
input
expected state
expected response
expected database state
```

TypeScript implementation ও NilLang implementation দুইটিতে চালানো হবে।

---

# ৫৩. Visual parity test

প্রতিটি screen:

```text
Login
POS
Products
Inventory
Sales
Customers
Reports
Settings
```

এর screenshot comparison থাকবে।

Target:

```text
reference screenshot
        ↕
Alap screenshot
```

---

# ৫৪. Performance target

POS-এর জন্য target metrics define কর:

```text
Initial UI load       < 2 sec
Product search        < 100 ms local
Cart update           < 16 ms target
Checkout UI response  < 100 ms
SQLite transaction    < 50 ms typical
1000 product list     smooth
10,000 product list   virtualized
```

এগুলো absolute hardware-independent guarantee নয়—benchmark target।

---

# ৫৫. CLI

বর্তমান CLI-কে POS workflow-এর জন্য বাড়ানো যায়:

```text
nil create pos my-store

nil dev

nil db migrate

nil db seed

nil test

nil test:e2e

nil test:visual

nil build --target web

nil build --target android

nil build --target desktop
```

---

# ৫৬. POS project scaffold

`nil create pos` দিলে:

```text
my-store/
├── alap.yaml
├── app.nil
├── entities/
│   ├── product.nil
│   ├── sale.nil
│   ├── customer.nil
│   └── inventory.nil
│
├── pages/
│   ├── login.nil
│   ├── pos.nil
│   ├── products.nil
│   ├── sales.nil
│   └── reports.nil
│
├── services/
│   ├── checkout.nil
│   ├── pricing.nil
│   └── inventory.nil
│
├── migrations/
│
├── locales/
│   ├── bn-BD/
│   └── en-IN/
│
└── tests/
```

---

# ৫৭. `alap.yaml`

যেমন:

```yaml
name: my-store
type: pos

runtime:
  target: web

database:
  driver: sqlite

locale:
  default: bn-BD

features:
  offline: true
  barcode: true
  printer: true
  cash_drawer: true
  realtime: true
```

---

# ৫৮. Android target

পরে:

```text
NilLang
 ↓
Alap
 ↓
Android runtime
 ↓
APK
```

POS Android app-এ:

```text
SQLite
Bluetooth printer
Bluetooth scanner
camera
network
```

native bridge-এর মাধ্যমে expose করতে হবে।

---

# ৫৯. Web target

Browser-এর জন্য:

```text
NilLang
 ↓
Alap Web
 ↓
WASM / JS-compatible runtime
```

তবে device functionality browser limitations অনুযায়ী abstraction-এর মাধ্যমে handle হবে।

---

# ৬০. Native desktop target

POS-এর জন্য desktop খুব গুরুত্বপূর্ণ:

```text
Windows
Linux
```

তাই:

```text
nil build --target windows
```

দিয়ে native POS binary/package পাওয়া উচিত।

---

# ৬১. এখন সবচেয়ে গুরুত্বপূর্ণ compiler changes

এগুলো না করলে উপরের সুন্দর syntax শুধু syntax থাকবে।

Compiler-এ implement করতে হবে:

```text
AST:
  ComponentDeclaration
  EntityDeclaration
  ServiceDeclaration
  PageDeclaration
  FormDeclaration
  JobDeclaration

Expressions:
  await
  match
  optional
  generic
  query
  transaction

Types:
  Money
  Decimal
  Date
  DateTime
  UUID
  Entity
  Relation
  Result
  Optional
```

তারপর HIR/MIR lowering।

---

# ৬২. Compiler-এ framework intrinsics

Alap-এর runtime call যেন compiler বুঝতে পারে:

```text
render
state
emit
await
transaction
query
route
permission
```

তাহলে compiler optimization করতে পারবে।

---

# ৬৩. Type safety example

এই code:

```nil
price: money
quantity: decimal

total = price * quantity
```

valid।

কিন্তু:

```nil
price + quantity
```

invalid।

Compiler:

```text
Money + Decimal is not defined.
```

এটাই production language-এর behaviour।

---

# ৬৪. Source maps/debugging

Compiled code থেকে runtime error যেন:

```text
checkout.nil:42
```

দেখায়।

না যে:

```text
vm.go:1837
```

Developer-এর কাছে।

---

# ৬৫. Package structure

Generic Alap এবং POS আলাদা:

```text
pkg/alap/
pkg/alap/pos/
```

এতে অন্য developer POS ছাড়াও Alap দিয়ে CRM, ERP, accounting, SaaS বানাতে পারবে।

এটাই দীর্ঘমেয়াদে তোর framework-এর আসল শক্তি।

---

# ৬৬. কোনটা আগে করবি — একদম নির্দিষ্ট order

এখানে ভুল করলে আবার project ছড়িয়ে যাবে।

## Phase 1 — Foundation

```text
1. Decimal
2. Money
3. Quantity
4. DateTime
5. Result/Error
6. Async/Await
7. Type system
```

## Phase 2 — Data

```text
8. SQLite driver
9. ORM
10. Transactions
11. Relations
12. Migration
13. Query builder
```

## Phase 3 — Application

```text
14. Router
15. HTTP
16. API
17. Forms
18. Validation
19. Auth
20. Permissions
21. i18n
```

## Phase 4 — UI

```text
22. Layout
23. Input
24. Form
25. Modal
26. Combobox
27. DataGrid
28. Toast
29. Charts
30. Keyboard
31. Focus
```

## Phase 5 — POS

```text
32. Product
33. Inventory
34. Cart
35. Pricing
36. Tax
37. Discount
38. Sale
39. Payment
40. Receipt
41. Customer
42. Register
43. Shift
44. Refund
```

## Phase 6 — Devices

```text
45. Barcode
46. Printer
47. Cash drawer
48. Camera
49. Bluetooth
```

## Phase 7 — Production

```text
50. Offline
51. Sync
52. Audit
53. Backup
54. Jobs
55. Realtime
56. Monitoring
```

## Phase 8 — Parity

```text
57. Reference TypeScript POS
58. Shared fixtures
59. API conformance
60. Business conformance
61. UI snapshots
62. Performance benchmarks
```

---

# ৬৭. সবচেয়ে গুরুত্বপূর্ণ প্রথম milestone

সব feature একসঙ্গে লিখবি না।

প্রথমে এইটা **পুরোপুরি কাজ করাবি**:

```text
                 POS VERTICAL SLICE #1

Login
  ↓
Product database
  ↓
Product search
  ↓
Barcode
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
Cash payment
  ↓
Change
  ↓
SQLite transaction
  ↓
Sale
  ↓
Inventory decrement
  ↓
Receipt
  ↓
Printer
  ↓
Audit
```

এটা শেষ হলে তোর Alap আর “demo framework” থাকবে না।

---

# ৬৮. Definition of Done

কোনো feature “done” বলা যাবে না যতক্ষণ না:

```text
[ ] NilLang syntax exists
[ ] Parser supports it
[ ] Type checker supports it
[ ] HIR supports it
[ ] MIR supports it
[ ] Runtime supports it
[ ] Alap API supports it
[ ] Tests exist
[ ] Documentation exists
[ ] POS uses it
[ ] Error handling exists
[ ] Bengali UI works
[ ] Web target works
[ ] Desktop/Android target considered
```

---

# ৬৯. এবং তোর মূল লক্ষ্যটি এভাবেই লিখে রাখ

```text
                    REFERENCE POS
                         │
          ┌──────────────┴──────────────┐
          │                             │
 TypeScript/Next.js/Node            NilLang/Alap
          │                             │
          └──────────────┬──────────────┘
                         │
                 SAME CONTRACT
                         │
          ┌──────────────┼──────────────┐
          │              │              │
        SAME UI       SAME BEHAVIOR   SAME DATA
          │              │              │
          └──────────────┼──────────────┘
                         │
                    SAME RESULT
```

**এটাই হবে “TypeScript Next POS = NilLang Alap POS”।**

আর একটা জিনিস বিশেষভাবে বলি: তোর বর্তমান repo-তে `pos-app` commit, Entity, ORM/data, Money/Decimal এবং compiler pipeline ইতিমধ্যেই আছে। তাই এগুলো আবার নতুন করে ছড়িয়ে ছিটিয়ে বানানোর দরকার নেই। **এখন কাজ হলো existing pieces-কে এই architecture অনুযায়ী refactor/integrate করা এবং প্রথম vertical slice-টা সত্যিকারের end-to-end করা।**

**পরবর্তী বাস্তব ধাপ:** `nilLang`-এর বর্তমান codebase ধরে আমি এই specification-টাকে এবার **ফাইল-by-file implementation plan**-এ নামিয়ে দিতে পারি—মানে `compiler/...`, `pkg/alap/...`, কোন নতুন `.go` file হবে, কোন existing file modify হবে, এবং **প্রথম Product → Cart → Checkout slice-এর জন্য ঠিক কী code/API/syntax লিখতে হবে**। সেটাই হবে সরাসরি coding শুরু করার blueprint।

# নীলাং UI ও Alap Framework: পূর্ণাঙ্গ নকশা, স্থাপত্য ও বাস্তবায়ন রোডম্যাপ

**পরিধি:** শুধু User Interface (UI), UX-এর সঙ্গে সম্পর্কিত framework behaviour, layout, rendering, component model, accessibility, responsive design, UI tooling এবং Android, iOS ও HarmonyOS-এ UI চালানোর platform adapters। ভাষার সাধারণ compiler, database, networking, package registry, game engine বা সম্পূর্ণ mobile toolchain-এর আলোচনা এখানে কেবল UI-র প্রয়োজন হলে এসেছে।

**লক্ষ্য-পর্যায়:** প্রস্তাবিত দীর্ঘমেয়াদি UI specification; প্রথম দিনের বাস্তবায়ন-নির্দেশ, MVP, production readiness এবং পরবর্তী উন্নতির কাজের তালিকা।

**এই নথি কীভাবে পড়বে:** README-তে বর্তমানে ঘোষিত আচরণকে “বর্তমান নথিভুক্ত ভিত্তি” বলা হয়েছে। যে API, syntax বা architecture এখানে প্রস্তাব করা হয়েছে, সেটিকে “প্রস্তাবিত” বলা হয়েছে; এটি এখনই নীলাংয়ে কাজ করে এমন দাবি নয়। প্রতিটি ধাপে compiler support, runtime support, platform adapter এবং automated test—চারটির প্রমাণ প্রয়োজন।

> **মূল সিদ্ধান্ত:** Alap-কে “JSON দিয়ে কয়েকটি HTML preview বানানোর ব্যবস্থা” হিসেবেই থামিয়ে রাখা যাবে না। একে একটি typed, declarative, testable এবং platform-neutral UI description system বানাতে হবে; তার উপরে Android, iOS, HarmonyOS ও browser preview-এর জন্য adapter থাকবে। একই UI declaration থেকে সম্ভাব্য সর্বাধিক একই আচরণ তৈরি হবে, কিন্তু প্রতিটি platform-এর interaction conventions, system insets, keyboard, accessibility এবং lifecycle-এর বাস্তব পার্থক্যকে অস্বীকার করা হবে না।

---

## সূচিপত্র

1. দৃষ্টিভঙ্গি, লক্ষ্য এবং সীমানা
2. বর্তমান UI ভিত্তি: কী আছে, কী এখনো প্রমাণিত নয়
3. “Cross-platform UI” কথাটির বাস্তব অর্থ
4. UI backend কৌশল: native, custom-rendered, hybrid
5. প্রস্তাবিত Alap UI স্থাপত্য
6. UI tree, typed node এবং intermediate representation
7. Nilang syntax ও UI declaration-এর সম্পর্ক
8. Component model ও component contract
9. State, binding, derived state ও event semantics
10. Layout engine: constraints, measurement, placement
11. Responsive, adaptive ও orientation behaviour
12. Design tokens, theme, typography ও visual consistency
13. Text layout ও বাংলা ভাষার বিশেষ প্রয়োজন
14. Buttons, forms এবং keyboard handling
15. Lists, grids, scrolling এবং virtualization
16. Navigation, routes, sheets, dialogs ও overlays
17. Gesture, touch, pointer, keyboard ও focus
18. Accessibility ও assistive technologies
19. Animation, transition ও motion preferences
20. Images, icons, fonts এবং resource pipeline
21. System bars, safe areas, keyboard insets ও device features
22. Platform adapters: Android, iOS, HarmonyOS
23. Custom drawing, canvas এবং GPU boundary
24. Errors, loading, empty states ও lifecycle correctness
25. UI performance ও profiling
26. UI test architecture ও conformance suite
27. Browser preview ও hot reload-এর সীমা
28. UI developer tools, formatter, language server ও inspector
29. Design system, component catalogue ও documentation
30. UI API design, versioning ও backward compatibility
31. Security, privacy ও untrusted content-এ UI boundary
32. Motion, contrast, colour এবং inclusive design policies
33. Data-heavy app patterns ও বাস্তব উদাহরণ
34. Example app: counter থেকে production-quality app
35. Repository layout ও module boundaries
36. Implementation phases ও dependency order
37. Acceptance criteria, test matrix ও release gate
38. Bug triage, performance budget ও engineering rituals
39. ঝুঁকি, যে কাজ এখনই করা উচিত নয়
40. দীর্ঘমেয়াদি product direction
41. Component-by-component implementation specification
42. UI component test matrix
43. প্রস্তাবিত syntax cookbook
44. প্রথম ৩০/৬০/৯০ দিনের কাজ
45. Release checklist
46. Official references ও পরিভাষা

---

## 1. দৃষ্টিভঙ্গি, লক্ষ্য এবং সীমানা

নীলাংয়ের UI লক্ষ্য হওয়া উচিত: ডেভেলপার একবার একটি screen/component tree ও তার interaction logic লিখবেন; Alap সেই ঘোষণাকে যাচাই করবে, state changes সমন্বয় করবে, layout-এর অর্থ নির্ধারণ করবে এবং নির্দিষ্ট platform adapter সেটিকে platform-উপযুক্ত UI-তে পরিণত করবে। এর অর্থ এই নয় যে একটি byte-for-byte একই rendering সব ফোনে হবে। Android-এর touch feedback, iOS-এর navigation and focus conventions এবং HarmonyOS-এর component behaviour আলাদা হতে পারে। একই source-এর প্রতিশ্রুতি মূলত structure, semantics, data binding এবং business interaction-এর ক্ষেত্রে হওয়া উচিত।

UI framework-এর প্রথম deliverable হওয়া উচিত একটি ছোট কিন্তু নির্ভরযোগ্য subset: `Page`, `Column`, `Row`, `Text`, `Button`, `TextField`, `ScrollView`, `List`, `Image`, `Spacer`, `Divider`, `Card` এবং navigation-এ যাওয়ার জন্য একটি stable foundation। প্রতিটি component-কে একাধিক adapter-এ বাস্তবে দেখানো না পর্যন্ত শুধু catalogue-এ নাম যোগ করা যাবে না। কম component কিন্তু পরিষ্কার semantics—অসংখ্য অসম্পূর্ণ component-এর চেয়ে বেশি মূল্যবান।

নির্দেশক নীতি:

- **Declarative:** UI হলো state-এর বর্ণনা; event এলে state বদলায়, তারপর framework প্রয়োজনীয় অংশ পুনরায় render করে।
- **Typed:** ভুল property, ভুল event payload, ভুল child type এবং অসমর্থিত platform capability যতটা সম্ভব compile time-এ ধরা হবে।
- **Deterministic:** একই input state, environment এবং theme দিলে একই semantic tree পাওয়া যাবে। সময়, random value বা network request render function-এর মধ্যে লুকিয়ে রাখা যাবে না।
- **Accessible by construction:** button-কে শুধু রঙিন rectangle করা যাবে না; তার accessible role, name, action এবং focus behavior থাকতে হবে।
- **Adaptive:** ছোট ফোন, বড় ফোন, foldable, tablet, landscape, split-window, font scaling ও keyboard appearance-এ layout ভেঙে পড়বে না।
- **Testable:** component logic, layout rules, platform adapter ও real-device behaviour আলাদা স্তরে পরীক্ষা করা যাবে।
- **Honest capability model:** যে API কোনো platform-এ নেই বা adapter-এ implement হয়নি, সেটি silent no-op হবে না; compile-time diagnostic, unsupported capability message অথবা documented fallback দিতে হবে।
- **Incremental:** আগে ordinary app UI, পরে custom painting ও advanced transition; শুরুতেই browser engine, custom compositor এবং full design editor বানানো নয়।

সীমানা: এই নথি সম্পূর্ণ নতুন operating system UI, launcher, desktop window manager, game engine অথবা generic graphics engine বানানোর নির্দেশিকা নয়। UI framework-এর জন্য GPU command submission প্রয়োজন হলে সেটি একটি নির্দিষ্ট backend task হবে; সেটি Alap UI-র সংজ্ঞা নয়।

### 1.1 UI framework-এর সাফল্য কীভাবে মাপবে

সাফল্যকে “README-তে Android/iOS/Harmony লেখা আছে কি না” দিয়ে মাপবে না। পাঁচ ধরনের প্রমাণ দরকার: (১) একটি source example parse ও type-check হয়; (২) একই semantic tree browser preview এবং mobile adapter-এ তৈরি হয়; (৩) screen reader component-গুলিকে সঠিক নাম ও role-এ পায়; (৪) app background/foreground, rotation, keyboard এবং window size বদলালে state ও layout ঠিক থাকে; (৫) CI-তে regression হলে পুরোনো screenshot বা semantic test fail হয়। প্রথম release-এ ১০০টি component দাবি করার চেয়ে ১২টি component-এর সম্পূর্ণ behaviour যাচাই করা ভালো।

### 1.2 “same source” প্রতিশ্রুতির সীমা

Shared source-এর তিনটি স্তর আলাদা করে document করো: `shared logic`, `shared UI semantics`, এবং `platform-specific presentation`. Shared logic-এ validation, calculations, form model ও event semantics থাকবে। Shared UI semantics-এ accessible label, hierarchy, interaction state, scroll meaning, focus order ও layout constraints থাকবে। Platform-specific presentation-এ native fonts, platform text-field implementation, system navigation conventions, haptic policies, permission dialogs এবং platform-specific picker থাকতে পারে। API-তে এই স্তরগুলো স্পষ্ট না করলে ডেভেলপাররা pixel-perfect identity-র ভুল ধারণা পাবে।

---

## 2. বর্তমান UI ভিত্তি: কী আছে, কী এখনো প্রমাণিত নয়

রিপোজিটরির README-তে Alap-কে `component`, `state`, `render`, `on`, `build`, `emit`-সহ declarative UI ও component model হিসেবে বর্ণনা করা হয়েছে। README-তে ANSI/HTML page rendering-এর কথাও আছে। `examples/ui-counter/src/main.nil`-এ `Counter` component-এর state হিসেবে `count: i32 = 0` আছে; `render` থেকে `Page`-সদৃশ map, একটি `Card`, এবং `increment` event-এর `Button` ফেরত আসে। `on increment(payload)` handler-এ `count` আপডেট হয় এবং `emit("changed", count)` করা হয়। README-তে browser preview output `build/preview.html` এবং `nil dev`-এর server-dispatched event/rerender flow-ও নথিভুক্ত।

README নিজেই বলে incremental DOM patching এখনো বাস্তবায়িত নয়। এই তথ্য গুরুত্বপূর্ণ: HTML preview-তে render হওয়া মানেই mobile native UI backend প্রস্তুত নয়; `component` keyword থাকা মানেই full component lifecycle, keyed reconciliation বা concurrent state scheduling প্রমাণিত নয়। একইভাবে `pkg/gpu`-তে backend abstraction থাকা মানেই mobile UI-র production compositor তৈরি হয়েছে, এমন দাবি করা যাবে না। UI roadmap এই বিদ্যমান ভিত্তিকে বাতিল না করে সুস্পষ্ট semantics ও conformance tests দিয়ে স্থিতিশীল করবে।

বর্তমান নথিভুক্ত ভিত্তি থেকে নিম্নলিখিত সিদ্ধান্ত নেওয়া যায়:

1. Nilang source-এ component declaration এবং state declaration-এর ধারণা আছে।
2. UI output-এর একটি map/list-ভিত্তিক description path আছে।
3. Event handler payload গ্রহণ করতে পারে এবং state পরিবর্তন ঘটাতে পারে।
4. Event emission-এর primitive রয়েছে।
5. Browser preview-এ একটি rendered representation তৈরি করা যায়।
6. Browser preview interaction path server-side state update ব্যবহার করে—অন্তত README-র বর্ণনায় তা বলা আছে।

কিন্তু এই নথি থেকে প্রমাণিত নয়: native Android accessibility tree-এর correctness, iOS VoiceOver integration, HarmonyOS ArkUI component adapter, list virtualization, layout engine-এর constraint behaviour, focus traversal, keyboard inset handling, native text input composition, dynamic type, renderer diffing, real-device frame pacing, অথবা iOS code signing/deployment-এর UI workflow। এগুলোকে আলাদা issue ও test হিসেবে লিখতে হবে।

### 2.1 প্রথম engineering task: behaviour inventory

প্রথমে `pkg/alap`-এর existing implementation-এর প্রতিটি public type ও command-এর inventory তৈরি করো। প্রতিটি item-এর জন্য লিখবে: exported API, syntax, expected semantics, actual implementation, tests, platform coverage, known limitations, owner ও next step। শুধু symbol count-কে progress metric করবে না। যদি `Button` string `type` দিয়ে তৈরি হয়, সেটি বর্তমানে typed `Button` node নয়, বরং unvalidated node description—এই পার্থক্য লিখবে।

### 2.2 বর্তমান counter example-এর শক্তি ও সীমা

Counter example একটি ভালো functional slice: state → render → user action → state update → emitted event. কিন্তু production UI-র জন্য এটিকে আরও প্রশ্নের উত্তর দিতে হবে: Button-এ accessibility role কী? payload-এর `step` string হলে কী হবে? count overflow হলে কী semantics? component dispose হলে event listener কবে সরবে? state বদলালে শুধু Card update হবে নাকি পুরো Page? button double-tap করলে event ordering কী? screen reader activation-এও একই event চলবে কি? process background-এ গেলে count থাকবে? native back gesture কী করবে? এসবের উত্তর না থাকলে UI demonstration থেকে UI framework-এ যাওয়া হয়নি।

### 2.3 baseline tests

বর্তমান API-র snapshot test, handler unit test, malformed payload test এবং HTML preview smoke test লিখে শুরু করো। নতুন architecture তৈরি করার সময়ও এগুলো চালু রাখবে। Legacy renderer-এর আচরণকে specification মনে করো না; test দিয়ে আগে documented behavior আর accidental implementation detail আলাদা করো।

---

## 3. “Cross-platform UI” কথাটির বাস্তব অর্থ

একই নীলাং declaration তিন প্ল্যাটফর্মে ব্যবহারযোগ্য করতে হলে framework-এর অন্তত পাঁচটি বিষয় একইভাবে সংজ্ঞায়িত হতে হবে: (১) component tree ও child ordering, (২) property-র type এবং default, (৩) state/event lifecycle, (৪) layout constraints ও measurement rules, (৫) accessibility semantics। UI-এর রং, font metrics, system padding বা native picker-এর pixel shape হুবহু এক হবে—এমন প্রতিশ্রুতি দেওয়া বাস্তবসম্মত নয়। Font rasterizer, system font, OS version এবং text shaping-এর কারণে একই nominal font size-এ ছোট পার্থক্য স্বাভাবিক।

Cross-platform correctness-এর তিনটি মানদণ্ড তৈরি করো:

- **Semantic parity:** button একই action করে; disabled button activate হয় না; label screen reader-এ পাওয়া যায়; text field text ও selection যথাযথভাবে সংরক্ষণ করে।
- **Layout parity:** একই constraints-এ hierarchy, alignment, wrapping policy ও scrolling rules সমতুল্য থাকে; platform-specific glyph metrics-এর কারণে কয়েক pixel পার্থক্য অনুমোদিত হতে পারে।
- **Native adaptation:** back navigation, swipe gestures, focus, keyboard, context menu, picker এবং system permission-এর ক্ষেত্রে platform conventions মানা হয়, shared semantics ভাঙা হয় না।

Framework-এর API-তে `appearance: platform` এবং `appearance: shared`-এর মতো নীতির প্রয়োজন হতে পারে, তবে শুরুতে প্রতিটি component-কে দুই রকম style দিয়ে জটিল করো না। প্রথমে default behaviour নির্ধারণ করো: standard control হলে native presentation এবং common custom UI হলে shared tokens; developer স্পষ্টভাবে custom style চাইলে তখন তা প্রয়োগ হবে। কোনো platform-এ native control-এর property সমর্থিত না হলে diagnostic দেখাবে।

### 3.1 Conformance definition

প্রতিটি UI feature-এর জন্য platform conformance record থাকবে:

| Feature | Semantic contract | Android | iOS | HarmonyOS | Browser preview |
|---|---|---|---|---|---|
| Button | role/name/action/disabled semantics | test required | test required | test required | smoke + semantic test |
| Text input | value/selection/composition/focus | test required | test required | test required | approximate |
| Scroll list | ordering/keys/scroll state | test required | test required | test required | virtualization comparison |
| Dialog | modal/focus dismissal | native adapter | native adapter | native adapter | preview approximation |
| Animation | start/end/cancel semantics | frame test | frame test | frame test | functional preview |

“test required” মানে এই নথিতে সেটা সফল বলে দাবি করা হচ্ছে না; implementation-এর আগে evidence জমা করতে হবে।

---

## 4. UI backend কৌশল: native, custom-rendered, hybrid

UI framework সাধারণত তিনটি পথ বেছে নিতে পারে। Native widget backend প্রতিটি UI component-কে platform-এর native widget-এ map করে। এতে accessibility, text input, focus এবং expected platform behaviour পাওয়ার সুযোগ বেশি, কিন্তু visual details platformভেদে বদলায় এবং সব platform-এর component capabilities এক হয় না। Custom rendering backend নিজস্ব canvas/compositor দিয়ে সব controls আঁকে। এতে pixel consistency বেশি এবং animation/custom visuals নিয়ন্ত্রণযোগ্য, কিন্তু text editing, accessibility, selection handles, focus, input methods এবং platform integration বাস্তবায়নের দায়িত্ব framework-এর উপর পড়ে। Hybrid backend সাধারণ control-কে native widget-এ map করে, আর custom visual বা high-performance scene প্রয়োজন হলে custom rendering ব্যবহার করে।

**প্রস্তাবিত default: hybrid, native-first for semantics.** প্রথম release-এ `Text`, `Button`, `TextField`, `Switch`, `Checkbox`, `Slider`, `DatePicker` ও standard navigation-এর মতো control platform adapter-এর native primitive ব্যবহার করবে, যেখানে target platform API উপযুক্ত। `Card`, `Divider`, spacing, decorative shape, custom chart এবং custom transitions shared rendering বা platform composition দিয়ে করা যেতে পারে। এই পথ সম্পূর্ণ pixel-identical নয়, কিন্তু একজন ছোট maintainer-এর পক্ষে accessibility ও system integration-এর ঝুঁকি কমায়।

### 4.1 Rendering contract

Alap UI core নিজে Android View, SwiftUI View বা ArkUI struct তৈরি করবে না। Core একটি typed semantic tree এবং layout intent তৈরি করবে। Backend সেটি platform-native node-এ map করবে, প্রয়োজনীয় native state রাখবে, event listener যুক্ত করবে, layout measurements ফেরত দেবে এবং platform-specific differences report করবে। UI tree-কে backend-এর concrete object-এর সঙ্গে অকারণে মিশিয়ে দিলে পরের platform যোগ করা কঠিন হবে।

### 4.2 কখন custom renderer দরকার

Custom renderer দরকার হতে পারে: graph/chart, diagram, canvas, game HUD, unusual text decoration, custom clipping বা অনেক moving item-এ। সাধারণ form field বা navigation bar custom আঁকা উচিত নয় যতক্ষণ পর্যন্ত এটি সত্যিই প্রয়োজন প্রমাণিত না হয়। Shared renderer তৈরি করলে text shaping, bidi, high-DPI scaling, GPU context loss, touch hit-testing, accessibility virtual nodes, focus, selection এবং software fallback—সবকিছুর দায়িত্ব নিতে হবে। প্রতিটি কাজ আলাদা project issue হবে।

### 4.3 Backend abstraction-এর সীমা

একটি `Renderer` interface-এ কেবল `draw(node)` রাখবে না। বাস্তব interface-এ build/update/dispose, measuring, event binding, host lifecycle, environment update, accessibility export, resource lookup এবং error reporting-এর boundary থাকতে হবে। তবে প্রথম implementation-এ interfaces অতিরিক্ত generic করে ৫০টি abstraction বানিও না; যতটুকু দ্বিতীয় backend তৈরির জন্য সত্যিই প্রয়োজন, ততটুকুই extract করবে।

---

## 5. প্রস্তাবিত Alap UI স্থাপত্য

প্রস্তাবিত architecture-এ সাতটি স্তর থাকবে:

1. **Nilang UI API:** developer-facing `Page`, `Column`, `Text`, `Button`, state declaration ও event handler.
2. **Type checker / UI schema checker:** component-এর নাম, property, child types, event payload, accessibility requirements ও feature availability validate করে।
3. **UI IR:** typed tree, stable node identity, key, style tokens, semantics, layout intent এবং source location.
4. **Reactive runtime:** state dependency tracking, update scheduling, transaction, event queue, lifecycle এবং cancellation.
5. **Layout engine:** logical units, constraints, intrinsic measurement, flex/grid rules, safe insets ও scroll viewport calculation.
6. **Platform adapter:** native node creation/update/disposal, input handling, accessibility bridge, focus/keyboard integration ও native dialog.
7. **Developer tooling:** preview renderer, inspector, hot reload, snapshot generator, UI tests, logs ও profiling.

এই স্তরগুলো build-time ও runtime-এ কিছুটা একত্র হতে পারে, কিন্তু conceptual responsibilities স্পষ্ট থাকবে। উদাহরণস্বরূপ, একটি `Button`-এর `disabled` property type-checker জানবে; semantic tree-তে disabled state থাকবে; runtime event dispatch blocked করবে; Android adapter native enabled-state-ও set করবে; accessibility tree disabled status প্রকাশ করবে; tests নিশ্চিত করবে screen-reader action-এর মাধ্যমেও disabled button চালু না হয়। একই behaviour-এর একাধিক layer-এ রক্ষাকবচ থাকবে, কারণ native UI-তে কেবল UI-তে grey দেখানো যথেষ্ট নয়।

### 5.1 Ownership rules

- API model-এর ownership UI core-এর; platform-specific code public UI semantics পরিবর্তন করতে পারবে না।
- Adapter native objects-এর ownership নেবে; component dispose হলে native listener/observer সরাতে হবে।
- Runtime state-এর ownership একটি নির্দিষ্ট component instance বা explicit store-এর; global accidental singleton নয়।
- Layout measurements adapter/backend থেকে আসতে পারে, কিন্তু layout intent একই specification থেকে আসবে।
- Resources immutable identifier দিয়ে refer হবে; native asset path source code-এ hardcode করা যাবে না।
- Platform availability configuration build-time-এ যাচাই হবে; runtime-এ unsupported operation silently ignore করা যাবে না।

### 5.2 Data flow

Initial state create → component function evaluate → typed IR তৈরি → schema diagnostics → layout constraints resolve → backend nodes create → mount lifecycle callbacks → events → state transaction → dependent subtree recompute → stable key reconciliation → property diff apply → layout/paint invalidation → frame presented. এই flow-টি একটি architecture document ও tracing ID দিয়ে debug করা যাবে। UI framework-এর ভিতরে event এলে ঠিক কোন state dependency recompute হয়েছে তা বোঝা না গেলে performance bug ডিবাগ করা কঠিন হবে।

### 5.3 Incremental updates

প্রথম সংস্করণে পুরো tree rerender করা সহজ হতে পারে, তবে API-র semantics শুরু থেকেই pure render এবং stable identity-র উপযোগী করে নকশা করো। পরে node diff যোগ করতে schema redesign করতে হবে না। Diff engine কোন property layout invalidation ঘটায়, কোনটি paint-only, কোনটি native property update—তা জানতে পারবে। `Text` বদলালে layout পুনরায় মাপতে হতে পারে; শুধু background color বদলালে layout invalidation সাধারণত লাগবে না। এই distinction পরে performance-এর ভিত্তি হবে।

---

## 6. UI tree, typed node এবং intermediate representation

বর্তমান counter example map/list-ভিত্তিক `render` object ফেরত দেয়। এটি prototype-এর জন্য সহজ, কিন্তু production UI-তে arbitrary string property নামের উপর নির্ভর করা যথেষ্ট নয়।

প্রস্তাবিত UI IR-এ property-র arbitrary map না রেখে typed field থাকবে। `Text`-এর `text`, `style`, `maxLines`, `overflow`; `Button`-এর `label`, `enabled`, `onActivate`; `Column`-এর `children`, `spacing`, `alignment`, `mainAxisSize`; `TextField`-এর `value`, `placeholder`, `inputType`, `enabled`, `error`—প্রতিটি schema-তে নির্দিষ্ট হবে। JSON/Map representation preview renderer-এর internal interchange format হিসেবে রাখা যেতে পারে, কিন্তু public API compile-time schema validation অতিক্রম করে arbitrary string property গ্রহণ করবে না।

প্রস্তাবিত একটি node record-এ থাকবে: `kind`, `key`, `sourceSpan`, `props`, `children`, `semantics`, `layout`, `styleRef`, এবং event declaration। `key` list reconciliation-এর স্থিতিশীল identity; `sourceSpan` error message-এ সঠিক `.nil` line দেখাতে সাহায্য করবে; `semantics` accessibility label/role/action বহন করবে; `layout` dimension ও constraint intent রাখবে। Runtime-only values যেমন native object handle, platform view ID, focus token ও texture handle IR-এ serialise করা হবে না।

### 6.1 Stable key এবং tree identity

List update-এ index-কে key হিসেবে ব্যবহার করা বিপজ্জনক, যখন item insertion/removal/reordering ঘটে। Developer-কে `key: item.id` দিতে শেখাও। Stable key থাকলে একই logical row-এর text field focus, scroll position, animation state এবং expanded/collapsed state অযথা অন্য row-তে চলে যাবে না। Duplicate key development mode-এ error বা warning হবে; production mode-এ deterministic fallback ও diagnostic log থাকবে। Duplicate-key behaviour অনির্ধারিত রাখা যাবে না।

### 6.2 Source location ও diagnostics

UI expression-এর প্রতিটি node-এর source location compiler ধরে রাখবে। `Button`-এর `onActivate` handler ভুল arity-তে লেখা, `Column`-এ unsupported property ব্যবহার, অথবা `TextField`-এ ভুল type দিলে error-এ exact file/line, expected type, received type, platform availability এবং safe fix hint থাকবে। Preview renderer যদি runtime node parsing-এ error পায়, সেটিও component path—যেমন `Page > Column[key=main] > TextField[key=email]`—সহ দেখাবে।

### 6.3 Serialization policy

Development preview-তে UI IR JSON-এ রূপান্তরিত হতে পারে; production mobile app-এ প্রতিটি frame-এ JSON serialize/deserialize করা উচিত নয়। Compile-time schema থেকে typed runtime records বা bytecode constants ব্যবহার করো। Debug mode-এ inspectable JSON dump থাকবে, release mode-এ তা optional হবে। UI node-এ closure/action serialized text হয়ে যাবে—এমন architecture নয়; event handler runtime symbol/closure reference দিয়ে সংযুক্ত হবে।

---

## 7. Nilang syntax ও UI declaration-এর সম্পর্ক

UI syntax ভাষার সাধারণ type system, control flow ও function model-এর উপরে বসবে। UI component declaration-এর জন্য নতুন mini-language বানানোর আগে বর্তমান `component`, `state`, `render`, `on`, `emit` syntax-এর semantics নির্দিষ্ট করো। UI-তে declarative child tree লিখতে গিয়ে language parser এবং runtime-এর আলাদা interpretation যেন না হয়। একটি property যদি compile-time constant চায় তবে compiler তা জানবে; reactive binding অনুমোদিত হলে dependency graph-এ সেটি ধরা পড়বে।

নিচের উদাহরণটি **প্রস্তাবিত design sketch**, বর্তমান নীলাংয়ে চলার দাবি নয়:

```nil
component Counter {
    state count: i32 = 0;

    render {
        Page(title: "Counter") {
            Column(spacing: 16) {
                Text("Count: \\(count)", style: theme.typography.titleLarge)
                Button(
                    label: "Increase",
                    onActivate: fn() { count = count + 1; }
                )
            }
        }
    }
}
```

এই style-টির সুবিধা: nesting স্পষ্ট, property type-check করা যায়, child order source-এ দেখা যায় এবং UI tree inspectable হয়। অসুবিধা: parser grammar এবং closure capture-এর rules আরও জটিল হতে পারে। তাই শুরুতে বর্তমান map-ভিত্তিক schema-কে typed API-তে রূপান্তর করতে হবে, তারপর syntax sugar যুক্ত করা যায়। UI declaration ও general data record-এর মধ্যে পার্থক্য পরিষ্কার করো; সাধারণ map-কে accidental UI node ধরে নেওয়া যাবে না।

### 7.1 API naming

UI names-এ একই convention: `Text`, `Button`, `TextField`, `Column`, `Row`, `Stack`, `ScrollView`, `LazyColumn`, `Image`, `Icon`, `Card`, `Divider`, `Dialog`, `NavigationStack`। Boolean properties-এর নাম positive semantics-এ হবে: `enabled: false`, `visible: true`, `dismissible: false`; ambiguous `noClick`, `hideOn`, `isOff` এড়িয়ে চলবে। Event names consistent হবে: `onActivate`, `onChange`, `onSubmit`, `onDismiss`, `onFocusChange`, `onScroll`. Event handler-এর semantics handler name দেখে অনুমান করতে হয় এমন হবে না; API reference-এ action payload ও call timing থাকবে।

### 7.2 Builder API নাকি declarative syntax

Builder-style API compile-time type safety ও code completion-এ সুবিধা দিতে পারে, কিন্তু deep nesting-এ verbosity বাড়ে। Declarative syntax readable, তবে parser/tooling cost আছে। প্রথম release-এ current syntax-এর উপর typed components তৈরি করো এবং একটি stable schema/API freeze করো। syntax sugar তখনই যোগ করবে, যখন type checking, diagnostics, formatter, syntax highlighting, LSP এবং tests প্রস্তুত। syntax আগে পাল্টে পরে semantics ঠিক করলে example code বারবার ভাঙবে।

### 7.3 Conditional content

UI-তে conditional rendering predictable হতে হবে। `if loggedIn { Dashboard() } else { SignIn() }` হলে state change-এর পর পুরোনো subtree dispose হবে এবং নতুন subtree mount হবে। Dispose হওয়া subtree-র observers ও event handlers সরতে হবে। Conditional branch বদলালে keys যদি একই হয়, তবে identity reuse-র policy documented থাকবে; incompatible node type reuse করে native view-এর state ভুলভাবে ধরে রাখা চলবে না।

---

## 8. Component model ও component contract

প্রতিটি component-এর একটি standard contract document থাকবে: purpose, constructor properties, child type, default sizing, layout participation, state ownership, events, semantics role, keyboard behaviour, platform deviations, lifecycle, errors, sample, unit tests এবং unsupported features। Component নিজে কী করে তা জানা না থাকলে developer বারবার native adapter-এর implementation পড়তে বাধ্য হবে।

Component-এর API surface ছোট রাখো। `Text`-কে image loading, tap gestures, animation controller, network fetch, semantic grouping এবং ৫০টি style modifier-এর এক বিশাল component বানিও না। একাধিক behaviour আলাদা concern হলে `Clickable`, `Tooltip`, `Semantics`, `Animated`, `Padding` অথবা modifier model বিবেচনা করো। তবে modifier chain নতুন করে implement করলে ordering semantics (padding→clip vs clip→padding) স্পষ্ট হতে হবে। প্রথম সংস্করণে explicit wrapper components অনেক সময় বোঝা ও debug করা সহজ।

### 8.1 Parent-child contract

`Column` children-এর ordered sequence গ্রহণ করে; `Text` সাধারণত child নেয় না; `Button`-এ একাধিক arbitrary child লাগলে তার meaning নির্দিষ্ট করতে হবে; `List` keyed items চায়; `Page` একটি body এবং ঐচ্ছিক top/bottom region রাখবে। Invalid child placement compile-time diagnostic দেবে। Generic `children: Any` ব্যবহার করে validation স্থগিত করা সহজ, কিন্তু production framework-এ এটি accidental malformed tree তৈরি করবে।

### 8.2 Stateful এবং stateless component

Stateless component-এর output inputs-এর function; stateful component internal UI state রাখতে পারে, যেমন expanded/collapsed। Business data global singleton-এ রাখা হবে না। State-এর owner কোথায়—parent component, explicit store, route scope নাকি persistent model—এটি API-তে চেনা যায় এমন হতে হবে। Parent rerender হলে stateless child reuse হবে; stateful child stable identity-সহ থাকলে local state ধরে রাখবে; dispose হলে scoped state ও subscription বন্ধ হবে।

### 8.3 Public API stability

প্রতিটি exported component-এর stability level দাও: `experimental`, `preview`, `stable`, `deprecated`। Experimental component release-to-release বদলাতে পারে, কিন্তু stable API semantic versioning ও migration notes ছাড়া ভাঙা যাবে না। API-র default value বদলানোও breaking change হতে পারে—যেমন `Text`-এর default max lines আগে unlimited পরে ১ করলে app layout ভেঙে যাবে। Property নাম বদলানো, event payload shape বদলানো, accessibility role বদলানো এবং focus order বদলানো সবই API compatibility review-এর আওতায় পড়বে।

---

## 9. State, binding, derived state ও event semantics

UI state-এর জন্য একটিমাত্র নীতি গ্রহণ করো: render function যেন side effect না করে; user/system event state mutation ঘটায়; state transaction শেষ হলে dependents recompute হয়। যদি render-এর মধ্যে database write, network fetch বা timer schedule করা যায়, তাহলে rerender count বাড়লেই operation একাধিকবার চলতে পারে। Render purity compile-time-এ পুরোপুরি প্রমাণ করা না গেলে lint rule ও runtime development warning দাও।

প্রস্তাবিত state categories:

- **Local ephemeral state:** current tab, checkbox, temporary text, expanded row; component lifecycle-এর সঙ্গে সম্পর্কিত।
- **Shared UI state:** selected item বা theme, যা explicit store/provider দিয়ে একাধিক screen-এ ব্যবহৃত হয়।
- **Application model state:** logged-in user, business data, form draft; UI component-এর বাইরে রাখা উচিত।
- **Persistent UI state:** user preference বা screen restoration state; storage adapter মাধ্যমে save/restore হবে।
- **Derived state:** count, validation message, filtered list; canonical data থেকে গণনা হবে, অপ্রয়োজনে duplicate mutable state নয়।

### 9.1 Event ordering

একই frame-এ দ্রুত একাধিক action এলে event order নির্দিষ্ট থাকবে। Button tap event-এর পর state transaction commit হবে; `emit` কী সময়ে ঘটবে—update-এর আগে, পরে, না transaction complete হলে—তা specification-এ লেখা থাকবে। প্রস্তাব: handler-এর মধ্যে state update transaction জমা হবে; transaction validate হলে commit হবে; এরপর change notifications emit হবে; UI rerender schedule হবে। Handler error হলে partial state mutation থাকবে কি না নির্দিষ্ট করো। Atomic transaction নিরাপদ, তবে প্রথম বাস্তবায়নে complexity কমাতে synchronous event loop ব্যবহার করা যায়।

### 9.2 Binding ও two-way input

`TextField`-এর text binding native IME composition ও selection behaviour ভাঙবে না। প্রতিটি keystroke-এর সময় framework পুরো field recreate করলে cursor position jump করবে; তাই native text field-এর editing state ও committed model state-এর synchronization rule দরকার। Binding-কে magic two-way mutation হিসেবে না দেখিয়ে `value` plus `onChange(newValue)` দিয়ে শুরু করা যায়। Debounce, validation এবং submit আলাদা handler হবে।

### 9.3 Derived state invalidation

`count` পরিবর্তন হলে যেসব subtree `count` read করেছে, কেবল সেগুলিই dependency graph অনুযায়ী invalidated হওয়া উচিত। প্রথমে dependency tracking ব্যয়বহুল হলে component-level rebuild দিয়ে শুরু করো, কিন্তু render purity ও stable key রাখো। পরে fine-grained reactivity-এর benchmark দ্বারা উন্নতি justify করো। অতিরিক্ত সূক্ষ্ম subscriptions lifecycle memory leak আনতে পারে; dispose-time cleanup বাধ্যতামূলক।

### 9.4 Async state

Async operation-এ তিনটি state স্পষ্ট থাকবে: loading, success, error; প্রয়োজন হলে stale-while-revalidate। Component dispose হলে request cancellation token বাতিল হবে; cancellation supported না হলেও result ignore করার guard থাকবে। একাধিক search query দ্রুত এলে পুরোনো response নতুন results overwrite করতে পারবে না—request generation ID বা cancellation ব্যবহার করো। Async callback থেকে disposed state mutate করার চেষ্টা development mode-এ diagnostic হবে।

### 9.5 Event payload typing

বর্তমান counter example-এ map payload থেকে `step` নেওয়া হয়। Production API-তে payload shape compile-time typed হওয়া ভালো। `onActivate: fn()`, `onChange: fn(value: String)` এবং `onScroll: fn(info: ScrollInfo)`-এর contract স্পষ্ট হবে। JSON/string event নাম ব্যবহার internal browser bridge-এর জন্য ঠিক, কিন্তু typed API-তে invalid event নাম ও field typo compile-time error হওয়া উচিত।

---

## 10. Layout engine: constraints, measurement, placement

একটি নির্ভরযোগ্য UI layout engine-এর কাজ হলো parent-এর available space, child-এর intrinsic size, min/max constraints, alignment এবং overflow policy থেকে স্থিতিশীল size ও position নির্ধারণ করা। Width/height-এ pixel hardcode করলেই responsive UI হয় না। Text wrapping, font scaling, system bars, keyboard, display cutout এবং multi-window size change-ও layout input-এর অংশ।

### 10.1 Units

Public API-তে logical unit ব্যবহার করো, physical pixel নয়। প্রস্তাবিত types: `Dp` বা `Vp` layout unit, `Sp` text scaling-aware unit, `Percent`, `Fill`, `Wrap`, `MinMax`, `Fraction`। Platform adapter logical unit-কে native density units-এ map করবে। CSS `px`-এর নামকে mobile density-independent unit-এর সঙ্গে এক করে ফেলো না। Dimension typed হলে `12` আর `12.dp`-এর মতো ambiguity এড়ানো যায়। API-র একটিমাত্র স্পষ্ট dimension system বেছে নিয়ে সব component-এ ব্যবহার করো।

### 10.2 Constraint model

প্রতিটি child layout pass-এ `minWidth`, `maxWidth`, `minHeight`, `maxHeight` বা equivalent constraints পাবে; child desired size ফেরত দেবে; parent size/position নির্ধারণ করবে। `Wrap` intrinsic content-এর মাপ নেবে; `Fill` available space নেবে; `MinMax` bounds প্রয়োগ করবে। Infinite height constraints-এর মধ্যে `Fill` কী করবে তা স্পষ্ট করা জরুরি—যেমন unbounded scroll content-এ `Fill` অবৈধ বা wrap fallback হতে পারে। Ambiguous combination warning দেবে।

### 10.3 Column ও Row

`Column` vertical flow, `Row` horizontal flow। `spacing`, main-axis alignment, cross-axis alignment, `mainAxisSize`, wrapping এবং child flex ratio-র semantics নির্দিষ্ট করো। `Row`-তে text field overflow হলে shrink, wrap না horizontal scroll—default policy documented হবে। Weighted child-এর size গণনায় first pass এবং second pass লাগতে পারে। শিশুদের measure order এবং spacing distribution নিয়ে backend parity tests থাকবে।

### 10.4 Stack ও overlay

`Stack` overlapping children layout করে; z-order tree order নাকি explicit `zIndex` নির্ধারণ করবে তা স্পষ্ট করো। Overlay-এর hit-test ও accessibility order visual order-এর সঙ্গে সামঞ্জস্যপূর্ণ হবে। Invisible overlay hit-test block করে থাকতে পারবে না, যদি না সেটি modal barrier হিসেবে কাজ করে। `position: absolute`-এর মতো ব্যাপক স্বাধীনতা শুরুতে না দিয়ে `Alignment`, `Inset`, `Overlay` primitives দিয়ে common use cases সমর্থন করো।

### 10.5 Grid

Grid শুরুতেই অত্যন্ত জটিল CSS Grid clone হবে না। MVP grid-এ fixed/`Fraction` columns, row gap, column gap, item key এবং adaptive column count থাকুক। Desktop/tablet breakpoint-এ column count বদলানো যাবে। Masonry grid, subgrid, arbitrary named area ও intrinsic track sizing পরে। Grid virtualized হলে visible row range গণনা এবং scroll restoration আলাদা test হবে।

### 10.6 Overflow ও clipping

প্রতিটি component overflow policy define করবে: visible, clip, scroll অথবা ellipsis। Default overflow behaviour backend-নির্দিষ্ট হতে পারবে না। `Text`-এ `maxLines` ও `overflow` একসঙ্গে কী করবে নির্দিষ্ট করো। Clipping accessibility label থেকে content মুছে ফেলবে না। Clip path expensive হলে performance counter থাকবে। Parent layout এবং child visual overflow আলাদা concepts হিসেবে document করো।

### 10.7 Intrinsic measurement

Text, image এবং wrapping controls intrinsic measurement চায়। Native adapter measured size ফেরত দিলে platform-specific font metrics-এর পার্থক্য আসবে; framework-কে consistent constraint rules বজায় রাখতে হবে। Layout pass-এ একই child-কে অনিয়ন্ত্রিতভাবে বারবার measure না করা, measurement cache invalidation ঠিক রাখা এবং environment changes-এ cache invalidate করা জরুরি। Cache key-তে width constraint, font scale, locale, theme typography এবং content hash থাকতে পারে।

### 10.8 Layout debugging

Developer mode-এ overlay দিয়ে bounding boxes, padding, baseline, flex factors, safe insets, overflow, row key এবং scroll viewport দেখাবে। `nil ui inspect` বা inspector UI panel-এ selected node-এর measured/desired size, source location ও adapter native class দেখা যাবে। Layout warning-এ “TextField width=Fill inside unbounded Row” এর মতো concrete context থাকবে, “layout failed” নয়।

---

## 11. Responsive, adaptive ও orientation behaviour

Responsive মানে screen width বদলালে শুধু font ছোট করা নয়। Layout decisions container size, readable content width, input mode, platform idioms এবং available panes থেকে নেওয়া উচিত। Phone-এ একটি column; wider tablet-এ master-detail; landscape-এ bottom controls reposition; keyboard এলে form scroll; foldable-এ hinge region এড়িয়ে layout—এসব আলাদা পরিস্থিতি।

### 11.1 Breakpoints

ব্রেকপয়েন্ট hardcode করে শুধু device model-এর নামের উপর নির্ভর করবে না। `compact`, `medium`, `expanded`-এর মতো semantic classes ব্যবহার করা যায়, কিন্তু threshold-এর বাস্তব মান documented ও সহজে configurable হতে হবে। Layout window size অনুযায়ী adapt করবে, device brand অনুযায়ী নয়। উদাহরণ: wide window-এ sidebar + content; compact window-এ bottom navigation অথবা navigation stack. UI developer নিজে `if width > 600` লিখে শুধু screen-এ নয়, component library level-এ reusable adaptive pattern পাবে।

### 11.2 Window size changes

Orientation change বা split-screen-এ framework নতুন constraints পাবে। State recreation হলে scroll position, text input selection, current route এবং transient form content হারানো চলবে না। Backend window-size callback দেবে; layout engine reflow করবে; app business data আবার network থেকে অকারণে fetch করবে না। Rotation-এর সময় animation চললে তা cancel/retarget হবে—stale layout coordinates ব্যবহার নয়।

### 11.3 Foldables ও dual-pane

Framework-এর প্রথম release-এ hinge-aware advanced foldable system বাধ্যতামূলক নয়, কিন্তু layout model এমন হবে যেন ভবিষ্যতে `DisplayFeature` বা window segment তথ্য দেওয়া যায়। Hinge region-এর উপর interactive button পড়লে hit testing খারাপ হতে পারে। Dual-pane apps-এর navigation model আলাদা করে দেখাও। API-তে physical model name-এর শর্ত না দিয়ে available safe rectangles এবং window class ব্যবহার করো।

### 11.4 Responsive testing

একই component test হবে compact portrait, compact landscape, expanded/tablet width, large font scale এবং keyboard-visible অবস্থায়। Screenshot test-এ প্রতি pixel-এর exact match প্রয়োজন নেই; semantic layout assertions—button visible, label clipped নয়, content scrollable—থাকবে। Visual snapshots platform/versionভিত্তিক baseline রাখবে; screenshot mismatch-এর জন্য flaky thresholds ও ignored regions কম ব্যবহার করবে।

---

## 12. Design tokens, theme, typography ও visual consistency

Alap UI-তে theme শুধু background ও text color-এর map হবে না। Theme token-এর মধ্যে colour roles, typography scale, spacing, corner radius, elevation, border, motion duration, focus ring, disabled opacity, density mode এবং component defaults থাকবে। Developer tokens ব্যবহার করলে dark theme, high contrast, future platform redesign এবং brand variation এক জায়গা থেকে নিয়ন্ত্রণ করা যাবে। Raw hex values component code-এ ছড়িয়ে থাকলে theme বদলানোর সময় একাধিক platform-এর UI ভেঙে যাবে।

### 12.1 Colour roles

`primary`, `onPrimary`, `surface`, `onSurface`, `surfaceVariant`, `outline`, `error`, `onError`, `success`-এর মতো semantic roles ব্যবহার করা যায়। `red500`-কে error colour হিসেবে ধরে নিও না; theme role meaning বহন করবে। Disabled state-এর contrast যথেষ্ট থাকবে, overlay-র মাধ্যমে text unreadable করা চলবে না। Colour state-এর একমাত্র carrier হতে পারবে না: error icon, text বা accessible description-ও থাকবে।

### 12.2 Typography scale

`display`, `headline`, `title`, `body`, `label`, `caption`-এর নির্দিষ্ট scale থাকবে। Developer নির্দিষ্ট font family override করতে পারবেন, কিন্তু user dynamic type/font-scale preference-কে অন্ধভাবে অস্বীকার করবে না। Font tokens-এর মধ্যে weight, line height, letter spacing, baseline policy ও fallback list থাকবে। Native backend নিজের system font ব্যবহার করতে পারে; cross-platform screenshot-এ font metrics-এর অনুমোদিত variance document করবে।

### 12.3 Spacing scale

প্রস্তাবিত base spacing scale—যেমন 4, 8, 12, 16, 24, 32 logical units—ডিজাইন consistent রাখতে সাহায্য করে। API arbitrary spacing সমর্থন করলেও component library এই scale-এর token প্রস্তাব করবে। `gap`, `padding`, `margin` আলাদা semantics; `padding` component content-এর ভিতরের space, `gap` siblings-এর মধ্যে space। একই spacing দুইবার apply হয়েছে কি না inspector দেখাতে পারবে।

### 12.4 Shape, elevation ও border

Corner radius semantic tokens হিসেবে থাকবে: `small`, `medium`, `large`, `pill`, `none`. Elevation সব platform-এ একই shadow pixels দেবে এমন নয়; semantic elevation layer নির্দিষ্ট করে platform-appropriate shadow/material apply করা যায়। Focus ring, border width ও outline theme-এর অংশ হবে। Dark theme-এ কেবল colour invert করলে elevation hierarchy ঠিক নাও থাকতে পারে; surface role tokens ব্যবহার করো।

### 12.5 Theme resolution order

Theme root → component-specific theme → explicit component property. এই precedence স্থির ও document করা হবে। Explicit override সর্বশেষে, কিন্তু accessibility preference যেমন high contrast বা reduced motion-এর বিরুদ্ধে app style override করা যাবে কি না তা স্পষ্ট policy হবে। Theme change runtime-এ হলে affected nodes update হবে; সব native node অকারণে recreate হবে না। Theme switching-এ text selection, scroll position ও navigation state অপরিবর্তিত থাকা উচিত।

---

## 13. Text layout ও বাংলা ভাষার বিশেষ প্রয়োজন

বাংলা UI-কে শুধু UTF-8 string support দিয়ে সম্পূর্ণ বলা যায় না। যুক্তাক্ষর, কার-ফলা, combining marks, Unicode normalization, line breaking, mixed Bengali/Latin text, Bengali numeral preferences, bidi text, text selection, IME composition এবং fallback fonts সব পরীক্ষা করতে হবে। নীলাংয়ের নিজস্ব ভাষা ও ecosystem বাংলা-কেন্দ্রিক হতে পারে; সেই সুবিধাকে UI-তে বাস্তব quality-তে পরিণত করার জন্য Bengali-first test suite আবশ্যক।

### 13.1 Text shaping

Platform-native text control ব্যবহার করলে platform shaping engine জটিল glyph shaping সামলানোর সুযোগ দেয়। Custom renderer নিজে text আঁকলে font shaping engine-এর integration, cluster-to-character mapping এবং accessibility offsets-এর পূর্ণ দায়িত্ব নিতে হবে। একটি grapheme cluster মাঝখানে cursor বা selection boundary কেটে দিলে বাংলা যুক্তাক্ষরে visual corruption হতে পারে। তাই character index, UTF-16 index, Unicode scalar index এবং grapheme boundary—এগুলো API boundary-তে মিশিয়ে ফেলবে না।

### 13.2 Normalization ও input

বাংলা লেখা input field-এ NFC/NFD-এর representation difference, zero-width joiner/non-joiner এবং external keyboard composition নিয়ে tests থাকবে। User-এর typed text-কে অকারণে normalize করে selection range নষ্ট করা চলবে না। Search matching-এর normalization policy data layer-এ হতে পারে, কিন্তু UI displayed text নীরবে পরিবর্তন করবে না। Paste, long-press selection, cut/copy এবং IME suggestions native adapter-এ পরীক্ষিত হবে।

### 13.3 Line breaking ও truncation

Text wrapping ভাষা-উপযোগী shaping rules মেনে হবে। `maxLines` + ellipsis হলে truncated visible text ছাড়াও semantic label-এ full text থাকবে—যদি privacy/security policy তাতে বাধা না দেয়। Ellipsis এমন জায়গায় বসবে না যা conjunct ভেঙে দেয়। Mixed Bengali and Latin/number strings, phone/email values, currency formatting এবং code snippets আলাদা mode-এ পরীক্ষা করবে।

### 13.4 Font fallback

Bengali glyph অনুপস্থিত হলে box/tofu glyph দেখা যাবে না। Font fallback order document করো; bundled font থাকলে license ও bundle size যাচাই করো। System font-এর fallback chain OS অনুযায়ী বদলাতে পারে; একই visual consistency দরকার হলে licensed bundled fonts optional resource হিসেবে দাও। তবে text scaling, font weight, language-specific line height এবং app size-এর trade-off পরিমাপ করতে হবে।

### 13.5 Numerals, date ও locale

Bengali digits (`০১২৩৪৫৬৭৮৯`) দেখানো যাবে কি না user/system locale এবং data-formatting policy অনুযায়ী নির্ধারিত হবে। Number value internal representation-এ number হিসেবেই থাকবে; string-এর মধ্যে localized digits hardcode করে arithmetic করা যাবে না। Date/time/currency formatting `Locale` service ব্যবহার করবে। Bengali, English এবং mixed UI-র golden test set রাখো।

### 13.6 Text selection ও accessibility

VoiceOver, TalkBack এবং HarmonyOS accessibility service-এ Bengali text সঠিকভাবে পড়া হচ্ছে কি না বাস্তব device-এ শুনে পরীক্ষা করো। punctuation, numerals, abbreviations ও code label-এর pronunciation system-ভেদে আলাদা হতে পারে। Framework accessibility label override দিতে পারে; developer-কে visual text থেকে আলাদা label দরকার হলে তা স্পষ্টভাবে লিখতে হবে। TextField-এ selection handles, cursor movement এবং keyboard navigation-এর test বাধ্যতামূলক।

---

## 14. Buttons, forms এবং keyboard handling

Mobile app-এ form input সবচেয়ে জটিল UI area-গুলির একটি। একটি TextField কেবল string দেখানো box নয়: focus, selection, composition, autocomplete, input type, keyboard action, validation, disabled state, error message, secure-entry mode, autofill এবং lifecycle আছে। প্রথম release-এ native text input ব্যবহার করার কারণ এখানেই। Custom-painted text field যতক্ষণ না editing/accessibility-এর সমান স্তর প্রমাণিত হয়, production default হবে না।

### 14.1 TextField contract

`value`, `placeholder`, `enabled`, `readOnly`, `inputType`, `autocorrect`, `autocapitalize`, `maxLength`, `obscureText`, `keyboardAction`, `onChange`, `onSubmit`, `onFocusChange`, `errorText`-এর semantics document করো। সব property সব platform-এ একই feature পাবে এমন নয়; adapter capability table থাকবে। `onChange`-এ প্রতিটি edit operation-এ value আসবে কি না এবং composition text include হবে কি না নির্দিষ্ট করতে হবে।

### 14.2 Validation

Validation তিন স্তরে আলাদা করো: input-format constraint, field validation, form-level validation. Format constraints যেমন numeric keyboard suggestion; validation correctness তার বাইরে। Invalid input হলে শুধু লাল border নয়, readable error text, accessibility announcement এবং submit logic block করতে হবে। Async server validation থাকলে pending state, cancellation ও stale response handling লাগবে। Error message screen reader-এ যুক্ত হবে এবং field-এর accessible description হিসেবে প্রকাশ পাবে।

### 14.3 Keyboard movement

`next`, `done`, `search`, `send`, `go`-এর মতো keyboard action semantic enum হবে। Form-এর next field focus পাবে কি না developer explicit navigation order অথবা platform default হিসেবে ঠিক করতে পারবেন। Keyboard visible হলে bottom action bar obscured হচ্ছে কি না পরীক্ষা করতে হবে। Hardcoded keyboard height ব্যবহার করবে না; system inset/resize information ব্যবহার করবে।

### 14.4 Focus lifecycle

Route change, dialog open, screen rotation, app resume ও text field disposal-এ focus behaviour নির্দিষ্ট করতে হবে। Dialog খুললে focus dialog content-এ যাবে; dismiss করলে triggering element-এ ফিরে আসার চেষ্টা করবে। Field dispose হওয়ার পরে delayed callback focus set করতে পারবে না। Autofocus অপব্যবহার screen reader-কে বিভ্রান্ত করতে পারে; opt-in ও accessible reason documented থাকবে।

### 14.5 Password ও sensitive fields

Password input content logs-এ যাবে না, screenshot diagnostics-এ mask করা হবে, accessibility label যথাযথ থাকবে, clipboard behaviour documented হবে এবং app-switcher preview-তে sensitive value leak হচ্ছে কি না platform policy যাচাই করবে। `obscureText` শুধু glyph mask করে; secure storage-এর বিকল্প নয়। UI framework developer-কে password value persistent UI snapshot-এ স্বয়ংক্রিয়ভাবে save করতে দেবে না।

---

## 15. Lists, grids, scrolling এবং virtualization

`List` component-কে simple array renderer ভাবা যাবে না। বাস্তব list-এ হাজারো item থাকতে পারে; সব child একসঙ্গে native view হিসেবে তৈরি করলে memory ও startup time বাড়ে। Lazy/virtualized list visible viewport এবং nearby prefetch region-এর item materialize করবে; item offscreen হলে view detach হতে পারে কিন্তু data identity ও scroll semantics থাকবে। Virtualization-এর আগে stable keys, scroll controller এবং item reuse contract ঠিক করো।

### 15.1 List API

প্রস্তাবিত API shape: `LazyColumn(items: data, key: fn(item) { item.id }, itemBuilder: fn(item) { ... })`. এটি illustrative; final syntax language grammar ও generics-এর সঙ্গে যাচাই করতে হবে। `key` mandatory হওয়া উচিত যখন list mutates. Static short list-এর জন্য `Column`-এ children রাখা যাবে; data-backed large list `LazyColumn` ব্যবহার করবে। API documentation-এ কখন lazy version প্রয়োজন তার guidance থাকবে।

### 15.2 Scroll state

Scroll offset logical content coordinates-এ রাখতে হবে; orientation/viewport changes-এ restore rule নির্দিষ্ট থাকবে। Data update-এর পর anchor item-এর key ব্যবহার করে position restore করা যায়। List prepend হলে scroll position জোর করে bottom-এ চলে যাওয়া উচিত নয়; chat-style list-এর জন্য separate reverse/anchor semantics দরকার। Scroll controller unmounted list-এ command পেলে safe error বা deferred operation দেবে।

### 15.3 Recycling hazards

একটি reusable native row এক item-এর input state অন্য item-এ leak করতে পারবে না। Rebind-এর সময় stale click listener, animation, focus, async request, image request ও accessibility label reset হবে। Lazy row disposal মানেই domain object dispose নয়। `key` ভুল হলে state mismatch detect করার debug option রাখো।

### 15.4 Pull-to-refresh ও infinite loading

Refresh gesture সব screen-এর default হবে না; explicit `RefreshContainer` বা equivalent component থাকবে। Loading indicator, error/retry row, end-of-list state ও duplicate request prevention-এর semantics থাকবে। Scroll-এর মাঝখানে refresh হলে scroll position policy নির্দিষ্ট করতে হবে। Network pagination-কে UI renderer-এর ভিতরে লুকিয়ে না রেখে application data layer-এ রাখা ভালো; UI শুধু loading/status state render করবে।

### 15.5 Grid layout

Grid item-এর row/column ordering, adaptive span ও key stability নিশ্চিত করো। Accessibility focus order visually expected order-এর সঙ্গে থাকবে। Responsive column count বদলালে item identity এবং scroll anchor অযথা হারাবে না। Large grid-এ virtualization বাধ্যতামূলক হবে।

---

## 16. Navigation, routes, sheets, dialogs ও overlays

Navigation UI framework-এর root architecture-এ থাকবে; প্রতিটি screen নিজে global string route বদলাবে না। `NavigationStack` বা `Navigator`-এর state একটি typed route stack, route arguments, restoration key এবং back policy নির্ধারণ করবে। Android system back, iOS swipe-back এবং HarmonyOS back mechanism UI navigation model-এর সঙ্গে যুক্ত হবে। Platform-এর native back gesture চললেও app-এর unsaved changes বা nested navigator-এর policy bypass হবে না।

### 16.1 Route model

প্রতিটি route-এর identifier এবং typed argument থাকবে। Raw string-only routes typo-প্রবণ; typed route declarations বা compile-time route registry ভালো। Route change হলে previous screen retain, suspend বা dispose হবে কি না policy থাকবে। Deep link route resolution, unknown route fallback এবং parameter validation আলাদা concern। Sensitive data route string-এ rawভাবে রাখবে না।

### 16.2 Back behaviour

Back press order: active modal/overlay dismissযোগ্য কি না → nested navigator consume করতে পারে কি না → current route pop করা যায় কি না → root-level behaviour. এই precedence স্পষ্টভাবে test করা হবে। Dialog খোলা অবস্থায় Android back dialog dismiss করতে পারে, কিন্তু unsaved form navigation pop করতে গিয়ে user-এর লেখা হারানো যাবে না। `onBack` interception সীমিত ও documented হবে; app ইচ্ছেমতো system back আটকাতে পারবে না।

### 16.3 Dialog ও modal sheet

Dialog-এ focus containment, initial focus, escape/back dismissal, outside tap policy, accessibility modal semantics এবং dismiss-এর পরে focus restore থাকতে হবে। Modal sheet keyboard, bottom insets এবং drag-to-dismiss threshold নিয়ে platform adapter ব্যবহার করবে। `dismissible: false` হলে UI visible affordance দিয়ে dismissal disable করতে হবে এবং system back policy explicit থাকবে। Dialog content বড় হলে scrollable; ছোট screen-এ screen overflow নয়।

### 16.4 Tabs ও bottom navigation

Bottom navigation top-level destinations-এর জন্য; সামান্য actions-এ ব্যবহার নয়। Tab বদলালে tab-local navigation stack এবং scroll state preserve হবে কি না স্পষ্ট rule থাকবে। Accessible role tab/tablist অথবা platform equivalent হবে। Selected tab শুধু colour দিয়ে বোঝানো যাবে না। ছোট screen-এ labels, large text এবং safe bottom inset-এর জন্য adaptive layout থাকবে।

### 16.5 Overlays, toast ও snackbar

Toast/snackbar-এর duration, action, interruption policy এবং accessibility announcement documented থাকবে। Critical error ephemeral toast-এ লুকানো যাবে না; user message পুনরায় পড়তে না পারলে তথ্য হারায়। Snackbar action-এ focus ও state consistency পরীক্ষা করবে। Multiple notifications stack করলে newest/oldest policy থাকবে।

---

## 17. Gesture, touch, pointer, keyboard ও focus

Interactive UI-তে touch target-এর আকার, gesture conflict, focus feedback এবং keyboard navigation গুরুত্বপূর্ণ। `onTap`-কে raw pointer sequence-এর সঙ্গে মিশিয়ে ফেলবে না। Tap, long press, drag, swipe এবং pan recognizer-এ gesture arena বা priority rule দরকার, যাতে nested ScrollView-এর ভিতরের Button tap করা যায় এবং vertical scroll ভুল করে long press action চালু না করে।

### 17.1 Hit testing

Visible, enabled এবং hit-testable nodes-এর ordering tree/z-order contract থেকে আসবে। Decorative icon interactive Button-এর child হলে আলাদা duplicate click event তৈরি করা যাবে না। Transparently painted overlay touch consume করবে কি না `pointerEvents`-এর মতো explicit policy থাকবে। Disabled control hit-testing ও accessibility action—দুটোতেই disabled থাকবে।

### 17.2 Touch target ও feedback

Visual button ছোট হতে পারে, কিন্তু interactive hit region যথেষ্ট বড় হওয়া দরকার। Target-size policy theme/design system-এ রাখো; platform-specific human-interface guidance অনুসারে minimum size validate করো। Pressed feedback, hover/focus state ও disabled appearance distinct হবে। Haptic feedback optional API এবং user/platform preferences মানবে। Accessibility switch-control navigation যেন target-গুলিকে reachable করতে পারে।

### 17.3 Gesture conflicts

Scrollable list-এ swipe-to-delete gesture vertical scrolling-এর সঙ্গে সংঘর্ষ করতে পারে। Recognizer-কে direction threshold, cancellation এবং child/parent precedence নির্ধারণ করতে হবে। Start করা gesture route বদলে গেলেও stale handler call করবে না। Gesture cancellation normal path, error নয়; animations এবং pressed state cancel হবে। Real device test ছাড়া emulator-only gesture implementation production-ready নয়।

### 17.4 Keyboard ও pointer

Tablet/desktop mode-এ keyboard focus, Tab/Shift+Tab, Enter/Space activation, Escape dismissal এবং arrow navigation প্রযোজ্য। Hover থাকলেও hover-কে একমাত্র affordance বানানো যাবে না। Native focus ring এবং spatial navigation support platform অনুযায়ী মানাতে হবে। `TextField` keyboard navigation ও `Button` activation একই semantic action dispatch করবে, যাতে keyboard এবং touch আলাদা business behaviour না চালায়।

---

## 18. Accessibility ও assistive technologies

Accessibility optional addon নয়; UI node-এর contract-এর অংশ। প্রতিটি interactive component-এর role, accessible name, value/state, action এবং focusability নির্ধারণ করা হবে। `Text` সাধারণত read-only text semantics দেয়; decorative icon accessibility tree থেকে বাদ যেতে পারে; অর্থবহ icon-এর label থাকবে; icon-button-এর accessible name বাধ্যতামূলক হবে। Native controls ব্যবহার করলে system accessibility semantics পাওয়ার সম্ভাবনা বেশি, কিন্তু custom labels, grouping এবং announcement পরীক্ষা করতে হবে।

Apple-এর SwiftUI documentation-এ common elements-এর জন্য কিছু accessibility semantics স্বয়ংক্রিয়ভাবে পাওয়ার কথা বলা আছে, এবং TalkBack/VoiceOver/manual testing ও automated test APIs-এর গুরুত্ব Android documentation-এও বলা হয়েছে। HarmonyOS-এর Accessibility Kit এবং ArkUI accessibility attributes-ও এই একই কারণে আলাদা integration surface। (Official references: Apple Accessibility Fundamentals, Android Compose Accessibility Testing, Huawei Accessibility Kit; শেষের References section-এ links আছে.)

### 18.1 Semantic tree

UI tree এবং accessibility tree এক-একটি হতে বাধ্য নয়। একটি visual row-এর মধ্যে label + value + edit button থাকতে পারে; semantics-এ আলাদা focus stops অথবা grouped announcement দরকার হতে পারে। Alap-এ `Semantics`/`Accessibility` modifier দিয়ে label, hint, value, role, disabled, selected, expanded/collapsed, live-region এবং custom actions নির্দিষ্ট করা যাবে। Platform adapter semantics-কে native accessibility node-এ map করবে।

### 18.2 Screen reader order

Tree order default focus/read order হতে পারে, কিন্তু visual order-এর সঙ্গে অসামঞ্জস্য হলে explicit ordering প্রয়োজন। Absolute positioning, overlay, collection rows এবং reordering-এর পরে focus order পরীক্ষা করবে। Dialog খোলার পর background content screen reader focusable থাকছে কি না নিশ্চিত করো। Hidden/offscreen content accessibility tree-এ অপ্রয়োজনে থাকলে navigation বিভ্রান্তিকর হয়।

### 18.3 Labels, hint ও value

Button-এর visible text accessible label হতে পারে; icon-only button-এ label mandatory। `TextField`-এর placeholder-কে permanent label হিসেবে ধরে নেওয়া যাবে না; value লেখা শুরু হলে placeholder সরে যায়। Slider-এর label/value/adjustable action থাকবে। Loading indicator progress-এর accessible state প্রকাশ করবে। Value অতিরিক্ত দীর্ঘ হলে short label + optional hint ব্যবহার করা যাবে।

### 18.4 Dynamic type ও contrast

User text scaling বাড়ালে UI clip হবে না, button label কেটে যাবে না এবং modal inaccessible হবে না। Hardcoded line height ও fixed-height card এড়িয়ে চল। Contrast checker design-time-এ ভূমিকা রাখবে, কিন্তু image backgrounds, disabled states এবং focus indicators-এর জন্য manual review লাগবে। Colour vision deficiency-র জন্য status শুধু green/red দিয়ে নয়; icon ও text দিয়েও বোঝাও।

### 18.5 Reduced motion

OS/user reduced-motion preference থাকলে nonessential motion কমবে বা বন্ধ হবে। Animation state transition-এর semantic ফল অপরিবর্তিত থাকবে—motion বন্ধ হলেও page navigation, expansion বা status change ঘটে যাবে। Reduced-motion policy component API-তে override করা যাবে কি না তা নির্দিষ্ট হবে; safety-critical accessibility preference override নিরুৎসাহিত করো।

### 18.6 Accessibility acceptance gate

Release-এ অন্তত TalkBack, VoiceOver এবং HarmonyOS-এর available screen reader/accessibility service দিয়ে core user journeys হাতে পরীক্ষা হবে; large text, switch control/keyboard navigation এবং contrast scan-ও থাকবে। Automated checker সব সমস্যার নিশ্চয়তা দিতে পারে না। Accessibility bugs “minor styling” label দিয়ে বাদ দেওয়া যাবে না, যদি critical controls unreachable বা unlabeled হয়।

---

## 19. Animation, transition ও motion preferences

README-তে animation engine, easing curves ও timeline-এর কথা রয়েছে; UI roadmap-এ এই ক্ষমতাকে app UI semantics-এর সঙ্গে বেঁধে দিতে হবে। Animation শুধু `x=10` থেকে `x=100` পরিবর্তন নয়—start, end, cancel, retarget, interruption, reduced-motion, lifecycle এবং layout interaction রয়েছে। Component-এ animation চালিয়ে route pop হলে animator dispose হবে; app background-এ গেলে nonessential animation pause/cancel হবে।

### 19.1 Animation categories

- **State animation:** expanded/collapsed, selected/unselected, progress change.
- **Transition animation:** route, dialog, bottom sheet এবং shared element.
- **Gesture-driven animation:** drag position, swipe, interactive back gesture.
- **Decorative motion:** shimmer, looping illustration—সীমিত ও reduced-motion compliant.
- **Physics-like motion:** spring, decay, fling; native physics ও shared engine-এর পার্থক্য test করা প্রয়োজন।

### 19.2 Timing এবং curves

Easing curve catalogue থাকলেই usable animation system হয় না। Standard duration token (short/medium/long), interruption rule এবং spring parameters দরকার। Button pressed animation action dispatch আটকে রাখতে পারবে না। Layout animation-এ child reorder ও remove ঘটলে stable key চাই। Overshoot curve clipping বা native bounds overflow তৈরি করতে পারে; test থাকবে।

### 19.3 Frame scheduling

Animation update UI thread-এ অতিরিক্ত কাজ করবে না। State event, measure/layout, draw এবং native frame commit schedule-এর মধ্যে batching দরকার। 60fps target মানে প্রায় 16.7ms frame budget; 90Hz/120Hz device-এ frame budget আরও ছোট। UI framework নিজের 60 FPS দাবি করবে না; representative devices-এ p50/p95 frame time পরিমাপ করবে। Animation core এবং real platform renderer-এর benchmark আলাদা report হবে।

### 19.4 Animation cancellation

Animation শুরু/শেষ/বাতিলের state machine নির্ধারণ করো। একই property-তে দ্বিতীয় animation এলে আগেরটা cancel, merge বা retarget—কোন policy হবে? Dialog dismiss হওয়ার পর বন্ধ হয়ে যাওয়া node-এর transform update চলতে পারবে না। Interrupted animation-এর target state final semantic state অনুযায়ী resolve হবে; halfway visual frame state যেন domain model-এর সত্যকে বদলে না দেয়।

### 19.5 Shared element

Shared-element transition প্রথম release-এর blocker নয়। এটি implement করলে source ও destination node identity, route lifecycle, clipping, missing target, reduced-motion এবং interrupted navigation-এর behaviour পরীক্ষা করতে হবে। HarmonyOS ArkUI-তে shared element/geometry transition facilities-এর official guidance আছে; Alap adapter সেগুলি ব্যবহার করবে কি না capability assessment দিয়ে ঠিক করতে হবে। সব platform-এ একরকম effect অনুকরণ করতে গিয়ে essential interaction ভেঙে ফেলবে না।

---

## 20. Images, icons, fonts এবং resource pipeline

Resource API stable name ও typed intent ব্যবহার করবে। `Image(source: "assets/foo.png")` local resource হতে পারে, `Image(url: ...)` remote image loader—এই দুটি API-কে পার্থক্য করতে হবে। Image loading-এ placeholder, error fallback, aspect ratio, decode size, cache policy, cancellation ও accessibility description থাকবে। বড় original image সরাসরি decode করলে memory spike হয়; requested display bounds অনুযায়ী downsample করতে হবে।

### 20.1 Asset catalogue

`resources/images`, `resources/icons`, `resources/fonts`, `resources/locales`-এর naming convention নির্ধারণ করো। Asset ID compile-time-এ validate হবে; missing asset build fail অথবা documented fallback দেবে। Density-specific raster assets এবং vector assets-এর mapping আলাদা হবে। Resource path host filesystem-এর arbitrary absolute path হবে না। Bundle-এ asset inclusion deterministic ও inspectable থাকবে।

### 20.2 Icon model

Icon-এর জন্য semantic name (যেমন `search`, `close`, `chevronBack`) এবং custom vector/resource override থাকবে। Unicode emoji-কে production icon system-এর বিকল্প হিসেবে ব্যবহার করবে না—এগুলোর appearance, accessibility label, baseline ও tint platform/Font অনুযায়ী বদলায়। Icon button-এ label mandatory; decorative icon semantics tree থেকে বাদ যেতে পারে। Mirrored direction icon (back/forward) RTL ও platform convention অনুযায়ী adapt হবে।

### 20.3 Image loading lifecycle

Remote image request visibility ও lifecycle-এর সঙ্গে সম্পর্কিত হবে। List item offscreen হলে request cancel বা lower priority হতে পারে; recycled cell নতুন URL পেলে আগের request নতুন item-এ image বসাতে পারবে না। Image cache memory/disk limits থাকবে। Error retry user action-এ triggered হবে; automatic infinite retry নয়। Sensitive image local cache-এ থাকলে logout clear policy document করতে হবে।

### 20.4 Vector, SVG ও custom drawing

Vector assets scale-independent, কিন্তু SVG-এর সব feature support করা প্রয়োজন নেই। Supported subset, unsupported filter/mask behaviour ও sanitizer document করো। Untrusted SVG/image decoder-এর attack surface বিবেচনায় resource validation ও size limits থাকবে। Animation asset থাকলে reduced-motion এবং autoplay policy মানতে হবে। `Canvas`-এর জন্য SVG pipeline-কে অবধারিতভাবে reuse করতে হবে না; প্রয়োজন অনুযায়ী common resource API থাকতে পারে।

### 20.5 Fonts ও licenses

Bundled font-এর license, file size, subset, fallback এবং update policy documented হবে। Icon font glyph-কে accessible name ভেবে নেবে না। Bengali fallback fonts include করলে Latin, numerals ও punctuation-এর visual balance পরীক্ষা করো। Font cache locale/theme/scale changes-এ invalidation করতে হবে। Third-party font package-এর license compatibility release gate-এর অংশ হবে।

---

## 21. System bars, safe areas, keyboard insets ও device features

UI-এর content area সবসময় পুরো screen rectangle নয়। Status bar, navigation bar, camera cutout, rounded corners, gesture region, keyboard, floating input panel, foldable hinge এবং system taskbar content overlap করতে পারে। Safe area hardcoded top padding দিয়ে সমাধান করবে না; environment থেকে dynamic insets পড়বে। Android-এর official Compose guide বিশেষভাবে system bars ও display cutout insets handle করার কথা বলে, এবং Android 15+ target apps-এ edge-to-edge আচরণের জন্য আরও সতর্কতা প্রয়োজন।

### 21.1 Insets model

Environment model-এ অন্তত `safeDrawing`, `systemBars`, `ime`, `displayCutout`, `gesture` বা equivalent inset classes থাকবে। `Page`-এর root safe area consume করবে কি না explicit policy থাকবে; nested child যেন একই inset দ্বিগুণ consume না করে। Full-bleed header background system bar-এর পেছনে যেতে পারে, কিন্তু interactive title/control safe region-এ থাকবে।

### 21.2 Keyboard-aware layout

Keyboard show/hide event এলে viewport বা bottom inset update হবে। Focused `TextField` visible রাখার জন্য scroll adjustment প্রয়োজন হতে পারে। Fixed keyboard height ব্যবহার করা হবে না। Floating keyboard, split keyboard বা landscape keyboard-এর মতো variants-এ layout আলাদাভাবে test করবে। Keyboard dismissal drag/tap behaviour platform-specific; adapter সেটি surface করবে।

### 21.3 System bars styling

Status/navigation bar icon contrast, edge-to-edge drawing এবং gesture navigation safe region platform adapter-এর কাজ। App-কে platform flags string-এ দিতে না দিয়ে semantic property—যেমন `systemBarStyle`—ব্যবহার করতে পারো। তবে advanced developer escape hatch থাকলেও raw system API capability clearly platform-specific হিসেবে চিহ্নিত হবে। Status bar colour বদলালে system icon unreadable না হয়।

### 21.4 Insets test suite

প্রতিটি root screen top notch, bottom gesture navigation, keyboard-visible form, landscape, modal sheet এবং edge-to-edge configuration-এ পরীক্ষা হবে। Screenshot baseline-এ system bar crop/size-এর পার্থক্য explicitly normalize করা যাবে, কিন্তু actual overlap bug লুকানোর জন্য large ignored region রাখা যাবে না।

---

## 22. Platform adapters: Android, iOS, HarmonyOS

UI core shared হতে পারে, কিন্তু adapter তিনটি আলাদা engineering deliverable। Android adapter-এর native view/Compose embedding, lifecycle owner, back dispatch, keyboard/insets, accessibility semantics এবং resource integration থাকবে। iOS adapter-এর UIKit/SwiftUI embedding, responder/focus chain, keyboard safe areas, VoiceOver, scene lifecycle ও navigation gesture-এর সঙ্গে integration লাগবে। HarmonyOS adapter ArkTS/ArkUI-এর component model, state binding, navigation, accessibility, lifecycle এবং available platform APIs-এর সঙ্গে mapping করবে। Native API mapping-কে hidden magic না রেখে conformance table-এ লিখবে।

### 22.1 Android adapter

Android-এ `Activity`/`View` বা Jetpack Compose host—যেটি বেছে নেওয়া হয়, সেটিকে স্পষ্টভাবে document করতে হবে। প্রথম proof-of-concept-এ one-screen native host বানাও; সমস্ত native lifecycle logic UI core-এ নয়, adapter-এ থাকবে। Window insets ও keyboard state host থেকে UI environment-এ যাবে। TalkBack semantics এবং system back event-এর integration automated test + device test-এ যাচাই হবে। Process recreation হলে app state restoration contract প্রয়োগ করতে হবে।

### 22.2 iOS adapter

iOS-এ UI-কে Xcode build ও signing pipeline-এর মধ্যে host করতে হবে; cross-platform declaration থাকলেও app target, native resources, entitlements, app lifecycle ও App Store constraints আলাদা। UIKit hosting শুরুতে সুবিধাজনক হতে পারে, অথবা SwiftUI bridge উপযোগী হতে পারে; implementation decision benchmark এবং component parity দেখে নিতে হবে। Keyboard dismissal, dynamic type, safe area, VoiceOver focus, swipe-back এবং native picker integration আলাদা checklist। এই roadmap কোনো iOS adapter ইতিমধ্যেই আছে বলে ধরে নেয় না।

### 22.3 HarmonyOS adapter

HarmonyOS NEXT-এর UI-তে ArkTS ও ArkUI প্রধান technology; ArkTS typed/declarative model এবং ArkUI component APIs-কে adapter assessment-এর ভিত্তি হিসেবে ধরতে হবে। HarmonyOS API version, target device এবং available development kit-এর পার্থক্য conformance record-এ থাকবে। `Button`, `TextInput`, `List`, `Navigation`, dialog এবং accessibility properties-এর mapping আগে implement করো। HarmonyOS classic/legacy Android-compatible environments ও HarmonyOS NEXT-কে একই target label-এর মধ্যে অস্পষ্টভাবে মিশিও না; supported OS distribution স্পষ্টভাবে document করতে হবে।

### 22.4 Feature support matrix

প্রতিটি platform adapter-এ feature status: `supported`, `supported-with-difference`, `fallback`, `unsupported`, `not-tested`. README-তে “works on iOS” লেখা হবে না যদি শুধু compile test চলে কিন্তু real device-এ text input, navigation এবং accessibility পরীক্ষা হয়নি। Adapter version এবং OS/API level সংযুক্ত থাকবে।

### 22.5 Native escape hatch

কোনো app-কে platform-specific native control দরকার হলে `PlatformView` বা typed interop boundary থাকতে পারে। তবে এই escape hatch shared UI source-এর প্রতিশ্রুতি কমায়; capability declaration, ownership, sizing, accessibility এবং lifecycle contract বাধ্যতামূলক। Native view host/unhost হলে stale callbacks, UI thread rules এবং disposal ঠিক থাকবে। Internal private platform object public API-তে leakage না হয়।

---

## 23. Custom drawing, canvas এবং GPU boundary

Custom drawing UI framework-এর একটি optional advanced subsystem; সমস্ত component-এর ভিত্তি নয়। প্রাথমিক UI-তে native text, buttons ও forms-এর accessibility নিশ্চিত করার আগে custom GPU renderer-এ বড় বিনিয়োগ করো না। `Canvas` API-তে coordinate system, density, clipping, draw ordering, invalidation, hit testing, text drawing, image decoding এবং lifecycle semantics থাকবে। Canvas drawing-এর জন্য accessibility annotation/semantics পৃথকভাবে দিতে হবে; painted text স্বয়ংক্রিয়ভাবে screen reader-এ পাওয়া যায় না।

### 23.1 Coordinate space

Canvas local logical units-এ draw করবে; device pixel ratio backend-এ apply হবে। Transform matrix, clip region ও alpha stacking order deterministically defined থাকবে। Hit test transform-এর inverse ব্যবহার করে correct local coordinates পাবে। Rotation/scale-এর পরে pointer coordinate screen-space-এ রেখে দিলে touch mismatch হবে।

### 23.2 Invalidation

Drawing state বদলালে invalidation request হবে; প্রতি animation frame-এ পুরো scene invalidate করা যাবে, তবে partial dirty region optimization পরে। Canvas drawing pure function হওয়া ভালো: input model + size → commands. Native GPU state global mutable singleton হলে একাধিক screen/component একে অপরের colour, clip বা transform বদলে দিতে পারে। Context loss ও app resume-এর পরে resource recreate করতে হবে।

### 23.3 Accessibility overlay

Canvas-এর মধ্যে chart bar বা diagram node interactive হলে semantic child nodes আলাদা করে export করতে হবে। Hit target mapping drawing geometry-এর সঙ্গে consistent থাকবে। Gesture-only interaction-এর alternative control থাকতে হবে। Chart-এ data table/list alternative দাও; custom visual alone content-এর একমাত্র representation হবে না।

### 23.4 GPU fallback

যদি GPU backend unavailable হয়, Canvas-এর supported operations software renderer-এ fallback করা যাবে কি না নির্ধারণ করো। Feature parity ভিন্ন হলে build/runtime diagnostic দাও। README-তে “GPU renderer” উল্লেখ থাকলেই Vulkan/Metal/D3D submission implemented ধরে নেওয়া যাবে না; renderer capability ও benchmark evidence পৃথক প্রকাশ করো।

---

## 24. Errors, loading, empty states ও lifecycle correctness

UI-তে error state first-class। Component render exception, missing resource, unsupported property, image decode failure, route resolution failure, network loading এবং form validation—এসবের আলাদা UX থাকতে হবে। “সব error-এ blank screen” গ্রহণযোগ্য নয়। Developer mode-এ detailed diagnostics, release mode-এ safe user-facing fallback এবং redacted logs থাকবে। Sensitive values error message-এ দেখানো যাবে না।

### 24.1 Loading state

Progress indicator কেবল সময় কাটানোর animation নয়; request state ও accessible announcement-এ যুক্ত থাকবে। Short operation-এ spinner flicker এড়াতে minimum display threshold থাকতে পারে, কিন্তু তা request completion-এর semantic state বদলাবে না। Long operation progress percentage থাকলে সত্যিকারের measurable progress দরকার; unknown progress-এ indeterminate indicator।

### 24.2 Empty state

Empty list, no search results, first-use no data এবং load failure আলাদা screen state। Empty state-এ explanation ও next action থাকবে, কিন্তু data নেই বলে error দেখানো যাবে না। Search query change হলে empty state দ্রুত update হবে; stale result visible থাকলে তা explicitly stale হিসেবে বিবেচনা করবে।

### 24.3 Error boundaries

Component subtree crash করলে app পুরোটা বন্ধ হবে নাকি nearest error boundary fallback দেখাবে—এটি deliberate policy। Error boundary state reset/retry policy দেবে। Development logs-এ stack trace, component path ও source span থাকবে। Production telemetry-তে user data, password, typed text এবং access token পাঠানো যাবে না।

### 24.4 Lifecycle events

Mount, update, focus, blur, background, foreground, dispose এবং route suspend-এর semantics স্থির করো। Lifecycle callback-এর মধ্যে synchronous state mutation infinite render loop তৈরি করতে পারবে না। Background-এ nonessential animation suspend হবে; resume-এ animation timeline wall-clock না active time ধরে এগোবে তা নির্দিষ্ট হবে। Component disposal-এর পরে subscription/timeout/network callback leak হবে না।

### 24.5 State restoration

Process death, app force stop, low memory এবং route reconstruction-এ কী state restore হবে তা app policy নির্ধারণ করবে। UI framework সব transient state অন্ধভাবে serialize করবে না: password, payment details বা sensitive field value restore-এ ফিরিয়ে আনা নিরাপদ নাও হতে পারে। State restoration keys versioned হবে; schema বদলালে stale serialized state migrate বা discard হবে।

---

## 25. UI performance ও profiling

Performance target কোনো marketing number নয়; measured workload, hardware ও OS versionসহ report করতে হবে। UI framework-এর first frame time, state update latency, scrolling frame time, memory per view, list memory growth, image decode time, layout pass count এবং native node count মাপা উচিত। Average FPS একা যথেষ্ট নয়; p95/p99 frame time ও dropped-frame count ব্যবহার করো। Low-end Android device-এ perform করা না গেলে সেটিকে known limitation হিসেবে নথিভুক্ত করো।

### 25.1 Performance budgets

MVP-তে budget নির্ধারণের আগে baseline benchmark তৈরি করো। তারপর target define করো: simple screen initial mount, ১০০-item virtualized list, rapid counter updates, text input typing, theme switch ও navigation transition। Budget target hardware class অনুযায়ী হবে। UI framework নিজস্ব computation budget রাখবে যাতে one state update পুরো tree rebuild না করে।

### 25.2 Hot path

Hot path-এ map allocation, JSON serialization, reflection-heavy property lookup ও full tree reparse এড়িয়ে চল। Schema/type info compile-time-এ generate করো। Render function বারবার equivalent nodes produce করলেও stable diffing অপ্রয়োজনীয় native object recreate করবে না। তবে micro-optimization-এর আগে tracing দিয়ে real bottleneck শনাক্ত করো।

### 25.3 Lists ও images

Virtualization ছাড়া large list benchmark pass বলবে না। Image decoding background-safe path-এ হবে; UI thread-এ blocking IO নিষিদ্ধ। Cache eviction memory pressure events সম্মান করবে। Scroll চলার সময় image decode spikes ও expensive shadow effects profiling-এ দেখা যাবে। Lazy list test-এ ১০,০০০ dataset দিয়ে actual materialized node count bounded আছে কি না verify করো।

### 25.4 Profiling tools

`nil ui profile`-এর মতো tool report দিতে পারে: recomposition count, render duration, measure/layout duration, node create/update/dispose count, event queue delay এবং adapter command cost. `nil ui inspect` tree path ও current state দেখাবে, sensitive fields redacted থাকবে। Profiler নিজেই অতিরিক্ত overhead যোগ করলে sampling mode ও disabled-release policy দরকার।

### 25.5 Performance regression

CI benchmark hardware-এ না চললে noisy number-এর ভিত্তিতে সব PR fail করবে না। Stable emulator/device runner-এ controlled smoke benchmark, local profiler guidance এবং nightly performance trend রাখো। প্রতিটি significant UI architecture change-এর আগে/পরে comparative report থাকবে। Performance regression threshold exceed করলে issue ও owner নির্ধারিত হবে।

---

## 26. UI test architecture ও conformance suite

UI testing-কে snapshot test-এ সীমিত রাখবে না। পাঁচটি test layer দরকার: pure component logic, schema/type diagnostics, layout unit test, renderer/backend integration test, এবং real-device end-to-end/accessibility test। Cross-platform UI framework-এর প্রধান সম্পদ হলো shared semantics; সেই কারণে একই behaviour assertions তিন platform-এ চালানো যাবে এমন conformance suite নির্মাণ করো।

### 26.1 Unit tests

State change, derived values, event payload validation, form validation, route stack transitions, key reconciliation এবং style resolution unit-test হবে। এগুলো দ্রুত এবং platform-independent হওয়া উচিত। Time-dependent animations-এর clock injectable হবে যাতে tests deterministic হয়। Random IDs, global singleton state ও wall-clock dependence test-এ এড়িয়ে চলবে।

### 26.2 Layout tests

Constraints, wrapping, flex distribution, overflow, safe insets, large font scale এবং RTL layout-কে isolated measurement tests-এ যাচাই করবে। Exact pixel snapshots-এর পাশাপাশি semantic assertions থাকবে। Text metrics platform-specific হলে expectation tolerance documented হবে; tolerance এত বড় হবে না যে actual clipping গোপন হয়।

### 26.3 Snapshot tests

Browser preview-এর HTML screenshot এবং native screenshots আলাদা baseline family। Operating system, font version, density, light/dark theme ও API level record করতে হবে। Screenshot change-এর review tooling diff overlay দেখাবে। Snapshots কখন update করা হয়েছে—reviewed expected change নাকি test workaround—commit note-এ স্পষ্ট থাকবে।

### 26.4 Semantic/accessibility tests

Test API দিয়ে button label/role/enabled state, TextField label/value/hint, focus order, dialog modal state এবং list item identity assert করতে হবে। Real device-এ screen reader manual testingও থাকবে। Automated accessibility checker অনেক সমস্যা ধরতে পারে, কিন্তু pronunciation, interaction discoverability ও task completion মানুষের পর্যালোচনা দাবি করে।

### 26.5 Lifecycle tests

Route push/pop, background/foreground, rotation/window resize, keyboard show/hide, component disposal, interrupted animation এবং process restoration test করবে। Async callback after dispose ঘটলে no-crash ও no-state-mutation guarantee verify হবে। App resume-এর পর duplicate listeners বা duplicate event emission হচ্ছে কি না counts দিয়ে পরীক্ষা করা হবে।

### 26.6 Cross-platform golden cases

Shared test fixture-এ screen description, data state, interactions, expected semantics এবং expected visual snapshots থাকবে। Android, iOS, HarmonyOS adapters একই semantic assertions pass করবে। Platform appearance differences-এর জন্য per-platform golden image থাকতে পারে, কিন্তু action/value/navigation/accessibility expectation shared হবে।

---

## 27. Browser preview ও hot reload-এর সীমা

Browser preview developer speed-এর জন্য মূল্যবান: UI tree দ্রুত দেখতে, state logic পরীক্ষা করতে এবং documentation examples render করতে সাহায্য করে। কিন্তু HTML/CSS renderer mobile native UI-র exact substitute নয়। CSS font wrapping, browser focus, DOM accessibility, pointer events, scrolling ও native keyboard behaviour mobile platform adapter-এর আচরণ থেকে আলাদা। তাই preview-এর result-কে iOS/Android/HarmonyOS parity-এর প্রমাণ হিসেবে দেখানো যাবে না।

### 27.1 Preview contract

একই typed UI IR থেকে HTML preview adapter তৈরি হবে। Preview adapter unsupported native feature-এর জন্য visual placeholder ও diagnostic দেখাবে। উদাহরণস্বরূপ native date picker-এর browser preview HTML input ব্যবহার করতে পারে, কিন্তু output message-এ `preview approximation` থাকবে। Preview-তে একটি feature নিখুঁত কাজ করলেও production native adapter আলাদা acceptance criteria pass করতে হবে।

### 27.2 Hot reload

Hot reload-এর লক্ষ্য source change-এর পরে state যতটা সম্ভব preserve করে updated UI দেখা। Hot reload সব state migration guarantee করবে না। Component type/property schema বদলালে state reset হতে পারে; dev server warning দেখাবে। Native platform hot reload support সীমিত হলে fast rebuild অথবা restart workflow document করতে হবে। Hot reload failure যেন runtime silently inconsistent রেখে না দেয়—safe full reload fallback থাকবে।

### 27.3 Event session

README-তে `nil dev` browser sessions server-side component instance হিসেবে বর্ণিত, cookie session-এর মাধ্যমে state আলাদা থাকে। এটি browser development mode-এর উপযোগী হতে পারে, কিন্তু native mobile production runtime-এ server session-নির্ভর local button state অনিবার্য করা উচিত নয়। Native app-এ ordinary tap local state update করতে network roundtrip লাগবে না, যদি developer স্পষ্টভাবে remote UI architecture বেছে না নেন। Preview ও mobile semantics-এর এই বিভাজন জরুরি।

### 27.4 Preview-to-device workflow

একটি developer command `nil ui preview` browser preview চালু করতে পারে; `nil ui run android` নির্বাচিত device/emulator-এ run করতে পারে; `nil ui screenshot --platform ...` visual baseline capture করতে পারে। Commands প্রস্তাবিত—বর্তমান implementation-এর দাবি নয়। CLI output কোন adapter, runtime version, platform SDK ও unsupported features ব্যবহৃত হয়েছে তা দেখাবে।

---

## 28. UI developer tools, formatter, language server ও inspector

UI API যত ভালোই হোক, developer experience দুর্বল হলে adoption হবে না। Syntax highlight, completion, hover documentation, go-to-definition, rename, diagnostic, preview, inspector এবং formatter-এর roadmap component library-র সঙ্গে চলবে। নীলাংয়ের type checker যেহেতু component/property schema জানবে, LSP একই schema থেকে code completion ও diagnostics সরবরাহ করতে পারে। Documentation আলাদা হাতে maintain করলে mismatch বাড়বে; সম্ভব হলে API metadata থেকে reference generate করো।

### 28.1 Formatter

Formatter child nesting, property order, event handler indentation এবং line wrapping standardize করবে। Format idempotent হবে: একবার format-এর পরে আবার চালালে output পরিবর্তন হবে না। Formatter semantic content বদলাতে পারবে না। Golden tests-এ comments, Bengali strings, nested maps এবং multiline closures থাকবে।

### 28.2 Inspector

Inspector-এ rendered tree, current properties, source span, measured size, layout constraints, accessibility node এবং platform-native object mapping দেখা যাবে। Selected node highlight করলে app UI-তে exact bounds overlay হবে। State inspector sensitive fields default-এ redact করবে। Developer event dispatch trace দেখে কোন action কোন state বদলেছে বুঝতে পারবে।

### 28.3 Diagnostics

Error শুধু “invalid node” নয়। Expected component, expected property type, actual value, source line, platform support এবং suggested repair থাকবে। Unsupported on HarmonyOS property থাকলে build-time error অথবা explicit fallback warning দেখাবে। Diagnostic codes stable হবে যাতে editor filters ও CI suppression policy নির্ভর করতে পারে।

### 28.4 Documentation preview

প্রতিটি component page-এ sample code, interactive preview, property table, semantics role, platform support matrix, accessibility guidance এবং pitfalls থাকবে। Preview unavailable platform-specific control-এর visual approximation হিসেবে চিহ্নিত থাকবে। Example copy button সম্পূর্ণ standalone source দেবে, hidden setup step বাদ দেবে না।

---

## 29. Design system, component catalogue ও documentation

Alap UI-র core component set ও higher-level design system আলাদা রাখো। Core UI layout, text, input, interaction এবং semantics-এর building blocks দেবে। Design system button variants, card styles, app bar, navigation bar, form field decoration, list tile, empty state ও common screen pattern দেবে। Design system-এ design token ব্যবহার হবে; app-specific theme core semantics override করবে না।

### 29.1 Core vs design kit

Core: `Page`, `Row`, `Column`, `Stack`, `Text`, `Button`, `TextField`, `Image`, `List`, `ScrollView`, `Dialog`, `Spacer`, `Divider`, `Semantics`. Design kit: `PrimaryButton`, `SectionHeader`, `ListTile`, `AppBar`, `SearchField`, `FormField`, `EmptyState`, `ErrorPanel`. Namespaces আলাদা রাখো যাতে native-like generic Button-কে branded PrimaryButton-এর সঙ্গে গুলিয়ে না ফেলা হয়।

### 29.2 Component maturity gates

`experimental` component internal prototype-এ ব্যবহার হতে পারে; `preview` API-র প্রধান আচরণ স্থিতিশীল, কিন্তু platform coverage অসম্পূর্ণ; `stable` component-এ semantics, accessibility, three-platform tests এবং documentation আছে; `deprecated` component migration path ও removal schedule সহ থাকবে। Component public export হওয়াই “stable” হওয়ার প্রমাণ নয়।

### 29.3 Example app portfolio

কমপক্ষে এই reference screens বানাও: counter; sign-in form; settings/preferences; searchable list; detail page; tabs + nested navigation; dialog/sheet; image gallery; dynamic type page; Bengali news/article view; offline/error states; adaptive master-detail. প্রতিটি app minimal dependency ব্যবহার করবে, UI pattern এবং test সহ থাকবে। শুধু visual demo নয়; state, empty/error, accessibility এবং lifecycle behaviour অন্তর্ভুক্ত থাকবে।

### 29.4 Design review

Component API review-তে প্রশ্ন: এটি core-এ থাকা উচিত? native adapter দিয়ে implement করা যায়? accessibility semantics কী? large text-এ চলে? empty/error/loading state আছে? keyboard/navigation-এ কী হয়? platform differences আছে? testability কেমন? API future extension-এ breaking change লাগবে? এসবের উত্তর না থাকলে feature merge স্থগিত রাখবে।

---

## 30. UI API design, versioning ও backward compatibility

UI semantics source code-এর অংশ, তাই minor implementation tweak-ও end-user behaviour বদলাতে পারে। `Button` activation আগে `onActivate`-এ একবার fire করত, পরে pointer-up ও accessibility activation উভয় route থেকে দুইবার fire করলে এটি bug। Layout defaults, focus order, accessibility label, scroll restoration এবং form submission behaviour semantic API। Release notes-এ শুধুমাত্র renamed function নয়, behavioural change-ও লিখবে।

### 30.1 Versioning policy

Stable component API semantic versioning অনুসরণ করবে। Breaking changes-এর আগে deprecation release, migration tool বা clear replacement থাকবে। Preview API বদলাতে পারে, কিন্তু compiler warning এবং migration note দিতে হবে। Internal platform adapter refactor-এর ফলে shared API behaviour বদলাতে পারবে না।

### 30.2 Feature flags

Experimental renderer বা নতুন layout engine feature flag দিয়ে চালু করা যায়। Feature flag-এর default deterministic, project-local এবং CI-তে explicit হবে। Production release-এ runtime remote flag-এর উপর core layout semantics নির্ভর করতে দিও না। Flag remove করার owner ও target version থাকবে।

### 30.3 Platform capability declarations

API-তে platform-specific feature প্রয়োজন হলে compiler `target` config জানবে। উদাহরণ: supported target-এ native date picker, কিন্তু preview browser control; unsupported platform-এ error. “Fallback to custom” চাইলে developer opt-in করবে, কারণ fallback accessibility বা interaction আলাদা হতে পারে। Static capability manifests API docs ও build diagnostics-কে একই source of truth দেবে।

### 30.4 Migration testing

একটি sample project পুরোনো API থেকে নতুন API-তে migrate করে test করো। Formatter ও compiler warning ব্যবহার করে deprecated property শনাক্ত করবে। Behaviour-sensitive snapshot/semantic tests migration-এর আগে-পরে run হবে। Migration guide শুধু syntactic find/replace নয়; layout defaults বা state lifetime বদলালে তা ব্যাখ্যা করবে।

---

## 31. Security, privacy ও untrusted content-এ UI boundary

UI framework নিজেই security boundary-র কিছু অংশ। User-provided HTML/Markdown/URL render করলে injection, phishing UI, malicious SVG, oversized image, deep links এবং log leakage-এর ঝুঁকি থাকে। Framework-এর default-এ untrusted content arbitrary native node, JavaScript execution বা platform capability পাবে না। Rendered text এবং executable UI description আলাদা data types হবে।

### 31.1 Untrusted text vs UI tree

`Text(userInput)` string render করবে; সেটি component declarations হিসেবে evaluate হবে না। Remote JSON দিয়ে UI schema চালানোর feature দরকার হলে allowlisted component/property schema, versioning, signature/trust policy, resource limits এবং sandbox দরকার। সাধারণ API response-কে dynamic executable UI code হিসেবে ধরে নেওয়া যাবে না।

### 31.2 UI logs ও screenshots

Inspector, snapshots ও crash report-এ input values redacted করার policy থাকবে। Password, OTP, payment details, auth token এবং sensitive personal data trace output-এ থাকবে না। Test snapshots-এ deterministic fake data ব্যবহার করো। Developer override দিয়ে redaction off করা হলেও production environment-এ সেটি disabled থাকবে।

### 31.3 Deep links ও dialogs

Untrusted deep-link arguments typed route validation পার হবে। App যদি URI থেকে sensitive action চালায়, UI confirm step বা appropriate authentication লাগতে পারে। Modal overlay visually trusted-looking UI imitate করতে পারে—framework নিজে phishing solve করতে পারবে না, কিন্তু app boundary ও system permission dialog-এর native ownership রক্ষা করবে। Native system prompts custom drawn replacement হিসেবে বানানো উচিত নয়।

### 31.4 Resource exhaustion

Huge text, deeply nested tree, enormous image dimensions, recursive component structures এবং oversized dynamic UI schema resource exhaustion ঘটাতে পারে। Compiler/runtime depth limits, image byte/pixel budgets, recursion safeguards ও diagnostic থাকবে। Layout recursion cycle detect করতে হবে। Input content size limits documented হবে।

---

## 32. Motion, contrast, colour এবং inclusive design policies

Inclusive UI হলো বাস্তব user environment-কে design input হিসেবে গ্রহণ করা। Text size preference, high contrast, reduced motion, screen reader, switch access, keyboard, colour scheme, display zoom এবং low vision UI component behaviour বদলাতে পারে। এই preferences per component hack হিসেবে নয়, environment model ও theme resolution-এ থাকবে।

### 32.1 Colour scheme ও contrast

`light`, `dark`, `system` theme preference-এর semantics থাকবে। System theme runtime-এ বদলালে current route/state না হারিয়ে visual tokens update হবে। Contrast validation theme tokens-এর উপর test করা যায়; image background ও dynamic text-এর জন্য app-level review প্রয়োজন। Focus outline, selection highlight এবং disabled state-এর contrast আলাদা করে দেখা হবে।

### 32.2 Reduced transparency ও motion

কিছু user transparency/blur effect পড়তে অসুবিধা বোধ করতে পারেন। Framework environment preference এলে blurred background-এর পরিবর্তে solid surface নিতে পারে। Reduced motion থাকলে parallax, auto-scrolling, looping decorative animation ও large spatial transition কমবে। Essential progress/status communication বজায় থাকবে।

### 32.3 Accessible defaults

Icon-only actionable component label ছাড়া compile warning দিতে পারে; `TextField`-এ label/labelledBy ছাড়া warning; clickable non-interactive `Text` ব্যবহার হলে semantic button wrapper পরামর্শ; touch target খুব ছোট হলে lint warning; contrast lint approximate বলে documentation থাকবে। Lint false-positive suppression reason সহ হতে পারে, কিন্তু suppression অকারণে সহজ হওয়া উচিত নয়।

### 32.4 User testing

Internal accessibility test যথেষ্ট নয়। বাস্তবে screen reader ব্যবহারকারী, low-vision user এবং keyboard/switch navigation ব্যবহারকারীদের feedback roadmap-এ রাখো। Engineering acceptance শুধু API presence নয়; user core workflow complete করতে পারছেন কি না তা দেখতে হবে। বাংলা text ও mixed numeral pronunciation স্থানীয় ভাষাভাষী testers দিয়ে যাচাই করা বিশেষভাবে গুরুত্বপূর্ণ।

---

## 33. Data-heavy app patterns ও বাস্তব উদাহরণ

নির্দিষ্ট app-এর UI বানিয়ে architecture যাচাই করো। UI framework “counter demo works” অবস্থায় থেকে গেলে production concern আড়ালে থাকবে। POS, বাংলা গানের database, typing practice এবং news/article app-এর মতো workloads ভিন্ন ধরনের UI pressure তৈরি করে: দ্রুত data entry, searchable list, Bengali text shaping, bulk list rendering, offline/error states এবং form validation। এই উদাহরণগুলি Alap-এর cross-platform promise বাস্তবে পরীক্ষা করতে কাজে লাগতে পারে।

### 33.1 POS product list

Search field, category filter, virtualized item list, quantity picker, total summary এবং checkout action থাকবে। Rapidly changing quantity-এর ফলে whole screen rerender না হওয়া, numeric keyboard, large text, price format, empty inventory, loading/error state ও keyboard inset পরীক্ষা করো। Touch targets দ্রুত দোকানে ব্যবহারের জন্য যথেষ্ট বড় হবে। Business logic UI component-এর ভিতরে লুকিয়ে থাকবে না; UI শুধু state ও actions render করবে।

### 33.2 বাংলা গানের database

Searchable list, detail page, long Bengali text, metadata, favourites ও offline empty/error states থাকবে। Text wrapping, mixed Bengali/Latin author names, scroll restoration এবং screen reader pronunciation গুরুত্বপূর্ণ। বড় dataset-এ virtualization ও search debouncing হবে। Long text-এ line height, font fallback এবং text selection বাস্তব Android/iOS/Harmony devices-এ যাচাই করা হবে।

### 33.3 Typing practice app

Text entry focus, live character feedback, timer, progress, keyboard handling এবং accessibility challenge থাকবে। বাংলা যুক্তাক্ষর বা combining sequence টাইপ করার সময় correctness calculation UTF-16 code unit count ধরে হবে না; grapheme/character policy স্পষ্ট হতে হবে। IME composition চলাকালে incomplete composition text-কে premature error হিসেবে গণনা না করার design দরকার। Reduced motion ও large font ব্যবহারেও exercise usable হবে।

### 33.4 Settings ও preferences

Switch, radio group, theme, font size, language, notification preference এবং reset action থাকবে। Preferences state update immediate হলেও persistence semantics error handle করবে। System setting-এ user ফিরে এলে environment refresh হবে। Theme change layout state reset করবে না। Accessible grouping, labels এবং section headings থাকবে।

### 33.5 Search, loading ও retry

Search query input, in-flight request indicator, result list, empty results, network error, retry এবং stale data-র UI state machine লিখো। Rapid query change-এ stale response drop হবে। Empty state এবং error state আলাদা হবে। Screen reader announcements spam করা যাবে না; loading announcement frequency controlled হবে।

### 33.6 Master-detail adaptive layout

Compact screen-এ list→detail navigation; expanded screen-এ side-by-side pane. Same route/model state দুই layout class-এ কাজ করবে। Window resize-এ detail selection হারাবে না। Back behaviour compact vs expanded-এ documented হবে। Foldable hinge/safe segment future integration-এর জন্য layout model flexible থাকবে।

---

## 34. Example app: counter থেকে production-quality app

প্রথম reference app counter হলেও acceptance criteria শুধু button count বাড়া নয়। Counter-এর production exercise-এ initial state, increment/decrement, step selection, reset confirmation, accessibility labels, disabled/min/max rules, theme switching, state restoration এবং event test থাকবে। এতে UI core-এর state/event semantics প্রথমেই প্রমাণ করা যায়।

### 34.1 Baseline screen

Screen title, current count, increment button, decrement button ও reset action। Layout declarative, values typed, component keys stable. `count`-এর formatted display derived state হবে; string version আলাদা mutable state নয়। Button event state transaction ব্যবহার করবে; disabled logic native/accessibility state-এও map হবে।

### 34.2 Edge cases

Count-এর min/max overflow, rapid tap, negative values allowed কি না, reset dialog dismissal, keyboard/device accessibility activation এবং route recreation test করতে হবে। Button tap দুইবার fire হচ্ছে কি না event count assertions থাকবে। Count change animation reduced-motion preference মানবে। Text size বাড়লেও button reachable থাকবে।

### 34.3 State restoration

App background/foreground, screen recreate এবং process restoration-এর policy define করো। Demo counter-এ persistent preference optional; কিন্তু test harness-এ transient state restoration mechanism থাকা উচিত। Component unmount/mount-এ state lifetime accidentalভাবে global থাকলে test fail করবে।

### 34.4 Cross-platform review

প্রতিটি target-এ button, label, spacing, system insets, accessibility action, event count এবং theme mode যাচাই হবে। Pixel differences report করা যাবে; semantic regressions গ্রহণযোগ্য নয়। Preview-only passing test native adapter coverage-এর বদলে গণ্য হবে না।

---

## 35. Repository layout ও module boundaries

UI implementation-কে `pkg/alap`-এর ভেতরে অস্পষ্টভাবে জমা না করে conceptual modules অনুযায়ী ভাগ করো। বিদ্যমান repository layout প্রজেক্টের evolving state অনুযায়ী ভিন্ন হতে পারে; নিচের structure একটি প্রস্তাব, existing files-এর উপর সরাসরি overwrite করার নির্দেশ নয়। প্রথমে inventory এবং dependency graph তৈরি করবে, তারপর ধীরে ধীরে package extract করবে।

```text
pkg/alap/
  api/             # public component/property/event types
  schema/          # component metadata, defaults, capability declarations
  ir/              # typed UI tree and source spans
  runtime/         # state, scheduling, lifecycle, event dispatch
  layout/          # measurement, constraints, placement, logical units
  semantics/       # accessibility roles, labels, actions, focus order
  theme/           # tokens, typography, colour schemes
  resources/       # image, icon, font, locale lookup
  navigation/      # route stack, back policy, restoration
  animation/       # animation orchestration and motion preferences
  renderer/        # renderer contracts and reconciliation
  preview/html/    # HTML preview adapter
  platform/android/# Android host/adapter
  platform/ios/    # iOS host/adapter
  platform/harmony/# HarmonyOS ArkUI adapter
  testing/         # test helpers, fixtures and conformance cases
  docs/            # generated or hand-maintained API references
```

### 35.1 Dependency rules

`api/schema/ir` platform modules import করবে না। Android adapter-এর type `UI core`-এ leak হবে না। Platform adapter `UI runtime`-এর public bridge ব্যবহার করবে। Preview HTML adapter production mobile code path-এর required dependency হবে না। Resources core format-এ logical IDs থাকবে; platform modules actual bundling resolve করবে। Circular dependency নিষিদ্ধ বা CI architecture test দিয়ে ধরা হবে।

### 35.2 Build tags ও test isolation

Go build tags, cgo/native bridges বা platform-specific source files থাকলে core package host machine-এ testable রাখতে হবে। Linux/Windows development machine-এ iOS runtime executable না হলেও schema/IR/layout unit tests চলতে পারবে। Platform integration tests appropriate SDK runner-এ চলবে। Test command-কে `go test ./...`-এর বাইরে hidden manual steps-এ নির্ভর করতে দিও না।

### 35.3 Public/internal distinction

`api` public stable types রাখবে; `internal` package implementation detail। Internal renderer node structure app developer import করতে পারবেন না। Public API-তে platform native view-এর pointer দিলে portability ভাঙে; escape hatch আলাদা experimental namespace-এ থাকবে।

### 35.4 Documentation as code

Component schema-তে descriptions, example, default, platform support এবং semantic role metadata রাখলে API docs generate করা যায়। কিন্তু generated docs মানব review ছাড়া publish করবে না; warnings, pitfalls ও conceptual explanations হাতে লেখা প্রয়োজন। Example files CI-তে compile/render/test হবে।

---

## 36. Implementation phases ও dependency order

রোডম্যাপকে parallel massive rewrite হিসেবে নয়, working vertical slice হিসেবে এগোতে হবে। প্রতিটি phase শেষে user-visible demo থাকবে এবং regressions detect করার test থাকবে। সময় অনুমান ব্যক্তিগত capacity ও repository state-এর উপর নির্ভরশীল; তাই এখানে milestone-based order দেওয়া হলো, অবাস্তব fixed deadline নয়।

### Phase 0 — Existing behaviour inventory

`pkg/alap`, examples, renderer এবং event path audit করো। Component registry, property types, current render format, event payload behaviour, session state, HTML renderer tests এবং existing bugs তালিকাভুক্ত করো। “implemented”, “prototype”, “declared-only” tag ব্যবহার করো। Current counter test stable করো। Deliverable: `docs/ui/STATUS.md` এবং current API inventory.

### Phase 1 — Formal schema ও typed tree

MVP components-এর schema এবং defaults নির্দিষ্ট করো। Current map renderer input থেকে typed IR build করার adapter তৈরি করা যেতে পারে, যাতে migration ধাপে ধাপে হয়। Unknown property ও invalid child-type diagnostics যোগ করো। Deliverable: schema tests, IR dump command এবং counter example-এর typed validation.

### Phase 2 — State/event contract

Render purity rules, event ordering, payload typing, stable keys, state transaction ও lifecycle cleanup define করো। Full tree rerender থাকলেও semantics deterministic হতে হবে। Duplicate event, dispose-after-callback এবং invalid payload test যোগ করো। Deliverable: `ui state doctor` বা equivalent test report.

### Phase 3 — Layout foundation

Logical units, constraints, Row/Column/Stack, text measurement boundary, wrapping, padding, gap ও scroll viewport implement করো। প্রথমে browser renderer ও layout unit tests-এ কাজ করা যায়, কিন্তু implementation contract mobile adapters-এর উপযোগী হতে হবে। Deliverable: responsive counter/form/list preview এবং layout debug overlay.

### Phase 4 — Native host proof on Android

একটি Android host shell তৈরি করো যা typed tree থেকে ৮–১২টি MVP component render করে। Native TextField, Button, focus, back, insets, keyboard এবং semantics validate করো। Android proof passing না হওয়া পর্যন্ত দশটি নতুন visual component যোগ করার কাজ থামাও। Deliverable: real phone install/run/demo plus automated smoke test.

### Phase 5 — Accessibility and design tokens

Semantic tree, labels, focus, large text, contrast, reduced motion, light/dark themes এবং Bengali typography tests implement করো। TalkBack-এ core flows পরীক্ষা করো। Deliverable: accessibility checklist, screenshot samples and known limitation list.

### Phase 6 — iOS adapter proof

iOS native host, component mappings, keyboard/safe area, VoiceOver, navigation back gesture এবং process/state restoration implement করো। Xcode toolchain, build configuration এবং test target document করো। Deliverable: signed development build on a real compatible device/test environment, core semantic tests.

### Phase 7 — HarmonyOS adapter proof

ArkTS/ArkUI host integration, typed state mapping, navigation, keyboard, accessibility এবং target/API version coverage implement করো। HarmonyOS NEXT-এর target assumptions লিখে রাখো। Deliverable: deployable development app on real device or supported test environment with documented parity report.

### Phase 8 — Virtualized lists, images, dialogs

Core semantics তিন platform-এ stable হলে virtualization, image caching, dialogs/sheets এবং common design system components যোগ করো। Each feature adds tests to all active adapters; platform gaps documented. Deliverable: searchable list/detail reference app.

### Phase 9 — Incremental renderer and profiling

Stable key reconciliation, property diff, layout vs paint invalidation, profiler এবং performance budgets যোগ করো। Baseline vs new renderer benchmarks report করবে। Deliverable: render count reduction evidence, not merely a code architecture diagram.

### Phase 10 — Stable API release

API stability audit, backward compatibility tests, documentation, migration guide, platform support matrix, accessibility review এবং issue triage complete করো। Stable ঘোষণার আগে core journeys তিন platform-এ passing হতে হবে অথবা release নাম স্পষ্টভাবে `Android stable; iOS preview; HarmonyOS experimental` বলতে হবে।

---

## 37. Acceptance criteria, test matrix ও release gate

UI release “complete” হবে যদি acceptance criteria পূরণ হয়। Feature list-এ tick দেওয়া যথেষ্ট নয়; test evidence, device/OS version এবং known limitations record করতে হবে।

### 37.1 Core component gate

প্রতিটি stable component-এর জন্য: typed property validation, default behaviour, disabled/error state, child contract, key/reconciliation behaviour, accessibility role/label/action, theme integration, large font test, layout test, Android/iOS/Harmony adapter mapping, browser preview approximation note এবং API reference থাকতে হবে।

### 37.2 Platform gate

প্রতি platform-এর জন্য build/install/run smoke test; lifecycle and back navigation; keyboard and insets; accessibility screen reader test; theme/system appearance; orientation/window resize; scroll/list performance; error/fallback behaviour; version/API metadata এবং known limits থাকতে হবে। এক platform test pass অন্য platform-এর evidence নয়।

### 37.3 Release artifacts

Release note-এ supported components, supported OS/API range, unsupported features, performance figures, accessibility status, migration notes এবং reproducible sample project links থাকবে। UI framework-এর version, compiler version, runtime version এবং adapter version compatibility matrix publish করো।

### 37.4 Blockers

Unlabeled essential control, inaccessible focus trap, password/log leakage, keyboard-covered submit button, crash on dispose, duplicate event dispatch, list item state leakage, invalidation infinite loop এবং native UI thread blocking—এসব release blocker। Cosmetic 1–2px screenshot variance documented tolerance-এর মধ্যে থাকতে পারে; user cannot complete primary workflow এমন mismatch চলবে না।

---

## 38. Bug triage, performance budget ও engineering rituals

UI bug report template-এ expected behaviour, actual behaviour, reproduction steps, source component path, platform/OS version, theme/font scale, screen recording/screenshot, accessibility service status, logs এবং minimal source sample থাকবে। “iPhone-এ কাজ করছে না” যথেষ্ট নয়। Platform-specific bug shared API semantics, adapter mapping, native OS difference নাকি unsupported capability—তা triage করতে হবে।

### 38.1 Severity

- **P0:** crash, data leak, app unusable, accessibility barrier across primary workflow.
- **P1:** primary action blocked, text input unusable, focus trap, major layout overflow or wrong state.
- **P2:** noncritical visual mismatch, secondary component degradation, documented workaround exists.
- **P3:** polish, documentation typos, minor developer ergonomics.

Accessibility blocker P2 হিসেবে নামিয়ে দেওয়া যাবে না শুধু visual issue বলে মনে হচ্ছে বলে। Severity user impact ও workaround-এর ভিত্তিতে হবে।

### 38.2 Pull request checklist

UI PR-এ API and semantics update, cross-platform impact, source docs, tests, accessibility label, theme compatibility, localization/Bengali strings, performance impact, lifecycle cleanup এবং known limitations উল্লেখ থাকবে। নতুন public component যোগ করতে চাইলে শুধু screenshot নয়, behaviour tests লাগবে। Adapter-specific change-এর সঙ্গে feature matrix update করতে হবে।

### 38.3 Weekly evidence

প্রতি development cycle শেষে dashboard/report-এ: stable component count, conformance pass rate per platform, open P0/P1 issues, crash/regression count, accessibility checklist completion, startup/scroll benchmarks, documentation coverage এবং known limitations দেখাও। Component count একমাত্র metric হলে unfinished widgets যোগ করে false progress হবে।

---

## 39. ঝুঁকি, যে কাজ এখনই করা উচিত নয়

Alap UI-কে পরিপূর্ণ করতে গিয়ে সবচেয়ে বড় ঝুঁকি হলো একই সঙ্গে অনেক subsystem শুরু করা। Custom renderer, complex DSL, 100+ widgets, animation timeline, state management, router, theme engine, low-level GPU API এবং three mobile backends একসঙ্গে তৈরি করলে integration surface দ্রুত বেড়ে যাবে। Core semantics স্থিতিশীল হওয়ার আগে UI API বারবার বদলাবে এবং app developers-এর বিশ্বাস কমবে।

এখনই করা উচিত নয়:

- Web CSS-কে সরাসরি mobile layout specification ধরে নেওয়া। CSS preview useful adapter, authoritative layout semantics নয়।
- সব UI control custom paint করা, তারপর accessibility ও text input later করতে বলা।
- README-তে support claim যোগ করা কিন্তু real-device evidence না রাখা।
- ১০০টি component বানিয়ে state, layout, keys, focus ও tests অসম্পূর্ণ রাখা।
- Platform-specific API-কে shared public contract-এ লুকিয়ে ফেলা।
- Render function-এ arbitrary side effects অনুমোদন করা।
- UI node-এর identity index বা memory pointer থেকে অনুমান করা।
- Large list-এ unbounded eager mount ব্যবহার করা।
- Raw string payload ও dynamic map-কে সবসময় typed API-এর বিকল্প হিসেবে চালু রাখা।
- Browser preview pass-কে native app acceptance ধরে নেওয়া।
- 60 FPS guarantee দাবি করা, কিন্তু device/OS/build configurationসহ benchmark না দেখানো।

### 39.1 Risk register

| ঝুঁকি | সম্ভাব্য ফল | প্রতিরোধ | প্রথম evidence |
|---|---|---|---|
| UI schema অনির্দিষ্ট | adapter-specific behaviour divergence | typed IR + conformance tests | counter/form parity suite |
| TextField custom-rendered | IME/selection/accessibility bug | native input default | Bengali input manual test |
| Reconciliation key দুর্বল | wrong row state/focus | stable keys + duplicate-key diagnostics | reorder tests |
| Insets hardcoded | keyboard/notch overlap | environment inset model | device matrix |
| Preview-কে native মনে করা | false support claim | platform acceptance gate | real-device test log |
| Too many components | maintenance debt | small stable core | release checklist |
| Untrusted UI schema | injection/resource abuse | data/code separation + limits | adversarial tests |

---

## 40. দীর্ঘমেয়াদি product direction

Alap UI-র শক্তি হতে পারে: Nilang-এর নিজস্ব typed declarative model, বাংলা ভাষা ও typography-র প্রথম-শ্রেণির পরীক্ষা, একই semantic UI tree থেকে multiple adapters, capability-aware diagnostics এবং সহজ onboarding। কিন্তু এই সুবিধাগুলি কেবল তখনই product differentiation, যখন core components dependable এবং documentation reproducible। “আমাদের UI সব platform-এ identical” বলার বদলে “এক source-এ behaviour, layout intent ও semantics share হয়; platform-native interaction সম্মান করা হয়”—এটি সত্য ও টেকসই অবস্থান।

### 40.1 Framework adoption

Developer adoption বাড়াতে শুরুতে official template, runnable sample, clear errors এবং one-command preview বেশি গুরুত্বপূর্ণ, advanced animation DSL নয়। Starter project-এ welcome screen, list/detail, login form এবং theme switching থাকবে। Documentation প্রতিটি command-এর expected output ও troubleshooting দেখাবে। Installation flow যদি native SDK requirement আড়াল করে, build fail হলে developer বুঝতে পারবে না; তাই prerequisites স্পষ্ট থাকবে।

### 40.2 Platform governance

প্রতিটি adapter-এর maintainer/owner, API-level support policy, test device matrix এবং release notes থাকবে। HarmonyOS API evolution, Android system UI changes বা Apple toolchain changes এলে তা scheduled compatibility audit-এ পড়বে। Platform adapter-এর maintenance burden shared UI core থেকে আলাদা করে পরিমাপ করো।

### 40.3 Community contribution

Component contribution guide-এ API proposal, semantics, platform mapping, screenshots, accessibility evidence এবং tests চাইবে। Contributor যদি শুধু HTML preview implement করে, component-কে stable ঘোষণা করা হবে না। First-party components-এর জন্য common test fixture বানাও যাতে external contributors-ও adapter parity test যোগ করতে পারেন।

### 40.4 Success statement

ভবিষ্যতে বলা যাবে Alap UI তিন প্ল্যাটফর্মে dependable, যখন: same source project তিন target-এ build হয়; core interaction semantics shared conformance suite pass করে; native text input ও navigation platform-compatible থাকে; accessibility checks বাস্তব device-এ pass করে; UI lifecycle leaks/repeated events নেই; layout ও scroll behaviour documented range-এ থাকে; release matrix এবং known limitations public থাকে। এর আগে status হবে pilot/preview, production-ready নয়।

---

## 41. Component-by-component implementation specification

এই অধ্যায়টি component catalog-এর কাজের চুক্তি। প্রতিটি component-এর জন্য common design rules এবং তার নিজস্ব edge cases দেওয়া হয়েছে। Component নামের spelling প্রস্তাবিত; schema/API freeze-এর আগে existing implementation-এর সঙ্গে মিলিয়ে নিতে হবে। তালিকায় component থাকা মানে এটি এখনই implemented বা stable—এমন নয়। MVP-তে আগে ছোট core set implement করবে; বাকিগুলো priority অনুযায়ী phase-এ যাবে।

### 41.1. `Page`

**উদ্দেশ্য ও API:** একটি route বা application screen-এর root container। Proposed properties: `title, body/child, optional topBar/bottomBar, safeAreaPolicy, background`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** route mount/unmount, system insets, scroll ownership। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** Page-এর ভিতরে status/navigation bars-এর fixed pixel padding দিয়ে double inset তৈরি হতে পারে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** notch, gesture navigation, landscape, route restore, large text। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.2. `Column`

**উদ্দেশ্য ও API:** children-কে vertical flow-তে সাজায়। Proposed properties: `children, spacing, crossAxisAlignment, mainAxisAlignment, mainAxisSize`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** child measurement, wrapping, fill/wrap constraints। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** unbounded height-এর মধ্যে Fill child কী করবে তা নির্ধারিত না থাকলে layout loop বা overflow হতে পারে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** long text, zero children, one oversized child, nested scroll। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.3. `Row`

**উদ্দেশ্য ও API:** children-কে horizontal axis-এ সাজায়। Proposed properties: `children, spacing, alignment, wrap, mainAxisSize`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** text field shrink, weighted children, RTL order। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** সব child fixed width নিলে narrow device-এ overflow হয়। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** compact width, Bengali text, accessibility font scaling। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.4. `Stack`

**উদ্দেশ্য ও API:** একাধিক child-কে overlapping layer-এ রাখে। Proposed properties: `children, alignment, clip, zIndex policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** hit testing, paint order, semantic reading order। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** invisible overlay tap block করে রাখা বা accessibility order visual order থেকে আলাদা হওয়া। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** overlapping actions, hit test, clipped child, hidden semantics। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.5. `Text`

**উদ্দেশ্য ও API:** শুধু text content দেখায়। Proposed properties: `text, style, maxLines, overflow, selectable, semantics label`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** font fallback, line wrapping, locale, dynamic type। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** fixed height দিয়ে text clipping বা grapheme cluster ভেঙে truncation। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** Bengali conjuncts, Latin mix, large type, empty text। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.6. `RichText`

**উদ্দেশ্য ও API:** একটি text block-এ বহু style/semantic span। Proposed properties: `spans, baseStyle, maxLines, selection policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** span offset mapping, link tap, bidi order। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** visual span boundary-কে Unicode character boundary ধরে নেওয়া। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** mixed-script, nested styles, screen reader reading order। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.7. `Button`

**উদ্দেশ্য ও API:** একটি named action trigger। Proposed properties: `label, enabled, variant, size, onActivate, accessibility label`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** activation once, pressed/focus feedback, keyboard activation। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** শুধু grey colour দিয়ে disabled করা কিন্তু event dispatch আটকানো নয়। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** tap, keyboard, screen reader, double activation, disabled। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.8. `IconButton`

**উদ্দেশ্য ও API:** icon-centric action with accessible name। Proposed properties: `icon, label, enabled, tooltip, onActivate`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** touch target, icon alignment, semantic name। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** icon visual আছে কিন্তু accessible label নেই। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** minimum target size, label announcement, RTL mirrored icon। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.9. `TextField`

**উদ্দেশ্য ও API:** editable single/multi-line text input। Proposed properties: `value, label, placeholder, inputType, enabled, onChange, onSubmit`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** IME composition, cursor/selection, focus, validation, keyboard action। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** প্রতিটি keystroke-এ native field recreate করলে cursor jump হয়। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** Bengali input, paste, composition, large font, keyboard shown। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.10. `SearchField`

**উদ্দেশ্য ও API:** query input with clear/search affordances। Proposed properties: `value, hint, onChange, onSubmit, clearable`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** debounce at app layer, stale results cancellation, clear semantics। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** query বদলালেও পুরোনো asynchronous result নতুন query-কে overwrite করে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** fast typing, clear, empty query, screen reader label। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.11. `PasswordField`

**উদ্দেশ্য ও API:** sensitive text input। Proposed properties: `value, label, obscureText, revealPolicy, autocomplete, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** secure entry, logging redaction, autofill, selection। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** password snapshots/logs-এ leaked হওয়া অথবা reveal toggle unlabeled। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** masking, reveal, autofill, keyboard, diagnostic redaction। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.12. `Checkbox`

**উদ্দেশ্য ও API:** স্বতন্ত্র boolean choice। Proposed properties: `checked, label, enabled, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** checked state, tri-state optionality, group labelling। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** toggle state visual পরিবর্তন হলেও semantic checked state update না হওয়া। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** label tap, disabled, keyboard, selected announcement। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.13. `CheckboxGroup`

**উদ্দেশ্য ও API:** related multiple choices। Proposed properties: `items, selected values, onChange, selectAll policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** stable option keys, partial selection state। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** index-ভিত্তিক state list order change-এ ভুল option select করে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** reorder, select all, partial state, large text labels। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.14. `RadioGroup`

**উদ্দেশ্য ও API:** পরস্পর-বর্জিত choice set। Proposed properties: `options, selected value, label, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** arrow-key movement, one selected item, grouping। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** multiple options simultaneously selected থাকা। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** keyboard arrow, selected state, empty/unknown value। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.15. `Switch`

**উদ্দেশ্য ও API:** boolean preference toggle। Proposed properties: `value, label, enabled, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** native toggle semantics, immediate vs save-later preference। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** switch tap আর label tap inconsistent behaviour। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** screen reader role/value, disabled, theme change। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.16. `Slider`

**উদ্দেশ্য ও API:** একটি numeric range থেকে value বেছে নেয়। Proposed properties: `value, min, max, step, label, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** clamping, step rounding, keyboard increment। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** min/max invalid বা non-divisible range-এ inconsistent step। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** boundary values, large font, accessibility adjustable action। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.17. `Stepper`

**উদ্দেশ্য ও API:** numeric value-এর discrete increment/decrement। Proposed properties: `value, min, max, step, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** disabled edge buttons, repeat/hold policy। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** minimum/maximum-এ visual enablement এবং action logic mismatch। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** min/max, rapid tap, keyboard, layout narrow width। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.18. `ProgressIndicator`

**উদ্দেশ্য ও API:** indeterminate বা determinate progress। Proposed properties: `value optional, label, kind, visible`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** progress announcements frequency, completion state। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** unknown progress-কে fake percentage দেখানো। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** 0/100%, indeterminate, screen reader, reduced motion। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.19. `ActivityIndicator`

**উদ্দেশ্য ও API:** স্বল্প বা দীর্ঘ operation চলার ইঙ্গিত। Proposed properties: `active, label, size`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** start/stop lifecycle, motion preferences। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** কারণ বা status ছাড়া অনির্দিষ্ট spinner-এ user আটকে থাকা। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** reduced motion, mount/unmount, accessible status। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.20. `Image`

**উদ্দেশ্য ও API:** local বা remote image দেখানো। Proposed properties: `source, fit, width/height or constraints, alt, placeholder`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** decode lifecycle, cache, cancellation, aspect ratio। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** recycled list row-তে আগের URL-এর ছবি নতুন item-এ দেখা যায়। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** missing asset, huge image, slow network, alt label। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.21. `Icon`

**উদ্দেশ্য ও API:** semantic icon resource display। Proposed properties: `name/resource, size, tint, decorative`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** RTL mirroring, accessibility inclusion/exclusion। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** Unicode emoji-কে consistent icon বা accessible label হিসেবে ধরা। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** missing icon, tint contrast, decorative semantics। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.22. `Divider`

**উদ্দেশ্য ও API:** visual separation। Proposed properties: `orientation, thickness, color, inset`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** pixel alignment, semantic decorativeness। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** divider-এর line thickness display density-তে inconsistent। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** vertical/horizontal, dark theme, screen reader exclusion। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.23. `Spacer`

**উদ্দেশ্য ও API:** layout-এ flexible বা fixed empty space। Proposed properties: `size or flex`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** constraint participation, accessibility exclusion। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** spacer অপ্রত্যাশিতভাবে সব available width/height খেয়ে ফেলে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** zero, fixed, flex, unbounded axis। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.24. `Card`

**উদ্দেশ্য ও API:** related content-এর visual grouping। Proposed properties: `children, padding, surface role, border/radius, elevation`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** no implicit button semantics unless interactive। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** পুরো Card-কে button ঘোষণা করে internal controls-এর activation conflict। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** nested Button, contrast, clipping, dynamic content। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.25. `List`

**উদ্দেশ্য ও API:** short/static sequence of children। Proposed properties: `children or items, item spacing`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** ordering, key identity if data-driven। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** large data set-এ eager mounting-এর কারণে scrolling/memory degrade। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** empty list, reorder, large text, item semantics। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.26. `LazyColumn`

**উদ্দেশ্য ও API:** virtualized vertical data list। Proposed properties: `items, key, itemBuilder, controller, padding`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** visible item range, recycling, scroll anchoring। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** stable key না থাকায় wrong row state/focus reuse। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** 10k items, insert/remove, image loading, scroll restore। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.27. `LazyRow`

**উদ্দেশ্য ও API:** virtualized horizontal data carousel/list। Proposed properties: `items, key, itemBuilder, controller`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** horizontal gesture conflict, focus traversal। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** nested vertical/horizontal gesture recognizers একে অপরকে block করে। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** nested scroll, keyboard focus, item recycling। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.28. `Grid`

**উদ্দেশ্য ও API:** regular rows/columns of children। Proposed properties: `items, key, column policy, gaps, itemBuilder`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** adaptive columns, visual/accessibility ordering। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** orientation change-এ item identity বা scroll anchor হারানো। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** narrow/wide windows, reorder, long labels। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.29. `ScrollView`

**উদ্দেশ্য ও API:** generic scrollable child container। Proposed properties: `child, axis, controller, physics/behavior`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** nested scroll policy, restoration, focus visibility। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** nested ScrollView-তে unbounded layout বা gesture competition। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** keyboard movement, huge content, restoration। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.30. `RefreshContainer`

**উদ্দেশ্য ও API:** explicit pull-to-refresh interaction। Proposed properties: `child, onRefresh, refreshing state`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** gesture threshold, cancellation, retry semantics। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** refresh requests duplicate firing on gesture/recompose। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** quick repeat, error, empty list, nested scroll। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.31. `PageView`

**উদ্দেশ্য ও API:** paged horizontal/vertical content। Proposed properties: `pages, initialPage, controller, onPageChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** swipe vs nested scroll, semantic page indication। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** programmatic page and indicator disagree। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** rotation, accessibility next/previous, page count। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.32. `TabBar`

**উদ্দেশ্য ও API:** top-level or local destination tabs। Proposed properties: `tabs, selected, onSelect, placement`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** keyboard traversal, selected semantics, state retention। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** tabs overflow at large font and no scroll/adaptive fallback। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** many tabs, large labels, nested stack, RTL। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.33. `NavigationStack`

**উদ্দেশ্য ও API:** typed route stack and back navigation। Proposed properties: `initial route, route registry, content builder, back policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** push/pop, state restoration, deep link, gestures। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** back gesture bypasses modal/unsaved-data policy। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** nested stack, root back, process restore, unknown route। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.34. `AppBar`

**উদ্দেশ্য ও API:** screen title and optional actions। Proposed properties: `title, leading, actions, scroll behavior`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** safe inset, title truncation, focus order। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** status bar padding applied twice or action labels clipped। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** long Bengali title, notch, large text, actions overflow। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.35. `BottomNavigation`

**উদ্দেশ্য ও API:** primary destinations on compact layouts। Proposed properties: `items, selected, onSelect, label policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** safe bottom inset, selected state, route stack retention। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** bottom actions collide with gesture region। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** large text, many items, route restoration। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.36. `Dialog`

**উদ্দেশ্য ও API:** modal decision or short interaction। Proposed properties: `title, content, actions, dismissible`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** focus trap, initial focus, back, return focus। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** background remains screen-reader accessible while modal is active। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** screen reader focus, back, outside tap, small screen। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.37. `BottomSheet`

**উদ্দেশ্য ও API:** modal or persistent sheet। Proposed properties: `content, modality, dismissible, detents`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** drag gestures, keyboard insets, focus restore। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** drag dismissal races with scrolling or keyboard। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** small screen, expanded content, back, nested scroll। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.38. `Snackbar`

**উদ্দেশ্য ও API:** temporary status with optional action। Proposed properties: `message, duration, action, announce`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** queue policy, action lifetime, interruption। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** critical information disappears too quickly or action callback is stale। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** rapid queue, screen reader announcement, action tap। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.39. `Tooltip`

**উদ্দেশ্য ও API:** supplementary help for a control। Proposed properties: `text, trigger, position, delay`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** keyboard/accessibility activation, hover/touch differences। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** essential information exists only on hover। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** long press, focus, small screen positioning। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.40. `DatePicker`

**উদ্দেশ্য ও API:** date selection using platform-appropriate UI। Proposed properties: `value, min/max, locale, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** timezone-free date semantics, system/native picker integration। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** date-only value accidentally shifts due to UTC/local time conversion। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** boundary dates, locale, cancellation, accessibility। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.41. `TimePicker`

**উদ্দেশ্য ও API:** time-of-day selection। Proposed properties: `value, 12/24h preference, onChange`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** locale conventions, cancellation, native picker mapping। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** time of day treated as timestamp with unintended timezone shift। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** 12/24h, locale, boundary values, cancel। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.42. `Modal`

**উদ্দেশ্য ও API:** generic presentation layer with explicit modality। Proposed properties: `content, barrier, dismiss policy, accessibility label`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** focus containment, z-order, background interaction। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** modal visually hidden behind another overlay but still active। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** multiple overlays, back, dispose, focus restoration। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.43. `Tooltip/Popover`

**উদ্দেশ্য ও API:** anchored transient content। Proposed properties: `anchor key, content, placement, trigger`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** screen edge collision, keyboard focus, dismissal। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** popover clipped by scroll parent and lost offscreen। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** edge placement, rotation, tap outside, keyboard। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.44. `Canvas`

**উদ্দেশ্য ও API:** custom drawing for charts/diagrams/visuals। Proposed properties: `size, draw callback or scene model, semantics`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** invalidation, transforms, hit testing, accessibility alternatives। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** custom text drawn without semantic representation। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** context loss, large scene, hit-test transform, fallback। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.45. `Chart`

**উদ্দেশ্য ও API:** data visualization with accessible alternatives। Proposed properties: `series, axes, labels, selected point, semantics summary`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** scaling, data updates, selection, reduced motion। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** essential values available only by colour/visual shape। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** empty series, large data, contrast, table alternative। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.46. `WebView`

**উদ্দেশ্য ও API:** explicitly embedded web content boundary। Proposed properties: `url or controlled content, navigation policy, accessibility`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** origin policy, keyboard, back, link handling। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** arbitrary web content granted unexpected native capability। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** untrusted URL, focus, back, load failure। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.47. `SemanticGroup`

**উদ্দেশ্য ও API:** group related nodes for accessibility announcement। Proposed properties: `label, hint, role, children, merge policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** child action preservation, focus grouping। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** group semantics hides actionable child controls। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** screen reader order, nested buttons, group labels। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.48. `FocusScope`

**উদ্দেশ্য ও API:** controls keyboard/accessibility focus traversal scope। Proposed properties: `children, initialFocus, traversal policy`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** enter/exit, dialog lifecycle, restoration। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** focus remains on disposed node or escapes modal unexpectedly। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** tab traversal, dialog open/close, route change। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.49. `SafeArea`

**উদ্দেশ্য ও API:** applies system safe insets exactly once। Proposed properties: `child, inset types, edges`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** nested inset consumption, edge-to-edge backgrounds। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** safe area consumed twice causing excessive whitespace। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** notch, keyboard, bottom gesture, landscape। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

### 41.50. `AnimatedContainer`

**উদ্দেশ্য ও API:** animates supported visual/layout properties। Proposed properties: `child, target properties, duration, curve`। প্রতিটি property-র type, default ও platform availability schema-তে থাকবে; unknown property স্পষ্ট diagnostic দেবে। নামের মিল থাকলেই component-গুলো একই behaviour পাবে—এমন অনুমান করা যাবে না।

**Behaviour contract:** interruption, reduced motion, layout invalidation। Child policy, size/overflow rules, event payload/timing, stable identity এবং dispose-time cleanup নির্দিষ্ট করো। Transient native state রাখা যায়, কিন্তু application value explicit binding/event API দিয়ে sync হবে।

**ঝুঁকি ও semantics:** animation callbacks fire on dead node or expensive full-tree relayout। Role, accessible name, value/state, focusability ও action নির্ধারণ করতে হবে। Accessibility-sensitive control-এ native mapping আগে মূল্যায়ন করো; custom rendering হলে keyboard/screen-reader gaps পরীক্ষা করো। Unsupported platform behaviour silent no-op নয়—diagnostic বা documented fallback হবে।

**Tests ও completion:** rapid target change, dispose, reduced motion। Default state, malformed properties, large text, theme, disposal এবং target adapters-এর conformance cases দরকার। Runnable example, API reference, platform status ও known limitations ছাড়া component `stable` হবে না।

## 42. UI component test matrix

নিচের matrix component catalogue-কে actionable acceptance test-এ রূপান্তর করবে। সব test প্রথম দিনেই সব platform-এ automation করা সম্ভব নাও হতে পারে; কিন্তু test case-গুলি specification-এ থাকবে এবং platform adapter চালু হওয়ার সঙ্গে সঙ্গে executable conformance suite-এ যোগ হবে। কোনো entry “manual” হলে তা চিরকাল অনির্দিষ্ট manual step থাকবে না—প্রয়োজনে issue তৈরি করে automation feasibility পর্যালোচনা হবে।

### 42.1 Shared mandatory cases

| Case | Setup | Action | Expected result | Priority |
|---|---|---|---|---|
| Mount/render | minimal component tree | launch | expected semantic tree and visible controls | P0 |
| State update | button + observed state | activate once | exactly one expected update/event | P0 |
| Disabled control | enabled=false | touch/keyboard/screen-reader activate | no action dispatched; disabled semantics | P0 |
| Invalid schema | unknown prop/wrong value type | compile/check | source-located diagnostic; deterministic failure | P0 |
| Large font | OS font scale increased | open main screen | no essential text/action lost | P0 |
| Insets | notch + bottom gesture area | launch and keyboard show | interactive controls not obscured | P0 |
| Lifecycle | open route, background, resume | return to app | no duplicate listeners/events | P0 |
| Disposal | mount then remove subtree | deliver stale async callback | no disposed state mutation/crash | P0 |
| Long list | 10,000 rows | scroll rapidly/reorder | bounded materialized items, stable row state | P1 |
| Theme change | light theme | switch to dark/system | no app-state reset; readable contrast | P1 |
| Locale/Bengali | Bengali + Latin + digits | input, wrap, select text | no broken clusters/cursor jumps | P0 for text controls |
| Reduced motion | enable OS preference | show transitions | essential state changes remain; motion reduced | P1 |
| Route restoration | nested routes and edited form | recreate process/scene per policy | expected route and safe state restore | P1 |
| Resource failure | missing/invalid image | render | fallback + useful diagnostic; no crash | P1 |
| Adapter unsupported | use unsupported optional property | build/run | explicit error/fallback message | P1 |

### 42.2 Per-platform rows

**Android:** test API-level matrix, gesture navigation, edge-to-edge, keyboard and IME composition, TalkBack, hardware/software keyboard, configuration change, activity/process recreation and app background/foreground. Insets must come from system information, not hardcoded status-bar height. UI tests will assert semantic nodes as well as images. Device vendors may differ in fonts and system picker appearance; those variations should be documented rather than causing brittle pixel-only tests.

**iOS:** test dynamic type, VoiceOver, safe area, keyboard/input accessory behaviour, swipe-back, modal focus, scene reactivation, app switcher privacy for sensitive screens, native picker and supported OS version. Test on a real device or official runtime environment when hardware-specific behaviour matters. Development build success alone is not acceptance evidence for text input and accessibility.

**HarmonyOS:** test supported API versions, ArkUI component mapping, keyboard/focus, navigation/back, accessibility service, app lifecycle and native picker/dialog. Emulator result and actual device result may differ; where a device-specific capability cannot be tested, feature status remains `not-tested` with a blocker/limitation note. Classic compatibility targets and HarmonyOS NEXT targets will be reported separately where relevant.

### 42.3 Visual test strategy

A screenshot baseline should have a manifest: platform, OS/API, device class, logical viewport, pixel density, text scale, locale, theme, font version and renderer revision. Compare screenshots after normalizing only known non-product regions, such as status-bar clock, not content areas. For cross-platform parity, compare semantic layout and interaction contracts first; separate visual baselines can accept legitimate native appearances. Review tool should show reference, current output, difference image and changed node tree together.

### 42.4 Flakiness policy

Animations, asynchronous image loads, system keyboard transitions and network responses can make UI tests flaky. Provide a deterministic test clock, fake asset loader, mock platform environment and controlled animations. Avoid arbitrary `sleep(2)` where state/idle synchronization can be used. If a test is quarantined, create an issue and owner; indefinite quarantine is a hidden regression. Snapshot update must be reviewed with explanation rather than blindly regenerated after every fail.

---

## 43. প্রস্তাবিত syntax cookbook

এই cookbook design sketch; code samples final parser acceptance-এর দাবি নয়। এগুলো API goals বোঝায়। Implement করার আগে existing parser/types/closures-এর সঙ্গে মিলিয়ে নিতে হবে, তারপর examples-কে executable tests-এ রূপান্তর করতে হবে।

### 43.1 Simple screen

```nil
component Welcome {
    render {
        Page(title: "নমস্কার") {
            Column(spacing: 16) {
                Text("নীলাং দিয়ে তৈরি UI")
                Button(label: "শুরু করুন", onActivate: fn() {
                    emit("startRequested", null);
                })
            }
        }
    }
}
```

এখানে title, Text এবং Button semantics component schema থেকে আসে। `emit`-এর payload type এবং event name-কে stable contract করতে হবে। Event handler render function-এর ভিতরে execution time-এ call হয় না; event activate হলে handler dispatch হয়। Example compile/render/run করতে না পারলে documentation-এ “conceptual sketch” label থাকবে।

### 43.2 Form with validation

```nil
component SignInForm {
    state email: String = "";
    state password: String = "";

    render {
        Page(title: "সাইন ইন") {
            Column(spacing: 12) {
                TextField(
                    label: "ইমেল",
                    value: email,
                    inputType: "email",
                    onChange: fn(next) { email = next; }
                )
                PasswordField(
                    label: "পাসওয়ার্ড",
                    value: password,
                    onChange: fn(next) { password = next; }
                )
                Button(label: "চালিয়ে যান", onActivate: fn() {
                    emit("submitRequested", {"email": email});
                })
            }
        }
    }
}
```

এটি illustrative; final API-তে strings-এর বদলে typed enum যেমন `InputType.email` এবং explicit `FormState` থাকতেই পারে। Password event payload-এ অকারণে পাঠানো যাবে না। Submit action validation error, loading state, duplicate submit এবং focus movement ঠিকভাবে সামলাবে।

### 43.3 Keyed list

```nil
component ProductList {
    state query: String = "";

    render {
        Page(title: "পণ্য") {
            Column {
                SearchField(value: query, onChange: fn(next) {
                    query = next;
                })
                LazyColumn(
                    items: products,
                    key: fn(item) { item.id },
                    itemBuilder: fn(item) {
                        ListTile(
                            title: item.name,
                            subtitle: item.priceText,
                            onActivate: fn() { emit("openProduct", item.id); }
                        )
                    }
                )
            }
        }
    }
}
```

List data source ও UI component state-এর ownership আলাদা রাখতে হবে। `products` example-এ external application model হিসেবে ধরে নেওয়া হয়েছে। Search results derived state হবে; request async হলে stale response handling data layer-এ থাকবে। Key stability, scroll restoration এবং item press accessibility tests বাধ্যতামূলক।

### 43.4 Theme এবং safe area

```nil
component SettingsScreen {
    render {
        Page(title: "সেটিংস", safeArea: "system") {
            Column(spacing: theme.spacing.medium) {
                Text("চেহারা", style: theme.typography.title)
                Switch(label: "গাঢ় থিম", value: darkMode, onChange: fn(next) {
                    setThemeMode(if (next) { "dark" } else { "light" });
                })
            }
        }
    }
}
```

এখানে `theme.spacing.medium` এবং `safeArea` syntax design goal বোঝায়। Public API-তে user’s system appearance, high contrast ও reduced motion preference precedence স্পষ্ট হবে। Runtime theme change current route/form state reset করবে না।

### 43.5 Dialog and route change

```nil
component DeleteConfirmation {
    render {
        Dialog(title: "মুছে ফেলবেন?") {
            Text("এই কাজটি ফিরিয়ে আনা যাবে না।")
            Row(spacing: 8) {
                Button(label: "বাতিল", onActivate: fn() { emit("cancel", null); })
                Button(label: "মুছে ফেলুন", variant: "danger", onActivate: fn() {
                    emit("confirmDelete", null);
                })
            }
        }
    }
}
```

Dialog API-তে focus containment ও return-focus semantics component code-এর বাইরে native adapter-ও সামলাবে। Destructive action-এর accessible label, confirmation copy এবং disabled/loading state থাকতে হবে। Platform native modal ব্যবহার করলে visual appearance আলাদা হতে পারে, কিন্তু cancel/confirm semantics shared হবে।

### 43.6 Syntax stabilization checklist

- Example parser ও type checker দিয়ে চলে কি না verify করা।
- All public properties schema-তে আছে কি না verify করা।
- Handler return type ও payload contract স্পষ্ট কি না।
- Render pure এবং side-effect free কি না।
- Stable identity/key কোথায় লাগে তা document করা।
- Unknown target capability compile-time-এ detect করা।
- Formatter, syntax highlight, LSP completion এবং docs একই API metadata ব্যবহার করছে কি না।
- Migration ও backward-compatibility test যোগ করা।

---

## 44. প্রথম ৩০/৬০/৯০ দিনের কাজ

এখানে calendar commitment নয়, executable milestones দেওয়া হচ্ছে। Solo maintainer হলে প্রতিটি milestone-এর সময় বেশি লাগতে পারে; phase order বদলানোর আগে dependency এবং evidence ভাববে।

### প্রথম ৩০ দিনের লক্ষ্য: UI foundation prove করা

**কাজ ১:** `pkg/alap` inventory। Existing types, component handlers, render output, preview renderer ও tests বুঝে লিখে রাখো। Existing behaviour-এর smoke tests যোগ করো।

**কাজ ২:** Core component schema। `Page`, `Column`, `Row`, `Text`, `Button`, `TextField`, `Spacer`, `Divider`, `Card`, `ScrollView`-এর properties/children/defaults/accessibility roles লিখে schema validate করো।

**কাজ ৩:** Type-checked UI IR। Raw map input থাকলে compatibility adapter-এর মাধ্যমে typed tree-তে পরিণত করো। Unknown property ও wrong type diagnostic দেখাও। Source span এবং stable keys সংরক্ষণ করো।

**কাজ ৪:** Reactive contract। Counter example-এ exactly-once event, state update, invalid payload, state disposal এবং repeated activation test করো।

**কাজ ৫:** Browser preview baseline। একই UI IR থেকে preview render করো। Preview-specific approximation ও native support status দেখাও।

**দৃশ্যমান ফল:** একটি counter এবং একটি sign-in-form UI schema/type-check, preview, events ও unit tests-সহ চলে। এটি এখনও “cross-platform native stable” নয়।

### ৬০ দিনের লক্ষ্য: Android native proof

**কাজ ৬:** Minimal Android host ও adapter। ৮–১২ core components native mappings পাবে।

**কাজ ৭:** Text input engineering। IME composition, cursor preservation, keyboard action, validation, safe insets ও lifecycle test করো।

**কাজ ৮:** Semantics/accessibility. Label/role/state/actions Android accessibility node-এ map করো; TalkBack দিয়ে screen flows যাচাই করো।

**কাজ ৯:** Layout and navigation. Constraint model, screen resize, back handling, modal focus এবং nested scrolling test করো।

**কাজ ১০:** Sample APK/build workflow-এর evidence record করো। SDK requirement, device/OS version, reproduction command, known limitations এবং smoke-test log সংরক্ষণ করো।

**দৃশ্যমান ফল:** বাস্তব বা supported test device-এ counter, sign-in form, searchable list এবং dialog app run হয়; accessibility ও input test-সহ Android adapter-এর স্পষ্ট status আছে।

### ৯০ দিনের লক্ষ্য: API stability ও দ্বিতীয় adapter-এর প্রস্তুতি

**কাজ ১১:** UI API public/stable candidate review। Types, layout, state, events, component schema ও docs-এর breaking-risk audit করো।

**কাজ ১২:** Themes/Bengali text/accessibility matrix। Bengali input, large font, dark mode, reduced motion এবং semantics test করো।

**কাজ ১৩:** iOS adapter spike। One-screen host-এ Button/Text/TextField/NavigationStack mapping-এর technical risk validate করো; native text input ও VoiceOver critical gate।

**কাজ ১৪:** HarmonyOS adapter feasibility spike। ArkUI/ArkTS host, component binding, focus/keyboard এবং accessibility integration-এর smallest proof-of-concept তৈরি করো।

**কাজ ১৫:** Prioritize backlog। Native integration যদি unresolved থাকে, আরও ২০টি widget নয়—প্রথমে সেই blocker solve করো। iOS ও Harmony support status honest preview হিসেবে প্রকাশ করো, production claim নয়।

**দৃশ্যমান ফল:** Alap-এর UI architecture ও API-এর ভিত্তি স্থিতিশীল, Android-এ প্রমাণিত; অন্য platform-গুলির technical feasibility ও exact missing pieces নথিভুক্ত।

### 44.1 কাজের দৈনিক নিয়ম

প্রতিদিন code commit-এর আগে relevant unit tests; সপ্তাহে একবার `docs/ui/STATUS.md` update; নতুন public property যোগ করার আগে schema/docs/test; platform-specific bug fix হলে conformance matrix update; unsupported feature-এর জন্য issue create; আর কোন feature actual device-এ test করা হয়েছে তার evidence সংরক্ষণ। একসঙ্গে তিন backend-এর unfinished work জমিয়ে রাখবে না। Working vertical slice সবসময় অগ্রাধিকার পাবে।

---

## 45. Release checklist

### API and semantics

- [ ] Public component schemas versioned এবং documented।
- [ ] Unknown properties ও invalid children deterministic diagnostics দেয়।
- [ ] State/event timing, payload typing, cancellation ও disposal policy test করা।
- [ ] Stable keys এবং duplicate-key diagnostics আছে।
- [ ] Render side effects-এর policy স্পষ্ট এবং development checks আছে।

### Layout and responsive behaviour

- [ ] Logical units এবং text scaling rules documented।
- [ ] Row/Column/Stack constraints tested।
- [ ] Overflow, clipping, wrapping ও ellipsis policies tested।
- [ ] Keyboard, notch, gesture area এবং landscape insets handled।
- [ ] Wide/compact layout এবং large font scenarios tested।

### Accessibility and text

- [ ] Essential controls have role/name/state/action semantics।
- [ ] Keyboard/focus order, dialog focus trap ও return focus tested।
- [ ] TalkBack/VoiceOver/available HarmonyOS service manual testing logged।
- [ ] Large text, contrast, reduced motion ও non-colour cues validated।
- [ ] Bengali conjuncts, IME composition, cursor/selection এবং mixed-script text tested।

### Platform adapter

- [ ] Supported OS/API versions প্রকাশিত।
- [ ] Build/install/run smoke test per platform।
- [ ] Native input, navigation, system insets ও lifecycle tests pass।
- [ ] Unsupported features `not-tested` লুকিয়ে `supported` হিসেবে mark হয়নি।
- [ ] Preview limitations ও platform-specific differences documented।

### Stability and release engineering

- [ ] Core unit/layout/integration tests CI-তে।
- [ ] Known P0/P1 bugs নেই বা প্রকাশিত blocker decision আছে।
- [ ] API migration notes ও version compatibility table আছে।
- [ ] Performance baseline device/build metadata-সহ আছে।
- [ ] Resource/font licenses reviewed।
- [ ] Sensitive content logs/snapshots-এ redact হয়।
- [ ] Runnable sample apps docs-এর সঙ্গে sync।

---

## 46. Official references ও পরিভাষা

এই roadmap-এর platform-specific recommendations-কে implementation-এর আগে current SDK/API version অনুযায়ী পুনরায় যাচাই করতে হবে। Official docs API behaviour-এর source; blog/tutorial supplemental হতে পারে, কিন্তু final conformance সিদ্ধান্ত official docs ও device tests-এর উপর নির্ভর করবে।

1. [Android Developers — About Window Insets in Jetpack Compose](https://developer.android.com/develop/ui/compose/system/insets): system bars, cutouts, keyboard এবং changing window environments থেকে content কীভাবে safe রাখা যায়।
2. [Android Developers — Set up edge-to-edge](https://developer.android.com/develop/ui/compose/system/setup-e2e): edge-to-edge window এবং inset handling-এর implementation details।
3. [Android Developers — Compose accessibility testing](https://developer.android.com/develop/ui/compose/accessibility/testing): automated semantic testing এবং TalkBack/Switch Access-এর manual verification।
4. [Apple Developer Documentation — SwiftUI Accessibility Fundamentals](https://developer.apple.com/documentation/swiftui/accessibility-fundamentals): common controls-এর accessibility এবং label/value semantics।
5. [Apple Developer Documentation — Accessible Navigation](https://developer.apple.com/documentation/swiftui/accessible-navigation): accessibility rotor ও navigation-related facilities।
6. [Huawei Developers — Getting Started with ArkTS](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/arkts-get-started-V5): HarmonyOS app development-এ ArkTS-এর typed/declarative role।
7. [Huawei Developers — ArkUI component overview / adding components](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V13/arkts-add-component-V13): ArkUI component development ও native component types।
8. [Huawei Developers — Accessibility Kit overview](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/accessibilitykit-overview-V5): accessibility service ও ArkUI component accessibility properties।
9. [Huawei Developers — ArkUI preview](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/ide-previewer-arkui-V5): ArkTS/ArkUI page ও component preview workflow।
10. [Huawei Developers — ArkUI shared element transition](https://developer.huawei.com/consumer/en/doc/harmonyos-guides-V5/arkts-shared-element-transition-V5): transition capabilities assessment-এর reference।

### শব্দকোষ

- **Adapter:** shared UI semantics-কে একটি platform-এর native component/system API-তে map করে।
- **Accessibility tree/semantics tree:** screen reader ও assistive technology-র কাছে UI meaning প্রকাশ করা structure।
- **Constraint:** parent থেকে child-কে দেওয়া allowable size bounds।
- **Declarative UI:** UI-কে বর্তমান state থেকে কী দেখা উচিত তার বর্ণনা হিসেবে প্রকাশ।
- **Diff/reconciliation:** আগের ও নতুন UI tree তুলনা করে প্রয়োজনীয় node/property update apply করা।
- **IME:** Input Method Editor; software keyboard/অন্যান্য text input method-এর composition process।
- **IR (Intermediate Representation):** source syntax ও platform backend-এর মাঝের typed canonical representation।
- **Logical unit:** device pixel নয়; density/scaling policy দিয়ে physical units-এ map হয়।
- **Native component:** target OS/framework-এর নিজস্ব UI control।
- **Stable key:** data item বা component instance-এর identity চিহ্নিত করে, positional index-এর উপর নির্ভর না করে।
- **Virtualization:** viewport-এর কাছাকাছি প্রয়োজনীয় list/grid items-ই materialize করা।
- **Conformance test:** shared UI contract একটি নির্দিষ্ট adapter-এ সঠিকভাবে পালিত হচ্ছে কি না যাচাই।

---

## পরিশিষ্ট A — UI design decisions যেগুলি লিখিত specification ছাড়া merge করা যাবে না

### A.1 State update কি synchronous হবে?

UI handlers developer-এর দৃষ্টিতে synchronous mutation করতে পারে, কিন্তু native painting সাধারণত frame scheduling-এর সঙ্গে batched হয়। Specification-এ লিখবে: state handler return করার পর new value observable কি না; event listeners কত দ্রুত notified হয়; render কখন schedule হয়; handler-এর মধ্যে nested state updates atomic কি না। UI thread-এ দীর্ঘ-running work নিষিদ্ধ করার rules থাকবে। Async state mutation event queue-তে thread-safe dispatch হবে; unsupported thread থেকে native object touch করা যাবে না।

### A.2 Native node reuse কবে হবে?

Stable key, component type, parent relation এবং adapter-specific compatibility-এর ভিত্তিতে node reuse হবে। `Button` একই key-তে `TextField` হয়ে গেলে native object forced reuse করবে না। Reuse হলে pressed/focus/editing state preserve করতে হবে কি না component semantics অনুযায়ী সিদ্ধান্ত নেবে। Unmount/mount callbacks ordering, resource release এবং listener cleanup tests থাকবে।

### A.3 Visual property update না layout invalidation?

Property metadata-তে `paint`, `layout`, `semantics`, `native-environment` invalidation classes থাকবে। Text change সাধারণত layout invalidation করতে পারে; button background শুধু paint/native style update; accessible label semantics invalidation; system font scale layout + text metrics invalidate করবে। Incorrect classification stale layout অথবা excessive rerender ঘটাবে। Debug mode-এ invalidation reason log করা যাবে।

### A.4 Error policy compile-time না runtime?

Component/property names, static types, required accessibility label policy ও target availability compile-time-এ যাচাই করা যায়। Network image failure, async validation এবং missing runtime resource runtime state। Compile-time errors release build block করতে পারে; preview development build warning/placeholder দেখাতে পারে। Production fallback কখনও critical UI quietly missing রাখবে না। Error severity ও exit status documented হবে।

### A.5 Default navigation pattern কী?

NavigationStack, tabs এবং modal presentation platform convention অনুযায়ী mapped হবে; shared route model app logic ধরে রাখবে। Root-level back behaviour, dirty form guard, nested navigation, deep link, route restoration এবং modal dismissal precedence লিখিত state machine দিয়ে define করতে হবে। Native gesturesকে artificial custom gesture দিয়ে replace করার আগে accessibility এবং user expectation impact মূল্যায়ন করবে।

### A.6 UI components কি server-driven হবে?

Remote configuration বা server-driven content আলাদা feature; সাধারণ component tree-কে JSON থেকে unrestricted UI execution করতে দিও না। Server-driven model দরকার হলে signed/schema-validated data, allowlisted components, strict resource limits, versioning, offline fallback এবং security review লাগবে। App's core interactions offline/host logic দিয়ে চলতে পারবে কি না design অনুসারে স্পষ্ট থাকবে।

### A.7 Native style বনাম shared style

প্রতিটি component-এর default presentation principle লিখে দাও। `Button` native look-কে default করবে নাকি design token দিয়ে custom style? UI library অন্য platform-এ consistent branded look চাইলে custom style opt-in হবে কি? Recommendation হলো: basic controls semantics native-first; design-kit components token-driven; advanced custom visuals explicit. একটি generic `appearance` parameter সব জটিলতা সমাধান করবে—এমন ধারণা এড়িয়ে চল।

---

## পরিশিষ্ট B — আরও বিস্তারিত quality gates

### B.1 Layout gate

একই test fixture বিভিন্ন viewport width, height, density ও text scale-এ run হবে। Layout result-এ bounding boxes, overflow flags এবং baseline metrics রেকর্ড থাকবে। Negative/zero spacing বা contradictory constraints input validation ছাড়া runtime crash করতে পারবে না। Parent-child measure cycle detect হলে diagnostic দেবে। Scrollable axis-এ unbounded constraints-এর expected behaviour tests-এ আছে কি না যাচাই করবে।

### B.2 Interaction gate

প্রতিটি interactive control-এর event matrix থাকবে: touch, keyboard, screen reader, programmatic activation, disabled state, rapid repeated activation, cancellation ও component disposal। Event handler call count assert করা হবে। Hover/focus/pressed state visual tokens এবং native accessibility state একই source semantics থেকে update হবে। Native callback থেকে shared runtime-এ event পাঠানোর thread boundary tested হবে।

### B.3 Resource gate

Missing image, corrupt image, invalid font, missing locale string, oversized resource, slow decode, cache eviction ও cancellation test থাকবে। Resource identifiers path traversal করতে পারবে না। Font/image load failure critical action block করবে না। Resource errors developer diagnostics-এ source node ও logical resource ID দেখাবে; raw private file paths exposure করবে না।

### B.4 Accessibility gate

All focusable nodes reachable; modal background inaccessible; labels not empty; state change announcement sane; target size acceptable; large type no critical clipping; contrast reviewed; reduced motion applies; focus restoration after dialog; scrolling reachable content. Automated checks-এর ফল manual test log-এর সঙ্গে জুড়ে রাখা হবে। Accessibility regressions code review checklist-এ বাধ্যতামূলক field।

### B.5 API governance gate

Stable component-এর new required property যোগ করা breaking change হতে পারে; default নিয়ে ভুল সিদ্ধান্ত app behaviour পরিবর্তন করতে পারে। Schema diff CI-তে public API changes detect করবে। Deprecated API uses warning দেবে; migration guide release-এর সঙ্গে থাকবে। Platform adapter property support বদলালে matrix update এবং tests required।

---

## পরিশিষ্ট C — UI roadmap-এর প্রস্তাবিত GitHub issue breakdown

এই backlog সরাসরি issue তৈরির আগে existing issues/branches-র সঙ্গে deduplicate করতে হবে। প্রতিটি issue ছোট, testable এবং এক owner-এ assignযোগ্য হবে।

1. `UI-001` — আলাপ UI existing behaviour inventory and status labels.
2. `UI-002` — current Counter event/state smoke test.
3. `UI-003` — typed schemas for Page/Text/Button/Column/Row.
4. `UI-004` — unknown-property diagnostics with source spans.
5. `UI-005` — typed UI IR and deterministic serialization for debug.
6. `UI-006` — layout logical unit type proposal.
7. `UI-007` — constraint solver/unit test harness.
8. `UI-008` — render purity and event ordering specification.
9. `UI-009` — state transaction and exactly-once event tests.
10. `UI-010` — stable key contract and duplicate-key diagnostics.
11. `UI-011` — component lifecycle/disposal cleanup tests.
12. `UI-012` — semantic tree schema for roles/labels/actions.
13. `UI-013` — Bengali Text/TextField test fixtures.
14. `UI-014` — theme token registry and default themes.
15. `UI-015` — keyboard/safe inset environment model.
16. `UI-016` — HTML preview adapter conformance disclaimer.
17. `UI-017` — Android native host technical spike.
18. `UI-018` — Android Button/Text mapping.
19. `UI-019` — Android TextField/IME mapping.
20. `UI-020` — Android TalkBack semantic tests.
21. `UI-021` — Android back/navigation and modal focus.
22. `UI-022` — real device smoke test automation notes.
23. `UI-023` — LazyColumn keys and virtualization design.
24. `UI-024` — image loader cancellation and cache policy.
25. `UI-025` — accessibility lint for icon-only controls.
26. `UI-026` — API support matrix generation.
27. `UI-027` — iOS native host spike and API risk report.
28. `UI-028` — iOS text input and VoiceOver proof.
29. `UI-029` — HarmonyOS ArkUI host spike and support report.
30. `UI-030` — HarmonyOS focus/keyboard/accessibility proof.
31. `UI-031` — responsive layout samples and screenshot manifests.
32. `UI-032` — UI inspector tree/property view prototype.
33. `UI-033` — stable API compatibility report generator.
34. `UI-034` — UI release gate checklist in CI.

### C.1 Issue template

```markdown
## Problem
What user/developer behaviour is missing or incorrect?

## Current evidence
Link source files, reproduction sample, device/OS and existing tests.

## Proposed contract
Expected semantics, API/property/event behaviour, defaults and errors.

## Platform impact
Android / iOS / HarmonyOS / HTML preview; supported, different, fallback, unsupported.

## Acceptance tests
Unit, layout, semantic/accessibility, adapter integration, real-device/manual evidence.

## Out of scope
Explicitly list features this issue does not solve.
```

---

## পরিশিষ্ট D — Final priority ranking

**P0 — UI foundations:** schema/type validation, typed IR, state/event semantics, core layout, Text/Button/TextField, safe insets, accessibility semantics, stable keys, lifecycle disposal, native Android vertical slice. এগুলো ছাড়া framework-এর বাকি কাজের উপর নির্ভর করা উচিত নয়।

**P1 — Core app construction:** forms/validation, navigation/back, dialog, LazyColumn, theme tokens, Bengali typography, responsive layouts, images/resources, automated UI tests, Android device evidence, iOS/HarmonyOS feasibility spikes।

**P2 — Production maturity:** API versioning, inspector, hot reload, platform screenshots, advanced focus/navigation, adaptive master-detail, performance profiling, accessibility audits, state restoration and resource caching.

**P3 — Advanced polish:** shared-element animation, complex custom drawing, chart library, advanced grid algorithms, server-driven UI, visual UI editor, cross-platform pixel matching, GPU-accelerated special effects. এগুলো core correctness-এর পরে।

### শেষ কথা

Alap UI-র সবচেয়ে মূল্যবান অংশ হবে না component-এর সংখ্যা; বরং একটি ছোট component tree তিনটি আলাদা OS-এ predictable state, correct layout, native input এবং accessible interaction সহ চলার প্রমাণ। প্রথমে semantics ও contracts লিখো, তারপর vertical slice implement করো, test evidence প্রকাশ করো, এবং প্রতিটি নতুন component-কে সেই contract-এর আওতায় রাখো। এই শৃঙ্খলাই নীলাংয়ের UI-কে demo থেকে usable framework-এ নিয়ে যেতে পারে।

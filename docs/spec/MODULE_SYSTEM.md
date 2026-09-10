# NilLang Module System Specification
**Version:** 1.0.0-draft  
**Status:** Draft Normative Specification
**Conformance Level:** Nilang 0.1 module subset

---

## 0. Implementation Boundary

Import parsing, evaluator module loading, cache-based single initialization,
and cycle rejection are implemented. Complete export/private visibility,
selective-import enforcement across every backend, package resolution through
`nil.lock`, and concurrent initialization are planned. Sections describing
those behaviors are target contracts until their module tests are enabled.

## 1. Overview & Principles

Modules in NilLang are discrete compilation units and lexical namespaces. The module system provides encapsulation, code reusability, dependency management, and deterministic initialization.

### Cardinal Rules:
1. **Explicit Exports**: Symbols defined inside a module are private to that module by default. Only identifiers explicitly preceded by `export` are accessible from external modules.
2. **No Ambient Globals**: Importing a module binds its exported symbols to a namespace or into the local lexical scope. Modules cannot inject symbols into the global scope.
3. **Strict Acyclic Dependency Graph**: Cycles in the import graph (`A -> B -> A`) are detected at compile time and rejected with diagnostic `E0301`.
4. **Deterministic Initialization Order**: Modules are initialized once in topological dependency order (post-order traversal).

---

## 2. Syntax & Grammar

```ebnf
ImportStatement ::= "import" StringLiteral ( "as" Identifier )? ";"
                  | "import" "{" ImportItem ( "," ImportItem )* "}" "from" StringLiteral ";"

ImportItem      ::= Identifier ( "as" Identifier )?

ExportStatement ::= "export" ( LetStatement | ConstStatement | FunctionDeclaration | StructDecl | EnumDecl )
```

### 2.1 Examples
```nil
// Import whole module with alias
import "std/math" as math;
let root = math.sqrt(16);

// Selective import with renaming
import { readFile as read, writeFile } from "std/fs";

// Exporting declarations
export const PI = 3.14159265359;
export fn add(a: Int, b: Int) -> Int {
    return a + b;
}
```

---

## 3. Path Resolution & Canonical Module Identity

1. **Standard Library (`std/*`)**: Resolves to the built-in or bundled Nilang standard library.
2. **Relative Imports (`./*`, `../*`)**: Resolves relative to the directory of the importing source file.
3. **Package Imports (`<pkg>/...`)**: Resolves through the project's dependency manifest (`nil.json` / `nil.lock`) stored in `packages/` or the global cache.

Modules are keyed by their absolute canonical filesystem path. A module is parsed, checked, and compiled at most once per build.

---

## 4. Circular Dependency Detection

The compiler constructs a directed module import graph $G = (V, E)$. Before type checking:
1. The compiler runs Tarjan's strongly connected components (SCC) or depth-first cycle detection.
2. If a cycle is detected, compilation halts immediately with diagnostic:
   ```text
   error[E0301]: circular dependency detected:
      --> app.nil imports services/auth.nil
      --> services/auth.nil imports services/user.nil
      --> services/user.nil imports app.nil
   ```

---

## 5. Initialization Order & Re-entrancy

1. Modules are initialized in topological order. Dependencies are guaranteed to have completed initialization before the dependent module's top-level code executes.
2. If module `A` imports `B` and `C`, and `B` imports `C`:
   `C` is initialized first, then `B`, then `A`.
3. Modules evaluate in a thread-safe singleton context; subsequent imports of the same module yield references to the already initialized namespace.
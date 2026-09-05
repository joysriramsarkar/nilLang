# Nil Ecosystem Architectural Blueprint
**Scope:** Nil Ecosystem Architecture (NilLang, Alap Framework, Onuron OS)  
**Version:** 1.0.0  

---

## 1. System Division & Hierarchy

The Nil computing ecosystem is strictly divided into three distinct, complementary layers:

```text
                            HUMAN / INTENT
                                  │
                                  ▼
                              AI AGENT
                                  │
                   ┌──────────────┴──────────────┐
                   ▼                             ▼
                NilLang                        Alap
            Language Layer               Application Layer
                   │                             │
                   └──────────────┬──────────────┘
                                  ▼
                          Compiler & Oracle
                                  │
                           Verification
                                  │
                                 MIR
                                  │
                   ┌──────────────┼──────────────┐
                   ▼              ▼              ▼
                 WASM           Native        Bytecode
               (Browser)       (Onuron)       (Stack VM)
```

### Layer Responsibilities
| Layer | Domain | Primary Purpose | Scope & Principles |
|---|---|---|---|
| **NilLang** | Programming Language | Express, type, compile, execute, verify | Strict, compact, stable core. Knows types, syntax, expressions, functions, memory, HIR/MIR, and bytecode. Does **not** include ad-hoc application abstractions. |
| **Alap** | Application Framework | Compose, connect, reuse, deploy | Rich, opinionated, dynamic application architecture (UI, routing, state, forms, database, server, networking, and SoftBus). |
| **Onuron** | Operating System / Platform | Run, manage, isolate, secure | Kernel, native device services, SoftBus peer network, system drivers, and encrypted keystore. |

---

## 2. NilLang Profiles vs Alap Modules

Instead of fragmenting into separate programming languages for Web, Mobile, and Server, **NilLang Core** remains unified, offering domain **Profiles**, while **Alap** provides application abstractions:

```text
               Nil Ecosystem
                     │
    ┌────────────────┴────────────────┐
    ▼                                 ▼
NilLang Profiles                Alap Modules
  • Core Profile                  • Alap UI (Components, Layout, Animation)
  • Web Profile (WASM, DOM)       • Alap Web (Routing, SSR, Browser APIs)
  • Mobile Profile (Android, FFI) • Alap Mobile (Navigation, Touch, Sensors)
  • Server Profile (TCP, HTTP)    • Alap Server (REST, DB, RPC, Auth)
  • Data Profile (Tensors, Math)  • Alap Data (Dataframes, Pipelines, ML)
  • OS Profile (SoftBus, IPC)     • Alap AI (Application Truth, Oracle Bridge)
```

---

## 3. Package Management & Application Packaging

### 3.1 Package Manager (`nilpkg`)
- Dependency resolution with semantic versioning.
- SHA-256 content-hash verification.
- HTTP REST package registry (`cmd/nilpkg-server`) with package publication and search endpoints.

### 3.2 Digital Signatures & Security (`nilkey`)
- Ed25519 asymmetric cryptographic signing for every package and executable bundle.
- Key derivation using Argon2id and local encrypted keystore via AES-256-GCM.
- Immutable provenance tracking of code origin.

### 3.3 Application Bundle (`.nilax`)
Production deployment artifact encapsulating:
```text
application.nilax
├── manifest.json       # App identity, version, targets, declared capabilities
├── bytecode/           # Compiled NABC bytecode (.nabc) or WebAssembly module (.wasm)
├── resources/          # Assets, images, stylesheets, fonts
└── signature.sig       # Ed25519 cryptographic signature
```

---

## 4. SoftBus Distributed Protocol

Onuron SoftBus enables peer-to-peer discovery and inter-device function calling without external cloud infrastructure:
- **LAN Discovery**: UDP multicast (`239.255.0.1:9001`) broadcasting discovery heartbeats every 2 seconds.
- **Framed TCP Transport**: Framed JSON-RPC 2.0 messages for remote execution, file streaming, and device state sync.
- **Daemon (`cmd/softbusd`)**: Background service coordinating local and peer nodes.

---

## 5. AI-Compiler Integration & Hallucination Resistance

The architecture treats Artificial Intelligence as a primary software creator and collaborator:
1. **Ground Truth Reflection**: The compiler exposes symbol tables, type structures, and operational capabilities via the **Language Oracle**, preventing AI engines from hallucinating non-existent APIs.
2. **State Categorization**: Every symbol and module belongs to a designated trust level:
   - `KNOWN`: Officially recognized system or package identifier.
   - `EXPERIMENTAL`: Novel AI-generated component pending formal certification.
   - `VERIFIED`: Proven safe and conformant through the 6-stage test and sandbox pipeline.
   - `STABLE`: Production-certified element locked for standard deployment.
3. **Verified Novelty Pipeline**: Unlocks rapid AI-driven innovation while ensuring zero security regressions and 100% type soundness.

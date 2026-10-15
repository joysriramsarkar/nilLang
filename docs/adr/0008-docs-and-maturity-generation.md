# ADR-0008: Documentation and Maturity Generation

## Status

Accepted

## Context

The ecosystem's maturity claims are written down in at least five places: each
repository's README, the Onuron Project website, the store's registry entries,
the docs site, and the audit reports that started this work.

They drift. A repository README says a feature is stable; the website says
experimental; the store entry says supported. Nothing in CI notices, because
prose is not checked against anything. The result is that "maturity" in this
ecosystem currently means "whichever document a reader found first".

That is the failure the ecosystem plan calls out directly: the ecosystem is only
integrated when the website shows the *true* support status of a version that the
other six layers actually agree on.

## Decision

**The owner repository holds the normative statement and the machine-readable
status. The website presents it. It never restates it.**

1. **One machine-readable support status per owner**, versioned with the code.
   For NilLang that is `contracts/compatibility/matrix.json`, generated into the
   checksummed contract bundle.
2. **Evidence or downgrade.** Every cell marked `stable` or `partial` names an
   evidence path — a test file, a CI run, a release tag or hardware output. A
   cell with no evidence is `experimental`. This rule is written into the
   contract file itself, so a table that loses its evidence fails its own
   contract.
3. **The website imports the versioned source.** The docs site links to the exact
   release the table came from. It does not restate the table in prose, because a
   prose copy is a second source of truth with no owner.
4. **No hand-edited status table anywhere else.** A maturity status appears in
   exactly one place per owner; everything else renders it. The same rule that
   forbids a duplicate compiler forbids a duplicate status list.
5. **Draft and experimental stay visible.** A feature marked partial or stub
   appears on the website marked partial or stub. Hiding it is not the same as
   fixing it, and a hidden limitation is a discovered one.
6. **Normative specs and user documentation are different documents.** The
   normative spec is the contract; the docs site teaches. The docs site links to
   the versioned normative source rather than paraphrasing it.

## Alternatives rejected

**Keep a single hand-written maturity table in the website.** Rejected: it is
exactly the arrangement that produced the disagreement, and it gives the
presentation layer ownership of a claim about the code.

**Auto-detect maturity from CI.** Tempting, and partially right — a passing test
is evidence. Rejected as the whole answer because "supported" is a claim about a
*subset*: a runtime can pass every test and still be missing a feature that no
test covers. Detection can supply evidence; a human still decides the claim, and
the decision is recorded where the code is.

**Generate the docs site entirely.** Rejected: user documentation needs prose.
What is generated is the status, the version and the evidence links.

## Migration plan

1. Publish the machine-readable support status in each owner repository.
   NilLang: `contracts/compatibility/matrix.json`. **Done.**
2. Enforce the evidence rule in the owner's tests.
   NilLang: `TestCompatibilityMatrixHasNoUnsupportedClaims`. **Done.**
3. Each consumer's validator reads the published table rather than its own list.
4. The website renders the table from the pinned source and stops restating it.
5. Registry entries gain a maturity field sourced from the owner's table.

## Evidence

- `contracts/compatibility/matrix.json` — the published status.
- `pkg/contract/contract_test.go`:
  - `TestCompatibilityMatrixHasNoUnsupportedClaims` fails any cell claiming
    `stable` with no evidence path.
  - `TestBundleFilesAreValidJSONWithAStatus` requires every contract file to
    declare its owner, version and status.
- `contracts/README.md` — the change policy that keeps status and evidence
  together.

## Reversal condition

If the statuses prove too fine-grained to maintain — every cell needs a review
that costs more than the matrix is worth — the granularity drops to backend
level rather than being abandoned. The two properties that must survive any
simplification are: the status lives with the code, and a `stable` claim names
its evidence.
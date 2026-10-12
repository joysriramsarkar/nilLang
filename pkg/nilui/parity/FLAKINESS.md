# Flakiness Policy (U14.9)

Golden and UI tests must be deterministic. Rules:

1. **No wall-clock asserts.** Timing is measured, never asserted, except the
   perf-budget CI test which uses p95 with 30 samples on a fixed workload.
2. **Fixed seeds.** Any randomness uses a fixed seed recorded in the test.
3. **Font-independent goldens.** Raster goldens assert pixel shapes from the
   5×7 bitmap font + geometry, never system font rendering.
4. **Tolerance-based pixel compare.** Exact bytes within one backend build;
   cross-backend compare allows 0 differing bytes today (same rasterizer) —
   if native compositors join later, allow ≤0.1% with a documented reason.
5. **No network, no sleep-then-hope.** The watcher test polls deterministically
   or waits on channels with timeouts that only fail (never pass spuriously).
6. **Quarantine.** A test failing >1% over 200 CI runs is quarantined to
   `TestQuarantined*` with an issue link, fixed within one release.

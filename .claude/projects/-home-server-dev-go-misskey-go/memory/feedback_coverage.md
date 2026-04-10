---
name: Coverage target policy
description: Test coverage goals - maintain 90%+ per package, aim for near 100%
type: feedback
---

All packages must maintain 90%+ test coverage (enforced by CI).
Target is near 100% — high coverage is critical for detecting regressions early.

**Why:** User wants tests that catch broken functionality. Coverage is not just a metric but a safety net for ongoing development. Always prioritize test quality.

**How to apply:**
- When adding new code, ALWAYS add corresponding tests in the same commit
- Aim for 95%+ per package, 100% where structurally possible
- CI threshold is set to 90% as a floor, not a goal
- Use misskey_test DB for DB-dependent tests (not mock-only)
- repository package has ~90-95% ceiling due to unreachable GORM error paths

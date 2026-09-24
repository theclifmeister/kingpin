# Engine alignment validation — 24 September 2026

Engine source: `4dafb95422cefe688200eccd293e8114d1124771` (main at retrieval).
Go 1.26.8, protocol 10, view 11. Compiled original engine without gameplay modifications.

Passed:
- Upstream `internal/engine` test suite.
- Upstream `internal/protocol` test suite, including transport tests. `GOFLAGS=-buildvcs=false` was needed because this environment's detached checkout cannot supply build-time VCS stamping.
- Actual WASM: 45-day fresh trading run, six dilemma choices; each day's dirty and clean cash flow reconciled opening plus categories to closing.
- Actual WASM: preview and preset queries left the public view unchanged.
- Imported pre-update Day 121 active, Day 261 Kingpin, and Day 236 retired saves; expected days/endings preserved and new report shape supplied.
- Browser: all seven sections on desktop and 390px mobile, no document horizontal overflow and no page errors.
- Browser: buy/sell, forecast without advancing, confirmed end-day, automatic save/reload.
- Browser: operation preset review/application and ambition pinning.
- Browser: imported Day121 save, captain dialog and successful appointment with zero bonus budget, dilemma consequence chips, day forecast, restock review/application.
- Frontend syntax and git whitespace checks.

The integration retains legacy report-history rendering. Save-load failures stop startup and leave the saved bytes intact. Illustrations and style are preserved.

Optional WebMCP: no supported context available; original feature-detected read/navigation integration retained. Not a publication blocker.

This is engine alignment and integration testing, not a balance certification or a claim that every engine command has a UI. See README for remaining UI boundaries.

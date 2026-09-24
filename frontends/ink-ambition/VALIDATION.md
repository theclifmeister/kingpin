# Engine alignment validation: protocol 15 / view 12 (#407)

Reviewed against `main` after #405. The builder now accepts protocol 15 and view 12.

Protocols 11 to 15 and view 12 added `set_export` and the lanes (`view.exports`), `buy_trophy`, `trophy_offers` and `view.trophies`, `cash_out` with `rules.laundering.cash_out_fee`, `go_straight` with `rules.laundering.can_go_straight`, and `forecast`. Each now has a control, and each is checked in two ways:

- **`smoke.mjs`** runs against the actual WASM build. It checks that view 12 carries the lanes and the trophies, that a lane order is set and then turned off with 0 units, that `trophy_offers` is a list, and that the forecast's fields add up (`pile = dirty + landings − wages`). It checks the cash-out moves clean to dirty less the quoted fee (or is refused with no clean cash), that the four endings each have a planning ambition, and that going straight before the streak is refused.
- **Headless Chromium** loaded a crafted save with the book owned, a load landing tonight and $60M clean, and there were no page errors. The risk panel's tonight line read the landing past the cover at the capped +35 heat. The Empire's lane card showed the load out and its payment, and an order set through the form was shown back. The trophy card listed the one owned. The Ledger's closed endings said what is short, and a $1M cash-out through the form landed $900,000 dirty.

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

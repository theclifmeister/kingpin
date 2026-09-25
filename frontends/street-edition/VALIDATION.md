# Engine alignment validation: protocol 19 / view 13 (#472)

The builder accepts protocol 19 and view 13. Protocol 19 added `rules.rivals.down`, what keeps a faction off the crown's count and for how long, which this edition does not call; the view's shape is unchanged.

# Engine alignment validation: protocol 18 / view 13 (#458, #459)

The builder accepts protocol 18 and view 13. View 13 (#458) gives alerts a `front`, for the new `front_shut` alert (a front shut for upkeep the clean pile could not pay); protocol 18 (#459) adds `rules.logistics.idle`, why a route on its dial sends nothing, which this edition does not call, and the engine's new `till` alert. The build copies the reference client's `session.js` and `alerts.js`, which word both alerts.

# Engine alignment validation: protocol 17 / view 12 (#474)

The builder accepts protocol 17 and view 12. Protocol 17 added the `characters` query, and made `new_run` refuse a character that is not one, `withdraw` refuse with no proposal made and `travel` refuse the city you stand in. Reviewed against `src/app.js`: it starts the default character, never calls `withdraw`, and draws the travel button only for a city you are not in, so none of the new refusals can reach it. The money refusals it can meet (`reserve`, `cash_out` and `fund` of nothing) now read `amount must be positive` and `clean cash only, and the clean pile is empty`, shown as the engine words them.

# Engine alignment validation: rules.logistics.idle (#459, shipped as protocol 18)

The builder accepts protocol 17 and view 12. Protocol 17 added `rules.logistics.idle`, why a route on its dial sends nothing, which this edition does not call; the view's shape is unchanged. The engine's new `till` alert (the wash has held dirty cash at the till for nights running) is worded by the shared `alerts.js` the builder copies.

# Engine alignment validation: protocol 16 / view 12 (#455)

The builder accepts protocol 16 and view 12. Protocol 16 added `rules.crew.lieutenancy`, the lieutenant's terms. The crew tab now words them through the shared `lieutenants.js`: a lieutenant's card says the city they run and their temper once it shows, `Run a city` / `Change city` opens a picker with the role in four sentences (the night's work, the cut and the crew slots, the tempers, the loyalty risk) and calls `assign` / `unassign`, a lieutenant in the pool says what they would be, and the tab says how lieutenants come while fewer than two cities are held.

- **`smoke.mjs`** reads `rules.crew.lieutenancy` off the actual WASM build and checks the four tempers, the cut and the slots, that the words carry no `undefined`, and that assigning nobody is refused.
- **Headless Chromium** loaded a crafted save with a violent lieutenant running Bayport, an unassigned one and one in the pool: the cards read `Runs Bayport` with `sells aggressive · heat ×1.25 · 3d stock · hits scouts`, `No city yet` with the reveal days, and the pool card the role; the picker showed the four sentences, and `Run this city` set the city through the engine. No page errors.

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

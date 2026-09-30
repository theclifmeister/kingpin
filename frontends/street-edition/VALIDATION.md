# Fixes against the TUI (#548, still protocol 24 / view 14)

The builder still accepts protocol 24 and view 14; nothing on the engine moved. A review against the TUI (#455–#547) found the edition wrong in places where the data was already on the wire, and `src/app.js` now reads it:

- **The sales approach.** The `#sale-dial` markup was broken (`</option value="normal">`), so Normal could not be picked and a sale went quiet. The three options are now built from one list, and the dial keeps its choice across redraws.
- **A laid-up member.** The crew card read `m.wounded`, which `MemberView` does not carry, so the read is dropped until the view has it (#550 adds it, #551 shows it).
- **The paper's MONEY section.** The section was filtered out. Its itemised lines (a lieutenant's cut, bail, a skim, a seizure, a cash-out fee) and `Cash $before → $after` now follow the flow table, as in the TUI's `moneyLines` (`ui/report.go`).
- **A card's outcome.** The `choose` result's `Outcome` now replaces the card under WHAT HAPPENED (`ui/card.go`), where before a toast read "Decision made".
- **The new-run list.** Built from the `characters` query: the six characters, each with its blurb (which names its start), the default picked. It used to hard-code two characters and "Start with $500 and a corner".
- **The offers.** Worded by kind as `game.Deal.String` and `World.Describe` word them (#537): `tribute: you pay them $X a day`, `homage: they pay you $X a day`, `a split: N corners your side of the line (names)`, `a joint shipment of N units`, `a N-day truce`. Note that #548's own text has tribute and homage the other way round; the engine's definitions (`game/diplomacy.go`) are followed here.
- **The ending cards.** The terms follow `ui/exit.go`'s `exitRows`, and every number is read from the engine: retire off `rules.laundering.offshore`; the crown's corners and days off the `city` plan's `share` and `streak` steps; the vanish chain's three costs off the `vanish` plan's steps; the businessman's nights off the `legit` plan's `streak`, or the fronts' income off `rules.laundering.legit_income` once the way is open. The old cards said "30 consecutive days" and "beats street revenue".
- **The confirms.** Each way out now has its own title and words, taken from `viewExit` (#498), followed by the TUI's `Left behind:` line. All four used to share one generic sentence.

Not ported, because the view does not carry the data (#550): the street window's night count (`street_window`) on the businessman card, the reign's day and the reign's income (homage and tax) on the crown's confirm, the score line and bodies, and the pending-transfer lines.

Checks:

- **`smoke.mjs`** (actual WASM) now also checks that each card's `choose` returns an `Outcome`, that the day's sales use quiet, normal and aggressive in turn and are accepted, that each report's `cash_before` and `cash_after` equal the flow's opening and closing, and that all six characters from `characters` have a name and a blurb and each starts a run.
- **Headless Chromium** (Playwright 1.61, `?test=1`), no page errors:
  - Picking Normal in the Market and selling sent `place_sell ["eastside","weed",2,"normal"]`, which the engine accepted, and the dial still read Normal after the redraw.
  - Ending days to the first card ("A line on the map") and picking a choice replaced the card with WHAT HAPPENED and the engine's outcome.
  - A save taken from the harness's `captained` policy (seed 1, the moves of day 48 made) was ended in the page. The paper's MONEY section read `Cece's cut of Eastside -$518` among its lines, then `Cash $513,300 → $521,857`.
  - Every number on the four ending cards was one the engine gave (the rules, the steps' have and need, the quiet days, the fronts' income). With each button forced open, the four confirms read in their own words.
  - The new-run dialog listed six characters, and each started as itself: the Cook with a chemist, the Bookkeeper with an accountant, the Ex-Cop with the scanner, the Dockhand in Bayport, the Heir with an enforcer.
  - A real tribute offer (the `crewed` policy, seed 4, day 45) read `Mother offers tribute: you pay them $1,000 a day`. The same offer re-kinded in the page's view read `homage: they pay you $1,000 a day`, and as a split `a split: 2 corners your side of the line (The Docks, Rail Yard)`. No harness policy draws a homage or a split offer.
  - At 390 px and 1280 px, the Market, Rivals, Ledger and Paper tabs and the new-run dialog had no horizontal overflow.

# Engine alignment validation: protocol 24 / view 14 (#530, #532)

The builder accepts protocol 24 and view 14. Protocol 24 added the event `HoldBegan` (the crown's hold begun, #530), which this edition shows through the report and the journal as it shows every event, and tuning fields on the rules the edition reads whole (`landless_burn`, `landless_floor`, `tribute_days`, `offer_quiet`); no method and no view field moved. Protocol 23 added the rule `law.odds` (the DA race's chances), which this edition does not call, and the engine now lists every danger alert before the rest, an order it shows as it comes. Protocol 22 added the `holds` query (the alert that keeps a fast-forward from starting a night), which this edition does not call; the engine's new `broke` and `war_muscle` alerts use fields alerts already had and are worded by the shared `alerts.js` the builder copies, and the flow gained a `debt` line (a connect's debt paid, apart from the purchases). Protocol 21 added `set_till`, the dirty cash the wash leaves in hand, and the rules `laundering.till` and `laundering.line`, which this edition does not call, and `place_standing` takes -1 for a standing order on the whole stash, which it does not send; the view's shape is unchanged. The engine's new `landed` alert (a route's stock with no order selling it) uses fields alerts already had (`city`, `product`, `count`) and is worded by the shared `alerts.js` the builder copies; its act is the market on the city. The new event `HandoffHeld` and the `All` fields on `SellOrder` and `StandingShort` are in the schema.

The engine's new `pages` alert (#492: the DA's file grown last night with no sting, raid or investigation; `level` the cause, `informant`, `retiree` or `tip`, `have` the pages, `count` and `amount` the file) uses fields alerts already had and is worded by the shared `alerts.js` the builder copies; its act is the crew screen. A member other than a lieutenant under the informant line is now a `crew_line` alert with `cross` `under`, as a lieutenant under the flip line was (#497). The view's shape and the protocol are unchanged.

# Engine alignment validation: protocol 20 / view 14 (#478)

The builder accepts protocol 20 and view 14. Protocol 20 added `set_sweep` and `stop_sweep`, the nightly sweep offshore, which this edition does not call; the view's shape is unchanged. The ambitions now carry a `dirty` unit (the vanish plan's lawyer on call, paid in dirty cash), which `src/app.js` formats as money beside `cash`, `clean` and `income`.

# Engine alignment validation: the arrest alert (#475, still protocol 19 / view 14)

The engine's new `arrest` alert (a warrant is out: sell nothing and lie low, or you are arrested tonight) uses fields alerts already had (`city`, `heat`, `line`, `due`, `days`), and the `heat` alert now carries `level`, the highest rung met; the view's shape is unchanged. Both are worded by the shared `alerts.js` the builder copies, and the law panel's arrest rung by the shared `police.js`. Reviewed against `src/app.js`: `openAlert` takes the arrest alert to the street tab with its city selected, as it does the heat alert (its act is the dashboard). The new events `WarrantSigned` and `WarrantLapsed` are in the schema.

# Engine alignment validation: protocol 19 / view 14 (#476)

The builder accepts protocol 19 and view 14. View 14 (#476) gives alerts a `share`, for the new `port` alert (the wholesaler's city untouched once its door is open or the corners at home have a ceiling: its free corners, the dearest product there, the wholesaler's share of street). The build copies the reference client's `alerts.js`, which words it; the protocol's methods are unchanged.

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

# The till, the wash and the road explained (#553, protocol 27 / view 16)

The builder accepts protocol 27 and view 16. Protocol 27 serves two laundering rules: `rules.laundering.float` (the float the till is never under, folded by the tree, a method the sim already had) and `rules.laundering.outlay` (the contracts' morning, `World.SupplyOutlay`, which `line` keeps back, now a method of its own that `Line` reads): without them the page could not word the till's range or the till that saves for the road. Nothing this edition read moved. The words are ported from the TUI into `src/wash.js`, which touches no DOM, and `src/app.js` draws them:

- **The till.** A panel on The empire, `#till`: the till (the float, or the line you set), the contracts' morning where it is over the till, the wash idle under the till, a route waiting on a lot with the till that would save for it (the field filled in with it), the fronts the night is expected to shut on their upkeep (`preview.wash.shuts`), what the wash takes tonight, and a field that sets the till (`set_till`): blank or under the float is the float, said; over the rot line is set to the top, said, and not sent. The `till` and `float` alerts land on the panel with the cursor in the field (`landing.js`'s new `focus`); `front_shut` picks out its front's card.
- **The fronts.** Each owned front says its status and why (`Shut 2 days: upkeep unpaid ($150 clean a night)`, `Audited: back in 13 days`, `Opens tomorrow`, `Idle: the dirty cash is under the till`) and its covered nights. The offers are `front_offers`, each with its terms (opens tomorrow, the till it washes over, the covered nights, a night unpaid shuts it), its lock (`$782,103 to go`, or the asset it waits on), and in red the shut the night is expected to bring (the TUI's `frontShort`); buying one of those takes a confirm. With nothing on offer the page says so, naming the fronts that wait on an asset (`fronts_waiting`).
- **The money cards.** The reserve is filled in with a lot, or the clean cash less tonight's upkeep where that is less; the cash-out with what tonight's wages are short (the fee on top), or $10,000, never into the upkeep. Each says tonight's upkeep, and a transfer or cash-out that would leave the clean pile under it asks first, in the TUI's words. A blank reserve with nothing to spare says why.
- **The road and the assets.** Each route in Transport says why it sends nothing (`rules.logistics.idle`): `Idle: too little over the $50,000 till for a lot`, the products it has none of in the source stash, no target, shut, or its target met. Each asset offer gives its upkeep and heat floor.

Checks:

- **`smoke.mjs`** now also holds `wash.js` to the live run: the till is the float until set, `set_till` stores what `setTill` sends and 0 is the float again, over the rot line nothing is sent, the reserve's and the cash-out's defaults keep tonight's upkeep back, a made-up front's status reads each reason, and every line (the wash's, a front's terms, a lock, a route's idle reason, an asset's terms) is free of `undefined` and `NaN`. The landing table has the till on `#till` with the focus on its field.
- **Chromium (Playwright)**, no page errors, on saves crafted from the harness's `boss` policy (seed 41, day 60, two fronts and the Channel): with the pile held at the float for fourteen nights, the laundromat shut on its upkeep and $100 clean, the till alert landed on The empire with `#till` picked out and the cursor in its field; the panel read `idle: dirty $50,000 is under the $50,000 till` and `Car Wash expected to shut tonight: upkeep $300 clean short`; the laundromat `Shut 2 days: upkeep unpaid ($150 clean a night)`; setting 80,000 stored the till, 999,999,999 set the field to the $50,000,000 top and sent nothing, and blank put the float back. The reserve and cash-out came up blank (the upkeep is all of the clean), a blank reserve said why, and $100 asked first: `Leaves $0 clean for $400 of upkeep tonight, and nothing over the till to wash: a front shuts 2 days.` With the Bayport stash empty and $100 over the float, the panel read `The Channel waits on $2,198 over the float; the wash takes it first: raise the till to $52,198 to save for it` and the route `Idle: too little over the $50,000 till for a lot`. With the restaurant just affordable and no clean, its card and its buy confirm read `Tonight's $550 of upkeep is expected to come up $550 clean short …`, and Keep playing left it unbought. An audited car wash read `Audited: back in 13 days`, a $120,000 till `the line you set`, and the reserve came up at the $50,000 lot. At 390px the page does not scroll sideways.

Not ported: the fund's upkeep warning (the TUI's `ui/law.go`), which belongs with the law's panel.

# The crew's answers (#551, still protocol 26 / view 16)

The builder still accepts protocol 26 and view 16; nothing on the engine moved. The crew tab now answers as the TUI's crew screen (`ui/crew.go`, `ui/life.go`, `ui/lieutenant.go`), its words in the new pure module `src/crew.js`:

- **Ask around.** A button on the tab with the price and odds (`rules.crew.investigate_cost`, `_odds`); its confirm says who asks, the odds of a name and what a blank costs. `landing.js` opens it from the `talking` alert and from `pages` an informant filed; `pages` from a tip or a retiree picks the button out.
- **Firing.** The confirm says what it costs the rest (nothing for a named snitch), who it puts at the walk line (`fireWalkLine`, #532) and, for an enforcer in a war that leaves fewer than `taken_out_muscle`, that the run ends taken out (`fireWarLine`, #520).
- **The member.** Tags for SNITCH (`exposed`), `laid up Nd`, `jailed Nd`, `out tomorrow` and `retiring`; the post by role (a driver's route, a lieutenant's city, an accountant's fronts, the chemist's lab, a corner, a guarded house, or idle / unposted); the lines they skim or turn under and walk at; the wage at the dial. Details adds the age and retirement, the wage at every notch, the temper and what firing does.
- **Money.** Pay-off and bail confirms with the cost and the loyalty they buy (bail warns when the clean cash is short); the pay dial lists each notch's wages a day and says what it does.
- **The roster.** `N of M on the payroll`, over the cap said; the crew warning (talking, a skim); a CREW summary: corners worked, idle runners, unposted enforcers, jailed, laid up, an accountant with no front, who runs each city, a lieutenant with no city, the snitch, the sloppy runners' heat (`rules.heat.sloppy_heat`).
- **Betrayal.** The shared `lieutenants.js` `roleLines` adds the betrayal sentence (`betrayLine`), so the reference client says it too; `play.mjs` checks it carries `BetrayShare` and `BetrayCorners`.
- **Three numbers from the TOML.** No rule serves crew.toml's `investigate_loyalty`, rivals.toml's `taken_out_muscle` or heat.toml's `sloppy_skill`, so `build.py` puts them in `engine-info.js`, as it does the traits, rather than bump the protocol.

Checks:

- **`smoke.mjs`** checks the confirms' words against the engine's numbers, the war line on a war order and on a war over the threshold, the tags and the laid-up post, `N of M`, every notch's wages, the landing table's new rows, and that `investigate` (once a night), `pay_off` and `fire` reach the engine.
- **`go test ./cmd/kingpin-web`** (`play.mjs`): the betrayal line; made to fail by changing the expected words.
- **Chromium** (Playwright, `?test=1`) on a save crafted from the harness's `boss` policy (seed 4, day 60) with the war order on Mother, one enforcer, `Heat.Leaks` 2, a runner laid up 3 days, one jailed 4 days and one named the informant: the `talking` alert opened `Investigate?` ($2,500, skill 23, ~31%, 2 loyalty) and confirming it asked (a second ask was refused); the cards read `LAID UP 3D`, `JAILED 4D` and `SNITCH`, `5 of 6 on the payroll`, and the summary; `Fire Tone?` on the last enforcer said `At war with Mother's crew, this leaves 0 of 2 enforcers: if they take your last corner, the run ends taken out.`; bail took Vee to `out tomorrow`; pay-off and the pay dial toasted in the TUI's words. At 390px no horizontal scroll. No page errors.

# The alerts: dangers apart, words caught up, land where answered (#549, protocol 26 / view 16)

The builder accepts protocol 26 and view 16. View 16 gives every alert `danger` and `notice` (the engine's `Alert.Danger()` and `Notice()`, always sent), the `reign` alert `slip` (the mornings a reign under the share has left) and the `scouts` alert `faction`; protocol 26 carries them in `holds` and `preview` too. No field this edition read moved. Reviewed against `src/app.js`:

- **Dangers apart.** One `alertButton` draws every alert in the business list, the risk panel and the end-day preview, red and bold on `danger`, muted on `notice` (`landing.js`'s `alertClass`). The risk panel lists every danger (it listed investigations alone), marks a warrant `WARRANT: served tonight on a sale`, gives the other cities' heat and says the evidence is one file for every city.
- **The morning.** A morning that opens on a danger rings a red toast in its words.
- **The preview.** The alerts lead, as in the TUI; each city's sales name what is handed over; the wash is a line, a front that shuts tonight in red; what the estimate leaves out is `p.unknown` worded, where a hard-coded list stood.
- **Words.** The shared `alerts.js` now words `file`, `crew_line` under, `investigation`, `reign`, `contract_due` and `scouts` as `ui/alerts.go` does (`play.mjs` checks each).
- **Landings.** `openAlert` follows `landing(a)`: the fronts, lanes, till, float and exposure on The empire; a house and a full stash in Properties; the ways out, the retirement and the plan on Ledger; the DA race on the fund; the favour on the police's risk; a contract, a connect's debt (now listed under the connect), a product and a faction picked out. `smoke.mjs` holds it to a table of every kind.

Checked in Chromium (Playwright) on saves crafted from the harness's `crewed` policy (seed 41, day 40): with a warrant out and the file at 5/5, the risk panel read `WARRANT: served tonight on a sale` and `Bayport heat 1`, both dangers were red in the risk panel, the business list and the preview while the gate (a notice) was muted and the heat alert plain, each alert clicked landed on its tab (the gate on The empire), and with the file at 4/5 ending the day rang the red toast `File 4/5: one more page is an indictment. …`. No page errors.

# Engine alignment validation: protocol 25 / view 15 (#550)

The builder accepts protocol 25 and view 15. View 15 exposes state the engine already kept and the view did not carry; it adds no alert kind, report line or method. Every field was added beside the old ones, and none was renamed, retyped or dropped, so nothing `src/app.js` reads has moved. `factions[].deals` is still the kinds, and the terms are in the new `deal_terms`. The new fields are listed in `docs/engine.md` (View 15): a member's `wounded`, `jailed_until`, `bailed`, `exposed` (a named informant only), `assigned`, `route` and `lab`; the law's `favours`, `campaign_open`, `chief_term_ends`, `chief_bought`, `da_bought`, `leads` and the patrol cap (`sell_cap`, `sell_cap_days`, `sell_cap_city`) and each city's `campaign`; a front's `city`, `bought`, `frozen_until`, `unpaid` and `audited`; `you.till`, `sweep_on`, `sweep_keep`, `war`, `score`, `bodies`, `pages_due`, `pages_pending` and `reserved_today`; the top-level `assets`, `cooks`, `orders`, `standing`, `supply`, `stats`, `stage` and `proposal`; a route's `target`, `days_target`, `checkpoint_until` and `closed_until`; and a faction's `deal_terms`, `police`, `last_raid` and `tribute_nights`. Protocol 25 adds `danger` to `fast_forward`'s answer and the rule `heat.tip_evidence`. This edition calls neither yet: they are for #551 to #556.

The crew card's `m.wounded` read was dropped by #548 until the view carried it. The view now carries `wounded`, the days still laid up, and #551 shows it.

- **`smoke.mjs`** passed on the view-15 build (day 45: forecasts, dilemmas, cash flow, lanes, trophies, cash-out, the ways out, the lieutenants and the save round-trip).
- **Not browser-checked**: no page was changed.

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

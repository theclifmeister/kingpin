# Parity with the TUI (standing, #558)

What the TUI shows a player and where this edition stands on it. **A PR that adds player information to the TUI adds a row here, or files the web follow-up in the "Street Edition parity" milestone and names it in the row** (`docs/web.md`, "Keeping up with the TUI"). A web PR that closes a gap updates its row. Status: **done (#N)**, the web shows it since issue N; **partial**, with what is left; **open #N**, owned by an open issue; **open, unowned**, needing a follow-up filed; **n/a**, ruled out of scope.

Seeded from the gap report against the TUI at 42d4284 (TUI PRs #455–#547), checked against the code at a334a7e (protocol 27 / view 17), after #548, #549, #550, #551, #552, #553 and #557; rows since updated by #569, #554, #573, #556, #581, #582, #574, #577, #576, #578, #575, #560 and #589. #550 added view fields only, so a row is credited to the issue that made the web show them.

Not counted: the key hints and the frame (#109, #536), since the web is click-driven with a button a row; and what was already shown at 42d4284 (trophies, export lanes, tonight's count, supply-short lines, quiet moves, the paper, card chips).

| Area | TUI item | Web status |
|---|---|---|
| Sale | The sell dial offers Normal (`ui/dialogs.go`) | done (#548) |
| Report | The MONEY section: itemised lines, cash before→after (`ui/report.go`) | done (#548) |
| Report | Lines that name TUI screens and keys (`sim/news`) | done (#560: the words name none; a link to the tab after the line, none for the intel screen the web lacks) |
| Cards | A card's outcome as WHAT HAPPENED (`ui/card.go`) | done (#548) |
| New run | Six characters with blurbs and starts (`ui/newrun.go`) | done (#548) |
| Alerts | Dangers and notices styled apart (#504, #492) | done (#549) |
| Alerts | Each alert lands where it is answered (`ui/alerts.go` `openAlert`) | done (#549; the law's, #552) |
| Alerts | Their answers: investigate, call in the favour, back a ticket, raise the till | done (#551, #552, #553) |
| Alerts | `file`, `crew_line`, `investigation`, `reign`, `contract_due`, `scouts` words | done (#549) |
| Alerts | Every other clause of `ui/alerts.go` | done (#573: the remedies and the missing facts; `webLeavesOut` keeps only the TUI's own way in and the retirement line, which the two count apart) |
| Turn flow | Fast-forward, holds, the most severe stop (`ui/fast.go`) | open #555 |
| Turn flow | End-day preview: danger styling, `unknown`, delivered, the wash (`ui/daypreview.go`) | done (#549) |
| Turn flow | The morning's danger alarm (`ui/model.go` `morning`) | done (#549, a red toast) |
| Dashboard | Warrant marked on the risk panel; other cities' heat; "one file, every city" | done (#549) |
| Dashboard | The crew-trouble summary | done (#574: on the street view and the crew tab) |
| Dashboard | The patrol sales cap (`sell_cap`, in the view since #550) | done (#574) |
| Dashboard | "Strike tonight" and the war's "nobody goes in" | done (#554: the war's "nobody goes in" on the faction at war's card; #574: the strike or the war order's corner on the street view, "⚔ push tonight" on the corner) |
| Law | The chief and the DA named; the DA RACE block (`ui/race.go`) | done (#552) |
| Law | Fund with city, goodwill preview, upkeep kept back; back a ticket | done (#552) |
| Law | Call in the favour, with what it saves (`ui/law.go`) | done (#552) |
| Law | Tip confirm: page odds, police attention (`ui/books.go`) | done (#552) |
| Law | Bribes, checkpoints, payoffs, pay a cop (`ui/bribes.go`, `ui/intel.go`) | done (#552) |
| Law | Lie-low words and the queued-handoff warning | done (#552) |
| Law | Sale heat estimate, pressure in numbers, the glossary | done (#575: the sale dialog's expect and heat rows with the dial's blurb, the TUI's `estHeat`; what the pressure cuts and fades to, and the goodwill's bite, under the risk panel's pressure line; the WORDS in the help overlay, held to `ui/keys.go` by `TestStreetGlossary`; sloppy heat #551) |
| Crew | Investigate with cost, odds, confirm (`ui/crew.go`) | done (#551) |
| Crew | SNITCH mark; fire confirm with cost, walk and war lines | done (#551) |
| Crew | Loyalty lines on the member; betrayal words (`ui/lieutenant.go`) | done (#551) |
| Crew | Posts by role; jail, laid up, bail; pay dial, pay-off, N of M | done (#551) |
| Crew | The CREW summary (`ui/crew.go`) | done (#551, #576) |
| Crew | Over-cap words on unassigning a lieutenant | done (#576) |
| Crew | Lieutenant temper countdown | done (#551) |
| Crew | Lieutenant "keeps" levels, pool countdown, kin | done (#576) |
| Crew | Captain picker: fixed budgets, who captains each city (`ui/captain.go`) | done (#576) |
| Lab | Chemist's hand, cook and cut dialogs (`ui/quality.go`) | done (#557) |
| Money | The till: shown, set, wash-idle and road-waits (`ui/till.go`) | done (#553) |
| Money | Reserve, cash-out and fund keep tonight's upkeep back | done (#553; the fund #552) |
| Money | Front-buy terms, shut warning, status reasons, shuts tonight | done (#553; the preview #549) |
| Money | Locked fronts on offer, "$N to go" | done (#553) |
| Logistics | Route idle reasons (`ui/routes.go`) | done (#553) |
| Routine | Standing orders (the whole stash, the edit words) and supply contracts (keep at, the morning's buy, holding on the road, the room) (`ui/market.go`, `ui/dialogs.go`) | done (#556; the room where you stand without your carry, and the warning on leaving, #582) |
| Routine | The cart from the view, with each line's take and remove (`ui/cart.go`) | done (#556: orders, standing orders and contracts; #582: the day's buys by hand, on credit and by contract this morning, each returned) |
| Routine | Sweep offshore (`ui/sweep.go`) | done (#581) |
| Routine | Owned houses with move, guard and drop; owned assets (`ui/houses.go`, `ui/assets.go`) | done (#581); an asset's "feds looked" and a lost asset are not on the view |
| Routine | Connect credit, route and market panes | done (debt #549, checkpoint #552, idle #553; #582: the connects with credit and debt and a buy on the book, the route pane, targets and driver, the market pane; #578: the pane's next product on the ladder, `ui/unlocks.go` `nextProductNote`) |
| Money | Wash audit odds, throughput, pile rot, tax | done (#577: the ledger's audit, capacity and legit line, the pile's weight and rot, the tax a city at a time, a front's washes at the dial with the accountants' share and its audit; a front's washed today open #588) |
| Money | Presets and upgrades in readable names | done (#582) |
| Money | Every figure as `internal/format` writes it: `-$N` for a negative, Go's ties to even | done (#589: one `src/format.js`) |
| Endings | The score (÷ 1 + bodies) on every way out and the ending screen | done (#554) |
| Endings | Ending cards' terms and each exit's confirm (`ui/exit.go`) | done (#548; street-window words and reign income #554) |
| Endings | Walk-away pending lines | done (#554) |
| Endings | Ambition steps by unit, the PLAN line, the quiet reset | done (#554) |
| Rivals | Crown progress: `crownShort`, the last clock, why each faction is off the count | done (#554) |
| Rivals | War declare and call-off with the confirm (`ui/rivals.go`) | done (#554) |
| Rivals | Propose with standard terms and odds; live deal terms (`deal_terms`) | done (#554) |
| Rivals | Faction detail: muscle, stance, mood, allies, stats | done (#554) |
| Progression | Stage interstitial and "new" mark (`v.stage`, since #550) | done (#578) |
| Progression | Journal, profile history, rank, slots | n/a (ruled out of scope, 2026-10-01) |

**The alert clauses the web leaves out** (`webLeavesOut`, held by `TestWebClient`). The TUI's own way in, kept out: `landed`'s `s sells it`, `exports`' `t on a lane`, `straight`'s and `vanish`'s `(walk away) or play on`, `talking`'s `Investigate`. The remedy, which the web lands on but does not say (ruled 2026-10-01: ported, #573): `pages` (fire whoever it names; nobody to fire; every tip can file a page), `war_muscle`, `skim`, `idle_corner`, `stash_full`, `scouts`, `house_known`, `wages` (cash out clean), `favour` (call it in), `unposted`, `no_corner`. Facts to port: `float` (the wash and the road wait), `till` (the wash takes the rest), `gate` (the peak line and what is to go), `exposure` (the loads and "before the wash"), `reign` ("2 crew", the web says "crews"), `favour` (the chief's name, "task force"), `no_corner` and `unposted` (the other city by name). The facts are #573 too. `retire` counts off the alert where the TUI counts off the world.

# One way to write a number (#589, no version moved)

`app.js` wrote a negative as `$-30,040` where the TUI's `format.Money` writes `-$30,040`, and each module kept its own `money`, four of them with the same fault (`app.js`, `crew.js`, `lab.js`, `law.js`). They now share `src/format.js`: `money`, `price` (`format.Price`), `cash` (`format.Cash`, `market.js`'s `short` with it), `pct` (`format.Pct`), `pctText` and `fixed`, Go's `%.*f` with an exact tie to the even digit, which every displayed `toFixed` and `Math.round(x * 100)` percent in the pure modules now goes through (a ×1.25 multiplier, a 12.5% share, a heat of 2.25). `wash.js` re-exports `fixed` and `cash`. The build stamps the revision into every module, not just `app.js` and `law.js`, so `format.js` loads once.

Where a negative now reads `-$N`: the Ledger's business income net of upkeep and its net worth, the cash flow's net change and lines, the fund dialog's clean left after giving (`law.js`), and any `money` of `app.js` given a debt or a loss. The inline percents in `app.js` (the investigation's odds, a proposal's odds, the captaincy's cut, the transfer fee, the driver's cut) still use `Math.round`, left for a PR that is in `app.js` anyway.

- `smoke.mjs`: `money` over zero, positives, negatives and sub-dollar amounts (`-$30,040`, `$0` for -0.4, `-$1` for -0.6); `price` with a tie at the cents; `pct`, `pctText` with ties both ways; `wash.cash` still served; and no source module but `format.js` defines `money` or calls `toFixed` with a fixed digit count.
- Chromium (Playwright headless, 1280×900 and 390×900) on a crafted save (`boss`, seed 4, 60 days, both fronts set to level 0): the Ledger reads `Business income, net of upkeep -$550/day`; no `$-` on any tab; no page errors; no horizontal scroll at 390.

# Where a report line points (#560, protocol 28 / view 22)

The builder now accepts view 22: a report section's `points` (`{line, at, kind, act}`) and a lead line's `at`. About fifteen report lines named TUI screens and keys (`on the ledger screen (7)`, `(map, r)`, `(2, d)`, `(f)`, `walk away on the dashboard`), which the page printed as they came. The user ruled (2026-10-02) that each line carries an act and each front end words its own pointer: the words now name no screen, the TUI puts its key hint back at `at` (`ui/report.go` `pointerWords`), and the page adds a link after the line. No sim changed; `TestSeedDigest` moved by the report's shape and words alone (with `World.Report` set aside it is main's on all sixty days), `TestMoneyCurve` did not.

- `src/report.js`: a section line's point (`pointOf`), the point as the alert `landing()` reads (`pointAlert`, null for the TUI's intel screen, which has no tab here, so a spy's and a lure's lines read whole), and the link's words, the tab's name (`linkWords`: `The empire →`).
- `src/landing.js`: the point kinds whose tab is not their screen's: a front and an asset on The empire (an asset in Properties), a seizure's dial in the market's transport routes (`open: "routes"`); the reign and going straight land on Ledger's walk away as their alerts do; `scouts` with no faction picks nothing out.

Checks:

- `smoke.mjs`: every point kind lands on its tab, opens what it names and is linked by the tab's name (the POINTS table); the intel points have no link; a section without points (a report from before view 22) has none; on the 45-day seed-41 run every section carries `points`, each on a line it has, and no line names a TUI screen or key.
- Chromium (Playwright headless, 1280×900 and 390×844) on a crafted save (the `boss` policy, seed 4, day 15, whose report has a front unlocked and a buyer's offer): the paper reads `The Car Wash is open to you: it washes over the till. The empire →` and `… Answer it: it stands 3 days. Market →`; the first link lands on The empire, the second on Market, and a `seizure` point opens the Transport & supply sheet on Market. No console errors; no horizontal scroll at phone width.
- The TUI: `TestReportLinesPointAndTheTUIWordsThem` feeds each kind's event through the news sim and holds the TUI's line to its words; ten read exactly as before, five were reworded so the words stand whole (`docs/market-and-journal.md`).
# The stage and the new mark (#578, no version moved)

The web reads view 15's `stage` (`Session.stageView`) and the ladder, in `src/stage.js`, pure:

- The interstitial (`ui/stage.go`, #149, #525, #537): `STAGE 2 · CREW`, the blurb, the text as one paragraph, OPENED, the pointer to the lanes on The empire while the exports alert stands on the stage the run is in, and NEXT (`Session.StageNext`, so the `crossed the line already` form too). The morning a stage is pending it opens before the card, after the day ends and on load; closing it, by Carry on, ×, Esc or the backdrop, calls `see_stage` and goes on to the card. A reload with it still open shows it again, as the TUI reopens a stage saved on the modal; once closed, a reload does not.
- The new mark: the header's tier reads `Crew · new` while the stage is not yet seen, as the TUI's tier fact and STREET title do.
- The market pane's next product (`ui/unlocks.go` `nextProductNote`): the last of a product's Notes, `Heroin lists at $3,000 peak cash ($2,500 to go).`, with `; the supplier here will not sell it, the road brings it` where the city's connect will not, and nothing once the ladder is listed. The view does not carry the ladder, so `build.py` puts it in `engine-info.js` from `market.toml` (`ladder`, the file's order with each `unlock_cash`) and `city.toml` (`noSupply`, a city's `no_supply` products), as #551 and #581 carry tuning; amounts are `format.Cash`'s (`wash.js` `cash`).

Checks:

- `smoke.mjs`: day 0 has no stage, no mark and the Heroin line with its distance; a city listing all but Designer reads the honest form, a city listing the ladder reads nothing; seed 41 with one hire on day 0 enters stage 2 the next morning: `STAGE 2 · CREW`, the words clean, the tier marked new, no lanes before the Cartel (and the lanes where the exports alert stands); the stage survives a save round trip until `see_stage`, and after it the mark and the stage are gone, through a save too.
- Chromium (Playwright headless, 1280×900 and 390×800), a fresh seed-41 run: Weed's pane ends `Heroin lists at $3,000 peak cash ($2,500 to go).`; after a hire and End the day the interstitial opens (`STAGE 2 · CREW` with its blurb, text, two OPENED lines and NEXT `Move $25K: the laundromat opens; …`) and the tier reads `Crew · new`; a reload opens it again; Carry on closes it, the tier reads `Crew` and `v.stage` is gone; a reload then opens nothing. Esc closes and marks it seen the same way. No page errors; no horizontal scroll at 390.

# The crew's last words (#576, protocol 28 / view 21)

The builder now accepts view 21, which carries state the engine kept and the view did not: a member's `kin` (on the payroll and in the pool), `you.carry_limit`, `you.capacity` (`World.Capacity`, a city) and `you.pool_day` (`Crew.PoolDay`). No sim changed, so no seed-pinned number moved; `TestViewCarriesTheCrewsOddsAndEnds` pins the fields. A lieutenant's keeps needed nothing new: view 15's `supply` names the lieutenant. The words are in `src/crew.js`, pure: the CREW summary's capacity rows (`ui/crew.go` `crewSection`), the city picker's rows, the over-cap warning and words and the assign results (`ui/lieutenant.go` `viewAssign`, `unassignCapLine`, `overCapWords`, `confirmAssign`), the captain picker (`ui/captain.go`: the cities with the corners held and who captains each, opening on theirs or where they work, the fixed `Budgets` a night, the refusals, the results), a lieutenant's keeps (`lieutenantKeeps`), the pool's countdown (`viewCrew`) and kin (`ui/life.go` `kinNames`, `relation`, the ♦ mark and its legend, a face's discount). Standing a lieutenant down over the cap asks first. A lieutenant has no Captaincy button; the captain picker's budget is a select of the fixed budgets, where it was a free number.

Checks:

- `smoke.mjs`: on the seed-41 crew run, the capacity row reads `you.capacity` where you stand and `N yours + M crew`; the over-cap warning and words on a roster one past the cap without the slots, and none with no city or under the cap; the budgets are fixed and each worded; no captaincy for a lieutenant and why a member is not ready (loyalty and days off `rules.crew.captaincy`); the picker's rows and who captains each; the keeps with yours winning; the pool's countdown, never under a day; kin by name and the TUI's word.
- Chromium (Playwright headless, 1280×1600 and 390×900) on a `delegated` seed-4 save at day 120 (Cal runs Eastside with five contracts; padded to 14 on a cap of 18 with Cal's 8 slots; Shorty captain of Bayport at $20,000): the CREW block reads `capacity · 1796 in Bayport`, `550 yours + 1246 crew`; the pool `new faces in 2 days`; Cal's card `Keeps 111 Weed · 43 Pills · 21 Coke · 31 Heroin · 2 Meth` and `Kin: Flaco (cousin)`; Wink in the pool `Kin: Flaco (partner) · came with the kin: fee at the discount`; the legend `♦ has kin on the payroll or looking for work`. Cal's picker warns `Off the city, the roster is 14 of 10: nobody is let go; you hire under 10.` with rows `Eastside · 2 corners · 682 units · runs: theirs now`; standing him down asks `Stand Cal down?` first and answers `Cal runs nothing now. The crew they posted stay where they are. The roster is 14 of 10: nobody is let go, and nobody is hired until it is under 10.` Cal's card has no Captaincy button; Flaco's picker refuses `Can't make Flaco captain yet: it takes loyalty 55 and 30 days on the payroll, at work.`; Shorty's opens on `Bayport · 3 corners · captain: theirs now` at `$20,000 a night`, offers `$10,000 a night | $20,000 a night | $5,000 a night | nothing: no pay-offs`, and answers `Shorty is captain of Bayport from tonight: $10,000 a night for pay-offs, keeps 2%.` No page errors; no horizontal scroll at 390.

Not covered: the TUI's key hints in the refusals (`Give them one with l` reads `with Run a city`).

# The sale's heat, pressure in numbers and the glossary (#575, no version moved)

- **The sale** (`routine.salePreview`, the TUI's `sellDialRows`): Sell opens a dialog with the quantity and the dial, and as either moves it shows `EXPECT ~5 of 5 at ~$22.00 = ~$110` and `HEAT +0.5` toned as `heatStyle` tones the city's heat plus four times the estimate, the dial's blurb, and "You work no corner in …" where none is worked. The heat is `rules.heat.sale_heat` on the units asked plus `rules.heat.sloppy_heat` on the units expected to move: the TUI's `estHeat`. The dial picked becomes the page's sales approach.
- **Pressure** (`law.pressureNote`, the TUI's police section): under the risk panel's pressure line, "Lowers the police lines 6% and a patrol's cap 15%; fades to 10.", and "Goodwill takes 1.2 a day." at 40 goodwill. The cuts at pressure 100 are law.toml's `[effects]`, which no rule serves, so they reach the page through `engine-info.js` (`pressure`), as #551's numbers do; the fade and the goodwill are `rules.law.tuning`'s.
- **The glossary** (`glossary.js`): the help overlay (`?`) ends with WORDS, the TUI's terms in its order, the numbers filled off the file (`engine-info.js` `glossary`). Two terms name the web's button where the TUI names a key (goodwill, lieutenant); the TUI's pane, strip, scene, daily and profile are left out. `TestStreetGlossary` (`cmd/kingpin-web/parity_test.go`) holds the list to `ui.Glossary`, term by term.

Checks:

- `smoke.mjs`: a run of its own (seed 41, four hired and posted, the stash sold aggressive) previews every dial of every night's sale and holds the units, the heat (to the rules' sum), the rows' words and tone to the TUI's, until runners on corners add their premium; the no-corner warning; the pressure note at 60 with and without goodwill and past 100; every glossary line filled, and the quiet day, sting, raid, betrayal and street night lines against the file.
- Chromium (Playwright, desktop 1280×900 and mobile 390×844, no console errors, no sideways scroll): a fresh seed-41 run stocks weed, opens Sell, moves the dial and the quantity and reads the engine's heat in the dialog, queues the sale into the view's orders; the risk panel carries the pressure note; the help overlay lists 62 terms.

# The dashboard's facts (#574, no version moved)

The TUI's STREET facts that the web lacked (`ui/dashboard.go` `streetLines`), as pure functions checked by `smoke.mjs`:

- `law.patrolCapLine` (`patrolCapLine`, #507), on the risk panel above the LAW lines, off view 15's `sell_cap`, `sell_cap_days` and `sell_cap_city`: "Patrols in Eastside: sales capped at 59% of demand for 2 days more."
- `rivals.tonightLine`, at the top of the street view: the strike queued tonight by the corner's name ("Enforcers go to The Projects tonight: push."), else the war order's corner and force, nobody to send (#554's words) or nowhere to go. The view does not carry `World.Today.Strike`, so the strike comes from the page's memo, which now keeps the corner and the force beside its text (the cart's line names the corner too, not its id); the memo is cleared at the end of the day as before. A rival corner with a strike on it reads "⚔ push tonight" on its map marker and in its dialog (`ui/map.go`). An old save's memo, without the corner, shows no line.
- `crew.trouble` (`crewTrouble`, #345, #522), on the street view and the crew tab: "2 near or under the line, 1 corner unworked", each member in the band once (a `crew_line` alert or under their skim or turn line), the corners off the `idle_corner` alerts.

Checks:

- `smoke.mjs`: a run of its own (seed 41, stingy pay, four hired, the stash sold aggressive) reaches a patrol's cap by day 9 and the line names Eastside and the cap's share and days; reaches crew trouble and counts the band as the TUI does; sends its enforcer at The Projects and reads the strike line by name; declares the war and reads its line, then calls it off.
- Chromium (Playwright, a day-18 save of that run): the cap on the risk panel, "Crew: 2 near or under the line" on the street view and the crew tab, and after sending enforcers from The Projects' dialog the street's strike line, the marker's "⚔ push tonight", the dialog's mark and the cart's "Enforcers sent to The Projects", with no console errors.

# The connects, the market pane and the routes (#582, protocol 28 / view 20)

The builder now accepts view 20, which carries state the engine kept and the view did not: `buys` (the day's buys, `World.Today.Buys`), `you.away` (`World.CapacityAway`) and `you.street_quality`; each connect's `day_cap`, `products`, `quality`, `small_lot`, `credit_ratio`, `locked`, `unlock_cash`, `unlock_rel`, `frozen_until`, `warned`, `late` and `extended`; each product's `glut`, `served` and `shock`; each city's `worked`; and `law.watch_until`. No sim changed, so no seed-pinned number moved; `TestViewCarriesTheBuysAndTheConnects` pins the fields. The words are in two new pure modules and `routine.js`:

- `src/market.js`: the SUPPLIERS block and the connect's pane (`ui/suppliers.go`: the note, the temper, the band and its word, the price against the street and at the next band, the lot and small-lot premium, today's left of the day's cap, the quality, credit and debt with the due word, the status, missed payments, how to buy, the rules), the buy dialog's connect step, credit terms and cash-short line (`ui/dialogs.go` `connectBlurb`, `creditTerms`, `buyTermsRows`, `confirmBuy`), the debt line, and the product's pane (`ui/market.go` `marketDetails`: range, glut, margin, demand on N corners and per standard, the lot's and the connect's quality, cooking, shock or slump, ELSEWHERE and NOTES).
- `src/routes.js`: the route's pane (`ui/routes.go` `routeFacts`: dial, closed tonight, the skies for the plane, days and capacity at the dial, fare and seizure odds, targets, the road, the driver) and the target dialog (`viewTarget`, `routeBuys`, `confirmTarget`), and the driver picker's words (`ui/life.go`).
- `src/routine.js`: the cart's buys (`ui/cart.go`: merged per city and product by how they were paid, `buy`, `credit` and `morning`, each returned with the TUI's words), the contract room where you stand off `you.away` and the leaving warning (`contractRoom`, `contractsLeft`), the preset review's names and estimates (`ui/presets.go`), and an upgrade's prerequisites by name.

Checks:

- `smoke.mjs`: on the 45-day seed-41 run, every connect's note, pane, blurb, rules and how-to-buy, and every product's pane, ELSEWHERE and NOTES read clean; a cash buy costs the page's quote, shows in the cart from `buys` and is returned with its refund; a buy on Cass's book costs the credit quote, shows as a credit line (`on the book`, the totals `on credit`) and is returned off the book; the room where you stand is counted without your carry; every route's pane reads, a days target is set (`3d (≈N) Weed`) and cleared; every preset's changes are named without an id; every prerequisite is an upgrade's name.
- Chromium (Playwright headless, 1280×900 and 390×900) on that run at day 45 with 3 Weed bought, 2 on Cass's book, a 2-day Coast Road target and a 30 Weed contract: the SUPPLIERS block reads `Owe $25, due in 7 days` and `Cass 50 1,495 55 owe $25 by d52`; the cart `Buying 2 lines for $57 (1 on credit) · 1 contract kept`, `Buy 3 Weed · $10.67 · $32`, `Credit 2 Weed · $12.50 · $25 on the book`; Cass's pane `REL 55 · neutral`, `PRICE ~54% of street`, `~53% at rel 60`, `TODAY 1,495 of 1,500 left`, `CREDIT $2,975 of $3,000`, `×1.15/u, due d52 with the debt`, `DEBT $25 due d52, in 7 days`; Weed's pane `GLUT 0.3%`, `MARGIN +81% over supplier`, `DEMAND ~62/day on 1 corner`, `BAYPORT $30.19 · sup $16.39`; the buy on credit `$49: $12.19 a unit, ×1.15 the cash $10.60`, `Due day 52 · $2,975 of the book left`, answered `Bought 4 Weed from Cass on credit: $49 on the book, due day 52.`, and its return `Returned 6 Weed: off the book.`; the cash line's `Returned 3 Weed, $32 back.`; the routes `DAYS 2 · capacity 60`, `FARE $8/u · seized ?`, `TARGET 2d (≈124) Weed`, `DRIVER nobody`; the target dialog `TODAY 3d ≈ 186 units` and `SHORT 183: 0 from the Bayport stash, 183 with nothing to ship + $1,464 fares`, answered `Coast Road keeps Eastside at 3 days of Weed's demand (≈186 today): it sends the shortfall every day.`; the driver with none on the payroll `Nobody to put on the road: no drivers. Hire one on Your people.`; Quiet trading's review `launder dial normal careful`, `route Coast Road normal slow`; the prerequisites `Stash spot`, `Supplier contact`; leaving Eastside asks first, `The Weed contract (keep 30) has room for 0 once you go: your carry leaves with you.` No page errors; no horizontal scroll at 390.

Not covered: the lieutenant's buy in a city you are not in (the market tab is the city you stand in), and saving a preset of your own (a TUI-profile feature).

# The sweep, the houses and the assets (#581, protocol 28 / view 19)

The builder now accepts view 19, which adds each house's `price`, `rent`, `bought` and `unpaid` (state the engine kept; the TUI's STASH table and house pane show them). The words are in `src/property.js`, pure: the sweep dialog (`ui/sweep.go`: the line, blank the upkeep, held to the clean in hand; tonight's move and its fee; the rules), the houses (`ui/houses.go`: the status in the TUI's vocabulary, the block, the stash by product, the rent and unpaid days, the guard, the robbery odds, since when; the move with what the destination holds and the drive's heat; the guard picker with where each enforcer is; the drop's words) and the assets (`ui/assets.go`: what each does, standing or shut and for how long, upkeep, heat floor, bought). The street's capacity in the move dialog is worked back from the view (`you.room` less the houses' free room). `engine-info.js` carries `move_heat` and each asset's effect numbers from the TOML. `you.stock` is the street and the houses (`World.Stock`); its comment in `view.go` said the street, and `wash.js` added the houses again (harmless there, as it only asks for none).

Checks:

- `smoke.mjs`: the sweep set and stopped through the engine with its words; every asset on offer says what it does; on a seed-7 run that trades Weed until a house is on offer and paid for (day 30), the house leased shows view 19's rent, price and day, its rows, a move between the street and the house reaching its place with the street's capacity unchanged, a guard posted and taken off when there is an enforcer, and the drop; the stash counts the house.
- Chromium (Playwright headless, 1280×900 and 390×900) on a seed-7 run with Rooms over Precinct Row leased and 5 Weed bought into it: Properties draws `Your houses` (`unknown`, `5 of 400`, `$60/day clean`, `nobody`, `0.4%/day`, `day 44 · $12,000`); Move stock offers `the street · 27/100` and the house, previews `+0.1 tonight for 4 units` and answers `Moved 4 Weed from the street to Rooms over Precinct Row. The drive is +0.1 heat tonight.`; Guard with no enforcer refuses `Nothing to post: no enforcers. Hire one on Your people.`, and as the Heir (an enforcer from day 0) posts Lolo: `Lolo is inside Rooms over Precinct Row: robbery 0.2%/day.`; the sweep at $500 previews `$1,390 moves, fee $70` and is on after a reload (`Sweep · on, keeping $500`); Drop confirms `The 9 units in it go with it. Move them out first.` and answers `Dropped Rooms over Precinct Row: 9 units went with it.`. No page errors; no horizontal scroll at 390.

Not covered: the TUI's "feds looked Nd ago" on an asset and a lost asset's note (the task force's last response and the lost assets are not on the view), and the dashboard's `carrying N/M · stashed N in K houses` line (#574's dashboard facts).

# The routine, part one (#556, protocol 28 / view 18)

No version moved: view 15 (#550) already carried `orders`, `standing` (with `all`) and `supply` (with `lieutenant`), and the commands `place_standing` (with -1 for the whole stash), `cancel_standing`, `set_supply` and `clear_supply` were on the protocol. The words are in `src/routine.js`, pure: the TUI's market pane rows (`standingRows`, `contractRows`), the sell and buy dialogs' edit words (#443), the whole-stash standing order (#503), what lands tonight, what the contract buys before the sales and why it brings none (#467, #524, off `rules.market.due` and `due_short`), the room warning for a contract in a city away, `supplyShortWords`, and the cart (`ui/cart.go`) with each order's take off `rules.market.capacity`, `dial` and `cut`. The page's memo of sales is gone: the cart is the view's, and an old save's memo sales are dropped on load. Other moves queued for tonight (a strike, a scout, a delivery, a transfer) stay in the memo.

Checks:

- `smoke.mjs` (Node, the built WASM, seed 41 to day 45): a contract and a whole-stash standing order set as the page sets them show on the view with the TUI's rows and edit words, over-the-stash numbers are refused in the TUI's words, every order, standing order and contract of the view is a cart line, and both survive `export_save`/`import_save`.
- Chromium (Playwright headless, 1280×900 and 390×900) on a fresh seed-41 run with 12 Weed bought: the Routine dialog places a quiet standing order on the whole stash and a contract at 30 (`Keeping 30 Weed in Eastside: topped up … at ×1.05 the supplier's price.`); the product row reads `Standing all quiet · cut 5%`, `Keep at 30`, `Brings 18 in the morning at $12.13`; a sale of 3 then reads `Sell 3 Weed · quiet · ~$57` over the `Keep 30 Weed · ~18 tonight` line; after a reload all three are on the view and the cart; cancelling the sale puts `Standing all Weed (30) · quiet · ~$541` in its place. No page errors; no horizontal scroll at 390.

Not covered here (the rest of #556): the sweep, owned houses and assets (#581); connects with credit, the route and market panes, presets and upgrades in readable names (#582). The TUI's contract warning where you stand counts the room without your carry, which the view does not carry; this edition gives it for a city away only (#582).

# The endings, the crown and the rivals' table (#554, protocol 28 / view 18)

The builder now accepts protocol 28 and view 18. View 18 exposes state the engine already kept, which the page needed and could not read: `you.reign` and `reign_slip`; each faction's `city`, `stance`, `absorbed`, `absorbed_by`, `fragmented`, `betrayed` and `split_lines`; `stats.deals`, `deals_refused`, `tribute`, `homage`, `informants` and `crew_poached`; `fallen`; and `over.title`, `won`, `epilogue`, `story` and `reached`. The epilogue and the story moved from `ui/summary.go` into `engine.Session.Epilogue` and `Session.Story`, so the TUI's summary and this ending screen tell a run the same way (`TestSummaryReadsTheRun` and the story tests still pass on the TUI; `TestViewCarriesTheTableAndTheEnding` pins the view). No sim changed, so no seed-pinned number moved. The words are ported into two pure modules, `src/endings.js` and `src/rivals.js`, and `engine-info.js` adds `streetWindow`, `kingpinShare` and `warForce` from the TOML.

Checks:

- `smoke.mjs` (Node, the built WASM, seed 41 to day 45): the four ways out with their terms, what is short and the same score row; each confirm scoring `Score $X:` off `you.score`; the pages due closing every way out with the TUI's words, and the pending lines; the crown's short, the reign's income and a faction off the count exactly when `rules.rivals.down` says it does not count; the slipping reign; the plans' step words by unit, the PLAN line with a sting resetting the quiet days, and a task force named; the summary's facts (the fallen, the bodies, the betrayals, a kingpin's reached day); for each live faction the three standard truce, tribute and split asks with odds in 0..1, the tribute basis, a proposal put and withdrawn, and, with an enforcer hired, war declared on it with the confirm naming the target and odds, then called off. The landing table now sends `retire`, `vanish`, `straight` and `reign` to their way out's card and `plan` to the PLAN line.
- Chromium (Playwright headless, 1280×900 and 390×900) on two crafted saves (the `boss` policy, seed 4, 60 days; one with $900,000 offshore, two bodies and a truce, one the same run ended as kingpin with a fallen runner): the Rivals tab draws the crown's card (`4 held of 10 in Eastside, 6 needed, 2 crews still standing, 4 crews yet to arrive`, then each crew off the count with why), each faction's card with its deal's days left and mood; Declare war opens `War on Mother's crew?` naming The Strip, the odds (`?`, the muscle unknown) and the heat, warns of the truce it breaks, declares (`you.war` set), and Call off the war confirms and clears it; Propose lists the live truce, opens the tribute's three asks (`$660/day 5% of your street (thin) ~44%`…) with the street's basis, and sends the proposal with the odds in the toast. The Ledger's four ways out each show `SCORES the account over 1 + 2 bodies: $300,000`. The ending modal's large number is `$300,000`, the engine's `you.score`, with the epilogue, the story, the money, the people and the city. No page errors, no horizontal scroll at either width.
- Fixed during the check: a negative figure read `$-550` (now `-$550`), and a crew not yet arrived read `run out of town` (now `not moved in yet`, with no mood line).

Not covered: the TUI's ambitions panel's "ready to take" key line and the stage interstitial (#149) stay out, as the gap report listed them apart; the propose dialog keeps the TUI's three asks and no longer takes a free number.

# The cook max keeps the wages (#569, protocol 28 / view 17)

The builder accepts protocol 28 and view 17. Protocol 28 serves two crew rules: `rules.crew.spare` (the dirty cash less tonight's wages at the dial, `crew.Sim.Spare`) and `rules.crew.cook_max` (the cook's most: the batch, the room with what is cooking counted, and what `spare` buys at the unit's precursors, `crew.Sim.CookMax`). A Cook who cooked the dialog's blank on day 0 went broke on day 2 with the lot still cooking: the max spent all the dirty cash and kept nothing for the chemist's wage. `src/lab.js`'s `cookMax` is now the engine's rule, the TUI's too, so the two cannot drift, and `cutMax` holds the till to `spare`. Nothing else this edition read moved.

The engine also counts a lot still cooking as stock (`World.TotalStock`), as it counts the road, so the broke check and the `broke` alert no longer end a run with a paid-for lot on its way: the owner's ruling, a sim change with no pinned number moved.

Checks:

- **`go test ./internal/engine`**: `TestCookMaxKeepsTheWages` plays seed 41's Cook through the blank to the lot landing (made to fail by leaving `Crew.Cooks` out of `TotalStock`). **`go test ./internal/ui`**: `TestCookBlankKeepsTheWages`.
- **`smoke.mjs`**: `spare` is the dirty cash less `rules.crew.wages`, and the cook max at the unit's cost fits in it.

# The law and its answers (#552, still protocol 27 / view 17)

The builder still accepts protocol 27 and view 17. One view field changed its count, not its shape: a city's `campaign` now includes today's backing (`World.Campaigning`, as the TUI's race reads it), where it held only the nights before, so money put behind a ticket showed nowhere until the morning (`TestViewCampaignCountsTodaysBacking`). No client read the field before this edition. The words are ported from the TUI into `src/law.js`, which touches no DOM, and `src/app.js` draws them:

- **The LAW lines** (the TUI's LAW panel) on the risk panel, in place of the pressure line: `Chief Kowalski · corrupt · owes one · 90d left`, `DA Ostrowski · reform · election in 12d`, the pressure and goodwill here (and the ticket you are backing), the other cities' pressure. A `Call in the favour` button sits on the chief's line while they owe one and no law-and-order DA sits.
- **The law on Ledger** (`#law`), in place of the community fund: the chief and DA card, the fund card and the PAYOFFS card; under them the **DA RACE** block (`#race`) while the tickets take money: the vote's day, the odds per ticket (`rules.law.odds`), who sits, the price a point, a row a city with the ticket, what your money holds and the points it buys, a `Back a ticket` button, and what it costs (`rules.law.campaign`). The `da_race` alert lands on it, and the `favour` alert on the law's card with the call's confirm open (`landing.js`; both rows moved in `smoke.mjs`'s table).
- **The fund dialog**: the city, the goodwill amount (blank is up to 100 less tonight's upkeep, #458), what it buys, the upkeep warning in `wash.js`'s words, and while the race runs the ticket and the amount behind it, with the price before the amount (#506) and a warning when the city's money is on the other ticket. Nothing is given until Give; a blank that is all upkeep is refused in the TUI's words.
- **The favour's confirm**: what the call stops (and where, when the hot city is not yours), what it saves (the rung's take, off `rules.heat.rungs`; a sting's named hit off the investigation alert), and its price; refused with why when it cannot be made.
- **The tip's confirm**: the police's attention before and after, the raid and arrest lines, a raid cooling off, the trust, `~10% the DA's file on you gains a page` (`rules.heat.tip_evidence`) and a peace it breaks.
- **Lie low** says who sells nothing (`your lieutenants' included`) and asks first when a handoff is queued (the page now remembers a delivery's contract and units).
- **The bought law**: the bribe dialog (the chief or the DA with what is known and their price, blank the price, what the envelope is likely to do, the terms); each route's checkpoint or customs agent in Transport, bought through a confirm with its price, cut and days and the risk the file knows (`rules.logistics.risk_from`); the PAYOFFS card with the live deals and their days, what a fixer hears (`leads`); and the cop dialog (blank the price, how straight the word is). A cop's price and accuracy are intel.toml's, which no rule serves, so `build.py` puts them in `engine-info.js` as #551 did.
- **`act`** now answers `true` for a command that answers nothing, so a caller can tell success from a refusal (`null`).

Read off what the view has, where the TUI reads the world: a DA you backed is one whose envelope odds are not nil off their ticket, or a moderate at the backed price (`rules.law.d_a_price`); a bought chief's `half the good` shows once their temper is known to be lazy.

Checks:

- **`go test ./internal/engine`**: `TestViewCampaignCountsTodaysBacking` (made to fail by reading `c.Campaign` again).
- **`smoke.mjs`** holds `law.js` to the live run: the LAW lines, the odds adding up, a race row a city and none while the tickets take nothing, the backed row's points, the fund's blank keeping the upkeep back and its preview, nothing to give refused, a backing on the other ticket warned, the most the campaign takes, the favour's three refusals and what it saves, the tip's attention, page odds, end of a crew and broken peace, the lie-low words and a held handoff, every official's words under each DA, each route's deal, the payoffs in order and the cop's accuracy; `call_favour`, `back`, `fund` and `bribe` refused where they should be and `pay_cop` once a day.
- **Chromium (Playwright)**, no page errors, on saves crafted from the harness's `boss` policy (seed 4, day 60) with the race open twelve days out under a reform DA, $500,000 clean, a favour owed and a raid due at home: the risk panel read `Chief Kowalski · corrupt · owes one · 90d left` and `DA Ostrowski · reform · election in 12d`; the `da_race` alert landed on Ledger with `#race` picked out, `DA RACE · THE VOTE ON DAY 72, IN 12 DAYS`, 23% law-and-order, 52% reform, 25% a moderate. `Back a ticket` on Eastside with $30,000 previewed `0.1 points of Eastside's vote for $30,000`, and Give put the row at `reform $30,000 0.1 points` at once. A blank fund gave $100,000 (goodwill to 100; $150 upkeep kept back). The favour alert opened the confirm on Ledger: `What it saves: the raid would take 50% of the stock and 30% of the dirty cash there, and 2 pages in the file if you sold.` A $60,000 envelope went to the chief, a $2,500 cop read `~38% straight`, the Coast Road's checkpoint was bought for $40,000 and listed in PAYOFFS until day 90; the tip on Carmine's corner read `0 → 15; at 60 they raid the corner, at 90 they take Carmine`, the page's 10% and the truce it breaks. With $100 clean, a blank fund was refused (`Blank keeps $150 clean back for tonight's upkeep, and that is all of it: type an amount to give it anyway.`) and $100 warned `Leaves $0 clean for $150 of upkeep tonight`; with 10 coke queued to a buyer, lying low asked first (`10 Coke to a banker with a habit, 53 owed by day 63.`). At 390px no horizontal scroll.

# The cook and the cut (#557, protocol 27 / view 17)

The builder accepts protocol 27 and view 17. View 17 adds two maps to `you`: `quality` (city → product → the quality of the lot held there, `World.Quality`, street and houses as one lot; a product with no units absent) and `room` (city → the units the stash there has free, `World.Free`). Without them the page could not say what a lot is, preview a cut, or hold a cook or a cut to the room as the TUI does; `TestViewCarriesTheLabsNumbers` pins both. No protocol method moved and nothing this edition read moved. The words are the TUI's (`ui/quality.go`, the chemist's hand in `ui/crew.go`), ported into the new pure module `src/lab.js`, and `src/app.js` draws them:

- **The lab.** A panel on the Market tab, `#lab`: the chemist's line (`Beaker cooks at quality 77, up to 158 a batch, ready in 3 days.`, ` The lab is here.` where the lab asset stands), the lots on their way (`Beaker is cooking 3 Meth in Eastside: quality 77, lands Day …`), and `Cook a batch` and `Cut stock`, each greyed with the TUI's refusal where it cannot open (lying low; nobody on the payroll can cook; nothing to cook; the stash here is empty).
- **The cook.** A product page (the products a chemist makes on the ladder here, the precursors a unit with the lab's discount, a connect's price, the stash), then the units: blank is a batch held to the room with what is on its way counted and to the till (`cookMax`), the live `Cost: $900 for 3, against $1,597 from a connect`, and the terms (`Precursors are $300.00 a unit, dirty, paid now; the lot lands in Eastside in 3 days at quality 77, Beaker's. A batch is 158.`). `cook` sends it; the toast is the TUI's `Beaker is cooking 3 Meth at quality 77, ready in 3 days. Cost $900.`
- **The cut.** A product page (each lot here with its stash, quality, the most and the price a unit added), then the percent added: blank is the most the product, the room and the till allow (`cutMax`), the live `After: 6 units at quality 55, sells at ×1.02, for $120`, and the terms with the chemist's hand. `cut` sends the ratio; the toast is the TUI's `Cut 3 Meth into 6: quality 77 → 55, sells at ×1.02. Beaker kept it at that. Cost $120.`
- **The lot and the hand.** The market's stock cell gives each lot's quality, amber under the street's. A chemist's crew card and details carry the TUI's hand: `Cooks · q 77 · 158 a batch` and `Cuts · keep 16 points` for the best, `Post · second to the best chemist` and why for another, and a candidate's `Cooks` row in the pool.

Checks:

- **`smoke.mjs`** now plays a Cook run (seed 41): it trades weed until there is cash for precursors and the chemist's wage, holds `lab.js`'s lines free of `undefined` and `NaN`, cooks through `cook` (blank reads a batch, `0` and letters are refused), ends days until the lot lands at the cook's quality in `you.quality`, then cuts it at the most through `cut`, the preview's units, quality and cost equal to the engine's `CutRecord`, and `you.quality` reading the cut.
- **Chromium (Playwright)**, no page errors, at 1280×900 and 390×844, on the same Cook run driven through `?test=1`: the chemist's card read `Cooks · q 77 · 158 a batch` and `Cuts · keep 16 points`; THE LAB read the chemist's line, with `Cut stock` greyed and `Nothing here to cut: the stash in Eastside is empty.`; the cook's product page listed Meth, 3 units showed the cost line, `0` stayed in the dialog with `Not a whole number.`, and Cook queued the batch with the TUI's toast and the lot on its way in the panel. After the days the meth row read its quality; the cut's number page, blank, previewed `6 units at quality 55, sells at ×1.02, for $120` with `Beaker's hand keeps 16 points of it.`, and Cut lowered `you.quality` with the TUI's toast. At 390px the page does not scroll sideways.

Found, not changed: a Cook who spends the starting cash on precursors on day 0 goes broke on day 2, the chemist's wage unpaid (the TUI's `cookMax` keeps no wages back either).

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

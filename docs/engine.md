# The engine is one session a front end drives

**The engine** (#293, `internal/engine`) is the game as one object: `engine.Session` assembles a run the one way, owns its world and its lifecycle, and publishes the day's events.
The TUI (`internal/ui`) and the harness (`internal/harness`) both play through it, and so will any other front end: a graphical client built in a real game engine, a script, a server that speaks to a client in another language. #293 is the epic and holds the whole design.
It lands in six phases, #296 to #301, and this file grows with each one.

**Phase 1, the session (#296).**
`engine.New(cfg)` builds every sim from the one `*content.Config` in `sim.Default`'s step order (`world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`, `docs/day-loop.md`), a `game.Clock` over them and an `events.Bus` the clock publishes on.
The session has no run until it is given one:

- `NewRun(seed, start)` is `sim.NewWorldWith` (a character, the hard DA, the daily, `docs/profile.md`).
- `Attach(w)` drives a world the caller built. The harness and the tests start theirs from `sim.NewWorld` on a config of their own.
- `Load(slot)` is `game.Load` through the sims' migration chain (`Set.Migrations`, `docs/saves.md`). A slot that does not load leaves the session's run as it was.
- `Save(slot)` is `game.Save`. Before a run it returns `ErrNoRun`.
- `EndDay()` is `Clock.EndDay` on the session's world: it returns the tick's events and publishes them. It is a no-op before a run and once the run is over.
- `Subscribe(h)` registers a handler for every event the session publishes, in the order the day emitted them, on the caller's goroutine.
- `World()` is the run, and `Config()` the tuning. `Rules()` and the commands are phase 2's (below). `Sims()` is the constructed `sim.Set` itself, for the harness and the tests that pin the sims; a front end never calls it (`TestUIActsThroughTheSession`).

The slot is the caller's: the TUI keeps `Model.slot` and passes it to `Load` and `Save`.

**Rulings.**

- The session sits above the clock. The sims still never call each other, and events stay their only channel (`docs/events.md`).
- It never reads the wall clock and never draws a seed: the caller passes the seed (`newSeed` and `Options.Seeds` stay in the TUI, #50, #292).
- It starts no goroutine. A transport that serves it to another process (phase 5, #300) lives outside it and runs under `-race`.
- A front end that stepped its own clock would have a second step order and a second migration chain to keep right, so **there is one assembly path**. `TestOneAssemblyPath` (`engine/session_test.go`) parses every non-test `.go` file outside `internal/engine` and `internal/sim` and fails on a call of `sim.Default` or `game.NewClock`. Tests are exempt: they pin the pieces the session is built from.

**What pins it.**
`TestSessionIsTheHandAssembledRun` plays the trader sixty days on seed 7 through the session and through a hand-assembled clock and finds the same events and the same world, byte for byte, and what `Subscribe` received is what `EndDay` returned.
`TestSessionSavesAndResumes` plays thirty days, saves, loads into a fresh session and plays thirty more, and gets the run that never stopped.
The move changed no number: the harness's runs are the same runs (`TestDeterministicForSeed`, `TestSeedDigest`, `TestMoneyCurve` and the difficulty ordering unchanged), and the TUI's tests and README captures pass as they were.

**Phase 2, the rules and the commands (#297).**
A front end reads the sims through `Rules` and acts on the run through the session's commands.
It never holds a sim, and it never calls a `World` method that changes the run.

- **`Session.Rules()`** (`engine/quotes.go`) is one interface per sim, `MarketRules`, `LogisticsRules`, `TerritoryRules`, `RivalsRules`, `CrewRules`, `HeatRules`, `LawRules` and `LaunderingRules`, each listing the read methods a front end calls on that sim: a cost, a price, the odds, a threshold, a preview, the tuning it explains itself with. The sims satisfy them as they are, so there is no forwarding layer and no second copy of a number. Every signature is in `game`, `content` and `events` types, never a sim package's. The lists hold what the TUI calls and nothing that steps, seeds, migrates or acts: `Laundering.Retire`, the one action a sim carried, is a command now. `logistics.Customs` and `logistics.Watched`, the two package functions the TUI read, gained methods of the same name on the sim so they read through `LogisticsRules`. The TUI keeps them as `Model.rules` (the old `m.set`).
- **The commands** (`engine/commands.go`) are one `Session` method per player action, 68 in all (beside them `HouseOffers`, the one query the buys by id needed), grouped by the market, the street, the crew, the road, the money, the law, the table and the endings. Each wraps the `World` method of the same name and never copies its rules. Where the `World` method takes a number the sims own, the command reads it off the sims and takes only what the player chose, so a front end can neither get it wrong nor name a price of its own:
  - `Hire(id)`: the crew's cap. `Investigate()`: its price. `PayOff(id)` and `Bail(id)`: the price for that member (and the loyalty a pay-off buys).
  - `Buy(supplier, product, qty, credit)`: the market's buy pressure off the tree.
  - `Cut(city, product, ratio)`: the market's most and cost, and the chemist's bonus and name. `Cook(city, product, units)` (`World.CookOrder`): the cost, days, quality and batch the chemist and the lab in that city give.
  - `BuyDeed(corner)`: the territory's price for the block. `BuyCheckpoint(route)`: the law's price, customs on a boat or plane edge and a checkpoint on the road, and its term. `BuyOffFrom(faction, units)`: units heads at the faction's price. `ScoutFaction(faction)`: the rivals' price.
  - `CallFavour()`: due when heat's response is on its way. `Vanish()`: what the owned tree does for it. `BuyUpgrade(id)`: the tree. `Retire()`: the account's terms.
  - The buys that took an offer take its id, and the engine finds the offer: `BuyFront(id)` in `Laundering.Offers`, `BuyAsset(id)` through `Laundering.AssetOffer`, `BuyHouse(id)` in `Session.HouseOffers()` (the city file's houses as offers; the TUI built them itself before), `Invest(front, levels)` through `Laundering.Levels`. Nothing on offer under the id is `ErrNoOffer`.
  - Where a lookup finds nothing (no such corner, member or faction), the command passes a zero price and the `World` method refuses the id before it touches money, with its own error.
  - A command needs a run: before `NewRun`, `Load` or `Attach` it panics, as a `World` method on a nil world would. The protocol (phase 5, #300) answers that case before it calls one.
- **What stays a read.** The TUI still reads the world directly (`m.w.Stock`, `m.w.CityName`, fields for display). Phase 4 (#299) gives front ends a versioned view in its place. The harness is a scripted player that pins the sims, and it still drives the `World` itself.

**Rulings.**

- The issue asked for a cross-check of `ui/keys.go` against the commands. A key opens a dialog and the action runs when the dialog confirms, so no table maps a key to a command. The guard instead checks the thing the cross-check was for, that no action is the TUI's alone. `TestUIActsThroughTheSession` (`engine/frontend_test.go`) type-checks `internal/ui` with `go/types` and fails on:
  - an import of `internal/sim` or any sim package
  - a call of `Session.Sims`
  - any `*game.World` method, called or taken as a method value, that is not in `worldReads`, the ninety-odd methods the TUI calls that only read
  
  An action called from the TUI fails by name. The fix is a command (or a line in `worldReads` for a method that only reads). A name in `worldReads` the TUI no longer calls fails too, so the list stays tight. `demo.go` is exempt: the title's demo builds a throwaway world to draw a scene over and never touches the run. A planted `m.w.Hire` in `crew.go` fails it by name.
- `TestCommandsChargeWhatTheRulesQuote` (`engine/commands_test.go`) pins that a command charges what `Rules` quotes: the investigation's cost, the block's price out of clean cash, and the scouting's cost once the rival is on the map. `TestBuyByIDRefusesWhatIsNotOffered` pins `ErrNoOffer` for the four buys by id and that every house offer carries an id and a price.
- No number moved. Every UI test, including the README captures, passes with the TUI acting through the commands.

**Phase 3, the facts (#298).**
The TUI used to work out some things it shows, and a second front end would have had to copy them or drift: what needs you this morning, the doors ahead, whether a day stops a fast-forward, what a price is doing.
The engine decides them now and hands them over as values.
The words and colours stay the front end's (the TUI's are in `docs/copy.md`'s voice).

- **Alerts** (`engine/alerts.go`). `Session.Alerts()` returns `[]Alert`, loudest first, in the order of the `AlertKind` constants: `talking`, `contract_due`, `debt_due`, `heat` (where you are, at or over the patrol line), `task_force`, `investigation` (#343: the open investigation, keyed by its city, target and opening night so a fast-forward stops the morning it opens; `city`, `target` corner | product | house with the `corner`, `product` or `house` id, `days` to the hit, 1 tonight), `float` (only while `FloatMatters`: a front, or a route with its dial on), `wages`, `crew_line`, `skim`, `idle_corner`, `gate`, `house_known`, `da_race`, `retire`, `favour` and `reign`.
  - Each alert carries a `Key`, its identity from one morning to the next. The keys are the strings the TUI used before (`contract 12 due today`, `debt street due 41`, `unlock:front:laundromat`, `known h1`, `heat in Eastside over the patrol line`, …), so a fast-forward stops on exactly what it did.
  - The kind's fields are ids and numbers, never words: `City`, `Contract`, `Supplier`, `House`, `Due`, `Amount`, `Have`, `Heat`, `Line`, `Days`, `Count`, `Ready`, `Level`, `Gate`.
  - The TUI's `alerts()` words each one (`alertOf`), and `debtAlerts` is `alertsOf(engine.AlertDebtDue)`.
  - The crew trouble (#345) is three kinds, read off the world with no sim state moved.
    `crew_line` is a member within `crew.toml`'s `alert_margin` (5) over the line ahead of them, or near enough that tonight's drift takes them over it: the skim line (a lieutenant's flip line) while they are over it, then the walk (`quit_threshold`).
    It names the member (`member`, an id), the line (`cross`: `skim`, `flip` or `walk`; `line`), the loyalty over it (`gap`) and the days to it at tonight's drift (`days`, 0 while loyalty is not falling), and is keyed `crew 7 near the walk line`, so a fast-forward stops once a member a line.
    The drift is `crew.Sim.Drift`, the read of a quiet night's move (the pay dial, the unpaid penalty when the wages are over the dirty cash, greed, the danger behind the enforcers) that shares its arithmetic with the night's (`memberDrift`), so `TestSeedDigest` did not move.
    The informant line is nobody's alert: informants stay silent (`docs/snitching.md`).
    `skim` is up while the crew screen's warning is (`LastSkim` within `suspect_days`), keyed once, so the skim stops a fast-forward the morning it is first suspected and not every night it goes on (`day` is when the money went).
    `idle_corner` is a corner you hold that nobody works this morning (`corner`, `city`), with the days before it drifts (`days`, territory's `DriftDays` less `Idle`), keyed per corner.
- **Gates** (#148). `engine.Gate` (`Kind`, `ID`, `Name`, `Line`, `Vouch`, `Clean`) with `Peak`, `ToGo` and `Near` (under `GateNear`, 0.5, of the line). `Session.GatesAhead()` and `NextGates()` find them. `Session.FrontOffers()` and `AssetOffers()` are the offers you do not own, the asset list less any the task force found; the TUI's `frontRows` and `assetRows` are those. The TUI keeps the words: `gateText`, `gateThe`, `gateToGo`.
- **Stops** (`engine/stops.go`).
  - `StopsOn(e)` decides whether an event stops a fast-forward, including each kind's conditions: an `Enforcement` past a patrol; a `CornerStruck` unless it is a war night that held; a `RivalBoosted` that failed; a `CornerTaken` from you; a `CornerLost` of yours that nobody worked (`idle`, #345); a `CrewShot` dead and yours; `PressureShifted` or `ReputationShifted` up; and a fixed list of kinds that always stop.
  - `Session.Stop(evs, before)` weighs a day in this order: a new stage, a card, an alert whose key the morning before lacked, the first event `StopsOn` names.
  - `Session.FastForward(days, after)` is the loop. It calls `EndDay` a day at a time, calls `after` with each day's events before weighing the day (the TUI saves and journals there, `dayEnded`), and returns the days run, the `Stop` (`stage`, `card`, `alert`, `event`, `over`, `cap`) and the stopping day's events.
  - The TUI's `stopEvent` is now only the words, and `stopWhy` words a `Stop`.
  - The TUI's per-day flash of enforcement for the bust scene is now taken from the day's events in `dayEnded`, in the order the bus published them. The bus subscriber (`onEvent`) is gone, because it could not be reset between days inside the engine's loop.
- **Price facts** (`engine/prices.go`). `engine.Facts(p)` and `FactsAt(p, unit)` return `PriceFacts` (`Unit`, `Delta`, `Lo`, `Hi`, `Margin`): the day's change, the range of the history, and the margin over a unit, with no unit where the city's supplier does not sell the product. The TUI's `factsAt` copies them into its own `priceFacts`, which keeps the lieutenant's markup and the rendering. `Session.MaxBuy(supplier, product, credit)` (`BuyRoom`) and `Session.RestockPlan(city, days)` are the buy's reads (#356): the most a buy takes and the stash it lands in, and the restock's lines at the supply contracts' float (`market.Sim.Float`).

**What pins it.**

- `TestFastForwardIsTheDayLoop` plays four seeds through `FastForward` and through `EndDay` weighed by `Stop` by hand. It finds the same days, the same stop and the same stopping events, and `after` called once a day with a whole day.
- `TestStopsOnTheReadings` pins each condition, both ways.
- `TestPriceFacts` pins the change, the range, the margin and `NoSupply`.
- `TestAlertsAreKeyedOnce` plays eighty aggressive days and finds no empty or repeated key on any morning.
- `TestEveryStopHasWords` (`ui/stops_test.go`) walks `events.All`: every kind the engine stops on at its zero value has words in `stopEvent`, and nothing it does not stop on is worded. `stopEvent` falls back to the kind's name, so a stop is never silent.
- Every existing fast-forward, alert, unlock, debt and delta test in the TUI passes unchanged, and no number moved.

**Phase 4, the view (#299).**
`Session.View()` (`engine/view.go`) returns `engine.View`, a snapshot of what the player can see.
It is what a front end in another process draws from, and it holds no pointer into the world, so it can be kept, compared, changed and sent as JSON.

- **What it holds.** `version`, `seed`, `day` and `over` (how the run ended), then:
  - `you`: the city you're in, dirty, clean and offshore cash, net worth, peak cash, lie-low, the tier and its name, the pay and launder dials by name, the DA's evidence, fear, respect and notoriety, your street stock by city, the upgrades owned, the quiet days, the character and the hard DA.
  - `cities`: heat, pressure, goodwill, the police's next rung as the file knows it with how sure the word is today (`response_sure`, #355), the `ladder` (#355: a rung each as `heat.Sim.Rungs` folds it, its `line`, the `stock_loss` and `cash_loss` shares, the `pages` it files on a day you sold, a patrol's `cap` before the chief's temper, which is intel, and its `cap_days`), every product (price, supplier price, demand, shock, history and its `PriceFacts`) and every corner (the cell, demand, owner, faction, runner, enforcer, since, deed).
  - `crew`: role, age, skill, loyalty, wage, the city a lieutenant runs, the post, jailed, and a lieutenant's personality once the report has named it.
  - `pool` (#332): who is looking for work, as the crew screen lists them: role, age, skill, loyalty, wage, `carry` (the sell capacity they add, which `crew` carries too) and `fee`, the id `hire` takes. There is no personality, since none has been observed, and no former faction, which the TUI does not show either.
  - `contracts` (#332): the buyers' contracts still somebody's business (offered or accepted), as the buyers screen shows them: the buyer's name and pitch, the city, the product, the units and those delivered, the premium, the street price it was offered against, the penalties, the status, and the last days to take it and to deliver. Each has the id that `accept_contract`, `decline_contract` and `deliver` take.
  - `offers` (#332): the deals the factions have on the table: the faction, the kind, the terms as the commands spell them, and the day it lapses, with the id that `accept` and `decline` take.
  - `upgrades` (#332): the whole tree in `upgrades.toml`'s order, each node with its branch, description, cost, pool (`clean`), prerequisites and `state`: `owned`, `available` (its prerequisites owned) or `locked`. The id is what `buy_upgrade` takes.
  - `routes` open to you: the dial by name, closed, the driver, and the risk the file holds.
  - `shipments`, `connects` (who sells what where, today's price for what they will sell you now, the day's cap, the lot, credit, the relationship, the debt and when it is due; the temper, which the TUI shows), `houses` (stock, guard, whether the police know it) and `fronts` (level, frozen, washed).
  - `factions`: leader, alive, arrival, corners held, trust, war, and, from the file only, the temper (`?` until known), the heads as a band, the last read of the books and the next move.
  - `law`: the chief, the temper the file knows, the DA and the stance, the next election, and (#355) the `arrest_line` (`heat.Sim.EvidenceArrest`), the `exposure_line` (`ExposureLine`) and the fronts' `cover` (`Cover`).
  - `card`: the one waiting, with its choices: each a `label` and a `preview`, what it does as chips (`text`, `tone`: `gain`, `cost`, `line` or `note`; `engine.ChoiceChips`, #358, `docs/dilemmas.md`).
  - `report`: the morning report's sections as lines, and `flow` (#351): the night's cash flow, `opening` and `closing` by pile (`dirty`, `clean`), one line a category in `game.FlowCats`' order (`cat`, `label`, the signed `dirty` and `clean`, and `big` past headlines.toml `[flow] big_share` of the opening) and the `net`. Opening plus the lines is the closing, pile by pile (`docs/market-and-journal.md`).
  - `alerts`: phase 3's typed alerts.
  
  Dials go out as names (`fair`, `normal`), never as ints. Every JSON key is snake_case, and optional ones are `omitempty`.
- **What it leaves out.** What the player does not know is not there:
  - a faction's true temper, heads, chest, grudge, arrears and the corner it is eyeing
  - the chief's true temper and a route's true risk
  - a planted fact's author
  - who on the payroll is informing, and a member's greed and nerve
  - the informant's leak count (the "somebody is talking" alert is the tell)
- **Versioning.** `engine.ViewVersion` (2 since #332 added the pool, the contracts, the offers and the tree; 3 since #345 added the crew trouble's alert fields; 4 since #358 made a card's choices a label and a preview; 5 since #351 added the report's cash flow, `report.flow`; 6 since #355 added the ladder, the word's sureness and the law's lines; 8 since #343 added the investigation alert's `target` and `product`) is the shape of `View` and moves when a field is added, renamed, retyped or dropped. It is never tied to the save's `game.SchemaVersion`: the world stays free to change shape, and the view is the contract. `TestViewShapeIsPinned` walks the type by reflection into one line per field, its JSON path and its Go kind (`.cities[].products[].facts.margin float64`), and compares that with `engine/testdata/view_shape.txt` (301 lines, headed `version 8`).
  - A shape change that keeps the number fails with the lines that moved.
  - With the number moved, `go test ./internal/engine -run TestViewShapeIsPinned -update` writes the new shape.
  - The file pins shape and never values, so it reads the same on amd64 CI and an arm64 machine, where fused multiply-add moves floats (the reason `TestNoUnlockIsTheOldRun` hashes nothing a float writes).

**Rulings.**

- The issue asked the TUI to read the view where that was a straight swap. It doesn't, deliberately. The TUI runs in the same process and redraws on every key; building a whole snapshot per frame to read a handful of fields would allocate the world's size each keypress for nothing, and the view exists for front ends that cannot hold a Go pointer. The TUI's direct reads are already confined to methods that only read (`TestUIActsThroughTheSession`'s `worldReads`, phase 2), and the rival panels to the file (`TestPanelsReadTheFile`). Those two lists are the remaining direct reads, and phase 5's protocol serves the view.

**What pins it.**

- `TestViewShapeIsPinned` (above).
- `TestViewRoundTripsJSON`: forty days of the informed player's run give a view that is byte-identical after a trip through JSON, and the view before a run carries its version.
- `TestViewHoldsNothingOfTheWorld`: every history, stock map, report line and corner list in the view is overwritten, and the world's JSON is unchanged.
- `TestViewReadsTheFile`: sixty days of the informed player (two of the three factions' tempers in the file by then). Every faction's temper and heads band, and the chief's temper, equal `game.Known`'s. It also type-checks the engine and fails on any read in `view.go` of the truths above; a planted `r.Muscle` fails it by name.
- `TestViewHasNoNull` (#333): a list or map in the view is never `null`: not before a run, not on day 0, not forty days in. `Session.View` ends with `noNulls`, which walks the view's own types by reflection and makes every nil slice and map empty, so a list added later is covered too. Without it, `encoding/json` wrote a nil slice as `null`, and a list came as `[]` one morning and `null` the next (`houses`, `shipments`, `fronts`, `alerts` and `you.upgrades` did, on a played run). A pointer that is absent (`over`, `card`, `books`) stays absent: each is `omitempty`, so none is ever `null` either. It is a change of value, not of shape, so `ViewVersion` stays 1, and a client that guarded the nulls keeps working.
- `TestViewCarriesThePolice` (#355): sixty days of the informed player and a cop paid. Every city's ladder is `heat.Sim.Rungs` rung for rung, the law's arrest line, exposure line and cover are the sim's, and the police's next move and its sureness are the file's.
- `TestViewCarriesWhatTheScreensList` (#332): sixty days of the informed player. The view's pool is the world's candidates, and its contracts are the live ones. Its offers are the world's, and its tree is every node, each in the state the world gives it. On a fresh run, pool[0] hired by the view's id lands on the payroll and leaves the pool, and an available node bought by its id reads `owned`. `protocol.TestHireFromTheView` does the hire over the wire, seeing only the view.

**Phase 5, the wire (#300).**
`cmd/kingpind` serves a session to a front end in another process.
`internal/protocol` is the protocol, `internal/protocol/schema.json` its contract, and `cmd/kingpin-client` the reference client.

- **Framing.** JSON-RPC 2.0, one message per line, over stdin and stdout (`protocol.Serve` takes any reader and writer).
  - A request has `"jsonrpc": "2.0"`, an `id` and a `method`, with `params` **by position** (a JSON array; empty or absent for none).
  - The server answers every request. Before the response it sends the notifications the call caused: every `event` the day published (`{"kind", "day", "payload"}`, the kind being the event's stable `Kind()`), then a `view` (the whole `engine.View`) after any call that may have changed the run.
  - A client that waits for its response has already read everything the call caused.
  - The server is one loop on one goroutine: read a line, run it, write and flush. It holds no lock and starts no goroutine, and `TestNoGoroutineInTheTree` walks the package like the rest of `internal/`.
- **The methods, 85 in all** (and the 148 quotes #325 added, below).
  - **68 commands.** Each session command is served under its name in snake_case (`buy`, `place_sell`, `buy_checkpoint`, `scout_faction`, …) by reflection over `engine.Session` (`protocol.commands`), with its parameters in order. A dial goes in by name (`"aggressive"`, `"fair"`, `"push"`), refused with the names listed when it matches none. Terms go as an object (`{"days", "per_day", "corners", "route", "units"}`). The result is the command's value, or null.
  - **11 queries.** `view`, `alerts`, `gates_ahead`, `next_gates`, `front_offers`, `asset_offers`, `house_offers`, `float_matters`, `export_save`, and the buy's two (#356): `max_buy [supplier, product, credit]` returns `{max, held, capacity}` (`engine.BuyRoom`: `World.MaxBuy`, the most a buy takes with no refusal for the cash, the room or the connect's day, and the stash it lands in), and `restock_plan [city, days]` returns the lines that top the stash up to days of demand (`[]game.RestockLine`, never null; see `docs/cart.md`).
  - **6 lifecycle methods, by hand** (`import_save` among them, #327).
    - `new_run [seed, character, hard_da]`: the seed is the client's, and the server never draws one. It returns the view.
    - `load [slot]` returns the view. `save [slot]`.
    - `end_day []` returns `{day, events}`, with the events already sent as notifications.
    - `fast_forward [days]` returns `{ran, day, stop, alert?, event?}`, the days weighed server-side by `Session.FastForward`.
  - Every method but `new_run` and `load` needs a run.
  - **Not on the wire,** each with its reason in `protocol.unserved`: `Attach`, `Config`, `Sims`, `World`, `Subscribe` and `Stop`. `Rules` is there too, served rule by rule as the quotes (#325, below). `TestEverySessionMethodIsClassed` fails on a session method in none of the three lists, so a new command is served, or refused, on purpose.
- **Errors.** JSON-RPC's codes: `-32700` not JSON, `-32600` not a request, `-32601` no such method, `-32602` params that do not fit, `-32603` the engine panicked (recovered; the message says so). The game adds three:
  - `-32000` **refused**: a move the rules do not allow, the message being the game's own words (`nothing on offer by that name`). `protocol.Refused(err)` tells it apart.
  - `-32001` **no run**: call `new_run` or `load` first.
  - `-32002` **no room** (#356): the refusal for the stash's room (`game.RoomError`, which `errors.Is(err, game.ErrNoRoom)` matches), its message the game's words (`can only hold 48 more units in Eastside`) and its `data` `{"free", "city"}`, so a client offers what fits rather than a bare no. `protocol.Refused` counts it as a refusal. `TestNoRoomIsTyped` pins the code, the data and `max_buy`.
- **Encoding.** The view is snake_case with dials by name (phase 4). A result or an event payload is the Go value as `encoding/json` writes it: Go field names, and a dial as the int a save holds. The schema marks it `integer` with `x-names` in order. `protocol.EventJSON` is the one encoding of an event, so a client in the same process can compare its events with the wire's byte for byte.
- **The schema.** `protocol.Schema()` generates a JSON Schema document (draft 2020-12) from the Go types: the protocol and view versions, the framing, the error codes, every method's `params` (`prefixItems`, a dial as its enum of names) and `result`, the two notifications, every event kind's payload under `events`, and 209 named types under `$defs`. It is checked in as `internal/protocol/schema.json` (about 250 KB). `TestSchemaIsCurrent` fails when the file is stale, and `go test ./internal/protocol -run TestSchemaIsCurrent -update` rewrites it. `protocol.Version` (2 since the quotes, #325; 3 since the saves as bytes, #327; 4 since `max_buy`, `restock_plan` and `no_room`, #356) moves with the methods; the view keeps `engine.ViewVersion`.
- **The reference client.** `protocol.Play(c, seed, days)` plays a run through the protocol alone. Each morning it reads the view, answers a card with its first choice, spends 60% of the dirty cash across what the street connect where it stands sells, puts everything it holds on the street at the aggressive dial, and ends the day, until the run ends. `cmd/kingpin-client -server <kingpind> -seed 7` starts `kingpind` and prints every event as it arrived and how the run ended. On seed 7 that is `indicted` on day 30, after 718 events.
  - One bug found on the way is now part of the client: it decodes each view into a fresh value. Decoded over the last one, a field the new view omits as empty (an answered `card`) would keep its old value.

**What pins it.**

- `TestProtocolIsTheSession` plays the reference game twice: on a session directly (an in-process client that calls the session's methods and encodes its events with `EventJSON`), and through a server in the same process. It finds every event byte-identical and the same last view, requires an ending, and checks that the last `view` notification is the last day's.
- `TestOverStdio` builds `cmd/kingpind`, plays the same game over a real process's stdin and stdout, and gets the same bytes as the direct run. `-short` skips it.
- `TestErrors` pins every code: no run, no method, wrong count, a dial by a wrong name (the message lists the right ones), refused (`travel` nowhere, `buy_front` of nothing), not JSON, not a request, and params that are not an array.
- `TestEverySessionMethodIsClassed` and `TestSchemaIsCurrent` (above).
- The day-0 view (before the first morning, `World.Report` still nil) is in phase 4's `TestViewRoundTripsJSON`; the protocol found the nil.

**The quotes (#325).**
A front end in another process prices a move before it makes it, with the numbers the TUI reads.
`internal/protocol/rules.go` serves every method of `engine.Rules` as `rules.<sim>.<method>` in snake_case: `rules.market.capacity`, `rules.crew.investigate_cost`, `rules.rivals.odds_on_at`.
That is 148 methods (#350 added `rules.heat.exposure_line`, #355 `rules.heat.rungs`), which makes 233 on the wire with the 85 methods above.
A quote needs a run, changes nothing and sends nothing but its answer: no `view` follows it.

- **The world is the server's.** A rule's `*game.World` is the run's and never a parameter.
- **A thing of the world goes by its id** and is resolved against the run (`protocol.ruleParams`):
  - a corner (`game.Corner` and `*game.Corner`), a faction (`*game.RivalState`), a supplier, a city, a house;
  - a deed, by its corner, refused on one without a deed;
  - a front, owned or on offer (an offer is the front as bought, level 0);
  - a crew member, on the payroll or in the pool;
  - a contract;
  - a route (`content.RouteConfig`).
  
  An id the run doesn't have is `-32602`, naming the parameter.
- **A deal goes as an object**, `{"kind", "terms"}` (`protocol.DealParams`, the terms as the commands spell them). A dial goes by name, as the commands' do.
- **A day is today or tomorrow**, the two the TUI asks about. Any other is `-32602`: `rules.logistics.watched` asked of a later day would read the task force's watch off the run before the run has told the player.
- **Results.** A `(value, ok)` answer is the value, or `-32000` refused when not ok (`rules.laundering.asset_offer` of nothing). Two values go as an object named after the rule's results (`rules.territory.tax_due` is `{"corners", "amount"}`). A result that would carry the truth or a pointer into the run goes by id (`protocol.ruleResults`): a corner, a city, the allies as faction ids, `routes_open` as route ids.
- **Not on the wire** (`protocol.unservedRules`, each with its reason): `Logistics.Route`. A route's config holds its true risk, and the view carries the route with the risk the file knows.
- **The names are quotes.go's.** Reflection can't see a parameter's name, so `internal/protocol/rules_names.go` is generated from `internal/engine/quotes.go`'s declarations (`go test ./internal/protocol -run TestRuleNamesAreCurrent -update`). A world thing is named for what it is (`corner`, `faction`, `member`), a dial for its type (`dial`, `force`, `pay`), and anything else by the name quotes.go gives it. Renaming one there renames it on the wire.
- **What the wire may say.** The ruling is parity with the TUI: `Rules` holds what the TUI reads. The truths it must not read (`TestPanelsReadTheFile`'s list: a faction's muscle and personality, the chief's, a route's risk, a planted fact) are kept out of the interfaces, and a rule that needs muscle (`odds_on_at`, `push_odds_at`, `defence_at`) takes the muscle the client read off the view's file.

**What pins it.**

- `TestEveryRuleIsClassed`: every method of every `Rules` interface is served or in `unservedRules`, and every one has its names.
- `TestRuleNamesAreCurrent`: `rules_names.go` is `quotes.go`'s names.
- `TestNoTruthOnTheWire` walks every quote's result type and fails on a `game.World`, `game.RivalState`, `game.Chief`, `game.Fact` or `content.RouteConfig` anywhere inside.
- `TestEveryQuoteIsTheRules` runs sixty days of the boss on seed 7 and lays one of every thing a rule takes by id. It asks all 148 quotes over the wire and compares each answer with `Session.Rules()` asked in the process. The run's JSON is byte-identical before and after.
- `TestQuoteRefusals` covers:
  - an unknown corner, faction or member;
  - a day before today or after tomorrow;
  - a dial by a wrong name;
  - an asset not on offer;
  - a quote with too many params;
  - the unserved route;
  - a quote before any run.
- `TestQuotedIsCharged` quotes the investigation and a block over the wire, makes both moves over the wire, and checks each charges its quote.

**Phase 6, the cues (#301).**
A graphical front end draws the view and moves its sprites on the day's events.
`engine.CueOf(e)` (`engine/cues.go`) reads an event as that movement, in ids and never in words.
It returns a `Cue` whose `Kind` is one of 17, with the ids that kind needs (`city`, `corner`, `house`, `route`, `shipment`, `member`, `faction`, `asset`, `product`, `units`), `from` and `to`, `level`, `phase` and `dead`:

- `corner_flip` carries the owners before and after. A strike that took the corner is a flip from `rival` to `player`; one that held is a `strike`. A corner lost carries its old owner and why (`idle` or `crackdown`).
- `shipment` carries the route, both cities, the shipment's id and the phase (`sent`, `landed`, `seized`).
- The crew cues (`crew_joined`, `crew_left`, `crew_down`, `crew_back`) carry the member's id.

The protocol sends the cue with each event it applies to (`event.params.cue`).
`cmd/kingpin-client` prints a `cue` line for every animated event, which is the check that the events are enough to animate.

**The audit.**
Every kind in `events.All` is decided in one table, `engine.cueKinds`: it gives a cue, or it is the report's and the journal's alone.
`TestEveryKindIsCuedOrNot` fails on a kind whose `CueOf` disagrees with the table, so a new kind is decided when it is added.
The table below is generated from it (`TestCueTableIsCurrent`; `go test ./internal/engine -run TestCueTableIsCurrent -update` rewrites it):

<!-- cues:begin (generated by TestCueTableIsCurrent -update) -->
| Cue | From the events |
|---|---|
| `corner_claimed` | `CornerClaimed` |
| `corner_flip` | `CornerLost`, `CornerTaken`, `RivalAbandoned`, `RivalRaided` |
| `crew_back` | `CrewBailed`, `CrewRecovered`, `CrewReleased` |
| `crew_down` | `CrewArrested`, `CrewShot` |
| `crew_joined` | `CrewHired` |
| `crew_left` | `CrewDefected`, `CrewFired`, `CrewQuit`, `CrewRetired`, `LieutenantWalked` |
| `market` | `PriceShock` |
| `overdose` | `Overdose` |
| `police` | `Enforcement`, `HouseRaided` |
| `property` | `DeedBought`, `DeedSeized`, `HouseBought`, `HouseLost` |
| `rival_move` | `FactionPushed`, `RivalBoosted`, `RivalEyeing`, `RivalMovedIn`, `RivalPushed` |
| `robbery` | `CornerRobbed`, `HouseRobbed` |
| `run` | `GameOver`, `ReignBegan`, `ReignBroken` |
| `sale` | `PlayerSold` |
| `shipment` | `ShipmentArrived`, `ShipmentSeized`, `ShipmentSent` |
| `strike` | `CornerStruck` |
| `task_force` | `AssetSeized`, `TaskForceFormed`, `TunnelFound` |

The report's and the journal's alone, no cue (97): `AssetBought`, `AssetFrozen`, `AssetUpkeepPaid`, `BribeAccepted`, `BribeBackfired`, `BribeRefused`, `CampaignBacked`, `CampaignHedged`, `CampaignLost`, `CashLaundered`, `CheckpointBought`, `ChiefReplaced`, `CityFunded`, `ClaimDeterred`, `ContractAccepted`, `ContractDelivered`, `ContractExpired`, `ContractFailed`, `ContractOffered`, `CookOrdered`, `Cooked`, `CreditTaken`, `CrewPaid`, `CrewPaidOff`, `CrewPoached`, `CrewSkimmed`, `CrewTurnedInformant`, `DAElected`, `DayEnded`, `DealAccepted`, `DealBroken`, `DealEnded`, `DealOffered`, `DealRefused`, `DebtLate`, `DebtPaid`, `DeedRent`, `DeedsBought`, `DilemmaAnswered`, `DilemmaDrawn`, `FallGuyBurned`, `FrontAudited`, `FrontBought`, `FrontFrozen`, `FrontGrew`, `FrontInvested`, `Headline`, `HeatChanged`, `HouseCompromised`, `Incident`, `IntelFalse`, `IntelGained`, `InvestigationClosed`, `InvestigationOpened`, `InvestigationRun`, `KinLooking`, `LaidLow`, `LeadFound`, `LeadsFiled`, `LieutenantActed`, `LieutenantFlipped`, `OfficialsCold`, `PlayerUndercut`, `PoliceTipped`, `PressureShifted`, `PriceMove`, `RaidFellThrough`, `RentPaid`, `ReputationShifted`, `Reserved`, `RivalAbsorbed`, `RivalLeaderArrested`, `RivalMusclePoached`, `RivalOutbid`, `RivalScouted`, `RivalTippedPolice`, `RivalUndercut`, `SpyFound`, `SpyPlanted`, `StandingShort`, `StockCut`, `StockMoved`, `SupplierBought`, `SupplierCollected`, `SupplierFrozen`, `SupplierWarned`, `SupplyBought`, `SupplyShort`, `Taxed`, `TierReached`, `TributePaid`, `TrustSpread`, `Unlocked`, `UpgradeBought`, `WarEnded`, `WarEscalated`, `WholesaleBought`.
<!-- cues:end -->

**The ids the events lacked.**
The audit found four crew events that named the member but not their id: `CrewHired`, `CrewQuit`, `CrewFired` and `CrewDefected`.
Each gained `ID` (additive; zero reads as the event before it), set where the crew sim emits them (`crew.go` `roster`, `loyalty.go`).
Every other animated kind already carried its ids.
No headline reads the new field, and no pinned number moves.

**The TUI's derivation, gone.**
`ui.mapFlips` (the corners the map's scene burns, #158) now reads the cues and keeps the flips between you and a rival: `player` to `rival`, or `rival` to `player`.
A corner going back to the street is a flip too, and the scene leaves it out as it always has.

**What pins it.**

- `TestEveryKindIsCuedOrNot`.
- `TestCueTableIsCurrent`.
- `TestCueKindsIsEveryCue`: `engine.CueKinds()` (#328) lists the 17 cues, sorted. A front end holds its animation table to that list, as the web client's `TestWebClient` does (`docs/web.md`).
- `TestCuesCarryIDs` plays 120 days of the boss. Every cue has a day. Every corner cue names a corner on the map, and every flip changes hands. Every shipment names its route, both cities and its id. Every crew cue names a member. Every police cue names a city and a level. The run sees at least one claim, shipment, hire, sale and police cue.
- `TestProtocolIsTheSession` and `TestOverStdio` (phase 5) carry the cues in the event bytes they compare.

**The WebSocket transport (#326).**
`kingpind -listen 127.0.0.1:7777` serves the same protocol over WebSocket (RFC 6455) instead of stdio.
It is for a browser client, or a game engine that would rather open a socket than start a process.

- **Framing.** One JSON-RPC message per text frame. The methods, the notifications, the error codes and the order are stdio's: a call's `event`s and its `view` go out as frames before its response.
  - `protocol.ServeWS` hands each message to `Server.Handle`, the loop `Serve` runs on stdio. The transport is framing and nothing else, so there is still one implementation of the protocol.
  - The client end reads and writes lines (`protocol.WS` is an `io.Reader` and an `io.Writer`), so `RPCClient` and the reference game play over it unchanged.
  - `kingpin-client -ws ws://127.0.0.1:7777/` plays against a `kingpind` already listening.
- **What the transport takes.**
  - Text frames, fragmented or whole, up to 16 MiB a message (stdio's line cap).
  - A ping is answered with a pong, and a close is echoed and ends the session.
  - It refuses, closing with the code for why:
    - `1002` for an unmasked client frame, a reserved bit, an unknown opcode, a continuation of nothing, or a fragmented or long control frame;
    - `1003` for a binary frame;
    - `1007` for a message that isn't UTF-8;
    - `1009` for a message over the cap.
- **The handshake.** `protocol.AcceptWS` takes a `GET /` upgrade at version 13 with a key. Anything else gets its HTTP error: `405`, `404`, `400`, or `426` for another version.
  - A request carrying an `Origin` is refused with `403` unless the page is on this machine (`localhost` or a loopback IP). Any website the player visits could otherwise open a socket to `localhost` and drive the game.
  - A client gets ten seconds to finish the handshake.
- **Loopback only.** `-listen` refuses any address but a loopback one: the protocol has no authentication, and remote play is not what it is for. An empty host is `127.0.0.1`, and port 0 picks one. `kingpind` prints the URL to connect to (`ws://127.0.0.1:PORT/`) on stdout, then runs until it is killed.
- **One connection at a time, a session each.** The standard library alone, no dependency: `net.Listen`, `http.ReadRequest` for the handshake, and the frames by hand in `internal/protocol/ws.go`.
  - `kingpind` accepts a connection, serves it on its own goroutine until it hangs up, then accepts the next. A second client waits.
  - This keeps the tree free of goroutines (`TestNoGoroutineInTheTree` walks `internal/protocol` too): there is no `net/http` server spawning one per connection, and nothing is shared that would want a lock. Each connection gets a fresh session and its own tuning, loaded when it arrives, with no run until it calls `new_run` or `load`.
  - A game played by one client needs nothing more. Serving several at once is a later change, and it would bring a lock and put the race detector's guard on the table.

**What pins it.**

- `TestOverWebSocket` builds `kingpind` and runs it with `-listen 127.0.0.1:0`. It plays the reference game on seed 7 over a real socket and gets `TestProtocolIsTheSession`'s direct run byte for byte: every event and the last view. It closes the connection and gets the server's close back. A second client then gets a session of its own, with no run.
- `TestListenIsLoopbackOnly`: `0.0.0.0`, `[::]` and a public address are refused.
- `TestHandshake`: RFC 6455's sample key gets RFC 6455's accept, a page on `localhost` or `127.0.0.1` may connect, and each bad request gets its status.
- `TestFrames`: fragments with a ping between them read as one message and the ping is answered; a 16-bit length reads; a close is echoed; each refused frame closes with its code.
- `TestServeWS`: the frames come in stdio's order.
- `TestOverStdio` is unchanged: stdio is still the default.
- Checked by hand against an independent client, Python's `websockets`: the handshake with a localhost Origin, a 200 KB message (a 64-bit length), ping and pong, and a clean close.

**Embedding (#327).**
A front end can run the engine inside its own process.
The protocol is still the contract: each embedding exposes one call, a request line in and the lines it produced out (`protocol.Server.Handle`, the loop stdio and WebSocket run).
There is no second API.

- **WebAssembly** (`cmd/kingpin-wasm`, `js && wasm`), for a browser or a web game engine: `GOOS=js GOARCH=wasm go build -o kingpin.wasm ./cmd/kingpin-wasm`, run with Go's `wasm_exec.js` (`$(go env GOROOT)/lib/wasm`). The module is about 12 MB.
  - It sets one global, `kingpin`: `kingpin.protocol` and `kingpin.view` are the versions, and `kingpin.open()` is a session whose `handle(line)` returns the answers as an array of strings, notifications first.
  - Each `open` is a session of its own, with its own tuning, like each `kingpind` connection.
  - A `handle` that isn't given one string returns an `Error`.
- **A C shared library** (`cmd/libkingpin`, `cgo`), for a native game engine (Godot, Unity, Unreal): `go build -buildmode=c-shared -o libkingpin.so ./cmd/libkingpin` writes `libkingpin.h` beside the library. The calls:
  - `kingpin_open()` returns a handle, `> 0`.
  - `kingpin_handle(h, line)` returns the answer lines, each ending in `\n`, in memory the host passes to `kingpin_free`.
  - `kingpin_close(h)` ends the session.
  - `kingpin_protocol()` and `kingpin_view()` are the versions.
  
  A handle that isn't open answers a JSON-RPC `-32600` line. One lock serialises every call, so a host may call from any thread. The lock lives in `cmd/`, which `TestNoGoroutineInTheTree` doesn't walk: a native host's threads are the host's, and nothing under `internal/` locks.
- **Saves.** Neither embedding has save slots to rely on, since a browser has no filesystem. `export_save` returns the run as a save's bytes (base64 on the wire), and `import_save [save]` makes them the run and returns the view, as `load` does. The host keeps them where it likes: `localStorage`, IndexedDB, the engine's user directory. The bytes are a slot file's exactly (`game.Encode` and `game.Decode`, `docs/saves.md`), so a save moves between the TUI and an embedding. The schema shows `[]byte` as a base64 string. `protocol.Version` is 3 (4 since #356).
- **CI.** The test job builds both targets on every PR (the `embeddings` step of `.github/actions/go`), and the lint job vets the WASM package for its own target, since `./...` on the runner skips it. The test step replays the reference game through both. In CI, a missing `node` or `cc` fails the test rather than skipping it.

**What pins it.**

- `protocol.Transcript(seed, days)` records the reference game on a server in the process: every request line and every line answered. The game's moves depend only on the answers, so an embedding handed the same requests must answer the same bytes.
- `TestWASMIsTheEngine` builds the module and runs it under Node with `wasm_exec.js`. It replays seed 7's 322 requests and gets the 2.6 MB of answers byte for byte. It also checks the two versions on the global and a refused non-string.
- `TestCIsTheEngine` builds the library, compiles and links a C program against it with `cc`, and replays the same transcript through `kingpin_handle` with the same bytes. It checks the versions and what a closed handle answers.
- `TestSaveBytesRoundTrip` plays twenty days, then `export_save`, then twenty more. Another server that runs `import_save` and plays the same twenty days gets the same events and the same view. Nothing is written to the slots. Rubbish is refused (`-32000`), and an export with no run is `-32001`.

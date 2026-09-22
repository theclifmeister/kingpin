# The engine is one session a front end drives

**The engine** (#293, `internal/engine`) is the game as one object: `engine.Session` assembles a run the one way, owns its world and its lifecycle, and publishes the day's events. The TUI (`internal/ui`) and the harness (`internal/harness`) both play through it, and so will any other front end: a graphical client built in a real game engine, a script, a server that speaks to a client in another language. #293 is the epic and holds the whole design. It lands in six phases, #296 to #301, and this file grows with each one.

**Phase 1, the session (#296).** `engine.New(cfg)` builds every sim from the one `*content.Config` in `sim.Default`'s step order (`world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`, `docs/day-loop.md`), a `game.Clock` over them and an `events.Bus` the clock publishes on. The session has no run until it is given one:

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

**What pins it.** `TestSessionIsTheHandAssembledRun` plays the trader sixty days on seed 7 through the session and through a hand-assembled clock and finds the same events and the same world, byte for byte, and what `Subscribe` received is what `EndDay` returned. `TestSessionSavesAndResumes` plays thirty days, saves, loads into a fresh session and plays thirty more, and gets the run that never stopped. The move changed no number: the harness's runs are the same runs (`TestDeterministicForSeed`, `TestSeedDigest`, `TestMoneyCurve` and the difficulty ordering unchanged), and the TUI's tests and README captures pass as they were.

**Phase 2, the rules and the commands (#297).** A front end reads the sims through `Rules` and acts on the run through the session's commands. It never holds a sim, and it never calls a `World` method that changes the run.

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

**Phase 3, the facts (#298).** The TUI used to work out some things it shows, and a second front end would have had to copy them or drift: what needs you this morning, the doors ahead, whether a day stops a fast-forward, what a price is doing. The engine decides them now and hands them over as values. The words and colours stay the front end's (the TUI's are in `docs/copy.md`'s voice).

- **Alerts** (`engine/alerts.go`). `Session.Alerts()` returns `[]Alert`, loudest first, in the order of the `AlertKind` constants: `talking`, `contract_due`, `debt_due`, `heat` (where you are, at or over the patrol line), `task_force`, `float` (only while `FloatMatters`: a front, or a route with its dial on), `wages`, `gate`, `house_known`, `da_race`, `retire`, `favour` and `reign`.
  - Each alert carries a `Key`, its identity from one morning to the next. The keys are the strings the TUI used before (`contract 12 due today`, `debt street due 41`, `unlock:front:laundromat`, `known h1`, `heat in Eastside over the patrol line`, …), so a fast-forward stops on exactly what it did.
  - The kind's fields are ids and numbers, never words: `City`, `Contract`, `Supplier`, `House`, `Due`, `Amount`, `Have`, `Heat`, `Line`, `Days`, `Count`, `Ready`, `Level`, `Gate`.
  - The TUI's `alerts()` words each one (`alertOf`), and `debtAlerts` is `alertsOf(engine.AlertDebtDue)`.
- **Gates** (#148). `engine.Gate` (`Kind`, `ID`, `Name`, `Line`, `Vouch`, `Clean`) with `Peak`, `ToGo` and `Near` (under `GateNear`, 0.5, of the line). `Session.GatesAhead()` and `NextGates()` find them. `Session.FrontOffers()` and `AssetOffers()` are the offers you do not own, the asset list less any the task force found; the TUI's `frontRows` and `assetRows` are those. The TUI keeps the words: `gateText`, `gateThe`, `gateToGo`.
- **Stops** (`engine/stops.go`).
  - `StopsOn(e)` decides whether an event stops a fast-forward, including each kind's conditions: an `Enforcement` past a patrol; a `CornerStruck` unless it is a war night that held; a `RivalBoosted` that failed; a `CornerTaken` from you; a `CrewShot` dead and yours; `PressureShifted` or `ReputationShifted` up; and a fixed list of kinds that always stop.
  - `Session.Stop(evs, before)` weighs a day in this order: a new stage, a card, an alert whose key the morning before lacked, the first event `StopsOn` names.
  - `Session.FastForward(days, after)` is the loop. It calls `EndDay` a day at a time, calls `after` with each day's events before weighing the day (the TUI saves and journals there, `dayEnded`), and returns the days run, the `Stop` (`stage`, `card`, `alert`, `event`, `over`, `cap`) and the stopping day's events.
  - The TUI's `stopEvent` is now only the words, and `stopWhy` words a `Stop`.
  - The TUI's per-day flash of enforcement for the bust scene is now taken from the day's events in `dayEnded`, in the order the bus published them. The bus subscriber (`onEvent`) is gone, because it could not be reset between days inside the engine's loop.
- **Price facts** (`engine/prices.go`). `engine.Facts(p)` and `FactsAt(p, unit)` return `PriceFacts` (`Unit`, `Delta`, `Lo`, `Hi`, `Margin`): the day's change, the range of the history, and the margin over a unit, with no unit where the city's supplier does not sell the product. The TUI's `factsAt` copies them into its own `priceFacts`, which keeps the lieutenant's markup and the rendering.

**What pins it.**

- `TestFastForwardIsTheDayLoop` plays four seeds through `FastForward` and through `EndDay` weighed by `Stop` by hand. It finds the same days, the same stop and the same stopping events, and `after` called once a day with a whole day.
- `TestStopsOnTheReadings` pins each condition, both ways.
- `TestPriceFacts` pins the change, the range, the margin and `NoSupply`.
- `TestAlertsAreKeyedOnce` plays eighty aggressive days and finds no empty or repeated key on any morning.
- `TestEveryStopHasWords` (`ui/stops_test.go`) walks `events.All`: every kind the engine stops on at its zero value has words in `stopEvent`, and nothing it does not stop on is worded. `stopEvent` falls back to the kind's name, so a stop is never silent.
- Every existing fast-forward, alert, unlock, debt and delta test in the TUI passes unchanged, and no number moved.

**Phase 4, the view (#299).** `Session.View()` (`engine/view.go`) returns `engine.View`, a snapshot of what the player can see. It is what a front end in another process draws from, and it holds no pointer into the world, so it can be kept, compared, changed and sent as JSON.

- **What it holds.** `version`, `seed`, `day` and `over` (how the run ended), then:
  - `you`: the city you're in, dirty, clean and offshore cash, net worth, peak cash, lie-low, the tier and its name, the pay and launder dials by name, the DA's evidence, fear, respect and notoriety, your street stock by city, the upgrades owned, the quiet days, the character and the hard DA.
  - `cities`: heat, pressure, goodwill, the police's next rung as the file knows it, every product (price, supplier price, demand, shock, history and its `PriceFacts`) and every corner (the cell, demand, owner, faction, runner, enforcer, since, deed).
  - `crew`: role, age, skill, loyalty, wage, the city a lieutenant runs, the post, jailed, and a lieutenant's personality once the report has named it.
  - `routes` open to you: the dial by name, closed, the driver, and the risk the file holds.
  - `shipments`, `connects` (who sells what where, today's price for what they will sell you now, the day's cap, the lot, credit, the relationship, the debt and when it is due; the temper, which the TUI shows), `houses` (stock, guard, whether the police know it) and `fronts` (level, frozen, washed).
  - `factions`: leader, alive, arrival, corners held, trust, war, and, from the file only, the temper (`?` until known), the heads as a band, the last read of the books and the next move.
  - `law`: the chief, the temper the file knows, the DA and the stance, the next election.
  - `card`: the one waiting, with its choices' labels.
  - `report`: the morning report's sections as lines.
  - `alerts`: phase 3's typed alerts.
  
  Dials go out as names (`fair`, `normal`), never as ints. Every JSON key is snake_case, and optional ones are `omitempty`.
- **What it leaves out.** What the player does not know is not there:
  - a faction's true temper, heads, chest, grudge, arrears and the corner it is eyeing
  - the chief's true temper and a route's true risk
  - a planted fact's author
  - who on the payroll is informing, and a member's greed and nerve
  - the informant's leak count (the "somebody is talking" alert is the tell)
- **Versioning.** `engine.ViewVersion` (1) is the shape of `View` and moves when a field is added, renamed, retyped or dropped. It is never tied to the save's `game.SchemaVersion`: the world stays free to change shape, and the view is the contract. `TestViewShapeIsPinned` walks the type by reflection into one line per field, its JSON path and its Go kind (`.cities[].products[].facts.margin float64`), and compares that with `engine/testdata/view_shape.txt` (215 lines, headed `version 1`).
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

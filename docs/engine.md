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

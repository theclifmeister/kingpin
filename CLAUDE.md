# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository. It is the contract: the rules, the invariants and a map of where things live. The detail of every subsystem is in `docs/` (one file each, `docs/README.md` the index); read the doc for the sim or screen you touch before changing it, and update it in the same PR.

## Workflow rules

Every change follows the same path. Do not skip steps.

1. **Start with an issue containing the plan.** Before writing code, open a GitHub issue (`gh issue create`) that states the goal, scope and acceptance criteria. Phase work references the design in issue #1 ("Part of #1"). If an issue already exists for the work, use it.
2. **One branch per issue, holding all of its changes.** Never commit to `main`. Branch from an up-to-date `main` and keep every change for that issue on that branch. `main` is protected: changes land only through a pull request with green CI.
3. **Link the PR to its issue.** The PR body must contain `Closes #N` for the issue it implements, so merging closes it. If a PR deliberately deviates from the issue's spec, say so in the PR.
4. **Docs travel with the code.** A PR that changes a subsystem updates its `docs/<topic>.md` (verbatim detail: the names, the numbers, the guard tests, the rulings and why) and adds at most a line to the map in this file. **CLAUDE.md stays under 20 KB** (#175): a paragraph that grows past a screen belongs in `docs/`.

CI (`.github/workflows/ci.yml`) runs on pull requests only (never on `main`; it changes only through merged PRs) and checks gofmt, `go mod tidy` drift, `go vet`, staticcheck, `go build`, `go test -race`, and a short balance smoke run. Make the same checks pass locally before pushing.

## Commands

Go is installed via Homebrew and is not on the default shell PATH: prefix commands with `PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH` or export it once.

```sh
go run ./cmd/kingpin                # play (needs a real terminal, >= 80x24); -slot N, -no-anim
go build ./... && go vet ./...
gofmt -l .                          # must print nothing
staticcheck ./...                   # go install honnef.co/go/tools/cmd/staticcheck@latest
go test -race ./...
go test ./internal/ui -run TestBuyThenSellFlow   # one test
go test ./internal/harness -run TestPriceInvariants -v

go run ./cmd/balance -policy managed -runs 50 -days 200   # headless balance numbers
go run ./cmd/balance -policy aggressive -seed 7 -trace    # per-day trace of one run
go run ./cmd/keys -w                                       # README key table from ui/keys.go
go test ./internal/ui -run TestReadmeCaptures -update      # README captures from the rich fixture
go run ./cmd/anim                                          # review every scene (-list, -scene, -seed)
```

`cmd/balance` policies: `idle | hide | quiet | normal | aggressive | careful | managed | upgraded | crewed | territory | war | diplomat | laundered | funded | distributor | delegated | dealer | stocked | routine | leveraged | boss | pricewar | saboteur | tipster | retiree` (an unknown one falls back to `normal`). What each plays, its flags (`-lielow -corners -force -rival -cash -own -cards -lt -chief -da -undercut -credit -houses -fronts -heat -pace -margin`) and what the output lines mean are in `docs/harness.md`. Day counts (`harness.Horizon`, `TierDays`, `-days`) are where the tooling *looks*, never a run length: the game has no day cap and a run ends only through an ending (#27). Do not add mechanics that end a run for playing on.

Set `KINGPIN_HOME` to keep test saves out of your real config dir (tests do this with `t.TempDir()`). To drive the TUI headlessly, run it under `tmux` and use `send-keys` / `capture-pane`; `internal/ui/ui_test.go` has a `key()` helper for feeding `tea.KeyMsg`s to the model directly. `KINGPIN_NO_ANIM=1` turns the animation off; `KINGPIN_ANIM_EFFECT=name` pins the title effect.

## Architecture

The game is a set of independent, deterministic simulations stepped once per in-game day over a shared `World`, with a Bubble Tea UI on top. The pieces are wired in `internal/ui/model.go` (`New`) and `internal/harness` (headless). These are the invariants; break one only with an issue that says so.

**The day loop** (`docs/day-loop.md`). `game.Clock.EndDay` builds a `Tick` with a per-day RNG from `game.RNGFor(seed, day)`, steps every `Simulation` in the fixed order `market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`, clears the player's per-day scratch and publishes the tick's events. Nothing about randomness is saved: a run is reproducible from its seed (`TestDeterministicForSeed`, `TestSaveRoundTripIsDeterministic`). `Tick.RNG` is the home city's stream; anything away from home, or any feature added since, rolls on `Tick.Sub(name)`, so **a run that never uses a feature is byte-for-byte the run before the feature existed** and no seed-pinned balance number moves. A sim that must draw on the home stream makes the roll whether or not the result is used.

**Events are the only cross-sim channel** (`docs/events.md`). Sims never call each other; a sim emits typed events (`internal/events`, each with a stable `Kind()`) via `Tick.Emit`, later sims in the same tick read `Tick.Events()`, and after the tick the bus delivers them to the UI. A new event kind gets a headline template in `content/headlines.toml` and a line in `TestEveryEmittedEventHasTemplate`, or is listed there as report-only. Each sim folds its own tuning (`content.*Config`, the tree's `game.FoldEffects`, reputation's `[effects]`, the law's tables) and never reads another sim's.

**World is a plain struct** (`docs/world.md`) that sims mutate directly; player actions queue intent that the sims resolve (or, for a buy and a trip, apply at once), and the morning `DayReport` is what the UI shows. Stock moves only through `World.AddStock` / `TakeStock` and the house helpers (`TestStashHasNoWriters`, #144); the per-day scratch is one struct, `World.Today`, that the clock zeroes as a unit (`ClearToday`); a sim writes only its own state (`TestSimsWriteOnlyTheirOwnState`, the exceptions listed there), and `TestSeedDigest` pins sixty days of the boss on one seed so a moved number names its day. `SchemaVersion` is 12; bump it when `World` changes shape incompatibly and add a `game.Migration` for the previous version to `sim.Set.Migrations` (`docs/saves.md`). Zero values are the pre-feature state wherever they can be, so most features need no bump.

**All tuning is data** (`docs/tuning.md`). `internal/content/*.toml` is embedded and decoded into `content.Config`; sims take their config struct at construction. Balance changes are TOML edits verified with `cmd/balance` and the harness: `TestMoneyCurve` pins one band a progression tier (t1 `managed` $85k at day 30, t2 `crewed` $605k at day 70, t3 `boss` $16.7M at day 120, t4 `boss` $84M at day 200; the checkpoints are `progression.toml`'s), the difficulty tests pin the ordering (aggressive is indicted, quiet survives and earns less, managed > quiet, crewed > managed, upgraded > crewed, laundered > crewed, distributor > laundered), and a pinned number that moves is named in the PR with the reason. Heat multipliers cannot make an always-aggressive player last longer (heat saturates); volume nodes cut what a *runner's* unit draws, never yours.

**The UI is one grammar** (`docs/ui.md`, epic #78). One Bubble Tea `Model`, a `mode` state machine and a `screen` tab; the three-part frame (`docs/frame-and-pane.md`), the details pane beside MAIN from 100 columns and the strip under it, one key table (`ui/keys.go` `bindings` / `modeBindings`, `docs/keys.md`: a key is bound once, listed where it is used, and no text names a key but the pane's KEYS, `keyRow`s, modal footers, `emptyState` and the dashboard's `tutorialLine`), one modal (`docs/modals.md`: `esc` closes, `shift+tab` goes back, every quantity is a `numberField`), one `table()`, `internal/format` for every number (`docs/format.md`), `theme` the only stylist, one voice (`docs/copy.md`: `say` / `refuse` / `alarm`, real plurals, `→`, `×`). Layout fits 80x24, 100x30 and 120x40 (`TestRendersAtCommonSizes`, `TestModalsFit` over `modeCount`). `enter` inside a modal never ends the day (`TestEnterDoesNotEndDay`). The README is generated from the code (`cmd/keys -w`, `TestReadmeCaptures -update`) and is stale until it is.

**Animation is a tick only while a scene runs** (`docs/animation.md`, #152). The UI redraws on a key, a resize, *and* a `tea.Tick` while `Model.scene != nil`: never in play mode, never in fast-forward's loop, never in the harness, never in the fixtures or captures (`Options{Anim: false}`); `Init` still returns nil. `internal/ui/anim` imports `theme` and nothing else of `ui`; a scene is a pure function of `(t, w, h, seed, text)`, reads the world and never writes it (`TestSceneNeverTouchesTheWorld`), any key skips it, and every effect is credited in `anim/NOTICE`.

### The map

Package layout: `cmd/kingpin` (the game), `cmd/balance` (headless runs), `cmd/keys` (the README's table), `cmd/anim` (the scenes, for review); `internal/game` (`World`, actions, clock, saves), `internal/sim/<name>` (one sim each), `internal/events`, `internal/content` (TOML and decode), `internal/format`, `internal/harness` (scripted policies and the acceptance tests), `internal/ui` (screens, dialogs, `theme`, `anim`).

| Subsystem | Code | Tuning | Doc | Pins it |
|---|---|---|---|---|
| Cities, stock, capacity | `game/world.go`, `sim/logistics.Migrate`, `sim/territory.Migrate` | `city.toml` | `docs/cities.md` | `TestSaveMigratesTheOneCity`, `TestSalesElsewhereAreRunnersOnly` |
| Corners, demand, robbery, drift | `game/territory.go`, `sim/territory` | `city.toml` | `docs/corners.md` | `territory_test.go` |
| Market, prices, orders, price war | `sim/market`, `game/actions.go` | `market.toml` | `docs/tuning.md`, `docs/rival.md` (#68) | `TestPriceInvariants`, `TestNoUndercutIsTheOldRun` |
| Connects (suppliers, credit) | `game/suppliers.go`, `sim/market/suppliers.go`, `ui/suppliers.go` | `suppliers.toml` | `docs/connects.md` | `TestSupplierInvariants`, `TestLeveragedIsALeverNotFreeMoney` |
| Supply contracts (buy routine) | `World.Supply`, `market.Sim.Plan`, `ui/dialogs.go` | `market.toml [supply]` | `docs/supply-contracts.md` | `TestSupplyMatchesTheHand`, `TestStockedIsWithinFifteenPercentOfCrewed` |
| Standing orders (sell routine) | `World.Standing`, `market.Sim.standing` | `market.toml [standing]` | `docs/standing-orders.md` | `TestStandingSellsLikeTheHand`, `TestRoutineIsWithinFifteenPercentOfCrewed` |
| Buyers (contracts) | `game/buyers.go`, `sim/market/buyers.go`, `ui/buyers.go` | `buyers.toml` | `docs/buyers.md` | `buyers_test.go`, `TestDealerBeatsCrewed` |
| Heat, evidence, the police response | `sim/heat`, levels `content.Patrol..Arrest` | `heat.toml` | `docs/corners.md`, `docs/snitching.md` | `heat_test.go`, `TestRichHiderIsNeverIndicted` (#27), `balance_test.go` |
| The law: chief, DA, pressure, goodwill, campaigns (#193) | `sim/law`, `game/law.go`, `ui/law.go` | `law.toml`, `names.toml` | `docs/law.md` | `law_test.go`, `TestQuietDayRuleHoldsUnderEveryLaw`, `TestNoCampaignIsTheOldRun` |
| Logistics: routes, targets, shipments | `sim/logistics`, `ui/routes.go` | `routes.toml` | `docs/logistics.md` | `TestStockIsConservedAcrossShipments`, `TestDistributorBeatsLaundered` |
| Laundering: fronts, the float, audits, the levels (#192), the offshore account and `Retire` (#195) | `sim/laundering`, `game/exit.go`, `ui/ledger.go`, `ui/invest.go`, `ui/reserve.go` | `laundering.toml` | `docs/laundering.md` | `laundering_test.go`, `TestLaunderingNodesPullTheirWay`, `TestLevelsPullTheirWay`, `TestNoInvestIsTheOldRun`, `TestStructuringFilesPages`, `TestRetireeRetires` |
| Stash houses | `game/houses.go`, `ui/houses.go`, raid in `sim/heat`, rent in `sim/territory` | `houses.toml` | `docs/houses.md` | `houses_test.go`, `TestDecoyHouseNeverShieldsTheStreet` |
| Crew, pay, snitching, investigation | `sim/crew`, `ui/crew.go` | `crew.toml` | `docs/snitching.md`, `docs/crew-and-upgrades-screens.md` | `crew_test.go`, `snitch_test.go` |
| Lieutenants, buying through them | `sim/crew/lieutenant.go`, `ui/lieutenant.go`, `Buy` in `game/suppliers.go` | `crew.toml [lieutenant]` | `docs/lieutenants.md` | `lieutenant_test.go`, `TestBuyThroughTheLieutenant` |
| The rival: pace, tell, price war, economy, books | `sim/rivals`, `game/territory.go`, `game/rivals.go`, `ui/rivals.go` | `rivals.toml` | `docs/rival.md` | `rivals_test.go`, `pricewar_test.go`, `TestRivalEconomyBinds`, `TestNoBooksIsTheOldRun`, `TestRivalStateHasOneWriter` |
| Diplomacy: truce, tribute, split | `game/diplomacy.go`, `sim/rivals/diplomacy.go`, `ui/diplomacy.go` | `rivals.toml [diplomacy]`, `[deal.*]` | `docs/diplomacy.md` | `diplomacy_test.go`, `TestTributeShareOfTheTake` |
| Reputation: fear, respect, notoriety | `sim/reputation` | `reputation.toml` | `docs/reputation.md` | `reputation_test.go`, `TestReputationCannotMaxAllThree` |
| Upgrades: the tree, the effect vocabulary | `game/upgrades.go`, `content/upgrades.toml`, `ui/upgrades.go` | `upgrades.toml` | `docs/upgrades.md` | `TestFoldEffects`, `TestUpgradedBeatsCrewed`, `Test*NodesMoveTheirNumbers` |
| Dilemma cards | `game/dilemmas.go`, `sim/news/dilemmas.go`, `ui/card.go` | `dilemmas.toml` | `docs/dilemmas.md` | `dilemmas_test.go`, `TestTriggersHold` |
| Progression tiers, the stage, unlocks | `game/progression.go`, `sim/news/progression.go`, `ui/stage.go`, `ui/unlocks.go`, `events.Unlocked` | `progression.toml` | `docs/progression.md`, `docs/stage.md`, `docs/unlocks.md` | `TestTiersAreOrdered`, `TestEveryGateIsAnnounced`, `TestNoUnlockIsTheOldRun` |
| News, report, journal | `sim/news`, `ui/journal.go` | `headlines.toml` | `docs/events.md`, `docs/market-and-journal.md` | `TestEveryEmittedEventHasTemplate`, `TestArticlesAgreeWithTheValue`, `TestSimsNeverImportEachOther` |
| Saves and slots | `game/save.go`, `modeStart` | | `docs/saves.md` | `TestOldSaveIsMigrated`, `TestUnreadableSaveIsRefused` |
| Fast-forward, alerts, stop events | `ui/fast.go`, `dashboard.go` `alerts()` | | `docs/ui.md` | `TestFastForwardIsTheSameDays`, `TestFastForwardStopsOnACard` |
| The cart, the dialogs, the delta | `ui/cart.go`, `ui/dialogs.go`, `ui/market.go` `priceFacts` | | `docs/cart.md` | `cart_test.go`, `delta_test.go`, `toggle_test.go` |
| Dashboard, map, ledger, rivals screens | `ui/dashboard.go`, `map.go`, `routes.go`, `ledger.go`, `rivals.go` | | `docs/frame-and-pane.md`, `docs/ui.md` | `dashboard_test.go`, `TestRouteMarkerMoves`, `TestTablesAreConsistent` |
| Animation: scenes, effects, the title loop, the registry, `cmd/anim` | `ui/anim` (`Scenes()`), `ui/scene*.go`, `ui/demo.go`, `cmd/anim` | | `docs/animation.md` | `TestEveryModeWithASceneIsListed`, `TestNoTickInPlayMode`, `TestSceneStopsTicking`, `TestScenesFit`, `TestSceneLengths` |

### Rules of thumb that took a PR to learn

- A new alert goes in `alerts()`, a new fast-forward stop in `stopEvent`; key them so `F` stops once.
- Sale heat is per unit moved, weighted by the corner and the city; only dealing builds a case (a sting on a quiet day adds no evidence). The one exception is an informant (`docs/snitching.md`).
- The rival's costs are in corner-days and its muscle is what its take pays for (`docs/rival.md`, #139); its claim is telegraphed a day ahead (#69), so a pinned seed that reads one day can shift by one.
- A cut off the take weighs ~2.5x on net worth at the crewed margin (`docs/standing-orders.md`): price a routine's cost against that.
- The laundering float is one number on the world (`World.Float`) that the wash and the road share; supply contracts have their own.
- The lone trader who buys the whole tree is meant to lose money on it; the tree is measured on the operation it is for (`upgraded`, `boss`).
- Products above the first three are gated by `unlock_cash`; designer is the port's product and reaches home only by road. A cash gate cannot separate tier 2 from tier 3; the ladder does.
- The tier is read, never written by a sim: gates live in their own files and `progression.toml` describes them.

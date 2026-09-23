# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository. It is the contract: the rules and the invariants. The detail of every subsystem is in `docs/` (one file each); `docs/README.md` is the index and **the map** of where each subsystem lives (code, tuning, doc, the tests that pin it). Read the doc for the sim or screen you touch before changing it, and update it in the same PR.

## Workflow rules

Every change follows the same path. Do not skip steps.

1. **Start with an issue containing the plan.** Before writing code, open a GitHub issue that states the goal, scope and acceptance criteria. Phase work references the design in issue #1 ("Part of #1"). If an issue already exists for the work, use it.
2. **One branch per issue, holding all of its changes.** Never commit to `main`; branch from an up-to-date `main`. `main` is protected: changes land only through a pull request with green CI.
3. **Link the PR to its issue** with `Closes #N`. If a PR deliberately deviates from the issue's spec, say so in the PR.
4. **Docs travel with the code.** A PR that changes a subsystem updates its `docs/<topic>.md` (the names, the numbers, the guard tests, the rulings and why) and its row in the map. **CLAUDE.md stays under 20 KB** (#175): detail belongs in `docs/`.

CI runs on pull requests only (`docs/harness.md`): gofmt, `go mod tidy` drift, vet, staticcheck, govulncheck, build, `go test` (`-race` on all but `internal/harness` and `internal/ui`) and a balance smoke run. Make the same checks pass locally before pushing.

## Commands

Go is installed via Homebrew and is not on the default shell PATH: prefix commands with `PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH` or export it once.

```sh
go run ./cmd/kingpin                # play (needs a real terminal, >= 80x24); -slot N, -no-anim
go build ./... && go vet ./...
gofmt -l .                          # must print nothing
go tool staticcheck ./...
go tool govulncheck ./...           # both pinned in go.mod's tool block (#276)
go test -race ./...
go test ./internal/ui -run TestBuyThenSellFlow   # one test

go run ./cmd/balance -policy managed -runs 50 -days 200   # headless balance numbers
go run ./cmd/balance -policy aggressive -seed 7 -trace    # per-day trace of one run
go run ./cmd/keys -w                                       # README key table from ui/bindings.go
go test ./internal/ui -run TestReadmeCaptures -update      # README captures from the rich fixture
go run ./cmd/anim                                          # review every scene (-list, -scene, -seed)
```

`cmd/balance -policy P` plays one of thirty-six scripted policies (`idle` to `informed`, `harness.Policies`; flags and output in `docs/harness.md`). Day counts (`harness.Horizon`, `TierDays`, `-days`) are where the tooling *looks*, never a run length: the game has no day cap and a run ends only through an ending (#27). Do not add mechanics that end a run for playing on.

`KINGPIN_HOME` keeps saves and the profile out of your real config dir (tests use `t.TempDir()`). To drive the TUI headlessly, run it under `tmux` (`send-keys` / `capture-pane`); UI tests feed keys with `key()` (`internal/ui/helpers_test.go`) and take `KINGPIN_TEST_SEED=n|random`. `KINGPIN_NO_ANIM=1` turns the animation off; `KINGPIN_ANIM_EFFECT=name` pins the title effect.

## Architecture

Independent, deterministic simulations stepped once per in-game day over a shared `World`; `engine.Session` assembles a run for every front end (the Bubble Tea UI, the harness, `kingpind`; `docs/engine.md`). Package layout: `cmd/` (`kingpin`, `balance`, `keys`, `anim`, and the engine's `kingpind`, `kingpin-client`, `kingpin-wasm`, `libkingpin`, `kingpin-web`), `internal/game` (`World`, actions, clock, saves), `internal/sim/<name>` (one sim each), `internal/engine`, `internal/protocol`, `internal/events`, `internal/content` (TOML), `internal/format`, `internal/harness` (policies and acceptance tests), `internal/gametest` (the sims' fixture, tests only), `internal/ui` (screens, `theme`, `anim`). These are the invariants; break one only with an issue that says so.

**The day loop** (`docs/day-loop.md`). `Clock.EndDay` steps every sim in the fixed order `world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news` on a per-day RNG, `game.RNGFor(seed, day)`; nothing about randomness is saved (`TestDeterministicForSeed`, `TestSaveRoundTripIsDeterministic`). `Tick.RNG` is the home stream; anything away from home or added since rolls on `Tick.Sub(game.Stream*)` (`TestStreamsAreNamed`), so **a run that never uses a feature is byte-for-byte the run before it** and no seed-pinned number moves. A sim that draws on the home stream makes the roll whether or not the result is used.

**Events are the only cross-sim channel** (`docs/events.md`). Sims never call each other (`TestSimsNeverImportEachOther`): they `Tick.Emit` typed events, later sims read `Tick.Events()`, the bus delivers them to the front end. A new kind goes in `events.All` and gets a headline in `content/headlines.toml` or a line in the news test's `reportOnly`. Each sim folds its own tuning and never reads another sim's.

**World is a plain struct** (`docs/world.md`). Player actions queue intent the sims resolve (a buy and a trip apply at once). Stock moves only through `World.AddStock` / `TakeStock` and the house helpers (`TestStashHasNoWriters`); the day's scratch is `World.Today`, zeroed as a unit; a sim writes only its own state (`TestSimsWriteOnlyTheirOwnState`, `TestCornerWritersAreDeclared`); `TestSeedDigest` pins sixty days of one seed so a moved number names its day. Bump `SchemaVersion` and add a `game.Migration` when `World` changes shape incompatibly (`docs/saves.md`); zero values are the pre-feature state wherever they can be.

**All tuning is data** (`docs/tuning.md`). `internal/content/*.toml` is embedded, decoded strictly and validated; sims take their config at construction. Balance changes are TOML edits checked with `cmd/balance`: `TestMoneyCurve` pins a band per progression tier (the figures are in `docs/tuning.md`), the difficulty tests pin the ordering, and a pinned number that moves is named in the PR with the reason. Heat multipliers cannot make an always-aggressive player last longer (heat saturates); volume nodes cut what a *runner's* unit draws, never yours.

**The UI is one grammar** (`docs/ui.md`). One `Model` with the `modes` and `screens` tables; the three-part frame and the details pane (`docs/frame-and-pane.md`); one key table, a key bound once (`docs/keys.md`); one modal, `esc` closes and `shift+tab` goes back, every quantity a `numberField` (`docs/modals.md`); one `table()`; `internal/format` for every number; `theme` the only stylist; one voice (`docs/copy.md`). Layout fits 80x24, 100x30 and 120x40 (`TestRendersAtCommonSizes`, `TestModalsFit`), and `enter` in a modal never ends the day (`TestEnterDoesNotEndDay`). The README is generated (`cmd/keys -w`, `TestReadmeCaptures -update`) and is stale until it is.

**Animation ticks only while a scene runs** (`docs/animation.md`). Never in play mode, fast-forward, the harness or the captures (`Options{Anim: false}`). A scene is a pure function of `(t, w, h, seed, text)` that never writes the world (`TestSceneNeverTouchesTheWorld`); any key skips it; every effect is credited in `anim/NOTICE`.

## Rules of thumb that took a PR to learn

- A new alert goes in `engine.Alerts`, a new stop in `engine.StopsOn`; key them so `F` stops once. A character is a start on day 0 and nothing a sim reads.
- Sale heat is per unit moved, weighted by the corner and the city; only dealing builds a case, bar an informant and a bribe that backfires (#42): both something you did.
- The rival's costs are in corner-days and its muscle is what its take pays for (#139); its claim is telegraphed a day ahead (#69), so a pinned seed reading one day can shift by one.
- A cut off the take weighs ~2.5x on net worth at the crewed margin.
- The laundering float (`World.Float`) is one number the wash and the road share; supply contracts have their own.
- The lone trader who buys the whole tree is meant to lose money on it; the tree is measured on `upgraded` and `boss`.
- Products above the first three are gated by `unlock_cash`; designer is the port's. A cash gate cannot separate tier 2 from tier 3; the ladder does. The tier is read, never written by a sim.
- The faction table is the duel's dice plus side streams (#43), so `harness.OneFaction` is the old run; the table shares the map (`shared_pace`, `table_share`) or the bands break.
- Incidents are weather, not difficulty: the harness boxes the table, and an effect writes only state a sim already reads or rides the `Incident` event.

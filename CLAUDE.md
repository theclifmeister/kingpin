# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Workflow rules

Every change follows the same path. Do not skip steps.

1. **Start with an issue containing the plan.** Before writing code, open a GitHub issue (`gh issue create`) that states the goal, scope and acceptance criteria. Phase work references the design in issue #1 ("Part of #1"). If an issue already exists for the work, use it.
2. **One branch per issue, holding all of its changes.** Never commit to `main`. Branch from an up-to-date `main` and keep every change for that issue on that branch. `main` is protected: changes land only through a pull request with green CI.
3. **Link the PR to its issue.** The PR body must contain `Closes #N` for the issue it implements, so merging closes it. If a PR deliberately deviates from the issue's spec, say so in the PR.

CI (`.github/workflows/ci.yml`) runs on pull requests only (never on `main`; it changes only through merged PRs) and checks gofmt, `go mod tidy` drift, `go vet`, staticcheck, `go build`, `go test -race`, and a short balance smoke run. Make the same checks pass locally before pushing.

## Commands

Go is installed via Homebrew and is not on the default shell PATH: prefix commands with `PATH=/opt/homebrew/bin:$HOME/go/bin:$PATH` or export it once.

```sh
go run ./cmd/kingpin                # play (needs a real terminal, >= 80x24)
go build ./... && go vet ./...
gofmt -l .                          # must print nothing
staticcheck ./...                   # go install honnef.co/go/tools/cmd/staticcheck@latest
go test -race ./...
go test ./internal/ui -run TestBuyThenSellFlow   # one test
go test ./internal/harness -run TestPriceInvariants -v

go run ./cmd/balance -policy managed -runs 50 -days 200   # headless balance numbers
go run ./cmd/balance -policy aggressive -seed 7 -trace    # per-day trace of one run
```

`cmd/balance` policies: `idle | hide | quiet | normal | aggressive | careful | managed | crewed | territory`. An unknown policy silently falls back to `normal`. `-lielow N` overrides the lie-low threshold of the managed-style policies; `-corners N` caps how many corners `territory` works (`crewed` works as many as it can staff); `-cash N` starts every run with that much dirty cash (`-policy hide -cash 5000000` is the rich player who sits on the pile).

Set `KINGPIN_HOME` to keep test saves out of your real config dir (tests do this with `t.TempDir()`). To drive the TUI headlessly, run it under `tmux` and use `send-keys` / `capture-pane`; `internal/ui/ui_test.go` has a `key()` helper for feeding `tea.KeyMsg`s to the model directly.

## Architecture

The game is a set of independent, deterministic simulations stepped once per in-game day over a shared `World`, with a Bubble Tea UI on top. The pieces are wired in `internal/ui/model.go` (`New`) and `internal/harness` (headless).

**Day loop.** `game.Clock.EndDay` (`internal/game/clock.go`) builds a `Tick` with a per-day RNG from `game.RNGFor(seed, day)`, steps every `Simulation` in the fixed order `market -> territory -> crew -> heat -> news` (territory sits in the rivals slot; the design reserves slots for logistics and laundering), then clears the player's per-day scratch (orders, buys, lie-low, hires, fires) and publishes the tick's events on the `events.Bus`. Because RNG is derived from `(seed, day)`, nothing about randomness is saved and a run is reproducible from its seed; `TestDeterministicForSeed` and `TestSaveRoundTripIsDeterministic` enforce this. Sims must stay deterministic given `(World, Tick.RNG)`.

**Events are the cross-sim channel.** Sims never call each other. A sim emits typed events (`internal/events/events.go`, each with a stable `Kind()`) via `Tick.Emit`; later sims in the same tick read `Tick.Events()` to react (territory, crew and heat consume the market's `PlayerSold`, news consumes everything). After the tick, the bus delivers events to the UI (`Model.onEvent`) for the ticker, alerts and journal. Adding an event kind means adding a headline template in `internal/content/headlines.toml` and listing it in `TestEveryEmittedEventHasTemplate`, unless it is report-only bookkeeping like `PriceMove` or `CrewPaid`.

**World is a plain struct** (`internal/game/world.go`) that sims mutate directly. Player actions (`Buy`, `PlaceSell`, `CancelSell`, `SetLieLow`, `Hire`, `Fire`, `SetPay` in `actions.go`; `Post`, `Recall`, `Abandon` in `territory.go`) only queue intent; the market sim resolves sell orders during `EndDay`, the crew sim reports hires and fires, the territory sim reports claims, and the morning `DayReport` is what the UI shows. `World.Capacity()` (carry limit plus runner units) is the holding limit.

**Corners are the demand.** `World.Territory.Corners` (seeded from `internal/content/city.toml`) each carry a size, per-product `Taste`, a heat multiplier and a robbery `Risk`. `ProductMarket.Demand` is per *standard corner*; what the player can sell is `World.Demand(product)` = that times `HeldShare` (the summed shares of corners that are `Worked()`: owned and with a runner, or `game.You`, posted). A held corner with nobody on it drifts back to `none` after `drift_days`; an enforcer posted on a corner cuts its robbery chance. `Fire` and crew quits `Recall` the member so a corner never points at someone who is gone. The heat sim weights every unit sold by the corner it moves on (`CornerWeight`: corner heat, and `crew_heat` for a runner's corner), so where you sell matters as much as how much. Dirty cash over `dirty_cash_threshold` is heat too, but only dealing builds a case: a sting or raid that fires on a day the player attempted no sale (no `PlayerSold` with `Wanted > 0`; lying low resolves none) still costs stock and cash and cools heat but adds no evidence, so a cash pile draws stings without being a countdown (#27, `TestRichHiderIsNeverIndicted`). `World` is gob-serialised as the single save slot (`save.go`, atomic write, `SchemaVersion` guard). Bump `SchemaVersion` when `World` changes shape incompatibly and add a `game.Migration` step for the previous version to `sim.Set.Migrations` (gob fills missing fields with zero values, so a step usually only seeds the new state); `Load` walks the chain and refuses saves it has no path for, or that are newer than the build.

**All tuning is data.** `internal/content/*.toml` is embedded and decoded into `content.Config`; sims take their config struct at construction (`sim.Default`). Balance changes should be TOML edits, verified with `cmd/balance` and the harness tests in `internal/harness/balance_test.go`, and `crew_test.go`, which pin the difficulty curve (always-aggressive gets indicted fast, always-quiet survives and earns less, managed beats quiet, crewed beats managed) and the crew invariants (loyalty monotone in pay, no skim above the threshold). `TestMoneyCurve` pins the economy's scale (issue #24): one row per progression tier giving the band the best policy's median net worth must land in at that tier's checkpoint (`harness.TierDays`); a phase that ships a tier's multiplier adds that tier's row (tier 1 `managed`, tier 2 `crewed`; tier 3 waits on somewhere to launder, since dirty cash over `dirty_cash_threshold` is itself heat and caps every policy near $1.5M). Day counts in the harness (`harness.Horizon`, `TierDays`, `-days`) are where the tooling *looks*, never a run length: the game has no day cap, a run ends only through an ending, and the player sets the pace. Do not add mechanics that end a run merely for playing on (#27 removed the one that did). `territory_test.go` pins the corner invariants: served demand is exactly the worked corners' share, losing every runner loses every corner within `drift_days`, and no corner means nothing sells. Products above the starting three are gated by `unlock_cash` in `market.toml`: the market sim lists a product the first day after peak cash reaches it. Sale heat is per unit moved (`sale_heat` per `street_units` heat-weighted units), so heat scales with volume, not with how many products exist.

**Cash formatting** in the UI: `cash()` (`internal/ui/format.go`) compacts totals (`$45K`, `$1.2M`, `$3.4B`) for the title bar, dashboard, report and run summary; `money()` keeps exact figures for itemised amounts (fees, wages, a purchase); `price()` is per-unit (cents under $1,000).

**UI** (`internal/ui`) is one Bubble Tea `Model` with a `mode` state machine (start menu, play, buy/sell dialog, report, help, confirm-new, confirm-fire, confirm-end, post picker, game over) and a `screen` tab (dashboard, market, journal, crew, map). Key handling is split: `handleKey` dispatches by mode, `keyDialog` owns the buy/sell flow (product -> quantity -> dial), `keyPlay` owns the main screen; `h`/`f` hire and fire only on the crew screen, `c`/`e`/`a` post a runner / post an enforcer / abandon only on the map screen (`map.go`; the picker is `modePost`), `p` cycles the pay dial anywhere. Up/down move the cursor on every screen; left/right switch tabs, so the map is walked as a list with the grid highlighting the selection. `n` ends the day at once; `enter` on the play screen opens a confirmation (`modeConfirmEnd`) and only a second `enter` or `y` ends it. Inside dialogs and the report `enter` confirms and closes and must never advance the day (`TestEnterDoesNotEndDay`). Layout must fit 80x24 and 120x40 (`TestRendersAtCommonSizes`), and the ticker must never exceed terminal width. The title bar drops to short tab names and then digits as width runs out; long lines on a screen should be `truncate`d, not left to wrap. Colours live in `internal/ui/theme`, one accent per sim.

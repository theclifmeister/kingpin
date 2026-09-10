# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Workflow rules

Every change follows the same path. Do not skip steps.

1. **Start with an issue containing the plan.** Before writing code, open a GitHub issue (`gh issue create`) that states the goal, scope and acceptance criteria. Phase work references the design in issue #1 ("Part of #1"). If an issue already exists for the work, use it.
2. **One branch per issue, holding all of its changes.** Never commit to `main`. Branch from an up-to-date `main` and keep every change for that issue on that branch. `main` is protected: changes land only through a pull request with green CI.
3. **Link the PR to its issue.** The PR body must contain `Closes #N` for the issue it implements, so merging closes it. If a PR deliberately deviates from the issue's spec, say so in the PR.

CI (`.github/workflows/ci.yml`) runs gofmt, `go mod tidy` drift, `go vet`, staticcheck, `go build`, `go test -race`, and a short balance smoke run. Make the same checks pass locally before pushing.

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

`cmd/balance` policies: `idle | quiet | normal | aggressive | careful | managed | crewed`. An unknown policy silently falls back to `normal`. `-lielow N` overrides the lie-low threshold of the managed-style policies.

Set `KINGPIN_HOME` to keep test saves out of your real config dir (tests do this with `t.TempDir()`). To drive the TUI headlessly, run it under `tmux` and use `send-keys` / `capture-pane`; `internal/ui/ui_test.go` has a `key()` helper for feeding `tea.KeyMsg`s to the model directly.

## Architecture

The game is a set of independent, deterministic simulations stepped once per in-game day over a shared `World`, with a Bubble Tea UI on top. The pieces are wired in `internal/ui/model.go` (`New`) and `internal/harness` (headless).

**Day loop.** `game.Clock.EndDay` (`internal/game/clock.go`) builds a `Tick` with a per-day RNG from `game.RNGFor(seed, day)`, steps every `Simulation` in the fixed order `market -> crew -> heat -> news` (the design reserves slots for logistics, rivals, laundering), then clears the player's per-day scratch (orders, buys, lie-low, hires, fires) and publishes the tick's events on the `events.Bus`. Because RNG is derived from `(seed, day)`, nothing about randomness is saved and a run is reproducible from its seed; `TestDeterministicForSeed` and `TestSaveRoundTripIsDeterministic` enforce this. Sims must stay deterministic given `(World, Tick.RNG)`.

**Events are the cross-sim channel.** Sims never call each other. A sim emits typed events (`internal/events/events.go`, each with a stable `Kind()`) via `Tick.Emit`; later sims in the same tick read `Tick.Events()` to react (crew and heat consume the market's `PlayerSold`, news consumes everything). After the tick, the bus delivers events to the UI (`Model.onEvent`) for the ticker, alerts and journal. Adding an event kind means adding a headline template in `internal/content/headlines.toml` and listing it in `TestEveryEmittedEventHasTemplate`, unless it is report-only bookkeeping like `PriceMove` or `CrewPaid`.

**World is a plain struct** (`internal/game/world.go`) that sims mutate directly. Player actions (`Buy`, `PlaceSell`, `CancelSell`, `SetLieLow`, `Hire`, `Fire`, `SetPay` in `actions.go`) only queue intent; the market sim resolves sell orders during `EndDay`, the crew sim reports hires and fires, and the morning `DayReport` is what the UI shows. `World.Capacity()` (carry limit plus runner units) is the holding limit, and `Reach()` scales how much of demand the market fills. `World` is gob-serialised as the single save slot (`save.go`, atomic write, `SchemaVersion` guard). Bump `SchemaVersion` when `World` changes shape incompatibly.

**All tuning is data.** `internal/content/*.toml` is embedded and decoded into `content.Config`; sims take their config struct at construction (`sim.Default`). Balance changes should be TOML edits, verified with `cmd/balance` and the harness tests in `internal/harness/balance_test.go`, and `crew_test.go`, which pin the difficulty curve (always-aggressive gets indicted fast, always-quiet survives and earns less, managed beats quiet, crewed beats managed) and the crew invariants (loyalty monotone in pay, no skim above the threshold).

**UI** (`internal/ui`) is one Bubble Tea `Model` with a `mode` state machine (start menu, play, buy/sell dialog, report, help, confirm-new, confirm-fire, game over) and a `screen` tab (dashboard, market, journal, crew). Key handling is split: `handleKey` dispatches by mode, `keyDialog` owns the buy/sell flow (product -> quantity -> dial), `keyPlay` owns the main screen; `h`/`f` hire and fire only on the crew screen, `p` cycles the pay dial anywhere. `n` is the only key that ends the day; `enter` confirms dialogs and closes the report and must never advance the day (`TestEnterDoesNotEndDay`). Layout must fit 80x24 and 120x40 (`TestRendersAtCommonSizes`), and the ticker must never exceed terminal width. Colours live in `internal/ui/theme`, one accent per sim.

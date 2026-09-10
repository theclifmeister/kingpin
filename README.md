# Kingpin

A terminal game about building a drug empire one day at a time while the
heat closes in. Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> You start with $500, a burner phone and a corner nobody wants. Everything
> after that is your fault.

The full design is in [issue #1](https://github.com/theclifmeister/kingpin/issues/1).
This is the Phase 1 build: one city, three products, a market that reacts to
you, and a police force that reacts to how loudly you sell.

## Play

```
go run ./cmd/kingpin
```

Needs Go 1.24 and a terminal at least 80x24. A single save slot lives in
`$KINGPIN_HOME` or your platform config directory under `kingpin/`; the game
autosaves at the end of every day.

| Key | Action |
|---|---|
| `1` `2` `3` / `tab` | Dashboard, Market, Journal |
| `b` | Buy from the supplier (blank quantity = as much as you can) |
| `s` | Queue a street sale and set the dial: quiet / normal / aggressive |
| `x` | Cancel the queued order on the selected product |
| `l` | Lie low today: no sales, heat fades faster |
| `n` | End the day |
| `r` | Reopen the morning report |
| `?` | Help |
| `q` | Save and quit |

## How it works

Every day the simulations step in a fixed order (`market -> heat -> news`),
each reading the world and emitting typed events that later sims and the UI
consume. Randomness is derived from the run seed and the day number, so a
run is fully reproducible and nothing about the RNG needs saving.

- **Market** drifts prices toward an equilibrium with noise, rolls supply
  shocks and demand slumps, and resolves your sell orders. Selling into
  demand barely moves the price; flooding past it craters it.
- **Heat** rises with the volume you *tried* to move and how loud the dial
  was, plus a little for sitting on a pile of dirty cash. It decays slowly,
  faster if you lie low. Thresholds trigger patrols, stings, raids and
  finally arrest. Every sting and raid goes in the DA's file; a thick enough
  file is an indictment.
- **News** turns everything into headlines and writes the morning report.

Tuning lives in `internal/content/*.toml`, not in code.

## Balance harness

```
go run ./cmd/balance -policy managed -runs 50 -days 200
go run ./cmd/balance -policy aggressive -seed 7 -trace
```

Policies: `idle`, `quiet`, `normal`, `aggressive`, `careful`, `managed`.
The tests in `internal/harness` assert the shape of the difficulty curve:
always-aggressive is indicted within 40 days, always-quiet survives, and a
player who sells normally but lies low when hot out-earns both.

## Layout

```
cmd/kingpin/        the game
cmd/balance/        headless balance tool
internal/events/    event types and bus
internal/game/      world state, clock, player actions, save/load
internal/sim/       simulations: market, heat, news
internal/content/   embedded TOML tuning and headline templates
internal/harness/   headless runner and balance tests
internal/ui/        Bubble Tea screens, dialogs, theme, sparklines
```

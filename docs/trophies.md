# Trophies, the rot and the rich list: the money shows

**The money shows** (#392). The export lanes (#391, `docs/exports.md`) put the cartel's net worth in the billions; this is where that shows. Nothing here moves a number below the cartel: tiers 1 to 4 stand to the dollar (`TestMoneyCurve`), and `TestSeedDigest` moved by shape alone.

## Trophies

**Trophies are clean cash spent to be seen spending it** (`internal/content/trophies.toml`, `content.TrophiesConfig`, `game/trophies.go`, `sim/laundering/trophies.go`, `ui/trophies.go`).

Each `[[trophy]]` has:
- `id`, `name`, `blurb` (the pane's line);
- `unlock_cash`: the peak clean cash that puts it on offer, as an asset's;
- `cost`: clean cash;
- `fear`, `respect`, `notoriety`: added to each axis every day it is owned (0 to 2 a day each, so one trophy cannot pin an axis).

The file:

| Trophy | Cost | Opens at | A day while owned |
|---|---|---|---|
| The Penthouse | $10M | $5M | respect 0.15, notoriety 0.05 |
| The Yacht | $40M | $15M | respect 0.25, notoriety 0.15 |
| The Private Zoo | $80M | $30M | fear 0.3, respect 0.1, notoriety 0.25 |
| A Football Club | $200M | $80M | respect 0.5, notoriety 0.2 |
| A Senator | $500M | $200M | fear 0.2, respect 0.3 |

**Buying** is `World.BuyTrophy(o TrophyOffer)`, priced by `laundering.Sim.TrophyOffers` (cheapest first) and served as `Session.BuyTrophy(id)` and `TrophyOffers()`, the wire's `buy_trophy` and `trophy_offers` (protocol 12). It is refused:
- under the line (`nobody sells X to somebody who has not held $Y clean`);
- short of clean cash (dirty never pays: `ShortError` on the clean pool);
- already owned (`ErrTrophyOwned`).

**While it is yours:**
- It counts in `NetWorth` at cost.
- The laundering sim reports it the morning after (`TrophyBought`: a `laundering` headline off `game.StreamTrophiesNews`, so being seen counts toward notoriety as the paper's headlines do; the MONEY line `Bought X -$Y clean. The whole city has heard.`, booked to investments).
- The reputation sim adds its axes every night from its own copy of the file (`reputation.Sim.trophies`), before the fade. A trophy holds the street's opinion up; the fade still pulls it home.

**The task force takes the costliest thing you own** (`heat.Sim.seize`): the asset it always took (the one in the city it came to, else the costliest), unless a trophy cost more, or there is no asset.
- Taking a trophy emits `TrophySeized` (a `heat` headline off the trophies' stream; the HEAT line `they took X: $Y of yours, gone, and on the evening news.`; a fast-forward stop, `the feds took X`; the `task_force` cue).
- The laundering sim, which owns the trophies (`TestSimsWriteOnlyTheirOwnState`), moves it to `World.TrophiesLost` (`LoseTrophy`). It is for sale again at the price.
- A trophy does not make the task force eligible: it still forms only against an asset or the pile (`docs/assets.md`).

**The state**: `World.Trophies` and `World.TrophiesLost` (`Trophy{ID, Name, Cost, Bought, Lost, Why}`); `Stats.Trophies`, `TrophyCash`, `TrophiesLost`. Zero is the run before, so no schema bump.

**The UI**: the ledger's **TROPHIES** block under EXPORTS, shown once a trophy is owned or lost or the first line is within `engine.GateNear` on peak clean cash (`trophiesShown`), so a run nowhere near the money reads the ledger it always did.
- The rows are the trophies owned (`yours since d240`), then the offers (`locked · $X clean to go`, `short $X clean`, `open to you · t to buy`), under the one cursor (`ledgerTrophy`, `ledgerTrophyOffer`); the columns are `name`, `cost`, `status`.
- The pane gives the blurb, the cost, what it adds a day (`talk`) and the task force's rule.
- **`t` on an open offer** (`ledgerOnTrophyOffer`; `t` is the lane's export order too, each row its own) asks first (`modeConfirm`, `y buy`) and buys. A locked one is refused before the question.

`TestTrophiesInTheGrammar` walks it at the three sizes.

## The rot

**A dirty pile nobody can keep dry rots** (`laundering.toml [laundering] rot_line` $50M and `rot` 0.001).
- Every night, after the wash, `rot` of what is over `rot_line` comes off the dirty pile (`laundering.Sim.rot`, `Rot(pile)`; `Stats.Rotted`).
- It emits `CashRotted` (report-only; the MONEY line `Rats and damp took -$X of the pile: $Y dirty, Z tonnes of it, and nowhere dry to keep it. Wash it.`, booked to losses).
- No dice. `rot_line = 0` turns it off.

Escobar's accountants wrote off a tenth a year to the damp; this is a third of that a month, on the excess alone, so washing is the answer and the casino and the bank (`docs/laundering.md`) are what it is for. No policy below the cartel keeps a pile over the line, so no pinned number below it moves. The cartel's day-300 median moved $1.690B → $1.679B on the money curve's twenty seeds (`docs/tuning.md`).

**The weight**: `format.CashWeight(n)` is a pile in hundred-dollar bills, a bill a gram (`450 kg`, `1.5 tonnes`, `12 tonnes`). Once dirty cash is over $10M, the ledger's header line under the piles reads it, and the rot a night once over the line (`pileLine`).

## The rich list

**Net worth crossing a line makes the paper** (`headlines.toml [richlist]`: `lines` $100M, $500M, $1B and $10B; `rank_scale` 1e11; `news.Sim.richList`).
- The news sim stamps the next line the morning net worth is over it, one a morning like the tiers, each once a run (`Progression.Rich`, the lines crossed).
- It emits `RichListed{Line, NetWorth, Rank}`. The rank is `rank_scale` over net worth, never under one: $100M is #1,000, $1B #100, $10B #10.
- It writes a `news` headline off `game.StreamRichNews` (not a source notoriety counts), and a TIER line `THE RICH LIST puts you at #N, worth $X.`
- No dice but the template's.

The boss crosses the first line about day 215 (seed 7); nothing but the journal moves. `TestRichListOneAMorning`.

## Tests

`TestTrophiesAreCleanMoney`, `TestDirtyPileRots` (`sim/laundering`), `TestTaskForceTakesATrophy` (`sim/heat`), `TestTrophiesTalk` (`sim/reputation`), `TestRichListOneAMorning` (`sim/news`), `TestCashWeight` (`format`), `TestTrophiesInTheGrammar` (`ui`).

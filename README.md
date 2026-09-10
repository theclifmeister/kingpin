# Kingpin

A terminal game about building a drug empire one day at a time while the
heat closes in. Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> You start with $500, a burner phone and a corner nobody wants. Everything
> after that is your fault.

The full design is in [issue #1](https://github.com/theclifmeister/kingpin/issues/1).
This is the Phase 3.2 build: two cities of corners to hold, a six-rung product
ladder the supplier opens up as your money grows, a market in each city that
reacts to you, a police force in each that reacts to how much you move
there, a crew that moves product for you as long as you keep them paid, an
upgrade tree to sink the money into, fronts that wash the money once there
is too much of it to sit on, and a road between the cities that runs on a
dial you set once: what is cheap at one end and dear at the other travels
it every day, and the police can take it.

## Play

```
go run ./cmd/kingpin
```

Needs Go 1.24 and a terminal at least 80x24. A single save slot lives in
`$KINGPIN_HOME` or your platform config directory under `kingpin/`; the game
autosaves at the end of every day. Saves from older builds are upgraded on
load; a save from a newer build than the one you are running is refused.

| Key | Action |
|---|---|
| `1`–`8` / `tab` | Dashboard, Market, Journal, Crew, Map, Upgrades, Ledger, Rivals (`shift+tab` goes back) |
| `↑` `↓` `←` `→` | Move the cursor: up and down a list, across the map grid and the upgrade columns, between the cities on the market; the dial in a dialog |
| `[` `]` | Turn the market and the map to the other city |
| `b` | Buy from the supplier where you are (blank quantity = as much as you can); on the ledger, buy a front |
| `s` | Queue a street sale and set the dial: quiet / normal / aggressive, in the city shown |
| `r` / `R` | On the map: turn the selected route's dial (off / slow / normal / fast) / set what it keeps the far city stocked with (`↓` past the grid reaches the routes) |
| `t` | On the crew screen, give the selected lieutenant a city to run |
| `g` | Go to the other city, after a confirmation that names the corner you leave; your corner and your stock stay behind |
| `x` | Cancel the queued order on the selected product |
| `l` | Lie low today: no sales, heat fades faster |
| `h` / `f` | Hire / fire the selected person (crew screen); on the ledger, `f` funds a city with clean cash |
| `i` / `$` | Investigate who is talking / pay off the selected person (crew screen) |
| `p` | Cycle crew pay: stingy / fair / generous |
| `c` / `e` / `a` | Post a runner / an enforcer / abandon the selected corner (map) |
| `u` / `enter` | Buy the selected upgrade, after a confirmation (upgrades) |
| `d` | Cycle the launder dial: careful / normal / greedy; on the rivals screen, propose a deal |
| `y` / `x` | Accept / decline the selected offer (rivals screen) |
| `n` | End the day |
| `enter` | End the day, after a confirmation |
| `r` | Reopen the morning report (anywhere but the map) |
| `?` | Help |
| `q` | Save and quit |

## How it works

Every day the simulations step in a fixed order (`market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`),
each reading the world and emitting typed events that later sims and the UI
consume. Randomness is derived from the run seed and the day number, so a
run is fully reproducible and nothing about the RNG needs saving; what
happens away from home rolls on its own side of the stream, so a run that
never leaves the first city plays the same as it always did.

- **Cities.** Everything happens in one of two cities, each with its own
  street prices, its own corners and its own police. Eastside is home;
  Bayport is the port down the coast, where coke and heroin come off the
  boats cheap and nobody much wants them, weed and pills cost more, and
  the police watch the water. You are in one city at a time: the supplier
  sells to you where you stand, into a stash there, and you can only work
  a corner yourself where you are; runners sell where they are posted.
  `g` moves you, for what only you can do there; the routes move the
  stock.
- **Market** drifts prices toward an equilibrium with noise, rolls supply
  shocks and demand slumps, and resolves your sell orders, city by city.
  Selling into demand barely moves the price; flooding past it craters it.
- **Logistics** is the road between the cities: a car, a truck and a boat,
  each a different point on the speed / cost / risk triangle, and each a
  **dial** you set once on the map: off, slow, normal or fast, with a
  target stock for the far end per product. Every day a route that is on
  sends what Eastside is short of its target, up to what the route
  carries, out of the Bayport stash first and then by the lot from
  Bayport's wholesaler once your peak cash says you can move weight,
  spending only what is over the float the fronts leave in the till. A
  shipment rides the route for its days (the dial trades days against the
  chance of a seizure on each) and lands in the other stash unless the
  police take it, in which case every unit is gone, heat rises in both
  cities, the street that was waiting for it spikes, the route sends it
  again tomorrow, and, if it was sent fast, the DA gets a page. The
  morning report reads as a supply line, and the first seizure is the cue
  to turn the dial down.
- **Territory** is each city's corners. Each is its own demand pool and only
  a corner somebody stands on sells; a runner holds one for you, an enforcer
  keeps it from being robbed, and a corner nobody works drifts back to the
  street. The rival fights over Eastside.
- **Crew** are runners, enforcers and accountants you hire from a rotating
  pool. Runners raise how much you can hold and how much of the street you
  reach; accountants put more through every front and keep the auditors
  away. The pay dial trades wages for loyalty; loyalty also falls with
  greed, danger and firings. Below a threshold a member skims the takings,
  or the wash (the report says so without naming names); lower still, the
  nervous start talking to the police, feeding the DA's file every few days
  whatever you sell, and a raid goes straight to your stash. The roster
  never shows it: the tell is a file that grows without a bust and a heat
  delta the dial does not explain, and after two of those the screens hint
  at it. Investigating (`i`) names them with odds that scale with your best
  enforcer's skill; firing them stops it. An audit turns a disloyal
  accountant the same way. At the floor a member walks, or,
  while the rival holds ground, defects to it and walks it onto the corner
  they ran.
- **Lieutenants** run a city for you. Once you hold corners in both
  cities, one turns up in the hiring pool now and then; give them a city
  (`t` on the crew screen) and every night they post your idle runners
  on its best corners, give up a corner after its second stick-up, and
  sell everything stashed there at the dial their temper favours: a
  **violent** one sells aggressive and runs the city hot, a **greedy**
  one skims on top of the cut, a **careful** one sells quiet and earns
  less, a **steady** one just runs it. You learn which after ten days on
  the job. They keep a cut of the city's takings, bring people of their
  own (the roster grows while they run a city), and your own order for a
  product there wins the day. Watch their loyalty more than anyone's:
  under the line they turn informant with no dice and feed the DA thick
  pages; at the floor they walk with the city, every corner they ran and
  the stash there.
- **Rivals** is the other crew in the city: one per run, with a leader and
  a temperament drawn from the seed. It moves in on a free corner, claims
  more, pushes on the corners of yours it borders, undercuts you there and
  calls the police when you hurt it. Enforcers on the war dial (warn /
  push / hit) are one answer; the table is the other. It keeps a **trust**
  in you, seeded by its temperament, and you can propose a **truce** (a
  term of peace: no pushes, no undercutting, no tips), **tribute** (you pay
  a cut a day and it leaves your corners alone) or a **territory split** (a
  line through the city, each side keeping to its own). It answers in the
  morning with odds the dialog shows, built from the deal, the terms, its
  trust, its temperament, how loud the war is and how feared you are; it
  makes offers of its own when its situation calls for one, and they stand
  a few days. Every day a deal holds earns trust (and a kept peace earns
  respect); a strike costs it; a push or a hit under a deal, a missed
  tribute or walking off a split corner is a **betrayal**: trust falls to
  the floor, it makes one call to the police, and it takes nothing for a
  month. A chaotic rival breaks deals on a whim; a defensive one never.
  Joint shipments wait on routes.
- **Heat** is per city: it rises with the volume you *tried* to move there
  and how loud the dial was, plus a little, where you are, for sitting on a
  pile of dirty cash. Units your crew moves count at a discount, but sloppy
  low-skill runners add a premium. It decays slowly, faster if you lie low.
  The hottest city's police answer at the thresholds: patrols, stings,
  raids and finally arrest, and what they take comes out of the stash
  there. Every sting and raid goes in the DA's file, which is yours
  wherever you are; a thick enough file is an indictment.
- **The law** has faces. A **police chief** with a temperament drawn from
  the seed and hidden until you have seen them work: a **zealous** one
  sends the stings and raids back sooner and lets heat fade slower, a
  **lazy** one the opposite and their patrols let more through, a
  **corrupt** one is neutral for now. They serve a term and the mayor
  names another. A **DA** elected every ninety days on a ticket: a
  **law-and-order** DA needs a thinner file to indict and stings sooner,
  a **reformer** the reverse, a **moderate** runs the courthouse by the
  book. Who wins is decided by **public pressure**, a number every city
  carries: violence, hard product (heroin, meth, designer) and headlines
  about you push it up, it fades on its own, and a loud city gets its
  police answering sooner (patrols, raids, the arrest line), gets the
  rival's phone calls returned, and elects a crackdown DA, who may fire
  the chief on the way in. Clean cash buys **goodwill** (`f` on the
  ledger): community centres, campaigns, benevolent funds, which take the
  pressure off a little every day. Dirty money is not welcome. The
  dashboard's LAW panel shows all of it, and the report carries elections
  and new chiefs. The law never adds a page to the file by itself: it
  moves the thresholds, the cooldowns and the decay.
- **Upgrades** are three branches of persistent, stacking bonuses bought
  with cash: Operations (stash, supplier, street network) to earn more,
  Security (burners, lookouts, safehouse, cold contacts) to take less
  damage and cool faster, Legal (lawyer, retainer, fall guy) to survive the
  case. Every node is a multiplier the sims read from `upgrades.toml`;
  nothing removes the heat curve, it only softens it. The retainer costs
  clean cash: money the fronts have washed.
- **Laundering** is the fronts: a laundromat, a car wash, a restaurant, a
  nightclub, a construction firm, a crypto exchange. Each washes dirty cash
  clean up to a daily cap for a daily upkeep, and each can be audited. The
  launder dial pushes them all harder or softer: greedy washes more, gets
  audited more, and an audit of a front run greedy goes in the DA's file.
  The wash always leaves a float in the till for the street. Dirty cash
  over the threshold is heat every day it sits there; clean cash is what
  the retainer, and the endgame, ask for.
- **News** turns everything into headlines and writes the morning report,
  and every five to eight days deals a **dilemma card**: an enforcer who
  wants to hit the rival's stash, a detective with a file to lose, a
  reporter on your corner, your mother on the phone. The card is shown
  before the report, `1`–`3` or `enter` decide, the effects land at once
  and the outcome goes in the journal. Quit on a card and it is waiting
  when you come back. The deck is `internal/content/dilemmas.toml`.

Tuning lives in `internal/content/*.toml`, not in code.

## Balance harness

```
go run ./cmd/balance -policy managed -runs 50 -days 200
go run ./cmd/balance -policy aggressive -seed 7 -trace
```

Policies: `idle`, `hide`, `quiet`, `normal`, `aggressive`, `careful`,
`managed`, `upgraded`, `crewed`, `vigilant`, `territory`, `war`, `diplomat`,
`laundered`, `distributor` (turns the biggest route into Eastside on with a
target of a few days of demand, moves to Bayport once the wholesaler deals
and sells at both ends),
`delegated` (the distributor with a lieutenant running Eastside; `-lt
violent|greedy|careful|steady` forces their temper), `funded` (the
laundered player who pays the town whenever the pressure is up).
`-chief corrupt|zealous|lazy` and `-da law_and_order|moderate|reform` hold
the law fixed for the run.
`-own stash,burners` starts every run owning those upgrades; `-snitch` starts
it with an informant on the payroll; `-cards decline|first` deals the
dilemma cards and answers each with its last (do-nothing) or first choice
(by default the harness plays without them, so its numbers measure the sims,
not the deck). The tests in `internal/harness` assert
the shape of the difficulty curve: always-aggressive is indicted within 40
days, always-quiet survives, a player who sells normally but lies low when
hot out-earns both, one who spends on the tree out-earns that, one who builds
a crew out-earns that, one who also washes the money out-earns *that* and is
never indicted for sitting on the pile, the Security branch buys an
aggressive player time without buying them out of the indictment, and an
informant nobody looks for indicts the always-quiet player within
`harness.SnitchDays` while one who reads the report (`vigilant`) survives.
`TestMoneyCurve` pins the scale per tier. `diplomacy_test.go` pins the
table: a truce holds and then lapses, the `diplomat` keeps more ground than
the passive player and runs cooler than a war, a defensive rival never
breaks a deal and a chaotic one does, and the table survives a save.
`lieutenant_test.go` pins delegation: the delegated player's home sells on
the lieutenant's orders alone, the four tempers pull their way on one seed,
a lieutenant at the floor walks with the city, your order beats theirs, a
flip is dice-free, and `delegated` at steady stays within 20% of
`distributor`. `law_test.go` pins the law: a zealous chief indicts the
aggressive trader sooner than a lazy one, a law-and-order DA the hot crewed
player sooner than a reformer, a hit war is louder than holding ground,
the funded player ends quieter than the laundered one on clean money
alone, loud cities elect law-and-order, and the quiet-day rule holds under
every chief and DA.

## Layout

```
cmd/kingpin/        the game
cmd/balance/        headless balance tool
internal/events/    event types and bus
internal/game/      world state, clock, player actions, save/load
internal/sim/       simulations: market, logistics, territory, rivals, crew, heat, law, laundering, reputation, news
internal/content/   embedded TOML tuning, names and headline templates
internal/harness/   headless runner and balance tests
internal/ui/        Bubble Tea screens, dialogs, theme, sparklines
```

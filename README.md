# Kingpin

A terminal game about building a drug empire one day at a time while the
heat closes in. Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).

> You start with $500, a burner phone and a corner nobody wants. Everything
> after that is your fault.

The full design is in [issue #1](https://github.com/theclifmeister/kingpin/issues/1).
This build has two cities of corners to hold, a six-rung product ladder the
supplier opens up as your money grows, a market in each city that reacts to
you, a police force in each that reacts to how much you move there, a chief
and a DA with faces and terms, a rival crew that fights over the home city
and sits at the table, a crew that moves product for you as long as you
keep them paid and lieutenants who run a city on their own, an upgrade tree
to sink the money into, fronts that wash the money once there is too much
of it to sit on, a road between the cities that runs on a dial you set
once, buyers who want product off-corner on a deadline, a reputation the
street keeps on you, and a dilemma card every few days.

## Play

```
go run ./cmd/kingpin
```

Needs Go 1.24 and a terminal at least 80x24. Three save slots live in
`$KINGPIN_HOME` or your platform config directory under `kingpin/`
(`save1.gob` to `save3.gob`; a `save.gob` from an older build is slot 1). The
start menu lists them (`Slot 1 · day 42 · $1.2M · Eastside · saved 2h ago`,
`Slot 2 · empty`): `enter` continues the run in a slot or starts one in an
empty slot, `D` empties a slot after a confirmation, and `go run ./cmd/kingpin
-slot 2` opens a slot without the menu. The game autosaves at the end of every
day into the slot the run came from; `N` starts over in the same slot. Saves
from older builds are upgraded on load; a save from a newer build than the one
you are running is refused.

## Layout

Every screen is the same frame. The **title bar** on the top row carries
the game's name, the eight screens as tabs (`1`–`8`, `tab`) with the
journal's unread count beside its name, and the day, the dirty cash and the
heat where you stand (the clean cash and the city where the width allows).
The **status bar** on the bottom row is your last action's reply: a
confirmation, a refusal (`Can't hire: …`) or a danger in red, with
`? help` at the right. Between them is the body: **MAIN**, the screen
itself, and, from 100 columns, the **DETAILS** pane beside it, which
holds whatever is under the cursor (the product, the corner, the route,
the person, the node, the front, the offer), the facts about it, what
the keys would do to it and, last, the KEYS section, the keys the screen
takes in the order they matter (`n end day` first, `? help` last).
`space` hides the pane and shows it again. Under 100
columns the pane collapses to the **details strip**, one line above the
status bar with the selection's name and its first facts, and `space`
opens the pane whole as an overlay over MAIN; `esc` or `space` closes
it. Dialogs, pickers, confirmations, the morning report, the dilemma card
and help are one **modal** box, 76 columns at most, with the keys it takes
in its footer; a body taller than the box scrolls. Inside a modal `enter`
confirms and closes and never ends the day; on the play screen `enter`
asks first and `n` ends it at once. Back is one key and close is one
key: `esc` closes any modal whole, from whatever step a dialog is on,
and `shift+tab` goes back a step in a dialog with steps (the buy, sell,
target, cart and propose dialogs), keeping what the earlier steps hold;
`tab` goes forward once the step is complete. On a screen `tab` and
`shift+tab` are the next and the previous screen.

The keys never move: a key does the same thing everywhere it works, and a
key pressed on a screen that does not take it says which screen does
(`Hire on the crew screen (4).`). A key is listed where it is used: the
globals (`b buy`, `s sell`, `p pay dial`, `g go to <city>`, …) work on
every screen and are in the KEYS of the screens they belong to.

## Screens

1. **Dashboard** — the street where you stand: the product table with the
   cursor, the stash and the corners, the crew, what is elsewhere and on
   the road; then HEAT (the gauge with the police thresholds marked, the
   DA's file and, from 100 columns, the reputation bars), CASH (dirty,
   clean and the day's wash), LAW (the chief, the DA, the pressure) and
   RIVALS (the leader, their corners, the war and the trust); the pane has
   the alerts and the selected product with what `s` would sell.
2. **Market** — the shown city's prices, supplier, stash, demand and your
   orders; `←→` turns it to the other city; the BUYERS under the table are
   the people who want product off-corner, with their own cursor.
3. **Journal** — every headline, newest first, in the colour of the sim
   that wrote it; the pane shows the one under the cursor whole.
4. **Crew** — the payroll and the faces looking for work, the pay dial,
   and in the pane the person: loyalty against the lines, wage, post,
   temper, and what hiring, firing, paying off and asking around cost.
5. **Map** — the shown city's corners as a grid (yours in blue, the
   rival's in purple, free ones plain) and the routes between the cities
   under it, each with its dial; the pane is the corner's inspector or
   the route's detail.
6. **Upgrades** — the three branches of the tree as columns; the pane is
   the node, its cost, what it needs and what it does.
7. **Ledger** — the till (dirty, clean, seized, the launder dial), the
   fronts, the routes' books and the fronts on offer, under one cursor;
   the pane is the selected front, route or offer and the wash.
8. **Rivals** — the rival's leader, trust and war, the deals that hold and
   the offers on the table; the pane is the deal or the offer, the rules
   of the table and the lifetime numbers.

The dashboard at 80x24, the smallest terminal the game takes (the strip
above the status bar stands in for the pane):

<!-- capture:dashboard-80x24 -->
```text
 KINGPIN  1  2  3  4  5  6  7  8                  Day 4 · dirty $465K · heat 12
╭─ STREET · Eastside ──────────────────────────────────────────────────────────╮
│   product     price     Δ  5d       stock  order                             │
│ ▸ Weed       $19.23   -6%  ▄▇▁█▁       40  -                                 │
│   Pills      $44.21  -14%  ▁▂▆█▁        0  -                                 │
│   Coke      $153.32   -1%  ▁▂▂█▇ ▲      0  -                                 │
│   Heroin    $375.84   -5%  █▁           0  -                                 │
│   Meth      $701.73  -12%  █▁           0  -                                 │
│   Designer   $2,740  +11%  ▁█           0  -                                 │
│ stash 40/310 · corners 3 worked, 3 held of 10, 1 theirs                      │
│ 240 units in Bayport · 60 units on the road, next in 2d                      │
│ crew 5 · fair pay $440/day · skimming suspected                              │
│ 1 offer on the market screen (2)                                             │
│ no upgrades yet: buy on the upgrades screen (6)                              │
│ No sales queued. Press s to sell, n to end the day.                          │
╰──────────────────────────────────────────────────────────────────────────────╯
╭─ HEAT ───────────────────╮╭─ CASH ──────────────╮╭─ LAW ─────────────────────╮
│ ███░░░░░░┆░░░┆░░░┆░░░░┆░ ││ dirty  $465K        ││ Chief Whitfield · new     │
│ 12/100 peak 12 file 0/7  ││ clean  $50K +$19K   ││ DA Bell · reform          │
│ patrol 40 · sting 58     ││ peak   $700K        ││ pressure ░░░░░░░░ 2       │
│ raid 75 · arrest 95      ││ Bayport heat 0      ││ Mona · 1 corner           │
╰──────────────────────────╯╰─────────────────────╯╰───────────────────────────╯
▸ WEED · EASTSIDE · price $19.23 -6% · supplier $10.67 · margin 80% · s…  ␣ more
                                                                         ? help
```
<!-- capture:end -->

The map at 120x40, with the pane beside it (the cursor is on the rival's
corner; the pane says what the enforcers' odds are):

<!-- capture:map-120x40 -->
```text
 KINGPIN  1 Dash  2 Market  3 Journal 12  4 Crew  5 Map  6 Upgr  7 Ledger  8 Rivals       Day 4 · dirty $465K · heat 12
MAP · Eastside  [ ◉ Eastside ]  Bayport  3/10 held · 3 worked · ~1828/day free      ╭─ DETAILS ────────────────────────╮
 ▴ THE DOCKS         ▪ RAIL YARD         ▪ OLD MILL                                 │ THE DOCKS                        │
   theirs              Dre                 Gato ⚔ Moose                             │ Mona's since day 0               │
   ~231/day quiet      ~130/day quiet      ~109/day quiet                           │ holds       1 corner             │
 ▪ FOURTH & MAIN     · BUS DEPOT         · THE PROJECTS      · PRECINCT ROW         │ size        ×1.2                 │
   you                 free                free                free                 │ heat        ×0.6 quiet           │
   ~142/day undercut   ~142/day warm       ~326/day average    ~269/day hot         │ risk        ×1.6 rough           │
                     · THE STRIP         · RIVERSIDE         · THE HEIGHTS          │ demand      Weed ~75             │
                       free                free                free                 │             Designer ~52         │
                       ~357/day warm       ~249/day average    ~254/day warm        │             Pills ~44            │
                                                                                    │             Heroin ~37           │
ROUTES                                                                              │             Coke ~17 · Meth ~5   │
▸ Coast Road   Bayport  ──car──▶ Eastside  normal  2d · 60 units · $8/u · ~0%  ▪60  │ runner      nobody               │
  Interstate   Bayport  ─truck─▶ Eastside  off     3d · 400 units · $3/u · ~9%      │ enforcer    nobody               │
  The Channel  Bayport  ─boat──▶ Eastside  off     5d · 2000 units · $1/u · ~7%     │ w  push takes it ~8%, hit ~19%   │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │                                  │
                                                                                    │ KEYS                             │
                                                                                    │ n  end day      ↑↓←→ pick        │
                                                                                    │ [ ] city        c  post runner   │
                                                                                    │ e  post enforcer                 │
                                                                                    │ a  abandon                       │
                                                                                    │ w  send enforcers                │
                                                                                    │ r  route dial   R  route target  │
                                                                                    │ g  go to Bayport                 │
                                                                                    │ ␣  details      ?  help          │
                                                                                    ╰──────────────────────────────────╯
                                                                                                                 ? help
```
<!-- capture:end -->

Both captures are the test fixture on a fixed seed:
`go test ./internal/ui -run TestReadmeCaptures -update` renders them into
this file, and the same test without the flag fails until the README
matches the screens.

## Keys

The keys are one table, `internal/ui/keys.go`: the details pane's KEYS
section, the modal footers, the help modal (`?`) and this table are
rendered from it (`go run ./cmd/keys -w` rewrites this section; the test
holds it to the code). The help modal has a `WORDS` group too: the dial,
the float, the file, drift, the pane and the strip in a line each.

<!-- keys:begin -->
| Key | Legend | What it does | Where |
|---|---|---|---|
| `n` | end day | end the day: the sims step and the run saves | everywhere |
| `↑↓` | pick | move the cursor (j and k move it too) | everywhere |
| `[ ]` | city | turn the market or the map to the other city | everywhere |
| `b` | buy | buy from the supplier where you stand | everywhere |
| `s` | sell | queue a street sale in the city shown | everywhere |
| `x` | cancel order | cancel the order on the selected product | everywhere |
| `l` | lie low | lie low today: no sales, heat fades faster | everywhere |
| `p` | pay dial | the pay dial: stingy, fair, generous | everywhere |
| `d` | launder dial | the launder dial: careful, normal, greedy | everywhere |
| `g` | go to \<city\> | go to the other city; the stock stays put | everywhere |
| `r` | report | reopen the morning report | everywhere |
| `␣` | details | show and hide the details | everywhere |
| `?` | help | this list | everywhere |
| `enter` | end day | end the day, after a confirmation | everywhere |
| `1-8` | switch screen | the screens in the title bar's order | everywhere |
| `tab` | next screen | next screen; shift+tab back, in dialogs too | everywhere |
| `ctrl+s` | save | save now; the end of the day saves too | everywhere |
| `N` | new run | start over, after a confirmation | everywhere |
| `q` | quit | save and quit | everywhere |
| `c` | cart | the day's cart: edit its buys and orders | dashboard, market |
| `←→` | city | turn the market to the other city | market |
| `a` | accept | take the buyer's offer | market |
| `x` | decline | turn the buyer's offer down | market |
| `d` | deliver | hand the buyer what the stash here holds | market |
| `pgup pgdn` | page | page through the journal | journal |
| `h` | hire | hire the selected candidate | crew |
| `f` | fire | fire the selected member, after asking | crew |
| `t` | assign | give the selected lieutenant a city to run | crew |
| `i` | investigate | ask who is talking to the police, for a fee | crew |
| `$` | pay off | buy the selected member's loyalty | crew |
| `↑↓←→` | pick | walk the map's grid or the tree's columns | map, upgrades |
| `c` | post runner | post a runner on the selected corner | map |
| `e` | post enforcer | post an enforcer on the selected corner | map |
| `a` | abandon | give the selected corner up | map |
| `w` | send enforcers | send the enforcers at the selected corner | map |
| `r` | route dial | the selected route: off, slow, normal, fast | map |
| `R` | route target | what the selected route keeps the far end at | map |
| `u` | buy upgrade | buy the node under the cursor (enter too) | upgrades |
| `b` | buy front | buy a front through the picker | ledger |
| `f` | fund city | give a city clean cash for goodwill | ledger |
| `enter` | buy / dial | buy the offer or turn the route selected | ledger |
| `d` | propose | offer the rival a truce, tribute or a split | rivals |
| `y` | accept | take the selected offer | rivals |
| `x` | decline | turn the selected offer down | rivals |
<!-- keys:end -->

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
  It also deals the **buyers**: every few days somebody in one of the
  cities wants product off-corner, on a deadline, at a premium over that
  city's street price on the day you hand it over (a club owner who wants
  pills for the weekend, a face from out of town who wants coke in bulk).
  Offers come to the market screen and lapse in a few days; take one and
  it is yours to deliver out of that city's stash, standing there, by its
  due day. A handoff needs no corner, is not capped by a patrol, takes no
  cut for a lieutenant and draws heat at the buyer's own rate; short at
  the due day, you lose respect, gain notoriety, the buyer collects for
  the rest and stays away for a month. The premium is a bet: it is against
  the street on the day, so a slump or a spike since the buyer asked is
  yours to eat. The deck is `internal/content/buyers.toml`.
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
  The joint shipment is on the list and not yet on the table.
- **Heat** is per city: it rises with the volume you *tried* to move there
  and how loud the dial was, plus a little, where you are, for sitting on a
  pile of dirty cash. Units your crew moves count at a discount, but sloppy
  low-skill runners add a premium. It decays slowly, faster if you lie low.
  The hottest city's police answer at the thresholds: patrols, stings,
  raids and finally arrest, and what they take comes out of the stash
  there. Every sting and raid goes in the DA's file, which is yours
  wherever you are; a thick enough file is an indictment.
- **Reputation** is the face the street keeps on you: **fear** (from
  strikes and pushes), **respect** (from a generous payroll, a pay-off and
  every night a deal holds; a full delivery to a buyer too) and
  **notoriety** (from volume and every headline about you). Each does one
  thing at full strength: fear slows the rival's pushes and claims and
  sways it at the table but sets a floor heat never falls under; respect
  keeps the crew loyal and the supplier friendly; notoriety raises what a
  new hire asks and makes every unit *you* move on your own corner hotter,
  so a notorious boss gets off the corner. The street has only so much
  attention: you cannot max all three.
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
laundered player who pays the town whenever the pressure is up), `dealer`
(the crewed player who works the buyers where it stands: takes every offer
it can cover, keeps the stock aside and hands it over when the heat
allows), `boss` (the delegated player who plays the whole game: every
corner in the hub, enforcers sent in only when the odds clear a line,
fronts and the tree bought at a margin, a lieutenant fired the morning
their orders turn aggressive). `-rival none` keeps the rival out of a run,
`-heat off` switches heat off and `-pace off` gives the rival its flat
pace, so a policy's ceiling can be measured against each wall.
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
every chief and DA. `buyers_test.go` pins the contracts: a handoff never
exceeds the stash and conserves stock and cash, a cornerless player can
still work one, the premium is a bet against the day, a welsher ends
with less respect and the buyer stays away, a handoff is dealing for the
DA's file and hard-product pressure, the `dealer` out-earns `crewed`,
every buyer is dealt and reads clean, and the deck boxed changes nothing
but the contracts.

## Source

```
cmd/kingpin/        the game
cmd/balance/        headless balance tool
cmd/keys/           prints the key table above from the UI's bindings (-w writes it here)
internal/events/    event types and bus
internal/game/      world state, clock, player actions, save/load
internal/sim/       simulations: market, logistics, territory, rivals, crew, heat, law, laundering, reputation, news
internal/content/   embedded TOML tuning, names and headline templates
internal/harness/   headless runner and balance tests
internal/format/    the one place a number is written: cash, money, price, arrows, plurals
internal/ui/        Bubble Tea: the frame, the pane, the key table, the modal, the tables, the screens, the theme
```

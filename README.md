# Kingpin

You start with $500 and a corner nobody wants. Buy stock, work the street,
keep the crew paid. The police watch what you move. The rival watches where
you stand. Your own people watch the payroll.

Kingpin is a terminal game about building a drug empire one day at a time.
Two cities, six products, and a growing number of people who want a cut.
Everything after the first $500 is your fault.

## Play

You need Go 1.24 and a terminal at least 80x24. Install and start:

```sh
git clone https://github.com/theclifmeister/kingpin.git
cd kingpin
go run ./cmd/kingpin
```

Choose a save slot with `enter`. An empty slot starts a run; an occupied
one takes you back to work.

Your first day needs four keys: **`b` buy, `s` sell, `n` end day, `?` help**.
On the dashboard, buy a little Weed. Pick the product, enter a quantity
(`h` buys half of what you can afford and hold), and choose `once`.
Close the buy dialog with `esc`, then sell: pick the product, quantity,
`normal` dial and `once`. Close the dialog. The sale is queued for tonight.
Press `n` and read the morning report. You now know what sold, what you
made, and how much attention it bought you.

Watch **cash, stock and heat** on the dashboard. The details beside the
selected product show demand and the expected sale; below 100 columns,
press `space` to see them. Buying stock does not sell it. A full stash and
no order is a quiet night for everyone but your wallet.

### Saves

The game saves at the end of every day; `q` saves and quits. There are
three slots in `$KINGPIN_HOME`, or your platform config directory under
`kingpin/`: `save1.gob` to `save3.gob`. An old `save.gob` counts as slot 1.
The menu shows each run's day, cash, city and save age. `D` empties a slot
after asking; `N` starts over in the current slot after asking.

To skip the menu and open slot 2:

```sh
go run ./cmd/kingpin -slot 2
```

Older saves upgrade on load. A save from a newer build is refused.

### Animation

The title screen resolves its block art, rests and plays again with
another effect, the way Omarchy's screensaver loops effects over its
logo: `decrypt`, `print`, `wipe`, `slide`, `pour`, `rain`, `beams`,
`burn`, `vhstape` and `matrix`, never the same one twice running.
Halfway through a `decrypt`:

<!-- capture:title-80x24 -->
```text

               ██  ██  ████  ██  ██   ████╏  ┬qy²Û   Ƅä╚ķ  â╋  ┑┵
               ██ ██    ██   ███ ██  ██      ┋Ʊ  ƫ┺   łF   ŔĂñ ē┕
               ████     ██   ██████  ██ ██╤  ┇ŽNŅř    ┹▟   ƴ╻ĜĹƦ╺
               ██ ██    ██   ██ ███  ██  █à  ÷╌       Ɖ<   Ć± ▖±┋
               ██  ██   ██   ██  ██  ██  ┇a  ▉Ļ       Ĉù   ŏ¾  Ɖź
               ██  ██  ████  ██  ██   ███Śĕ  ¿ƌ      "İ╹ŷ  ┽ě  îY

  ╔══════════════════════════════════════════════════════════════════════════╗
  ║ KINGPIN                                                                  ║
  ║                                                                          ║
  ║ a drug empire, one day at a time                                         ║
  ║                                                                          ║
  ║ ▸  Slot 1 · empty                                                        ║
  ║    Slot 2 · empty                                                        ║
  ║    Slot 3 · empty                                                        ║
  ║    Quit                                                                  ║
  ║                                                                          ║
  ║ ↑↓ pick  enter select  D delete  q quit                                  ║
  ╚══════════════════════════════════════════════════════════════════════════╝




```
<!-- capture:end -->

In play, a **scene** marks a morning that matters: the day rolls over
in the title bar as the report opens; a dilemma card is dealt, its title
decrypting and its prose wiping in; a sting or a raid strobes the
report's title and glitches its level in on a bad tape; a new stage
prints in and is swept by light; a corner that changed hands overnight
burns on the map from one colour to the other and its name slides into
the pane; and a run's ending plays the DA's file, the arrest or the
empty till before the summary. Scenes are short (250 ms to 1.5 s), any
key skips one, and nothing animates while you play: the screen redraws
only when you press a key. To turn them off, for a slow terminal, a
capture, or because they wear, to turn off only the morning's (the most
frequent), or to pin the title's effect to one of the set:

```sh
go run ./cmd/kingpin -no-anim
KINGPIN_NO_ANIM=1 go run ./cmd/kingpin
KINGPIN_NO_MORNING_ANIM=1 go run ./cmd/kingpin
KINGPIN_ANIM_EFFECT=matrix go run ./cmd/kingpin
```

Under 80x24 no scene plays. To watch every scene without playing to it:

```sh
go run ./cmd/anim          # ←→ scene · r replay · e effect · 1-9 seed · q quit
go run ./cmd/anim -list    # the registry: name, length, effects, what starts it
```

The effects are ports of
[TerminalTextEffects](https://github.com/ChrisBuilds/terminaltexteffects)
(MIT; see `internal/ui/anim/NOTICE`), each readable in sixteen colours.

## Layout

The top bar tells you the day, dirty cash and local heat, with clean cash
and your city when there is room. It also holds the eight screen tabs and
the journal's unread count. Use `1`–`8`, or `tab` and `shift+tab`, to move
between screens.

The bottom bar answers your last action. A refusal tells you why; danger
shows in red. `?` opens help, including a **WORDS** glossary for the dial,
the float and the other things the street expects you to know.

At 100 columns and up, **DETAILS** sits beside the main screen. It follows
the cursor: the selected product, person, corner or offer, its facts, and
what you can do with it. **KEYS** is at the bottom, with `n end day` first
and `? help` last. In a narrower terminal, a one-line **details strip**
takes its place. `space` opens the whole pane; `esc` or `space` closes it.
The wide pane stays open wherever it fits.

Dialogs, reports and help share a box, at most 76 columns wide, with their
keys in the footer. Longer pages scroll. `esc` closes the whole box;
`shift+tab` goes back a step in a dialog, and `tab` goes forward when the
step is complete. Inside a dialog, `enter` advances or confirms. It never
ends the day. On a play screen, `enter` asks before ending it; `n` does not.

Number fields take digits and `backspace`. Use `m` (or `a`) for the maximum,
`h` for half, `↑`/`↓` for one at a time, or `pgup`/`pgdn` for ten. These
shortcuts stay within the limit shown after the number. A blank means the
maximum, except on a route target, where it means none.

Keys are listed on the screens where they belong. Global actions work
across screens unless a local action takes that key: `b` buys stock on the
dashboard and fronts on the ledger. A misplaced key points you back:
`Hire on the crew screen (4).`

## Screens

1. **Dashboard** — your stock, orders, corners and crew, with cash, heat,
   the law and the rival below. Start here each morning. Details carries
   the alerts and the selected product's sale estimate.
2. **Market** — prices, suppliers, demand, orders and buyers with deadlines.
   `←`/`→` shows the other city. Looking there does not move you there.
3. **Journal** — every headline, newest first, coloured by its source.
   `f` filters by source; details shows the selected headline in full.
4. **Crew** — who's on the payroll and who's looking for work. Details
   shows loyalty, wages, posts and the cost of hiring or asking questions.
5. **Map** — your corners in blue, theirs in purple, free ones plain.
   `?` marks the corner the rival is eyeing; `$` marks your undercut tonight.
   Routes sit below the grid, each with a `▪` on its road for every
   shipment in flight, placed by the days it has been out (`▪2` where two
   share a day). Details gives the corner or route its numbers.
6. **Upgrades** — seven branches; `←`/`→` changes branch. Each node sits
   under the one it needs. Details tells you the cost and what you get.
7. **Ledger** — dirty and clean cash, fronts, route costs and fronts for
   sale. This is where you put the money through the wash.
8. **Rivals** — the leader, trust, war, deals and offers. Details gives
   the terms. Sometimes the table costs less than the street.

The dashboard at 80x24, with the details strip above the status bar:

<!-- capture:dashboard-80x24 -->
```text
 KINGPIN  1  2  3  4  5  6  7  8                  Day 4 · dirty $452K · heat 12
╭─ STREET · Eastside ──────────────────────────────────────────────────────────╮
│   product     price     Δ  5d       stock  order                             │
│ ▸ Weed       $19.23   -6%  ▄▇▁█▁       40  -                                 │
│   Pills      $44.21  -14%  ▁▂▆█▁       30  30 normal ↻                       │
│   Coke      $153.32   -1%  ▁▂▂█▇ ▲      0  -                                 │
│   Heroin    $375.84   -5%  █▁           0  -                                 │
│   Meth      $701.73  -12%  █▁           0  -                                 │
│   Designer   $2,740  +11%  ▁█           0  -                                 │
│ carrying 46/310 · 24 in 1 house · corners 3 worked, 3 held of 10, 1 theirs   │
│ tier Distribution · 240 units in Bayport · 60 units on the road, next in 2d  │
│ crew 5 · fair pay $440/day · skimming suspected                              │
│ supply 1 contract · $895 this morning · 1 offer on the market screen (2)     │
│ no upgrades yet: buy on the upgrades screen (6)                              │
│ Standing orders sell tonight; the crew keep 5%.                              │
╰──────────────────────────────────────────────────────────────────────────────╯
╭─ HEAT ───────────────────╮╭─ CASH ──────────────╮╭─ LAW ─────────────────────╮
│ ███░░░░░░┆░░░┆░░░┆░░░░┆░ ││ dirty  $452K        ││ Chief Whitfield · new     │
│ 12/100 peak 12 file 0/7  ││ clean  $50K +$19K   ││ DA Bell · reform          │
│ patrol 40 · sting 58     ││ peak   $700K        ││ pressure ░░░░░░░░ 2       │
│ raid 75 · arrest 95      ││ Bayport heat 0      ││ Mona · 1 corner           │
╰──────────────────────────╯╰─────────────────────╯╰───────────────────────────╯
▸ CART · buying 1 line, $895 · 1 by contract · selling 1 line, ~$1,260 …  ␣ more
                                                                         ? help
```
<!-- capture:end -->

The map at 120x40. The cursor is on the rival's corner; details shows
what your enforcers' chances look like:

<!-- capture:map-120x40 -->
```text
 KINGPIN  1 Dash  2 Market  3 Journal 18  4 Crew  5 Map  6 Upgr  7 Ledger  8 Rivals       Day 4 · dirty $452K · heat 12
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
▸ Coast Road   Bayport  ──car──▪────▶ Eastside  normal  2d · 60 units · $8/u · ~0%  │ runner      nobody               │
  Interstate   Bayport  ─truck──────▶ Eastside  off     3d · 400 units · $3/u · ~9% │ enforcer    nobody               │
  The Channel  Bayport  ─boat───────▶ Eastside  off     5d · 2000 units · $1/u · ~7%│ w  push takes it ~10%, hit ~24%  │
                                                                                    │ w  boost: the till, ~$24K        │
                                                                                    │ t  tip the police: at 0 of 60    │
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
                                                                                    │ u  undercut     t  tip police    │
                                                                                    │ r  route dial   R  route target  │
                                                                                    │ g  go to Bayport                 │
                                                                                    │ ?  help                          │
                                                                                    ╰──────────────────────────────────╯
                                                                                                                 ? help
```
<!-- capture:end -->

## How a run goes

At first you work the corner yourself. Buy only what you can move, leave
some cash in the till, and read the report before doing it again. The
supplier opens the product ladder as your money grows. Better margins are
useful. So is a night the police have nothing to report.

As the takings grow, hire a runner and post them on another corner.
Now two corners can sell. Now there is payroll. An enforcer helps keep a
corner from being robbed; fair pay helps keep the people on it yours.
If money goes missing or the DA's file grows without a bust, read the
crew screen. The roster does not tell you who is talking.

The rival moves in. The report names the corner they want before they
take it, and the map marks it `?`. Post somebody there in time, fight for
it, sell cheap next door, or talk terms on the rivals screen. Winning a
corner and keeping a quiet city are separate jobs.

Eventually the dirty cash itself draws heat. Your first front washes
some of it clean each day and leaves a float for the street. The upkeep
is real; so are the audits. Accountants help. The greedy dial is always
there when you think you have solved the money problem.

Bayport is down the coast. Coke and heroin come off the boats cheap;
weed and pills cost more. Travel there with `g` when you need to do
business yourself. Your stock stays put. Set a route and a target to keep
Eastside supplied, then watch the road in the report. Once you hold
corners in both cities, a lieutenant can turn up to run one for you.
They take a cut. They also take loyalty rather seriously.

The game names these stages. A run starts as a **Corner** trader, becomes
a **Crew** with the first hire, **Territory** once $25K has moved and
the laundromat opens, and **Distribution** at $500K, when the wholesaler
sells lots to the road. The morning a stage is entered the game stops
and says so: one screen before the card and the report that names the
stage, what just opened and what the next one takes, once per stage per
run. The report opens with it too, the dashboard reads `tier Territory`
(`· new` until you have seen the stage) and the run summary says which
you reached and when. A stage once reached stays reached, and it changes
nothing by itself: it is a name for what has opened.

### Let the routine run

A buy's `keep at` option sets a **supply contract**: refill that city's
stash to a level each morning, at a small markup. A sale's `standing`
option repeats the same quantity and dial each night. The crew keeps 5%
of those sales. A hand-placed order wins that night; the standing order
returns the next. `x` cancels the day's order first, then the standing
order, then the supply contract.

The cart (`c` on dashboard or market) lets you edit buys and orders.
Contract buys are marked `contract`, recurring sales `standing`; the order
column marks a standing order with `↻`. You can queue sales against stock
a supply contract will bring in.

`F` fast-forwards until something needs you, up to a cap you choose
(7 days by default, 30 at most). Cards, new alerts, police action, rival
moves or offers, crew departures, audits, seizures, buyers, changes at
the courthouse, short orders and doors opening stop it. The report tells
you why: `Stopped after 3 days: contract due today.`

### Doors open on the way up

Everything the game keeps behind a line is announced the morning it
opens: a product listing, a front for sale, a connect who will deal with
you, accountants or lieutenants looking for work. The report opens with
an **UNLOCKED** section, the journal has a headline under the `unlock`
source (`f` filters to them: every door you have opened, in order) and
`F` stops. The next door is named before it opens: the dashboard alerts
once you are within half its line (`The Laundromat opens at $25K peak:
$18K to go.`), the market's notes name the next product on the ladder
and what it takes, the ledger's fronts for sale read the distance
(`locked · $18K to go`) and the crew screen says what the lieutenants
wait on.

## Keys

The full table is below; `?` brings it up in the game.

<!-- keys:begin -->
| Key | Legend | What it does | Where |
|---|---|---|---|
| `n` | end day | end the day: the sims step and the run saves | everywhere |
| `F` | fast-forward | run days until something needs you | everywhere |
| `↑↓` | pick | move the cursor (j and k move it too) | everywhere |
| `[ ]` | city | turn the market or the map to the other city | everywhere |
| `b` | buy | buy where you stand or a lieutenant runs | everywhere |
| `s` | sell | queue a street sale in the city shown | everywhere |
| `x` | cancel order | cancel order, else standing, else contract | everywhere |
| `l` | lie low | lie low today: no sales, heat fades faster | everywhere |
| `p` | pay dial | the pay dial: stingy, fair, generous | everywhere |
| `d` | launder dial | the launder dial: careful, normal, greedy | everywhere |
| `g` | go to \<city\> | go to the other city; the stock stays put | everywhere |
| `r` | report | reopen the morning report | everywhere |
| `␣` | more | open the details whole (under 100 columns) | everywhere |
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
| `t` | cut | cut a product in the stash where you stand | market |
| `o` | cook | the chemist cooks a batch where you stand | market |
| `d` | deliver | hand the buyer what the stash here holds | market |
| `pgup pgdn` | page | page through the journal | journal |
| `f` | filter | show one source's headlines, then all again | journal |
| `h` | hire | hire the selected candidate | crew |
| `f` | fire | fire the selected member, after asking | crew |
| `t` | assign | give the selected lieutenant a city to run | crew |
| `i` | investigate | ask who is talking to the police, for a fee | crew |
| `$` | pay off | buy the selected member's loyalty | crew |
| `↑↓←→` | pick | walk the map's grid | map |
| `c` | post runner | post a runner on the selected corner | map |
| `e` | post enforcer | post an enforcer on the selected corner | map |
| `a` | abandon | give the selected corner up | map |
| `w` | send enforcers | send the enforcers at the selected corner | map |
| `u` | undercut | sell cheap on the rival's corner next door | map |
| `t` | tip police | tip the police on the selected rival corner | map |
| `r` | route dial | the selected route: off, slow, normal, fast | map |
| `R` | route target | what the selected route keeps the far end at | map |
| `$` | buy checkpoint | buy the checkpoint or customs on the route | map |
| `←→` | branch | turn the tree to the next branch | upgrades |
| `u` | buy upgrade | buy the node under the cursor (enter too) | upgrades |
| `b` | buy front | buy a front or rent a house | ledger |
| `m` | move stock | move stock between the street and the houses | ledger |
| `e` | guard house | post an enforcer inside the selected house | ledger |
| `x` | drop house | drop the selected house, after asking | ledger |
| `$` | bribe | an envelope for the chief or the DA | ledger |
| `f` | fund city | give a city clean cash for goodwill | ledger |
| `enter` | buy / dial | buy the offer or turn the route selected | ledger |
| `d` | propose | offer the rival a truce, tribute or a split | rivals |
| `y` | accept | take the selected offer | rivals |
| `x` | decline | turn the selected offer down | rivals |
| `i` | scout | buy a look at the rival's books | rivals |
| `$` | buy off | pay the rival's muscle to go home | rivals |
<!-- keys:end -->

## How it works

For implementation details, invariants and test coverage, see [CLAUDE.md](CLAUDE.md).
Tuning lives in `internal/content/*.toml`. These are the systems behind a run.

Every day the simulations step in a fixed order (`world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`),
each reading the world and emitting typed events that later sims and the UI
consume. Randomness is derived from the run seed and the day number, so a
run is fully reproducible and nothing about the RNG needs saving; what
happens away from home rolls on its own side of the stream, so a run that
never leaves the first city plays the same as it always did.

### Cities

Everything happens in one of two cities, each with its own
street prices, its own corners and its own police. Eastside is home;
Bayport is the port down the coast, where coke and heroin come off the
boats cheap and nobody much wants them, weed and pills cost more, and
the police watch the water. You are in one city at a time: the supplier
sells to you where you stand, into a stash there, and you can only work
a corner yourself where you are; runners sell where they are posted.
`g` moves you, for what only you can do there; the routes move the
stock.

### Market

Market drifts prices toward an equilibrium with noise, rolls supply
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

It fills **supply contracts** every morning before anything sells: a
contract keeps the stash in a city at a level, bought from the
supplier there at a small markup, out of dirty cash, through the same
path as a buy by hand, so the supplier reacts to it exactly as to you;
short of cash or room it buys what it can and the report says so.

It resolves the **standing orders** every night where you placed no
order of the day: the same units at the same dial, through the same
path as an order by hand, for at most what is stashed, at the crew's
cut of the take (`[standing] cut` in `market.toml`); one short of stock
says so in the report.

### Logistics

Logistics is the road between the cities: a car, a truck and a boat,
each a different point on the speed / cost / risk triangle, and each a
**dial** you set once on the map: off, slow, normal or fast, with a
target stock for the far end per product, in units or in **days of
demand** (what the corners you hold there sell in that many days,
sized again every morning, so the target follows the ground you hold;
the dialog says what the days mean today, `3d ≈ 180 units`).

Every day a route that is on sends what Eastside is short of its target, up to
what the route carries, out of the Bayport stash first and then by the lot from
Bayport's wholesaler once your peak cash says you can move weight,
spending only what is over the float the fronts leave in the till.

A shipment rides the route for its days (the dial trades days against the
chance of a seizure on each; the map's route line shows where it is,
`Bayport ─truck──▪───▶ Eastside`, and the route's details say `day 2 of
3`) and lands in the other stash unless the
police take it, in which case every unit is gone, heat rises in both
cities, the street that was waiting for it spikes, the route sends it
again tomorrow, and, if it was sent fast, the DA gets a page. The
morning report reads as a supply line, and the first seizure is the cue
to turn the dial down.

### Territory

Territory is each city's corners. Each is its own demand pool and only
a corner somebody stands on sells; a runner holds one for you, an enforcer
keeps it from being robbed, and a corner nobody works drifts back to the
street. The rival fights over Eastside.

### Crew

Crew are runners, enforcers and accountants you hire from a rotating
pool. Runners raise how much you can hold and how much of the street you
reach; accountants put more through every front and keep the auditors
away.

The pay dial trades wages for loyalty; loyalty also falls with
greed, danger and firings. Below a threshold a member skims the takings,
or the wash (the report says so without naming names); lower still, the
nervous start talking to the police, feeding the DA's file every few days
whatever you sell, and a raid goes straight to your stash. The roster
never shows it: the tell is a file that grows without a bust and a heat
delta the dial does not explain, and after two of those the screens hint
at it.

Investigating (`i`) names them with odds that scale with your best
enforcer's skill; firing them stops it. An audit turns a disloyal
accountant the same way. At the floor a member walks, or,
while the rival holds ground, defects to it and walks it onto the corner
they ran.

### Lieutenants

Lieutenants run a city for you. Once you hold corners in both
cities, one turns up in the hiring pool now and then; give them a city
(`t` on the crew screen) and every night they post your idle runners
on its best corners, give up a corner after its second stick-up, and
sell everything stashed there at the dial their temper favours: a
**violent** one sells aggressive and runs the city hot, a **greedy**
one skims on top of the cut, a **careful** one sells quiet and earns
less, a **steady** one just runs it. You learn which after ten days on
the job.

They keep a cut of the city's takings, bring people of their
own (the roster grows while they run a city), and your own order for a
product there wins the day. Watch their loyalty more than anyone's:
under the line they turn informant with no dice and feed the DA thick
pages; at the floor they walk with the city, every corner they ran and
the stash there.

### Rivals

The rival is the other crew in the city: one per run, with a leader and
a temperament drawn from the seed. It moves in on a free corner, claims
more, pushes on the corners of yours it borders, undercuts you there and
calls the police when you hurt it. A claim is **telegraphed**: the day
it picks a corner the report says so, the map marks it `?` and the
RIVALS panel reads `eyeing Riverside`; it sets up there the next night
unless somebody of yours is posted on it by then, in which case it
leaves, keeps its money and holds a grudge.

Enforcers on the war dial (warn /
push / hit) are one answer; the table is another; money is the third:
work a corner next to one of theirs and `u` on the map **undercuts** it,
tonight's orders serving a share of its customers cheap on top of your
own, which costs you margin and gluts the product, draws no heat, and
cuts what the corner earns them; starve a corner for days and they push
back or, by temper, give it up.

It keeps a **trust**
in you, seeded by its temperament, and you can propose a **truce** (a
term of peace: no pushes, no undercutting, no tips), **tribute** (you pay
a cut a day and it leaves your corners alone) or a **territory split** (a
line through the city, each side keeping to its own). It answers in the
morning with odds the dialog shows, built from the deal, the terms, its
trust, its temperament, how loud the war is and how feared you are; it
makes offers of its own when its situation calls for one, and they stand
a few days.

Every day a deal holds earns trust (and a kept peace earns
respect); a strike costs it; a push or a hit under a deal, a missed
tribute or walking off a split corner is a **betrayal**: trust falls to
the floor, it makes one call to the police, and it takes nothing for a
month. A chaotic rival breaks deals on a whim; a defensive one never.
The joint shipment is on the list and not yet on the table.

### Heat

Heat is per city: it rises with the volume you *tried* to move there
and how loud the dial was, plus a little, where you are, for sitting on a
pile of dirty cash. Units your crew moves count at a discount, but sloppy
low-skill runners add a premium. It decays slowly, faster if you lie low.

The hottest city's police answer at the thresholds: patrols, stings,
raids and finally arrest, and what they take comes out of the stash
there. Every sting and raid goes in the DA's file, which is yours
wherever you are; a thick enough file is an indictment.

### Reputation

Reputation is the face the street keeps on you: **fear** (from
strikes and pushes), **respect** (from a generous payroll, a pay-off and
every night a deal holds; a full delivery to a buyer too) and
**notoriety** (from volume and every headline about you). Each does one
thing at full strength: fear slows the rival's pushes and claims and
sways it at the table but sets a floor heat never falls under; respect
keeps the crew loyal and the supplier friendly; notoriety raises what a
new hire asks and makes every unit *you* move on your own corner hotter,
so a notorious boss gets off the corner. The street has only so much
attention: you cannot max all three.

### The law

The law has faces. A **police chief** with a temperament drawn from
the seed and hidden until you have seen them work: a **zealous** one
sends the stings and raids back sooner and lets heat fade slower, a
**lazy** one the opposite and their patrols let more through, a
**corrupt** one is neutral for now. They serve a term and the mayor
names another.

A **DA** elected every ninety days on a ticket: a
**law-and-order** DA needs a thinner file to indict and stings sooner,
a **reformer** the reverse, a **moderate** runs the courthouse by the
book. Who wins is decided by **public pressure**, a number every city
carries: violence, hard product (heroin, meth, designer) and headlines
about you push it up, it fades on its own, and a loud city gets its
police answering sooner (patrols, raids, the arrest line), gets the
rival's phone calls returned, and elects a crackdown DA, who may fire
the chief on the way in.

Clean cash buys **goodwill** (`f` on the
ledger): community centres, campaigns, benevolent funds, which take the
pressure off a little every day. Dirty money is not welcome. The
dashboard's LAW panel shows all of it, and the report carries elections
and new chiefs. The law never adds a page to the file by itself: it
moves the thresholds, the cooldowns and the decay.

### Upgrades

There are fifty-six upgrades across seven branches. Bonuses stack and
stay with the run:

- **Operations** earns more: storage, supplier terms and street trade.
- **Security** cools heat faster and softens the damage.
- **Legal** helps you survive the case.
- **Crew** makes the payroll cheaper and your people more likely to stay.
- **Laundering** washes more with less attention from auditors.
- **Logistics** moves more stock for less on the road.
- **Street** helps you hold your corners.

The nodes and their effects live in `internal/content/upgrades.toml`;
[CLAUDE.md](CLAUDE.md) covers the full tree. Nothing removes the heat
curve. Clean-cash nodes cost money the fronts have washed.

### Laundering

Laundering is the fronts: a laundromat, a car wash, a restaurant, a
nightclub, a construction firm, a crypto exchange. Each washes dirty cash
clean up to a daily cap for a daily upkeep, and each can be audited.

The launder dial pushes them all harder or softer: greedy washes more, gets
audited more, and an audit of a front run greedy goes in the DA's file.
The wash always leaves a float in the till for the street. Dirty cash
over the threshold is heat every day it sits there; clean cash is what
the retainer, and the endgame, ask for.

### News

News turns everything into headlines and writes the morning report,
and every five to eight days deals a **dilemma card**: an enforcer who
wants to hit the rival's stash, a detective with a file to lose, a
reporter on your corner, your mother on the phone. The card is shown
before the report, `1`–`3` or `enter` decide, the effects land at once
and the outcome goes in the journal. Quit on a card and it is waiting
when you come back. The deck is `internal/content/dilemmas.toml`.

### World incidents

The world changes without you. First thing every day the world sim deals
at most one **incident** from a weighted table, paced like the cards (a
gap, then rising odds, fixed for the seed): a port strike shuts the boat
routes and spikes coke at the port, a hurricane or a snowstorm closes a
road, a harvest glut makes coke cheap, a new synthetic halves the pills
crowd for two months, a celebrity overdose or a documentary turns the
pressure up, a reporter puts your name about, the DA calls a snap
election, the chief resigns, the feds open an office and heat cools at
half its pace. An incident lands on the world, never on you: a route's
closure, a market shock, a city's pressure, the law's actors. The
headline comes under the `world` source, in its own colour, and the
report opens with it; a shut route says so on the map's pane and holds
what is on it until it reopens. Headlines name names: the DA, the chief,
the rival's crew. The table is `internal/content/incidents.toml`.

### Progression

The tier a run is in (Corner, Crew, Territory, Distribution) lives in
`internal/content/progression.toml`: a name, a line on the stage, what it
opens, the trigger that enters it and the prose of the stage screen shown
the morning it is entered (`text`, a few lines; `closing` on the last
tier, what its screen says where there is no next). A tier describes the
gates it names and is never one itself: nothing in the sims reads it. The
news sim stamps it the first morning its trigger holds, one tier a
morning, and emits the headline; the harness reads each tier's checkpoint
day from the same file. What the player has been shown is on the world
(`Progression.Seen`), so a save on the stage screen reopens it.

## Balance harness

```sh
go run ./cmd/balance -policy managed -runs 50 -days 200
go run ./cmd/balance -policy aggressive -seed 7 -trace
```

### Choose a policy

Basic policies: `idle`, `hide`, `quiet`, `normal`, `aggressive`, `careful`,
`managed`, `upgraded`, `crewed`, `vigilant`, `territory`, `war`, `diplomat`
and `laundered`.

The later policies exercise a particular part of the business:

| Policy | What it does |
| --- | --- |
| `distributor` | Supplies Eastside by the biggest route, targets days of demand, then moves to Bayport when wholesale opens and sells at both ends. |
| `delegated` | Adds a lieutenant in Eastside to the distributor. |
| `funded` | Runs fronts and pays the city when pressure rises. |
| `dealer` | Runs a crew, reserves stock for buyers and delivers when heat allows. |
| `stocked` | Runs a crew with supply contracts at one day's corner demand. |
| `routine` | After $20k peak cash, uses normal standing orders that grow with the stash. |
| `boss` | Works the hub's corners, sends enforcers when odds justify it, buys fronts and upgrades at a margin, and fires lieutenants who sell aggressive. |

### Isolate a system

Use these flags to compare runs with the same conditions:

| Flag | Effect |
| --- | --- |
| `-rival none` | Keeps the rival out. |
| `-heat off` | Switches heat off. |
| `-pace off` | Gives the rival its flat pace. |
| `-lt violent\|greedy\|careful\|steady` | Forces the delegated lieutenant's temper. |
| `-chief corrupt\|zealous\|lazy` | Holds the chief's temperament fixed. |
| `-da law_and_order\|moderate\|reform` | Holds the DA's stance fixed. |
| `-own stash,burners` | Starts with those upgrades. |
| `-snitch` | Starts with an informant on the payroll. |
| `-cards decline\|first` | Answers cards with the last (do-nothing) or first choice. |
| `-incidents off` | Boxes the world's incident table (on by default). |

Cards are off by default, and the harness tests box the incidents too, so
they measure the sims without the deck or the weather; `-incidents off`
reads a pinned number.
For all policies and flags, run:

```sh
go run ./cmd/balance -help
```

### What the tests hold

The tests in [internal/harness](internal/harness) hold the difficulty curve:
quiet survives, aggression gets indicted, and managing heat, crew, upgrades
and fronts pays better than ignoring them. They also check conservation of
stock and cash, reproducible runs, save/load, deals, delegation, the law,
buyers, supply contracts and standing orders. `TestMoneyCurve` pins the
scale per tier at each tier's checkpoint in `progression.toml` (days 30,
70, 120, 200), and `cmd/balance` prints the median day a policy enters
each tier. See [CLAUDE.md](CLAUDE.md) for the detailed expectations.

## Source

Written in Go with [Bubble Tea](https://github.com/charmbracelet/bubbletea).
The full design is in [issue #1](https://github.com/theclifmeister/kingpin/issues/1).

```text
cmd/kingpin/        the game
cmd/balance/        headless balance tool
cmd/keys/           prints the key table above from the UI's bindings (-w writes it here)
cmd/anim/           plays every scene on a demo run, for review
internal/events/    event types and bus
internal/game/      world state, clock, player actions, save/load
internal/sim/       simulations: market, logistics, territory, rivals, crew, heat, law, laundering, reputation, news
internal/content/   embedded TOML tuning, names and headline templates
internal/harness/   headless runner and balance tests
internal/format/    the one place a number is written: cash, money, price, arrows, plurals
internal/ui/        Bubble Tea: the frame, the pane, the key table, the modal, the tables, the screens, the theme
internal/ui/anim/   the scenes: the effects, the canvas, the player, the title's art
```

The key table comes from `internal/ui/keys.go`, shared with the pane, help
and modal footers. The screen captures use a fixed-seed test fixture and
the title's still is the loop's first pass at a fixed moment.
Regenerate these blocks after changing the UI:

```sh
go run ./cmd/keys -w
go test ./internal/ui -run TestReadmeCaptures -update
go test ./internal/ui -run 'Readme|Docs'
go test ./internal/ui
```

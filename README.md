# Kingpin

You start with $500 and a corner nobody wants. Buy stock, work the street,
keep the crew paid. The police watch what you move. The rivals watch where
you stand. Your own people watch the payroll.

Kingpin is a terminal game about building a drug empire one day at a time.
Two cities, six products, and a growing number of people who want a cut.
Everything after the first $500 is your fault.

## Play

You need Go 1.26 and a terminal at least 80x24. Install and start:

```sh
git clone https://github.com/theclifmeister/kingpin.git
cd kingpin
go run ./cmd/kingpin
```

Choose a save slot with `enter`. An empty slot opens the new-run dialog:
pick a character (a start and nothing more; a locked one shows the run
that opens it), type a seed or leave it blank, and go. `Daily` at the foot
of the list plays today's date as a seed: the first attempt of a day is
scored against your own history, a second is practice. An occupied slot
takes you back to work.

Your first day needs four keys: **`b` buy, `s` sell, `n` end day, `?` help**.
On the dashboard, buy a little Weed. Pick the product, enter a quantity
(`h` fills the field to half of what you can afford and hold), and choose
`once`. Close the buy dialog with `esc`, then sell: pick the product,
quantity, `normal` dial and `once`. Close the dialog. The sale is queued
for tonight. Press `n` and read the morning report. You now know what
sold, what you made, and how much attention it bought you.

Watch **cash, stock and heat** on the dashboard. The details beside the
selected product show demand and the expected sale; below 100 columns,
press `space` to see them. Buying stock does not sell it. A full stash and
no order is a quiet night for everyone but your wallet.

### Saves

The game saves at the end of every day; `q` saves and quits, and `ctrl+s`
saves now. There are three slots in `$KINGPIN_HOME`, or your platform
config directory under `kingpin/`: `save1.gob` to `save3.gob`. An old
`save.gob` counts as slot 1. The menu shows each run's day, cash, city
and save age. `D` empties a slot after asking; `N` starts over in the
current slot, as the same character, after asking.

To skip the menu and open slot 2:

```sh
go run ./cmd/kingpin -slot 2
```

Older saves upgrade on load. A save from a newer build is refused.

### Characters and the profile

Beside the saves sits `profile.json`: every run that ended, with its
character, ending, score and days; what those endings unlocked; and the
dailies. A character is a start: the Dealer is the run as it always was,
the Cook begins with a chemist and meth on the ladder, the Bookkeeper
(unlocked by ending as a businessman) with an accountant, the Ex-Cop
(unlocked by vanishing) with a police scanner and the chief's temper
known, the Dockhand (unlocked by reaching Distribution) on Bayport's Fish
Market, the Heir (unlocked by taking the crown) with the old man's
enforcer on Riverside, a stash spot, and a name the street already
fears. Taking the crown also unlocks the Hard DA toggle: a
law-and-order DA and a zealous chief on the first morning, never pinned.
No simulation reads the profile or the character: the same seed and the
same start play the same run whatever the profile says. A corrupt
profile is set aside as `profile.json.corrupt-<date>` and a fresh one
written beside it. The summary says where the run ranks among yours, and
`n` there starts again as the same character.

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
the pane; and a run's ending plays the DA's file, the arrest, the
empty till or the exit's title and line before the summary. Scenes are short (250 ms to 1.5 s), any
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
and your city when there is room. It also holds the nine screen tabs and
the journal's unread count. Use `1`–`9`, or `tab` and `shift+tab`, to move
between screens.

The bottom bar answers your last action. A refusal tells you why; danger
shows in red. `?` opens help, including a **WORDS** glossary for the dial,
the float, the file and the other things the street expects you to know.

At 100 columns and up, **DETAILS** sits beside the main screen. It follows
the cursor: the selected product, person, corner, offer or fact, its
facts, and what you can do with it. **KEYS** is at the bottom, with
`n end day` first and `? help` last. In a narrower terminal, a one-line
**details strip** takes its place. `space` opens the whole pane; `esc` or
`space` closes it. The wide pane stays open wherever it fits.

Dialogs, reports and help share a box, at most 76 columns wide, with their
keys in the footer. Longer pages scroll. `esc` or `q` closes the whole
box from any step; `shift+tab` goes back a step in a dialog, and `tab`
goes forward when the step is complete. Inside a dialog, `enter` advances
or confirms. It never ends the day. A confirmation takes `y` and nothing
else; any other key declines. On a play screen, `enter` asks before
ending the day; `n` does not.

Number fields take digits and `backspace`. Use `m` for the maximum, `h`
for half, `↑`/`↓` for one at a time, or `pgup`/`pgdn` for ten. These
shortcuts stay within the limit shown after the number. A blank means
what the field's placeholder says: the maximum on a buy or a sale, the
price on an envelope, none on a route target. In a picker, `↑`/`↓` move
and `1`–`9` choose.

Keys are listed on the screens where they belong. Global actions work
across screens unless a local action takes that key: `b` buys stock on
the dashboard and a front, a house or an asset on the ledger; `i` is
always information (investigate, intel, scout); `$` is always a payment.
A misplaced key points you back: `Hire on the crew screen (4).`

## Screens

1. **Dashboard** — your stock, orders, corners and crew, with cash, heat,
   the law and the rivals below. Start here each morning. Details carries
   the alerts, the selected product's sale estimate and, under YOUR
   NAME, what your reputation is doing in words. `l` lies low, `F`
   fast-forwards, `w` walks away from the run, `i` opens the file on
   the chief.
2. **Market** — prices, quality, connects, demand, orders and buyers with
   deadlines. `←`/`→` shows the other city. Looking there does not move
   you there. With a chemist on the payroll, `%` cuts a lot and `o` cooks.
3. **Journal** — every headline, newest first, coloured by its source.
   `f` filters by source; details shows the selected headline in full.
4. **Crew** — who's on the payroll, with age and skill, and who's looking
   for work. `h` hires, `f` fires, `l` gives a lieutenant a city, `i`
   asks who is talking, `$` buys loyalty, `b` bails somebody out.
5. **Map** — your corners in blue, theirs in purple, free ones plain; a
   block you own carries `⌂`, the corner a rival is eyeing `?`, your
   undercut tonight `$`. Routes sit below the grid, each with a `▪` on
   its road for every shipment in flight, placed by the days it has been
   out (`▪2` where two share a day). Post, strike, undercut, tip and buy
   the block on a corner; set the dial, the target, a checkpoint and a
   driver on a route. Details gives the corner or route its numbers.
6. **Upgrades** — seven branches; `←`/`→` changes branch. Each node sits
   under the one it needs. Details tells you the cost and what you get.
7. **Ledger** — dirty, clean and offshore cash, the fronts, the stash
   houses, the blocks you own, the assets, the road's totals, the
   envelopes out and what is for sale. This is where you put the money
   through the wash, and where you invest it, reserve it, bribe with it,
   give it away and call in the favour a bought chief owes you.
8. **Rivals** — the leader, trust, war, the table of factions (`[`/`]`
   turns it), deals and offers. `i` buys a look at their books, `$` pays
   their muscle to go home, `w` declares war on one of them. Sometimes the
   table costs less than the street.
9. **Intel** — what you know against what is true: every fact in your
   file, how sure it is and who said so, and the spies you have under.
   `$` pays a cop for a word on the police here; `p` plants a spy.

The dashboard at 80x24, with the details strip above the status bar:

<!-- capture:dashboard-80x24 -->
```text
 KINGPIN  1  2  3  4  5  6  7  8  9               Day 4 · dirty $452K · heat 12
╭─ STREET · Eastside ──────────────────────────────────────────────────────────╮
│   product     price     Δ  5d       stash  order                             │
│ ▸ Weed       $19.23   -6%  ▄▇▁█▁       40  -                                 │
│   Pills      $44.21  -14%  ▁▂▆█▁       30  30 normal ↻                       │
│   Coke      $153.32   -1%  ▁▂▂█▇ ▲      0  -                                 │
│   Heroin    $375.84   -5%  █▁           0  -                                 │
│   Meth      $701.73  -12%  █▁           0  -                                 │
│   Designer   $2,740  +11%  ▁█           0  -                                 │
│ carrying 46/410 · 24 in 1 house · corners 3 worked, 3 held of 10, 1 theirs   │
│ tier Distribution · 240 units in Bayport · 60 units on the road, next in 2d  │
│ crew 6 · fair pay $495/day · skimming suspected                              │
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
 KINGPIN  1 Dash  2 Mkt  3 Journal  4 Crew  5 Map  6 Upgr  7 Ledger  8 Rivals  9 Intel    Day 4 · dirty $452K · heat 12
MAP · Eastside   [ ◉ Eastside ]  Bayport  3/10 held · 3 worked · ~1828/day free     ╭─ DETAILS ────────────────────────╮
 ▴ THE DOCKS         ▪ RAIL YARD         ▪ OLD MILL                                 │ THE DOCKS                        │
   theirs              Dre                 Gato ⚔ Moose                             │ Mona's since day 0               │
   ~231/day quiet      ~130/day quiet      ~109/day quiet                           │ corners     1                    │
 ▪ FOURTH & MAIN     · BUS DEPOT         · THE PROJECTS      · PRECINCT ROW         │ size        ×1.2                 │
   you                 free                free                free                 │ heat        ×0.6 quiet           │
   ~142/day undercut   ~142/day warm       ~326/day average    ~269/day hot         │ risk        ×1.6 rough           │
                     · THE STRIP         · RIVERSIDE         · THE HEIGHTS          │ d  buy the block: $15M clean, +… │
                       free                free                free                 │ demand      Weed ~75             │
                       ~357/day warm       ~249/day average    ~254/day warm        │             Designer ~52         │
                                                                                    │             Pills ~44            │
ROUTES                                                                              │             Heroin ~37           │
▸ Coast Road   Bayport  ──car──▪────▶ Eastside  normal  2d · 60 units · $8/u · ?    │             Coke ~17 · Meth ~5   │
  Interstate   Bayport  ─truck──────▶ Eastside  off     3d · 400 units · $3/u · ~3% │ runner      nobody               │
  The Channel  Bayport  ─boat───────▶ Eastside  off     5d · 2000 units · $1/u · ?  │ enforcer    nobody               │
                                                                                    │ w  push takes it ~6–10%, hit ~1… │
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
                                                                                    │ KEYS                             │
                                                                                    │ n  end day      ↑↓←→ pick        │
                                                                                    │ [ ] city        c  post runner   │
                                                                                    │ e  post enforcer                 │
                                                                                    │ a  abandon                       │
                                                                                    │ w  send enforcers                │
                                                                                    │ u  undercut     t  tip police    │
                                                                                    │ d  buy block    r  route dial    │
                                                                                    │ R  route target i  intel         │
                                                                                    │ g  go to Bayport                 │
                                                                                    │ ?  help                          │
                                                                                    ╰──────────────────────────────────╯
                                                                                                                 ? help
```
<!-- capture:end -->

## How a run goes

At first you work the corner yourself. Buy only what you can move, leave
some cash in the till, and read the report before doing it again. The
connect opens the product ladder as your money grows. Better margins are
useful. So is a night the police have nothing to report.

As the takings grow, hire a runner and post them on another corner.
Now two corners can sell. Now there is payroll. An enforcer helps keep a
corner from being robbed; fair pay helps keep the people on it yours.
If money goes missing or the DA's file grows without a bust, read the
crew screen. The roster does not tell you who is talking. A sting or a
raid takes names too: whoever stood on the corner can be picked up the
next morning, and bail is clean cash.

The rival moves in on day ten, and it is the first of several. The
report names the corner they want before they take it, and the map marks
it `?`. Post somebody there in time, fight for it, sell cheap next door,
buy the block it stands on, or talk terms on the rivals screen. Winning
a corner and keeping a quiet city are separate jobs.

Eventually the dirty cash itself draws heat. Your first front washes
some of it clean each day and leaves a float for the street. The upkeep
is real; so are the audits. Accountants help. The greedy dial is always
there when you think you have solved the money problem. Clean cash pays
the rent on a stash house, which keeps stock off the street and out of a
raid that finds somewhere else; a raid hits one place, and a house the
police know about is the place.

Stock has a quality. What the connects sell is middling; a cut stretches
a bag and drops the price it fetches, and a corner sold bad product loses
its regulars. Hard product cut too far kills people, which is pressure
and news. A chemist, once meth is on the ladder, cooks meth and designer
for well under what the connect asks.

Bayport is down the coast. Coke and heroin come off the boats cheap;
weed and pills cost more. Travel there with `g` when you need to do
business yourself. Your stock stays put. Set a route and a target to keep
Eastside supplied, then watch the road in the report; a driver on the
route and a bought checkpoint both cut what the police take. Once you
hold corners in both cities, a lieutenant can turn up to run one for you.
They take 8% of the city, keep it stocked and sell it at the dial their
temper favours. They also take loyalty rather seriously.

Clean money, once there is enough of it, buys the supply side. The
**assets** go on sale on the ledger one at a time as your clean peak
climbs: a tunnel under the county line at $8M (found once, gone for
good), the Dutchman's book at $10M (his lots at a tenth of street,
without limit), the lab at $14M (a chemist's batch eight times over at
quality 90, precursors at half), the port at $20M (the boats through it
carry double and clear customs), an airstrip at $28M (a day's flight).
Each costs clean cash and clean upkeep, puts a floor under the heat in
every city, and is a reason for the feds to form a **task force**: the
rung above the raid, announced the morning before it comes, and it takes
an asset with it. Lie low that day; what they find on a quiet night is
not a case.

The game names these stages. A run starts as a **Corner** trader, becomes
a **Crew** with the first hire, **Territory** once $25K has moved and
the laundromat opens, **Distribution** at $500K, when the wholesaler
sells lots to the road, and **Cartel** at $8M clean, when the assets go
on sale. The morning a stage is entered the game stops
and says so: one screen before the card and the report that names the
stage, what just opened and what the next one takes, once per stage per
run. The report opens with it too, the dashboard reads `tier Territory`
(`· new` until you have seen the stage) and the run summary says which
you reached and when. A stage once reached stays reached, and it changes
nothing by itself: it is a name for what has opened.

There is no last day. A run ends when the law, the street or the till
ends it, or when you walk away: `w` on the dashboard retires on the
offshore account once it holds enough and the city has been quiet long
enough, vanishes on a new identity bought from the Legal branch, or,
once every crew in the city is gone or paying you and you hold more
than half of it, takes the crown. That last is a **reign** you can play
on: the free corners pay you a tax, the paper writes about you, and the
crown is yours whenever you want it. The account is the score.

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
(7 days by default, 30 at most). The run ending, a new stage, a card, a
new alert, police action, rival moves or offers, crew arrested, shot,
retired or gone, a spy found, audits, seizures, buyers, changes at the
courthouse, envelopes that came back, a raid that fell through, a war
night that took a corner, the reign beginning or breaking, houses
robbed or raided, short orders and doors opening stop it. The report tells you why: `Stopped
after 3 days: contract due today.`

### Doors open on the way up

Everything the game keeps behind a line is announced the morning it
opens: a product listing, a front for sale, a connect who will deal with
you, accountants or lieutenants looking for work, a house or an asset on
the ledger. The report opens with an **UNLOCKED** section, the journal
has a headline under the `unlock` source (`f` filters to them: every door
you have opened, in order) and `F` stops. The next door is named before
it opens: the dashboard alerts once you are within half its line (`The
Laundromat opens at $25K peak: $18K to go.`), the market's notes name the
next product on the ladder and what it takes, the ledger's fronts for
sale read the distance (`locked · $18K to go`) and the crew screen says
what the lieutenants wait on.

## Keys

The full table is below; `?` brings it up in the game.

<!-- keys:begin -->
| Key | Legend | What it does | Where |
|---|---|---|---|
| `n` | end day | end the day: the sims step and the run saves | everywhere |
| `F` | fast-forward | run days until something needs you | everywhere |
| `↑↓` | pick | move the cursor (j and k move it too) | everywhere |
| `[ ]` | city | the next city on the market and the map | everywhere |
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
| `1-9` | switch screen | the screens in the title bar's order | everywhere |
| `tab` | next screen | next screen; shift+tab back, in dialogs too | everywhere |
| `ctrl+s` | save | save now; the end of the day saves too | everywhere |
| `N` | new run | start over, after a confirmation | everywhere |
| `q` | quit | save and quit | everywhere |
| `c` | cart | the day's cart: edit its buys and orders | dashboard, market |
| `w` | walk away | retire, vanish, or take the crown | dashboard |
| `i` | intel | the file on the chief and the police here | dashboard |
| `←→` | city | turn the market to the other city | market |
| `a` | accept | take the buyer's offer | market |
| `x` | decline | turn the buyer's offer down | market |
| `%` | cut | cut a product in the stash where you stand | market |
| `o` | cook | the chemist cooks a batch where you stand | market |
| `d` | deliver | hand the buyer what the stash here holds | market |
| `pgup pgdn` | page | page through the journal | journal |
| `f` | filter | show one source's headlines, then all again | journal |
| `h` | hire | hire the selected candidate | crew |
| `f` | fire | fire the selected member, after asking | crew |
| `l` | assign | give the selected lieutenant a city to run | crew |
| `i` | investigate | ask who is talking to the police, for a fee | crew |
| `$` | pay off | buy the selected member's loyalty | crew |
| `b` | bail | clean cash to walk the selected member out | crew |
| `↑↓←→` | pick | walk the map's grid | map |
| `c` | post runner | post a runner on the selected corner | map |
| `e` | post enforcer | post an enforcer on the selected corner | map |
| `a` | abandon | give the selected corner up | map |
| `w` | send enforcers | send the enforcers at the selected corner | map |
| `u` | undercut | sell cheap on the rival's corner next door | map |
| `t` | tip police | tip the police on the selected rival corner | map |
| `d` | buy block | buy the block the selected corner is on | map |
| `r` | route dial | the selected route: off, slow, normal, fast | map |
| `R` | route target | what the selected route keeps the far end at | map |
| `$` | buy checkpoint | buy the checkpoint or customs on the route | map |
| `v` | driver | put a driver on the selected route | map |
| `i` | intel | the file on the faction holding the corner | map |
| `←→` | branch | turn the tree to the next branch | upgrades |
| `u` | buy upgrade | buy the node under the cursor (enter too) | upgrades |
| `b` | buy | buy a front, a house or an asset | ledger |
| `m` | move stock | move stock between the street and the houses | ledger |
| `e` | guard house | post an enforcer inside the selected house | ledger |
| `x` | drop house | drop the selected house, after asking | ledger |
| `$` | bribe | an envelope for the chief or the DA | ledger |
| `v` | call favour | the bought chief owes you: no raid tonight | ledger |
| `f` | fund city | give a city clean cash for goodwill | ledger |
| `u` | invest | clean cash into the selected front's levels | ledger |
| `o` | reserve | clean cash into the offshore account | ledger |
| `[ ]` | faction | the next faction at the table | rivals |
| `d` | propose | offer the rival a truce, tribute or a split | rivals |
| `a` | accept | take the selected offer | rivals |
| `x` | decline | turn the selected offer down | rivals |
| `i` | scout | buy a look at the rival's books | rivals |
| `w` | declare war | enforcers on the faction every night, hit | rivals |
| `w` | call off war | stand the enforcers down | rivals |
| `$` | buy off | pay the rival's muscle to go home | rivals |
| `$` | pay cop | a cop's word on the chief and the police | intel |
| `p` | plant spy | send a crew member under with a faction | intel |
<!-- keys:end -->

## How it works

For implementation details, invariants and test coverage, see [CLAUDE.md](CLAUDE.md)
and the one file a subsystem in [docs/](docs/README.md).
Tuning lives in `internal/content/*.toml`. These are the systems behind a run.

Every day the simulations step in a fixed order (`world -> market -> logistics -> territory -> rivals -> crew -> heat -> law -> laundering -> reputation -> news`),
each reading the world and emitting typed events that later sims and the UI
consume. Randomness is derived from the run seed and the day number, so a
run is fully reproducible and nothing about the RNG needs saving; what
happens away from home, and every feature added since the first city,
rolls on its own side of the stream, so a run that never uses one plays
the same as it always did.

### Cities

Everything happens in one of two cities, each with its own
street prices, its own corners and its own police. Eastside is home;
Bayport is the port down the coast, where coke and heroin come off the
boats cheap and nobody much wants them, weed and pills cost more, and
the police watch the water. You are in one city at a time: the connects
sell to you where you stand, into a stash there, and you can only work
a corner yourself where you are; runners sell where they are posted.
`g` moves you, for what only you can do there; the routes move the
stock.

### Market

Market drifts prices toward an equilibrium with noise, rolls supply
shocks and demand slumps, and resolves your sell orders, city by city.
Selling into demand barely moves the price; flooding past it craters it.

Every lot has a **quality**, one number per product per city, blended
as stock arrives. The connects sell at 50, where the price is the street
price; at 100 a unit fetches a quarter more, at 0 nearly a third less,
on every sale and on a buyer's premium. `%` on the market **cuts** a lot: so many
units of nothing at a price per unit added, at most doubling it, and
the quality falls by the same share. The street remembers: a city sold under
40 loses a little of every worked corner's regulars each night, and only
better product wins them back. Heroin, meth or designer sold under 30 off
your own corners rolls an **overdose** per twenty units, which is pressure
in the city, notoriety and a headline, never a page in the DA's file.
A **chemist** turns up in the pool once meth is on the ladder: `o`
**cooks** a batch of meth or designer for the price of precursors, landing
three days later at their quality, and their hand on a cut keeps some of
the quality back.

The **connects** are the supply side. Each city has a street connect who
sells everything the city sells, in lots of fifty, on credit up to a
small book; the Dutchman in Bayport wholesales in lots of a hundred once
$500K has moved; and one pool connect a run, with their own range and
terms, deals with you once $25K has moved and the street connect in their
city vouches. Every lot you or the road buys earns a little standing;
paying a debt on its day earns more, paying late loses it, a bust that
takes their product loses it. Standing moves the price toward the bottom
of their band, how much they will run you a day and how much credit they
extend, and at the top they tip you off the day before a shock or a slump
on something they sell. Credit costs a markup and is collected on its
day, dirty cash then clean; short, a patient connect extends once, a
sharp one freezes you out and adds a fee, a connected one sends somebody
round. A debt never ends the run.

It also deals the **buyers**: every few days somebody in one of the
cities wants product off-corner, on a deadline, at a multiple of that
city's street price on the day you hand it over (a club owner who wants
pills for the weekend, a face from out of town who wants coke in bulk;
the mover who deals by the case pays under the street as often as over
it). Offers come to the market screen and lapse in a few days; take one
and it is yours to deliver out of that city's stash, standing there, by
its due day. A handoff needs no corner, is not capped by a patrol, takes
no cut for a lieutenant and draws heat at the buyer's own rate; short at
the due day, you lose respect, gain notoriety, the buyer collects a share
of what you owed and stays away for a month. The premium is a bet: it is
against the street on the day, so a slump or a spike since the buyer
asked is yours to eat. The deck is `internal/content/buyers.toml`.

It fills **supply contracts** every morning before anything sells: a
contract keeps the stash in a city at a level, bought from the connect
there at a small markup, out of dirty cash, through the same path as a
buy by hand, so the connect reacts to it exactly as to you; short of
cash or room it buys what it can and the report says so.

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
the dialog says what the days mean today, `3d ≈ 180 units`). The
airstrip adds a plane and the tunnel a tunnel, once you own them.

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

Two things cut the odds on a road. A **driver** comes looking for work
once a route has run; `v` on the map puts them on a route, and they take
up to half off its risk each day, by their skill. A shipment taken with
them on it puts them in a cell that night. A **checkpoint** on a road, or
the customs agent on a boat's, is bought with dirty cash on the map for
thirty days and takes a good deal more off; the two compound, and the
odds the map shows are the odds the dice use. The map's `seized` column
reads `?` on a road you have never lost a shipment on: the risk is what
you have learnt, not what the file says.

### Territory

Territory is each city's corners. Each is its own demand pool and only
a corner somebody stands on sells; a runner holds one for you, an enforcer
keeps it from being robbed, and a corner nobody works drifts back to the
street. The factions fight over the ground too.

Once the city is yours in the kingpin's sense (every faction at the
table gone or paying you homage) and you hold more than half its
corners, the corners nobody holds are worked by independents who pay
you a **tax** for the right: a tenth of each corner's trade a night, in
dirty cash, with no heat and no page, since nobody of yours moved a
unit. The report's MONEY line and the ledger carry it. A faction setting
up on a free corner ends that corner's tax; the share falling ends it
all.

### Stash houses

A house is a rented place on a named block: a lease in dirty cash, rent
in clean every day, so much room. Everything that arrives in a city goes
into its houses first and the street last; everything that leaves comes
off the street first. A house is not a bigger bag (what you carry on the
street is unchanged); it is somewhere a raid has to find. A sting or a
raid hits **one place** in a city, rolled over the houses and the street
by the heat of the block each stands on, so an empty house on a cheap
block shields nothing. A house the police **know** (an informant told
them, it was robbed, or a bust took something from it) is the target
until you drop it; with an informant on the payroll a raid empties the
fullest house whole. A house is robbed like a corner, behind a door, and
an enforcer posted inside (`e` on the ledger) cuts that like a guard on a
corner. `m` moves stock between the street and the houses, free and at
once, for a little exposure and no page. Five days of unpaid rent and
the landlord throws you out with everything inside. Houses are bought
with `b` on the ledger and dropped with `x`.

### Property

`d` on the map buys the **block** a corner is on, with clean cash only,
at ninety days of that corner's street trade at today's prices. A deed
stays with the block whoever holds the corner: it halves the robbery
chance on it and on a house there, halves a rival's odds of pushing you
off it and of defending it against you (a deed slows the rival, never
stops it), halves the block's weight in a raid's roll, and pays you clean
rent that returns the price in five hundred days. A deed on a rival's block
is legal and the businessman's way to a corner. Deeds are public: each
adds a little pressure a day, and the fourth in a city makes the paper.
The DA reads them against the wash: hold more in deeds than half of what
your fronts have ever laundered and the newest is **forfeited**, one a
night, no refund, and the file gains two pages. Card money and the
fronts' own income explain nothing.

### Crew

Crew are runners, enforcers, accountants, lieutenants, chemists, drivers
and fixers, hired from a rotating pool; the last four come looking once
their door opens. Runners raise how much you can hold and how much of the
street you reach; accountants put more through every front and keep the
auditors away; a fixer improves an envelope's odds.

The pay dial trades wages for loyalty; loyalty also falls with
greed, danger and firings. Below a threshold a member skims the takings,
or the wash (the report says so without naming names); lower still, the
nervous start talking to the police, feeding the DA's file every few days
whatever you sell, and a raid empties your fullest house. The roster
never shows it: the tell is a file that grows without a bust and a heat
delta the dial does not explain, and after two of those the screens hint
at it.

Investigating (`i`) names them, for a fee, with odds that scale with your
best enforcer's skill and with every night that named nobody; firing
them stops it, and the crew do not hold that firing against you. An
audit turns a disloyal accountant the same way, a backfired envelope its
fixer, a spy caught and turned comes home talking, and somebody left in
a cell may come out talking. `$` buys a member's loyalty outright, for
two months of their wage: it buys loyalty, not silence. At the floor a
member walks, or, while a faction holds ground in the city they stood
in, defects to it and walks it onto the corner they ran; a faction with
money poaches the same way, offering your least loyal member better pay.

People have **lives**. They sign on between nineteen and forty-four and
age a year every thirty days; skill grows with the pay dial up to a cap,
nerve falls past thirty-eight, and at fifty they retire (loyal, they
recommend a cousin; sour, they leave a page behind). Hiring somebody
brings their **kin** into the pool at half the fee, marked `♦`, and kin
remember a firing or a body. A sting or a raid takes the names of
everyone who stood on a held corner there, and each rolls a quarter
chance of ten days in a cell the next morning, where they work nothing
and the wage runs on. **Bail** (`b` on the crew screen) is clean cash,
the whole amount by role, and buys loyalty; unbailed, they come out
under the line and may turn. The Legal branch's bail bondsman pays it
from the clean account the night of the arrest, when the account
covers it. Enforcers going in on a strike, and the
guard on a corner a rival pushed and failed to take, can be **shot**:
wounded for twelve days or dead, by the force used. Every body on
either side counts against the score.

### Lieutenants

Lieutenants run a city for you. Once you hold corners in both
cities, one turns up in the hiring pool now and then; give them a city
(`l` on the crew screen) and every night they post your idle runners
on its best corners, give up a corner after its second stick-up, keep
the stash there topped up on a supply contract of their own and
sell everything stashed there at the dial their temper favours: a
**violent** one sells aggressive and runs the city hot, a **greedy**
one skims on top of the cut, a **careful** one sells quiet and earns
less, a **steady** one just runs it. You learn which after ten days on
the job.

They keep 8% of the city's takings, bring eight roster slots of people
of their own, and your own order for a product there wins the day.
Through them you can buy in a city you are not standing in, at the
contract markup. Watch their loyalty more than anyone's: under the line
they turn informant with no dice and feed the DA thick pages, and a
lieutenant who turns while running half your corners ends the run
**betrayed**; at the floor they walk with the city, every corner they
ran and the stash there.

### Rivals

A run seats **three to six factions** at the table, each with a leader
and a temperament drawn from the seed (at most one chaotic). The first
moves into Eastside on day ten and the rest arrive one every twenty-five
days after it. Each moves in on a free corner, claims more, pushes on
the corners of yours it borders, undercuts you there and calls the
police when you hurt it. A claim is **telegraphed**: the day it picks a
corner the report says so, the map marks it `?` and the RIVALS panel
reads `eyeing Riverside`; it sets up there the next night unless
somebody of yours is posted on it by then, in which case it leaves,
keeps its money and holds a grudge.

The table shares the map: between them the factions stop claiming free
corners past a share of the city and push on you instead, and they push
on each other, muscle against muscle, which never brings a crackdown on
you. A faction with no corners for a month is **absorbed** by the one
that took its last, unless you or the police routed it. Defensive
factions side with whoever an expansionist pushes, and at enough trust
half their front line joins the victim's pushes, yours included. A
faction your enforcers have taken three corners off may offer
**homage**, a fifth of its take a day, theirs to offer and never yours
to ask. `[`/`]` on the rivals screen turns the table; breaking a deal
with one costs you trust with them all.

Enforcers on the war dial (warn / push / hit, `w` on the map) are one
answer, and `w` on the rivals screen makes them a routine: a **war** on
one faction sends the hand's strike every night you send none, at the
hit dial, on their corner nearest your front line, with the same dice,
heat and toll as a strike by hand; it ends when they are gone, paying
homage or hold nothing in a city you do. The table is another answer; money is the third: work a corner next to
one of theirs and `u` on the map **undercuts** it, tonight's orders
serving a share of its customers cheap on top of your own, which costs
you margin and gluts the product, draws no heat, and cuts what the
corner earns them; starve a corner for days and they push back or, by
temper, give it up. The **books** are the fourth: `i` on the rivals
screen **scouts** a faction's cash, income, muscle and wages, for a fee
and at odds your best enforcer sets; the strike picker's last rows
**boost** a corner's till instead of taking the corner; `t` on the map
**tips** the police on a rival corner, free, and enough tips bring a
raid that puts a share of their muscle in the van and, past the line,
takes the leader (the corners drift to the street and the muscle turns
up in your hiring pool cheap); `$` on the rivals screen **buys off** a
head of their muscle, dirty cash, before the night's fighting, and they
go home and never join you.

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
respect); a strike costs it; a push or a hit under a deal, a tip, a
missed tribute or walking off a split corner is a **betrayal**: trust
falls to the floor, it makes one call to the police, and it takes
nothing for a month. A chaotic rival breaks deals on a whim; a defensive
one never. The joint shipment is on the list and not yet on the table.

### Heat

Heat is per city: it rises with the volume you *tried* to move there
and how loud the dial was, plus a little, where you are, for sitting on a
pile of dirty cash. Units your crew moves count at a discount, but sloppy
low-skill runners add a premium, and a notorious boss's own units count
extra. It decays slowly, faster if you lie low (`l`), down to a floor
your fear or an asset you own sets.

The hottest city's police answer at the thresholds: patrols, stings,
raids and finally arrest, one response a day, and what they take comes
out of one place in that city: a house they know, else one rolled by the
heat of its block, else the street. Every sting and raid on a day you
dealt there goes in the DA's file, which is yours wherever you are; a
raid on a quiet night costs stock and cash and no page. A thick enough
file is an indictment, thinner under a law-and-order DA, thicker under a
reformer or a bought one. The file also grows without a bust: an
informant's page every few days, a tip that comes back on you, an
envelope that backfires, offshore lots over the line, a forfeited deed.
Between the raid and the arrest is the **task force**, which forms only
against an asset you own or a dirty pile over $25M: announced the
morning before it comes, it takes stock, cash and one asset, and no
chief shortens its wait. The gauge marks its line once it can form.

### Reputation

Reputation is the face the street keeps on you: **fear** (from
strikes, pushes and boosts), **respect** (from a generous payroll, a
pay-off and every night a deal holds; a full delivery to a buyer too;
short wages and a failed delivery cost it) and **notoriety** (from
volume, strikes, overdoses, bodies and every headline about you). Each
pulls its own way at full strength: fear slows the rivals' pushes and
claims and sways them at the table but sets a floor heat never falls
under; respect keeps the crew loyal, the connects friendly and their
muscle cheap to buy off; notoriety makes hiring cheap and every unit
*you* move on your own corner hotter, so a notorious boss gets off the
corner. The street has only so much attention: you cannot max all three.
The dashboard's pane says what each axis is doing in words, under YOUR
NAME, with the numbers the dice use; a candidate who has read about you
says so, and once the city is yours the paper writes about the boss.

### The law

The law has faces. A **police chief** with a temperament drawn from
the seed and hidden until you have seen them work, or twenty days in, or
paid a cop: a **zealous** one sends the stings and raids back sooner and
lets heat fade slower, a **lazy** one the opposite and their patrols let
more through, a **corrupt** one is the one who takes an envelope. They
serve a term and the mayor names another.

A **DA** elected every ninety days on a ticket: a
**law-and-order** DA needs a thinner file to indict and stings sooner,
a **reformer** the reverse, a **moderate** runs the courthouse by the
book. Who wins is decided by **public pressure**, a number every city
carries: violence, hard product (heroin, meth, designer), deeds,
overdoses, bodies and headlines about you push it up, it fades on its
own, and a loud city gets its police answering sooner (patrols, raids,
the task force, the arrest line), gets the rivals' phone calls returned,
and elects a crackdown DA, who may fire the chief on the way in.

Clean cash buys **goodwill** (`f` on the ledger), which takes the
pressure off a little every day, and, in the month before a vote, backs
a **campaign**: the second page of the same dialog puts clean cash
behind the reform or the law-and-order ticket in a city, so much a point
of its vote, up to a ceiling; money moves the vote and never decides it,
backing both sides buys nothing, and a city that backed the loser pays
for it. A DA you backed lets the sting line sit higher and sells for half.
Dirty money is not welcome there; it buys the rest. `$` on the ledger
sends an **envelope**: a corrupt chief takes it (a lazy one at half
effect, a zealous one files it), and heat fades faster and the stings
and raids come back later for a month, and the chief owes you one: the
morning a sting, a raid or the task force is due tonight, `v` on the
ledger calls in the **favour** and it does not come, nothing taken and
nothing cooled, for a page in the file with the chief's name on it; a moderate DA takes it at odds
the dialog shows, a reformer refuses, a law-and-order DA files it, and a
bought DA needs a thicker file. A backfire is heat and a page whatever
you sold; three envelopes taken open a file of their own; a law-and-order
DA taking office ends every deal in five days and nobody takes a call
while they sit. A **fixer** in the pool, once you have paid anyone,
improves the odds. The dashboard's LAW panel shows all of it, and the
report carries elections and new chiefs. The chief and the DA never add
a page by themselves: they move the thresholds, the cooldowns and the
decay. The pages the law does add are for things you did.

### Intel

The sims keep the truth; the screens read your **file**. A faction's
temper, its muscle, the chief's temper, the police's next move and a
road's risk read `?` until something has told you: a fight your
enforcers were in, a scout, a bust you took, a chief seen at work.
Every fact carries how sure it is and fades from the day it was
learnt, a temper apart, which never fades. `$` on the intel screen pays
a cop for a word on the chief and on what the police here will do next
and when, right three times in four, a rung or a night off otherwise,
never off the ladder; a cop is not a bribe. `p` sends one of your own
under with a faction: they work nothing for you while they are under,
report every five nights on its muscle, its next move and its fattest
till at odds their skill sets, and each report risks being found, half
of those coming home turned and the rest shot. A faction that does not
trust you feeds you lies, `a contact says`: a road that reads safe and
is not, a till that reads fat and is empty. Once one bites, the source
reads `fed by` in red. `i` on the dashboard opens the file on the chief;
`i` on a rival's corner on the map opens the file on its faction.

### Upgrades

There are fifty-eight upgrades across seven branches. Bonuses stack and
stay with the run:

- **Operations** earns more: storage, connect terms and street trade.
- **Security** cools heat faster and softens the damage.
- **Legal** helps you survive the case: the lawyer, the bondsman, the
  fall guys and, last, a new identity.
- **Crew** makes the payroll cheaper and your people more likely to stay,
  and grows the roster and the pool.
- **Laundering** washes more with less attention from auditors.
- **Logistics** moves more stock for less on the road.
- **Street** helps you hold your corners against the rivals' pushes.

The nodes and their effects live in `internal/content/upgrades.toml`;
[docs/upgrades.md](docs/upgrades.md) covers the full tree. Nothing removes
the heat curve. Clean-cash nodes cost money the fronts have washed.

### Laundering

Laundering is the fronts: a laundromat, a car wash, a restaurant, a
nightclub, a construction firm, a crypto exchange. Each washes dirty cash
clean up to a daily cap for a daily upkeep, and each can be audited.

The launder dial pushes them all harder or softer: greedy washes more, gets
audited more, and an audit of a front run greedy goes in the DA's file.
The wash always leaves a float in the till for the street. Dirty cash
over the threshold is heat every day it sits there; clean cash is what
the retainer, the rent, the deeds, the assets and the endgame ask for.

A front grows by levels: clean cash invested in it (`u` on the ledger)
earns clean income of its own every day, dirty or no dirty, and pays
itself back in a hundred days. Each level washes more, costs more, and is
looked at more; a front whose wash outruns what its income explains is
the one the auditors find, and one that grows big enough makes the paper.

Clean cash can go offshore (`o` on the ledger), less a small fee: the
account survives every ending, cannot be spent, and is safe from the fall
guy, the auditors and the forfeiture; it is the score. Up to a lot a day
moves unnoticed; every lot over it is a page in the DA's file the morning
after. Enough offshore and enough quiet days in a row, and you can retire
(`w` on the dashboard, asked twice).

### Endings

A run ends nine ways, and the summary says which: **indicted** (the
file), **arrested** (the heat), **broke** (the till), **retired** on the
account, a **businessman** whose fronts out-earn the street for thirty
days with the city on side, **kingpin** by taking the crown during a
**reign** (every faction at the table absorbed, broken or paying you
homage for two weeks while you hold more than half of home; the reign
is announced, can be played on as long as it holds, and breaks the
morning a crew sets up again or the share falls), **betrayed** by a lieutenant who knew where
everything was or an ally who broke a deal while you were at war,
**taken out** when the last corner falls at war with fewer than two
enforcers on the payroll, or **vanished** on a new identity from the
tree, which also turns the indictment the fall guys do not take into an
exit. The **exit plans** run in order: the fall guys take the first
case each, then the identity turns the next into an exit, then the run
ends. `w` on the dashboard walks away on either plan, or takes the
crown, by choice. The
score is the account over one plus the bodies; the pile left behind is
printed, never scored, and the days are shown, never scored.

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

The tier a run is in (Corner, Crew, Territory, Distribution, Cartel) lives in
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
| `driven` | The distributor with a driver on its busiest route. |
| `corrupt` | The distributor that pays the chief when the heat is up and keeps a checkpoint on its road. |
| `funded` | Runs fronts, pays the city when pressure rises and backs the reform ticket. |
| `favoured` | The distributor that bribes the chief when hot and calls in the favour when a raid is due. |
| `warlord` | Runs a crew and keeps a war order on the nearest faction until it folds. |
| `dealer` | Runs a crew, reserves stock for buyers and delivers when heat allows. |
| `stocked` | Runs a crew with supply contracts at one day's corner demand. |
| `routine` | After $20k peak cash, uses normal standing orders that grow with the stash. |
| `leveraged` | Runs a crew that restocks on the connects' credit first and pays cash for the rest. |
| `pricewar` | Holds territory by undercutting the biggest rival corner next door instead of striking. |
| `saboteur` | Runs a crew that works the rival's books: scouts, boosts the till, buys off muscle. |
| `tipster` | The saboteur that tips the police every night the rival holds a corner. |
| `stashed` | Runs fronts and keeps its stock in stash houses. |
| `cook` | Runs fronts with a chemist, cooking meth and designer instead of buying them. |
| `retiree` | Runs fronts, reserves a lot a day and retires on the account once the days are quiet. |
| `informed` | The diplomat that pays a cop when the heat is up and lies low on the nights the word says the police can move. |
| `boss` | Works the hub's corners, sends enforcers when odds justify it, buys fronts, deeds and upgrades at a margin, and fires lieutenants who sell aggressive. |
| `cartel` | The boss with the assets: buys each as its price comes into clean cash, opens the plane and the tunnel, cooks in the lab. |
| `reckless` | The cartel that never lies low and sells aggressive from its first asset. |

### Isolate a system

Use these flags to compare runs with the same conditions:

| Flag | Effect |
| --- | --- |
| `-rival none` | Keeps the rivals out; `-rival expansionist` and the like pin the first faction's temper. |
| `-factions 1` | The duel the game was before the table; `0` is the file's count by seed. |
| `-heat off` | Switches heat off. |
| `-pace off` | Gives the rival its flat pace. |
| `-lt violent\|greedy\|careful\|steady` | Forces the delegated lieutenant's temper. |
| `-chief corrupt\|zealous\|lazy` | Holds the chief's temperament fixed. |
| `-da law_and_order\|moderate\|reform` | Holds the DA's stance fixed. |
| `-own stash,burners` | Starts with those upgrades. |
| `-snitch` | Starts with an informant on the payroll. |
| `-cards decline\|first` | Answers cards with the last (do-nothing) or first choice. |
| `-incidents off` | Boxes the world's incident table (on by default). |
| `-life off` | Boxes crew life: nobody ages, is arrested, wounded or killed. |
| `-deeds off` | Takes every block off the market. |
| `-credit off` | Withdraws every connect's credit. |
| `-cut 0.5` | Cuts everything the policy buys by that ratio. |
| `-houses N` | How many stash houses the stashed policy keeps a city. |
| `-character cook` | Starts every run as that character (`dealer`, `cook`, `bookkeeper`, `excop`, `dockhand`, `heir`). |
| `-hardda` | Seats a law-and-order DA and a zealous chief on day 0, never pinned. |

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
buyers, supply contracts, standing orders, houses, deeds, the assets, the
intel file and every ending. `TestMoneyCurve` pins the
scale per tier at each tier's checkpoint in `progression.toml` (days 30,
70, 120, 200, 300), and `cmd/balance` prints the median day a policy enters
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
internal/sim/       simulations: world, market, logistics, territory, rivals, crew, heat, law, laundering, reputation, news
internal/content/   embedded TOML tuning, names and headline templates
internal/harness/   headless runner and balance tests
internal/format/    the one place a number is written: cash, money, price, arrows, plurals
internal/ui/        Bubble Tea: the frame, the pane, the key table, the modal, the tables, the screens, the theme
internal/ui/anim/   the scenes: the effects, the canvas, the player, the title's art
docs/               one file a subsystem: the names, the numbers and the tests that pin them
```

The key table comes from `internal/ui/bindings.go`, shared with the pane, help
and modal footers. The screen captures use a fixed-seed test fixture and
the title's still is the loop's first pass at a fixed moment.
Regenerate these blocks after changing the UI:

```sh
go run ./cmd/keys -w
go test ./internal/ui -run TestReadmeCaptures -update
go test ./internal/ui -run 'Readme|Docs'
go test ./internal/ui
```

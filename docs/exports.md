# Exports are the cartel's demand

**Exports are the cartel's demand** (#391, `internal/content/exports.toml`, `content.ExportsConfig`, `game/exports.go`, `World.Exports`, `sim/logistics/exports.go`, `ui/exports.go`, `harness.Export`).
Before the lanes, tier 5 did not read as a cartel. The tier-4 operation is bound by demand (two cities' corners) and by the wash (a few hundred thousand a day). The assets (`docs/assets.md`) only make supply cheaper, so the cartel read a few percent *under* the boss at day 300 (about $170M; #205's ruling, with #223 asking for a demand-side lever).
A lane is that lever: demand the corners do not bound, sold abroad by the ton. The **cartel's wash** is paired with it: two fronts sized for what the lanes land (`docs/laundering.md`).

## The file

`[exports]` holds what every lane shares:
- `watched_mul` (3): a lane's risk while the feds watch (`HeatState.WatchUntil`, the task force's cooldown).
- `glut_floor` (0.4): the lowest a glutted price goes, as a fraction of the lane's `price`.
- `record_days` (14): how long a landed or seized load stays on the record.

Each `[[lane]]` has:
- `id`, `name`.
- `city`: where it leaves from. The price abroad is a multiple of the street price here.
- `mode`: `boat` or `plane`.
- `asset`: the asset that opens it beside the book; `""` means the book alone.
- `capacity`: units one night's load carries at most.
- `days`: time in transit.
- `price`: the price abroad, a multiple of street in `city`.
- `risk`: the chance a load is seized.
- `glut`: what a full load takes off the lane's next price for that product (a part load takes its share).
- `recover`: how much of the glut comes back a day.
- `products`: what the lane ships.

Decode refuses:
- a lane without the book in the assets file;
- an unknown city, product or asset;
- a mode that is not boat or plane;
- a number out of range;
- a lane that ships nothing.

The three lanes:

| Lane | Leaves from | Opens with | Units a night | Days | Price | Risk | Glut / recover |
|---|---|---|---|---|---|---|---|
| Rotterdam freighter | Bayport, boat | the book | 16,000 | 8 | 0.8 | 0.03 | 0.05 / 0.03 |
| Gulf container line | Bayport, boat | the port | 20,000 | 12 | 0.7 | 0.04 | 0.05 / 0.03 |
| Night charter | Eastside, plane | the airstrip | 5,000 | 2 | 1.1 | 0.02 | 0.08 / 0.04 |

All three ship coke, heroin, meth and designer.

## The rules

**Every lane needs the Dutchman's book** (the `supplier` asset, `logistics.Sim.LaneOpen`), and its own asset if it names one.
- A load is bought straight off the owned book at its `own_ratio` of the street price in the book's city (`ExportCost`).
- It never touches a stash, so no house and no corner caps it.
- **The port doubles a boat out of its city** (`LaneCapacity`: its `capacity_mul`, the port's rule for the road).
- The book is now the **first door at tier 5**: `unlock_cash` $8M peak clean, the tier's line, swapped with the tunnel's $10M. The lanes are what the tier is for, so the cartel ships from about day 215.

**The standing order** is `World.SetExport(lane, product, units)`, the one action (`Session.SetExport`, the wire's `set_export`, protocol 11). Zero units or no product turns the lane off.

**The night** is `logistics.Sim.exportsStep`, after the road, in this order:
1. **Landings.** Every load due lands or is seized on **one roll a load**, off `game.StreamExports`.
   - The roll uses `LaneRisk`: the file's `risk`, times `watched_mul` under the watch.
   - A landing pays `Units × Price` in dirty cash (`ExportLanded`).
   - A seizure takes the load, and the cost already paid is gone (`ExportSeized`).
   - Either way the load goes to `Exports.Record`.
2. **The glut eases** by each lane's `recover`.
3. **The record is pruned** to `record_days`.
4. **Loads.** Every lane with an order loads `LoadTonight`: the order, capped by the capacity and by what the till over the float pays for (`Budget`).
   - It locks `Price` at `ExportPrice`: street in the lane's city × `price` × what the glut leaves (`GlutMul`).
   - It adds `glut × units / capacity` to the lane's glut on that product.
   - It emits `ExportShipped`.

A run with no lane open draws nothing, so **a run that never owns the book is byte-for-byte the run before the file** (`TestNoBookIsTheOldRun`; `TestSeedDigest` re-pinned for the shape alone).

**The state** is `World.Exports`, the logistics sim's (`TestSimsWriteOnlyTheirOwnState`):
- `NextID`;
- `Orders` (lane → `ExportOrder{Product, Units}`);
- `Loads` (out, in the order they left);
- `Record`;
- `Glut` (`lane:product` → the fraction off).

The zero value is the run before, so there is **no schema bump**. `NetWorth` counts a load out at its cost. `Stats` gained `ExportLoads`, `ExportUnits`, `ExportCash`, `ExportCost` and `ExportsSeized`.

**Heat and the law.** A load draws no street heat, files no page and builds no case: #27 holds. The risk is the load itself, and the task force already takes an asset. What reaches the police is the dirty cash that lands, which is what the cartel's wash is for: its cover is ten times what the fronts cost (`heat.toml dirty_cash_cover`). Since #397 a landing is forecast the morning before: the logistics sim lands the loads before the heat sim counts the pile and the wash comes after the count, so `engine.Forecast` adds the loads due tonight (`Lands <= day + 1`, `ExportLoad.Revenue`, as if none is seized) to the pile in hand less tonight's wages, and the dashboard's `exposure` alert (`Tonight's 2 loads land $31M past your cover: +35 heat before the wash.`) and the ledger's `tonight` line say what the count will find (`docs/laundering.md`).

**Events.**
- `ExportShipped` is report-only: the SHIPMENTS line, plus the MONEY line `The X's load off the book -$Y`, booked to purchases.
- `ExportLanded` gives the SHIPMENTS and MONEY lines, booked to sales. It makes a headline (source `logistics`, templates off `game.StreamExportsNews`) only on the first landing of the run: the city learns its size.
- `ExportSeized` always makes a headline, and it is a fast-forward stop (`engine.StopsOn`, `a load seized abroad`).

All three give the `shipment` cue.

## The UI

**The EXPORTS block on the ledger** sits under ASSETS. It is shown once a lane is open or a load is out or on the record (`exportsShown`), so a run that never owns the book reads the ledger it always did. Every lane in the file is a row under the ledger's one cursor (`ledgerLane`).

The columns are `line`, `city`, `product`, `units`, `price`, `days` and `status`:
- `status` is one of `needs The Port`, `idle · t to order`, `loading tonight` or `2 out · lands d241`.
- Where MAIN is narrow, the days column goes first, then the city.

The pane's section (`laneSection`) shows:
- what a night carries, the days and the risk;
- each product's rate abroad over its cost off the book, with the glut;
- tonight's load and what it costs;
- the loads out, with their day and what each is due to pay.

**`t` on a lane** (`ledgerOnLane`) opens the order dialog, `modeExport`, an amount dialog:
- `←→` turns the product;
- the number field takes the units a night (max is the capacity; blank is off);
- `enter` sets the order;
- the dialog shows what tonight's load costs off the book and what it would fetch on landing.

`TestExportsInTheGrammar` walks it at the three sizes.

## The pointer (#505)

A playtest's income went from $0.3-0.7M a day to $10-20M the week it found the lanes, and none of seven testers found them before the Cartel stage; the one that did read the ledger line by line. Nothing pointed at them. Now three things do, the way #476 points at Bayport:
- **The alert.** `engine.AlertExports` (`Session.exports`, `ui/alerts.go` `exportsAlert`, the web's `exports`) fires from the morning the run is at the Cartel stage (the tier with id `cartel`, reached) until a lane has an order on, a load is out or one has ever gone (`Stats.ExportLoads`). It names the first lane in the file with no asset of its own you lack (Rotterdam, the book's alone, before the port), the product with the best margin tonight (the harness's pick, glut and all), its price abroad and its cost off the book, and what a night carries: `The lanes abroad: Designer pays $1,400 a unit out of Bayport on $190 off the book, 16000 units a night. Buy The Dutchman's Book, then t on a lane on the ledger screen (7).`; once the book is owned, `The lanes abroad are open: … t on a lane sets a nightly load on the ledger screen (7).` Keyed once (`the lanes abroad`), so a fast-forward stops on it once a run; the act is the ledger, where the book is bought and the lanes take their orders. No dice and no sim reads it: no number moved.
- **The stage.** The Cartel tier's `opens` reads `the lanes abroad: buy the Dutchman's book, ship by the ton` and its text leads with the lanes (`docs/progression.md`). Distribution's `next` is left as it was: the report carries it, and `TestSeedDigest` would move for words alone.
- **The cover.** The Cartel tier's `opens` says the casino and the bank each cover ten times their price of the pile, the lever the pile's ruling names (below).

`TestExportsAlert` (engine: the table of mornings, the fields and the key held when the book is bought), `TestExportsAlertWords` (ui: the words either side of the book and `o` to the ledger) and `TestExportsAlertOnce` (harness: the boss is told once, the morning it reaches the stage, and the alert stands; the cartel, which buys the book the night the door opens and orders before the stage is stamped, at most once and never while the lanes are in use).

## The pile's ceiling (#505)

The same playtest's dirty pile stopped growing at $0.5-0.6B: past its cover the pile adds `dirty_cash_heat_max` (35) a night where you stand, lying low every night did not hold it under the raid and task-force lines, and the raids and a task force took $26-97M a time. **Ruling: the ceiling is the cover, and the lever is the Private Bank.** A front covers ten times its price of the pile (`dirty_cash_cover`): every front but the bank covers $0.35B (the casino $300M of it), which is where the playtest capped; the bank ($120M dirty, on the book, $100M peak) adds $1.2B, so every front covers $1.55B and a skilled player holds a billion dirty with no heat off the pile at all. Past every front's cover the pile is meant to be washed (the bank washes $6M a day), spent or sent offshore, not held. No heat spread over houses or cities: #396 set the bound where lying low every night cannot hold a pile of any size under 100 (at 25 or under it could, a pardon while the lanes pay), and spreading the same heat over cities is that pardon by another road. No number moved. `TestTheBankCoversABillion` (`sim/heat`) pins it: every front but the bank covers under $0.5B and a $590M pile draws the bound; with the bank, $1B draws nothing.

## The harness and the numbers

`harness.Export` sets every open lane to its capacity on the product with the best margin tonight (the price abroad, glut and all, over the cost off the book), so the glut on one product turns the lane to the next. `Cartel` calls it every morning. `cmd/balance` prints an `exports:` line: loads per run, units landed and what they paid, the cost off the book, and the seizures.

On the money curve's twenty seeds at day 300 the cartel reads **$1.7B** (median), up from $166M ($2.0B before #389, whose factions resolve late seats and succession and hold more of the map). Per run, on `cmd/balance -runs 20`, that is about 124 loads, 2.4M units landing for $1.4B on $376M off the book, and 2.9 loads seized; 17 of 20 runs ship. The boss is unmoved (it never owns the book).

The cartel's tier-5 row in `TestMoneyCurve` is re-banded to **$1B–$5B** (#48's ask), and `TestTaxAtTierFive` pins the same band. `TestCartelDwarfsBoss` replaces `TestCartelIsWithinFifteenPercentOfBoss`: on every seed where a lane shipped the cartel beats the boss, and its median is at least five times the boss's (×9.2 on ten seeds). `TestExportsSaveAndReplay` pins determinism and the save round-trip with loads out. `TestExportLanes` and `TestExportSeizures` (`sim/logistics/exports_test.go`) pin the rules.

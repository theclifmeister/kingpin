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

**Heat and the law.** A load draws no street heat, files no page and builds no case: #27 holds. The risk is the load itself, and the task force already takes an asset. What reaches the police is the dirty cash that lands, which is what the cartel's wash is for: its cover is ten times what the fronts cost (`heat.toml dirty_cash_cover`).

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

## The harness and the numbers

`harness.Export` sets every open lane to its capacity on the product with the best margin tonight (the price abroad, glut and all, over the cost off the book), so the glut on one product turns the lane to the next. `Cartel` calls it every morning. `cmd/balance` prints an `exports:` line: loads per run, units landed and what they paid, the cost off the book, and the seizures.

On the money curve's twenty seeds at day 300 the cartel reads **$1.7B** (median), up from $166M ($2.0B before #389, whose factions resolve late seats and succession and hold more of the map). Per run, on `cmd/balance -runs 20`, that is about 124 loads, 2.4M units landing for $1.4B on $376M off the book, and 2.9 loads seized; 17 of 20 runs ship. The boss is unmoved (it never owns the book).

The cartel's tier-5 row in `TestMoneyCurve` is re-banded to **$1B–$5B** (#48's ask), and `TestTaxAtTierFive` pins the same band. `TestCartelDwarfsBoss` replaces `TestCartelIsWithinFifteenPercentOfBoss`: on every seed where a lane shipped the cartel beats the boss, and its median is at least five times the boss's (×9.2 on ten seeds). `TestExportsSaveAndReplay` pins determinism and the save round-trip with loads out. `TestExportLanes` and `TestExportSeizures` (`sim/logistics/exports_test.go`) pin the rules.

# The market and the journal in the grammar

**The market and the journal in the grammar** (#84, `ui/market.go`, `ui/journal.go`).
The market's MAIN is the title `MARKET · Eastside` with the city tabs (`cityTabs`: `[ ◉ Eastside ]  Bayport`, the shown city in brackets and `Selected`, the one you stand in marked; `screenTitle(name)` builds the line for the market and the map), the product table through `table()` (`product price Δ Nd supplier stash demand/day order`; `productRows` is the dashboard's too, which has no supplier, quality or demand), a blank, then #71's BUYERS; nothing else, because the detail is the pane's (`marketDetails`): the selected product (`WEED · EASTSIDE`: `range 30d`, `glut`, `margin`, `demand ~N/day on N corners` and `~N per standard`, a `shock`/`slump` while one runs, the `↓` row when buyers wait), ELSEWHERE (the other city's street and supplier price, your stash there, the road), NOTES (not sold here, the "you are in X" note when the shown city is not where you stand, the wholesale line when it is the wholesale city), or the contract's sections while the cursor is on the buyers, then KEYS.
The strip at 80 is the product and its range, glut and margin.
The journal (`journal.go`) is a list with a cursor, newest first: MAIN is `JOURNAL · N headlines, newest first` and one row a headline (`▸ d8  text`, the text cut with `…` to the width, in the source's colour; `journalCursor`, `journalTop` the window, `journalMove`, `journalPage` for `pgup`/`pgdn`, `journalFollow` keeps the cursor in view and `resize` calls it; `refreshJournal` puts the cursor back on the newest headline each morning and on a new or continued run), the pane (`journalDetails`) is the selected headline (`D8 · LAUNDERING`, the text wrapped to where it is drawn: `paneTextW` beside MAIN, the overlay's width under 100 columns) and the LEGEND, one source a line in its colour (`journalSources`; `world`, first, in `theme.World`'s tan, is the incidents, #44; the news sim's own `news` source reads `flavour`; `unlock`, in the ledger's gold, is every door that opened, #148), packed into cells in the overlay.
**`f` filters it by source** (#122): `Model.journalFilter` is a source name as the legend spells it or empty for every one, a view cursor like `journalSeen` (never saved; `startRun` and `continueRun` clear it, `refreshJournal` keeps it across a morning), `cycleFilter` turns it to the next source the journal has a headline from in the legend's order (`journalSourcesPresent`) and after the last back to every one, silently, since the title carries it; `headlines()` is the filtered list, so `journalCursor`, `journalTop`, the arrows and the paging keys index and walk that list and the cursor goes to its newest on every turn; the title reads `JOURNAL · 14 of 212 headlines · law` (`journalTitle`, the source in its colour) and the LEGEND marks the source shown `Selected`; the unread count is the whole journal's, filter or not (`viewJournal` still sets `journalSeen` to the journal's length: shown is read, whatever the filter hides).
`f` is the journal's own binding, shadowing nothing there (`f` fires on the crew screen and funds on the ledger).
`market_test.go` (`TestMarketDetailInPane`, `TestMarketRendersInTheGrammar`, `TestMarketArrowsTurnTheCity`) and `journal_test.go` (`TestJournalTruncatesWithEllipsis`, `TestJournalPages`, `TestJournalFilterCycles`; `TestJournalUnreadCount` in `ui_test.go` holds the count under a filter) pin it.

## The cash flow (#351)

Every night's change in cash is explained, opening to closing, by cause.

- **`game.CashFlow`** (`game/cashflow.go`) is on `DayReport.Flow`: the piles the day opened on (`Opening`, a `game.Pools` of `Dirty` and `Clean`), one `FlowLine` a category in `game.FlowCats`' order, signed by pile (money in positive), and the piles it closed on (`Closing`).
- **The categories** run in the order money moves through a night, and `game.FlowLabel` is the words for each:
  - `sales`: the street and the buyers, net of the cut the crew and the lieutenants keep;
  - `purchases`: the connects, the contracts, the cuts, the cook, a debt paid down;
  - `routes`: lots and fares, checkpoints, signing fees, the rent on the houses, investigations, scouts, cops, bail;
  - `wages`: the payroll and the loyalty bought on top;
  - `laundering`: the wash (dirty out, clean in), the upkeep of the fronts and the assets, what the fronts earn, the offshore account;
  - `investments`: upgrades, fronts, assets, levels, houses, deeds and their rent, the cities funded, campaigns, bribes, the buy-offs, tribute paid and homage received;
  - `losses`: robbed, skimmed, seized by the police or the auditors, the fall guy's price, a buyer collecting;
  - `tax`: the free corners of a city you hold (#231);
  - `other`: a card's cash, and the rival's takings the enforcers boosted.
- **It reconciles by construction.** The news sim books every amount into the reporter's `flow` as it writes the line (`reporter.book(cat, dirty, clean)`), reads the closing off the piles, and works the opening back pile by pile (`game.NewCashFlow`). `CashBefore` and `CashAfter` are its two ends.
- **What makes it true** is that the opening is the night before's closing. `TestCashFlowReconciles` holds that across every harness policy to day 200, with the cards and the incidents in play.
- It found the moves the old `CASH BEFORE` missed:
  - a card's cash (`Answer.Cash`, the choice's effect on each pile after the clamp);
  - the assets' upkeep (now `events.AssetUpkeepPaid`, report-only, the MONEY line `Upkeep on 2 assets -$X clean`);
  - a cop's word (`Today.Cop`, the line `A cop's word -$X`);
  - homage received;
  - a scout or a buy-off whose faction was gone by night (both are booked off the order in `Today`);
  - a greedy lieutenant's skim, counted once off `LieutenantActed` and again inside `CrewSkimmed`.
- **The piles** of a cost taken dirty first and clean for the rest are kept where it is paid. `World.spend` returns the clean part for `InvestigationOrder.Clean`, `Payoff.Clean` and `ScoutOrder.Clean`, and `World.TakeCash` returns a `Pools`. `InvestigationRun`, `CrewPaidOff`, `RivalScouted`, `DebtPaid`, `DebtLate`, `ContractFailed` and `FallGuyBurned` carry `Clean`.
- **The history.** `World.Flows` keeps the last `headlines.toml [flow] days` (14) nights, oldest first. It is the news sim's (`TestSimsWriteOnlyTheirOwnState`) and zero on a save from before it, so there is no schema bump.
- **It is a report.** No sim reads it and it adds no dice. `TestSeedDigest` leaves `World.Flows` and `DayReport.Flow` out of its walk (`unwalked`). The report's own words moved on eight days of the pinned run (days 9, 24, 26 and 47, a robbery's line naming its corner; days 57 to 60, the lieutenant's skim no longer counted twice), and no world number moved.
- `TestFlowIsTheReportersTotals` holds the lines to the night's events read on their own: the sales, the losses, the tax and the wash's dirty side.

**The TUI.**
The report's MONEY section leads with the waterfall (`ui/report.go`, `moneyLines`): one `table()` of `flow`, `dirty`, `clean` and `total`, the opening and the closing in gold, and a row a category that moved.
A row past `[flow] big_share` (0.25) of the opening is drawn in `theme.Bad` or `theme.Good`.
The itemised lines follow and name the cause (`Seized by police in Eastside -$42,000 (the raid on the stash)`, `Robbed on The Docks in Eastside -$1,900`, `The fall guy's price -$X`), then `Cash $749K → $502K`.
A night that moved nothing is the lines alone.
The ledger's foot is **FLOW** (`ui/ledger.go`, `flowTable`): the last `[flow] shown` (7) nights as columns (`d41` and on), a row a category that moved in any of them, and the `Net` under them.
The oldest nights go first where MAIN is too narrow, and the window runs down to it with the cursor on the last row.
`TestReportLeadsWithTheFlow` and `TestLedgerShowsTheFlow` pin both, at 80x24 and up.

**Engine and web.**
`ReportView.Flow` carries it (`docs/engine.md`), and the web report draws it as a table in place of the flat money lines (`docs/web.md`).

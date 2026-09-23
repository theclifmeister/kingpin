# Kingpin docs

One file a subsystem, the detail CLAUDE.md points at (#175).
Each is the contract for the sim or screen it names: read it before touching that code and update it in the same PR.

- [The engine is one session a front end drives](engine.md)
- [The web client: the game drawn in a browser](web.md)
- [The harness and cmd/balance](harness.md)
- [Day loop](day-loop.md)
- [Events are the cross-sim channel](events.md)
- [World is a plain struct](world.md)
- [Cities](cities.md)
- [Corners are the demand](corners.md)
- [The law has actors](law.md)
- [Logistics is the road, and the road is a dial](logistics.md)
- [Fronts are the laundering](laundering.md)
- [Reputation is the player's face](reputation.md)
- [Saves](saves.md)
- [The rival](rival.md)
- [Diplomacy is the table](diplomacy.md)
- [Upgrades are multipliers the sims read](upgrades.md)
- [Snitching](snitching.md)
- [Lieutenants run a city for you](lieutenants.md)
- [All tuning is data](tuning.md)
- [Dilemma cards](dilemmas.md)
- [World incidents are the weather](incidents.md)
- [Progression tiers are a name for what has opened](progression.md)
- [The stage is the interstitial](stage.md)
- [Buyers are the market screen's reason to exist](buyers.md)
- [Supply contracts are the buy side of the routine](supply-contracts.md)
- [Standing orders are the sell side of the routine](standing-orders.md)
- [The connects are the supply side](connects.md)
- [Unlocks are announced](unlocks.md)
- [Cash formatting](format.md)
- [UI (the grammar of #78)](ui.md)
- [The frame and the details pane](frame-and-pane.md)
- [The market and the journal in the grammar](market-and-journal.md)
- [The keys are one table](keys.md)
- [Modals are one component](modals.md)
- [Crew and upgrades in the grammar](crew-and-upgrades-screens.md)
- [Copy, colour and status](copy.md)
- [The cart is the day's shopping](cart.md)
- [Stash houses are where you keep it, and which one the raid finds](houses.md)
- [Animation](animation.md)
- [Quality is the last dial](quality.md)
- [Crew life: kin, ageing, arrests and getting shot](crew-life.md)
- [Property is the block a corner is on, bought with clean cash](property.md)
- [Assets are the supply side bought with clean money](assets.md)
- [Intel: what you know against what is true](intel.md)
- [The endings, the exit plans and the run summary](endings.md)
- [The profile: characters, unlocks and the daily](profile.md)

## The map

Where each subsystem lives: its code, its tuning, its doc and the tests that pin it (moved here from CLAUDE.md, #276).
A PR that changes a subsystem updates its row.

| Subsystem | Code | Tuning | Doc | Pins it |
|---|---|---|---|---|
| Cities, stock, capacity | `game/world.go`, `game/stash.go`, `sim/logistics.Migrate`, `sim/territory.Migrate` | `city.toml` | `docs/cities.md` | `TestSaveMigratesTheOneCity` |
| Corners, demand, robbery, drift, the tax (#231) | `game/territory.go`, `sim/territory` | `city.toml` | `docs/corners.md` | `territory_test.go` |
| Market, prices, orders, price war | `sim/market`, `game/actions_trade.go` | `market.toml` | `docs/tuning.md`, `docs/rival.md` (#68) | `TestPriceInvariants`, `TestNoUndercutIsTheOldRun` |
| Connects (suppliers, credit) | `game/suppliers.go`, `sim/market/suppliers.go`, `ui/suppliers.go` | `suppliers.toml` | `docs/connects.md` | `TestSupplierInvariants`, `TestLeveragedIsALeverNotFreeMoney` |
| Buying: max, the room refusal, restock (#356) | `game/restock.go` (`MaxBuy`, `RoomError`, `StockLevels`, `RestockPlan`), `engine/prices.go`, `ui/restock.go`, `ui/dialogs.go` (`afterRow`) | `market.toml [supply] float` | `docs/cart.md`, `docs/modals.md` | `TestRestockMatchesStocked`, `TestMaxBuyNeverRefused`, `TestErrNoRoomIsTyped`, `TestNoRoomIsTyped` |
| Supply contracts (buy routine) | `World.Supply`, `market.Sim.Plan`, `World.StockLevels`, `ui/dialogs.go` | `market.toml [supply]` | `docs/supply-contracts.md` | `TestSupplyMatchesTheHand`, `TestStockedIsWithinFifteenPercentOfCrewed`, `TestRestockMatchesStocked` |
| Standing orders (sell routine) | `World.Standing`, `market.Sim.standing` | `market.toml [standing]` | `docs/standing-orders.md` | `TestStandingSellsLikeTheHand`, `TestRoutineIsWithinFifteenPercentOfCrewed` |
| Buyers (contracts) | `game/buyers.go`, `sim/market/buyers.go`, `ui/buyers.go` | `buyers.toml` | `docs/buyers.md` | `buyers_test.go`, `TestDealerBeatsCrewed` |
| Heat, evidence, the police response; the POLICE section (#355) | `sim/heat` (`Rungs`, `bite`), levels `content.Patrol..Arrest` (the task force before the arrest, #48), `ui/police.go` | `heat.toml` | `docs/corners.md`, `docs/snitching.md`, `docs/assets.md`, `docs/law.md` | `heat_test.go`, `taskforce_test.go`, `TestRichHiderIsNeverIndicted` (#27), `TestRungsAreWhatTheyTake`, `TestPoliceSectionAgreesWithTheSim` |
| The law: chief, DA, pressure, goodwill, campaigns (#193), bribes and checkpoints (#42) | `sim/law`, `game/law.go`, `ui/law.go`, `ui/bribes.go` | `law.toml`, `names.toml` | `docs/law.md` | `law_test.go`, `TestQuietDayRuleHoldsUnderEveryLaw`, `TestNoCampaignIsTheOldRun`, `TestNoBribeIsTheOldRun` |
| Logistics: routes, targets, shipments | `sim/logistics`, `ui/routes.go` | `routes.toml` | `docs/logistics.md` | `TestStockIsConservedAcrossShipments`, `TestDistributorBeatsLaundered` |
| Laundering: fronts, the float, audits, the levels (#192), the offshore account (#195) | `sim/laundering`, `ui/ledger.go`, `ui/invest.go`, `ui/reserve.go` | `laundering.toml` | `docs/laundering.md` | `laundering_test.go`, `TestExposureLineIsOneNumber`, `TestLevelsPullTheirWay`, `TestNoInvestIsTheOldRun`, `TestStructuringFilesPages` |
| Endings: nine causes and their owners, the exit plans, the reign (#227), the score, the summary (#49) | `game/exit.go` (`End`, `Score`, `Retire`, `Vanish`, `Crown`), the owning sims, `ui/summary.go`, `ui/exit.go` | `endings.toml`; thresholds in the owners' | `docs/endings.md` | `TestEveryEndingIsReachable`, `TestExitPlansRunInOrder`, `TestEndingFrequencies`, `TestNoEndingIsTheOldRun` |
| Stash houses | `game/houses.go`, `ui/houses.go`, raid in `sim/heat`, rent in `sim/territory` | `houses.toml` | `docs/houses.md` | `houses_test.go`, `TestDecoyHouseNeverShieldsTheStreet` |
| Crew, pay, snitching, investigation, the crew trouble alerts (#345) | `sim/crew`, `ui/crew.go`, `engine/alerts.go` (`crewLines`, `idleCorners`), `ui/alerts.go` | `crew.toml` (`alert_margin`) | `docs/snitching.md`, `docs/crew-and-upgrades-screens.md` | `crew_test.go`, `TestCrewTroubleAlerts`, `TestFastForwardStopsOnIdleCorner`, `TestCrewTroubleWords` |
| Property: deeds, rent, the forfeiture (#194) | `game/territory.go` (`BuyDeed`), `sim/territory`, `sim/rivals` (`OddsOn`), `sim/heat` (`RaidWeight`), `sim/law` (the forfeiture), `ui/deeds.go` | `city.toml [deed]` | `docs/property.md` | `TestDeedsPullTheirWay` ×4, `TestNoDeedIsTheOldRun`, `TestDeedSlowsTheRivalNeverStopsIt`, `TestForfeiture` |
| Assets: the connect, port, airstrip, lab, tunnel; the task force; tier 5 (#48) | `game/assets.go`, `sim/laundering`, `sim/heat`, `ui/assets.go`, `harness/assets.go` | `assets.toml`, `heat.toml` taskforce, `routes.toml`, `progression.toml` | `docs/assets.md` | `TestTaskForceNeverMeetsTierThree`, `TestNoAssetIsTheOldRun`, `TestCartelIsWithinFifteenPercentOfBoss` |
| The profile: characters, unlocks, the hard DA, the daily (#50) | `game/profile.go`, `content/characters.go`, `sim.NewWorldWith`, `ui/newrun.go`, `harness/characters.go` | `characters.toml` | `docs/profile.md` | `TestProfileNeverTouchesTheRun`, `TestCharacterIsDayZero`, `TestCharactersAreStartsNotCheats`, `TestDailySeedIsTheDate`, `TestNoWallClockInTheSims` |
| Crew life: kin, ageing, arrests, bail, shot, the driver (#46) | `sim/crew/life.go`, `Heat.Sweep`, `Bail` / `SetRouteDriver` in `game/actions_crew.go` / `actions_routes.go`, `ui/life.go` | `crew.toml [life]`, `[role.driver]` | `docs/crew-life.md` | `life_test.go` ×3, `TestNoLifeIsTheOldRun`, `TestDriverCutsSeizures` |
| Quality: lots, the cut, repeat, overdoses, the chemist and the cook | `game/stash.go` (`Cut`, `CookOrder`, `MigrateLots`), `sim/market/quality.go`, `sim/crew`, `ui/quality.go` | `market.toml [quality]`, `crew.toml [role.chemist]` | `docs/quality.md` | `TestNoCutIsTheOldRun`, `TestGreedCurve`, `TestOverdosesArePressureNeverEvidence`, `TestCookBeatsBuying` |
| Lieutenants, buying through them | `sim/crew/lieutenant.go`, `ui/lieutenant.go`, `Buy` in `game/suppliers.go` | `crew.toml [lieutenant]` | `docs/lieutenants.md` | `lieutenant_test.go`, `TestBuyThroughTheLieutenant` |
| The rival: pace, tell, price war, economy, books, the war order (#229) | `sim/rivals`, `game/territory.go`, `game/rivals.go`, `ui/rivals.go` | `rivals.toml` | `docs/rival.md` | `rivals_test.go`, `pricewar_test.go`, `TestRivalEconomyBinds`, `TestNoBooksIsTheOldRun`, `TestRivalStateHasOneWriter` |
| Intel: the file, the cop, the spy, the feed (#45) | `game/intel.go` (`Known`), writers in every sim that owns a truth, `ui/intel.go` | `intel.toml` | `docs/intel.md` | `TestPanelsReadTheFile`, `TestNoIntelIsTheOldRun`, `TestInformedOutlivesDiplomat` |
| Factions: the table, arrivals, contests, absorption, alliance, poaching, homage, `Dominant` (#43) | `game/factions.go`, `sim/rivals/factions.go`, `sim/crew/factions.go`, `ui/rivals.go` | `rivals.toml [factions]`, `law.toml [pressure] factions` | `docs/rival.md` | `TestOneFactionIsTheOldRun`, `TestFactionsContestBeforeDay120`, `TestNobodyIsDominantByAccident`, `TestBrokeRoutedFactionScatters`, `factions_test.go` |
| Diplomacy: truce, tribute, split, homage | `game/diplomacy.go`, `sim/rivals/diplomacy.go`, `ui/diplomacy.go` | `rivals.toml [diplomacy]`, `[deal.*]` | `docs/diplomacy.md` | `diplomacy_test.go`, `TestTributeShareOfTheTake` |
| Reputation: fear, respect, notoriety | `sim/reputation` | `reputation.toml` | `docs/reputation.md` | `reputation_test.go`, `TestReputationCannotMaxAllThree` |
| Upgrades: the tree, the effect vocabulary | `game/upgrades.go`, `content/upgrades.toml`, `ui/upgrades.go` | `upgrades.toml` | `docs/upgrades.md` | `TestFoldEffects`, `TestUpgradedBeatsCrewed`, `Test*NodesMoveTheirNumbers` |
| Dilemma cards | `game/dilemmas.go`, `game/preview.go`, `sim/news/dilemmas.go`, `engine/card.go`, `ui/card.go` | `dilemmas.toml` | `docs/dilemmas.md` | `dilemmas_test.go`, `TestTriggersHold`, `TestEverySlotIsFilledByItsTrigger`, `TestPersonalStakesAreCapped`, `TestRichCardsMoveMoreThanCash`, `TestRichDeckLeansToTheBand`, `TestPreviewIsTheOutcome`, `TestChoiceChipsSayTheLines`, `TestCardShowsWhatEachChoiceDoes` |
| World incidents: the table, closures, named headlines | `sim/world`, `game/incidents.go`, `Route.ClosedUntil` | `incidents.toml`, `names.toml` | `docs/incidents.md` | `world_test.go`, `TestEveryIncidentFires`, `TestNoIncidentsIsTheOldRun` |
| Progression tiers, the stage, unlocks | `game/progression.go`, `sim/news/progression.go`, `ui/stage.go`, `ui/unlocks.go`, `events.Unlocked` | `progression.toml` | `docs/progression.md`, `docs/stage.md`, `docs/unlocks.md` | `TestTiersAreOrdered`, `TestEveryGateIsAnnounced`, `TestNoUnlockIsTheOldRun` |
| News, report, journal, the cash flow (#351) | `sim/news`, `game/cashflow.go`, `ui/journal.go`, `ui/report.go`, `ui/ledger.go` (FLOW) | `headlines.toml` (`[flow]`) | `docs/events.md`, `docs/market-and-journal.md` | `TestEveryEmittedEventHasTemplate`, `TestArticlesAgreeWithTheValue`, `TestSimsNeverImportEachOther`, `TestCashFlowReconciles`, `TestFlowIsTheReportersTotals`, `TestReportLeadsWithTheFlow` |
| Engine (#293): the session, the view, the protocol, the embeddings | `engine` (`view.go`), `protocol`, `cmd/kingpind`, `cmd/kingpin-wasm`, `cmd/libkingpin` | | `docs/engine.md` | `TestUIActsThroughTheSession`, `TestViewShapeIsPinned`, `TestViewHasNoNull`, `TestViewCarriesWhatTheScreensList`, `TestProtocolIsTheSession` |
| The web client (#328) | `cmd/kingpin-web` (`web/js`) | | `docs/web.md` | `TestWebClient` |
| Saves and slots | `game/save.go`, `modeStart` | | `docs/saves.md` | `TestOldSaveIsMigrated`, `TestUnreadableSaveIsRefused` |
| Fast-forward, alerts, stop events, the alert jump (#352) | `engine/stops.go`, `alerts.go` (`Act`, `ActsOf`), `ui/fast.go`, `ui/alerts.go` (`openAlert`) | `houses.toml` (`full_share`) | `docs/ui.md`, `docs/engine.md` | `TestFastForwardIsTheSameDays`, `TestFastForwardStopsOnACard`, `TestEveryAlertHasAnAct`, `TestUnpostedAlerts`, `TestStashFullAlerts`, `TestEveryAlertHasATarget`, `TestAlertJumpOpensItsTarget`, `TestEveryActLands` |
| The cart, the dialogs, the delta | `ui/cart.go`, `ui/dialogs.go`, `ui/market.go` `priceFacts` | | `docs/cart.md` | `cart_test.go`, `delta_test.go`, `toggle_test.go` |
| Dashboard, map, ledger, rivals screens | `ui/dashboard.go`, `map.go`, `routes.go`, `ledger.go`, `rivals.go` | | `docs/frame-and-pane.md`, `docs/ui.md` | `dashboard_test.go`, `TestRouteMarkerMoves`, `TestTablesAreConsistent` |
| Animation: scenes, effects, the title loop, the registry | `ui/anim` (`Scenes()`), `ui/scene*.go`, `ui/demo.go`, `cmd/anim` | | `docs/animation.md` | `TestEveryModeWithASceneIsListed`, `TestNoTickInPlayMode`, `TestSceneStopsTicking`, `TestScenesFit` |

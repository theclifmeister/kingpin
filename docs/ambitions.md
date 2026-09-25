# Ambitions are the endings shown early as plans

**Ambitions are the endings shown early as plans, each with its progress** (#347, `game/ambitions.go`, `content/ambitions.toml`, `engine/ambitions.go`, `ui/ambitions.go`).
A playtest found that past $5M the only goal left was watching the balance grow.
The game already had its goals, the endings (`docs/endings.md`), but most of them could not be seen until they were nearly met.
An ambition is one of those goals shown as a plan: a few steps, each a reading of the world against what the ending needs, and a bar.

## The plans

`content.AmbitionIDs` lists them in the panel's order, and `content.AmbitionSteps` gives each one's steps in order.
The file names them and labels each step.
The thresholds belong to the files that own them, never to `ambitions.toml`.

| Id | Name | Steps | Done exactly when | Thresholds |
|---|---|---|---|---|
| `retire` | Retire clean | `offshore` (the account toward `retire_cash`), `quiet` (`QuietDays` toward `retire_days`) | `Offshore >= retire_cash` and `QuietDays >= retire_days`: `World.CanRetire`'s terms | `laundering.toml [offshore]` |
| `legit` | Go legitimate | `income` (the fronts' own income, `laundering.Sim.LegitIncome`, against what the street sold for last night), `goodwill` (home's goodwill against its pressure), `streak` (`World.LegitDays` toward `legit_days`) | `LegitDays >= legit_days`: the laundering sim's own count at its own line | `laundering.toml [businessman]` |
| `city` | Take the city | `share` (home's corners held against more than `kingpin_share`), `factions` (factions arrived and gone or paying homage, against the table), `streak` (days since `World.DominantSince` while both hold, toward `dominant_days`) | `World.Reign > 0`: the rivals sim's stamp, the crown open | `rivals.toml [endings]` |
| `vanish` | Disappear | `retainer`, `identity` (the tree's nodes: owned, or their cost in clean cash against the clean pile) | an identity from the tree (`fx.Identities > 0`, `World.CanVanish`) | `upgrades.toml` |
| `two_cities` | Two-city operation | `ground` (cities where you hold `share` of the corners), `lieutenants` (those a lieutenant or a captain runs, #346), `held` (those whose corners at the share have been yours `days` days, `Corner.Since`) | `cities` of them held | `ambitions.toml [two_cities]`: 0.4, 14 days, 2 cities |

A plan is left out when its owner's file boxes the ending with a zero (`retire_cash`, `legit_days`, `dominant_days`), so `harness.NoEndings` shows no plan for an ending it has turned off.
The two-city plan is a milestone, not an ending (`content.AmbitionEnding` has no cause for it), and it ends nothing.

**The bar reads 100% exactly when the ending's own condition holds.**
`Ambition.Progress` is 1 when the plan is `Done`, and `Done` is the ending's own predicate from the table above.
Otherwise the bar is the mean of the steps' fractions, each `Have / Need` capped at 0.99, and the total is capped at 0.99 too.
So a full bar is never a promise the ending does not keep.
The city's streak reads one short of `dominant_days` until the reign is stamped.
The rivals sim reads the city after the territory has stepped, and a later sim can still move a corner that night (a lieutenant's walk), so only the stamp counts.
`Ambition.Next` is the first step not yet met, and nil once the plan is done.
`Ambition.Reached` counts the steps met in order, stopping at the first one that is not.

**What the street sold for last night** is the tick's `PlayerSold` revenue, the number the businessman's count reads.
`Session.EndDay` keeps it.
For a run loaded or attached since that night, the engine reads the report's `sales` flow line instead: the street and the buyers net of the crew's cut, the closest number the world keeps.
Only the `income` step reads it, and that step never decides whether the plan is done.

## Read-only

Ambitions are pure queries, as `game.Eligible` and the tiers are.
`game.Ambitions(w, terms)` and `game.AmbitionOf` write nothing.
The engine passes in the owners' thresholds and the two numbers only a sim can work out (`Session.AmbitionTerms`), the way `Retire` takes its terms.
No sim reads a plan, nothing ends a run for one (#27), and the score is unchanged.
`World.DominantSince` moved from the rivals sim to the world so the city plan and the detector read the same day (`rivals.Sim.DominantSince` delegates to it).

**The pin** is the one new field: `World.Ambition`, an id or `""`, set by `World.PinAmbition` (`ErrNoAmbition` for an id the game lacks, `""` unpins).
It is the session command `PinAmbition` and the wire's `pin_ambition` (protocol 10).
Its zero value means no plan, so no schema bump.
No sim reads it, and `TestSeedDigest`'s walk leaves it out (`unwalked`), so no pinned number moved.

## Where it shows

- **The panel** (`modeAmbitions`, `ui/ambitions.go`): `a ambitions` on the dashboard (#472: the dashboard's own key, in help and the README), on the walk-away dialog's first page (`w` on the dashboard) and on the stage modal, where it marks the stage seen and closing the panel goes on to the card and the report.
  The panel is a table with a row for each plan: the name, the bar (`done %`), the next step's label (or `ready`, or `made` for the milestone) and `plan` in gold on the one pinned.
  Under the table are the selected plan's blurb and its steps, each `✓` or `·` with its reading (`the account: $412,000 of $750,000`, `quiet days: 3 of 14 days`).
  The crown's `crews down` step, until it is met, is followed by a line a faction that does not count, with what keeps it off and for how long (`Preacher: run out 3d ago; gone in 27d unless it claims again`, `Model.downLines`, the rivals sim's own rule, `docs/rival.md`, #472).
  The step counts a seat that stood down before it arrived as gone, as `Dominant` does (it read `3 of 4` for good).
  `↑↓ pick`, `1-9 choose` (select and pin), `enter pin` / `enter unpin`, `esc close`; enter never ends the day.
- **The dashboard**: while a plan is pinned, STREET carries `plan Retire clean · the account 0% · quiet days 3/14` in gold (`plan … ready` in green once done), worth `priPlan` (over the counts, under the stage).
  **The line shows the parts** (#465, `planParts`, `stepPart`: a count as `3/14`, anything else as how far along it is): the one bar was the mean of the steps, and `Retire clean 50%` with $0 of $750,000 (the quiet days full) read as half the money, `Disappear 53%` at $261K of $4M as more than half the papers.
  When the quiet days go back to zero the line names what reset them, `· reset by a sting in Eastside on day 41` (`quietReset`, off the laundering sim's report-only `events.QuietBroken`, `docs/laundering.md`, kept on the model as `Model.quietBroke` and never saved), while the plan is Retire clean and its quiet days are short.
- **The report**: a `PLAN` section after `TIER`, one line: `Retire clean: the account 0% · quiet days 3/14. Next, the account: $0 of $750,000.`, with `, the quiet days reset by …` before the next step when the dashboard's line has it (`TestPlanShowsItsParts`).
- **The alert**: `engine.AlertPlan` (`plan`, the quietest, its act the dashboard) appears while the pinned plan has at least one step met, keyed `plan <id>: <Reached> of <steps>`.
  A fast-forward stops once as each step is met in order and once when the plan is done.
  It stops again only if a step is lost and met again.
  `Alert.Ambition`, `Count` (the steps met), `Steps` and `Ready` carry it; the TUI words it `The plan, Retire clean: 1 of 2 steps met.` / `The plan, Retire clean: ready.`
- **The view** (view 10): `ambitions[]` (`id`, `name`, `ending`, `pinned`, `progress`, `done`, `next`, `steps[]` with `label`, `have`, `need`, `unit`, `done`) and `you.ambition`.

With no plan pinned the dashboard, the report and the alerts are the same as before, and so are the README captures.

The Cartel tier's `next` and `closing` (`progression.toml`) now point at the panel: `Pick an ambition and chase it`.

## Guards

- `TestAmbitionsNeverWriteTheWorld` (`harness`): every morning of four players with a plan pinned, and of the retire, businessman, kingpin and vanish scenarios, reading the plans, the view and the alerts leaves the digest and the pin as they were.
- `TestAmbitionProgressAgreesWithTheEnding` (`harness`): every morning, each plan reads 100%, and has no next step, exactly when its ending's condition holds.
  The scenarios that reach retired, businessman, kingpin and vanished each read 100% on the morning the ending is taken.
  Five players on two seeds over 120 days hold it too.
- `TestNoAmbitionIsTheOldRun` (`harness`): every policy, seed 1, 90 days, pinned (each a plan, in turn) or not.
  The daily net worth, the ending and the final digest are the same either way.
- `TestViewCarriesTheAmbitions` (`engine`): the view lists every plan in order as the session reads it, with the file's names and labels, and marks the pin.
  An unknown id is refused, and `""` unpins.
- `TestAmbitionsPanel`, `TestAmbitionsFromTheStage` (`ui`); `TestModalsFit` has `ambitions` and `ambitions pinned`; `TestViewShapeIsPinned` and `TestSchemaIsCurrent` are regenerated.

## Rulings and why

- **The two-city milestone pays nothing yet.** The issue has it pay reputation and a headline through the news sim, like `TierReached`.
  A reward paid once needs a stamp on the world saying it was paid, and the only new field this change allows is the pin.
  So it is a plan with a bar and an alert, and the reward is left for a follow-up.
- **A lieutenant or a captain** (#346) runs a city for the milestone: `CrewState.Lieutenant` or `CrewState.Captain`, as the issue has it.
- **The panel is a key of the dashboard's own since #472**, `a ambitions`: a playtest found it only from a stage card or the walk-away dialog, and help never named it.
  One more key on the dashboard's KEYS took the row POLICE needs whole in the 100x30 pane (`TestPoliceKeys`), so `a` is listed while the arrows are off the police (`listed: offPolice`) and works either way.
- **The milestone alert keys on the steps met in order**, so a later step met out of turn is not a milestone until the steps before it are met.

# Presets are the routine's dials in one bundle

**An operation preset is a named list of the routine's existing commands** (#357, `content/presets.toml`, `engine/presets.go`, `game/presets.go`, `ui/presets.go`).
There is no mechanic of its own: applying a preset is issuing its commands by hand, and the player reviews exactly what it changes before anything moves.
A run that never applies one is byte for byte the run before the feature: the presets are reads and the commands the player already had, no sim reads the file, and nothing is saved in the world.

**The commands.**
`engine.Command{Op, City, Product, Route, N, Dial, On}` is one session command by its name on the wire, its dial by name: `cancel_standing`, `clear_supply`, `set_supply`, `place_standing`, `set_launder_dial`, `set_pay`, `set_route`, `set_route_target`, `set_route_days` and `set_lie_low` (`engine.Ops`, the order a preset issues them in, so a contract is set before a standing order counts on it).
`Session.Do(c)` issues one through the session method of that name; `TestPresetsOnTheWire` pins that every op is a command the protocol serves, so a client can issue a preset's commands itself.
`Session.PresetCommands(id)` is a preset's commands against the run now, only those that set something to what it is not.

**The built-ins** are `presets.toml`'s `[[preset]]` rows: `id`, `name`, `blurb` (the list's line of copy) and what it sets, a key left out leaving that setting alone (`content.PresetConfig`, strict and validated: a dial by one of its names, `standing = "cancel"`, `supply = "clear"`, and at least one setting).

| Preset | Sets |
|---|---|
| `quiet` Quiet trading | every standing order of yours at the quiet dial, the launder dial careful, every route running at slow |
| `preserve` Preserve cash | every supply contract of yours cleared (the standing orders kept), the pay dial fair, the launder dial normal |
| `push` Push | every standing order of yours aggressive, the launder dial greedy |
| `dark` Go dark | every standing order and supply contract of yours ended, lie low today |

A rule reads the routine as it stands: `standing` re-places each standing order of yours at its quantity and the new dial (`place_standing`), `routes` turns every route whose dial is on (an off route stays off).
The lieutenant's orders and contracts are theirs and never touched.

**Saved presets.**
`Session.Snapshot(name)` is the routine as it stands as a `game.Preset{Name, Day, Standing, Supply, Launder, Pay, Routes []RoutePreset{ID, Dial, Target, Days}}`: every standing order and contract of yours, the two dials, and every route running or keeping a target; lying low is the day's and never saved.
It lives in the **profile** (`Profile.Presets`, `SavePreset` replacing one of the same name in any case, `DeletePreset`, `Preset(name)`; `docs/profile.md`), not the run, so it carries across runs; `ProfileSchema` stays 1, a profile from before reading with none.
The front end hands them to the session with `UsePresets(saved)`, and `Presets()` lists the built-ins in the file's order then the saved ones (their id is their name; a built-in wins a name both have).
A saved preset's commands set the routine back to it: the standing orders and contracts it lacks are ended, its own set where they differ, the dials turned, each open route's dial and targets set (a route it lacks turned off and its targets cleared), and **whatever the run has not got is skipped**: a city not reached, a product not there, a route not open.

**The review.**
`Session.PresetDiff(id)` runs the commands on a copy of the run (`game.Encode` and `Decode`) and returns `engine.Review{Preset, Changes, Refused, Same}`; `ApplyPreset(id)` runs them on the run through `Do` and returns the review of what moved, which is the diff's (`TestPresetDiffIsExact`: for every preset the review before is the review after, the run untouched by the diff, and the settings listed are exactly those whose words differ between the routine before and after, read without the diff).
A `Change{Setting, City, Product, Route, From, To}` is one setting that moves (`standing`, `supply`, `launder`, `pay`, `route`, `target`, `lie_low`), in words (`40 normal` → `40 quiet`, `keep at 120` → `none`, `3 days`); a standing order carries `Was` and `Now` for the front end's estimate, a contract `Cost` and `CostTo`, its buy tomorrow morning as `market.Sim.Plan` lays it out now and after, and lying low `Dropped`, tonight's orders it drops.
A command the rules refuse is skipped and listed in `Refused{Command, Why}` with the game's words, and its setting is no change: the quiet dial on a standing order larger than the stash is `only 10 Pills in Eastside`, as `PlaceStanding` always refused it.
`Same` counts the routine's settings that stay as they are (the standing orders, contracts, running routes and targets either side holds, and the two dials).

**UI** (`ui/presets.go`, `modePresets`, `presetsDialog{stepper, cursor, list, review}`).
`P presets` on the market opens a modal with two pages.
The first is the list, `preset  what it sets`, the built-ins then yours (a saved one's line says the day and what it holds); `↑↓ pick`, `1-9 pick`, `enter review`, `s save current` (the routine saved as `Day N`, a second save that day replacing the first, the profile written at once), `x delete` on one of yours, `esc close`.
The second is the review: the blurb, a `setting  now  after  estimate` table, one row a change, the estimates labelled `~` (a standing order's take and heat a night by `orderEstimate`, as a share where it stands both sides, `~take -5.1%, heat -40%`, in money and heat where it comes or goes; a contract's morning buy; the orders lying low drops), then `N other settings of the routine stay as they are.` and a `Refused, left as it is:` line a refusal.
`enter apply` applies it and closes on `Quiet trading: 4 settings changed.` (the first refusal in `alarm` after it); with nothing to change it says so and stays; `⇧tab back`, `esc close`; `enter` never ends the day.
`preset` is a help WORD.
`TestPresetsInTheGrammar` walks it, `TestModalsFit` opens the list and the review at 80x24 and 120x40.

**Engine and web.**
On the wire (`docs/engine.md`) `presets`, `preset_commands [preset]` and `preset_diff [preset]` are queries and `apply_preset [preset]` a command; `Do`, `Snapshot` and `UsePresets` are not served, a saved preset being the TUI's profile's (a client keeps its own and can issue its commands).
The web page has a preset picker and `Review preset`, which shows the diff in a confirmation and applies it (`docs/web.md`).

**Rulings.**
The issue's *Preserve cash* pauses the contracts; there is no pause, so the preset clears them, and saving the routine first keeps a way back.
*Routes careful* is the route dial's `slow`, the careful notch, on the routes running; an off route is not switched on.
A saved preset is named for the day it was saved (`Day 34`) rather than typed: the UI has no text field and a modal's letters are keys; the engine and the profile take any name.
The diff runs on a copy rather than predicting each command, so a refusal (a standing order over the stash) is exactly the game's and never a guess.
`TestNoPresetIsTheOldRun` plays thirty days reading every preset's commands and diff each morning against a run that never looks, byte for byte, and `TestProfileNeverTouchesTheRun` holds with a saved preset in the full profile.

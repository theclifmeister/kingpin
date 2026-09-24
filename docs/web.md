# The web client

**The web client** (#328, `cmd/kingpin-web`) is the second front end, and the first graphical one.
The game is drawn in a browser: sprites on a canvas, moved by the engine's cues and played through the protocol alone.
It proves the engine is front-end agnostic.
It reads nothing but the view, the events and their cues, and acts through nothing but protocol calls, so what it needed and the engine lacked became engine issues (#332, #333), never workarounds.

```sh
go run ./cmd/kingpin-web              # builds the site and serves it on http://127.0.0.1:8080/ (?seed=7 pins the run)
go run ./cmd/kingpin-web -out site/   # writes the static site: any static host serves it
```

**The choice (the issue asked for it first).**
A browser page on the WebAssembly build (#327), drawn with Canvas 2D in plain ES modules, with no third-party code and no build step.
Why:

- **It needs nothing installed.** Godot, Unity or Unreal would each add an editor, a project format and a toolchain to the repo, and a client nobody here can build or test would drift. The browser is on every machine, and the site is static files: the page, its modules, `kingpin.wasm` and Go's `wasm_exec.js`.
- **It runs the engine in the page** (`kingpin.open()`, #327): no server, no socket, no process. Saves live in the browser (`export_save` into `localStorage`, `import_save` back).
- **It can be tested here.** The page's own modules run under Node against the same module (`TestWebClient`), and the page itself was played in headless Chromium to an ending with no console error.
- **A real engine can still come later.** The contract it would speak is the same: the protocol over the C library (`cmd/libkingpin`) or over WebSocket (#326). This client is what that one would be measured against.

**The files** (`cmd/kingpin-web/web/`, embedded in the command):

- `js/session.js`: the protocol as the client speaks it. `Session` wraps one `kingpin.open()`: `call(method, ...params)` sends a request line and keeps the `event` notifications (`take()` hands them over) and the last `view`. It throws `RPCError` (`refused` for `-32000`, the game's words) on an error. The moves the page makes are one method each: `newRun`, `endDay`, `fastForward`, `choose`, `travel`, `buy`, `maxBuy`, `restockPlan`, `presets`, `presetDiff`, `applyPreset`, `sell`, `exportSave`, `importSave`. A `no_room` refusal (`NO_ROOM`, `-32002`, #356) is `refused` too, with `free`, what fits. `streetConnect(view, city)` is the open street connect where you stand. It touches no DOM.
- **The versions.** `SUPPORTED` is `{protocol: [5], view: [8]}` (view 3 since #345, protocol 4 since #356, view 4 since #358, view 5 since #351, view 6 since #355, view 7 since #352, protocol 5 since #357, view 8 since #343). `checkVersions` refuses a module whose `kingpin.protocol` or `kingpin.view` is another (`VersionError`, shown full-page) before a run starts: a field renamed under the client would draw a wrong game instead of failing. A version bump in the engine fails `TestWebClient` until `SUPPORTED` moves with the client.
- `js/alerts.js` (#352): the alerts in words. `WORDS` is one sentence a kind, off the alert's fields and the view (a number the view left out is zero), `alertText(v, a)` the alert's; `PANELS` maps the engine's screens the page has onto its elements (the market onto the product table, the map onto the canvas) and `alertPanel(a)` is the element that answers an alert, or none. It touches no DOM.
- `js/autoplay.js`: `autoDay(session)` is the autopilot, the reference client's greedy dealer (`protocol.Play`) a day at a time. It answers a card with its first choice, spends 60% of the dirty cash across the street connect's products, sells everything at `aggressive` and ends the day. A refused move is part of play.
- `js/layout.js`: where things are, a pure function of the view and the canvas size.
  - The cities stand side by side in the view's order, each a block of its corners on their `city.toml` cells. One cell size fits every city.
  - Each road has its own lane under the blocks, and the deeper lanes leave the blocks wider, so the roads nest and none crosses another.
  - Corners are coloured green for yours, the faction's colour in the view's order for a rival's, and grey for the street's. A deed has a gold edge.
  - `cornerSpot`, `cityHead`, `routePath`, `along`, `memberSpot` (a member's post, a lieutenant's city, else beside you) and `houseSpot` place a cue's ids.
- `js/sprites.js`: placeholder pixel art, a string a row and a letter a colour, drawn a pixel a `fillRect`. The sprites are the player, a runner, an enforcer, a cop, a robber, the car, truck, boat and plane (a route's mode picks one), a squad car, a helicopter, a house, a coin, a skull, a flag, a crate, a badge and a fist.
- `js/cues.js`: `ANIMATIONS`, one entry for each of the 17 cues (`engine.CueKinds`). An entry turns a cue into timed drawings over the layout, `{dur, draw(ctx, p)}` with `p` running from 0 to 1:
  - `corner_claimed`: a flag goes up.
  - `corner_flip`: the old owner's colour drains off the corner.
  - `strike`: a fist shakes.
  - `rival_move`: the faction's colour closes in with an enforcer.
  - `robbery`: a masked man runs off.
  - `police`: a squad car arrives with its lights going.
  - `task_force`: a helicopter crosses the city.
  - `shipment`: a crate is loaded and the vehicle sets off; it drops at the far end; or it is seized under a badge.
  - The crew cues: a runner pops in, walks off, is arrested (or a skull, when shot) or bounces back.
  - `sale`: coins rise off your corners.
  - `market`: an arrow over the city.
  - `property`: a house builds or crumbles.
  - `overdose`: a skull.
  - `run`: a banner (the reign begins, is broken, the end).
  
  None of them reads the world: they draw only the cue and where the layout puts its ids.
- `js/scene.js`: `drawMap(ctx, L, now)` draws the blocks tinted by heat, the roads with their dials, the corners and who works or guards them, the houses, and the shipments on the road at their share of the trip. `Scene` owns the canvas and the frame loop. `play(cues)` queues a night's cues 160 ms apart, so a busy night reads as a sequence.
- `js/police.js` (#355): `policeLines(view, city)` and `fileWord(view)`, the law panel as `{text, warn}` lines off the view's `ladder` and the law's lines, the TUI's POLICE section in sentences. It touches no DOM.
- `js/main.js`: the page. It loads the module, opens a session and starts a run (the URL's `?seed=`, else a random one).
  - The header shows the day, the tier, the cash, the net worth and the file, `file 3/6` against the arrest line (red within two pages).
  - The panel opens on NEEDS YOU, the view's `alerts` in words (#352): each a button that brings its panel into view and flashes it where the page has that screen (`alertPanel`), its words alone where it does not yet.
  - The panel is the city you stand in. Each product shows the street price, the connect's price and what you hold, with a buy quantity and buy and sell buttons at the chosen dial. Below is the law (#355): the police risk where you stand, a line each for the heat and the next rung, every rung with what it takes, the file and what fills it, the pressure and the goodwill, the dirty pile against the exposure line with the fronts' cover, and the cop's word as an estimate or `No word from inside`, what is close in red. Below that are travel, the pool looking for work (the view's `pool`, #332: each with a hire button, off when the fee is more than your dirty cash) and the morning report's sections. The money section is the night's cash flow (#351, `report.flow`, `flowTable`): the opening, a row a category that moved, dirty, clean and both, the big ones in green or red, and the closing; a night that moved nothing falls back to the money lines.
  - The buy (#356) is the TUI's: the quantity's `max` is `max_buy`, clamped into the default of 30% of the cash, a `Max` button fills it, and the line under it is `after $X dirty · room H/C` as the number changes, red when either is over. A buy refused for the room sets the quantity to what fits, so the next click buys it. `Restock` for N days of demand (2 by default) shows `restock_plan` in a confirmation and buys its lines one `buy` each.
  - A preset picker and `Review preset` (#357): `preset_diff` for the chosen one in a confirmation, one line a change (`launder: normal → careful`) and the refusals, then `apply_preset`; the list is `presets`, filled when the run starts.
  - The footer holds end day (`space`), the next 7 days (`fast_forward`), the autopilot (`a`) and a toast that gives a refusal in the game's words, or where a fast-forward stopped (an alert in its words).
  - A card is a modal with its choices. Each is a button with its label and, under it, its `preview` (#358): the chips joined with ` · `, a gain green, a cost red, a line crossed gold, a note dim. The ending is an overlay that waits for the night's last animation.
- `cmd/kingpin-web` builds the site (`Build(dir)`): the embedded files, `kingpin.wasm` built for `js/wasm`, and the toolchain's own `wasm_exec.js`, which must match the Go that built the module. It serves the site on `-addr`, or writes it to `-out` and exits.

**What it does not do yet.**
It covers the loop the issue asked for: buy, sell, travel, end the day, answer a card, see the ending.
It hires from the pool (#332) and lists what needs you (#352).
Everything else the TUI offers is still missing: routes, houses, fronts, the crew's posts, contracts, the factions' offers, the upgrade tree and the rivals' table. The law is read (#355) but not acted on: no bribe, fund or cop yet.
Each is a panel over what the view carries (the contracts, offers and tree since #332) and the commands and quotes (#325) the protocol already serves.
The art is placeholder.

**What pins it.**

- `TestWebClient` builds the site as the command does and runs `testdata/play.mjs` under Node. The script loads the page's own modules with the engine loaded the way the page loads it, minus the DOM.
  - The client speaks this build's protocol and view versions and refuses another.
  - `ANIMATIONS` is `engine.CueKinds`, no more and no less, and every sprite's rows are the same width.
  - Seed 7 with the autopilot reaches an ending (indicted, day 30).
  - Every morning of those runs draws the law panel: a line for the heat and one a rung of the view's ladder, the header's `file E/A`, and no `undefined` or `NaN` (#355).
  - A buy of `max_buy` on day 0 is never refused, and a restock plan's lines each buy (#356).
  - Every card the four runs meet has a label and at least one chip in one of the four tones on every choice (#358), and at least one card comes up.
  - The presets (#357): the list holds the built-ins, `preset_diff` leaves the view as it was, and `apply_preset` returns the review `preset_diff` gave.
  - Seeds 7, 11, 23 and 42, and 120 nights of the harness's `boss` on seed 3 (written by the Go test, so the client sees a run that ships, hires and fights), draw every day's map. Every cue in those runs names a corner or route on the map, gives animations with a length, and draws at `p` 0, ½ and 1, on a context that records nothing but must not throw.
  - That is 14 of the 17 kinds from the engine itself. A kind no run gave (`overdose`, `strike` and `task_force` today) is made up on the boss's last morning, with ids off its map and in each of its phases, and drawn too.
  - Every alert of every played morning and every boss night is worded with nothing missing in it and carries an act, `WORDS` is `engine.AlertKinds`, no more and no less, and some alert over the runs links to a panel (#352).
  - In CI, a missing `node` fails the test rather than skipping it.
- `engine.CueKinds` is the list the table is held to. `TestCueKindsIsEveryCue` pins it to the cue table.
- The page was played by hand and by the autopilot in headless Chromium (Playwright) to an ending: seed 7 is indicted on day 31, with no console error.

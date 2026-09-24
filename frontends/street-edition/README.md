# Street Edition

An alternative illustrated browser frontend for Kingpin, tracked by #383 (named Ink & Ambition until #409, after its art style; renamed for the street the game is played on, and the paper it reads like). Paper, navy ink, coral, teal and gold surround a hand-drawn city. The original Go engine runs in WebAssembly; all actions go through the JSON-RPC session.

## Build and play

Requirements: Python 3.11+, the Go version declared in the repository's `go.mod`, and Node 20+ for the smoke check. Run from the repository root:

```sh
python3 frontends/street-edition/build.py
node frontends/street-edition/smoke.mjs
python3 -m http.server 8080 --bind 127.0.0.1 --directory frontends/street-edition/dist
```

Open http://127.0.0.1:8080. `KINGPIN_GO=/path/to/go` selects a Go executable. The generated `dist/` folder can be served by any static host; there is no server-side game process. Generated WASM, shared JS, loader and derived content are not committed. The builder refuses a different protocol/view contract until this frontend is reviewed. It copies `session.js`, `alerts.js` and `police.js` from the reference web client, and reads front roles and trait descriptions from the same checkout's TOML.

## Deploying on Vercel

The site that plays is `dist/`, which the build makes; `src/` alone hangs on "Opening the city ledger…", because it has no engine or helpers. `vercel.json` has Vercel run `vercel-build.sh`, which fetches the Go version `go.mod` names (Vercel's build image has no Go), picks a Python 3.11+ and runs `build.py`, then serves `dist/`. The build stamps the commit from `VERCEL_GIT_COMMIT_SHA`, because the checkout has no `.git`. Set the project's root directory to `frontends/street-edition` and keep "Include files outside the root directory in the Build Step" on (Vercel's default), because the build compiles the engine from the whole repository (#411).

## Play

Start with $500 on seed 41. Buy stock in Market, queue sales, preview tonight and end the day. Your story offers new games, export and import. Saves stay in this browser under `kingpin-street-v1` (a save still under the old `kingpin-ink-v1` is read and moved over); exported `.gob` saves move between the browser and the other engine frontends. Loading failures retain the existing saved bytes. There are no cloud saves or multiplayer.

## Controls

- Streets: city travel, corner assignment/abandonment, enforcer strikes, police tips and undercutting.
- Market: purchases, max-buy capacity, reviewed restocking, queued sales, private buyers, routes and reviewed built-in operation presets.
- Crew: hire/dismiss, assignments, pay policies, bonuses, veteran traits and city captains with nightly budgets.
- Empire: front roles, investment levels, upgrades, properties and assets; export lanes (what each waits on, or its capacity, order, rate abroad and loads out, and an order form) and trophies (owned, and the offers to buy with clean cash).
- Rivals: scout, negotiate, accept/decline offers, expansion warnings and confront scouts.
- Ledger: offshore transfers, cash out (clean back to dirty at the quoted fee), community funding, engine-calculated ambitions and the four ending actions, each closed one saying what its plan still needs.
- Paper: engine-ordered reports, leading stories, cash-flow reconciliation and 90 report editions.
- Persistent risk panel: heat ladder, evidence, cash exposure, tonight's count (the pile the police will count after tonight's landings and wages, before the wash) and its heat, investigation warnings and alert navigation.

Dilemmas display the engine's consequence chips. The day preview is explicitly an estimate and explains what it cannot predict. It requires a separate end-day action.

## Boundaries and artwork

This UI does not yet expose every engine command: cooking/cutting, undercover work, detailed politics and direct standing-order configuration are not included. Built-in presets can operate on routines in imported saves; custom saved presets remain a TUI-profile feature. Bayport reuses the city illustration. Optional, feature-detected WebMCP tools read state and navigate sections; they do not play turns. `?test=1` exposes a local browser test hook.

`src/assets/city.webp` is original AI-generated artwork created for this frontend in the accompanying development session. It is decorative: the labelled controls carry real game state. It is not a geographic map. Go's loader license is copied into the build output. No Sites project metadata or user playthrough saves are included.

See `VALIDATION.md` for the prototype's validation, and `docs/web.md` for the repository's reference client and protocol contract.

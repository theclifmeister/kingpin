// The web client's acceptance (#328), run by TestWebClient under Node:
// the page's own modules, with the engine loaded as the page loads it,
// minus the DOM. It checks the client's versions against the engine's,
// its animation table against engine.CueKinds, and plays seeds to see
// every cue of the runs animate and every day's map draw, on a context
// that records nothing but must not throw. It prints what it found as
// JSON for the Go test to judge.
import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";

const [site, kindsJSON, bossPath] = process.argv.slice(2);
const kinds = JSON.parse(kindsJSON);
const require = createRequire(import.meta.url);

globalThis.require = require;
globalThis.fs = fs;
globalThis.path = path;
globalThis.TextEncoder ??= require("util").TextEncoder;
globalThis.TextDecoder ??= require("util").TextDecoder;
globalThis.performance ??= require("perf_hooks").performance;
globalThis.crypto ??= require("crypto");
require(path.join(site, "wasm_exec.js"));

const mod = (name) => import(pathToFileURL(path.join(site, "js", name)).href);
const { Session, SUPPORTED, VersionError } = await mod("session.js");
const { autoDay } = await mod("autoplay.js");
const { ANIMATIONS } = await mod("cues.js");
const { layout } = await mod("layout.js");
const { drawMap } = await mod("scene.js");
const { SPRITES } = await mod("sprites.js");
const { fileWord, policeLines } = await mod("police.js");

const go = new Go();
const { instance } = await WebAssembly.instantiate(fs.readFileSync(path.join(site, "kingpin.wasm")), go.importObject);
go.run(instance);
const kingpin = globalThis.kingpin;

// ctx is a 2D context that draws nothing: any method is a no-op, any
// property takes what is set.
const ctx = new Proxy({}, { get: (t, k) => (k in t ? t[k] : () => {}), set: (t, k, v) => ((t[k] = v), true) });

const out = { engine: { protocol: kingpin.protocol, view: kingpin.view }, supported: SUPPORTED, problems: [] };
const problem = (s) => out.problems.length < 40 && out.problems.push(s);

// The versions: this build's are spoken, another is refused.
out.accepts = SUPPORTED.protocol.includes(kingpin.protocol) && SUPPORTED.view.includes(kingpin.view);
try {
  new Session({ ...kingpin, protocol: kingpin.protocol + 100 });
  out.refusesOther = false;
} catch (e) {
  out.refusesOther = e instanceof VersionError;
}

// The table: every cue the engine gives, and nothing it does not.
out.missing = kinds.filter((k) => !ANIMATIONS[k]);
out.extra = Object.keys(ANIMATIONS).filter((k) => !kinds.includes(k));

for (const [name, rows] of Object.entries(SPRITES)) {
  if (rows.some((r) => r.length !== rows[0].length)) problem(`sprite ${name} has ragged rows`);
}

// animate plays one cue over a layout: it must name what is on the
// map, give animations and draw them.
function animate(c, L, v, where) {
  if (!ANIMATIONS[c.kind]) return;
  if ((c.corner && !L.corners[c.corner]) || (c.route && !(v.routes || []).some((r) => r.id === c.route) && c.kind !== "crew_down")) {
    problem(`${where}: ${c.kind} names ${c.corner || c.route}, which is not on the map`);
  }
  const anims = ANIMATIONS[c.kind](c, L);
  if (!Array.isArray(anims) || anims.length === 0) problem(`${where}: ${c.kind} gives no animation`);
  for (const a of anims) {
    if (!(a.dur > 0) || typeof a.draw !== "function") problem(`${where}: ${c.kind} gives an animation with no length or drawing`);
    for (const p of [0, 0.5, 1]) a.draw(ctx, p);
  }
}

// police is the law panel (#355) on a morning: a line for the heat,
// one a rung of the view's ladder, the file with its arrest line in the
// header, and nothing that reads undefined or NaN.
function police(v, where) {
  const c = v.cities.find((x) => x.id === v.you.city);
  const lines = policeLines(v, v.you.city);
  if (!c || !c.ladder.length) problem(`${where}: no ladder where you stand`);
  else if (lines.length < 1 + c.ladder.length) problem(`${where}: the law panel has ${lines.length} lines for ${c.ladder.length} rungs`);
  if (v.law.arrest_line > 0 && fileWord(v) !== `${v.you.evidence}/${v.law.arrest_line}`) problem(`${where}: the header reads file ${fileWord(v)}`);
  for (const l of lines) if (/undefined|NaN/.test(l.text)) problem(`${where}: the law panel reads "${l.text}"`);
}

// play is one seed with the autopilot until it ends or days run out.
function play(seed, days) {
  const s = new Session(kingpin);
  let v = s.newRun(seed);
  s.take();
  const seen = {};
  let day = 0;
  for (; day < days && !v.over; day++) {
    v = autoDay(s);
    const L = layout(v, 1200, 760);
    drawMap(ctx, L, day * 16);
    police(v, `seed ${seed} day ${v.day}`);
    for (const e of s.take()) {
      if (!e.cue) continue;
      seen[e.cue.kind] = (seen[e.cue.kind] || 0) + 1;
      animate(e.cue, L, v, `seed ${seed} day ${v.day}`);
    }
  }
  return { seed, day: v.day, over: v.over ? v.over.cause : "", seen };
}

// Hiring (#332): the pool's first id, by the page's own call, lands on
// the payroll.
{
  const s = new Session(kingpin);
  const v = s.newRun(7);
  const id = (v.pool[0] || {}).id;
  if (!id) problem("nobody in the pool on day 0");
  else {
    s.hire(id);
    const after = s.refresh();
    if (!after.crew.some((m) => m.id === id) || after.pool.some((m) => m.id === id)) problem(`hired ${id} and not on the payroll`);
  }
}

out.reference = play(7, 400);
out.more = [11, 23, 42].map((seed) => play(seed, 150));

// The boss's nights: the cues of a run that ships, hires and fights,
// each over the morning it led to.
const nights = JSON.parse(fs.readFileSync(bossPath, "utf8"));
const bossSeen = {};
let last = null;
for (const n of nights) {
  const L = layout(n.view, 1200, 760);
  drawMap(ctx, L, 0);
  police(n.view, `boss day ${n.view.day}`);
  for (const c of n.cues || []) {
    bossSeen[c.kind] = (bossSeen[c.kind] || 0) + 1;
    animate(c, L, n.view, `boss day ${n.view.day}`);
  }
  last = n.view;
}
out.boss = { seed: 3, day: last.day, over: last.over ? last.over.cause : "", seen: bossSeen };

// A cue no run gave is made up on the boss's last morning, with ids off
// its map, in each of its phases, and drawn too.
const all = new Set([bossSeen, out.reference.seen, ...out.more.map((r) => r.seen)].flatMap((seen) => Object.keys(seen)));
const L = layout(last, 1200, 760);
const city = last.cities[0].id;
const corner = last.cities[0].corners[0].id;
const first = (list) => (list || [])[0] || {}; // an empty list comes as null
const ids = {
  day: last.day, city, corner, faction: first(last.factions).id, member: first(last.crew).id || 1,
  route: first(last.routes).id, from: city, to: (last.cities[1] || last.cities[0]).id, house: first(last.houses).id,
  product: "weed", units: 10, shipment: 1, level: "raid",
};
const phases = { shipment: ["sent", "landed", "seized"], property: ["bought", "lost"], run: ["began", "broke", "ended"], market: ["shock", "slump"], crew_down: [""], corner_flip: [""] };
out.synthetic = [];
for (const k of kinds) {
  if (all.has(k)) continue;
  out.synthetic.push(k);
  for (const phase of phases[k] || [""]) {
    const c = { kind: k, ...ids, phase, from: k === "corner_flip" ? "rival" : ids.from, to: k === "corner_flip" ? "player" : ids.to };
    animate(c, L, last, `made-up ${k}`);
    if (k === "crew_down") animate({ ...c, dead: true }, L, last, `made-up ${k}`);
  }
}
console.log(JSON.stringify(out));
process.exit(0);

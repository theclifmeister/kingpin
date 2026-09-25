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

const [site, kindsJSON, bossPath, alertKindsJSON, exitsPath] = process.argv.slice(2);
const kinds = JSON.parse(kindsJSON);
const alertKinds = JSON.parse(alertKindsJSON);
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
const { Session, SUPPORTED, VersionError, streetConnect } = await mod("session.js");
const { autoDay } = await mod("autoplay.js");
const { ANIMATIONS } = await mod("cues.js");
const { layout } = await mod("layout.js");
const { drawMap } = await mod("scene.js");
const { SPRITES } = await mod("sprites.js");
const { WORDS, PANELS, alertText, alertPanel } = await mod("alerts.js");
const { fileWord, policeLines } = await mod("police.js");
const { howTheyCome, roleLines, temperLine, tempers } = await mod("lieutenants.js");
const { EXITS, exits, claim, tonightText } = await mod("exits.js");

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

// The alerts (#352): words for every kind the engine raises, and no
// more.
out.alertsMissing = alertKinds.filter((k) => !WORDS[k]);
out.alertsExtra = Object.keys(WORDS).filter((k) => !alertKinds.includes(k));
out.alertsWorded = 0;
out.alertsLinked = 0;

// word checks every alert of a morning: words with nothing missing in
// them, an act on a screen, and a panel the page has or none.
function word(v, where) {
  for (const a of v.alerts || []) {
    const text = alertText(v, a);
    if (typeof text !== "string" || !text || /undefined|NaN|null/.test(text)) problem(`${where}: ${a.kind} reads ${JSON.stringify(text)}`);
    if (!a.act || !a.act.screen) problem(`${where}: ${a.kind} has no act`);
    const panel = alertPanel(a);
    if (panel && !Object.values(PANELS).includes(panel)) problem(`${where}: ${a.kind} links to ${panel}`);
    out.alertsWorded++;
    if (panel) out.alertsLinked++;
  }
  // The lead (#354): words and an act on a screen, a panel the page has
  // or none; the sections one ordered list, every one of them.
  const rep = v.report || {};
  for (const l of rep.lead || []) {
    if (!l.text || !l.act || !l.act.screen) problem(`${where}: lead ${JSON.stringify(l)}`);
    const panel = alertPanel(l);
    if (panel && !Object.values(PANELS).includes(panel)) problem(`${where}: lead ${l.kind} links to ${panel}`);
  }
  if (v.day > 0 && (!Array.isArray(rep.sections) || rep.sections.length !== 15 || rep.sections[0].id !== "incident")) {
    problem(`${where}: the report's sections ${JSON.stringify((rep.sections || []).map((s) => s.id))}`);
  }
}

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

// A card's choices (#358) each carry a label and what they do, a chip
// a thing in one of the four tones.
let cards = 0;
function checkCard(card, where) {
  cards++;
  for (const c of card.choices) {
    if (typeof c.label !== "string" || !Array.isArray(c.preview) || c.preview.length === 0) problem(`${where}: card ${card.id} has a choice with no preview`);
    else if (c.preview.some((p) => !p.text || !["gain", "cost", "line", "note"].includes(p.tone))) problem(`${where}: card ${card.id} has a chip ${JSON.stringify(c.preview)}`);
  }
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
    word(v, `seed ${seed} day ${v.day}`);
    if (v.card) checkCard(v.card, `seed ${seed} day ${v.day}`);
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

// The lieutenants (#455): the role in words off rules.crew.lieutenancy,
// every temper in a line, the terms' cut and slots in the words, and the
// hint on a run with one city held; assign and unassign by the page's
// own calls reach the engine (refused here: nobody to assign, the
// game's words, not a protocol error).
{
  const s = new Session(kingpin);
  const v = s.newRun(7);
  const t = s.lieutenancy();
  const lines = roleLines(t);
  if (!(t.Cut > 0) || !(t.Crew > 0) || (t.Tempers || []).length !== 4) problem(`lieutenancy terms: ${JSON.stringify(t)}`);
  if (lines.some((l) => /undefined|NaN/.test(l))) problem(`lieutenant lines: ${lines}`);
  if (!lines[1].includes(`${Math.round(t.Cut * 100)}%`) || !lines[1].includes(`${t.Crew} crew slots`)) problem(`the cut line: ${lines[1]}`);
  if (tempers(t) !== "violent, greedy, careful or steady") problem(`the tempers: ${tempers(t)}`);
  for (const tt of t.Tempers || []) if (/undefined|NaN/.test(temperLine(tt)) || !temperLine(tt).startsWith(`sells ${tt.Dial}`)) problem(`temper ${tt.Name}: ${temperLine(tt)}`);
  if (!howTheyCome(v).includes("two cities")) problem(`the hint on day 0: ${howTheyCome(v)}`);
  try {
    s.assign(999, v.cities[1].id);
    problem("assign of nobody went through");
  } catch (e) {
    if (e.code !== -32000) problem(`assign refused with ${e.code}: ${e.message}`);
  }
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

// Buying (#356): max_buy is never refused, and a restock plan is a
// list its lines buy (the no_room code is the protocol test's).
{
  const s = new Session(kingpin);
  const v = s.newRun(7);
  const k = streetConnect(v, v.you.city);
  const id = k && Object.keys(k.prices)[0];
  if (!id) problem("no street connect selling on day 0");
  else {
    const room = s.maxBuy(k.id, id);
    if (!(room.max > 0) || !(room.capacity > 0)) problem(`max_buy on day 0: ${JSON.stringify(room)}`);
    else {
      try {
        s.buy(k.id, id, room.max);
      } catch (e) {
        problem(`buying max_buy's ${room.max} was refused: ${e.message}`);
      }
    }
    const plan = s.restockPlan(v.you.city, 2);
    if (!Array.isArray(plan)) problem("restock_plan is not a list");
    for (const l of plan) {
      if (!(l.units > 0) || !(l.cost > 0)) problem(`restock line ${JSON.stringify(l)}`);
      s.buy(l.supplier, l.product, l.units);
    }
  }
}

// The presets (#357): the list, a diff that leaves the run alone, and
// the apply that does what the diff said.
{
  const s = new Session(kingpin);
  s.newRun(7);
  const list = s.presets();
  if (!Array.isArray(list) || !list.some((p) => p.id === "dark")) problem(`presets: ${JSON.stringify(list)}`);
  s.call("set_launder_dial", "greedy");
  const before = s.refresh();
  const review = s.presetDiff("quiet");
  if (!review.changes.some((c) => c.setting === "launder" && c.to === "careful")) problem(`preset_diff: ${JSON.stringify(review)}`);
  if (JSON.stringify(s.refresh()) !== JSON.stringify(before)) problem("preset_diff changed the run");
  const applied = s.applyPreset("quiet");
  if (JSON.stringify(applied) !== JSON.stringify(review)) problem(`apply_preset did not do what preset_diff said: ${JSON.stringify(applied)}`);
}

// The day's preview (#353): a sale queued shows in tonight's money, the
// flow's lines are every category, and the preview changes nothing.
{
  const s = new Session(kingpin);
  const v = s.newRun(7);
  const k = streetConnect(v, v.you.city);
  const id = k && Object.keys(k.prices).sort((a, b) => k.prices[a] - k.prices[b])[0];
  const qty = id ? Math.min(10, Math.floor(v.you.dirty_cash / (2 * k.prices[id]))) : 0;
  if (qty > 0) {
    s.buy(k.id, id, qty);
    s.sell(v.you.city, id, qty, "normal");
    const before = JSON.stringify(s.refresh());
    const p = s.preview();
    if (!p || p.day !== v.day + 1) problem(`preview on day ${v.day}: ${JSON.stringify(p)}`);
    else {
      if (p.flow.lines.length !== 10 || !(p.flow.lines[0].dirty > 0)) problem(`the preview's flow: ${JSON.stringify(p.flow)}`);
      if (!p.sales.length || !Array.isArray(p.idle) || !Array.isArray(p.unknown) || !p.unknown.length) problem(`the preview's lists: ${JSON.stringify(p)}`);
      if (JSON.stringify(p.alerts) !== JSON.stringify(s.view.alerts)) problem("the preview's alerts are not the morning's");
      for (const a of p.alerts) if (!a.act || !a.act.screen) problem(`a preview alert with no act: ${JSON.stringify(a)}`);
      const sum = p.flow.lines.reduce((n, l) => n + l.dirty + l.clean, 0);
      if (p.flow.opening.dirty + p.flow.opening.clean + sum !== p.flow.closing.dirty + p.flow.closing.clean) problem("the preview's flow does not add up");
    }
    if (JSON.stringify(s.refresh()) !== before) problem("the preview changed the run");
  } else problem("nothing affordable on day 0 to preview a sale of");
}

out.reference = play(7, 400);
out.more = [11, 23, 42].map((seed) => play(seed, 150));
if (cards === 0) problem("no card came up in four runs");

// The boss's nights: the cues of a run that ships, hires and fights,
// each over the morning it led to.
const nights = JSON.parse(fs.readFileSync(bossPath, "utf8"));
const bossSeen = {};
let last = null;
for (const n of nights) {
  word(n.view, `boss day ${n.view.day}`);
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
const phases = { shipment: ["sent", "landed", "seized"], property: ["bought", "lost"], run: ["began", "broke", "straight", "lapsed", "ended"], market: ["shock", "slump"], crew_down: [""], corner_flip: [""] };
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
// The ways out and the money (#405): each save the Go test built has
// one way out open; the page's exits read it open and the rest closed
// with what is short, the claim ends the run on it, and on the first a
// cash-out lands the clean cash dirty less the fee the rule quotes, and
// the tonight line reads.
const saves = JSON.parse(fs.readFileSync(exitsPath, "utf8"));
out.exits = {};
for (const e of EXITS) {
  const s = new Session(kingpin);
  s.importSave(saves[e.id]);
  const read = exits(s.view);
  const mine = read.find((x) => x.id === e.id);
  for (const x of read) if (x.id !== e.id && (x.open || !x.why || /undefined|NaN|null/.test(x.why))) problem(`${e.id}'s save: ${x.id} reads ${JSON.stringify(x)}`);
  if (e.id === "retire") {
    const before = s.view.you;
    const fee = s.cashOutFee(1000);
    s.cashOut(1000);
    const after = s.refresh().you;
    out.cashOut = { dirty: after.dirty_cash - before.dirty_cash, clean: before.clean_cash - after.clean_cash, fee };
    const t = tonightText(s.forecast());
    if (!/Tonight the police count/.test(t) || /undefined|NaN/.test(t)) problem(`the tonight line: ${t}`);
  }
  let over = "";
  try {
    over = claim(s, e.id).over?.cause || "";
  } catch (err) {
    problem(`${e.id}: the claim was refused: ${err.message}`);
  }
  out.exits[e.id] = { open: mine.open, over };
}
// A closed way out is refused by the engine, not the page.
{
  const s = new Session(kingpin);
  s.newRun(7);
  try {
    s.crown();
    problem("the crown was taken on day 0");
  } catch (err) {
    if (!err.refused) problem(`the crown on day 0 threw ${err}`);
  }
}

console.log(JSON.stringify(out));
process.exit(0);

import assert from "node:assert/strict";
import fs from "node:fs";
import vm from "node:vm";
import { webcrypto } from "node:crypto";
import { pathToFileURL, fileURLToPath } from "node:url";
import path from "node:path";

const dist = path.join(path.dirname(fileURLToPath(import.meta.url)), "dist");
globalThis.crypto ??= webcrypto;
vm.runInThisContext(
  fs.readFileSync(path.join(dist, "assets/wasm_exec.js"), "utf8"),
);
const go = new Go();
const { instance } = await WebAssembly.instantiate(
  fs.readFileSync(path.join(dist, "assets/kingpin.wasm")),
  go.importObject,
);
void go.run(instance);
const { Session } = await import(pathToFileURL(path.join(dist, "session.js")));
const session = new Session(globalThis.kingpin);
session.newRun(41);
// A list with nothing in it is [] on the wire, never null: with every
// asset owned, Properties & assets filtered a null asset_offers and
// crashed. Day 0 raises no alert, so alerts is an empty list.
assert.deepEqual(session.call("alerts"), [], "an empty list is []");
const before = JSON.stringify(session.refresh());
assert.ok(session.preview());
for (const preset of session.presets()) session.presetDiff(preset.id);
assert.equal(
  JSON.stringify(session.refresh()),
  before,
  "read queries changed visible game state",
);
for (let i = 0; i < 45 && !session.view.over; i++) {
  if (session.view.card) {
    assert.equal(typeof session.view.card.choices[0].label, "string");
    assert.ok(Array.isArray(session.view.card.choices[0].preview));
    // The card's outcome is what the modal shows in its place (#548).
    const answer = session.choose(0);
    assert.ok(typeof answer.Outcome === "string" && answer.Outcome, "a card's outcome");
  }
  const v = session.view;
  const supplier = v.connects.find(
    (s) => s.open && !s.wholesale && s.city === v.you.city,
  );
  if (supplier) {
    const n = Math.min(10, session.maxBuy(supplier.id, "weed").max);
    if (n) {
      session.buy(supplier.id, "weed", n);
      // Every approach the sales dial offers reaches the engine (#548:
      // Normal could not be picked).
      session.sell(v.you.city, "weed", n, ["quiet", "normal", "aggressive"][i % 3]);
    }
  }
  session.endDay();
  const report = session.view.report;
  assert.ok(Array.isArray(report.sections));
  // The MONEY section's cash line (#548): before and after are the
  // flow's opening and closing.
  assert.equal(report.cash_before, report.flow.opening.dirty + report.flow.opening.clean);
  assert.equal(report.cash_after, report.flow.closing.dirty + report.flow.closing.clean);
  for (const pile of ["dirty", "clean"]) {
    assert.equal(
      report.flow.opening[pile] +
        report.flow.lines.reduce((n, l) => n + l[pile], 0),
      report.flow.closing[pile],
    );
  }
  session.take();
}
// Protocol 11–15 and view 12 (#407): the lanes and trophies in the
// view, a lane's order, the trophy offers, the forecast, the cash-out's
// fee and refusal, and the four ways out's queries.
{
  const v = session.refresh();
  assert.ok(Array.isArray(v.exports) && v.exports.length > 0, "view 12 carries the lanes");
  assert.ok(Array.isArray(v.trophies), "view 12 carries the trophies");
  const lane = v.exports[0];
  session.call("set_export", lane.id, lane.products[0], 100);
  const ordered = session.refresh().exports.find((l) => l.id === lane.id);
  assert.equal(ordered.product, lane.products[0]);
  assert.equal(ordered.units, 100);
  session.call("set_export", lane.id, lane.products[0], 0);
  assert.ok(!session.refresh().exports.find((l) => l.id === lane.id).units, "0 units turns the lane off");
  assert.ok(Array.isArray(session.call("trophy_offers")));
  const f = session.call("forecast");
  for (const k of ["dirty", "landings", "loads", "wages", "pile", "line", "heat"]) assert.equal(typeof f[k], "number", `forecast.${k}`);
  assert.equal(f.pile, f.dirty + f.landings - f.wages);
  assert.ok(session.call("rules.laundering.cash_out_fee", 100000) > 0);
  const clean = session.view.you.clean_cash;
  if (clean > 0) {
    const dirty = session.view.you.dirty_cash,
      fee = session.call("rules.laundering.cash_out_fee", clean);
    session.call("cash_out", clean);
    const after = session.refresh().you;
    assert.equal(after.clean_cash, 0);
    assert.equal(after.dirty_cash, dirty + clean - fee);
  } else assert.throws(() => session.call("cash_out", 1000), "cash_out with no clean cash is refused");
  assert.equal(typeof session.call("rules.laundering.can_go_straight"), "boolean");
  assert.equal(typeof session.call("rules.laundering.can_retire"), "boolean");
  for (const ending of ["retired", "kingpin", "vanished", "businessman"])
    assert.ok(session.view.ambitions.some((a) => a.ending === ending), `an ambition plans ${ending}`);
  assert.throws(() => session.call("go_straight"), "going straight before the streak is refused");
}
{
  // The lieutenants (#455): the terms the crew tab and the city picker
  // word, in the shared module's words, and the picker's calls.
  const { roleLines, temperLine, temperOf } = await import(pathToFileURL(path.join(dist, "lieutenants.js")));
  const lt = session.call("rules.crew.lieutenancy");
  assert.ok(lt.Cut > 0 && lt.Crew > 0 && lt.Tempers.length === 4, "the lieutenancy terms");
  for (const l of roleLines(lt)) assert.doesNotMatch(l, /undefined|NaN/, "a lieutenant line");
  assert.match(temperLine(temperOf(lt, "violent")), /^sells aggressive/, "the violent temper's line");
  assert.throws(() => session.call("assign", 999, session.view.cities[0].id), "assigning nobody is refused");
}
{
  // The crew's answers (#551): the confirms and the tab in the TUI's
  // words, off the engine's numbers, and the acts they ask for.
  const crew = await import(pathToFileURL(path.join(dist, "crew.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  assert.ok(engineInfo.takenOutMuscle > 0 && engineInfo.investigateLoyalty > 0 && engineInfo.sloppySkill > 0, "the crew's numbers from the TOML");
  const s = new Session(globalThis.kingpin);
  let v = s.newRun(41);
  const tun = s.call("rules.crew.tuning");
  for (const m of [...v.pool].sort((a, b) => a.fee - b.fee)) if (m.fee <= s.refresh().you.dirty_cash) s.call("hire", m.id);
  v = s.refresh();
  assert.ok(v.crew.length >= 1, "somebody hired");
  const m = v.crew[0];
  assert.match(crew.fireCost(m, tun), new RegExp(`lose ${Math.round(tun.FireLoyalty)} loyalty`), "the fire's cost");
  assert.match(crew.fireCost({ ...m, exposed: true }, tun), /stops growing/, "the snitch costs nothing");
  const low = { ...v, crew: v.crew.map((o, i) => (i ? { ...o, loyalty: tun.QuitThreshold + 1 } : o)) };
  if (v.crew.length > 1) assert.match(crew.fireWalkLine(low, low.crew[0], tun), /within .* of the walk line/, "the walk line names the near");
  assert.equal(crew.fireWalkLine({ ...v, crew: [m] }, m, tun), "", "nobody near, no walk line");
  // The war line: the last enforcer, a war on, the run can end taken out.
  const need = engineInfo.takenOutMuscle,
    enf = { id: 90, name: "Ace", role: "enforcer", loyalty: 60 },
    war = { ...v, crew: [enf], factions: [{ id: "f1", leader: "Preacher", alive: true, war: 0 }], you: { ...v.you, war: "f1" } };
  assert.match(crew.fireWarLine(war, enf, need, 50), /^At war with Preacher's crew, this leaves 0 of .*the run ends taken out\.$/, "the war line");
  assert.equal(crew.fireWarLine({ ...war, you: { ...v.you, war: "" } }, enf, need, 50), "", "no war, no war line");
  assert.match(crew.fireWarLine({ ...war, you: { ...v.you, war: "" }, factions: [{ ...war.factions[0], war: 50 }] }, enf, need, 50), /taken out/, "a war over the threshold");
  const cost = s.call("rules.crew.investigate_cost"),
    odds = s.call("rules.crew.investigate_odds"),
    lines = crew.investigateLines(v, cost, odds, engineInfo.investigateLoyalty);
  assert.ok(lines[0].includes("$" + cost.toLocaleString("en-US")) && lines[2].includes(`~${Math.round(odds * 100)}%`), "the investigation's price and odds");
  assert.match(crew.payOffLines(m, s.call("rules.crew.payoff_cost", m.id), s.call("rules.crew.payoff_loyalty"))[0], /^\$[\d,]+ for .*: loyalty \d+ → \d+\.$/, "the pay-off line");
  assert.equal(crew.crewTag(v, { wounded: 3 }), "laid up 3d", "laid up with the days");
  assert.equal(crew.crewTag({ day: 10 }, { jailed: true, jailed_until: 14 }), "jailed 4d", "jailed with the days");
  assert.equal(crew.crewTag(v, { jailed: true, bailed: true }), "out tomorrow", "bailed");
  assert.deepEqual(crew.post(v, { ...m, wounded: 2 }), { text: "laid up 2d", warn: true }, "a laid-up member's post");
  assert.match(crew.countLine(v, s.call("rules.crew.max_crew")), /^\d+ of \d+ on the payroll$/, "N of M");
  for (const p of crew.PAY) assert.ok(s.call("rules.crew.wages", p) >= 0 && crew.payBlurb(p), `the ${p} notch`);
  assert.ok(crew.summary(v, 0, engineInfo.sloppySkill)[2][1].endsWith("worked"), "the summary");
  // The crew's last words (#576): capacity, the over-cap words, the
  // captain picker, a lieutenant's keeps, the pool's turn and kin.
  const sum = crew.summary(v, 0, engineInfo.sloppySkill),
    here = v.cities.find((c) => c.id === v.you.city);
  assert.equal(sum[0][1], `${v.you.capacity[v.you.city]} in ${here.name}`, "the capacity row");
  assert.match(sum[1][1], new RegExp(`^${v.you.carry_limit} yours \\+ \\d+ crew$`), "yours and the crew's");
  assert.ok(v.you.capacity[v.you.city] >= v.you.carry_limit && v.you.carry_limit > 0, "the carry is in what the city holds");
  const most = s.call("rules.crew.max_crew"),
    ltT = s.call("rules.crew.lieutenancy"),
    boss = { id: 77, name: "Yaya", role: "lieutenant", city: here.id },
    full = { ...v, crew: [...Array(most).fill(0).map((_, i) => ({ id: 100 + i, name: "N" + i, role: "runner" })), boss] };
  assert.equal(crew.unassignCapLine(full, boss, most + ltT.Crew, ltT.Crew), `Off the city, the roster is ${most + 1} of ${most}: nobody is let go; you hire under ${most}.`, "the over-cap warning");
  assert.equal(crew.unassignCapLine(full, { ...boss, city: "" }, most + ltT.Crew, ltT.Crew), "", "no city, no warning");
  assert.equal(crew.unassignCapLine(v, boss, most + ltT.Crew, ltT.Crew), "", "under the cap, no warning");
  assert.match(crew.overCapWords(full, most), /^ The roster is \d+ of \d+: nobody is let go, and nobody is hired until it is under \d+\.$/, "the over-cap words");
  const cp = s.call("rules.crew.captaincy");
  assert.ok(cp.Budgets.length > 1, "fixed budgets");
  assert.ok(cp.Budgets.every((b) => crew.budgetWord(b)), "each budget in words");
  assert.match(crew.captainRefusal(boss, cp, true), /a lieutenant runs a city/, "no captain from a lieutenant");
  assert.match(crew.captainRefusal({ ...m, captain: "" }, cp, false), new RegExp(`takes loyalty ${Math.round(cp.Loyalty)} and ${cp.Days} days`), "why not yet");
  const capd = { ...v, crew: [{ ...m, captain: here.id, budget: cp.Budgets[0] }, { id: 5, name: "Kit", role: "runner" }] };
  const rows = crew.captainRows(capd, capd.crew[1]);
  assert.equal(rows.length, v.cities.length, "a row a city");
  assert.equal(rows.find((r) => r.id === here.id).who, m.name, "who captains it");
  assert.equal(crew.captainRows(capd, capd.crew[0]).find((r) => r.id === here.id).who, "theirs now", "theirs now");
  assert.equal(crew.captainAt(capd, capd.crew[0]), here.id, "the picker opens on theirs");
  const prods = here.products;
  const kept = { ...v, supply: [{ city: here.id, product: prods[0].id, units: 40, lieutenant: 77 }, { city: here.id, product: prods[1].id, units: 9, lieutenant: 77 }, { city: here.id, product: prods[1].id, units: 5 }] };
  assert.equal(crew.keeps(kept, here.id), `40 ${prods[0].name}`, "their keeps, yours winning");
  assert.equal(crew.keeps(kept, ""), "", "no city, no keeps");
  const pd = s.call("rules.crew.pool_days");
  assert.equal(crew.poolNext({ day: v.you.pool_day, you: v.you }, pd), pd, "a full turn to go");
  assert.equal(crew.poolNext({ day: v.you.pool_day + pd + 3, you: v.you }, pd), 1, "never under a day");
  const kin = { ...v, crew: [{ id: 3, name: "Ana", kin: [4] }], pool: [{ id: 4, name: "Bo", kin: [3] }] };
  assert.deepEqual(crew.kinNames(kin, kin.crew[0]), ["Bo (partner)"], "kin by name and word");
  assert.equal(crew.kinLine(kin, kin.pool[0], true), "Kin: Ana (partner) · came with the kin: fee at the discount", "a face with kin");
  assert.equal(crew.relation(1, 2), "cousin", "the TUI's word");
  // The acts the confirms ask for reach the engine: each at its quoted
  // price, or refused for the cash.
  const dirty = () => s.refresh().you.dirty_cash;
  if (cost <= dirty()) {
    s.call("investigate");
    assert.throws(() => s.call("investigate"), "asking twice is refused");
  } else assert.throws(() => s.call("investigate"), /need/, "an investigation past the cash is refused");
  if (s.call("rules.crew.payoff_cost", m.id) <= dirty()) assert.equal(s.call("pay_off", m.id).Name, m.name, "the pay-off answers with the member");
  else assert.throws(() => s.call("pay_off", m.id), /need/, "a pay-off past the cash is refused");
  s.call("fire", m.id);
  assert.ok(!s.refresh().crew.some((x) => x.id === m.id), "fired");
  assert.throws(() => s.call("bail", v.crew[1]?.id ?? 999), "bail of somebody not in a cell is refused");
}
{
  // The new-run list (#548): every character the query lists starts.
  const chars = session.call("characters");
  assert.equal(chars.length, 6, "six characters");
  assert.equal(chars.filter((c) => c.default).length, 1, "one default");
  const other = new Session(globalThis.kingpin);
  for (const c of chars) {
    assert.ok(c.name && c.blurb, `${c.id} has a name and a blurb`);
    assert.equal(other.newRun(41, c.id).day, 0, `${c.id} starts`);
  }
}
{
  // A report line's pointer (#560): the words name no screen; a point
  // lands as an alert of its kind, linked by the tab's name, and one the
  // page has no tab for (the intel screen) reads whole.
  const { landing } = await import(pathToFileURL(path.join(dist, "landing.js")));
  const report = await import(pathToFileURL(path.join(dist, "report.js")));
  const act = (screen, mode = "") => ({ screen, mode });
  // [kind, act, tab, open, select, link]
  const POINTS = [
    ["reign", act("dashboard", "walk_away"), "ledger", "", "exit-crown", "Ledger →"],
    ["straight", act("dashboard", "walk_away"), "ledger", "", "exit-go_straight", "Ledger →"],
    ["idle_crew", act("map"), "street", "", "", "The streets →"],
    ["front", act("ledger"), "empire", "", "", "The empire →"],
    ["asset", act("ledger"), "empire", "properties", "", "The empire →"],
    ["role", act("crew"), "crew", "", "", "Your people →"],
    ["contract_offer", act("market"), "market", "", "", "Market →"],
    ["contract", act("market", "deliver"), "market", "", "", "Market →"],
    ["informant", act("crew", "fire"), "crew", "", "", "Your people →"],
    ["seizure", act("map", "dial"), "market", "routes", "", "Market →"],
    ["books", act("rivals"), "rivals", "", "", "Rivals →"],
    ["offer", act("rivals"), "rivals", "", "", "Rivals →"],
    ["scouts", act("rivals"), "rivals", "", "", "Rivals →"],
  ];
  for (const [kind, a, tab, open, select, link] of POINTS) {
    const al = report.pointAlert({ line: 0, at: 1, kind, act: a });
    const l = landing(al);
    assert.deepEqual([l.tab, l.open, l.select, report.linkWords(l.tab)], [tab, open, select, link], `the ${kind} point lands`);
  }
  for (const kind of ["spy", "lure"])
    assert.equal(report.pointAlert({ line: 0, at: 1, kind, act: act("intel") }), null, `the ${kind} point has no tab`);
  const sec = { lines: ["a", "b"], points: [{ line: 1, at: 1, kind: "role", act: act("crew") }] };
  assert.equal(report.pointOf(sec, 0), null);
  assert.equal(report.pointOf(sec, 1).kind, "role");
  assert.equal(report.pointOf({ lines: ["a"] }, 0), null, "a report from before view 22 has no points");
  // The run's own report: every section carries its points, each on a
  // line it has, and no line names a TUI screen or key.
  for (const s of session.refresh().report.sections) {
    assert.ok(Array.isArray(s.points), `${s.id} carries points`);
    for (const p of s.points) assert.ok(p.line < s.lines.length && p.at <= s.lines[p.line].length && p.act.screen, `${s.id} point ${JSON.stringify(p)}`);
    for (const t of s.lines) assert.doesNotMatch(t, /screen \(\d\)|\(\w+, \w\)|\(\w\)|on the dashboard/, `${s.id} names a screen`);
  }
}
{
  // The alerts (#549): every kind lands on the tab that shows what it
  // names, with that thing opened or picked out; a danger is drawn as
  // one and a plan is not; the preview words what it cannot know.
  const { landing, alertClass, unknownLine } = await import(pathToFileURL(path.join(dist, "landing.js")));
  const { WORDS } = await import(pathToFileURL(path.join(dist, "alerts.js")));
  const act = (screen, subject = "", mode = "") => ({ screen, subject, mode });
  // [alert, tab, open, id, select]: the table the page is held to.
  const LANDS = [
    [{ kind: "arrest", act: act("dashboard") }, "street", "", "", "risk"],
    [{ kind: "broke", act: act("market") }, "market", "", "", ""],
    [{ kind: "talking", act: act("crew") }, "crew", "investigate", "", ""],
    [{ kind: "pages", level: "informant", act: act("crew") }, "crew", "investigate", "", ""],
    [{ kind: "pages", level: "tip", act: act("crew") }, "crew", "", "", "investigate"],
    [{ kind: "task_force", act: act("dashboard") }, "street", "", "", "risk"],
    [{ kind: "file", act: act("dashboard") }, "street", "", "", "risk"],
    [{ kind: "investigation", target: "corner", corner: "k1", act: act("map", "corner") }, "street", "corner", "k1", ""],
    [{ kind: "investigation", target: "product", product: "weed", act: act("market") }, "market", "", "", "product-weed"],
    [{ kind: "investigation", target: "house", house: "h1", act: act("ledger", "house") }, "empire", "properties", "h1", ""],
    [{ kind: "war_muscle", act: act("crew") }, "crew", "", "", ""],
    [{ kind: "no_corner", corner: "k1", act: act("map", "corner", "post") }, "street", "corner", "k1", ""],
    [{ kind: "no_corner", act: act("map") }, "street", "", "", ""],
    [{ kind: "contract_due", contract: 4, act: act("market", "contract") }, "market", "", "", "contract-4"],
    [{ kind: "debt_due", supplier: "cass", act: act("market", "supplier") }, "market", "", "", "connect-cass"],
    [{ kind: "heat", act: act("dashboard") }, "street", "", "", "risk"],
    [{ kind: "front_shut", front: "laundromat", act: act("ledger") }, "empire", "", "", "front-laundromat"],
    [{ kind: "float", act: act("ledger") }, "empire", "", "", "till"],
    [{ kind: "till", act: act("ledger") }, "empire", "", "", "till"],
    [{ kind: "wages", act: act("crew") }, "crew", "", "", ""],
    [{ kind: "crew_line", member: 3, act: act("crew", "member") }, "crew", "member", 3, ""],
    [{ kind: "skim", act: act("crew") }, "crew", "", "", ""],
    [{ kind: "unposted", member: 3, corner: "k1", act: act("map", "member", "post") }, "street", "corner", "k1", ""],
    [{ kind: "unposted", member: 3, act: act("crew", "member") }, "crew", "member", 3, ""],
    [{ kind: "idle_corner", corner: "k1", act: act("map", "corner", "post") }, "street", "corner", "k1", ""],
    [{ kind: "stash_full", city: "eastside", act: act("ledger", "city") }, "empire", "properties", "", ""],
    [{ kind: "landed", product: "weed", city: "eastside", act: act("market", "city") }, "market", "", "", "product-weed"],
    [{ kind: "scouts", faction: "f1", act: act("rivals") }, "rivals", "", "", "faction-f1"],
    [{ kind: "gate", gate: { kind: "product" }, act: act("market") }, "market", "", "", ""],
    [{ kind: "gate", gate: { kind: "connect" }, act: act("market") }, "market", "", "", ""],
    [{ kind: "gate", gate: { kind: "front" }, act: act("ledger") }, "empire", "", "", ""],
    [{ kind: "gate", gate: { kind: "asset" }, act: act("ledger") }, "empire", "properties", "", ""],
    [{ kind: "port", city: "bayport", act: act("map", "city") }, "street", "", "", ""],
    [{ kind: "exports", act: act("ledger") }, "empire", "", "", ""],
    [{ kind: "house_known", house: "h1", act: act("ledger", "house") }, "empire", "properties", "h1", ""],
    [{ kind: "da_race", city: "eastside", act: act("ledger", "city") }, "ledger", "", "", "race"],
    [{ kind: "retire", act: act("ledger") }, "ledger", "", "", "exit-retire"],
    [{ kind: "retire", ready: true, act: act("dashboard") }, "ledger", "", "", "exit-retire"],
    [{ kind: "favour", act: act("ledger") }, "ledger", "favour", "", "law"],
    [{ kind: "reign", act: act("dashboard") }, "ledger", "", "", "exit-crown"],
    [{ kind: "straight", act: act("dashboard") }, "ledger", "", "", "exit-go_straight"],
    [{ kind: "vanish", act: act("dashboard") }, "ledger", "", "", "exit-vanish"],
    [{ kind: "exposure", act: act("ledger") }, "empire", "", "", ""],
    [{ kind: "plan", act: act("dashboard") }, "ledger", "", "", "plan-line"],
  ];
  for (const kind of Object.keys(WORDS))
    assert.ok(LANDS.some(([a]) => a.kind === kind), `${kind} has a row in the landing table`);
  for (const [a, tab, open, id, select] of LANDS) {
    const l = landing(a);
    assert.deepEqual([l.tab, l.open, l.id, l.select], [tab, open, id, select], `${a.kind} ${JSON.stringify(a)} lands`);
  }
  assert.equal(landing({ kind: "till", act: act("ledger") }).focus, "till-amount", "the till alert lands on the till's field");
  assert.match(alertClass({ kind: "arrest", danger: true }), /\bdanger\b/);
  assert.match(alertClass({ kind: "broke", danger: true }), /\bdanger\b/);
  assert.doesNotMatch(alertClass({ kind: "plan", danger: false, notice: false }), /danger/);
  assert.match(alertClass({ kind: "till", danger: false, notice: true }), /\bnotice\b/);
  // The engine's own: every alert the run above raised is classed off
  // its danger and notice, which it always carries.
  for (const a of session.refresh().alerts) {
    assert.equal(typeof a.danger, "boolean", `${a.kind} carries danger`);
    assert.equal(/\bdanger\b/.test(alertClass(a)), a.danger, `${a.kind} is drawn by its danger`);
  }
  const p = session.preview();
  assert.ok(p && Array.isArray(p.unknown) && p.unknown.length, "the preview says what it cannot know");
  assert.match(unknownLine(p.unknown), /^An estimate before the dice: robberies, .* are not in it\.$/, "every unknown worded");
  assert.ok(p.wash && typeof p.wash.washed === "number", "the preview carries the wash");
}
{
  // The till, the wash and the road (#553): wash.js's words on the live
  // run, the till set and read back through set_till, and the reserve's
  // default kept off tonight's upkeep.
  const wash = await import(pathToFileURL(path.join(dist, "wash.js")));
  const q = (m, ...p) => session.call(m, ...p);
  const clean = (s, what) => assert.doesNotMatch(String(s), /undefined|NaN|\[object/, what);
  let v = session.refresh();
  const t = wash.till(q);
  assert.equal(t.till, t.float, "a till never set is the float");
  assert.ok(t.float > 0 && t.max >= t.float && t.line >= t.till, "the till's range and line");
  assert.equal(typeof t.outlay, "number", "rules.laundering.outlay");
  for (const s of [wash.tillWords(q), wash.tonight(v, q), wash.tillRules(q), wash.noFrontsOnOffer(q)]) clean(s, "a wash line");
  assert.equal(wash.setTill(q, "").send, 0, "blank is the float");
  assert.equal(wash.setTill(q, "1").send, 0, "under the float is the float");
  const over = wash.setTill(q, String(t.max + 1));
  assert.ok(over.refuse && over.field === t.max && over.send === undefined, "over the rot line is set to the top, not sent");
  const raise = wash.setTill(q, String(t.float + 25000));
  assert.equal(raise.send, t.float + 25000);
  session.call("set_till", raise.send);
  assert.equal(session.refresh().you.till, t.float + 25000, "set_till stores the till");
  assert.ok(wash.till(q).set && /the line you set/.test(wash.tillWords(q)), "a till set says so");
  session.call("set_till", 0);
  assert.equal(wash.till(q).till, t.float, "0 is the float again");
  v = session.refresh();
  for (const l of wash.washLines(v, q, session.preview())) clean(l.text + l.more, `the ${l.label} line`);
  // The reserve at its default never leaves the clean pile under
  // tonight's upkeep (#458).
  const due = wash.upkeepTonight(q),
    blank = wash.reserveBlank(v, q);
  assert.ok(blank >= 0 && blank <= v.you.clean_cash, "the reserve's default is in the pile");
  assert.ok(v.you.clean_cash - blank >= Math.min(due, v.you.clean_cash), "the reserve's default keeps the upkeep back");
  assert.ok(v.you.clean_cash - wash.cashOutBlank(v, q) >= Math.min(due, v.you.clean_cash), "the cash-out's default keeps the upkeep back");
  assert.equal(wash.upkeepWarning(q, 0, 0) === null, due <= 0, "an empty clean pile is warned whenever upkeep is due");
  // A front's status and why, on made-up fronts at today's day.
  const offer = session.call("rules.laundering.offers")[0];
  const at = (f) => wash.frontStatus(v, q, { id: offer.ID, bought: 0, ...f }).text;
  assert.match(at({ frozen_until: v.day + 3, unpaid: 150 }), /^Shut 3 days: upkeep unpaid \(\$[\d,]+ clean a night\)$/);
  assert.match(at({ frozen_until: v.day + 3 }), /^Shut: back in 3 days$/);
  assert.equal(at({ bought: v.day }), "Opens tomorrow");
  clean(wash.frontTerms(q, offer), "a front's terms");
  assert.match(wash.frontTerms(q, offer), /opens tomorrow .* a night unpaid shuts it \d+ days?\.$/i);
  for (const o of session.call("front_offers")) {
    clean(wash.offerLock(v, o) + wash.frontShutWarning(v, q, o, session.preview()), `${o.ID}'s lock and shut`);
    if (v.you.peak_cash < o.UnlockCash && !o.Asset) assert.match(wash.offerLock(v, o), /to go$/, `${o.ID} says how far to go`);
  }
  for (const r of v.routes) {
    const idle = wash.routeIdle(v, q, r);
    if (idle) clean(idle.text, `${r.id}'s idle reason`);
  }
  for (const a of session.call("asset_offers")) clean(wash.assetTerms(a), `${a.ID}'s terms`);
}
{
  // One way to write a number (#589): every module draws money, price,
  // cash, pct and Go's ties to even from format.js, so a negative reads
  // -$N (format.Money), never $-N, wherever the page writes it.
  const f = await import(pathToFileURL(path.join(dist, "format.js")));
  const M = { 0: "$0", 999: "$999", 30040: "$30,040", [-30040]: "-$30,040", [-0.4]: "$0", [-0.6]: "-$1", [-1234567]: "-$1,234,567" };
  for (const [n, want] of Object.entries(M)) assert.equal(f.money(Number(n)), want, `format.Money(${n})`);
  assert.equal(f.money(undefined), "$0", "no amount is $0");
  assert.deepEqual([12.125, 999.5, 1234.4].map(f.price), ["$12.12", "$999.50", "$1,234"], "format.Price");
  assert.deepEqual([[0.125, 0], [0.045, 1], [undefined, 0]].map(([x, d]) => f.pct(x, d)), ["12%", "4.5%", "0%"], "format.Pct");
  assert.deepEqual([2.25, -2.25, 12.5, -12.5].map(f.pctText), ["2.2%", "-2.2%", "12%", "-12%"], "ui.pctText");
  const wash = await import(pathToFileURL(path.join(dist, "wash.js")));
  assert.equal(wash.cash(-12500000), "-$12M", "wash.js still serves format.js's cash");
  const src = path.join(path.dirname(fileURLToPath(import.meta.url)), "src");
  for (const name of fs.readdirSync(src).filter((n) => n.endsWith(".js") && n !== "format.js")) {
    const text = fs.readFileSync(path.join(src, name), "utf8");
    assert.doesNotMatch(text, /^\s*(const |let |  )money = /m, `${name} writes money with format.js's`);
    assert.doesNotMatch(text, /\.toFixed\(\d\)[^.]/, `${name} rounds with format.js's fixed, not toFixed`);
  }
}
{
  // The wash's audit odds, throughput, pile rot and tax (#577): the
  // numbers are the engine's queries, written as the TUI's ledger writes
  // them (format.Cash, format.CashWeight and Go's %.*f, ties to even).
  const wash = await import(pathToFileURL(path.join(dist, "wash.js")));
  const q = (m, ...p) => session.call(m, ...p);
  const C = { 0: "$0", 500: "$500", 9999: "$9,999", [-2500]: "-$2,500", 10000: "$10K", 999499: "$999K", 999600: "$1.0M", 1234567: "$1.2M", 12345678: "$12M", 12500000: "$12M", 13500000: "$14M", 3400000000: "$3.4B", [-1500000]: "-$1.5M" };
  for (const [n, want] of Object.entries(C)) assert.equal(wash.cash(Number(n)), want, `format.Cash(${n})`);
  for (const [n, want] of [[50000, "under a kilo"], [100000, "1 kg"], [45000000, "450 kg"], [99990000, "1.0 tonnes"], [150000000, "1.5 tonnes"], [1200000000, "12 tonnes"]])
    assert.equal(wash.cashWeight(n), want, `format.CashWeight(${n})`);
  assert.deepEqual([[0.25, 1], [0.35, 1], [2.5, 0], [3.5, 0], [-2.5, 0], [1.15, 1]].map(([x, d]) => wash.fixed(x, d)), ["0.2", "0.3", "2", "4", "-2", "1.1"], "Go's %.*f");
  const v = session.refresh(),
    any = q("rules.laundering.any_audit_risk");
  const signed = (n) => (n < 0 ? "-$" : "$") + Math.abs(n).toLocaleString("en-US"); // a front's upkeep makes legit income negative
  assert.equal(wash.odds(q), `audit ${wash.fixed(any * 100, 1)}%/day · up to ${signed(q("rules.laundering.capacity"))}/day · legit ${signed(q("rules.laundering.legit_income"))}/day`);
  // The pile: nothing under $10M; over it its weight, and the rot over
  // the rot line.
  assert.equal(wash.pileLine({ ...v, you: { ...v.you, dirty_cash: 9_999_999 } }, q), null, "no pile line under $10M");
  const big = 120_000_000,
    rot = q("rules.laundering.rot", big),
    line = q("rules.laundering.rot_line");
  assert.ok(rot > 0 && line > 0 && line < big, "a $120M pile rots");
  assert.equal(wash.pileLine({ ...v, you: { ...v.you, dirty_cash: big } }, q).text, `the pile weighs 1.2 tonnes in hundreds · rats and damp take ~$${rot.toLocaleString("en-US")} a night over ${wash.cash(line)}`);
  assert.ok(wash.washLines({ ...v, you: { ...v.you, dirty_cash: big } }, q, null)[0].label === "pile", "the pile leads the wash lines");
  // The tax: a city held pays off its free corners; one not held says
  // nothing.
  const c0 = v.cities[0],
    held = (m, ...p) => (m === "rules.territory.tax_due" ? (p[0] === c0.id ? { corners: 3, amount: 1234 } : { corners: 0, amount: 0 }) : q(m, ...p));
  assert.deepEqual(
    wash.taxLines({ ...v, stats: { ...v.stats, taxed: 45000 } }, held).map((l) => l.text + l.more),
    [`3 free corners in ${c0.name} pay ~$1,234/night · $45K so far`],
  );
  for (const c of v.cities) assert.equal(typeof q("rules.territory.tax_due", c.id).corners, "number", `${c.id}'s tax`);
  assert.equal(wash.taxLines(v, q).length, v.cities.filter((c) => q("rules.territory.tax_due", c.id).corners > 0).length);
  // A front's throughput and audit at the dial; an accountant's share
  // is what the throughput is over the base at the dial.
  const o = q("rules.laundering.offers")[0],
    f = { id: o.ID, bought: 0, washed: 12345, washed_today: 350 },
    tp = q("rules.laundering.throughput", o.ID),
    mul = q("rules.laundering.dial", v.you.launder).Mul;
  const rows = wash.frontRows({ ...v, crew: [] }, q, f, o);
  assert.deepEqual(rows[0], ["washes", `$${tp.toLocaleString("en-US")}/day`]);
  assert.deepEqual(rows[1], ["today", "$350 · lifetime $12,345"], "a front's washed today and lifetime, as the TUI's front pane rows them");
  assert.match(rows[2][1], new RegExp(`^\\d+(\\.\\d)?%/day at ${v.you.launder}$`));
  const withAcct = wash.frontRows({ ...v, crew: [{ role: "accountant" }] }, q, f, o)[0][1];
  assert.equal(withAcct, `$${tp.toLocaleString("en-US")}/day (+$${(tp - Math.round(o.Throughput * mul)).toLocaleString("en-US")} accountants)`);
  assert.doesNotMatch(wash.frontRows({ ...v, crew: [{ role: "accountant", jailed: true }] }, q, f, o)[0][1], /accountants/, "a jailed accountant is not at work");
  assert.match(wash.offerAudit(o), /^\d+(\.\d)?%$/);
}
{
  // The cut and the cook (#557): a Cook run cooks a batch and cuts the
  // lot in the web's words, lab.js's numbers on view 17's quality and
  // room, and each act reaches the engine.
  const lab = await import(pathToFileURL(path.join(dist, "lab.js")));
  const s = new Session(globalThis.kingpin);
  const q = (m, ...p) => s.call(m, ...p);
  const clean = (x, what) => assert.doesNotMatch(String(x), /undefined|NaN|\[object/, what);
  let v = s.newRun(41, "cook");
  // A day's weed trade, as the main run does: the start's cash alone
  // pays a precursor or two and then not the chemist's wage.
  const day = () => {
    if (s.view.card) s.choose(0);
    const sup = s.view.connects.find((c) => c.open && !c.wholesale && c.city === s.view.you.city);
    const n = sup ? Math.min(20, s.maxBuy(sup.id, "weed").max) : 0;
    if (n) {
      s.buy(sup.id, "weed", n);
      s.sell(s.view.you.city, "weed", n, "normal");
    }
    s.endDay();
    return s.refresh();
  };
  for (let i = 0; i < 40 && v.you.dirty_cash < 1500 && !v.over; i++) v = day();
  assert.ok(!v.over && v.you.dirty_cash >= 1500, "the Cook earns a cook's cash");
  const here = v.you.city,
    chem = lab.chemist(v);
  assert.ok(chem, "the Cook starts with a chemist at the lab");
  assert.equal(lab.refuseCook(v, q), null, "the Cook can cook");
  assert.equal(typeof v.you.room[here], "number", "view 17 carries the room");
  const ids = lab.cookProducts(v, q, here);
  assert.ok(ids.length, "a product the chemist cooks is on the ladder");
  const id = ids[0];
  for (const r of lab.cookRows(v, q, here)) clean(Object.values(r).join(" "), "a cook row");
  for (const l of [lab.chemistLine(v, q, here), lab.cookNote(v, q, here, id), lab.cookCostLine(v, q, here, id, 5), ...lab.hand(v, q, chem, false).flat()]) clean(l, "a cook line");
  const top = lab.cookMax(v, q, here, id);
  assert.ok(top > 0 && top <= q("rules.crew.batch_in", here), "a batch to cook");
  // #569: the blank keeps tonight's wages back (rules.crew.cook_max).
  const spare = q("rules.crew.spare");
  assert.equal(spare, Math.max(0, v.you.dirty_cash - q("rules.crew.wages", v.you.pay)), "spare is the till less tonight's wages");
  assert.ok(top * lab.cookCost(q, here, id) <= spare, "the cook max leaves the wages in hand");
  assert.equal(lab.readQty("", top).n, top, "blank is a batch");
  assert.ok(lab.readQty("x", top).error && lab.readQty("0", top).error, "not a whole number");
  const most = Math.min(top, 3);
  const k = s.call("cook", here, id, most);
  v = s.refresh();
  assert.equal(k.Units, most, "the cook reaches the engine");
  assert.match(lab.cookDone(v, k), new RegExp(`is cooking ${most} .* ready in ${q("rules.crew.cook_days")} days?\\. Cost \\$`), "the cook's words");
  assert.equal(lab.cooking(v, here, id), most, "the lot is on its way");
  assert.equal(lab.onTheWay(v).length, 1, "the lot on its way is listed");
  for (let i = 0; i < 10 && v.cooks.length && !v.over; i++) v = day();
  assert.ok(lab.stock(v, here, id) >= most, "the batch landed");
  assert.equal(Math.round(v.you.quality[here][id]), Math.round(k.Quality), "the lot is at the cook's quality");
  assert.equal(lab.refuseCut(v, q), null, "the lot can be cut");
  assert.ok(lab.cutProducts(v, q, here).includes(id), "the cooked product can be cut");
  for (const r of lab.cutRows(v, q, here)) clean(Object.values(r).join(" "), "a cut row");
  clean(lab.cutNote(v, q, here, id), "the cut's terms");
  assert.match(lab.cutNote(v, q, here, id), new RegExp(`${chem.name}'s hand keeps`), "the chemist's hand in the cut");
  const pct = lab.cutMax(v, q, here, id);
  assert.ok(pct > 0, "room and cash to cut");
  const preview = lab.cutPreview(v, q, here, id, pct);
  clean(lab.cutAfterLine(v, q, here, id, pct), "the cut's after line");
  const rec = s.call("cut", here, id, pct / 100);
  v = s.refresh();
  assert.equal(rec.Units + rec.Added, preview.units, "the preview's units are the cut's");
  assert.ok(Math.abs(rec.To - preview.quality) < 1e-6, "the preview's quality is the cut's");
  assert.equal(rec.Cost, preview.cost, "the preview's cost is the cut's");
  assert.ok(Math.abs(v.you.quality[here][id] - rec.To) < 1e-6, "view 17 carries the cut quality");
  clean(lab.cutDone(v, q, rec), "the cut's words");
}
{
  // The law and its answers (#552): law.js's words on the live run and
  // on made-up views, and the acts its dialogs send.
  const law = await import(pathToFileURL(path.join(dist, "law.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const q = (m, ...p) => session.call(m, ...p);
  const clean = (s, what) => assert.doesNotMatch(String(s), /undefined|NaN|\[object/, what);
  const v = session.refresh();
  const lines = law.lawLines(v);
  assert.match(lines[0].text, new RegExp(`^Chief ${v.law.chief} · `), "the chief's line");
  assert.match(lines[1].text, new RegExp(`^DA ${v.law.da} · ${law.stanceWord(v.law.da_stance)}`), "the DA's line");
  for (const l of lines) clean(l.text, "a LAW line");
  // The race: the odds per ticket add up, a row a city, the costs.
  const odds = law.raceOdds(q);
  assert.equal(odds.length, 3, "three tickets' odds");
  assert.ok(Math.abs(odds.reduce((n, [, o]) => n + o, 0) - 1) < 1e-9, "the odds add up");
  const open = { ...v, law: { ...v.law, campaign_open: true, next_election: v.day + 12 } };
  assert.equal(law.raceRows(v, q).length, v.law.campaign_open ? v.cities.length : 0, "no race rows while the tickets take nothing");
  assert.equal(law.raceRows(open, q).length, v.cities.length, "a race row a city");
  assert.equal(law.raceNote(open), `the vote on day ${v.day + 12}, in 12 days`);
  const cmp = q("rules.law.campaign"),
    backed = { ...open, cities: open.cities.map((c, i) => (i ? c : { ...c, campaign: { ticket: "reform", cash: cmp.Cash * 3 } })) };
  assert.deepEqual([law.raceRows(backed, q)[0].ticket, law.raceRows(backed, q)[0].points], ["reform", "3.0 points"], "the backed row");
  for (const [, t] of law.raceLines(backed, q, backed.cities[0].id)) clean(t, "a race line");
  clean(law.raceCosts(q), "the race's costs");
  // The fund: blank keeps tonight's upkeep back, the preview says what
  // it buys, nothing given is refused, a backing on the other ticket is
  // warned.
  const c = v.cities[0],
    rich = { ...v, you: { ...v.you, clean_cash: 50_000 } };
  assert.ok(law.fundBlank(rich, q, c) <= rich.you.clean_cash - Math.min(rich.you.clean_cash, (await import(pathToFileURL(path.join(dist, "wash.js")))).upkeepTonight(q)), "blank keeps the upkeep back");
  assert.ok(law.maxFund(rich, q, c) <= Math.trunc((100 - c.goodwill) * q("rules.law.tuning").GoodwillCash), "no more than goodwill 100");
  for (const [, t] of law.fundLines(rich, q, c, "5000")) clean(t, "a fund line");
  assert.ok(law.fundLines(rich, q, c, "5000").some(([k, t]) => k === "buys" && t.startsWith(`+${Math.round(q("rules.law.goodwill", 5000))} goodwill for $5,000`)), "the goodwill preview");
  assert.deepEqual(law.fundPlan(rich, q, c, "5000", ""), { fund: 5000, back: 0 }, "goodwill alone");
  assert.equal(law.fundPlan({ ...v, you: { ...v.you, clean_cash: 0 } }, q, c, "", "").err, "Nothing to give.", "nothing to give");
  assert.deepEqual(law.fundPlan({ ...open, you: rich.you }, q, c, "0", "20000"), { fund: 0, back: 20000 }, "the campaign alone");
  assert.ok(law.campaignLines(backed, q, backed.cities[0], 0, "law_and_order", "1000").some(([, t, k]) => k === "danger" && /buys nothing/.test(t)), "both tickets warned");
  assert.equal(law.maxBack(backed, q, backed.cities[0], 0), Math.min(backed.you.clean_cash, law.fill(cmp) - cmp.Cash * 3), "the most the campaign takes");
  // The favour: why not, and the confirm's words.
  assert.match(law.favourRefusal({ ...v, law: { ...v.law, da_stance: "reform", favours: 0 } }, "raid"), /owes you nothing/);
  assert.match(law.favourRefusal({ ...v, law: { ...v.law, da_stance: "reform", favours: 1 } }, ""), /nothing is coming tonight/);
  assert.match(law.favourRefusal({ ...v, law: { ...v.law, da_stance: "law_and_order", favours: 1 } }, "raid"), /law-and-order DA/);
  assert.equal(law.favourRefusal({ ...v, law: { ...v.law, da_stance: "reform", favours: 1 } }, "raid"), "", "a favour that can be called");
  const raid = q("rules.heat.rungs", q("rules.heat.hottest")).find((r) => r.Level === "raid");
  if (raid) assert.match(law.favourSaves(v, q, "raid"), new RegExp(`^What it saves: the raid would take ${Math.round(raid.StockLoss * 100)}% of the stock`), "what the favour saves");
  for (const [t] of law.favourLines({ ...v, law: { ...v.law, favours: 1 } }, q)) clean(t, "a favour line");
  assert.throws(() => q("call_favour"), "a favour with nothing owed is refused");
  // The tip: the attention, the line, the page's odds and the peace.
  const corner = v.cities.flatMap((x) => x.corners).find((k) => k.owner === "rival") || { id: "k", name: "The Docks", faction: v.factions[0].id };
  const f = v.factions.find((x) => x.id === (corner.faction || "rival")) || v.factions[0],
    tipped = (police, deals = []) => ({ ...v, factions: v.factions.map((x) => (x.id === f.id ? { ...x, police, deals } : x)) }),
    tp = q("rules.rivals.tip_tuning");
  const tip = law.tipLines(tipped(0), q, { ...corner, faction: f.id });
  assert.match(tip[1][0], new RegExp(`goes 0 → ${Math.round(tp.Heat)}; at ${Math.round(tp.PoliceNotice)} they raid`), "the attention");
  assert.ok(tip.some(([t]) => t.includes(`~${Math.round(q("rules.heat.tip_evidence") * 100)}% the DA's file`)), "the page's odds before it is sent");
  assert.ok(law.tipLines(tipped(q("rules.rivals.factions").LeaderArrestHeat), q, { ...corner, faction: f.id }).some(([t]) => /the police take/.test(t)), "the end of them");
  assert.ok(law.tipLines(tipped(0, ["truce"]), q, { ...corner, faction: f.id }).some(([t]) => /breaks the peace/.test(t)), "the peace");
  for (const [t] of tip) clean(t, "a tip line");
  clean(law.tipSaid(v, q, corner, f), "the tip's word");
  // Lying low: who sells nothing, and a queued handoff named.
  assert.equal(law.lieLowWords({ ...v, crew: [{ role: "lieutenant", city: "eastside" }] }), "no sales, your lieutenants' included");
  assert.equal(law.lieLowWords({ ...v, crew: [] }), "no sales");
  const contract = { id: 9, name: "Mr Lime", product: "weed", units: 40, delivered: 10, due: v.day + 3 };
  assert.match(law.lieLowHandoffs({ ...v, contracts: [contract] }, [{ contract: 9, units: 12 }])[0], /^12 Weed to Mr Lime, 30 owed by day \d+\.$/i, "the handoff held");
  assert.deepEqual(law.lieLowHandoffs(v, []), [], "nothing queued, no confirm");
  // The bought law: each official's odds, a route's deal, the payoffs
  // and the cop.
  for (const stance of ["law_and_order", "reform", "moderate"]) {
    const sv = { ...v, law: { ...v.law, da_stance: stance } };
    for (const t of law.TARGETS) {
      clean(law.officialWord(sv, q, t) + law.bribeOdds(sv, q, t, law.bribePrice(q, t)).text, `${t} under a ${stance} DA`);
    }
  }
  assert.match(law.bribeOdds(v, q, "chief", 1).text, /^Under the price/, "an envelope under the price");
  for (const [t] of law.bribeTerms(v, q)) clean(t, "the bribe's terms");
  for (const r of v.routes) {
    assert.ok(law.dealPrice(q, r.id) > 0, `${r.id}'s deal has a price`);
    for (const [t] of law.checkpointLines({ ...v, routes: v.routes }, q, { ...r, risk_known: true, risk: 0.05 })) clean(t, `${r.id}'s deal`);
  }
  const bought = { ...v, law: { ...v.law, chief_bought: v.day + 5, da_bought: v.day + 9 }, routes: v.routes.map((r, i) => (i ? r : { ...r, checkpoint_until: v.day + 30 })) };
  assert.deepEqual(law.payoffRows(bought, q).map((p) => p.until), [v.day + 5, v.day + 9, v.day + 30], "the live deals in order");
  assert.deepEqual(law.payoffRows(v, q).filter((p) => !p.route), [], "nobody bought");
  assert.ok(engineInfo.copPrice > 0 && engineInfo.copAccuracy > 0, "the cop's terms from the TOML");
  assert.equal(law.copAccuracy(engineInfo, engineInfo.copPrice / 2), engineInfo.copAccuracy / 2, "less money, less often");
  for (const [t] of law.copLines(v, engineInfo, "")) clean(t, "a cop line");
  // The acts the dialogs send reach the engine, or are refused in its
  // words.
  if (!v.law.campaign_open) assert.throws(() => q("back", c.id, "reform", 1000), "no backing while the tickets take nothing");
  if (!v.you.clean_cash) assert.throws(() => q("fund", c.id, 1000), "no fund with no clean cash");
  if (law.cold(v)) assert.throws(() => q("bribe", "chief", 60000), /law-and-order/, "the chief takes nothing under a law-and-order DA");
  if (v.you.dirty_cash >= 1000) {
    q("pay_cop", 1000);
    assert.throws(() => q("pay_cop", 1000), "one cop a day");
  }
}
{
  // The endings, the crown and the rivals' table (#554): endings.js and
  // rivals.js on the live run and on made-up views, and the acts the
  // table sends.
  const endings = await import(pathToFileURL(path.join(dist, "endings.js")));
  const table = await import(pathToFileURL(path.join(dist, "rivals.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const q = (m, ...p) => session.call(m, ...p);
  const clean = (s, what) => assert.doesNotMatch(String(s), /undefined|NaN|\[object/, what);
  // An enforcer on the payroll, so the war can be declared.
  const hand = session.refresh().pool.find((m) => m.role === "enforcer");
  if (hand && !session.view.crew.some((m) => m.role === "enforcer")) q("hire", hand.id);
  const v = session.refresh();
  assert.ok(engineInfo.streetWindow >= 0 && engineInfo.kingpinShare > 0 && engineInfo.warForce, "the walk away's and the war's terms from the TOML");
  // Every way out says what it scores, the same for all four (#478).
  const rows = endings.exitRows(v, q, engineInfo);
  assert.deepEqual(rows.map((r) => r.action), ["retire", "vanish", "crown", "go_straight"]);
  assert.equal(rows[0].open, !!q("rules.laundering.can_retire") && !v.you.pages_due && !v.you.pages_pending, "retire open as the rule says");
  for (const r of rows) {
    clean(r.terms, r.action + "'s terms");
    if (!r.open) assert.ok(r.short, r.action + " says what is short"), clean(r.short, r.action + "'s short");
  }
  assert.ok(endings.scoreRow(v).endsWith("$" + v.you.score.toLocaleString("en-US")), "the score row is the engine's");
  const bloody = { ...v, you: { ...v.you, offshore: 600_000, score: 200_000, bodies: 2 } };
  assert.equal(endings.scoreRow(bloody), "the account over 1 + 2 bodies: $200,000");
  assert.equal(endings.scoreLine(bloody), "Score $200,000: $600,000 over 1 + 2 bodies.");
  assert.equal(endings.scoreWords(bloody), "the offshore account $600,000 ÷ (1 + 2 bodies)");
  for (const a of ["retire", "crown", "go_straight", "vanish"]) {
    const lines = endings.exitConfirm(v, q, a);
    for (const [t] of lines) clean(t, a + "'s confirm");
    assert.ok(lines.some(([t, k]) => k === "gold" && t.startsWith(`Score $${v.you.score.toLocaleString("en-US")}:`)), a + "'s confirm scores");
  }
  // The pages close every way out (#494, #525), and the pending lines say why.
  const owed = { ...v, you: { ...v.you, pages_due: 2, reserved_today: 50_000, upgrades: [...v.you.upgrades, "identity"] } };
  assert.ok(endings.exitRows(owed, q, engineInfo).every((r) => !r.open), "the pages close every way out");
  assert.match(endings.exitRows(owed, q, engineInfo)[1].short, /^2 pages from last night's transfer go in the DA's file tonight/);
  assert.deepEqual(endings.pendingLines(owed).map(([, k]) => k), ["danger", "warn"]);
  assert.match(endings.pendingLines(owed)[1][0], /^\$50,000 lands offshore tonight/);
  assert.match(endings.streetWords(engineInfo), /^the street's average night over the (last \d+|run)$/);
  // The crown: each faction off the count with why, the last clock.
  clean(endings.crownShort(v, q), "the crown's short");
  clean(endings.reignIncome(v, q), "the reign's income");
  const down = endings.downRows(v, q);
  for (const f of v.factions) {
    const words = endings.downWords(v, q, f);
    assert.equal(!words, !!q("rules.rivals.down", f.id).Counts, f.id + " is off the count with a reason, or counts");
    clean(words, f.id + "'s down words");
  }
  assert.equal(down.length, v.factions.filter((f) => !q("rules.rivals.down", f.id).Counts).length);
  clean(endings.lastClockWords(v, q), "the last clock");
  assert.equal(endings.crownShort({ ...v, you: { ...v.you, reign: 3, reign_slip: 1 } }, q), "the city is slipping under the share: hold more corners");
  assert.equal(endings.reignDay({ ...v, you: { ...v.you, reign: v.day - 4 } }), 5);
  // The plans by unit, the PLAN line and what reset the quiet.
  assert.equal(endings.stepWords({ unit: "reign", have: 5, need: 14 }), "day 5 of the reign");
  assert.equal(endings.stepWords({ unit: "days", have: 3, need: 14 }), "3 of 14 days");
  assert.equal(endings.stepWords({ unit: "cash", have: 412000, need: 750000 }), "$412,000 of $750,000");
  assert.equal(endings.stepPart({ label: "quiet days", unit: "days", have: 14, need: 14 }), "quiet days 14/14");
  for (const a of v.ambitions) for (const st of a.steps) clean(endings.stepWords(st), `${a.id}'s ${st.id}`);
  const retire = v.ambitions.find((a) => a.id === "retire"),
    pinned = { ...v, ambitions: v.ambitions.map((a) => ({ ...a, pinned: a.id === "retire", done: false, steps: a.steps.map((st) => ({ ...st, done: false })) })) },
    ev = { Day: 41, Cause: "police", City: v.cities[0].id, Level: "sting" };
  assert.equal(endings.planLine(v, null), v.ambitions.some((a) => a.pinned) ? endings.planLine(v, null) : "", "no plan, no line");
  if (retire) {
    const line = endings.planLine(pinned, ev);
    assert.ok(line.startsWith(retire.name + " "), "the PLAN line names the plan");
    assert.ok(line.includes(`the quiet days reset by a sting in ${v.cities[0].name} on day 41`), "the quiet reset: " + line);
    clean(line, "the PLAN line");
  }
  assert.equal(endings.quietCause(v, { Day: 9, Cause: "police", Level: "taskforce" }), "a task force on day 9");
  // The summary: the facts, the fallen, the betrayals.
  const over = {
    ...v,
    over: { day: v.day, cause: "kingpin", title: "Kingpin", won: true, epilogue: "x", story: [], reached: 30 },
    you: { ...v.you, reign: 30, bodies: 2 },
    stats: { ...v.stats, fallen: 1, betrayed_by: 1, crew_poached: 2 },
    fallen: [{ name: "Ziggy", role: "runner", day: 3 }],
  };
  assert.equal(endings.reachedLine(over), "Kingpin on day 30");
  const facts = endings.summarySections(over).flatMap((sec) => sec.rows);
  assert.ok(facts.some(([k, t]) => k === "fallen" && t === "Ziggy (runner · day 3)"), "the fallen");
  assert.ok(facts.some(([k, t]) => k === "bodies" && t === "2, 1 of them yours"), "the bodies");
  assert.ok(facts.some(([k, t]) => k === "betrayals" && t === "2 members poached · 1 deal broken by them"), "the betrayals");
  for (const [, t] of facts) clean(t, "a summary fact");
  // The table: the standard asks with the dice's odds, the war's
  // confirm, declare and call off, a proposal and its withdrawal.
  for (const f of v.factions) {
    clean(table.factionLine(v, q, f), f.id + "'s line");
    clean(table.moodLine(v, q, f)[0], f.id + "'s mood");
    for (const d of table.dealRows(v, f)) clean(d.terms, f.id + "'s deal");
    if (!f.alive) continue;
    const dip = q("rules.rivals.diplomacy");
    for (const kind of ["truce", "tribute", "split"]) {
      const asks = table.termRows(v, q, f, kind);
      assert.equal(asks.length, kind === "split" ? f.split_lines.length : kind === "truce" ? dip.TruceDays.length : dip.TributeCuts.length, `${f.id}'s ${kind} asks`);
      for (const r of asks) assert.ok(r.odds >= 0 && r.odds <= 1, `${kind} odds`), clean(r.label + r.words, `${kind} ask`);
    }
    clean(table.tributeBasis(v, q, f), "the tribute basis");
    const truce = table.termRows(v, q, f, "truce")[1].deal;
    if (!table.liveDeal(v, f, "truce") && !table.distrusted(v, q, f)) {
      q("propose_to", f.id, truce.kind, truce.terms);
      const pv = session.refresh();
      assert.equal(pv.proposal?.faction, f.id, "the proposal is on the table");
      clean(table.proposalLine(pv, q), "tonight's proposal");
      q("withdraw");
      assert.ok(!session.refresh().proposal, "withdrawn");
    }
    const why = table.warRefusal(v, f);
    if (!why && !v.over) {
      const lines = table.warConfirm(v, q, engineInfo, f);
      assert.match(lines[0][0], new RegExp(`${engineInfo.warForce} ${f.leader}'s crew's nearest corner: .+ tonight, odds (~\\d|\\?)`), "the confirm names the target and the odds");
      for (const [t] of lines) clean(t, "the war's confirm");
      q("declare_war", f.id);
      assert.equal(session.refresh().you.war, f.id, "war declared");
      q("call_off_war");
      assert.ok(!session.refresh().you.war, "war called off");
    } else clean(why, "the war's refusal");
  }
  assert.match(table.warRefusal({ ...v, you: { ...v.you, war: v.factions[0].id } }, v.factions[0]), /^Can't declare war on .+: one war at a time/);
  for (const [, t] of table.lifetimeRows(v)) clean(t, "a lifetime row");
}
// The routine (#556): routine.js's words on the live run; a standing
// order and a supply contract set as the page sets them survive the
// save, and the cart is the engine's orders, never a memo.
if (!session.view.over) {
  const routine = await import(pathToFileURL(path.join(dist, "routine.js")));
  const q = (m, ...p) => session.call(m, ...p),
    clean = (s, what) => assert.ok(typeof s === "string" && !/undefined|NaN|\[object/.test(s), `${what}: ${s}`);
  let v = session.refresh();
  const here = v.you.city,
    supplied = v.cities.find((c) => c.id === here).products.find((p) => !p.no_supply);
  assert.ok(supplied, "a product the connects sell here");
  const id = supplied.id;
  assert.equal(routine.readKeep(v, here, id, "").units, routine.keepMax(v, here, id), "blank keeps what the stash holds");
  assert.ok(routine.readKeep(v, here, id, String(routine.keepMax(v, here, id) + 1)).err, "over the stash is refused");
  const keep = Math.min(20, routine.keepMax(v, here, id));
  q("set_supply", here, id, keep);
  clean(routine.keepSaid(v, q, here, id, keep), "the contract's answer");
  v = session.refresh();
  const k = routine.contract(v, here, id);
  assert.ok(k && k.own && k.units === keep, "the contract is on the view");
  assert.match(routine.contractRows(v, q, here, id)[0][0], new RegExp(`^keep at ${keep}$`));
  assert.match(routine.editingContract(v, here, id), /^Editing the contract \(keep \d+\); /);
  assert.match(routine.keptLine(v, here, id), /^Kept at \d+ by contract; this buys once and leaves it\.$/);
  for (const [t] of routine.contractRows(v, q, here, id)) clean(t, "a contract row");
  // A standing order kept at the whole stash (#503), where there is any.
  const most = routine.standable(v, q, here, id);
  if (most > 0) {
    const r = routine.readStanding(v, q, here, id, "");
    assert.equal(r.qty, routine.ALL, "blank is the whole stash");
    assert.ok(routine.readStanding(v, q, here, id, String(most + 1)).err, "over what may stand is refused");
    q("place_standing", here, id, r.qty, "quiet");
    v = session.refresh();
    const st = routine.standing(v, here, id);
    assert.ok(st && st.all && st.dial === "quiet", "the standing order is on the view, kept at all");
    assert.match(routine.standingRow(v, q, here, id), /^all quiet · cut \d+%$/);
    assert.match(routine.editingStanding(v, here, id), /^Editing the standing order \(all quiet\); /);
    clean(routine.standingSaid(v, q, here, id, routine.ALL, "quiet"), "the standing order's answer");
  } else clean(routine.sellRefusal(v, q, here, id), "nothing to stand on");
  // The cart is the engine's: each order of the day, each standing order
  // where none was placed (none on a quiet day), each contract of yours.
  const lines = routine.cart(v, q);
  for (const o of v.orders || []) assert.ok(lines.some((l) => l.kind === "sell" && l.city === o.city && l.product === o.product && l.qty === o.qty), "an order of the day is in the cart");
  for (const o of v.standing || [])
    assert.equal(lines.some((l) => l.kind === "standing" && l.city === o.city && l.product === o.product), !v.you.lie_low && !routine.order(v, o.city, o.product), "a standing order sells tonight unless an order stands over it");
  for (const c of (v.supply || []).filter((x) => !x.lieutenant)) assert.ok(lines.some((l) => l.kind === "keep" && l.city === c.city && l.product === c.product && l.qty === c.units), "a contract is a keep line");
  for (const l of lines) clean(routine.cartLine(v, l), "a cart line");
  clean(routine.cartTotals(lines), "the cart's totals");
  for (const why of ["road", "room", "supplier", "cash"]) clean(routine.supplyShortWords(why), "a supply short");
  // Both survive the save and show on reload.
  const again = new Session(globalThis.kingpin);
  again.importSave(session.exportSave());
  assert.deepEqual(again.view.supply, v.supply, "the contract survives the save");
  assert.deepEqual(again.view.standing, v.standing, "the standing order survives the save");
}
// The sweep, the houses and the assets (#581): property.js's words on
// the live run; a sweep set and a house leased, stocked, guarded and
// dropped as the page does it, through the engine.
if (!session.view.over) {
  const property = await import(pathToFileURL(path.join(dist, "property.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const q = (m, ...p) => session.call(m, ...p),
    clean = (s, what) => assert.ok(typeof s === "string" && !/undefined|NaN|\[object/.test(s), `${what}: ${s}`);
  let v = session.refresh();
  assert.equal(property.readSweep(v, "").keep, 0, "blank keeps the upkeep alone");
  assert.equal(property.readSweep(v, String(property.sweepMax(v) + 1)).field, property.sweepMax(v), "over the clean in hand is held to it");
  for (const [, t] of property.sweepLines(v, q, 0)) clean(t, "a sweep line");
  clean(property.sweepRules(q), "the sweep's rules");
  q("set_sweep", 0);
  v = session.refresh();
  assert.ok(v.you.sweep_on, "the sweep is on");
  assert.match(property.sweepState(v, q), /^on, keeping \$[\d,]+$/);
  clean(property.sweepSaid(q, 0), "the sweep's answer");
  q("stop_sweep");
  assert.ok(!session.refresh().you.sweep_on, "the sweep is off");
  for (const a of q("asset_offers") || []) clean(property.assetBlurb(v, a, engineInfo), `${a.ID}'s blurb`), assert.ok(property.assetBlurb(v, a, engineInfo), `${a.ID} says what it does`);
  // A house, on a run of its own (seed 7 trades weed until a house is on
  // offer and paid for, day 30): leased, stocked, the stock moved,
  // guarded and dropped.
  const rich = new Session(globalThis.kingpin),
    r = (m, ...p) => rich.call(m, ...p);
  rich.newRun(7);
  let offer = null;
  for (let i = 0; i < 120 && !rich.view.over && !offer; i++) {
    if (rich.view.card) rich.choose(0);
    v = rich.refresh();
    offer = (r("house_offers") || []).filter((h) => h.City === v.you.city && h.Price <= v.you.dirty_cash && h.UnlockCash <= v.you.peak_cash).sort((a, b) => a.Price - b.Price)[0];
    if (offer) break;
    const sup = v.connects.find((x) => x.city === v.you.city && x.open && !x.wholesale),
      n = Math.floor(rich.maxBuy(sup.id, "weed").max * 0.8);
    if (n) rich.buy(sup.id, "weed", n);
    v = rich.refresh();
    for (const [p, n] of Object.entries(v.you.stock[v.you.city] || {})) if (n) rich.sell(v.you.city, p, n, "normal");
    rich.endDay();
  }
  assert.ok(offer, "the trading run can lease a house");
  {
    r("buy_house", offer.ID);
    const sup = rich.refresh().connects.find((x) => x.city === rich.view.you.city && x.open && !x.wholesale);
    rich.buy(sup.id, "weed", Math.min(5, rich.maxBuy(sup.id, "weed").max));
    v = rich.refresh();
    const h = v.houses.find((x) => x.id === offer.ID);
    assert.ok(h && h.rent === offer.Rent && h.price === offer.Price && h.bought === v.day, "view 19 carries the house's rent, price and day");
    assert.deepEqual(property.houseStatus(h), ["unknown", "good"]);
    for (const [, t] of property.houseRows(v, r, h)) clean(t, "a house row");
    const city = v.you.city,
      routine = await import(pathToFileURL(path.join(dist, "routine.js")));
    // The stash is the street and the houses (`you.stock`, World.Stock);
    // the street is what the houses do not hold.
    assert.equal(routine.stock(v, city, "weed"), property.held(v, city, "", "weed") + (h.stock.weed || 0), "the stash is the street and the house");
    {
      const src = property.places(v, city)[0],
        dst = property.places(v, city, src)[0],
        product = "weed",
        n = Math.min(3, property.moveMax(v, city, src, dst, product));
      clean(property.moveNote(v, city, dst, engineInfo), "the move's note");
      const before = property.placeHolds(v, city, ""),
        had = property.held(v, city, dst, product);
      const moved = r("move", city, src, dst, product, n);
      v = rich.refresh();
      assert.equal(property.held(v, city, dst, product), had + moved, "the move reached its place");
      assert.equal(property.placeHolds(v, city, "").capacity, before.capacity, "the street's capacity does not move with a move");
      clean(property.movedSaid(v, r, city, src, dst, product, moved), "the move's answer");
    }
    const guard = property.guardRows(v, h);
    assert.equal(guard[0].id, 0, "nobody is the first guard");
    const enforcer = guard.find((g) => g.id && !v.crew.find((m) => m.id === g.id).jailed);
    if (enforcer) {
      r("guard", h.id, enforcer.id);
      v = rich.refresh();
      assert.equal(v.houses.find((x) => x.id === h.id).guard, enforcer.id, "the guard is inside");
      clean(property.guardSaid(v, r, h, enforcer.id), "the guard's answer");
      r("guard", h.id, 0);
    }
    const units = property.houseUnits(rich.refresh().houses.find((x) => x.id === h.id));
    for (const l of property.dropLines(h)) clean(l, "the drop's words");
    r("drop", h.id);
    assert.ok(!rich.refresh().houses.some((x) => x.id === h.id), "dropped");
    clean(property.droppedSaid(h, units), "the drop's answer");
  }
}
// The connects, the market pane, the routes and the cart's buys (#582):
// market.js's, routes.js's and routine.js's words on the live run; a buy
// for cash and one on a connect's book, each in the cart from view 20's
// buys at the quote the page shows and returned as the cart returns it;
// a route's days target set and cleared; the presets reviewed in names.
if (!session.view.over) {
  const market = await import(pathToFileURL(path.join(dist, "market.js")));
  const roads = await import(pathToFileURL(path.join(dist, "routes.js")));
  const routine = await import(pathToFileURL(path.join(dist, "routine.js")));
  const q = (m, ...p) => session.call(m, ...p),
    clean = (s, what) => assert.ok(typeof s === "string" && !/undefined|NaN|\[object/.test(s), `${what}: ${s}`);
  let v = session.refresh();
  const here = v.you.city;
  assert.ok(Array.isArray(v.buys) && v.you.away && typeof v.you.street_quality === "number", "view 20's buys, away and street quality");
  for (const c of v.connects) {
    assert.ok(typeof c.day_cap === "number" && Array.isArray(c.products) && c.quality, `${c.id}: view 20's connect fields`);
    clean(market.note(v, c)[0], "a connect's note");
    for (const [l, t] of market.pane(v, q, c)) clean(l + t, `${c.name}'s pane`);
    for (const [t] of market.blurb(v, c)) clean(t, "a connect's blurb");
    clean(market.paneRules(q, c), "the connect's rules");
    clean(market.howToBuy(v, q, c), "how to buy");
  }
  assert.ok(market.connectsIn(v, here).length, "connects where you stand");
  assert.deepEqual([-2, 0, 1, 3].map(market.dueWord), ["2 days late", "today", "tomorrow", "in 3 days"]);
  clean(market.whyNobodySells(v, here), "nobody sells");
  for (const c of v.cities)
    for (const p of c.products) {
      for (const [l, t] of [...market.productPane(v, q, c.id, p.id), ...market.elsewhere(v, c.id, p.id)]) clean(l + t, `${p.id}'s pane`);
      for (const [t] of market.notes(v, q, c.id, p.id)) clean(t, "a market note");
      assert.equal(typeof p.glut, "number", "view 20's glut");
    }
  assert.match(market.productPane(v, q, here, "weed")[1][0], /^glut$/);
  // A cash buy: the quote is what it costs, the cart lists it, and the
  // return gives it back.
  const sup = market.sellers(v, "weed")[0];
  if (sup && session.maxBuy(sup.id, "weed").max >= 2) {
    const want = market.quote(sup, "weed", 2, false),
      p = q("buy", sup.id, "weed", 2, false);
    assert.equal(p.Cost, want, "the quote is the cost");
    clean(market.boughtSaid(v, sup, "weed", p), "the buy's answer");
    v = session.refresh();
    const line = routine.cart(v, q).find((l) => l.kind === "buy" && l.product === "weed" && l.city === here);
    assert.ok(line && line.qty >= 2, "the buy is in the cart");
    clean(routine.cartLine(v, line), "a buy's cart line");
    assert.match(routine.cartTotals(routine.cart(v, q)), /^Buying \d+ lines? for \$/);
    const refund = q(routine.giveBack(line), line.city, line.product, line.qty);
    assert.ok(refund > 0, "the return gives the cash back");
    clean(routine.returned(v, line, line.qty, refund), "the return's answer");
    assert.ok(!session.refresh().buys.some((b) => b.city === here && b.product === "weed" && !b.contract && !b.credit), "returned");
  }
  // On the book, where a connect gives credit.
  v = session.refresh();
  const lender = v.connects.find((c) => c.city === here && c.open && market.credit(c) > 0 && market.available(c, "weed"));
  if (lender && q("max_buy", lender.id, "weed", true).max >= 1) {
    const want = market.quote(lender, "weed", 1, true);
    for (const [t] of market.creditTerms(v, lender, "weed", 1)) clean(t, "the credit terms");
    const p = q("buy", lender.id, "weed", 1, true);
    assert.ok(p.Credit && p.Cost === want, "a buy on the book at the quote");
    v = session.refresh();
    const line = routine.cart(v, q).find((l) => l.kind === "credit");
    assert.ok(line, "the credit buy is in the cart");
    assert.match(routine.cartLine(v, line), / on the book$/);
    assert.match(routine.cartTotals(routine.cart(v, q)), /on credit/);
    q("return_credit", line.city, line.product, line.qty);
    assert.ok(!session.refresh().buys.some((b) => b.credit), "off the book");
  }
  // The room where you stand is counted without your carry (#524).
  v = session.refresh();
  assert.ok(v.you.away[here] <= v.you.room[here] + Object.values(v.you.stock[here] || {}).reduce((n, x) => n + x, 0), "the room without you is no more than the stash's");
  assert.match(routine.contractRoom(v, here, "weed", 1e6), / without you: your carry leaves with you/);
  for (const t of routine.contractsLeft(v, here)) clean(t, "a contract left behind");
  // The routes: every pane row reads, a days target sets and clears.
  for (const r of v.routes) {
    for (const [l, t] of roads.facts(v, q, r)) clean(l + t, `${r.id}'s pane`);
    clean(roads.targetIntro(v, q, r), "the target's intro");
    for (const x of roads.targetRows(v, q, r)) clean(x.name + x.target, "a target row");
  }
  const r = v.routes.find((x) => x.from === here || x.to === here) || v.routes[0];
  if (r) {
    for (const [l, t] of roads.preview(v, q, r, "weed", 3, true)) clean(l + t, "a target preview");
    q("set_route_days", r.id, "weed", 3);
    v = session.refresh();
    const rr = v.routes.find((x) => x.id === r.id);
    assert.match(roads.targetLine(v, q, rr), /^3d \(≈\d+\) Weed/);
    clean(roads.targetSaid(v, q, rr, "weed", 3, true), "the target's answer");
    q("set_route_days", r.id, "weed", 0);
    assert.equal(roads.targetLine(session.refresh(), q, session.view.routes.find((x) => x.id === r.id)), roads.targetLine(v, q, r), "cleared");
    clean(roads.driverSaid(v, q, r, 0), "nobody drives");
  }
  // The presets in names, and the upgrades' prerequisites.
  for (const pr of session.presets()) {
    const rv = session.presetDiff(pr.id);
    for (const c of rv.changes) {
      clean(routine.changeName(v, c), "a change's name");
      clean(routine.changeEstimate(v, q, c), "a change's estimate");
      assert.doesNotMatch(routine.changeName(v, c), /_/, "a change in words");
    }
    for (const x of rv.refused) clean(routine.commandName(v, x.command), "a refused command");
  }
  for (const u of v.upgrades) for (const x of routine.requiresNames(v, u)) assert.ok(v.upgrades.some((w) => w.name === x.name), `${u.id} requires by name`);
}
// The dashboard's facts (#574): a run of its own (seed 41, stingy pay,
// four hired, the stash sold aggressive each night) reaches a patrol's
// cap by day 9 and crew under the line by day 16; its enforcer is sent
// at a rival corner and the war order is declared, as the page does it.
{
  const crew = await import(pathToFileURL(path.join(dist, "crew.js")));
  const law = await import(pathToFileURL(path.join(dist, "law.js")));
  const table = await import(pathToFileURL(path.join(dist, "rivals.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const s = new Session(globalThis.kingpin),
    q = (m, ...p) => s.call(m, ...p);
  s.newRun(41);
  q("set_pay", "stingy");
  const tun = q("rules.crew.tuning"),
    flip = q("rules.crew.flip_line");
  let capped = "",
    troubled = "",
    struck = "",
    warred = "";
  for (let i = 0; i < 40 && !s.view.over && !(capped && troubled && struck && warred); i++) {
    if (s.view.card) s.choose(0);
    let v = s.refresh();
    for (const m of v.pool)
      if (v.crew.length < 4 && m.fee < v.you.dirty_cash * 0.3) {
        s.hire(m.id);
        v = s.refresh();
      }
    const supplier = v.connects.find((x) => x.open && !x.wholesale && x.city === v.you.city);
    const n = supplier ? s.maxBuy(supplier.id, "weed").max : 0;
    if (n) {
      s.buy(supplier.id, "weed", n);
      s.sell(v.you.city, "weed", n, "aggressive");
    }
    v = s.refresh();
    // The cap on the risk panel, naming the city whose patrol set it.
    if (v.law.sell_cap_days > 0 && !capped) {
      capped = law.patrolCapLine(v);
      const where = v.cities.find((c) => c.id === v.law.sell_cap_city).name;
      assert.equal(capped, `Patrols in ${where}: sales capped at ${Math.round(v.law.sell_cap * 100)}% of demand for ${v.law.sell_cap_days} day${v.law.sell_cap_days === 1 ? "" : "s"} more.`, "the patrol's cap");
    }
    // The crew's trouble: each member in the band once, the idle corners.
    const t = crew.trouble(v, tun, flip);
    if (t && !troubled) {
      troubled = t;
      const band = new Set([...v.alerts.filter((a) => a.kind === "crew_line").map((a) => a.member), ...v.crew.filter((m) => m.loyalty < crew.lineOf(m, tun, flip)).map((m) => m.id)]);
      assert.ok(t.startsWith(`${band.size} near or under the line`), "the crew's trouble: " + t);
    }
    // The strike tonight, by the corner's name, and the war order's line.
    const target = v.cities.find((c) => c.id === v.you.city).corners.find((c) => c.owner === "rival");
    const enforcer = v.crew.find((m) => m.role === "enforcer" && !m.jailed && !m.wounded);
    if (target && enforcer && !struck) {
      q("send_enforcers", target.id, "push");
      struck = table.tonightLine(s.refresh(), q, engineInfo, { corner: target.id, force: "push" });
      assert.equal(struck, `Enforcers go to ${target.name} tonight: push.`, "the strike tonight");
      assert.equal(table.tonightLine(v, q, engineInfo, null), "", "no strike and no war, no line");
    }
    const f = target && v.factions.find((x) => x.id === (target.faction || "rival"));
    if (f && struck && !warred && !table.warRefusal(v, f)) {
      q("declare_war", f.id);
      warred = table.tonightLine(s.refresh(), q, engineInfo, null);
      assert.match(warred, new RegExp(`^War on ${f.leader}'s crew: (enforcers go to .+ tonight, ${engineInfo.warForce}\\.|no enforcer at work|nowhere to go tonight\\.)`), "the war's line: " + warred);
      q("call_off_war");
    }
    s.endDay();
    s.take();
  }
  assert.ok(capped, "the run reaches a patrol's cap");
  assert.ok(troubled, "the run reaches crew trouble");
  assert.ok(struck, "the run sends its enforcer");
  assert.ok(warred, "the run declares the war");
  assert.equal(law.patrolCapLine({ ...s.view, law: { ...s.view.law, sell_cap_days: 0 } }), "", "no cap, no line");
}
// The stage and the new mark (#578): stage.js's words on a run that
// hires on day 0 and so enters the Crew stage the next morning; the
// stage waits through a save until see_stage, and is gone after it. The
// market pane's next product, off engine-info's ladder.
{
  const stage = await import(pathToFileURL(path.join(dist, "stage.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const clean = (t, what) => assert.ok(typeof t === "string" && t && !/undefined|NaN|\[object/.test(t), `${what}: ${t}`);
  const s = new Session(globalThis.kingpin);
  let v = s.newRun(41);
  assert.equal(stage.pending(v), 0, "no stage on day 0");
  assert.equal(stage.tierLabel(v), v.you.tier_name, "no mark on day 0");
  assert.equal(stage.interstitial(v), null, "no interstitial on day 0");
  assert.deepEqual(engineInfo.ladder.map((p) => p.id), ["weed", "pills", "coke", "heroin", "meth", "designer"], "the ladder in the file's order");
  assert.equal(stage.nextProductNote(v, engineInfo, v.you.city), `Heroin lists at $3,000 peak cash ($${(3000 - v.you.peak_cash).toLocaleString("en-US")} to go).`, "the next product");
  const eastside = { ...v, you: { ...v.you, peak_cash: 12_000 }, cities: v.cities.map((c) => ({ ...c, products: engineInfo.ladder.slice(0, 5).map((p) => ({ id: p.id })) })) };
  assert.equal(stage.nextProductNote(eastside, engineInfo, "eastside"), "Designer lists at $500K peak cash ($488K to go); the supplier here will not sell it, the road brings it.", "the honest form");
  assert.equal(stage.nextProductNote({ ...eastside, cities: eastside.cities.map((c) => ({ ...c, products: engineInfo.ladder })) }, engineInfo, "eastside"), "", "no line once the ladder is listed");
  for (const m of [...v.pool].sort((a, b) => a.fee - b.fee)) if (m.fee <= s.refresh().you.dirty_cash && !s.refresh().crew.length) s.call("hire", m.id);
  assert.ok(s.refresh().crew.length, "somebody hired");
  s.endDay();
  v = s.refresh();
  assert.equal(stage.pending(v), 2, "the Crew stage the morning after the hire");
  assert.equal(stage.tierLabel(v), `${v.you.tier_name} · new`, "the tier marked new");
  const i = stage.interstitial(v);
  assert.equal(i.title, "STAGE 2 · CREW");
  for (const t of [i.blurb, i.text, i.next, ...i.opened]) clean(t, "the stage's words");
  assert.ok(i.opened.length, "what it opened");
  assert.equal(i.lanes, "", "no lanes before the Cartel");
  assert.equal(stage.lanes({ ...v, you: { ...v.you, tier: 2 }, alerts: [{ kind: "exports" }] }), true, "the lanes where the exports alert stands");
  const reloaded = new Session(globalThis.kingpin);
  reloaded.importSave(s.exportSave());
  assert.equal(stage.pending(reloaded.refresh()), 2, "a reload before it is seen still shows it");
  s.call("see_stage", 2);
  v = s.refresh();
  assert.equal(stage.pending(v), 0, "seen");
  assert.equal(stage.tierLabel(v), v.you.tier_name, "the mark goes once seen");
  reloaded.importSave(s.exportSave());
  assert.equal(reloaded.refresh().stage, undefined, "a reload after it is seen does not show it again");
}
// The sale's heat, pressure in numbers and the glossary (#575): a run of
// its own (seed 41, four hired, the stash sold aggressive each night)
// previews every sale before it is placed, and the estimate is the
// TUI's (ui/dialogs.go estHeat: the sale's heat on the units asked plus
// the sloppy runners' on the units expected to move) until a night
// where the runners add theirs; the risk panel's pressure note and the
// glossary are filled off the file.
{
  const routine = await import(pathToFileURL(path.join(dist, "routine.js")));
  const law = await import(pathToFileURL(path.join(dist, "law.js")));
  const { glossary, WORDS } = await import(pathToFileURL(path.join(dist, "glossary.js")));
  const { engineInfo } = await import(pathToFileURL(path.join(dist, "engine-info.js")));
  const s = new Session(globalThis.kingpin),
    q = (m, ...p) => s.call(m, ...p);
  s.newRun(41);
  let previewed = 0,
    sloppy = 0;
  for (let i = 0; i < 30 && !s.view.over && !(previewed > 6 && sloppy); i++) {
    if (s.view.card) s.choose(0);
    let v = s.refresh();
    for (const m of v.pool)
      if (v.crew.length < 4 && m.fee < v.you.dirty_cash * 0.3) {
        s.hire(m.id);
        v = s.refresh();
      }
    // The runners go on the corners nobody works, as the page posts them.
    for (const m of v.crew.filter((x) => x.role === "runner")) {
      const corners = v.cities.find((c) => c.id === v.you.city).corners;
      if (corners.some((c) => c.runner === m.id)) continue;
      const open = corners.find((c) => c.owner !== "rival" && !c.runner);
      if (open && q("post", open.id, m.id) !== null) v = s.refresh();
    }
    const supplier = v.connects.find((x) => x.open && !x.wholesale && x.city === v.you.city);
    const n = supplier ? s.maxBuy(supplier.id, "weed").max : 0;
    if (n) {
      s.buy(supplier.id, "weed", n);
      v = s.refresh();
      const here = v.cities.find((c) => c.id === v.you.city),
        p = here.products.find((x) => x.id === "weed");
      for (const dial of ["quiet", "normal", "aggressive"]) {
        const r = routine.salePreview(v, q, here.id, "weed", n, dial),
          est = Math.min(n, q("rules.market.capacity", here.id, "weed", dial)),
          heat = q("rules.heat.sale_heat", here.id, "weed", n, dial) + q("rules.heat.sloppy_heat", here.id, est),
          unit = p.price * q("rules.market.dial", dial).Price;
        assert.equal(r.est, est, "the units expected");
        assert.equal(r.heat, heat, "the estimate is the TUI's estHeat");
        assert.equal(r.rows[0][1], `~${est} of ${n} at ~$${unit.toFixed(2)} = ~$${Math.trunc(est * unit).toLocaleString("en-US")}`, "the expect row");
        assert.equal(r.rows[1][1], `+${heat.toFixed(1)}`, "the heat row");
        assert.equal(r.rows[1][2], routine.heatTone(here.heat + heat * 4), "the heat row's tone");
        assert.equal(r.rows[2][1], routine.DIAL_BLURBS[dial], "the dial's blurb");
        previewed++;
      }
      if (q("rules.heat.sloppy_heat", here.id, 100) > 0) sloppy++;
      s.sell(here.id, "weed", n, "aggressive");
    }
    s.endDay();
    s.take();
  }
  assert.ok(previewed > 6, "the run previews its sales");
  assert.ok(sloppy, "the run's runners add their premium to an estimate");
  const v = s.refresh(),
    here = v.cities.find((c) => c.id === v.you.city);
  // Nowhere worked, nothing sells: the dialog says so.
  const none = routine.salePreview({ ...v, cities: v.cities.map((c) => ({ ...c, worked: 0 })) }, q, here.id, "weed", 5, "normal");
  assert.equal(none.rows.at(-1)[1], `You work no corner in ${here.name}: nothing will sell. Post somebody on the street view.`, "no corner worked");
  // Pressure in numbers: the cuts at the city's pressure, the fade, and
  // the goodwill's bite where some is bought.
  const tun = q("rules.law.tuning"),
    fx = engineInfo.pressure,
    at = (pressure, goodwill) => ({ ...v, cities: v.cities.map((c) => (c.id === here.id ? { ...c, pressure, goodwill } : c)) });
  assert.ok(fx.thresholdCut > 0 && fx.capCut > 0, "law.toml's [effects] reach the page");
  assert.equal(
    law.pressureNote(at(60, 0), q, here.id, fx),
    `Lowers the police lines ${Math.round(fx.thresholdCut * 60)}% and a patrol's cap ${Math.round(fx.capCut * 60)}%; fades to ${Math.round(tun.Baseline)}.`,
    "the pressure note",
  );
  assert.match(law.pressureNote(at(60, 40), q, here.id, fx), new RegExp(` Goodwill takes ${((tun.GoodwillCut * 40) / 100).toFixed(1)} a day\\.$`), "the goodwill's bite");
  assert.match(law.pressureNote(at(250, 0), q, here.id, fx), new RegExp(`^Lowers the police lines ${Math.round(fx.thresholdCut * 100)}% `), "pressure counts to 100");
  assert.ok(law.pressureNote(v, q, here.id, fx).startsWith("Lowers the police lines "), "the live city's note");
  // The glossary: the TUI's terms, the numbers filled off the file.
  const g = glossary(engineInfo.glossary),
    line = (t) => g.find(([term]) => term === t)[1];
  assert.equal(g.length, WORDS.length);
  for (const [term, l] of g) assert.ok(term && l && !/[{}]/.test(l), "a filled glossary line: " + term);
  assert.equal(line("quiet day"), `heat under ${engineInfo.glossary.retireHeat}; no strike, push, war, bust, buyer's order owed`);
  assert.equal(line("sting"), `a rung: stock and cash; 1 page if you sold, ${engineInfo.glossary.hitPages} on a named hit`);
  assert.equal(line("raid"), "a big rung: much of the stock and cash; 2 pages if you sold");
  assert.equal(line("betrayed"), `a lieutenant turns on ${Math.round(engineInfo.glossary.betrayShare * 100)}% of your corners, ${engineInfo.glossary.betrayCorners}+: that night`);
  assert.equal(line("street night"), `a night's dealing; going straight: fronts over its last ${engineInfo.streetWindow}`);
}
// Fast-forward (#555): the cap the dialog reads, a run fast-forwarded
// to its stops and each worded by the stop rule's kind, the hold that
// refuses the start in the alert's own words, and a danger's numbers.
{
  const fast = await import(pathToFileURL(path.join(dist, "fast.js")));
  const { alertText } = await import(pathToFileURL(path.join(dist, "alerts.js")));
  assert.deepEqual(fast.cap(""), { days: 7 }, "blank is the default cap");
  assert.deepEqual(fast.cap(" 12 "), { days: 12 });
  assert.deepEqual(fast.cap("30"), { days: 30 });
  assert.deepEqual(fast.cap("31"), { error: "up to 30 days at a time" });
  for (const bad of ["0", "-2", "1.5", "x"]) assert.deepEqual(fast.cap(bad), { error: "enter a whole number above zero" }, bad);
  assert.equal(fast.ask(1), "Run up to 1 day, stopping when something needs you.");
  assert.equal(fast.fileClose(3, 6), "file 3/6: 3 pages from an indictment");
  assert.equal(fast.fileClose(5, 6), "file 5/6: one more page is an indictment");
  // Seed 11 left alone: it stops for buyers, rivals and cards, every
  // stop worded as the TUI words it, until the run is broke tonight and
  // the hold refuses the start.
  const s = new Session(globalThis.kingpin);
  s.newRun(11);
  assert.equal(s.call("holds"), null, "a fresh run holds nothing");
  const kinds = new Set();
  let first = null;
  for (let i = 0; i < 40 && !s.call("holds") && !s.view.over; i++) {
    const before = s.refresh().day,
      r = s.fastForward(30),
      v = s.refresh();
    s.take();
    assert.equal(r.day, v.day);
    assert.equal(r.day - before, r.ran, "the days run");
    assert.ok(r.ran >= 1 && r.ran <= 30);
    assert.equal(!!r.payload, r.stop === "event", "an event stop carries its event");
    const st = fast.stop(v, r);
    assert.match(st.text, new RegExp(`^Stopped after ${r.ran} days?${r.danger ? " on a danger" : ""}: .+\\.$`), st.text);
    assert.equal(st.tone, r.danger ? "danger" : "warn");
    if (r.stop === "event") {
      assert.notEqual(fast.eventWhy(v, r.event, r.payload), r.event, "the event has words: " + r.event);
      kinds.add(r.event);
    }
    if (r.stop === "alert") assert.ok(st.text.includes(alertText(v, r.alert).replace(/[.\s]+$/, "")), "an alert stop is the alert's words");
    first ??= st.text;
    while (s.refresh().card) s.choose(0);
    if (s.refresh().stage?.pending) s.call("see_stage", s.refresh().stage.pending);
  }
  assert.equal(first, "Stopped after 6 days: Marco at the Velvet Room is asking.", "the first stop, a buyer");
  assert.ok(kinds.has("RivalMovedIn") && kinds.has("ContractOffered"), "the run stops on events");
  const v = s.refresh(),
    hold = s.call("holds");
  assert.equal(hold?.kind, "broke", "the idle run is broke tonight");
  assert.equal(fast.held(v, hold), `Fast-forward will not run tonight: ${alertText(v, hold).replace(/[.\s]+$/, "")}. Deal with it, or end the day by hand.`);
  const held = s.fastForward(7);
  assert.equal(held.ran, 0, "the hold runs no night");
  assert.equal(held.day, v.day);
  assert.equal(fast.stop(v, held), null, "a run of no days has no stop line");
  assert.equal(fast.held(v, null), "");
  // A danger stop on an event carries the file (#504).
  const d = fast.stop(v, { ran: 3, stop: "event", event: "TaskForceFormed", payload: { City: v.you.city }, danger: true });
  assert.equal(d.text, `Stopped after 3 days on a danger: a task force formed in ${v.cities.find((c) => c.id === v.you.city).name}, ${fast.fileClose(v.you.evidence, v.law.arrest_line)}.`);
  assert.equal(d.tone, "danger");
  assert.equal(fast.stop(v, { ran: 2, stop: "cap" }).text, "Stopped after 2 days: the cap.");
  assert.equal(fast.stop(v, { ran: 2, stop: "over" }), null, "the ending says it");
}
const restored = new Session(globalThis.kingpin);
restored.importSave(session.exportSave());
assert.deepEqual(restored.view, session.refresh());
console.log(
  `Street Edition engine integration passed at day ${session.view.day}: forecasts, dilemmas and their outcomes, the three sales approaches, cash flow and the cash line, the six characters, lanes, trophies, cash-out, the ways out, the lieutenants, the crew's answers and last words, where every alert lands, the till, the wash and the road, the wash's audit odds, throughput, rot and tax, a front's washed today, a Cook's batch and cut, the law and its answers, the endings, the crown and the rivals' table, the routine and the cart, the sweep and the houses, the connects, the market pane, the routes and the cart's buys, the dashboard's facts, the report lines' pointers, the stage and the next product, the sale's heat, pressure in numbers and the glossary, one way to write a number, fast-forward to a stop and its hold, and save round-trip.`,
);
process.exit(0);

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
  assert.ok(crew.summary(v, 0, engineInfo.sloppySkill)[0][1].endsWith("worked"), "the summary");
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
    [{ kind: "retire", act: act("ledger") }, "ledger", "", "", ""],
    [{ kind: "retire", ready: true, act: act("dashboard") }, "ledger", "", "", ""],
    [{ kind: "favour", act: act("ledger") }, "ledger", "favour", "", "law"],
    [{ kind: "reign", act: act("dashboard") }, "ledger", "", "", ""],
    [{ kind: "straight", act: act("dashboard") }, "ledger", "", "", ""],
    [{ kind: "vanish", act: act("dashboard") }, "ledger", "", "", ""],
    [{ kind: "exposure", act: act("ledger") }, "empire", "", "", ""],
    [{ kind: "plan", act: act("dashboard") }, "ledger", "", "", ""],
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
const restored = new Session(globalThis.kingpin);
restored.importSave(session.exportSave());
assert.deepEqual(restored.view, session.refresh());
console.log(
  `Street Edition engine integration passed at day ${session.view.day}: forecasts, dilemmas and their outcomes, the three sales approaches, cash flow and the cash line, the six characters, lanes, trophies, cash-out, the ways out, the lieutenants, the crew's answers, where every alert lands, the till, the wash and the road, a Cook's batch and cut, the law and its answers, and save round-trip.`,
);
process.exit(0);

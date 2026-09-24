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
    session.choose(0);
  }
  const v = session.view;
  const supplier = v.connects.find(
    (s) => s.open && !s.wholesale && s.city === v.you.city,
  );
  if (supplier) {
    const n = Math.min(10, session.maxBuy(supplier.id, "weed").max);
    if (n) {
      session.buy(supplier.id, "weed", n);
      session.sell(v.you.city, "weed", n, "quiet");
    }
  }
  session.endDay();
  const report = session.view.report;
  assert.ok(Array.isArray(report.sections));
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
const restored = new Session(globalThis.kingpin);
restored.importSave(session.exportSave());
assert.deepEqual(restored.view, session.refresh());
console.log(
  `Ink engine integration passed at day ${session.view.day}: forecasts, dilemmas, cash flow, lanes, trophies, cash-out, the ways out and save round-trip.`,
);
process.exit(0);

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
const restored = new Session(globalThis.kingpin);
restored.importSave(session.exportSave());
assert.deepEqual(restored.view, session.refresh());
console.log(
  `Ink engine integration passed at day ${session.view.day}: forecasts, dilemmas, cash flow and save round-trip.`,
);
process.exit(0);

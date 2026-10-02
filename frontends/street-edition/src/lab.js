// The cut and the cook (#557), as the TUI's market dialogs word them
// (ui/quality.go, #47), and the chemist's hand on the crew screen
// (ui/crew.go). Both are for where you stand: a cut is done in the
// stash you can reach, and the chemist works where you are. Each
// function takes the view and `q`, the engine's query (session.call),
// and returns words or numbers: it touches no DOM, so smoke.mjs checks
// them on a live run. The lot's quality and the stash's room are view
// 17's `you.quality` and `you.room`.

import { fixed, money, pct, price } from "./format.js?v=__BUILD_REVISION__";

const plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  times = (x) => "×" + fixed(x, 2),
  round = (x) => Math.round(x).toString();

const cityOf = (v, city) => v.cities.find((c) => c.id === city),
  cityName = (v, city) => cityOf(v, city)?.name || city,
  productOf = (v, city, id) => cityOf(v, city)?.products.find((p) => p.id === id),
  productName = (v, city, id) => productOf(v, city, id)?.name || id;

// chemist is the best chemist on the payroll, the one who cooks and
// cuts (view 15's `lab`), or undefined.
export function chemist(v) {
  return (v.crew || []).find((m) => m.lab);
}

// streetQuality is the quality the street pays full at, the file's
// default (World.StreetQuality).
export function streetQuality(q) {
  return q("rules.market.tuning").Default || 50;
}

// stock is what you hold of a product in a city, street and houses.
export function stock(v, city, id) {
  return v.you.stock?.[city]?.[id] || 0;
}

// quality is the lot's quality, the street's where it has none.
export function quality(v, q, city, id) {
  return v.you.quality?.[city]?.[id] || streetQuality(q);
}

// cooking is what is on its way to a city's stash of a product.
export function cooking(v, city, id) {
  return (v.cooks || []).filter((k) => k.city === city && k.product === id).reduce((n, k) => n + k.units, 0);
}

// cookCost is a unit's precursors in a city, the lab's discount in.
export function cookCost(q, city, id) {
  return q("rules.crew.cook_cost_in", city, q("rules.market.cook_cost", id));
}

// cutProducts are the products the stash in the city holds that the
// file lets you cut.
export function cutProducts(v, q, city) {
  return (cityOf(v, city)?.products || []).map((p) => p.id).filter((id) => stock(v, city, id) > 0 && q("rules.market.cut_max", id) > 0);
}

// cookProducts are the products a chemist cooks that are on the ladder
// in the city.
export function cookProducts(v, q, city) {
  return (cityOf(v, city)?.products || []).map((p) => p.id).filter((id) => q("rules.market.cooks", id));
}

// refuseCut is why the cut cannot open where you stand, or null.
export function refuseCut(v, q) {
  if (v.you.lie_low) return "Lying low today: nobody is working.";
  if (!cutProducts(v, q, v.you.city).length) return `Nothing here to cut: the stash in ${cityName(v, v.you.city)} is empty.`;
  return null;
}

// refuseCook is why the cook cannot open, or null.
export function refuseCook(v, q) {
  if (v.you.lie_low) return "Lying low today: nobody is working.";
  if (!chemist(v)) return "Nobody on the payroll can cook. Hire a chemist on the crew tab.";
  if (!cookProducts(v, q, v.you.city).length) return "Nothing to cook: no product a chemist makes is on the ladder yet.";
  return null;
}

// cutMax is the most percent a cut of the product can add: cut_max,
// held to the room in the city and the till with tonight's wages kept
// back (rules.crew.spare, #569).
export function cutMax(v, q, city, id) {
  const units = stock(v, city, id);
  let most = Math.trunc(q("rules.market.cut_max", id) * 100);
  if (units <= 0) return 0;
  const room = v.you.room?.[city] ?? 0;
  if (room < Math.trunc((units * most) / 100)) most = Math.min(most, Math.trunc((room * 100) / units));
  const cost = q("rules.market.cut_cost", id);
  if (cost > 0) {
    const affordable = Math.trunc(q("rules.crew.spare") / cost);
    if (affordable < Math.trunc((units * most) / 100)) most = Math.min(most, Math.trunc((affordable * 100) / units));
  }
  return Math.max(0, most);
}

// cookMax is the most units a cook order can be for: the batch where
// it stands, the room with what is on its way counted, and the till
// with tonight's wages kept back. The engine's rule (rules.crew.cook_max,
// #569), the TUI's too, so the two cannot drift.
export function cookMax(v, q, city, id) {
  return q("rules.crew.cook_max", city, id, q("rules.market.cook_cost", id));
}

// readQty reads a dialog's number (the TUI's readQty): blank is the
// most, and nothing to do where that is none. {n} or {error}.
export function readQty(raw, most) {
  const s = String(raw ?? "").trim();
  if (s === "") return most > 0 ? { n: most } : { error: "Nothing to do." };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n <= 0) return { error: "Not a whole number." };
  return { n };
}

// cutRows are the cut's product page: the lot, its quality, the most
// the file lets it grow and the price a unit added.
export function cutRows(v, q, city) {
  return cutProducts(v, q, city).map((id) => ({
    id,
    name: productName(v, city, id),
    units: stock(v, city, id),
    quality: Math.round(quality(v, q, city, id)),
    most: "+" + pct(q("rules.market.cut_max", id)),
    price: price(q("rules.market.cut_cost", id)),
  }));
}

// cookRows are the cook's product page: the precursors a unit, what a
// connect asks and what you hold.
export function cookRows(v, q, city) {
  return cookProducts(v, q, city).map((id) => ({
    id,
    name: productName(v, city, id),
    cook: price(cookCost(q, city, id)),
    buy: price(productOf(v, city, id).supplier_price),
    stash: stock(v, city, id),
  }));
}

// chemistLine is under the cook's product page: who cooks, at what,
// how many and how long, and the lab where it stands.
export function chemistLine(v, q, city) {
  let line = `${q("rules.crew.chemist_name")} cooks at quality ${round(q("rules.crew.quality_in", city))}, up to ${q("rules.crew.batch_in", city)} a batch, ready in ${plural(q("rules.crew.cook_days"), "day")}.`;
  if (q("rules.crew.lab", city)) line += " The lab is here.";
  return line;
}

// cookNote is the cook's number page's terms.
export function cookNote(v, q, city, id) {
  const chem = q("rules.crew.chemist_name");
  return `Precursors are ${price(cookCost(q, city, id))} a unit, dirty, paid now; the lot lands in ${cityName(v, city)} in ${plural(q("rules.crew.cook_days"), "day")} at quality ${round(q("rules.crew.quality_in", city))}, ${chem}'s. A batch is ${q("rules.crew.batch_in", city)}.`;
}

// cookCostLine is the cook's cost row for n units, against a connect.
export function cookCostLine(v, q, city, id, n) {
  return `${money(n * cookCost(q, city, id))} for ${n}, against ${money(n * productOf(v, city, id).supplier_price)} from a connect`;
}

// cutNote is the cut's number page's terms, the chemist's hand in.
export function cutNote(v, q, city, id) {
  let note = `The cut adds that share of the units at nothing, so the quality falls by the same share; ${price(q("rules.market.cut_cost", id))} a unit added, dirty. The street pays full at quality ${round(streetQuality(q))} and less under it; a corner sold under ${round(q("rules.market.tuning").RepeatFloor)} stops coming back.`;
  const chem = chemist(v);
  if (chem) note += ` ${chem.name}'s hand keeps ${round(q("rules.crew.cut_bonus"))} points of it.`;
  return note;
}

// cutPreview is what a cut at a percent makes of the lot: the units
// after, the quality after with the chemist's hand, and the cost.
export function cutPreview(v, q, city, id, pct) {
  const have = stock(v, city, id),
    added = Math.trunc((have * pct) / 100 + 0.5),
    from = quality(v, q, city, id);
  let after = from;
  if (have + added > 0) after = Math.min(from, (from * have) / (have + added) + q("rules.crew.cut_bonus"));
  return { units: have + added, quality: after, cost: added * q("rules.market.cut_cost", id) };
}

// cutAfterLine is the cut's after row.
export function cutAfterLine(v, q, city, id, pct) {
  const p = cutPreview(v, q, city, id, pct);
  return `${p.units} units at quality ${round(p.quality)}, sells at ${times(q("rules.market.quality_mul", p.quality))}, for ${money(p.cost)}`;
}

// cutDone is what the page says after a cut (the engine's CutRecord).
export function cutDone(v, q, rec) {
  const hand = rec.Chemist ? ` ${rec.Chemist} kept it at that.` : "";
  return `Cut ${rec.Units} ${productName(v, rec.City, rec.Product)} into ${rec.Units + rec.Added}: quality ${round(rec.From)} → ${round(rec.To)}, sells at ${times(q("rules.market.quality_mul", rec.To))}.${hand} Cost ${money(rec.Cost)}.`;
}

// cookDone is what the page says after a cook order (the engine's Cook).
export function cookDone(v, k) {
  return `${k.Chemist} is cooking ${k.Units} ${productName(v, k.City, k.Product)} at quality ${round(k.Quality)}, ready in ${plural(k.Ready - k.Ordered, "day")}. Cost ${money(k.Cost)}.`;
}

// onTheWay is the chemist's lots on their way, a line each.
export function onTheWay(v) {
  return (v.cooks || []).map((k) => `${k.chemist} is cooking ${k.units} ${productName(v, k.city, k.product)} in ${cityName(v, k.city)}: quality ${round(k.quality)}, lands ${k.ready - v.day <= 1 ? "tomorrow" : "Day " + k.ready}.`);
}

// hand is a chemist's hand as the crew screen words it (ui/crew.go):
// rows of [label, text, warn]. In the pool, what they would cook at;
// on the payroll, the best's cook and cut, another waiting.
export function hand(v, q, m, pool) {
  if (m.role !== "chemist") return [];
  if (pool) return [["cooks", `q ${round(q("rules.crew.quality_of", m.skill))} · ${q("rules.crew.batch_of", m.skill)} a batch`, false]];
  if (m.lab)
    return [
      ["cooks", `q ${round(q("rules.crew.chemist_quality"))} · ${q("rules.crew.batch")} a batch`, false],
      ["cuts", `keep ${round(q("rules.crew.cut_bonus"))} points`, false],
    ];
  return [
    ["post", "second to the best chemist", true],
    ["", "Only the best chemist cooks and cuts; this one is paid and waits for the job.", false],
  ];
}

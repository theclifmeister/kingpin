// The sweep, the houses and the assets (#581), as the TUI's sweep
// dialog, ledger STASH table, move, guard and drop dialogs and ASSETS
// block word them (ui/sweep.go, ui/houses.go, ui/assets.go). Each
// function takes the view and `q`, the engine's query (session.call),
// and the asset and move tuning the builder copies from the TOML
// (`info`, engine-info.js), and returns words, rows or numbers: it
// touches no DOM, so smoke.mjs checks them on a live run.

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n || 0)).toLocaleString("en-US"),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  pct = (x, d = 0) => (x * 100).toFixed(d) + "%",
  times = (x) => "×" + Number(x.toPrecision(3)).toString();
const cityName = (v, id) => v.cities.find((c) => c.id === id)?.name || id,
  productName = (v, id) => v.cities.flatMap((c) => c.products).find((p) => p.id === id)?.name || id,
  member = (v, id) => v.crew.find((m) => m.id === id);

// upkeepTonight is tonight's clean bill, the sweep's floor: the fronts'
// upkeep and the assets'.
const upkeepTonight = (q) => q("rules.laundering.upkeep") + q("rules.laundering.asset_upkeep");

// The sweep (#478): the clean over the line goes offshore every night,
// up to the day's lot less what was reserved by hand.
export function sweepState(v, q) {
  return v.you.sweep_on ? `on, keeping ${money(Math.max(v.you.sweep_keep || 0, upkeepTonight(q)))}` : "off";
}
// sweepMax is the most the line can be (#526): the clean in hand, or
// the line already set where that is more.
export const sweepMax = (v) => Math.max(v.you.clean_cash, v.you.sweep_keep || 0);
// sweepTonight is what the sweep would move tonight at keep, the
// laundering sim's own sum (Sweepable).
export function sweepTonight(v, q, keep) {
  const off = q("rules.laundering.offshore");
  return Math.max(0, Math.min(off.Lot - (v.you.reserved_today || 0), v.you.clean_cash - Math.max(keep, upkeepTonight(q))));
}
// sweepLines are the dialog's rows under the field at keep: tonight's
// move and its fee, and the upkeep kept back. Each [label, text, tone].
export function sweepLines(v, q, keep) {
  const n = sweepTonight(v, q, keep),
    due = upkeepTonight(q),
    rows = [n > 0 ? ["tonight", `${money(n)} moves, fee ${money(q("rules.laundering.fee", n))}: under the lot, nobody reads it`, "good"] : ["tonight", "nothing over the line to move", "subtle"]];
  if (due > 0) rows.push(["upkeep", `${money(due)} clean kept back every night`, ""]);
  return rows;
}
export function sweepRules(q) {
  return `Every night the clean over the line goes offshore, up to ${money(q("rules.laundering.offshore").Lot)} a day less what you reserve by hand, so it never files a page. The account keeps its fee. Off unless you turn it on.`;
}
// readSweep reads the field: blank keeps the upkeep alone (0), a line
// over the most is held to it. {keep} or {err, field}.
export function readSweep(v, raw) {
  const s = String(raw ?? "").trim();
  if (s === "") return { keep: 0 };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n < 0) return { err: "Enter a whole number of dollars." };
  const most = sweepMax(v);
  if (n > most) return { err: `The sweep keeps at most ${money(most)}, the clean in hand. The line is now ${money(most)}; set it again to keep it.`, field: most };
  return { keep: n };
}
export const sweepSaid = (q, keep) => `The sweep is on: clean over ${money(Math.max(keep, upkeepTonight(q)))} goes offshore every night, up to a lot.`;
export const sweepOffSaid = "The sweep is off: the clean stays in hand.";

// houseStatus is a house's state in the one lowercase vocabulary: known
// (the police have it), rent unpaid 2d, unknown. [text, tone].
export function houseStatus(h) {
  if (h.known) return ["known", "danger"];
  if (h.unpaid > 0) return [`rent unpaid ${h.unpaid}d`, "warn"];
  return ["unknown", "good"];
}
export const houseUnits = (h) => Object.values(h.stock || {}).reduce((n, x) => n + x, 0);

// houseRows are a house's detail (the TUI pane's): the block, what it
// holds and of what, the rent and how long unpaid, the guard, the
// robbery odds, since when. Each [label, text, tone].
export function houseRows(v, q, h) {
  const corner = v.cities.flatMap((c) => c.corners).find((c) => c.id === h.corner),
    rows = [["block", corner ? corner.name : h.corner || "-", ""], ["stash", `${houseUnits(h)} of ${h.capacity}`, ""]];
  for (const [id, n] of Object.entries(h.stock || {})) if (n > 0) rows.push(["", `${n} ${productName(v, id)}`, "subtle"]);
  rows.push(["rent", `${money(h.rent)}/day clean${h.unpaid > 0 ? ` · unpaid ${h.unpaid} of ${q("rules.territory.rent_days")} days` : ""}`, h.unpaid > 0 ? "warn" : ""]);
  const g = member(v, h.guard);
  rows.push(["guard", g ? `${g.name} · skill ${g.skill}` : "nobody", g ? "" : "subtle"]);
  rows.push(["robbery", `${pct(q("rules.territory.house_robbery_chance", h.id), 1)}/day`, ""]);
  rows.push(["since", `day ${h.bought} · ${money(h.price)}`, "subtle"]);
  return rows;
}
export const knownWords = "The police know this house: it is the one the raid finds. Move the stock out and drop it.";

// The move (ui/houses.go): from the street or a house in a city to
// another, a product, a quantity. A place is "" (the street, game.Street)
// or a house id.
export const placeName = (v, id) => (id ? v.houses.find((h) => h.id === id)?.name || id : "the street");
// placeHolds is what a place holds and can hold. The street's capacity
// is World.StreetCapacity, worked back from the view: the city's free
// room less the houses' free room, over what the street holds (an
// over-full street reads over its capacity, as it is).
export function placeHolds(v, city, id) {
  if (id) {
    const h = v.houses.find((x) => x.id === id);
    return { units: houseUnits(h), capacity: h.capacity };
  }
  const street = Object.keys(v.you.stock?.[city] || {}).reduce((n, id) => n + held(v, city, "", id), 0),
    houseRoom = v.houses.filter((h) => h.city === city).reduce((n, h) => n + h.capacity - houseUnits(h), 0);
  return { units: street, capacity: street + (v.you.room?.[city] || 0) - houseRoom };
}
// places are the places in a city: every one holding something for
// from, every other for to.
export function places(v, city, from) {
  const out = [""].concat(v.houses.filter((h) => h.city === city).map((h) => h.id));
  return from === undefined ? out.filter((p) => placeHolds(v, city, p).units > 0) : out.filter((p) => p !== from);
}
// held is what a place holds of a product: a house's own, the street's
// the stash (`you.stock`, World.Stock) less the houses'.
export function held(v, city, place, id) {
  if (place) return v.houses.find((h) => h.id === place)?.stock?.[id] || 0;
  return (v.you.stock?.[city]?.[id] || 0) - v.houses.filter((h) => h.city === city).reduce((n, h) => n + (h.stock?.[id] || 0), 0);
}
// moveMax is what the move can take: what the source holds of the
// product, up to a house's room (the street takes what it holds).
export function moveMax(v, city, from, to, id) {
  const have = held(v, city, from, id);
  if (!to) return have;
  const h = v.houses.find((x) => x.id === to);
  return Math.min(have, h.capacity - houseUnits(h));
}
// moveRefusal is why nothing in a city can move: no house, or no stock.
export function moveRefusal(v, city) {
  if (!v.houses.some((h) => h.city === city)) return `Nothing to move to: no house in ${cityName(v, city)}. Lease one in Properties.`;
  if (!places(v, city).length) return `Nothing to move: no stock in ${cityName(v, city)}.`;
  return "";
}
export function moveNote(v, city, to, info) {
  const { units, capacity } = placeHolds(v, city, to);
  return `${placeName(v, to)} holds ${units} of ${capacity}. A move is free and instant; the units moved are exposure tonight, at ${times(info.moveHeat)} of a unit sold.`;
}
export const moveHeatLine = (q, city, id, n) => `+${q("rules.heat.move_heat", city, id, n).toFixed(1)} tonight for ${n} units`;
export const movedSaid = (v, q, city, from, to, id, n) => `Moved ${n} ${productName(v, id)} from ${placeName(v, from)} to ${placeName(v, to)}. The drive is +${q("rules.heat.move_heat", city, id, n).toFixed(1)} heat tonight.`;

// The guard (ui/houses.go): an enforcer for a house, or nobody. Each
// candidate with where they are now.
export function guardRows(v, h) {
  const rows = [{ id: 0, name: "Nobody", where: ["takes the guard off", "subtle"] }];
  for (const m of v.crew.filter((x) => x.role === "enforcer")) {
    const elsewhere = v.houses.find((x) => x.guard === m.id && x.id !== h.id),
      corner = v.cities.flatMap((c) => c.corners).find((c) => c.enforcer === m.id);
    rows.push({
      id: m.id,
      name: m.name,
      skill: m.skill,
      where: h.guard === m.id ? ["already here", "good"] : elsewhere ? [`in ${elsewhere.name}, will move`, "warn"] : corner ? [`on ${corner.name}, will move`, "warn"] : ["unposted", "subtle"],
    });
  }
  return rows;
}
export const guardAsk = (h) => `Who should guard ${h.name}? One enforcer, one job: the house or a corner.`;
export const noGuards = "Nothing to post: no enforcers. Hire one on Your people.";
export function guardSaid(v, q, h, id) {
  if (!id) return `Nobody is guarding ${h.name} now.`;
  return `${member(v, id).name} is inside ${h.name}: robbery ${pct(q("rules.territory.house_robbery_chance", h.id), 1)}/day.`;
}

// The drop (ui/houses.go): what walking away loses.
export function dropLines(h) {
  const n = houseUnits(h);
  return [`Walk away from ${h.name}?`, n > 0 ? `The ${plural(n, "unit")} in it go with it. Move them out first.` : "It is empty. The price is not refunded."];
}
export const droppedSaid = (h, units) => (units > 0 ? `Dropped ${h.name}: ${plural(units, "unit")} went with it.` : `Dropped ${h.name}.`);

// The assets (ui/assets.go): what each does, in the file's terms, and
// an owned one's status.
export function assetBlurb(v, a, info) {
  const t = info.assets?.[a.id || a.ID] || {},
    city = cityName(v, a.city || a.City);
  switch (t.effect) {
    case "supplier":
      return `The wholesaler in ${city} sells at ${pct(t.own_ratio)} of street, without limit, and a buy never nudges their price.`;
    case "port":
      return `Every boat through ${city} carries ${times(t.capacity_mul)} and clears customs for nothing.`;
    case "airstrip":
      return "The plane route is open: a day's flight, the priciest fare, and a risk only the task force's watch touches.";
    case "lab":
      return `A cook in ${city} is ${times(t.lab_mul)} the batch at quality ${Math.round(t.lab_quality)}, precursors at ${pct(t.lab_cost_mul)} of the cost.`;
    case "tunnel":
      return "The tunnel route is open: cheap, slow and nearly never seen, until it is found once.";
  }
  return "";
}
export function assetStatus(v, a) {
  if (a.frozen_until > v.day + 1) return [`shut, back in ${a.frozen_until - v.day - 1}d`, "warn"];
  return ["standing", "good"];
}
export function assetRows(v, q, a) {
  const o = q("rules.laundering.asset_offer", a.id),
    rows = [["city", cityName(v, a.city), ""], ["upkeep", `${money(a.upkeep)}/day clean`, ""]];
  if (o?.HeatFloor > 0) rows.push(["heat floor", `${Math.round(o.HeatFloor)} in every city while it stands`, ""]);
  rows.push(["bought", `day ${a.bought} · ${money(a.cost)} clean`, "subtle"]);
  return rows;
}

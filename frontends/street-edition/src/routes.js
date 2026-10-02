// The routes (#582), as the TUI's map pane and target dialog word them
// (ui/routes.go routeFacts, viewTarget, confirmTarget, routeBuys) and
// its driver picker (ui/life.go): the days and capacity at the dial, the
// fare and the odds of a seizure, the targets, what is on the road, the
// driver, and the target dialog's table and preview. Each function takes
// the view and `q`, the engine's query (session.call), and returns
// words, rows or numbers: it touches no DOM, so smoke.mjs checks them on
// a live run. A row is [label, text, tone].

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n || 0)).toLocaleString("en-US"),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  pct = (x) => Math.round(x * 100) + "%",
  // fare is a fare a unit: whole dollars as money prints them, cents
  // where the tree has cut one under a dollar (#119).
  fare = (x) => (x === Math.floor(x) ? money(x) : "$" + x.toFixed(2));

export const cityName = (v, id) => v.cities.find((c) => c.id === id)?.name || id;
export const productName = (v, id) => v.cities.flatMap((c) => c.products).find((p) => p.id === id)?.name || id;
const products = (v) => v.cities[0]?.products.map((p) => p.id) || [];

// ship is the shipping speed a route's dial carries (RouteDial.Ship):
// an idle route is costed at normal.
export const ship = (dial) => (dial === "slow" || dial === "fast" ? dial : "normal");

// stock and bound: what the stash at a city holds of a product, and what
// is on the road to it (World.Bound).
const stock = (v, city, id) => v.you.stock?.[city]?.[id] || 0;
export const bound = (v, city, id) => v.shipments.filter((s) => s.to === city && s.product === id).reduce((n, s) => n + s.units, 0);
const served = (v, city, id) => v.cities.find((c) => c.id === city)?.products.find((p) => p.id === id)?.served || 0;

// targetLine is a route's targets in one line, "3d (≈180) Weed · 400
// Coke", a days target with the units it means today; "" with none.
export function targetLine(v, q, r) {
  const parts = [];
  for (const id of products(v)) {
    if (r.days_target?.[id] > 0) parts.push(`${r.days_target[id]}d (≈${q("rules.logistics.target", r.id, id)}) ${productName(v, id)}`);
    else if (r.target?.[id] > 0) parts.push(`${r.target[id]} ${productName(v, id)}`);
  }
  return parts.join(" · ");
}

// onTheRoad is a route's shipments: "day 2 of 3 · 40 Weed".
export function onTheRoad(v, r) {
  return v.shipments
    .filter((s) => s.route === r.id)
    .map((s) => {
      const days = Math.max(1, s.arrives - s.sent),
        day = Math.min(days, Math.max(0, v.day - s.sent) + 1);
      return `day ${day} of ${days} · ${s.units} ${productName(v, s.product)}`;
    });
}

// driverLine is who drives a route (ui/life.go driverLine): nobody, a
// driver gone, one jailed or laid up, or one at the wheel with what
// their skill cuts off the risk. [text, tone].
export function driverLine(v, q, r) {
  if (!r.driver) return ["nobody", "subtle"];
  const m = v.crew.find((x) => x.id === r.driver);
  if (!m) return ["nobody: the driver is gone", "warn"];
  if (m.jailed) return [`${m.name} · ${m.bailed ? "out tomorrow" : `jailed ${m.jailed_until - v.day}d`}`, "warn"];
  if (m.wounded) return [`${m.name} · laid up ${m.wounded}d`, "warn"];
  return [`${m.name} · risk −${pct(q("rules.logistics.driver_cut", m.skill))}`, ""];
}

// seized is the odds a shipment is seized at the dial, as the file
// knows the route's risk: "~4%", or "?" where it does not.
export const seized = (q, r) => (r.risk_known ? "~" + pct(q("rules.logistics.risk_from", r.id, ship(r.dial), r.risk)) : "?");

// facts is the route's pane (routeFacts): the edge, the dial, a shut
// road, the skies for the plane, the days and capacity, the fare and the
// seizure, the targets, what is on the road and the driver. The idle
// reason is wash.routeIdle's.
export function facts(v, q, r) {
  const rows = [
    ["", `${cityName(v, r.from)} → ${cityName(v, r.to)} by ${r.mode}`, "subtle"],
    ["dial", r.dial, r.dial === "off" ? "subtle" : "gold"],
  ];
  // Shut by an incident (#44) tonight (World.RouteClosed): the view's
  // closed is today's, so a road that opens tonight says nothing.
  if (r.closed_until > v.day + 1) rows.push(["closed", plural(r.closed_until - v.day - 1, "night") + " to go", "warn"]);
  if (r.mode === "plane") {
    rows.push(v.law.watch_until > v.day + 1 ? ["watched", `the feds, ${plural(v.law.watch_until - v.day - 1, "night")} to go`, "warn"] : ["watched", "nobody: the sky is clear", "subtle"]);
  }
  rows.push(
    ["days", `${q("rules.logistics.days", r.id, ship(r.dial))} · capacity ${q("rules.logistics.capacity", r.id)}`, ""],
    ["fare", `${fare(q("rules.logistics.fare", r.id))}/u · seized ${seized(q, r)}`, ""],
  );
  const t = targetLine(v, q, r);
  if (!t) rows.push(["target", r.dial !== "off" ? "none: it sends nothing" : "none", r.dial !== "off" ? "warn" : "subtle"]);
  else rows.push(["target", t, ""]);
  onTheRoad(v, r).forEach((l, i) => rows.push([i ? "" : "on the road", l, "road"]));
  const [d, tone] = driverLine(v, q, r);
  rows.push(["driver", d, tone]);
  return rows;
}

// routeBuys is the target dialog's word on what a route buys (#496):
// whole lots off the wholesaler at the source while their door is open,
// out of the dirty cash over the till, or nothing.
export function routeBuys(v, q, r) {
  const sup = v.connects.find((c) => c.city === r.from && c.wholesale),
    from = cityName(v, r.from);
  if (!sup) return `It buys nothing: stock the ${from} stash yourself.`;
  if (sup.open) return `What the stash lacks it buys by the lot from ${sup.name}, out of the dirty cash over the ${money(q("rules.laundering.till"))} till.`;
  if (sup.locked) return `It buys nothing until ${sup.name} deals with you (${money(sup.unlock_cash)} moved): stock the ${from} stash yourself.`;
  return `It buys by the lot from ${sup.name} while they deal; today they do not: stock the ${from} stash yourself.`;
}

// targetIntro is the target dialog's first paragraph: what the route
// does every day, and what it buys.
export function targetIntro(v, q, r) {
  return `${r.name} keeps ${cityName(v, r.to)} stocked: every day it ships what is short of the target, up to ${q("rules.logistics.capacity", r.id)} units, out of the ${cityName(v, r.from)} stash, fares in dirty cash. ${routeBuys(v, q, r)}`;
}

// targetRows is the target dialog's table: each product's target (a
// days target with what it means today), what the far end holds, what
// is on the road to it and what sells there a day.
export function targetRows(v, q, r) {
  return products(v).map((id) => ({
    id,
    name: productName(v, id),
    target: r.days_target?.[id] > 0 ? `${r.days_target[id]}d (≈${q("rules.logistics.target", r.id, id)})` : r.target?.[id] > 0 ? String(r.target[id]) : "",
    there: stock(v, r.to, id),
    road: bound(v, r.to, id),
    sells: Math.round(served(v, r.to, id)),
  }));
}

// kindLine is what a target of either kind keeps: units kept whatever
// sells, or days of what the corners at the far end sell, which the road
// sizes again every morning.
export function kindLine(v, r, id, days) {
  return days
    ? `Days of ${productName(v, id)}'s demand in ${cityName(v, r.to)}, ~${Math.round(served(v, r.to, id))}/day today: the target follows the corners you hold there.`
    : `Units of ${productName(v, id)} kept in ${cityName(v, r.to)}, whatever sells there.`;
}

// targetMax is the most a target may be: the far end's stash, or the
// days of its demand it holds.
export function targetMax(v, r, id, days) {
  const room = (v.you.room?.[r.to] || 0) + Object.values(v.you.stock?.[r.to] || {}).reduce((n, x) => n + x, 0);
  if (!days) return room;
  const d = served(v, r.to, id);
  return d > 0 ? Math.floor(room / d) : 0;
}

// preview is the target dialog's rows for a number typed: the units a
// days target means this morning, and the shortfall it sends and where
// it comes from, with the fares.
export function preview(v, q, r, id, n, days) {
  if (!(n > 0)) return [];
  const rows = [];
  let units = n;
  if (days) {
    units = q("rules.logistics.days_target", r.id, id, n);
    rows.push(["today", `${n}d ≈ ${plural(units, "unit")}: ~${Math.round(served(v, r.to, id))}/day on your corners in ${cityName(v, r.to)}`, "subtle"]);
  }
  if (v.cities.find((c) => c.id === r.from)?.products.some((p) => p.id === id)) {
    const short = Math.max(0, units - stock(v, r.to, id) - bound(v, r.to, id)),
      stashed = Math.min(short, stock(v, r.from, id)),
      rest = short - stashed,
      sup = v.connects.find((c) => c.city === r.from && c.wholesale);
    let how = `${stashed} from the ${cityName(v, r.from)} stash`;
    if (rest > 0) how += sup && sup.open && sup.prices[id] > 0 ? `, ${rest} by the lot from ${sup.name} ~${money(Math.floor(rest * sup.prices[id]))}` : `, ${rest} with nothing to ship`;
    rows.push(["short", `${short}: ${how} + ${money(Math.ceil(short * q("rules.logistics.fare", r.id)))} fares`, "subtle"]);
  }
  return rows;
}

// readTarget reads the target's field: blank or 0 is none, a whole
// number up to targetMax. {n} or {err}.
export function readTarget(raw) {
  const s = String(raw ?? "").trim();
  if (s === "") return { n: 0 };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n < 0) return { err: "Enter a whole number, or nothing for none." };
  return { n };
}

// targetSaid is the answer to a target set (confirmTarget).
export function targetSaid(v, q, r, id, n, days) {
  let kept = days ? `${plural(n, "day")} of ${productName(v, id)}'s demand` : `${n} ${productName(v, id)}`;
  if (days && n > 0) kept += ` (≈${q("rules.logistics.target", r.id, id)} today)`;
  if (n === 0) return `${r.name}: no target for ${productName(v, id)}; the route leaves it alone.`;
  if (r.dial === "off") return `${r.name} keeps ${cityName(v, r.to)} at ${kept} once its dial is on.`;
  return `${r.name} keeps ${cityName(v, r.to)} at ${kept}: it sends the shortfall every day.`;
}

// drivers are the driver picker's rows: every driver on the payroll.
// driverSaid is the answer to a driver put on the road, or taken off.
export const drivers = (v) => v.crew.filter((m) => m.role === "driver");
export const noDrivers = "Nobody to put on the road: no drivers. Hire one on Your people.";
export function driverSaid(v, q, r, id) {
  const m = v.crew.find((x) => x.id === id);
  if (!m) return `Nobody drives ${r.name} now.`;
  return `${m.name} drives ${r.name} from tomorrow: risk −${pct(q("rules.logistics.driver_cut", m.skill))} a day on the road, and jailed if a shipment is seized.`;
}

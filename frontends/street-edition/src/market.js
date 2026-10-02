// The connects and the market pane (#582), as the TUI's market screen
// words them: the SUPPLIERS block and the connect's pane
// (ui/suppliers.go), the buy on a connect's book (ui/dialogs.go
// buyTermsRows, creditTerms, confirmBuy) and the product's pane with
// ELSEWHERE and NOTES (ui/market.go marketDetails). Each function takes
// the view and `q`, the engine's query (session.call), and returns
// words, rows or numbers: it touches no DOM, so smoke.mjs checks them
// on a live run. A row is [label, text, tone], tone one of "", "subtle",
// "warn", "bad", "good".

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n || 0)).toLocaleString("en-US"),
  price = (n) => "$" + (n || 0).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  times = (x, sig = 2) => "×" + Number((x || 0).toFixed(sig)).toString(),
  pctText = (f) => (f < 10 && f > -10 ? f.toFixed(1) : f.toFixed(0)) + "%",
  signedPct = (f) => (f >= 0 ? "+" : "") + Math.round(f) + "%",
  // short is format.Cash: whole dollars under $10K, then $12K, $1.5M.
  short = (n) => {
    if (n > -10000 && n < 10000) return money(n);
    const sign = n < 0 ? "-" : "",
      units = ["K", "M", "B", "T"];
    let x = Math.abs(n) / 1000,
      i = 0;
    while (i < units.length - 1 && Math.round(x) >= 1000) {
      x /= 1000;
      i++;
    }
    return `${sign}$${x < 10 ? x.toFixed(1) : x.toFixed(0)}${units[i]}`;
  };

export const cityName = (v, id) => v.cities.find((c) => c.id === id)?.name || id;
export const productName = (v, id) => v.cities.flatMap((c) => c.products).find((p) => p.id === id)?.name || id;
const productIn = (v, city, id) => v.cities.find((c) => c.id === city)?.products.find((p) => p.id === id);

// streetConnect is the street connect in a city (World.StreetSupplier):
// the first who is not the wholesaler, whose word opens the others.
const streetConnect = (v, city) => v.connects.find((x) => x.city === city && !x.wholesale);

// connectsIn are the connects the market lists for a city (the TUI's
// supplierRows): every one seeded there, in order, the locked ones too
// once the peak cash they want is moved, so they say what they wait for.
export const connectsIn = (v, city) => v.connects.filter((c) => c.city === city && v.you.peak_cash >= (c.unlock_cash || 0));

// credit is what a connect's book will still run you to today
// (Supplier.Credit): nothing without credit terms.
export const credit = (c) => (c.credit_days > 0 ? Math.max(0, (c.limit || 0) - (c.debt || 0)) : 0);

// available is whether a connect sells you a product today
// (World.Available): the view carries their price only then.
export const available = (c, id) => c.prices[id] != null;

// dueWord is a due day in words: today, tomorrow, in N days, N days late.
export function dueWord(days) {
  if (days < 0) return plural(-days, "day") + " late";
  if (days === 0) return "today";
  if (days === 1) return "tomorrow";
  return "in " + plural(days, "day");
}

// creditDue is when credit taken today is due, as the connect's book
// works it (#518): "7d to pay" on a clean book, "due d12 with the debt"
// on one that owes already.
export const creditDue = (c) => (c.debt > 0 ? `due d${c.debt_due} with the debt` : `${c.credit_days}d to pay`);

// temperShort is a temper in three or four words; temperWords what it
// does about a missed payment.
export function temperShort(t) {
  return { patient: "extends once", sharp: "freezes, fees", connected: "sends somebody" }[t] || "";
}
export function temperWords(t) {
  return (
    {
      patient: "they let it ride once, then stop taking your calls",
      sharp: "they stop taking your calls and add a fee",
      connected: "they send somebody for your muscle",
    }[t] || "they remember"
  );
}

// bandWord is where a relationship band stands, in a word; bandTone its
// colour: the floor red, under neutral a warning, over it good.
export function bandWord(band, bands) {
  const n = Math.floor(bands / 2);
  if (band === 0) return "the floor";
  if (band < n) return "cool";
  if (band === n) return "neutral";
  if (band === bands - 1) return "the top";
  return "warm";
}
export function bandTone(band, bands) {
  const n = Math.floor(bands / 2);
  if (band === 0) return "bad";
  if (band < n) return "warn";
  if (band > n) return "good";
  return "subtle";
}

// note is the tail of a connect's row (supplierNote): frozen, waiting
// for a word, what you owe, a tip today, out for the day, the credit
// they give, or cash only. [text, tone].
export function note(v, c) {
  if (c.frozen_until > v.day) return [`not taking calls, ${c.frozen_until - v.day}d`, "bad"];
  if (c.locked) return ["won't deal yet", "subtle"];
  if (c.debt > 0) return [`owe ${short(c.debt)} by d${c.debt_due}`, c.debt_due <= v.day + 1 ? "bad" : "warn"];
  if (c.warned) return ["has a tip: see the report", "good"];
  if (c.cap === 0) return ["nothing left today", "warn"];
  if (c.credit_days > 0) return ["credit " + short(credit(c)), "subtle"];
  return ["cash only", "subtle"];
}

// row is a connect's SUPPLIERS line for a product: their price for it
// (null where they do not sell it today), the lot, what they have left
// today and the relationship.
export const row = (c, id) => ({ unit: available(c, id) ? c.prices[id] : null, lot: c.lot, left: c.cap, rel: Math.round(c.rel) });

// deals is what a connect deals in, by name: "everything sold here" with
// no list of their own.
export const deals = (v, c) => (c.products?.length ? c.products.map((id) => productName(v, id)).join(", ") : "everything sold here");

// pane is the connect's pane (supplierSections): who they are and where
// they stand with you, what they sell you today, their credit and your
// debt, the door, and how you buy from them. Rows [label, text, tone].
export function pane(v, q, c) {
  const bands = q("rules.market.bands"),
    band = q("rules.market.band", c.rel),
    tun = q("rules.market.suppliers_tuning"),
    rows = [
      ["temper", c.temper, ""],
      ["", temperShort(c.temper), "subtle"],
      ["rel", `${Math.round(c.rel)} · ${bandWord(band, bands)}`, bandTone(band, bands)],
      ["price", `~${Math.round(q("rules.market.supplier_ratio", c.id) * 100)}% of street`, ""],
    ];
  if (band < bands - 1) rows.push(["", `~${Math.round(q("rules.market.ratio_at", c.id, band + 1) * 100)}% at rel ${Math.round((band + 1) * tun.BandWidth)}`, "subtle"]);
  rows.push(["lot", String(c.lot), ""]);
  if (c.small_lot > 1) rows.push(["", `under it ${times(c.small_lot)} a unit`, "subtle"]);
  rows.push(["today", `${c.cap.toLocaleString("en-US")} of ${c.day_cap.toLocaleString("en-US")} left`, ""]);
  const graded = Object.entries(c.quality || {}).map(([id, qy]) => `${productName(v, id)} ${Math.round(qy)}`);
  if (graded.length) rows.push(["quality", graded.join(", "), ""]);
  if (c.products?.length) rows.push(["deals in", deals(v, c), ""]);
  if (c.credit_days <= 0) rows.push(["credit", "none", "subtle"]);
  else rows.push(["credit", `${short(credit(c))} of ${short(c.limit || 0)}`, ""], ["", `${times(c.credit_ratio)}/u, ${creditDue(c)}`, "subtle"]);
  if (c.debt > 0) {
    rows.push(["debt", `${money(c.debt)} due d${c.debt_due}, ${dueWord(c.debt_due - v.day)}`, c.debt_due <= v.day + 1 ? "bad" : "warn"]);
    if (c.extended) rows.push(["", "extended once already", "warn"]);
  }
  const street = streetConnect(v, c.city);
  if (c.frozen_until > v.day) rows.push(["status", `no calls for ${plural(c.frozen_until - v.day, "day")}`, "bad"]);
  else if (c.locked && v.you.peak_cash < c.unlock_cash) rows.push(["status", `wants ${short(c.unlock_cash)} moved`, "subtle"]);
  else if (c.locked && street) rows.push(["status", `needs ${street.name} at ${Math.round(c.unlock_rel)} (${Math.round(street.rel)})`, "subtle"]);
  else if (c.warned) rows.push(["status", "tipped you off today", "good"]);
  if (c.late > 0) rows.push(["late", plural(c.late, "payment") + " missed", ""]);
  return rows;
}

// paneRules is the pane's RULES paragraph: what moves the relationship.
export function paneRules(q, c) {
  const t = q("rules.market.suppliers_tuning");
  return `Every lot bought is +${Number(t.RelPerLot.toPrecision(2))} rel; a debt cleared on its day +${Math.round(t.RelPaid)}; a late one -${Math.round(t.RelLate)}, and ${temperWords(c.temper)}. A bust that takes their product costs rel; under ${Math.round(t.FreezeRel)} they stop taking calls. Left alone ${plural(t.QuietDays, "day")}, they forget you.`;
}

// blurb is the buy dialog's note on a connect (connectBlurb): their
// temper, what they deal in, and the door if it is shut. [text, tone].
export function blurb(v, c) {
  const out = [[`${c.name}: ${c.temper}, deals in ${deals(v, c)} by the ${c.lot}.`, "subtle"]],
    street = streetConnect(v, c.city);
  if (c.locked && v.you.peak_cash < c.unlock_cash) out.push([`They deal with people who have moved ${short(c.unlock_cash)}.`, "warn"]);
  else if (c.locked && street) out.push([`They want a word from ${street.name} first: rel ${Math.round(street.rel)}, they need ${Math.round(c.unlock_rel)}.`, "warn"]);
  else if (c.frozen_until > v.day) out.push([`Not taking your calls for ${plural(c.frozen_until - v.day, "day")}.`, "bad"]);
  return out;
}

// howToBuy is the pane's last line: buy from them where you stand,
// through the lieutenant who runs their city, or go there.
export function howToBuy(v, q, c) {
  const lt = v.crew.find((m) => m.role === "lieutenant" && m.city === c.city && !m.jailed);
  if (c.city === v.you.city && c.open) return "Buy from them on the product's row.";
  if (c.city !== v.you.city && lt && c.open) return `${lt.name} buys from them for you at ${times(q("rules.market.markup"))}.`;
  if (c.city !== v.you.city) return `Go to ${cityName(v, c.city)} to buy.`;
  return "";
}

// whyNobodySells is the refusal when no connect in a city deals with you
// today: nobody there, frozen, out for the day, or locked.
export function whyNobodySells(v, city) {
  const cs = v.connects.filter((c) => c.city === city && !c.wholesale);
  if (!cs.length) return `Nobody sells in ${cityName(v, city)}.`;
  const frozen = cs.find((c) => c.frozen_until > v.day);
  if (frozen) return `${frozen.name} is not taking your calls for ${plural(frozen.frozen_until - v.day, "day")}.`;
  const out = cs.find((c) => !c.locked && c.cap === 0);
  if (out) return `${out.name} has nothing left today.`;
  return `Nobody in ${cityName(v, city)} will deal with you yet.`;
}

// sellers are the connects that sell you a product where you stand
// today, cheapest first: the buy's choices.
export const sellers = (v, id) =>
  v.connects.filter((c) => c.city === v.you.city && c.open && available(c, id)).sort((a, b) => a.prices[id] - b.prices[id]);

// unitAt is the price a unit of a buy from a connect where you stand
// (Supplier.UnitAt): the small-lot premium under the lot, the credit
// premium on the book. quote is the whole (Supplier.QuoteAt).
export function unitAt(c, id, qty, onCredit) {
  let unit = c.prices[id] || 0;
  if (qty < c.lot && c.small_lot > 1) unit *= c.small_lot;
  if (onCredit && c.credit_ratio > 0) unit *= c.credit_ratio;
  return unit;
}
export const quote = (c, id, qty, onCredit) => Math.ceil(unitAt(c, id, qty, onCredit) * qty);

// creditTerms is a buy on the book as the debt will read (#502):
// "$2,985: $248.71 a unit, ×1.15 the cash $216.27"; then the due day and
// what is left of the book, and what you owe already or what a missed
// day does. [text, tone] lines.
export function creditTerms(v, c, id, qty) {
  const due = c.debt > 0 ? c.debt_due : v.day + c.credit_days,
    out = [
      [`${money(quote(c, id, qty, true))}: ${price(unitAt(c, id, qty, true))} a unit, ${times(c.credit_ratio)} the cash ${price(unitAt(c, id, qty, false))}`, ""],
      [`Due day ${due} · ${short(credit(c))} of the book left`, ""],
    ];
  out.push(c.debt > 0 ? [`You owe them ${money(c.debt)} already, due day ${c.debt_due}.`, "warn"] : [`Miss the day and ${temperWords(c.temper)}.`, "subtle"]);
  return out;
}

// cashShort is the buy's note where the cash covers fewer than asked and
// their book covers more (#467): "Cash covers 3 of 10: put it on Cass's
// book."; "" otherwise.
export function cashShort(c, qty, cashMax, bookMax) {
  return qty > cashMax && bookMax > cashMax ? `Cash covers ${cashMax} of ${qty}: put it on ${c.name}'s book.` : "";
}

// boughtSaid is the buy's answer, the TUI's.
export function boughtSaid(v, c, id, p) {
  const from = c.name + (p.Lieutenant ? " through " + p.Lieutenant : "");
  if (p.Credit) return `Bought ${p.Qty} ${productName(v, id)} from ${from} on credit: ${money(p.Cost)} on the book, due day ${v.connects.find((x) => x.id === c.id)?.debt_due || c.debt_due}.`;
  return `Bought ${p.Qty} ${productName(v, id)} from ${from} for ${money(p.Cost)}.`;
}

// debtLine is the dashboard's fact on what you owe the connects: the
// total and the nearest day; "" with no debt. [text, tone].
export function debtLine(v) {
  const owing = v.connects.filter((c) => c.debt > 0);
  if (!owing.length) return null;
  const due = Math.min(...owing.map((c) => c.debt_due));
  return [`Owe ${short(owing.reduce((n, c) => n + c.debt, 0))}, due ${dueWord(due - v.day)}`, due <= v.day + 1 ? "bad" : "warn"];
}

// productPane is the market's pane on a product in a city
// (marketDetails): the range, the glut, the margin or that the supplier
// here does not sell it, the demand your corners serve and a standard
// corner's, the lot's quality and the connect's, what is cooking, and a
// shock or a slump while one runs. Rows [label, text, tone].
export function productPane(v, q, city, id) {
  const p = productIn(v, city, id);
  if (!p) return [];
  const c = v.cities.find((x) => x.id === city),
    f = p.facts,
    street = v.you.street_quality,
    rows = [
      ["range 30d", `${price(f.lo)} – ${price(f.hi)}`, ""],
      ["glut", pctText(p.glut * 100), ""],
      p.no_supply ? ["supplier", "not sold here", "warn"] : ["margin", signedPct(f.margin) + " over supplier", ""],
      ["demand", `~${Math.round(p.served)}/day on ${plural(c.worked, "corner")}`, ""],
      ["", `~${Math.round(p.demand)} per standard`, "subtle"],
    ];
  const held = v.you.stock?.[city]?.[id] || 0,
    lot = v.you.quality?.[city]?.[id];
  if (held > 0 && lot != null && lot !== street) rows.push(["quality", `${Math.round(lot)}, sells at ${times(q("rules.market.quality_mul", lot))}`, lot < street ? "warn" : ""]);
  const best = v.connects.filter((x) => x.city === city && x.open && available(x, id)).sort((a, b) => a.prices[id] - b.prices[id])[0];
  if (best && best.quality?.[id] != null) rows.push(["", `${best.name} sells it at ${Math.round(best.quality[id])}`, "subtle"]);
  const cooking = (v.cooks || []).filter((k) => k.city === city && k.product === id).reduce((n, k) => n + k.units, 0);
  if (cooking > 0) rows.push(["cooking", `${cooking} on the way`, ""]);
  if (p.shock_days > 0) rows.push([p.slump ? "slump" : "shock", `${times(p.shock)}, ${plural(p.shock_days, "day")} more`, p.slump ? "warn" : "good"]);
  return rows;
}

// elsewhere is the pane's ELSEWHERE (the other city's price is what a
// route is worth): each other city's street and supplier price, your
// stash there, and what is on the road with the days it has left.
export function elsewhere(v, city, id) {
  const rows = [];
  for (const c of v.cities) {
    if (c.id === city) continue;
    const o = c.products.find((p) => p.id === id);
    if (o) rows.push([c.name, `${price(o.price)} · sup ${price(o.supplier_price)}`, ""]);
    const n = v.you.stock?.[c.id]?.[id] || 0;
    if (n > 0) rows.push(["stash", `${n} in ${c.name}`, ""]);
  }
  for (const s of v.shipments) if (s.product === id) rows.push(["road", `${s.units} → ${cityName(v, s.to)}, ${Math.max(0, s.arrives - v.day)}d`, "road"]);
  return rows;
}

// notes is the pane's NOTES: a product the supplier here does not sell,
// where you are when the market shown is not, and the wholesaler who
// feeds the routes. [text, tone].
export function notes(v, q, city, id) {
  const p = productIn(v, city, id),
    out = [],
    lt = v.crew.find((m) => m.role === "lieutenant" && m.city === city && !m.jailed),
    here = cityName(v, v.you.city);
  if (p?.no_supply) out.push(["Not sold here: it comes in by the road (the transport routes) or in your pockets.", "warn"]);
  if (city !== v.you.city && lt) out.push([`You are in ${here}: ${lt.name} buys here for you at ${times(q("rules.market.markup"))} the connect's price, and keeps the stash stocked where you set no contract. Runners sell what is stashed here.`, "subtle"]);
  else if (city !== v.you.city) out.push([`You are in ${here}: the supplier here sells to you there, not here. Runners sell what is stashed here.`, "subtle"]);
  const o = v.connects.find((c) => c.city === city && c.wholesale);
  if (o) out.push(o.locked ? [`${o.name} sells lots of ${o.lot} to the routes once you have moved ${short(o.unlock_cash)}.`, "subtle"] : [`Wholesale: ${o.name}'s lots of ${o.lot} feed the routes out of here, run from Transport routes.`, "good"]);
  return out;
}

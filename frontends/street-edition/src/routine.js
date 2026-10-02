// The routine (#556): standing orders, supply contracts and the cart,
// as the TUI's market pane, sell and buy dialogs and cart word them
// (ui/market.go standingRows and contractRows, ui/dialogs.go, ui/cart.go,
// ui/fast.go supplyShortWords). Each function takes the view and `q`,
// the engine's query (session.call), and returns words, rows or
// numbers: it touches no DOM, so smoke.mjs checks them on a live run.
// The cart is read off the view (`orders`, `standing`, `supply`), never
// a memo of the page's.

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n || 0)).toLocaleString("en-US"),
  price = (n) => "$" + (n || 0).toLocaleString("en-US", { minimumFractionDigits: 2, maximumFractionDigits: 2 }),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  times = (x) => "×" + (Math.round(x * 100) / 100).toString(),
  pct = (x) => Math.round(x * 100) + "%";

// ALL is the quantity a standing order kept at the whole stash is
// placed with (game.AllUnits, #503).
export const ALL = -1;

const key = (city, id) => (o) => o.city === city && o.product === id;
export const productName = (v, id) => v.cities.flatMap((c) => c.products).find((p) => p.id === id)?.name || id;
export const cityName = (v, id) => v.cities.find((c) => c.id === id)?.name || id;

// order is today's sell order for a product in a city, standing the
// standing order of yours, contract the supply contract standing there
// with own false for a lieutenant's (#174), which fills only where you
// set none.
export const order = (v, city, id) => (v.orders || []).find(key(city, id));
export const standing = (v, city, id) => (v.standing || []).find(key(city, id));
export function contract(v, city, id) {
  const all = (v.supply || []).filter(key(city, id)),
    c = all.find((x) => !x.lieutenant) || all[0];
  return c ? { ...c, own: !c.lieutenant } : null;
}

// standingQty is a standing order's units as a line says them: "all"
// for one kept at the whole stash (#503). dialShort is the dial short,
// so the row fits.
export const standingQty = (o) => (o.all ? "all" : String(o.qty));
export const dialShort = (d) => (d === "aggressive" ? "aggr." : d);

// stock is what is stashed, the street and the houses (World.Stock);
// landing is what lands in the city tonight
// after the sales (World.Landing: the road's shipments, the chemist's
// batches); road is what of a contract's shortfall is on the road
// (World.Road, #503).
export const stock = (v, city, id) => v.you.stock?.[city]?.[id] || 0;
export function landing(v, city, id) {
  const day = v.day + 1,
    closed = (r) => (v.routes || []).find((x) => x.id === r)?.closed_until > day;
  return (
    (v.shipments || []).filter((s) => s.to === city && s.product === id && s.arrives <= day && !closed(s.route)).reduce((n, s) => n + s.units, 0) +
    (v.cooks || []).filter((k) => k.city === city && k.product === id && k.ready <= day).reduce((n, k) => n + k.units, 0)
  );
}
export function road(v, city, id) {
  const c = contract(v, city, id);
  if (!c) return 0;
  const bound = (v.shipments || []).filter((s) => s.to === city && s.product === id).reduce((n, s) => n + s.units, 0);
  return Math.min(bound, Math.max(0, c.units - stock(v, city, id)));
}
// productIn is a city's product from the view.
const productIn = (v, city, id) => v.cities.find((c) => c.id === city)?.products.find((p) => p.id === id);
// noSupply is why a contract cannot be set for a product in a city
// (game.ErrNotSupplied): nobody there sells it.
export const noSupply = (v, city, id) => (productIn(v, city, id)?.no_supply ? `Nobody in ${cityName(v, city)} sells ${productName(v, id)}: no contract can keep it.` : "");

// sellable is what an order may be for: the stash and what the contract
// buys in the morning by the market sim's plan. standable is what a
// standing order may be for (#503): that and what lands tonight. keepMax
// is the most a contract may keep: the stash's room for the product.
export const sellable = (v, q, city, id) => stock(v, city, id) + q("rules.market.due", city, id);
export const standable = (v, q, city, id) => sellable(v, q, city, id) + landing(v, city, id);
export const keepMax = (v, city, id) => Math.max(1, (v.you.room?.[city] || 0) + stock(v, city, id));

// standingRow is the market pane's row on a product's standing order
// (#114): "120 aggr. · cut 5%"; "" with none.
export function standingRow(v, q, city, id) {
  const o = standing(v, city, id);
  return o ? `${standingQty(o)} ${dialShort(o.dial)} · cut ${pct(q("rules.market.cut"))}` : "";
}

// contractRows is the pane's rows on a product's supply contract
// (#113): the level ("(lt)" for the lieutenant's), what it brings in the
// morning and at what, and what is held on the road (#503) with the
// night it leaves empty (#524). Each [text, tone]; none with none.
export function contractRows(v, q, city, id) {
  const c = contract(v, city, id);
  if (!c) return [];
  const rows = [[c.own ? `keep at ${c.units}` : `keep at ${c.units} (lt)`, c.own ? "gold" : ""]],
    due = q("rules.market.due", city, id);
  if (due > 0) rows.push([`brings ${due} in the morning at ${price(q("rules.market.supply_price", city, id))}`, "subtle"]);
  const r = road(v, city, id);
  if (r > 0) {
    rows.push([`holding: ${r} on the road`, "warn"]);
    if (stock(v, city, id) + due === 0) rows.push(["it lands after the sales: none to sell tonight", "warn"]);
  }
  return rows;
}

// editing is the dialog's warning when what is typed replaces an order
// that stands (#443): the standing order, or the contract. The TUI's
// "pick once" is the market row's Sell or Buy here.
export function editingStanding(v, city, id) {
  const o = standing(v, city, id);
  return o ? `Editing the standing order (${standingQty(o)} ${o.dial}); the market's Sell sells tonight only.` : "";
}
export function editingContract(v, city, id) {
  const c = contract(v, city, id);
  return c && c.own ? `Editing the contract (keep ${c.units}); the market's Buy buys just once.` : "";
}
// keptLine is the buy's note on a product kept by contract (#500): the
// buy leaves it alone.
export function keptLine(v, city, id) {
  const c = contract(v, city, id);
  return c && c.own ? `Kept at ${c.units} by contract; this buys once and leaves it.` : "";
}
// landingLine is the sale's note on what lands tonight (#503).
export function landingLine(v, q, city, id) {
  const land = landing(v, city, id);
  return land > 0 ? `${land} land tonight after the sales: a standing order may be sized for them (up to ${standable(v, q, city, id)}).` : "";
}
// dueLine is the sale's note on what the contract buys before tonight.
export function dueLine(v, q, city, id) {
  const due = q("rules.market.due", city, id);
  return due > 0 ? `${stock(v, city, id)} stashed and ${due} the contract buys before tonight's sales.` : "";
}

// contractTerms is what a contract is, the TUI's keep at line.
export function contractTerms(q) {
  return `Topped up at the end of each day, before the night's sales, at ${times(q("rules.market.markup"))} the supplier's price. It fills as far as the room and the cash allow.`;
}

// contractRoom is the warning a contract set at level gets when the
// city cannot hold it (#524), "" when it can. Where you stand the TUI
// counts the room without your carry; the view has the room as it is,
// so this edition says that only for a city away.
export function contractRoom(v, city, id, level) {
  if (!(level > 0) || city === v.you.city) return "";
  const room = (v.you.room?.[city] || 0) + stock(v, city, id);
  return room < level ? `The stash in ${cityName(v, city)} has room for ${Math.max(0, room)} of it: the contract fills only that far.` : "";
}

// contractBringsNone is why the contract standing for a product brings
// nothing before tonight's sales (#467, #524), as the market sim's plan
// finds it; "" with no contract, or one that brings some or has nothing
// to bring.
export function contractBringsNone(v, q, city, id) {
  const c = contract(v, city, id);
  if (!c || q("rules.market.due", city, id) > 0 || c.units <= stock(v, city, id) + road(v, city, id)) return "";
  const why = q("rules.market.due_short", city, id),
    free = v.you.room?.[city] || 0;
  if (why === "room" || (why === "" && free <= 0)) return free <= 0 ? "the stash there is full" : "the stash there has no room left once the contracts before it have bought";
  if (why === "supplier") return "nobody there has any left today";
  return `it buys for cash, and ${money(v.you.dirty_cash)} dirty does not cover one`;
}

// sellRefusal is why a product cannot be sold here (#467): none of it,
// and the contract's reason where it brings none.
export function sellRefusal(v, q, city, id) {
  if (sellable(v, q, city, id) > 0) return "";
  const why = contractBringsNone(v, q, city, id);
  return why ? `You have none of that here, and the contract brings none tonight: ${why}.` : "You have none of that here.";
}

// supplyShortWords is a contract's shortfall in the fast-forward's and
// the report's words (ui/fast.go).
export function supplyShortWords(why) {
  switch (why) {
    case "road":
      return "waiting on what is on the road";
    case "room":
      return "short of room";
    case "supplier":
      return "short: nobody there sells it today";
  }
  return "short of cash";
}

// readStanding reads the standing order's field (#503): blank is the
// whole stash every night (ALL), a number up to what may stand.
// {qty} or {err}.
export function readStanding(v, q, city, id, raw) {
  const s = String(raw ?? "").trim(),
    most = standable(v, q, city, id);
  if (s === "") return most > 0 ? { qty: ALL } : { err: sellRefusal(v, q, city, id) };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n <= 0) return { err: "Enter a whole number above zero." };
  if (n > most) return { err: `Only ${most} ${productName(v, id)} in ${cityName(v, city)}.` };
  return { qty: n };
}

// readKeep reads the contract's level: a number up to what the stash
// holds of it (the TUI's keepMax), blank the most.
export function readKeep(v, city, id, raw) {
  const s = String(raw ?? "").trim(),
    most = keepMax(v, city, id);
  if (s === "") return { units: most };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n <= 0) return { err: "Enter a whole number above zero." };
  if (n > most) return { err: `The stash in ${cityName(v, city)} holds ${most} of it; the level is now ${most}.`, units: most };
  return { units: n };
}

// The game's answers, in the TUI's status-bar words.
export function standingSaid(v, q, city, id, qty, dial) {
  const what = qty === ALL ? `all the ${productName(v, id)}` : `${qty} ${productName(v, id)}`;
  return `Standing: ${what} in ${cityName(v, city)}, ${dial}, every night until you cancel it; the crew keep ${pct(q("rules.market.cut"))}.`;
}
export function keepSaid(v, q, city, id, units) {
  const room = contractRoom(v, city, id, units);
  return `Keeping ${units} ${productName(v, id)} in ${cityName(v, city)}: topped up at the end of each day, before the night's sales, at ${times(q("rules.market.markup"))} the supplier's price.${room ? " " + room : ""}`;
}
export const clearedSaid = (v, city, id, units) => `Contract cleared: ${productName(v, id)} in ${cityName(v, city)} is no longer kept at ${units}.`;
export const cancelledSaid = "Standing order cancelled.";

// estimate is what an order is expected to move and take (the TUI's
// orderEstimate): a standing order sells at most what may be sold, and
// its take is after the crew's cut.
export function estimate(v, q, o, isStanding) {
  const p = productIn(v, o.city, o.product);
  if (!p) return { units: 0, take: 0 };
  let qty = o.qty,
    cut = 0;
  if (isStanding) {
    const most = sellable(v, q, o.city, o.product);
    qty = Math.min(o.all ? most : qty, most);
    cut = q("rules.market.cut");
  }
  const units = Math.min(qty, q("rules.market.capacity", o.city, o.product, o.dial));
  return { units, take: Math.floor(units * p.price * q("rules.market.dial", o.dial).Price * (1 - cut)) };
}

// cart is the day's sell orders and the contracts, read off the view
// (the TUI's cartModalLines, ui/cart.go): each order queued, each
// standing order of yours that sells tonight where you placed no order
// of the day (never on a quiet day: nothing sells), then each contract
// of yours as a keep line, in city and ladder order. A line is {kind:
// "sell" | "standing" | "keep", city, product, qty, dial, take}.
export function cart(v, q) {
  const lines = [];
  for (const c of v.cities)
    for (const p of c.products) {
      let o = order(v, c.id, p.id),
        kind = "sell";
      if (!o && !v.you.lie_low) {
        o = standing(v, c.id, p.id);
        kind = "standing";
      }
      if (!o) continue;
      const e = estimate(v, q, o, kind === "standing");
      lines.push({ kind, city: c.id, product: p.id, qty: kind === "standing" && o.all ? sellable(v, q, c.id, p.id) : o.qty, all: !!o.all, dial: o.dial, take: e.take });
    }
  for (const c of v.cities)
    for (const p of c.products) {
      const k = contract(v, c.id, p.id);
      if (k && k.own) lines.push({ kind: "keep", city: c.id, product: p.id, qty: k.units, due: q("rules.market.due", c.id, p.id) });
    }
  return lines;
}

// cartLine is one line of the cart in words, the TUI pane's CART rows:
// "Sell 10 Weed, normal, ~$400", "Standing all Weed (12), aggr., ~$350",
// "Keep 40 Weed, ~12 tonight"; the city named where it is not here.
export function cartLine(v, l) {
  const what = `${l.qty} ${productName(v, l.product)}`,
    where = l.city === v.you.city ? "" : ` in ${cityName(v, l.city)}`;
  switch (l.kind) {
    case "keep":
      return `Keep ${what}${where} · ~${l.due} tonight`;
    case "standing":
      return `Standing ${l.all ? `all ${productName(v, l.product)} (${l.qty})` : what}${where} · ${dialShort(l.dial)} · ~${money(l.take)}`;
  }
  return `Sell ${what}${where} · ${dialShort(l.dial)} · ~${money(l.take)}`;
}

// cartTotals is the cart's bottom line, the TUI's: "Selling 2 lines
// (1 standing), ~$1,200 · 1 contract kept"; "" for an empty cart.
export function cartTotals(lines) {
  const sells = lines.filter((l) => l.kind !== "keep"),
    standingN = sells.filter((l) => l.kind === "standing").length,
    keeps = lines.length - sells.length,
    parts = [];
  if (sells.length) parts.push(`Selling ${plural(sells.length, "line")}${standingN ? ` (${standingN} standing)` : ""}, ~${money(sells.reduce((n, l) => n + l.take, 0))}`);
  if (keeps) parts.push(`${plural(keeps, "contract")} kept`);
  return parts.join(" · ");
}

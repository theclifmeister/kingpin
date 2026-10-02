// The routine (#556): standing orders, supply contracts and the cart,
// as the TUI's market pane, sell and buy dialogs and cart word them
// (ui/market.go standingRows and contractRows, ui/dialogs.go, ui/cart.go,
// ui/fast.go supplyShortWords). Each function takes the view and `q`,
// the engine's query (session.call), and returns words, rows or
// numbers: it touches no DOM, so smoke.mjs checks them on a live run.
// The cart is read off the view (`buys`, `orders`, `standing`, `supply`),
// never a memo of the page's. The preset review's names and estimates
// (ui/presets.go) and the upgrades' prerequisites by name are here too:
// presets are bundles of the routine's dials.

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

// held is what a city's stash holds of every product (World.StockIn).
const held = (v, city) => Object.values(v.you.stock?.[city] || {}).reduce((n, x) => n + x, 0);

// contractRoom is the warning a contract set at level gets when the
// city cannot hold it (#524), "" when it can. Where you stand it counts
// the room without your carry (`you.away`, World.CapacityAway), which
// leaves with you.
export function contractRoom(v, city, id, level) {
  if (!(level > 0)) return "";
  const other = held(v, city) - stock(v, city, id);
  if (city === v.you.city) {
    const room = Math.max(0, (v.you.away?.[city] || 0) - other);
    return room < level ? `${cityName(v, city)} holds ${room} of it without you: your carry leaves with you, and the contract fills only that far once you go.` : "";
  }
  const room = (v.you.room?.[city] || 0) + stock(v, city, id);
  return room < level ? `The stash in ${cityName(v, city)} has room for ${Math.max(0, room)} of it: the contract fills only that far.` : "";
}

// contractsLeft is what leaving a city says of your contracts there
// (#524): each whose level the city cannot hold once you go.
export function contractsLeft(v, city) {
  const out = [];
  for (const c of v.supply || []) {
    if (c.city !== city || c.lieutenant) continue;
    const room = Math.max(0, (v.you.away?.[city] || 0) - (held(v, city) - stock(v, city, c.product)));
    if (room < c.units) out.push(`The ${productName(v, c.product)} contract (keep ${c.units}) has room for ${room} once you go: your carry leaves with you.`);
  }
  return out;
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

// estimate is what an order is expected to move, take and cost in heat
// (the TUI's orderEstimate): a standing order sells at most what may be
// sold, and its take is after the crew's cut; the heat is the sale's
// and the sloppy runners' (estHeat).
export function estimate(v, q, o, isStanding) {
  const p = productIn(v, o.city, o.product);
  if (!p) return { units: 0, take: 0, heat: 0 };
  let qty = o.qty,
    cut = 0;
  if (isStanding) {
    const most = sellable(v, q, o.city, o.product);
    qty = Math.min(o.all ? most : qty, most);
    cut = q("rules.market.cut");
  }
  const units = Math.min(qty, q("rules.market.capacity", o.city, o.product, o.dial));
  return {
    units,
    take: Math.floor(units * p.price * q("rules.market.dial", o.dial).Price * (1 - cut)),
    heat: q("rules.heat.sale_heat", o.city, o.product, qty, o.dial) + q("rules.heat.sloppy_heat", o.city, units),
  };
}

// cart is the day's shopping, read off the view (the TUI's
// cartModalLines, ui/cart.go): the day's buys in the order made, merged
// per city and product and kept apart by how they were paid (by hand,
// on a connect's book, or by a contract this morning), each order
// queued, each standing order of yours that sells tonight where you
// placed no order of the day (never on a quiet day: nothing sells), then
// each contract of yours as a keep line, in city and ladder order. A
// line is {kind: "buy" | "credit" | "morning" | "sell" | "standing" |
// "keep", city, product, qty, ...}: a buy's unit and cost, an order's
// dial and take.
export function cart(v, q) {
  const lines = [],
    at = {};
  for (const b of v.buys || []) {
    const kind = b.contract ? "morning" : b.credit ? "credit" : "buy",
      k = `${kind} ${b.city}/${b.product}`;
    if (k in at) {
      lines[at[k]].qty += b.qty;
      lines[at[k]].cost += b.cost;
      continue;
    }
    at[k] = lines.length;
    lines.push({ kind, city: b.city, product: b.product, qty: b.qty, cost: b.cost });
  }
  for (const l of lines) l.unit = l.qty > 0 ? l.cost / l.qty : 0;
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
    case "buy":
      return `Buy ${what}${where} · ${price(l.unit)} · ${money(l.cost)}`;
    case "credit":
      return `Credit ${what}${where} · ${price(l.unit)} · ${money(l.cost)} on the book`;
    case "morning":
      return `Morning ${what}${where} · ${price(l.unit)} · ${money(l.cost)} by contract this morning`;
    case "keep":
      return `Keep ${what}${where} · ~${l.due} tonight`;
    case "standing":
      return `Standing ${l.all ? `all ${productName(v, l.product)} (${l.qty})` : what}${where} · ${dialShort(l.dial)} · ~${money(l.take)}`;
  }
  return `Sell ${what}${where} · ${dialShort(l.dial)} · ~${money(l.take)}`;
}

// isBuy is whether a cart line is one of the day's buys.
export const isBuy = (l) => l.kind === "buy" || l.kind === "credit" || l.kind === "morning";

// cartTotals is the cart's bottom line, the TUI's: "Buying 2 lines for
// $785 (1 by contract this morning) · Selling 2 lines (1 standing),
// ~$1,200 · 1 contract kept"; "" for an empty cart. What went on a
// connect's book counts in the buying, not in the spending.
export function cartTotals(lines) {
  const buys = lines.filter(isBuy),
    sells = lines.filter((l) => !isBuy(l) && l.kind !== "keep"),
    standingN = sells.filter((l) => l.kind === "standing").length,
    keeps = lines.filter((l) => l.kind === "keep").length,
    contracts = buys.filter((l) => l.kind === "morning").length,
    credits = buys.filter((l) => l.kind === "credit").length,
    parts = [];
  if (buys.length) {
    let line = `Buying ${plural(buys.length, "line")} for ${money(buys.reduce((n, l) => n + l.cost, 0))}`;
    if (contracts && credits) line += ` (${contracts} by contract this morning, ${credits} on credit)`;
    else if (contracts) line += ` (${contracts} by contract this morning)`;
    else if (credits) line += ` (${credits} on credit)`;
    parts.push(line);
  }
  if (sells.length) parts.push(`Selling ${plural(sells.length, "line")}${standingN ? ` (${standingN} standing)` : ""}, ~${money(sells.reduce((n, l) => n + l.take, 0))}`);
  if (keeps) parts.push(`${plural(keeps, "contract")} kept`);
  return parts.join(" · ");
}

// giveBack is the command that returns a buy line (ui/cart.go giveBack):
// a contract's morning buy, a buy on the book, or a buy for cash.
export const giveBack = (l) => (l.kind === "morning" ? "return_supplied" : l.kind === "credit" ? "return_credit" : "return");

// returned is the answer to a return: what came back to the till, or,
// for a credit line, what came off the book (#72).
export function returned(v, l, qty, refund) {
  return l.kind === "credit" ? `Returned ${qty} ${productName(v, l.product)}: off the book.` : `Returned ${qty} ${productName(v, l.product)}, ${money(refund)} back.`;
}

// morningNote is the cart's sentence under a contract's morning buy
// (#444): what the contract keeps against what it brought this morning.
export function morningNote(v, l) {
  const c = contract(v, l.city, l.product);
  if (!c || !c.own) return `Bought this morning by a contract since cleared: ${l.qty} ${productName(v, l.product)}.`;
  return `The contract keeps ${c.units}; it brought ${l.qty} this morning. Its level is its keep line below.`;
}

// The preset review (ui/presets.go). DIALS are game.SellOrder's dial
// numbers on the wire.
const DIALS = ["quiet", "normal", "aggressive"];
const routeName = (v, id) => (v.routes || []).find((r) => r.id === id)?.name || id;

// changeName is the setting a change moves, as the review names it.
export function changeName(v, c) {
  switch (c.setting) {
    case "standing":
      return `standing ${productName(v, c.product)}, ${cityName(v, c.city)}`;
    case "supply":
      return `contract ${productName(v, c.product)}, ${cityName(v, c.city)}`;
    case "launder":
      return "launder dial";
    case "pay":
      return "pay dial";
    case "route":
      return "route " + routeName(v, c.route);
    case "target":
      return `target ${productName(v, c.product)}, ${routeName(v, c.route)}`;
    case "lie_low":
      return "lie low today";
  }
  return c.setting;
}

// commandName is what a refused command was for.
export function commandName(v, c) {
  if (c.city && c.product) return `${productName(v, c.product)}, ${cityName(v, c.city)}`;
  if (c.route) return routeName(v, c.route);
  return c.op;
}

// changeEstimate is a change's estimate, "~" before it: a standing
// order's take and heat a night (as a share where the order stands
// before and after, in money and heat where it comes or goes), a
// contract's buy tomorrow morning, and the orders lying low drops
// tonight; "" with none.
export function changeEstimate(v, q, c) {
  const signedMoney = (n) => (n >= 0 ? "+" : "-") + money(Math.abs(n)),
    signedPct = (f) => (f >= 0 ? "+" : "") + (Math.abs(f) < 10 ? f.toFixed(1) : f.toFixed(0)) + "%",
    order = (o) => ({ city: o.City, product: o.Product, qty: o.Qty, dial: DIALS[o.Dial] || "normal", all: o.All });
  switch (c.setting) {
    case "standing": {
      const a = c.was ? estimate(v, q, order(c.was), true) : { take: 0, heat: 0 },
        b = c.now ? estimate(v, q, order(c.now), true) : { take: 0, heat: 0 };
      if (c.was && c.now && a.take > 0 && a.heat > 0) return `~take ${signedPct(((b.take - a.take) / a.take) * 100)}, heat ${signedPct(((b.heat - a.heat) / a.heat) * 100)}`;
      return `~take ${signedMoney(b.take - a.take)}, heat ${(b.heat - a.heat >= 0 ? "+" : "") + (b.heat - a.heat).toFixed(1)}`;
    }
    case "supply":
      return (c.cost || 0) === (c.cost_to || 0) ? "" : `~${signedMoney((c.cost_to || 0) - (c.cost || 0))} a morning`;
    case "lie_low":
      return c.dropped > 0 ? `drops ${plural(c.dropped, "order")} tonight` : "";
  }
  return "";
}

// presetSaid is the answer to a preset applied: the settings changed,
// and the first refusal where the rules refused any.
export function presetSaid(r) {
  const say = `${r.preset.name}: ${plural(r.changes.length, "setting")} changed.`;
  return r.refused.length ? `${say} ${plural(r.refused.length, "command")} refused: ${r.refused[0].why}.` : say;
}

// requiresNames are an upgrade's prerequisites by name, each marked
// owned or missing: [{name, owned}].
export function requiresNames(v, u) {
  return u.requires.map((id) => {
    const r = v.upgrades.find((x) => x.id === id);
    return { name: r?.name || id, owned: r?.state === "owned" };
  });
}

// The till, the wash and the road explained (#553), as the TUI's
// ledger, till dialog, buy picker and routes word them (ui/till.go,
// ui/ledger.go, ui/reserve.go, ui/cashout.go, ui/routes.go). Each
// function takes the view and `q`, the engine's query (session.call),
// and returns words, a tone ("" plain, "warn", "danger", "subtle") or
// numbers: it touches no DOM, so smoke.mjs checks them on a live run.

const money = (n, r = Math.round(n || 0)) => (r < 0 ? "-$" : "$") + Math.abs(r).toLocaleString("en-US"), // format.Money: -$30,040
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  pct = (x, d) => fixed(x * 100, d) + "%", // format.Pct
  pctText = (f) => fixed(f, f < 10 && f > -10 ? 1 : 0) + "%"; // ui.pctText, on a percent

// fixed is Go's %.*f: toFixed, but an exact tie goes to the even digit
// as Go rounds it ($12.5M is $12M in the TUI, where toFixed says $13M).
// toFixed(100) is the float's exact decimal, so a tie is a 5 and zeros.
export function fixed(x, d) {
  const s = x.toFixed(d);
  const exact = Math.abs(x).toFixed(100),
    rest = exact.slice(exact.indexOf(".") + 1 + d);
  if (!/^50*$/.test(rest)) return s;
  const down = (Math.trunc(Math.abs(x) * 10 ** d) / 10 ** d).toFixed(d), // the digits kept, unrounded
    last = Number(down[down.length - 1]);
  return last % 2 === 0 ? (x < 0 ? "-" : "") + down : s;
}

// cash is a big amount the short way, as format.Cash writes it: under
// $10,000 in full, then $12K, $1.2M, $3.4B.
export function cash(n) {
  n = Math.round(n || 0);
  if (n > -10_000 && n < 10_000) return money(n);
  const sign = n < 0 ? "-" : "",
    units = ["K", "M", "B", "T"];
  let x = Math.abs(n) / 1000,
    i = 0;
  while (i < units.length - 1 && Math.round(x) >= 1000) (x /= 1000), i++; // $999,600 reads $1.0M
  return `${sign}$${fixed(x, x < 10 ? 1 : 0)}${units[i]}`;
}

// cashWeight is what a pile of dirty cash weighs in hundreds, as
// format.CashWeight writes it.
export function cashWeight(n) {
  const kg = n / 100 / 1000;
  if (kg < 1) return "under a kilo";
  if (Math.round(kg) < 1000) return `${fixed(kg, 0)} kg`;
  if (kg < 9950) return `${fixed(kg / 1000, 1)} tonnes`;
  return `${fixed(kg / 1000, 0)} tonnes`;
}

// till is the wash's line as it stands (#496, #526): the till (the
// float, or the player's line over it), the float under it, the rot
// line over it, and the line the wash stops at tonight (the till, or
// the contracts' morning where that is more, `outlay`).
export function till(q) {
  const t = q("rules.laundering.till"),
    float = q("rules.laundering.float"),
    rot = q("rules.laundering.rot_line");
  return { till: t, float, set: t > float, max: rot > 0 ? Math.max(rot, float) : 999_999_999, line: q("rules.laundering.line"), outlay: q("rules.laundering.outlay") };
}

// upkeepTonight is tonight's clean bill (#458): the open fronts' upkeep
// and the assets', taken from the clean pile after the wash.
export function upkeepTonight(q) {
  return q("rules.laundering.upkeep") + q("rules.laundering.asset_upkeep");
}

// tillWords is the wash panel's till line (#526: it read the float with
// a till set).
export function tillWords(q) {
  const t = till(q);
  return t.set
    ? `The till keeps ${money(t.till)} dirty, the line you set; the wash takes only what is over it.`
    : `The till keeps ${money(t.till)} dirty for the street (the float); the wash takes only what is over it.`;
}

// tonight is what the wash would take tonight on the cash in hand at
// the till as it stands (the till dialog's "tonight" row).
export function tonight(v, q) {
  const t = till(q),
    keep = Math.max(t.till, t.outlay),
    n = Math.min(q("rules.laundering.capacity"), Math.max(0, v.you.dirty_cash - keep));
  return n > 0
    ? `On the cash in hand the wash takes up to ${money(n)} tonight and leaves ${money(v.you.dirty_cash - n)}.`
    : "Nothing over the line to wash tonight.";
}

// tillRules is the till's rules, as the dialog words them.
export function tillRules(q) {
  const t = till(q);
  return `The wash takes only the dirty cash over the till, after the night's sales. Raise it to save dirty cash for a contract, a chemist's lot or the next front, over the cash in hand if you like; never under the ${money(t.float)} float, never over the ${money(t.max)} rot line. The contracts' morning is kept back whatever the till.`;
}

// setTill reads the till control's field and says what to send (#526):
// blank or under the float sends 0 (the float), over the rot line sets
// the field to the top and sends nothing. {send, field, say, refuse}.
export function setTill(q, raw) {
  const t = till(q),
    s = String(raw ?? "").trim();
  if (s === "") return { send: 0, say: `The till is the float again: the wash leaves ${money(t.float)} dirty in hand.` };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n < 0) return { refuse: "Enter a whole number of dollars, or leave it blank for the float." };
  if (n > t.max)
    return { field: t.max, refuse: `The till tops out at ${money(t.max)}, the rot line: dirty cash over it rots. The till is now ${money(t.max)}; set it again to keep it.` };
  if (n < t.float) return { send: 0, say: `The till is never under the ${money(t.float)} float: it is the float, and the wash leaves ${money(t.float)} dirty in hand.` };
  if (n === t.float) return { send: 0, say: `The till is the float again: the wash leaves ${money(t.float)} dirty in hand.` };
  return { send: n, say: `The till is ${money(n)}: the wash leaves that much dirty in hand every night.` };
}

// roadWaits is the first route idle on the till and what it waits on
// over the float (#524, rules.logistics.waits), or null.
export function roadWaits(v, q) {
  for (const r of v.routes || []) {
    const need = q("rules.logistics.waits", r.id);
    if (need > 0) return { name: r.name, need };
  }
  return null;
}

// odds is the ledger's summary beside the launder dial (#577,
// ui/ledger.go): the odds of an audit tonight on any open front, what
// the open fronts wash between them, and what they earn on their own.
export function odds(q) {
  return `audit ${pct(q("rules.laundering.any_audit_risk"), 1)}/day · up to ${money(q("rules.laundering.capacity"))}/day · legit ${money(q("rules.laundering.legit_income"))}/day`;
}

// pileLine is the ledger's word on a dirty pile big enough to be a
// storage problem (#392, the TUI's pileLine): its weight in hundreds,
// and the rot a night once it is over the rot line. null under $10M.
export function pileLine(v, q) {
  const dirty = v.you.dirty_cash;
  if (dirty < 10_000_000) return null;
  const rot = q("rules.laundering.rot", dirty);
  return {
    label: "pile",
    text: `the pile weighs ${cashWeight(dirty)} in hundreds${rot > 0 ? ` · rats and damp take ~${money(rot)} a night over ${cash(q("rules.laundering.rot_line"))}` : ""}`,
    more: "",
    tone: "gold",
  };
}

// taxLines are the tax (#231): what the free corners of each city you
// hold pay a night, with the run's take so far.
export function taxLines(v, q) {
  return v.cities.flatMap((c) => {
    const t = q("rules.territory.tax_due", c.id);
    return t.corners > 0
      ? [{ label: "tax", text: `${plural(t.corners, "free corner")} in ${c.name} pay ~${money(t.amount)}/night`, more: ` · ${cash(v.stats.taxed)} so far`, tone: "gold" }]
      : [];
  });
}

// frontRows are an owned front's wash, as the TUI's front pane rows
// them (#577): what it washes a day at the dial with the accountants'
// share, and its audit odds at the dial. o is its offer
// (rules.laundering.offers), for the base the accountants add to.
// [[label, text]].
export function frontRows(v, q, f, o) {
  const dial = v.you.launder;
  let washes = `${money(q("rules.laundering.throughput", f.id))}/day`;
  if (v.crew.some((m) => m.role === "accountant" && !m.jailed && !m.wounded)) {
    const base = Math.round(o.Throughput * q("rules.laundering.dial", dial).Mul);
    washes += ` (+${money(q("rules.laundering.throughput", f.id) - base)} accountants)`;
  }
  return [
    ["washes", washes],
    ["audit", `${pctText(q("rules.laundering.audit_risk", f.id) * 100)}/day at ${dial}`],
  ];
}

// offerAudit is an offer's audit odds a day at the normal dial, as the
// TUI's offer pane writes the percent.
export const offerAudit = (o) => pctText(o.AuditRisk * 100);

// washLines are the ledger's lines under the wash (#417, #524): the
// pile's weight and rot, the wash idle under the till, a route waiting
// on a lot and the till that would save for it, the fronts the night is
// expected to shut on their upkeep (preview.wash.shuts), and the tax
// (#577). [{label, text, more, tone}].
export function washLines(v, q, p) {
  const t = till(q),
    out = [];
  const pile = pileLine(v, q);
  if (pile) out.push(pile);
  if (v.fronts.length && v.you.dirty_cash <= t.till)
    out.push({ label: "wash", text: `idle: dirty ${money(v.you.dirty_cash)} is under the ${money(t.till)} till`, more: "; the wash takes only what is over it", tone: "warn" });
  const road = roadWaits(v, q);
  if (road) {
    const save = t.outlay + t.float + road.need;
    if (save > t.till)
      out.push({ label: "road", text: `${road.name} waits on ${money(road.need)} over the float; the wash takes it first`, more: `: raise the till to ${money(save)} to save for it`, tone: "warn", raise: save });
    else out.push({ label: "road", text: `${road.name} waits on ${money(road.need)} over the float`, more: "; the till saves for it", tone: "warn" });
  }
  if (p && p.wash && p.wash.shuts && p.wash.shuts.length)
    out.push({ label: "upkeep", text: `${p.wash.shuts.join(", ")} expected to shut tonight: upkeep ${money(p.wash.short)} clean short`, more: `; the wash leaves ${money(t.line)} dirty in hand`, tone: "danger" });
  return out.concat(taxLines(v, q));
}

// upkeepWarning is the warning a move shows when it leaves the clean
// pile (clean after it, dirty the dirty pile) under tonight's upkeep
// (#458), or null: a danger with nothing over the till to wash, a
// warning where the wash may yet cover it.
export function upkeepWarning(q, clean, dirty) {
  const due = upkeepTonight(q);
  if (due <= 0 || clean >= due) return null;
  const shuts = plural(q("rules.laundering.tuning").UpkeepFreezeDays, "day");
  if (dirty > till(q).till)
    return { tone: "warn", text: `Leaves ${money(Math.max(0, clean))} clean for ${money(due)} of upkeep tonight: a front the wash does not cover shuts ${shuts}.` };
  return { tone: "danger", text: `Leaves ${money(Math.max(0, clean))} clean for ${money(due)} of upkeep tonight, and nothing over the till to wash: a front shuts ${shuts}.` };
}

// reserveBlank is the reserve's default (#458): a lot, or the clean
// cash less tonight's upkeep where that is less, so a transfer at the
// default never leaves the pile under the upkeep.
export function reserveBlank(v, q) {
  return Math.max(0, Math.min(q("rules.laundering.offshore").Lot, v.you.clean_cash - upkeepTonight(q)));
}

// cashOutFor is the clean cash to draw for want to land dirty, the fee
// on top, or the clean cash where that is less (ui/cashout.go).
function cashOutFor(v, q, want) {
  let amt = want;
  const fee = (n) => q("rules.laundering.cash_out_fee", n);
  for (let i = 0; i < 20 && amt < v.you.clean_cash && amt - fee(amt) < want; i++) amt += want - (amt - fee(amt));
  return Math.min(amt, v.you.clean_cash);
}

// cashOutBlank is the cash-out's default: what tonight's wages are
// short over the dirty cash, the fee on top (the TUI's blank), or
// $10,000 with none short, and never into tonight's upkeep (#553).
export function cashOutBlank(v, q) {
  const short = Math.max(0, q("rules.crew.wages", v.you.pay) - v.you.dirty_cash),
    want = short > 0 ? cashOutFor(v, q, short) : 10000;
  return Math.max(0, Math.min(want, v.you.clean_cash - upkeepTonight(q)));
}

// frontStatus is an owned front's state and why (#417, #458, #531): open,
// opens tomorrow, audit back in Nd, shut Nd: upkeep unpaid, idle under
// the till. {text, tone}.
export function frontStatus(v, q, f) {
  const day = v.day,
    frozen = (f.frozen_until || 0) > day + 1;
  if (frozen && f.unpaid > 0)
    return { tone: "warn", text: `Shut ${plural(f.frozen_until - day, "day")}: upkeep unpaid (${money(q("rules.laundering.front_upkeep", f.id))} clean a night)` };
  if (frozen && f.audited > 0 && f.frozen_until === f.audited + q("rules.laundering.tuning").AuditFreezeDays)
    return { tone: "danger", text: `Audited: back in ${plural(f.frozen_until - day, "day")}` };
  if (frozen) return { tone: "warn", text: `Shut: back in ${plural(f.frozen_until - day, "day")}` };
  if (f.bought === day) return { tone: "subtle", text: "Opens tomorrow" };
  if (v.you.dirty_cash <= till(q).till) return { tone: "warn", text: "Idle: the dirty cash is under the till" };
  return { tone: "", text: "Open for business" };
}

// coveredNights is how many nights from tonight an owned front's upkeep
// is still covered (#528), 0 once they are gone.
export function coveredNights(v, q, f) {
  const grace = q("rules.laundering.tuning").UpkeepGraceDays,
    night = v.day + 1;
  return night > f.bought && night - f.bought <= grace ? f.bought + grace - v.day : 0;
}

// frontTerms is what a front does once bought (#458, #528): it washes
// from tomorrow night, only the dirty over the till; its upkeep is clean
// cash, covered for its first nights; a night unpaid shuts it.
export function frontTerms(q, o) {
  const tun = q("rules.laundering.tuning"),
    up = q("rules.laundering.front_upkeep", o.ID);
  let s = `It opens tomorrow and washes the dirty over the ${money(till(q).till)} till. `;
  s += tun.UpkeepGraceDays > 0
    ? `Its first ${plural(tun.UpkeepGraceDays, "night")} of upkeep are covered; then ${money(up)} a day, paid in clean cash`
    : `Its ${money(up)} a day upkeep is paid in clean cash, from tonight`;
  return s + `; a night unpaid shuts it ${plural(tun.UpkeepFreezeDays, "day")}.`;
}

// assetLive is whether an owned asset stands tonight (World.AssetLive).
function assetLive(v, id) {
  return (v.assets || []).some((a) => a.id === id && !((a.frozen_until || 0) > v.day + 1));
}

// locked is whether an offer is still gated (game.FrontOffer.Locked):
// behind peak cash, or an asset not standing.
function locked(v, o) {
  return v.you.peak_cash < o.UnlockCash || (!!o.Asset && !assetLive(v, o.Asset));
}

// frontShort is the clean cash tonight's upkeep is expected to come up
// short by with the offer bought (#496, the TUI's frontShort), or 0:
// the preview's piles at the wash, the price off the dirty, the new
// front's throughput joined to the fronts' and its upkeep to the
// night's. An estimate, as the preview is.
export function frontShort(v, q, o, p) {
  if (!p || locked(v, o) || o.Cost > v.you.dirty_cash) return 0;
  const tun = q("rules.laundering.tuning"),
    newUpkeep = tun.UpkeepGraceDays > 0 ? 0 : q("rules.laundering.front_upkeep", o.ID),
    due = upkeepTonight(q) + newUpkeep,
    w = p.wash,
    dirty = p.flow.closing.dirty + w.washed - o.Cost,
    clean = p.flow.closing.clean - w.washed + w.upkeep - w.income + w.assets,
    washed = Math.min(q("rules.laundering.capacity") + q("rules.laundering.throughput", o.ID), Math.max(0, dirty - till(q).line));
  return Math.max(0, due - clean - washed);
}

// frontShutWarning is the buy's warning on an offer the night is
// expected to shut, or "": nothing is said of a front that will pay.
export function frontShutWarning(v, q, o, p) {
  const short = frontShort(v, q, o, p);
  if (short <= 0) return "";
  const tun = q("rules.laundering.tuning"),
    due = upkeepTonight(q) + (tun.UpkeepGraceDays > 0 ? 0 : q("rules.laundering.front_upkeep", o.ID));
  return `Tonight's ${money(due)} of upkeep is expected to come up ${money(short)} clean short: the wash takes only the dirty over the ${money(till(q).till)} till, after the sales. A front shuts ${plural(tun.UpkeepFreezeDays, "day")}.`;
}

// offerLock is a locked offer's line (#148, #463, #525): "Locked until
// the Dutchman's book stands", or "Locked until peak cash $X · $Y to
// go", or "".
export function offerLock(v, o) {
  if (o.Asset && !assetLive(v, o.Asset)) return `Locked until ${o.AssetName} stands`;
  if (v.you.peak_cash < o.UnlockCash) return `Locked until peak cash ${money(o.UnlockCash)} · ${money(o.UnlockCash - v.you.peak_cash)} to go`;
  return "";
}

// noFrontsOnOffer is what the page says with no front on offer (#525):
// every front owned, or the fronts that wait on an asset never bought.
export function noFrontsOnOffer(q) {
  const waiting = q("fronts_waiting") || [];
  if (!waiting.length) return "You own every front there is.";
  return "No front on offer: " + waiting.map((o) => `${o.Name} waits on ${o.AssetName}`).join("; ") + ".";
}

// routeShortOf is what a route idle on an empty stash has none of at
// its source (#537): "no Weed or Coke", or "nothing it is short of".
function routeShortOf(v, r) {
  const from = v.cities.find((c) => c.id === r.from),
    stock = (id) => v.you.stock?.[r.from]?.[id] || 0, // the street and the houses (World.Stock)
    names = Object.keys({ ...(r.target || {}), ...(r.days_target || {}) })
      .filter((id) => (r.target?.[id] || r.days_target?.[id]) && stock(id) <= 0)
      .map((id) => from?.products.find((p) => p.id === id)?.name || id);
  return names.length && names.length <= 2 ? "no " + names.join(" or ") : "nothing it is short of";
}

// routeIdle is why a route on its dial would send nothing (#459,
// rules.logistics.idle), or null for one that sends or is off.
// {text, tone}.
export function routeIdle(v, q, r) {
  const t = money(till(q).till),
    from = v.cities.find((c) => c.id === r.from)?.name || r.from;
  switch (q("rules.logistics.idle", r.id)) {
    case "till":
      return q("rules.logistics.budget") > 0
        ? { tone: "warn", text: `Idle: too little over the ${t} till for a lot` }
        : { tone: "warn", text: `Idle: no dirty cash over the ${t} till` };
    case "stock":
      return { tone: "warn", text: `Idle: ${routeShortOf(v, r)} in the ${from} stash` };
    case "target":
      return { tone: "warn", text: "Idle: no target" };
    case "closed":
      return { tone: "warn", text: "Idle: shut" };
    case "met":
      return { tone: "subtle", text: "Idle: target met" };
  }
  return null;
}

// assetTerms is an asset offer's cost to keep (ui/assets.go): its
// upkeep in clean cash, and the floor it puts under the heat.
export function assetTerms(a) {
  return `${money(a.Upkeep)} clean a day upkeep${a.HeatFloor > 0 ? ` · heat floor ${Math.round(a.HeatFloor)} in every city while it stands` : ""}`;
}

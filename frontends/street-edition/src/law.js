// The law and its answers (#552), as the TUI words them: the LAW panel
// (ui/dashboard_panels.go lawLines), the DA RACE block (ui/race.go),
// the fund dialog and its campaign page (ui/law.go), the favour call-in
// (ui/law.go), the tip confirm (ui/books.go), the lie-low words
// (ui/model.go), and the bought law: the bribe dialog, the checkpoint
// confirm and the PAYOFFS block (ui/bribes.go) and the cop dialog
// (ui/intel.go). Each function takes the view and `q`, the engine's
// query (session.call), and returns words, rows or numbers: it touches
// no DOM, so smoke.mjs checks them on a live run.

import { upkeepTonight, upkeepWarning } from "./wash.js?v=__BUILD_REVISION__";

const money = (n) => "$" + Math.round(n || 0).toLocaleString("en-US"),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  pct = (x) => `${Math.round((x || 0) * 100)}%`,
  the = (name) => (String(name).startsWith("The ") ? name : "the " + name);

const cityOf = (v, id) => v.cities.find((c) => c.id === id),
  cityName = (v, id) => cityOf(v, id)?.name || id;

// TICKETS are the tickets money goes behind (game.Tickets): a moderate
// runs, but takes no money.
export const TICKETS = ["reform", "law_and_order"];

// stanceWord is a DA's ticket in words: `law-and-order`.
export function stanceWord(stance) {
  return stance === "law_and_order" ? "law-and-order" : stance;
}

// swing is what a campaign's cash moves a city's vote by
// (CampaignTuning.Swing), and fill the cash that buys the whole swing.
export function swing(cmp, cash) {
  if (cmp.Cash <= 0 || cash <= 0) return 0;
  return Math.min(cmp.SwingMax, cash / cmp.Cash / 100);
}
export function fill(cmp) {
  return Math.round(cmp.SwingMax * 100 * cmp.Cash);
}

// swingWord is a campaign's pull on a city's vote: `4.5 points`.
export function swingWord(s) {
  return `${(s * 100).toFixed(1)} points`;
}

// cold is a law-and-order DA sitting (World.Cold): nobody takes calls.
export function cold(v) {
  return v.law.da_stance === "law_and_order";
}

// campaign is the money behind a ticket in the city, today's included
// (World.Campaigning): {ticket, cash, hedged}.
export function campaign(v, city) {
  const c = cityOf(v, city)?.campaign;
  return { ticket: c?.ticket || "", cash: c?.cash || 0, hedged: !!c?.hedged };
}

// chiefWord is what you know of the chief: their temper once seen at
// work, or `new`.
export function chiefWord(v) {
  const t = v.law.chief_temper;
  return t && t !== "?" ? t : "new";
}

// lawLines are the LAW panel's lines (lawLines): the chief and what
// they are like, the favour they owe and the days they have left; the
// DA, their ticket and the days to the election; the pressure where
// you are with the goodwill you have bought it and a campaign you have
// money in; the other cities' pressure. Each {text, tone}, `owes` set
// on the chief's when the favour can be called on.
export function lawLines(v) {
  const l = v.law,
    here = cityOf(v, v.you.city);
  const owes = l.favours > 0 && !cold(v);
  let chief = `Chief ${l.chief} · ${chiefWord(v)}`;
  if (owes) chief += " · owes one";
  if (l.chief_term_ends > 0) chief += ` · ${Math.max(0, l.chief_term_ends - v.day)}d left`;
  let da = `DA ${l.da} · ${stanceWord(l.da_stance)}`;
  if (l.next_election > 0) da += ` · election in ${Math.max(0, l.next_election - v.day)}d`;
  let pressure = `Pressure ${Math.round(here.pressure)} · goodwill ${Math.round(here.goodwill)}`;
  const camp = campaign(v, here.id);
  if (l.campaign_open && camp.cash > 0)
    pressure += camp.hedged ? ` · backing both ${money(camp.cash)}` : ` · backing ${stanceWord(camp.ticket)} ${money(camp.cash)}`;
  const lines = [
    { text: chief, tone: "", owes },
    { text: da, tone: "" },
    { text: pressure, tone: "" },
  ];
  const others = v.cities.filter((c) => c.id !== here.id).map((c) => `${c.name} pressure ${Math.round(c.pressure)}`);
  if (others.length) lines.push({ text: others.join(" · "), tone: "subtle" });
  return lines;
}

// raceShown is whether there is a DA RACE to show (raceShown): the
// tickets take money and there is an election to come.
export function raceShown(v) {
  return !!v.law.campaign_open && v.law.next_election > 0;
}

// raceWhen is how far off the vote is: `tonight`, `in 12 days`.
export function raceWhen(days) {
  return days <= 0 ? "tonight" : "in " + plural(days, "day");
}

// raceNote is the block's heading note: `the vote on day 90, in 12 days`.
export function raceNote(v) {
  return `the vote on day ${v.law.next_election}, ${raceWhen(v.law.next_election - v.day)}`;
}

// raceRows are the block's rows (raceTable): a city each, the ticket
// your money is behind there, what the campaign holds and the points
// of the vote it moves.
export function raceRows(v, q) {
  if (!raceShown(v)) return [];
  const cmp = q("rules.law.campaign");
  return v.cities.map((c) => {
    const camp = campaign(v, c.id);
    let ticket = "-",
      points = "-";
    if (camp.hedged) ticket = "both";
    else if (camp.cash > 0) [ticket, points] = [stanceWord(camp.ticket), swingWord(swing(cmp, camp.cash))];
    return { city: c.id, name: c.name, ticket, backed: camp.cash, points, hedged: camp.hedged };
  });
}

// raceOdds are the odds per ticket as the count would read them this
// morning (rules.law.odds): [[ticket words, odds]].
export function raceOdds(q) {
  const o = q("rules.law.odds");
  return [
    ["law-and-order", o.law_and_order],
    ["reform", o.reform],
    ["a moderate", o.moderate],
  ];
}

// raceLines are the race's details for a city (raceSection): the vote,
// who sits, what your money there holds and buys, and the price. Each
// [label, text, tone].
export function raceLines(v, q, city) {
  const cmp = q("rules.law.campaign"),
    camp = campaign(v, city),
    next = v.law.next_election;
  const rows = [
    ["vote", `day ${next}, ${raceWhen(next - v.day)}`, ""],
    ["sitting", `DA ${v.law.da}, ${stanceWord(v.law.da_stance)}`, ""],
  ];
  if (camp.hedged) rows.push(["yours", `${money(camp.cash)} on both: it buys nothing`, "danger"]);
  else if (camp.cash > 0) rows.push(["yours", `${money(camp.cash)} ${stanceWord(camp.ticket)}, ${swingWord(swing(cmp, camp.cash))} of the vote`, "good"]);
  else rows.push(["yours", "nothing yet", "subtle"]);
  rows.push(["price", `${money(cmp.Cash)} a point, ${Math.round(cmp.SwingMax * 100)} points at most`, ""]);
  return rows;
}

// raceCosts is what the race costs (raceSection's last line).
export function raceCosts(q) {
  const cmp = q("rules.law.campaign");
  let s = `Clean cash, spent at the count. A campaign adds ${cmp.Pressure.toFixed(1)} pressure a day; a losing ticket adds ${Math.round(cmp.LoserPressure)} here`;
  if (cmp.LoserChief) s += ", and a zealous chief if law-and-order wins";
  return s + ". A winner you backed owes you: the sting line sits higher.";
}

// The fund dialog (ui/law.go viewFund): the city, the amount for
// goodwill (blank is the most that does not take tonight's upkeep),
// and while the tickets take money the campaign: the ticket and the
// amount behind it (blank is nothing). Nothing is given until the
// confirm.

// maxFund is enough clean cash to take the city's goodwill to 100, or
// all of it if that is less: goodwill stops at 100.
export function maxFund(v, q, c) {
  const need = Math.trunc((100 - c.goodwill) * q("rules.law.tuning").GoodwillCash);
  return Math.max(0, Math.min(need, v.you.clean_cash));
}

// fundBlank is what a blank amount gives (#458): maxFund less what it
// would take of tonight's upkeep.
export function fundBlank(v, q, c) {
  return Math.max(0, Math.min(maxFund(v, q, c), v.you.clean_cash - upkeepTonight(q)));
}

// readAmount reads a money field: blank is `blank`, else a whole
// number of dollars. {amount} or {err}.
export function readAmount(raw, blank) {
  const s = String(raw ?? "")
    .trim()
    .replace(/[$,]/g, "");
  if (s === "") return { amount: blank, blank: true };
  const n = Number(s);
  if (!Number.isSafeInteger(n) || n < 0) return { err: "Enter a whole number of dollars." };
  return { amount: n };
}

// fundLines are the first page's lines for the amount typed: the city
// now, what it buys, the upkeep warning, tonight's upkeep and what
// goodwill does. Each [label, text, tone]; the label "" is prose.
export function fundLines(v, q, c, raw) {
  const tun = q("rules.law.tuning"),
    due = upkeepTonight(q),
    rows = [["now", `pressure ${Math.round(c.pressure)} · goodwill ${Math.round(c.goodwill)}`, ""]];
  const r = readAmount(raw, fundBlank(v, q, c));
  if (r.err) rows.push(["", r.err, "danger"]);
  else if (r.amount > 0) {
    rows.push(["buys", `+${Math.round(q("rules.law.goodwill", r.amount))} goodwill for ${money(r.amount)} (${money(tun.GoodwillCash)} a point, 100 at most)`, r.amount > v.you.clean_cash ? "danger" : ""]);
    const warn = upkeepWarning(q, v.you.clean_cash - r.amount, v.you.dirty_cash);
    if (warn) rows.push(["", warn.text, warn.tone]);
  }
  if (due > 0) rows.push(["upkeep", `${money(due)} clean tonight (blank keeps it back)`, "subtle"]);
  rows.push(["", `Full goodwill takes ${tun.GoodwillCut.toFixed(1)} pressure off the city a day; it fades ${pct(tun.GoodwillDecay)} a day. Community centres, campaigns, benevolent funds: clean money only.`, "subtle"]);
  if (raceShown(v)) rows.push(["", `DA race in ${plural(Math.max(0, v.law.next_election - v.day), "day")}: the tickets are taking money below.`, "gold"]);
  return rows;
}

// maxBack is the most the campaign field takes (#193): the clean cash
// left after the goodwill, up to what buys the city's campaign the
// whole swing over what it holds.
export function maxBack(v, q, c, given) {
  const room = fill(q("rules.law.campaign")) - campaign(v, c.id).cash;
  return Math.max(0, Math.min(v.you.clean_cash - given, room));
}

// campaignLines are the campaign page's lines (viewCampaign): what the
// city's campaign holds, the price before the amount (#506), what the
// amount buys and the upkeep it would take.
export function campaignLines(v, q, c, given, ticket, raw) {
  const cmp = q("rules.law.campaign"),
    camp = campaign(v, c.id),
    rows = [];
  if (given > 0) rows.push(["if you give", `${money(given)} goodwill above too: ${money(v.you.clean_cash - given)} clean left`, ""]);
  if (camp.hedged) rows.push(["campaign", `${money(camp.cash)} on both tickets: it buys nothing`, "danger"]);
  else if (camp.cash > 0) rows.push(["campaign", `${money(camp.cash)} behind ${stanceWord(camp.ticket)}, ${swingWord(swing(cmp, camp.cash))} of the vote`, "good"]);
  else rows.push(["campaign", "nothing yet", "subtle"]);
  rows.push(["price", `${money(cmp.Cash)} a point of the vote, ${Math.round(cmp.SwingMax * 100)} at most`, "gold"]);
  const r = readAmount(raw, 0);
  if (r.err) rows.push(["", r.err, "danger"]);
  else if (r.amount > 0) {
    rows.push(["buys", `${swingWord(swing(cmp, camp.cash + r.amount))} of ${c.name}'s vote for ${money(r.amount)}`, r.amount > v.you.clean_cash - given ? "danger" : ""]);
    if (camp.cash > 0 && !camp.hedged && camp.ticket !== ticket) rows.push(["", `The campaign's money is behind ${stanceWord(camp.ticket)}: money on both tickets buys nothing.`, "danger"]);
    const warn = upkeepWarning(q, v.you.clean_cash - given - r.amount, v.you.dirty_cash);
    if (warn) rows.push(["", warn.text, warn.tone]);
  }
  rows.push(["", `A winner you backed owes you: the sting line sits higher. A loser's rival knows who paid. Money on both tickets buys nothing; a campaign adds ${cmp.Pressure.toFixed(1)} pressure a day.`, "subtle"]);
  return rows;
}

// fundPlan is what the dialog's confirm gives (confirmFund): {fund,
// back} or {err}, with the TUI's refusal for a blank that keeps the
// whole pile back for tonight's upkeep.
export function fundPlan(v, q, c, rawFund, rawBack) {
  const f = readAmount(rawFund, fundBlank(v, q, c));
  if (f.err) return { err: f.err };
  const b = raceShown(v) ? readAmount(rawBack, 0) : { amount: 0 };
  if (b.err) return { err: b.err };
  if (f.amount <= 0 && b.amount <= 0) {
    const due = upkeepTonight(q);
    if (f.blank && c.goodwill < 100 && due > 0) return { err: `Blank keeps ${money(due)} clean back for tonight's upkeep, and that is all of it: type an amount to give it anyway.` };
    return { err: "Nothing to give." };
  }
  return { fund: f.amount, back: b.amount };
}

// fundSaid is what the page says once given (confirmFund): read after
// the acts, so the campaign is what it holds now.
export function fundSaid(v, q, c, fund, back, ticket) {
  const said = [];
  if (fund > 0) said.push(`Gave ${c.name} ${money(fund)} clean. Goodwill +${Math.round(q("rules.law.goodwill", fund))} tonight; it takes the pressure off a little every day.`);
  if (back > 0) {
    const camp = campaign(v, c.id);
    said.push(`Put ${money(back)} clean behind the ${stanceWord(ticket)} ticket in ${c.name}: the campaign holds ${money(camp.cash)}, ${swingWord(swing(q("rules.law.campaign"), camp.cash))} of the city's vote.`);
  }
  return said.join(" ");
}

// The favour (#228, ui/law.go): called in on a morning a sting, a raid
// or the task force is due tonight while the chief owes one.

const favourWord = (level) => (level === "taskforce" ? "task force" : level);

// favourRefusal is why the call cannot be made, or "" when it can
// (askFavour). `due` is rules.heat.due.
export function favourRefusal(v, due) {
  if (cold(v)) return "Can't call in the favour: nobody takes a call while a law-and-order DA sits.";
  if (!(v.law.favours > 0)) return `Can't call in the favour: Chief ${v.law.chief} owes you nothing. A bribe that takes leaves them owing one.`;
  if (!due) return "Can't call in the favour: nothing is coming tonight that a call could stop. Keep it for a raid.";
  return "";
}

// leadName is an investigation's target by name (World.LeadName).
function leadName(v, a) {
  if (a.target === "corner") return v.cities.flatMap((c) => c.corners).find((k) => k.id === a.corner)?.name || a.corner;
  if (a.target === "house") return (v.houses || []).find((h) => h.id === a.house)?.name || a.house;
  return v.cities.flatMap((c) => c.products).find((p) => p.id === a.product)?.name || a.product;
}

// favourSaves is what the rung due tonight would take (#479, the
// ladder heat.Sim.Rungs reads): a named hit takes from its target
// alone. "" with no rung.
export function favourSaves(v, q, due) {
  const hot = q("rules.heat.hottest");
  if (!hot) return "";
  const inv = v.alerts.find((a) => a.kind === "investigation");
  if (due === "sting" && inv) return `What it saves: the hit on ${leadName(v, inv)}, and the pages it files if it is in use tonight.`;
  const r = (q("rules.heat.rungs", hot) || []).find((x) => x.Level === due);
  if (!r) return "";
  let take = `${pct(r.StockLoss)} of the stock and ${pct(r.CashLoss)} of the dirty cash there`;
  if (r.Level === "taskforce") take = "an asset, " + take;
  if (r.Evidence > 0) take += `, and ${plural(r.Evidence, "page")} in the file if you sold`;
  return `What it saves: the ${favourWord(due)} would take ${take}.`;
}

// favourLines are the confirm's paragraphs (favourConfirm): what the
// call stops, what it saves and its price. Each [text, tone].
export function favourLines(v, q) {
  const due = q("rules.heat.due"),
    hot = q("rules.heat.hottest"),
    where = hot && hot !== v.you.city ? ` in ${cityName(v, hot)}` : "",
    saves = favourSaves(v, q, due);
  const lines = [[`Chief ${v.law.chief}'s people stand down tonight and the ${favourWord(due)} due${where} does not come. Nothing taken, nothing cooled: the heat stays where it is and the rung stands when its cooldown lifts.`, ""]];
  if (saves) lines.push([saves, ""]);
  lines.push([`The price: the chief's name is in your ledger, and the DA's file grows by ${q("rules.law.bribes").FavourEvidence} tomorrow. Favours left after this: ${Math.max(0, v.law.favours - 1)}.`, "subtle"]);
  return lines;
}

// favourSaid is the page's word once the call is made.
export function favourSaid(v) {
  return `The call is made. Chief ${v.law.chief}'s people stand down tonight.`;
}

// The tip (#70, #479, ui/books.go tipConfirm): a word to the police
// about a faction's corner.

// tipLines are the confirm's paragraphs: where the police's attention
// stands and what the tip does to it, when they act, the trust, the
// page's odds and the peace. Each [text, tone].
export function tipLines(v, q, corner) {
  const f = v.factions.find((x) => x.id === (corner.faction || "rival")) || v.factions[0];
  if (!f) return [["Nobody to tip on.", ""]];
  const tp = q("rules.rivals.tip_tuning"),
    arrest = q("rules.rivals.factions").LeaderArrestHeat,
    heat = f.police || 0,
    after = Math.min(100, heat + tp.Heat);
  const lines = [
    [`A word to the police about ${f.leader}'s people on ${corner.name}. Free.`, ""],
    [`Their attention on ${f.leader}'s crew goes ${Math.round(heat)} → ${Math.round(after)}; at ${Math.round(tp.PoliceNotice)} they raid the corner, at ${Math.round(arrest)} they take ${f.leader}.`, ""],
  ];
  if (after >= arrest) lines.push([`That is the end of them: the police take ${f.leader} tonight and the crew comes apart.`, "good"]);
  else if (!q("rules.rivals.raid_ready", f.id, v.day + 1))
    lines.push([`They raided ${plural(v.day - (f.last_raid || 0), "day")} ago and will not be back for ${plural((f.last_raid || 0) + tp.RaidDays - v.day - 1, "day")}; the attention builds meanwhile.`, "subtle"]);
  else if (after >= tp.PoliceNotice) lines.push([`That is the line: a raid tonight takes the corner and ${pct(tp.RaidMuscle)} of their muscle.`, "good"]);
  lines.push([`Trust -${Math.round(tp.Trust)}, and ~${pct(q("rules.heat.tip_evidence"))} the DA's file on you gains a page.`, "warn"]);
  if ((f.deals || []).some((d) => ["truce", "tribute", "homage"].includes(d))) lines.push(["Under a truce or a tribute a tip breaks the peace: trust hits the floor.", "danger"]);
  return lines;
}

// tipSaid is the page's word once the tip is queued, the page's odds
// said again (#479: the attention alone read as free). `f` is the
// faction as it stood before the tip.
export function tipSaid(v, q, corner, f) {
  const tp = q("rules.rivals.tip_tuning"),
    heat = f?.police || 0;
  return `The police hear about ${corner.name} tonight. Their attention on ${f?.leader || "the rival"}: ${Math.round(heat)} → ${Math.round(Math.min(100, heat + tp.Heat))} of ${Math.round(tp.PoliceNotice)}. ~${pct(q("rules.heat.tip_evidence"))} the DA's file on you gains a page.`;
}

// Lying low (#497, #503, ui/model.go).

// lieLowWords is who sells nothing on a lie-low day: a lieutenant
// running a city is named, since the market skips their orders too.
export function lieLowWords(v) {
  return v.crew.some((m) => m.role === "lieutenant" && m.city) ? "no sales, your lieutenants' included" : "no sales";
}

// lieLowSaid is the page's word on lying low or not.
export function lieLowSaid(v) {
  return v.you.lie_low ? `Lying low today: ${lieLowWords(v)}, heat fades faster; wages and contracts still run.` : "Back on the corner.";
}

// lieLowHandoffs are the confirm's lines over lying low with a handoff
// queued (#503): what does not go tonight. `queued` are the page's
// queued deliveries, {contract, units}.
export function lieLowHandoffs(v, queued) {
  const lines = [];
  for (const d of queued) {
    const c = v.contracts.find((x) => x.id === d.contract);
    if (!c) continue;
    const name = v.cities.flatMap((x) => x.products).find((p) => p.id === c.product)?.name || c.product;
    lines.push(`${d.units} ${name} to ${c.name}, ${c.units - c.delivered} owed by day ${c.due}.`);
  }
  if (lines.length) lines.push("Lying low, nothing is handed over tonight: the handoff stays queued through the night, and goes the first night you deal.");
  return lines;
}

// The bought law (#42, ui/bribes.go): the bribe, the deal on a route,
// the PAYOFFS block.

export const TARGETS = ["chief", "da"];

// officialName is the chief's or the DA's name with their title.
export function officialName(v, target) {
  return target === "da" ? `DA ${v.law.da}` : `Chief ${v.law.chief}`;
}

// bribePrice is what the target wants: the chief's price, or the DA's
// (halved under one you backed, less the fixer's cut).
export function bribePrice(q, target) {
  return target === "da" ? q("rules.law.d_a_price") : q("rules.law.bribes").ChiefPrice;
}

// daBacked is whether the sitting DA's ticket ran on your money
// (#193): one who takes an envelope off their ticket's odds, or a
// moderate at the backed price.
export function daBacked(v, q) {
  if (v.law.da_stance !== "moderate") return q("rules.law.d_a_odds", q("rules.law.d_a_price")) > 0;
  const b = q("rules.law.bribes"),
    mul = q("rules.law.campaign").BackedDAPriceMul,
    fixer = v.crew.filter((m) => m.role === "fixer").reduce((n, m) => Math.max(n, m.skill), -1),
    unbacked = Math.max(1, Math.round(b.DAPrice * (fixer >= 0 ? 1 - (b.FixerDiscount * fixer) / 100 : 1)));
  return mul > 0 && mul < 1 && q("rules.law.d_a_price") < unbacked;
}

// officialWord is what you know of an official for the picker: the
// chief's temper once seen, the DA's ticket, and `bought` while they
// are.
export function officialWord(v, q, target) {
  if (target === "da") {
    if (v.law.da_bought) return "bought";
    return stanceWord(v.law.da_stance) + (daBacked(v, q) ? ", yours" : "");
  }
  return v.law.chief_bought ? "bought" : chiefWord(v);
}

// bribeTerms is the picker's prose: what an envelope does, by whom.
export function bribeTerms(v, q) {
  const tun = q("rules.law.bribes");
  const lines = [
    [`A corrupt chief takes it: heat fades ${q("rules.heat.bribe_decay_mul").toFixed(1)}x faster and stings and raids come ${plural(q("rules.heat.bribe_cooldown"), "day")} later for ${plural(tun.BribeDays, "day")}. A lazy one takes half the good. A zealous one, or a law-and-order DA, files it: a page and heat ${Math.round(tun.BackfireHeat)} in the morning.`, "subtle"],
    [`A DA who takes it needs a thicker file to indict for ${plural(tun.BribeDays, "day")}. Every envelope taken is a lead; at ${tun.LeadsCase} the DA opens a file. Dirty cash only.`, "subtle"],
  ];
  if (cold(v)) lines.push(["A law-and-order DA sits: the chief is not taking calls.", "danger"]);
  return lines;
}

// bribeOdds is what the envelope is likely to do, as far as you know.
// {text, tone}.
export function bribeOdds(v, q, target, amt) {
  if (target === "da") {
    const odds = pct(q("rules.law.d_a_odds", amt));
    if (daBacked(v, q)) return { text: `~${odds} they take it: they owe you the election. Refused, the money is gone and nothing else happens.`, tone: "" };
    if (v.law.da_stance === "law_and_order") return { text: "A law-and-order DA does not take envelopes. This one goes in an evidence bag.", tone: "danger" };
    if (v.law.da_stance === "reform") return { text: "A reformer sends it back with no note. Nothing happens, and the money is gone.", tone: "warn" };
    return { text: `~${odds} they take it (${money(q("rules.law.d_a_price"))} is even odds). Refused, the money is gone and nothing else happens.`, tone: "" };
  }
  const price = q("rules.law.bribes").ChiefPrice;
  if (amt < price) return { text: `Under the price (${money(price)}): the chief's people pocket it and nothing changes.`, tone: "warn" };
  switch (chiefWord(v)) {
    case "zealous":
      return { text: `Chief ${v.law.chief} is zealous: this goes in an evidence bag, a page in the file and heat in the morning.`, tone: "danger" };
    case "lazy":
      return { text: "A lazy chief takes it at half the good: heat fades a little faster and the stings come a little later.", tone: "" };
    case "new":
      return { text: "You have not seen this chief work: a corrupt one takes it, a lazy one takes half the good, a zealous one files it.", tone: "" };
  }
  return { text: "A corrupt chief takes it: heat fades faster and stings and raids come later while it holds.", tone: "" };
}

// bribeSaid is the page's word once the envelope goes.
export function bribeSaid(v, target, amt) {
  return `${money(amt)} in an envelope for ${officialName(v, target)}. You hear tonight.`;
}

// dealWord is what a route's deal is called: a checkpoint on the road,
// a customs agent at the water; dealPrice what it costs.
export function dealWord(q, route) {
  return q("rules.logistics.customs", route) ? "customs agent" : "checkpoint";
}
export function dealPrice(q, route) {
  const b = q("rules.law.bribes");
  return q("rules.logistics.customs", route) ? b.CustomsPrice : b.CheckpointPrice;
}

const EDGE = { car: "by car", truck: "by truck", boat: "by boat", plane: "by plane" };

// checkpointLines are the confirm's paragraphs (checkpointConfirm):
// the deal, its price and what it cuts, the days already bought, the
// risk as the file knows it, and what ends it. Each [text, tone].
export function checkpointLines(v, q, r) {
  const tun = q("rules.law.bribes"),
    word = dealWord(q, r.id);
  const lines = [[`Buy the ${word} on ${the(r.name)} (${cityName(v, r.from)} to ${cityName(v, r.to)} ${EDGE[r.mode] || r.mode}) for ${money(dealPrice(q, r.id))}, dirty: ${pct(q("rules.logistics.deal_cut", r.id))} of the risk off every day on that edge for ${plural(tun.CheckpointDays, "day")}.`, ""]];
  if (r.checkpoint_until > v.day) lines.push([`Yours until day ${r.checkpoint_until} already; this adds to it.`, "subtle"]);
  const ship = ["slow", "normal", "fast"].includes(r.dial) ? r.dial : "normal";
  if (r.risk_known) lines.push([`Seized now: ~${pct(q("rules.logistics.risk_from", r.id, ship, r.risk))} a run at ${ship} · ~${pct(r.risk)} a day known.`, ""]);
  else lines.push(["Seized now: not known, no seizure or intel on this road yet.", "subtle"]);
  lines.push([`A law-and-order DA taking office ends it within ${plural(tun.CallsStopDays, "day")}, and nothing is for sale while they sit.`, "warn"]);
  if (cold(v)) lines.push(["A law-and-order DA sits: nobody is taking calls.", "danger"]);
  return lines;
}

// checkpointSaid is the page's word once the deal is bought, read off
// the view after it.
export function checkpointSaid(v, q, r, price) {
  return `The ${dealWord(q, r.id)} on ${the(r.name)} is yours until day ${r.checkpoint_until}: ${money(price)}. Risk on that edge cut ${pct(q("rules.logistics.deal_cut", r.id))} while it holds.`;
}

// payoffRows are the live deals (payoffRows): the chief, the DA, then
// the routes; the view carries a deal's day only while it holds. Each
// {who, what, until, route}.
export function payoffRows(v, q) {
  const rows = [];
  if (v.law.chief_bought)
    rows.push({ who: `Chief ${v.law.chief}`, what: v.law.chief_temper === "lazy" ? "half the good: a lazy chief" : "heat fades faster, stings and raids come later", until: v.law.chief_bought });
  if (v.law.da_bought) rows.push({ who: `DA ${v.law.da}`, what: "a thicker file to indict", until: v.law.da_bought });
  for (const r of v.routes || [])
    if (r.checkpoint_until)
      rows.push({ who: r.name, what: `${dealWord(q, r.id)}, risk cut ${pct(q("rules.logistics.deal_cut", r.id))}`, until: r.checkpoint_until, route: r.id });
  return rows;
}

// payoffNote is what the DA's office has heard, if a fixer is on the
// payroll to tell you.
export function payoffNote(v) {
  const f = v.crew.filter((m) => m.role === "fixer").sort((a, b) => b.skill - a.skill)[0];
  return f ? `${f.name} says the DA has ${plural(v.law.leads || 0, "lead")}.` : "";
}

// The cop (#45, ui/intel.go): a cop's word on the chief and the police
// here. `intel` is intel.toml's cop_price and cop_accuracy, which no
// rule serves (build.py's engine-info.js).

// copBlank is a blank amount: the price, or the dirty cash where that
// is less.
export function copBlank(v, intel) {
  return Math.min(intel.copPrice, v.you.dirty_cash);
}

// copAccuracy is how straight the word is at the amount
// (IntelTuning.Accuracy).
export function copAccuracy(intel, amt) {
  if (intel.copPrice <= 0 || amt >= intel.copPrice) return intel.copAccuracy;
  return (intel.copAccuracy * amt) / intel.copPrice;
}

// copLines are the dialog's paragraphs (viewPayCop).
export function copLines(v, intel, raw) {
  const r = readAmount(raw, copBlank(v, intel)),
    lines = [];
  if (r.err) lines.push([r.err, "danger"]);
  else if (r.amount > 0) lines.push([`Straight ~${pct(copAccuracy(intel, r.amount))} for ${money(r.amount)}.`, r.amount > v.you.dirty_cash ? "danger" : "gold"]);
  lines.push([`Buys what Chief ${v.law.chief} is like and the ${cityName(v, v.you.city)} police's next move: the rung they stand at and the first night they can fire. The price is ${money(intel.copPrice)} for ${pct(intel.copAccuracy)}; less money, less often. A wrong word is off by a rung or a few days.`, "subtle"]);
  lines.push(["The word is in the morning's report. A cop is not a bribe: nothing is filed.", "subtle"]);
  return lines;
}

// copSaid is the page's word once the cop is paid.
export function copSaid(intel, amt) {
  return `A cop takes ${money(amt)}. What they know of the chief and the police here is in the morning's report, ~${pct(copAccuracy(intel, amt))} straight.`;
}

// The endings (#554), as the TUI words them: the walk away's ways out
// with their terms, what is short and what each scores (ui/exit.go
// exitRows, crownShort, straightShort, streetWords, pendingLines,
// reignIncome, viewExit), the crown's count, its clocks and why each
// crew is off it (ui/rivals.go downWords, lastClock, lastClockWords),
// the plans' steps by unit, the PLAN line and what reset the quiet days
// (ui/ambitions.go), and the run summary (ui/summary.go). Each function
// takes the view, `q`, the engine's query (session.call), and `info`,
// engine-info.js's tuning where a number has no rule, and returns
// words, rows or numbers: it touches no DOM, so smoke.mjs checks them
// on a live run.

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n || 0)).toLocaleString("en-US"),
  pct = (x) => `${Math.round((x || 0) * 100)}%`;

// count is a number and its noun: `1 body`, `2 bodies`, `3 crews`.
export function count(n, w) {
  if (w === "body") return `${(n || 0).toLocaleString("en-US")} ${n === 1 ? "body" : "bodies"}`;
  if (w === "business") return `${(n || 0).toLocaleString("en-US")} ${n === 1 ? "business" : "businesses"}`;
  return `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`;
}
// reignDay is the reign's day today (World.ReignDay): 0 with no reign.
export function reignDay(v) {
  return v.you.reign > 0 ? v.day - v.you.reign + 1 : 0;
}

// plan is an ambition by its id: retire, legit, city, vanish.
export function plan(v, id) {
  return v.ambitions.find((a) => a.id === id);
}

// scoreRow is what every way out scores (#478): the account over one
// plus the bodies, whatever the ending.
export function scoreRow(v) {
  return v.you.bodies > 0 ? `the account over 1 + ${count(v.you.bodies, "body")}: ${money(v.you.score)}` : `the account: ${money(v.you.score)}`;
}

// scoreLine is the confirmation's: `Score $200,000: $600,000 over 1 + 2 bodies.`
export function scoreLine(v) {
  return `Score ${money(v.you.score)}: ${money(v.you.offshore)} over 1 + ${count(v.you.bodies, "body")}.`;
}

// scoreWords is how the summary's score was reached (#498), spelled for
// a beginner.
export function scoreWords(v) {
  let s = `the offshore account ${money(v.you.offshore)} ÷ (1 + ${count(v.you.bodies, "body")})`;
  if (v.you.offshore <= 0) s += ": nothing was moved offshore, and only the account scores";
  return s;
}

// pendingLines are what a walk away this morning would leave unsettled
// (#494, #525): the pages last night's lump files tonight, and a
// reserve made today that is not in the account until tonight. Each
// [text, tone].
export function pendingLines(v) {
  const out = [];
  if (v.you.pages_due > 0) out.push([`Last night's transfer offshore puts ${count(v.you.pages_due, "page")} in the DA's file tonight: end the day first.`, "danger"]);
  if (v.you.reserved_today > 0) out.push([`${money(v.you.reserved_today)} lands offshore tonight: end the day first, or it is neither scored nor left behind.`, "warn"]);
  return out;
}

// streetWords is what going straight measures the fronts against
// (#529): the street's average night over the file's window.
export function streetWords(info) {
  return info.streetWindow > 0 ? `the street's average night over the last ${info.streetWindow}` : "the street's average night over the run";
}

// straightShort is what going straight waits on (#498), in the legit
// plan's own steps: the nights so far, and whichever of the fronts
// over the street and goodwill over pressure fails this morning.
export function straightShort(v) {
  const a = plan(v, "legit"),
    streak = a?.steps.find((s) => s.id === "streak"),
    parts = [`${streak?.have ?? 0} of ${count(streak?.need ?? 0, "night")} so far`];
  for (const s of a?.steps || []) {
    if (s.done) continue;
    if (s.id === "income") parts.push(`the fronts' ${money(s.have)}/day under the street's ${money(s.need)} a night`);
    else if (s.id === "goodwill") parts.push("goodwill under pressure");
  }
  return parts.join(", ");
}

// claimWords is what stops a run-out faction's clock (#495).
export function claimWords(q) {
  const n = q("rules.rivals.factions").SettleDays;
  return n > 0 ? `keeps a claim ${n}d` : "claims again";
}

// downWords is what keeps a faction off the crown's count of crews down
// and for how long (#472, rivals.Sim.Down); "" for one that counts.
export function downWords(v, q, f) {
  const d = q("rules.rivals.down", f.id),
    day = v.day,
    inDays = (on) => (on <= day + 1 ? "tonight" : `in ${on - day}d`),
    ago = (since) => (since > day ? "run out" : since === day ? "run out today" : `run out ${day - since}d ago`),
    strand = q("rules.rivals.factions").StrandDays;
  if (d.Counts) return "";
  if (d.Scouting) return "on its way; it counts once it is here and down";
  if (d.Due > day + 1) return `not here yet: may move in from day ${d.Due}`;
  if (d.Due > 0 && d.GoneOn > 0) return "waiting for room to move in; stands down " + inDays(d.GoneOn) + " if it finds none";
  if (d.Due > 0) return "waiting for room to move in";
  if (d.Corners > 0 && d.Settles > 0)
    return `holds ${count(d.Corners, "corner")}; ${ago(d.Since)}, the clock runs on unless it holds one ${inDays(d.Settles).replace(/^in /, "")} more`;
  if (d.Corners > 0) return `holds ${count(d.Corners, "corner")}; no clock while it holds one`;
  if (d.Rich && d.GoneOn > 0 && (strand <= 0 || d.GoneOn < d.Since + strand))
    return `${ago(d.Since)}, living off its chest; broke and gone ${inDays(d.GoneOn)} unless it ${claimWords(q)}`;
  if (d.Rich && d.GoneOn > 0) return `${ago(d.Since)}, can afford a claim; gone ${inDays(d.GoneOn)} unless it ${claimWords(q)}, sooner if broke`;
  if (d.Rich) return `${ago(d.Since)}, can afford a claim; gone once it cannot`;
  return `${ago(d.Since)}; gone ${inDays(d.GoneOn)} unless it ${claimWords(q)}`;
}

// lastClock is the day the last faction's clock runs out (#530), when
// every faction off the count has one; 0 otherwise.
export function lastClock(v, q) {
  let last = 0;
  for (const f of v.factions) {
    const d = q("rules.rivals.down", f.id);
    if (d.Counts) continue;
    if (d.GoneOn <= 0) return 0;
    last = Math.max(last, d.GoneOn);
  }
  return last;
}

// lastClockWords is the crown's countdown (#530): `the last clock runs
// out on day 114 (in 12d), then 14 days held`, or "".
export function lastClockWords(v, q) {
  const day = lastClock(v, q);
  if (!day) return "";
  let s = `the last clock runs out on day ${day}`;
  if (day - v.day > 1) s += ` (in ${day - v.day}d)`;
  const hold = plan(v, "city")?.steps.find((x) => x.id === "streak")?.need || 0;
  if (hold > 0) s += `, then ${count(hold, "day")} held`;
  return s;
}

// downRows are the crown's crews off the count (#472): each faction
// that does not count yet, with what keeps it off and for how long.
export function downRows(v, q) {
  return v.factions.map((f) => ({ id: f.id, name: f.leader || "a crew to come", words: downWords(v, q, f) })).filter((r) => r.words);
}

// crownShort is what the crown waits on (#399), in the city plan's own
// steps: the corners short of the share at home, the crews standing and
// still to arrive, the last clock, the days of the streak; or that the
// reign is slipping.
export function crownShort(v, q) {
  if (v.you.reign > 0 && v.you.reign_slip > 0) return "the city is slipping under the share: hold more corners";
  const parts = [],
    a = plan(v, "city"),
    home = v.cities[0];
  for (const s of a?.steps || []) {
    if (s.done) continue;
    if (s.id === "share") parts.push(`${Math.round(s.have)} held of ${home.corners.length} in ${home.name}, ${Math.round(s.need)} needed`);
    else if (s.id === "factions") {
      const coming = v.factions.filter((f) => !f.arrived && !q("rules.rivals.down", f.id).Counts).length,
        standing = Math.round(s.need - s.have) - coming;
      if (standing > 0) parts.push(count(standing, "crew") + " still standing");
      if (coming > 0) parts.push(count(coming, "crew") + " yet to arrive");
      const last = lastClockWords(v, q);
      if (last) parts.push(last);
    } else if (s.id === "streak" && (s.have > 0 || !parts.length)) parts.push(`day ${Math.round(s.have)} of ${Math.round(s.need)}`);
  }
  return parts.length ? parts.join(", ") : "the city is not yours";
}

// reignIncome is what the reign pays a night (#537): the homage the
// crews pay and the tax the free corners of the cities you hold pay.
export function reignIncome(v, q) {
  let crews = 0,
    homage = 0,
    tax = 0,
    corners = 0;
  for (const f of v.factions)
    for (const d of f.deal_terms || [])
      if (d.kind === "homage") {
        crews++;
        homage += d.terms.per_day || 0;
      }
  for (const c of v.cities) {
    const t = q("rules.territory.tax_due", c.id);
    corners += t.corners;
    tax += t.amount;
  }
  const parts = [];
  if (crews > 0) parts.push(`${money(homage)} homage from ${count(crews, "crew")}`);
  if (corners > 0) parts.push(`~${money(tax)} tax off ${count(corners, "free corner")}`);
  return parts.length ? `~${money(homage + tax)} a night (${parts.join(", ")})` : "every crew gone and nothing paid a night";
}

// exitRows are the four ways out (exitRows): each its action, name,
// terms, whether it is open and what is short; every one closed while
// a transfer's pages are still to be read (#494, #525).
export function exitRows(v, q, info) {
  const off = q("rules.laundering.offshore"),
    retire = { action: "retire", cause: "retired", name: "Retire", open: !!q("rules.laundering.can_retire") };
  retire.terms = `${money(off.RetireCash)} offshore and ${count(off.RetireDays, "day")} quiet`;
  const parts = [];
  if (off.RetireCash - v.you.offshore > 0) parts.push(money(off.RetireCash - v.you.offshore) + " short");
  if (off.RetireDays > v.you.quiet_days) parts.push(`${v.you.quiet_days} of ${count(off.RetireDays, "quiet day")}`);
  retire.short = parts.join(", ");
  const vanish = { action: "vanish", cause: "vanished", name: "Vanish", terms: "a new identity from the tree, after the lawyer on call and on retainer", open: v.you.upgrades.includes("identity") };
  if (!vanish.open) vanish.short = "no new identity";
  const city = plan(v, "city"),
    streak = city?.steps.find((s) => s.id === "streak"),
    crown = { action: "crown", cause: "kingpin", name: "Take the crown", open: !!city?.done };
  crown.terms = `more than ${pct(info.kingpinShare)} of home's corners and every crew gone or paying, ${count(streak?.need || 0, "day")} running`;
  if (crown.open) crown.terms = `day ${reignDay(v)} of the reign`;
  else crown.short = crownShort(v, q);
  const straight = { action: "go_straight", cause: "businessman", name: "Go straight", open: !!q("rules.laundering.can_go_straight") },
    days = plan(v, "legit")?.steps.find((s) => s.id === "streak")?.need || 0;
  if (straight.open) straight.terms = `the fronts at ${money(q("rules.laundering.legit_income"))} a day`;
  else {
    straight.terms = `the fronts out-earn ${streetWords(info)} and goodwill tops pressure at home, ${count(days, "night")} running`;
    straight.short = straightShort(v);
  }
  const rows = [retire, vanish, crown, straight];
  for (const r of rows) {
    if (!r.open) continue;
    if (v.you.pages_due > 0) {
      r.open = false;
      r.short = `${count(v.you.pages_due, "page")} from last night's transfer ${v.you.pages_due === 1 ? "goes" : "go"} in the DA's file tonight: end the day first`;
    } else if (v.you.pages_pending > 0) {
      r.open = false;
      r.short = `today's transfer puts ${count(v.you.pages_pending, "page")} in the DA's file: end the day first, and walk away once it is read`;
    }
  }
  return rows;
}

// exitConfirm is a way out's confirmation (viewExit), each in its own
// words, then what would be left unsettled, the score and what the run
// leaves behind. Each [text, tone].
export function exitConfirm(v, q, action) {
  const off = money(v.you.offshore),
    stock = Object.values(v.you.stock || {}).reduce((n, c) => n + Object.values(c).reduce((m, x) => m + x, 0), 0);
  const words = {
    retire: `Retire on ${off} offshore, ${count(v.you.quiet_days, "day")} quiet. Nobody comes looking. The run ends now, on day ${v.day}.`,
    crown: `Take the crown on day ${reignDay(v)} of the reign: ${reignIncome(v, q)}, ${off} offshore. The city stays yours in the epilogue; the run ends now, on day ${v.day}.`,
    go_straight: `Go straight on ${count(v.fronts.length, "front")}: the fronts at ${money(q("rules.laundering.legit_income"))} a day, the street given up, ${off} offshore. The DA's file goes to the archive; the run ends now, on day ${v.day}.`,
    vanish: `Vanish on the new identity with ${off} offshore. The DA keeps looking; the papers are good. The run ends now, on day ${v.day}.`,
  };
  return [
    [words[action], ""],
    ...pendingLines(v),
    [scoreLine(v), "gold"],
    [`Left behind: ${money(v.you.dirty_cash)} dirty, ${money(v.you.clean_cash)} clean, ${count(stock, "unit")} in stock, ${count(v.crew.length, "member")}.`, "subtle"],
  ];
}

// walkAwayNote is the walk away's closing line: whatever you leave
// with, the account over one plus the bodies is the score.
export function walkAwayNote(v) {
  return `The account holds ${money(v.you.offshore)} and ${count(v.you.quiet_days, "day")} quiet. Whatever you leave with, the run ends this morning: the pile, the stock, the crew and the fronts stay behind, and the account over one plus the bodies is the score.`;
}

// The plans (ui/ambitions.go).

// stepWords is a step's reading by its unit: `$412,000 of $750,000`,
// `3 of 14 days`, `day 5 of the reign`, `40 against 62`.
export function stepWords(st) {
  const have = Math.trunc(st.have),
    need = Math.trunc(st.need);
  switch (st.unit) {
    case "cash":
      return `${money(have)} of ${money(need)}`;
    case "clean":
    case "dirty":
      return st.done ? "owned" : `${money(need)} ${st.unit}, ${money(have)} in hand`;
    case "days":
      return `${have} of ${count(need, "day")}`;
    case "reign":
      return `day ${have} of the reign`;
    case "points":
      return `${Math.round(st.have)} against ${Math.round(st.need)}`;
    case "income":
      return `${money(have)}/day against the street's ${money(need)} a night`;
  }
  return `${have} of ${need}`;
}

// frac is how far along a step is (game.AmbitionStep.Frac).
function frac(st) {
  if (st.done) return 1;
  if (st.need <= 0) return 0;
  return Math.max(0, Math.min(1, st.have / st.need));
}

// stepPart is a step as a part of the plan's line (#465): a count as
// `14/14`, the reign by its day, anything else as a share.
export function stepPart(st) {
  if (["days", "corners", "count"].includes(st.unit)) return `${st.label} ${Math.trunc(st.have)}/${Math.trunc(st.need)}`;
  if (st.unit === "reign") return `${st.label} ${stepWords(st)}`;
  return `${st.label} ${pct(frac(st))}`;
}

// planParts is the plan's steps, joined: `the account 0% · quiet days 14/14`.
export function planParts(a) {
  return a.steps.map(stepPart).join(" · ");
}

// doneWord is what a plan met says: an ending is ready, a milestone made.
export function doneWord(a) {
  return a.ending ? "ready" : "made";
}

// quietCause is what broke a quiet streak (#465, #519), off the
// QuietBroken event's payload: `a sting in Eastside on day 41`.
export function quietCause(v, ev) {
  const c = v.cities.find((x) => x.id === ev.City),
    where = c ? " in " + c.name : "";
  const what = {
    heat: "heat at the retire line" + where,
    police: `a ${ev.Level === "taskforce" ? "task force" : ev.Level}${where}`,
    strike: "your strike on a corner",
    push: "a push on your corners",
    war: "the war getting loud",
    contract: "a buyer's contract still open" + where,
  }[ev.Cause];
  return what ? `${what} on day ${ev.Day}` : "";
}

// quietReset is what last broke the quiet streak, while the plan is
// Retire clean and its quiet days are short; "" otherwise. A page
// reloaded forgets it, as the TUI's status bar does.
export function quietReset(v, a, ev) {
  if (!ev || a.id !== "retire" || a.steps.some((s) => s.id === "quiet" && s.done)) return "";
  return quietCause(v, ev);
}

// planLine is the PLAN line, the report's (planReport): the pinned
// plan's steps and its next one in words; "" with none pinned.
export function planLine(v, ev) {
  const a = v.ambitions.find((x) => x.pinned);
  if (!a) return "";
  if (a.done) return a.ending ? `${a.name}: ready. Walk away from the Ledger to take it, or play on.` : `${a.name}: ${doneWord(a)}.`;
  let line = `${a.name} ${pct(a.progress)}: ${planParts(a)}`;
  const why = quietReset(v, a, ev);
  if (why) line += ", the quiet days reset by " + why;
  const next = a.steps.find((s) => s.id === a.next);
  if (next) line += `. Next, ${next.label}: ${stepWords(next)}`;
  return line + ".";
}

// The run summary (ui/summary.go).

// bodiesLine is the bodies on both sides and yours (#46).
export function bodiesLine(v) {
  return v.you.bodies ? `${v.you.bodies}, ${v.stats.fallen} of them yours` : "none";
}

// fallenLine names the fallen: `Ziggy (runner · day 3)`.
export function fallenLine(v) {
  return v.fallen.map((f) => `${f.name} (${f.role} · day ${f.day})`).join(", ");
}

// betrayalsLine counts who turned on you (#465), `none` with nobody.
export function betrayalsLine(v) {
  const s = v.stats,
    parts = [];
  if (s.informants > 0) parts.push(count(s.informants, "informant"));
  if (s.defections > 0) parts.push(count(s.defections, "defector"));
  if (s.crew_poached > 0) parts.push(`${count(s.crew_poached, "member")} poached`);
  if (s.walked > 0) parts.push(`${count(s.walked, "lieutenant")} walked`);
  if (s.betrayed_by > 0) parts.push(`${count(s.betrayed_by, "deal")} broken by them`);
  if (s.betrayals > 0) parts.push(`${count(s.betrayals, "deal")} broken by you`);
  return parts.length ? parts.join(" · ") : "none";
}

// bestCrew is the first of the fallen, else the member whose tenure
// times loyalty is highest; "" with nobody.
export function bestCrew(v) {
  if (v.fallen.length) {
    const f = v.fallen[0];
    return `${f.name}, ${f.role}, fell on day ${f.day}`;
  }
  let best = null,
    score = -1;
  for (const c of v.crew) {
    const s = (v.day - c.hired + 1) * c.loyalty;
    if (s > score) [best, score] = [c, s];
  }
  return best ? `${best.name}, ${best.role}, ${count(Math.max(1, v.day - best.hired), "day")} on the payroll at loyalty ${Math.round(best.loyalty)}` : "";
}

// owedLine is the debt still on the connects' books (#518), or "".
export function owedLine(v) {
  return v.connects
    .filter((c) => c.debt > 0)
    .map((c) => `${money(c.debt)} to ${c.name}, due day ${c.debt_due}`)
    .join(" · ");
}

// reachedLine is the tier reached and the day it was entered (#147); a
// kingpin's is the reign's first morning (#227).
export function reachedLine(v) {
  const o = v.over;
  if (o.cause === "kingpin" && v.you.reign > 0) return `Kingpin on day ${o.reached}`;
  return o.reached > 0 ? `${v.you.tier_name} on day ${o.reached}` : v.you.tier_name;
}

// summarySections are the summary's facts (summaryLines): THE MONEY,
// THE PEOPLE and THE CITY, each [label, text] rows.
export function summarySections(v) {
  const s = v.stats,
    stock = Object.values(v.you.stock || {}).reduce((n, c) => n + Object.values(c).reduce((m, x) => m + x, 0), 0);
  const moneyRows = [
    ["offshore", `${money(v.you.offshore)} · the score`],
    ["left behind", `${money(v.you.dirty_cash)} dirty · ${money(v.you.clean_cash)} clean · ${count(stock, "unit")} in stock`],
    ["peak wealth", `${money(v.you.peak_cash)} · revenue ${money(s.revenue)} off ${count(s.units_sold, "unit")}`],
    ["washed", `${money(s.laundered)}, ${money(s.seized)} seized · lost ${money(s.wages)} wages · ${money(s.skimmed)} skimmed · ${money(s.robbed)} robbed`],
  ];
  const owed = owedLine(v);
  if (owed) moneyRows.push(["owed", owed]);
  if (s.cuts > 0) moneyRows.push(["cuts", `${money(s.cuts)} kept by the crew who ran it for you, a greedy lieutenant's take with it, apart from the skim`]);
  if (s.earned + s.invested > 0) moneyRows.push(["the fronts", `${money(s.invested)} in levels, ${money(s.earned)} earned`]);
  if (s.taxed > 0) moneyRows.push(["the tax", `${money(s.taxed)} off the free corners of a city you held`]);
  const people = [["bodies", bodiesLine(v)]];
  if (v.fallen.length) people.push(["fallen", fallenLine(v)]);
  people.push(["betrayals", betrayalsLine(v)]);
  const best = bestCrew(v);
  if (best) people.push(["best of them", best]);
  const held = v.cities.reduce((n, c) => n + c.corners.filter((k) => k.owner === "player").length, 0),
    chief = v.law.chief_temper && v.law.chief_temper !== "?" ? v.law.chief_temper : "?",
    stance = v.law.da_stance === "law_and_order" ? "law-and-order" : v.law.da_stance;
  const city = [
    ["reputation", `fear ${Math.round(v.you.fear)} · respect ${Math.round(v.you.respect)} · notoriety ${Math.round(v.you.notoriety)}`],
    ["ground", `${count(held, "corner")} held · ${s.corners_won} won · ${s.corners_lost} lost · ${count(s.stings, "sting")} · ${count(s.raids, "raid")}`],
    ["the law", `Chief ${v.law.chief} (${chief}) · DA ${v.law.da} (${stance}) · peak heat ${Math.round(s.peak_heat)}`],
  ];
  return [
    { title: "THE MONEY", rows: moneyRows },
    { title: "THE PEOPLE", rows: people },
    { title: "THE CITY", rows: city },
  ];
}

// The rivals' table (#554), as the TUI words it: a faction's line, its
// deals and their terms and days left, where you stand, what keeps it
// off the crown's count (ui/rivals.go), the propose dialog's standard
// lengths and cuts with the dice's odds, withdraw, the tribute basis
// and the distrust (ui/diplomacy.go), and the war order's declaration
// and stand-down with their confirms (ui/rivals.go askWar, warConfirm,
// callOffWarConfirm). Each function takes the view, `q`, the engine's
// query (session.call), and `info`, engine-info.js's tuning where a
// number has no rule; it touches no DOM, so smoke.mjs checks them on a
// live run.

import { fixed, money, pct } from "./format.js?v=__BUILD_REVISION__";

const count = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`,
  pctBand = (lo, hi) => (pct(lo) === pct(hi) ? pct(lo) : `${fixed(lo * 100, 0)}–${pct(hi)}`),
  cornerOf = (v, id) => v.cities.flatMap((c) => c.corners).find((c) => c.id === id);

// rivalName is how a faction is named everywhere: `Big Sal's crew`, or
// "the rival" for one with no leader yet.
export function rivalName(f) {
  return f?.leader ? `${f.leader}'s crew` : "the rival";
}

// gone is a faction absorbed, scattered or leaderless (RivalState.Gone).
export function gone(f) {
  return f.absorbed > 0 || f.fragmented > 0;
}

// muscleWord is a faction's muscle as the file holds it: `4`, `3–6`, `?`.
export function muscleWord(f) {
  if (!f.muscle_known) return "?";
  return f.muscle_lo === f.muscle_hi ? String(f.muscle_lo) : `${f.muscle_lo}–${f.muscle_hi}`;
}

// oddsWord is a strike's odds at a force on a corner as the file lets
// you read them (#464): `~40%`, `~30–45%`, or `?`.
export function oddsWord(q, f, corner, force) {
  if (!f.muscle_known) return "?";
  return "~" + pctBand(q("rules.rivals.odds_on_at", f.id, corner, force, f.muscle_hi), q("rules.rivals.odds_on_at", f.id, corner, force, f.muscle_lo));
}

// eyeingWord is the tell (#69): `eyeing Riverside`, or "".
export function eyeingWord(v, q, f) {
  const c = cornerOf(v, q("rules.rivals.eyeing_by", f.id));
  return c && c.owner === "none" ? "eyeing " + c.name : "";
}

// cornersWord is how much of the city a faction holds, in words.
export function cornersWord(f) {
  return f.corners > 0 ? count(f.corners, "corner") : "run out of town";
}

// factionLine is the faction's line under its name: `aggressive · 3
// corners · muscle 4 · eyeing Riverside`; a gone one's stance alone,
// and one in the wings not moved in yet.
export function factionLine(v, q, f) {
  if (!f.arrived) return "not moved in yet";
  const parts = gone(f) ? [f.stance] : [f.personality === "?" ? "unknown" : f.personality, cornersWord(f), "muscle " + muscleWord(f)],
    eye = eyeingWord(v, q, f);
  if (eye) parts.push(eye);
  return parts.join(" · ");
}

// dealTerms is a deal's terms for a row, who pays whom said (#537).
export function dealTerms(v, d) {
  const t = d.terms || {};
  switch (d.kind) {
    case "truce":
      return count(t.days, "day");
    case "tribute":
      return money(t.per_day) + " a day from you";
    case "homage":
      return money(t.per_day) + " a day to you";
    case "split":
      return "yours " + side(v, t.corners);
  }
  return d.kind;
}

// side is a split's corners by name: `Rail Yard, Docks`.
export function side(v, corners) {
  return (corners || []).map((id) => cornerOf(v, id)?.name || id).join(", ");
}

// dealRows are a faction's live deals (the DEALS table): the kind, the
// terms, the days left (blank for one with no end) and whose.
export function dealRows(v, f) {
  return (f.deal_terms || []).map((d) => ({ kind: d.kind, terms: dealTerms(v, d), left: d.until > 0 ? d.left : null, who: d.theirs ? "theirs" : "yours", since: d.since }));
}

// dealDoes is what a deal of the kind does while it holds.
export function dealDoes(kind) {
  return (
    {
      truce: "Neither side pushes; no undercutting, no tips, for the term.",
      tribute: "You pay the cut each night; they leave every corner of yours alone until you stop.",
      split: "They neither claim nor push on your side of the line, and you post nobody past it.",
      homage: "They pay you the cut each night out of their chest and stay off your corners; it ends when they cannot pay.",
    }[kind] || "Half the cost of a run, half the loss."
  );
}

// dealBreaks is what breaks a deal of the kind and what that costs.
export function dealBreaks(kind) {
  const how = { tribute: "A missed night, a push or a hit", split: "Walking off a split corner, a push or a hit" }[kind] || "A push or a hit under it";
  return how + " breaks it: trust hits the floor and they call the police.";
}

// DEAL_RULES are the rivals pane's RULES lines.
export const DEAL_RULES = [
  "Truce, tribute: off your corners. Split: off your side.",
  "A push or a hit under a deal breaks it: trust hits the floor and they make a call.",
  "So does a missed tribute, or walking off a split corner; the others hear of it.",
];

// distrusted is a faction refusing everything after a betrayal, as of
// tonight (rivals.Sim.Distrusted).
export function distrusted(v, q, f) {
  return !!q("rules.rivals.distrusted", f.id, v.day + 1);
}

// moodLine is where you stand with a faction: what trust buys you, or
// how long a betrayal keeps them off the phone ([text, tone]).
export function moodLine(v, q, f) {
  if (f.absorbed > 0 && !f.absorbed_by) return [`Scattered on day ${f.absorbed}. There is nobody left to talk to.`, ""];
  if (f.absorbed > 0) return [`Absorbed on day ${f.absorbed}. There is nobody left to talk to.`, ""];
  if (f.fragmented > 0) return [`Leaderless since day ${f.fragmented}. Their corners go back to the street.`, ""];
  if (distrusted(v, q, f)) return [`You broke a deal. They take nothing for ${count(f.betrayed + q("rules.rivals.diplomacy").DistrustDays - v.day - 1, "more day")}.`, "danger"];
  if (f.trust >= 60) return ["They take you at your word. A deal is cheap to strike.", ""];
  if (f.trust < 20) return ["They do not trust you. Keep a deal a while and that changes.", ""];
  return ["Trust grows a little every day a deal holds and falls with every strike.", ""];
}

// cityWord is where a faction's street is: `here`, or `in Bayport`.
export function cityWord(v, f) {
  return f.city && f.city !== v.you.city ? "in " + (v.cities.find((c) => c.id === f.city)?.name || f.city) : "here";
}

// tributeBasis is what a tribute is a cut of, with its number (#532).
export function tributeBasis(v, q, f) {
  const base = money(q("rules.rivals.tribute_base", f.id));
  return f.tribute_nights > 0
    ? `${base} a day, what your corners ${cityWord(v, f)} sold for a night over the last ${count(f.tribute_nights, "night")}, before the crew's cut.`
    : `${base} a day, what your corners ${cityWord(v, f)} could move at today's prices, not what they sold.`;
}

// tributeCut is a tribute's cut of your street today, as the pane says it.
export function tributeCut(q, f, perDay) {
  const base = q("rules.rivals.tribute_base", f.id);
  return base > 0 ? `~${Math.round((100 * perDay) / base)}% of your street` : "-";
}

// PROPOSE_KINDS are the propose dialog's first page: the three deals
// that can be struck, and the joint shipment, listed but waiting on
// routes.
export const PROPOSE_KINDS = [
  ["truce", "neither side pushes; no undercutting, no tips, for a term"],
  ["tribute", "you pay a cut a day; they leave your corners alone, until you stop"],
  ["split", "a line through the city; each side keeps to its own"],
  ["shipment", "half the cost of a run, half the loss (needs routes)"],
];

// liveDeal is the deal of a kind live with a faction tonight, or null.
export function liveDeal(v, f, kind) {
  return (f.deal_terms || []).find((d) => d.kind === kind && (!d.until || v.day + 1 < d.until)) || null;
}

// termRows are the dialog's second page for a kind (termRows): the
// three standard asks, each its deal, its words and the dice's odds.
export function termRows(v, q, f, kind) {
  const dip = q("rules.rivals.diplomacy"),
    rows = [];
  if (kind === "truce")
    dip.TruceDays.forEach((d, i) => rows.push({ deal: { kind, terms: { days: d } }, label: `${d} days`, words: ["short", "standard", "long"][i] }));
  else if (kind === "tribute")
    dip.TributeCuts.forEach((c, i) => {
      const perDay = q("rules.rivals.cut", f.id, c);
      rows.push({ deal: { kind, terms: { per_day: perDay } }, label: `${money(perDay)}/day`, words: pct(c) + " of your street" + [" (thin)", "", " (fat)"][i] });
    });
  else if (kind === "split")
    (f.split_lines || []).forEach((line, i) =>
      rows.push({ deal: { kind, terms: { corners: line } }, label: count(line.length, "corner"), words: ["what you hold", "plus the free corners on your side", "plus every free corner"][i], side: side(v, line) }),
    );
  for (const r of rows) r.odds = q("rules.rivals.chance", f.id, r.deal);
  return rows;
}

// proposeRefusal is why a kind cannot be proposed to a faction, or "".
export function proposeRefusal(v, f, kind) {
  if (kind === "shipment") return "Can't propose a shipment: joint shipments need routes, and there are none yet.";
  const d = liveDeal(v, f, kind);
  return d ? `Can't propose that: you already have ${dealWords(v, d)}.` : "";
}

// dealWords is a deal in words, who pays whom said (World.Describe).
export function dealWords(v, d) {
  const t = d.terms || {};
  switch (d.kind) {
    case "truce":
      return `a ${t.days}-day truce`;
    case "tribute":
      return `tribute: you pay them ${money(t.per_day)} a day`;
    case "homage":
      return `homage: they pay you ${money(t.per_day)} a day`;
    case "split":
      return `a split: ${count((t.corners || []).length, "corner")} your side of the line (${side(v, t.corners)})`;
    case "shipment":
      return `a joint shipment of ${(t.units || 0).toLocaleString("en-US")} units`;
  }
  return d.kind;
}

// askProposeRefusal is why the dialog does not open, or "".
export function askProposeRefusal(f) {
  if (!f.arrived) return "Nothing to propose: nobody is contesting the city yet.";
  if (gone(f)) return `Nothing to propose: ${rivalName(f)} is no more. Turn to another faction.`;
  return "";
}

// proposalLine is tonight's proposal (the rivals screen's Tonight
// line): what, to whom, and the odds the dice use.
export function proposalLine(v, q) {
  const p = v.proposal;
  if (!p) return "";
  const to = v.factions.find((f) => f.id === p.faction) || v.factions[0];
  return `Tonight you propose ${dealWords(v, p)} to ${to.leader}; they answer in the morning, ~${pct(q("rules.rivals.chance", to.id, { kind: p.kind, terms: p.terms }))}`;
}

// replaceLines is the question before tonight's proposal is replaced (#536).
export function replaceLines(v, f, d) {
  const p = v.proposal,
    to = v.factions.find((x) => x.id === p.faction);
  return `Tonight you have proposed ${dealWords(v, p)} to ${rivalName(to)}. One proposal goes a night: ${dealWords(v, d)} to ${rivalName(f)} takes its place, and the first is never answered.`;
}

// proposedSaid is the status after a proposal: what, to whom, the odds,
// and the one it replaced (#506).
export function proposedSaid(v, q, f, d, before) {
  let s = `Proposed ${dealWords(v, d)} to ${rivalName(f)}. They answer in the morning; odds ~${pct(q("rules.rivals.chance", f.id, d))}.`;
  if (before) s += ` It replaces ${dealWords(v, before)} to ${rivalName(v.factions.find((x) => x.id === before.faction))}: one a night.`;
  return s;
}

// The war order (#229, docs/rival.md).

const working = (m) => !m.jailed && !m.wounded,
  enforcers = (v) => v.crew.filter((m) => m.role === "enforcer");

// warRefusal is why war cannot be declared on a faction (askWar), or "".
export function warRefusal(v, f) {
  const at = v.factions.find((x) => x.id === v.you.war);
  if (v.you.war) return `Can't declare war on ${rivalName(f)}: one war at a time, and the enforcers are on ${rivalName(at)}. Call it off on their card first.`;
  if (!f.arrived) return "Nothing to fight: nobody is contesting the city yet.";
  if (gone(f)) return `Nothing to fight: ${rivalName(f)} is no more. Turn to another faction.`;
  if (!enforcers(v).length) return "Nobody to send: no enforcers. Hire one on the Crew tab.";
  const ground = v.cities.some((c) => c.corners.some((k) => k.owner === "rival" && k.faction === f.id) && c.corners.some((k) => k.owner === "player"));
  if (!ground) return `Nowhere to go: ${rivalName(f)} holds no corner in a city you hold ground in.`;
  return "";
}

// takenOut is which corner loss ends the run taken out (#520).
const TAKEN_OUT = "your last corner, taken by the crew you are at war with,";

// warMuscleLine is the taken-out warning (#478), or "" with muscle
// enough.
export function warMuscleLine(v, info) {
  const need = info.takenOutMuscle;
  if (need <= 0 || enforcers(v).length >= need) return "";
  return `With fewer than ${count(need, "enforcer")} on the payroll, ${TAKEN_OUT} ends the run.`;
}

// warNobodyWords is what a war does with no enforcer at work to send
// (#506), or "".
export function warNobodyWords(v) {
  if (!v.you.war || enforcers(v).some(working)) return "";
  return "no enforcer at work, so nobody goes in tonight and nothing is taken; they can still push you. Hire one on the Crew tab.";
}

// tonightLine is where the enforcers go tonight (the TUI dashboard's
// strike fact): the strike you queued, `{corner, force}` off the page's
// memo since the view does not carry World.Today.Strike, named by the
// corner; else the war order's corner and force, nobody to send, or
// nowhere to go; "" with neither.
export function tonightLine(v, q, info, strike) {
  if (strike) {
    const c = cornerOf(v, strike.corner);
    return c ? `Enforcers go to ${c.name} tonight: ${strike.force}.` : "";
  }
  const f = v.you.war && v.factions.find((x) => x.id === v.you.war);
  if (!f) return "";
  const who = rivalName(f),
    nobody = warNobodyWords(v);
  if (nobody) return `War on ${who}: ${nobody}`;
  const c = cornerOf(v, q("rules.rivals.war_target", f.id));
  return c ? `War on ${who}: enforcers go to ${c.name} tonight, ${info.warForce}.` : `War on ${who}: nowhere to go tonight.`;
}

// atPeace is a truce, a tribute or a homage live with the faction.
export function atPeace(v, f) {
  return ["truce", "tribute", "homage"].some((k) => liveDeal(v, f, k));
}

// warConfirm is the declaration's body (warConfirm): the target, the
// force, the odds and the heat, a deal it breaks and the taken-out
// line. Each [text, tone].
export function warConfirm(v, q, info, f) {
  const force = info.warForce,
    id = q("rules.rivals.war_target", f.id),
    c = cornerOf(v, id);
  const lines = [
    [
      `Every night you send nobody yourself, the enforcers ${force} ${rivalName(f)}'s nearest corner: ${c ? c.name : "none"} tonight, odds ${c ? oddsWord(q, f, id, force) : "?"}, heat +${c ? Math.round(q("rules.rivals.strike_heat", id, force)) : 0}. The same roll, the same toll on the crew, until they fold, bow or hold nothing left where you do.`,
      "",
    ],
  ];
  if (atPeace(v, f)) lines.push(["You have a deal with them: the first night breaks it, and the table remembers.", "danger"]);
  const muscle = warMuscleLine(v, info);
  if (muscle) lines.push([muscle, "danger"]);
  lines.push(["The war ends on its own when there is nothing left to take, or when you call it off here.", "subtle"]);
  return lines;
}

// warSaid is the status after declaring it.
export function warSaid(f) {
  return `War on ${rivalName(f)}: the enforcers go in every night from tonight.`;
}

// callOffLines is the stand-down's body.
export function callOffLines(f) {
  return `Call off the war on ${rivalName(f)}: the enforcers stand down from tonight. What was taken stays taken; the grudge stays too.`;
}

// calledOffSaid is the status after it.
export function calledOffSaid(f) {
  return `The war on ${rivalName(f)} is off. The enforcers stay home tonight.`;
}

// lifetimeRows are the pane's LIFETIME: the deals struck and refused,
// broken by either side, the tribute paid and the homage taken.
export function lifetimeRows(v) {
  const s = v.stats,
    rows = [
      ["struck", `${s.deals} · refused ${s.deals_refused}`],
      ["broken", `by you ${s.betrayals}, by them ${s.betrayed_by}`],
      ["tribute", money(s.tribute) + " paid"],
    ];
  if (s.homage > 0) rows.push(["homage", money(s.homage) + " to you"]);
  return rows;
}

// alliesLine names the defensive factions the expansionist pushed
// toward you (#43), or "".
export function alliesLine(v, q) {
  return (q("rules.rivals.allies") || []).map((id) => v.factions.find((f) => f.id === id)?.leader || id).join(", ");
}

// The crew in words (#551): the TUI's crew screen and its confirms
// (ui/crew.go, ui/life.go) as Street Edition shows them, every number
// off the view, the rules the page quotes, or engineInfo. Pure
// functions of what they are handed, so smoke.mjs runs them against the
// engine; they touch no DOM and return text, never HTML.

const money = (n) => "$" + Math.round(n || 0).toLocaleString("en-US"),
  plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`,
  round = (x) => Math.round(x),
  shown = (l) => Math.floor(l); // loyalty as the roster shows it (ui loyaltyShown)

// and joins names the TUI's way: "A", "A and B", "A, B and C".
function and(names) {
  return names.length < 2 ? names.join("") : `${names.slice(0, -1).join(", ")} and ${names[names.length - 1]}`;
}

// crewTag is a member's state when they are nowhere (ui crewTag, #46):
// out tomorrow on the bail you put down, jailed or laid up with the
// days to go, or retiring; "" for one at work.
export function crewTag(v, m, retiring = false) {
  if (m.jailed && m.bailed) return "out tomorrow";
  if (m.jailed) return `jailed ${m.jailed_until - v.day}d`;
  if (m.wounded) return `laid up ${m.wounded}d`;
  return retiring ? "retiring" : "";
}

// cornerName is a corner's name by id, or the id.
function cornerName(v, id) {
  for (const c of v.cities) for (const k of c.corners) if (k.id === id) return k.name;
  return id;
}

// post is where a member is (ui post): `{ text, warn }`, warn true for
// one who is nowhere, or whose role wants a post they have not got. An
// accountant works every front; a chemist the lab, or waits behind the
// best; a driver a route; a lieutenant a city.
export function post(v, m) {
  const tag = crewTag(v, m);
  if (tag) return { text: tag, warn: true };
  switch (m.role) {
    case "driver": {
      const r = m.route && (v.routes || []).find((x) => x.id === m.route);
      return m.route ? { text: "drives " + (r ? r.name : m.route), warn: false } : { text: "no route", warn: true };
    }
    case "lieutenant": {
      const c = m.city && v.cities.find((x) => x.id === m.city);
      return m.city ? { text: "runs " + (c ? c.name : m.city), warn: false } : { text: "no city", warn: true };
    }
    case "accountant":
      return v.fronts.length ? { text: plural(v.fronts.length, "front"), warn: false } : { text: "no front", warn: true };
    case "chemist":
      if (!m.lab) return { text: "second chemist", warn: true };
      return { text: (v.cooks || []).length ? "cooking" : "the lab", warn: false };
  }
  if (m.post) return { text: cornerName(v, m.post), warn: false };
  const h = (v.houses || []).find((x) => x.guard === m.id);
  if (h) return { text: "guards " + h.name, warn: false };
  return { text: m.role === "enforcer" ? "unposted" : "idle", warn: true };
}

// lineOf is the line a member skims under, or a lieutenant turns under
// (ui crewLine), off rules.crew.tuning and rules.crew.flip_line.
export function lineOf(m, tun, flip) {
  return m.role === "lieutenant" ? flip : tun.SkimThreshold;
}

// linesLine is the member's two lines in words (the TUI pane's row
// under the loyalty bar): "skims under 30 · walks at 10".
export function linesLine(m, tun, flip) {
  return `${m.role === "lieutenant" ? "turns" : "skims"} under ${round(lineOf(m, tun, flip))} · walks at ${round(tun.QuitThreshold)}`;
}

// PAY are the pay dial's notches, bottom to top.
export const PAY = ["stingy", "fair", "generous"];

// payBlurb is what a notch does to the crew (ui payBlurb, #536).
export function payBlurb(p) {
  return { stingy: "Loyalty bleeds.", generous: "Loyalty climbs." }[p] || "The greedy drift.";
}

// fireCost is what a firing costs the rest (ui fireCost, #522):
// nothing for the snitch the investigation named.
export function fireCost(m, tun) {
  if (m && m.exposed) return "They were talking to the police. Nobody will miss them: the rest lose no loyalty, and the file stops growing.";
  return `No severance in this business. The rest lose ${round(tun.FireLoyalty)} loyalty tonight, unless the one you fire was talking to the police.`;
}

// fireWalkLine names who the firing puts at the walk line (ui
// fireWalkLine, #532): every other member within fire_loyalty of
// quit_threshold, as the roster shows them; "" when nobody is that
// close, or for the named snitch, who costs nothing.
export function fireWalkLine(v, m, tun) {
  if (!m || m.exposed || !(tun.FireLoyalty > 0)) return "";
  const near = v.crew.filter((o) => o.id !== m.id && shown(o.loyalty) - tun.FireLoyalty <= tun.QuitThreshold).map((o) => `${o.name} (${shown(o.loyalty)})`);
  if (!near.length) return "";
  return `${and(near)} ${near.length > 1 ? "are" : "is"} within ${round(tun.FireLoyalty)} of the walk line (${round(tun.QuitThreshold)}): this firing takes them to it, and tonight they walk, or defect to a crew with corners.`;
}

// fireWarLine is the taken-out warning (ui fireWarLine, #520): an
// enforcer let go while a war is open (the war order on a faction, or a
// faction's war at rules.rivals.tuning's WarThreshold) that leaves the
// payroll under rivals.toml's taken_out_muscle (engineInfo); "" otherwise.
export function fireWarLine(v, m, need, warThreshold) {
  if (!m || m.role !== "enforcer" || !(need > 0)) return "";
  const left = v.crew.filter((x) => x.role === "enforcer").length - 1;
  if (left >= need) return "";
  const r = v.factions.find((f) => f.alive && (f.id === v.you.war || f.war >= warThreshold));
  if (!r) return "";
  return `At war with ${r.leader ? r.leader + "'s crew" : "the rival"}, this leaves ${left} of ${plural(need, "enforcer")}: if they take your last corner, the run ends taken out.`;
}

// investigateLines are the ask-around confirm (ui investigateConfirm):
// the price, who asks, the odds of a name, the loyalty a blank costs.
// cost and odds are rules.crew.investigate_cost and _odds; lost is
// crew.toml [informant] investigate_loyalty (engineInfo).
export function investigateLines(v, cost, odds, lost) {
  const best = Math.max(0, ...v.crew.filter((m) => m.role === "enforcer").map((m) => m.skill));
  const lines = [
    `Somebody goes through the crew tonight for ${money(cost)}.`,
    best > 0 ? `Your best enforcer (skill ${best}) does the asking.` : "With no enforcer on the payroll you are asking yourself.",
    `If somebody is talking to the police, ~${round(odds * 100)}% it names them.`,
  ];
  if (lost > 0) lines.push(`Naming nobody costs everyone ${round(lost)} loyalty.`);
  return lines;
}

// payOffLines are the pay-off confirm (ui payOffConfirm): the price and
// the loyalty it buys, off rules.crew.payoff_cost and payoff_loyalty.
export function payOffLines(m, cost, gain) {
  return [`${money(cost)} for ${m.name}: loyalty ${round(m.loyalty)} → ${round(Math.min(100, m.loyalty + gain))}.`, "It buys loyalty, not silence: somebody already talking keeps talking."];
}

// bailLines are the bail confirm (ui bailConfirm): the clean price, the
// loyalty it buys (rules.crew.life's BailLoyalty), what leaving them in
// means, and a word when the clean cash is short.
export function bailLines(v, m, cost, gain) {
  const lines = [
    `${money(cost)} clean for ${m.name}: out tomorrow, loyalty ${round(m.loyalty)} → ${round(Math.min(100, m.loyalty + gain))}.`,
    `Left in, they are out in ${plural(m.jailed_until - v.day, "day")}, sour, and the DA has had them a while.`,
  ];
  if (cost > v.you.clean_cash) lines.push(`Only ${money(v.you.clean_cash)} clean: a bondsman takes a cheque, never the bag.`);
  return lines;
}

// crewWarning is the crew screen's warning (ui crewWarning), off the
// alerts: somebody talking, or a skim on record; "".
export function crewWarning(v, tun) {
  if (v.alerts.some((a) => a.kind === "talking")) return "Somebody is talking. The file grew without a bust. Ask around or fire your suspect.";
  const skim = v.alerts.find((a) => a.kind === "skim");
  if (skim) return `Skimming suspected. Money went missing on day ${skim.day}. Somebody's loyalty is under ${round(tun.SkimThreshold)}.`;
  return "";
}

// trouble is the morning's crew trouble in a line (ui crewTrouble,
// #345, #522): "2 near or under the line, 1 corner unworked", or "".
// Every member in the band counts once, whether an alert names them or
// they are under their skim line (a lieutenant's turn line) however far
// from the walk; the corners are the idle-corner alerts.
export function trouble(v, tun, flip) {
  const band = new Set();
  let idle = 0;
  for (const a of v.alerts) {
    if (a.kind === "crew_line") band.add(a.member);
    else if (a.kind === "idle_corner") idle++;
  }
  for (const m of v.crew) if (m.loyalty < lineOf(m, tun, flip)) band.add(m.id);
  const parts = [];
  if (band.size) parts.push(`${band.size} near or under the line`);
  if (idle) parts.push(plural(idle, "corner") + " unworked");
  return parts.join(", ");
}

// countLine is "N of M on the payroll" and the word past the cap (#497).
export function countLine(v, most) {
  const n = v.crew.length;
  return `${n} of ${most} on the payroll${n > most ? " · over: nobody hired until under" : ""}`;
}

// summary is the CREW block (ui crewSection): rows of [label, text,
// warn], for the capacity where you stand (#576), the corners worked, who is off a corner, in a cell or laid
// up, who runs what, the lieutenant with no city, the snitch and the
// sloppy runners' heat. sloppy is rules.heat.sloppy_heat where you stand
// on 100 units, and sloppySkill heat.toml's sloppy_skill (engineInfo).
export function summary(v, sloppy = 0, sloppySkill = 0) {
  const rows = capacity(v);
  const fit = (m) => !m.jailed && !m.wounded;
  let worked = 0;
  for (const c of v.cities) worked += c.corners.filter((k) => k.owner === "player" && k.runner).length;
  rows.push(["corners", `${worked} worked`, false]);
  const idle = v.crew.filter((m) => m.role === "runner" && !m.post && fit(m)).length,
    unposted = v.crew.filter((m) => m.role === "enforcer" && !m.post && fit(m)).length,
    jailed = v.crew.filter((m) => m.jailed).length,
    wounded = v.crew.filter((m) => m.wounded).length;
  if (idle) rows.push(["idle", plural(idle, "runner"), true]);
  if (unposted) rows.push(["unposted", plural(unposted, "enforcer"), true]);
  if (jailed) rows.push(["jailed", plural(jailed, "member"), true]);
  if (wounded) rows.push(["laid up", plural(wounded, "member"), true]);
  if (idle + unposted) rows.push(["", "A runner earns nothing and an enforcer guards nothing off a corner. Post them on the streets.", true]);
  if (v.crew.some((m) => m.role === "accountant") && !v.fronts.length) rows.push(["accountant", "An accountant with no front is a wage. Buy one on the empire.", true]);
  for (const c of v.cities) {
    const lt = v.crew.find((m) => m.role === "lieutenant" && m.city === c.id);
    if (lt) rows.push([c.name, lt.name + (lt.personality ? " · " + lt.personality : ""), false]);
  }
  for (const m of v.crew) if (m.role === "lieutenant" && !m.city) rows.push(["no city", m.name + " is a wage", true]);
  const snitch = v.crew.find((m) => m.exposed);
  if (snitch) rows.push(["snitch", snitch.name + ", fire them", true]);
  if (sloppy > 0) rows.push(["sloppy", `+${sloppy.toFixed(1)} heat/100 units: runners under skill ${sloppySkill}, and a hothead on a corner`, true]);
  return rows;
}

// capacity is the CREW block's capacity rows (ui crewSection, #576):
// what the operation holds where you stand, and how much of it is your
// own carry, off view 21's you.capacity and you.carry_limit.
export function capacity(v) {
  const here = v.cities.find((c) => c.id === v.you.city),
    n = (v.you.capacity || {})[v.you.city] || 0;
  return [
    ["capacity", `${n} in ${here ? here.name : v.you.city}`, false],
    ["", `${v.you.carry_limit} yours + ${n - v.you.carry_limit} crew`, false],
  ];
}

// unassignCapLine is the city picker's word on the crew past the cap
// should the lieutenant come off their city (ui unassignCapLine, #497):
// most is rules.crew.max_crew, slots the lieutenancy's Crew; "" when
// they run none or the roster fits without their slots.
export function unassignCapLine(v, m, most, slots) {
  if (!m.city) return "";
  const n = v.crew.length,
    left = most - slots;
  return n > left ? `Off the city, the roster is ${n} of ${left}: nobody is let go; you hire under ${left}.` : "";
}

// overCapWords is what happens to the crew past the cap once a
// lieutenant's slots are gone (ui overCapWords, #497), with a leading
// space, read after the unassign: most is rules.crew.max_crew then.
export function overCapWords(v, most) {
  const n = v.crew.length;
  return n > most ? ` The roster is ${n} of ${most}: nobody is let go, and nobody is hired until it is under ${most}.` : "";
}

// assignedSaid and unassignedSaid are the city picker's results (ui
// confirmAssign): cut is the lieutenancy's Cut.
export const assignedSaid = (m, cityName, cut) => `${m.name} runs ${cityName} from tonight: posts the idle crew, sells the stash, keeps ${round(cut * 100)}%.`;
export const unassignedSaid = (m) => `${m.name} runs nothing now. The crew they posted stay where they are.`;

// held counts the corners you hold in a city (ui heldIn).
export function held(c) {
  return c.corners.filter((k) => k.owner === "player").length;
}

// captainRows are the captain picker's cities (ui viewCaptain): each
// with the corners held and who captains it, `theirs now` for the one
// asked about; `{ id, name, corners, who, mine, other }`.
export function captainRows(v, m) {
  return v.cities.map((c) => {
    const o = v.crew.find((x) => x.captain === c.id);
    return { id: c.id, name: c.name, corners: held(c), who: !o ? "nobody" : o.id === m.id ? "theirs now" : o.name, mine: !!o && o.id === m.id, other: !!o && o.id !== m.id };
  });
}

// captainAt is the city the picker opens on (ui askCaptain): theirs,
// else where they work, else where you stand.
export function captainAt(v, m) {
  if (m.captain) return m.captain;
  for (const c of v.cities) if (m.post && c.corners.some((k) => k.id === m.post)) return c.id;
  return v.you.city;
}

// budgetWord is a pay-off budget as the picker shows it.
export const budgetWord = (b) => (b > 0 ? `${money(b)} a night` : "nothing: no pay-offs");

// captainNotes are the picker's lines (ui viewCaptain): what a captain
// does each night and the cut, off rules.crew.captaincy.
export function captainNotes(cp) {
  return [`Each night: pull a suspected skimmer off a corner, post the idle runners on held corners, pay off one near the walk-out. Cut ${round(cp.Cut * 100)}%.`];
}

// captainRefusal is why a member cannot be captain (ui askCaptain), or
// "": a lieutenant runs a city, and the rest wait on loyalty and days
// at work. can is rules.crew.can_captain.
export function captainRefusal(m, cp, can) {
  if (m.role === "lieutenant") return `Can't make ${m.name} captain: a lieutenant runs a city. Give them one with Run a city.`;
  if (!m.captain && !can) return `Can't make ${m.name} captain yet: it takes loyalty ${round(cp.Loyalty)} and ${plural(cp.Days, "day")} on the payroll, at work.`;
  return "";
}

// captainSaid and dropCaptainSaid are the picker's results (ui
// confirmCaptain).
export const captainSaid = (m, cityName, budget, cut) => `${m.name} is captain of ${cityName} from tonight: ${money(budget)} a night for pay-offs, keeps ${round(cut * 100)}%.`;
export const dropCaptainSaid = (m) => (m.captain ? `${m.name} is one of the crew again. Nobody looks after them tonight.` : `${m.name} is nobody's captain.`);

// keeps is a lieutenant's stock levels in their city (ui
// lieutenantKeeps, #524): each contract of theirs standing, yours
// winning, in ladder order, `678 Designer · 40 Weed`; "" with none.
export function keeps(v, city) {
  const c = city && v.cities.find((x) => x.id === city);
  if (!c) return "";
  const supply = v.supply || [],
    parts = [];
  for (const p of c.products) {
    if (supply.some((s) => s.city === city && s.product === p.id && !s.lieutenant)) continue;
    const s = supply.find((s) => s.city === city && s.product === p.id && s.lieutenant);
    if (s && s.units > 0) parts.push(`${s.units} ${p.name}`);
  }
  return parts.join(" · ");
}

// poolNext is the days until new faces come looking (ui viewCrew): days
// is rules.crew.pool_days, and view 21's you.pool_day the last turn.
export function poolNext(v, days) {
  return Math.max(1, days - (v.day - (v.you.pool_day || 0)));
}

// KIN is the mark on a name with kin (ui kinGlyph, #46), and KIN_LEGEND
// what it says.
export const KIN = "♦",
  KIN_LEGEND = "♦ has kin on the payroll or looking for work";

// relation is the word for a pair of kin (ui relation): fixed by the
// pair and nothing saved.
export function relation(a, b) {
  return ["cousin", "partner", "friend"][(a + b) % 3];
}

// kinNames are a member's kin, on the payroll or in the pool, with the
// word for each (ui kinNames): ["Bo (cousin)"].
export function kinNames(v, m) {
  const out = [];
  for (const id of m.kin || []) {
    const k = v.crew.find((x) => x.id === id) || v.pool.find((x) => x.id === id);
    if (k) out.push(`${k.name} (${relation(m.id, k.id)})`);
  }
  return out;
}

// kinLine is the kin row for a member or a face in the pool (ui
// personLines): their kin, and for a face, the discount it brings; "".
export function kinLine(v, m, inPool) {
  const names = kinNames(v, m),
    parts = names.length ? [`Kin: ${names.join(", ")}`] : [];
  if (inPool && (m.kin || []).length) parts.push(names.length ? "came with the kin: fee at the discount" : "Came with the kin: fee at the discount");
  return parts.join(" · ");
}

// assignRows are the lieutenant's city picker (ui viewAssign): each
// city with the corners held, the units stashed and who runs it,
// `theirs now` for the one asked about.
export function assignRows(v, m) {
  return v.cities.map((c) => {
    const o = v.crew.find((x) => x.role === "lieutenant" && x.city === c.id),
      units = Object.values((v.you.stock || {})[c.id] || {}).reduce((a, b) => a + b, 0);
    return { id: c.id, name: c.name, corners: held(c), units, runs: !o ? "nobody" : o.id === m.id ? "theirs now" : o.name };
  });
}

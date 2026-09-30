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

// countLine is "N of M on the payroll" and the word past the cap (#497).
export function countLine(v, most) {
  const n = v.crew.length;
  return `${n} of ${most} on the payroll${n > most ? " · over: nobody hired until under" : ""}`;
}

// summary is the CREW block (ui crewSection): rows of [label, text,
// warn], for the corners worked, who is off a corner, in a cell or laid
// up, who runs what, the lieutenant with no city, the snitch and the
// sloppy runners' heat. sloppy is rules.heat.sloppy_heat where you stand
// on 100 units, and sloppySkill heat.toml's sloppy_skill (engineInfo).
export function summary(v, sloppy = 0, sloppySkill = 0) {
  const rows = [];
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

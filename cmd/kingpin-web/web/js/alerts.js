// The alerts (#352): what needs you this morning, the view's `alerts`,
// in words, each with where the page answers it. The engine names the
// act (`act.screen`, `act.mode`, `act.subject`); the page maps the
// screens it has onto its own panels and shows the rest as words
// alone. It touches no DOM, so the Node test words every alert a run
// raises through the very same code the page does.

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n)).toLocaleString("en-US");
const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;
const byId = (list, id) => (list || []).find((x) => x.id === id);
const n = (x) => x || 0; // a number the view left out is zero (omitempty)

function cityName(v, id) {
  const c = byId(v.cities, id);
  return c ? c.name : id;
}

function cornerName(v, id) {
  for (const c of v.cities || []) {
    const k = byId(c.corners, id);
    if (k) return k.name;
  }
  return id;
}

const memberName = (v, id) => (byId(v.crew, id) || { name: "Somebody" }).name;

// WORDS is one sentence a kind, off the alert's own fields and the
// view: engine.AlertKinds, no more and no less (TestWebClient).
export const WORDS = {
  talking: () => "Somebody on the payroll is talking.",
  contract_due: (v, a) => {
    const c = byId(v.contracts, a.contract);
    return `${c ? c.name : "A buyer"}: due ${a.due <= v.day ? "today" : "tomorrow"}.`;
  },
  debt_due: (v, a) => `${(byId(v.connects, a.supplier) || { name: "A connect" }).name}: ${money(n(a.amount))} due tomorrow, ${money(n(a.have))} in hand.`,
  heat: (v, a) => `Heat ${Math.round(n(a.heat))} in ${cityName(v, a.city)} is over the patrol line (${Math.round(n(a.line))}).`,
  task_force: () => "A task force formed this morning. It comes tonight: lie low.",
  investigation: (v, a) => {
    const name =
      a.target === "corner" ? cornerName(v, a.corner) : a.target === "house" ? (byId(v.houses, a.house) || { name: "a house" }).name : `the ${a.product} trade`;
    return `Police are working ${name}: they hit ${n(a.days) <= 1 ? "tonight" : `in ${plural(a.days, "day")}`}.`;
  },
  float: (v, a) => `Dirty cash ${money(n(a.have))} is under the float (${money(n(a.amount))}).`,
  wages: (v, a) => `Wages ${money(n(a.amount))} due tonight, ${money(n(a.have))} dirty in hand.`,
  crew_line: (v, a) => {
    const cross = { skim: "skimming", flip: "turning", walk: "walking" }[a.cross] || a.cross;
    return `${memberName(v, a.member)} is ${Math.max(1, Math.ceil(n(a.gap)))} from ${cross}${a.days ? ` (${plural(a.days, "day")})` : ""}.`;
  },
  skim: (v, a) => `Skimming suspected: money went missing on day ${n(a.day)}.`,
  unposted: (v, a) =>
    a.corner ? `${memberName(v, a.member)} has no post: ${cornerName(v, a.corner)} is free for them.` : `${memberName(v, a.member)} has no post.`,
  idle_corner: (v, a) => `Nobody works ${cornerName(v, a.corner)}: back to the street ${n(a.days) <= 1 ? "tonight" : `in ${plural(a.days, "day")}`}.`,
  stash_full: (v, a) => `The stash in ${cityName(v, a.city)} is full: ${n(a.count)} of ${n(a.amount)}.`,
  scouts: (v, a) => `A faction is ${a.level === "recruiting" ? "recruiting" : "scouting"} in ${cityName(v, a.city)}: ${n(a.days) <= 0 ? "due now" : `in ${plural(n(a.days), "day")}`}.`,
  gate: (v, a) => `${a.gate ? a.gate.name : "A door"} is within reach.`,
  house_known: (v, a) => `The police know about ${(byId(v.houses, a.house) || { name: "a house" }).name}.`,
  da_race: (v, a) => `The DA race is ${plural(n(a.days), "day")} off and the tickets are taking money.`,
  retire: (v, a) => {
    if (a.ready) return "You could retire.";
    const parts = [];
    if (a.days) parts.push(`${plural(a.days, "quiet day")} to go`);
    if (a.amount) parts.push(`${money(a.amount)} short`);
    return `Retirement: ${parts.join(", ") || "nearly there"}.`;
  },
  favour: (v, a) => `The chief owes you one and the ${a.level || "police"} comes tonight.`,
  reign: (v, a) => `The city is yours: day ${n(a.days)} of the reign.`,
  exposure: (v, a) => `Tonight's landings put ${money(a.amount)} past what your fronts cover: about +${Math.round(a.heat || 0)} heat.`,
  straight: (v, a) => `The fronts earn ${money(a.amount)} a day, more than the street: you could go straight.`,
  plan: (v, a) => {
    const p = (v.ambitions || []).find((x) => x.id === a.ambition);
    const name = p ? p.name : "The plan";
    return a.ready ? `The plan, ${name}: ready.` : `The plan, ${name}: ${n(a.count)} of ${n(a.steps)} steps met.`;
  },
};

// alertText is the alert in words; a kind this client does not know is
// named by its kind.
export function alertText(v, a) {
  const f = WORDS[a.kind];
  return f ? f(v, a) : a.kind;
}

// PANELS are the screens the page has, by the engine's names: the
// market is the city panel's product table, the map the canvas. An act
// on another screen is words alone until the page grows that panel.
export const PANELS = { market: "market", map: "map" };

// alertPanel is the id of the page element that answers the alert, or
// "" where the page has no such screen yet.
export function alertPanel(a) {
  return (a.act && PANELS[a.act.screen]) || "";
}

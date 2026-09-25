// The ways out (#405): the four endings the player claims (retire,
// vanish, take the crown, go straight), each open or with what is
// short, read off the view's ambitions (#347), whose steps are the
// endings' own terms (TestAmbitionProgressAgreesWithTheEnding), and the
// tonight line off the forecast (#397). It touches no DOM, so the Node
// test reads and claims every exit through the very same code the page
// does.

const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n)).toLocaleString("en-US");
const n = (x) => x || 0; // a number the view left out is zero (omitempty)

// EXITS are the ways out in the TUI's walk-away order: the ambition
// that plans each and the session's method that takes it.
export const EXITS = [
  { id: "retire", name: "Retire", ambition: "retire", method: "retire" },
  { id: "vanish", name: "Vanish", ambition: "vanish", method: "vanish" },
  { id: "crown", name: "Take the crown", ambition: "city", method: "crown" },
  { id: "straight", name: "Go straight", ambition: "legit", method: "goStraight" },
];

// amount is a step's have or need in its unit's words.
function amount(unit, x) {
  return unit === "cash" || unit === "clean" || unit === "dirty" || unit === "income" ? money(n(x)) : String(Math.round(n(x)));
}

// exits is every way out as it stands: {id, name, method, open, why},
// why the steps short ("offshore $0 of $750,000; quiet days 3 of 14"),
// empty when open.
export function exits(v) {
  return EXITS.map((e) => {
    const a = (v.ambitions || []).find((x) => x.id === e.ambition);
    const open = !!(a && a.done) && !v.over;
    const short = a ? (a.steps || []).filter((s) => !s.done).map((s) => `${s.label} ${amount(s.unit, s.have)} of ${amount(s.unit, s.need)}`) : ["not in this game"];
    return { ...e, open, why: open ? "" : short.join("; ") || "not yet" };
  });
}

// claim takes the way out: the run ends on it, and the view comes back
// over. A closed one is refused by the engine in its own words.
export function claim(session, id) {
  const e = EXITS.find((x) => x.id === id);
  if (!e) throw new Error(`no way out called ${id}`);
  session[e.method]();
  return session.refresh();
}

// tonightText is the forecast in words (#397): the pile the count will
// find before tonight's sales, and the heat it adds past the cover. The
// wash comes after the count, so it cannot help tonight.
export function tonightText(f) {
  let s = `Tonight the police count ${money(n(f.pile))} dirty (${money(n(f.dirty))} now`;
  if (n(f.loads)) s += ` + ${money(n(f.landings))} landing`;
  if (n(f.wages)) s += ` − ${money(n(f.wages))} wages`;
  s += ", before sales)";
  if (n(f.heat) > 0) s += `: ${money(n(f.pile) - n(f.line))} past your cover, +${Math.round(f.heat)} heat before the wash.`;
  else s += ", under your cover.";
  return s;
}

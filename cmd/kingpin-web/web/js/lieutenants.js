// A lieutenant in words (#455): what running a city means, what it costs,
// the tempers and the risk, off rules.crew.lieutenancy (the crew sim's
// terms, the numbers the TUI's pane and assign picker read), and how they
// come, off the view. Pure functions, so both web clients word it the
// same way and the Node test runs it; it touches no DOM.

const pct = (x) => `${Math.round(x * 100)}%`;
const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

// tempers is the four in the terms' order: "violent, greedy, careful or
// steady".
export function tempers(t) {
  const names = (t.Tempers || []).map((x) => x.Name);
  return names.length > 1 ? `${names.slice(0, -1).join(", ")} or ${names[names.length - 1]}` : names.join("");
}

// roleLines is the role in four sentences: the night's work, the cut and
// the slots, the temper, and where they talk and where they walk.
export function roleLines(t) {
  return [
    "Runs a city for you: sells it every night and keeps it stocked; your own orders there win.",
    `Takes ${pct(t.Cut)} of the city's takings and brings ${plural(t.Crew, "crew slot")} of their own.`,
    `Their temper is ${tempers(t)}; it shows after ${plural(t.RevealDays, "day")} running a city.`,
    `Under ${Math.round(t.Flip)} loyalty they talk to the police; at ${Math.round(t.Quit)} they walk and take the city with them.`,
  ];
}

// temperLine is one temper in a line: the dial they sell at, the heat
// against a normal hand's, the days of stock they keep, a skim and
// whether they go after a faction's scouts.
export function temperLine(tt) {
  const parts = [`sells ${tt.Dial}`];
  if (tt.Heat !== 1) parts.push(`heat ×${Number(tt.Heat.toPrecision(3))}`);
  parts.push(`${Number(tt.StockDays.toPrecision(3))}d stock`);
  if (tt.Skim > 0) parts.push(`skims ${pct(tt.Skim)}`);
  if (tt.HitScouts) parts.push("hits scouts");
  return parts.join(" · ");
}

// temperOf is the terms of the named temper, or undefined.
export function temperOf(t, name) {
  return (t.Tempers || []).find((x) => x.Name === name);
}

// howTheyCome is how lieutenants come looking, while they do not: once
// corners are held in two cities, so a runner posted in a city where you
// hold none. Empty once two cities are held.
export function howTheyCome(v) {
  const held = v.cities.filter((c) => c.corners.some((k) => k.owner === "player"));
  if (held.length >= 2) return "";
  const other = v.cities.find((c) => !held.includes(c));
  return other ? `Lieutenants come looking once you hold corners in two cities: post a runner on a corner in ${other.name}.` : "";
}

// The law panel (#355): the police risk in a city, heat, the file,
// pressure and the cash exposure together, each said in a clause, the
// way the TUI's POLICE section says it. A pure function of the view, so
// the page draws it and the Node test runs it; it touches no DOM.

const pct = (x) => `${Math.round(x * 100)}%`;
const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n)).toLocaleString("en-US");
const plural = (n, word) => `${n} ${word}${n === 1 ? "" : "s"}`;

// rungWord is what a rung does, in one clause.
export function rungWord(r) {
  switch (r.level) {
    case "patrol":
      return `sales capped near ${pct(r.cap || 0)} of demand for ${plural(r.cap_days || 0, "day")}`;
    case "arrest":
      return "you are arrested: the run ends";
    case "taskforce":
      return `the feds seize an asset and take ${pct(r.stock_loss || 0)} of the stock and ${pct(r.cash_loss || 0)} of the dirty cash`;
    default:
      return `takes ${pct(r.stock_loss || 0)} of the stock and ${pct(r.cash_loss || 0)} of the dirty cash`;
  }
}

const rungName = (level) => (level === "taskforce" ? "task force" : level);

// fileWord is the header's `3/6`, or the page count alone with no file.
export function fileWord(v) {
  const line = v.law && v.law.arrest_line;
  return line > 0 ? `${v.you.evidence}/${line}` : String(v.you.evidence);
}

// policeLines is the panel for a city: a list of {text, warn} lines,
// warn set on what is close.
export function policeLines(v, cityId) {
  const c = v.cities.find((x) => x.id === cityId);
  if (!c) return [];
  const out = [];
  const say = (text, warn = false) => out.push({ text, warn });
  const ladder = c.ladder || [];
  const next = ladder.find((r) => c.heat < r.line);
  say(`Heat ${Math.round(c.heat)}/100${next ? `: ${Math.ceil(next.line - c.heat)} to the ${rungName(next.level)}` : ""}.`, !!next && next.line - c.heat <= 10);
  for (const r of ladder) {
    const pages = r.pages ? `, ${plural(r.pages, "page")} if you sold that day` : "";
    say(`${rungName(r.level)} ${Math.round(r.line)}: ${rungWord(r)}${pages}.`, c.heat >= r.line);
  }
  const line = v.law.arrest_line;
  if (line > 0) {
    const left = Math.max(0, line - v.you.evidence);
    say(`The file: ${v.you.evidence}/${line} pages, ${plural(left, "page")} from an indictment. A bust on a day you sold files pages; a quiet day files nothing.`, left <= 2);
  }
  say(`Pressure ${Math.round(c.pressure)}, goodwill ${Math.round(c.goodwill)}: pressure lowers the patrol, raid and arrest lines and tightens a patrol; goodwill wears it down.`);
  const exp = v.law.exposure_line;
  if (exp > 0) {
    const cover = v.law.cover > 0 ? ` (${money(exp - v.law.cover)} plus ${money(v.law.cover)} your fronts cover)` : "";
    const past = v.you.dirty_cash > exp;
    say(`Dirty cash ${money(v.you.dirty_cash)} of a ${money(exp)} line${cover}: ${past ? "over it, the pile draws heat every day" : "under it, the pile draws nothing"}.`, past);
  }
  if (c.response) {
    say(`Estimate: a cop says the ${rungName(c.response)} from day ${c.response_day}, ${pct(c.response_sure || 0)} sure.`);
  } else {
    say("No word from inside: nothing tells you when they move.");
  }
  return out;
}

// The glossary (#575): the TUI help modal's WORDS (ui/keys.go `words`),
// the terms the screens use without explaining, one line each, in the
// TUI's order and its words. A term that names a TUI key says where the
// web answers it instead (ruled for #560: each front end words its own
// pointer), and the TUI's own frame, scenes and profile are left out
// (ruled out of scope 2026-10-01). cmd/kingpin-web's TestStreetGlossary
// holds WORDS to the TUI's, so a term the TUI gains or rewords reaches
// here. A pure module: it touches no DOM, and smoke.mjs checks it.

// WORDS are [term, line], the line with the TUI's placeholders for the
// numbers the file sets ({retire_heat}, {sting_pages}, ...), filled by
// glossary().
export const WORDS = [
  ["dial", "quiet, normal or aggressive: a sale's volume against its heat"],
  ["float", "the dirty cash the wash and the road leave for the street"],
  ["till", "the float in hand: the wash and the road spend only over it"],
  ["target", "what a route keeps the far end at: units or days of demand"],
  ["heat", "a city's police eye, 0-100: sales raise it, days fade it"],
  ["patrol", "the first rung: caps what sells for a few days, files nothing"],
  ["sting", "a rung: stock and cash; {sting_pages} if you sold, {hit_pages} on a named hit"],
  ["raid", "a big rung: much of the stock and cash; {raid_pages} if you sold"],
  ["arrest", "the top rung: a warrant, served if you sell the next night"],
  ["file", "the DA's pages: a bust on a day you sold adds; enough indicts"],
  ["pressure", "a city's mood, 0-100: lowers police lines, tightens patrols"],
  ["goodwill", "bought with clean cash (Fund a city, on the Ledger): wears pressure down"],
  ["DA race", "the DA's vote each term: clean cash on a ticket moves it"],
  ["cover", "what your fronts explain of the dirty pile: it draws no heat"],
  ["estimate", "a forecast, such as a cop's word: shown with how sure it is"],
  ["drift", "a held corner nobody works goes back to the street in days"],
  ["undercut", "sell cheap on a rival corner next door: they lose, no heat"],
  ["keep at", "a supply contract: the stash bought back to a level daily"],
  ["through", "a buy into a city a lieutenant runs for you, at a markup"],
  ["standing", "a sell order that stands nightly until cancelled, at a cut"],
  ["connect", "who sells you product: a price, a lot, a temper, a rel"],
  ["credit", "a connect's book: take now, pay in days, or they answer"],
  ["unlock", "a line crossed: a product, front, connect or role opens"],
  ["house", "a rented stash off the street: rent in clean; a raid hits one"],
  ["deed", "the block a corner is on, bought clean: rent; the DA asks"],
  ["tier", "the stage a run is in, shown once: Corner to Cartel"],
  ["scout", "a paid look at the rival's books: a snapshot that goes stale"],
  ["boost", "the enforcers rob a rival corner's till, not the corner"],
  ["world", "the weather: an incident that lands on the world, not on you"],
  ["quality", "a lot's grade, 0-100: the price pays it, corners remember it"],
  ["cut", "add units at nothing: more today, fewer customers tomorrow"],
  ["cook", "a chemist's batch of meth or designer, from precursors"],
  ["repeat", "the share of a corner's customers who come back"],
  ["overdose", "bad hard product on your corner: pressure and news, no page"],
  ["jailed", "in a cell after a bust, working nothing; bail is clean cash"],
  ["kin", "a cousin, partner or friend on the payroll: they remember"],
  ["driver", "rides a route's shipments and cuts the risk; seized, jailed"],
  ["trait", "what a veteran showed after their days of service; some bad"],
  ["captain", "a trusted veteran who looks after a city's crew each night"],
  ["lieutenant", "runs a city for a cut: sells it, stocks it; Run a city, on the crew tab"],
  ["temper", "a lieutenant's way: violent, greedy, careful or steady"],
  ["asset", "the supply side bought clean: a connect, port, plane, lab"],
  ["feds", "the task force above the raid: a day's notice, takes an asset"],
  ["intel", "what you know, with how sure: seen, bought, sent out, or fed"],
  ["spy", "a crew member under with a faction: reports, sells nothing"],
  ["ending", "how a run ends: nine ways, each a summary and a score"],
  ["score", "the offshore account over one plus the bodies; days shown"],
  ["taken out", "your last corner, taken by the crew you are at war with"],
  ["betrayed", "a lieutenant turns on {betray_share} of your corners, {betray_corners}+: that night"],
  ["quiet day", "heat under {retire_heat}; no strike, push, war, bust, buyer's order owed"],
  ["the street", "your corners and their trade; a corner back to it is nobody's"],
  ["street night", "a night's dealing; going straight: fronts over its last {street_window}"],
  ["run out", "no corner left: a claim it loses soon keeps the clock running"],
  ["absorbed", "run out long enough: it joins the faction that took its last"],
  ["scattered", "run out too long, or broke: it stands down, nobody's"],
  ["gone", "absorbed, scattered or leaderless: a crew down for the crown"],
  ["walk away", "retire on the account, vanish, or take the crown: asked twice"],
  ["reign", "the city yours: every crew gone or bowing, most of home held"],
  ["favour", "a bought chief owes you one; call it in, no raid"],
  ["war", "enforcers hit one faction every night until it folds"],
  ["character", "a start and nothing more: what is on the world on day 0"],
  ["preset", "the routine's dials in one bundle: reviewed, then applied"],
];

const plural = (n, w) => `${n} ${w}${n === 1 ? "" : "s"}`;

// glossary is WORDS with the numbers filled as the TUI's helpLines fills
// them, off the file (engineInfo.glossary, build.py): the quiet day's
// heat line, the sting's and the raid's pages if you sold and a named
// hit's, a betrayal's share and corners, and the street's window.
export function glossary(info) {
  const g = info || {},
    pages = g.pages || {},
    fill = {
      retire_heat: String(Math.round(g.retireHeat || 0)),
      sting_pages: plural(pages.sting || 0, "page"),
      raid_pages: plural(pages.raid || 0, "page"),
      hit_pages: String(g.hitPages || 0),
      betray_share: Math.round((g.betrayShare || 0) * 100) + "%",
      betray_corners: String(g.betrayCorners || 0),
      street_window: String(g.streetWindow || 0),
    };
  return WORDS.map(([term, line]) => [term, line.replace(/\{([a-z_]+)\}/g, (all, k) => fill[k] ?? all)]);
}

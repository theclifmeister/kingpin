// Where a report line points in Street Edition (#560). A line that
// sends you somewhere names no screen in its words: it carries a point
// (view 22's section `points`: the line, the byte `at`, the `kind` and
// the `act`), and each front end words its own pointer there. The TUI
// puts " on the ledger screen (7)" at `at`; this page puts a link to the
// tab after the line, landed as an alert of the point's kind is
// (landing.js). It touches no DOM, so smoke.mjs checks it.

// TABS are the page's tabs as a link names them.
export const TABS = {
  street: "The streets",
  market: "Market",
  crew: "Your people",
  empire: "The empire",
  rivals: "Rivals",
  ledger: "Ledger",
};

// LACKS are the engine's screens this page has no tab for: a line that
// points there reads whole, with no link (the TUI's intel screen).
const LACKS = new Set(["intel"]);

// pointOf is the point of line i of a report section, or null: a
// section from before view 22 has none.
export function pointOf(sec, i) {
  return (sec.points || []).find((p) => p.line === i) || null;
}

// pointAlert is the point as the alert landing() reads, its kind and
// act, or null where the page has no tab for its screen.
export function pointAlert(p) {
  if (!p || !p.act || LACKS.has(p.act.screen)) return null;
  return { kind: p.kind, act: p.act };
}

// linkWords is the link's words for the tab a point lands on:
// "The empire →".
export function linkWords(tab) {
  return `${TABS[tab] || tab} →`;
}

// The stage and the "new" mark (#578), as the TUI words them: the
// interstitial the morning a stage is entered (ui/stage.go, #149, #525,
// #537), the tier marked new while that stage is not yet seen
// (ui/dashboard.go), and the market pane's next product on the ladder
// (ui/unlocks.go nextProductNote). Each function takes the view and
// returns words: it touches no DOM, so smoke.mjs checks them on a live
// run. The view's `stage` is the engine's (Session.stageView): the tier,
// the file's words and the NEXT line; see_stage with `pending` marks it
// seen.

import { cash as short } from "./wash.js?v=__BUILD_REVISION__";

// pending is the stage waiting to be seen, or 0.
export const pending = (v) => v.stage?.pending || 0;

// tierLabel is the header's tier: its name, and `· new` while its stage
// is not yet seen, the way the TUI's STREET title and tier fact mark it.
export const tierLabel = (v) => v.you.tier_name + (pending(v) ? " · new" : "");

// title is the interstitial's title: `STAGE 3 · TERRITORY`.
export const title = (s) => (s.name ? `STAGE ${s.pending} · ${s.name.toUpperCase()}` : `STAGE ${s.pending}`);

// lanes is whether the stage points at the lanes abroad (#525): it is
// the stage the run is in and the exports alert stands this morning.
export const lanes = (v) => !!v.stage && v.stage.pending === v.you.tier && v.alerts.some((a) => a.kind === "exports");

// interstitial is the modal's words: the title, the blurb, the text as
// one paragraph, OPENED with what the stage opens, the pointer to the
// lanes where it stands, and NEXT. Null with no stage pending.
export function interstitial(v) {
  const s = v.stage;
  if (!s) return null;
  if (!s.name) return { title: title(s), blurb: "", text: "A stage with no name in the file.", opened: [], lanes: "", next: s.next || "" };
  return {
    title: title(s),
    blurb: s.blurb,
    text: (s.text || []).join(" "),
    opened: s.opened || [],
    lanes: lanes(v) ? "The lanes are on The empire; the alert says what they pay." : "",
    next: s.next,
  };
}

// nextProduct is the next product on the ladder (the market file's
// order, from engine-info's `ladder`) not yet listed in the city, or
// null once the whole ladder is.
export function nextProduct(v, ladder, city) {
  const listed = new Set((v.cities.find((c) => c.id === city)?.products || []).map((p) => p.id));
  return (ladder || []).find((p) => !listed.has(p.id)) || null;
}

// nextProductNote is the market pane's last note: `Heroin lists at $3K
// peak cash ($1.2K to go).`, with the honest form where the supplier in
// the city will not sell it. "" once the ladder is listed.
export function nextProductNote(v, info, city) {
  const p = nextProduct(v, info.ladder, city);
  if (!p) return "";
  let line = `${p.name} lists at ${short(p.unlock_cash)} peak cash (${short(p.unlock_cash - v.you.peak_cash)} to go)`;
  if ((info.noSupply?.[city] || []).includes(p.id)) line += "; the supplier here will not sell it, the road brings it";
  return line + ".";
}

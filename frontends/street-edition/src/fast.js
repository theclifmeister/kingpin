// Fast-forward (#555), as the TUI words it (ui/fast.go, #116, #504,
// #518, #519): the cap the dialog reads, the refusal while a night
// stands that ends the run (the `holds` query), and the line that says
// why the days stopped. What stops is the engine's (engine/stoprule.go,
// docs/engine.md, #541): a danger, an ending's door, something to answer
// before the next night, or something lost or changing hands; never a
// notice. fast_forward weighs the days and answers the stop, its alert
// or its event (`payload`), and whether it is a danger; this module only
// words it. It touches no DOM, so smoke.mjs words a live run's stop.

import { alertText } from "./alerts.js?v=__BUILD_REVISION__";
import { money } from "./format.js?v=__BUILD_REVISION__";

// DAYS is the cap the dialog opens on, MAX the most one run takes.
export const DAYS = 7,
  MAX = 30;

const plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`;
const a = (noun) => (/^[aeiou]/i.test(noun || "") ? "an " : "a ") + noun; // format.A
const byId = (list, id) => (list || []).find((x) => x.id === id);
const cityName = (v, id) => byId(v.cities, id)?.name || id;
function productName(v, id) {
  for (const c of v.cities || []) {
    const p = byId(c.products, id);
    if (p) return p.name;
  }
  return id;
}

// RULE is the dialog's footer: the rule, as the TUI's states it.
export const RULE = "Stops for a danger, an ending's door, something to answer before tonight, or something lost or changing hands. Never for a notice.";

// ask is the dialog's sentence on the cap the field reads.
export const ask = (n) => `Run up to ${plural(n, "day")}, stopping when something needs you.`;

// cap is the cap the field reads (fastCap): DAYS for a blank, else
// {error} for a number that does not read or is over MAX.
export function cap(text) {
  const s = String(text ?? "").trim();
  if (!s) return { days: DAYS };
  const n = Number(s.replace(/,/g, ""));
  if (!Number.isInteger(n) || n <= 0) return { error: "enter a whole number above zero" };
  if (n > MAX) return { error: `up to ${MAX} days at a time` };
  return { days: n };
}

// held is the refusal while the holds query names an alert (fastHeld,
// #518): the alert's own words, then what to do. "" with nothing held.
export function held(v, hold) {
  if (!hold) return "";
  return `Fast-forward will not run tonight: ${trim(alertText(v, hold))}. Deal with it, or end the day by hand.`;
}

// trim drops a sentence's closing stop, so the line can go on.
const trim = (s) => String(s).replace(/[.\s]+$/, "");

// fileClose is the file against the indictment line (ui fileClose):
// `file 3/6: 3 pages from an indictment`.
export function fileClose(pages, limit) {
  const left = limit - pages;
  if (limit <= 0) return `file ${pages}`;
  if (left <= 1) return `file ${pages}/${limit}: one more page is an indictment`;
  return `file ${pages}/${limit}: ${plural(left, "page")} from an indictment`;
}

// fileNumbers is what a danger stop carries (#504): `, file 3/6: 3 pages
// from an indictment`, or "" for a stop that is no danger or a run with
// no file. The file's own alerts and the warrant carry their numbers.
function fileNumbers(v, r) {
  const limit = v.law?.arrest_line || 0;
  if (!r.danger || limit <= 0) return "";
  if (r.alert && ["file", "pages", "arrest"].includes(r.alert.kind)) return "";
  return ", " + fileClose(v.you.evidence || 0, limit);
}

// stop is the line a fast-forward's answer gets (fastForward, stopWhy):
// {text, tone, alert}, the tone "danger" for a danger stop, else
// "warn"; alert the alert it stopped on, for the jump to it. Null for a
// run that ended (the ending says it) or ran no day.
export function stop(v, r) {
  if (!r || r.stop === "over" || !r.ran) return null;
  const days = plural(r.ran, "day"),
    head = r.danger ? `Stopped after ${days} on a danger:` : `Stopped after ${days}:`;
  return { text: `${head} ${why(v, r)}.`, tone: r.danger ? "danger" : "warn", alert: r.stop === "alert" ? r.alert : null };
}

// why is the reason as the stop line names it (stopWhy): a new stage,
// a card, the alert's words, the event's (eventWhy) or the cap.
export function why(v, r) {
  switch (r.stop) {
    case "stage":
      return "a new stage";
    case "card":
      return "a card to answer";
    case "alert":
      return trim(alertText(v, r.alert)) + fileNumbers(v, r);
    case "event":
      return eventWhy(v, r.event, r.payload || {}) + fileNumbers(v, r);
  }
  return "the cap";
}

// level is a police level as a line names it (bustLevelWord, favourWord).
const level = (l) => (l === "taskforce" ? "task force" : l);

// quietCause is what reset a quiet streak (ui quietCause), or "".
function quietCause(v, ev) {
  const where = byId(v.cities, ev.City) ? " in " + cityName(v, ev.City) : "";
  const what = {
    heat: "heat at the retire line" + where,
    police: "a " + level(ev.Level) + where,
    strike: "your strike on a corner",
    push: "a push on your corners",
    war: "the war getting loud",
    contract: "a buyer's contract still open" + where,
  }[ev.Cause];
  return what ? `${what} on day ${ev.Day}` : "";
}

// supplyShort is why a supply contract came up short (supplyShortWords).
const supplyShort = (why) =>
  ({ road: "waiting on what is on the road", room: "short of room", supplier: "short: nobody there sells it today" })[why] || "short of cash";

// eventWhy is the reason an event stopped the days, as the TUI's
// stopEvent words it: every kind the stop rule does not class a notice
// has words here (TestStreetFastWords), and one without is named by its
// kind.
export function eventWhy(v, kind, ev) {
  const city = (id) => cityName(v, id);
  switch (kind) {
    case "Enforcement":
      return ev.Level === "taskforce" ? "the task force in " + city(ev.City) : `${a(ev.Level)} in ${city(ev.City)}`;
    case "TaskForceFormed":
      return "a task force formed in " + city(ev.City);
    case "WarrantSigned":
      return "a warrant for your arrest";
    case "InvestigationOpened":
      return "the police are working " + ev.Name;
    case "AssetSeized":
      return "the feds took " + ev.Name;
    case "TunnelFound":
      return "the tunnel was found";
    case "CrewPaid":
      return `payroll missed, ${money(ev.Short)} short: the crew's loyalty falls`;
    case "QuietBroken": {
      const cause = quietCause(v, ev);
      return `the quiet streak (${plural(ev.Days, "day")}) was reset${cause ? " by " + cause : ""}`;
    }
    case "ReignBegan":
      return "the city is yours";
    case "ReignBroken":
      return "the reign is over: " + ev.Why;
    case "HoldBegan":
      return `the crown's hold began: the reign on day ${ev.Reign} if it holds`;
    case "RivalAbsorbed":
      return ev.By ? `${ev.Rival}'s crew is finished, gone over to ${ev.By}` : `${ev.Rival}'s crew scattered`;
    case "StraightOpened":
      return "you could go straight";
    case "StraightLapsed":
      return "going straight is off again";
    case "RivalMovedIn":
      return `${ev.Rival} moved in on ${ev.Name}`;
    case "RivalEyeing":
      return `${ev.Rival} is eyeing ${ev.Name}`;
    case "RivalScouting":
      return `${ev.Rival} has scouts in ${city(ev.City)}`;
    case "RivalRecruiting":
      return `${ev.Rival} is recruiting in ${city(ev.City)}`;
    case "CornerStruck":
      return "the strike on " + ev.Name;
    case "WarEnded":
      return `the war on ${ev.Rival}'s crew is over${ev.Why ? ": " + ev.Why : ""}`;
    case "RivalBoosted":
      return `the boost on ${ev.Name} failed`;
    case "RivalRaided":
      return "the police raided " + ev.Name;
    case "CornerTaken":
      return `${ev.Rival} took ${ev.Name}`;
    case "CornerLost":
      return ev.Reason === "crackdown" ? "the police cleared " + ev.Name : ev.Name + " went back to the street";
    case "RivalAbandoned":
      return `${ev.Rival} gave up ${ev.Name}`;
    case "CrewQuit":
      return ev.Name + " quit";
    case "CrewDefected":
      return ev.Name + " defected";
    case "CrewArrested":
      return ev.Name + " was arrested";
    case "CrewShot":
      return ev.Name + " was shot dead";
    case "CrewRetired":
      return ev.Name + " retired";
    case "SpyFound":
      return ev.Dead ? ev.Name + " was found and shot" : ev.Name + " came home";
    case "IntelFalse":
      return `the word on ${ev.Name} was ${ev.Rival}'s`;
    case "LieutenantWalked":
      return `${ev.Name} walked with ${ev.CityName}`;
    case "FrontAudited":
      return "an audit at " + ev.Name;
    case "ShipmentSeized":
      return "a shipment seized";
    case "ExportSeized":
      return "a load seized abroad";
    case "TrophySeized":
      return "the feds took " + ev.Name;
    case "DeedSeized":
      return "the DA took the deed to " + ev.Name;
    case "CrewPoached":
      return `${ev.Name} went over to ${ev.Rival}`;
    case "DealOffered":
      return `${ev.Rival} offers ${a(ev.Deal)}`;
    case "DealBroken":
      return ev.By === "you" ? "you broke " + a(ev.Deal) : `${ev.Rival} broke ${a(ev.Deal)}`;
    case "ContractOffered":
      return ev.Name + " is asking";
    case "ChiefReplaced":
      return "a new chief";
    case "DAElected":
      return "the election";
    case "BribeBackfired":
      return "the envelope came back";
    case "RaidFellThrough":
      return `the ${level(ev.Level)} fell through`;
    case "LeadsFiled":
      return "the DA's file on your envelopes";
    case "OfficialsCold":
      return "the officials going cold";
    case "SupplyShort":
      return `the ${productName(v, ev.Product)} contract in ${city(ev.City)} ${supplyShort(ev.Why)}`;
    case "StandingShort":
      return `the standing order for ${productName(v, ev.Product)} in ${city(ev.City)} short of stock`;
    case "HouseRobbed":
      return ev.Name + " robbed";
    case "HouseRaided":
      return `${a(ev.Level)} at ${ev.Name}`;
    case "HouseLost":
      return "the landlord threw you out of " + ev.Name;
  }
  return kind;
}

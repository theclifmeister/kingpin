// Where an alert lands in Street Edition (#549): the tab that shows
// what the alert names, and the thing on it to open or select. The
// engine names the act by the TUI's screens (`act.screen`); the TUI's
// ledger is split here, its fronts, lanes and the wash on The empire,
// its houses in Properties, its account, plan and ways out on Ledger.
// It touches no DOM, so smoke.mjs checks every kind against a table.

// SCREENS are the engine's screens as this page's tabs.
const SCREENS = { dashboard: "street", market: "market", crew: "crew", map: "street", ledger: "ledger", rivals: "rivals" };

// KINDS are the kinds whose screen is not the tab that shows them: the
// fronts and the lanes on The empire, the ways out, the plan, the DA
// race and the favour on Ledger (#552: the law's card is there).
const KINDS = {
  front_shut: "empire",
  float: "empire",
  till: "empire",
  exports: "empire",
  exposure: "empire",
  stash_full: "empire",
  house_known: "empire",
  retire: "ledger",
  reign: "ledger",
  straight: "ledger",
  vanish: "ledger",
  plan: "ledger",
  da_race: "ledger",
  favour: "ledger",
};

// landing is where the alert is answered: `tab`, the `city` to turn
// to, `open` what to open once there (a corner's dialog, a member's,
// Properties, the police's risk, the investigation's confirm or the
// favour's) on
// `id`, `select`, the page element to pick out, or "", and `focus`,
// the field to put the cursor in (the till's, #553), or "".
export function landing(a) {
  const act = a.act || {};
  const out = { tab: KINDS[a.kind] || SCREENS[act.screen] || "street", city: a.city || "", open: "", id: "", select: "", focus: "" };
  const post = act.mode === "post" || act.subject === "corner";
  switch (a.kind) {
    case "arrest":
    case "heat":
    case "task_force":
    case "file":
      out.select = "risk";
      break;
    case "favour":
      // The favour is called in from the law's card (#552): its confirm.
      Object.assign(out, { open: "favour", select: "law" });
      break;
    case "till":
    case "float":
      // The wash panel's till control (#553): raise the till there.
      Object.assign(out, { select: "till", focus: "till-amount" });
      break;
    case "front_shut":
      if (a.front) out.select = "front-" + a.front;
      break;
    case "gate":
      // A front's or an asset's door on The empire (an asset's in
      // Properties); a product's or a connect's on the market.
      out.tab = a.gate && (a.gate.kind === "front" || a.gate.kind === "asset") ? "empire" : "market";
      if (a.gate && a.gate.kind === "asset") out.open = "properties";
      break;
    case "investigation":
      if (a.target === "corner") Object.assign(out, { tab: "street", open: "corner", id: a.corner });
      else if (a.target === "house") Object.assign(out, { tab: "empire", open: "properties", id: a.house });
      else Object.assign(out, { tab: "market", select: "product-" + a.product });
      break;
    case "stash_full":
    case "house_known":
      Object.assign(out, { open: "properties", id: a.house || "" });
      break;
    case "contract_due":
      out.select = "contract-" + a.contract;
      break;
    case "debt_due":
      out.select = "connect-" + a.supplier;
      break;
    case "landed":
      out.select = "product-" + a.product;
      break;
    case "scouts":
      out.select = "faction-" + a.faction;
      break;
    case "retire":
    case "vanish":
    case "straight":
    case "reign":
      // The way out on the Ledger's walk away (#554): its terms, what is
      // short and what it scores.
      out.select = "exit-" + { retire: "retire", vanish: "vanish", straight: "go_straight", reign: "crown" }[a.kind];
      break;
    case "plan":
      out.select = "plan-line";
      break;
    case "da_race":
      // The DA RACE block (#552, the TUI's #534): the vote and the odds.
      out.select = "race";
      break;
    case "talking":
      // Somebody talking is answered by asking around (#551): the
      // investigation's confirm; pages with no bust by the same button,
      // opened when the cause is an informant.
      out.open = "investigate";
      break;
    case "pages":
      if (a.level === "informant") out.open = "investigate";
      else out.select = "investigate";
      break;
    default:
      if (post && a.corner) Object.assign(out, { tab: "street", open: "corner", id: a.corner });
      else if (act.subject === "member" && a.member) Object.assign(out, { tab: "crew", open: "member", id: a.member });
  }
  return out;
}

// alertClass is the alert's class on the page (#549): a danger (the
// engine's `danger`, #504) red, a notice (`notice`) quiet, the rest
// plain.
export function alertClass(a) {
  return "alert-item" + (a.danger ? " danger" : a.notice ? " notice" : "");
}

// UNKNOWN are the preview's `unknown` ids as it words them (the TUI's
// unknownWords).
export const UNKNOWN = {
  robbery: "robberies",
  police: "the police",
  prices: "tomorrow's prices",
  audit: "audits",
  skim: "skims",
  rivals: "the rivals",
  crew: "the crew's nights",
};

// unknownLine is what the preview's estimate leaves out, in words:
// "Robberies, the police and audits are not in it."
export function unknownLine(ids) {
  const words = (ids || []).map((id) => UNKNOWN[id] || id);
  if (!words.length) return "";
  const list = words.length < 2 ? words[0] : `${words.slice(0, -1).join(", ")} and ${words[words.length - 1]}`;
  return `An estimate before the dice: ${list} are not in it.`;
}

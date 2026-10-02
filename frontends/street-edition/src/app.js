import { Session } from "./session.js?v=__BUILD_REVISION__";
import { policeLines } from "./police.js?v=__BUILD_REVISION__";
import { alertText } from "./alerts.js?v=__BUILD_REVISION__";
import { alertClass, landing, unknownLine } from "./landing.js?v=__BUILD_REVISION__";
import { howTheyCome, roleLines, temperLine, temperOf } from "./lieutenants.js?v=__BUILD_REVISION__";
import * as crew from "./crew.js?v=__BUILD_REVISION__";
import { engineInfo } from "./engine-info.js?v=__BUILD_REVISION__";
import * as wash from "./wash.js?v=__BUILD_REVISION__";
import * as lab from "./lab.js?v=__BUILD_REVISION__";
import * as law from "./law.js?v=__BUILD_REVISION__";
import * as endings from "./endings.js?v=__BUILD_REVISION__";
import * as table from "./rivals.js?v=__BUILD_REVISION__";
import * as routine from "./routine.js?v=__BUILD_REVISION__";
import * as property from "./property.js?v=__BUILD_REVISION__";
import * as market from "./market.js?v=__BUILD_REVISION__";
import * as roads from "./routes.js?v=__BUILD_REVISION__";
const $ = (s) => document.querySelector(s),
  esc = (s) =>
    String(s ?? "").replace(
      /[&<>"']/g,
      (c) =>
        ({
          "&": "&amp;",
          "<": "&lt;",
          ">": "&gt;",
          '"': "&quot;",
          "'": "&#39;",
        })[c],
    );
const money = (n) => "$" + Math.round(n || 0).toLocaleString("en-US"),
  pct = (n) => Math.max(0, Math.min(100, n || 0)),
  plural = (n, w) => `${(n || 0).toLocaleString("en-US")} ${w}${n === 1 ? "" : "s"}`;
const paths = {
  map: "M3 5l6-2 6 2 6-2v16l-6 2-6-2-6 2V5zm6-2v16m6-14v16",
  crew: "M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2m16-13a4 4 0 0 1 0 8m4 5v-2a4 4 0 0 0-3-3M13 7a4 4 0 1 1-8 0 4 4 0 0 1 8 0",
  shop: "M3 10v11h18V10M2 10l2-7h16l2 7M3 10q3 4 6 0 3 4 6 0 3 4 6 0M9 21v-7h6v7",
  rival: "M12 3l9 4v6c0 5-9 9-9 9s-9-4-9-9V7l9-4zm-4 9 3 3 5-6",
  ledger:
    "M5 3h15v19H5a3 3 0 0 1-3-3V6a3 3 0 0 1 3-3zm0 0v19m4-14h7m-7 4h7m-7 4h5",
  journal: "M4 3h16v18H4V3zm4 4h8m-8 4h8m-8 4h5",
  arrow: "M5 12h14m-6-6 6 6-6 6",
  coin: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0m-9-6v12m3-9c-4-3-8 2-3 3s1 6-3 3",
  check: "M4 12l5 5L20 5",
  clock: "M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0M12 6v6l4 2",
  sun: "M12 2v2m0 16v2M2 12h2m16 0h2M5 5l2 2m10 10 2 2M5 19l2-2M17 7l2-2M17 12a5 5 0 1 1-10 0 5 5 0 0 1 10 0",
  road: "M8 2 3 22m13-20 5 20M12 3v3m0 4v3m0 4v4",
};
const icon = (k) =>
  `<svg class="icon" viewBox="0 0 24 24" aria-hidden="true"><path d="${paths[k] || paths.map}"/></svg>`;
let session,
  v,
  tab = "street",
  branch = "all",
  orders = [],
  history = [],
  sound = false,
  mutedSave = false,
  timer,
  selectedCity,
  confirmAction,
  quietBroke = null, // the last night that broke a quiet streak (#465), the PLAN line's; not saved, as the TUI's
  saleDial = "quiet"; // the sales approach picked, kept across redraws
// The save's key in this browser. Before #409 this frontend was "Ink &
// Ambition" and kept it under OLD_SAVE: a save found only there is read
// and moved over on the next save, so no run is lost to the rename.
const SAVE = "kingpin-street-v1",
  OLD_SAVE = "kingpin-ink-v1",
  tabs = [
    ["street", "map", "The streets"],
    ["market", "coin", "Market"],
    ["crew", "crew", "Your people"],
    ["empire", "shop", "The empire"],
    ["rivals", "rival", "Rivals"],
    ["ledger", "ledger", "Ledger"],
    ["journal", "journal", "The paper"],
  ];
const city = () => v.cities.find((c) => c.id === v.you.city),
  shownCity = () =>
    v.cities.find((c) => c.id === (selectedCity || v.you.city)) || city();
function query(m, ...p) {
  return session.call(m, ...p);
}
function notify(msg, bad = false) {
  clearTimeout(timer);
  $("#toast").textContent = msg;
  $("#toast").style.background = bad ? "#9d4738" : "#173345";
  $("#toast").classList.add("show");
  timer = setTimeout(() => $("#toast").classList.remove("show"), 4600);
}
function chime() {
  if (!sound) return;
  try {
    const ac = new (window.AudioContext || window.webkitAudioContext)(),
      o = ac.createOscillator(),
      g = ac.createGain();
    o.connect(g);
    g.connect(ac.destination);
    o.type = "sine";
    o.frequency.setValueAtTime(440, ac.currentTime);
    o.frequency.exponentialRampToValueAtTime(660, ac.currentTime + 0.15);
    g.gain.setValueAtTime(0.035, ac.currentTime);
    g.gain.exponentialRampToValueAtTime(0.001, ac.currentTime + 0.3);
    o.start();
    o.stop(ac.currentTime + 0.3);
    o.onended = () => ac.close();
  } catch {}
}
function persist() {
  try {
    localStorage.setItem(
      SAVE,
      JSON.stringify({ save: session.exportSave(), orders, history }),
    );
    localStorage.removeItem(OLD_SAVE);
    mutedSave = false;
  } catch {
    mutedSave = true;
  }
  $("#save-status").textContent = mutedSave
    ? "Storage unavailable — export a save"
    : "Saved on this device";
}
function act(m, p = [], label = "Done", options = {}) {
  try {
    const result = session.call(m, ...p);
    v = session.refresh();
    const events = session.take();
    const broke = events.filter((e) => e.kind === "QuietBroken").pop();
    if (broke) quietBroke = broke.payload;
    if (options.order) {
      orders = orders.filter((o) => o.key !== options.order.key);
      orders.push(options.order);
    }
    if (m === "end_day") {
      orders = [];
      history.unshift({ day: v.day, report: v.report });
      history = history.slice(0, 90);
      chime();
    }
    persist();
    render();
    if (options.close !== false) $("#sheet").close();
    if (label) notify(label);
    if (m === "end_day" && !v.over) morning();
    if (v.over) ending();
    // A command that answers nothing answers true (#552), so a caller
    // can tell it from a refusal, which answers null.
    return result ?? true;
  } catch (e) {
    notify(e.message, true);
    return null;
  }
}
function btn(text, action, id = "", style = "", disabled = false) {
  return `<button class="button ${style}" data-action="${action}" data-id="${esc(id)}" ${disabled ? "disabled" : ""}>${text}</button>`;
}
// alertButton is an alert as the page draws it wherever it is listed
// (the business list, the risk panel, the preview): its words, a
// danger in red (#549), a button to where it is answered.
function alertButton(a) {
  return `<button class="${alertClass(a)}" data-action="alert" data-id="${esc(a.key)}">${esc(alertText(v, a))} →</button>`;
}
function modal(html) {
  $("#sheet-content").innerHTML = html;
  if (!$("#sheet").open) $("#sheet").showModal();
}
// confirm asks before an act: text is a paragraph, or a list of them,
// each a string or [text, class] (a warning in "danger-text").
function confirm(title, text, callback) {
  confirmAction = callback;
  const paras = [].concat(text).map((l) => (Array.isArray(l) ? `<p class="${l[1]}">${esc(l[0])}</p>` : `<p>${esc(l)}</p>`));
  modal(
    `<div class="eyebrow">A DECISION TO MAKE</div><h2>${esc(title)}</h2>${paras.join("")}<div class="card-actions">${btn("Confirm", "confirm", "", "primary")}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
}
function render() {
  if (!v) return;
  $("#day").textContent = "Day " + v.day;
  $("#tier").textContent = v.you.tier_name;
  for (const [el, key] of [
    ["dirty", "dirty_cash"],
    ["clean", "clean_cash"],
    ["offshore", "offshore"],
    ["worth", "net_worth"],
  ])
    $("#" + el).textContent = money(v.you[key]);
  $("#end-day").disabled = !!v.over;
  $("#end-day").innerHTML = v.over
    ? "Story complete ✓"
    : "End the day <span>→</span>";
  $("#nav").innerHTML = tabs
    .map(
      ([id, ic, name]) =>
        `<button data-tab="${id}" class="${tab === id ? "active" : ""}">${icon(ic)}${name}</button>`,
    )
    .join("");
  const title = {
    street: [
      "Your city. Your next move.",
      "Every corner has a story. Choose where yours begins.",
    ],
    market: [
      "A little buying. A little selling.",
      "Buy stock now. Queue sales for the end of the day.",
    ],
    crew: [
      "People make an empire.",
      "Keep them paid, keep them loyal, give them somewhere to work.",
    ],
    empire: [
      "Build something that lasts.",
      "Businesses, properties and the tools of your trade.",
    ],
    rivals: [
      "The other names in town.",
      "Watch their ground. Read their intentions. Choose your battles.",
    ],
    ledger: [
      "Put your money to work.",
      "The pile you hold, the businesses you own, the money you keep.",
    ],
    journal: [
      "The Eastside Chronicle",
      "The city remembers. Read what happened last night.",
    ],
  }[tab];
  $("#section-heading").innerHTML =
    `<div><h2>${title[0]}</h2><p>${title[1]}</p></div>${tab === "street" ? `<div class="city-switch">${v.cities.map((c) => `<button data-action="view-city" data-id="${c.id}" class="${shownCity().id === c.id ? "active" : ""}">${esc(c.name)}</button>`).join("")}</div>` : ""}`;
  renderRisk();
  renderBusiness();
  const renders = {
    street: renderStreet,
    market: renderMarket,
    crew: renderCrew,
    empire: renderEmpire,
    rivals: renderRivals,
    ledger: renderLedger,
    journal: renderJournal,
  };
  $("#main-content").innerHTML = renders[tab]();
  $("#tip-text").textContent = {
    street: "Own your choices before you try to own the city.",
    market: "A sale happens tonight. The heat arrives with it.",
    crew: "A loyal crew costs money. A disloyal one costs more.",
    empire: "Big plans are easier to afford with a little cash in reserve.",
    rivals: "A rival with no corners may still have a place at the table.",
    ledger: "Net worth is not spending money. Keep an eye on the till.",
    journal: "Yesterday’s small decisions become tomorrow’s headlines.",
  }[tab];
}
// warrantHTML is the risk panel's warrant marker (#549, the TUI's heat
// panel, #519): "WARRANT: served tonight on a sale", or "".
function warrantHTML() {
  const a = v.alerts.find((x) => x.kind === "arrest");
  if (!a) return "";
  return `<p class="warrant">WARRANT: served ${(a.days || 0) <= 1 ? "tonight" : "in " + plural(a.days, "night")} on a sale</p>`;
}
function renderRisk() {
  const c = city(),
    limit = v.law.arrest_line,
    others = v.cities.filter((x) => x.id !== c.id);
  $("#risk").innerHTML =
    `${warrantHTML()}<div class="risk-line"><div class="risk-label"><span>City heat</span><strong>${Math.round(c.heat)} / 100</strong></div><div class="meter"><span style="width:${pct(c.heat)}%"></span></div>${others.length ? `<small class="subtle-text other-heat">${others.map((x) => `${esc(x.name)} heat ${Math.round(x.heat)}`).join(" · ")}</small>` : ""}</div><div class="risk-line"><div class="risk-label"><span>Evidence${v.cities.length > 1 ? ` <small class="subtle-text">one file, every city</small>` : ""}</span><strong class="${limit && v.you.evidence >= limit - 2 ? "danger-text" : ""}">${v.you.evidence} / ${limit || "—"}</strong></div><div class="evidence-dots">${Array.from({ length: Math.min(limit, 24) }, (_, i) => `<i class="${i < v.you.evidence ? "filled" : ""}"></i>`).join("")}</div></div>${v.alerts
      .filter((a) => a.danger)
      .map(alertButton)
      .join(
        "",
      )}${patrolCapHTML()}${lawPanelHTML()}<p class="${v.you.dirty_cash > v.law.exposure_line ? "danger-text" : ""}">Dirty cash exposure: ${money(v.you.dirty_cash)} / ${money(v.law.exposure_line)}</p>${tonightHTML()}<details id="police-risk"><summary>Understand the police risk</summary>${policeLines(
      v,
      c.id,
    )
      .map(
        (l) => `<p class="${l.warn ? "danger-text" : ""}">${esc(l.text)}</p>`,
      )
      .join("")}</details>`;
  $("#lie-low").textContent = v.you.lie_low
    ? "Resume street sales"
    : "Take a quiet day";
}
// The dashboard's facts (#574, ui/dashboard.go streetLines): the
// patrol's cap on the risk panel; where the enforcers go tonight and
// the crew's trouble on the street view, the trouble on the crew tab too.
function patrolCapHTML() {
  const line = law.patrolCapLine(v);
  return line ? `<p id="patrol-cap" class="danger-text">${esc(line)}</p>` : "";
}
function troubleHTML() {
  const line = crew.trouble(v, query("rules.crew.tuning"), query("rules.crew.flip_line"));
  return line ? `<div id="crew-trouble" class="tip-box warn-text" style="margin-bottom:16px">Crew: ${esc(line)}</div>` : "";
}
// strikeOrder is the strike queued tonight, off the memo, or undefined.
function strikeOrder() {
  return orders.find((o) => o.key === "strike" && o.corner);
}
function streetFactsHTML() {
  const tonight = table.tonightLine(v, query, engineInfo, strikeOrder());
  return `${tonight ? `<div id="tonight-strike" class="tip-box warn-text" style="margin-bottom:16px">${esc(tonight)}</div>` : ""}${troubleHTML()}`;
}
function renderBusiness() {
  let h = "";
  if (v.card)
    h += `<button class="choice" data-action="dilemma">${icon("journal")} ${esc(v.card.title)} <small>A decision is waiting →</small></button>`;
  if (v.over) h += btn("Read your ending", "ending", "", "primary full");
  for (const a of v.alerts) h += alertButton(a);
  if (v.you.lie_low)
    h += `<div class="todo">${icon("sun")}<div><span>A quiet night</span><small>${esc(law.lieLowSaid(v))}</small></div></div>`;
  const queued = [...routine.cart(v, query).filter((l) => l.kind !== "keep" && !routine.isBuy(l)).map((l) => routine.cartLine(v, l)), ...orders.map((o) => o.text)];
  h += queued
    .slice(0, 4)
    .map(
      (t) =>
        `<div class="todo">${icon("check")}<div><span>${esc(t)}</span><small>Queued for tonight</small></div></div>`,
    )
    .join("");
  if (!queued.length && !v.card)
    h += `<div class="todo">${icon("clock")}<div><span>Nothing queued yet</span><small>Visit the market, hire someone, or choose a corner.</small></div></div>`;
  h += `<div class="todo">${icon("map")}<div><span>${city().corners.filter((c) => c.owner === "player").length} / ${city().corners.length} corners held</span><small>${esc(city().name)} · ${v.crew.length} people on payroll</small></div></div>`;
  const plan = v.ambitions.find((a) => a.pinned);
  if (plan)
    h += `<div class="todo"><div><b>${esc(plan.name)}</b><div class="meter teal"><span style="width:${plan.progress * 100}%"></span></div><small>${esc(plan.steps.find((x) => x.id === plan.next)?.label || "Plan complete")}</small>${btn("View plan", "plans", "", "small subtle")}</div></div>`;
  h += btn("Preview tonight", "preview", "", "small full", !!v.over);
  $("#business").innerHTML = h;
}
function renderStreet() {
  const c = shownCity(),
    maxX = Math.max(...c.corners.map((x) => x.x)),
    maxY = Math.max(...c.corners.map((x) => x.y));
  const markers = c.corners
    .map((corner, i) => {
      const x = 12 + (corner.x / (maxX || 1)) * 72 + (corner.y % 2 ? 3 : 0),
        y = 25 + (corner.y / (maxY || 1)) * 54;
      return `<button class="map-marker ${corner.owner}" style="left:${x}%;top:${y}%" data-action="corner" data-id="${corner.id}" aria-label="${esc(corner.name)}, ${corner.owner}"><i>${corner.owner === "player" ? "✓" : corner.owner === "rival" ? "!" : i + 1}</i><span class="pin-label">${esc(corner.name)}<small>${corner.owner === "player" ? "Yours" : corner.owner === "rival" ? (strikeOrder()?.corner === corner.id ? `⚔ ${esc(strikeOrder().force)} tonight` : "Rival ground") : "Unclaimed"}</small></span></button>`;
    })
    .join("");
  return `${streetFactsHTML()}<div class="map-frame"><img class="city-art" src="assets/city.webp" alt="Hand-drawn waterfront city in colorful ink"><div class="map-wash"></div><div class="map-badge">${esc(c.name.toUpperCase())} · ${c.id === v.you.city ? "YOU ARE HERE" : "ACROSS THE WATER"}</div>${markers}<div class="map-caption">Small beginnings. Bigger possibilities.</div></div><div class="map-legend"><div class="legend-keys"><span><i class="dot"></i>Your ground</span><span><i class="dot rival"></i>Rival ground</span><span><i class="dot none"></i>Unclaimed</span></div><span>Select a corner to act ↗</span></div>${c.id !== v.you.city ? btn("Travel to " + esc(c.name), "travel", c.id, "full") : ""}<div class="quick-actions"><button class="quick-card" data-tab="market">${icon("coin")}<div><b>Work the market</b><small>Stock up & queue sales →</small></div></button><button class="quick-card" data-tab="crew">${icon("crew")}<div><b>Meet your people</b><small>Hire, post & look after →</small></div></button><button class="quick-card" data-tab="empire">${icon("shop")}<div><b>Think bigger</b><small>Businesses & upgrades →</small></div></button></div>${dispatch()}`;
}
function dispatch() {
  const lead = v.report.lead || [],
    news = v.report.sections?.find((s) => s.id === "news")?.lines || [],
    sales = v.report.sections?.find((s) => s.id === "sales")?.lines || [];
  return `<section class="dispatch"><div class="dispatch-heading"><h3>Around the neighborhood</h3><small>THE DAILY DISPATCH · ${v.day === 0 ? "FIRST EDITION" : "DAY " + v.report.day}</small></div><div class="headlines">${lead.length ? lead.map((l, i) => `<button class="headline headline-link" data-action="lead" data-id="${i}"><small>TODAY · ${esc(l.kind)}</small>${esc(l.text)} →</button>`).join("") : `<div class="headline"><small>WORD ON THE STREET</small>${esc(news[0] || (v.day === 0 ? "A new face in Eastside. A single corner. Five hundred dollars. What happens next is up to you." : "The city is watching. Read the paper for last night’s events."))}</div><div class="headline"><small>${v.day ? "THE NIGHT’S BUSINESS" : "YOUR FIRST MOVE"}</small>${esc(sales[0] || news[1] || (v.day ? "Plan your next move, and preview tonight before you commit." : "Visit the market to buy stock, then queue a sale. Nothing moves until you end the day."))}</div>`}</div></section>`;
}
function renderMarket() {
  const c = city(),
    stock = v.you.stock[c.id] || {};
  return `${marketTools()}${suppliersHTML(c.id)}<div class="table-wrap"><table class="data-table"><thead><tr><th>PRODUCT</th><th>BUY / SELL</th><th>STOCK</th><th>QUANTITY</th><th>YOUR MOVE</th></tr></thead><tbody>${c.products
    .map((p) => {
      const supplier = market.sellers(v, p.id)[0],
        room = supplier ? session.maxBuy(supplier.id, p.id) : null;
      return `<tr id="product-${p.id}"><td><b>${esc(p.name)}</b><br><small class="subtle-text">Demand ${Math.round(p.demand)}</small>${routineRowsHTML(c.id, p.id)}</td><td>${supplier ? money(supplier.prices[p.id]) : "—"} / ${money(p.price)}</td><td>${routine.stock(v, c.id, p.id)}${labQualityHTML(p.id)}${room ? `<br><small>Stash ${room.held}/${room.capacity}</small>` : ""}</td><td><input aria-label="${esc(p.name)} quantity" type="number" min="1" max="99999" value="${Math.min(10, routine.stock(v, c.id, p.id) || 10)}" id="qty-${p.id}">${supplier ? btn("Max " + room.max, "max-buy", p.id, "small subtle", !room.max || !!v.over) : ""}</td><td>${btn("Buy", "buy", p.id, "small", !supplier || !room?.max || !!v.over)} ${btn("Sell", "sell", p.id, "small", !routine.sellable(v, query, c.id, p.id) || !!v.over)} ${btn("Routine", "routine", p.id, "small subtle", !!v.over)} ${btn("Details", "product-pane", p.id, "small subtle")}</td></tr>`;
    })
    .join(
      "",
    )}</tbody></table></div><div class="row" style="margin-top:14px"><label class="subtle-text">Sales approach <select id="sale-dial" class="inline-select" data-change="sale-dial">${[["quiet", "Quiet · lower profile"], ["normal", "Normal · balanced"], ["aggressive", "Aggressive · more heat"]].map(([d, label]) => `<option value="${d}" ${d === saleDial ? "selected" : ""}>${label}</option>`).join("")}</select></label>${btn("Sell all held stock", "sell-all", "", "small", !!v.over)}</div>${cartHTML()}${labPanelHTML()}<h3 class="section-gap">Private buyers</h3>${v.contracts.length ? `<div class="cards">${v.contracts.map((c) => `<article class="card" id="contract-${c.id}"><span class="tag">${esc(c.status)}</span><h3>${esc(c.name)}</h3><p>${esc(c.pitch)}</p><div class="stat-row"><span><b>${c.units}</b>units</span><span><b>${c.delivered}</b>delivered</span><span><b>${c.due}</b>due day</span></div>${c.status === "offered" ? btn("Accept contract", "contract", c.id, "small") : btn("Deliver stock", "deliver", c.id, "small")}</article>`).join("")}</div>` : '<div class="empty">No private offers today. Check back tomorrow.</div>'}`;
}
function renderCrew() {
  const lt = query("rules.crew.lieutenancy"),
    tun = query("rules.crew.tuning"),
    flip = query("rules.crew.flip_line"),
    most = query("rules.crew.max_crew"),
    hint = howTheyCome(v),
    warn = crew.crewWarning(v, tun);
  return `${hint ? `<div class="tip-box" style="margin-bottom:16px">${esc(hint)}</div>` : ""}${troubleHTML()}${warn ? `<div class="tip-box danger-text" style="margin-bottom:16px">${esc(warn)}</div>` : ""}<div class="row" style="margin-bottom:16px"><span class="subtle-text">${esc(crew.countLine(v, most))}</span><label>Pay <select id="pay-dial" data-change="pay">${crew.PAY.map((x) => `<option value="${x}" ${v.you.pay === x ? "selected" : ""}>${x} · ${money(query("rules.crew.wages", x))}/day</option>`).join("")}</select></label>${askAroundButton()}</div><p class="subtle-text">${esc(crew.payBlurb(v.you.pay))}</p><div class="cards">${v.crew.map((m) => memberCard(m, lt, tun, flip)).join("") || '<div class="empty">For now, it’s just you. Find someone you can count on below.</div>'}</div>${crewSummary()}<h3 class="section-gap">New faces in town <small class="subtle-text">· new faces in ${plural(crew.poolNext(v, query("rules.crew.pool_days")), "day")}</small></h3><div class="cards">${v.pool.map((m) => `<article class="card"><div class="card-top"><span class="tag">${esc(m.role)}</span><span class="price">${money(m.fee)}</span></div><h3>${esc(m.name)}${kinMark(m)}</h3><p>Age ${m.age} · Skill ${m.skill} · Loyalty ${Math.round(m.loyalty)}%<br>Base wage ${money(m.wage)} / day${kinHTML(m, true)}${labHandHTML(m, true)}</p>${m.role === "lieutenant" ? `<p class="front-role">${roleLines(lt).slice(0, 2).map(esc).join(" ")}</p>` : ""}${btn("Hire " + esc(m.name), "hire", m.id, "small", v.you.dirty_cash < m.fee || !!v.over)}</article>`).join("")}</div>${[...v.crew, ...v.pool].some((m) => (m.kin || []).length) ? `<p class="subtle-text">${esc(crew.KIN_LEGEND)}</p>` : ""}`;
}
// kinMark is the mark on a name with kin (#576, ui kinGlyph), and
// kinHTML the kin row under a member or a face (ui personLines).
function kinMark(m) {
  return (m.kin || []).length ? ` <span class="subtle-text" title="has kin on the payroll or looking for work">${crew.KIN}</span>` : "";
}
function kinHTML(m, inPool) {
  const line = crew.kinLine(v, m, inPool);
  return line ? `<br><small>${esc(line)}</small>` : "";
}
// The crew tab's answers (#551, the TUI's crew screen): each member's
// card with where they are, the SNITCH mark, the lines they cross and
// the acts with their prices; the ask around; the CREW summary.
function askAroundButton() {
  const label = v.crew.length ? `Ask around · ${money(query("rules.crew.investigate_cost"))}, names ~${Math.round(query("rules.crew.investigate_odds") * 100)}%` : "Ask around · nobody to ask";
  return `<button id="investigate" class="button small" data-action="investigate" ${!v.crew.length || v.over ? "disabled" : ""}>${label}</button>`;
}
function memberCard(m, lt, tun, flip) {
  const tag = crew.crewTag(v, m, query("rules.crew.retiring", m.id)),
    where = crew.post(v, m),
    status = m.role === "lieutenant" && !m.jailed && !m.wounded ? lieutenantStatus(m, lt) : `<span class="${where.warn ? "danger-text" : ""}">${esc(where.text[0].toUpperCase() + where.text.slice(1))}</span>`;
  return `<article class="card" id="member-${m.id}"><div class="crew-header"><div class="portrait ${m.role === "runner" ? "teal" : ""}">${esc(m.name.slice(0, 1))}</div><div><span class="tag">${esc(m.role)}</span>${m.exposed ? ' <span class="tag coral">SNITCH</span>' : ""}${tag ? ` <span class="tag ${tag === "retiring" ? "gold" : "coral"}">${esc(tag)}</span>` : ""}<h3>${esc(m.name)}${kinMark(m)}</h3></div></div><div class="stat-row"><span><b>${m.skill}</b>skill</span><span><b>${Math.floor(m.loyalty)}</b>loyalty</span><span><b>${money(query("rules.crew.wage_at", m.id, v.you.pay))}</b>a day, ${esc(v.you.pay)}</span></div><div class="meter teal"><span style="width:${pct(m.loyalty)}%"></span></div><p><small class="${m.loyalty < crew.lineOf(m, tun, flip) ? "danger-text" : "subtle-text"}">${esc(crew.linesLine(m, tun, flip))}</small><br>${m.trait ? `<span class="tag">${esc(m.trait)}</span><small> ${esc(engineInfo.traits[m.trait] || "")}</small> ` : ""}${m.captain ? `<b>Captain of ${esc(cityName(m.captain))}</b> · ${money(m.budget)} a night<br>` : ""}${status}${kinHTML(m, false)}${labHandHTML(m, false)}${m.exposed ? '<br><span class="danger-text">Talking to the police.</span>' : ""}</p><div class="card-actions">${["runner", "enforcer"].includes(m.role) ? btn("Assign corner", "assign-person", m.id, "small") : ""}${m.role === "lieutenant" ? btn(m.city ? "Change city" : "Run a city", "lieutenant-city", m.id, "small") : ""}${m.jailed && !m.bailed ? btn(`Bail · ${money(query("rules.crew.bail_cost", m.id))} clean`, "bail", m.id, "small", !!v.over) : ""}${btn(`Pay off · ${money(query("rules.crew.payoff_cost", m.id))}`, "bonus", m.id, "small", !!v.over)}${m.role !== "lieutenant" ? btn("Captaincy", "captain", m.id, "small subtle") : ""}${btn("Details", "crew-detail", m.id, "small subtle")}</div></article>`;
}
// crewSummary is the CREW block under the cards (the TUI pane's CREW).
function crewSummary() {
  const rows = crew.summary(v, query("rules.heat.sloppy_heat", v.you.city, 100), engineInfo.sloppySkill);
  return `<h3 class="section-gap">The crew</h3><div class="paper">${rows.map(([k, t, bad]) => `<p>${k ? `<b>${esc(k)}</b> · ` : ""}<span class="${bad ? "danger-text" : ""}">${esc(t)}</span></p>`).join("")}</div>`;
}
// memberSheet is a member's details (the TUI pane's person): the role,
// the age and when they retire, the loyalty and its lines, the wage at
// every notch, where they are, the temper, and the acts with what each
// would do.
function memberSheet(id) {
  const m = v.crew.find((x) => x.id === Number(id)),
    tun = query("rules.crew.tuning"),
    flip = query("rules.crew.flip_line"),
    life = query("rules.crew.life"),
    retiring = query("rules.crew.retiring", m.id),
    where = crew.post(v, m),
    lt = query("rules.crew.lieutenancy");
  const age = m.age ? `Age ${m.age}${life.YearDays > 0 ? (retiring ? ` · retires Day ${query("rules.crew.birthday", m.id, v.day)}` : ` · retires at ${life.RetireAge}`) : ""}` : "";
  const wages = crew.PAY.map((p) => `${p} ${money(query("rules.crew.wage_at", m.id, p))}`).join(" · ");
  const temper = m.role !== "lieutenant" ? "" : m.personality ? `Temper: ${m.personality} · ${temperLine(temperOf(lt, m.personality) || {})}` : m.city ? `Temper shows after ${Math.max(1, lt.RevealDays - (v.day - m.assigned))} more days running it` : "Temper shows on the job";
  const fire = m.exposed ? "Fire: the file stops growing; nobody minds." : `Fire: the rest lose ${Math.round(tun.FireLoyalty)} loyalty, not for a snitch.`;
  modal(
    `<span class="tag">${esc(m.role)}</span>${m.exposed ? ' <span class="tag coral">SNITCH</span>' : ""}<h2>${esc(m.name)}</h2><p>Skill ${m.skill} · Hired Day ${m.hired}${age ? "<br>" + esc(age) : ""}<br>Loyalty ${Math.floor(m.loyalty)} · <span class="subtle-text">${esc(crew.linesLine(m, tun, flip))}</span><br>Wage ${money(query("rules.crew.wage_at", m.id, v.you.pay))}/day ${esc(v.you.pay)} · <span class="subtle-text">${esc(wages)}</span><br>Carry capacity ${m.carry}${kinHTML(m, false)}<br><span class="${where.warn ? "danger-text" : ""}">${esc(where.text[0].toUpperCase() + where.text.slice(1))}</span>${temper ? "<br>" + esc(temper) : ""}${keepsHTML(m)}${labHandHTML(m, false)}</p>${m.exposed ? '<p class="danger-text"><b>SNITCH</b>: talking to the police.</p>' : ""}<p class="subtle-text">${esc(fire)}</p><div class="card-actions">${m.jailed && !m.bailed ? btn(`Bail · ${money(query("rules.crew.bail_cost", m.id))} clean`, "bail", id, "small", !!v.over) : ""}${btn(`Pay off · ${money(query("rules.crew.payoff_cost", m.id))}`, "bonus", id, "small", !!v.over)}${btn("Fire", "fire", id, "subtle", !!v.over)}</div>`,
  );
}
function renderEmpire() {
  const p = session.preview(),
    onOffer = query("front_offers") || [],
    offers = query("rules.laundering.offers").filter((f) => v.fronts.some((x) => x.id === f.ID) || onOffer.some((o) => o.ID === f.ID));
  return `<div class="row"><h3>Your businesses</h3><select aria-label="Laundering approach" data-change="launder">${["careful", "normal", "greedy"].map((x) => `<option ${x === v.you.launder ? "selected" : ""}>${x}</option>`).join("")}</select></div>${washPanelHTML(p)}<div class="cards" style="margin-top:15px">${offers
    .map((f) => {
      const own = v.fronts.find((x) => x.id === f.ID),
        level = own ? query("rules.laundering.levels", own.id, 1) : null;
      return `<article class="card" id="front-${esc(f.ID)}"><div class="card-top">${icon("shop")}<span class="tag ${own ? "" : "gold"}">${own ? "LEVEL " + own.level : "BUSINESS OPPORTUNITY"}</span></div><h3>${esc(f.Name)}</h3><p class="front-role">${esc(engineInfo.frontRoles[f.ID] || "")}</p><p>Base capacity ${money(f.Throughput)} / day<br>Base upkeep ${money(f.Upkeep)} clean / day<br>Base audit risk ${wash.offerAudit(f)} / day</p>${own ? `${frontStatusHTML(own)}${frontRowsHTML(own, f)}<span class="subtle-text">${money(own.washed)} washed</span><div class="card-actions">${btn(level.Levels > 0 ? "Invest " + money(level.Cost) : "Maximum level", "invest", own.id, "small", level.Levels === 0 || v.you.clean_cash < level.Cost || !!v.over)}</div>` : `${frontOfferHTML(f, p)}<div class="row"><strong class="price">${money(f.Cost)}</strong>${btn("Buy business", "buy-front", f.ID, "small", v.you.dirty_cash < f.Cost || !!wash.offerLock(v, f) || !!v.over)}</div>`}</article>`;
    })
    .join(
      "",
    )}</div>${onOffer.length ? "" : `<p class="subtle-text">${esc(wash.noFrontsOnOffer(query))}</p>`}${lanesHTML()}${trophiesHTML()}<div class="row section-gap"><h3>Make your next investment</h3>${btn("Properties & assets", "properties", "", "small subtle")}</div><div class="filters">${["all", "operations", "security", "legal", "crew", "laundering", "street", "logistics"].map((b) => `<button data-branch="${b}" class="${branch === b ? "active" : ""}">${b}</button>`).join("")}</div><div class="cards">${v.upgrades
    .filter((u) => branch === "all" || u.branch === branch)
    .map(
      (u) =>
        `<article class="card"><div class="card-top"><span class="tag ${u.state === "owned" ? "" : "gold"}">${esc(u.branch)}</span><small>${esc(u.state)}</small></div><h3>${esc(u.name)}</h3><p>${esc(u.desc)}</p>${u.requires.length ? `<p>Requires: ${routine.requiresNames(v, u).map((r) => `<span class="${r.owned ? "good-text" : ""}">${esc(r.name)}${r.owned ? " ✓" : ""}</span>`).join(", ")}</p>` : ""}<div class="row"><span><strong>${money(u.cost)}</strong> <small>${u.clean ? "clean" : "dirty"}</small></span>${btn(u.state === "owned" ? "Owned ✓" : "Buy upgrade", "upgrade", u.id, "small", u.state !== "available" || v.you[u.clean ? "clean_cash" : "dirty_cash"] < u.cost || !!v.over)}</div></article>`,
    )
    .join("")}</div>`;
}
// The wash, the till and the road (#553), as the TUI's ledger words
// them (wash.js): the audit odds, capacity and legit income (#577); the
// till and its control; the pile's rot, the wash idle under it, a route
// waiting on a lot, the fronts the night is expected to shut, the tax; a
// front's status and why; an offer's terms, its lock and its shut.
const tone = (t) => ({ warn: "warn-text", danger: "danger-text", bad: "danger-text", subtle: "subtle-text", good: "good-text", gold: "warn-text", road: "subtle-text" })[t] || "";
function washPanelHTML(p) {
  const t = wash.till(query),
    lines = wash.washLines(v, query, p);
  return `<section class="paper wash-panel" id="till"><div class="eyebrow">THE WASH AND THE TILL</div><p class="subtle-text">${esc(wash.odds(query))}</p><div class="row"><span>The till${t.set ? "" : ' <small class="subtle-text">(the float)</small>'}</span><b>${money(t.till)} dirty kept</b></div>${t.line > t.till ? `<div class="row"><span>The contracts' morning</span><b>${money(t.outlay)} kept back</b></div>` : ""}<p class="subtle-text">${esc(wash.tillWords(query))}</p>${lines.map((l) => `<p class="wash-line"><small>${esc(l.label.toUpperCase())}</small> <span class="${tone(l.tone)}">${esc(l.text)}</span><span class="subtle-text">${esc(l.more)}</span></p>`).join("")}<p>${esc(wash.tonight(v, query))}</p><label class="row till-set"><input id="till-amount" aria-label="The till, in dirty dollars" type="number" min="0" step="1000" placeholder="blank = the float"${lines.find((l) => l.raise) ? ` value="${lines.find((l) => l.raise).raise}"` : ""}>${btn("Set the till", "set-till", "", "small", !!v.over)}</label><p class="subtle-text">${esc(wash.tillRules(query))}</p></section>`;
}
function frontStatusHTML(f) {
  const st = wash.frontStatus(v, query, f),
    covered = wash.coveredNights(v, query, f);
  return `<p class="front-status ${tone(st.tone)}"><b>${esc(st.text)}</b>${covered ? `<br><small class="subtle-text">Upkeep covered ${plural(covered, "more night")}</small>` : ""}</p>`;
}
// frontRowsHTML is an owned front's wash (#577): its real throughput
// at the dial and its audit odds.
function frontRowsHTML(f, o) {
  return `<p class="front-rows">${wash
    .frontRows(v, query, f, o)
    .map(([k, t]) => `<small class="subtle-text">${esc(k)}</small> ${esc(t)}`)
    .join("<br>")}</p>`;
}
function frontOfferHTML(o, p) {
  const lock = wash.offerLock(v, o);
  if (lock) return `<p class="subtle-text">${esc(lock)}</p>`;
  const shut = wash.frontShutWarning(v, query, o, p);
  return `<p class="subtle-text">${esc(wash.frontTerms(query, o))}</p>${shut ? `<p class="danger-text">${esc(shut)}</p>` : ""}`;
}
// upkeepHTML is the money cards' line on tonight's clean bill (#458):
// the defaults keep it back.
function upkeepHTML() {
  const due = wash.upkeepTonight(query);
  return due > 0 ? `<p class="subtle-text">Upkeep tonight: ${money(due)} clean. The amount filled in keeps it back.</p>` : "";
}
function renderRivals() {
  return `${v.alerts
    .filter((a) => a.kind === "scouts")
    .map(
      (a) =>
        `<div class="paper"><p>${esc(alertText(v, a))}</p>${btn("Confront scouts", "hit-scouts", scoutFaction(a), "small", !!v.over)}</div>`,
    )
    .join("")}${crownHTML()}${v.proposal ? `<p class="warn-text">${esc(table.proposalLine(v, query))}</p>` : ""}<div class="cards">${v.factions.map(factionCard).join("")}</div><h3 class="section-gap">Offers on the table</h3>${v.offers.length ? v.offers.map(offerHTML).join("") : '<div class="empty">No offers today. The table is quiet.</div>'}<div class="cards section-gap"><article class="card"><div class="eyebrow">RULES</div>${table.DEAL_RULES.map((l) => `<p class="subtle-text">${esc(l)}</p>`).join("")}</article><article class="card"><div class="eyebrow">LIFETIME</div>${rowsHTML(table.lifetimeRows(v))}</article></div>`;
}
// crownHTML is the crown's card (#399, #472, #530): what it waits on,
// each crew off the count with why and for how long, the last clock.
function crownHTML() {
  const city = endings.plan(v, "city");
  if (!city) return "";
  const rows = endings.downRows(v, query),
    last = endings.lastClockWords(v, query),
    open = city.done;
  return `<section class="paper crown-card" id="crown"><div class="eyebrow">THE CROWN</div><h3>${open ? `Day ${endings.reignDay(v)} of the reign` : "Not yours yet"}</h3><p class="${open ? "good-text" : "subtle-text"}">${esc(open ? endings.reignIncome(v, query) : endings.crownShort(v, query))}</p>${rows.length ? `<p><b>Off the count</b></p>${rows.map((r) => `<p class="law-row"><small>${esc(r.name.toUpperCase())}</small> <span class="subtle-text">${esc(r.words)}</span></p>`).join("")}` : ""}${last && rows.length ? `<p class="subtle-text">${esc(last[0].toUpperCase() + last.slice(1))}.</p>` : ""}</section>`;
}
// factionCard is a faction at the table (the TUI's rivals screen and
// pane): its line, what keeps it off the crown's count, trust and the
// war, its deals with their terms and days left, where you stand, and
// its moves: scout, propose, declare or call off the war.
function factionCard(f) {
  const atWar = v.you.war === f.id,
    down = endings.downWords(v, query, f),
    [mood, moodTone] = table.moodLine(v, query, f),
    deals = table.dealRows(v, f),
    tun = query("rules.rivals.tuning"),
    nobody = atWar ? table.warNobodyWords(v) : "",
    muscle = atWar ? table.warMuscleLine(v, engineInfo) : "",
    allies = table.alliesLine(v, query);
  return `<article class="card" id="faction-${esc(f.id)}"><div class="card-top">${icon("rival")}<span class="tag ${atWar || f.stance === "war" ? "coral" : f.alive ? "" : "gold"}">${esc((atWar ? "at war" : f.stance).toUpperCase())}</span></div><h3>${esc(table.rivalName(f))}</h3><p>${esc(table.factionLine(v, query, f))}${f.city ? ` · ${esc(v.cities.find((c) => c.id === f.city)?.name || f.city)}` : ""}</p>${down ? `<p class="subtle-text">For the crown: ${esc(down)}</p>` : ""}<div class="stat-row"><span><b>${f.corners}</b>corners</span><span><b class="${f.trust < 20 ? "danger-text" : f.trust >= 60 ? "good-text" : ""}">${Math.round(f.trust)}</b>trust</span><span><b class="${f.war >= tun.WarThreshold ? "danger-text" : ""}">${Math.round(f.war)}/${Math.round(tun.CrackdownThreshold)}</b>war</span></div>${
    deals.length
      ? `<div class="table-wrap"><table class="data-table"><thead><tr><th>DEAL</th><th>TERMS</th><th>DAYS</th><th>WHO</th></tr></thead><tbody>${deals.map((d) => `<tr><td>${esc(d.kind)}</td><td>${esc(d.terms)}${d.kind === "tribute" ? `<br><small>${esc(table.tributeCut(query, f, f.deal_terms.find((x) => x.kind === "tribute").terms.per_day))}</small>` : ""}</td><td>${d.left ?? "-"}</td><td>${d.who}</td></tr>`).join("")}</tbody></table></div>`
      : ""
  }${f.arrived ? `<p class="${tone(moodTone)}">${esc(mood)}</p>` : ""}${allies ? `<p class="subtle-text">Allies: ${esc(allies)}</p>` : ""}${nobody ? `<p class="warn-text">War: ${esc(nobody)}</p>` : ""}${muscle ? `<p class="danger-text">${esc(muscle)}</p>` : ""}${f.books ? `<p>Last scouted on Day ${f.books.day}<br>${money(f.books.cash)} cash · ${f.books.muscle} muscle</p>` : ""}<div class="card-actions">${btn("Scout", "scout", f.id, "small", !f.alive || !!v.over)}${btn("Propose", "diplomacy", f.id, "small", !f.alive || !!v.over)}${atWar ? btn("Call off the war", "call-off-war", f.id, "small", !!v.over) : btn("Declare war", "declare-war", f.id, "small subtle", !f.alive || !!v.over)}</div></article>`;
}
// offerHTML is an offer on the table: who, what, the days to answer,
// what it does and what breaks it, a tribute's cut of your street.
function offerHTML(o) {
  const f = v.factions.find((x) => x.id === o.faction),
    left = o.expires - v.day + 1;
  return `<div class="paper row"><div><h3>${esc(f?.leader || o.faction)} offers ${esc(dealWords(o.kind, o.terms))}</h3><p class="subtle-text">${plural(left, "day")} to answer (until Day ${o.expires})${o.kind === "tribute" && f ? ` · ${esc(table.tributeCut(query, f, o.terms.per_day))}` : ""}</p><p>${esc(table.dealDoes(o.kind))}</p><p class="subtle-text">${esc(table.dealBreaks(o.kind))}</p></div><div>${btn("Accept", "accept", o.id, "small")}${btn("Decline", "decline", o.id, "small subtle")}</div></div>`;
}
// proposeDialog is the propose dialog (ui/diplomacy.go viewPropose):
// the kinds with a live deal's terms and withdraw, then a kind's three
// standard asks with the odds the dice use, the tribute's basis or the
// split's side, and the distrust after a betrayal.
function proposeDialog(id, kind = "") {
  const f = v.factions.find((x) => x.id === id),
    distrust = table.distrusted(v, query, f) ? '<p class="danger-text">They are not taking your calls. You broke a deal.</p>' : "";
  if (!kind) {
    modal(
      `<div class="eyebrow">A SEAT AT THE TABLE</div><h2>Propose to ${esc(table.rivalName(f))}</h2><p class="subtle-text">${esc(f.personality === "?" ? "unknown" : f.personality)} · trust ${Math.round(f.trust)}</p>${table.PROPOSE_KINDS.map(([k, note]) => {
        const live = table.liveDeal(v, f, k);
        return `<button class="choice" data-action="propose-kind" data-id="${esc(id)}|${k}"><b>${k}</b> <span class="${live ? "good-text" : k === "shipment" ? "subtle-text" : ""}">${esc(live ? "live: " + table.dealTerms(v, live) : note)}</span></button>`;
      }).join("")}${v.proposal ? `<button class="choice" data-action="withdraw"><b>withdraw</b> take back tonight's proposal, ${esc(dealWords(v.proposal.kind, v.proposal.terms))}</button>` : ""}${distrust}`,
    );
    return;
  }
  const rows = table.termRows(v, query, f, kind);
  modal(
    `<div class="eyebrow">A SEAT AT THE TABLE</div><h2>Propose to ${esc(table.rivalName(f))}</h2><p class="subtle-text">${esc(kind[0].toUpperCase() + kind.slice(1))} to ${esc(table.rivalName(f))}. Odds are what the dice use.</p>${rows
      .map(
        (r, i) =>
          `<button class="choice" data-action="propose" data-id="${esc(id)}|${kind}|${i}"><b>${esc(r.label)}</b> ${esc(r.words)} <span class="${r.odds === 0 ? "danger-text" : "warn-text"}">${r.odds === 0 ? "refused" : "~" + Math.round(r.odds * 100) + "%"}</span>${r.side ? `<br><small class="subtle-text">Your side: ${esc(r.side)}</small>` : ""}</button>`,
      )
      .join("")}${kind === "tribute" ? `<p class="subtle-text">Your street: ${esc(table.tributeBasis(v, query, f))}</p>` : ""}${distrust}<div class="card-actions">${btn("Back", "diplomacy", id, "subtle")}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
}
// dealWords is a deal in words, who pays whom said (game.Deal.String
// and World.Describe, #537): a tribute is paid by you, a homage to you,
// and a split names the corners it leaves your side of the line.
function dealWords(kind, t = {}) {
  switch (kind) {
    case "truce":
      return `a ${t.days}-day truce`;
    case "tribute":
      return `tribute: you pay them ${money(t.per_day)} a day`;
    case "homage":
      return `homage: they pay you ${money(t.per_day)} a day`;
    case "split": {
      const corners = v.cities.flatMap((c) => c.corners),
        names = (t.corners || []).map((id) => corners.find((c) => c.id === id)?.name || id);
      return `a split: ${plural(names.length, "corner")} your side of the line (${names.join(", ")})`;
    }
    case "shipment":
      return `a joint shipment of ${(t.units || 0).toLocaleString("en-US")} units`;
  }
  return kind;
}
function renderLedger() {
  const off = query("rules.laundering.offshore");
  return `<div class="cards"><article class="card"><div class="eyebrow">WHAT YOU’VE BUILT</div><h3>Total net worth</h3><div class="cash-total">${money(v.you.net_worth)}</div><p>Cash, offshore funds, inventory and property. Not all of it is spendable.</p><div class="row"><span>Business income, net of upkeep</span><b>${money(query("rules.laundering.legit_income"))}/day</b></div></article><article class="card"><div class="eyebrow">A FUTURE SOMEWHERE ELSE</div><h3>The offshore account</h3><div class="cash-total">${money(v.you.offshore)}</div><p>Transfers cost ${Math.round(off.Fee * 100)}%. Moving more than ${money(off.Lot)} in a day adds evidence.</p><label class="row"><input id="reserve-amount" aria-label="Amount to transfer" type="number" min="1" step="100" value="${wash.reserveBlank(v, query) || ""}">${btn("Transfer", "reserve", "", "small", !v.you.clean_cash || !!v.over)}</label>${upkeepHTML()}<p id="sweep"><b>Sweep</b> · <span class="${v.you.sweep_on ? "good-text" : "subtle-text"}">${esc(property.sweepState(v, query))}</span> ${btn("Sweep offshore", "sweep-open", "", "small subtle", !!v.over)}</p></article><article class="card"><div class="eyebrow">CASH FOR THE STREET</div><h3>Cash out</h3><div class="cash-total">${money(v.you.clean_cash)}</div><p>Stock and wages are paid in dirty cash. Drawing clean money back costs ${money(query("rules.laundering.cash_out_fee", 100000))} per $100,000, and a dirty pile past your cover draws heat.</p><label class="row"><input id="cashout-amount" aria-label="Clean cash to cash out" type="number" min="1" step="100" value="${wash.cashOutBlank(v, query) || ""}">${btn("Cash out", "cash-out", "", "small", !v.you.clean_cash || !!v.over)}</label>${upkeepHTML()}</article></div><div class="tip-box">Your final score is offshore money divided by one plus the run’s body count: ${esc(endings.scoreRow(v))} today. A large empire and a high score are different goals.</div>${lawSectionHTML()}${ambitionsHTML()}${exitsHTML()}`;
}
function reportHTML(r) {
  if (!r.sections)
    return Object.entries(r)
      .filter(([k, x]) => Array.isArray(x) && x.length)
      .map(
        ([k, x]) =>
          `<div class="report-section"><div class="eyebrow">${esc(k.toUpperCase())}</div>${x.map((t) => `<p>${esc(t)}</p>`).join("")}</div>`,
      )
      .join("");
  return `${r.lead?.length ? `<div class="tip-box"><div class="eyebrow">TODAY’S BIGGEST CHANGES</div>${r.lead.map((l) => `<p>${esc(l.text)}</p>`).join("")}</div>` : ""}${r.sections
    .filter((sec) => sec.id !== "money" && sec.lines.length)
    .map(
      (sec) =>
        `<div class="report-section"><div class="eyebrow">${esc(sec.title)}</div>${sec.lines.map((t) => `<p>${esc(t)}</p>`).join("")}</div>`,
    )
    .join("")}${r.day ? flowHTML(r.flow) + moneyHTML(r) : ""}`;
}
// moneyHTML is the rest of the report's MONEY section, under the flow
// table as the TUI draws it (ui/report.go moneyLines, #351, #522): the
// itemised lines naming each cause (a cut, bail, a skim, a seizure, the
// cash-out's fee), then the cash before and after.
function moneyHTML(r) {
  const lines = r.sections.find((sec) => sec.id === "money")?.lines || [];
  return `<div class="report-section"><div class="eyebrow">MONEY</div>${lines.map((t) => `<p>${esc(t)}</p>`).join("")}<p>Cash ${money(r.cash_before)} → ${money(r.cash_after)}</p></div>`;
}
function renderJournal() {
  // The report's PLAN section (ui/ambitions.go planReport): the pinned
  // plan's steps and its next one in words.
  const line = endings.planLine(v, quietBroke),
    plan = line ? `<div class="report-section"><div class="eyebrow">PLAN</div><p>${esc(line)}</p></div>` : "";
  return `<div class="paper"><div class="row"><h2>The morning edition</h2><span class="tag">Day ${v.report.day}</span></div>${plan}${reportHTML(v.report) || '<div class="empty">A blank page. Your first report arrives tomorrow.</div>'}</div>${
    history.length > 1
      ? `<h3 class="section-gap">Earlier editions</h3><div class="paper">${history
          .slice(1)
          .map(
            (h) =>
              `<details class="report-section"><summary>Day ${h.day}</summary>${reportHTML(h.report)}</details>`,
          )
          .join("")}</div>`
      : ""
  }`;
}
function cornerModal(id) {
  const c = v.cities.flatMap((x) => x.corners).find((x) => x.id === id),
    owner =
      c.owner === "rival"
        ? v.factions.find((f) => f.id === (c.faction || "rival"))?.leader
        : c.owner === "player"
          ? "Your operation"
          : "Nobody yet",
    postable = [
      { id: -1, name: "Work it yourself", role: "runner" },
      ...v.crew.filter((m) => ["runner", "enforcer"].includes(m.role)),
    ];
  modal(
    `<div class="eyebrow">ON THE CORNER</div><h2>${esc(c.name)}</h2><p>Held by ${esc(owner)} · Demand multiplier ${c.demand.toFixed(1)}×</p>${strikeOrder()?.corner === c.id ? `<p class="warn-text">⚔ ${esc(strikeOrder().force)} tonight</p>` : ""}${c.owner !== "rival" ? `<label>Who should work here?<select id="post-member">${postable.map((m) => `<option value="${m.id}">${esc(m.name)} · ${m.role}</option>`).join("")}</select></label>${btn("Assign to this corner", "post", c.id, "primary")}${c.owner === "player" ? btn("Abandon corner", "abandon", c.id, "subtle") : ""}` : `<div class="tip-box">Contesting territory can increase heat, evidence, and retaliation. Check your crew before committing.</div><label>Force<select id="force-dial"><option>warn</option><option>push</option><option>hit</option></select></label>${btn("Send enforcers", "strike", c.id, "primary", !v.crew.some((m) => m.role === "enforcer"))}${btn("Tip the police", "tip", c.id, "subtle")}<label>Price competition<select id="undercut-dial"><option>quiet</option><option>normal</option><option>aggressive</option></select></label>${btn("Undercut tonight", "undercut", c.id, "subtle")}`}`,
  );
}
// ending is the run summary (ui/summary.go, #465, #498, #518): the
// ending's title and the tier reached, the epilogue, the story, the
// score large with how it was reached, then the money, the people and
// the city.
function ending() {
  const o = v.over;
  modal(
    `<div class="ending"><div class="crown">${o.won ? "♛" : "✦"}</div><div class="eyebrow">${o.won ? "AN ENDING EARNED" : "EVERY CITY HAS ITS CONSEQUENCES"}</div><h2>${esc(o.title || o.cause)}</h2><p>Day ${o.day} · reached ${esc(endings.reachedLine(v))}</p>${o.epilogue ? `<p>${esc(o.epilogue)}</p>` : ""}<div class="cash-total" id="ending-score">${money(v.you.score)}</div><p>Score · ${esc(endings.scoreWords(v))} · ${plural(o.day, "day")}</p><div class="card-actions" style="justify-content:center">${btn("Export this story", "export", "", "primary")}${btn("Start another story", "new", "", "subtle")}</div></div>${
      o.story.length ? `<div class="report-section"><div class="eyebrow">THE STORY</div>${o.story.map((h) => `<p class="law-row"><small>DAY ${h.day}</small> ${esc(h.text)}</p>`).join("")}</div>` : ""
    }${endings
      .summarySections(v)
      .map((sec) => `<div class="report-section"><div class="eyebrow">${esc(sec.title)}</div>${rowsHTML(sec.rows)}</div>`)
      .join("")}`,
  );
}
function dilemma() {
  if (!v.card) return;
  modal(
    `<div class="eyebrow">A KNOCK AT THE DOOR</div><h2>${esc(v.card.title)}</h2><p>${esc(v.card.text)}</p>${v.card.choices.map((s, i) => `<button class="choice" data-action="choose" data-id="${i}">${i + 1}. ${esc(s.label)}<span class="choice-chips">${s.preview.map((c) => `<span class="chip ${esc(c.tone)}">${esc(c.text)}</span>`).join("")}</span></button>`).join("")}`,
  );
}
function saveMenu() {
  modal(
    `<div class="eyebrow">YOUR STORY, YOUR DEVICE</div><h2>Keep a page for later.</h2><p>The game saves after every action. Export a portable save to move your story to another device or keep a checkpoint.</p><div class="card-actions">${btn("Export save", "export", "", "primary")}${btn("Import save", "import", "", "subtle")}${btn("New story", "new", "", "subtle")}</div><p class="subtle-text">Original .gob game saves are supported. Imported saves replace the current browser session after confirmation.</p>`,
  );
}
function routes() {
  modal(
    `<div class="eyebrow">ACROSS THE WATER</div><h2>Transport & supply</h2>${v.routes
      .map(
        (r) =>
          `<div class="report-section" id="route-${esc(r.id)}"><div class="row"><h3>${esc(r.name)}</h3><span class="tag">${esc(r.mode)}</span></div>${routeIdleHTML(r)}${paneRowsHTML(roads.facts(v, query, r))}${checkpointHTML(r)}<div class="card-actions"><select id="route-${r.id}" aria-label="The dial">${["off", "slow", "normal", "fast"].map((x) => `<option ${x === r.dial ? "selected" : ""}>${x}</option>`).join("")}</select>${btn("Set pace", "route", r.id, "small", !!v.over)}${btn("Set a target", "route-targets", r.id, "small", !!v.over)}${btn("Driver", "route-driver", r.id, "small subtle", !!v.over)}</div></div>`,
      )
      .join("")}`,
  );
}
// routeIdleHTML is why a route on its dial sends nothing (#459, #537).
function routeIdleHTML(r) {
  const idle = wash.routeIdle(v, query, r);
  return idle ? `<p class="${tone(idle.tone)}">${esc(idle.text)}</p>` : "";
}
function properties() {
  const houses = query("house_offers"),
    assets = query("asset_offers");
  modal(
    `<div class="eyebrow">PROPERTY LEDGER</div><h2>A place of your own.</h2>${ownedHousesHTML()}${ownedAssetsHTML()}<h3>Houses</h3>${houses
      .map(
        (h) =>
          `<div class="report-section"><b>${esc(h.Name)}</b><p>${esc(h.City)} · ${h.Capacity} capacity · ${money(h.Price)} upfront</p>${btn(
            v.houses.some((x) => x.id === h.ID) ? "Owned" : "Lease",
            "house",
            h.ID,
            "small",
            v.houses.some((x) => x.id === h.ID),
          )}</div>`,
      )
      .join(
        "",
      )}<h3>Assets</h3>${assets.filter((a) => !(v.assets || []).some((x) => x.id === a.ID)).map((a) => `<div class="report-section"><b>${esc(a.Name)}</b><p>${money(a.Cost)} clean · ${esc(a.City)}<br><small class="subtle-text">${esc(wash.assetTerms(a))}</small><br><small class="subtle-text">${esc(property.assetBlurb(v, a, engineInfo))}</small></p>${btn("Purchase", "asset", a.ID, "small")}</div>`).join("")}`,
  );
}
function integer(id) {
  const x = Number($(id)?.value);
  if (!Number.isSafeInteger(x) || x <= 0)
    throw Error("Enter a positive whole number.");
  return x;
}
function exportFile() {
  const base64 = session.exportSave(),
    bytes = Uint8Array.from(atob(base64), (c) => c.charCodeAt(0)),
    url = URL.createObjectURL(
      new Blob([bytes], { type: "application/octet-stream" }),
    ),
    a = document.createElement("a");
  a.href = url;
  a.download = `kingpin-street-seed${v.seed}-day${v.day}.gob`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
  notify("Save exported");
}
async function action(a, id) {
  if (alignedAction(a, id) || labAction(a, id) || lawAction(a, id) || routineAction(a, id) || propertyAction(a, id) || supplyAction(a, id)) return;
  try {
    switch (a) {
      case "close":
        $("#sheet").close();
        break;
      case "confirm": {
        const f = confirmAction;
        confirmAction = null;
        $("#sheet").close();
        f?.();
        break;
      }
      case "view-city":
        selectedCity = id;
        render();
        break;
      case "travel": {
        const left = routine.contractsLeft(v, v.you.city),
          go = () => {
            act("travel", [id], "Arrived in " + cityName(id));
            selectedCity = id;
            render();
          };
        if (left.length) confirm(`Leave ${cityName(v.you.city)} for ${cityName(id)}?`, left.map((t) => [t, "warn-text"]), go);
        else go();
        break;
      }
      case "corner":
        cornerModal(id);
        break;
      case "post":
        act("post", [id, Number($("#post-member").value)], "Corner assigned");
        break;
      case "abandon":
        confirm(
          "Leave this corner?",
          "Your crew will give up this ground.",
          () => act("abandon", [id], "Corner abandoned"),
        );
        break;
      case "strike": {
        // The memo keeps the corner and the force for the street's
        // tonight line (#574): the view does not carry the strike.
        const force = $("#force-dial").value,
          c = v.cities.flatMap((x) => x.corners).find((x) => x.id === id);
        act("send_enforcers", [id, force], "Action queued", {
          order: { key: "strike", text: `Enforcers sent to ${c ? c.name : id}`, corner: id, force },
        });
        break;
      }
      case "tip": {
        // The tip confirm (#552, ui/books.go tipConfirm): the police's
        // attention, when they act, the trust and the page's odds.
        const c = v.cities.flatMap((x) => x.corners).find((x) => x.id === id),
          f = v.factions.find((x) => x.id === (c.faction || "rival"));
        confirm("Tip the police?", law.tipLines(v, query, c).map(([t, k]) => [t, tone(k)]), () => {
          if (act("tip", [id], null, { order: { key: "tip", text: "Police tip: " + c.name } }) !== null) notify(law.tipSaid(v, query, c, f));
        });
        break;
      }
      case "undercut":
        act(
          "undercut",
          [id, $("#undercut-dial").value],
          "Price competition queued",
          { order: { key: "cut-" + id, text: "Undercut " + id } },
        );
        break;
      case "buy":
        buyDialog(id);
        break;
      case "sell": {
        // Sized for what lands tonight (#503): that comes after the
        // sales, so only a standing order counts on it.
        const here = city().id,
          n = integer("#qty-" + id),
          most = routine.sellable(v, query, here, id),
          land = routine.landing(v, here, id),
          dial = $("#sale-dial").value;
        if (n > most && land > 0) throw Error(`Only ${most} sell tonight: the ${land} landing come after the sales; a standing order (Routine) counts them from tomorrow night.`);
        act("place_sell", [here, id, n, dial], `Queued ${n} ${routine.productName(v, id)} in ${city().name}, ${dial}. It sells at the end of the day.`);
        break;
      }
      case "sell-all": {
        const dial = $("#sale-dial").value;
        for (const p of city().products) {
          const n = routine.stock(v, city().id, p.id);
          if (n) act("place_sell", [city().id, p.id, n, dial], null);
        }
        notify("Held stock queued for sale");
        break;
      }
      case "hire":
        act("hire", [Number(id)], "Welcome to the crew");
        break;
      case "bonus": {
        const m = v.crew.find((x) => x.id === Number(id));
        confirm(`Pay off ${m.name}?`, crew.payOffLines(m, query("rules.crew.payoff_cost", m.id), query("rules.crew.payoff_loyalty")), () => {
          const got = act("pay_off", [m.id], null);
          if (got) notify(`${got.Name} pocketed it. Loyalty ${Math.round(got.Loyalty)}.`);
        });
        break;
      }
      case "bail": {
        const m = v.crew.find((x) => x.id === Number(id)),
          cost = query("rules.crew.bail_cost", m.id),
          lines = crew.bailLines(v, m, cost, query("rules.crew.life").BailLoyalty);
        confirm(`Bail ${m.name}?`, lines.map((l, i) => (i === 2 ? [l, "danger-text"] : l)), () => act("bail", [m.id], `Bail is down for ${m.name}: they walk tomorrow, and they know who paid.`));
        break;
      }
      case "investigate": {
        const odds = query("rules.crew.investigate_odds"),
          lines = crew.investigateLines(v, query("rules.crew.investigate_cost"), odds, engineInfo.investigateLoyalty);
        confirm("Investigate?", lines.map((l, i) => (i === 3 ? [l, "danger-text"] : l)), () => act("investigate", [], `Questions get asked tonight. Odds of a name ~${Math.round(odds * 100)}%.`));
        break;
      }
      case "assign-person": {
        const m = v.crew.find((m) => m.id === Number(id));
        modal(
          `<h2>A corner for ${esc(m.name)}</h2><label>Available ground<select id="assign-corner">${city()
            .corners.filter((c) => c.owner !== "rival")
            .map((c) => `<option value="${c.id}">${esc(c.name)}</option>`)
            .join(
              "",
            )}</select></label>${btn("Assign", "assign-submit", id, "primary")}`,
        );
        break;
      }
      case "assign-submit":
        act(
          "post",
          [$("#assign-corner").value, Number(id)],
          "Assignment updated",
        );
        break;
      case "crew-detail":
        memberSheet(id);
        break;
      case "fire": {
        // The fire confirm (#551, ui fireConfirm): what it costs the
        // rest, who it puts at the walk line, and the war it can lose.
        const m = v.crew.find((x) => x.id === Number(id)),
          tun = query("rules.crew.tuning"),
          walk = crew.fireWalkLine(v, m, tun),
          war = crew.fireWarLine(v, m, engineInfo.takenOutMuscle, query("rules.rivals.tuning").WarThreshold);
        confirm(`Fire ${m.name}?`, [crew.fireCost(m, tun), ...[walk, war].filter(Boolean).map((l) => [l, "danger-text"])], () => {
          if (!act("fire", [m.id], null)) return;
          const n = v.crew.length,
            most = query("rules.crew.max_crew");
          notify(`${m.name} is gone. The rest noticed.${n > most ? ` The roster is ${n} of ${most}: nobody is let go, and nobody is hired until it is under ${most}.` : ""}`);
        });
        break;
      }
      case "buy-front": {
        // Bought into a shut (#496): only a confirm buys a front the
        // night is expected to shut.
        const o = (query("front_offers") || []).find((x) => x.ID === id),
          shut = o ? wash.frontShutWarning(v, query, o, session.preview()) : "",
          go = () => act("buy_front", [id], o ? `Bought ${o.Name} for ${money(o.Cost)}. It opens tomorrow, washing up to ${money(query("rules.laundering.throughput", id))} a day.` : "Business purchased");
        if (shut) confirm(`Buy ${o.Name}?`, [[shut, "danger-text"]], go);
        else go();
        break;
      }
      case "invest":
        act("invest", [id, 1], "Business expanded");
        break;
      case "upgrade":
        act("buy_upgrade", [id], "Upgrade acquired");
        break;
      case "properties":
        properties();
        break;
      case "house":
        act("buy_house", [id], "House leased");
        break;
      case "asset":
        act("buy_asset", [id], "Asset purchased");
        break;
      case "scout":
        act("scout_faction", [id], "Scouting queued", {
          order: { key: "scout", text: "Scout " + id },
        });
        break;
      case "diplomacy": {
        const why = table.askProposeRefusal(v.factions.find((x) => x.id === id));
        if (why) throw Error(why);
        proposeDialog(id);
        break;
      }
      case "propose-kind": {
        const [fid, kind] = id.split("|"),
          why = table.proposeRefusal(v, v.factions.find((x) => x.id === fid), kind);
        if (why) throw Error(why);
        proposeDialog(fid, kind);
        break;
      }
      case "propose": {
        const [fid, kind, i] = id.split("|"),
          f = v.factions.find((x) => x.id === fid),
          d = table.termRows(v, query, f, kind)[Number(i)].deal,
          before = v.proposal ? { ...v.proposal } : null,
          send = () => {
            if (act("propose_to", [fid, d.kind, d.terms], null, { order: { key: "proposal", text: d.kind + " proposal" } }) !== null) notify(table.proposedSaid(v, query, f, d, before));
          };
        // One proposal a night (#506), and replacing it asks first (#536).
        if (before) confirm("Replace tonight's proposal?", table.replaceLines(v, f, d), send);
        else send();
        break;
      }
      case "withdraw":
        if (act("withdraw", [], "Proposal withdrawn.") !== null) orders = orders.filter((o) => o.key !== "proposal");
        break;
      case "declare-war": {
        const f = v.factions.find((x) => x.id === id),
          why = table.warRefusal(v, f);
        if (why) throw Error(why);
        confirm(`War on ${table.rivalName(f)}?`, table.warConfirm(v, query, engineInfo, f).map(([t, k]) => [t, tone(k)]), () => {
          if (act("declare_war", [id], null) !== null) notify(table.warSaid(f));
        });
        break;
      }
      case "call-off-war": {
        const f = v.factions.find((x) => x.id === v.you.war);
        if (!f) throw Error("There is no war on.");
        confirm(`Call off the war on ${table.rivalName(f)}?`, table.callOffLines(f), () => {
          if (act("call_off_war", [], null) !== null) notify(table.calledOffSaid(f));
        });
        break;
      }
      case "accept":
        act("accept", [Number(id)], "Offer accepted");
        break;
      case "decline":
        act("decline", [Number(id)], "Offer declined");
        break;
      case "cash-out": {
        const n = integer("#cashout-amount"),
          warn = wash.upkeepWarning(query, v.you.clean_cash - n, v.you.dirty_cash + n - query("rules.laundering.cash_out_fee", n)),
          go = () => act("cash_out", [n], `Cashed out ${money(n)} clean, less the banker's fee`);
        if (warn) confirm(`Cash out ${money(n)}?`, [[warn.text, tone(warn.tone)]], go);
        else go();
        break;
      }
      case "set-export": {
        const units = nonnegative(`#lane-units-${CSS.escape(id)}`),
          product = $(`#lane-product-${CSS.escape(id)}`).value;
        act("set_export", [id, product, units], units ? "Lane order set: it loads tonight" : "Lane turned off", { close: false });
        break;
      }
      case "buy-trophy": {
        const o = (query("trophy_offers") || []).find((x) => x.ID === id);
        if (o)
          confirm(`Buy ${o.Name}?`, `${money(o.Cost)} of clean cash. The city will notice, and so may the task force.`, () =>
            act("buy_trophy", [id], `${o.Name} is yours`),
          );
        break;
      }
      case "reserve": {
        if (!$("#reserve-amount").value.trim() && !wash.reserveBlank(v, query))
          throw Error(`The account would take the ${money(wash.upkeepTonight(query))} clean kept back for tonight's upkeep, and that is all of it: type an amount to move it anyway.`);
        const n = integer("#reserve-amount"),
          warn = wash.upkeepWarning(query, v.you.clean_cash - n, v.you.dirty_cash),
          go = () =>
            act("reserve", [n], "Transfer queued", {
              order: { key: "reserve", text: "Offshore transfer" },
            });
        if (warn) confirm(`Send ${money(n)} offshore?`, [[warn.text, tone(warn.tone)]], go);
        else go();
        break;
      }
      case "set-till": {
        const r = wash.setTill(query, $("#till-amount").value);
        if (r.field !== undefined) $("#till-amount").value = r.field;
        if (r.refuse) notify(r.refuse, true);
        else act("set_till", [r.send], r.say, { close: false });
        break;
      }
      case "retire":
      case "crown":
      case "go_straight":
      case "vanish":
        confirm(
          { retire: "Retire?", crown: "Take the crown?", go_straight: "Go straight?", vanish: "Vanish?" }[a],
          endings.exitConfirm(v, query, a).map(([t, k]) => [t, tone(k)]),
          () => act(a, [], "Your story is complete"),
        );
        break;
      case "ending":
        ending();
        break;
      case "dilemma":
        dilemma();
        break;
      case "choose": {
        // The outcome in the card's place, as the TUI's WHAT HAPPENED
        // (ui/card.go): the effects have landed, the words stay up
        // until the player moves on.
        const a = act("choose", [Number(id)], "", { close: false });
        if (a && !v.over)
          modal(
            `<div class="eyebrow">WHAT HAPPENED</div><h2>${esc(a.Title)}</h2>${String(a.Outcome || "")
              .split("\n")
              .filter(Boolean)
              .map((t) => `<p>${esc(t)}</p>`)
              .join("")}<div class="card-actions">${btn("Carry on", "close", "", "primary")}</div>`,
          );
        break;
      }
      case "export":
        exportFile();
        break;
      case "import":
        $("#import-file").click();
        break;
      case "new":
        modal(
          `<div class="eyebrow">A BLANK PAGE</div><h2>A new name in town.</h2><p>Export your current story first if you want to keep it.</p><label>World seed<input id="new-seed" type="number" min="1" max="2147483647" value="${Math.floor(Math.random() * 99999) + 1}"></label><h3>Who you start as</h3>${query("characters")
            .map(
              (ch) =>
                `<label class="choice"><span><input type="radio" name="new-character" value="${esc(ch.id)}" ${ch.default ? "checked" : ""}> <b>${esc(ch.name)}</b></span><br><small class="subtle-text">${esc(ch.blurb)}</small></label>`,
            )
            .join("")}<label><span><input id="hard-da" type="checkbox"> Hard district attorney</span></label>${btn("Begin a new story", "new-submit", "", "primary")}`,
        );
        break;
      case "new-submit": {
        const seed = integer("#new-seed"),
          character = $('input[name="new-character"]:checked')?.value || "",
          hard = $("#hard-da").checked;
        confirm(
          "Replace your current story?",
          "This replaces the automatic save on this device. Export first if you want to keep it.",
          () => {
            session.newRun(seed, character, hard);
            v = session.refresh();
            session.take();
            orders = [];
            history = [];
            selectedCity = null;
            tab = "street";
            persist();
            render();
            notify("A new story begins");
          },
        );
        break;
      }
      case "contract":
        act("accept_contract", [Number(id)], "Contract accepted");
        break;
      case "deliver": {
        const c = v.contracts.find((x) => x.id === Number(id));
        modal(
          `<h2>Deliver to ${esc(c.name)}</h2><p>${c.units - c.delivered} units still due.</p><label>Units<input id="delivery-units" type="number" min="1" value="${Math.max(1, c.units - c.delivered)}"></label>${btn("Queue delivery", "deliver-submit", id, "primary")}`,
        );
        break;
      }
      case "deliver-submit":
        act(
          "deliver",
          [Number(id), integer("#delivery-units")],
          "Delivery queued",
          { order: { key: "delivery-" + id, text: "Private buyer delivery", contract: Number(id), units: integer("#delivery-units") } },
        );
        break;
      case "routes":
        routes();
        break;
      case "route":
        act("set_route", [id, $("#route-" + id).value], "Route pace changed");
        routes();
        break;
    }
  } catch (e) {
    notify(e.message, true);
  }
}
document.addEventListener("click", (e) => {
  const t = e.target.closest("[data-tab],[data-action],[data-branch]");
  if (!t) return;
  if (t.dataset.tab) {
    tab = t.dataset.tab;
    render();
  } else if (t.dataset.branch) {
    branch = t.dataset.branch;
    render();
  } else action(t.dataset.action, t.dataset.id);
});
document.addEventListener("change", (e) => {
  if (e.target.dataset.change === "sale-dial") saleDial = e.target.value;
  if (e.target.dataset.change === "pay")
    act("set_pay", [e.target.value], `Pay ${e.target.value}, ${money(query("rules.crew.wages", e.target.value))}/day. ${crew.payBlurb(e.target.value)}`);
  if (e.target.dataset.change === "launder")
    act("set_launder_dial", [e.target.value], "Laundering policy updated");
});
$("#sheet .close").onclick = () => $("#sheet").close();
$("#sheet").addEventListener("click", (e) => {
  if (e.target === $("#sheet")) {
    const r = e.target.getBoundingClientRect();
    if (
      e.clientX < r.left ||
      e.clientX > r.right ||
      e.clientY < r.top ||
      e.clientY > r.bottom
    )
      e.target.close();
  }
});
$("#end-day").onclick = () => (v.card ? dilemma() : previewTonight());
$("#lie-low").onclick = () => action("lie-low");
$("#save-menu").onclick = () => v && saveMenu();
$("#help").onclick = () =>
  modal(
    `<div class="eyebrow">WELCOME TO THE NEIGHBORHOOD</div><h2>Start small. Think ahead.</h2><h3>1. Stock the shelf</h3><p>Open Market. Buy a few units with dirty cash. Keep enough money for wages.</p><h3>2. Plan your night</h3><p>Queue sales, hire crew and assign corners. Most orders resolve when you end the day. Quiet selling draws less attention.</p><h3>3. Watch the file</h3><p>Heat and evidence are different threats. The risk panel always shows your current indictment threshold. A quiet day pauses street sales, but it does not erase existing evidence.</p><h3>4. Choose your ambition</h3><p>Grow businesses, control the city, or move money offshore and leave. The Ledger explains the endings.</p><div class="tip-box">Automatic saves stay on this device. Your story → Export save creates a portable checkpoint.</div>`,
  );
$("#sound").onclick = () => {
  sound = !sound;
  $("#sound").textContent = sound ? "Sound on" : "Sound off";
  chime();
};
$("#import-file").onchange = async (e) => {
  const f = e.target.files[0];
  if (!f) return;
  try {
    const bytes = new Uint8Array(await f.arrayBuffer());
    let b64;
    if (f.name.endsWith(".json")) {
      const parsed = JSON.parse(new TextDecoder().decode(bytes));
      b64 = parsed.save || parsed;
    } else {
      let binary = "";
      for (let i = 0; i < bytes.length; i += 8192)
        binary += String.fromCharCode(...bytes.subarray(i, i + 8192));
      b64 = btoa(binary);
    }
    confirm(
      "Continue this saved story?",
      `Import ${f.name} and replace the current browser save?`,
      () => {
        try {
          session.importSave(b64);
          v = session.refresh();
          session.take();
          orders = [];
          history = [];
          selectedCity = null;
          persist();
          render();
          notify("Story restored");
          if (v.over) ending();
        } catch (err) {
          notify(err.message, true);
        }
      },
    );
  } catch (err) {
    notify("Could not read save: " + err.message, true);
  }
  e.target.value = "";
};
async function boot() {
  try {
    const go = new Go();
    const bytes = window.KINGPIN_WASM_BASE64
      ? Uint8Array.from(atob(window.KINGPIN_WASM_BASE64), (c) =>
          c.charCodeAt(0),
        )
      : new Uint8Array(
          await (
            await fetch("assets/kingpin.wasm?v=" + engineInfo.commit)
          ).arrayBuffer(),
        );
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance).catch((e) => notify("Engine stopped: " + e.message, true));
    if (!window.kingpin) throw Error("The game engine did not initialize");
    session = new Session(window.kingpin);
    let loaded = false;
    try {
      const saved = JSON.parse(
        localStorage.getItem(SAVE) || localStorage.getItem(OLD_SAVE) || "null",
      );
      if (saved?.save) {
        session.importSave(saved.save);
        // A sale was the page's memo before #556; the cart is the
        // view's now, so an old save's sales are dropped from it.
        orders = (saved.orders || []).filter((o) => !o.product);
        history = saved.history || [];
        loaded = true;
      }
    } catch (e) {
      throw new Error(
        "Your saved story could not be loaded; it has been kept unchanged. " +
          e.message,
      );
    }
    if (!loaded) session.newRun(41);
    v = session.refresh();
    session.take();
    if (new URLSearchParams(location.search).has("test"))
      window.__ink = {
        get view() {
          return v;
        },
        get session() {
          return session;
        },
        act,
        render,
      };
    $("#loading").hidden = true;
    $("#app").hidden = false;
    persist();
    render();
    if (v.over) ending();
  } catch (e) {
    $("#loading").innerHTML =
      `<div class="loading-error"><h2>Could not open the city.</h2><p>${esc(e.message)}</p><p>Reload the page to retry. If this persists, keep your saved story and report this message.</p></div>`;
  }
}
boot();

// Optional browser agent interface; ordinary browsers require no polyfill.
if (document.modelContext?.registerTool) {
  const lifecycle = new AbortController();
  window.addEventListener("pagehide", () => lifecycle.abort(), { once: true });
  const validate = (input, keys) => {
    if (
      !input ||
      typeof input !== "object" ||
      Array.isArray(input) ||
      Object.keys(input).some((k) => !keys.includes(k))
    )
      throw new Error("Invalid tool input");
    if (!v) throw new Error("Game is still loading");
  };
  const tools = [
    {
      name: "read_kingpin_game",
      title: "Read the current game",
      description:
        "Read the current day, player finances, city, queued orders and game ending status without changing the game.",
      inputSchema: {
        type: "object",
        properties: {},
        additionalProperties: false,
      },
      annotations: { readOnlyHint: true, untrustedContentHint: false },
      execute(input) {
        validate(input, []);
        return JSON.parse(
          JSON.stringify({
            day: v.day,
            player: v.you,
            city: city(),
            orders,
            over: v.over,
          }),
        );
      },
    },
    {
      name: "navigate_kingpin_section",
      title: "Open a game section",
      description:
        "Open a visible game section. Does not advance time or place any game orders.",
      inputSchema: {
        type: "object",
        properties: {
          section: { type: "string", enum: tabs.map((t) => t[0]) },
        },
        required: ["section"],
        additionalProperties: false,
      },
      annotations: { readOnlyHint: false, untrustedContentHint: false },
      execute(input) {
        validate(input, ["section"]);
        if (!tabs.some((t) => t[0] === input.section))
          throw new Error("Unknown game section");
        tab = input.section;
        render();
        return { section: tab, day: v.day };
      },
    },
  ];
  for (const tool of tools) {
    try {
      Promise.resolve(
        document.modelContext.registerTool(tool, { signal: lifecycle.signal }),
      ).catch(() => {});
    } catch {}
  }
}

function flowHTML(f) {
  if (!f) return "";
  const row = (label, d, c, cls = "") =>
    `<tr class="${cls}"><th>${esc(label)}</th><td>${money(d)}</td><td>${money(c)}</td></tr>`;
  return `<h3 class="section-gap">Where the money went</h3><div class="table-wrap"><table class="data-table flow-table"><thead><tr><th>CASH FLOW</th><th>DIRTY</th><th>CLEAN</th></tr></thead><tbody>${row("Opening", f.opening.dirty, f.opening.clean)}${(f.lines || []).map((l) => row(l.label, l.dirty, l.clean, l.big ? "flow-big" : "")).join("")}${row("Closing", f.closing.dirty, f.closing.clean, "flow-big")}</tbody></table></div><p>Net cash change <b>${money(f.net)}</b></p>`;
}
// washHTML is the preview's wash (#549, p.wash): what the fronts take
// in and pay out tonight, and a front whose upkeep the clean will not
// cover, which shuts (#524), as a danger.
function washHTML(w) {
  if (!w || !(w.washed || w.upkeep || w.income || w.assets)) return "";
  const shut = w.shuts?.length
    ? `<p class="danger-text"><b>${esc(w.shuts.join(", "))} ${w.shuts.length === 1 ? "shuts" : "shut"} tonight:</b> upkeep ${money(w.short)} clean short.</p>`
    : "";
  return `<p>The wash: ~${money(w.washed)} through the fronts · ${money(w.income)} income · ${money(w.upkeep)} upkeep${w.assets ? ` · ${money(w.assets)} the assets' upkeep` : ""}</p>${shut}`;
}
const cityName = (id) => v.cities.find((c) => c.id === id)?.name || id,
  cornerName = (id) => v.cities.flatMap((c) => c.corners).find((k) => k.id === id)?.name || id;
function previewTonight() {
  if (v.over) return;
  if (v.card) {
    dilemma();
    return;
  }
  const p = session.preview();
  modal(
    `<div class="eyebrow">BEFORE THE CITY SLEEPS</div><h2>Tonight, estimated.</h2><p>Day ${v.day} → ${p.day}${p.lie_low ? " · Lying low; no street sales" : ""}</p>${(p.alerts || []).map(alertButton).join("")}${flowHTML(p.flow)}${(p.sales || []).map((s) => `<p>${esc(cityName(s.city))}: ~${plural(s.units, "unit")} sold${s.delivered ? `, ${s.delivered} handed over` : ""} · ~${money(s.take)} take · ${s.heat >= 0 ? "+" : ""}${s.heat.toFixed(1)} heat</p>`).join("")}${washHTML(p.wash)}${p.idle?.length ? `<div class="tip-box">Idle crew: ${p.idle.map((x) => esc(x.name)).join(", ")}</div>` : ""}${p.corners?.length ? `<p>Unworked corners: ${p.corners.map((x) => `${esc(cornerName(x.corner))} (${plural(x.days, "day")} until loss)`).join(", ")}</p>` : ""}<p class="subtle-text">${esc(unknownLine(p.unknown))}</p><div class="card-actions">${btn("End the day", "advance-day", "", "primary")}${btn("Keep planning", "close", "", "subtle")}</div>`,
  );
}
// ambitionsHTML is the plans (ui/ambitions.go): each with its bar, its
// steps in words by unit, the crews off the crown's count under its
// factions step, and the PLAN line over them with what reset the quiet.
function ambitionsHTML() {
  const line = endings.planLine(v, quietBroke);
  return `<h3 class="section-gap">Your ambitions</h3>${line ? `<p class="warn-text" id="plan-line">PLAN · ${esc(line)}</p>` : ""}<div class="cards">${v.ambitions
    .map(
      (a) =>
        `<article class="card"><div class="row"><h3>${esc(a.name)}</h3><span class="tag">${a.done ? endings.doneWord(a).toUpperCase() : Math.floor(a.progress * 100) + "%"}</span></div><div class="meter teal"><span style="width:${a.progress * 100}%"></span></div>${a.steps
          .map(
            (step) =>
              `<p class="plan-step ${step.done ? "met" : ""}">${step.done ? "✓" : "○"} ${esc(step.label)}<small>${esc(endings.stepWords(step))}</small></p>${
                a.id === "city" && step.id === "factions" && !step.done
                  ? endings
                      .downRows(v, query)
                      .map((r) => `<p class="subtle-text"><small>${esc(r.name)}: ${esc(r.words)}</small></p>`)
                      .join("")
                  : ""
              }`,
          )
          .join("")}${btn(a.pinned ? "Unpin plan" : "Follow this plan", "pin-plan", a.pinned ? "" : a.id, "small", !!v.over)}</article>`,
    )
    .join("")}</div>`;
}
// The export lanes (#391, view 12 since #405): every lane, shut on what
// it waits for, or open with what it carries tonight, its standing order
// and the loads out; an open lane takes an order (set_export: 0 units
// turns it off).
function lanesHTML() {
  const lanes = v.exports || [];
  if (!lanes.some((l) => l.open || l.out)) {
    return lanes.length
      ? `<h3 class="section-gap">Export lanes</h3><div class="tip-box">Ship by the ton abroad once you own ${esc(lanes[0].needs || "the book")}. What lands comes home dirty.</div>`
      : "";
  }
  const productName = (id) =>
    v.cities.flatMap((c) => c.products).find((p) => p.id === id)?.name || id;
  return `<h3 class="section-gap">Export lanes</h3><div class="cards">${lanes
    .map(
      (l) =>
        `<article class="card"><div class="card-top"><span class="tag ${l.open ? "" : "gold"}">${l.open ? esc(l.mode).toUpperCase() : "SHUT"}</span><small>${l.days} days out</small></div><h3>${esc(l.name)}</h3>${
          l.open
            ? `<p>Carries up to ${l.capacity.toLocaleString()} units a night.${l.product ? ` Ordered: ${l.units.toLocaleString()} ${esc(productName(l.product))} at ${money(l.price)} a unit abroad.` : " No order."}</p><p class="subtle-text">${l.out ? `${l.out} out · the next lands Day ${l.lands} for ${money(l.pays)}` : "Nothing out."}</p><div class="card-actions"><select id="lane-product-${esc(l.id)}" aria-label="Product for ${esc(l.name)}">${l.products.map((p) => `<option value="${esc(p)}" ${p === l.product ? "selected" : ""}>${esc(productName(p))}</option>`).join("")}</select><input id="lane-units-${esc(l.id)}" type="number" min="0" aria-label="Units a night" value="${l.units || l.capacity}">${btn("Set order", "set-export", l.id, "small", !!v.over)}</div>`
            : `<p>Opens with ${esc(l.needs)}.</p>`
        }</article>`,
    )
    .join("")}</div>`;
}
// The trophies (#392, view 12 since #405): the ones you own, and the
// offers, bought with clean cash once your peak clean cash reaches the
// line (the engine refuses one still locked, in its own words).
function trophiesHTML() {
  const offers = query("trophy_offers") || [],
    owned = v.trophies || [];
  if (!owned.length && !offers.some((o) => v.you.clean_cash >= o.Cost)) return "";
  return `<h3 class="section-gap">Trophies</h3>${owned.length ? `<p>${owned.map((t) => `${esc(t.name)} (Day ${t.bought}, ${money(t.cost)})`).join(" · ")}</p>` : ""}<div class="cards">${offers
    .map(
      (o) =>
        `<article class="card"><div class="card-top"><span class="tag gold">TROPHY</span><small>on offer at ${money(o.UnlockCash)} peak clean</small></div><h3>${esc(o.Name)}</h3><div class="row"><strong class="price">${money(o.Cost)}</strong>${btn("Buy", "buy-trophy", o.ID, "small", v.you.clean_cash < o.Cost || !!v.over)}</div></article>`,
    )
    .join("")}</div>`;
}
// exitsHTML is the walk away (ui/exit.go viewExit, #478, #494): what
// this morning would leave unsettled, then each way out with its terms,
// what is short and what it scores, the same for all four.
function exitsHTML() {
  const rows = endings.exitRows(v, query, engineInfo),
    labels = { retire: ["THE QUIET EXIT", "Retired Clean", "Retire now"], crown: ["THE CITY IS YOURS", "Kingpin", "Take the crown"], vanish: ["A NEW CHAPTER", "Vanished", "Vanish now"], go_straight: ["A DIFFERENT KIND OF EMPIRE", "A Businessman", "Go straight"] };
  return `<h3 class="section-gap" id="walk-away">Choose your ending</h3>${parasHTML(endings.pendingLines(v))}<div class="cards">${rows
    .map((r) => {
      const [tag, title, go] = labels[r.action];
      return `<article class="card" id="exit-${r.action}"><span class="tag ${r.action === "crown" ? "gold" : ""}">${tag}</span><h3>${title}</h3><p>${esc(r.terms[0].toUpperCase() + r.terms.slice(1))}.</p>${r.open ? "" : `<p class="warn-text">Short: ${esc(r.short)}.</p>`}<p class="law-row"><small>SCORES</small> <span class="warn-text">${esc(endings.scoreRow(v))}</span></p>${btn(r.open ? go : "Not yet", r.action, "", "small", !r.open || !!v.over)}</article>`;
    })
    .join("")}</div><p class="subtle-text">${esc(endings.walkAwayNote(v))}</p>`;
}
// tonightHTML is tonight's pile as the police will count it (#397): the
// landings and the wages come in before the count, the wash after it.
function tonightHTML() {
  const f = query("forecast");
  if (!f || v.over) return "";
  const past = f.heat > 0;
  return `<p class="${past ? "danger-text" : ""}">Tonight's count: ${money(f.pile)} dirty${f.loads ? ` (${money(f.landings)} landing)` : ""}${past ? ` · ${money(f.pile - f.line)} past your cover, +${Math.round(f.heat)} heat before the wash` : " · under your cover"}</p>`;
}
function marketTools() {
  return `<div class="paper operation-tools"><div><div class="eyebrow">PLAN YOUR OPERATION</div><label>Demand to cover <select id="restock-days"><option value="1">1 day</option><option value="2" selected>2 days</option><option value="3">3 days</option><option value="7">7 days</option></select></label>${btn("Review restock", "restock", "", "small", !!v.over)}</div><div><label>Routine <select id="preset-id">${session
    .presets()
    .map((p) => `<option value="${esc(p.id)}">${esc(p.name)}</option>`)
    .join(
      "",
    )}</select></label>${btn("Review preset", "preset-review", "", "small", !!v.over)}</div></div>`;
}
// openAlert goes where the alert is answered (landing.js, #549): the
// tab that shows what it names, that thing opened or picked out.
function openAlert(a) {
  if (!a) return;
  const l = landing(a);
  tab = l.tab;
  if (l.city) selectedCity = l.city;
  $("#sheet").close();
  render();
  if (tab === "market" && l.city && l.city !== v.you.city)
    notify("This alert concerns " + cityName(l.city) + ". Travel there to manage its market.");
  if (l.open === "corner" && v.cities.some((c) => c.corners.some((k) => k.id === l.id))) {
    cornerModal(l.id);
    if (a.member && $("#post-member")) $("#post-member").value = a.member;
  } else if (l.open === "member" && v.crew.some((m) => m.id === l.id)) action("crew-detail", l.id);
  else if (l.open === "properties") {
    properties();
    const h = l.id && document.getElementById("house-" + l.id);
    if (h) {
      h.classList.add("picked");
      h.scrollIntoView({ block: "center" });
    }
  }
  else if (l.open === "favour") action("favour");
  else if (l.open === "investigate" && v.crew.length && !v.over) action("investigate");
  const picked = l.select && document.getElementById(l.select);
  if (picked) {
    picked.classList.add("picked");
    setTimeout(() => picked.classList.remove("picked"), 2600);
    picked.scrollIntoView({ behavior: "smooth", block: "center" });
    if (l.focus) document.getElementById(l.focus)?.focus({ preventScroll: true });
  } else $("#section-heading").scrollIntoView({ behavior: "smooth", block: "start" });
}
// morning is the alarm on a morning that opens on a danger (#549, the
// TUI's morning, #519): the loudest one, the engine's first, in a red
// toast.
function morning() {
  const a = v.alerts[0];
  if (a && a.danger) notify(alertText(v, a), true);
}
// A lieutenant's line on their card (#455): the city they run or none
// yet, and their temper with what it does once it has shown.
function lieutenantStatus(m, lt) {
  const city = m.city ? v.cities.find((c) => c.id === m.city) : null;
  const tt = m.personality ? temperOf(lt, m.personality) : null;
  return `${city ? "Runs " + esc(city.name) : "No city yet"}<br>${tt ? `<span class="tag">${esc(m.personality)}</span> <small>${esc(temperLine(tt))}</small>` : `<small>Temper shows after ${lt.RevealDays} days running a city</small>`}${keepsHTML(m)}`;
}
// keepsHTML is a lieutenant's stock levels in their city (#576, ui
// lieutenantKeeps); "" with none.
function keepsHTML(m) {
  const k = m.role === "lieutenant" ? crew.keeps(v, m.city) : "";
  return k ? `<br><small>Keeps ${esc(k)}</small>` : "";
}
// lieutenantDialog is the city picker with the role in words (#455):
// what they do, the cut and the slots, the tempers and the risk.
function lieutenantDialog(id) {
  const m = v.crew.find((x) => x.id === Number(id)),
    lt = query("rules.crew.lieutenancy"),
    cap = crew.unassignCapLine(v, m, query("rules.crew.max_crew"), lt.Crew);
  modal(
    `<div class="eyebrow">SOMEONE TO RUN A CITY</div><h2>${esc(m.name)} · Lieutenant</h2>${roleLines(lt)
      .map((l, i) => `<p${i === 3 ? ' class="tip-box"' : ""}>${esc(l)}</p>`)
      .join("")}${cap ? `<p class="danger-text">${esc(cap)}</p>` : ""}<label>City<select id="lieutenant-city">${crew
      .assignRows(v, m)
      .map((r) => `<option value="${r.id}" ${r.id === m.city ? "selected" : ""}>${esc(`${r.name} · ${plural(r.corners, "corner")} · ${plural(r.units, "unit")} · runs: ${r.runs}`)}</option>`)
      .join("")}${m.city ? '<option value="">Nobody\'s: stand them down</option>' : ""}</select></label>${btn(m.city ? "Change city" : "Run this city", "assign-city", m.id, "primary", !!v.over)}`,
  );
}
// captainDialog is the captain picker (#576, ui viewCaptain): the
// cities with the corners held and who captains each, the fixed
// budgets a night, and the captaincy off them; a member who cannot be
// captain gets why instead.
function captainDialog(id) {
  const m = v.crew.find((x) => x.id === Number(id)),
    cp = query("rules.crew.captaincy"),
    no = crew.captainRefusal(m, cp, query("rules.crew.can_captain", Number(id))),
    at = crew.captainAt(v, m),
    budgets = cp.Budgets || [],
    pick = m.captain && budgets.includes(m.budget) ? m.budget : budgets[0];
  const head = `<div class="eyebrow">SOMEONE YOU CAN TRUST</div><h2>${esc(m.name)} · Captaincy</h2>${m.trait ? `<p>Veteran trait: ${esc(m.trait)} — ${esc(engineInfo.traits[m.trait] || "")}.</p>` : ""}${crew
    .captainNotes(cp)
    .map((l) => `<p class="subtle-text">${esc(l)}</p>`)
    .join("")}${m.captain ? `<p>Captain of ${esc(cityName(m.captain))} · ${esc(crew.budgetWord(m.budget))} for pay-offs.</p>` : ""}`;
  if (no) return modal(`${head}<p class="danger-text">${esc(no)}</p>`);
  modal(
    `${head}<label>City<select id="captain-city">${crew
      .captainRows(v, m)
      .map((r) => `<option value="${r.id}" ${r.id === at ? "selected" : ""}>${esc(`${r.name} · ${plural(r.corners, "corner")} · captain: ${r.who}`)}</option>`)
      .join("")}</select></label><label>Pay-off budget<select id="captain-budget">${budgets.map((b) => `<option value="${b}" ${b === pick ? "selected" : ""}>${esc(crew.budgetWord(b))}</option>`).join("")}</select></label><div class="card-actions">${btn(m.captain ? "Move the captaincy" : "Make them captain", "name-captain", m.id, "primary", !!v.over)}${m.captain ? btn("Take it off them", "drop-captain", m.id, "subtle", !!v.over) : ""}</div>`,
  );
}
function alignedAction(a, id) {
  try {
    switch (a) {
      case "preview":
        previewTonight();
        break;
      case "advance-day":
        if (v.card) {
          dilemma();
          break;
        }
        act("end_day", [], "A new day in " + city().name);
        if (v.card && !v.over) dilemma();
        break;
      case "lead":
        openAlert(v.report.lead[Number(id)]);
        break;
      case "alert":
        openAlert(v.alerts.find((x) => x.key === id));
        break;
      case "plans":
        tab = "ledger";
        render();
        break;
      case "pin-plan":
        act("pin_ambition", [id], id ? "Plan pinned" : "Plan unpinned");
        break;
      case "max-buy": {
        const sup = market.sellers(v, id)[0];
        $("#qty-" + id).value = session.maxBuy(sup.id, id).max;
        break;
      }
      case "restock": {
        const days = Number($("#restock-days").value),
          plan = session.restockPlan(v.you.city, days);
        if (!plan.length) {
          notify("No restock needed or no room within your budget.");
          break;
        }
        modal(
          `<div class="eyebrow">TOP UP THE SHELVES</div><h2>Restock for ${days} days</h2>${plan.map((l) => `<p>${l.units} ${esc(l.product)} · ${money(l.cost)}</p>`).join("")}<p>Total <b>${money(plan.reduce((n, l) => n + l.cost, 0))}</b> · Dirty cash left ${money(v.you.dirty_cash - plan.reduce((n, l) => n + l.cost, 0))}</p>${btn("Buy this restock", "restock-apply", days, "primary")}`,
        );
        break;
      }
      case "restock-apply": {
        const plan = session.restockPlan(v.you.city, Number(id));
        let bought = 0;
        try {
          for (const l of plan) {
            session.buy(l.supplier, l.product, l.units);
            bought += l.units;
          }
        } finally {
          v = session.refresh();
          session.take();
          persist();
          render();
          $("#sheet").close();
        }
        notify(`${bought} units purchased`);
        break;
      }
      case "preset-review":
        presetReview($("#preset-id").value);
        break;
      case "preset-apply": {
        const r = act("apply_preset", [id], null);
        if (r) notify(routine.presetSaid(r), r.refused.length > 0);
        break;
      }
      case "lieutenant-city":
        lieutenantDialog(id);
        break;
      case "assign-city": {
        const city = $("#lieutenant-city").value;
        const m = v.crew.find((x) => x.id === Number(id));
        if (city) act("assign", [Number(id), city], crew.assignedSaid(m, cityName(city), query("rules.crew.lieutenancy").Cut));
        else {
          // Over the cap without their slots, it warns first (#576).
          const cap = crew.unassignCapLine(v, m, query("rules.crew.max_crew"), query("rules.crew.lieutenancy").Crew),
            off = () => {
              if (act("unassign", [Number(id)], null) !== null) notify(crew.unassignedSaid(m) + crew.overCapWords(v, query("rules.crew.max_crew")));
            };
          if (cap) confirm(`Stand ${m.name} down?`, [[cap, "danger-text"]], off);
          else off();
        }
        break;
      }
      case "captain":
        captainDialog(id);
        break;
      case "name-captain": {
        const m = v.crew.find((x) => x.id === Number(id)),
          city = $("#captain-city").value,
          budget = Number($("#captain-budget").value);
        act("name_captain", [Number(id), city, budget], crew.captainSaid(m, cityName(city), budget, query("rules.crew.captaincy").Cut));
        break;
      }
      case "drop-captain": {
        const m = v.crew.find((x) => x.id === Number(id));
        act("drop_captain", [Number(id)], crew.dropCaptainSaid(m));
        break;
      }
      case "hit-scouts":
        confirm(
          "Confront these scouts?",
          "Your enforcers will try to disrupt their arrival tonight. This can provoke retaliation.",
          () => act("hit_scouts", [id], "Enforcers sent after the scouts"),
        );
        break;
      default:
        return false;
    }
  } catch (e) {
    notify(e.message, true);
  }
  return true;
}

function nonnegative(id) {
  const n = Number($(id).value);
  if (!Number.isSafeInteger(n) || n < 0)
    throw Error("Enter a nonnegative whole number.");
  return n;
}
function scoutFaction(a) {
  return a.faction || "";
}

// The cut and the cook (#557, the TUI's market dialogs, ui/quality.go):
// the market's lab panel, a product page and a number page for each,
// and the chemist's hand on the crew cards. The words are lab.js's.
function labQualityHTML(id) {
  const held = lab.stock(v, v.you.city, id);
  if (!held) return "";
  const q = lab.quality(v, query, v.you.city, id);
  return `<br><small class="${q < lab.streetQuality(query) ? "warn-text" : "subtle-text"}">Quality ${Math.round(q)}</small>`;
}
function labPanelHTML() {
  const cook = lab.refuseCook(v, query),
    cut = lab.refuseCut(v, query),
    lots = lab.onTheWay(v);
  return `<section class="paper lab-panel" id="lab"><div class="eyebrow">THE LAB</div><p class="subtle-text">${esc(lab.chemist(v) && !v.you.lie_low && lab.cookProducts(v, query, v.you.city).length ? lab.chemistLine(v, query, v.you.city) : cook)}</p>${lots.map((l) => `<p class="lab-lot">${esc(l)}</p>`).join("")}<div class="card-actions">${btn("Cook a batch", "lab-cook", "", "small", !!cook || !!v.over)}${btn("Cut stock", "lab-cut", "", "small", !!cut || !!v.over)}</div>${cut && !v.over ? `<p class="subtle-text">${esc(cut)}</p>` : ""}</section>`;
}
function labHandHTML(m, pool) {
  return lab
    .hand(v, query, m, pool)
    .map(([k, t, warn]) => `<br><small class="${warn ? "warn-text" : "subtle-text"}">${k ? `${esc(k[0].toUpperCase() + k.slice(1))} · ` : ""}${esc(t)}</small>`)
    .join("");
}
// labDialog is the cut's or the cook's dialog where you stand: with no
// product the product page, with one the number page.
function labDialog(kind, id) {
  const here = v.you.city,
    cook = kind === "cook",
    title = `${cook ? "Cook" : "Cut"} · ${cityName(here)}`,
    refusal = cook ? lab.refuseCook(v, query) : lab.refuseCut(v, query);
  if (refusal) {
    modal(`<h2>${esc(title)}</h2><p>${esc(refusal)}</p>`);
    return;
  }
  if (!id) {
    const head = cook ? ["Product", "Cook", "Buy", "Stash"] : ["Product", "Stash", "Quality", "Most", "Price"],
      rows = cook ? lab.cookRows(v, query, here).map((r) => [r.id, r.name, r.cook, r.buy, r.stash]) : lab.cutRows(v, query, here).map((r) => [r.id, r.name, r.units, r.quality, r.most, r.price]);
    modal(
      `<div class="eyebrow">THE LAB</div><h2>${esc(title)}</h2><div class="table-wrap"><table class="data-table"><thead><tr>${head.map((h) => `<th>${h.toUpperCase()}</th>`).join("")}<th></th></tr></thead><tbody>${rows.map(([pid, ...cells]) => `<tr id="lab-${esc(pid)}">${cells.map((c, i) => `<td>${i ? esc(String(c)) : `<b>${esc(c)}</b>`}</td>`).join("")}<td>${btn(cook ? "Cook" : "Cut", "lab-" + kind, pid, "small")}</td></tr>`).join("")}</tbody></table></div><p class="subtle-text">${esc(cook ? lab.chemistLine(v, query, here) : "Cut what?")}</p>`,
    );
    return;
  }
  const most = cook ? lab.cookMax(v, query, here, id) : lab.cutMax(v, query, here, id),
    name = v.cities.find((c) => c.id === here).products.find((p) => p.id === id)?.name || id,
    head = cook ? `${query("rules.crew.chemist_name")} cooks ${name}` : `${name}: ${lab.stock(v, here, id)} at quality ${Math.round(lab.quality(v, query, here, id))}`,
    note = cook ? lab.cookNote(v, query, here, id) : lab.cutNote(v, query, here, id);
  modal(
    `<div class="eyebrow">THE LAB</div><h2>${esc(title)}</h2><p><b>${esc(head)}</b></p><p class="subtle-text">In hand ${money(v.you.dirty_cash)} dirty · ${money(v.you.clean_cash)} clean</p><label>${cook ? "Units" : "Percent added"} <small class="subtle-text">up to ${most}</small><input id="lab-qty" type="number" min="1" max="${most}" placeholder="${cook ? "blank = a batch" : "blank = the most"}"></label><p class="lab-live" id="lab-live"></p><p class="subtle-text">${esc(note)}</p><div class="card-actions">${btn(cook ? "Cook" : "Cut", "lab-" + kind + "-submit", id, "primary", most <= 0 || !!v.over)}${btn("Back", "lab-" + kind, "", "subtle")}</div>`,
  );
  const live = () => {
    const r = lab.readQty($("#lab-qty").value, most);
    $("#lab-live").textContent = r.error || !r.n ? "" : cook ? "Cost: " + lab.cookCostLine(v, query, here, id, r.n) : "After: " + lab.cutAfterLine(v, query, here, id, r.n);
  };
  $("#lab-qty").addEventListener("input", live);
  live();
  $("#lab-qty").focus();
}
function labAction(a, id) {
  switch (a) {
    case "lab-cook":
    case "lab-cut":
      labDialog(a.slice(4), id);
      return true;
    case "lab-cook-submit":
    case "lab-cut-submit": {
      const cook = a === "lab-cook-submit",
        here = v.you.city,
        r = lab.readQty($("#lab-qty").value, cook ? lab.cookMax(v, query, here, id) : lab.cutMax(v, query, here, id));
      if (r.error) {
        $("#lab-live").innerHTML = `<span class="danger-text">${esc(r.error)}</span>`;
        return true;
      }
      if (cook) {
        const k = act("cook", [here, id, r.n], "");
        if (k) notify(lab.cookDone(v, k));
      } else {
        const rec = act("cut", [here, id, r.n / 100], "");
        if (rec) notify(lab.cutDone(v, query, rec));
      }
      return true;
    }
  }
  return false;
}

// The law and its answers (#552), worded by law.js as the TUI's LAW
// panel, DA RACE block, fund dialog, favour call-in, tip confirm,
// lie-low words, bribe dialog, checkpoint confirm, PAYOFFS block and
// cop dialog word them.
const rowsHTML = (rows) => rows.map(([label, text, t]) => `<p class="law-row">${label ? `<small>${esc(label.toUpperCase())}</small> ` : ""}<span class="${tone(t)}">${esc(text)}</span></p>`).join("");
const parasHTML = (lines) => lines.map(([text, t]) => `<p class="${tone(t)}">${esc(text)}</p>`).join("");
// lawPanelHTML is the risk panel's LAW lines: the chief, the DA, the
// pressure here and elsewhere; the favour's call while the chief owes.
function lawPanelHTML() {
  return `<div class="law-lines">${law
    .lawLines(v)
    .map((l) => `<p class="${tone(l.tone)}">${esc(l.text)}${l.owes ? ` ${btn("Call in the favour", "favour", "", "small", !!v.over)}` : ""}</p>`)
    .join("")}</div>`;
}
// lawSectionHTML is the Ledger's law: the chief and the DA, the DA
// race while the tickets take money, the fund, and the bought law.
function lawSectionHTML() {
  const l = v.law,
    lines = law.lawLines(v);
  return `<h3 class="section-gap">The law</h3><div class="cards" id="law"><article class="card"><div class="eyebrow">THE CHIEF AND THE DA</div><h3>Chief ${esc(l.chief)}</h3><p>${esc(lines[0].text.replace(/^Chief [^·]*· /, ""))}${lines[0].owes ? `<br>${btn("Call in the favour", "favour", "", "small", !!v.over)}` : ""}</p><h3>DA ${esc(l.da)}</h3><p>${esc(lines[1].text.replace(/^DA [^·]*· /, ""))}</p><p class="subtle-text">${esc(lines.slice(2).map((x) => x.text).join(". "))}</p></article><article class="card" id="fund"><div class="eyebrow">GIVE SOMETHING BACK</div><h3>Fund a city</h3><p>Goodwill takes pressure off a city a little every day${law.raceShown(v) ? "; while the DA race runs, the same clean cash can back a ticket" : ""}. Clean cash only.</p>${upkeepHTML()}${btn("Fund a city", "fund-open", v.you.city, "small", !v.you.clean_cash || !!v.over)}</article>${payoffsHTML()}</div>${raceHTML()}`;
}
// raceHTML is the DA RACE block (ui/race.go, #534): the vote's day and
// the odds per ticket, who sits, a row a city with what your money
// holds there, and what it costs; the da_race alert lands on it.
function raceHTML() {
  if (!law.raceShown(v)) return "";
  const head = law.raceLines(v, query, v.you.city).filter(([k]) => k !== "yours");
  return `<section class="paper race" id="race"><div class="eyebrow">DA RACE · ${esc(law.raceNote(v).toUpperCase())}</div><div class="stat-row">${law
    .raceOdds(query)
    .map(([t, o]) => `<span><b>${Math.round(o * 100)}%</b>${esc(t)}</span>`)
    .join("")}</div>${rowsHTML(head)}<div class="table-wrap"><table class="data-table"><thead><tr><th>CITY</th><th>TICKET</th><th>BACKED</th><th>POINTS</th><th></th></tr></thead><tbody>${law
    .raceRows(v, query)
    .map((r) => `<tr id="race-${esc(r.city)}"><td>${esc(r.name)}</td><td class="${r.hedged ? "danger-text" : ""}">${esc(r.ticket)}</td><td>${r.backed ? money(r.backed) : "-"}</td><td>${esc(r.points)}</td><td>${btn("Back a ticket", "fund-open", r.city, "small", !v.you.clean_cash || !!v.over)}</td></tr>`)
    .join("")}</tbody></table></div><p class="subtle-text">${esc(law.raceCosts(query))}</p></section>`;
}
// payoffsHTML is the PAYOFFS card: the live deals and when each runs
// out, what a fixer hears, and the envelopes: a bribe, a cop's word.
function payoffsHTML() {
  const rows = law.payoffRows(v, query),
    note = law.payoffNote(v);
  return `<article class="card" id="payoffs"><div class="eyebrow">PAYOFFS</div><h3>The bought law</h3>${rows.length ? rows.map((p) => `<p><b>${esc(p.who)}</b> · ${esc(p.what)}<br><small class="subtle-text">until day ${p.until}, ${plural(Math.max(0, p.until - v.day), "day")} left</small></p>`).join("") : '<p class="subtle-text">Nobody on the payroll. A route\'s checkpoint is bought on its card in Transport.</p>'}${note ? `<p class="subtle-text">${esc(note)}</p>` : ""}${law.cold(v) && rows.length ? '<p class="danger-text">A law-and-order DA sits: it ends within the week.</p>' : ""}<div class="card-actions">${btn("Bribe", "bribe-open", "", "small", !v.you.dirty_cash || !!v.over)}${btn("Pay a cop", "cop-open", "", "small subtle", !v.you.dirty_cash || !!v.over)}</div></article>`;
}
// checkpointHTML is a route's deal on its Transport card: held until a
// day, or for sale at its price.
function checkpointHTML(r) {
  const word = law.dealWord(query, r.id);
  return `<p class="subtle-text">${r.checkpoint_until ? `The ${esc(word)} is yours until day ${r.checkpoint_until}. ` : ""}${btn(`Buy the ${esc(word)} · ${money(law.dealPrice(query, r.id))}`, "checkpoint", r.id, "small subtle", !v.you.dirty_cash || !!v.over)}</p>`;
}
// fundDialog is the fund dialog (ui/law.go viewFund, viewCampaign): the
// city, the goodwill amount and what it buys, and while the tickets
// take money the ticket and the amount behind it. Nothing is given
// until Give.
function fundDialog(cityId) {
  const race = law.raceShown(v);
  modal(
    `<div class="eyebrow">GIVE SOMETHING BACK</div><h2>Fund a city</h2><p>${money(v.you.clean_cash)} clean in hand. Nothing is given until you press Give.</p><label>City<select id="fund-city" data-input="fund">${v.cities.map((c) => `<option value="${c.id}" ${c.id === cityId ? "selected" : ""}>${esc(c.name)}</option>`).join("")}</select></label><label>Goodwill<input id="fund-amount" data-input="fund" type="number" min="0" step="1000" aria-label="Clean cash for goodwill" placeholder="blank = up to 100, upkeep kept back"></label><div id="fund-preview"></div>${
      race
        ? `<h3>The campaign</h3><label>Ticket<select id="fund-ticket" data-input="fund">${law.TICKETS.map((t) => `<option value="${t}">${esc(law.stanceWord(t))}</option>`).join("")}</select></label><label>Behind the ticket<input id="back-amount" data-input="fund" type="number" min="0" step="10000" aria-label="Clean cash behind the ticket" placeholder="blank = nothing"></label><div id="back-preview"></div>`
        : ""
    }<p class="danger-text" id="fund-error"></p><div class="card-actions">${btn("Give", "fund-give", "", "primary", !!v.over)}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  const camp = law.campaign(v, cityId);
  if (race && camp.ticket && !camp.hedged) $("#fund-ticket").value = camp.ticket;
  fundPreview();
  (race && cityId !== v.you.city ? $("#back-amount") : $("#fund-amount"))?.focus();
}
function fundPreview() {
  const c = v.cities.find((x) => x.id === $("#fund-city").value),
    f = law.readAmount($("#fund-amount").value, law.fundBlank(v, query, c));
  $("#fund-preview").innerHTML = rowsHTML(law.fundLines(v, query, c, $("#fund-amount").value));
  $("#fund-amount").max = law.maxFund(v, query, c);
  if ($("#back-preview")) {
    const given = f.err ? 0 : f.amount;
    $("#back-preview").innerHTML = rowsHTML(law.campaignLines(v, query, c, given, $("#fund-ticket").value, $("#back-amount").value));
    $("#back-amount").max = law.maxBack(v, query, c, given);
  }
  $("#fund-error").textContent = "";
}
// bribeDialog is the bribe dialog (ui/bribes.go viewBribe): the chief
// or the DA with what is known of them and their price, the amount
// (blank is the price) and what the envelope is likely to do.
function bribeDialog() {
  modal(
    `<div class="eyebrow">AN ENVELOPE</div><h2>Bribe</h2><p>${money(v.you.dirty_cash)} dirty in hand.</p>${law.TARGETS.map((t, i) => `<label class="choice"><span><input type="radio" name="bribe-target" value="${t}" data-input="bribe" ${i === 0 ? "checked" : ""}> <b>${esc(law.officialName(v, t))}</b> · ${esc(law.officialWord(v, query, t))} · ${money(law.bribePrice(query, t))}</span></label>`).join("")}<label>Amount<input id="bribe-amount" data-input="bribe" type="number" min="1" step="1000" aria-label="Dirty cash in the envelope" placeholder="blank = the price"></label><p id="bribe-odds"></p>${parasHTML(law.bribeTerms(v, query))}<div class="card-actions">${btn("Pay", "bribe-pay", "", "primary", !!v.over)}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  bribePreview();
}
function bribeTarget() {
  return $('input[name="bribe-target"]:checked')?.value || "chief";
}
function bribePreview() {
  const t = bribeTarget(),
    r = law.readAmount($("#bribe-amount").value, law.bribePrice(query, t)),
    o = r.err ? { text: r.err, tone: "danger" } : law.bribeOdds(v, query, t, r.amount);
  $("#bribe-odds").className = tone(o.tone);
  $("#bribe-odds").textContent = o.text;
}
// copDialog is the cop dialog (ui/intel.go viewPayCop): the amount,
// blank the price, and how straight the word is at it.
function copDialog() {
  modal(
    `<div class="eyebrow">A WORD ON THE POLICE</div><h2>Pay a cop</h2><p>${money(v.you.dirty_cash)} dirty in hand.</p><label>Amount<input id="cop-amount" data-input="cop" type="number" min="1" step="500" aria-label="Dirty cash for a cop" placeholder="blank = the price"></label><div id="cop-preview"></div><div class="card-actions">${btn("Pay", "cop-pay", "", "primary", !!v.over)}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  copPreview();
}
function copPreview() {
  $("#cop-preview").innerHTML = parasHTML(law.copLines(v, engineInfo, $("#cop-amount").value));
}
document.addEventListener("input", (e) => {
  const kind = e.target.dataset?.input;
  if (kind === "fund") fundPreview();
  else if (kind === "bribe") bribePreview();
  else if (kind === "cop") copPreview();
});
function lawAction(a, id) {
  try {
    switch (a) {
      case "fund-open":
        if (!v.you.clean_cash) throw Error("Can't fund a city: goodwill is bought with clean cash, and you have none.");
        fundDialog(id || v.you.city);
        break;
      case "fund-give": {
        const c = v.cities.find((x) => x.id === $("#fund-city").value),
          ticket = $("#fund-ticket")?.value || "",
          plan = law.fundPlan(v, query, c, $("#fund-amount").value, $("#back-amount")?.value);
        if (plan.err) {
          $("#fund-error").textContent = plan.err;
          break;
        }
        if (plan.fund > 0 && act("fund", [c.id, plan.fund], null, { close: false, order: { key: "fund-" + c.id, text: `Goodwill for ${c.name}` } }) === null) break;
        if (plan.back > 0 && act("back", [c.id, ticket, plan.back], null, { close: false }) === null) break;
        $("#sheet").close();
        notify(law.fundSaid(v, query, c, plan.fund, plan.back, ticket));
        break;
      }
      case "favour": {
        const why = law.favourRefusal(v, query("rules.heat.due"));
        if (why) throw Error(why);
        confirm("Call in the favour?", law.favourLines(v, query).map(([t, k]) => [t, tone(k)]), () => {
          if (act("call_favour", [], null) !== null) notify(law.favourSaid(v));
        });
        break;
      }
      case "bribe-open":
        if (!v.you.dirty_cash) throw Error("Can't bribe anybody: an envelope is dirty cash, and you have none.");
        bribeDialog();
        break;
      case "bribe-pay": {
        const t = bribeTarget(),
          r = law.readAmount($("#bribe-amount").value, law.bribePrice(query, t));
        if (r.err || r.amount <= 0) throw Error(r.err || "Enter a whole number of dollars.");
        if (act("bribe", [t, r.amount], null) !== null) notify(law.bribeSaid(v, t, r.amount));
        break;
      }
      case "cop-open":
        if (!v.you.dirty_cash) throw Error("Can't pay a cop: a cop takes dirty cash, and you have none.");
        copDialog();
        break;
      case "cop-pay": {
        const r = law.readAmount($("#cop-amount").value, law.copBlank(v, engineInfo));
        if (r.err || r.amount <= 0) throw Error(r.err || "Enter a whole number of dollars.");
        if (act("pay_cop", [r.amount], null) !== null) notify(law.copSaid(engineInfo, r.amount));
        break;
      }
      case "checkpoint": {
        const r = v.routes.find((x) => x.id === id),
          price = law.dealPrice(query, id),
          word = law.dealWord(query, id);
        confirm(`Buy the ${word}?`, law.checkpointLines(v, query, r).map(([t, k]) => [t, tone(k)]), () => {
          if (act("buy_checkpoint", [id], null) !== null) notify(law.checkpointSaid(v, query, v.routes.find((x) => x.id === id), price));
        });
        break;
      }
      case "lie-low": {
        // Lying low with a handoff queued holds it tonight (#503): ask.
        const queued = orders.filter((o) => o.contract && o.units),
          lines = v.you.lie_low ? [] : law.lieLowHandoffs(v, queued),
          go = () => act("set_lie_low", [!v.you.lie_low], null) !== null && notify(law.lieLowSaid(v));
        if (lines.length) confirm("Lie low? A handoff is queued", lines.map((l, i) => (i === lines.length - 1 ? [l, "subtle-text"] : l)), go);
        else go();
        break;
      }
      default:
        return false;
    }
  } catch (e) {
    notify(e.message, true);
  }
  return true;
}

// The routine (#556), worded by routine.js as the TUI's market pane,
// sell and buy dialogs and cart word them: a product's standing order
// and contract on its row, the routine dialog that places, edits,
// cancels and clears them, and the cart read off the view.
function routineRowsHTML(cityId, id) {
  const st = routine.standingRow(v, query, cityId, id),
    rows = routine.contractRows(v, query, cityId, id);
  return `${st ? `<br><small class="warn-text">Standing ${esc(st)}</small>` : ""}${rows.map(([t, k]) => `<br><small class="${tone(k)}">${esc(t[0].toUpperCase() + t.slice(1))}</small>`).join("")}`;
}
// cartHTML is tonight's cart (ui/cart.go): each order, each standing
// order that sells tonight and each contract kept, with the totals and
// the line's remove; the page's other moves queued for tonight after.
function cartHTML() {
  const lines = routine.cart(v, query);
  if (!lines.length && !orders.length) return "";
  const remove = { sell: "Cancel", standing: "Cancel standing", keep: "Clear", buy: "Return", credit: "Return", morning: "Return" };
  return `<h3 class="section-gap" id="cart">Tonight’s orders</h3>${lines.length ? `<p class="subtle-text">${esc(routine.cartTotals(lines))}</p>` : ""}${lines
    .map((l) => `<div class="order-item" id="cart-${l.kind}-${esc(l.city)}-${esc(l.product)}">${esc(routine.cartLine(v, l))}${l.kind === "morning" ? `<br><small class="subtle-text">${esc(routine.morningNote(v, l))}</small>` : ""} <button class="quiet-link" data-action="cart-remove" data-id="${l.kind}|${esc(l.city)}|${esc(l.product)}" ${v.over ? "disabled" : ""}>${remove[l.kind]}</button></div>`)
    .join("")}${orders.map((o) => `<div class="order-item">${esc(o.text)}</div>`).join("")}`;
}
// routineDialog is a product's routine where you stand: its standing
// order (the TUI's sell dialog at standing, #114, #503) and its supply
// contract (the buy dialog at keep at, #113), each with what stands,
// the edit words (#443), the field and what it does.
function routineDialog(id) {
  const here = v.you.city,
    name = routine.productName(v, id),
    st = routine.standing(v, here, id),
    k = routine.contract(v, here, id),
    lines = (xs) => xs.filter(Boolean).map((t) => `<p class="subtle-text">${esc(t)}</p>`).join(""),
    nos = routine.noSupply(v, here, id),
    sellNo = routine.sellRefusal(v, query, here, id) && !routine.landing(v, here, id) ? routine.sellRefusal(v, query, here, id) : "";
  modal(
    `<div class="eyebrow">THE ROUTINE</div><h2>${esc(name)} · ${esc(cityName(here))}</h2><p>${routine.stock(v, here, id)} stashed · ${v.you.room?.[here] || 0} room in the stash</p>
<h3>Standing order</h3><p>${st ? `<b>${esc(routine.standingRow(v, query, here, id))}</b> every night until you cancel it` : "None. A standing order sells every night at the crew's cut, wherever you queue no sale of your own."}</p>${st ? `<p class="warn-text">${esc(routine.editingStanding(v, here, id))}</p>` : ""}${lines([routine.dueLine(v, query, here, id), routine.landingLine(v, query, here, id)])}
<label>Units a night <small class="subtle-text">up to ${routine.standable(v, query, here, id)}</small><input id="standing-qty" type="number" min="1" max="${routine.standable(v, query, here, id)}" placeholder="blank = all of it, every night" value="${st && !st.all ? st.qty : ""}"></label><label>Approach <select id="standing-dial">${["quiet", "normal", "aggressive"].map((d) => `<option value="${d}" ${d === (st?.dial || saleDial) ? "selected" : ""}>${d}</option>`).join("")}</select></label>${sellNo ? `<p class="subtle-text">${esc(sellNo)}</p>` : ""}<p class="danger-text" id="standing-error"></p><div class="card-actions">${btn(st ? "Change the standing order" : "Place standing order", "standing-place", id, "small", !!sellNo || !!v.over)}${st ? btn("Cancel standing order", "standing-cancel", id, "small subtle", !!v.over) : ""}</div>
<h3>Supply contract</h3>${nos ? `<p class="subtle-text">${esc(nos)}</p>` : `${routine
      .contractRows(v, query, here, id)
      .map(([t, tn]) => `<p class="${tone(tn)}">${esc(t[0].toUpperCase() + t.slice(1))}</p>`)
      .join("") || "<p>None.</p>"}${k && !k.own ? `<p class="subtle-text">Your lieutenant's to keep; a contract of yours takes its place.</p>` : ""}${k?.own ? `<p class="warn-text">${esc(routine.editingContract(v, here, id))}</p>` : ""}<p class="subtle-text">${esc(routine.contractTerms(query))}</p><label>Keep at <small class="subtle-text">up to ${routine.keepMax(v, here, id)}</small><input id="keep-qty" type="number" min="1" max="${routine.keepMax(v, here, id)}" placeholder="blank = what the stash holds" value="${k?.own ? k.units : ""}"></label><p class="danger-text" id="keep-error"></p><div class="card-actions">${btn(k?.own ? "Change the contract" : "Set contract", "keep-set", id, "small", !!v.over)}${k?.own ? btn("Clear contract", "keep-clear", id, "small subtle", !!v.over) : ""}</div>`}`,
  );
}
function routineAction(a, id) {
  try {
    const here = v?.you.city;
    switch (a) {
      case "routine":
        routineDialog(id);
        break;
      case "standing-place": {
        const dial = $("#standing-dial").value,
          r = routine.readStanding(v, query, here, id, $("#standing-qty").value);
        if (r.err) {
          $("#standing-error").textContent = r.err;
          break;
        }
        if (act("place_standing", [here, id, r.qty, dial], null, { close: false }) !== null) {
          notify(routine.standingSaid(v, query, here, id, r.qty, dial));
          routineDialog(id);
        }
        break;
      }
      case "standing-cancel":
        if (act("cancel_standing", [here, id], routine.cancelledSaid, { close: false }) !== null) routineDialog(id);
        break;
      case "keep-set": {
        const r = routine.readKeep(v, here, id, $("#keep-qty").value);
        if (r.err) {
          if (r.units) $("#keep-qty").value = r.units;
          $("#keep-error").textContent = r.err;
          break;
        }
        if (act("set_supply", [here, id, r.units], null, { close: false }) !== null) {
          notify(routine.keepSaid(v, query, here, id, r.units));
          routineDialog(id);
        }
        break;
      }
      case "keep-clear": {
        const k = routine.contract(v, here, id);
        if (act("clear_supply", [here, id], routine.clearedSaid(v, here, id, k?.units), { close: false }) !== null) routineDialog(id);
        break;
      }
      case "cart-remove": {
        // x on a cart line (ui/cart.go removeCartLine): an order is
        // cancelled, a standing one for good, a contract cleared.
        const [kind, c, p] = id.split("|"),
          k = routine.contract(v, c, p);
        const line = routine.cart(v, query).find((l) => l.kind === kind && l.city === c && l.product === p);
        if (line && routine.isBuy(line)) {
          const refund = act(routine.giveBack(line), [c, p, line.qty], null);
          if (refund !== null) notify(routine.returned(v, line, line.qty, refund));
        } else if (kind === "keep") act("clear_supply", [c, p], routine.clearedSaid(v, c, p, k?.units));
        else if (kind === "standing") act("cancel_standing", [c, p], routine.cancelledSaid);
        else act("cancel_sell", [c, p], "Order cancelled.");
        break;
      }
      default:
        return false;
    }
  } catch (e) {
    notify(e.message, true);
  }
  return true;
}

// The sweep, the houses and the assets (#581), worded by property.js as
// the TUI's sweep dialog, STASH table, move, guard and drop dialogs and
// ASSETS block word them.
function sweepDialog() {
  modal(
    `<div class="eyebrow">A FUTURE SOMEWHERE ELSE</div><h2>Sweep offshore</h2><p>${money(v.you.clean_cash)} clean in hand · ${money(v.you.offshore)} offshore</p><p>Sweep: <span class="${v.you.sweep_on ? "good-text" : "subtle-text"}">${esc(property.sweepState(v, query))}</span></p><label>Keep in hand <small class="subtle-text">up to ${money(property.sweepMax(v))}</small><input id="sweep-keep" data-input="sweep" type="number" min="0" step="1000" aria-label="Clean cash the sweep keeps in hand" placeholder="blank = the upkeep" value="${v.you.sweep_on && v.you.sweep_keep > 0 ? v.you.sweep_keep : ""}"></label><div id="sweep-preview"></div><p class="subtle-text">${esc(property.sweepRules(query))}</p><p class="danger-text" id="sweep-error"></p><div class="card-actions">${btn(v.you.sweep_on ? "Change the line" : "Turn the sweep on", "sweep-set", "", "primary", !!v.over)}${v.you.sweep_on ? btn("Turn it off", "sweep-stop", "", "subtle", !!v.over) : ""}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  sweepPreview();
}
function sweepPreview() {
  const r = property.readSweep(v, $("#sweep-keep").value);
  $("#sweep-preview").innerHTML = rowsHTML(property.sweepLines(v, query, r.err ? 0 : r.keep));
  $("#sweep-error").textContent = "";
}
// ownedHousesHTML is the houses you lease (the TUI's STASH table and a
// house's pane): the status, what it holds, the rent, the guard, the
// robbery odds, and move, guard and drop.
function ownedHousesHTML() {
  if (!v.houses?.length) return "";
  return `<h3>Your houses</h3>${v.houses
    .map((h) => {
      const [st, stTone] = property.houseStatus(h);
      return `<div class="report-section" id="house-${esc(h.id)}"><div class="row"><b>${esc(h.name)}</b><span class="tag">${esc(cityName(h.city))}</span></div><p class="${tone(stTone)}">${esc(st)}</p>${rowsHTML(property.houseRows(v, query, h))}${h.known ? `<p class="danger-text">${esc(property.knownWords)}</p>` : ""}<div class="card-actions">${btn("Move stock", "move-open", h.city, "small", !!property.moveRefusal(v, h.city) || !!v.over)}${btn("Guard", "guard-open", h.id, "small subtle", !!v.over)}${btn("Drop", "drop-house", h.id, "small subtle", !!v.over)}</div></div>`;
    })
    .join("")}`;
}
// ownedAssetsHTML is the assets you own: what each does, its status,
// its upkeep and heat floor, when it was bought.
function ownedAssetsHTML() {
  if (!v.assets?.length) return "";
  return `<h3>Your assets</h3>${v.assets
    .map((a) => {
      const [st, stTone] = property.assetStatus(v, a);
      return `<div class="report-section" id="asset-${esc(a.id)}"><b>${esc(a.name)}</b><p class="${tone(stTone)}">${esc(st)}</p><p class="subtle-text">${esc(property.assetBlurb(v, a, engineInfo))}</p>${rowsHTML(property.assetRows(v, query, a))}</div>`;
    })
    .join("")}`;
}
// moveDialog is the move (ui/houses.go): from, to, the product and the
// quantity, with what the destination holds and the drive's heat.
function moveDialog(cityId, from = null) {
  const sources = property.places(v, cityId);
  from = from ?? sources[0];
  const dests = property.places(v, cityId, from),
    products = v.cities.find((c) => c.id === cityId).products.filter((p) => property.held(v, cityId, from, p.id) > 0),
    opt = (id, sel) => `<option value="${esc(id)}" ${id === sel ? "selected" : ""}>${esc(property.placeName(v, id))} · ${property.placeHolds(v, cityId, id).units}/${property.placeHolds(v, cityId, id).capacity}</option>`;
  modal(
    `<div class="eyebrow">MOVE STOCK</div><h2>${esc(cityName(cityId))}</h2><label>From<select id="move-from" data-change="move-from" data-city="${esc(cityId)}">${sources.map((p) => opt(p, from)).join("")}</select></label><label>To<select id="move-to" data-input="move">${dests.map((p) => opt(p, dests[0])).join("")}</select></label><label>Product<select id="move-product" data-input="move">${products.map((p) => `<option value="${esc(p.id)}">${esc(p.name)} · ${property.held(v, cityId, from, p.id)}</option>`).join("")}</select></label><label>Quantity<input id="move-qty" data-input="move" type="number" min="1" placeholder="blank = all"></label><div id="move-preview"></div><p class="danger-text" id="move-error"></p><div class="card-actions">${btn("Move", "move-submit", cityId, "primary", !!v.over)}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  movePreview();
}
function moveForm() {
  const city = $("#move-from").dataset.city,
    from = $("#move-from").value,
    to = $("#move-to").value,
    product = $("#move-product").value,
    most = product ? property.moveMax(v, city, from, to, product) : 0,
    raw = $("#move-qty").value.trim(),
    n = raw === "" ? most : Number(raw);
  return { city, from, to, product, most, n, err: !product ? "Nothing to move." : !Number.isSafeInteger(n) || n <= 0 ? (most <= 0 ? `${property.placeName(v, to)} has no room for it.` : "Enter a whole number above zero.") : n > most ? `At most ${most} can move.` : "" };
}
function movePreview() {
  const f = moveForm();
  $("#move-qty").max = f.most;
  $("#move-preview").innerHTML = `<p class="subtle-text">${esc(property.moveNote(v, f.city, f.to, engineInfo))}</p>${f.product && !f.err ? rowsHTML([["heat", property.moveHeatLine(query, f.city, f.product, f.n), "subtle"]]) : ""}`;
  $("#move-error").textContent = "";
}
// guardDialog is the guard picker: nobody, or an enforcer with where
// they are now.
function guardDialog(id) {
  const h = v.houses.find((x) => x.id === id),
    rows = property.guardRows(v, h);
  modal(
    `<div class="eyebrow">SOMEONE INSIDE</div><h2>Guard ${esc(h.name)}</h2><p class="subtle-text">${esc(rows.length > 1 ? property.guardAsk(h) : property.noGuards)}</p>${rows
      .map((r) => `<button class="choice" data-action="guard-set" data-id="${esc(id)}|${r.id}"><b>${esc(r.name)}</b>${r.skill != null ? ` · skill ${r.skill}` : ""} <span class="${tone(r.where[1])}">${esc(r.where[0])}</span></button>`)
      .join("")}<div class="card-actions">${btn("Back", "properties", "", "subtle")}</div>`,
  );
}
document.addEventListener("input", (e) => {
  const kind = e.target.dataset?.input;
  if (kind === "sweep") sweepPreview();
  else if (kind === "move") movePreview();
});
document.addEventListener("change", (e) => {
  if (e.target.dataset?.change === "move-from") moveDialog(e.target.dataset.city, e.target.value);
});
function propertyAction(a, id) {
  try {
    switch (a) {
      case "sweep-open":
        sweepDialog();
        break;
      case "sweep-set": {
        const r = property.readSweep(v, $("#sweep-keep").value);
        if (r.err) {
          if (r.field !== undefined) $("#sweep-keep").value = r.field;
          $("#sweep-error").textContent = r.err;
          break;
        }
        act("set_sweep", [r.keep], property.sweepSaid(query, r.keep));
        break;
      }
      case "sweep-stop":
        act("stop_sweep", [], property.sweepOffSaid);
        break;
      case "move-open": {
        const why = property.moveRefusal(v, id);
        if (why) throw Error(why);
        moveDialog(id);
        break;
      }
      case "move-submit": {
        const f = moveForm();
        if (f.err) {
          $("#move-error").textContent = f.err;
          break;
        }
        const n = act("move", [f.city, f.from, f.to, f.product, f.n], null, { close: false });
        if (n !== null) {
          notify(property.movedSaid(v, query, f.city, f.from, f.to, f.product, n === true ? f.n : n));
          properties();
        }
        break;
      }
      case "guard-open": {
        const h = v.houses.find((x) => x.id === id);
        if (!v.crew.some((m) => m.role === "enforcer") && !h.guard) throw Error(property.noGuards);
        guardDialog(id);
        break;
      }
      case "guard-set": {
        const [hid, mid] = id.split("|"),
          h = v.houses.find((x) => x.id === hid);
        if (act("guard", [hid, Number(mid)], null, { close: false }) !== null) {
          notify(property.guardSaid(v, query, h, Number(mid)));
          properties();
        }
        break;
      }
      case "drop-house": {
        const h = v.houses.find((x) => x.id === id),
          units = property.houseUnits(h),
          [ask, what] = property.dropLines(h);
        confirm(`Drop ${h.name}?`, [ask, units > 0 ? [what, "danger-text"] : what], () => {
          if (act("drop", [id], null) !== null) notify(property.droppedSaid(h, units));
        });
        break;
      }
      default:
        return false;
    }
  } catch (e) {
    notify(e.message, true);
  }
  return true;
}

// The connects, the market pane, the buy and the routes (#582), worded
// by market.js and routes.js as the TUI's SUPPLIERS block and connect's
// pane, the market's product pane, the buy dialog's connect and pay
// steps, and the map's route pane and target dialog; the preset review
// by routine.js.
function paneRowsHTML(rows) {
  return rows.map(([label, text, t]) => `<p class="pane-row"><small>${esc(label.toUpperCase())}</small><span class="${tone(t)}">${esc(text)}</span></p>`).join("");
}
// suppliersHTML is the market's SUPPLIERS block for a city: one row a
// connect with the lot, what they have left today, the relationship and
// the note, and the debt line where you owe.
function suppliersHTML(cityId) {
  const rows = market.connectsIn(v, cityId),
    debt = market.debtLine(v);
  return `<div class="paper suppliers" id="suppliers" style="margin-bottom:17px"><div class="row"><div><div class="eyebrow">SUPPLIERS · ${esc(cityName(cityId).toUpperCase())}</div><p class="subtle-text">All purchases use dirty cash, or a connect's book on credit.</p></div>${btn("Transport routes", "routes", "", "small")}</div>${debt ? `<p class="${tone(debt[1])}">${esc(debt[0])}</p>` : ""}${
    rows.length
      ? `<div class="table-wrap"><table class="data-table"><thead><tr><th>CONNECT</th><th>LOT</th><th>LEFT</th><th>REL</th><th></th></tr></thead><tbody>${rows
          .map((c) => {
            const r = market.row(c, ""),
              [n, t] = market.note(v, c);
            return `<tr id="connect-${esc(c.id)}"><td><b>${esc(c.name)}</b>${c.wholesale ? ' <small class="subtle-text">wholesale</small>' : ""}</td><td>${r.lot}</td><td>${r.left.toLocaleString("en-US")}</td><td>${r.rel}</td><td><small class="${tone(t)}">${esc(n)}</small> ${btn("Details", "connect-pane", c.id, "small subtle")}</td></tr>`;
          })
          .join("")}</tbody></table></div>`
      : '<p class="subtle-text">Nobody sells here. Yet.</p>'
  }</div>`;
}
function connectPane(id) {
  const c = v.connects.find((x) => x.id === id),
    how = market.howToBuy(v, query, c);
  modal(
    `<div class="eyebrow">A CONNECT · ${esc(cityName(c.city).toUpperCase())}</div><h2>${esc(c.name)}</h2>${paneRowsHTML(market.pane(v, query, c))}${how ? `<p>${esc(how)}</p>` : ""}<h3>Rules</h3><p class="subtle-text">${esc(market.paneRules(query, c))}</p><div class="card-actions">${btn("Back", "close", "", "subtle")}</div>`,
  );
}
function productPane(id) {
  const c = city(),
    p = c.products.find((x) => x.id === id),
    st = routine.standingRow(v, query, c.id, id),
    rows = [...market.productPane(v, query, c.id, id), ...(st ? [["standing", st, "gold"]] : []), ...routine.contractRows(v, query, c.id, id).map(([t, k], i) => [i ? "" : "contract", t, k])],
    away = market.elsewhere(v, c.id, id),
    notes = market.notes(v, query, c.id, id);
  modal(
    `<div class="eyebrow">THE MARKET · ${esc(c.name.toUpperCase())}</div><h2>${esc(p.name)}</h2>${paneRowsHTML(rows)}${away.length ? `<h3>Elsewhere</h3>${paneRowsHTML(away)}` : ""}${notes.length ? `<h3>Notes</h3>${notes.map(([t, k]) => `<p class="${tone(k)}">${esc(t)}</p>`).join("")}` : ""}<div class="card-actions">${btn("Back", "close", "", "subtle")}</div>`,
  );
}
// buyDialog is the buy (ui/dialogs.go): the connects that sell it here,
// cheapest first, with their price, lot, what is left and the note; the
// quantity; cash or their book; and what the buy comes to.
function buyDialog(id) {
  const cs = market.sellers(v, id);
  if (!cs.length) {
    notify(market.whyNobodySells(v, v.you.city), true);
    return;
  }
  const qty = integer("#qty-" + id) || 1;
  modal(
    `<div class="eyebrow">STOCK UP</div><h2>Buy ${esc(routine.productName(v, id))}</h2>${cs
      .map((c, i) => {
        const [n, t] = market.note(v, c);
        return `<label class="choice"><span><input type="radio" name="buy-connect" value="${esc(c.id)}" data-input="buy" ${i === 0 ? "checked" : ""}> <b>${esc(c.name)}</b> · ${market.row(c, id).unit != null ? "$" + c.prices[id].toFixed(2) : "—"} · lot ${c.lot} · ${c.cap.toLocaleString("en-US")} left · rel ${Math.round(c.rel)} <small class="${tone(t)}">${esc(n)}</small></span></label>`;
      })
      .join(
        "",
      )}<label>Quantity<input id="buy-qty" data-input="buy" type="number" min="1" value="${qty}" aria-label="Units to buy"></label><label class="choice"><span><input type="radio" name="buy-pay" value="cash" data-input="buy" checked> Cash</span></label><label class="choice"><span><input type="radio" name="buy-pay" value="credit" data-input="buy" id="buy-credit"> Credit, on their book</span></label><div id="buy-preview"></div><p class="danger-text" id="buy-error"></p><div class="card-actions">${btn("Buy", "buy-go", id, "primary", !!v.over)}${btn("Keep playing", "close", "", "subtle")}</div>`,
  );
  buyPreview(id);
}
function buyChoice(id) {
  const c = v.connects.find((x) => x.id === $('input[name="buy-connect"]:checked')?.value) || market.sellers(v, id)[0],
    onCredit = $('input[name="buy-pay"]:checked')?.value === "credit",
    qty = integer("#buy-qty"),
    cashMax = session.maxBuy(c.id, id).max,
    bookMax = query("max_buy", c.id, id, true).max;
  return { c, onCredit, qty, cashMax, bookMax };
}
function buyPreview(id) {
  if (!$("#buy-preview")) return;
  const { c, onCredit, qty, cashMax, bookMax } = buyChoice(id),
    room = session.maxBuy(c.id, id),
    lines = [...market.blurb(v, c)];
  $("#buy-credit").disabled = market.credit(c) <= 0;
  if (onCredit) lines.push(...market.creditTerms(v, c, id, qty));
  else {
    const short = market.cashShort(c, qty, cashMax, bookMax);
    lines.push(short ? [short, "warn"] : [`${money(market.quote(c, id, qty, false))} dirty, up to ${cashMax}. Bought now, once.`, "subtle"]);
  }
  lines.push([`Stash after: ${room.held + qty} / ${room.capacity}`, room.held + qty > room.capacity ? "bad" : "subtle"]);
  const kept = routine.keptLine(v, v.you.city, id);
  if (kept) lines.push([kept, "subtle"]);
  $("#buy-preview").innerHTML = lines.map(([t, k]) => `<p class="${tone(k)}">${esc(t)}</p>`).join("");
}
// presetReview is the preset's review (ui/presets.go viewPresets): each
// setting it moves, from what to what, with the estimate.
function presetReview(pid) {
  const r = session.presetDiff(pid);
  modal(
    `<div class="eyebrow">REVIEW THE ROUTINE</div><h2>${esc(r.preset.name)}</h2><p>${esc(r.preset.blurb)}</p>${
      r.changes.length
        ? `<div class="table-wrap"><table class="data-table"><thead><tr><th>SETTING</th><th>NOW</th><th>AFTER</th><th>ESTIMATE</th></tr></thead><tbody>${r.changes.map((c) => `<tr><td>${esc(routine.changeName(v, c))}</td><td>${esc(c.from)}</td><td><b>${esc(c.to)}</b></td><td><small class="subtle-text">${esc(routine.changeEstimate(v, query, c))}</small></td></tr>`).join("")}</tbody></table></div>`
        : '<p class="subtle-text">Nothing would change: the routine is set that way already.</p>'
    }${r.same ? `<p class="subtle-text">${plural(r.same, "other setting")} of the routine stay as they are.</p>` : ""}${r.refused.map((x) => `<p class="danger-text">Refused, left as it is: ${esc(routine.commandName(v, x.command))}: ${esc(x.why)}.</p>`).join("")}${btn("Apply preset", "preset-apply", r.preset.id, "primary", !r.changes.length || !!v.over)}`,
  );
}
// targetDialog is a route's targets (ui/routes.go viewTarget): what it
// does and buys, each product's target and where it stands, and the
// field with its kind and preview.
function targetDialog(rid, pid) {
  const r = v.routes.find((x) => x.id === rid),
    rows = roads.targetRows(v, query, r);
  pid ||= rows[0]?.id;
  const days = r.days_target?.[pid] > 0;
  modal(
    `<div class="eyebrow">TARGET · ${esc(r.name.toUpperCase())}</div><h2>Keep ${esc(cityName(r.to))} stocked</h2><p class="subtle-text">${esc(roads.targetIntro(v, query, r))}</p><div class="table-wrap"><table class="data-table"><thead><tr><th>PRODUCT</th><th>TARGET</th><th>THERE</th><th>ROAD</th><th>SELLS/DAY</th></tr></thead><tbody>${rows.map((x) => `<tr><td>${esc(x.name)}</td><td>${esc(x.target || "—")}</td><td>${x.there}</td><td>${x.road}</td><td>~${x.sells}</td></tr>`).join("")}</tbody></table></div><label>Product<select id="target-product" data-change="target-product" data-route="${esc(rid)}">${rows.map((x) => `<option value="${x.id}" ${x.id === pid ? "selected" : ""}>${esc(x.name)}</option>`).join("")}</select></label><label class="choice"><span><input type="radio" name="target-kind" value="units" data-input="target" ${days ? "" : "checked"}> Units</span></label><label class="choice"><span><input type="radio" name="target-kind" value="days" data-input="target" ${days ? "checked" : ""}> Days of demand</span></label><p class="subtle-text" id="target-kind-line"></p><label>Target<input id="target-n" data-input="target" type="number" min="0" placeholder="blank = none" value="${(days ? r.days_target[pid] : r.target?.[pid]) || ""}"></label><div id="target-preview"></div><p class="danger-text" id="target-error"></p><div class="card-actions">${btn("Set target", "target-set", rid, "primary", !!v.over)}${btn("Back", "routes", "", "subtle")}</div>`,
  );
  targetPreview();
}
function targetChoice() {
  const rid = $("#target-product").dataset.route;
  return { r: v.routes.find((x) => x.id === rid), pid: $("#target-product").value, days: $('input[name="target-kind"]:checked')?.value === "days" };
}
function targetPreview() {
  if (!$("#target-preview")) return;
  const { r, pid, days } = targetChoice(),
    got = roads.readTarget($("#target-n").value);
  $("#target-kind-line").textContent = roads.kindLine(v, r, pid, days) + ` Up to ${roads.targetMax(v, r, pid, days)}.`;
  $("#target-preview").innerHTML = got.n > 0 ? paneRowsHTML(roads.preview(v, query, r, pid, got.n, days)) : "";
}
function driverDialog(rid) {
  const r = v.routes.find((x) => x.id === rid),
    ds = roads.drivers(v);
  if (!ds.length) {
    notify(roads.noDrivers, true);
    return;
  }
  modal(
    `<div class="eyebrow">THE ROAD</div><h2>Who drives ${esc(r.name)}?</h2>${[{ id: 0, name: "Nobody" }, ...ds].map((m) => `<button class="choice" data-action="driver-set" data-id="${esc(rid)}|${m.id}"><b>${esc(m.name)}</b>${m.id ? ` · skill ${m.skill} · risk −${Math.round(query("rules.logistics.driver_cut", m.skill) * 100)}%` : ""}${m.id && m.id === r.driver ? " · driving it" : ""}${m.route && m.route !== rid ? ` · drives ${esc(v.routes.find((x) => x.id === m.route)?.name || m.route)}` : ""}</button>`).join("")}<div class="card-actions">${btn("Back", "routes", "", "subtle")}</div>`,
  );
}
document.addEventListener("input", (e) => {
  const kind = e.target.dataset?.input;
  if (kind === "buy") buyPreview(e.target.closest("dialog")?.querySelector('[data-action="buy-go"]')?.dataset.id);
  else if (kind === "target") targetPreview();
});
document.addEventListener("change", (e) => {
  if (e.target.dataset?.input === "buy") buyPreview(e.target.closest("dialog")?.querySelector('[data-action="buy-go"]')?.dataset.id);
  else if (e.target.dataset?.input === "target") targetPreview();
  else if (e.target.dataset?.change === "target-product") targetDialog(e.target.dataset.route, e.target.value);
});
function supplyAction(a, id) {
  try {
    switch (a) {
      case "connect-pane":
        connectPane(id);
        break;
      case "product-pane":
        productPane(id);
        break;
      case "buy-go": {
        const { c, onCredit, qty, cashMax, bookMax } = buyChoice(id),
          most = onCredit ? bookMax : cashMax;
        if (!(qty > 0)) {
          $("#buy-error").textContent = "Enter a whole number above zero.";
          break;
        }
        if (qty > most) {
          const short = !onCredit && market.cashShort(c, qty, cashMax, bookMax);
          $("#buy-error").textContent = short || (most > 0 ? `${c.name} can sell you ${most} of it now; the quantity is now ${most}, what fits.` : market.whyNobodySells(v, v.you.city));
          if (most > 0) $("#buy-qty").value = most;
          buyPreview(id);
          break;
        }
        const p = act("buy", [c.id, id, qty, onCredit], null);
        if (p !== null) notify(market.boughtSaid(v, c, id, p));
        break;
      }
      case "route-targets":
        targetDialog(id);
        break;
      case "target-set": {
        const { r, pid, days } = targetChoice(),
          got = roads.readTarget($("#target-n").value);
        if (got.err) {
          $("#target-error").textContent = got.err;
          break;
        }
        if (act(days ? "set_route_days" : "set_route_target", [r.id, pid, got.n], null, { close: false }) !== null) {
          notify(roads.targetSaid(v, query, v.routes.find((x) => x.id === r.id), pid, got.n, days));
          targetDialog(r.id, pid);
        }
        break;
      }
      case "route-driver":
        driverDialog(id);
        break;
      case "driver-set": {
        const [rid, mid] = id.split("|"),
          r = v.routes.find((x) => x.id === rid);
        if (act("set_route_driver", [rid, Number(mid)], null, { close: false }) !== null) {
          notify(roads.driverSaid(v, query, r, Number(mid)));
          routes();
        }
        break;
      }
      default:
        return false;
    }
  } catch (e) {
    notify(e.message, true);
  }
  return true;
}

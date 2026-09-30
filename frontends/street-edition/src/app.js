import { Session } from "./session.js?v=__BUILD_REVISION__";
import { policeLines } from "./police.js?v=__BUILD_REVISION__";
import { alertText } from "./alerts.js?v=__BUILD_REVISION__";
import { alertClass, landing, unknownLine } from "./landing.js?v=__BUILD_REVISION__";
import { howTheyCome, roleLines, temperLine, temperOf } from "./lieutenants.js?v=__BUILD_REVISION__";
import * as crew from "./crew.js?v=__BUILD_REVISION__";
import { engineInfo } from "./engine-info.js?v=__BUILD_REVISION__";
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
    return result;
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
      )}<p>Pressure ${Math.round(c.pressure)} · Goodwill ${Math.round(c.goodwill)}</p><p class="${v.you.dirty_cash > v.law.exposure_line ? "danger-text" : ""}">Dirty cash exposure: ${money(v.you.dirty_cash)} / ${money(v.law.exposure_line)}</p>${tonightHTML()}<details id="police-risk"><summary>Understand the police risk</summary>${policeLines(
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
function renderBusiness() {
  let h = "";
  if (v.card)
    h += `<button class="choice" data-action="dilemma">${icon("journal")} ${esc(v.card.title)} <small>A decision is waiting →</small></button>`;
  if (v.over) h += btn("Read your ending", "ending", "", "primary full");
  for (const a of v.alerts) h += alertButton(a);
  if (v.you.lie_low)
    h += `<div class="todo">${icon("sun")}<div><span>A quiet night</span><small>Street sales are suspended.</small></div></div>`;
  h += orders
    .slice(0, 4)
    .map(
      (o) =>
        `<div class="todo">${icon("check")}<div><span>${esc(o.text)}</span><small>Queued for tonight</small></div></div>`,
    )
    .join("");
  if (!orders.length && !v.card)
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
      return `<button class="map-marker ${corner.owner}" style="left:${x}%;top:${y}%" data-action="corner" data-id="${corner.id}" aria-label="${esc(corner.name)}, ${corner.owner}"><i>${corner.owner === "player" ? "✓" : corner.owner === "rival" ? "!" : i + 1}</i><span class="pin-label">${esc(corner.name)}<small>${corner.owner === "player" ? "Yours" : corner.owner === "rival" ? "Rival ground" : "Unclaimed"}</small></span></button>`;
    })
    .join("");
  return `<div class="map-frame"><img class="city-art" src="assets/city.webp" alt="Hand-drawn waterfront city in colorful ink"><div class="map-wash"></div><div class="map-badge">${esc(c.name.toUpperCase())} · ${c.id === v.you.city ? "YOU ARE HERE" : "ACROSS THE WATER"}</div>${markers}<div class="map-caption">Small beginnings. Bigger possibilities.</div></div><div class="map-legend"><div class="legend-keys"><span><i class="dot"></i>Your ground</span><span><i class="dot rival"></i>Rival ground</span><span><i class="dot none"></i>Unclaimed</span></div><span>Select a corner to act ↗</span></div>${c.id !== v.you.city ? btn("Travel to " + esc(c.name), "travel", c.id, "full") : ""}<div class="quick-actions"><button class="quick-card" data-tab="market">${icon("coin")}<div><b>Work the market</b><small>Stock up & queue sales →</small></div></button><button class="quick-card" data-tab="crew">${icon("crew")}<div><b>Meet your people</b><small>Hire, post & look after →</small></div></button><button class="quick-card" data-tab="empire">${icon("shop")}<div><b>Think bigger</b><small>Businesses & upgrades →</small></div></button></div>${dispatch()}`;
}
function dispatch() {
  const lead = v.report.lead || [],
    news = v.report.sections?.find((s) => s.id === "news")?.lines || [],
    sales = v.report.sections?.find((s) => s.id === "sales")?.lines || [];
  return `<section class="dispatch"><div class="dispatch-heading"><h3>Around the neighborhood</h3><small>THE DAILY DISPATCH · ${v.day === 0 ? "FIRST EDITION" : "DAY " + v.report.day}</small></div><div class="headlines">${lead.length ? lead.map((l, i) => `<button class="headline headline-link" data-action="lead" data-id="${i}"><small>TODAY · ${esc(l.kind)}</small>${esc(l.text)} →</button>`).join("") : `<div class="headline"><small>WORD ON THE STREET</small>${esc(news[0] || (v.day === 0 ? "A new face in Eastside. A single corner. Five hundred dollars. What happens next is up to you." : "The city is watching. Read the paper for last night’s events."))}</div><div class="headline"><small>${v.day ? "THE NIGHT’S BUSINESS" : "YOUR FIRST MOVE"}</small>${esc(sales[0] || news[1] || (v.day ? "Plan your next move, and preview tonight before you commit." : "Visit the market to buy stock, then queue a sale. Nothing moves until you end the day."))}</div>`}</div></section>`;
}
function renderMarket() {
  const c = city(),
    con = v.connects.filter((x) => x.city === c.id && x.open && !x.wholesale),
    stock = v.you.stock[c.id] || {};
  return `${marketTools()}<div class="paper" style="margin-bottom:17px"><div class="row"><div><div class="eyebrow">YOUR CONNECT</div><h3>${con.length ? esc(con[0].name) : "No open supplier"}</h3><p class="subtle-text">All purchases use dirty cash. Supplier prices are estimates; the engine applies the final terms.</p></div>${btn("Transport routes", "routes", "", "small")}</div>${v.connects
    .filter((k) => k.debt > 0)
    .map((k) => `<p class="debt" id="connect-${k.id}"><b>${esc(k.name)}</b>: ${money(k.debt)} owed, due day ${k.debt_due}</p>`)
    .join("")}</div><div class="table-wrap"><table class="data-table"><thead><tr><th>PRODUCT</th><th>BUY / SELL</th><th>STOCK</th><th>QUANTITY</th><th>YOUR MOVE</th></tr></thead><tbody>${c.products
    .map((p) => {
      const supplier = con.find((x) => x.prices[p.id] != null),
        room = supplier ? session.maxBuy(supplier.id, p.id) : null;
      return `<tr id="product-${p.id}"><td><b>${esc(p.name)}</b><br><small class="subtle-text">Demand ${Math.round(p.demand)}</small></td><td>${supplier ? money(supplier.prices[p.id]) : "—"} / ${money(p.price)}</td><td>${stock[p.id] || 0}${room ? `<br><small>Stash ${room.held}/${room.capacity}</small>` : ""}</td><td><input aria-label="${esc(p.name)} quantity" type="number" min="1" max="99999" value="${Math.min(10, stock[p.id] || 10)}" id="qty-${p.id}">${supplier ? btn("Max " + room.max, "max-buy", p.id, "small subtle", !room.max || !!v.over) : ""}</td><td>${btn("Buy", "buy", p.id, "small", !supplier || !room?.max || !!v.over)} ${btn("Sell", "sell", p.id, "small", !stock[p.id] || !!v.over)}</td></tr>`;
    })
    .join(
      "",
    )}</tbody></table></div><div class="row" style="margin-top:14px"><label class="subtle-text">Sales approach <select id="sale-dial" class="inline-select" data-change="sale-dial">${[["quiet", "Quiet · lower profile"], ["normal", "Normal · balanced"], ["aggressive", "Aggressive · more heat"]].map(([d, label]) => `<option value="${d}" ${d === saleDial ? "selected" : ""}>${label}</option>`).join("")}</select></label>${btn("Sell all held stock", "sell-all", "", "small", !!v.over)}</div>${orders.length ? `<h3 class="section-gap">Tonight’s orders</h3>${orders.map((o) => `<div class="order-item">${esc(o.text)}${o.product ? ` <button class="quiet-link" data-action="cancel-sale" data-id="${o.product}">Cancel</button>` : ""}</div>`).join("")}` : ""}<h3 class="section-gap">Private buyers</h3>${v.contracts.length ? `<div class="cards">${v.contracts.map((c) => `<article class="card" id="contract-${c.id}"><span class="tag">${esc(c.status)}</span><h3>${esc(c.name)}</h3><p>${esc(c.pitch)}</p><div class="stat-row"><span><b>${c.units}</b>units</span><span><b>${c.delivered}</b>delivered</span><span><b>${c.due}</b>due day</span></div>${c.status === "offered" ? btn("Accept contract", "contract", c.id, "small") : btn("Deliver stock", "deliver", c.id, "small")}</article>`).join("")}</div>` : '<div class="empty">No private offers today. Check back tomorrow.</div>'}`;
}
function renderCrew() {
  const lt = query("rules.crew.lieutenancy"),
    tun = query("rules.crew.tuning"),
    flip = query("rules.crew.flip_line"),
    most = query("rules.crew.max_crew"),
    hint = howTheyCome(v),
    warn = crew.crewWarning(v, tun);
  return `${hint ? `<div class="tip-box" style="margin-bottom:16px">${esc(hint)}</div>` : ""}${warn ? `<div class="tip-box danger-text" style="margin-bottom:16px">${esc(warn)}</div>` : ""}<div class="row" style="margin-bottom:16px"><span class="subtle-text">${esc(crew.countLine(v, most))}</span><label>Pay <select id="pay-dial" data-change="pay">${crew.PAY.map((x) => `<option value="${x}" ${v.you.pay === x ? "selected" : ""}>${x} · ${money(query("rules.crew.wages", x))}/day</option>`).join("")}</select></label>${askAroundButton()}</div><p class="subtle-text">${esc(crew.payBlurb(v.you.pay))}</p><div class="cards">${v.crew.map((m) => memberCard(m, lt, tun, flip)).join("") || '<div class="empty">For now, it’s just you. Find someone you can count on below.</div>'}</div>${crewSummary()}<h3 class="section-gap">New faces in town</h3><div class="cards">${v.pool.map((m) => `<article class="card"><div class="card-top"><span class="tag">${esc(m.role)}</span><span class="price">${money(m.fee)}</span></div><h3>${esc(m.name)}</h3><p>Age ${m.age} · Skill ${m.skill} · Loyalty ${Math.round(m.loyalty)}%<br>Base wage ${money(m.wage)} / day</p>${m.role === "lieutenant" ? `<p class="front-role">${roleLines(lt).slice(0, 2).map(esc).join(" ")}</p>` : ""}${btn("Hire " + esc(m.name), "hire", m.id, "small", v.you.dirty_cash < m.fee || !!v.over)}</article>`).join("")}</div>`;
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
  return `<article class="card" id="member-${m.id}"><div class="crew-header"><div class="portrait ${m.role === "runner" ? "teal" : ""}">${esc(m.name.slice(0, 1))}</div><div><span class="tag">${esc(m.role)}</span>${m.exposed ? ' <span class="tag coral">SNITCH</span>' : ""}${tag ? ` <span class="tag ${tag === "retiring" ? "gold" : "coral"}">${esc(tag)}</span>` : ""}<h3>${esc(m.name)}</h3></div></div><div class="stat-row"><span><b>${m.skill}</b>skill</span><span><b>${Math.floor(m.loyalty)}</b>loyalty</span><span><b>${money(query("rules.crew.wage_at", m.id, v.you.pay))}</b>a day, ${esc(v.you.pay)}</span></div><div class="meter teal"><span style="width:${pct(m.loyalty)}%"></span></div><p><small class="${m.loyalty < crew.lineOf(m, tun, flip) ? "danger-text" : "subtle-text"}">${esc(crew.linesLine(m, tun, flip))}</small><br>${m.trait ? `<span class="tag">${esc(m.trait)}</span><small> ${esc(engineInfo.traits[m.trait] || "")}</small> ` : ""}${m.captain ? `<b>Captain of ${esc(m.captain)}</b> · Budget ${money(m.budget)}<br>` : ""}${status}${m.exposed ? '<br><span class="danger-text">Talking to the police.</span>' : ""}</p><div class="card-actions">${["runner", "enforcer"].includes(m.role) ? btn("Assign corner", "assign-person", m.id, "small") : ""}${m.role === "lieutenant" ? btn(m.city ? "Change city" : "Run a city", "lieutenant-city", m.id, "small") : ""}${m.jailed && !m.bailed ? btn(`Bail · ${money(query("rules.crew.bail_cost", m.id))} clean`, "bail", m.id, "small", !!v.over) : ""}${btn(`Pay off · ${money(query("rules.crew.payoff_cost", m.id))}`, "bonus", m.id, "small", !!v.over)}${btn("Captaincy", "captain", m.id, "small subtle")}${btn("Details", "crew-detail", m.id, "small subtle")}</div></article>`;
}
// crewSummary is the CREW block under the cards (the TUI pane's CREW).
function crewSummary() {
  if (!v.crew.length) return "";
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
    `<span class="tag">${esc(m.role)}</span>${m.exposed ? ' <span class="tag coral">SNITCH</span>' : ""}<h2>${esc(m.name)}</h2><p>Skill ${m.skill} · Hired Day ${m.hired}${age ? "<br>" + esc(age) : ""}<br>Loyalty ${Math.floor(m.loyalty)} · <span class="subtle-text">${esc(crew.linesLine(m, tun, flip))}</span><br>Wage ${money(query("rules.crew.wage_at", m.id, v.you.pay))}/day ${esc(v.you.pay)} · <span class="subtle-text">${esc(wages)}</span><br>Carry capacity ${m.carry}<br><span class="${where.warn ? "danger-text" : ""}">${esc(where.text[0].toUpperCase() + where.text.slice(1))}</span>${temper ? "<br>" + esc(temper) : ""}</p>${m.exposed ? '<p class="danger-text"><b>SNITCH</b>: talking to the police.</p>' : ""}<p class="subtle-text">${esc(fire)}</p><div class="card-actions">${m.jailed && !m.bailed ? btn(`Bail · ${money(query("rules.crew.bail_cost", m.id))} clean`, "bail", id, "small", !!v.over) : ""}${btn(`Pay off · ${money(query("rules.crew.payoff_cost", m.id))}`, "bonus", id, "small", !!v.over)}${btn("Fire", "fire", id, "subtle", !!v.over)}</div>`,
  );
}
function renderEmpire() {
  const offers = query("rules.laundering.offers");
  return `<div class="row"><h3>Your businesses</h3><select aria-label="Laundering approach" data-change="launder">${["careful", "normal", "greedy"].map((x) => `<option ${x === v.you.launder ? "selected" : ""}>${x}</option>`).join("")}</select></div><div class="cards" style="margin-top:15px">${offers
    .map((f) => {
      const own = v.fronts.find((x) => x.id === f.ID),
        level = own ? query("rules.laundering.levels", own.id, 1) : null;
      return `<article class="card"><div class="card-top">${icon("shop")}<span class="tag ${own ? "" : "gold"}">${own ? "LEVEL " + own.level : "BUSINESS OPPORTUNITY"}</span></div><h3>${esc(f.Name)}</h3><p class="front-role">${esc(engineInfo.frontRoles[f.ID] || "")}</p><p>Base capacity ${money(f.Throughput)} / day<br>Base upkeep ${money(f.Upkeep)} clean / day</p>${own ? `<span class="subtle-text">${own.frozen ? "Temporarily frozen" : "Open for business"} · ${money(own.washed)} washed</span><div class="card-actions">${btn(level.Levels > 0 ? "Invest " + money(level.Cost) : "Maximum level", "invest", own.id, "small", level.Levels === 0 || v.you.clean_cash < level.Cost || !!v.over)}</div>` : `<div class="row"><strong class="price">${money(f.Cost)}</strong>${btn("Buy business", "buy-front", f.ID, "small", v.you.dirty_cash < f.Cost || v.you.peak_cash < f.UnlockCash || !!v.over)}</div>`}</article>`;
    })
    .join(
      "",
    )}</div>${lanesHTML()}${trophiesHTML()}<div class="row section-gap"><h3>Make your next investment</h3>${btn("Properties & assets", "properties", "", "small subtle")}</div><div class="filters">${["all", "operations", "security", "legal", "crew", "laundering", "street", "logistics"].map((b) => `<button data-branch="${b}" class="${branch === b ? "active" : ""}">${b}</button>`).join("")}</div><div class="cards">${v.upgrades
    .filter((u) => branch === "all" || u.branch === branch)
    .map(
      (u) =>
        `<article class="card"><div class="card-top"><span class="tag ${u.state === "owned" ? "" : "gold"}">${esc(u.branch)}</span><small>${esc(u.state)}</small></div><h3>${esc(u.name)}</h3><p>${esc(u.desc)}</p>${u.requires.length ? `<p>Requires: ${u.requires.map(esc).join(", ")}</p>` : ""}<div class="row"><span><strong>${money(u.cost)}</strong> <small>${u.clean ? "clean" : "dirty"}</small></span>${btn(u.state === "owned" ? "Owned ✓" : "Buy upgrade", "upgrade", u.id, "small", u.state !== "available" || v.you[u.clean ? "clean_cash" : "dirty_cash"] < u.cost || !!v.over)}</div></article>`,
    )
    .join("")}</div>`;
}
function renderRivals() {
  return `${v.alerts
    .filter((a) => a.kind === "scouts")
    .map(
      (a) =>
        `<div class="paper"><p>${esc(alertText(v, a))}</p>${btn("Confront scouts", "hit-scouts", scoutFaction(a), "small", !!v.over)}</div>`,
    )
    .join(
      "",
    )}<div class="tip-box">Follow your Kingpin plan in the Ledger for the current requirements. Routed, insolvent factions can now scatter. Profitable cities can attract new factions.</div><div class="cards">${v.factions.map((f) => `<article class="card" id="faction-${f.id}"><div class="card-top">${icon("rival")}<span class="tag ${f.alive ? "coral" : ""}">${f.alive ? "ACTIVE" : f.arrived ? "GONE" : "NOT YET ARRIVED"}</span></div><h3>${esc(f.leader)}</h3><p>${f.personality === "?" ? "Personality unknown" : esc(f.personality)}${f.deals?.length ? " · " + f.deals.map(esc).join(", ") : ""}</p><div class="stat-row"><span><b>${f.corners}</b>corners</span><span><b>${Math.round(f.trust)}</b>trust</span><span><b>${Math.round(f.war)}</b>war</span></div>${f.books ? `<p>Last scouted on Day ${f.books.day}<br>${money(f.books.cash)} cash · ${f.books.muscle} muscle</p>` : ""}<div class="card-actions">${btn("Scout", "scout", f.id, "small", !f.alive || !!v.over)}${btn("Talk terms", "diplomacy", f.id, "small", !f.alive || !!v.over)}</div></article>`).join("")}</div><h3 class="section-gap">Offers on the table</h3>${v.offers.length ? v.offers.map((o) => `<div class="paper row"><div><h3>${esc(v.factions.find((f) => f.id === o.faction)?.leader || o.faction)} offers ${esc(dealWords(o.kind, o.terms))}</h3><p class="subtle-text">Expires Day ${o.expires}</p></div><div>${btn("Accept", "accept", o.id, "small")}${btn("Decline", "decline", o.id, "small subtle")}</div></div>`).join("") : '<div class="empty">No offers today. The table is quiet.</div>'}`;
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
  const off = query("rules.laundering.offshore"),
    canRetire = query("rules.laundering.can_retire"),
    canStraight = query("rules.laundering.can_go_straight"),
    canVanish = v.you.upgrades.includes("identity"),
    canCrown = v.ambitions.some((a) => a.ending === "kingpin" && a.done),
    terms = endingTerms(canStraight);
  return `<div class="cards"><article class="card"><div class="eyebrow">WHAT YOU’VE BUILT</div><h3>Total net worth</h3><div class="cash-total">${money(v.you.net_worth)}</div><p>Cash, offshore funds, inventory and property. Not all of it is spendable.</p><div class="row"><span>Business income, net of upkeep</span><b>${money(query("rules.laundering.legit_income"))}/day</b></div></article><article class="card"><div class="eyebrow">A FUTURE SOMEWHERE ELSE</div><h3>The offshore account</h3><div class="cash-total">${money(v.you.offshore)}</div><p>Transfers cost ${Math.round(off.Fee * 100)}%. Moving more than ${money(off.Lot)} in a day adds evidence.</p><label class="row"><input id="reserve-amount" aria-label="Amount to transfer" type="number" min="1" step="100" value="${Math.min(off.Lot, v.you.clean_cash) || 1000}">${btn("Transfer", "reserve", "", "small", !v.you.clean_cash || !!v.over)}</label></article><article class="card"><div class="eyebrow">CASH FOR THE STREET</div><h3>Cash out</h3><div class="cash-total">${money(v.you.clean_cash)}</div><p>Stock and wages are paid in dirty cash. Drawing clean money back costs ${money(query("rules.laundering.cash_out_fee", 100000))} per $100,000, and a dirty pile past your cover draws heat.</p><label class="row"><input id="cashout-amount" aria-label="Clean cash to cash out" type="number" min="1" step="100" value="${Math.min(v.you.clean_cash, 10000) || 1000}">${btn("Cash out", "cash-out", "", "small", !v.you.clean_cash || !!v.over)}</label></article></div><div class="tip-box">Your final score is offshore money divided by one plus the run’s body count. A large empire and a high score are different goals.</div><h3 class="section-gap">Give something back</h3><div class="paper" id="race"><p class="subtle-text">Community funding builds goodwill using clean cash.</p><div class="card-actions"><input id="fund-amount" type="number" aria-label="Community funding amount" min="1" value="2000">${btn("Fund " + esc(city().name), "fund", city().id, "small", !v.you.clean_cash || !!v.over)}</div></div>${ambitionsHTML()}<h3 class="section-gap">Choose your ending</h3><div class="cards"><article class="card"><span class="tag">THE QUIET EXIT</span><h3>Retired Clean</h3><p>${money(off.RetireCash)} offshore and ${plural(off.RetireDays, "day")} quiet. You have ${plural(v.you.quiet_days, "quiet day")}.</p>${canRetire ? "" : `<p class="subtle-text">${esc(short("retired"))}</p>`}${btn(canRetire ? "Retire now" : "Not ready yet", "retire", "", "small", !canRetire || !!v.over)}</article><article class="card"><span class="tag gold">THE CITY IS YOURS</span><h3>Kingpin</h3><p>${esc(terms.kingpin)}</p>${canCrown ? "" : `<p class="subtle-text">${esc(short("kingpin"))}</p>`}${btn(canCrown ? "Take the crown" : "No reign yet", "crown", "", "small", !canCrown || !!v.over)}</article><article class="card"><span class="tag">A NEW CHAPTER</span><h3>Vanished</h3><p>${esc(terms.vanished)}</p>${canVanish ? "" : `<p class="subtle-text">${esc(short("vanished"))}</p>`}${btn(canVanish ? "Vanish now" : "An identity is required", "vanish", "", "small", !canVanish || !!v.over)}</article><article class="card"><span class="tag">A DIFFERENT KIND OF EMPIRE</span><h3>A Businessman</h3><p>${esc(terms.businessman)}</p>${canStraight ? "" : `<p class="subtle-text">${esc(short("businessman"))}</p>`}${btn(canStraight ? "Go straight" : "Not yet", "go_straight", "", "small", !canStraight || !!v.over)}</article></div>`;
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
  return `<div class="paper"><div class="row"><h2>The morning edition</h2><span class="tag">Day ${v.report.day}</span></div>${reportHTML(v.report) || '<div class="empty">A blank page. Your first report arrives tomorrow.</div>'}</div>${
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
    `<div class="eyebrow">ON THE CORNER</div><h2>${esc(c.name)}</h2><p>Held by ${esc(owner)} · Demand multiplier ${c.demand.toFixed(1)}×</p>${c.owner !== "rival" ? `<label>Who should work here?<select id="post-member">${postable.map((m) => `<option value="${m.id}">${esc(m.name)} · ${m.role}</option>`).join("")}</select></label>${btn("Assign to this corner", "post", c.id, "primary")}${c.owner === "player" ? btn("Abandon corner", "abandon", c.id, "subtle") : ""}` : `<div class="tip-box">Contesting territory can increase heat, evidence, and retaliation. Check your crew before committing.</div><label>Force<select id="force-dial"><option>warn</option><option>push</option><option>hit</option></select></label>${btn("Send enforcers", "strike", c.id, "primary", !v.crew.some((m) => m.role === "enforcer"))}${btn("Tip the police", "tip", c.id, "subtle")}<label>Price competition<select id="undercut-dial"><option>quiet</option><option>normal</option><option>aggressive</option></select></label>${btn("Undercut tonight", "undercut", c.id, "subtle")}`}`,
  );
}
function ending() {
  const names = {
      kingpin: "Kingpin",
      retired: "Retired Clean",
      businessman: "A Businessman",
      vanished: "Vanished",
      broke: "Broke",
      indicted: "Indicted",
      arrested: "Arrested",
      betrayed: "Betrayed",
      taken_out: "Taken Out",
    },
    win = ["kingpin", "retired", "businessman", "vanished"].includes(
      v.over?.cause,
    );
  modal(
    `<div class="ending"><div class="crown">${win ? "♛" : "✦"}</div><div class="eyebrow">${win ? "AN ENDING EARNED" : "EVERY CITY HAS ITS CONSEQUENCES"}</div><h2>${names[v.over.cause] || esc(v.over.cause)}</h2><p>Your story ends on Day ${v.over.day}.</p><div class="cash-total">${money(v.you.net_worth)}</div><p>Net worth · ${money(v.you.offshore)} offshore</p><div class="card-actions" style="justify-content:center">${btn("Export this story", "export", "", "primary")}${btn("Start another story", "new", "", "subtle")}</div></div>`,
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
          `<div class="report-section"><div class="row"><h3>${esc(r.name)}</h3><span class="tag">${esc(r.mode)}</span></div><p>${esc(r.from)} → ${esc(r.to)} · ${r.risk_known ? "Risk " + Math.round(r.risk * 100) + "%" : "Risk unknown"}</p><select id="route-${r.id}">${["off", "slow", "normal", "fast"].map((x) => `<option ${x === r.dial ? "selected" : ""}>${x}</option>`).join("")}</select>${btn("Set pace", "route", r.id, "small")}<div class="card-actions"><select id="product-${r.id}">${city()
            .products.map(
              (p) => `<option value="${p.id}">${esc(p.name)}</option>`,
            )
            .join(
              "",
            )}</select><input style="width:90px" id="units-${r.id}" type="number" min="0" value="10" aria-label="Target units">${btn("Set target", "route-target", r.id, "small")}</div></div>`,
      )
      .join(
        "",
      )}<h3>In transit</h3>${v.shipments.length ? v.shipments.map((x) => `<p>${esc(x.product)} · ${x.units} units</p>`).join("") : "<p>No shipments on the road.</p>"}`,
  );
}
function properties() {
  const houses = query("house_offers"),
    assets = query("asset_offers");
  modal(
    `<div class="eyebrow">PROPERTY LEDGER</div><h2>A place of your own.</h2><h3>Houses</h3>${houses
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
      )}<h3>Assets</h3>${assets.map((a) => `<div class="report-section"><b>${esc(a.Name)}</b><p>${money(a.Cost)} clean · ${esc(a.City)}</p>${btn("Purchase", "asset", a.ID, "small")}</div>`).join("")}`,
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
  if (alignedAction(a, id)) return;
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
      case "travel":
        act("travel", [id], "Arrived in " + id);
        selectedCity = id;
        render();
        break;
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
      case "strike":
        act("send_enforcers", [id, $("#force-dial").value], "Action queued", {
          order: { key: "strike", text: "Enforcers sent to " + id },
        });
        break;
      case "tip":
        act("tip", [id], "Tip queued", {
          order: { key: "tip", text: "Police tip: " + id },
        });
        break;
      case "undercut":
        act(
          "undercut",
          [id, $("#undercut-dial").value],
          "Price competition queued",
          { order: { key: "cut-" + id, text: "Undercut " + id } },
        );
        break;
      case "buy": {
        const sup = v.connects.find(
          (x) =>
            x.city === city().id &&
            x.open &&
            !x.wholesale &&
            x.prices[id] != null,
        );
        const qty = integer("#qty-" + id),
          room = session.maxBuy(sup.id, id);
        if (qty > room.max) {
          notify(
            `You can buy at most ${room.max} units with your cash and stash space.`,
            true,
          );
          $("#qty-" + id).value = Math.max(1, room.max);
          break;
        }
        confirm(
          "Buy this stock?",
          `${qty} ${id}. Stash after purchase: ${room.held + qty} / ${room.capacity}.`,
          () => act("buy", [sup.id, id, qty, false], "Stock purchased"),
        );
        break;
      }
      case "sell": {
        const n = integer("#qty-" + id);
        act(
          "place_sell",
          [city().id, id, n, $("#sale-dial").value],
          "Sale queued for tonight",
          {
            order: { key: "sell-" + id, product: id, text: `Sell ${n} ${id}` },
          },
        );
        break;
      }
      case "sell-all": {
        const dial = $("#sale-dial").value;
        for (const [p, n] of Object.entries(v.you.stock[city().id] || {}))
          if (n)
            act("place_sell", [city().id, p, n, dial], null, {
              order: { key: "sell-" + p, product: p, text: `Sell ${n} ${p}` },
            });
        notify("Held stock queued for sale");
        break;
      }
      case "cancel-sale":
        act("cancel_sell", [city().id, id], "Sale cancelled");
        orders = orders.filter((o) => o.product !== id);
        persist();
        render();
        break;
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
      case "buy-front":
        act("buy_front", [id], "Business purchased");
        break;
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
      case "diplomacy":
        modal(
          `<div class="eyebrow">A SEAT AT THE TABLE</div><h2>Make an offer.</h2><label>Agreement<select id="deal-kind"><option value="truce">Truce · duration in days</option><option value="tribute">Tribute · dirty cash paid per day</option></select></label><label>Days or daily payment<input id="deal-value" type="number" min="1" value="14"></label><p>The rival can accept or refuse tonight.</p>${btn("Send proposal", "propose", id, "primary")}`,
        );
        break;
      case "propose": {
        const kind = $("#deal-kind").value,
          n = integer("#deal-value");
        act(
          "propose_to",
          [id, kind, kind === "truce" ? { days: n } : { per_day: n }],
          "Proposal sent",
          { order: { key: "proposal", text: kind + " proposal" } },
        );
        break;
      }
      case "accept":
        act("accept", [Number(id)], "Offer accepted");
        break;
      case "decline":
        act("decline", [Number(id)], "Offer declined");
        break;
      case "cash-out": {
        const n = integer("#cashout-amount");
        act("cash_out", [n], `Cashed out ${money(n)} clean, less the banker's fee`);
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
      case "reserve":
        act("reserve", [integer("#reserve-amount")], "Transfer queued", {
          order: { key: "reserve", text: "Offshore transfer" },
        });
        break;
      case "fund":
        act("fund", [id, integer("#fund-amount")], "Funding queued", {
          order: { key: "fund-" + id, text: "Community funding" },
        });
        break;
      case "retire":
      case "crown":
      case "go_straight":
      case "vanish":
        confirm(
          { retire: "Retire?", crown: "Take the crown?", go_straight: "Go straight?", vanish: "Vanish?" }[a],
          exitConfirm(a),
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
          { order: { key: "delivery-" + id, text: "Private buyer delivery" } },
        );
        break;
      case "routes":
        routes();
        break;
      case "route":
        act("set_route", [id, $("#route-" + id).value], "Route pace changed");
        routes();
        break;
      case "route-target":
        act(
          "set_route_target",
          [id, $("#product-" + id).value, integer("#units-" + id)],
          "Route target set",
        );
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
$("#lie-low").onclick = () =>
  act(
    "set_lie_low",
    [!v.you.lie_low],
    v.you.lie_low ? "Back to business" : "Lying low tonight",
  );
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
        orders = saved.orders || [];
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
function ambitionsHTML() {
  return `<h3 class="section-gap">Your ambitions</h3><div class="cards">${v.ambitions
    .map(
      (a) =>
        `<article class="card"><div class="row"><h3>${esc(a.name)}</h3><span class="tag">${a.done ? "READY" : Math.floor(a.progress * 100) + "%"}</span></div><div class="meter teal"><span style="width:${a.progress * 100}%"></span></div>${a.steps
          .map((step) => {
            const fmt = (n) =>
              ["cash", "clean", "dirty", "income"].includes(step.unit)
                ? money(n)
                : Math.round(n).toLocaleString();
            return `<p class="plan-step ${step.done ? "met" : ""}">${step.done ? "✓" : "○"} ${esc(step.label)}<small>${fmt(step.have)} / ${fmt(step.need)} ${["cash", "clean", "dirty", "income"].includes(step.unit) ? "" : esc(step.unit)}</small></p>`;
          })
          .join(
            "",
          )}${btn(a.pinned ? "Unpin plan" : "Follow this plan", "pin-plan", a.pinned ? "" : a.id, "small", !!v.over)}</article>`,
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
// step is an ending's plan's step by id, for the terms it reads
// against (the ambitions, #347: the endings' own thresholds).
function step(ending, id) {
  return v.ambitions.find((a) => a.ending === ending)?.steps.find((s) => s.id === id);
}
// endingTerms are the ending cards' terms in the walk-away's words
// (ui/exit.go exitRows, #493, #498, #529), every number read off the
// engine: the crown's share and days and the businessman's nights off
// their plans' steps, the vanish's chain off its plan's costs.
function endingTerms(canStraight) {
  const need = (e, id) => step(e, id)?.need || 0,
    cost = (e, id) => {
      const s = step(e, id);
      return s ? `${money(s.need)} ${s.unit === "dirty" ? "dirty" : "clean"}` : "";
    };
  return {
    kingpin: `${need("kingpin", "share")} of home's corners held and every crew gone or paying, ${plural(need("kingpin", "streak"), "day")} running.`,
    vanished: `A new identity from the tree (${cost("vanished", "identity")}), after the lawyer on call (${cost("vanished", "lawyer")}) and on retainer (${cost("vanished", "retainer")}).`,
    businessman: canStraight
      ? `The fronts at ${money(query("rules.laundering.legit_income"))} a day.`
      : `The fronts out-earn the street's average night and goodwill tops pressure at home, ${plural(need("businessman", "streak"), "night")} running.`,
  };
}
// exitConfirm is a way out's confirmation in its own words (ui/exit.go
// viewExit, #498), then what the run leaves behind.
function exitConfirm(a) {
  const off = money(v.you.offshore),
    ends = `the run ends now, on day ${v.day}.`,
    stock = Object.values(v.you.stock || {}).reduce((n, c) => n + Object.values(c).reduce((m, x) => m + x, 0), 0),
    left = `Left behind: ${money(v.you.dirty_cash)} dirty, ${money(v.you.clean_cash)} clean, ${plural(stock, "unit")} in stock, ${plural(v.crew.length, "member")}.`;
  const words = {
    retire: `Retire on ${off} offshore, ${plural(v.you.quiet_days, "day")} quiet. Nobody comes looking. The run ends now, on day ${v.day}.`,
    crown: `Take the crown with ${off} offshore. The city stays yours in the epilogue; ${ends}`,
    go_straight: `Go straight on ${plural(v.fronts.length, "front")}: the fronts at ${money(query("rules.laundering.legit_income"))} a day, the street given up, ${off} offshore. The DA's file goes to the archive; ${ends}`,
    vanish: `Vanish on the new identity with ${off} offshore. The DA keeps looking; the papers are good. The run ends now, on day ${v.day}.`,
  };
  return `${words[a]} ${left}`;
}
// short is what an ending's plan still needs, in its steps' own words
// (the ambitions, #347: the endings' own terms), or "" when it is open.
function short(ending) {
  const a = v.ambitions.find((x) => x.ending === ending);
  if (!a) return "";
  const fmt = (s, n) => (["cash", "clean", "dirty", "income"].includes(s.unit) ? money(n) : Math.round(n).toLocaleString());
  return a.done ? "" : a.steps.filter((s) => !s.done).map((s) => `${s.label} ${fmt(s, s.have)} of ${fmt(s, s.need)}`).join("; ");
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
  else if (l.open === "properties") properties();
  else if (l.open === "police") $("#police-risk").open = true;
  else if (l.open === "investigate" && v.crew.length && !v.over) action("investigate");
  const picked = l.select && document.getElementById(l.select);
  if (picked) {
    picked.classList.add("picked");
    setTimeout(() => picked.classList.remove("picked"), 2600);
    picked.scrollIntoView({ behavior: "smooth", block: "center" });
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
  return `${city ? "Runs " + esc(city.name) : "No city yet"}<br>${tt ? `<span class="tag">${esc(m.personality)}</span> <small>${esc(temperLine(tt))}</small>` : `<small>Temper shows after ${lt.RevealDays} days running a city</small>`}`;
}
// lieutenantDialog is the city picker with the role in words (#455):
// what they do, the cut and the slots, the tempers and the risk.
function lieutenantDialog(id) {
  const m = v.crew.find((x) => x.id === Number(id)),
    lt = query("rules.crew.lieutenancy");
  modal(
    `<div class="eyebrow">SOMEONE TO RUN A CITY</div><h2>${esc(m.name)} · Lieutenant</h2>${roleLines(lt)
      .map((l, i) => `<p${i === 3 ? ' class="tip-box"' : ""}>${esc(l)}</p>`)
      .join("")}<label>City<select id="lieutenant-city">${v.cities.map((c) => `<option value="${c.id}" ${c.id === m.city ? "selected" : ""}>${esc(c.name)}</option>`).join("")}${m.city ? '<option value="">Nobody\'s: stand them down</option>' : ""}</select></label>${btn(m.city ? "Change city" : "Run this city", "assign-city", m.id, "primary", !!v.over)}`,
  );
}
function captainDialog(id) {
  const m = v.crew.find((x) => x.id === Number(id)),
    cfg = query("rules.crew.captaincy"),
    eligible = query("rules.crew.can_captain", Number(id));
  modal(
    `<div class="eyebrow">SOMEONE YOU CAN TRUST</div><h2>${esc(m.name)} · Captaincy</h2><p>${m.trait ? "Veteran trait: " + esc(m.trait) + " — " + esc(engineInfo.traits[m.trait] || "") + ". " : ""}A captain assigns idle runners, recalls suspected skimmers and pays loyalty bonuses within your budget.</p><p>Requires ${cfg.Days} days of service and ${cfg.Loyalty} loyalty. Takes ${Math.round(cfg.Cut * 100)}% of the city's revenue.</p>${m.captain ? `<p>Currently captain of ${esc(m.captain)} · ${money(m.budget)} nightly bonus budget.</p>${btn("Remove captaincy", "drop-captain", m.id, "subtle")}` : ""}<label>City<select id="captain-city">${v.cities.map((c) => `<option value="${c.id}">${esc(c.name)}</option>`).join("")}</select></label><label>Nightly bonus budget<input type="number" id="captain-budget" value="${m.budget || cfg.Budgets?.[0] || 0}" min="0" step="1000"></label>${btn("Appoint captain", "name-captain", m.id, "primary", !eligible || !!v.over)}${!eligible ? "<p>This member is not currently eligible.</p>" : ""}`,
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
        const sup = v.connects.find(
          (x) =>
            x.city === v.you.city &&
            x.open &&
            !x.wholesale &&
            x.prices[id] != null,
        );
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
      case "preset-review": {
        const r = session.presetDiff($("#preset-id").value);
        modal(
          `<div class="eyebrow">REVIEW THE ROUTINE</div><h2>${esc(r.preset.name)}</h2><p>${esc(r.preset.blurb)}</p>${r.changes.map((c) => `<p><b>${esc([c.setting, c.city, c.product, c.route].filter(Boolean).join(" "))}</b><br>${esc(c.from)} → ${esc(c.to)}${c.dropped ? ` · drops ${c.dropped} orders tonight` : ""}</p>`).join("") || "<p>No settings would change.</p>"}<p>${r.same} settings stay as they are.</p>${r.refused.map((x) => `<p class="danger-text">Not changed: ${esc(x.why)}</p>`).join("")}${btn("Apply preset", "preset-apply", r.preset.id, "primary", !r.changes.length)}`,
        );
        break;
      }
      case "preset-apply": {
        const r = act("apply_preset", [id], "Routine updated");
        if (r) {
          if (v.you.lie_low) orders = orders.filter((o) => !o.product);
          persist();
          render();
          if (r.refused.length)
            notify(r.refused.map((x) => x.why).join("; "), true);
        }
        break;
      }
      case "lieutenant-city":
        lieutenantDialog(id);
        break;
      case "assign-city": {
        const city = $("#lieutenant-city").value;
        const m = v.crew.find((x) => x.id === Number(id));
        if (city) act("assign", [Number(id), city], `${m.name} runs ${v.cities.find((c) => c.id === city).name} from tonight`);
        else act("unassign", [Number(id)], `${m.name} runs nothing now`);
        break;
      }
      case "captain":
        captainDialog(id);
        break;
      case "name-captain":
        act(
          "name_captain",
          [
            Number(id),
            $("#captain-city").value,
            nonnegative("#captain-budget"),
          ],
          "Captain appointed",
        );
        break;
      case "drop-captain":
        act("drop_captain", [Number(id)], "Captaincy removed");
        break;
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

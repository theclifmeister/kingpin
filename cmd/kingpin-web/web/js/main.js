// The page (#328): loads the engine as WebAssembly (#327), opens a
// session and plays it through the protocol alone. The map is the
// scene's; the panel, the card and the ending are the DOM's. Every move
// is a protocol call; a move the game refuses says why on the toast.
import { alertPanel, alertText } from "./alerts.js";
import { autoDay } from "./autoplay.js";
import { Scene } from "./scene.js";
import { Session, streetConnect } from "./session.js";

const $ = (id) => document.getElementById(id);
const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n)).toLocaleString("en-US");
const SAVE_KEY = "kingpin.save";

let session = null;
let scene = null;
let auto = null; // the autopilot's timer
let endingTimer = null;

async function boot() {
  let kingpin;
  try {
    const go = new Go();
    const bytes = await (await fetch("kingpin.wasm")).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    go.run(instance);
    kingpin = globalThis.kingpin;
    session = new Session(kingpin);
  } catch (e) {
    $("fatal-text").textContent = e.message;
    $("fatal").hidden = false;
    return;
  }
  scene = new Scene($("map"));
  addEventListener("resize", () => {
    scene.resize();
    render();
  });
  wire();
  newRun(seedFromURL());
}

function seedFromURL() {
  const s = new URLSearchParams(location.search).get("seed");
  return s && /^\d+$/.test(s) ? Number(s) : 1 + Math.floor(Math.random() * 1e9);
}

function newRun(seed) {
  stopAuto();
  session.newRun(seed);
  session.take();
  $("seed").textContent = `seed ${seed}`;
  render();
}

// act makes one move and redraws; a refusal is the game's words.
function act(f, okText) {
  try {
    f();
    toast(okText || "", true);
  } catch (e) {
    if (!e.refused) throw e;
    toast(e.message);
  }
  session.refresh();
  render();
}

// endDays ends n days (fast-forward past one), then plays the night's
// cues over the map.
function endDays(n) {
  if (n === 1) session.endDay();
  else {
    const r = session.fastForward(n);
    if (r.stop && r.stop !== "cap") toast(`stopped after ${r.ran} days: ${r.alert ? alertText(session.view, r.alert) : r.event || r.stop}`, true);
  }
  afterNight();
}

function afterNight() {
  const cues = session.take().filter((e) => e.cue).map((e) => e.cue);
  session.refresh();
  render();
  scene.play(cues);
}

function toggleAuto() {
  if (auto) return stopAuto();
  $("auto").classList.add("on");
  auto = setInterval(() => {
    if (!session.view || session.view.over) return stopAuto();
    if (scene.busy() && Math.random() < 0.6) return; // let the night play out, mostly
    autoDay(session);
    afterNight();
  }, 700);
}

function stopAuto() {
  clearInterval(auto);
  auto = null;
  $("auto").classList.remove("on");
}

function toast(text, ok = false) {
  const t = $("toast");
  t.textContent = text;
  t.className = ok ? "ok" : "";
}

function wire() {
  $("end").onclick = () => endDays(1);
  $("week").onclick = () => endDays(7);
  $("auto").onclick = toggleAuto;
  $("new").onclick = () => newRun(1 + Math.floor(Math.random() * 1e9));
  $("again").onclick = () => newRun(1 + Math.floor(Math.random() * 1e9));
  $("save").onclick = () => {
    try {
      localStorage.setItem(SAVE_KEY, session.exportSave());
      toast(`saved day ${session.view.day} in this browser`, true);
    } catch (e) {
      toast(e.message);
    }
  };
  $("load").onclick = () => {
    const b = localStorage.getItem(SAVE_KEY);
    if (!b) return toast("nothing saved in this browser");
    stopAuto();
    act(() => session.importSave(b), "loaded");
  };
  addEventListener("keydown", (e) => {
    if (e.target.tagName === "INPUT" || e.target.tagName === "SELECT") return;
    if (e.key === " " && !$("end").disabled) {
      e.preventDefault();
      endDays(1);
    } else if (e.key === "a") toggleAuto();
  });
}

function render() {
  const v = session.view;
  if (!v) return;
  scene.setView(v);
  $("day").textContent = `day ${v.day}`;
  $("tier").textContent = v.you.tier_name;
  $("dirty").textContent = money(v.you.dirty_cash);
  $("clean").textContent = money(v.you.clean_cash);
  $("net").textContent = money(v.you.net_worth);
  $("evidence").textContent = v.you.evidence;

  const city = v.cities.find((c) => c.id === v.you.city);
  $("here").textContent = city ? city.name : v.you.city;
  const k = streetConnect(v, v.you.city);
  const held = (v.you.stock || {})[v.you.city] || {};
  const rows = (city ? city.products : []).map((p) => {
    const tr = document.createElement("tr");
    const offer = k && k.prices[p.id];
    for (const text of [p.name, money(p.price), offer ? money(offer) : "–", String(held[p.id] || 0), ""]) {
      tr.append(Object.assign(document.createElement("td"), { textContent: text }));
    }
    const qty = document.createElement("input");
    qty.type = "number";
    qty.min = "1";
    qty.value = offer ? String(Math.max(1, Math.floor((v.you.dirty_cash * 0.3) / offer))) : "";
    const buy = button("Buy", !offer, () => act(() => session.buy(k.id, p.id, Number(qty.value)), `bought ${qty.value} ${p.name}`));
    const sell = button("Sell", !held[p.id], () =>
      act(() => session.sell(v.you.city, p.id, held[p.id], $("dial").value), `${held[p.id]} ${p.name} on the street tonight`),
    );
    tr.lastChild.append(qty, buy, sell);
    return tr;
  });
  $("market").tBodies[0].replaceChildren(...rows);

  $("travel").replaceChildren(
    ...v.cities.map((c) => button(c.name, c.id === v.you.city, () => act(() => session.travel(c.id), `on the road to ${c.name}`))),
  );

  const pool = (v.pool || []).map((c) => {
    const tr = document.createElement("tr");
    for (const text of [c.name, c.role, String(c.skill), money(c.wage), money(c.fee)]) {
      tr.append(Object.assign(document.createElement("td"), { textContent: text }));
    }
    const td = document.createElement("td");
    td.append(button("Hire", c.fee > v.you.dirty_cash, () => act(() => session.hire(c.id), `${c.name} is on the payroll`)));
    tr.append(td);
    return tr;
  });
  if (!pool.length) {
    const tr = document.createElement("tr");
    tr.append(Object.assign(document.createElement("td"), { className: "dim", colSpan: 6, textContent: "Nobody right now." }));
    pool.push(tr);
  }
  $("pool").tBodies[0].replaceChildren(...pool);

  // What needs you (#352): each alert a button to the panel that
  // answers it where the page has one, else its words alone.
  const alerts = (v.alerts || []).map((a) => {
    const li = document.createElement("li");
    const panel = alertPanel(a);
    if (panel) li.append(button(alertText(v, a), false, () => showPanel(panel)));
    else li.textContent = alertText(v, a);
    return li;
  });
  if (!alerts.length) alerts.push(Object.assign(document.createElement("li"), { className: "dim", textContent: "Nobody is looking at you. Yet." }));
  $("alerts").replaceChildren(...alerts);

  const sections = ["incident", "unlocked", "tier", "prices", "sales", "heat", "crew", "territory", "shipments", "law", "intel", "money", "upgrades", "news"];
  const parts = [];
  for (const s of sections) {
    const lines = (v.report && v.report[s]) || [];
    if (!lines.length) continue;
    const h = document.createElement("h3");
    h.textContent = s;
    parts.push(h, ...lines.map((l) => Object.assign(document.createElement("p"), { textContent: l })));
  }
  if (!parts.length) parts.push(Object.assign(document.createElement("p"), { className: "dim", textContent: "A quiet morning." }));
  $("report").replaceChildren(...parts);

  $("card").hidden = !v.card;
  if (v.card) {
    $("card-title").textContent = v.card.title;
    $("card-text").textContent = v.card.text;
    $("card-choices").replaceChildren(...v.card.choices.map((c, i) => button(c, false, () => act(() => session.choose(i)))));
  }
  const over = !!v.over;
  // The ending waits for the night's last animation, THE END among them.
  clearTimeout(endingTimer);
  if (!over) $("ending").hidden = true;
  else
    endingTimer = setTimeout(() => {
      $("ending-text").textContent = `Day ${v.over.day}: ${v.over.cause}${v.over.who ? ` (${v.over.who})` : ""}. Net worth ${money(v.you.net_worth)}.`;
      $("ending").hidden = false;
    }, 2400);
  for (const id of ["end", "week"]) $(id).disabled = over || !!v.card;
  $("auto").disabled = over;
}

// showPanel brings the panel an alert names into view and flashes it.
function showPanel(id) {
  const el = $(id);
  el.scrollIntoView({ behavior: "smooth", block: "nearest" });
  el.classList.remove("flash");
  void el.offsetWidth; // restart the animation
  el.classList.add("flash");
}

function button(text, disabled, onclick) {
  const b = document.createElement("button");
  b.textContent = text;
  b.disabled = disabled;
  b.onclick = onclick;
  return b;
}

boot();

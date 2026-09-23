// The page (#328): loads the engine as WebAssembly (#327), opens a
// session and plays it through the protocol alone. The map is the
// scene's; the panel, the card and the ending are the DOM's. Every move
// is a protocol call; a move the game refuses says why on the toast.
import { autoDay } from "./autoplay.js";
import { Scene } from "./scene.js";
import { Session, streetConnect } from "./session.js";

const $ = (id) => document.getElementById(id);
const money = (n) => (n < 0 ? "-$" : "$") + Math.abs(Math.round(n)).toLocaleString("en-US");
const signed = (n) => (n > 0 ? "+" : "") + money(n);

// flowTable is the night's cash flow (#351): the opening, one row a
// category that moved (dirty, clean and both, the big ones marked), and
// the closing. The rows sum to the closing, pile by pile.
function flowTable(f) {
  const t = Object.assign(document.createElement("table"), { className: "flow" });
  const row = (label, d, c, cls, fmt) => {
    const tr = document.createElement("tr");
    if (cls) tr.className = cls;
    for (const text of [label, fmt(d), fmt(c), fmt(d + c)]) tr.append(Object.assign(document.createElement("td"), { textContent: text }));
    return tr;
  };
  const head = document.createElement("tr");
  for (const text of ["", "dirty", "clean", "total"]) head.append(Object.assign(document.createElement("th"), { textContent: text }));
  const rows = [head, row("Opening", f.opening.dirty, f.opening.clean, "total", money)];
  for (const l of f.lines) {
    if (l.dirty || l.clean) rows.push(row(l.label, l.dirty, l.clean, l.big ? (l.dirty + l.clean < 0 ? "big bad" : "big good") : "", signed));
  }
  rows.push(row("Closing", f.closing.dirty, f.closing.clean, "total", money));
  t.append(...rows);
  return t;
}
const SAVE_KEY = "kingpin.save";

let session = null;
let scene = null;
let auto = null; // the autopilot's timer
let endingTimer = null;
let fits = {}; // product id -> the quantity a no-room refusal set (#356), for the next draw

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
    if (r.stop && r.stop !== "cap") toast(`stopped after ${r.ran} days: ${r.alert ? r.alert.kind : r.event || r.stop}`, true);
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
  $("restock").onclick = restock;
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
    // The buy (#356): the field's max is what the connect, the cash and
    // the stash allow (max_buy), and the after line under it is the cash
    // and the room the typed number leaves, red when it is over.
    const room = offer ? session.maxBuy(k.id, p.id) : null;
    const qty = document.createElement("input");
    qty.type = "number";
    qty.min = "1";
    if (room) qty.max = String(room.max);
    qty.value = offer ? String(fits[p.id] || Math.max(1, Math.min(room.max, Math.floor((v.you.dirty_cash * 0.3) / offer)))) : "";
    const after = Object.assign(document.createElement("div"), { className: "dim after" });
    const showAfter = () => {
      if (!room) return;
      const n = Math.max(0, Number(qty.value) || 0);
      const left = v.you.dirty_cash - n * offer;
      const stash = room.held + n;
      after.textContent = `after ${money(left)} dirty · room ${stash}/${room.capacity}`;
      after.classList.toggle("over", left < 0 || stash > room.capacity);
    };
    qty.oninput = showAfter;
    showAfter();
    const most = button("Max", !room || !room.max, () => {
      qty.value = String(room.max);
      showAfter();
    });
    const buy = button("Buy", !offer, () => buyWhatFits(k.id, p, Number(qty.value)));
    const sell = button("Sell", !held[p.id], () =>
      act(() => session.sell(v.you.city, p.id, held[p.id], $("dial").value), `${held[p.id]} ${p.name} on the street tonight`),
    );
    tr.lastChild.append(qty, most, buy, sell, after);
    return tr;
  });
  $("market").tBodies[0].replaceChildren(...rows);
  fits = {};

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

  const sections = ["incident", "unlocked", "tier", "prices", "sales", "heat", "crew", "territory", "shipments", "law", "intel", "money", "upgrades", "news"];
  const parts = [];
  for (const s of sections) {
    if (s === "money" && v.report && v.report.flow && v.report.flow.lines.some((l) => l.dirty || l.clean)) {
      // The cash flow (#351) in place of the flat money lines.
      const h = document.createElement("h3");
      h.textContent = "money";
      parts.push(h, flowTable(v.report.flow));
      continue;
    }
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
    $("card-choices").replaceChildren(...v.card.choices.map((c, i) => choiceButton(c, () => act(() => session.choose(i)))));
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

// choiceButton is a card's choice: its label, and under it what it does
// (#358), a chip a thing in the tone's colour.
function choiceButton(choice, onclick) {
  const b = button(choice.label, false, onclick);
  const chips = document.createElement("span");
  chips.className = "chips";
  chips.append(...choice.preview.map((c) => Object.assign(document.createElement("span"), { className: `chip ${c.tone}`, textContent: c.text })));
  b.append(chips);
  return b;
}

// buyWhatFits buys qty of a product; a refusal for the room (#356)
// sets the field to what fits, so the next click buys it.
function buyWhatFits(supplier, p, qty) {
  try {
    session.buy(supplier, p.id, qty);
    toast(`bought ${qty} ${p.name}`, true);
  } catch (e) {
    if (!e.refused) throw e;
    if (e.free > 0) {
      fits[p.id] = e.free;
      toast(`${e.message}: the quantity is now what fits`);
    } else toast(e.message);
  }
  session.refresh();
  render();
}

// restock tops the stash where you stand up to the days of demand
// (#356): the plan is shown for review, then bought a line at a time.
function restock() {
  const v = session.view;
  const days = Math.max(1, Number($("restock-days").value) || 2);
  const plan = session.restockPlan(v.you.city, days);
  if (!plan.length) return toast("nothing to restock: the stash holds the demand, or nothing more fits");
  const city = v.cities.find((c) => c.id === v.you.city);
  const name = (id) => ((city && city.products.find((p) => p.id === id)) || { name: id }).name;
  const lines = plan.map((l) => `${l.units} ${name(l.product)} for ${money(l.cost)}`);
  const total = plan.reduce((n, l) => n + l.cost, 0);
  if (!confirm(`Restock for ${days} days of demand, ${money(total)}:\n${lines.join("\n")}`)) return;
  act(() => {
    for (const l of plan) session.buy(l.supplier, l.product, l.units);
  }, `restocked: ${lines.join(", ")}`);
}

function button(text, disabled, onclick) {
  const b = document.createElement("button");
  b.textContent = text;
  b.disabled = disabled;
  b.onclick = onclick;
  return b;
}

boot();

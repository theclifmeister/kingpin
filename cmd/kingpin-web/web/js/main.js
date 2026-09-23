import { Session, streetConnect } from "./session.js";
import { autoDay } from "./autoplay.js";
import { readRisk, safeFastForward } from "./risk.js";
import { CityMap } from "./phaser-map.js";
const $ = (id) => document.getElementById(id),
  money = (n) =>
    new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: "USD",
      maximumFractionDigits: 0,
    }).format(n || 0);
const SAVE = "kingpin.save",
  TABS = [
    ["market", "◈", "Market"],
    ["territory", "▦", "Territory"],
    ["crew", "♟", "Crew"],
    ["logistics", "⇄", "Routes"],
    ["property", "▤", "Property"],
    ["contracts", "▧", "Contracts"],
    ["rivals", "♜", "Rivals"],
    ["upgrades", "⌘", "Upgrades"],
    ["law", "⚖", "Law"],
    ["journal", "≡", "Journal"],
  ];
let session,
  map,
  tab = "market",
  cityID,
  cornerID,
  sellDial = "normal",
  actionsDisabled = false,
  autoTimer = null,
  reviewedEnding = false;
const el = (tag, text, cls) => {
  const e = document.createElement(tag);
  if (text !== undefined) e.textContent = text;
  if (cls) e.className = cls;
  return e;
};
function button(text, fn, disabled = false, cls = "") {
  const b = el("button", text, cls);
  b.disabled = disabled;
  b.onclick = fn;
  return b;
}
function status(text, error = false) {
  $("status").textContent = text;
  $("status").style.color = error ? "var(--red)" : "var(--mint)";
}
function select(options, current) {
  const s = el("select");
  for (const [value, label] of options) {
    const o = el("option", label);
    o.value = value;
    s.append(o);
  }
  s.value = current;
  return s;
}
function number(value = 1) {
  const n = el("input");
  n.type = "number";
  n.min = "1";
  n.step = "1";
  n.value = value;
  n.setAttribute("aria-label", "Quantity");
  return n;
}
function field(title, input) {
  const l = el("label", title);
  l.append(input);
  return l;
}
function card(title, desc, tag) {
  const c = el("article", undefined, "card");
  const row = el("div", undefined, "row");
  row.append(el("h3", title));
  if (tag) row.append(el("span", tag, "tag"));
  c.append(row);
  if (desc) c.append(el("p", desc));
  return c;
}
function stats(c, items) {
  const r = el("div", undefined, "stats");
  for (const [label, value] of items) {
    const d = el("div");
    d.append(el("small", label), el("b", value));
    r.append(d);
  }
  c.append(r);
}
function actions(c, ...bs) {
  const a = el("div", undefined, "actions");
  a.append(...bs);
  c.append(a);
}
function empty(text) {
  $("panel").append(el("p", text, "empty"));
}
function heading(text) {
  $("panel").append(el("h3", text, "subheading"));
}
function cmd(text, method, args = [], disabled = false, cls = "") {
  return button(
    text,
    () => act(method, ...args),
    disabled || actionsDisabled,
    cls,
  );
}
function confirmAction(title, text, fn) {
  modal(title, text, [
    button("Cancel", () => closeModal()),
    button(
      "Confirm",
      () => {
        closeModal();
        fn();
      },
      false,
      "primary",
    ),
  ]);
}
function modal(title, text, buttons, locked = false) {
  const d = $("modal");
  d.oncancel = (e) => {
    if (locked) e.preventDefault();
  };
  $("modal-content").replaceChildren(el("h2", title), el("p", text));
  actions($("modal-content"), ...buttons);
  if (!d.open) d.showModal();
}
function closeModal() {
  if ($("modal").open) $("modal").close();
}
function save() {
  try {
    localStorage.setItem(SAVE, session.exportSave());
    $("save-status").textContent = `Saved locally · day ${session.view.day}`;
    return true;
  } catch (e) {
    $("save-status").textContent = "Not saved — use Export save";
    status(`Save failed: ${e.message}`, true);
    return false;
  }
}
function act(method, ...args) {
  if (!session) return;
  try {
    const r =
      method === "fast_forward"
        ? safeFastForward(session, args[0])
        : session.call(method, ...args);
    session.refresh();
    const cues = session
      .take()
      .filter((e) => e.cue)
      .map((e) => e.cue);
    const saved = save();
    render();
    map.play(cues);
    if (!saved) return;
    status(
      method === "fast_forward"
        ? `Advanced ${r.ran} days · ${r.stop === "cap" ? "complete" : `stopped: ${r.alert?.kind || r.event || r.stop}`}`
        : method === "end_day"
          ? `Day ${session.view.day}. A new morning.`
          : {
              buy: "Purchase complete. Queue a sale when you are ready.",
              place_sell: "Sale queued for tonight.",
              hire: "Your new recruit is ready for an assignment.",
              post: "Assignment updated.",
              travel: "You have arrived.",
              buy_upgrade: "Upgrade acquired.",
            }[method] || "Operation updated.",
    );
  } catch (e) {
    status(e.message, true);
  }
}
function openLaw(city) {
  tab = "law";
  cityID = city || cityID;
  render();
  if (innerWidth < 800) $("panel-title").scrollIntoView({ behavior: "smooth" });
}
function renderRisk(risk) {
  const city = risk.hottest;
  $("heat-risk").disabled = $("evidence-risk").disabled = false;
  $("heat-label").textContent = `Heat · ${city.name}`;
  $("heat-value").textContent = `${city.heat.toFixed(1)} / 100`;
  $("heat-detail").textContent = city.reached
    ? `${city.reached.Level} line reached${city.next ? ` · ${city.next.Level} at ${city.next.Threshold.toFixed(1)}` : ""}`
    : `Below patrol · next ${city.next?.Level || "response"} at ${city.next?.Threshold.toFixed(1) || "—"}`;
  $("heat-fill").style.width = `${Math.max(0, Math.min(100, city.heat))}%`;
  $("heat-risk").dataset.severity = risk.heatCritical
    ? "critical"
    : city.reached
      ? "caution"
      : "quiet";
  $("evidence-value").textContent =
    `${risk.evidence} / ${risk.limit > 0 ? risk.limit : "—"}`;
  $("evidence-detail").textContent =
    risk.limit > 0
      ? `Indictment at ${risk.limit} · ${risk.remaining} remaining`
      : "No active indictment threshold";
  $("evidence-fill").style.width =
    `${risk.limit > 0 ? Math.min(100, (risk.evidence / risk.limit) * 100) : 0}%`;
  $("evidence-risk").dataset.severity = risk.evidenceCritical
    ? "critical"
    : risk.evidence > 0
      ? "caution"
      : "quiet";
  $("risk-warning").hidden = !risk.caution;
  $("risk-warning").textContent =
    `${risk.critical ? "DANGER — " : "CAUTION — "}${risk.warnings.join(" ")}`;
  $("risk-warning").dataset.severity = risk.critical ? "critical" : "caution";
  $("heat-risk").onclick = () => openLaw(city.id);
  $("evidence-risk").onclick = () => openLaw();
}
function advanceDays(days) {
  if (actionsDisabled) return;
  const risk = readRisk(session);
  if (risk.critical) {
    stopAuto();
    modal(
      "Danger before tonight",
      `${risk.warnings.join(" ")} Lying low cancels today's sales and deliveries and helps heat cool; it does not instantly remove evidence or guarantee safety.`,
      [
        button("Review the case", () => {
          closeModal();
          openLaw();
        }),
        button(
          "Lie low instead",
          () => {
            closeModal();
            act("set_lie_low", true);
          },
          false,
          "primary",
        ),
        button(
          "Advance one day anyway",
          () => {
            closeModal();
            act("end_day");
          },
          false,
          "danger",
        ),
      ],
    );
    return;
  }
  act(days === 1 ? "end_day" : "fast_forward", ...(days === 1 ? [] : [days]));
}
function render() {
  const v = session.view;
  const risk = readRisk(session);
  renderRisk(risk);
  if (risk.critical && autoTimer) {
    stopAuto();
    status(
      "Autopilot paused — review heat and evidence before continuing.",
      true,
    );
  }
  actionsDisabled = !!v.over || !!v.card;
  if (actionsDisabled) stopAuto();
  cityID = v.cities.some((c) => c.id === cityID) ? cityID : v.you.city;
  const city = v.cities.find((c) => c.id === cityID);
  $("day").textContent = String(v.day).padStart(2, "0");
  $("tier").textContent = v.you.tier_name.toUpperCase();
  for (const [k, val] of Object.entries({
    dirty: v.you.dirty_cash,
    clean: v.you.clean_cash,
    net: v.you.net_worth,
  }))
    $(k).textContent = money(val);
  $("cities").replaceChildren(
    ...v.cities.map((c) => {
      const b = button(
        c.name,
        () => {
          cityID = c.id;
          cornerID = null;
          render();
          if (innerWidth < 800) map.focusCity(c.id);
        },
        false,
        c.id === cityID ? "active" : "",
      );
      b.append(
        el(
          "small",
          `${c.id === v.you.city ? "You are here · " : ""}${c.corners.filter((k) => k.owner === "player").length}/${c.corners.length} corners · Heat ${Math.round(c.heat)}`,
        ),
      );
      return b;
    }),
  );
  map.setView(v, cityID, cornerID);
  for (const b of $("nav").children) {
    b.classList.toggle("active", b.dataset.tab === tab);
    b.setAttribute("aria-pressed", String(b.dataset.tab === tab));
  }
  $("end").disabled = $("week").disabled = actionsDisabled;
  $("low").disabled = actionsDisabled;
  $("low").textContent = v.you.lie_low ? "Resume operations" : "Lie low";
  $("brief-day").textContent = `DAY ${v.day} / ${v.alerts.length} ALERTS`;
  const brief = $("briefing");
  brief.replaceChildren();
  let count = 0;
  for (const [key, lines] of Object.entries(v.report)) {
    if (!Array.isArray(lines) || !lines.length) continue;
    const b = el("div", undefined, "brief-item");
    b.append(el("small", key.toUpperCase()), el("span", lines[0]));
    brief.append(b);
    if (++count === 4) break;
  }
  if (!count) {
    const b = el("div", undefined, "brief-item");
    b.append(
      el("small", "YOUR FIRST MOVE"),
      el(
        "span",
        "Buy from a connect, queue a sale, then end the day. Hire runners and post them to corners to expand.",
      ),
    );
    brief.append(b);
    const x = el("div", undefined, "brief-item");
    x.append(
      el("small", "KEEP IT QUIET"),
      el(
        "span",
        "Selling aggressively earns attention. Watch local heat and the DA’s evidence.",
      ),
    );
    brief.append(x);
  }
  $("panel").replaceChildren();
  $("panel-kicker").textContent = city.name.toUpperCase();
  const titles = {
    market: ["Street market", "Buy now. Sales resolve when you end the day."],
    territory: [
      "Territory",
      "Select a block on the map or choose a corner below.",
    ],
    crew: ["Your people", "Build a crew. Keep them paid and loyal."],
    logistics: ["Supply lines", "Set route pace, cargo targets, and drivers."],
    property: [
      "The legitimate side",
      "Stash houses, business fronts, and assets.",
    ],
    contracts: [
      "Buyer contracts",
      "Deliver on your promises before the deadline.",
    ],
    rivals: [
      "The other families",
      "Intelligence, negotiations, and open conflict.",
    ],
    upgrades: [
      "Build your advantage",
      "Invest in the next chapter of your operation.",
    ],
    law: ["Under scrutiny", "Know the people building a case against you."],
    journal: ["The morning report", "Every development from the last night."],
  };
  $("panel-title").textContent = titles[tab][0];
  $("panel-intro").textContent = titles[tab][1];
  panels[tab](v, city);
  if (v.card) {
    modal(
      v.card.title,
      v.card.text,
      v.card.choices.map((t, i) =>
        button(t, () => {
          closeModal();
          act("choose", i);
        }),
      ),
      true,
    );
  } else if (v.over && !reviewedEnding) {
    modal(
      "The run is over",
      `Day ${v.over.day}: ${v.over.cause}${v.over.who ? ` · ${v.over.who}` : ""}. Final net worth: ${money(v.you.net_worth)}.${v.over.cause === "indicted" ? ` The DA had ${risk.evidence} evidence; the current indictment threshold is ${risk.limit}. Evidence is separate from district heat. See Law and the heat report to review what built the case.` : ""}`,
      [
        button("Start a new run", () => startNew(), false, "primary"),
        button("Review the city", () => {
          reviewedEnding = true;
          closeModal();
        }),
      ],
    );
  }
}
const panels = {
  market(v, city) {
    if (city.id !== v.you.city) {
      const c = card(
        "Away from home",
        `You are currently in ${v.cities.find((x) => x.id === v.you.city)?.name}. Travel here to trade.`,
      );
      actions(
        c,
        cmd(`Travel to ${city.name}`, "travel", [city.id], false, "primary"),
      );
      $("panel").append(c);
    }
    const dial = select(
      ["quiet", "normal", "aggressive"].map((x) => [x, x]),
      sellDial,
    );
    dial.setAttribute("aria-label", "Sales approach");
    dial.onchange = () => {
      sellDial = dial.value;
    };
    $("panel").append(field("Sales approach", dial));
    const connect = streetConnect(v, city.id),
      held = v.you.stock[city.id] || {};
    for (const p of city.products) {
      const offer = connect?.prices[p.id],
        qty = number(
          offer
            ? Math.max(1, Math.min(10, Math.floor(v.you.dirty_cash / offer)))
            : 1,
        );
      const c = card(p.name, null, `${held[p.id] || 0} IN STOCK`);
      stats(c, [
        ["CONNECT", offer ? money(offer) : "Locked"],
        ["STREET", money(p.price)],
        [
          "CHANGE",
          `${p.facts.delta >= 0 ? "+" : ""}${p.facts.delta.toFixed(1)}%`,
        ],
      ]);
      if (p.history.length > 1) {
        const svg = document.createElementNS(
          "http://www.w3.org/2000/svg",
          "svg",
        );
        svg.setAttribute("viewBox", "0 0 280 28");
        svg.setAttribute("class", "spark");
        svg.setAttribute("aria-label", "Price history");
        const lo = Math.min(...p.history),
          hi = Math.max(...p.history),
          line = document.createElementNS(svg.namespaceURI, "polyline");
        line.setAttribute(
          "points",
          p.history
            .map(
              (y, i) =>
                `${(i / (p.history.length - 1)) * 280},${26 - ((y - lo) / Math.max(1, hi - lo)) * 24}`,
            )
            .join(" "),
        );
        line.setAttribute("fill", "none");
        line.setAttribute("stroke", "#7ce8bc");
        line.setAttribute("stroke-width", "1.6");
        svg.append(line);
        c.append(svg);
      }
      const r = el("div", undefined, "actions");
      r.append(
        qty,
        button(
          "Buy",
          () => {
            if (qty.reportValidity())
              act("buy", connect.id, p.id, Number(qty.value), false);
          },
          !offer || city.id !== v.you.city || actionsDisabled,
          "primary",
        ),
        button(
          "Sell all",
          () => act("place_sell", city.id, p.id, held[p.id], sellDial),
          !held[p.id] || actionsDisabled,
        ),
      );
      c.append(r);
      $("panel").append(c);
    }
  },
  territory(v, city) {
    const picker = select(
      city.corners.map((k) => [
        k.id,
        `${k.name} · ${k.owner === "player" ? "Yours" : k.owner === "rival" ? "Rival" : "Unclaimed"}`,
      ]),
      cornerID || city.corners[0]?.id,
    );
    cornerID = picker.value;
    picker.onchange = () => {
      cornerID = picker.value;
      render();
    };
    $("panel").append(field("Corner", picker));
    const k = city.corners.find((k) => k.id === cornerID);
    if (!k) return;
    const c = card(
      k.name,
      `Control: ${k.owner === "player" ? "your operation" : k.owner === "rival" ? k.faction : "unclaimed street"}. ${k.deed ? "You own the deed." : ""}`,
    );
    stats(c, [
      ["DEMAND", k.demand.toFixed(1)],
      [
        "RUNNER",
        k.runner === -1
          ? "You"
          : v.crew.find((m) => m.id === k.runner)?.name || "None",
      ],
    ]);
    const members = [
      [-1, "You"],
      ...v.crew
        .filter((m) => !m.jailed)
        .map((m) => [m.id, `${m.name} · ${m.role}`]),
    ];
    const s = select(members, k.runner || -1);
    c.append(field("Post a crew member", s));
    actions(
      c,
      button(
        "Post to corner",
        () => act("post", k.id, Number(s.value)),
        actionsDisabled,
        "primary",
      ),
    );
    const price = session.call("rules.territory.deed_price", k.id);
    actions(
      c,
      cmd(
        `Buy deed · ${money(price)}`,
        "buy_deed",
        [k.id],
        k.deed || v.you.clean_cash < price,
      ),
    );
    if (k.owner === "player")
      actions(
        c,
        button(
          "Abandon corner",
          () =>
            confirmAction("Abandon this corner?", k.name, () =>
              act("abandon", k.id),
            ),
          actionsDisabled,
          "danger",
        ),
      );
    if (k.owner === "rival")
      actions(
        c,
        cmd("Warn", "send_enforcers", [k.id, "warn"]),
        cmd("Push", "send_enforcers", [k.id, "push"]),
        cmd("Hit", "send_enforcers", [k.id, "hit"]),
      );
    $("panel").append(c);
  },
  crew(v) {
    const pay = select(
      ["stingy", "fair", "generous"].map((x) => [x, x]),
      v.you.pay,
    );
    pay.onchange = () => act("set_pay", pay.value);
    pay.disabled = actionsDisabled;
    $("panel").append(field("Payroll policy", pay));
    heading(`On your payroll · ${v.crew.length}`);
    if (!v.crew.length) empty("Nobody yet. Recruit your first runner below.");
    for (const m of v.crew) {
      const c = card(
        m.name,
        `${m.role} · ${m.jailed ? "In custody" : m.post || m.city || "Available"}`,
        `SKILL ${m.skill}`,
      );
      stats(c, [
        ["DAILY PAY", money(m.wage)],
        ["LOYALTY", `${Math.round(m.loyalty)}%`],
      ]);
      if (m.jailed) {
        const fee = session.call("rules.crew.bail_cost", m.id);
        actions(
          c,
          cmd(`Bail · ${money(fee)}`, "bail", [m.id], v.you.dirty_cash < fee),
        );
      } else {
        const post = select(
          [
            ["", "Choose a corner"],
            ...v.cities.flatMap((c) =>
              c.corners.map((k) => [k.id, `${c.name} / ${k.name}`]),
            ),
          ],
          "",
        );
        post.setAttribute("aria-label", `Post ${m.name}`);
        actions(
          c,
          post,
          button(
            "Post",
            () => {
              if (post.value) act("post", post.value, m.id);
            },
            actionsDisabled,
          ),
        );
      }
      actions(
        c,
        button(
          "Dismiss",
          () =>
            confirmAction(
              `Dismiss ${m.name}?`,
              "They will leave your payroll.",
              () => act("fire", m.id),
            ),
          actionsDisabled,
          "danger",
        ),
      );
      $("panel").append(c);
    }
    heading("Available recruits");
    for (const m of v.pool) {
      const c = card(m.name, `${m.role} · Age ${m.age}`, `SKILL ${m.skill}`);
      stats(c, [
        ["DAILY PAY", money(m.wage)],
        ["SIGNING FEE", money(m.fee)],
      ]);
      actions(
        c,
        cmd("Hire", "hire", [m.id], m.fee > v.you.dirty_cash, "primary"),
      );
      $("panel").append(c);
    }
    if (!v.pool.length) empty("No recruits available today.");
  },
  upgrades(v) {
    for (const u of v.upgrades) {
      const c = card(u.name, u.desc, u.branch);
      if (u.requires.length)
        c.append(
          el(
            "p",
            `Requires: ${u.requires.map((id) => v.upgrades.find((x) => x.id === id)?.name || id).join(", ")}`,
          ),
        );
      actions(
        c,
        cmd(
          u.state === "owned"
            ? "Owned"
            : u.state === "locked"
              ? "Prerequisites needed"
              : `${money(u.cost)} · ${u.clean ? "clean" : "street"} cash`,
          "buy_upgrade",
          [u.id],
          u.state !== "available" ||
            u.cost > (u.clean ? v.you.clean_cash : v.you.dirty_cash),
          u.state === "available" ? "primary" : "",
        ),
      );
      $("panel").append(c);
    }
  },
  contracts(v) {
    if (!v.contracts.length)
      empty(
        "No buyers are offering a contract yet. Keep building your reputation.",
      );
    for (const b of v.contracts) {
      const c = card(b.name, b.pitch, b.status);
      stats(c, [
        ["PRODUCT", b.product],
        ["DELIVERED", `${b.delivered}/${b.units}`],
        ["DUE", `Day ${b.due}`],
      ]);
      c.append(
        el(
          "p",
          `Accept by day ${b.expires}. Penalty: ${money(b.penalty_cash)} cash; reputation ${b.penalty}.`,
        ),
      );
      if (b.status === "offered")
        actions(
          c,
          cmd("Accept", "accept_contract", [b.id], false, "primary"),
          cmd("Decline", "decline_contract", [b.id]),
        );
      else {
        const q = number(Math.max(1, b.units - b.delivered));
        c.append(field("Units to deliver", q));
        actions(
          c,
          button(
            "Deliver",
            () => {
              if (q.reportValidity()) act("deliver", b.id, Number(q.value));
            },
            actionsDisabled,
            "primary",
          ),
        );
      }
      $("panel").append(c);
    }
  },
  logistics(v) {
    if (!v.routes.length)
      empty(
        "No routes are open yet. Build your operation to unlock connections.",
      );
    for (const r of v.routes) {
      const c = card(
        r.name,
        `${r.from} → ${r.to} · ${r.mode}`,
        r.closed ? "CLOSED" : r.dial,
      );
      const pace = select(
        ["off", "slow", "normal", "fast"].map((x) => [x, x]),
        r.dial,
      );
      pace.disabled = actionsDisabled;
      pace.onchange = () => act("set_route", r.id, pace.value);
      c.append(field("Route pace", pace));
      const products = v.cities.find((x) => x.id === r.from)?.products || [];
      const prod = select(
          products.map((p) => [p.id, p.name]),
          products[0]?.id,
        ),
        q = number(10);
      c.append(field("Cargo", prod), field("Target units", q));
      actions(
        c,
        button(
          "Set cargo target",
          () => {
            if (q.reportValidity())
              act("set_route_target", r.id, prod.value, Number(q.value));
          },
          !products.length || actionsDisabled,
        ),
      );
      const drivers = select(
        [
          [0, "No driver"],
          ...v.crew
            .filter((m) => m.role === "driver")
            .map((m) => [m.id, m.name]),
        ],
        r.driver || 0,
      );
      drivers.disabled = actionsDisabled;
      drivers.onchange = () =>
        act("set_route_driver", r.id, Number(drivers.value));
      c.append(field("Driver", drivers));
      c.append(
        el(
          "p",
          r.risk_known
            ? `Known risk: ${Math.round(r.risk * 100)}%`
            : "Risk: intelligence unavailable",
        ),
      );
      $("panel").append(c);
    }
    heading("In transit");
    for (const s of v.shipments)
      $("panel").append(
        card(
          `${s.units} ${s.product}`,
          `${s.from} → ${s.to}. Arrives day ${s.arrives}.`,
        ),
      );
    if (!v.shipments.length) empty("No shipments in transit.");
  },
  property(v) {
    const d = select(
      ["careful", "normal", "greedy"].map((x) => [x, x]),
      v.you.launder,
    );
    d.disabled = actionsDisabled;
    d.onchange = () => act("set_launder_dial", d.value);
    $("panel").append(field("Laundering pace", d));
    heading("Your properties");
    for (const h of v.houses)
      $("panel").append(
        card(
          h.name,
          `${h.city} · Capacity ${h.capacity} · ${h.known ? "Known to police" : "Not known to police"}. Stock: ${
            Object.entries(h.stock)
              .map(([k, n]) => `${n} ${k}`)
              .join(", ") || "empty"
          }`,
        ),
      );
    for (const f of v.fronts) {
      const c = card(
        f.name,
        `Level ${f.level} · Washed ${money(f.washed)}`,
        f.frozen ? "FROZEN" : "ACTIVE",
      );
      const cost = session.call("rules.laundering.level_cost", f.id, 1);
      actions(c, cmd(`Invest · ${money(cost)}`, "invest", [f.id, 1], f.frozen));
      $("panel").append(c);
    }
    if (!v.houses.length && !v.fronts.length) empty("No properties owned yet.");
    for (const [query, title, method] of [
      ["house_offers", "Stash houses", "buy_house"],
      ["front_offers", "Business fronts", "buy_front"],
      ["asset_offers", "Assets", "buy_asset"],
    ]) {
      heading(title);
      const offers = session.call(query) || [];
      for (const o of offers) {
        const id = o.ID ?? o.id,
          name = o.Name ?? o.name,
          cost = o.Price ?? o.price ?? o.Cost ?? o.cost;
        const c = card(name || id, null);
        actions(
          c,
          cmd(`Buy${cost !== undefined ? ` · ${money(cost)}` : ""}`, method, [
            id,
          ]),
        );
        $("panel").append(c);
      }
      if (!offers.length) empty("No offers available.");
    }
  },
  rivals(v) {
    for (const f of v.factions) {
      const c = card(
        f.leader,
        `${f.personality === "?" ? "Temper unknown" : f.personality} · ${f.move || "No known next move"}`,
        f.alive ? "ACTIVE" : f.arrived ? "GONE" : "NOT ARRIVED",
      );
      stats(c, [
        ["CORNERS", f.corners],
        ["TRUST", f.trust.toFixed(2)],
        ["WAR", f.war.toFixed(2)],
      ]);
      const price = session.call("rules.rivals.scout_cost");
      actions(
        c,
        cmd(`Scout · ${money(price)}`, "scout_faction", [f.id], !f.alive),
        button(
          "Declare war",
          () =>
            confirmAction(
              "Declare war?",
              `Open conflict with ${f.leader}.`,
              () => act("declare_war", f.id),
            ),
          actionsDisabled || !f.alive,
          "danger",
        ),
      );
      $("panel").append(c);
    }
    heading("Offers on the table");
    for (const o of v.offers) {
      const c = card(
        o.kind,
        `${o.faction} · Expires day ${o.expires}. ${o.terms.days || 0} days, ${money(o.terms.per_day)} per day.`,
      );
      actions(
        c,
        cmd("Accept", "accept", [o.id]),
        cmd("Decline", "decline", [o.id]),
      );
      $("panel").append(c);
    }
    if (!v.offers.length) empty("No diplomatic offers right now.");
  },
  law(v, city) {
    const risk = readRisk(session);
    const local = risk.cities.find((c) => c.id === city.id);
    const c = card(
      "The case against you",
      `District attorney: ${v.law.da} · ${v.law.da_stance}`,
    );
    stats(c, [
      ["EVIDENCE", `${risk.evidence} / ${risk.limit || "—"}`],
      ["LOCAL HEAT", Math.round(city.heat)],
    ]);
    c.append(
      el(
        "p",
        `Chief ${v.law.chief} · ${v.law.chief_temper}. Next election: day ${v.law.next_election}.`,
      ),
    );
    actions(
      c,
      cmd(
        v.you.lie_low ? "Resume operations" : "Lie low",
        "set_lie_low",
        [!v.you.lie_low],
        false,
        "primary",
      ),
    );
    c.append(
      el(
        "p",
        "Heat is local police attention. Evidence is the DA’s separate case against you; cooling heat does not automatically erase it. Stings and raids while dealing can add evidence. Informants and other actions can also grow the file.",
      ),
    );
    c.append(
      el(
        "p",
        "Lie low cancels today’s sales and deliveries and increases heat decay. A legal retainer can let evidence decay after quiet days; lying low alone is not an instant reset.",
      ),
    );
    $("panel").append(c);
    heading(`${city.name} · current police thresholds`);
    for (const rung of local.ladder) {
      const line = card(
        rung.Level,
        `Heat ${rung.Threshold.toFixed(1)}${rung.Evidence ? ` · base evidence from response: ${rung.Evidence}` : ""}`,
        city.heat >= rung.Threshold ? "LINE REACHED" : "",
      );
      $("panel").append(line);
    }
    const reasons = (v.report.heat || []).concat(v.report.law || []);
    if (reasons.length) {
      heading("What happened last night");
      for (const reason of reasons)
        $("panel").append(el("p", reason, "list-line"));
    }
    heading("Current alerts");
    for (const a of v.alerts)
      $("panel").append(card(a.kind.replaceAll("_", " "), a.key));
    if (!v.alerts.length) empty("No immediate warnings.");
  },
  journal(v) {
    for (const [key, lines] of Object.entries(v.report)) {
      if (!Array.isArray(lines) || !lines.length) continue;
      heading(key);
      for (const l of lines) $("panel").append(el("p", l, "list-line"));
    }
    if (!Object.values(v.report).some((x) => Array.isArray(x) && x.length))
      empty("Your story starts today. End the day to see the first report.");
  },
};
function startNew() {
  stopAuto();
  reviewedEnding = false;
  closeModal();
  map.clearEffects();
  session.newRun(1 + Math.floor(Math.random() * 1e9));
  session.take();
  cityID = session.view.you.city;
  cornerID = null;
  tab = "market";
  save();
  render();
  status("Your new operation is ready.");
}
function stopAuto() {
  clearInterval(autoTimer);
  autoTimer = null;
}
function toggleAuto() {
  if (readRisk(session).critical) {
    stopAuto();
    closeModal();
    advanceDays(1);
    return;
  }
  if (autoTimer) {
    stopAuto();
    closeModal();
    status("Autopilot paused.");
    return;
  }
  closeModal();
  status("Autopilot is trading and advancing days. Open settings to pause.");
  autoTimer = setInterval(() => {
    if (actionsDisabled || readRisk(session).critical) {
      stopAuto();
      return;
    }
    try {
      autoDay(session);
      session.refresh();
      const cues = session
        .take()
        .filter((e) => e.cue)
        .map((e) => e.cue);
      const saved = save();
      render();
      map.play(cues);
      if (!saved) stopAuto();
    } catch (e) {
      stopAuto();
      status(e.message, true);
    }
  }, 2200);
}
function settings() {
  const wasAuto = !!autoTimer;
  stopAuto();
  if (wasAuto) status("Autopilot paused while settings are open.");
  modal(
    "Your operation",
    `Day ${session.view.day} · Seed ${session.view.seed}. Progress is automatically saved after every action.`,
    [
      button(
        autoTimer ? "Pause autopilot" : "Autopilot · trades for you",
        toggleAuto,
        actionsDisabled,
      ),
      button("Save now", () => {
        save();
        closeModal();
        status("Saved in this browser.");
      }),
      button("Load saved run", () => {
        try {
          const b = localStorage.getItem(SAVE);
          if (!b) throw new Error("No saved run in this browser.");
          stopAuto();
          reviewedEnding = false;
          session.importSave(b);
          session.take();
          map.clearEffects();
          closeModal();
          render();
          status("Saved run restored.");
        } catch (e) {
          status(e.message, true);
        }
      }),
      button("Export save", () => {
        const blob = new Blob([session.exportSave()], { type: "text/plain" }),
          u = URL.createObjectURL(blob),
          a = el("a");
        a.href = u;
        a.download = `kingpin-day-${session.view.day}.save`;
        a.click();
        setTimeout(() => URL.revokeObjectURL(u), 1000);
      }),
      button("Import save", () => {
        const input = el("input");
        input.type = "file";
        input.accept = ".save,.txt";
        input.hidden = true;
        document.body.append(input);
        input.addEventListener("cancel", () => input.remove());
        input.onchange = async () => {
          input.remove();
          try {
            if (!input.files[0]) return;
            stopAuto();
            reviewedEnding = false;
            session.importSave(await input.files[0].text());
            session.take();
            map.clearEffects();
            closeModal();
            save();
            render();
            status("Save imported.");
          } catch (e) {
            status(`Import refused: ${e.message}`, true);
          }
        };
        input.click();
      }),
      button(
        "New run",
        () =>
          confirmAction(
            "Start over?",
            "This replaces the saved run in this browser. Export it first if you want to keep it.",
            startNew,
          ),
        false,
        "danger",
      ),
      button("Back to the city", () => closeModal()),
    ],
  );
}
async function boot() {
  try {
    const go = new Go();
    const response = await fetch("kingpin.wasm");
    if (!response.ok)
      throw new Error(`Engine download failed (${response.status})`);
    const { instance } = await WebAssembly.instantiate(
      await response.arrayBuffer(),
      go.importObject,
    );
    go.run(instance).catch((e) => status(e.message, true));
    session = new Session(globalThis.kingpin);
    map = new CityMap($("map"), (city, corner) => {
      cityID = city;
      cornerID = corner;
      if (corner) tab = "territory";
      render();
      if (innerWidth < 800 && corner)
        $("panel-title").scrollIntoView({ behavior: "smooth" });
    });
    for (const [id, icon, title] of TABS) {
      const b = button("", () => {
        tab = id;
        render();
        if (innerWidth < 800)
          $("panel-title").scrollIntoView({
            behavior: matchMedia("(prefers-reduced-motion: reduce)").matches
              ? "instant"
              : "smooth",
          });
      });
      b.dataset.tab = id;
      b.append(el("span", icon, "icon"), el("span", title));
      $("nav").append(b);
    }
    $("end").onclick = () => advanceDays(1);
    $("week").onclick = () => advanceDays(7);
    $("low").onclick = () => act("set_lie_low", !session.view.you.lie_low);
    $("menu").onclick = settings;
    $("zoom-in").onclick = () => map.zoom(1.2);
    $("zoom-out").onclick = () => map.zoom(0.8);
    $("reset-map").onclick = () => map.fit(true);
    addEventListener("keydown", (e) => {
      if (
        e.code === "Space" &&
        !e.repeat &&
        !$("modal").open &&
        !["INPUT", "SELECT", "TEXTAREA", "BUTTON"].includes(e.target.tagName)
      ) {
        e.preventDefault();
        if (!actionsDisabled) advanceDays(1);
      }
    });
    const seed = new URLSearchParams(location.search).get("seed");
    let saved;
    try {
      saved = localStorage.getItem(SAVE);
    } catch {}
    if (saved && !seed) {
      try {
        session.importSave(saved);
      } catch (e) {
        session.newRun(1 + Math.floor(Math.random() * 1e9));
        status(`Saved run could not load: ${e.message}`, true);
      }
    } else
      session.newRun(
        seed && /^\d{1,9}$/.test(seed)
          ? Number(seed)
          : 1 + Math.floor(Math.random() * 1e9),
      );
    session.take();
    cityID = session.view.you.city;
    render();
    $("loading").hidden = true;
    status(
      `Welcome to ${session.view.cities.find((c) => c.id === cityID).name}. Your next move is waiting.`,
    );
  } catch (e) {
    $("loading-text").textContent = `Unable to start: ${e.message}`;
    $("loading").append(button("Retry", () => location.reload()));
    console.error(e);
  }
}
boot();

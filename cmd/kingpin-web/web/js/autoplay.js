// The autopilot (#328): the reference client's greedy dealer
// (protocol.Play) as the page's button plays it, one day a call. It
// answers a card with its first choice, spends 60% of the dirty cash
// across what the street connect where it stands sells, puts everything
// it holds on the street at the aggressive dial and ends the day. A
// move the game refuses is part of play.
import { streetConnect } from "./session.js";

export function autoDay(session) {
  let v = session.refresh();
  if (v.over) return v;
  if (v.card) {
    session.choose(0);
    return session.refresh();
  }
  const city = v.you.city;
  const k = streetConnect(v, city);
  if (k) {
    const products = Object.keys(k.prices).sort();
    for (const pid of products) {
      const price = k.prices[pid];
      if (!(price > 0)) continue;
      const qty = Math.floor((v.you.dirty_cash * 0.6) / products.length / price);
      if (qty > 0) tryMove(() => session.buy(k.id, pid, qty));
    }
  }
  v = session.refresh();
  const held = (v.you.stock || {})[city] || {};
  for (const pid of Object.keys(held).sort()) {
    if (held[pid] > 0) tryMove(() => session.sell(city, pid, held[pid], "aggressive"));
  }
  session.endDay();
  return session.view;
}

function tryMove(f) {
  try {
    f();
  } catch (e) {
    if (!e.refused) throw e;
  }
}

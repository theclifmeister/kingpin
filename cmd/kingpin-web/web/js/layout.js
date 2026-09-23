// Where things are on the canvas (#328): a pure function of the view
// and the canvas's size, so the renderer, the animations and the Node
// test agree on it. The cities stand side by side in the view's order,
// each a block of its corners on their map cells (city.toml's x and y);
// a route is a line between two blocks.

export const PLAYER = -1; // game.You: the corner's runner when it is you

export const COLOURS = {
  bg: "#0f1115",
  city: "#161a22",
  cityEdge: "#2a3140",
  text: "#d8dee9",
  dim: "#7b8494",
  none: "#2b303b",
  player: "#2fb36b",
  deed: "#f5c542",
  road: "#465063",
  closed: "#6b2b2b",
};

// FACTIONS are the rivals' colours in the order the view lists them.
const FACTIONS = ["#e5484d", "#f76b15", "#b15bc7", "#3e8ef7", "#d6409f"];

// LANE is the room a road takes under the blocks, its name above it.
const LANE = 26;

export function layout(view, W, H) {
  const L = {
    W,
    H,
    cities: {},
    corners: {},
    cityOrder: [],
    factionColour: {},
    view,
  };
  (view.factions || []).forEach((f, i) => (L.factionColour[f.id] = FACTIONS[i % FACTIONS.length]));
  const cities = view.cities || [];
  const n = Math.max(1, cities.length);
  const margin = 20;
  const gap = Math.min(140, W * 0.1);
  const cw = (W - 2 * margin - gap * (n - 1)) / n;
  const lanes = Math.max(1, (view.routes || []).length);
  const roads = 30 + lanes * LANE; // the roads' room under the blocks
  const room = H - 2 * margin - roads; // the blocks' room
  const head = 36; // a block's name and heat
  const pad = 10;
  // One cell size for every city, so a corner is a corner's size
  // wherever it is: the largest the widest and the tallest grid allow.
  const grids = cities.map((c) => ({ cols: Math.max(1, ...c.corners.map((k) => k.x + 1)), rows: Math.max(1, ...c.corners.map((k) => k.y + 1)) }));
  const cell = Math.max(24, Math.min(...grids.map((g) => Math.min((cw - 2 * pad) / g.cols, (room - head - 2 * pad) / g.rows)), 150));
  const bh = Math.max(...grids.map((g) => head + g.rows * cell + 2 * pad));
  const top = margin + Math.max(0, (room - bh) / 2);
  cities.forEach((c, i) => {
    const x = margin + i * (cw + gap);
    const r = { id: c.id, name: c.name, x, y: top, w: cw, h: bh };
    L.cities[c.id] = r;
    L.cityOrder.push(c.id);
    const g = grids[i];
    const gx = x + (cw - cell * g.cols) / 2;
    const gy = top + head + pad;
    for (const k of c.corners) {
      L.corners[k.id] = { id: k.id, city: c.id, x: gx + k.x * cell + 4, y: gy + k.y * cell + 4, w: cell - 8, h: cell - 8, corner: k };
    }
  });
  L.roadTop = top + bh + 24;
  return L;
}

export function centre(r) {
  return { x: r.x + r.w / 2, y: r.y + r.h / 2 };
}

// ownerColour is a corner's colour: yours, a faction's, or the street's.
export function ownerColour(L, owner, faction) {
  if (owner === "player") return COLOURS.player;
  if (owner === "rival") return L.factionColour[faction] || FACTIONS[0];
  return COLOURS.none;
}

// cornerSpot is a corner's centre; cityHead the spot under a city's
// name, where what happens to the city as a whole is drawn.
export function cornerSpot(L, id) {
  const r = L.corners[id];
  return r ? centre(r) : null;
}
export function cityHead(L, id) {
  const r = L.cities[id];
  return r ? { x: r.x + r.w / 2, y: r.y + 30 } : { x: L.W / 2, y: 40 };
}
export function cityMid(L, id) {
  const r = L.cities[id];
  return r ? centre(r) : { x: L.W / 2, y: L.H / 2 };
}

// routePath is the road a route takes: from the bottom of one city's
// block, down under the blocks and up into the other's.
export function routePath(L, routeId) {
  const routes = L.view.routes || [];
  const i = routes.findIndex((r) => r.id === routeId);
  const r = routes[i];
  if (!r || !L.cities[r.from] || !L.cities[r.to]) return null;
  const a = L.cities[r.from];
  const b = L.cities[r.to];
  // Each road its own lane, the deeper the wider where it leaves the
  // blocks, so the roads nest and none crosses another.
  const depth = L.roadTop + i * LANE;
  const off = (i - (routes.length - 1) / 2) * 14;
  const ax = a.x + a.w / 2 + (a.x < b.x ? -off : off);
  const bx = b.x + b.w / 2 + (a.x < b.x ? off : -off);
  return [
    { x: ax, y: a.y + a.h },
    { x: ax, y: depth },
    { x: bx, y: depth },
    { x: bx, y: b.y + b.h },
  ];
}

// along is the point a fraction f of the way down a path of points.
export function along(path, f) {
  const lens = [];
  let total = 0;
  for (let i = 1; i < path.length; i++) {
    const d = Math.hypot(path[i].x - path[i - 1].x, path[i].y - path[i - 1].y);
    lens.push(d);
    total += d;
  }
  let t = Math.max(0, Math.min(1, f)) * total;
  for (let i = 0; i < lens.length; i++) {
    if (t <= lens[i] || i === lens.length - 1) {
      const p = lens[i] ? t / lens[i] : 0;
      return { x: path[i].x + (path[i + 1].x - path[i].x) * p, y: path[i].y + (path[i + 1].y - path[i].y) * p };
    }
    t -= lens[i];
  }
  return path[0];
}

// memberSpot is where a crew member stands: the corner they work or
// guard, the city a lieutenant runs, or beside you.
export function memberSpot(L, id) {
  const m = (L.view.crew || []).find((c) => c.id === id);
  if (m && m.post && L.corners[m.post]) return cornerSpot(L, m.post);
  if (m && m.city) return cityHead(L, m.city);
  return cityHead(L, L.view.you ? L.view.you.city : L.cityOrder[0]);
}

// houseSpot is a stash house's corner.
export function houseSpot(L, id) {
  const h = (L.view.houses || []).find((x) => x.id === id);
  return h ? cornerSpot(L, h.corner) : null;
}

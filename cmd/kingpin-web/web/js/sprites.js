// The sprites (#328): placeholder pixel art, a string a row and a letter
// a colour, drawn a pixel a rect so it needs nothing but a 2D context
// (the Node test's stub one included).

const PALETTE = {
  k: "#111418",
  w: "#eef1f5",
  s: "#e0ac69",
  g: "#2fb36b",
  h: "#1d2126",
  r: "#e5484d",
  b: "#3e8ef7",
  y: "#f5c542",
  o: "#f76b15",
  n: "#8d5524",
  c: "#9aa3b2",
  d: "#4a5160",
  p: "#b15bc7",
};

export const SPRITES = {
  player: ["..hhhh..", ".hhhhhh.", "..ssss..", "..skks..", ".gggggg.", "g.gggg.g", "..gggg..", "..k..k.."],
  runner: ["..kkkk..", "..ssss..", "..skks..", "...ss...", ".bbbbbb.", "b.bbbb.b", "..bbbb..", "..k..k.."],
  enforcer: ["..kkkk..", "..ssss..", "..skks..", ".dddddd.", "dddddddd", "d.dddd.d", "..dddd..", "..k..k.."],
  cop: ["..bbbb..", ".bbbbbb.", "..ssss..", "..skks..", ".bbybbb.", "b.bbbb.b", "..bbbb..", "..k..k.."],
  robber: ["..kkkk..", "..kwwk..", "..kkkk..", "...ss...", ".dddddd.", "d.dddd.d", "..dddd..", "..k..k.."],
  car: ["..........", "...cccc...", "..cwwwwc..", ".cccccccc.", ".cccccccc.", "..kk..kk.."],
  truck: ["............", ".nnnnnn.....", ".nnnnnn.bb..", ".nnnnnn.bwb.", ".nnnnnnbbbbb", ".nnnnnnbbbbb", "..kk....kk.."],
  boat: ["....w.......", "....ww......", "....www.....", "nnnnnnnnnnnn", ".nnnnnnnnnn.", "..nnnnnnnn.."],
  plane: [".....w......", ".....ww.....", "wwwwwwwwwwww", ".....ww.....", ".....w......", "....www....."],
  copcar: ["...rb.....", "..wwwwww..", ".wbbwwbbw.", ".wwwwwwww.", "..kk..kk.."],
  heli: ["cccccccccccc", ".....c......", "...dddddd...", "..dddwwddddd", "...dddddd...", "....k..k...."],
  house: ["...rr...", "..rrrr..", ".rrrrrr.", "rrrrrrrr", ".nnnnnn.", ".nwnnwn.", ".nnnkkn.", ".nnnkkn."],
  coin: [".yyy.", "yykyy", "ykyky", "yykyy", ".yyy."],
  skull: [".wwwww.", "wwwwwww", "wkwwwkw", "wwwwwww", ".wwkww.", ".w.w.w."],
  flag: ["k.....", "kggg..", "kgggg.", "kggg..", "k.....", "k.....", "k.....", "k....."],
  crate: ["nnnnnn", "nynnyn", "nnyynn", "nnyynn", "nynnyn", "nnnnnn"],
  badge: ["..yy..", ".yyyy.", "yybbyy", "yybbyy", ".yyyy.", "..yy.."],
  fist: [".oooo.", "oooooo", "oooooo", "oooooo", ".oooo.", "..oo.."],
};

// VEHICLE is the sprite a route's mode travels as.
export const VEHICLE = { car: "car", truck: "truck", boat: "boat", plane: "plane", tunnel: "truck" };

// drawSprite draws a sprite centred on (x, y) at scale pixels a pixel,
// at alpha, flipped left-right when flip is set.
export function drawSprite(ctx, name, x, y, scale = 3, alpha = 1, flip = false) {
  const rows = SPRITES[name];
  if (!rows) return;
  const w = rows[0].length;
  const h = rows.length;
  const x0 = Math.round(x - (w * scale) / 2);
  const y0 = Math.round(y - (h * scale) / 2);
  ctx.globalAlpha = alpha;
  for (let j = 0; j < h; j++) {
    for (let i = 0; i < w; i++) {
      const c = rows[j][flip ? w - 1 - i : i];
      if (c === ".") continue;
      ctx.fillStyle = PALETTE[c];
      ctx.fillRect(x0 + i * scale, y0 + j * scale, scale, scale);
    }
  }
  ctx.globalAlpha = 1;
}

// The scene (#328): the map drawn from the view, and the animations
// playing over it. drawMap is a pure function of the layout and the
// clock, so the Node test draws it on a stub context; Scene owns the
// canvas, the frame loop and the queue of animations.
import { ANIMATIONS } from "./cues.js";
import { COLOURS, PLAYER, along, cityHead, layout, ownerColour, routePath } from "./layout.js";
import { VEHICLE, drawSprite } from "./sprites.js";

// drawMap draws the cities, the roads, the corners and who stands on
// them, the houses and the shipments on the road.
export function drawMap(ctx, L, now = 0) {
  const v = L.view;
  ctx.fillStyle = COLOURS.bg;
  ctx.fillRect(0, 0, L.W, L.H);

  for (const r of v.routes || []) {
    const path = routePath(L, r.id);
    if (!path) continue;
    ctx.strokeStyle = r.closed ? COLOURS.closed : COLOURS.road;
    ctx.lineWidth = 4;
    ctx.setLineDash(r.mode === "boat" || r.mode === "plane" ? [8, 8] : []);
    ctx.beginPath();
    ctx.moveTo(path[0].x, path[0].y);
    for (const p of path.slice(1)) ctx.lineTo(p.x, p.y);
    ctx.stroke();
    ctx.setLineDash([]);
    const mid = along(path, 0.5);
    ctx.fillStyle = COLOURS.dim;
    ctx.font = "11px ui-monospace, monospace";
    ctx.textAlign = "center";
    ctx.fillText(`${r.name} (${r.mode}) · ${r.closed ? "closed" : r.dial}`, mid.x, mid.y - 6);
  }

  const heat = {};
  for (const c of v.cities || []) heat[c.id] = c.heat;
  for (const id of L.cityOrder) {
    const r = L.cities[id];
    ctx.fillStyle = COLOURS.city;
    ctx.fillRect(r.x, r.y, r.w, r.h);
    ctx.fillStyle = `rgba(229,72,77,${Math.min(0.3, (heat[id] || 0) / 330)})`;
    ctx.fillRect(r.x, r.y, r.w, r.h);
    ctx.strokeStyle = v.you && v.you.city === id ? COLOURS.player : COLOURS.cityEdge;
    ctx.lineWidth = 2;
    ctx.strokeRect(r.x, r.y, r.w, r.h);
    ctx.fillStyle = COLOURS.text;
    ctx.font = "bold 14px ui-monospace, monospace";
    ctx.textAlign = "left";
    ctx.fillText(r.name, r.x + 10, r.y + 20);
    ctx.fillStyle = COLOURS.dim;
    ctx.font = "12px ui-monospace, monospace";
    ctx.textAlign = "right";
    ctx.fillText(`heat ${Math.round(heat[id] || 0)}`, r.x + r.w - 10, r.y + 20);
  }

  const bob = Math.sin(now / 300) * 1.5;
  const houses = {};
  for (const h of v.houses || []) houses[h.corner] = h;
  for (const k of Object.values(L.corners)) {
    const c = k.corner;
    ctx.fillStyle = ownerColour(L, c.owner, c.faction);
    ctx.globalAlpha = c.owner === "none" ? 1 : 0.85;
    ctx.fillRect(k.x, k.y, k.w, k.h);
    ctx.globalAlpha = 1;
    if (c.deed) {
      ctx.strokeStyle = COLOURS.deed;
      ctx.lineWidth = 3;
      ctx.strokeRect(k.x + 1.5, k.y + 1.5, k.w - 3, k.h - 3);
    }
    ctx.fillStyle = COLOURS.text;
    ctx.font = "11px ui-monospace, monospace";
    ctx.textAlign = "left";
    ctx.fillText(c.name.length > 14 ? c.name.slice(0, 13) + "…" : c.name, k.x + 5, k.y + 14);
    const s = Math.max(2, Math.min(4, Math.floor(k.w / 26)));
    if (c.runner === PLAYER) drawSprite(ctx, "player", k.x + k.w * 0.35, k.y + k.h * 0.62 + bob, s);
    else if (c.runner > 0) drawSprite(ctx, "runner", k.x + k.w * 0.35, k.y + k.h * 0.62 + bob, s);
    if (c.enforcer > 0) drawSprite(ctx, "enforcer", k.x + k.w * 0.68, k.y + k.h * 0.62 - bob, s);
    if (houses[c.id]) drawSprite(ctx, "house", k.x + k.w - 14, k.y + 16, 2);
  }

  // You stand in your city even on no corner of it.
  if (v.you && !Object.values(L.corners).some((k) => k.city === v.you.city && k.corner.runner === PLAYER)) {
    const at = cityHead(L, v.you.city);
    drawSprite(ctx, "player", at.x, at.y + 4 + bob, 3);
  }

  for (const sh of v.shipments || []) {
    const path = routePath(L, sh.route);
    if (!path) continue;
    const route = (v.routes || []).find((r) => r.id === sh.route);
    let f = sh.arrives > sh.sent ? (v.day - sh.sent) / (sh.arrives - sh.sent) : 0.5;
    if (route && route.from !== sh.from) path.reverse();
    f = Math.max(0.08, Math.min(0.92, f));
    const at = along(path, f);
    drawSprite(ctx, VEHICLE[route && route.mode] || "truck", at.x, at.y - 2 + bob, 3, 1, path[path.length - 1].x < path[0].x);
  }
}

export class Scene {
  constructor(canvas) {
    this.canvas = canvas;
    this.ctx = canvas.getContext("2d");
    this.view = null;
    this.L = null;
    this.anims = []; // { start, dur, draw }
    this.frame = this.frame.bind(this);
    this.resize();
    requestAnimationFrame(this.frame);
  }

  resize() {
    const dpr = window.devicePixelRatio || 1;
    const r = this.canvas.getBoundingClientRect();
    this.canvas.width = Math.round(r.width * dpr);
    this.canvas.height = Math.round(r.height * dpr);
    this.ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    this.W = r.width;
    this.H = r.height;
    if (this.view) this.L = layout(this.view, this.W, this.H);
  }

  setView(view) {
    this.view = view;
    this.L = layout(view, this.W, this.H);
  }

  // play queues a day's cues, each a little after the last, so a busy
  // night reads as a sequence and not a flash.
  play(cues, stagger = 160) {
    const now = performance.now();
    cues.forEach((c, i) => {
      const make = ANIMATIONS[c.kind];
      if (!make || !this.L) return;
      for (const a of make(c, this.L)) this.anims.push({ start: now + i * stagger, dur: a.dur, draw: a.draw });
    });
  }

  busy() {
    return this.anims.length > 0;
  }

  frame(now) {
    if (this.L) {
      drawMap(this.ctx, this.L, now);
      this.anims = this.anims.filter((a) => now < a.start + a.dur);
      for (const a of this.anims) {
        if (now < a.start) continue;
        a.draw(this.ctx, (now - a.start) / a.dur);
      }
    }
    requestAnimationFrame(this.frame);
  }
}

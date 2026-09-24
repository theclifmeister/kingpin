// The animations (#328): one for every cue the engine gives
// (engine.CueKinds; the Go test holds this table to it). A cue arrives
// with the event it animates (event.params.cue, #301) and names what
// moved in ids; ANIMATIONS[kind](cue, L) turns it into timed drawings
// over the layout L, and the scene plays them over the map. An
// animation is { dur, draw(ctx, p) }: dur in milliseconds, p running
// from 0 to 1. None of them reads the world: what they show is the cue
// and where the layout puts its ids.
import { COLOURS, along, cityHead, cityMid, cornerSpot, houseSpot, memberSpot, ownerColour, routePath } from "./layout.js";
import { VEHICLE, drawSprite } from "./sprites.js";

const ease = (p) => 1 - Math.pow(1 - p, 3);

// The drawings the animations are made of.
function ring(at, colour, radius = 36, dur = 900) {
  return {
    dur,
    draw(ctx, p) {
      ctx.globalAlpha = 1 - p;
      ctx.strokeStyle = colour;
      ctx.lineWidth = 3;
      ctx.beginPath();
      ctx.arc(at.x, at.y, 6 + radius * ease(p), 0, Math.PI * 2);
      ctx.stroke();
      ctx.globalAlpha = 1;
    },
  };
}

function float(at, text, colour, dur = 1200) {
  return {
    dur,
    draw(ctx, p) {
      ctx.globalAlpha = p < 0.7 ? 1 : 1 - (p - 0.7) / 0.3;
      ctx.fillStyle = colour;
      ctx.font = "bold 14px ui-monospace, monospace";
      ctx.textAlign = "center";
      ctx.fillText(text, at.x, at.y - 10 - 34 * ease(p));
      ctx.globalAlpha = 1;
    },
  };
}

function sprite(name, from, to, dur = 1000, opts = {}) {
  return {
    dur,
    draw(ctx, p) {
      const q = opts.linear ? p : ease(p);
      const x = from.x + (to.x - from.x) * q;
      const y = from.y + (to.y - from.y) * q - (opts.hop ? Math.sin(p * Math.PI) * opts.hop : 0);
      const alpha = opts.fadeOut ? 1 - p : opts.fadeIn ? Math.min(1, p * 3) : 1;
      drawSprite(ctx, name, x, y, opts.scale || 3, alpha, to.x < from.x);
    },
  };
}

function burst(at, colours, n = 14, dur = 800) {
  const bits = Array.from({ length: n }, (_, i) => ({ a: (i / n) * Math.PI * 2, v: 20 + ((i * 37) % 23), c: colours[i % colours.length] }));
  return {
    dur,
    draw(ctx, p) {
      ctx.globalAlpha = 1 - p;
      for (const b of bits) {
        ctx.fillStyle = b.c;
        ctx.fillRect(at.x + Math.cos(b.a) * b.v * ease(p) * 2 - 2, at.y + Math.sin(b.a) * b.v * ease(p) * 2 - 2, 4, 4);
      }
      ctx.globalAlpha = 1;
    },
  };
}

function wipe(L, cornerId, fromColour, dur = 900) {
  const r = L.corners[cornerId];
  return {
    dur,
    draw(ctx, p) {
      if (!r) return;
      ctx.fillStyle = fromColour;
      ctx.fillRect(r.x, r.y, r.w, r.h * (1 - ease(p)));
    },
  };
}

function shake(at, name, dur = 700) {
  return {
    dur,
    draw(ctx, p) {
      const dx = Math.sin(p * 40) * 5 * (1 - p);
      drawSprite(ctx, name, at.x + dx, at.y, 4, 1 - p * 0.3);
    },
  };
}

function banner(L, text, colour, dur = 2200) {
  return {
    dur,
    draw(ctx, p) {
      ctx.globalAlpha = p < 0.15 ? p / 0.15 : p > 0.8 ? (1 - p) / 0.2 : 1;
      ctx.fillStyle = "rgba(0,0,0,0.6)";
      ctx.fillRect(0, L.H / 2 - 34, L.W, 68);
      ctx.fillStyle = colour;
      ctx.font = "bold 28px ui-monospace, monospace";
      ctx.textAlign = "center";
      ctx.fillText(text, L.W / 2, L.H / 2 + 10);
      ctx.globalAlpha = 1;
    },
  };
}

function siren(at, dur = 1100) {
  return {
    dur,
    draw(ctx, p) {
      const on = Math.floor(p * 12) % 2 === 0;
      ctx.globalAlpha = 0.35 * (1 - p);
      ctx.fillStyle = on ? "#e5484d" : "#3e8ef7";
      ctx.beginPath();
      ctx.arc(at.x, at.y, 30, 0, Math.PI * 2);
      ctx.fill();
      ctx.globalAlpha = 1;
    },
  };
}

// place is the spot a cue happens at: its corner, else its house, else
// its city, else the middle of the map.
function place(L, c) {
  return (c.corner && cornerSpot(L, c.corner)) || (c.house && houseSpot(L, c.house)) || (c.city ? cityMid(L, c.city) : cityMid(L, L.cityOrder[0]));
}

const offscreen = (L, at) => ({ x: at.x < L.W / 2 ? -30 : L.W + 30, y: at.y });

export const ANIMATIONS = {
  // A corner is yours: a flag goes up on it.
  corner_claimed(c, L) {
    const at = place(L, c);
    return [ring(at, COLOURS.player), sprite("flag", { x: at.x, y: at.y + 20 }, at, 700, { fadeIn: true }), float(at, "claimed", COLOURS.player)];
  },
  // A corner changed hands: the old owner's colour drains off it.
  corner_flip(c, L) {
    const at = place(L, c);
    const from = ownerColour(L, c.from, c.from === "rival" ? c.faction : "");
    const to = ownerColour(L, c.to, c.to === "rival" ? c.faction : "");
    return [wipe(L, c.corner, from), burst(at, [from, to]), ring(at, to)];
  },
  // Muscle went in and the corner held.
  strike(c, L) {
    const at = place(L, c);
    return [shake(at, "fist"), burst(at, ["#f76b15", "#f5c542"], 10), float(at, "held", "#f76b15")];
  },
  // A faction moves on a corner: its colour closes in.
  rival_move(c, L) {
    const at = place(L, c);
    const col = L.factionColour[c.faction] || "#e5484d";
    return [ring(at, col, 30, 1000), sprite("enforcer", offscreen(L, at), at, 1000, { hop: 6 })];
  },
  // A stick-up: a masked man runs off with the takings.
  robbery(c, L) {
    const at = place(L, c);
    return [sprite("robber", at, offscreen(L, at), 1200, { linear: true }), sprite("coin", at, { x: at.x, y: at.y + 30 }, 700, { fadeOut: true }), float(at, "robbed", "#e5484d")];
  },
  // The police: a squad car with its lights going.
  police(c, L) {
    const at = place(L, c);
    return [siren(at), sprite("copcar", offscreen(L, at), at, 1000), float(at, c.level || "police", "#3e8ef7")];
  },
  // The task force: a helicopter sweeps the city.
  task_force(c, L) {
    const head = cityHead(L, c.city);
    const r = L.cities[c.city];
    const from = { x: r ? r.x - 20 : -20, y: head.y + 40 };
    const to = { x: r ? r.x + r.w + 20 : L.W + 20, y: head.y + 40 };
    return [sprite("heli", from, to, 2000, { linear: true, scale: 4 }), float(head, "task force", "#e5484d", 2000)];
  },
  // Product on the road: loaded, landed or seized.
  shipment(c, L) {
    const path = routePath(L, c.route);
    const route = (L.view.routes || []).find((r) => r.id === c.route);
    const veh = VEHICLE[route && route.mode] || "truck";
    if (!path) return [float(cityMid(L, c.from), c.phase || "shipment", COLOURS.text)];
    const start = along(path, 0);
    const end = along(path, 1);
    if (c.phase === "seized") {
      const mid = along(path, 0.5);
      return [shake(mid, "badge", 900), burst(mid, ["#3e8ef7", "#e5484d"]), float(mid, `seized ${c.units || ""}`.trim(), "#e5484d")];
    }
    if (c.phase === "landed") return [sprite("crate", { x: end.x, y: end.y - 40 }, end, 700), ring(end, COLOURS.deed), float(end, `+${c.units || ""}`, COLOURS.deed)];
    return [sprite("crate", start, { x: start.x, y: start.y + 16 }, 600), sprite(veh, start, along(path, 0.15), 1200)];
  },
  // Someone joined the crew.
  crew_joined(c, L) {
    const at = memberSpot(L, c.member);
    return [sprite("runner", { x: at.x, y: at.y + 24 }, at, 800, { fadeIn: true, hop: 10 }), burst(at, [COLOURS.player, "#eef1f5"], 10)];
  },
  // Someone left: they walk off.
  crew_left(c, L) {
    const at = memberSpot(L, c.member);
    return [sprite("runner", at, offscreen(L, at), 1500, { linear: true, fadeOut: true }), float(at, c.phase || "gone", COLOURS.dim)];
  },
  // Arrested or shot.
  crew_down(c, L) {
    const at = c.corner ? cornerSpot(L, c.corner) || memberSpot(L, c.member) : memberSpot(L, c.member);
    if (c.dead) return [shake(at, "skull", 1400), burst(at, ["#e5484d"], 16)];
    return [siren(at), sprite("cop", offscreen(L, at), at, 900), float(at, "arrested", "#3e8ef7")];
  },
  // Back on the street.
  crew_back(c, L) {
    const at = memberSpot(L, c.member);
    return [sprite("runner", { x: at.x, y: at.y - 30 }, at, 700, { hop: 12 }), float(at, c.phase || "back", COLOURS.player)];
  },
  // Units sold: coins rise off your corners in the city.
  sale(c, L) {
    const mine = Object.values(L.corners).filter((r) => r.city === c.city && r.corner.owner === "player");
    const spots = mine.length ? mine.map((r) => cornerSpot(L, r.id)) : [cityHead(L, c.city)];
    return spots
      .slice(0, 6)
      .map((at, i) => sprite("coin", at, { x: at.x, y: at.y - 30 }, 900 + i * 90, { fadeOut: true }))
      .concat([float(cityHead(L, c.city), `sold ${c.units}`, COLOURS.deed)]);
  },
  // A price shock or a slump.
  market(c, L) {
    const at = cityHead(L, c.city);
    const up = c.phase !== "slump";
    return [float(at, `${c.product} ${up ? "▲" : "▼"}`, up ? "#e5484d" : "#3e8ef7", 1600), ring(at, up ? "#e5484d" : "#3e8ef7", 24)];
  },
  // A house or a block bought, or lost.
  property(c, L) {
    const at = place(L, c);
    if (c.phase === "bought") return [sprite("house", { x: at.x, y: at.y + 20 }, at, 800, { fadeIn: true }), ring(at, COLOURS.deed)];
    return [sprite("house", at, { x: at.x, y: at.y + 24 }, 900, { fadeOut: true }), burst(at, ["#8d5524", "#e5484d"])];
  },
  // A customer went down on bad product.
  overdose(c, L) {
    const at = place(L, c);
    return [shake(at, "skull", 1200), ring(at, "#b15bc7")];
  },
  // The run turned: the reign began or broke, going straight opened or
  // lapsed, or it ended.
  run(c, L) {
    const text = { began: "THE REIGN BEGINS", broke: "THE REIGN IS BROKEN", straight: "YOU COULD GO STRAIGHT", lapsed: "THE BOOKS SLIPPED", ended: "THE END" }[c.phase] || "THE RUN TURNS";
    return [banner(L, text, c.phase === "began" || c.phase === "straight" ? COLOURS.deed : "#e5484d")];
  },
};

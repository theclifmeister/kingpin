// Phaser owns rendering and camera controls. Every location/owner is from View;
// animations are presentation only and never advance the simulation.
const COLORS = { player: 0x7ce8bc, rival: 0xf77a7d, none: 0x628193 };
export const CUE_STYLES = {
  corner_claimed: [0x7ce8bc, "TERRITORY CLAIMED"],
  corner_flip: [0xf77a7d, "TERRITORY CHANGED"],
  strike: [0xf77a7d, "STRIKE"],
  rival_move: [0xedc479, "RIVAL ACTIVITY"],
  robbery: [0xf77a7d, "ROBBERY"],
  police: [0x87b9ff, "POLICE ACTIVITY"],
  task_force: [0x87b9ff, "TASK FORCE"],
  shipment: [0x7ce8bc, "SHIPMENT"],
  crew_joined: [0x7ce8bc, "CREW JOINED"],
  crew_left: [0xedc479, "CREW LEFT"],
  crew_down: [0xf77a7d, "CREW DOWN"],
  crew_back: [0x7ce8bc, "CREW RETURNED"],
  sale: [0x7ce8bc, "SALE"],
  market: [0xedc479, "MARKET SHIFT"],
  property: [0xedc479, "PROPERTY"],
  overdose: [0xf77a7d, "OVERDOSE"],
  run: [0xedc479, "REIGN"],
};
export class CityMap {
  constructor(parent, onSelect) {
    this.onSelect = onSelect;
    this.selected = null;
    this.city = null;
    this.ready = false;
    const owner = this;
    this.game = new Phaser.Game({
      type: Phaser.AUTO,
      parent,
      backgroundColor: "#0d1721",
      antialias: true,
      scale: {
        mode: Phaser.Scale.RESIZE,
        width: parent.clientWidth,
        height: parent.clientHeight,
      },
      scene: {
        create() {
          owner.scene = this;
          owner.ready = true;
          owner.layer = this.add.container();
          owner.fx = this.add.container();
          this.input.on("wheel", (_p, _o, _x, dy) =>
            owner.zoom(dy > 0 ? 0.9 : 1.1),
          );
          this.input.on("pointermove", (p) => {
            if (p.isDown) {
              this.cameras.main.scrollX -=
                (p.x - p.prevPosition.x) / this.cameras.main.zoom;
              this.cameras.main.scrollY -=
                (p.y - p.prevPosition.y) / this.cameras.main.zoom;
            }
          });
          this.scale.on("resize", () => owner.fit());
          if (owner.view) owner.draw();
        },
      },
    });
    this.observer = new ResizeObserver(() =>
      this.game.scale.resize(parent.clientWidth, parent.clientHeight),
    );
    this.observer.observe(parent);
  }
  setView(view, city, selected) {
    this.view = view;
    this.city = city;
    this.selected = selected;
    if (this.ready) this.draw();
  }
  zoom(by) {
    if (this.ready)
      this.scene.cameras.main.setZoom(
        Phaser.Math.Clamp(this.scene.cameras.main.zoom * by, 0.25, 2.5),
      );
  }
  focusCity(id) {
    const b = this.cityBounds?.[id];
    if (!b) return;
    const c = this.scene.cameras.main;
    c.setZoom(Math.min(c.width / (b.w + 65), c.height / (b.h + 100), 1.7));
    c.centerOn(b.x + b.w / 2, b.y + b.h / 2 + 15);
  }
  fit(all = false) {
    if (!this.ready || !this.bounds) return;
    const c = this.scene.cameras.main,
      b = this.bounds;
    if (!all && c.width < 500) {
      this.focusCity(this.city);
      return;
    }
    c.setZoom(Math.min(c.width / (b.w + 130), c.height / (b.h + 140), 1.2));
    c.centerOn(b.x + b.w / 2, b.y + b.h / 2 + 12);
  }
  text(x, y, t, size = 12, color = "#91a9ba") {
    const o = this.scene.add.text(x, y, t, {
      fontFamily: "Barlow, sans-serif",
      fontSize: size,
      color,
    });
    this.layer.add(o);
    return o;
  }
  draw() {
    const scene = this.scene;
    this.layer.removeAll(true);
    this.spots = {};
    this.citySpots = {};
    this.cityBounds = {};
    const g = scene.add.graphics();
    this.layer.add(g);
    const iso = (x, y, ox, oy) => ({
      x: ox + (x - y) * 43,
      y: oy + (x + y) * 23,
    });
    const cities = this.view.cities;
    const positions = [];
    cities.forEach((city, i) => {
      const cols = Math.max(1, ...city.corners.map((k) => k.x + 1)),
        rows = Math.max(1, ...city.corners.map((k) => k.y + 1));
      const ox = (i % 2) * 440 + rows * 43,
        oy = Math.floor(i / 2) * 480 + (i % 2) * 85;
      const poly = [
        iso(-0.7, -0.7, ox, oy),
        iso(cols + 0.4, -0.7, ox, oy),
        iso(cols + 0.4, rows + 0.4, ox, oy),
        iso(-0.7, rows + 0.4, ox, oy),
      ];
      g.fillStyle(city.id === this.city ? 0x142d34 : 0x13222d, 0.8);
      g.fillPoints(poly, true);
      g.lineStyle(1, city.id === this.city ? 0x3a766e : 0x2a3c4a, 0.8);
      g.strokePoints(poly, true);
      for (let x = 0; x <= cols; x++) {
        let a = iso(x - 0.5, -0.5, ox, oy),
          b = iso(x - 0.5, rows - 0.5, ox, oy);
        g.lineStyle(7, 0x0a1520, 1);
        g.lineBetween(a.x, a.y, b.x, b.y);
        g.lineStyle(1, 0x254050, 0.65);
        g.lineBetween(a.x, a.y, b.x, b.y);
      }
      for (let y = 0; y <= rows; y++) {
        let a = iso(-0.5, y - 0.5, ox, oy),
          b = iso(cols - 0.5, y - 0.5, ox, oy);
        g.lineStyle(7, 0x0a1520, 1);
        g.lineBetween(a.x, a.y, b.x, b.y);
        g.lineStyle(1, 0x254050, 0.65);
        g.lineBetween(a.x, a.y, b.x, b.y);
      }
      const mid = iso(cols / 2 - 0.5, rows / 2 - 0.5, ox, oy);
      this.citySpots[city.id] = mid;
      const label = this.text(
        ox,
        -55 + oy,
        city.name.toUpperCase(),
        20,
        city.id === this.city ? "#7ce8bc" : "#c3d2de",
      )
        .setOrigin(0.5, 0)
        .setInteractive({ useHandCursor: true });
      label.on("pointerup", (p) => {
        if (p.getDistance() < 8) this.onSelect(city.id, null);
      });
      this.text(
        ox,
        -28 + oy,
        `${city.id === this.view.you.city ? "YOU ARE HERE  /  " : ""}HEAT ${Math.round(city.heat)}`,
        11,
      ).setOrigin(0.5, 0);
      city.corners
        .slice()
        .sort((a, b) => a.x + a.y - b.x - b.y)
        .forEach((k, j) => {
          const p = iso(k.x, k.y, ox, oy);
          this.spots[k.id] = { ...p, city: city.id };
          const color = COLORS[k.owner] || COLORS.none;
          const height = 16 + ((j * 17 + i * 7) % 4) * 9;
          const width = 27;
          const deep = 15;
          const base = [
            { x: p.x, y: p.y - deep },
            { x: p.x + width, y: p.y },
            { x: p.x, y: p.y + deep },
            { x: p.x - width, y: p.y },
          ];
          const roof = base.map((a) => ({ x: a.x, y: a.y - height }));
          g.fillStyle(
            k.owner === "player"
              ? 0x214e48
              : k.owner === "rival"
                ? 0x4a2939
                : 0x233949,
          );
          g.fillPoints([roof[3], roof[2], base[2], base[3]], true);
          g.fillStyle(
            k.owner === "player"
              ? 0x163f38
              : k.owner === "rival"
                ? 0x352330
                : 0x192c3b,
          );
          g.fillPoints([roof[2], roof[1], base[1], base[2]], true);
          g.fillStyle(
            k.owner === "player"
              ? 0x3e8070
              : k.owner === "rival"
                ? 0x854651
                : 0x3c5567,
          );
          g.fillPoints(roof, true);
          g.lineStyle(
            k.id === this.selected ? 3 : 1,
            k.id === this.selected ? 0xffffff : color,
            k.owner === "none" ? 0.4 : 0.85,
          );
          g.strokePoints(roof, true);
          for (let floor = 7; floor < height - 3; floor += 9) {
            g.lineStyle(2, color, 0.55);
            g.lineBetween(p.x - 20, p.y - floor - 5, p.x - 6, p.y - floor + 3);
            g.lineBetween(p.x + 6, p.y - floor + 3, p.x + 20, p.y - floor - 5);
          }
          if (k.deed) {
            g.lineStyle(2, 0xedc479);
            g.strokePoints(base, true);
          }
          if (k.runner) {
            g.fillStyle(0x7ce8bc);
            g.fillCircle(p.x - 30, p.y + 10, 3);
          }
          const hit = scene.add
            .zone(p.x, p.y - height / 2, 76, 55 + height)
            .setInteractive({ useHandCursor: true });
          this.layer.add(hit);
          hit.on("pointerup", (pointer) => {
            if (pointer.getDistance() < 8) this.onSelect(city.id, k.id);
          });
          hit.on("pointerover", () => {
            this.tooltip?.destroy();
            this.tooltip = this.text(
              p.x,
              p.y - height - 27,
              `${k.name} · ${k.owner === "player" ? "Yours" : k.owner === "rival" ? "Rival" : "Unclaimed"}`,
              12,
              "#ffffff",
            )
              .setOrigin(0.5)
              .setBackgroundColor("#0a1520")
              .setPadding(7)
              .setDepth(100);
          });
          hit.on("pointerout", () => {
            this.tooltip?.destroy();
            this.tooltip = null;
          });
          positions.push(...base, ...roof);
        });
      const px = poly.map((p) => p.x),
        py = poly.map((p) => p.y);
      this.cityBounds[city.id] = {
        x: Math.min(...px),
        y: oy - 65,
        w: Math.max(...px) - Math.min(...px),
        h: Math.max(...py) - oy + 65,
      };
      positions.push(...poly, { x: ox, y: oy - 65 });
    });
    // Routes are engine topology, behind the district layer.
    const roads = scene.add.graphics();
    this.layer.addAt(roads, 0);
    this.routePaths = {};
    const roadTop = Math.max(...positions.map((p) => p.y)) + 32;
    for (const [index, r] of this.view.routes.entries()) {
      const a = this.citySpots[r.from],
        b = this.citySpots[r.to];
      if (!a || !b) continue;
      const lane = roadTop + index * 32;
      const points = [a, { x: a.x + 60, y: lane }, { x: b.x - 60, y: lane }, b];
      roads.lineStyle(7, 0x0a131d, 0.9);
      roads.strokePoints(points);
      roads.lineStyle(
        2,
        r.closed ? 0x754050 : r.dial === "off" ? 0x334a5e : 0x72bca5,
        0.8,
      );
      roads.strokePoints(points);
      this.routePaths[r.id] = points;
      positions.push(...points);
      this.text(
        (a.x + b.x) / 2,
        lane - 16,
        `${r.name.toUpperCase()} / ${r.dial.toUpperCase()}`,
        10,
        r.dial === "off" ? "#688194" : "#7ce8bc",
      ).setOrigin(0.5, 0);
    }
    for (const sh of this.view.shipments) {
      const a = this.citySpots[sh.from],
        b = this.citySpots[sh.to];
      if (!a || !b) continue;
      const p = Phaser.Math.Clamp(
        (this.view.day - sh.sent) / Math.max(1, sh.arrives - sh.sent),
        0,
        1,
      );
      g.fillStyle(0xedc479);
      const spot = this.along(sh.route, p);
      g.fillCircle(spot.x, spot.y, 5);
    }
    const xs = positions.map((p) => p.x),
      ys = positions.map((p) => p.y);
    const bounds = {
      x: Math.min(...xs),
      y: Math.min(...ys),
      w: Math.max(...xs) - Math.min(...xs),
      h: Math.max(...ys) - Math.min(...ys),
    };
    const first = !this.bounds;
    this.bounds = bounds;
    if (first) {
      this.fit();
      if (scene.cameras.main.width < 500) this.focusCity(this.city);
    }
  }
  along(route, progress) {
    const points = this.routePaths[route];
    if (!points) return this.citySpots[this.view.you.city];
    const lengths = points
      .slice(1)
      .map((p, i) => Math.hypot(p.x - points[i].x, p.y - points[i].y));
    let distance = progress * lengths.reduce((a, b) => a + b, 0);
    for (let i = 0; i < lengths.length; i++) {
      if (distance <= lengths[i] || i === lengths.length - 1) {
        const t = Math.min(1, distance / Math.max(1, lengths[i]));
        return {
          x: points[i].x + (points[i + 1].x - points[i].x) * t,
          y: points[i].y + (points[i + 1].y - points[i].y) * t,
        };
      }
      distance -= lengths[i];
    }
  }
  play(cues) {
    if (!this.ready || matchMedia("(prefers-reduced-motion: reduce)").matches)
      return;
    cues.forEach((cue, i) =>
      this.scene.time.delayedCall(i * 90, () => {
        const style = CUE_STYLES[cue.kind];
        if (!style) return;
        const member = this.view.crew.find((m) => m.id === cue.member);
        const p =
          this.spots[cue.corner] ||
          this.spots[member?.post] ||
          this.citySpots[cue.city || member?.city || this.view.you.city];
        if (!p) return;
        const ring = this.scene.add
          .circle(p.x, p.y, 12, style[0], 0.18)
          .setStrokeStyle(2, style[0]);
        this.fx.add(ring);
        this.scene.tweens.add({
          targets: ring,
          scale: 5,
          alpha: 0,
          duration: 1100,
          onComplete: () => ring.destroy(),
        });
        const label = this.scene.add
          .text(
            p.x,
            p.y - 30,
            cue.kind === "sale" ? `+ ${cue.units} SOLD` : style[1],
            {
              fontSize: "13px",
              fontFamily: "Barlow, sans-serif",
              color: "#" + style[0].toString(16),
              backgroundColor: "#0b1721",
            },
          )
          .setOrigin(0.5)
          .setPadding(5);
        this.fx.add(label);
        this.scene.tweens.add({
          targets: label,
          y: p.y - 85,
          alpha: 0,
          delay: 500,
          duration: 1200,
          onComplete: () => label.destroy(),
        });
        if (cue.kind === "shipment") {
          const a = this.citySpots[cue.from],
            b = this.citySpots[cue.to];
          if (a && b) {
            const dot = this.scene.add.circle(a.x, a.y, 6, style[0]);
            this.fx.add(dot);
            dot.progress = 0;
            this.scene.tweens.add({
              targets: dot,
              progress: 1,
              duration: 1800,
              onUpdate: () => {
                const p = this.along(cue.route, dot.progress);
                dot.setPosition(p.x, p.y);
              },
              onComplete: () => dot.destroy(),
            });
          }
        }
      }),
    );
  }
  clearEffects() {
    if (!this.ready) return;
    this.scene.time.removeAllEvents();
    this.scene.tweens.killAll();
    this.fx.removeAll(true);
  }
}

// The protocol, as the client speaks it (#328). A Session wraps one
// kingpin.open() of the WebAssembly module: it sends a request line,
// reads the lines it produced (the event and view notifications, then
// the response) and keeps what they said. It touches no DOM, so the
// Node test plays through the very same code the page does.

// The versions this client is written against. A module with another
// protocol or view version is refused before a run starts: a field
// renamed under the client would draw a wrong game, not fail.
export const SUPPORTED = { protocol: [2], view: [1] };

export class VersionError extends Error {}

// checkVersions refuses a module whose protocol or view version this
// client does not know.
export function checkVersions(kingpin) {
  if (!SUPPORTED.protocol.includes(kingpin.protocol) || !SUPPORTED.view.includes(kingpin.view)) {
    throw new VersionError(
      `this client speaks protocol ${SUPPORTED.protocol.join("/")} and view ${SUPPORTED.view.join("/")}; ` +
        `the engine is protocol ${kingpin.protocol}, view ${kingpin.view}`,
    );
  }
}

// RPCError is a call the engine answered with an error. refused is the
// game saying no (-32000): a move the rules do not allow, its message in
// the game's words.
export class RPCError extends Error {
  constructor(method, error) {
    super(error.message);
    this.method = method;
    this.code = error.code;
    this.refused = error.code === -32000;
  }
}

export class Session {
  constructor(kingpin) {
    checkVersions(kingpin);
    this.s = kingpin.open();
    this.next = 0;
    this.view = null;
    this.pending = []; // events since the last take(), as the wire sent them
  }

  // call sends one request and returns its result, keeping the
  // notifications that came before it.
  call(method, ...params) {
    const id = ++this.next;
    const lines = this.s.handle(JSON.stringify({ jsonrpc: "2.0", id, method, params }));
    if (lines instanceof Error) throw lines;
    let answer = null;
    for (const line of lines) {
      const msg = JSON.parse(line);
      if (msg.method === "event") this.pending.push(msg.params);
      else if (msg.method === "view") this.view = msg.params;
      else answer = msg;
    }
    if (!answer || answer.id !== id) throw new Error(`${method}: no answer to request ${id}`);
    if (answer.error) throw new RPCError(method, answer.error);
    return answer.result;
  }

  // take hands over the events received since the last take.
  take() {
    const out = this.pending;
    this.pending = [];
    return out;
  }

  newRun(seed, character = "", hardDA = false) {
    this.view = this.call("new_run", seed, character, hardDA);
    return this.view;
  }
  refresh() {
    this.view = this.call("view");
    return this.view;
  }
  endDay() {
    return this.call("end_day");
  }
  fastForward(days) {
    return this.call("fast_forward", days);
  }
  choose(i) {
    return this.call("choose", i);
  }
  travel(city) {
    return this.call("travel", city);
  }
  buy(supplier, product, qty) {
    return this.call("buy", supplier, product, qty, false);
  }
  sell(city, product, qty, dial) {
    return this.call("place_sell", city, product, qty, dial);
  }
  exportSave() {
    return this.call("export_save");
  }
  importSave(b64) {
    this.view = this.call("import_save", b64);
    return this.view;
  }
}

// streetConnect is the connect in the city that sells to you on the
// street today: open, not the wholesaler.
export function streetConnect(view, city) {
  return (view.connects || []).find((k) => k.city === city && k.open && !k.wholesale) || null;
}

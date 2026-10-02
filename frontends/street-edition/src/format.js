// How Street Edition writes a number, as the TUI's internal/format does
// (#589): one money formatter for every module, so a negative reads
// -$30,040 everywhere, and Go's rounding where a figure must match the
// TUI digit for digit. Pure: no DOM, no view.

// fixed is Go's %.*f: toFixed, but an exact tie goes to the even digit
// as Go rounds it ($12.5M is $12M in the TUI, where toFixed says $13M).
// toFixed(100) is the float's exact decimal, so a tie is a 5 and zeros.
export function fixed(x, d) {
  const s = x.toFixed(d);
  const exact = Math.abs(x).toFixed(100),
    rest = exact.slice(exact.indexOf(".") + 1 + d);
  if (!/^50*$/.test(rest)) return s;
  const down = (Math.trunc(Math.abs(x) * 10 ** d) / 10 ** d).toFixed(d), // the digits kept, unrounded
    last = Number(down[down.length - 1]);
  return last % 2 === 0 ? (x < 0 ? "-" : "") + down : s;
}

// money is format.Money: whole dollars with separators, the sign before
// the $ (-$30,040). It rounds first, so -$0.40 is $0.
export function money(n) {
  const r = Math.round(n || 0);
  return (r < 0 ? "-$" : "$") + Math.abs(r).toLocaleString("en-US");
}

// price is format.Price: cents under $1,000, whole dollars above.
export const price = (n) => (n < 1000 ? "$" + fixed(n || 0, 2) : money(n));

// cash is a big amount the short way, as format.Cash writes it: under
// $10,000 in full, then $12K, $1.2M, $3.4B.
export function cash(n) {
  n = Math.round(n || 0);
  if (n > -10_000 && n < 10_000) return money(n);
  const sign = n < 0 ? "-" : "",
    units = ["K", "M", "B", "T"];
  let x = Math.abs(n) / 1000,
    i = 0;
  while (i < units.length - 1 && Math.round(x) >= 1000) (x /= 1000), i++; // $999,600 reads $1.0M
  return `${sign}$${fixed(x, x < 10 ? 1 : 0)}${units[i]}`;
}

// pct is format.Pct, a fraction as a percent to d decimals; pctText is
// ui.pctText, a percent already, to one decimal under 10 and none above.
export const pct = (x, d = 0) => fixed((x || 0) * 100, d) + "%",
  pctText = (f) => fixed(f, f < 10 && f > -10 ? 1 : 0) + "%";

// Presentation policy over public engine quotes; no duplicated game thresholds.
export function readRisk(session) {
  const view = session.view;
  const limit = session.call("rules.heat.evidence_arrest");
  const cities = view.cities.map((city) => {
    const ladder = session
      .call("rules.heat.ladder", city.id)
      .slice()
      .sort((a, b) => a.Threshold - b.Threshold);
    const reached = ladder.filter((r) => city.heat >= r.Threshold).at(-1);
    const next = ladder.find((r) => city.heat < r.Threshold);
    return { ...city, ladder, reached, next };
  });
  const hottest = cities.reduce((a, b) => (a.heat >= b.heat ? a : b));
  const evidence = view.you.evidence;
  const remaining = limit > 0 ? Math.max(0, limit - evidence) : null;
  const responsePages = Math.max(
    1,
    ...cities.flatMap((c) => c.ladder.map((r) => r.Evidence)),
  );
  // Warn before one ordinary evidence-producing response could finish the file.
  // This is an attention threshold, not a forecast or a guarantee of safety.
  const evidenceCritical =
    limit > 0 && evidence > 0 && remaining <= responsePages;
  const heatCritical = cities.some(
    (c) => c.reached && c.reached.Level !== "patrol",
  );
  const critical = evidenceCritical || heatCritical;
  const caution = evidence > 0 || cities.some((c) => c.reached);
  const warnings = [];
  if (evidenceCritical)
    warnings.push(
      remaining === 0
        ? `The DA's evidence has reached the indictment threshold (${evidence}/${limit}).`
        : `${remaining} more evidence ${remaining === 1 ? "point" : "points"} would reach indictment (${evidence}/${limit}). A sting or raid can add evidence.`,
    );
  else if (evidence > 0)
    warnings.push(
      `The DA has ${evidence}/${limit > 0 ? limit : "—"} evidence. Heat cooling down does not clear this file.`,
    );
  for (const c of cities.filter((c) => c.reached))
    warnings.push(
      `${c.name}: heat ${c.heat.toFixed(1)} has reached the ${c.reached.Level} line (${c.reached.Threshold.toFixed(1)}).`,
    );
  return {
    cities,
    hottest,
    evidence,
    limit,
    remaining,
    critical,
    caution,
    evidenceCritical,
    heatCritical,
    warnings,
  };
}

// Keep the engine's stage/card/event/alert stops, adding a frontend danger stop
// after every day so a seven-day click cannot run past a visible critical state.
export function safeFastForward(session, days) {
  let ran = 0;
  while (ran < days) {
    if (readRisk(session).critical)
      return { ran, stop: "danger", day: session.view.day };
    const result = session.call("fast_forward", 1);
    ran += result.ran;
    if (result.stop !== "cap" || !result.ran) return { ...result, ran };
    if (readRisk(session).critical) return { ...result, ran, stop: "danger" };
  }
  return { ran, stop: "cap", day: session.view.day };
}

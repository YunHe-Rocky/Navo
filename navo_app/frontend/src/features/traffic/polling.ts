const healthyMetricsPollDelay = 2_000;
const maximumMetricsPollDelay = 30_000;

export function metricsPollDelay(failures: number): number {
  const normalized = Number.isFinite(failures)
    ? Math.max(0, Math.floor(failures))
    : 0;
  if (normalized === 0) return healthyMetricsPollDelay;
  return Math.min(maximumMetricsPollDelay, healthyMetricsPollDelay * (2 ** Math.min(normalized, 4)));
}

const captureTransitionStates = new Set([
  "starting_system_proxy",
  "starting_tun",
  "stopping",
  "recovering",
]);

export function shouldRecordTrafficSample(captureState: string): boolean {
  return !captureTransitionStates.has(captureState.trim().toLowerCase());
}

export function trafficRouteUpdate(
  previousRouteID: string,
  currentRouteID: string,
  captureState: string,
): { routeID: string; reset: boolean } {
  const previous = previousRouteID.trim();
  const current = currentRouteID.trim();
  if (!shouldRecordTrafficSample(captureState) || !current) return { routeID: previous, reset: false };
  return { routeID: current, reset: Boolean(previous && previous !== current) };
}

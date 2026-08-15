export const CLOSE_PREFERENCE_KEY = "navo-close-preference-v1";
export const CLOSE_PREFERENCE_TTL_MS = 24 * 60 * 60 * 1000;

function currentBootStartedAt(now, systemUptimeSeconds) {
  if (!Number.isFinite(systemUptimeSeconds) || systemUptimeSeconds < 0) return 0;
  return now - systemUptimeSeconds * 1000;
}

export function createClosePreference(action, now, systemUptimeSeconds) {
  if (action !== "minimize" && action !== "exit") throw new Error("invalid close action");
  return JSON.stringify({
    action,
    bootStartedAt: currentBootStartedAt(now, systemUptimeSeconds),
    expiresAt: now + CLOSE_PREFERENCE_TTL_MS,
  });
}

export function resolveClosePreference(serialized, now, systemUptimeSeconds) {
  if (!serialized) return undefined;
  try {
    const stored = JSON.parse(serialized);
    if (stored.action !== "minimize" && stored.action !== "exit") return undefined;
    if (!Number.isFinite(stored.expiresAt) || stored.expiresAt <= now) return undefined;
    const bootStartedAt = currentBootStartedAt(now, systemUptimeSeconds);
    // The derived boot timestamp can drift slightly as wall-clock and uptime
    // samples are read at different moments; a two-minute tolerance is ample.
    if (!bootStartedAt || Math.abs(stored.bootStartedAt - bootStartedAt) > 120000) return undefined;
    return stored.action;
  } catch {
    return undefined;
  }
}

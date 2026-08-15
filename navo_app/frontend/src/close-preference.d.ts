export type CloseAction = "minimize" | "exit";
export const CLOSE_PREFERENCE_KEY: string;
export const CLOSE_PREFERENCE_TTL_MS: number;
export function createClosePreference(action: CloseAction, now: number, systemUptimeSeconds: number): string;
export function resolveClosePreference(serialized: string | null, now: number, systemUptimeSeconds: number): CloseAction | undefined;

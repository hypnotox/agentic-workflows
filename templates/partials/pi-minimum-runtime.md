export const MIN_PI_VERSION = "0.81.1";
const MINIMUM_RUNTIME_NOTICE = Symbol.for("awf.pi.minimum-runtime-notified");
export interface MinimumRuntimeDependencies { packageVersion: string; }
function parseVersion(value: string): [number, number, number] | undefined {
  const match = /^(\d+)\.(\d+)\.(\d+)(?:[-+].*)?$/.exec(value);
  return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : undefined;
}
export function versionSupported(value: string): boolean {
  const actual = parseVersion(value);
  const minimum = parseVersion(MIN_PI_VERSION)!;
  if (!actual) return false;
  for (let i = 0; i < minimum.length; i++) {
    if (actual[i] !== minimum[i]) return actual[i] > minimum[i];
  }
  return true;
}
export function guardMinimumRuntime(pi: ExtensionAPI, deps: MinimumRuntimeDependencies): boolean {
  const requirements = {
    queueCommand: typeof pi.queueCommand === "function",
    eventHooks: typeof pi.on === "function" && typeof pi.events?.on === "function" && typeof pi.events?.emit === "function",
    persistedEntries: typeof pi.appendEntry === "function",
    tools: typeof pi.registerTool === "function",
    overlayCommands: typeof pi.registerCommand === "function",
    shutdownHooks: typeof pi.on === "function",
  };
  const missing = Object.entries(requirements).filter(([, available]) => !available).map(([name]) => name);
  if (versionSupported(deps.packageVersion) && missing.length === 0) return true;
  if (typeof pi.on !== "function") return false;
  pi.on("session_start", async (_event, ctx) => {
    if ((globalThis as any)[MINIMUM_RUNTIME_NOTICE]) return;
    (globalThis as any)[MINIMUM_RUNTIME_NOTICE] = true;
    const missingAPI = missing.length > 0 ? ` Missing runtime APIs: ${missing.join(", ")}.` : "";
    ctx.ui.notify(`awf Pi extensions require Pi ${MIN_PI_VERSION} or newer with event hooks, persisted custom entries, tools, widget/overlay commands, and shutdown hooks; found ${deps.packageVersion}.${missingAPI} Upgrade Pi and reload.`, "error");
  });
  return false;
}

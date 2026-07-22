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
  const queueCommandMissing = typeof pi.queueCommand !== "function";
  if (versionSupported(deps.packageVersion) && !queueCommandMissing) return true;
  pi.on("session_start", async (_event, ctx) => {
    if ((globalThis as any)[MINIMUM_RUNTIME_NOTICE]) return;
    (globalThis as any)[MINIMUM_RUNTIME_NOTICE] = true;
    const missingAPI = queueCommandMissing ? " ExtensionAPI.queueCommand is missing." : "";
    ctx.ui.notify(`awf Pi extensions require Pi ${MIN_PI_VERSION} or newer with ExtensionAPI.queueCommand; found ${deps.packageVersion}.${missingAPI} Upgrade Pi and reload.`, "error");
  });
  return false;
}

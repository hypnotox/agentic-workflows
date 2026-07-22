export const MIN_PI_VERSION = "0.81.1";
const MINIMUM_RUNTIME_NOTICE = Symbol.for("awf.pi.minimum-runtime-notified");
export interface MinimumRuntimeDependencies { packageVersion: string; }
export type MinimumRuntimeAPI = "on" | "eventsOn" | "eventsEmit" | "appendEntry" | "registerTool" | "registerCommand" | "queueCommand" | "exec" | "getThinkingLevel";
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
export function guardMinimumRuntime(pi: ExtensionAPI, deps: MinimumRuntimeDependencies, required: readonly MinimumRuntimeAPI[]): boolean {
  const requirements: Record<MinimumRuntimeAPI, boolean> = {
    on: typeof pi.on === "function", eventsOn: typeof pi.events?.on === "function", eventsEmit: typeof pi.events?.emit === "function",
    appendEntry: typeof pi.appendEntry === "function", registerTool: typeof pi.registerTool === "function", registerCommand: typeof pi.registerCommand === "function",
    queueCommand: typeof pi.queueCommand === "function", exec: typeof pi.exec === "function", getThinkingLevel: typeof pi.getThinkingLevel === "function",
  };
  const missing = required.filter((name) => !requirements[name]);
  if (versionSupported(deps.packageVersion) && missing.length === 0) return true;
  if (typeof pi.on !== "function") return false;
  pi.on("session_start", async (_event, ctx) => {
    if ((globalThis as any)[MINIMUM_RUNTIME_NOTICE]) return;
    (globalThis as any)[MINIMUM_RUNTIME_NOTICE] = true;
    const missingAPI = missing.length > 0 ? ` Missing runtime APIs: ${missing.join(", ")}.` : "";
    ctx.ui.notify(`awf Pi extensions require Pi ${MIN_PI_VERSION} or newer with their factory event, persistence, tool, command, process, and thinking APIs; found ${deps.packageVersion}.${missingAPI} Upgrade Pi and reload.`, "error");
  });
  return false;
}

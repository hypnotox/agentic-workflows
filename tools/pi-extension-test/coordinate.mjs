// Coordinates the complete host lane. The lock is a hard link to a fully-written
// owner record, so no contender can observe an unattributable acquired lock.
import { existsSync, linkSync, readFileSync, rmSync, unlinkSync, writeFileSync } from "node:fs";
import { randomBytes } from "node:crypto";
import { spawn } from "node:child_process";

const [runner, root, toolDir] = process.argv.slice(2);
const lock = `${toolDir}/.host-lane.lock`;
const token = `${process.pid}-${randomBytes(8).toString("hex")}`;
const owner = `${toolDir}/.host-lane.owner-${token}`;
const gate = `${toolDir}/.host-lane.start-${token}`;
const pause = () => new Promise((resolve) => setTimeout(resolve, 100));
const alive = (pid) => {
  try { process.kill(pid, 0); return true; } catch { return false; }
};
const lockIsLive = () => {
  try {
    const state = JSON.parse(readFileSync(lock, "utf8"));
    return alive(state.manager) || (Number.isInteger(state.group) && alive(-state.group));
  } catch { return false; }
};
const remove = (path) => { try { unlinkSync(path); } catch {} };
let child;
let ownsLock = false;
const cleanup = (removeLock = true) => {
  remove(gate);
  if (ownsLock && removeLock) remove(lock);
  remove(owner);
};
for (const signal of ["SIGINT", "SIGTERM", "SIGHUP"]) {
  process.on(signal, () => {
    if (child?.pid) { try { process.kill(-child.pid, signal); } catch {} }
    // Keep the record if a descendant survives the signal; a later contender
    // may reclaim it only after the recorded process group is gone.
    cleanup(false);
    process.exit(128);
  });
}
while (true) {
  // The worker cannot start until the manager has atomically published both
  // identities. It also exits before work if this manager disappears.
  child = spawn("bash", [runner], {
    cwd: root,
    detached: true,
    stdio: "inherit",
    env: { ...process.env, AWF_PI_TEST_WORKER: "1", AWF_PI_TEST_MANAGER_PID: String(process.pid), AWF_PI_TEST_START_GATE: gate, AWF_PI_TEST_WORKER_PGID: "pending" },
  });
  const state = JSON.stringify({ manager: process.pid, group: child.pid });
  writeFileSync(owner, state, { mode: 0o600 });
  try {
    linkSync(owner, lock); // atomic create-or-observe ownership publication
    ownsLock = true;
    remove(owner); // the lock now is the sole link to its immutable record
    writeFileSync(gate, `${child.pid}\n`, { mode: 0o600 });
    break;
  } catch (error) {
    try { process.kill(-child.pid, "SIGTERM"); } catch {}
    remove(owner);
    if (error.code !== "EEXIST") throw error;
    if (!lockIsLive()) remove(lock);
    await pause();
  }
}
child.on("exit", (code, signal) => {
  cleanup();
  process.exitCode = code ?? (signal ? 1 : 0);
});

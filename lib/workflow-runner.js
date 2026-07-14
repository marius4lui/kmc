import { spawn, spawnSync } from "node:child_process";
import path from "node:path";
import { selectSteps } from "./workflows.js";

const WINDOWS = process.platform === "win32";

function executableExists(command) {
  const checker = WINDOWS ? "where" : "which";
  return spawnSync(checker, [command], { stdio: "ignore" }).status === 0;
}

export function resolveShell(requested) {
  const shell = requested || (WINDOWS ? (executableExists("pwsh") ? "pwsh" : "powershell") : (process.env.SHELL || "sh"));
  const name = path.basename(shell).toLowerCase();
  const allowed = WINDOWS ? ["powershell", "powershell.exe", "pwsh", "pwsh.exe", "cmd", "cmd.exe"] : ["bash", "sh", "zsh", "dash", "ksh"];
  if (!allowed.includes(name)) throw new Error(`Shell "${shell}" is not supported on this platform.`);
  if (!executableExists(shell)) throw new Error(`Shell "${shell}" was not found on this system.`);
  if (["cmd", "cmd.exe"].includes(name)) return { command: shell, args: ["/d", "/s", "/c"] };
  if (["powershell", "powershell.exe", "pwsh", "pwsh.exe"].includes(name)) return { command: shell, args: ["-NoLogo", "-NonInteractive", "-Command"] };
  return { command: shell, args: ["-c"] };
}

function killProcess(child) {
  if (!child.pid) return;
  try {
    if (!WINDOWS) process.kill(-child.pid, "SIGTERM");
    else spawnSync("taskkill", ["/pid", String(child.pid), "/t", "/f"], { stdio: "ignore" });
  } catch { child.kill("SIGTERM"); }
}

export function executeCommand({ command, shell, cwd, env, timeout, signal }) {
  return new Promise((resolve, reject) => {
    const resolved = resolveShell(shell);
    const child = spawn(resolved.command, [...resolved.args, command], {
      cwd, env, stdio: "inherit", detached: !WINDOWS, windowsHide: true
    });
    let timedOut = false;
    const timer = timeout ? setTimeout(() => { timedOut = true; killProcess(child); }, timeout * 1000) : null;
    const abort = () => killProcess(child);
    signal?.addEventListener("abort", abort, { once: true });
    child.once("error", reject);
    child.once("exit", (code, exitSignal) => {
      if (timer) clearTimeout(timer);
      signal?.removeEventListener("abort", abort);
      resolve({ code: code ?? (signal?.aborted ? 130 : 1), signal: exitSignal, timedOut });
    });
  });
}

export function buildPlan(script, projectRoot, { step, env = {} } = {}) {
  return selectSteps(script.workflow, step).map((item) => ({
    ...item,
    cwd: path.resolve(projectRoot, item.cwd ?? script.workflow.defaults?.cwd ?? "."),
    shell: item.shell ?? script.workflow.defaults?.shell,
    env: { ...process.env, ...scalarEnv(script.workflow.env), ...scalarEnv(item.env), ...env },
    retries: item.retries ?? 0,
    continueOnError: item.continue_on_error ?? false
  }));
}

function scalarEnv(env = {}) {
  return Object.fromEntries(Object.entries(env).map(([key, value]) => [key, value === null ? "" : String(value)]));
}

export async function runWorkflow(script, projectRoot, options = {}) {
  const plan = buildPlan(script, projectRoot, options);
  if (options.dryRun) return { ok: true, dryRun: true, plan };
  const results = [];
  for (const item of plan) {
    options.onEvent?.({ type: "stepStarted", step: item });
    let result;
    for (let attempt = 0; attempt <= item.retries; attempt++) {
      if (attempt) options.onEvent?.({ type: "stepRetried", step: item, attempt });
      result = await executeCommand({ command: item.run, shell: item.shell, cwd: item.cwd, env: item.env, timeout: item.timeout, signal: options.signal });
      if (result.code === 0 && !result.timedOut) break;
    }
    results.push({ step: item, ...result });
    options.onEvent?.({ type: result.code === 0 && !result.timedOut ? "stepSucceeded" : "stepFailed", step: item, result });
    if ((result.code !== 0 || result.timedOut) && !item.continueOnError) return { ok: false, results, plan, exitCode: result.timedOut ? 124 : result.code };
  }
  return { ok: true, results, plan };
}

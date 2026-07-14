import { mkdir, writeFile } from "node:fs/promises";
import path from "node:path";
import { confirm } from "@inquirer/prompts";
import { color, setColorsEnabled } from "./theme.js";
import { runWorkflow, buildPlan, resolveShell } from "./workflow-runner.js";
import { loadWorkflows, WorkflowValidationError } from "./workflows.js";
import { trustRepository, trustStatus, untrustRepository } from "./trust.js";

function parseRunOptions(args) {
  const options = { env: {} };
  for (let index = 0; index < args.length; index++) {
    const arg = args[index];
    if (arg === "--dry-run") options.dryRun = true;
    else if (arg === "--verbose") options.verbose = true;
    else if (arg === "--no-color") options.noColor = true;
    else if (arg === "--yes" || arg === "-y") options.yes = true;
    else if (arg === "--step") options.step = requiredValue(args, ++index, arg);
    else if (arg === "--env") {
      const pair = requiredValue(args, ++index, arg);
      const equals = pair.indexOf("=");
      if (equals < 1) throw new Error(`--env expects KEY=value, received "${pair}".`);
      options.env[pair.slice(0, equals)] = pair.slice(equals + 1);
    } else throw new Error(`Unknown run option "${arg}".`);
  }
  return options;
}

function requiredValue(args, index, option) {
  if (!args[index] || args[index].startsWith("--")) throw new Error(`${option} needs a value.`);
  return args[index];
}

async function ensureTrusted(loaded, options) {
  const status = await trustStatus(loaded);
  if (status.trusted) return;
  if (options.yes) { await trustRepository(loaded); return; }
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    throw new Error(`Repository is not trusted${status.changed ? " because its KMC scripts changed" : ""}. Review it and run "kmc trust" first.`);
  }
  console.log(`This repository contains KMC scripts that can execute local commands.\n\nRepository:\n${loaded.projectRoot}\n\nReview the files before continuing:`);
  console.log(`  ${path.relative(loaded.projectRoot, loaded.registryFile)}`);
  for (const script of loaded.scripts.values()) console.log(`  ${path.relative(loaded.projectRoot, script.file)}`);
  console.log("");
  const accepted = await confirm({ message: "Trust and execute scripts from this repository?", default: false });
  if (!accepted) throw new Error("Repository was not trusted; no commands were executed.");
  await trustRepository(loaded);
}

export async function listScripts(root = process.cwd()) {
  const loaded = await loadWorkflows(root, { validateCwds: false });
  console.log("Available scripts:\n");
  for (const script of loaded.scripts.values()) console.log(`  ${script.id.padEnd(12)} ${script.description}`.trimEnd());
}

export async function validateScripts(root = process.cwd()) {
  try {
    const loaded = await loadWorkflows(root);
    console.log(`${color.green("✓")} ${path.relative(root, loaded.registryFile)}`);
    for (const script of loaded.scripts.values()) {
      for (const step of script.workflow.steps) resolveShell(step.shell ?? script.workflow.defaults?.shell);
      console.log(`${color.green("✓")} ${script.id}`);
    }
    console.log("\nAll KMC scripts are valid.");
    return true;
  } catch (error) {
    if (!(error instanceof WorkflowValidationError)) throw error;
    for (const issue of error.errors) console.error(`${color.red("✗")} ${issue}`);
    process.exitCode = 1;
    return false;
  }
}

export async function runScript(id, args = [], root = process.cwd()) {
  if (!id) throw new Error("A script name is required.");
  const options = parseRunOptions(args);
  if (options.noColor) setColorsEnabled(false);
  const loaded = await loadWorkflows(root);
  const script = loaded.scripts.get(id);
  if (!script) throw new Error(`Script "${id}" was not found.`);
  const plan = buildPlan(script, loaded.projectRoot, options);
  if (options.dryRun) {
    console.log(`Dry run: ${id}\n`);
    plan.forEach((step, index) => console.log(`${index + 1}. ${step.name}\n   cwd: ${step.cwd}\n   shell: ${step.shell ?? "default"}\n   run: ${step.run.replace(/\n/g, "\n        ")}\n`));
    return;
  }
  await ensureTrusted(loaded, options);
  console.log(`KMC Script: ${id}\n`);
  const started = Date.now();
  const controller = new AbortController();
  const interrupt = () => controller.abort();
  process.once("SIGINT", interrupt);
  const result = await runWorkflow(script, loaded.projectRoot, {
    ...options,
    signal: controller.signal,
    onEvent(event) {
      if (event.type === "stepStarted") console.log(`[${event.step.index + 1}/${script.workflow.steps.length}] ${event.step.name}\n$ ${event.step.run}`);
      if (event.type === "stepRetried") console.log(color.yellow(`Retry ${event.attempt}/${event.step.retries}...`));
      if (event.type === "stepSucceeded") console.log(`${color.green("✓")} Completed\n`);
      if (event.type === "stepFailed") console.log(`${color.red("✗")} ${event.result.timedOut ? "Timed out" : `Failed with exit code ${event.result.code}`}\n`);
    }
  });
  process.removeListener("SIGINT", interrupt);
  if (!result.ok) {
    process.exitCode = controller.signal.aborted ? 130 : (result.exitCode || 1);
    throw Object.assign(new Error(`Script "${id}" failed.`), { reported: true });
  }
  console.log(`${color.green("✓")} Script completed in ${((Date.now() - started) / 1000).toFixed(1)}s`);
}

export async function initScripts(root = process.cwd()) {
  const registry = path.join(root, ".kmc", "scripts.yml");
  const workflow = path.join(root, ".kmc", "scripts", "test.yml");
  await mkdir(path.dirname(workflow), { recursive: true });
  const created = [];
  try { await writeFile(registry, "version: 1\n\nscripts:\n  test:\n    file: ./scripts/test.yml\n    description: Run project tests\n", { flag: "wx" }); created.push(path.relative(root, registry)); } catch (error) { if (error.code !== "EEXIST") throw error; }
  try { await writeFile(workflow, "name: Test project\n\nsteps:\n  - name: Install dependencies\n    run: npm install\n\n  - name: Run tests\n    run: npm test\n", { flag: "wx" }); created.push(path.relative(root, workflow)); } catch (error) { if (error.code !== "EEXIST") throw error; }
  console.log(created.length ? `Created:\n${created.map((file) => `  ${file}`).join("\n")}` : "KMC scripts are already initialized; no files changed.");
}

export async function scriptsCommand(args, root = process.cwd()) {
  const [command, ...rest] = args;
  if (command === "list") return listScripts(root);
  if (command === "validate") return validateScripts(root);
  if (command === "run") return runScript(rest[0], rest.slice(1), root);
  if (command === "init") return initScripts(root);
  throw new Error(`Unknown scripts command "${command ?? ""}". Use list, validate, run, or init.`);
}

export async function trustCommand(args, root = process.cwd()) {
  if (args[0] === "status") {
    const loaded = await loadWorkflows(root, { validateCwds: false });
    const status = await trustStatus(loaded);
    console.log(status.trusted ? "Repository is trusted." : status.changed ? "Repository scripts changed and require renewed trust." : "Repository is not trusted.");
    if (!status.trusted) process.exitCode = 1;
    return;
  }
  const loaded = await loadWorkflows(root, { validateCwds: false });
  await trustRepository(loaded);
  console.log(`Trusted KMC scripts in ${loaded.projectRoot}.`);
}

export async function untrustCommand(root = process.cwd()) {
  const removed = await untrustRepository(root);
  console.log(removed ? `Removed trust for ${path.resolve(root)}.` : "Repository was not trusted.");
}

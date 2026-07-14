import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { promisify } from "node:util";
import test from "node:test";
import { buildPlan, runWorkflow } from "../lib/workflow-runner.js";
import { loadWorkflows, WorkflowValidationError } from "../lib/workflows.js";

const execFileAsync = promisify(execFile);
const bin = new URL("../bin/kmc.js", import.meta.url).pathname;

async function project(registry, workflow) {
  const root = await mkdtemp(path.join(tmpdir(), "kmc-workflow-"));
  await mkdir(path.join(root, ".kmc", "scripts"), { recursive: true });
  await writeFile(path.join(root, ".kmc", "scripts.yml"), registry);
  if (workflow !== undefined) await writeFile(path.join(root, ".kmc", "scripts", "test.yml"), workflow);
  return root;
}

const registry = "version: 1\nscripts:\n  test:\n    file: ./scripts/test.yml\n    description: Run checks\n";

test("loads and validates workflow files", async () => {
  const root = await project(registry, "name: Checks\nenv:\n  BASE: yes\ndefaults:\n  shell: sh\nsteps:\n  - name: Test all\n    run: echo ok\n");
  const loaded = await loadWorkflows(root);
  assert.equal(loaded.scripts.get("test").workflow.steps[0].name, "Test all");
});

test("rejects unknown fields and path traversal", async () => {
  const root = await project(registry, "steps:\n  - name: nope\n    run: echo nope\n    uses: build\n    cwd: ../../\n");
  await assert.rejects(() => loadWorkflows(root), (error) => {
    assert(error instanceof WorkflowValidationError);
    assert.match(error.message, /field "uses" is not supported/);
    assert.match(error.message, /escapes the project root/);
    return true;
  });
});

test("rejects invalid timeout, retries, and duplicate names", async () => {
  const root = await project(registry, "steps:\n  - name: Same\n    run: echo 1\n    timeout: 0\n  - name: Same\n    run: echo 2\n    retries: -1\n");
  await assert.rejects(() => loadWorkflows(root), /timeout.*positive integer[\s\S]*retries.*non-negative integer[\s\S]*duplicated/);
});

test("builds selected plans with inherited environment and defaults", async () => {
  const script = { workflow: { env: { A: "workflow", B: 1 }, defaults: { shell: "sh", cwd: "sub" }, steps: [{ name: "Run tests", run: "true", env: { A: "step" } }] } };
  const plan = buildPlan(script, "/tmp/root", { step: "run-tests", env: { C: "cli" } });
  assert.equal(plan[0].shell, "sh");
  assert.equal(plan[0].cwd, "/tmp/root/sub");
  assert.deepEqual({ A: plan[0].env.A, B: plan[0].env.B, C: plan[0].env.C }, { A: "step", B: "1", C: "cli" });
});

test("runner retries and honors continue_on_error", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "kmc-runner-"));
  const marker = path.join(root, "marker");
  const script = { workflow: { steps: [
    { name: "retry", run: `test -f '${marker}' || (touch '${marker}'; exit 1)`, shell: "sh", retries: 1 },
    { name: "ignored", run: "exit 2", shell: "sh", continue_on_error: true },
    { name: "finish", run: "echo done", shell: "sh" }
  ] } };
  const result = await runWorkflow(script, root);
  assert.equal(result.ok, true);
  assert.equal(result.results.length, 3);
});

test("runner reports timeouts without running later steps", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "kmc-timeout-"));
  const script = { workflow: { steps: [{ name: "slow", run: "sleep 2", shell: "sh", timeout: 1 }, { name: "later", run: "true", shell: "sh" }] } };
  const result = await runWorkflow(script, root);
  assert.equal(result.ok, false);
  assert.equal(result.exitCode, 124);
  assert.equal(result.results.length, 1);
});

test("scripts init never overwrites existing files", async () => {
  const root = await mkdtemp(path.join(tmpdir(), "kmc-init-"));
  await execFileAsync("node", [bin, "scripts", "init"], { cwd: root });
  const first = await readFile(path.join(root, ".kmc", "scripts.yml"), "utf8");
  await execFileAsync("node", [bin, "scripts", "init"], { cwd: root });
  assert.equal(await readFile(path.join(root, ".kmc", "scripts.yml"), "utf8"), first);
});

test("CLI lists, validates, dry-runs, and blocks untrusted execution", async () => {
  const root = await project(registry, "steps:\n  - name: Test all\n    run: echo SHOULD_NOT_RUN\n    shell: sh\n");
  const listed = await execFileAsync("node", [bin, "scripts", "list"], { cwd: root });
  assert.match(listed.stdout, /test.*Run checks/);
  const valid = await execFileAsync("node", [bin, "scripts", "validate"], { cwd: root });
  assert.match(valid.stdout, /All KMC scripts are valid/);
  const dry = await execFileAsync("node", [bin, "run", "test", "--dry-run"], { cwd: root });
  assert.match(dry.stdout, /Dry run: test/);
  await assert.rejects(() => execFileAsync("node", [bin, "scripts", "run", "test"], { cwd: root, env: { ...process.env, KMC_CONFIG_HOME: path.join(root, "config") } }), /Repository is not trusted/);
});

test("trust fingerprint changes when a workflow changes", async () => {
  const root = await project(registry, "steps:\n  - name: Before\n    run: echo before\n");
  const env = { ...process.env, KMC_CONFIG_HOME: path.join(root, "config") };
  await execFileAsync("node", [bin, "trust"], { cwd: root, env });
  await execFileAsync("node", [bin, "trust", "status"], { cwd: root, env });
  await writeFile(path.join(root, ".kmc", "scripts", "test.yml"), "steps:\n  - name: After\n    run: echo after\n");
  await assert.rejects(() => execFileAsync("node", [bin, "trust", "status"], { cwd: root, env }), (error) => {
    assert.match(error.stdout, /scripts changed/);
    return true;
  });
});

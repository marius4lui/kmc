import { execFile } from "node:child_process";
import { access, readFile } from "node:fs/promises";
import { constants } from "node:fs";
import { promisify } from "node:util";
import test from "node:test";
import assert from "node:assert/strict";

const execFileAsync = promisify(execFile);

test("prints the package version", async () => {
  const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
  const { stdout } = await execFileAsync("node", ["./bin/kmc.js", "--version"]);

  assert.equal(stdout.trim(), packageJson.version);
});

test("prints help for the interactive CLI", async () => {
  const { stdout } = await execFileAsync("node", ["./bin/kmc.js", "--help"]);

  assert.match(stdout, /interactive project command launcher/);
  assert.match(stdout, /Run:\n  kmc/);
});

test("bin file is executable for npm global installs", async () => {
  await access(new URL("../bin/kmc.js", import.meta.url), constants.X_OK);
});

test("package does not depend on itself", async () => {
  const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
  const dependencies = packageJson.dependencies ?? {};

  assert.equal(dependencies[packageJson.name], undefined);
});

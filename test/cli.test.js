import { execFile } from "node:child_process";
import { access, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { constants } from "node:fs";
import { tmpdir } from "node:os";
import path from "node:path";
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
  assert.match(stdout, /kmc run <command-id>/);
});

test("bin file is executable for npm global installs", async () => {
  await access(new URL("../bin/kmc.js", import.meta.url), constants.X_OK);
});

test("package does not depend on itself", async () => {
  const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
  const dependencies = packageJson.dependencies ?? {};

  assert.equal(dependencies[packageJson.name], undefined);
});

test("imports package scripts into the npm group", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-import-"));
  await writeFile(
    path.join(cwd, "package.json"),
    JSON.stringify({ scripts: { dev: "vite", test: "node --test" } }, null, 2)
  );

  const { stdout } = await execFileAsync("node", [new URL("../bin/kmc.js", import.meta.url).pathname, "import"], {
    cwd
  });
  const config = JSON.parse(await readFile(path.join(cwd, "kmc.json"), "utf8"));

  assert.match(stdout, /Imported 2 commands/);
  assert.equal(config.groups[0].id, "npm");
  assert.equal(config.groups[0].label, "NPM Scripts");
  assert.equal(config.groups[0].type, "imported");
  assert.deepEqual(
    config.groups[0].commands.map((command) => command.id).sort(),
    ["npm.dev", "npm.test"]
  );
  assert.equal(config.groups[0].commands[0].source, "package.json");
});

test("validates grouped config", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-validate-"));
  await writeFile(
    path.join(cwd, "kmc.json"),
    JSON.stringify(
      {
        groups: [
          {
            id: "deployment",
            label: "Deployment",
            type: "manual",
            commands: [
              {
                id: "deployment.deploy",
                name: "deploy",
                command: "echo deploy"
              }
            ]
          }
        ]
      },
      null,
      2
    )
  );

  const { stdout } = await execFileAsync("node", [new URL("../bin/kmc.js", import.meta.url).pathname, "validate"], {
    cwd
  });

  assert.match(stdout, /Validate kmc project/);
  assert.match(stdout, /OK.*command ids are unique/s);
  assert.match(stdout, /Everything looks okay/);
  assert.match(stdout, /1 command configured/);
});

test("custom groups display by label", async () => {
  const { groupDisplayLabel } = await import("../lib/ui.js");
  const config = {
    groups: [
      {
        id: "my-tools",
        label: "My Tools",
        type: "manual",
        commands: [{ id: "my-tools.dev", name: "dev", command: "echo dev" }]
      }
    ]
  };

  assert.equal(groupDisplayLabel(config, "my-tools"), "My Tools");
});

test("detects newer package versions", async () => {
  const { compareVersions, isNewerVersion } = await import("../lib/update.js");

  assert.equal(compareVersions("1.0.3", "1.0.4"), 1);
  assert.equal(compareVersions("1.0.3", "1.0.3"), 0);
  assert.equal(compareVersions("1.0.4", "1.0.3"), -1);
  assert.equal(isNewerVersion("1.0.3", "1.0.4"), true);
  assert.equal(isNewerVersion("1.0.3", "1.0.3"), false);
});

import { execFile } from "node:child_process";
import { access, mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
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

test("detects Next.js projects for dev URLs", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-next-"));
  await writeFile(path.join(cwd, "package.json"), JSON.stringify({ dependencies: { next: "^15.0.0" } }, null, 2));
  const { detectProject } = await import("../lib/project-detect.js");

  const project = await detectProject(cwd);

  assert.equal(project.type, "nextjs");
  assert.equal(project.label, "Next.js");
});

test("discovers workspace projects for dev URLs", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-workspace-"));
  const appRoot = path.join(cwd, "apps", "landing");
  await mkdir(appRoot, { recursive: true });
  await writeFile(path.join(cwd, "package.json"), JSON.stringify({ workspaces: ["apps/*"] }, null, 2));
  await writeFile(path.join(appRoot, "package.json"), JSON.stringify({ dependencies: { next: "^15.0.0" } }, null, 2));
  const { discoverProjects } = await import("../lib/project-detect.js");

  const projects = await discoverProjects(cwd);

  assert.equal(projects.length, 1);
  assert.equal(projects[0].name, "apps/landing");
  assert.equal(projects[0].root, appRoot);
  assert.equal(projects[0].type, "nextjs");
});

test("discovers deep conventional monorepo app projects", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-deep-monorepo-"));
  const appRoot = path.join(cwd, "apps", "web", "landing");
  await mkdir(appRoot, { recursive: true });
  await writeFile(path.join(cwd, "package.json"), JSON.stringify({ private: true }, null, 2));
  await writeFile(path.join(appRoot, "package.json"), JSON.stringify({ devDependencies: { vite: "^6.0.0" } }, null, 2));
  const { discoverProjects } = await import("../lib/project-detect.js");

  const projects = await discoverProjects(cwd);

  assert.deepEqual(
    projects.map((project) => project.name),
    ["apps/web/landing"]
  );
  assert.equal(projects[0].type, "vite");
});

test("finds the workspace root from a nested app directory", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-nested-root-"));
  const appRoot = path.join(cwd, "apps", "web", "landing");
  await mkdir(appRoot, { recursive: true });
  await writeFile(path.join(cwd, "pnpm-workspace.yaml"), "packages:\n  - apps/**\n");
  await writeFile(path.join(cwd, "package.json"), JSON.stringify({ private: true }, null, 2));
  await writeFile(path.join(appRoot, "package.json"), JSON.stringify({ dependencies: { next: "^15.0.0" } }, null, 2));
  const { discoverProjects, findProjectSearchRoot } = await import("../lib/project-detect.js");

  const searchRoot = await findProjectSearchRoot(appRoot);
  const projects = await discoverProjects(searchRoot);

  assert.equal(searchRoot, cwd);
  assert.equal(projects[0].name, "apps/web/landing");
});

test("creates a persistent dev URL config", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-dev-url-"));
  const { ensureDevConfig } = await import("../lib/dev-config.js");

  const first = await ensureDevConfig({ type: "vite" }, cwd);
  const second = await ensureDevConfig({ type: "vite" }, cwd);

  assert.equal(first.name, path.basename(cwd).replace(/^kmc-dev-url-/, "kmc-dev-url-").toLowerCase());
  assert.equal(first.type, "vite");
  assert.equal(first.host, `${first.name}.kmc.localhost`);
  assert.equal(first.port, second.port);
});

test("stores a nested dev URL project root", async () => {
  const cwd = await mkdtemp(path.join(tmpdir(), "kmc-dev-url-root-"));
  const appRoot = path.join(cwd, "apps", "web");
  await mkdir(appRoot, { recursive: true });
  const { ensureDevConfig } = await import("../lib/dev-config.js");

  const config = await ensureDevConfig({ type: "nextjs" }, cwd, appRoot);

  assert.equal(config.root, appRoot);
  assert.equal(config.name, "web");
});

test("explains how to install caddy when reload cannot spawn it", async () => {
  const { caddyErrorMessage } = await import("../lib/caddy.js");

  const message = caddyErrorMessage(Object.assign(new Error("spawn caddy ENOENT"), { code: "ENOENT" }));

  assert.match(message, /Caddy is not installed or not in PATH/);
  assert.match(message, /sudo apt install caddy/);
});

test("explains how to start caddy when the admin API is not running", async () => {
  const { caddyErrorMessage } = await import("../lib/caddy.js");

  const message = caddyErrorMessage(new Error('Post "http://localhost:2019/load": dial tcp [::1]:2019: connect: connection refused'), "/tmp/Caddyfile");

  assert.match(message, /no running Caddy admin instance/);
  assert.match(message, /caddy start --config \/tmp\/Caddyfile/);
});

test("explains how to trust caddy certs manually when trust fails", async () => {
  const { caddyTrustErrorMessage } = await import("../lib/caddy.js");

  const message = caddyTrustErrorMessage(new Error("permission denied"));

  assert.match(message, /Could not install Caddy's local CA/);
  assert.match(message, /caddy trust/);
});

test("builds dev server commands with the stored port", async () => {
  const { startCommandFor } = await import("../lib/dev-server.js");

  assert.equal(startCommandFor({ type: "nextjs", port: 43172 }), "npx next dev --port 43172");
  assert.equal(startCommandFor({ type: "vite", port: 43172 }), "npx vite --host 127.0.0.1 --port 43172");
});

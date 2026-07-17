import { select } from "@inquirer/prompts";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { color } from "./theme.js";

const execFileAsync = promisify(execFile);
const PACKAGE_NAME = "@marius4lui/kmc";
const SKILL_NAME = "kmc";

export function compareVersions(current, latest) {
  const currentParts = String(current).split(".").map((part) => Number.parseInt(part, 10) || 0);
  const latestParts = String(latest).split(".").map((part) => Number.parseInt(part, 10) || 0);
  const length = Math.max(currentParts.length, latestParts.length);
  for (let index = 0; index < length; index += 1) {
    const currentPart = currentParts[index] ?? 0;
    const latestPart = latestParts[index] ?? 0;
    if (latestPart > currentPart) return 1;
    if (latestPart < currentPart) return -1;
  }
  return 0;
}

export function isNewerVersion(current, latest) {
  return compareVersions(current, latest) > 0;
}

export async function latestVersion() {
  const { stdout } = await execFileAsync("npm", ["view", PACKAGE_NAME, "version"], {
    timeout: 5000,
    windowsHide: true
  });
  return stdout.trim();
}

async function installLatest() {
  const command = "npm";
  const args = ["install", "-g", `${PACKAGE_NAME}@latest`];
  console.log("");
  console.log(`${color.cyan("$")} ${command} ${args.join(" ")}`);
  console.log("");

  const code = await new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit", shell: process.platform === "win32" });
    child.on("error", reject);
    child.on("exit", (exitCode) => resolve(exitCode ?? 0));
  });

  if (code !== 0) throw new Error(`Update failed with exit code ${code}.`);
}

async function installedSkills(global = false) {
  const args = ["--yes", "skills", "list", ...(global ? ["--global"] : []), "--json"];
  try {
    const { stdout } = await execFileAsync("npx", args, { timeout: 15000, windowsHide: true });
    const result = JSON.parse(stdout);
    return Array.isArray(result) ? result : [];
  } catch {
    return [];
  }
}

export function installedKmcSkillScopes(projectSkills, globalSkills) {
  const hasKmc = (skills) => skills.some((skill) => skill?.name === SKILL_NAME);
  return [
    ...(hasKmc(projectSkills) ? ["project"] : []),
    ...(hasKmc(globalSkills) ? ["global"] : [])
  ];
}

export function skillUpdateArgs(scope) {
  return ["--yes", "skills", "update", SKILL_NAME, scope === "global" ? "--global" : "--project", "--yes"];
}

async function updateInstalledKmcSkill() {
  const [projectSkills, globalSkills] = await Promise.all([installedSkills(false), installedSkills(true)]);
  const scopes = installedKmcSkillScopes(projectSkills, globalSkills);
  if (scopes.length === 0) {
    console.log(color.dim("KMC skill is not installed; skipping skill update."));
    return;
  }

  for (const scope of scopes) {
    const args = skillUpdateArgs(scope);
    console.log("");
    console.log(`${color.cyan("$")} npx ${args.join(" ")}`);
    console.log(color.dim(`Updating the ${scope} KMC skill with its existing installation settings.`));
    console.log("");
    const code = await new Promise((resolve, reject) => {
      const child = spawn("npx", args, { stdio: "inherit", shell: process.platform === "win32" });
      child.on("error", reject);
      child.on("exit", (exitCode) => resolve(exitCode ?? 0));
    });
    if (code !== 0) throw new Error(`KMC was updated, but the ${scope} skill update failed with exit code ${code}.`);
  }
}

export async function promptForUpdate(currentVersion) {
  if (!process.stdin.isTTY || !process.stdout.isTTY) return;

  let nextVersion;
  try {
    nextVersion = await latestVersion();
  } catch {
    return;
  }

  if (!isNewerVersion(currentVersion, nextVersion)) return;

  console.log("");
  console.log(`${color.yellow("Update available:")} kmc ${currentVersion} -> ${nextVersion}`);
  const action = await select({
    message: "Update kmc now?",
    loop: false,
    choices: [
      { name: "Update now", value: "update", description: "Update the CLI and existing KMC skill installations" },
      { name: "Skip", value: "skip", description: "Continue with the current version for this run" }
    ]
  });

  if (action === "update") {
    await installLatest();
    await updateInstalledKmcSkill();
  }
}

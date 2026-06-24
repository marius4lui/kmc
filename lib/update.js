import { select } from "@inquirer/prompts";
import { execFile, spawn } from "node:child_process";
import { promisify } from "node:util";
import { color } from "./theme.js";

const execFileAsync = promisify(execFile);
const PACKAGE_NAME = "@marius4lui/kmc";

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
      { name: "Update now", value: "update", description: `Run npm install -g ${PACKAGE_NAME}@latest` },
      { name: "Skip", value: "skip", description: "Continue with the current version for this run" }
    ]
  });

  if (action === "update") await installLatest();
}

import { readFile } from "node:fs/promises";
import { color } from "./theme.js";
import { findCommand, getCommands, readConfig, readSettings } from "./store.js";
import { importCommands } from "./importers.js";
import { runCommand } from "./runner.js";
import { printValidationReport, validateConfig } from "./validate.js";
import { Back, deleteCommand, devUrlsMenu, editCommand, importScreen, mainMenu, manageMenu, runGroupedPicker, saveCommand, settingsMenu } from "./ui.js";
import { promptForUpdate } from "./update.js";
import { access } from "node:fs/promises";
import path from "node:path";
import { runScript, scriptsCommand, trustCommand, untrustCommand } from "./scripts-cli.js";

async function packageVersion() {
  const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
  return packageJson.version;
}

export function help() {
  console.log(`${color.bold("kmc")} ${color.dim("interactive project command launcher")}

Usage:
  kmc
  kmc run <command-id>
  kmc scripts list|validate|init
  kmc scripts run <script> [--dry-run] [--step <step>] [--env KEY=value]
  kmc trust [status]
  kmc untrust
  kmc add
  kmc edit <command-id>
  kmc delete <command-id>
  kmc import
  kmc validate
  kmc settings

In interactive screens, Esc goes back. Checkbox screens use Space to select and Enter to confirm.`);
}

async function openInteractiveMenu() {
  let config = await readConfig();
  let settings = await readSettings();
  while (true) {
    try {
      const action = await mainMenu(config, settings);
      if (action === "quit") return;
      if (action === "run") {
        const command = await runGroupedPicker(config, settings);
        if (command) runCommand(command);
        if (command) return;
      }
      if (action === "devUrls") await devUrlsMenu();
      if (action === "manage") config = await manageMenu(config);
      if (action === "import") config = await importScreen(config);
      if (action === "settings") settings = await settingsMenu(settings, config);
    } catch (error) {
      if (error instanceof Back) continue;
      throw error;
    }
  }
}

export async function main(argv = process.argv.slice(2)) {
  const [arg, value, ...rest] = argv;
  if (arg === "--help" || arg === "-h" || arg === "help") return help();
  if (arg === "--version" || arg === "-v") {
    console.log(await packageVersion());
    return;
  }
  await promptForUpdate(await packageVersion());
  if (!arg) return openInteractiveMenu();

  if (arg === "scripts") return scriptsCommand(argv.slice(1));
  if (arg === "trust") return trustCommand(argv.slice(1));
  if (arg === "untrust") return untrustCommand();

  if (arg === "run") {
    if (value) {
      try {
        await access(path.join(process.cwd(), ".kmc", "scripts.yml"));
        try { return await runScript(value, rest); } catch (error) { if (!/^Script ".*" was not found\.$/.test(error.message)) throw error; }
      } catch (error) { if (error.code !== "ENOENT") throw error; }
    }
    const config = await readConfig();
    const settings = await readSettings();
    const command = value ? findCommand(getCommands(config), value) : await runGroupedPicker(config, settings);
    if (!command) throw new Error(value ? `Command "${value}" was not found.` : "No command selected.");
    return runCommand(command);
  }
  let config = await readConfig();
  const settings = await readSettings();
  if (arg === "add") return saveCommand(config);
  if (arg === "edit") return editCommand(config, value);
  if (arg === "delete" || arg === "remove") return deleteCommand(config, value);
  if (arg === "import") {
    if (process.stdout.isTTY && process.stdin.isTTY) return importScreen(config);
    return importCommands(config, ["npm", "make", "flutter", "docker"]);
  }
  if (arg === "validate") {
    const result = validateConfig(config, settings);
    printValidationReport(result);
    if (!result.ok) process.exitCode = 1;
    return;
  }
  if (arg === "settings") return settingsMenu(settings, config);
  throw new Error(`Unknown command "${arg}". Run "kmc help".`);
}

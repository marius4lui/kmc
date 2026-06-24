import { readFile } from "node:fs/promises";
import { color } from "./theme.js";
import { findCommand, getCommands, readConfig, readSettings } from "./store.js";
import { importCommands } from "./importers.js";
import { runCommand } from "./runner.js";
import { printValidationReport, validateConfig } from "./validate.js";
import { Back, deleteCommand, editCommand, importScreen, mainMenu, manageMenu, runGroupedPicker, saveCommand, settingsMenu } from "./ui.js";

export function help() {
  console.log(`${color.bold("kmc")} ${color.dim("interactive project command launcher")}

Usage:
  kmc
  kmc run <command-id>
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
  const [arg, value] = argv;
  if (arg === "--help" || arg === "-h" || arg === "help") return help();
  if (arg === "--version" || arg === "-v") {
    const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
    console.log(packageJson.version);
    return;
  }
  if (!arg) return openInteractiveMenu();

  let config = await readConfig();
  const settings = await readSettings();

  if (arg === "run") {
    const command = value ? findCommand(getCommands(config), value) : await runGroupedPicker(config, settings);
    if (!command) throw new Error(value ? `Command "${value}" was not found.` : "No command selected.");
    return runCommand(command);
  }
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

#!/usr/bin/env node

import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import { readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { confirm, input, select } from "@inquirer/prompts";

const CONFIG_FILE = "kmc.json";

const color = {
  dim: (value) => `\x1b[2m${value}\x1b[22m`,
  cyan: (value) => `\x1b[36m${value}\x1b[39m`,
  green: (value) => `\x1b[32m${value}\x1b[39m`,
  red: (value) => `\x1b[31m${value}\x1b[39m`,
  yellow: (value) => `\x1b[33m${value}\x1b[39m`,
  bold: (value) => `\x1b[1m${value}\x1b[22m`
};

function help() {
  console.log(`${color.bold("kmc")} ${color.dim("interactive project command launcher")}

Run:
  kmc

All command setup, editing, deleting, and execution happens in the interactive menu.`);
}

async function readConfig() {
  const configPath = path.join(process.cwd(), CONFIG_FILE);
  if (!existsSync(configPath)) {
    return {
      "$schema": "https://raw.githubusercontent.com/marius/kmc/main/schema.json",
      commands: []
    };
  }

  const raw = await readFile(configPath, "utf8");
  const parsed = JSON.parse(raw);
  if (!Array.isArray(parsed.commands)) {
    throw new Error(`${CONFIG_FILE} must contain a "commands" array.`);
  }

  return parsed;
}

async function writeConfig(config) {
  const configPath = path.join(process.cwd(), CONFIG_FILE);
  await writeFile(configPath, `${JSON.stringify(config, null, 2)}\n`);
}

function normalizeCommand(command) {
  if (!command || typeof command.name !== "string" || typeof command.command !== "string") {
    throw new Error(`Each command in ${CONFIG_FILE} needs "name" and "command" strings.`);
  }

  return {
    name: command.name,
    command: command.command,
    description: command.description ?? "",
    cwd: command.cwd ?? "."
  };
}

function getCommands(config) {
  return config.commands.map(normalizeCommand);
}

function clearScreen() {
  if (process.stdout.isTTY) {
    console.clear();
  }
}

function banner(commands) {
  clearScreen();
  const count = `${commands.length} command${commands.length === 1 ? "" : "s"}`;
  console.log(color.cyan("╭────────────────────────────────────────╮"));
  console.log(`${color.cyan("│")} ${color.bold("kmc")} ${color.dim("interactive command center")}       ${color.cyan("│")}`);
  console.log(`${color.cyan("│")} ${color.dim(`${CONFIG_FILE} · ${count}`.padEnd(38, " "))}${color.cyan("│")}`);
  console.log(color.cyan("╰────────────────────────────────────────╯"));
  console.log("");
}

function commandHint(command) {
  const description = command.description ? ` ${color.dim(command.description)}` : "";
  const cwd = command.cwd && command.cwd !== "." ? ` ${color.yellow(`cwd:${command.cwd}`)}` : "";
  return `${color.green(command.name)}${description}${cwd}\n${color.dim(`$ ${command.command}`)}`;
}

function runCommand(command) {
  clearScreen();
  console.log(`${color.bold(command.name)} ${color.dim("is running")}`);
  console.log(color.dim("─".repeat(48)));
  console.log(`${color.cyan("$")} ${command.command}\n`);

  const child = spawn(command.command, {
    cwd: path.resolve(process.cwd(), command.cwd || "."),
    shell: true,
    stdio: "inherit",
    env: process.env
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 0);
  });
}

async function commandForm(existingCommand = null) {
  const name = await input({
    message: "Name",
    default: existingCommand?.name,
    validate: (value) => value.trim().length > 0 || "Name is required"
  });

  const command = await input({
    message: "Command",
    default: existingCommand?.command,
    validate: (value) => value.trim().length > 0 || "Command is required"
  });

  const description = await input({
    message: "Description",
    default: existingCommand?.description ?? ""
  });

  const cwd = await input({
    message: "Working directory",
    default: existingCommand?.cwd ?? ".",
    validate: (value) => value.trim().length > 0 || "Working directory is required"
  });

  return {
    name: name.trim(),
    command: command.trim(),
    description: description.trim(),
    cwd: cwd.trim()
  };
}

async function saveCommand(config, existingCommand = null) {
  const nextCommand = await commandForm(existingCommand);
  const currentName = existingCommand?.name;
  const commands = getCommands(config).filter(
    (command) => command.name !== currentName && command.name !== nextCommand.name
  );

  commands.push(nextCommand);
  await writeConfig({ ...config, commands });
  console.log(`\n${color.green("Saved")} ${color.bold(nextCommand.name)} ${color.dim(`to ${CONFIG_FILE}`)}`);
  return { ...config, commands };
}

async function chooseExisting(commands, message) {
  if (commands.length === 0) return null;

  return select({
    message,
    pageSize: 12,
    choices: commands.map((command) => ({
      name: command.name,
      value: command,
      description: command.description || command.command
    }))
  });
}

async function editCommand(config) {
  const command = await chooseExisting(getCommands(config), "Edit which command?");
  if (!command) return config;
  return saveCommand(config, command);
}

async function deleteCommand(config) {
  const commands = getCommands(config);
  const command = await chooseExisting(commands, "Delete which command?");
  if (!command) return config;

  const shouldDelete = await confirm({
    message: `Delete "${command.name}"?`,
    default: false
  });
  if (!shouldDelete) return config;

  const nextCommands = commands.filter((item) => item.name !== command.name);
  await writeConfig({ ...config, commands: nextCommands });
  console.log(`\n${color.red("Deleted")} ${color.bold(command.name)}`);
  return { ...config, commands: nextCommands };
}

async function mainMenu(config) {
  const commands = getCommands(config);
  banner(commands);

  if (commands.length > 0) {
    console.log(commands.map(commandHint).join("\n\n"));
    console.log("");
  } else {
    console.log(color.dim("No commands yet. Add your first script and kmc.json will be created."));
    console.log("");
  }

  const choices = [
    ...commands.map((command) => ({
      name: `Run ${command.name}`,
      value: { type: "run", command },
      description: command.description || command.command
    })),
    { name: "Add command", value: { type: "add" }, description: "Create a new script entry" }
  ];

  if (commands.length > 0) {
    choices.push(
      { name: "Edit command", value: { type: "edit" }, description: "Change name, command, cwd, or description" },
      { name: "Delete command", value: { type: "delete" }, description: "Remove an entry from kmc.json" }
    );
  }

  choices.push({ name: "Quit", value: { type: "quit" } });

  return select({
    message: "What do you want to do?",
    pageSize: 12,
    choices
  });
}

async function openInteractiveMenu() {
  let config = await readConfig();

  while (true) {
    const action = await mainMenu(config);

    if (action.type === "quit") return;
    if (action.type === "run") {
      runCommand(action.command);
      return;
    }
    if (action.type === "add") {
      config = await saveCommand(config);
      continue;
    }
    if (action.type === "edit") {
      config = await editCommand(config);
      continue;
    }
    if (action.type === "delete") {
      config = await deleteCommand(config);
    }
  }
}

async function main() {
  const arg = process.argv[2];
  if (arg === "--help" || arg === "-h") {
    help();
    return;
  }

  if (arg === "--version" || arg === "-v") {
    const packageJson = JSON.parse(await readFile(new URL("../package.json", import.meta.url), "utf8"));
    console.log(packageJson.version);
    return;
  }

  if (arg) {
    console.log(color.yellow("kmc is interactive. Start it with:"));
    console.log(`  ${color.bold("kmc")}`);
    return;
  }

  await openInteractiveMenu();
}

main().catch((error) => {
  if (error.name === "ExitPromptError") {
    console.log("");
    process.exit(0);
  }

  console.error(`${color.red("kmc:")} ${error.message}`);
  process.exit(1);
});

import { existsSync } from "node:fs";
import { appendFile, mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { CONFIG_FILE, DEFAULT_SETTINGS, GROUPS, SCHEMA_URL, SETTINGS_DIR, SETTINGS_FILE } from "./constants.js";

async function readJson(filePath, fallback) {
  if (!existsSync(filePath)) return fallback;
  return JSON.parse(await readFile(filePath, "utf8"));
}

export async function readConfig() {
  const config = await readJson(path.join(process.cwd(), CONFIG_FILE), {
    "$schema": SCHEMA_URL,
    commands: []
  });
  if (!Array.isArray(config.commands)) throw new Error(`${CONFIG_FILE} must contain a "commands" array.`);
  return { "$schema": config.$schema ?? SCHEMA_URL, ...config };
}

export async function writeConfig(config) {
  await writeFile(path.join(process.cwd(), CONFIG_FILE), `${JSON.stringify(config, null, 2)}\n`);
}

export async function readSettings() {
  return { ...DEFAULT_SETTINGS, ...(await readJson(path.join(process.cwd(), SETTINGS_FILE), {})) };
}

export async function writeSettings(settings) {
  await mkdir(path.join(process.cwd(), SETTINGS_DIR), { recursive: true });
  await writeFile(path.join(process.cwd(), SETTINGS_FILE), `${JSON.stringify(settings, null, 2)}\n`);
  await ensureSettingsIgnored();
}

export async function ensureSettingsIgnored() {
  const gitignorePath = path.join(process.cwd(), ".gitignore");
  const entry = `${SETTINGS_DIR}/`;
  const current = existsSync(gitignorePath) ? await readFile(gitignorePath, "utf8") : "";
  if (current.split(/\r?\n/).includes(entry)) return;
  const prefix = current.length > 0 && !current.endsWith("\n") ? "\n" : "";
  await appendFile(gitignorePath, `${prefix}${entry}\n`);
}

export function commandId(command) {
  return command.id ?? `${command.group ?? "manual"}.${command.name}`;
}

export function normalizeCommand(command) {
  if (!command || typeof command.name !== "string" || typeof command.command !== "string") {
    throw new Error(`Each command in ${CONFIG_FILE} needs "name" and "command" strings.`);
  }
  const group = command.group ?? "manual";
  return {
    id: command.id ?? `${group}.${command.name}`,
    name: command.name,
    command: command.command,
    description: command.description ?? "",
    cwd: command.cwd ?? ".",
    group,
    source: command.source ?? GROUPS[group]?.source ?? "manual",
    imported: Boolean(command.imported)
  };
}

export function getCommands(config) {
  return config.commands.map(normalizeCommand);
}

export function serializeCommand(command) {
  const group = command.group ?? "manual";
  return {
    id: command.id ?? `${group}.${command.name}`,
    name: command.name,
    command: command.command,
    description: command.description ?? "",
    cwd: command.cwd ?? ".",
    group,
    source: command.source ?? GROUPS[group]?.source ?? "manual",
    imported: Boolean(command.imported)
  };
}

export function findCommand(commands, identifier) {
  return commands.find((command) => {
    return commandId(command) === identifier || command.name === identifier || `${command.group}.${command.name}` === identifier;
  });
}

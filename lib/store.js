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
    groups: []
  });
  return normalizeConfig({ "$schema": config.$schema ?? SCHEMA_URL, ...config });
}

export async function writeConfig(config) {
  await writeFile(path.join(process.cwd(), CONFIG_FILE), `${JSON.stringify(serializeConfig(config), null, 2)}\n`);
}

export async function readSettings() {
  return { ...DEFAULT_SETTINGS, ...(await readJson(path.join(process.cwd(), SETTINGS_FILE), {})) };
}

export async function writeSettings(settings) {
  await mkdir(path.join(process.cwd(), SETTINGS_DIR), { recursive: true });
  await writeFile(path.join(process.cwd(), SETTINGS_FILE), `${JSON.stringify(settings, null, 2)}\n`);
  await ensureSettingsIgnored();
}

export async function ensureSettingsIgnored(cwd = process.cwd()) {
  const gitignorePath = path.join(cwd, ".gitignore");
  const entry = `${SETTINGS_DIR}/`;
  const current = existsSync(gitignorePath) ? await readFile(gitignorePath, "utf8") : "";
  if (current.split(/\r?\n/).includes(entry)) return;
  const prefix = current.length > 0 && !current.endsWith("\n") ? "\n" : "";
  await appendFile(gitignorePath, `${prefix}${entry}\n`);
}

export function commandId(command) {
  return command.id ?? `${command.groupId ?? command.group ?? "manual"}.${command.name}`;
}

export function normalizeGroup(group) {
  if (!group || typeof group.id !== "string") throw new Error(`Each group in ${CONFIG_FILE} needs an "id" string.`);
  const defaults = GROUPS[group.id] ?? {};
  return {
    id: group.id,
    label: group.label ?? defaults.label ?? group.id,
    description: group.description ?? defaults.description ?? "",
    icon: group.icon ?? defaults.icon ?? "",
    type: group.type ?? defaults.type ?? "manual",
    source: group.source ?? defaults.source,
    commands: (group.commands ?? []).map((command) => normalizeCommand(command, group.id))
  };
}

export function normalizeCommand(command, groupId = command?.groupId ?? command?.group ?? "manual") {
  if (!command || typeof command.name !== "string" || typeof command.command !== "string") {
    throw new Error(`Each command in ${CONFIG_FILE} needs "name" and "command" strings.`);
  }
  return {
    id: command.id ?? `${groupId}.${command.name}`,
    name: command.name,
    command: command.command,
    description: command.description ?? "",
    cwd: command.cwd ?? ".",
    groupId,
    group: groupId,
    source: command.source ?? GROUPS[groupId]?.source ?? "manual",
    imported: Boolean(command.imported)
  };
}

export function normalizeConfig(config) {
  if (Array.isArray(config.groups)) {
    return { "$schema": config.$schema ?? SCHEMA_URL, groups: config.groups.map(normalizeGroup) };
  }
  if (Array.isArray(config.commands)) {
    const groups = new Map();
    for (const command of config.commands.map((item) => normalizeCommand(item))) {
      const groupId = command.groupId;
      if (!groups.has(groupId)) groups.set(groupId, groupShell(groupId, command.imported));
      groups.get(groupId).commands.push(command);
    }
    return { "$schema": config.$schema ?? SCHEMA_URL, groups: [...groups.values()].map(normalizeGroup) };
  }
  throw new Error(`${CONFIG_FILE} must contain a "groups" array.`);
}

export function groupShell(groupId, imported = false) {
  const defaults = GROUPS[groupId] ?? {};
  return {
    id: groupId,
    label: defaults.label ?? groupId,
    description: defaults.description ?? "",
    icon: defaults.icon ?? "",
    type: defaults.type ?? (imported ? "imported" : "manual"),
    source: defaults.source,
    commands: []
  };
}

export function getGroups(config) {
  return normalizeConfig(config).groups;
}

export function getCommands(config) {
  return getGroups(config).flatMap((group) => group.commands.map((command) => normalizeCommand(command, group.id)));
}

export function serializeCommand(command) {
  const groupId = command.groupId ?? command.group ?? "manual";
  return {
    id: command.id ?? `${groupId}.${command.name}`,
    name: command.name,
    command: command.command,
    description: command.description ?? "",
    cwd: command.cwd ?? ".",
    source: command.source ?? GROUPS[groupId]?.source ?? "manual",
    imported: Boolean(command.imported)
  };
}

export function serializeGroup(group) {
  const normalized = normalizeGroup(group);
  return {
    id: normalized.id,
    label: normalized.label,
    description: normalized.description,
    icon: normalized.icon,
    type: normalized.type,
    ...(normalized.source ? { source: normalized.source } : {}),
    commands: normalized.commands.map(serializeCommand)
  };
}

export function serializeConfig(config) {
  return {
    "$schema": config.$schema ?? SCHEMA_URL,
    groups: getGroups(config).filter((group) => group.commands.length > 0).map(serializeGroup)
  };
}

export function replaceGroups(config, groups) {
  return serializeConfig({ ...config, groups });
}

export function findCommand(commands, identifier) {
  return commands.find((command) => {
    return commandId(command) === identifier || command.name === identifier || `${command.group}.${command.name}` === identifier;
  });
}

import { existsSync } from "node:fs";
import path from "node:path";
import { CONFIG_FILE, GROUPS, SETTINGS_FILE } from "./constants.js";
import { commandId, getCommands } from "./store.js";
import { color } from "./theme.js";

export function validateConfig(config, settings) {
  const checks = [];
  const commands = getCommands(config);
  const ids = commands.map(commandId);
  const duplicates = ids.filter((id, index) => ids.indexOf(id) !== index);
  const groups = new Set(Object.keys(GROUPS));

  checks.push({ ok: Array.isArray(config.commands), label: `${CONFIG_FILE} contains a commands array` });
  checks.push({ ok: duplicates.length === 0, label: "command ids are unique", detail: [...new Set(duplicates)].join(", ") });
  checks.push({ ok: commands.every((command) => command.name && command.command), label: "each command has name and command" });
  checks.push({ ok: commands.every((command) => groups.has(command.group)), label: "all command groups are known" });
  checks.push({ ok: Number.isInteger(settings.maxFavoriteCommands) && settings.maxFavoriteCommands >= 1, label: "maxFavoriteCommands is a positive integer" });
  checks.push({ ok: (settings.favoriteCommands ?? []).length <= settings.maxFavoriteCommands, label: "favoriteCommands respects maxFavoriteCommands" });
  checks.push({ ok: existsSync(path.join(process.cwd(), SETTINGS_FILE)) || true, label: `${SETTINGS_FILE} is optional and local` });

  return { ok: checks.every((check) => check.ok), checks, commandCount: commands.length };
}

export function printValidationReport(result) {
  console.log(color.bold("Validate kmc project"));
  console.log(color.dim("Checking config shape, command ids, groups, and local settings."));
  console.log("");
  for (const check of result.checks) {
    const marker = check.ok ? color.green("OK") : color.red("FAIL");
    console.log(`${marker} ${check.label}${check.detail ? color.dim(` (${check.detail})`) : ""}`);
  }
  console.log("");
  console.log(result.ok ? color.green("Everything looks okay.") : color.red("Validation failed."));
  console.log(`${result.commandCount} command${result.commandCount === 1 ? "" : "s"} configured.`);
}

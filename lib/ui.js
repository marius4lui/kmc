import { checkbox, confirm, input, number, select } from "@inquirer/prompts";
import { GROUPS } from "./constants.js";
import { detectImportSources, importCommands } from "./importers.js";
import { commandId, findCommand, getCommands, serializeCommand, writeConfig, writeSettings } from "./store.js";
import { banner, clearScreen, color, waitForEnter } from "./theme.js";
import { printValidationReport, validateConfig } from "./validate.js";

export class Back extends Error {
  constructor() {
    super("Back");
    this.name = "Back";
  }
}

async function prompt(run) {
  try {
    return await run();
  } catch (error) {
    // Inquirer emits ExitPromptError for Esc/Ctrl+C. Inside screens we treat it as Back.
    if (error.name === "ExitPromptError") throw new Back();
    throw error;
  }
}

function groupCommands(commands, settings) {
  const hidden = new Set(settings.hiddenGroups ?? []);
  const groups = new Map();
  for (const command of commands) {
    if (hidden.has(command.group)) continue;
    if (!groups.has(command.group)) groups.set(command.group, []);
    groups.get(command.group).push(command);
  }
  return groups;
}

function orderedGroups(groups, settings) {
  const preferred = [settings.lastSelectedGroup, settings.defaultGroup].filter(Boolean);
  return [...new Set([...preferred, "manual", "npm", "make", "flutter", "docker", ...groups.keys()])].filter((id) =>
    groups.has(id)
  );
}

function summary(commands, groups, settings) {
  const favorites = settings.favoriteCommands?.length ?? 0;
  const parts = [
    `${commands.length} command${commands.length === 1 ? "" : "s"}`,
    `${groups.size} group${groups.size === 1 ? "" : "s"}`
  ];
  if (favorites > 0) parts.push(`${favorites} favorite${favorites === 1 ? "" : "s"}`);
  return parts.join(" · ");
}

function commandLabel(command) {
  const description = command.description ? ` ${color.dim(command.description)}` : "";
  return `${command.favorite ? "★ " : ""}${command.name}${description}`;
}

async function commandForm(existingCommand = null) {
  const name = await prompt(() =>
    input({ message: "Name", default: existingCommand?.name, validate: (value) => value.trim().length > 0 || "Name is required" })
  );
  const command = await prompt(() =>
    input({
      message: "Command",
      default: existingCommand?.command,
      validate: (value) => value.trim().length > 0 || "Command is required"
    })
  );
  const description = await prompt(() => input({ message: "Description", default: existingCommand?.description ?? "" }));
  const cwd = await prompt(() =>
    input({
      message: "Working directory",
      default: existingCommand?.cwd ?? ".",
      validate: (value) => value.trim().length > 0 || "Working directory is required"
    })
  );
  return serializeCommand({ name: name.trim(), command: command.trim(), description: description.trim(), cwd: cwd.trim(), group: "manual" });
}

export async function saveCommand(config, existingCommand = null) {
  const nextCommand = await commandForm(existingCommand);
  const currentId = existingCommand ? commandId(existingCommand) : null;
  const commands = getCommands(config).filter((command) => commandId(command) !== currentId && commandId(command) !== commandId(nextCommand));
  commands.push(nextCommand);
  await writeConfig({ ...config, commands: commands.map(serializeCommand) });
  console.log(`\n${color.green("Saved")} ${color.bold(commandId(nextCommand))}`);
  await waitForEnter();
  return { ...config, commands: commands.map(serializeCommand) };
}

async function chooseCommand(commands, message, filter = () => true) {
  const filtered = commands.filter(filter);
  if (filtered.length === 0) return null;
  return prompt(() =>
    select({
      message,
      pageSize: 12,
      choices: [...filtered.map((command) => ({ name: commandId(command), value: command, description: command.command })), { name: "Back", value: null }]
    })
  );
}

export async function editCommand(config, identifier = null) {
  const commands = getCommands(config);
  const command = identifier ? findCommand(commands, identifier) : await chooseCommand(commands, "Edit which command?");
  if (!command) return config;
  if (command.imported) throw new Error("Imported commands are managed by their source files.");
  return saveCommand(config, command);
}

export async function deleteCommand(config, identifier = null) {
  const commands = getCommands(config);
  const command = identifier ? findCommand(commands, identifier) : await chooseCommand(commands, "Delete which command?");
  if (!command) return config;
  const shouldDelete = identifier || (await prompt(() => confirm({ message: `Delete "${commandId(command)}"?`, default: false })));
  if (!shouldDelete) return config;
  const nextCommands = commands.filter((item) => commandId(item) !== commandId(command));
  await writeConfig({ ...config, commands: nextCommands.map(serializeCommand) });
  console.log(`${color.red("Deleted")} ${color.bold(commandId(command))}`);
  if (!identifier) await waitForEnter();
  return { ...config, commands: nextCommands.map(serializeCommand) };
}

export async function importScreen(config) {
  clearScreen();
  const sources = detectImportSources();
  console.log(color.bold("Import project commands"));
  console.log(color.dim("Space selects sources. Enter imports selected detected sources."));
  console.log("");
  for (const source of sources) {
    const status = source.detected ? color.green(`detected: ${source.file}`) : color.dim("not detected");
    console.log(`${source.title}: ${status}`);
  }
  console.log("");

  const selected = await prompt(() =>
    checkbox({
      message: "Import from",
      required: false,
      choices: sources.map((source) => ({
        name: source.title,
        value: source.id,
        checked: source.detected,
        disabled: source.detected ? false : "not detected",
        description: source.files.join(", ")
      }))
    })
  );
  if (selected.length === 0) return config;
  const nextConfig = await importCommands(config, selected);
  await waitForEnter();
  return nextConfig;
}

export async function validateScreen(config, settings) {
  clearScreen();
  const result = validateConfig(config, settings);
  printValidationReport(result);
  await waitForEnter();
  if (!result.ok) throw new Error("Validation failed.");
}

export async function settingsMenu(settings, config) {
  while (true) {
    clearScreen();
    console.log(color.bold("Settings"));
    console.log(color.dim(".kmc/settings.json"));
    console.log("");
    console.log(JSON.stringify(settings, null, 2));
    console.log("");
    const action = await prompt(() =>
      select({
        message: "Preferences",
        choices: [
          { name: "Set default group", value: "defaultGroup" },
          { name: "Set hidden groups", value: "hiddenGroups" },
          { name: "Set favorite commands", value: "favoriteCommands" },
          { name: "Set max favorite commands", value: "maxFavoriteCommands" },
          { name: "Validate config", value: "validate" },
          { name: "Back", value: "back" }
        ]
      })
    );
    if (action === "back") return settings;
    if (action === "validate") {
      await validateScreen(config, settings);
    }
    if (action === "defaultGroup") {
      settings.defaultGroup = await prompt(() =>
        select({ message: "Default group", choices: Object.values(GROUPS).map((group) => ({ name: group.title, value: group.id })) })
      );
    }
    if (action === "hiddenGroups") {
      settings.hiddenGroups = await prompt(() =>
        checkbox({
          message: "Hidden groups",
          choices: Object.values(GROUPS).map((group) => ({ name: group.title, value: group.id, checked: settings.hiddenGroups?.includes(group.id) }))
        })
      );
    }
    if (action === "maxFavoriteCommands") {
      settings.maxFavoriteCommands = await prompt(() =>
        number({ message: "Max favorite commands", default: settings.maxFavoriteCommands ?? 3, min: 1, required: true })
      );
      if ((settings.favoriteCommands ?? []).length > settings.maxFavoriteCommands) {
        settings.favoriteCommands = settings.favoriteCommands.slice(0, settings.maxFavoriteCommands);
      }
    }
    if (action === "favoriteCommands") {
      const commands = getCommands(config);
      settings.favoriteCommands = await prompt(() =>
        checkbox({
          message: `Favorite commands (max ${settings.maxFavoriteCommands})`,
          pageSize: 12,
          validate: (values) => values.length <= settings.maxFavoriteCommands || `Choose at most ${settings.maxFavoriteCommands}.`,
          choices: commands.map((command) => ({
            name: commandId(command),
            value: commandId(command),
            checked: settings.favoriteCommands?.includes(commandId(command)),
            description: command.command
          }))
        })
      );
    }
    await writeSettings(settings);
  }
}

export async function runGroupedPicker(config, settings) {
  const favoriteIds = new Set(settings.favoriteCommands ?? []);
  const commands = getCommands(config).map((command) => ({ ...command, favorite: favoriteIds.has(commandId(command)) }));
  const groups = groupCommands(commands, settings);
  const favoriteCommands = commands.filter((command) => favoriteIds.has(commandId(command)));
  if (groups.size === 0 && favoriteCommands.length === 0) return null;
  const groupChoices = [
    ...(favoriteCommands.length > 0
      ? [{ name: "Favorites", value: "favorites", description: `${favoriteCommands.length} pinned command${favoriteCommands.length === 1 ? "" : "s"}` }]
      : []),
    ...orderedGroups(groups, settings).map((id) => ({
      name: GROUPS[id]?.title ?? id,
      value: id,
      description: `${groups.get(id).length} command${groups.get(id).length === 1 ? "" : "s"}`
    })),
    { name: "Back", value: null }
  ];
  const groupId = await prompt(() =>
    select({
      message: "Run",
      choices: groupChoices
    })
  );
  if (!groupId) return null;
  const selectedCommands = groupId === "favorites" ? favoriteCommands : groups.get(groupId);
  if (groupId !== "favorites") {
    settings.lastSelectedGroup = groupId;
    await writeSettings(settings);
  }
  return prompt(() =>
    select({
      message: groupId === "favorites" ? "Favorites" : GROUPS[groupId]?.title ?? groupId,
      pageSize: 12,
      choices: [
        ...selectedCommands.map((command) => ({ name: commandLabel(command), value: command, description: command.command })),
        { name: "Back", value: null }
      ]
    })
  );
}

export async function manageMenu(config) {
  while (true) {
    clearScreen();
    console.log(color.bold("Manage"));
    console.log(color.dim("Manual commands and existing entries"));
    console.log("");
    const action = await prompt(() =>
      select({
        message: "Manage",
        choices: [
          { name: "Add manual command", value: "add" },
          { name: "Edit command", value: "edit" },
          { name: "Delete command", value: "delete" },
          { name: "Back", value: "back" }
        ]
      })
    );
    if (action === "back") return config;
    if (action === "add") config = await saveCommand(config);
    if (action === "edit") config = await editCommand(config);
    if (action === "delete") config = await deleteCommand(config);
  }
}

export async function mainMenu(config, settings) {
  const commands = getCommands(config);
  banner(commands.length);
  const groups = groupCommands(commands, settings);
  console.log(groups.size === 0 ? color.dim("No commands yet. Add one manually or import detected project tools.") : color.dim(summary(commands, groups, settings)));
  console.log("");
  return prompt(() =>
    select({
      message: "Choose",
      choices: [
        { name: "Run", value: "run" },
        { name: "Manage", value: "manage" },
        { name: "Import", value: "import" },
        { name: "Preferences", value: "settings" },
        { name: "Quit", value: "quit" }
      ]
    })
  );
}

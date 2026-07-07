import { Separator, checkbox, confirm, input, number, select } from "@inquirer/prompts";
import { spawn } from "node:child_process";
import { existsSync } from "node:fs";
import path from "node:path";
import { upsertCaddySite, reloadCaddy, trustCaddy, caddyErrorMessage, caddyTrustErrorMessage, CADDYFILE } from "./caddy.js";
import { GROUPS } from "./constants.js";
import { ensureDevConfig, projectDevName, readDevConfig, writeDevConfig } from "./dev-config.js";
import { startDevServer } from "./dev-server.js";
import { detectImportSources, importCommands } from "./importers.js";
import { detectProject, discoverProjects, findProjectSearchRoot, projectLabel } from "./project-detect.js";
import { commandId, findCommand, getCommands, getGroups, groupShell, serializeCommand, serializeGroup, writeConfig, writeSettings } from "./store.js";
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
    const groupId = command.groupId ?? command.group;
    if (hidden.has(groupId)) continue;
    if (!groups.has(groupId)) groups.set(groupId, []);
    groups.get(groupId).push(command);
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

function displayList(values, fallback = "None") {
  return values.length > 0 ? values.join(", ") : color.dim(fallback);
}

export function groupDisplayLabel(config, id) {
  const group = getGroups(config).find((item) => item.id === id);
  return group?.label ?? GROUPS[id]?.label ?? id;
}

function settingsSummary(settings, config) {
  const groups = getGroups(config);
  const commands = getCommands(config);
  const commandNames = new Map(commands.map((command) => [commandId(command), commandId(command)]));
  const groupLabel = (id) => groupDisplayLabel(config, id);
  const favoriteCommands = (settings.favoriteCommands ?? []).map((id) => commandNames.get(id) ?? id);
  const hiddenGroups = (settings.hiddenGroups ?? []).map(groupLabel);
  const favoriteGroups = (settings.favoriteGroups ?? []).map(groupLabel);

  console.log(`${color.dim("Default group")}      ${groupLabel(settings.defaultGroup)}`);
  console.log(`${color.dim("Last run group")}     ${groupLabel(settings.lastSelectedGroup)}`);
  console.log(`${color.dim("Hidden groups")}      ${displayList(hiddenGroups)}`);
  console.log(`${color.dim("Favorite groups")}    ${displayList(favoriteGroups)}`);
  console.log(`${color.dim("Favorite commands")}  ${displayList(favoriteCommands)}`);
  console.log(`${color.dim("Favorite limit")}     ${settings.maxFavoriteCommands ?? 3}`);
}

function commandsToGroups(commands, config) {
  const existing = new Map(getGroups(config).map((group) => [group.id, group]));
  const groups = new Map();
  for (const command of commands) {
    const groupId = command.groupId ?? command.group ?? "manual";
    if (!groups.has(groupId)) {
      groups.set(groupId, { ...(existing.get(groupId) ?? groupShell(groupId, command.imported)), commands: [] });
    }
    groups.get(groupId).commands.push(command);
  }
  return [...groups.values()].map(serializeGroup);
}

async function chooseManualGroup(config, existingCommand = null) {
  const groups = getGroups(config).filter((group) => group.type === "manual");
  const existingGroupId = existingCommand?.groupId ?? existingCommand?.group;
  const selected = await prompt(() =>
    select({
      message: "Group",
      loop: false,
      choices: [
        ...groups.map((group) => ({
          name: group.label,
          value: group.id,
          description: group.description
        })),
        { name: "Create new group", value: "__new" }
      ],
      default: existingGroupId
    })
  );
  if (selected !== "__new") return { groupId: selected, groups };
  const id = await prompt(() =>
    input({
      message: "Group id",
      validate: (value) => /^[a-z0-9_.-]+$/.test(value.trim()) || "Use lowercase letters, numbers, dots, dashes, or underscores."
    })
  );
  const label = await prompt(() =>
    input({
      message: "Group label",
      default: id
        .trim()
        .split(/[-_.]/)
        .map((part) => `${part.slice(0, 1).toUpperCase()}${part.slice(1)}`)
        .join(" ")
    })
  );
  const description = await prompt(() => input({ message: "Group description", default: "" }));
  const group = {
    id: id.trim(),
    label: label.trim(),
    description: description.trim(),
    icon: "",
    type: "manual",
    commands: []
  };
  return { groupId: group.id, groups: [...groups, group] };
}

async function commandForm(config, existingCommand = null) {
  const groupChoice = await chooseManualGroup(config, existingCommand);
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
  return {
    command: {
      ...serializeCommand({ name: name.trim(), command: command.trim(), description: description.trim(), cwd: cwd.trim(), groupId: groupChoice.groupId }),
      groupId: groupChoice.groupId
    },
    groups: groupChoice.groups
  };
}

export async function saveCommand(config, existingCommand = null) {
  const form = await commandForm(config, existingCommand);
  const nextCommand = form.command;
  const currentId = existingCommand ? commandId(existingCommand) : null;
  const commands = getCommands(config).filter((command) => commandId(command) !== currentId && commandId(command) !== commandId(nextCommand));
  commands.push(nextCommand);
  const groups = commandsToGroups(commands, { ...config, groups: [...getGroups(config).filter((group) => group.type !== "manual"), ...form.groups] });
  await writeConfig({ ...config, groups });
  console.log(`\n${color.green("Saved")} ${color.bold(commandId(nextCommand))}`);
  await waitForEnter();
  return { ...config, groups };
}

async function chooseCommand(commands, message, filter = () => true) {
  const filtered = commands.filter(filter);
  if (filtered.length === 0) return null;
  return prompt(() =>
    select({
      message,
      loop: false,
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
  const groups = commandsToGroups(nextCommands, config);
  await writeConfig({ ...config, groups });
  console.log(`${color.red("Deleted")} ${color.bold(commandId(command))}`);
  if (!identifier) await waitForEnter();
  return { ...config, groups };
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
      loop: false,
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

async function installKmcSkill() {
  clearScreen();
  const command = "npx";
  const args = ["skills", "add", "marius4lui/kmc"];
  console.log(color.bold("Install kmc skill"));
  console.log(color.dim("This runs the official skills installer for this repository."));
  console.log("");
  console.log(`${color.cyan("$")} ${command} ${args.join(" ")}`);
  console.log("");

  const code = await new Promise((resolve, reject) => {
    const child = spawn(command, args, { stdio: "inherit", shell: process.platform === "win32" });
    child.on("error", reject);
    child.on("exit", (exitCode) => resolve(exitCode ?? 0));
  });

  if (code !== 0) throw new Error(`Skill installation failed with exit code ${code}.`);
  await waitForEnter();
}

export async function settingsMenu(settings, config) {
  while (true) {
    clearScreen();
    console.log(color.bold("Settings"));
    console.log(color.dim("Local preferences for this project"));
    console.log("");
    settingsSummary(settings, config);
    console.log("");
    const action = await prompt(() =>
      select({
        message: "Change",
        loop: false,
        choices: [
          { name: "Default group", value: "defaultGroup", description: "Choose which group opens first" },
          { name: "Hidden groups", value: "hiddenGroups", description: "Hide groups from the run menu" },
          { name: "Favorite groups", value: "favoriteGroups", description: "Pin groups near the top" },
          { name: "Favorite commands", value: "favoriteCommands", description: "Pin commands in Favorites" },
          { name: "Favorite command limit", value: "maxFavoriteCommands", description: "Set how many command favorites are allowed" },
          { name: "Validate config", value: "validate" },
          { name: "Install kmc skill", value: "installSkill" },
          { name: "Back", value: "back" }
        ]
      })
    );
    if (action === "back") return settings;
    if (action === "validate") {
      await validateScreen(config, settings);
    }
    if (action === "installSkill") {
      await installKmcSkill();
    }
    if (action === "defaultGroup") {
      const groups = getGroups(config);
      settings.defaultGroup = await prompt(() =>
        select({ message: "Default group", loop: false, choices: groups.map((group) => ({ name: group.label, value: group.id, description: group.description })) })
      );
    }
    if (action === "hiddenGroups") {
      const groups = getGroups(config);
      settings.hiddenGroups = await prompt(() =>
        checkbox({
          message: "Hidden groups",
          loop: false,
          choices: groups.map((group) => ({ name: group.label, value: group.id, checked: settings.hiddenGroups?.includes(group.id), description: group.description }))
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
          loop: false,
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
    if (action === "favoriteGroups") {
      const groups = getGroups(config);
      settings.favoriteGroups = await prompt(() =>
        checkbox({
          message: "Favorite groups",
          loop: false,
          choices: groups.map((group) => ({
            name: group.label,
            value: group.id,
            checked: settings.favoriteGroups?.includes(group.id),
            description: group.description
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
  const groupLabel = (id) => groupDisplayLabel(config, id);
  const favoriteCommands = commands.filter((command) => favoriteIds.has(commandId(command)));
  if (groups.size === 0 && favoriteCommands.length === 0) return null;
  const allGroupIds = orderedGroups(groups, settings);
  const favoriteGroupIds = (settings.favoriteGroups ?? []).filter((id) => groups.has(id));
  const otherGroupIds = allGroupIds.filter((id) => !favoriteGroupIds.includes(id));
  const groupChoices = [
    ...(favoriteCommands.length > 0
      ? [{ name: "Favorites", value: "favorites", description: `${favoriteCommands.length} pinned command${favoriteCommands.length === 1 ? "" : "s"}` }]
      : []),
    ...(favoriteGroupIds.length > 0 ? [new Separator("Favorite Groups")] : []),
    ...favoriteGroupIds.map((id) => ({
      name: groupLabel(id),
      value: id,
      description: `${groups.get(id).length} command${groups.get(id).length === 1 ? "" : "s"}`
    })),
    ...(favoriteGroupIds.length > 0 && otherGroupIds.length > 0 ? [new Separator("Other Groups")] : []),
    ...otherGroupIds.map((id) => ({
      name: groupLabel(id),
      value: id,
      description: `${groups.get(id).length} command${groups.get(id).length === 1 ? "" : "s"}`
    })),
    { name: "Back", value: null }
  ];
  const groupId = await prompt(() =>
    select({
      message: "Run",
      loop: false,
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
      message: groupId === "favorites" ? "Favorites" : groupLabel(groupId),
      loop: false,
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
        loop: false,
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

function printDevSummary(project, devConfig, caddyStatus = "not reloaded yet") {
  console.log(`${color.dim("Detected project")} ${project ? project.label : color.yellow("Not supported")}`);
  if (!devConfig) return;
  console.log(`${color.dim("Name")}             ${devConfig.name}`);
  console.log(`${color.dim("Type")}             ${projectLabel(devConfig.type)}`);
  console.log(`${color.dim("Path")}             ${devConfig.root ?? process.cwd()}`);
  console.log(`${color.dim("URL")}              ${color.cyan(`https://${devConfig.host}`)}`);
  console.log(`${color.dim("Port")}             ${devConfig.port}`);
  console.log(`${color.dim("Caddyfile")}        ${CADDYFILE}`);
  console.log(`${color.dim("Caddy")}            ${caddyStatus}`);
}

async function configureDevUrl() {
  const existing = await readDevConfig();
  const projectRoot = existing?.root ?? process.cwd();
  const project = await detectProject(projectRoot);
  if (!project) return selectDetectedDevProject();
  const devConfig = await ensureDevConfig(project, process.cwd(), projectRoot);
  await upsertCaddySite(devConfig, devConfig.root);
  try {
    await reloadCaddy();
    return { project, devConfig, caddyStatus: color.green("reloaded") };
  } catch (error) {
    return {
      project,
      devConfig,
      caddyStatus: color.yellow(caddyErrorMessage(error, CADDYFILE))
    };
  }
}

async function configureProject(project, root) {
  const existing = await readDevConfig();
  const baseConfig = existing ?? (await ensureDevConfig(project, process.cwd(), root));
  const rootChanged = existing?.root && path.resolve(existing.root) !== path.resolve(root);
  const defaultName = projectDevName(project, root);
  const name = !existing || rootChanged ? defaultName : existing.name;
  const nextConfig = {
    ...baseConfig,
    name,
    type: project.type,
    root,
    host: !existing || rootChanged ? `${name}.kmc.localhost` : existing.host
  };
  await writeDevConfig(nextConfig);
  await upsertCaddySite(nextConfig, root);
  try {
    await reloadCaddy();
    return { project, devConfig: nextConfig, caddyStatus: color.green("reloaded") };
  } catch (error) {
    return { project, devConfig: nextConfig, caddyStatus: color.yellow(caddyErrorMessage(error, CADDYFILE)) };
  }
}

async function selectDetectedDevProject() {
  const searchRoot = await findProjectSearchRoot(process.cwd());
  const projects = await discoverProjects(searchRoot);
  if (projects.length === 0) return { project: null, devConfig: null, caddyStatus: "not configured" };
  if (projects.length === 1) return configureProject(projects[0], projects[0].root);
  const root = await prompt(() =>
    select({
      message: "Project",
      loop: false,
      choices: projects.map((project) => ({
        name: `${project.name} (${project.label})`,
        value: project.root,
        description: project.root
      }))
    })
  );
  const project = projects.find((item) => item.root === root);
  return configureProject(project, root);
}

async function changeDevProjectPath(currentRoot = process.cwd()) {
  const value = await prompt(() =>
    input({
      message: "Project path",
      default: currentRoot,
      validate: (rawValue) => {
        const resolved = path.resolve(rawValue.trim() || ".");
        if (!existsSync(resolved)) return "Path does not exist.";
        if (!existsSync(path.join(resolved, "package.json"))) return "Path must contain package.json.";
        return true;
      }
    })
  );
  const root = path.resolve(value.trim() || ".");
  const project = await detectProject(root);
  if (!project) return { project, devConfig: null, caddyStatus: "not configured" };
  return configureProject(project, root);
}

export async function devUrlsMenu() {
  let state = await configureDevUrl();
  while (true) {
    clearScreen();
    console.log(color.bold("Dev URLs"));
    console.log(color.dim("Stable local project URL through Caddy"));
    console.log("");
    printDevSummary(state.project, state.devConfig, state.caddyStatus);
    console.log("");

    const choices = state.devConfig
      ? [
          { name: "Start project", value: "start", description: `Open at https://${state.devConfig.host}` },
          { name: "Reload Caddy", value: "reload", description: "Regenerate Caddyfile and run caddy reload" },
          { name: "Trust local HTTPS certs", value: "trust", description: "Install Caddy's local CA for the browser" },
          { name: "Change name/host", value: "rename", description: "Keep the same port and update the local hostname" },
          { name: "Select detected project", value: "selectProject", description: "Choose from detected monorepo apps" },
          { name: "Change project path", value: "path", description: "Use a nested app folder for detection and start" },
          { name: "Back", value: "back" }
        ]
      : [
          { name: "Select detected project", value: "selectProject", description: "Choose from detected monorepo apps" },
          { name: "Change project path", value: "path", description: "Point Dev URLs at a nested app folder" },
          { name: "Back", value: "back" }
        ];

    const action = await prompt(() => select({ message: "Dev URLs", loop: false, choices }));
    if (action === "back") return;
    if (action === "start") return startDevServer(state.devConfig);
    if (action === "reload") {
      await upsertCaddySite(state.devConfig, state.devConfig.root);
      try {
        await reloadCaddy();
        state = { ...state, caddyStatus: color.green("reloaded") };
      } catch (error) {
        state = { ...state, caddyStatus: color.yellow(caddyErrorMessage(error, CADDYFILE)) };
      }
    }
    if (action === "trust") {
      try {
        await trustCaddy();
        state = { ...state, caddyStatus: color.green("local CA trusted; restart your browser if it still warns") };
      } catch (error) {
        state = { ...state, caddyStatus: color.yellow(caddyTrustErrorMessage(error)) };
      }
    }
    if (action === "path") {
      state = await changeDevProjectPath(state.devConfig?.root ?? process.cwd());
    }
    if (action === "selectProject") {
      state = await selectDetectedDevProject();
    }
    if (action === "rename") {
      const current = await readDevConfig();
      const name = await prompt(() =>
        input({
          message: "Name",
          default: current.name,
          validate: (value) => /^[a-z0-9-]+$/.test(value.trim()) || "Use lowercase letters, numbers, and dashes."
        })
      );
      const nextConfig = { ...current, name: name.trim(), host: `${name.trim()}.kmc.localhost` };
      await writeDevConfig(nextConfig);
      await upsertCaddySite(nextConfig, nextConfig.root);
      try {
        await reloadCaddy();
        state = { ...state, devConfig: nextConfig, caddyStatus: color.green("reloaded") };
      } catch (error) {
        state = { ...state, devConfig: nextConfig, caddyStatus: color.yellow(caddyErrorMessage(error, CADDYFILE)) };
      }
    }
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
      loop: false,
      choices: [
        { name: "Run", value: "run" },
        { name: "Dev URLs", value: "devUrls" },
        { name: "Manage", value: "manage" },
        { name: "Import", value: "import" },
        { name: "Preferences", value: "settings" },
        { name: "Quit", value: "quit" }
      ]
    })
  );
}

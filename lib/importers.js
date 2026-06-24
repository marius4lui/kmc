import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { SCHEMA_URL } from "./constants.js";
import { getCommands, serializeCommand, writeConfig } from "./store.js";
import { color } from "./theme.js";

export const IMPORT_SOURCES = [
  { id: "npm", title: "NPM Scripts", files: ["package.json"] },
  { id: "make", title: "Make Commands", files: ["Makefile", "makefile"] },
  { id: "flutter", title: "Flutter Commands", files: ["pubspec.yaml"] },
  { id: "docker", title: "Docker Commands", files: ["docker-compose.yml", "docker-compose.yaml", "compose.yml", "compose.yaml"] }
];

function firstExisting(files) {
  return files.map((file) => path.join(process.cwd(), file)).find(existsSync);
}

export function detectImportSources() {
  return IMPORT_SOURCES.map((source) => {
    const filePath = firstExisting(source.files);
    return { ...source, detected: Boolean(filePath), filePath, file: filePath ? path.basename(filePath) : null };
  });
}

function parseMakefile(content) {
  const commands = [];
  for (const line of content.split(/\r?\n/)) {
    const match = line.match(/^([A-Za-z0-9_.-]+):(?!=)(?:\s|$)/);
    if (!match || match[1].startsWith(".")) continue;
    const name = match[1];
    if (commands.some((command) => command.name === name)) continue;
    commands.push({ name, command: `make ${name}`, description: `Make target ${name}` });
  }
  return commands;
}

function parseDockerCompose(content) {
  const commands = [
    { name: "up", command: "docker compose up", description: "Start Docker Compose services" },
    { name: "up-detached", command: "docker compose up -d", description: "Start Docker Compose in the background" },
    { name: "down", command: "docker compose down", description: "Stop Docker Compose services" },
    { name: "logs", command: "docker compose logs -f", description: "Follow Docker Compose logs" },
    { name: "ps", command: "docker compose ps", description: "List Docker Compose services" }
  ];
  for (const [, service] of content.matchAll(/^\s{2}([A-Za-z0-9_.-]+):\s*$/gm)) {
    commands.push({ name: `restart-${service}`, command: `docker compose restart ${service}`, description: `Restart ${service}` });
  }
  return commands;
}

async function importSource(source) {
  if (!source.detected) return [];
  if (source.id === "npm") {
    const packageJson = JSON.parse(await readFile(source.filePath, "utf8"));
    return Object.entries(packageJson.scripts ?? {}).map(([name, script]) => ({
      name,
      command: `npm run ${name}`,
      description: script
    }));
  }
  if (source.id === "make") return parseMakefile(await readFile(source.filePath, "utf8"));
  if (source.id === "flutter") {
    return [
      ["pub-get", "flutter pub get", "Install Flutter dependencies"],
      ["analyze", "flutter analyze", "Run Flutter analyzer"],
      ["test", "flutter test", "Run Flutter tests"],
      ["run", "flutter run", "Run the Flutter app"],
      ["build-apk", "flutter build apk", "Build Android APK"]
    ].map(([name, command, description]) => ({ name, command, description }));
  }
  if (source.id === "docker") return parseDockerCompose(await readFile(source.filePath, "utf8"));
  return [];
}

export async function detectImports(sourceIds = null) {
  const selected = new Set(sourceIds ?? IMPORT_SOURCES.map((source) => source.id));
  const imports = [];
  for (const source of detectImportSources()) {
    if (!source.detected || !selected.has(source.id)) continue;
    const commands = await importSource(source);
    for (const command of commands) {
      imports.push(
        serializeCommand({
          ...command,
          cwd: ".",
          group: source.id,
          source: source.file,
          imported: true
        })
      );
    }
  }
  return imports;
}

export async function importCommands(config, sourceIds, verbose = true) {
  const imports = await detectImports(sourceIds);
  const selected = new Set(sourceIds);
  const keptCommands = getCommands(config).filter((command) => !command.imported || !selected.has(command.group));
  const nextCommands = [...keptCommands, ...imports].map(serializeCommand);
  await writeConfig({ ...config, "$schema": SCHEMA_URL, commands: nextCommands });
  if (verbose) {
    const byGroup = imports.reduce((acc, command) => {
      acc[command.group] = (acc[command.group] ?? 0) + 1;
      return acc;
    }, {});
    console.log(color.green(`Imported ${imports.length} command${imports.length === 1 ? "" : "s"}.`));
    for (const [group, count] of Object.entries(byGroup)) console.log(`  ${group}: ${count}`);
  }
  return { ...config, "$schema": SCHEMA_URL, commands: nextCommands };
}

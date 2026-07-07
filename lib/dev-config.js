import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import path from "node:path";
import { DEV_CONFIG_FILE, SETTINGS_DIR } from "./constants.js";
import { ensureSettingsIgnored } from "./store.js";
import { findFreePort } from "./ports.js";

function slugifyName(value) {
  const slug = value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug || "app";
}

export function defaultProjectName(cwd = process.cwd()) {
  return slugifyName(path.basename(cwd));
}

export function projectDevName(project, cwd = process.cwd()) {
  return slugifyName(project?.name ?? path.basename(cwd));
}

export async function readDevConfig(cwd = process.cwd()) {
  const filePath = path.join(cwd, DEV_CONFIG_FILE);
  if (!existsSync(filePath)) return null;
  return JSON.parse(await readFile(filePath, "utf8"));
}

export async function writeDevConfig(config, cwd = process.cwd()) {
  await mkdir(path.join(cwd, SETTINGS_DIR), { recursive: true });
  await writeFile(path.join(cwd, DEV_CONFIG_FILE), `${JSON.stringify(config, null, 2)}\n`);
  await ensureSettingsIgnored(cwd);
}

export async function ensureDevConfig(project, cwd = process.cwd(), projectRoot = cwd) {
  const existing = await readDevConfig(cwd);
  if (existing) {
    if (!existing.root) {
      const next = { ...existing, root: projectRoot };
      await writeDevConfig(next, cwd);
      return next;
    }
    return existing;
  }
  if (!project) throw new Error("No supported dev project detected.");

  const name = defaultProjectName(projectRoot);
  const config = {
    name,
    type: project.type,
    root: projectRoot,
    port: await findFreePort(),
    host: `${name}.kmc.localhost`
  };
  await writeDevConfig(config, cwd);
  return config;
}

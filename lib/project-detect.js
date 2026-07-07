import { existsSync } from "node:fs";
import { readFile } from "node:fs/promises";
import { readdir } from "node:fs/promises";
import path from "node:path";

async function readPackageJson(cwd = process.cwd()) {
  const filePath = path.join(cwd, "package.json");
  if (!existsSync(filePath)) return null;
  return JSON.parse(await readFile(filePath, "utf8"));
}

const IGNORED_DIRS = new Set([".git", ".next", ".nuxt", ".output", "build", "coverage", "dist", "node_modules", "out"]);

function workspacePatterns(packageJson) {
  if (Array.isArray(packageJson?.workspaces)) return packageJson.workspaces;
  if (Array.isArray(packageJson?.workspaces?.packages)) return packageJson.workspaces.packages;
  return [];
}

function hasWorkspaceConfig(cwd, packageJson) {
  return (
    workspacePatterns(packageJson).length > 0 ||
    existsSync(path.join(cwd, "pnpm-workspace.yaml")) ||
    existsSync(path.join(cwd, "pnpm-workspace.yml")) ||
    existsSync(path.join(cwd, "lerna.json")) ||
    existsSync(path.join(cwd, "turbo.json")) ||
    existsSync(path.join(cwd, "nx.json"))
  );
}

export async function findProjectSearchRoot(cwd = process.cwd()) {
  let current = path.resolve(cwd);
  let fallback = current;

  while (true) {
    const packageJson = await readPackageJson(current);
    if (packageJson) {
      fallback = current;
      if (hasWorkspaceConfig(current, packageJson)) return current;
    }

    const parent = path.dirname(current);
    if (parent === current) return fallback;
    current = parent;
  }
}

function projectName(cwd, root = process.cwd()) {
  const relative = path.relative(root, cwd);
  return relative && !relative.startsWith("..") ? relative : path.basename(cwd);
}

async function directoriesForPattern(root, pattern) {
  const parts = pattern.split("/").filter(Boolean);
  const results = [];

  async function walk(current, index) {
    if (index >= parts.length) {
      results.push(current);
      return;
    }
    const part = parts[index];
    if (part === "**") {
      results.push(current);
      for (const entry of await safeReadDir(current)) {
        if (!entry.isDirectory() || IGNORED_DIRS.has(entry.name)) continue;
        await walk(path.join(current, entry.name), index);
      }
      return;
    }
    if (part === "*") {
      for (const entry of await safeReadDir(current)) {
        if (!entry.isDirectory() || IGNORED_DIRS.has(entry.name)) continue;
        await walk(path.join(current, entry.name), index + 1);
      }
      return;
    }
    await walk(path.join(current, part), index + 1);
  }

  await walk(root, 0);
  return results;
}

async function safeReadDir(cwd) {
  try {
    return await readdir(cwd, { withFileTypes: true });
  } catch {
    return [];
  }
}

async function scanForPackageDirs(root, maxDepth = 4) {
  const results = [];

  async function walk(current, depth) {
    if (existsSync(path.join(current, "package.json"))) results.push(current);
    if (depth >= maxDepth) return;
    for (const entry of await safeReadDir(current)) {
      if (!entry.isDirectory() || IGNORED_DIRS.has(entry.name)) continue;
      await walk(path.join(current, entry.name), depth + 1);
    }
  }

  await walk(root, 0);
  return results;
}

function hasDependency(packageJson, name) {
  return Boolean(packageJson?.dependencies?.[name] ?? packageJson?.devDependencies?.[name]);
}

function scriptCommand(packageJson, scriptName) {
  const command = packageJson?.scripts?.[scriptName];
  return typeof command === "string" ? command : null;
}

export async function detectProject(cwd = process.cwd()) {
  const packageJson = await readPackageJson(cwd);
  if (!packageJson) return null;

  if (hasDependency(packageJson, "next")) {
    return {
      type: "nextjs",
      label: "Next.js",
      startCommand: "npx next dev --port {port}"
    };
  }

  if (hasDependency(packageJson, "vite")) {
    return {
      type: "vite",
      label: "Vite",
      startCommand: "npx vite --host 127.0.0.1 --port {port}"
    };
  }

  if (hasDependency(packageJson, "@nestjs/core")) {
    const command = scriptCommand(packageJson, "start:dev") ?? scriptCommand(packageJson, "dev") ?? scriptCommand(packageJson, "start");
    return command
      ? {
          type: "nestjs",
          label: "NestJS",
          startCommand: `PORT={port} npm run ${command === scriptCommand(packageJson, "start:dev") ? "start:dev" : command === scriptCommand(packageJson, "dev") ? "dev" : "start"}`
        }
      : { type: "nestjs", label: "NestJS", startCommand: "PORT={port} npx nest start --watch" };
  }

  if (hasDependency(packageJson, "express")) {
    const command = scriptCommand(packageJson, "dev") ? "npm run dev" : scriptCommand(packageJson, "start") ? "npm start" : null;
    return command ? { type: "express", label: "Express", startCommand: `PORT={port} ${command}` } : null;
  }

  return null;
}

export async function discoverProjects(root = process.cwd()) {
  const rootPackageJson = await readPackageJson(root);
  const candidates = new Set([root]);

  for (const pattern of workspacePatterns(rootPackageJson)) {
    for (const directory of await directoriesForPattern(root, pattern)) candidates.add(directory);
  }

  if (candidates.size === 1) {
    for (const directory of await scanForPackageDirs(root)) candidates.add(directory);
  }

  const projects = [];
  for (const cwd of candidates) {
    const project = await detectProject(cwd);
    if (!project) continue;
    projects.push({ ...project, root: cwd, name: projectName(cwd, root) });
  }

  return projects.sort((left, right) => left.name.localeCompare(right.name));
}

export function projectLabel(type) {
  const labels = {
    nextjs: "Next.js",
    vite: "Vite",
    nestjs: "NestJS",
    express: "Express"
  };
  return labels[type] ?? type;
}

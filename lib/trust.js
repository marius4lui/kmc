import { createHash } from "node:crypto";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

export function trustFile() {
  const base = process.env.KMC_CONFIG_HOME || (process.platform === "win32"
    ? (process.env.APPDATA || path.join(os.homedir(), "AppData", "Roaming"))
    : process.platform === "darwin" ? path.join(os.homedir(), "Library", "Application Support")
      : (process.env.XDG_CONFIG_HOME || path.join(os.homedir(), ".config")));
  return path.join(base, "kmc", "trusted-repositories.json");
}

async function readStore() {
  try { return JSON.parse(await readFile(trustFile(), "utf8")); } catch (error) {
    if (error.code === "ENOENT") return { version: 1, repositories: {} };
    throw error;
  }
}

export async function repositoryFingerprint(loaded) {
  const hash = createHash("sha256");
  hash.update(await readFile(loaded.registryFile));
  for (const script of [...loaded.scripts.values()].sort((a, b) => a.file.localeCompare(b.file))) hash.update(await readFile(script.file));
  return hash.digest("hex");
}

function id(root) { return createHash("sha256").update(path.resolve(root)).digest("hex"); }

export async function trustStatus(loaded) {
  const store = await readStore();
  const entry = store.repositories[id(loaded.projectRoot)];
  const fingerprint = await repositoryFingerprint(loaded);
  return { trusted: Boolean(entry && entry.root === loaded.projectRoot && entry.fingerprint === fingerprint), changed: Boolean(entry && entry.fingerprint !== fingerprint), entry, fingerprint };
}

export async function trustRepository(loaded) {
  const store = await readStore();
  store.repositories[id(loaded.projectRoot)] = { root: loaded.projectRoot, fingerprint: await repositoryFingerprint(loaded), trustedAt: new Date().toISOString() };
  await mkdir(path.dirname(trustFile()), { recursive: true });
  await writeFile(trustFile(), `${JSON.stringify(store, null, 2)}\n`, { mode: 0o600 });
}

export async function untrustRepository(projectRoot = process.cwd()) {
  const store = await readStore();
  const removed = delete store.repositories[id(projectRoot)];
  await mkdir(path.dirname(trustFile()), { recursive: true });
  await writeFile(trustFile(), `${JSON.stringify(store, null, 2)}\n`, { mode: 0o600 });
  return removed;
}

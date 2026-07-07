import { execFile } from "node:child_process";
import { existsSync } from "node:fs";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";

const KMC_CONFIG_DIR = path.join(os.homedir(), ".config", "kmc");
const SITES_FILE = path.join(KMC_CONFIG_DIR, "dev-sites.json");
export const CADDYFILE = path.join(KMC_CONFIG_DIR, "Caddyfile");

export const CADDY_INSTALL_HINT = [
  "Caddy is not installed or not in PATH.",
  "Install it first:",
  "  Ubuntu/Debian: sudo apt install caddy",
  "  Fedora: sudo dnf install caddy",
  "  Arch: sudo pacman -S caddy",
  "  macOS: brew install caddy",
  "Then run Dev URLs > Reload Caddy again."
].join("\n");

export function caddyStartHint(configPath = CADDYFILE) {
  return [
    "Caddy is installed, but no running Caddy admin instance answered on localhost:2019.",
    "Start Caddy first:",
    `  caddy start --config ${configPath}`,
    "Then run Dev URLs > Reload Caddy again.",
    "If Caddy is managed by systemd, use:",
    "  sudo systemctl start caddy"
  ].join("\n");
}

async function readSites() {
  if (!existsSync(SITES_FILE)) return [];
  return JSON.parse(await readFile(SITES_FILE, "utf8"));
}

async function writeSites(sites) {
  await mkdir(KMC_CONFIG_DIR, { recursive: true });
  await writeFile(SITES_FILE, `${JSON.stringify(sites, null, 2)}\n`);
}

function renderCaddyfile(sites) {
  return `${sites
    .sort((left, right) => left.host.localeCompare(right.host))
    .map((site) => `${site.host} {\n    reverse_proxy localhost:${site.port}\n}`)
    .join("\n\n")}\n`;
}

export async function upsertCaddySite(config, cwd = process.cwd()) {
  const sites = await readSites();
  const nextSite = { cwd, name: config.name, type: config.type, host: config.host, port: config.port };
  const nextSites = [...sites.filter((site) => site.cwd !== cwd && site.host !== config.host), nextSite];
  await writeSites(nextSites);
  await writeFile(CADDYFILE, renderCaddyfile(nextSites));
  return CADDYFILE;
}

export function reloadCaddy(configPath = CADDYFILE) {
  return new Promise((resolve, reject) => {
    const child = execFile("caddy", ["reload", "--config", configPath], (error, stdout, stderr) => {
      if (error) {
        error.stdout = stdout;
        error.stderr = stderr;
        reject(error);
        return;
      }
      resolve({ stdout, stderr });
    });
    child.stdin?.end();
  });
}

export function trustCaddy() {
  return new Promise((resolve, reject) => {
    const child = execFile("caddy", ["trust"], (error, stdout, stderr) => {
      if (error) {
        error.stdout = stdout;
        error.stderr = stderr;
        reject(error);
        return;
      }
      resolve({ stdout, stderr });
    });
    child.stdin?.end();
  });
}

export function caddyTrustErrorMessage(error) {
  if (error?.code === "ENOENT" || /spawn caddy ENOENT/.test(error?.message ?? "")) return CADDY_INSTALL_HINT;
  return [
    "Could not install Caddy's local CA into the trust store.",
    "Run it manually in a terminal so sudo/password prompts are visible:",
    "  caddy trust",
    `Original error: ${error.message}`
  ].join("\n");
}

export function caddyErrorMessage(error, configPath = CADDYFILE) {
  if (error?.code === "ENOENT" || /spawn caddy ENOENT/.test(error?.message ?? "")) return CADDY_INSTALL_HINT;
  const output = [error?.message, error?.stdout, error?.stderr].filter(Boolean).join("\n");
  if (/localhost:2019|connect: connection refused|dial tcp .*:2019/.test(output)) return caddyStartHint(configPath);
  return `reload failed: ${error.message}`;
}

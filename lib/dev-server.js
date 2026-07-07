import { spawn } from "node:child_process";
import { projectLabel } from "./project-detect.js";
import { clearScreen, color } from "./theme.js";

export function startCommandFor(config) {
  if (config.type === "nextjs") return `npx next dev --port ${config.port}`;
  if (config.type === "vite") return `npx vite --host 127.0.0.1 --port ${config.port}`;
  if (config.type === "nestjs") return `PORT=${config.port} npm run start:dev`;
  if (config.type === "express") return `PORT=${config.port} npm run dev`;
  throw new Error(`Unsupported project type "${config.type}".`);
}

export function startDevServer(config) {
  const command = startCommandFor(config);
  clearScreen();
  console.log(`${color.bold(projectLabel(config.type))} ${color.dim("dev server")}`);
  console.log(color.dim("─".repeat(48)));
  console.log(`${color.dim("URL")} ${color.cyan(`https://${config.host}`)}`);
  if (config.root) console.log(`${color.dim("Path")} ${config.root}`);
  console.log(`${color.cyan("$")} ${command}\n`);

  const child = spawn(command, {
    cwd: config.root ?? process.cwd(),
    detached: process.platform !== "win32",
    shell: true,
    stdio: "inherit",
    env: process.env
  });

  let exiting = false;
  function stop(signal = "SIGINT") {
    if (exiting) return;
    exiting = true;
    if (child.pid) {
      try {
        if (process.platform === "win32") child.kill(signal);
        else process.kill(-child.pid, signal);
      } catch {
        child.kill(signal);
      }
    }
    setTimeout(() => {
      if (!child.killed && child.pid) {
        try {
          if (process.platform === "win32") child.kill("SIGTERM");
          else process.kill(-child.pid, "SIGTERM");
        } catch {
          child.kill("SIGTERM");
        }
      }
    }, 1500).unref();
  }

  return new Promise((resolve) => {
    const onSigint = () => stop("SIGINT");
    const onSigterm = () => stop("SIGTERM");
    process.once("SIGINT", onSigint);
    process.once("SIGTERM", onSigterm);

    child.on("exit", (code, signal) => {
      exiting = true;
      process.off("SIGINT", onSigint);
      process.off("SIGTERM", onSigterm);
      resolve({ code: code ?? 0, signal });
    });
  });
}

import { spawn } from "node:child_process";
import path from "node:path";
import { commandId } from "./store.js";
import { clearScreen, color } from "./theme.js";

export function runCommand(command) {
  clearScreen();
  console.log(`${color.bold(commandId(command))} ${color.dim("is running")}`);
  console.log(color.dim("─".repeat(48)));
  console.log(`${color.cyan("$")} ${command.command}\n`);

  const child = spawn(command.command, {
    cwd: path.resolve(process.cwd(), command.cwd || "."),
    shell: true,
    stdio: "inherit",
    env: process.env
  });

  child.on("exit", (code, signal) => {
    if (signal) {
      process.kill(process.pid, signal);
      return;
    }
    process.exit(code ?? 0);
  });
}

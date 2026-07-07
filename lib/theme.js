export const color = {
  dim: (value) => `\x1b[2m${value}\x1b[22m`,
  cyan: (value) => `\x1b[36m${value}\x1b[39m`,
  green: (value) => `\x1b[32m${value}\x1b[39m`,
  red: (value) => `\x1b[31m${value}\x1b[39m`,
  yellow: (value) => `\x1b[33m${value}\x1b[39m`,
  bold: (value) => `\x1b[1m${value}\x1b[22m`
};

export function clearScreen() {
  if (!process.stdout.isTTY) return;
  process.stdout.write("\x1b[2J\x1b[3J\x1b[H");
}

export async function waitForEnter(message = "OK") {
  const { input } = await import("@inquirer/prompts");
  await input({ message, default: "" });
}

export function banner(commandCount) {
  clearScreen();
  const count = `${commandCount} command${commandCount === 1 ? "" : "s"}`;
  console.log(color.cyan("╭────────────────────────────────────────╮"));
  console.log(`${color.cyan("│")} ${color.bold("kmc")} ${color.dim("interactive command center")}       ${color.cyan("│")}`);
  console.log(`${color.cyan("│")} ${color.dim(`kmc.json · ${count}`.padEnd(38, " "))}${color.cyan("│")}`);
  console.log(color.cyan("╰────────────────────────────────────────╯"));
  console.log("");
}

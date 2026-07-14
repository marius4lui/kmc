let colorsEnabled = !process.env.NO_COLOR;
const paint = (open, close, value) => colorsEnabled ? `\x1b[${open}m${value}\x1b[${close}m` : String(value);

export function setColorsEnabled(enabled) { colorsEnabled = enabled; }

export const color = {
  dim: (value) => paint(2, 22, value),
  cyan: (value) => paint(36, 39, value),
  green: (value) => paint(32, 39, value),
  red: (value) => paint(31, 39, value),
  yellow: (value) => paint(33, 39, value),
  bold: (value) => paint(1, 22, value)
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

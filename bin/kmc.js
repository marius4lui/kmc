#!/usr/bin/env node

import { main } from "../lib/cli.js";
import { color } from "../lib/theme.js";

main().catch((error) => {
  if (error.name === "ExitPromptError" || error.name === "Back") {
    console.log("");
    process.exit(0);
  }

  if (!error.reported) console.error(`${color.red("kmc:")} ${error.message}`);
  process.exit(process.exitCode || 1);
});

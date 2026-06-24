# kmc

`kmc` is an interactive npm CLI for running project commands from `kmc.json`.

Run one command:

```sh
kmc
```

The menu lets you add, edit, delete, and run commands with a modern terminal prompt. If `kmc.json` does not exist yet, `kmc` creates it automatically when you add the first command.

## Install

```sh
npm install -g kmc
```

## Usage

```sh
kmc
```

Inside the menu:

- use the arrow keys to choose an action
- press Enter to run, add, edit, delete, or quit
- answer the prompts when adding or editing a command

## `kmc.json`

You normally manage this file through the interactive CLI, but it is plain JSON and can also be edited by hand:

```json
{
  "$schema": "https://raw.githubusercontent.com/marius/kmc/main/schema.json",
  "commands": [
    {
      "name": "deploy",
      "command": "python deploy.py",
      "description": "Deploy this project",
      "cwd": "."
    },
    {
      "name": "dev",
      "command": "npm run dev",
      "description": "Start the development server",
      "cwd": "."
    }
  ]
}
```

Fields:

- `name`: short name shown in the menu.
- `command`: shell command to execute.
- `description`: optional text shown in the menu.
- `cwd`: optional working directory relative to the current folder. Defaults to `.`.

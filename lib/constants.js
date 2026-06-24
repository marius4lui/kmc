import path from "node:path";

export const CONFIG_FILE = "kmc.json";
export const SETTINGS_DIR = ".kmc";
export const SETTINGS_FILE = path.join(SETTINGS_DIR, "settings.json");
export const SCHEMA_URL = "https://github.com/marius4lui/kmc/blob/main/schema.json";

export const GROUPS = {
  development: {
    id: "development",
    label: "Development",
    description: "Local development commands",
    icon: "",
    type: "manual"
  },
  deployment: {
    id: "deployment",
    label: "Deployment",
    description: "Release and deployment commands",
    icon: "",
    type: "manual"
  },
  database: {
    id: "database",
    label: "Database",
    description: "Database maintenance commands",
    icon: "",
    type: "manual"
  },
  manual: {
    id: "manual",
    label: "Manual Commands",
    description: "User-created commands",
    icon: "",
    type: "manual"
  },
  npm: {
    id: "npm",
    label: "NPM Scripts",
    description: "Commands imported from package.json",
    icon: "",
    type: "imported",
    source: "package.json"
  },
  make: {
    id: "make",
    label: "Make Commands",
    description: "Commands imported from Makefile",
    icon: "",
    type: "imported",
    source: "Makefile"
  },
  flutter: {
    id: "flutter",
    label: "Flutter",
    description: "Commands imported from pubspec.yaml",
    icon: "",
    type: "imported",
    source: "pubspec.yaml"
  },
  docker: {
    id: "docker",
    label: "Docker",
    description: "Commands imported from Docker Compose",
    icon: "",
    type: "imported",
    source: "docker-compose.yml"
  },
  skills: {
    id: "skills",
    label: "Skills",
    description: "Commands provided by agents or skills",
    icon: "",
    type: "skill"
  }
};

export const DEFAULT_SETTINGS = {
  defaultGroup: "development",
  lastSelectedGroup: "development",
  hiddenGroups: [],
  favoriteGroups: [],
  favoriteCommands: [],
  maxFavoriteCommands: 3
};

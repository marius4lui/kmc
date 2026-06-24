import path from "node:path";

export const CONFIG_FILE = "kmc.json";
export const SETTINGS_DIR = ".kmc";
export const SETTINGS_FILE = path.join(SETTINGS_DIR, "settings.json");
export const SCHEMA_URL = "https://github.com/marius4lui/kmc/blob/main/schema.json";

export const GROUPS = {
  manual: { id: "manual", title: "Manual Commands", source: "manual" },
  npm: { id: "npm", title: "NPM Scripts", source: "package.json" },
  make: { id: "make", title: "Make Commands", source: "Makefile" },
  flutter: { id: "flutter", title: "Flutter Commands", source: "pubspec.yaml" },
  docker: { id: "docker", title: "Docker Commands", source: "docker-compose.yml" }
};

export const DEFAULT_SETTINGS = {
  defaultGroup: "manual",
  lastSelectedGroup: "manual",
  hiddenGroups: [],
  favoriteCommands: [],
  maxFavoriteCommands: 3
};

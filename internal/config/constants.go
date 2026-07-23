package config

const (
	ConfigFile   = "kmc.json"
	SettingsDir  = ".kmc"
	SettingsFile = ".kmc/settings.json"
	SchemaURL    = "https://github.com/marius4lui/kmc/blob/main/schema.json"
)

type GroupDefaults struct {
	ID          string
	Label       string
	Description string
	Icon        string
	Type        string
	Source      string
}

var KnownGroups = map[string]GroupDefaults{
	"development": {ID: "development", Label: "Development", Description: "Local development commands", Type: "manual"},
	"deployment":  {ID: "deployment", Label: "Deployment", Description: "Release and deployment commands", Type: "manual"},
	"database":    {ID: "database", Label: "Database", Description: "Database maintenance commands", Type: "manual"},
	"manual":      {ID: "manual", Label: "Manual Commands", Description: "User-created commands", Type: "manual"},
	"npm":         {ID: "npm", Label: "NPM Scripts", Description: "Commands imported from package.json", Type: "imported", Source: "package.json"},
	"make":        {ID: "make", Label: "Make Commands", Description: "Commands imported from Makefile", Type: "imported", Source: "Makefile"},
	"flutter":     {ID: "flutter", Label: "Flutter", Description: "Commands imported from pubspec.yaml", Type: "imported", Source: "pubspec.yaml"},
	"docker":      {ID: "docker", Label: "Docker", Description: "Commands imported from Docker Compose", Type: "imported", Source: "docker-compose.yml"},
	"skills":      {ID: "skills", Label: "Skills", Description: "Commands provided by agents or skills", Type: "skill"},
}

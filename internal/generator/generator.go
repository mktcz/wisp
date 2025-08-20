package generator

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mktcz/wisp/internal/analyzer"
)

func GenerateConfig(project *analyzer.ProjectStructure) string {
	var config strings.Builder

	config.WriteString(fmt.Sprintf("# wisp.toml - %s\n", project.Name))
	config.WriteString("# Generated automatically by Wisp - customize as needed\n\n")

	if project.HasCmd && len(project.CmdDirs) > 0 {

		for i, cmdDir := range project.CmdDirs {
			appName := filepath.Base(cmdDir)

			config.WriteString(fmt.Sprintf("# %s application\n", strings.Title(appName)))
			config.WriteString(fmt.Sprintf("[%s]\n", appName))
			config.WriteString(fmt.Sprintf("  cmd = \"go build -o ./tmp/%s ./%s\"\n", appName, cmdDir))
			config.WriteString(fmt.Sprintf("  bin = \"./tmp/%s\"\n", appName))
			config.WriteString("  args = []\n")
			config.WriteString(fmt.Sprintf("  watch_dir = \"./%s\"\n", cmdDir))
			config.WriteString("  tmp_dir = \"./tmp\"\n")
			config.WriteString("  delay = 1000\n")
			config.WriteString("  exclude_dir = [\"vendor\", \"tmp\", \"testdata\", \".git\"]\n")
			config.WriteString("  exclude_file = [\"*_test.go\", \"*.log\"]\n")
			config.WriteString("  clean_on_exit = true\n")

			if appName == "api" || appName == "server" || appName == "web" {
				config.WriteString("  env = { PORT = \"8080\", GIN_MODE = \"release\" }\n")
			} else {
				config.WriteString("  env = {}\n")
			}

			if i < len(project.CmdDirs)-1 {
				config.WriteString("\n")
			}
		}
	} else {

		appName := project.Name
		if appName == "" {
			appName = "app"
		}

		config.WriteString(fmt.Sprintf("# %s application\n", strings.Title(appName)))
		config.WriteString(fmt.Sprintf("[%s]\n", appName))
		config.WriteString(fmt.Sprintf("  cmd = \"go build -o ./tmp/%s .\"\n", appName))
		config.WriteString(fmt.Sprintf("  bin = \"./tmp/%s\"\n", appName))
		config.WriteString("  args = []\n")
		config.WriteString("  watch_dir = \".\"\n")
		config.WriteString("  tmp_dir = \"./tmp\"\n")
		config.WriteString("  delay = 1000\n")
		config.WriteString("  exclude_dir = [\"vendor\", \"tmp\", \"testdata\", \".git\"]\n")
		config.WriteString("  exclude_file = [\"*_test.go\", \"*.log\"]\n")
		config.WriteString("  clean_on_exit = true\n")
		config.WriteString("  env = { PORT = \"8080\", GIN_MODE = \"release\" }\n")
	}

	return config.String()
}

func GenerateSummary(project *analyzer.ProjectStructure) string {
	var summary strings.Builder

	summary.WriteString(fmt.Sprintf("📁 Project: %s\n", project.Name))

	if project.GoModulePath != "" {
		summary.WriteString(fmt.Sprintf("📦 Module: %s\n", project.GoModulePath))
	}

	if project.HasCmd {
		summary.WriteString(fmt.Sprintf("🚀 Commands found: %d\n", len(project.CmdDirs)))
		for _, cmd := range project.CmdDirs {
			summary.WriteString(fmt.Sprintf("   • %s\n", cmd))
		}
	}

	summary.WriteString(fmt.Sprintf("👁️  Watch directories: %d\n", len(project.WatchDirs)))
	for _, dir := range project.WatchDirs {
		summary.WriteString(fmt.Sprintf("   • %s\n", dir))
	}

	return summary.String()
}

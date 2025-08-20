package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// represents the analyzed Go project structure
type ProjectStructure struct {
	Name         string
	MainFile     string
	BuildDir     string
	OutputName   string
	WatchDirs    []string
	HasCmd       bool
	CmdDirs      []string
	GoModulePath string
}

// analyzes the current directory to determine Go project structure
func AnalyzeProject(dir string) (*ProjectStructure, error) {
	if dir == "" {
		dir = "."
	}

	project := &ProjectStructure{
		Name:      filepath.Base(dir),
		BuildDir:  "/tmp",
		WatchDirs: []string{},
	}

	// get absolute path for better name detection
	absDir, err := filepath.Abs(dir)
	if err == nil {
		project.Name = filepath.Base(absDir)
	}

	// analyze project structure
	if err := analyzeGo(dir, project); err != nil {
		// if no go.mod found, treat as generic project
		project.WatchDirs = []string{"."}
		project.OutputName = project.Name
	}

	return project, nil
}

// analyzes Go project structure
func analyzeGo(dir string, project *ProjectStructure) error {
	// check for go.mod
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); err != nil {
		return fmt.Errorf("no go.mod found")
	}

	// read go.mod to get module path and name
	if data, err := os.ReadFile(goModPath); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "module ") {
				project.GoModulePath = strings.TrimSpace(strings.TrimPrefix(line, "module"))
				// use module name as project name if it's more descriptive
				parts := strings.Split(project.GoModulePath, "/")
				if len(parts) > 0 {
					modName := parts[len(parts)-1]
					if modName != "" && modName != "." {
						project.Name = modName
					}
				}
				break
			}
		}
	}

	// check for cmd directory structure
	cmdDir := filepath.Join(dir, "cmd")
	if stat, err := os.Stat(cmdDir); err == nil && stat.IsDir() {
		project.HasCmd = true

		// find all subdirectories in cmd
		entries, err := os.ReadDir(cmdDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					cmdPath := filepath.Join("cmd", entry.Name())
					project.CmdDirs = append(project.CmdDirs, cmdPath)
				}
			}
		}

		// set primary watch directory
		if len(project.CmdDirs) > 0 {
			project.WatchDirs = project.CmdDirs
		} else {
			project.WatchDirs = []string{"./cmd"}
		}
	} else {
		// check for main.go in root
		if _, err := os.Stat(filepath.Join(dir, "main.go")); err == nil {
			project.MainFile = "main.go"
			project.WatchDirs = []string{"."}
		} else {
			// look for .go files in subdirectories
			project.WatchDirs = []string{"."}
		}
	}

	project.OutputName = project.Name
	return nil
}

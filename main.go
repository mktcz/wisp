package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/mktcz/wisp/internal/analyzer"
	"github.com/mktcz/wisp/internal/config"
	"github.com/mktcz/wisp/internal/generator"
	"github.com/mktcz/wisp/internal/runner"
)

const (
	version = "1.0.0"
	banner  = `
╦ ╦╦╔═╗╔═╗
║║║║╚═╗╠═╝
╚╩╝╩╚═╝╩  
`
)

func main() {
	log.SetFlags(log.Ltime)
	log.SetPrefix("wisp: ")

	var (
		showHelp    bool
		showVersion bool
		configFile  string
	)

	flag.BoolVar(&showHelp, "help", false, "Show help message")
	flag.BoolVar(&showHelp, "h", false, "Show help message (shorthand)")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (shorthand)")
	flag.StringVar(&configFile, "config", "wisp.toml", "Path to configuration file")
	flag.StringVar(&configFile, "c", "wisp.toml", "Path to configuration file (shorthand)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n", banner)
		fmt.Fprintf(os.Stderr, "Wisp %s - A lighter, simpler Go development runner\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  wisp              Run all applications defined in wisp.toml\n")
		fmt.Fprintf(os.Stderr, "  wisp init         Create a sample wisp.toml configuration\n")
		fmt.Fprintf(os.Stderr, "  wisp run <app>    Run a specific application\n")
		fmt.Fprintf(os.Stderr, "  wisp --help       Show this help message\n")
		fmt.Fprintf(os.Stderr, "  wisp --version    Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fmt.Fprintf(os.Stderr, "  -c, --config      Path to configuration file (default: wisp.toml)\n")
		fmt.Fprintf(os.Stderr, "  -h, --help        Show help message\n")
		fmt.Fprintf(os.Stderr, "  -v, --version     Show version information\n\n")
		fmt.Fprintf(os.Stderr, "Examples:\n")
		fmt.Fprintf(os.Stderr, "  wisp              # Run all apps defined in wisp.toml\n")
		fmt.Fprintf(os.Stderr, "  wisp run api      # Run only the 'api' application\n")
		fmt.Fprintf(os.Stderr, "  wisp init         # Create a sample wisp.toml file\n")
		fmt.Fprintf(os.Stderr, "  wisp -c custom.toml  # Use a custom config file\n\n")
	}

	flag.Parse()

	if showVersion {
		fmt.Printf("Wisp version %s\n", version)
		os.Exit(0)
	}

	if showHelp {
		flag.Usage()
		os.Exit(0)
	}

	args := flag.Args()
	command := ""
	if len(args) > 0 {
		command = args[0]
	}

	switch command {
	case "init":
		handleInit()
	case "run":
		if len(args) < 2 {
			log.Fatal("Error: 'run' command requires an application name")
		}
		handleRun(configFile, args[1:]...)
	case "":
		// run all apps
		handleRun(configFile)
	default:
		log.Fatalf("Unknown command: %s\nRun 'wisp --help' for usage", command)
	}
}

func handleInit() {
	filename := "wisp.toml"

	if _, err := os.Stat(filename); err == nil {
		fmt.Printf("File %s already exists. Overwrite? (y/N): ", filename)
		var response string
		fmt.Scanln(&response)
		if !strings.HasPrefix(strings.ToLower(response), "y") {
			fmt.Println("Cancelled.")
			os.Exit(0)
		}
	}

	fmt.Println("🔍 Analyzing project structure...")
	project, err := analyzer.AnalyzeProject(".")
	if err != nil {
		log.Fatalf("Failed to analyze project: %v", err)
	}

	generatedConfig := generator.GenerateConfig(project)
	if err := os.WriteFile(filename, []byte(generatedConfig), 0644); err != nil {
		log.Fatalf("Failed to create %s: %v", filename, err)
	}

	fmt.Printf("✓ Created %s with Go project configuration\n\n", filename)
	fmt.Print(generator.GenerateSummary(project))
	fmt.Println("\nCustomize the configuration to fit your project's needs!")
	fmt.Println("\nTo run all applications:")
	fmt.Println("  $ wisp")
	if project.HasCmd && len(project.CmdDirs) > 0 {
		fmt.Printf("\nTo run a specific application:\n")
		for _, cmdDir := range project.CmdDirs {
			appName := strings.TrimPrefix(cmdDir, "cmd/")
			fmt.Printf("  $ wisp run %s\n", appName)
		}
	}
}

// loads the configuration and runs the specified apps
func handleRun(configFile string, appNames ...string) {
	// print banner
	fmt.Print(banner)
	fmt.Printf("Wisp %s - Starting...\n\n", version)

	// load configuration
	cfg, err := config.Load(configFile)
	if err != nil {
		if os.IsNotExist(err) || strings.Contains(err.Error(), "not found") {
			log.Printf("Configuration file '%s' not found.\n", configFile)
			log.Println("Run 'wisp init' to create a sample configuration.")
			os.Exit(1)
		}
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// validate cfg
	if len(cfg.Apps) == 0 {
		log.Fatal("No applications configured in wisp.toml")
	}

	// create and run the runner
	r := runner.New(cfg)

	// run specified apps or all apps if no app names are provided
	if err := r.Run(appNames...); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

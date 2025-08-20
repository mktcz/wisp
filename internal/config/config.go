package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type App struct {
	Name          string
	RunCmd        string            `toml:"run_cmd"`
	BuildCmd      string            `toml:"build_cmd"`
	Cmd           string            `toml:"cmd"`
	Bin           string            `toml:"bin"`
	Args          []string          `toml:"args"`
	WatchDir      string            `toml:"watch_dir"`
	TmpDir        string            `toml:"tmp_dir"`
	Env           map[string]string `toml:"env"`
	Delay         int               `toml:"delay"`
	KillDelay     string            `toml:"kill_delay"`
	Rerun         bool              `toml:"rerun"`
	RerunDelay    int               `toml:"rerun_delay"`
	ExcludeDir    []string          `toml:"exclude_dir"`
	ExcludeFile   []string          `toml:"exclude_file"`
	ExcludeRegex  []string          `toml:"exclude_regex"`
	FollowSymlink bool              `toml:"follow_symlink"`
	PreCmd        []string          `toml:"pre_cmd"`
	PostCmd       []string          `toml:"post_cmd"`
	SendInterrupt bool              `toml:"send_interrupt"`
	StopOnError   bool              `toml:"stop_on_error"`
	LogSilent     bool              `toml:"log_silent"`
	CleanOnExit   bool              `toml:"clean_on_exit"`
}

type Config struct {
	Apps map[string]*App
}

func Load(configPath string) (*Config, error) {
	if configPath == "" {
		configPath = "wisp.toml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("configuration file %s not found", configPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var rawConfig map[string]interface{}
	if err := toml.Unmarshal(data, &rawConfig); err != nil {
		return nil, fmt.Errorf("failed to parse TOML: %w", err)
	}

	config := &Config{
		Apps: make(map[string]*App),
	}

	for name, value := range rawConfig {
		appMap, ok := value.(map[string]interface{})
		if !ok {
			continue
		}

		app := &App{
			Name: name,
		}

		if runCmd, ok := appMap["run_cmd"].(string); ok {
			app.RunCmd = runCmd
		}
		if buildCmd, ok := appMap["build_cmd"].(string); ok {
			app.BuildCmd = buildCmd
		}
		if cmd, ok := appMap["cmd"].(string); ok {
			app.Cmd = cmd
		}
		if bin, ok := appMap["bin"].(string); ok {
			app.Bin = bin
		}
		if watchDir, ok := appMap["watch_dir"].(string); ok {
			app.WatchDir = watchDir
		} else {

			app.WatchDir = "."
		}
		if tmpDir, ok := appMap["tmp_dir"].(string); ok {
			app.TmpDir = tmpDir
		} else {
			app.TmpDir = "/tmp"
		}
		if killDelay, ok := appMap["kill_delay"].(string); ok {
			app.KillDelay = killDelay
		}

		if delay, ok := appMap["delay"].(int64); ok {
			app.Delay = int(delay)
		} else {
			app.Delay = 1000
		}
		if rerunDelay, ok := appMap["rerun_delay"].(int64); ok {
			app.RerunDelay = int(rerunDelay)
		} else {
			app.RerunDelay = 500
		}

		if rerun, ok := appMap["rerun"].(bool); ok {
			app.Rerun = rerun
		}
		if followSymlink, ok := appMap["follow_symlink"].(bool); ok {
			app.FollowSymlink = followSymlink
		}
		if sendInterrupt, ok := appMap["send_interrupt"].(bool); ok {
			app.SendInterrupt = sendInterrupt
		}
		if stopOnError, ok := appMap["stop_on_error"].(bool); ok {
			app.StopOnError = stopOnError
		}
		if logSilent, ok := appMap["log_silent"].(bool); ok {
			app.LogSilent = logSilent
		}
		if cleanOnExit, ok := appMap["clean_on_exit"].(bool); ok {
			app.CleanOnExit = cleanOnExit
		}

		if args, ok := appMap["args"].([]interface{}); ok {
			for _, arg := range args {
				if strArg, ok := arg.(string); ok {
					app.Args = append(app.Args, strArg)
				}
			}
		}
		if excludeDir, ok := appMap["exclude_dir"].([]interface{}); ok {
			for _, dir := range excludeDir {
				if strDir, ok := dir.(string); ok {
					app.ExcludeDir = append(app.ExcludeDir, strDir)
				}
			}
		}
		if excludeFile, ok := appMap["exclude_file"].([]interface{}); ok {
			for _, file := range excludeFile {
				if strFile, ok := file.(string); ok {
					app.ExcludeFile = append(app.ExcludeFile, strFile)
				}
			}
		}
		if excludeRegex, ok := appMap["exclude_regex"].([]interface{}); ok {
			for _, regex := range excludeRegex {
				if strRegex, ok := regex.(string); ok {
					app.ExcludeRegex = append(app.ExcludeRegex, strRegex)
				}
			}
		}
		if preCmd, ok := appMap["pre_cmd"].([]interface{}); ok {
			for _, cmd := range preCmd {
				if strCmd, ok := cmd.(string); ok {
					app.PreCmd = append(app.PreCmd, strCmd)
				}
			}
		}
		if postCmd, ok := appMap["post_cmd"].([]interface{}); ok {
			for _, cmd := range postCmd {
				if strCmd, ok := cmd.(string); ok {
					app.PostCmd = append(app.PostCmd, strCmd)
				}
			}
		}

		if envMap, ok := appMap["env"].(map[string]interface{}); ok {
			app.Env = make(map[string]string)
			for k, v := range envMap {
				if strVal, ok := v.(string); ok {
					app.Env[k] = strVal
				}
			}
		}

		config.Apps[name] = app
	}

	for _, app := range config.Apps {
		if !filepath.IsAbs(app.WatchDir) {
			absPath, err := filepath.Abs(app.WatchDir)
			if err == nil {
				app.WatchDir = absPath
			}
		}
	}

	return config, nil
}

func SampleConfig() string {
	return `# wisp.toml - Wisp configuration file

# The main API server
[api]
  # required: command to run after build
  run_cmd = "/tmp/api-server"
  # OR use bin + args:
  # bin = "/tmp/api-server"
  # args = ["-verbose", "-config=dev"]
  
  # build configuration
  build_cmd = "go build -o /tmp/api-server ./cmd/api"
  # OR use 'cmd' as alias:
  # cmd = "go build -o /tmp/api-server ./cmd/api"
  
  # watch configuration
  watch_dir = "./cmd/api"
  # exclude_dir = ["assets", "tmp", "vendor", "testdata"]
  # exclude_file = ["*.log", "*.tmp"]
  # exclude_regex = [".*_test\\.go$"]
  # follow_symlink = false
  
  # timing configuration (all in milliseconds unless specified)
  # delay = 1000                    # Delay before starting (ms)
  # kill_delay = "500ms"           # Delay after stopping
  # rerun = false                   # Rerun even if build fails
  # rerun_delay = 500              # Delay before rerun (ms)
  
  # command hooks
  # pre_cmd = ["echo 'Building...'"]   # Commands before build
  # post_cmd = ["echo 'Built'"]        # Commands after build
  
  # process control
  # send_interrupt = false          # Send SIGINT instead of SIGTERM
  # stop_on_error = false          # Stop watching on error
  # log_silent = false             # Suppress app output
  # clean_on_exit = false          # Clean tmp files on exit
  # tmp_dir = "/tmp"              # Temp directory path
  
  # environment variables
  env = { PORT = "8080", GIN_MODE = "debug" }
`
}

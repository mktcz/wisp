package process

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/mktcz/wisp/internal/config"
)

type Manager struct {
	app          *config.App
	cmd          *exec.Cmd
	mu           sync.Mutex
	running      bool
	stopDelay    time.Duration
	startDelay   time.Duration
	buildTimeout time.Duration
	tmpFiles     []string
}

func NewManager(app *config.App) *Manager {
	return &Manager{
		app:          app,
		stopDelay:    500 * time.Millisecond,
		startDelay:   200 * time.Millisecond,
		buildTimeout: 30 * time.Second,
	}
}

func (m *Manager) SetDelays(stopDelay, startDelay time.Duration) {
	m.stopDelay = stopDelay
	m.startDelay = startDelay
}

func (m *Manager) Restart() error {
	if !m.app.LogSilent {
		log.Printf("[%s] Restarting...", m.app.Name)
	}

	if err := m.Stop(); err != nil {
		log.Printf("[%s] Warning: failed to stop cleanly: %v", m.app.Name, err)
	}

	if m.app.KillDelay != "" {
		if duration, err := time.ParseDuration(m.app.KillDelay); err == nil && duration > 0 {
			if !m.app.LogSilent {
				log.Printf("[%s] Waiting %v after stop...", m.app.Name, duration)
			}
			time.Sleep(duration)
		}
	}

	for _, preCmd := range m.app.PreCmd {
		if !m.app.LogSilent {
			log.Printf("[%s] Running pre-command: %s", m.app.Name, preCmd)
		}
		if err := m.runCommand(preCmd); err != nil {
			log.Printf("[%s] Pre-command failed: %v", m.app.Name, err)
			if m.app.StopOnError {
				return fmt.Errorf("pre-command failed: %w", err)
			}
		}
	}

	buildCmd := m.app.BuildCmd
	if buildCmd == "" && m.app.Cmd != "" {
		buildCmd = m.app.Cmd
	}

	if buildCmd != "" {
		if !m.app.LogSilent {
			log.Printf("[%s] Building: %s", m.app.Name, buildCmd)
		}
		if err := m.runCommand(buildCmd); err != nil {
			log.Printf("[%s] Build failed: %v", m.app.Name, err)

			if !m.app.Rerun {
				if m.app.StopOnError {
					return fmt.Errorf("build failed: %w", err)
				}
				return err
			}

			if m.app.RerunDelay > 0 {
				time.Sleep(time.Duration(m.app.RerunDelay) * time.Millisecond)
			}
		} else if !m.app.LogSilent {
			log.Printf("[%s] Build successful", m.app.Name)
		}

		if m.app.Bin != "" && m.app.CleanOnExit {
			m.tmpFiles = append(m.tmpFiles, m.app.Bin)
		}
	}

	for _, postCmd := range m.app.PostCmd {
		if !m.app.LogSilent {
			log.Printf("[%s] Running post-command: %s", m.app.Name, postCmd)
		}
		if err := m.runCommand(postCmd); err != nil {
			log.Printf("[%s] Post-command failed: %v", m.app.Name, err)
			if m.app.StopOnError {
				return fmt.Errorf("post-command failed: %w", err)
			}
		}
	}

	if m.app.Delay > 0 {
		if !m.app.LogSilent {
			log.Printf("[%s] Waiting %dms before starting...", m.app.Name, m.app.Delay)
		}
		time.Sleep(time.Duration(m.app.Delay) * time.Millisecond)
	}

	if err := m.Start(); err != nil {
		log.Printf("[%s] Failed to start: %v", m.app.Name, err)
		if m.app.StopOnError {
			return fmt.Errorf("failed to start: %w", err)
		}
		return err
	}

	return nil
}

func (m *Manager) runCommand(command string) error {
	if command == "" {
		return nil
	}

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = "."

	cmd.Env = os.Environ()
	for key, value := range m.app.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %v\nOutput:\n%s", err, string(output))
	}

	if !m.app.LogSilent && len(output) > 0 {
		log.Printf("[%s] %s", m.app.Name, string(output))
	}

	return nil
}

/*
	func (m *Manager) build() error {
		if m.app.BuildCmd == "" {
			return nil
		}

		parts := strings.Fields(m.app.BuildCmd)
		if len(parts) == 0 {
			return fmt.Errorf("empty build command")
		}

		cmd := exec.Command(parts[0], parts[1:]...)
		cmd.Dir = "."

		cmd.Env = os.Environ()
		for key, value := range m.app.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("build failed: %v\nOutput:\n%s", err, string(output))
		}

		return nil
	}
*/

func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("process already running")
	}

	var cmdParts []string

	if m.app.Bin != "" {

		cmdParts = append([]string{m.app.Bin}, m.app.Args...)
	} else if m.app.RunCmd != "" {

		cmdParts = strings.Fields(m.app.RunCmd)
	} else {
		if !m.app.LogSilent {
			log.Printf("[%s] No run command specified, skipping", m.app.Name)
		}
		return nil
	}

	if len(cmdParts) == 0 {
		return fmt.Errorf("empty run command")
	}

	m.cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	m.cmd.Dir = "."

	m.cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	m.cmd.Env = os.Environ()
	for key, value := range m.app.Env {
		m.cmd.Env = append(m.cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	stdout, err := m.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := m.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := m.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	m.running = true

	go m.streamOutput(stdout, "stdout")
	go m.streamOutput(stderr, "stderr")

	go func() {
		err := m.cmd.Wait()
		m.mu.Lock()
		m.running = false
		m.cmd = nil
		m.mu.Unlock()

		if err != nil {
			log.Printf("[%s] Process exited with error: %v", m.app.Name, err)
		} else {
			log.Printf("[%s] Process exited normally", m.app.Name)
		}
	}()

	log.Printf("[%s] Started successfully (PID: %d)", m.app.Name, m.cmd.Process.Pid)
	return nil
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running || m.cmd == nil || m.cmd.Process == nil {
		return nil
	}

	if !m.app.LogSilent {
		log.Printf("[%s] Stopping process (PID: %d)...", m.app.Name, m.cmd.Process.Pid)
	}

	signal := syscall.SIGTERM
	if m.app.SendInterrupt {
		signal = syscall.SIGINT
	}

	pgid, err := syscall.Getpgid(m.cmd.Process.Pid)
	if err == nil {

		if err := syscall.Kill(-pgid, signal); err != nil {
			log.Printf("[%s] Failed to send signal to process group: %v", m.app.Name, err)
		}
	} else {

		if err := m.cmd.Process.Signal(signal); err != nil {
			log.Printf("[%s] Failed to send signal: %v", m.app.Name, err)
		}
	}

	done := make(chan error, 1)
	go func() {
		_, err := m.cmd.Process.Wait()
		done <- err
	}()

	select {
	case <-done:
		log.Printf("[%s] Process stopped gracefully", m.app.Name)
	case <-time.After(5 * time.Second):

		log.Printf("[%s] Process didn't stop gracefully, force killing...", m.app.Name)
		if pgid, err := syscall.Getpgid(m.cmd.Process.Pid); err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			m.cmd.Process.Kill()
		}
		<-done
	}

	m.running = false
	m.cmd = nil
	return nil
}

func (m *Manager) streamOutput(pipe io.ReadCloser, streamType string) {
	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		if m.app.LogSilent {
			continue
		}
		line := scanner.Text()
		if streamType == "stderr" {
			log.Printf("[%s] %s", m.app.Name, line)
		} else {
			fmt.Printf("[%s] %s\n", m.app.Name, line)
		}
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		if !m.app.LogSilent {
			log.Printf("[%s] Error reading %s: %v", m.app.Name, streamType, err)
		}
	}
}

func (m *Manager) CleanUp() {
	if !m.app.CleanOnExit {
		return
	}

	for _, file := range m.tmpFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			log.Printf("[%s] Failed to remove temp file %s: %v", m.app.Name, file, err)
		} else if !m.app.LogSilent {
			log.Printf("[%s] Removed temp file: %s", m.app.Name, file)
		}
	}

	if m.app.TmpDir != "" && m.app.TmpDir != "." && m.app.TmpDir != "/" {
		os.RemoveAll(m.app.TmpDir)
	}
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.running
}

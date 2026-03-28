package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

const (
	maxFileSize = 10 * 1024 * 1024 // 10 MB
	maxBackups  = 3
	logFileName = "connector.log"
)

// Setup configures the global logger to write to both stdout and a log file
// in the given directory. It rotates the log file on startup if it exceeds
// maxFileSize. Returns a cleanup function to close the file.
func Setup(stateDir string) (func(), error) {
	logDir := filepath.Join(stateDir, "logs")
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		return nil, fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(logDir, logFileName)

	// Rotate if the existing file is too large.
	rotateIfNeeded(logDir, logPath)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) //nolint:gosec // logPath is derived from stateDir, not user input
	if err != nil {
		return nil, fmt.Errorf("opening log file: %w", err)
	}

	multi := io.MultiWriter(os.Stdout, f)
	log.SetOutput(multi)
	log.SetFlags(log.LstdFlags)

	return func() { _ = f.Close() }, nil
}

func rotateIfNeeded(logDir, logPath string) {
	info, err := os.Stat(logPath)
	if err != nil || info.Size() < maxFileSize {
		return
	}
	// Shift old backups: connector.log.3 <- .2 <- .1 <- connector.log
	for i := maxBackups; i > 0; i-- {
		old := filepath.Join(logDir, fmt.Sprintf("%s.%d", logFileName, i))
		newer := filepath.Join(logDir, fmt.Sprintf("%s.%d", logFileName, i-1))
		if i == 1 {
			newer = logPath
		}
		_ = os.Remove(old)
		_ = os.Rename(newer, old)
	}
}

// ReadLastLines reads the last n lines from the current log file.
func ReadLastLines(stateDir string, n int) (string, error) {
	logPath := filepath.Join(stateDir, "logs", logFileName)
	data, err := os.ReadFile(logPath) //nolint:gosec // logPath is derived from stateDir constant, not user input
	if err != nil {
		if os.IsNotExist(err) {
			return "No logs yet.", nil
		}
		return "", err
	}

	lines := strings.Split(string(data), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n"), nil
}

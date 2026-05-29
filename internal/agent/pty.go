package agent

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

type PTYBackend struct {
	pty       *os.File
	cmd       *exec.Cmd
	output    bytes.Buffer
	logDir    string
	logFile   *os.File
	closeOnce sync.Once
	done      chan struct{}
	mu        sync.Mutex
	oldState  *term.State
}

func NewPTYBackend(logDir string) *PTYBackend {
	return &PTYBackend{
		logDir: logDir,
		done:   make(chan struct{}),
	}
}

func (b *PTYBackend) Start(cmd *exec.Cmd) error {
	rows, cols, err := term.GetSize(int(os.Stdin.Fd()))
	if err != nil {
		rows, cols = 24, 80
	}
	ws := &pty.Winsize{Rows: uint16(rows), Cols: uint16(cols)}

	f, err := pty.StartWithSize(cmd, ws)
	if err != nil {
		return fmt.Errorf("pty start: %w", err)
	}
	b.pty = f
	b.cmd = cmd

	if term.IsTerminal(int(os.Stdin.Fd())) {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			f.Close()
			return fmt.Errorf("raw terminal: %w", err)
		}
		b.oldState = oldState
	}

	if b.logDir != "" {
		logPath := filepath.Join(b.logDir, fmt.Sprintf("pty_%d.log", time.Now().UnixMilli()))
		logFH, err := os.Create(logPath)
		if err == nil {
			b.logFile = logFH
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		var writers []io.Writer
		writers = append(writers, os.Stdout)
		writers = append(writers, &b.output)
		if b.logFile != nil {
			writers = append(writers, b.logFile)
		}
		multi := io.MultiWriter(writers...)
		io.Copy(multi, f)
		b.closeOnce.Do(func() {
			close(b.done)
		})
	}()

	go func() {
		defer wg.Done()
		io.Copy(f, os.Stdin)
	}()

	go func() {
		wg.Wait()
		b.closeOnce.Do(func() {
			close(b.done)
		})
	}()

	return nil
}

func (b *PTYBackend) Wait() error {
	err := b.cmd.Wait()
	<-b.done
	return err
}

func (b *PTYBackend) Signal(sig os.Signal) error {
	if b.pty != nil {
		return b.pty.Close()
	}
	return nil
}

func (b *PTYBackend) CapturedOutput() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.output.String()
}

func (b *PTYBackend) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), b.oldState)
		b.oldState = nil
	}

	if b.pty != nil {
		b.pty.Close()
	}
	if b.logFile != nil {
		b.logFile.Close()
	}
	return nil
}

func (b *PTYBackend) Stdin() *os.File {
	return b.pty
}

func (b *PTYBackend) Resize(rows, cols uint16) error {
	if b.pty == nil {
		return nil
	}
	return pty.Setsize(b.pty, &pty.Winsize{Rows: rows, Cols: cols})
}

func init() {
	_ = syscall.SIGWINCH
}

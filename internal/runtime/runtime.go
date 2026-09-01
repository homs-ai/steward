package runtime

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ProbeResult struct {
	Probe     string        `json:"probe"`
	Data      interface{}   `json:"data"`
	Error     string        `json:"error,omitempty"`
	Duration  time.Duration `json:"duration"`
	Available bool          `json:"available"`
}

type ProcessInfo struct {
	PID    int    `json:"pid"`
	Name   string `json:"name"`
	User   string `json:"user"`
	Cmd    string `json:"cmd"`
	Status string `json:"status"`
}

type NetworkListener struct {
	Protocol string `json:"protocol"`
	Address  string `json:"address"`
	PID      int    `json:"pid,omitempty"`
}

type ContainerInfo struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Image string `json:"image"`
	State string `json:"state"`
}

type SystemdUnit struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Active string `json:"active"`
}

type ProbeResults struct {
	Processes   []ProcessInfo     `json:"processes,omitempty"`
	Listeners   []NetworkListener `json:"listeners,omitempty"`
	Containers  []ContainerInfo   `json:"containers,omitempty"`
	SystemdUnits []SystemdUnit    `json:"systemd_units,omitempty"`
}

type Probe struct {
	Name       string
	Timeout    time.Duration
	Capability func() bool
	Run        func(ctx context.Context) (interface{}, error)
}

func NewProbes() []Probe {
	return []Probe{
		{
			Name:       "process_list",
			Timeout:    10 * time.Second,
			Capability: func() bool { return true },
			Run:        probeProcessList,
		},
		{
			Name:       "network_listeners",
			Timeout:    10 * time.Second,
			Capability: func() bool { return true },
			Run:        probeNetworkListeners,
		},
		{
			Name:    "docker_containers",
			Timeout: 5 * time.Second,
			Capability: func() bool {
				path, err := exec.LookPath("docker")
				if err != nil {
					return false
				}
				info, err := os.Stat(path)
				return err == nil && info.Mode().Perm()&0100 != 0
			},
			Run: probeDockerContainers,
		},
		{
			Name:    "systemd_units",
			Timeout: 5 * time.Second,
			Capability: func() bool {
				path, err := exec.LookPath("systemctl")
				if err != nil {
					return false
				}
				info, err := os.Stat(path)
				if err != nil {
					return false
				}
				if info.Mode().Perm()&0100 == 0 {
					return false
				}
				_, err = os.Stat("/run/systemd/system")
				return err == nil
			},
			Run: probeSystemdUnits,
		},
	}
}

func RunProbes(ctx context.Context) ProbeResults {
	probes := NewProbes()
	var results ProbeResults

	for _, probe := range probes {
		if !probe.Capability() {
			continue
		}

		probeCtx, cancel := context.WithTimeout(ctx, probe.Timeout)
		start := time.Now()
		data, err := probe.Run(probeCtx)
		_ = time.Since(start)
		cancel()

		if err != nil {
			continue
		}

		switch d := data.(type) {
		case []ProcessInfo:
			results.Processes = append(results.Processes, d...)
		case []NetworkListener:
			results.Listeners = append(results.Listeners, d...)
		case []ContainerInfo:
			results.Containers = append(results.Containers, d...)
		case []SystemdUnit:
			results.SystemdUnits = append(results.SystemdUnits, d...)
		}
	}

	return results
}

func probeProcessList(ctx context.Context) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "ps", "aux")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ps aux failed: %w", err)
	}

	var processes []ProcessInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		processes = append(processes, ProcessInfo{
			User: fields[0],
			Cmd:  strings.Join(fields[10:], " "),
		})
	}
	return processes, nil
}

func probeNetworkListeners(ctx context.Context) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "ss", "-tlnp")
	out, err := cmd.Output()
	if err != nil {
		cmd = exec.CommandContext(ctx, "netstat", "-tlnp")
		out, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("ss/netstat failed: %w", err)
		}
	}

	var listeners []NetworkListener
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for i, line := range lines {
		if i == 0 {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		listeners = append(listeners, NetworkListener{
			Protocol: fields[0],
			Address:  fields[3],
		})
	}

	if len(listeners) == 0 {
		return nil, fmt.Errorf("no listeners found")
	}
	return listeners, nil
}

func probeDockerContainers(ctx context.Context) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--format", "{{.ID}}\t{{.Names}}\t{{.Image}}\t{{.Status}}")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps failed: %w", err)
	}

	var containers []ContainerInfo
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		fields := strings.SplitN(line, "\t", 4)
		if len(fields) < 4 {
			continue
		}
		containers = append(containers, ContainerInfo{
			ID:    fields[0],
			Name:  fields[1],
			Image: fields[2],
			State: fields[3],
		})
	}
	return containers, nil
}

func probeSystemdUnits(ctx context.Context) (interface{}, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "list-units", "--type=service", "--no-pager", "--plain", "--no-legend")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("systemctl failed: %w", err)
	}

	var units []SystemdUnit
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		units = append(units, SystemdUnit{
			Name:   strings.TrimSuffix(fields[0], ".service"),
			State:  fields[1],
			Active: fields[2],
		})
	}
	return units, nil
}

func EnsureDir(path string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create dir %s: %w", path, err)
	}
	return nil
}

func ExecutableExists(name string) bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().Perm()&0100 != 0
}

func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func DirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func ReadFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func WriteFile(path string, content string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(content), 0644)
}

func Glob(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

func Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}

func ListenTCP(addr string) (net.Listener, error) {
	return net.Listen("tcp", addr)
}

func ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

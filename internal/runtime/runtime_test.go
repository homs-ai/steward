package runtime

import (
	"context"
	"testing"
	"time"
)

func TestRunProbesReturnsResults(t *testing.T) {
	ctx := context.Background()
	results := RunProbes(ctx)
	if results.Processes == nil && results.Listeners == nil {
		t.Log("probes returned no data (expected in some environments)")
	}
}

func TestProbeProcessList(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := probeProcessList(ctx)
	if err != nil {
		t.Fatalf("probeProcessList failed: %v", err)
	}
	procs, ok := data.([]ProcessInfo)
	if !ok || len(procs) == 0 {
		t.Error("expected at least one process")
	}
}

func TestProbeNetworkListeners(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := probeNetworkListeners(ctx)
	if err != nil {
		t.Skipf("network listeners probe skipped: %v", err)
	}
	listeners, ok := data.([]NetworkListener)
	if !ok {
		t.Error("expected []NetworkListener type")
	}
	t.Logf("found %d listeners", len(listeners))
}

func TestProbeDockerContainers(t *testing.T) {
	probes := NewProbes()
	var dockerProbe *Probe
	for i := range probes {
		if probes[i].Name == "docker_containers" {
			dockerProbe = &probes[i]
			break
		}
	}
	if dockerProbe == nil {
		t.Fatal("docker_containers probe not found")
	}

	if !dockerProbe.Capability() {
		t.Skip("docker not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := dockerProbe.Run(ctx)
	if err != nil {
		t.Skipf("docker probe failed: %v", err)
	}
	containers, ok := data.([]ContainerInfo)
	if !ok {
		t.Error("expected []ContainerInfo type")
	}
	t.Logf("found %d containers", len(containers))
}

func TestProbeSystemdUnits(t *testing.T) {
	probes := NewProbes()
	var systemdProbe *Probe
	for i := range probes {
		if probes[i].Name == "systemd_units" {
			systemdProbe = &probes[i]
			break
		}
	}
	if systemdProbe == nil {
		t.Fatal("systemd_units probe not found")
	}

	if !systemdProbe.Capability() {
		t.Skip("systemd not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := systemdProbe.Run(ctx)
	if err != nil {
		t.Skipf("systemd probe failed: %v", err)
	}
	units, ok := data.([]SystemdUnit)
	if !ok {
		t.Error("expected []SystemdUnit type")
	}
	t.Logf("found %d units", len(units))
}

func TestExecutableExists(t *testing.T) {
	if !ExecutableExists("ls") {
		t.Error("expected ls to exist")
	}
	if ExecutableExists("nonexistent-binary-xyz789") {
		t.Error("expected nonexistent binary to not exist")
	}
}

func TestFileExists(t *testing.T) {
	if FileExists("/nonexistent/path") {
		t.Error("expected false for nonexistent file")
	}
}

func TestDirExists(t *testing.T) {
	if !DirExists("/tmp") {
		t.Error("expected /tmp to exist")
	}
	if DirExists("/nonexistent/path") {
		t.Error("expected false for nonexistent dir")
	}
}

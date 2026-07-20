package agent

import (
	"fmt"
	"os"
	"sync"

	"github.com/k/steward/internal/config"
)

// forceManualEnv is the environment kill-switch owned by steward. When set to a
// truthy value it forces manual permission mode regardless of the --manual flag,
// and cannot be delegated to or overridden by any agent.
const forceManualEnv = "STEWARD_FORCE_MANUAL"

// PermissionMode names the effective permission posture for a run. It is
// recorded in telemetry and surfaced in the startup banner.
type PermissionMode string

const (
	// PermissionAuto emits the agent's skip/auto-approve flag so the agent runs
	// unattended (dangerously skips permission prompts).
	PermissionAuto PermissionMode = "auto"
	// PermissionManual emits no permission flag so the agent prompts as usual.
	PermissionManual PermissionMode = "manual"
	// PermissionForceManual is manual mode forced by STEWARD_FORCE_MANUAL.
	PermissionForceManual PermissionMode = "force-manual"
)

// forceManual reports whether the STEWARD_FORCE_MANUAL kill-switch is engaged.
func forceManual() bool {
	switch os.Getenv(forceManualEnv) {
	case "", "0", "false", "FALSE", "False":
		return false
	default:
		return true
	}
}

// EffectiveMode resolves the permission mode from the requested manual flag and
// the STEWARD_FORCE_MANUAL kill-switch. The kill-switch always wins.
func EffectiveMode(manual bool) PermissionMode {
	if forceManual() {
		return PermissionForceManual
	}
	if manual {
		return PermissionManual
	}
	return PermissionAuto
}

// PermissionArgs returns the CLI args that set the agent's permission mode.
// manual=false → emit the agent's skip/auto-approve flag; manual=true (or the
// STEWARD_FORCE_MANUAL kill-switch) → emit nothing. An empty SkipPermsFlag is
// treated as "no flag" so misconfigured agents fail closed (manual).
func PermissionArgs(agentCfg *config.AgentConfig, manual bool) []string {
	if EffectiveMode(manual) != PermissionAuto {
		return nil
	}
	if agentCfg == nil || agentCfg.SkipPermsFlag == "" {
		return nil
	}
	return []string{agentCfg.SkipPermsFlag}
}

// bannerOnce guards the one-shot per-session permission banner.
var bannerOnce sync.Once

// PrintPermissionBanner prints a one-line banner naming the active permission
// mode. It fires at most once per process so repeated phase runs don't spam it.
func PrintPermissionBanner(manual bool) {
	bannerOnce.Do(func() {
		mode := EffectiveMode(manual)
		switch mode {
		case PermissionAuto:
			fmt.Fprintln(os.Stderr, "[steward] permission mode: auto — agents may run destructive commands unattended (use --manual to require approval).")
		case PermissionManual:
			fmt.Fprintln(os.Stderr, "[steward] permission mode: manual — the agent will prompt before privileged actions.")
		case PermissionForceManual:
			fmt.Fprintf(os.Stderr, "[steward] permission mode: force-manual — %s is set; manual approval is enforced regardless of --manual.\n", forceManualEnv)
		}
	})
}

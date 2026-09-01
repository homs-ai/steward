package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Report struct {
	Feature    string        `json:"feature"`
	Timestamp  time.Time     `json:"timestamp"`
	Summary    string        `json:"summary"`
	Topology   string        `json:"topology"`
	Analysis   string        `json:"analysis"`
	RawData    *RawData      `json:"raw_data,omitempty"`
	Structured *StructuredOutput `json:"structured,omitempty"`
}

type RawData struct {
	DiscoveryResults string   `json:"discovery_results,omitempty"`
	RuntimeProbes    string   `json:"runtime_probes,omitempty"`
	DetectionResults []string `json:"detection_results,omitempty"`
}

type StructuredOutput struct {
	BuildPatterns []BuildPatternInfo `json:"build_patterns"`
	Dependencies  []DependencyInfo   `json:"dependencies,omitempty"`
	IPC           []IPCInfo          `json:"ipc,omitempty"`
	Platform      []PlatformInfo     `json:"platform,omitempty"`
}

type BuildPatternInfo struct {
	Name string `json:"name"`
	File string `json:"file"`
}

type DependencyInfo struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	Version     string `json:"version,omitempty"`
	Direct      bool   `json:"direct"`
}

type IPCInfo struct {
	Type    string `json:"type"`
	Address string `json:"address,omitempty"`
	Command string `json:"command,omitempty"`
}

type PlatformInfo struct {
	Type    string `json:"type"`
	Details string `json:"details"`
}

func WriteReport(featDir string, report *Report) error {
	md := renderMarkdown(report)
	mdPath := filepath.Join(featDir, "research.md")
	if err := os.WriteFile(mdPath, []byte(md), 0644); err != nil {
		return fmt.Errorf("write research.md: %w", err)
	}

	jsonData, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	jsonPath := filepath.Join(featDir, "research.json")
	if err := os.WriteFile(jsonPath, jsonData, 0644); err != nil {
		return fmt.Errorf("write research.json: %w", err)
	}

	return nil
}

func renderMarkdown(r *Report) string {
	var md string

	md += fmt.Sprintf("# Research: %s\n\n", r.Feature)
	md += fmt.Sprintf("*Generated: %s*\n\n", r.Timestamp.Format(time.RFC3339))

	if r.Summary != "" {
		md += "## L1: Executive Summary\n\n"
		md += r.Summary + "\n\n"
	}

	if r.Topology != "" {
		md += "## L2: Topology Diagram\n\n"
		md += "```mermaid\n" + r.Topology + "\n```\n\n"
	}

	if r.Analysis != "" {
		md += "## L3: Full Analysis\n\n"
		md += r.Analysis + "\n\n"
	}

	if r.RawData != nil {
		md += "## L4: Raw Data Appendix\n\n"
		if r.RawData.DiscoveryResults != "" {
			md += "### Discovery Commands\n\n```\n" + r.RawData.DiscoveryResults + "```\n\n"
		}
		if r.RawData.RuntimeProbes != "" {
			md += "### Runtime Probes\n\n```\n" + r.RawData.RuntimeProbes + "```\n\n"
		}
		if len(r.RawData.DetectionResults) > 0 {
			md += "### Detection Results\n\n"
			for _, dr := range r.RawData.DetectionResults {
				md += "- " + dr + "\n"
			}
			md += "\n"
		}
	}

	return md
}

func LoadStructuredOutput(featDir string) (*StructuredOutput, error) {
	jsonPath := filepath.Join(featDir, "research.json")
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return report.Structured, nil
}

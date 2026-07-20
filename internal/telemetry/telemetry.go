package telemetry

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
)

type PhaseTelemetry struct {
	StartedAt      string `yaml:"started_at,omitempty"`
	CompletedAt    string `yaml:"completed_at,omitempty"`
	Agent          string `yaml:"agent"`
	Model          string `yaml:"model,omitempty"`
	PermissionMode string `yaml:"permission_mode,omitempty"`
	TokensIn       int    `yaml:"tokens_in"`
	TokensOut      int    `yaml:"tokens_out"`
	DurationSec    int    `yaml:"duration_sec"`
	Iterations     int    `yaml:"iterations"`
	ExitCode       int    `yaml:"exit_code"`
	Retries        int    `yaml:"retries"`
	HumanRating    int    `yaml:"human_rating,omitempty"`
	Error          string `yaml:"error,omitempty"`
}

type FeatureTelemetry struct {
	Feature string                    `yaml:"feature"`
	Phases  map[string]*PhaseTelemetry `yaml:"phases"`
}

type AgentSummary struct {
	Name         string
	Features     int
	AvgTokensIn  float64
	AvgTokensOut float64
	AvgTimeSec   float64
	AvgIter      float64
	FailRate     float64
	AvgCost      float64
	AvgRating    float64
}

func Load(feat *feature.Feature) (*FeatureTelemetry, error) {
	t := &FeatureTelemetry{
		Feature: feat.Name,
		Phases:  make(map[string]*PhaseTelemetry),
	}

	data, err := os.ReadFile(feat.TelemetryFile())
	if err != nil {
		if os.IsNotExist(err) {
			return t, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, t); err != nil {
		return nil, err
	}
	if t.Phases == nil {
		t.Phases = make(map[string]*PhaseTelemetry)
	}
	return t, nil
}

func Save(feat *feature.Feature, ft *FeatureTelemetry) error {
	data, err := yaml.Marshal(ft)
	if err != nil {
		return err
	}
	return os.WriteFile(feat.TelemetryFile(), data, 0644)
}

func RecordPhaseStart(feat *feature.Feature, phase, agent, permissionMode string) error {
	ft, err := Load(feat)
	if err != nil {
		return err
	}

	if ft.Phases[phase] == nil {
		ft.Phases[phase] = &PhaseTelemetry{}
	}
	ft.Phases[phase].Agent = agent
	ft.Phases[phase].PermissionMode = permissionMode
	ft.Phases[phase].StartedAt = time.Now().UTC().Format(time.RFC3339)

	return Save(feat, ft)
}

func RecordPhaseEnd(feat *feature.Feature, phase string, tokensIn, tokensOut int, exitCode, retries int, errStr string) error {
	ft, err := Load(feat)
	if err != nil {
		return err
	}

	if ft.Phases[phase] == nil {
		return fmt.Errorf("no telemetry for phase %s (start not recorded)", phase)
	}

	p := ft.Phases[phase]
	startTime, _ := time.Parse(time.RFC3339, p.StartedAt)
	p.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	p.TokensIn = tokensIn
	p.TokensOut = tokensOut
	p.DurationSec = int(time.Since(startTime).Seconds())
	p.ExitCode = exitCode
	p.Retries = retries
	p.Error = errStr

	if errStr == "" {
		p.Iterations++
	}

	return Save(feat, ft)
}

func RecordRating(feat *feature.Feature, phase string, rating int) error {
	ft, err := Load(feat)
	if err != nil {
		return err
	}
	if ft.Phases[phase] == nil {
		return fmt.Errorf("no telemetry for phase %s", phase)
	}
	ft.Phases[phase].HumanRating = rating
	return Save(feat, ft)
}

func EstimateCost(cfg *config.Config, agentName string, tokensIn, tokensOut int) float64 {
	for name, agent := range cfg.Agents {
		if name == agentName {
			costIn := (float64(tokensIn) / 1000.0) * agent.CostPer1KIn
			costOut := (float64(tokensOut) / 1000.0) * agent.CostPer1KOut
			return costIn + costOut
		}
	}
	return 0
}

func AggregateAgents(cfg *config.Config) ([]AgentSummary, error) {
	features, err := feature.ListFeatures(cfg, "")
	if err != nil {
		return nil, err
	}

	type agentData struct {
		count     int
		totalIn   int
		totalOut  int
		totalTime int
		totalIter int
		totalFail int
		totalCost float64
		totalRate int
		rateCount int
	}

	agg := make(map[string]*agentData)

	for _, f := range features {
		ft, err := Load(&f)
		if err != nil {
			continue
		}
		for _, p := range ft.Phases {
			aName := p.Agent
			if aName == "" {
				aName = cfg.DefaultAgent
			}
			if agg[aName] == nil {
				agg[aName] = &agentData{}
			}
			d := agg[aName]
			d.count++
			d.totalIn += p.TokensIn
			d.totalOut += p.TokensOut
			d.totalTime += p.DurationSec
			d.totalIter += p.Iterations
			if p.ExitCode != 0 || p.Error != "" {
				d.totalFail++
			}
			d.totalCost += EstimateCost(cfg, aName, p.TokensIn, p.TokensOut)
			if p.HumanRating > 0 {
				d.totalRate += p.HumanRating
				d.rateCount++
			}
		}
	}

	var summaries []AgentSummary
	for name, d := range agg {
		s := AgentSummary{
			Name:     name,
			Features: len(features),
		}
		if d.count > 0 {
			s.AvgTokensIn = float64(d.totalIn) / float64(d.count)
			s.AvgTokensOut = float64(d.totalOut) / float64(d.count)
			s.AvgTimeSec = float64(d.totalTime) / float64(d.count)
			s.AvgIter = float64(d.totalIter) / float64(d.count)
			s.FailRate = (float64(d.totalFail) / float64(d.count)) * 100
			s.AvgCost = d.totalCost / float64(d.count)
		}
		if d.rateCount > 0 {
			s.AvgRating = float64(d.totalRate) / float64(d.rateCount)
		}
		summaries = append(summaries, s)
	}

	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].Name < summaries[j].Name
	})

	return summaries, nil
}

func FormatDuration(sec int) string {
	if sec < 60 {
		return fmt.Sprintf("%ds", sec)
	}
	if sec < 3600 {
		return fmt.Sprintf("%dm", sec/60)
	}
	return fmt.Sprintf("%dh%dm", sec/3600, (sec%3600)/60)
}

func FormatCost(cost float64) string {
	return fmt.Sprintf("$%.2f", cost)
}

func FormatTokens(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%.1fk", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(n)/1000000)
}

func PrintMetrics(feat *feature.Feature, ft *FeatureTelemetry, cfg *config.Config) {
	fmt.Printf("Feature: %s\n", feat.DisplayName())
	fmt.Println(strings.Repeat("─", 100))
	fmt.Printf("%-15s %-12s %-12s %-12s %-8s %-10s %-8s %-8s %s\n",
		"Phase", "Agent", "Tokens In", "Tokens Out", "Cost", "Iter", "Time", "Rating", "Status")

	var totalIn, totalOut int
	var totalCost float64
	var totalTime int

	phases := []string{"brainstorm", "research", "analysis", "implement", "test"}
	for _, phase := range phases {
		p := ft.Phases[phase]
		if p == nil {
			fmt.Printf("%-15s %-12s %-12s %-12s %-8s %-10s %-8s %-8s %s\n",
				phase, "-", "-", "-", "-", "-", "-", "-", "not started")
			continue
		}
		agent := p.Agent
		cost := EstimateCost(cfg, agent, p.TokensIn, p.TokensOut)
		totalIn += p.TokensIn
		totalOut += p.TokensOut
		totalCost += cost
		totalTime += p.DurationSec

		rating := "-"
		if p.HumanRating > 0 {
			rating = fmt.Sprintf("%d/5", p.HumanRating)
		}
		status := "ok"
		if p.Error != "" {
			status = "error"
		}

		fmt.Printf("%-15s %-12s %-12s %-12s %-8s %-10s %-8s %-8s %s\n",
			phase, agent,
			FormatTokens(p.TokensIn), FormatTokens(p.TokensOut),
			FormatCost(cost),
			fmt.Sprintf("%d", p.Iterations),
			FormatDuration(p.DurationSec),
			rating, status)
	}

	fmt.Println(strings.Repeat("─", 100))
	fmt.Printf("%-15s %-12s %-12s %-12s %-8s %-10s %-8s\n",
		"TOTAL", "",
		FormatTokens(totalIn), FormatTokens(totalOut),
		FormatCost(totalCost),
		"", FormatDuration(totalTime))
}

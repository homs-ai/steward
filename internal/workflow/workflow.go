package workflow

import (
	"context"
	"fmt"
	"os"

	"github.com/k/steward/internal/agent"
	"github.com/k/steward/internal/config"
	"github.com/k/steward/internal/feature"
	"github.com/k/steward/internal/prompts"
	"github.com/k/steward/internal/telemetry"
)

type PhaseRunner struct {
	Config      *config.Config
	Runner      *agent.Runner
	ProjectRoot string
	Interactive bool
	// Manual, when true, opts back into agent permission prompting instead of
	// the default auto (skip-permissions) behavior.
	Manual bool
}

func NewPhaseRunner(cfg *config.Config) *PhaseRunner {
	return &PhaseRunner{
		Config:      cfg,
		Runner:      agent.NewRunner(cfg),
		Interactive: true,
	}
}

func BuildPhasePrompt(cfg *config.Config, feat *feature.Feature, phase string, contextFiles []string) string {
	context := ""
	for _, f := range contextFiles {
		content, _, err := feat.ReadAfterStale(f)
		if err != nil {
			continue
		}
		context += fmt.Sprintf("\n=== %s ===\n%s\n", f, content)
	}

	return fmt.Sprintf(`You are helping build a software feature called '%s'.

%s

Only read content after the last occurrence of <<<STALE>>> in each file above.
Write this after the last occurrence of <<<STALE>>> in the relevant file. Do not write to any other files.
Format it clearly in Markdown.`, feat.DisplayName(), context)
}

func (pr *PhaseRunner) buildPhasePrompt(feat *feature.Feature, phase string, extraContext string) string {
	context := fmt.Sprintf("You are working on a software feature called '%s'.", feat.DisplayName())
	if extraContext != "" {
		context += "\n\n" + extraContext
	}
	return context
}

func (pr *PhaseRunner) runPhase(ctx context.Context, feat *feature.Feature, phase, batchPrompt string) error {
	if !pr.Interactive {
		pr.Runner.Manual = pr.Manual
		_, err := pr.Runner.Run(ctx, feat, phase, batchPrompt, pr.ProjectRoot)
		return err
	}

	promptText := batchPrompt
	if promptText == "" {
		var loadErr error
		promptText, loadErr = prompts.PromptForPhase(pr.Config.StewardHome, phase)
		if loadErr != nil {
			return fmt.Errorf("load prompt for %s: %w", phase, loadErr)
		}
	}

	interactivePrompt := pr.buildPhasePrompt(feat, phase, promptText)

	interactiveRunner := agent.NewInteractiveRunner(pr.Config)
	interactiveRunner.ProjectRoot = pr.ProjectRoot
	interactiveRunner.Manual = pr.Manual

	_, err := interactiveRunner.RunInteractive(ctx, feat, phase, interactivePrompt, agent.InteractiveOptions{})
	return err
}

func (pr *PhaseRunner) Brainstorm(ctx context.Context, feat *feature.Feature, input string) error {
	prompt := fmt.Sprintf(`You are helping brainstorm for a software feature called '%s'.

The user's initial description or problem statement is:
%s

This is a divergent thinking phase. Your goal is raw idea generation, problem identification, and exploring potential solutions without judgment.

Generate a structured brainstorm document covering:
- Problem statement and goals
- Potential solutions and approaches (explore multiple directions, including unconventional ones)
- Key assumptions and constraints
- Risks and challenges
- Wild or innovative ideas worth exploring

Write this after the last occurrence of <<<STALE>>> in %s/brainstorm.md. Format it clearly in Markdown.`,
		feat.DisplayName(), input, feat.Dir)

	return pr.runPhase(ctx, feat, "brainstorm", prompt)
}

func (pr *PhaseRunner) Research(ctx context.Context, feat *feature.Feature) error {
	brainstormContent, _, _ := feat.ReadAfterStale("brainstorm.md")

	prompt := fmt.Sprintf(`You are performing a research phase for a software feature called '%s'.

BRAINSTORM OUTPUT:
%s

Only read content after the last occurrence of <<<STALE>>> in the brainstorm content above.

This is the grounding phase. Transform the brainstormed ideas into facts by researching feasibility, existing tools, APIs, and alternatives.

Produce a structured research document covering:
- Feasibility assessment for each key idea from the brainstorm
- Existing tools, libraries, or frameworks relevant to this feature (with links where available)
- Relevant API availability and documentation pointers
- Competitor or prior art analysis
- Proof-of-concept notes or code snippets where they aid clarity
- Key assumptions from the brainstorm that are validated or invalidated
- Open questions that remain after research, and any new directions they suggest

Write this after the last occurrence of <<<STALE>>> in %s/research.md. Format it clearly in Markdown.`,
		feat.DisplayName(), brainstormContent, feat.Dir)

	return pr.runPhase(ctx, feat, "research", prompt)
}

func (pr *PhaseRunner) Analysis(ctx context.Context, feat *feature.Feature) error {
	brainstormContent, _, _ := feat.ReadAfterStale("brainstorm.md")
	researchContent, _, _ := feat.ReadAfterStale("research.md")

	prompt := fmt.Sprintf(`You are performing an analysis phase for a software feature called '%s'.

BRAINSTORM OUTPUT:
%s

RESEARCH OUTPUT:
%s

Only read content after the last occurrence of <<<STALE>>> in each file above.

This is the convergent thinking phase. Distill ideas into a structured, actionable implementation plan.

Produce a structured analysis document covering:
- Chosen solution: select the best approach and justify the choice against the alternatives
- Scope definition: what is explicitly in scope and out of scope
- Architecture overview: high-level design decisions, components, and their interactions
- Constraints and non-negotiables: technical, product, or operational constraints that shape the design
- Implementation blocks: break the chosen solution into discrete, independently implementable units
- Open risks: any remaining unknowns or assumptions that could affect the plan

Write this after the last occurrence of <<<STALE>>> in %s/analysis.md. Format it clearly in Markdown.`,
		feat.DisplayName(), brainstormContent, researchContent, feat.Dir)

	return pr.runPhase(ctx, feat, "analysis", prompt)
}

func (pr *PhaseRunner) Implement(ctx context.Context, feat *feature.Feature) error {
	reqContent, _ := feat.ReadFile("req.md")
	analysisContent, _, _ := feat.ReadAfterStale("analysis.md")

	prompt := fmt.Sprintf(`You are implementing a software feature called '%s'.

REQUIREMENTS:
%s

ANALYSIS:
%s

Only read content after the last occurrence of <<<STALE>>> in the analysis content above.

Implement the feature according to the requirements and analysis. Write clean, well-structured code following best practices.

After implementation, capture the current changes:
1. Run: git diff HEAD > %s/diff.md
2. Write a brief summary of what was implemented to %s/report.md

Write this after the last occurrence of <<<STALE>>> in %s/report.md.`,
		feat.DisplayName(), reqContent, analysisContent, feat.Dir, feat.Dir, feat.Dir)

	return pr.runPhase(ctx, feat, "implement", prompt)
}

func (pr *PhaseRunner) Test(ctx context.Context, feat *feature.Feature) error {
	reqContent, _ := feat.ReadFile("req.md")
	reviewContent, _ := feat.ReadFile("review.md")
	reportContent, _, _ := feat.ReadAfterStale("report.md")

	prompt := fmt.Sprintf(`You are testing a software feature called '%s'.

REQUIREMENTS:
%s

HUMAN REVIEW:
%s

TECHNICAL REPORT:
%s

Only read content after the last occurrence of <<<STALE>>> in the report content above.

Generate a comprehensive test scope covering:
- Positive and negative scenarios
- Edge cases
- Boundary conditions

Write the test scope after the last occurrence of <<<STALE>>> in %s/test_scope.md.

Then perform the testing as described and write a detailed test report to %s/test_report.md.

Write this after the last occurrence of <<<STALE>>> in the relevant files.`,
		feat.DisplayName(), reqContent, reviewContent, reportContent, feat.Dir, feat.Dir)

	return pr.runPhase(ctx, feat, "test", prompt)
}

func RequireRating(feat *feature.Feature, phase string) {
	fmt.Print("\nOptional: Rate this phase (1-5, or press Enter to skip): ")
	var input string
	fmt.Scanln(&input)

	if input != "" {
		rating := 0
		if _, err := fmt.Sscanf(input, "%d", &rating); err == nil && rating >= 1 && rating <= 5 {
			if err := telemetry.RecordRating(feat, phase, rating); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to save rating: %v\n", err)
			}
		}
	}
}

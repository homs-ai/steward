# Test Scope: steward:brainstorm-agent-improvements

## Feature Summary

The **Brainstorm** phase is the first execution step in the Steward feature lifecycle (`init → brainstorm → research → analysis → implement → test`). It performs divergent idea generation: given a user's feature description, it produces a structured brainstorm document covering problem statements, potential solutions, assumptions, risks, and innovative ideas.

The implementation spans four packages:
- `cmd/brainstorm.go` — CLI entry point
- `internal/workflow/workflow.go` — phase orchestration (`Brainstorm()` method)
- `internal/agent/` — batch (`Runner`) and interactive (`InteractiveRunner`) agent backends
- `internal/prompts/` — prompt template loading (`brainstorm.txt`)
- `internal/telemetry/` — phase timing and cost recording
- `internal/feature/` — feature artifact management (brainstorm.md)

## Test Levels

| Level | Scope | Approach |
|-------|-------|----------|
| **Unit** | Individual functions in workflow, agent, prompts, telemetry, feature | Table-driven tests with temp directories, stdin piping, config fixtures |
| **Integration** | Brainstorm batch flow end-to-end | Full `pr.Brainstorm()` call with `echo` agent, verified telemetry + artifact creation |
| **Edge Cases** | Boundary inputs, error paths | Empty input, long input, unknown phase, missing config, invalid ratings |

---

## 1. Workflow Package Tests (`internal/workflow/`)

### 1.1 Brainstorm Prompt Construction

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| WF-01 | Basic brainstorm prompt includes feature name and input | Positive | `input="Build a login system"`, feature `"test-feature"` | Prompt contains `test-feature`, divergent thinking instructions, user input |
| WF-02 | Brainstorm prompt instructs writing to brainstorm.md | Positive | Any valid input | Prompt references `brainstorm.md` and `<<<STALE>>>` |
| WF-03 | Brainstorm prompt includes divergent thinking instruction | Positive | Any valid input | Prompt contains "divergent thinking", "idea generation" |
| WF-04 | Empty input string | Edge | `input=""` | Prompt still generates correctly with empty input section |
| WF-05 | Very long input (10KB+) | Edge | 10,000-character input | Prompt builds without truncation or panic |

### 1.2 Phase Prompt Building (`buildPhasePrompt`)

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| WF-06 | buildPhasePrompt with extra context | Positive | Extra prompt template text | Output includes feature name + extra context |
| WF-07 | buildPhasePrompt without extra context | Positive | Empty extra context | Output contains just the feature name |

### 1.3 Batch Mode Execution

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| WF-08 | Brainstorm in batch mode succeeds | Integration | `echo` agent, sample input | No error, telemetry recorded with exit code 0 |
| WF-09 | Brainstorm in batch mode records telemetry | Integration | Same as WF-08 | Telemetry file exists with tokens_in, tokens_out, exit_code=0 |
| WF-10 | Batch mode with non-existent agent | Negative | Agent binary not in PATH | Error returned (command not found) |
| WF-11 | Batch mode with unknown phase | Negative | Phase not in config | Returns error about unknown phase |

### 1.4 Rating System

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| WF-12 | Skip rating (Enter) | Positive | `"\n"` | No error, no rating recorded |
| WF-13 | Valid rating 1 | Positive | `"1\n"` | Rating recorded as 1 |
| WF-14 | Valid rating 5 | Boundary | `"5\n"` | Rating recorded as 5 |
| WF-15 | Invalid rating 0 | Negative | `"0\n"` | Not recorded (out of range) |
| WF-16 | Invalid rating 6 | Negative | `"6\n"` | Not recorded (out of range) |
| WF-17 | Non-numeric input | Negative | `"abc\n"` | Not recorded |
| WF-18 | Negative rating | Negative | `"-1\n"` | Not recorded |

---

## 2. Agent Package Tests (`internal/agent/`)

### 2.1 Agent Argument Building (`buildAgentArgs`)

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| AG-01 | opencode agent args | Positive | agentName=`"opencode"` | Returns `["--prompt", promptText]` |
| AG-02 | claude agent args | Positive | agentName=`"claude"` | Returns `[]` (empty) |
| AG-03 | claude-code agent args | Positive | agentName=`"claude-code"` | Returns `[]` (empty) |
| AG-04 | aider agent args | Positive | agentName=`"aider"` | Returns `[]` (empty) |
| AG-05 | Default/unknown agent args | Edge | agentName=`"unknown-agent"` | Returns `[]` (empty) |

### 2.2 Token Estimation (`estimateTokens`)

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| AG-06 | Empty string | Edge | `""` | Returns 0 |
| AG-07 | Single word | Edge | `"hello"` | Returns > 0 |
| AG-08 | Single character | Edge | `"a"` | Returns > 0 |
| AG-09 | Whitespace only | Edge | `"   \n  \t  "` | Returns > 0 |
| AG-10 | Unicode/multi-byte | Edge | `"你好世界"` | Returns reasonable estimate |
| AG-11 | Very long text (100K chars) | Boundary | 100,000 character string | Returns without panic |

### 2.3 InteractiveRunner

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| AG-12 | NewInteractiveRunner construction | Positive | Default config | Returns non-nil runner with config set |
| AG-13 | RunInteractive with unknown phase | Negative | Phase not in config | Returns error about unknown phase |
| AG-14 | RunInteractive with unknown agent | Negative | Agent not configured | Returns error about unconfigured agent |
| AG-15 | Backend selection (default pty) | Positive | No backend configured | PTYBackend created |

### 2.4 Exit Confirmation (`confirmExit`)

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| AG-16 | Confirm with "y" | Positive | `"y\n"` | Returns true |
| AG-17 | Confirm with "Y" | Positive | `"Y\n"` | Returns true (case-insensitive) |
| AG-18 | Confirm with "n" | Negative | `"n\n"` | Returns false |
| AG-19 | Confirm with "N" | Negative | `"N\n"` | Returns false |
| AG-20 | Confirm default (Enter) | Edge | `"\n"` | Returns true (default yes) |
| AG-21 | Confirm with "no" | Negative | `"no\n"` | Returns false |
| AG-22 | Confirm with "YES" | Positive | `"YES\n"` | Returns true |

### 2.5 Session Capture

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| AG-23 | CaptureSession is non-nil | Positive | Feature, phase, output | Returns non-nil session |
| AG-24 | SaveArtifacts creates session file | Positive | Feature, phase="brainstorm", output | Creates `brainstorm_session.md` in feature dir |

---

## 3. Prompts Package Tests (`internal/prompts/`)

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| PR-01 | Load default brainstorm prompt | Positive | phase=`"brainstorm"` | Returns non-empty prompt with divergent thinking content |
| PR-02 | Load default research prompt | Positive | phase=`"research"` | Returns non-empty prompt |
| PR-03 | Load default analysis prompt | Positive | phase=`"analysis"` | Returns non-empty prompt |
| PR-04 | Load default implement prompt | Positive | phase=`"implement"` | Returns non-empty prompt |
| PR-05 | Load default test prompt | Positive | phase=`"test"` | Returns non-empty prompt |
| PR-06 | Unknown phase | Negative | phase=`"nonexistent"` | Returns error |
| PR-07 | Custom prompt override | Integration | Write custom `brainstorm.txt` | Custom content returned instead of default |
| PR-08 | PromptDir path construction | Positive | stewardHome=`"/tmp/test"` | Returns `"/tmp/test/prompts"` |
| PR-09 | PromptPath construction | Positive | stewardHome=`"/tmp/test"`, phase=`"brainstorm"` | Returns `"/tmp/test/prompts/brainstorm.txt"` |
| PR-10 | WriteDefaultPrompts creates files | Integration | Empty prompt dir | 5 prompt files created |

---

## 4. Feature Package Tests (`internal/feature/`)

Existing tests cover Init, Open, ReadWrite, stale marker, FindPhase, ListFeatures, EnsureExists, DisplayName (11 tests). Additional coverage:

| ID | Test | Type | Input | Expected |
|----|------|------|-------|----------|
| FT-01 | PhaseFile for brainstorm | Positive | `"brainstorm"` | Returns `"brainstorm.md"` |
| FT-02 | PhaseFile for all phases | Positive | All 5 phase names | Each returns correct filename |

---

## 5. Telemetry Package Tests (`internal/telemetry/`)

Existing tests cover load, record start/end, rating, cost, format functions, save, aggregate (9 tests). White-box review confirms adequate coverage for the brainstorm flow.

---

## 6. Regression Tests

| ID | Test | Type | Scope |
|----|------|------|-------|
| RG-01 | All existing tests pass | Regression | All 37 existing tests across 6 packages |

---

## Summary of Test Count

| Package | Existing | New | Total |
|---------|----------|-----|-------|
| `cmd/` | 7 | 0 | 7 |
| `internal/workflow/` | 4 | 12 | 16 |
| `internal/agent/` | 3 | 13 | 16 |
| `internal/prompts/` | 0 | 10 | 10 |
| `internal/config/` | 4 | 0 | 4 |
| `internal/feature/` | 11 | 2 | 13 |
| `internal/telemetry/` | 9 | 0 | 9 |
| **Total** | **37** | **37** | **74** |

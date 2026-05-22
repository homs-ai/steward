# Steward

Orchestrate software features with AI coding agents.

**You steer, agents execute.** Steward guides features through a structured lifecycle — from idea to tested code — while tracking cost, tokens, time, and human ratings per agent per phase. Use that data to optimize which agent handles which phase.

## Lifecycle

```
init → brainstorm → research → analysis → implement → test
```

| Phase | Description |
|---|---|
| `init` | Create a feature and its tracking files |
| `brainstorm` | Divergent idea generation and problem exploration |
| `research` | Validate ideas against reality — feasibility, tools, APIs |
| `analysis` | Converge on a structured implementation plan |
| `implement` | Coding agent builds the feature, captures diff |
| `test` | Generate test scope, execute, and report |

## Install

```bash
go install github.com/k/steward@latest
```

Or build from source:

```bash
git clone git@github.com:homs-ai/steward.git
cd steward
go build -o steward .
```

## Quick Start

```bash
# Initialize steward (interactive setup)
steward init

# Create a new feature
steward init my-feature

# Run through the lifecycle
steward brainstorm my-feature
steward research my-feature
steward analysis my-feature
steward implement my-feature
steward test my-feature
```

## Configuration

Set which agent handles each phase:

```bash
steward agent set implement aider
steward agent set brainstorm claude-code
```

View current config:

```bash
steward config
```

## Metrics & Optimization

```bash
# Per-feature metrics dashboard
steward metrics my-feature

# Cross-feature agent comparison
steward agents

# Execution timeline
steward log my-feature

# Feature status overview
steward status
steward status --all
```

Steward tracks tokens in/out, cost, duration, iterations, fail rate, and human ratings (1–5) per phase. Use `steward agents` to compare agent performance across features and decide which agent works best for each phase.

## Full Report

```bash
steward report my-feature
```

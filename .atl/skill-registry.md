# Skill Registry — gentax

Generated: 2026-04-06
Project: gentax (greenfield — stack TBD)

---

## Compact Rules

### branch-pr
- Always open a GitHub issue before creating a PR (issue-first enforcement)
- PR title: `<type>(<scope>): <short description>` (conventional commits)
- PR body must include: Summary, Test plan, linked issue

### issue-creation
- Create GitHub issue before starting any feature or bug fix
- Use labels: `bug`, `feature`, `chore`, `docs`
- Include: problem statement, acceptance criteria, affected area

### go-testing
- Use `testify/assert` for assertions; `testify/mock` for mocks
- Table-driven tests preferred for multiple cases
- For Bubbletea TUI: use `teatest` package
- Run: `go test ./...`

### judgment-day
- Trigger after completing a significant feature or architecture decision
- Two blind judge sub-agents review simultaneously
- Synthesize findings, fix critical issues, re-judge until both pass

---

## User Skills (trigger table)

| Skill | Trigger |
|-------|---------|
| `branch-pr` | Creating a pull request, opening a PR, preparing changes for review |
| `issue-creation` | Creating a GitHub issue, reporting a bug, requesting a feature |
| `go-testing` | Writing Go tests, using teatest, adding test coverage |
| `judgment-day` | Adversarial review of completed feature or architecture |
| `skill-creator` | Creating a new skill, adding agent instructions |
| `skill-registry` | Updating skills, after installing/removing skills |
| `sdd-explore` | Investigating an idea, comparing approaches |
| `sdd-propose` | Creating a change proposal |
| `sdd-spec` | Writing specifications |
| `sdd-design` | Writing technical design document |
| `sdd-tasks` | Breaking down a change into tasks |
| `sdd-apply` | Implementing tasks from a change |
| `sdd-verify` | Validating implementation against specs |
| `sdd-archive` | Closing and archiving a completed change |

---

## Project Conventions

- **Language**: TBD (greenfield project)
- **Domain**: Taxi expense management + Telegram bot + OCR invoice extraction
- **Conventions file**: none yet (CLAUDE.md not present at project level)

---

## Notes

- No project-level CLAUDE.md, agents.md, or cursorrules found
- Stack selection pending (see project context in engram: `sdd-init/gentax`)
- Strict TDD Mode: enabled (from global config) — activate test runner once stack is chosen

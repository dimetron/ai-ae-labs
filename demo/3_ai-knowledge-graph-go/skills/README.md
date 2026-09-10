# Skills

Project-specific skills for AI coding agents working on this Go codebase.
Each skill is a self-contained `SKILL.md` with rules, file references, and
patterns to follow.

## Available skills

| Skill | Use when |
|-------|----------|
| [`go-conventions`](./go-conventions/SKILL.md) | Editing or creating any `.go` file. |
| [`project-architecture`](./project-architecture/SKILL.md) | Onboarding, adding stages, tracing data flow. |
| [`adk-workflow`](./adk-workflow/SKILL.md) | Adding/removing/reordering pipeline stages. |
| [`llm-extraction`](./llm-extraction/SKILL.md) | Changing LLM prompts, parsing, or adding a new LLM call. |
| [`build-and-run`](./build-and-run/SKILL.md) | Compiling, running, verifying the project. |

## How to use

- Read the `go-conventions` and `project-architecture` skills first if
  you are new to the codebase.
- Before editing a file, read the matching skill so you know which
  patterns to preserve.
- Skills are not executable — they are guidance for the agent. Apply
  them, then verify with the commands in `build-and-run`.

## Adding a new skill

1. Create `skills/<name>/SKILL.md`.
2. Start with "When to use this skill" so the agent knows when to load it.
3. Reference real file paths and line ranges in this repo, not abstract
   descriptions.
4. Keep it short — bullet points and small code blocks, no essays.
5. Update the table above.

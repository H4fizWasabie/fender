# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

Developers working in a local repository, from people using a coding agent for the first time to experts who want to inspect its decisions, tools, evidence, and guardrails. The default experience must be understandable without hiding the deeper runtime truth experts need.

## Product Purpose

Fender is a from-scratch Go coding agent that takes a software task from instruction through repository work, verification, and a truthful final handoff. Success means the user can direct Fender, understand its current state, intervene when required, and judge the outcome without manually operating an editor inside Fender.

## Positioning

Fender gives one persistent main agent freedom inside deterministic harness guardrails. It combines native code intelligence, layered project memory, artifact-aware context, skills, resumable sessions, and an ephemeral child agent without introducing a planner, swarm, or orchestration system.

## Operating Context

Fender runs locally inside the current project. The browser workbench is the complete visual experience; the terminal remains a fast companion for interactive and autonomous use. A user starts a task, observes activity, responds to approvals or blockers, reviews changed files and verification evidence, and may resume an older session explicitly.

## Capabilities and Constraints

- Opening Fender starts a new session by default; previous sessions remain resumable through an explicit action. This confirmed behavior is not yet implemented everywhere.
- Users inspect source, changes, tests, context, memory, skills, and evidence, but do not directly edit source code in Fender for now.
- One persistent main agent owns the session and final outcome. `delegate` creates one synchronous ephemeral child with isolated working state and no descendants.
- A separately configured provider/API key is fallback resilience for the same agent, never another agent.
- Tool execution is sequential and guarded by strict, balanced, or yolo permission modes; hard refusals remain enforced in every mode.
- The product remains a single Go binary with an embedded localhost web UI. The frontend uses plain HTML, CSS, and JavaScript; no UI framework or new dependency is assumed.
- The current observer and HTTP APIs expose only a subset of the truthful state the redesigned workbench will require; backend surface changes must remain runtime-grounded.

## Brand Commitments

The product name is Fender. Its voice is direct, calm, technically honest, and concise. It should feel like a focused agent workbench: IDE capabilities without IDE clutter, and never a generic AI chat wrapper or a VS Code imitation.

## Evidence on Hand

- The implemented runtime and decisions are recorded in `DECISIONS.md`, especially D50.
- Canonical agent terminology is recorded in `CONTEXT.md`.
- The existing browser surface is `cmd/fender/static/index.html`, served by `cmd/fender/dashboard.go`.
- The terminal experience is implemented in `cmd/fender/repl.go`.
- Real session, tool, guardrail, context, memory, code-intelligence, thinking, child-agent, completion, and provider-fallback behavior exists in the repository with Go tests.
- No logo system, illustration library, customer proof, benchmark claims, or other visual assets have been supplied; future design work must not fabricate them.

## Product Principles

- Start calm; reveal detail only when it becomes relevant or requested.
- Show runtime truth, not decorative activity or implied capabilities.
- Keep one obvious next action while preserving expert inspectability.
- Treat completion as an evidence-backed review handoff, not merely another chat response.
- Keep the main agent responsible; child work and provider fallback remain subordinate implementation details.

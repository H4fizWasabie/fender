---
version: 1
slug: "cmd-fender-static-index-html"
primary_target: "cmd/fender/static/index.html"
related_targets: ["cmd/fender/dashboard.go"]
---

# Fender Workbench Redesign Brief

## Scope and mode

- Surface: the browser workbench, with the terminal renderer remaining a compact companion.
- Mode: operate. Users arrive to give Fender a coding task, follow its progress, intervene when needed, and verify the result.
- Audience: developers of any experience level. The interface must be understandable without hiding the engineering truth experts need.

## Job, action, and proof

- Job: move one repository task from intent to a trustworthy handoff.
- Primary action: describe the task in a fresh session.
- Secondary action: explicitly resume an earlier session.
- Proof: show the real current agent state, guardrail holds, tool activity, changed files, checks, failures, and final evidence when the runtime can supply them.
- Source code remains inspectable through Fender's work, not directly editable by the user in this redesign.

## Constraints

- Every launch opens a new session; history stays one quiet action away.
- One persistent main agent owns the task. Ephemeral child work appears as subordinate activity, never as a swarm.
- Progressive disclosure is mandatory: default to the task and next action; reveal raw details on demand.
- Do not imply backend observer data that Fender does not expose. Missing data needs an honest empty or unavailable state.
- Plain HTML, CSS, and JavaScript embedded in the single Go binary; no UI framework.
- Preserve strict, balanced, and yolo guardrail meaning. A hard refusal remains visible and final in every mode.

## Chosen direction

**Engineer's Job Docket.** One task sheet gathers every action, exception, and proof until it earns a signed outcome. The metaphor supplies hierarchy and language, not literal office decoration.

Palette: warm technical stock, graphite, safety orange for attention, verification green for proven success. Component character: clipped sheets, thin rules, index tabs, restrained stamps, flat matte surfaces, and almost no elevation.

## Memorable moment

At completion, the active task does not merely say “done.” Its scattered activity resolves into a compact verification handoff: what changed, what was checked, what remains uncertain, and the next safe action.

## States to design

1. Fresh session: focused composer, small resume affordance, repository and permission context.
2. Active run: current objective, concise narrative updates, subordinate activity, interrupt or answer affordance.
3. Guardrail hold: exact command or action, verdict, reason, and a single clear decision.
4. Failure or thrash: visible problem, what Fender tried, and the recovery path.
5. Completed run: outcome, changed files, checks, evidence, caveats, and follow-up.
6. Resumed session: restored context is explicit; the user can distinguish old evidence from new work.

## Approved composition

**Centered Docket** (`.impeccable/mocks/comp-centered.png`). A compact left index holds New session, explicit Resume, repository, and permission context. The task docket owns the center. The right evidence lane is absent in a fresh session and attaches slips only when real observer events exist. Completion resolves the activity into a compact verification handoff.

The comp's demonstration task, counts, logo mark, and signed certificate are not literal product content. Fender must render only live session messages, observer events, guardrail state, and completion status available from its runtime.

## Implementation inventory

| Ingredient | Commitment | Medium |
|---|---|---|
| Top ledger | Product identity plus live model and permission context | Semantic HTML/CSS |
| Session index | New session primary; explicit resumable history in an on-demand drawer | Semantic HTML/CSS/JS |
| Main docket | One task composer and chronological user/agent record | Semantic HTML/CSS/JS |
| Activity slips | Tool, child, thinking, error, and completion events only when emitted | Semantic HTML/CSS/JS |
| Completion handoff | Runtime status and final reply; no inferred tests or file counts | Semantic HTML/CSS/JS |
| Symbols | Small authored line icons with consistent 1.75px strokes | Inline SVG |
| Material | Warm stock color, clipped corners, graphite rules; no raster texture | CSS |
| Primary action | Safety-orange submit tab attached to the composer | Semantic button/CSS |

Component grammar: flat clipped sheets; 1px graphite rules; 2px state rules; 8px control corners; no floating cards or soft shadows. Type ramp: 12px machine labels, 14–16px controls/body, 20px section titles, 32–44px task headline. System humanist sans for UI; system mono for machine facts. Motion is limited to one lane reveal, drawer slide, progress pulse, and reduced-motion fallbacks.

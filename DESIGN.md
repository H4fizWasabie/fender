---
name: Fender
description: A warm technical job docket for truthful, evidence-backed coding work.
colors:
  stock: "#f2f0e8"
  stock-deep: "#e8e5da"
  paper: "#faf8f1"
  graphite: "#1d2321"
  muted: "#626763"
  rule: "#c8ccc4"
  rule-dark: "#9da39d"
  white: "#ffffff"
  orange: "#d94f16"
  orange-dark: "#a9380e"
  orange-soft: "#f6e1d7"
  green: "#286a4a"
  green-dark: "#1e553b"
  green-soft: "#deebe3"
  green-border: "#87a794"
  completion-border: "#71927f"
  red: "#a9362c"
  red-soft: "#f2dfdc"
  red-border: "#c89790"
  amber: "#9a5a0a"
  amber-soft: "#f4e7ce"
  amber-border: "#c8a66e"
  disabled: "#747873"
typography:
  display:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "clamp(32px, 4vw, 46px)"
    fontWeight: 700
    lineHeight: 1.04
    letterSpacing: "-0.035em"
  title:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "19px"
    fontWeight: 700
    lineHeight: 1.2
    letterSpacing: "-0.02em"
  body:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "15px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", ui-monospace, monospace'
    fontSize: "10px"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "0.09em"
  data:
    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", ui-monospace, monospace'
    fontSize: "11px"
    fontWeight: 500
    lineHeight: 1.5
    letterSpacing: "0.08em"
  event-meta:
    fontFamily: '"SFMono-Regular", Consolas, "Liberation Mono", ui-monospace, monospace'
    fontSize: "9px"
    fontWeight: 650
    lineHeight: 1.5
    letterSpacing: "0.09em"
  supporting:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.55
  detail:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
  compact-control:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.35
  empty-copy:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "clamp(17px, 2vw, 20px)"
    fontWeight: 400
    lineHeight: 1.55
  wordmark:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "24px"
    fontWeight: 760
    lineHeight: 1
    letterSpacing: "-0.025em"
  wordmark-compact:
    fontFamily: 'Aptos, "Segoe UI Variable", "Segoe UI", "Helvetica Neue", Arial, sans-serif'
    fontSize: "21px"
    fontWeight: 760
    lineHeight: 1
    letterSpacing: "-0.025em"
rounded:
  flat: "0px"
  hairline: "1px"
  clipped-detail: "2px"
  key: "3px"
  detail: "4px"
  decision: "6px"
  transient: "7px"
  action: "8px"
  action-tab: "8px 8px 2px 8px"
  slip: "2px 8px 2px 8px"
  pill: "999px"
spacing:
  xxs: "4px"
  xs: "8px"
  sm: "12px"
  md: "16px"
  lg: "24px"
  xl: "32px"
  xxl: "48px"
components:
  button-primary:
    backgroundColor: "{colors.orange}"
    textColor: "{colors.white}"
    typography: "{typography.body}"
    rounded: "{rounded.action-tab}"
    padding: "0 14px"
    height: "48px"
  button-primary-hover:
    backgroundColor: "{colors.orange-dark}"
    textColor: "{colors.white}"
  button-quiet:
    backgroundColor: "transparent"
    textColor: "{colors.graphite}"
    typography: "{typography.body}"
    rounded: "{rounded.action}"
    padding: "0 12px"
    height: "44px"
  button-approval:
    backgroundColor: "{colors.green}"
    textColor: "{colors.white}"
    rounded: "{rounded.decision}"
    padding: "0 10px"
    height: "36px"
  input-composer:
    backgroundColor: "#fffdf7"
    textColor: "{colors.graphite}"
    typography: "{typography.body}"
    rounded: "{rounded.action-tab}"
    padding: "12px 18px 18px"
  status-complete:
    backgroundColor: "{colors.green-soft}"
    textColor: "{colors.green}"
    typography: "{typography.data}"
    rounded: "{rounded.pill}"
    padding: "0 12px"
    height: "34px"
  card-docket:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.graphite}"
    rounded: "{rounded.flat}"
    padding: "24px"
  card-evidence:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.graphite}"
    rounded: "{rounded.slip}"
    padding: "16px"
---

# Design System: Fender

## Overview

**Creative North Star: "Engineer's Job Docket"**

Fender's browser visual system treats one coding task as a technical job docket: a calm sheet that gathers instruction, activity, exceptions, and the final runtime handoff. The metaphor supplies hierarchy, clipped geometry, index tabs, stamps, and inspection slips without becoming literal office decoration. It is direct, matte, and operational rather than ornamental.

The browser is the complete visual expression of this system. It begins quietly with one obvious task action, reveals evidence only when the runtime emits it, and preserves deeper facts for inspection. The terminal remains a plain streaming transcript and is not a second visual expression of the docket. No raster texture, external imagery, fabricated logo, or decorative proof asset is part of the system.

**Key Characteristics:**

- Warm technical stock and paper surfaces separated by graphite and neutral rules.
- One dominant task sheet, one compact session index, and evidence slips that attach only when real events exist.
- Flat matte geometry with clipped corners, restrained tabs, and almost no elevation.
- Safety orange for action and attention; verification green only for proven completion.
- Authored line symbols and machine labels that make runtime facts scannable without resembling an IDE.

## Colors

The palette resembles warm archival stock marked with restrained operational inks; every state color has a specific runtime meaning.

### Primary

- **Safety Orange** (`orange`): the singular action and attention ink for primary tabs, active progress, pending guardrail review, and small sheet accents.
- **Burnt Orange** (`orange-dark`): the high-contrast hover and keyboard-focus companion to Safety Orange.
- **Pale Orange Wash** (`orange-soft`): a quiet supporting tint when an orange state needs a surface, never a substitute for a textual status.

### Secondary

- **Verification Green** (`green`): proven completion, successful approval actions, online presence, and assistant-origin markers.
- **Deep Verification Green** (`green-dark`): hover feedback for an already-validating green approval action.
- **Verification Wash** (`green-soft`): the background for completed status chips and final handoff slips.
- **Verification Rule** (`green-border`) and **Completion Rule** (`completion-border`): borders that retain state meaning without filling another surface.

### Tertiary

- **Hold Amber** (`amber`) and **Hold Wash** (`amber-soft`): blocked or stalled work that needs attention but has not failed.
- **Hold Rule** (`amber-border`): the restrained outline companion for an amber status chip.
- **Failure Red** (`red`) and **Failure Wash** (`red-soft`): errors, denial, cancellation, and unavailable connections.
- **Failure Rule** (`red-border`): the restrained outline companion for a red status chip.

### Neutral

- **Technical Stock** (`stock`) and **Deep Stock** (`stock-deep`): the workbench field and outer canvas.
- **Docket Paper** (`paper`) and **White** (`white`): the active sheet, attached slips, and high-contrast action text.
- **Graphite Ink** (`graphite`): primary type, structural borders, icons, and decisive focus geometry.
- **Muted Graphite** (`muted`): secondary copy and machine context that must remain readable.
- **Rule** (`rule`) and **Dark Rule** (`rule-dark`): internal dividers and stronger sheet boundaries.
- **Disabled Graphite** (`disabled`): busy primary controls that must remain legible while clearly unavailable.

### Named Rules

**The Evidence Color Rule.** Green means the runtime has actually completed or approved something; orange means action, progress, or a pending decision; amber means blocked or stalled; red means error, denial, cancellation, or disconnection. Text, iconography, and geometry must carry the same meaning so color is never the only signal.

**The Stock, Not Texture Rule.** Warmth comes from flat color fields and rules. Do not add paper grain, noise, gradients, glass, or photographic texture.

## Typography

**Display Font:** Aptos with Segoe UI Variable, Segoe UI, Helvetica Neue, Arial, then sans-serif fallbacks.

**Body Font:** The same system humanist sans stack.

**Label/Mono Font:** SFMono-Regular with Consolas, Liberation Mono, ui-monospace, then monospace fallbacks.

**Character:** Humanist sans type keeps instructions and handoffs calm and approachable. Monospaced type marks machine facts, runtime labels, keys, IDs, and compact status metadata without turning the whole interface into a terminal.

### Hierarchy

- **Display** (700, `clamp(32px, 4vw, 46px)`, 1.04): the task-sheet question or current task headline; keep it to roughly 22 characters per line when the composition allows.
- **Title** (700, 19px, 1.2): evidence-lane and drawer headings.
- **Body** (400, 15px, 1.5): controls and default interface copy; chronological messages open to 1.68 line-height and stop near 72 characters per line.
- **Empty-state lead** (400, `clamp(17px, 2vw, 20px)`, 1.55): the one larger explanatory line before work begins.
- **Supporting copy** (400, 12–14px, about 1.35–1.55): context explanations, evidence detail, session notes, compact navigation, and empty states.
- **Label** (650–700, 9–11px, up to 0.09em tracking): uppercase machine labels, roles, stamps, and event metadata.
- **Wordmark** (760, 24px desktop and 21px compact, 1): product name in the ledger; never reused as a general heading.

### Named Rules

**The Human First, Machine Second Rule.** Use the humanist stack for tasks, decisions, and explanations; reserve mono for short runtime facts. Never set long-form conversation or handoff copy in mono.

## Layout

The desktop shell uses a 66px ledger above a 224px session index and a fluid workbench. The main docket is centered and capped at 920px. A 300px evidence lane becomes a third column only after runtime events exist; its appearance may compress the center but must never precede the task. Workbench padding scales between 22px and 42px, while the docket's internal horizontal padding scales between 24px and 48px.

At 1180px and below, the rail narrows to 200px and evidence moves below the docket in a two-column slip grid. At 820px and below, the header becomes 58px, the session index becomes an inert off-canvas drawer, and evidence follows the docket in the document flow. At 580px and below, evidence becomes one column, secondary empty-state instructions disappear, and the orange submit action remains attached to the composer.

Progressive disclosure is structural. Fresh sessions show the task and runtime context without an empty evidence shell. Saved sessions live in an on-demand drawer. Resumed history is visibly separated from new work. Reasoning detail uses native disclosure; guardrail holds expose one approve-or-deny decision; completion resolves to the runtime's status and final reply only.

### Named Rules

**The Docket Owns the Center Rule.** Repository context supports the task from the edge, and evidence attaches after activity; neither becomes a competing primary workspace.

**The Evidence Must Exist Rule.** Do not reserve space for, count, or label checks, files, tool activity, or completion evidence until the backend has supplied that truth.

## Elevation & Depth

Fender is flat by default. Paper, stock, borders, clipped corners, and state bars establish hierarchy; ordinary sheets and slips cast no shadow. A restrained shadow appears only when the composer has focus (`0 8px 22px rgba(29, 35, 33, .07)`) or when a transient toast must separate from the workbench (`0 10px 28px rgba(29, 35, 33, .22)`).

### Shadow Vocabulary

- **Composer Focus:** a low, diffuse lift that confirms the active text-entry surface.
- **Transient Toast:** a stronger temporary lift for a message that floats above the docket.

### Named Rules

**The Flat by Default Rule.** No soft card shadows, stacked floating panels, blur, or glass. Elevation is an interaction exception, not a resting style.

## Shapes

The main docket is square-edged with two 12px clipped corners, not a rounded card. Thin 1px rules define sheets and sections. Evidence slips use asymmetric 2px/8px corners and a 4px right-edge state marker. Action controls use 8px corners with one corner tightened to 2px so they read as attached tabs; micro-details use 1px, 3px, or 4px corners, paired decisions use 6px, transient toasts use 7px, and status chips alone may use a full pill. Small circles are reserved for connection, run, message, and icon markers.

Icons are authored inline SVG, normally 20px square with a 1.75px stroke, round caps, round joins, and no fill. Slip icons reduce to 17px inside a 32px circular holder. Icons always accompany a visible label for primary actions and runtime states; external icon packages, icon fonts, raster symbols, and invented brand marks are outside the system.

### Named Rules

**The Not-a-Logo Rule.** The Fender wordmark is text. Orange slashes, bars, and skewed rules are compositional accents, never a fabricated logo or certificate mark.

## Components

### Buttons

- **Primary tabs:** Safety Orange with white text, a minimum 48px height, 14px horizontal padding, and the attached-tab corner treatment. Hover shifts to Burnt Orange; active presses down by 1px.
- **Submit tab:** the same orange action language, attached to the composer's lower-right edge with a minimum 154px width on wide screens and 52px height on narrow screens.
- **Quiet actions:** transparent by default, at least 44px tall, with an 8px corner. Hover adds a faint white stock fill and a neutral rule.
- **Approval actions:** paired 36px decisions. Deny remains outlined; “Approve once” uses Verification Green because approval is an explicit successful action.
- **Disabled/busy:** muted graphite on pale type with a wait cursor. Do not communicate busy state through opacity alone.
- **Focus:** every interactive button, link, and textarea receives a 3px Burnt Orange outline with a 3px offset.

### Chips

Run status uses an uppercase mono pill with a dot, border, tint, and text label. Working pulses the orange dot; complete uses the green pair; blocked and stalled use amber; error and cancelled use red. Connection status uses the same dot-and-label grammar at ledger scale.

### Cards / Containers

- **Task docket:** flat Docket Paper, a Dark Rule border, clipped corners, and a short orange registration bar near the top edge.
- **Evidence slips:** flat paper with asymmetric corners, a circular line-icon holder, compact mono metadata, and a right-edge state marker. Completion changes to Verification Wash only when the runtime reports completion.
- **Conversation records:** chronological ruled rows with role labels and small origin dots; user and Fender messages remain text, never chat bubbles.

### Inputs / Fields

The task composer is a warm near-white field bounded by a Dark Rule and the attached-tab corner treatment. Its label is uppercase mono; its textarea has no inner border and grows to 260px. Focus strengthens the outer rule and applies Composer Focus. `Enter` submits; `Shift+Enter` inserts a line break.

### Navigation

The session index is a contextual rail on wide screens and an off-canvas drawer on narrow screens. “New session” is primary; “Resume earlier work” is quiet and opens a separate saved-session drawer. While either drawer is closed it is inert; the saved-session drawer traps keyboard focus, closes on Escape or scrim activation, and restores focus to its trigger.

### Progressive Evidence

Activity slips represent emitted tool, child, thinking, approval, error, and completion events. Raw reasoning stays behind native `details`. The conversation and evidence regions announce updates politely; toasts use a status live region. Resumed provenance is marked before restored messages, and completion displays only the real runtime status and final reply—never inferred changed files, checks, counts, signatures, or certificates.

### Motion

Motion is limited to a 480ms evidence-lane reveal, a 420ms session-drawer slide, 160ms action feedback, a 1.4s working pulse, and short scrim/toast transitions. The primary easing is `cubic-bezier(.16, 1, .3, 1)`. Under `prefers-reduced-motion: reduce`, animations and transitions collapse to 0.01ms and smooth scrolling is disabled.

## Do's and Don'ts

### Do:

- **Do** keep the browser hierarchy task first, supporting context second, and emitted evidence third.
- **Do** use the exact state-color semantics and pair color with text, icon, border, or marker changes.
- **Do** keep sheets matte and separated by 1px rules, clipped geometry, and selective stock tones.
- **Do** preserve visible focus, keyboard submission, Escape behavior, focus trapping, inert off-canvas regions, live-region announcements, and reduced-motion fallbacks.
- **Do** show resumed provenance and progressively disclose raw reasoning or machine detail.
- **Do** render only repository, session, model, permission, observer, approval, and completion facts supplied by the runtime.

### Don't:

- **Don't** fabricate a logo, paper texture, task count, changed-file count, check result, signature, handoff package, or verification certificate.
- **Don't** reserve an empty evidence column or make activity compete with the central task docket.
- **Don't** turn conversation into chat bubbles, source-editor chrome, a VS Code imitation, or a generic AI sidebar.
- **Don't** use Verification Green for aspiration, decoration, in-progress work, or an unproven claim.
- **Don't** add external images, raster assets, icon libraries, gradients, glass, ambient card shadows, or decorative motion.
- **Don't** apply this visual redesign to the terminal; it remains a plain streaming transcript.

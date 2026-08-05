# Fender Agent Model

Fender has one persistent coding agent and may create a short-lived child for bounded delegated work. Provider resilience is deliberately separate from agent identity.

## Language

**Main agent**:
The single persistent Fender actor that owns the user session, makes decisions, operates tools, and remains responsible for the final outcome.
_Avoid_: Parent agent, orchestrator

**Child agent**:
An ephemeral instance of Fender's one Agent type, created for one self-contained delegated task with fresh conversation, context, artifacts, and working state. It may use project grounding and guarded tools, cannot create another child, and disappears after returning its result.
_Avoid_: Worker, swarm member, provider agent

**Project memory**:
The canonical ICM material that grounds both the main agent and a child in the same repository truth.
_Avoid_: Child memory, private knowledge base

**Working state**:
Conversation and artifacts owned by one agent instance for its current task. A child's working state is isolated and never consolidated as a session.
_Avoid_: Project memory, durable child memory

**Provider fallback**:
A backup provider configuration, usually carrying a second API key, used only when the primary model request fails before producing usable output. It is resilience for the same agent, never a child agent.
_Avoid_: Backup agent, fallback subagent

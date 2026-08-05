import { api, humanize } from './api.js';
import { state, ui } from './dom.js';
import { ensureAssistant, setRunStatus, showToast } from './docket.js';
import { addCompletion, addSlip, clearEvidence } from './evidence-view.js';

export { addSlip } from './evidence-view.js';

function addThinking(text, source) {
  if (!state.currentThinking?.isConnected) {
    const slip = addSlip({ kind: 'thinking', title: 'Reasoning available', status: 'working', source });
    slip.classList.add('thinking-slip');
    const details = document.createElement('details');
    const summary = document.createElement('summary');
    summary.textContent = 'Show reasoning';
    const copy = document.createElement('p');
    copy.className = 'thinking-copy';
    details.append(summary, copy);
    slip.appendChild(details);
    state.currentThinking = copy;
  }
  state.currentThinking.textContent += text.replace(/\s+/g, ' ');
}

function approvalActions(slip, id) {
  const actions = document.createElement('div');
  actions.className = 'approval-actions';
  for (const [label, allowed] of [['Deny', false], ['Approve once', true]]) {
    const button = document.createElement('button');
    button.type = 'button';
    button.className = `approval-action${allowed ? ' allow' : ''}`;
    button.textContent = label;
    button.addEventListener('click', () => answerApproval(id, allowed));
    actions.appendChild(button);
  }
  slip.appendChild(actions);
}

function addApproval(event) {
  const existing = event.id ? ui.evidenceList.querySelector(`[data-event-id="${CSS.escape(event.id)}"]`) : null;
  if (existing) {
    existing.className = `evidence-slip is-${event.status}`;
    const meta = existing.querySelector('.event-meta');
    if (meta) meta.textContent = `Main agent · ${humanize(event.status)}`;
    if (event.status !== 'pending') existing.querySelector('.approval-actions')?.remove();
    return existing;
  }
  if (event.status !== 'pending') return null;
  const slip = addSlip({ id: event.id, kind: 'approval', title: 'Guardrail review required', status: 'pending', detail: event.detail || 'This action requires your decision.', command: event.text });
  approvalActions(slip, event.id);
  return slip;
}

async function answerApproval(id, allowed) {
  const actions = ui.evidenceList.querySelector(`[data-event-id="${CSS.escape(id)}"] .approval-actions`);
  actions?.querySelectorAll('button').forEach((button) => { button.disabled = true; });
  try {
    await api('/api/approval', { method: 'POST', body: JSON.stringify({ id, allowed }) });
  } catch (error) {
    showToast(error.message, true);
    actions?.querySelectorAll('button').forEach((button) => { button.disabled = false; });
  }
}

export function renderEvidence(snapshot) {
  clearEvidence();
  const events = Array.isArray(snapshot.events) ? snapshot.events : [];
  for (const event of events) {
    if (event.kind === 'tool') addSlip({ kind: 'tool', title: humanize(event.text || 'Tool call'), status: event.status || 'ok', detail: event.detail, source: event.source });
    if (event.kind === 'approval') addApproval(event);
    if (event.kind === 'done' && !event.source) addCompletion(event.status, event.text);
  }
  if (snapshot.approval) addApproval({ status: 'pending', id: snapshot.approval.id, text: snapshot.approval.command, detail: snapshot.approval.reason });
  const hasCompletion = events.some((event) => event.kind === 'done' && !event.source);
  if (snapshot.terminal && !hasCompletion) {
    const lastAssistant = [...(snapshot.messages || [])].reverse().find((message) => message.role === 'assistant');
    addCompletion(snapshot.status, lastAssistant?.content || 'This saved turn has no additional reply.');
  }
  if (snapshot.persistenceError) addSlip({ kind: 'tool', title: 'Session not saved', status: 'error', detail: snapshot.persistenceError });
}

export function handleEvent(event) {
  const source = event.source || '';
  if (event.kind === 'delta') ensureAssistant().textContent += event.text || '';
  if (event.kind === 'thinking') addThinking(event.text || '', source);
  if (event.kind === 'tool') {
    state.currentAssistant = null;
    state.currentThinking = null;
    addSlip({ kind: 'tool', title: humanize(event.text || 'Tool call'), status: event.status || 'ok', detail: event.detail, source });
  }
  if (event.kind === 'approval') addApproval(event);
  if (event.kind !== 'done' || source) return false;
  const reply = event.text || '';
  if (reply) {
    const target = ensureAssistant();
    if (!target.textContent.includes(reply)) target.textContent += `${target.textContent ? '\n\n' : ''}${reply}`;
  }
  state.currentAssistant = null;
  state.currentThinking = null;
  setRunStatus(event.status, false);
  addCompletion(event.status, reply);
  return true;
}

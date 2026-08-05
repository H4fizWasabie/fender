import { readableStatus } from './api.js';
import { ui, state } from './dom.js';

export function showToast(message, isError = false) {
  clearTimeout(state.toastTimer);
  ui.toast.textContent = message;
  ui.toast.classList.toggle('is-error', isError);
  ui.toast.classList.add('is-visible');
  state.toastTimer = setTimeout(() => ui.toast.classList.remove('is-visible'), 3600);
}

export function setConnection(kind, label) {
  ui.connection.className = `connection-state is-${kind}`;
  ui.connection.querySelector('span').textContent = label;
}

export function setRunStatus(status, busy = status === 'working') {
  const normalized = status || 'ready';
  ui.status.className = `run-status status-${normalized}`;
  ui.status.querySelector('span').textContent = readableStatus(normalized);
	ui.input.disabled = busy;
  ui.send.disabled = busy;
  ui.newSession.disabled = busy;
}

function modeDescription(mode) {
  const descriptions = {
    strict: 'Every tool action asks before it runs.',
    balanced: 'Safe work runs; risky actions require review.',
    yolo: 'Actions run freely; hard refusals still apply.',
  };
  return descriptions[mode] || 'Guardrail mode is reported by the runtime.';
}

export function renderMeta(snapshot) {
  const providerModel = [snapshot.provider, snapshot.model].filter(Boolean).join(' / ') || 'Unavailable';
  document.querySelector('#workspaceName').textContent = snapshot.workspace || 'Current repository';
  document.querySelector('#topWorkspace').textContent = snapshot.workspace || '—';
  document.querySelector('#modeName').textContent = readableStatus(snapshot.mode);
  document.querySelector('#modeDescription').textContent = modeDescription(snapshot.mode);
  document.querySelector('#topMode').textContent = snapshot.mode || '—';
  document.querySelector('#modelName').textContent = providerModel;
  document.querySelector('#topModel').textContent = providerModel;
  const hasMessages = Array.isArray(snapshot.messages) && snapshot.messages.length > 0;
  ui.stamp.textContent = state.resumed ? `RESUMED · ${snapshot.sessionId}` : hasMessages ? snapshot.sessionId : 'NEW SESSION';
  const displayStatus = snapshot.busy ? 'working' : snapshot.status === 'working' ? 'interrupted' : snapshot.status;
  setRunStatus(displayStatus, snapshot.busy);
}

export function appendMessage(role, content) {
  if (!content) return null;
  ui.empty.hidden = true;
  const article = document.createElement('article');
  article.className = `message message-${role}`;
  const label = document.createElement('p');
  label.className = 'message-role';
  label.textContent = role === 'user' ? 'You' : 'Fender';
  const body = document.createElement('p');
  body.className = 'message-content';
  body.textContent = content;
  article.append(label, body);
  ui.conversation.appendChild(article);
  article.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  return body;
}

export function renderMessages(messages, restoredCount = 0) {
  ui.conversation.replaceChildren();
  ui.conversation.appendChild(ui.empty);
  ui.empty.hidden = messages.length > 0;
  if (restoredCount > 0 && messages.length) {
    const divider = document.createElement('div');
    divider.className = 'restored-divider';
    divider.textContent = 'Restored session history';
    ui.conversation.appendChild(divider);
  }
  messages.forEach((message, index) => {
    const content = appendMessage(message.role, message.content);
    if (index < restoredCount && content) content.closest('.message')?.classList.add('message-restored');
  });
  state.currentAssistant = null;
}

export function ensureAssistant() {
  if (state.currentAssistant?.isConnected) return state.currentAssistant;
  state.currentAssistant = appendMessage('assistant', '');
  if (state.currentAssistant) return state.currentAssistant;
  ui.empty.hidden = true;
  const article = document.createElement('article');
  article.className = 'message message-assistant';
  const label = document.createElement('p');
  label.className = 'message-role';
  label.textContent = 'Fender';
  const body = document.createElement('p');
  body.className = 'message-content';
  article.append(label, body);
  ui.conversation.appendChild(article);
  state.currentAssistant = body;
  return body;
}

import { api, formatDate, readableStatus } from './api.js';
import { state, ui } from './dom.js';
import { renderMessages, renderMeta, showToast } from './docket.js';
import { renderEvidence } from './evidence.js';

export const narrowViewport = window.matchMedia('(max-width: 820px)');

function hydrateSnapshot(snapshot) {
  state.snapshot = snapshot;
  state.restoredCount = snapshot.restoredCount || 0;
  state.resumed = state.restoredCount > 0;
  renderMeta(snapshot);
  renderMessages(snapshot.messages || [], state.restoredCount);
  renderEvidence(snapshot);
}

export async function loadSnapshot() {
  const snapshot = await api('/api/state');
  hydrateSnapshot(snapshot);
  return snapshot;
}

export async function loadSessions(showDrawer = true) {
  try {
    const sessions = await api('/api/sessions');
    ui.sessionList.replaceChildren();
    if (!sessions.length) {
      const empty = document.createElement('p');
      empty.className = 'session-empty';
      empty.textContent = 'No saved sessions yet.';
      ui.sessionList.appendChild(empty);
    }
    for (const session of sessions) {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'session-item';
      const title = document.createElement('strong');
      title.textContent = session.title;
      const time = document.createElement('time');
      time.dateTime = session.updated || session.started;
      time.textContent = formatDate(session.updated || session.started);
      const status = document.createElement('span');
      status.className = 'session-status';
      status.textContent = readableStatus(session.status || 'saved');
      button.append(title, time, status);
      button.addEventListener('click', () => resumeSession(session.id));
      ui.sessionList.appendChild(button);
    }
    if (showDrawer) openSessions();
  } catch (error) {
    showToast(error.message, true);
  }
}

function openSessions() {
  closeMobileIndex();
  ui.body.classList.add('sessions-open');
  ui.drawer.setAttribute('aria-hidden', 'false');
  ui.drawer.inert = false;
  ui.shell.inert = true;
  ui.resume.setAttribute('aria-expanded', 'true');
  ui.closeSessions.focus();
}

export function closeSessions() {
  ui.body.classList.remove('sessions-open');
  ui.drawer.setAttribute('aria-hidden', 'true');
  ui.drawer.inert = true;
  ui.shell.inert = false;
  ui.resume.setAttribute('aria-expanded', 'false');
  (narrowViewport.matches ? ui.mobileIndex : ui.resume).focus();
}

export async function startNewSession() {
  try {
    const snapshot = await api('/api/session/new', { method: 'POST' });
    hydrateSnapshot(snapshot);
    closeMobileIndex();
    closeSessionsIfOpen();
    ui.input.focus();
    showToast('Fresh session ready.');
  } catch (error) {
    showToast(error.message, true);
  }
}

async function resumeSession(id) {
  try {
    const snapshot = await api('/api/session/resume', { method: 'POST', body: JSON.stringify({ id }) });
    hydrateSnapshot(snapshot);
    closeSessions();
    ui.input.focus();
    showToast('Session resumed.');
  } catch (error) {
    showToast(error.message, true);
  }
}

export function closeSessionsIfOpen() {
  if (ui.body.classList.contains('sessions-open')) closeSessions();
}

export function closeMobileIndex() {
  ui.body.classList.remove('mobile-index-open');
  ui.mobileIndex.setAttribute('aria-expanded', 'false');
  syncMobileIndexInert();
}

export function syncMobileIndexInert() {
  ui.sessionIndex.inert = narrowViewport.matches && !ui.body.classList.contains('mobile-index-open');
}

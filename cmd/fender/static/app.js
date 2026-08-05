import { ui, state } from './dom.js';
import { api } from './api.js';
import { appendMessage, setConnection, setRunStatus, showToast } from './docket.js';
import { addSlip, handleEvent } from './evidence.js';
import {
  closeMobileIndex,
  closeSessions,
  closeSessionsIfOpen,
  loadSessions,
  loadSnapshot,
  narrowViewport,
  startNewSession,
  syncMobileIndexInert,
} from './sessions.js';

async function submitTask(event) {
  event.preventDefault();
  const text = ui.input.value.trim();
  if (!text || ui.send.disabled) return;
  ui.input.value = '';
  ui.input.style.height = '';
  appendMessage('user', text);
  state.currentAssistant = null;
  setRunStatus('working', true);
  const sessionID = state.snapshot?.sessionId || 'ACTIVE SESSION';
  ui.stamp.textContent = state.resumed ? `RESUMED · ${sessionID}` : sessionID;
  try {
    await api('/api/message', { method: 'POST', body: JSON.stringify({ text }) });
    await loadSnapshot();
  } catch (error) {
    setRunStatus('error');
    addSlip({ kind: 'tool', title: 'Request failed', status: 'error', detail: error.message });
    showToast(error.message, true);
  }
}

ui.form.addEventListener('submit', submitTask);
ui.input.addEventListener('keydown', (event) => {
  if (event.key === 'Enter' && !event.shiftKey) {
    event.preventDefault();
    ui.form.requestSubmit();
  }
});
ui.input.addEventListener('input', () => {
  ui.input.style.height = 'auto';
  ui.input.style.height = `${Math.min(ui.input.scrollHeight, 260)}px`;
});
ui.newSession.addEventListener('click', startNewSession);
ui.resume.addEventListener('click', () => loadSessions(true));
ui.closeSessions.addEventListener('click', closeSessions);
ui.drawerScrim.addEventListener('click', closeSessions);
ui.mobileIndex.addEventListener('click', () => {
  const open = ui.body.classList.toggle('mobile-index-open');
  ui.mobileIndex.setAttribute('aria-expanded', String(open));
  syncMobileIndexInert();
});
narrowViewport.addEventListener('change', syncMobileIndexInert);
syncMobileIndexInert();

document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') {
    closeSessionsIfOpen();
    closeMobileIndex();
  }
  if (event.key !== 'Tab' || !ui.body.classList.contains('sessions-open')) return;
  const focusable = [...ui.drawer.querySelectorAll('button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')];
  if (!focusable.length) return;
  const [first] = focusable;
  const last = focusable[focusable.length - 1];
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
});

const events = new EventSource('/api/events');
events.onopen = () => setConnection('online', 'Live');
events.onerror = () => setConnection('offline', 'Reconnecting');
events.onmessage = ({ data }) => {
  try {
    if (handleEvent(JSON.parse(data))) loadSessions(false);
  } catch (error) {
    showToast(`Event could not be read: ${error.message}`, true);
  }
};

loadSnapshot()
  .then(() => setConnection('online', 'Live'))
  .catch((error) => {
    setConnection('offline', 'Unavailable');
    showToast(error.message, true);
  });

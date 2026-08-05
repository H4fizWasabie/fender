const $ = (selector) => document.querySelector(selector);

const ui = {
  body: document.body,
  shell: $('#appShell'),
  conversation: $('#conversation'),
  empty: $('#emptyDocket'),
  form: $('#taskComposer'),
  input: $('#taskInput'),
  send: $('#sendTask'),
  status: $('#runStatus'),
  stamp: $('#sessionStamp'),
  evidence: $('#evidenceLane'),
  evidenceList: $('#evidenceList'),
  evidenceCount: $('#evidenceCount'),
  newSession: $('#newSession'),
  resume: $('#resumeSessions'),
  drawer: $('#sessionDrawer'),
  sessionList: $('#sessionList'),
  closeSessions: $('#closeSessions'),
  drawerScrim: $('#drawerScrim'),
  mobileIndex: $('#mobileIndexToggle'),
  sessionIndex: $('#sessionIndex'),
  connection: $('#connectionState'),
  toast: $('#toast'),
};

const state = {
  snapshot: null,
  currentAssistant: null,
  currentThinking: null,
  evidenceTotal: 0,
  toastTimer: null,
  resumed: false,
  restoredCount: 0,
};

function icon(name) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', `#icon-${name}`);
  svg.appendChild(use);
  return svg;
}

async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `${response.status} ${response.statusText}`);
  return data;
}

function showToast(message, isError = false) {
  clearTimeout(state.toastTimer);
  ui.toast.textContent = message;
  ui.toast.classList.toggle('is-error', isError);
  ui.toast.classList.add('is-visible');
  state.toastTimer = setTimeout(() => ui.toast.classList.remove('is-visible'), 3600);
}

function setConnection(kind, label) {
  ui.connection.className = `connection-state is-${kind}`;
  ui.connection.querySelector('span').textContent = label;
}

function modeDescription(mode) {
  const descriptions = {
    strict: 'Every tool action asks before it runs.',
    balanced: 'Safe work runs; risky actions require review.',
    yolo: 'Actions run freely; hard refusals still apply.',
  };
  return descriptions[mode] || 'Guardrail mode is reported by the runtime.';
}

function readableStatus(status) {
  const labels = {
    ready: 'Ready', working: 'Working', complete: 'Complete', blocked: 'Blocked',
    stalled: 'Stalled', error: 'Error', cancelled: 'Cancelled',
  };
  return labels[status] || status || 'Ready';
}

function setRunStatus(status) {
  const normalized = status || 'ready';
  ui.status.className = `run-status status-${normalized}`;
  ui.status.querySelector('span').textContent = readableStatus(normalized);
  const busy = normalized === 'working';
  ui.input.disabled = busy;
  ui.send.disabled = busy;
  ui.newSession.disabled = busy;
}

function renderMeta(snapshot) {
  const providerModel = [snapshot.provider, snapshot.model].filter(Boolean).join(' / ') || 'Unavailable';
  $('#workspaceName').textContent = snapshot.workspace || 'Current repository';
  $('#topWorkspace').textContent = snapshot.workspace || '—';
  $('#modeName').textContent = readableStatus(snapshot.mode);
  $('#modeDescription').textContent = modeDescription(snapshot.mode);
  $('#topMode').textContent = snapshot.mode || '—';
  $('#modelName').textContent = providerModel;
  $('#topModel').textContent = providerModel;
  const hasMessages = Array.isArray(snapshot.messages) && snapshot.messages.length > 0;
  ui.stamp.textContent = state.resumed ? `RESUMED · ${snapshot.sessionId}` : hasMessages ? snapshot.sessionId : 'NEW SESSION';
  setRunStatus(snapshot.busy ? 'working' : snapshot.status);
}

function appendMessage(role, content) {
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

function renderMessages(messages, restoredCount = 0) {
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

function ensureAssistant() {
  if (!state.currentAssistant || !state.currentAssistant.isConnected) {
    state.currentAssistant = appendMessage('assistant', '');
    if (!state.currentAssistant) {
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
    }
  }
  return state.currentAssistant;
}

function humanize(value) {
  return String(value || '').replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

function revealEvidence() {
  if (!ui.evidence.hidden) return;
  ui.evidence.hidden = false;
  ui.body.classList.add('has-evidence');
}

function clearEvidence() {
  ui.evidenceList.replaceChildren();
  ui.evidence.hidden = true;
  ui.body.classList.remove('has-evidence');
  state.evidenceTotal = 0;
  state.currentThinking = null;
  ui.evidenceCount.textContent = '0';
}

function addSlip({ id, kind = 'tool', title, status = '', detail = '', source = '', command = '' }) {
  revealEvidence();
  const slip = document.createElement('article');
  slip.className = `evidence-slip is-${status || 'neutral'}`;
  if (id) slip.dataset.eventId = id;
  const heading = document.createElement('div');
  heading.className = 'slip-heading';
  const mark = document.createElement('span');
  mark.className = 'slip-icon';
  const iconName = kind === 'approval' || status === 'blocked' ? 'alert'
    : kind === 'done' ? 'check'
      : kind === 'thinking' ? 'think'
        : source ? 'child' : 'tool';
  mark.appendChild(icon(iconName));
  const titleWrap = document.createElement('div');
  titleWrap.className = 'slip-title';
  const strong = document.createElement('strong');
  strong.textContent = title;
  const meta = document.createElement('p');
  meta.className = 'event-meta';
  meta.textContent = [source ? 'Child agent' : 'Main agent', status && humanize(status)].filter(Boolean).join(' · ');
  titleWrap.append(strong, meta);
  heading.append(mark, titleWrap);
  slip.appendChild(heading);
  if (detail) {
    const copy = document.createElement('p');
    copy.className = 'slip-detail';
    copy.textContent = detail;
    slip.appendChild(copy);
  }
  if (command) {
    const code = document.createElement('p');
    code.className = 'slip-detail is-command';
    code.textContent = command;
    slip.appendChild(code);
  }
  ui.evidenceList.appendChild(slip);
  state.evidenceTotal += 1;
  ui.evidenceCount.textContent = String(state.evidenceTotal);
  slip.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  return slip;
}

function addThinking(text, source) {
  revealEvidence();
  if (!state.currentThinking || !state.currentThinking.isConnected) {
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

function addApproval(event) {
  const existing = event.id ? ui.evidenceList.querySelector(`[data-event-id="${CSS.escape(event.id)}"]`) : null;
  if (existing) {
    existing.className = `evidence-slip is-${event.status}`;
    const meta = existing.querySelector('.event-meta');
    if (meta) meta.textContent = `Main agent · ${humanize(event.status)}`;
    existing.querySelector('.approval-actions')?.remove();
    return;
  }
  if (event.status !== 'pending') return;
  const slip = addSlip({
    id: event.id,
    kind: 'approval',
    title: 'Guardrail review required',
    status: 'pending',
    detail: event.detail || 'This action requires your decision.',
    command: event.text,
  });
  const actions = document.createElement('div');
  actions.className = 'approval-actions';
  const deny = document.createElement('button');
  deny.type = 'button';
  deny.className = 'approval-action';
  deny.textContent = 'Deny';
  const allow = document.createElement('button');
  allow.type = 'button';
  allow.className = 'approval-action allow';
  allow.textContent = 'Approve once';
  deny.addEventListener('click', () => answerApproval(event.id, false));
  allow.addEventListener('click', () => answerApproval(event.id, true));
  actions.append(deny, allow);
  slip.appendChild(actions);
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

function addCompletion(status, reply) {
  const slip = addSlip({
    kind: 'done',
    title: status === 'complete' ? 'Handoff ready' : `Run ${readableStatus(status).toLowerCase()}`,
    status,
    detail: reply || 'The runtime ended this turn without an additional reply.',
  });
  slip.classList.add('completion-slip');
}

function handleEvent(event) {
  const source = event.source || '';
  switch (event.kind) {
    case 'delta': {
      const target = ensureAssistant();
      target.textContent += event.text || '';
      break;
    }
    case 'thinking':
      addThinking(event.text || '', source);
      break;
    case 'tool':
      state.currentAssistant = null;
      state.currentThinking = null;
      addSlip({ kind: 'tool', title: humanize(event.text || 'Tool call'), status: event.status || 'ok', source });
      break;
    case 'approval':
      addApproval(event);
      break;
    case 'done': {
      if (source) break;
      const reply = event.text || '';
      if (reply) {
        const target = ensureAssistant();
        if (!target.textContent.includes(reply)) {
          target.textContent += `${target.textContent ? '\n\n' : ''}${reply}`;
        }
      }
      state.currentAssistant = null;
      state.currentThinking = null;
      setRunStatus(event.status);
      addCompletion(event.status, reply);
      loadSessions(false);
      break;
    }
  }
}

async function loadSnapshot({ preserveEvidence = false } = {}) {
  const snapshot = await api('/api/state');
  state.snapshot = snapshot;
  state.restoredCount = snapshot.restoredCount || 0;
  state.resumed = state.restoredCount > 0;
  renderMeta(snapshot);
  renderMessages(snapshot.messages || [], state.restoredCount);
  if (!preserveEvidence) clearEvidence();
  if (snapshot.approval) {
    addApproval({ kind: 'approval', status: 'pending', id: snapshot.approval.id, text: snapshot.approval.command, detail: snapshot.approval.reason });
  } else if (!preserveEvidence && snapshot.status && snapshot.status !== 'ready' && snapshot.messages?.length) {
    const lastAssistant = [...snapshot.messages].reverse().find((message) => message.role === 'assistant');
    addCompletion(snapshot.status, lastAssistant?.content || 'This saved turn has no additional reply.');
  }
  return snapshot;
}

function formatDate(value) {
  if (!value) return 'Time unavailable';
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return value;
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

async function loadSessions(showDrawer = true) {
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

function closeSessions() {
  ui.body.classList.remove('sessions-open');
  ui.drawer.setAttribute('aria-hidden', 'true');
  ui.drawer.inert = true;
  ui.shell.inert = false;
  ui.resume.setAttribute('aria-expanded', 'false');
  (narrowViewport.matches ? ui.mobileIndex : ui.resume).focus();
}

async function startNewSession() {
  try {
    const snapshot = await api('/api/session/new', { method: 'POST' });
    state.resumed = false;
    state.restoredCount = 0;
    state.snapshot = snapshot;
    clearEvidence();
    renderMeta(snapshot);
    renderMessages([]);
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
    state.resumed = true;
    state.restoredCount = (snapshot.messages || []).length;
    state.snapshot = snapshot;
    clearEvidence();
    renderMeta(snapshot);
    renderMessages(snapshot.messages || [], state.restoredCount);
    if (snapshot.status && snapshot.status !== 'ready') {
      const lastAssistant = [...(snapshot.messages || [])].reverse().find((message) => message.role === 'assistant');
      addCompletion(snapshot.status, lastAssistant?.content || 'This saved turn has no additional reply.');
    }
    closeSessions();
    ui.input.focus();
    showToast('Session resumed.');
  } catch (error) {
    showToast(error.message, true);
  }
}

function closeSessionsIfOpen() {
  if (ui.body.classList.contains('sessions-open')) closeSessions();
}

function closeMobileIndex() {
  ui.body.classList.remove('mobile-index-open');
  ui.mobileIndex.setAttribute('aria-expanded', 'false');
  syncMobileIndexInert();
}

const narrowViewport = window.matchMedia('(max-width: 820px)');
function syncMobileIndexInert() {
  ui.sessionIndex.inert = narrowViewport.matches && !ui.body.classList.contains('mobile-index-open');
}

async function submitTask(event) {
  event.preventDefault();
  const text = ui.input.value.trim();
  if (!text || ui.send.disabled) return;
  ui.input.value = '';
  ui.input.style.height = '';
  appendMessage('user', text);
  state.currentAssistant = null;
  setRunStatus('working');
  const sessionID = state.snapshot?.sessionId || 'ACTIVE SESSION';
  ui.stamp.textContent = state.resumed ? `RESUMED · ${sessionID}` : sessionID;
  try {
    await api('/api/message', { method: 'POST', body: JSON.stringify({ text }) });
    await loadSnapshot({ preserveEvidence: true });
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
  if (event.key === 'Tab' && ui.body.classList.contains('sessions-open')) {
    const focusable = [...ui.drawer.querySelectorAll('button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])')];
    if (!focusable.length) return;
    const first = focusable[0];
    const last = focusable[focusable.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first.focus();
    }
  }
});

const events = new EventSource('/api/events');
events.onopen = () => setConnection('online', 'Live');
events.onerror = () => setConnection('offline', 'Reconnecting');
events.onmessage = ({ data }) => {
  try { handleEvent(JSON.parse(data)); }
  catch (error) { showToast(`Event could not be read: ${error.message}`, true); }
};

loadSnapshot()
  .then(() => {
    setConnection('online', 'Live');
  })
  .catch((error) => {
    setConnection('offline', 'Unavailable');
    showToast(error.message, true);
  });

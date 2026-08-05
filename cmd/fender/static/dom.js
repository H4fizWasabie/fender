const $ = (selector) => document.querySelector(selector);

export const ui = {
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

export const state = {
  snapshot: null,
  currentAssistant: null,
  currentThinking: null,
  evidenceTotal: 0,
  toastTimer: null,
  resumed: false,
  restoredCount: 0,
};

export function icon(name) {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svg.setAttribute('aria-hidden', 'true');
  const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
  use.setAttribute('href', `#icon-${name}`);
  svg.appendChild(use);
  return svg;
}

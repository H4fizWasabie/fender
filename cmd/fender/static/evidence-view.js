import { humanize, readableStatus } from './api.js';
import { icon, state, ui } from './dom.js';

function revealEvidence() {
  if (!ui.evidence.hidden) return;
  ui.evidence.hidden = false;
  ui.body.classList.add('has-evidence');
}

export function clearEvidence() {
  ui.evidenceList.replaceChildren();
  ui.evidence.hidden = true;
  ui.body.classList.remove('has-evidence');
  state.evidenceTotal = 0;
  state.currentThinking = null;
  ui.evidenceCount.textContent = '0';
}

function appendDetail(slip, kind, detail) {
  if (!detail) return;
  if (kind !== 'tool') {
    const copy = document.createElement('p');
    copy.className = 'slip-detail';
    copy.textContent = detail;
    slip.appendChild(copy);
    return;
  }
  const disclosure = document.createElement('details');
  disclosure.className = 'slip-disclosure';
  const summary = document.createElement('summary');
  summary.textContent = 'Inspect result';
  const output = document.createElement('pre');
  output.textContent = detail;
  disclosure.append(summary, output);
  slip.appendChild(disclosure);
}

export function addSlip({ id, kind = 'tool', title, status = '', detail = '', source = '', command = '' }) {
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
  appendDetail(slip, kind, detail);
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

export function addCompletion(status, reply) {
  const slip = addSlip({ kind: 'done', title: status === 'complete' ? 'Handoff ready' : `Run ${readableStatus(status).toLowerCase()}`, status, detail: reply || 'The runtime ended this turn without an additional reply.' });
  slip.classList.add('completion-slip');
}

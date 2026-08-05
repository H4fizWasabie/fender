import { api } from './api.js';
import { showToast } from './docket.js';

const drawer = document.getElementById('settingsDrawer');
const body = document.getElementById('settingsBody');
let providers = [];

export function openSettings() {
  document.body.classList.add('settings-open');
  drawer.setAttribute('aria-hidden', 'false');
  renderSettings();
}
export function closeSettings() {
  document.body.classList.remove('settings-open');
  drawer.setAttribute('aria-hidden', 'true');
}

function providerRow(p) {
  return `
    <div class="setting-provider" data-name="${p.name}">
      <div class="setting-row">
        <label>Provider name</label>
        <input class="p-name" value="${p.name}" placeholder="e.g. zen">
      </div>
      <div class="setting-row">
        <label>Base URL</label>
        <input class="p-base" value="${p.base_url || ''}" placeholder="https://opencode.ai/zen">
      </div>
      <div class="setting-row">
        <label>API path <span class="muted">(default /v1; OpenRouter needs /api/v1)</span></label>
        <input class="p-path" value="${p.path || ''}" placeholder="/v1">
      </div>
      <div class="setting-row">
        <label>API key <span class="muted">${p.api_key ? 'current: ' + p.api_key : ''}</span></label>
        <input class="p-key" type="password" placeholder="${p.key_hint ? 'blank = keep existing key' : 'sk-…'}">
      </div>
      <div class="setting-row">
        <label>Models (comma separated)</label>
        <input class="p-models" value="${(p.models || []).join(', ')}" placeholder="deepseek-v4-flash-free">
      </div>
      <div class="setting-row">
        <label>Default model</label>
        <input class="p-default" value="${p.default_model || ''}" placeholder="deepseek-v4-flash-free">
      </div>
      <label class="setting-check"><input type="checkbox" class="p-thinking" ${p.thinking ? 'checked' : ''}> reasoning model (thinking levels low/medium/high)</label>
      <button class="quiet-action p-remove" type="button">remove provider</button>
    </div>`;
}

async function renderSettings() {
  body.innerHTML = '<p class="settings-loading">Loading…</p>';
  try {
    const s = await api('/api/settings');
    providers = s.providers || [];
    body.innerHTML = `
      <div class="setting-row">
        <label>Guardrail mode</label>
        <select id="setMode">
          <option value="strict" ${s.mode === 'strict' ? 'selected' : ''}>strict — ask before every tool call</option>
          <option value="balanced" ${!s.mode || s.mode === 'balanced' ? 'selected' : ''}>balanced — ask on risky, REFUSE hard</option>
          <option value="yolo" ${s.mode === 'yolo' ? 'selected' : ''}>yolo — no questions, guardrail stays</option>
        </select>
      </div>
      <div class="setting-row">
        <label>Fallback provider (second key, used when the main provider fails)</label>
        <select id="setFallback">
          <option value="">none</option>
          ${providers.map(p => `<option value="${p.name}" ${s.fallback === p.name ? 'selected' : ''}>${p.name}</option>`).join('')}
        </select>
      </div>
      <h3>Providers</h3>
      <div id="providerList">${providers.map(providerRow).join('')}</div>
      <button class="quiet-action" id="addProvider" type="button">+ add provider</button>`;
  } catch (err) {
    body.innerHTML = `<p class="settings-error">Failed to load settings: ${err.message}</p>`;
  }
}

document.getElementById('settingsGear').addEventListener('click', openSettings);
document.getElementById('closeSettings').addEventListener('click', closeSettings);
document.getElementById('settingsScrim').addEventListener('click', closeSettings);
document.addEventListener('keydown', (event) => {
  if (event.key === 'Escape') closeSettings();
});

body.addEventListener('click', (e) => {
  if (e.target.id === 'addProvider') {
    const div = document.createElement('div');
    div.className = 'setting-provider';
    div.dataset.name = '';
    div.innerHTML = providerRow({ name: '', base_url: '', path: '', api_key: '', key_hint: '', models: [], default_model: '', thinking: false });
    document.getElementById('providerList').appendChild(div);
  }
  if (e.target.classList.contains('p-remove')) {
    e.target.closest('.setting-provider').remove();
  }
});

document.getElementById('saveSettings').addEventListener('click', async () => {
  const mode = document.getElementById('setMode').value;
  const fallback = document.getElementById('setFallback').value;
  const list = [...document.querySelectorAll('.setting-provider')].map((el) => ({
    name: el.querySelector('.p-name').value.trim(),
    base_url: el.querySelector('.p-base').value.trim(),
    path: el.querySelector('.p-path').value.trim(),
    api_key: el.querySelector('.p-key').value,
    key_hint: '',
    models: el.querySelector('.p-models').value.split(',').map((s) => s.trim()).filter(Boolean),
    default_model: el.querySelector('.p-default').value.trim(),
    thinking: el.querySelector('.p-thinking').checked,
  })).filter((p) => p.name);
  try {
    const res = await api('/api/settings', {
      method: 'POST',
      body: JSON.stringify({ mode, fallback, providers: list }),
    });
    showToast(`Settings saved (${res.path}); agent rebuilt.`);
    closeSettings();
  } catch (err) {
    showToast(`Save failed: ${err.message}`, true);
  }
});

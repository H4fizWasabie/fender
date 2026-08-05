export async function api(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    signal: options.signal,
    headers: {
      ...(options.body ? { 'Content-Type': 'application/json' } : {}),
      ...(options.headers || {}),
    },
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `${response.status} ${response.statusText}`);
  return data;
}

export function humanize(value) {
  return String(value || '').replaceAll('_', ' ').replace(/\b\w/g, (letter) => letter.toUpperCase());
}

export function readableStatus(status) {
  const labels = {
    ready: 'Ready', working: 'Working', interrupted: 'Interrupted', complete: 'Complete', blocked: 'Blocked',
    stalled: 'Stalled', error: 'Error', cancelled: 'Cancelled',
  };
  return labels[status] || status || 'Ready';
}

export function formatDate(value) {
  if (!value) return 'Time unavailable';
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) return value;
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(date);
}

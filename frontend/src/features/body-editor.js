import { escapeHtml } from '../lib/dom.js';

const FORM_DATA_BOUNDARY = '----gpFormBoundary7MA4YWxkTrZu0gW';

export function setupBodyEditor() {
  const bodyTypeSelect = document.getElementById('body-type');
  const bodyRawField = document.getElementById('body-raw-field');
  const bodyTextarea = document.getElementById('body');
  const kvField = document.getElementById('body-kv-field');
  const kvTableBody = document.getElementById('body-kv-table-body');
  const kvAddBtn = document.getElementById('body-kv-add-btn');
  const headersField = document.getElementById('headers');
  if (!bodyTypeSelect || !bodyTextarea || !kvTableBody) return;

  const formatBtn = document.getElementById('body-format-btn');
  const jsonWarningEl = document.getElementById('body-json-warning');

  // Key/value rows shared between x-www-form-urlencoded and form-data, kept
  // as an array of { enabled, key, value }. Rebuilt into #body whenever the
  // table changes or the selected body type changes between the two.
  let kvRows = [];

  // Tracks the exact Content-Type value we last auto-set into #headers, so
  // switching away from urlencoded/form-data can remove it again without
  // clobbering a value the user hand-edited in the meantime.
  let lastAutoContentType = null;

  function findContentTypeIndex(lines) {
    for (let i = 0; i < lines.length; i++) {
      const idx = lines[i].indexOf(':');
      if (idx === -1) continue;
      if (lines[i].slice(0, idx).trim().toLowerCase() === 'content-type') return i;
    }
    return -1;
  }

  function setAutoContentType(value) {
    let lines = headersField.value.split('\n');
    if (lines.length === 1 && lines[0] === '') lines = [];
    const idx = findContentTypeIndex(lines);
    if (idx !== -1) lines.splice(idx, 1);
    lines.push(`Content-Type: ${value}`);
    headersField.value = lines.join('\n');
    lastAutoContentType = value;
  }

  function removeAutoContentTypeIfUnchanged() {
    if (lastAutoContentType === null) return;
    const lines = headersField.value.split('\n');
    const idx = findContentTypeIndex(lines);
    if (idx === -1) {
      lastAutoContentType = null;
      return;
    }
    const value = lines[idx].slice(lines[idx].indexOf(':') + 1).trim();
    if (value === lastAutoContentType) {
      lines.splice(idx, 1);
      headersField.value = lines.join('\n');
    }
    lastAutoContentType = null;
  }

  function buildUrlEncodedBody(rows) {
    return rows.filter((r) => r.enabled && r.key !== '')
      .map((r) => `${encodeURIComponent(r.key)}=${encodeURIComponent(r.value)}`)
      .join('&');
  }

  function buildFormDataBody(rows) {
    let out = '';
    rows.filter((r) => r.enabled && r.key !== '').forEach((r) => {
      out += `--${FORM_DATA_BOUNDARY}\r\n`;
      out += `Content-Disposition: form-data; name="${r.key.replace(/"/g, '\\"')}"\r\n\r\n`;
      out += `${r.value}\r\n`;
    });
    out += `--${FORM_DATA_BOUNDARY}--\r\n`;
    return out;
  }

  // Re-applies the Content-Type header on every call (type switch AND table
  // edit), relying on setAutoContentType's strip-before-insert behavior to
  // avoid ever accumulating duplicate Content-Type lines.
  function syncKvToBody() {
    const type = bodyTypeSelect.value;
    if (type === 'urlencoded') {
      setAutoContentType('application/x-www-form-urlencoded');
      bodyTextarea.value = buildUrlEncodedBody(kvRows);
    } else if (type === 'form-data') {
      setAutoContentType(`multipart/form-data; boundary=${FORM_DATA_BOUNDARY}`);
      bodyTextarea.value = buildFormDataBody(kvRows);
    }
  }

  function renderKvTable() {
    let html = '';
    kvRows.forEach((row, i) => {
      html += `<tr data-index="${i}">` +
        `<td class="body-kv-col-check"><input type="checkbox" class="body-kv-enabled"${row.enabled ? ' checked' : ''}></td>` +
        `<td><input type="text" class="body-kv-key" value="${escapeHtml(row.key)}"></td>` +
        `<td><input type="text" class="body-kv-value" value="${escapeHtml(row.value)}"></td>` +
        '<td class="body-kv-col-del"><button type="button" class="icon-btn body-kv-del" aria-label="削除"><span class="icon">delete</span></button></td>' +
        '</tr>';
    });
    kvTableBody.innerHTML = html;
  }

  function applyBodyType() {
    const type = bodyTypeSelect.value;
    jsonWarningEl.innerHTML = '';
    if (type === 'none') {
      removeAutoContentTypeIfUnchanged();
      bodyRawField.hidden = true;
      kvField.hidden = true;
      bodyTextarea.value = '';
      bodyTextarea.readOnly = true;
    } else if (type === 'raw') {
      removeAutoContentTypeIfUnchanged();
      bodyRawField.hidden = false;
      kvField.hidden = true;
      bodyTextarea.readOnly = false;
    } else if (type === 'urlencoded') {
      bodyTextarea.readOnly = false;
      bodyRawField.hidden = true;
      kvField.hidden = false;
      syncKvToBody();
    } else if (type === 'form-data') {
      bodyTextarea.readOnly = false;
      bodyRawField.hidden = true;
      kvField.hidden = false;
      syncKvToBody();
    }
  }

  if (formatBtn) {
    formatBtn.addEventListener('click', () => {
      jsonWarningEl.innerHTML = '';
      try {
        const obj = JSON.parse(bodyTextarea.value);
        bodyTextarea.value = JSON.stringify(obj, null, 2);
      } catch (e) {
        jsonWarningEl.innerHTML = `<div class="warn-box"><span class="icon">warning</span>不正なJSONです: ${
          escapeHtml(e && e.message ? e.message : String(e))
        }</div>`;
      }
    });
  }

  kvTableBody.addEventListener('input', (e) => {
    const tr = e.target.closest('tr');
    if (!tr) return;
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !kvRows[i]) return;
    if (e.target.classList.contains('body-kv-key')) kvRows[i].key = e.target.value;
    if (e.target.classList.contains('body-kv-value')) kvRows[i].value = e.target.value;
    syncKvToBody();
  });

  kvTableBody.addEventListener('change', (e) => {
    const tr = e.target.closest('tr');
    if (!tr) return;
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !kvRows[i]) return;
    if (e.target.classList.contains('body-kv-enabled')) {
      kvRows[i].enabled = e.target.checked;
      syncKvToBody();
    }
  });

  kvTableBody.addEventListener('click', (e) => {
    const delBtn = e.target.closest('.body-kv-del');
    if (!delBtn) return;
    const tr = delBtn.closest('tr');
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !kvRows[i]) return;
    kvRows.splice(i, 1);
    renderKvTable();
    syncKvToBody();
  });

  kvAddBtn.addEventListener('click', () => {
    kvRows.push({ enabled: true, key: '', value: '' });
    renderKvTable();
  });

  bodyTypeSelect.addEventListener('change', applyBodyType);

  applyBodyType();
}

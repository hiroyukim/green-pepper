import { escapeHtml } from '../lib/dom.js';
import { splitUrl, parseQuery, buildQuery } from '../lib/url-params.js';

export function setupParamsTable() {
  const urlField = document.getElementById('url');
  const tableBody = document.getElementById('params-table-body');
  const addBtn = document.getElementById('params-add-btn');
  if (!urlField || !tableBody || !addBtn) return;

  // Params are tracked as an array of { enabled, key, value } rows, kept in
  // sync with the URL field's query string (the part after the first "?").
  let params = [];
  let syncing = false; // guards against update loops between table <-> field

  function parseUrlIntoParams() {
    const parts = splitUrl(urlField.value);
    params = parseQuery(parts.query);
    renderTable();
  }

  function applyParamsToUrl() {
    const parts = splitUrl(urlField.value);
    const query = buildQuery(params);
    syncing = true;
    urlField.value = query ? `${parts.base}?${query}` : parts.base;
    syncing = false;
  }

  function renderTable() {
    let html = '';
    params.forEach((row, i) => {
      html += `<tr data-index="${i}">` +
        `<td class="params-col-check"><input type="checkbox" class="params-enabled"${row.enabled ? ' checked' : ''}></td>` +
        `<td><input type="text" class="params-key" value="${escapeHtml(row.key)}"></td>` +
        `<td><input type="text" class="params-value" value="${escapeHtml(row.value)}"></td>` +
        '<td class="params-col-del"><button type="button" class="icon-btn params-del" aria-label="削除"><span class="icon">delete</span></button></td>' +
        '</tr>';
    });
    tableBody.innerHTML = html;
  }

  tableBody.addEventListener('input', (e) => {
    const tr = e.target.closest('tr');
    if (!tr) return;
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !params[i]) return;
    if (e.target.classList.contains('params-key')) params[i].key = e.target.value;
    if (e.target.classList.contains('params-value')) params[i].value = e.target.value;
    applyParamsToUrl();
  });

  tableBody.addEventListener('change', (e) => {
    const tr = e.target.closest('tr');
    if (!tr) return;
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !params[i]) return;
    if (e.target.classList.contains('params-enabled')) {
      params[i].enabled = e.target.checked;
      applyParamsToUrl();
    }
  });

  tableBody.addEventListener('click', (e) => {
    const delBtn = e.target.closest('.params-del');
    if (!delBtn) return;
    const tr = delBtn.closest('tr');
    const i = Number(tr.dataset.index);
    if (Number.isNaN(i) || !params[i]) return;
    params.splice(i, 1);
    renderTable();
    applyParamsToUrl();
  });

  addBtn.addEventListener('click', () => {
    params.push({ enabled: true, key: '', value: '' });
    renderTable();
  });

  urlField.addEventListener('input', () => {
    if (syncing) return;
    parseUrlIntoParams();
  });

  parseUrlIntoParams();
}

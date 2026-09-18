import { escapeHtml } from '../lib/dom.js';

// Naive preview parser: good enough to show columns/row count/first rows
// before submitting. The server (internal/model.ParseCSV) is authoritative.
function parseCSVPreview(text) {
  const lines = text.split(/\r\n|\n/).filter((l) => l.length > 0);
  if (lines.length === 0) return null;
  const splitLine = (l) => l.split(',').map((c) => c.trim().replace(/^"|"$/g, ''));
  return { header: splitLine(lines[0]), rows: lines.slice(1, 6).map(splitLine), total: lines.length - 1 };
}

export function setupCsvDropzone() {
  const form = document.getElementById('request-form');
  const dropzone = document.getElementById('dropzone');
  if (!form || !dropzone) return;

  const usedVars = (form.dataset.usedVars || '').split(',').filter(Boolean);
  const fileInput = document.getElementById('csv');
  const filenameEl = document.getElementById('filename');
  const previewEl = document.getElementById('csv-preview');
  const warningEl = document.getElementById('csv-warning');
  const envField = document.getElementById('env');

  function envKeys() {
    return envField.value.split('\n')
      .map((l) => l.split('=')[0].trim())
      .filter(Boolean);
  }

  function renderPreview(file) {
    if (!file) {
      filenameEl.textContent = '';
      previewEl.innerHTML = '';
      warningEl.innerHTML = '';
      return;
    }
    filenameEl.textContent = `${file.name} (${(file.size / 1024).toFixed(1)} KB)`;

    const reader = new FileReader();
    reader.onload = () => {
      const parsed = parseCSVPreview(String(reader.result));
      if (!parsed) {
        previewEl.innerHTML = '';
        warningEl.innerHTML = '';
        return;
      }

      let html = `<table><thead><tr>${
        parsed.header.map((h) => `<th>${escapeHtml(h)}</th>`).join('')
      }</tr></thead><tbody>`;
      parsed.rows.forEach((row) => {
        html += `<tr>${row.map((c) => `<td>${escapeHtml(c)}</td>`).join('')}</tr>`;
      });
      html += `</tbody></table><p class="meta">${parsed.total}行 / 列: ${escapeHtml(parsed.header.join(', '))}</p>`;
      previewEl.innerHTML = html;

      const covered = new Set(parsed.header.concat(envKeys()));
      const missing = usedVars.filter((v) => !covered.has(v));
      warningEl.innerHTML = missing.length
        ? `<div class="warn-box"><span class="icon">warning</span>CSV・環境変数のどちらにも無い変数があります: ${escapeHtml(missing.join(', '))}</div>`
        : '';
    };
    reader.readAsText(file);
  }

  dropzone.addEventListener('click', () => fileInput.click());
  dropzone.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); fileInput.click(); }
  });
  fileInput.addEventListener('change', () => renderPreview(fileInput.files[0]));

  ['dragenter', 'dragover'].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => { e.preventDefault(); dropzone.classList.add('dragover'); });
  });
  ['dragleave', 'drop'].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => { e.preventDefault(); dropzone.classList.remove('dragover'); });
  });
  dropzone.addEventListener('drop', (e) => {
    const file = e.dataTransfer.files[0];
    if (file) {
      fileInput.files = e.dataTransfer.files;
      renderPreview(file);
    }
  });
}

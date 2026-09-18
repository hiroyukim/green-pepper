import { parseCurlCommand, parseHeadersTextarea, buildCurlCommand } from '../lib/curl.js';

export function setupCurlTools() {
  setupImport();
  setupCopy();
}

function setupImport() {
  const importInput = document.getElementById('curl-import-input');
  const importBtn = document.getElementById('curl-import-btn');
  const importError = document.getElementById('curl-import-error');
  if (!importBtn) return;

  importBtn.addEventListener('click', () => {
    importError.innerHTML = '';
    try {
      const parsed = parseCurlCommand(importInput.value);
      document.getElementById('method').value = parsed.method;
      document.getElementById('url').value = parsed.url;
      document.getElementById('headers').value = parsed.headers.join('\n');
      document.getElementById('body').value = parsed.body;
    } catch (e) {
      importError.innerHTML = `<div class="error-box"><span class="icon">error</span>${
        e && e.message ? e.message : 'cURLコマンドを解析できませんでした。'
      }</div>`;
    }
  });
}

function setupCopy() {
  const copyBtn = document.getElementById('curl-copy-btn');
  const copyFallback = document.getElementById('curl-copy-fallback');
  if (!copyBtn) return;

  function showCopyFallback(command) {
    copyFallback.innerHTML = '';
    const label = document.createElement('p');
    label.className = 'meta';
    label.textContent = 'クリップボードにコピーできませんでした。以下のコマンドを手動でコピーしてください。';
    const box = document.createElement('textarea');
    box.readOnly = true;
    box.rows = 3;
    box.value = command;
    copyFallback.appendChild(label);
    copyFallback.appendChild(box);
    box.focus();
    box.select();
  }

  copyBtn.addEventListener('click', () => {
    const command = buildCurlCommand({
      method: document.getElementById('method').value,
      url: document.getElementById('url').value,
      headers: parseHeadersTextarea(document.getElementById('headers').value),
      body: document.getElementById('body').value,
    });
    const originalLabel = copyBtn.innerHTML;
    const restore = () => {
      setTimeout(() => { copyBtn.innerHTML = originalLabel; }, 1500);
    };

    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(command).then(() => {
        copyFallback.innerHTML = '';
        copyBtn.innerHTML = '<span class="icon">check</span>コピーしました';
        restore();
      }).catch(() => {
        showCopyFallback(command);
      });
    } else {
      showCopyFallback(command);
    }
  });
}

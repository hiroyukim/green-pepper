import { highlightJson } from '../lib/json-highlight.js';

export function setupResponseViewer() {
  const body = document.getElementById('response-body');
  if (!body) return; // response block only renders after a "send" action

  // The browser already un-escapes HTML entities when reading textContent,
  // so this is the real original response body text.
  const rawText = body.textContent;

  // Try to pretty-print + highlight; both stay null if the body isn't valid
  // JSON, and callers fall back to raw text without throwing.
  let prettyText = null;
  let prettyHtml = null;
  try {
    prettyText = JSON.stringify(JSON.parse(rawText), null, 2);
    prettyHtml = highlightJson(prettyText);
  } catch (e) {
    prettyText = null;
    prettyHtml = null;
  }

  function contentTypeHeader() {
    const rows = document.querySelectorAll('.response table tr');
    for (const row of rows) {
      const cells = row.querySelectorAll('td');
      if (cells.length >= 2 && cells[0].textContent.trim().toLowerCase() === 'content-type') {
        return cells[1].textContent.toLowerCase();
      }
    }
    return '';
  }

  const toolbar = document.getElementById('response-toolbar');
  const tabPretty = document.getElementById('response-tab-pretty');
  const tabRaw = document.getElementById('response-tab-raw');
  const copyBtn = document.getElementById('response-copy-btn');
  const downloadBtn = document.getElementById('response-download-btn');
  let currentView = 'raw';

  function setView(view) {
    if (view === 'pretty' && prettyText === null) view = 'raw';
    currentView = view;
    if (view === 'pretty') {
      body.innerHTML = prettyHtml;
      if (tabPretty) tabPretty.classList.remove('secondary');
      if (tabRaw) tabRaw.classList.add('secondary');
    } else {
      body.textContent = rawText;
      if (tabRaw) tabRaw.classList.remove('secondary');
      if (tabPretty) tabPretty.classList.add('secondary');
    }
  }

  if (tabPretty && tabRaw) {
    if (prettyText === null) {
      tabPretty.disabled = true;
      tabPretty.title = 'JSONとして解釈できませんでした';
    }
    tabPretty.addEventListener('click', () => setView('pretty'));
    tabRaw.addEventListener('click', () => setView('raw'));
  }

  const isJsonContentType = contentTypeHeader().includes('json');
  setView(isJsonContentType && prettyText !== null ? 'pretty' : 'raw');

  function currentText() {
    return currentView === 'pretty' && prettyText !== null ? prettyText : rawText;
  }

  function showCopyFallback(text) {
    const existing = document.getElementById('response-copy-fallback');
    if (existing) existing.remove();
    const box = document.createElement('textarea');
    box.id = 'response-copy-fallback';
    box.readOnly = true;
    box.rows = 3;
    box.style.width = '100%';
    box.style.marginTop = '0.5rem';
    box.value = text;
    toolbar.parentNode.insertBefore(box, body);
    box.focus();
    box.select();
  }

  if (copyBtn) {
    copyBtn.addEventListener('click', () => {
      const text = currentText();
      const originalLabel = copyBtn.innerHTML;
      const restore = () => {
        setTimeout(() => { copyBtn.innerHTML = originalLabel; }, 1500);
      };
      if (navigator.clipboard && navigator.clipboard.writeText) {
        navigator.clipboard.writeText(text).then(() => {
          const fallback = document.getElementById('response-copy-fallback');
          if (fallback) fallback.remove();
          copyBtn.innerHTML = '<span class="icon">check</span>コピーしました';
          restore();
        }).catch(() => {
          showCopyFallback(text);
        });
      } else {
        showCopyFallback(text);
      }
    });
  }

  if (downloadBtn) {
    downloadBtn.addEventListener('click', () => {
      const text = currentText();
      const isJson = currentView === 'pretty' && prettyText !== null;
      const blob = new Blob([text], { type: isJson ? 'application/json' : 'text/plain' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = isJson ? 'response.json' : 'response.txt';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      setTimeout(() => URL.revokeObjectURL(url), 0);
    });
  }
}

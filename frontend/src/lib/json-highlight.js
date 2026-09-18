import { escapeHtml } from './dom.js';

// Regex-based JSON tokenizer/highlighter (no external library). Matches
// strings (optionally followed by a colon, treated as an object key),
// booleans, null, and numbers; everything else passes through escaped.
export function highlightJson(json) {
  const pattern = /("(\\u[a-fA-F0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\btrue\b|\bfalse\b|\bnull\b|-?\d+(?:\.\d+)?(?:[eE][+-]?\d+)?)/g;
  let out = '';
  let lastIndex = 0;
  let match;
  while ((match = pattern.exec(json)) !== null) {
    out += escapeHtml(json.slice(lastIndex, match.index));
    const token = match[0];
    let cls;
    if (/^"/.test(token)) {
      cls = /:\s*$/.test(token) ? 'json-key' : 'json-string';
    } else if (token === 'true' || token === 'false') {
      cls = 'json-bool';
    } else if (token === 'null') {
      cls = 'json-null';
    } else {
      cls = 'json-number';
    }
    out += `<span class="${cls}">${escapeHtml(token)}</span>`;
    lastIndex = pattern.lastIndex;
  }
  out += escapeHtml(json.slice(lastIndex));
  return out;
}

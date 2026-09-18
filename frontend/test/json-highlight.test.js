import { test } from 'node:test';
import assert from 'node:assert/strict';
import { highlightJson } from '../src/lib/json-highlight.js';

test('highlightJson wraps keys, strings, numbers, booleans, and null', () => {
  const html = highlightJson('{\n  "a": 1,\n  "b": "x",\n  "c": true,\n  "d": null\n}');
  assert.match(html, /<span class="json-key">&quot;a&quot;:<\/span>/);
  assert.match(html, /<span class="json-number">1<\/span>/);
  assert.match(html, /<span class="json-string">&quot;x&quot;<\/span>/);
  assert.match(html, /<span class="json-bool">true<\/span>/);
  assert.match(html, /<span class="json-null">null<\/span>/);
});

test('highlightJson escapes HTML found outside of tokens', () => {
  const html = highlightJson('{"a": "<b>"}');
  assert.match(html, /&lt;b&gt;/);
  assert.doesNotMatch(html, /<b>/);
});

import { test } from 'node:test';
import assert from 'node:assert/strict';
import { stripHeaderLines, appendHeaderLine, stripQueryParam, applyAuth } from '../src/lib/auth-headers.js';

test('stripHeaderLines removes matching keys case-insensitively', () => {
  const result = stripHeaderLines('Authorization: Bearer x\nAccept: json', ['authorization']);
  assert.equal(result, 'Accept: json');
});

test('appendHeaderLine appends without doubling newlines', () => {
  assert.equal(appendHeaderLine('', 'A: 1'), 'A: 1');
  assert.equal(appendHeaderLine('A: 1\n', 'B: 2'), 'A: 1\nB: 2');
});

test('stripQueryParam removes only the matching key', () => {
  assert.equal(stripQueryParam('a=1&key=x&b=2', 'key'), 'a=1&b=2');
  assert.equal(stripQueryParam('a=1', null), 'a=1');
});

test('applyAuth injects a Basic auth header', () => {
  const result = applyAuth({
    type: 'basic',
    headersText: '',
    urlText: 'https://example.com',
    currentKeyName: '',
    lastApiKeyHeaderName: null,
    lastApiKeyQueryName: null,
    basicUsername: 'alice',
    basicPassword: 'secret',
  });
  assert.equal(result.headersText, `Authorization: Basic ${Buffer.from('alice:secret').toString('base64')}`);
  assert.equal(result.urlText, 'https://example.com');
});

test('applyAuth injects an API key into the query string and remembers the name', () => {
  const result = applyAuth({
    type: 'apikey',
    headersText: '',
    urlText: 'https://example.com?existing=1',
    currentKeyName: 'api_key',
    lastApiKeyHeaderName: null,
    lastApiKeyQueryName: null,
    apikeyValue: 'abc123',
    apikeyLocation: 'query',
  });
  assert.equal(result.urlText, 'https://example.com?existing=1&api_key=abc123');
  assert.equal(result.lastApiKeyQueryName, 'api_key');
});

test('applyAuth cleans up a previously injected API key query param on rename', () => {
  const result = applyAuth({
    type: 'apikey',
    headersText: '',
    urlText: 'https://example.com?old_key=abc123',
    currentKeyName: 'new_key',
    lastApiKeyHeaderName: null,
    lastApiKeyQueryName: 'old_key',
    apikeyValue: 'abc123',
    apikeyLocation: 'query',
  });
  assert.equal(result.urlText, 'https://example.com?new_key=abc123');
});

test('applyAuth strips any auth remnants when switching to none', () => {
  const result = applyAuth({
    type: 'none',
    headersText: 'Authorization: Bearer x\nAccept: json',
    urlText: 'https://example.com',
    currentKeyName: '',
    lastApiKeyHeaderName: null,
    lastApiKeyQueryName: null,
  });
  assert.equal(result.headersText, 'Accept: json');
});

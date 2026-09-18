import { test } from 'node:test';
import assert from 'node:assert/strict';
import { parseCurlCommand, parseHeadersTextarea, buildCurlCommand, shellQuote } from '../src/lib/curl.js';

test('parseCurlCommand extracts method/url/headers/body', () => {
  const parsed = parseCurlCommand(
    "curl -X POST 'https://api.example.com/users' -H 'Content-Type: application/json' -d '{\"name\":\"Alice\"}'"
  );
  assert.equal(parsed.method, 'POST');
  assert.equal(parsed.url, 'https://api.example.com/users');
  assert.deepEqual(parsed.headers, ['Content-Type: application/json']);
  assert.equal(parsed.body, '{"name":"Alice"}');
});

test('parseCurlCommand defaults to GET without -d, POST with it', () => {
  assert.equal(parseCurlCommand('curl https://example.com').method, 'GET');
  assert.equal(parseCurlCommand("curl https://example.com -d 'a=1'").method, 'POST');
});

test('parseCurlCommand skips value-consuming flags it does not support', () => {
  const parsed = parseCurlCommand("curl -u user:pass --max-time 5 'https://example.com'");
  assert.equal(parsed.url, 'https://example.com');
});

test('parseCurlCommand throws without a URL', () => {
  assert.throws(() => parseCurlCommand('curl -X POST'), /URL/);
});

test('parseCurlCommand throws on empty input', () => {
  assert.throws(() => parseCurlCommand('   '), /コマンドを入力/);
});

test('parseHeadersTextarea skips blanks and comments, keeps first colon as separator', () => {
  const headers = parseHeadersTextarea('# comment\n\nAuthorization: Bearer abc:def\nX-Empty:\n');
  assert.deepEqual(headers, [
    { name: 'Authorization', value: 'Bearer abc:def' },
    { name: 'X-Empty', value: '' },
  ]);
});

test('buildCurlCommand round-trips a simple GET with headers', () => {
  const cmd = buildCurlCommand({
    method: 'GET',
    url: 'https://example.com',
    headers: [{ name: 'Accept', value: 'application/json' }],
    body: '',
  });
  assert.equal(cmd, `curl ${shellQuote('https://example.com')} -H ${shellQuote('Accept: application/json')}`);
});

test('buildCurlCommand includes -X for non-GET and --data for a body', () => {
  const cmd = buildCurlCommand({ method: 'post', url: 'https://example.com', headers: [], body: 'a=1' });
  assert.equal(cmd, `curl -X ${shellQuote('post')} ${shellQuote('https://example.com')} --data ${shellQuote('a=1')}`);
});

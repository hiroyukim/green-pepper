import { test } from 'node:test';
import assert from 'node:assert/strict';
import { splitUrl, parseQuery, buildQuery } from '../src/lib/url-params.js';

test('splitUrl separates base and query on the first "?"', () => {
  assert.deepEqual(splitUrl('https://example.com/a?x=1'), { base: 'https://example.com/a', query: 'x=1' });
  assert.deepEqual(splitUrl('https://example.com/a'), { base: 'https://example.com/a', query: '' });
});

test('splitUrl tolerates a templated origin', () => {
  assert.deepEqual(splitUrl('{{base_url}}/a?x=1'), { base: '{{base_url}}/a', query: 'x=1' });
});

test('parseQuery parses key=value pairs and bare keys', () => {
  assert.deepEqual(parseQuery('a=1&b=&c'), [
    { enabled: true, key: 'a', value: '1' },
    { enabled: true, key: 'b', value: '' },
    { enabled: true, key: 'c', value: '' },
  ]);
  assert.deepEqual(parseQuery(''), []);
});

test('buildQuery drops disabled/empty-key rows', () => {
  const query = buildQuery([
    { enabled: true, key: 'a', value: '1' },
    { enabled: false, key: 'b', value: '2' },
    { enabled: true, key: '', value: '3' },
  ]);
  assert.equal(query, 'a=1');
});

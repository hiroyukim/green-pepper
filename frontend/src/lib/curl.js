// cURL <-> request-form conversion, used by the "cURLからインポート" and
// "cURLとしてコピー" features. Parsing uses a minimal shell-like tokenizer
// (handles single/double-quoted arguments plus unquoted whitespace-separated
// words) rather than full shell semantics, which is enough for the curl
// invocations this feature targets.

export function shellQuote(value) {
  return `'${String(value).replace(/'/g, "'\\''")}'`;
}

export function tokenizeCurlCommand(input) {
  const tokens = [];
  let i = 0;
  const n = input.length;
  while (i < n) {
    while (i < n && /\s/.test(input[i])) i++;
    if (i >= n) break;
    let token = '';
    while (i < n && !/\s/.test(input[i])) {
      const c = input[i];
      if (c === "'") {
        i++;
        while (i < n && input[i] !== "'") {
          if (input[i] === '\\' && input[i + 1] === "'") {
            token += "'";
            i += 2;
          } else {
            token += input[i];
            i++;
          }
        }
        i++;
      } else if (c === '"') {
        i++;
        while (i < n && input[i] !== '"') {
          if (input[i] === '\\' && (input[i + 1] === '"' || input[i + 1] === '\\')) {
            token += input[i + 1];
            i += 2;
          } else {
            token += input[i];
            i++;
          }
        }
        i++;
      } else if (c === '\\' && i + 1 < n) {
        token += input[i + 1];
        i += 2;
      } else {
        token += c;
        i++;
      }
    }
    tokens.push(token);
  }
  return tokens;
}

// Flags known to consume a following value that we don't otherwise support;
// skipped together with their value so we don't misdetect the value as URL.
const CURL_VALUE_FLAGS = [
  '-u', '--user', '-b', '--cookie', '-e', '--referer', '-A', '--user-agent',
  '-o', '--output', '--connect-timeout', '--max-time', '-w', '--write-out',
  '--cacert', '--cert', '--key', '-x', '--proxy', '--limit-rate', '--retry',
];

export function parseCurlCommand(text) {
  if (!text || !text.trim()) {
    throw new Error('コマンドを入力してください。');
  }
  const normalized = text.trim().replace(/\\\r?\n/g, ' ');
  const tokens = tokenizeCurlCommand(normalized);
  if (tokens.length === 0) {
    throw new Error('コマンドを解析できませんでした。');
  }

  let idx = tokens[0] === 'curl' ? 1 : 0;
  let method = null;
  let url = null;
  const headers = [];
  const dataParts = [];

  while (idx < tokens.length) {
    const t = tokens[idx];
    if (t === '-X' || t === '--request') {
      method = tokens[idx + 1] || method;
      idx += 2;
    } else if (t === '-H' || t === '--header') {
      if (tokens[idx + 1] !== undefined) headers.push(tokens[idx + 1]);
      idx += 2;
    } else if (t === '-d' || t === '--data' || t === '--data-raw' || t === '--data-binary' || t === '--data-ascii' || t === '--data-urlencode') {
      if (tokens[idx + 1] !== undefined) dataParts.push(tokens[idx + 1]);
      idx += 2;
    } else if (t === '--url') {
      url = tokens[idx + 1] || url;
      idx += 2;
    } else if (CURL_VALUE_FLAGS.includes(t)) {
      idx += 2;
    } else if (t.charAt(0) === '-' && t.length > 1) {
      idx += 1;
    } else {
      if (url === null) url = t;
      idx += 1;
    }
  }

  if (!url) {
    throw new Error('URLが見つかりませんでした。');
  }

  return {
    method: method || (dataParts.length ? 'POST' : 'GET'),
    url,
    headers,
    body: dataParts.join('&'),
  };
}

// Mirrors linesToMap in internal/server/server.go: "Key: Value" per line,
// blank lines and "#"-comments skipped, first separator wins.
export function parseHeadersTextarea(text) {
  const result = [];
  text.split('\n').forEach((line) => {
    const trimmed = line.trim();
    if (!trimmed || trimmed.charAt(0) === '#') return;
    const sepIdx = trimmed.indexOf(':');
    if (sepIdx === -1) return;
    const name = trimmed.slice(0, sepIdx).trim();
    const value = trimmed.slice(sepIdx + 1).trim();
    if (!name) return;
    result.push({ name, value });
  });
  return result;
}

export function buildCurlCommand({ method, url, headers, body }) {
  const normalizedMethod = (method || 'GET').trim() || 'GET';
  const parts = ['curl'];
  if (normalizedMethod.toUpperCase() !== 'GET') {
    parts.push('-X', shellQuote(normalizedMethod));
  }
  parts.push(shellQuote(url.trim()));
  headers.forEach((h) => {
    parts.push('-H', shellQuote(`${h.name}: ${h.value}`));
  });
  if (body) {
    parts.push('--data', shellQuote(body));
  }
  return parts.join(' ');
}

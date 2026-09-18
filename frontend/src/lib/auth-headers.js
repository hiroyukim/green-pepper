// Pure text-manipulation helpers behind the Authorization panel: folding a
// selected auth type into the #headers textarea / #url query string, and
// cleaning up whatever a previous submit injected there.

// Removes any header line whose key (case-insensitive, split on the first
// ":") matches one of the given names.
export function stripHeaderLines(text, names) {
  const lowerNames = names.filter(Boolean).map((n) => n.toLowerCase());
  if (lowerNames.length === 0) return text;
  return text.split('\n').filter((line) => {
    const idx = line.indexOf(':');
    if (idx === -1) return true;
    const key = line.slice(0, idx).trim().toLowerCase();
    return !lowerNames.includes(key);
  }).join('\n');
}

export function appendHeaderLine(text, line) {
  const trimmed = text.replace(/\n+$/, '');
  return trimmed === '' ? line : `${trimmed}\n${line}`;
}

// Removes any "key=value" pair from a raw (already-split-off) query string
// whose key matches name, case-insensitively.
export function stripQueryParam(query, name) {
  if (!name) return query;
  const lowerName = name.toLowerCase();
  return query.split('&').filter((part) => {
    if (part === '') return false;
    const eq = part.indexOf('=');
    const key = (eq === -1 ? part : part.slice(0, eq)).toLowerCase();
    return key !== lowerName;
  }).join('&');
}

// Folds the current Authorization selection into headersText/urlText,
// replacing whatever a previous call injected there (lastApiKeyHeaderName/
// lastApiKeyQueryName carry that state across calls). Pure so it can be unit
// tested without a DOM/form.
export function applyAuth({
  type, headersText, urlText, currentKeyName,
  lastApiKeyHeaderName, lastApiKeyQueryName,
  basicUsername, basicPassword, bearerToken, apikeyValue, apikeyLocation,
}) {
  let headers = stripHeaderLines(headersText, ['authorization', currentKeyName, lastApiKeyHeaderName]);

  const qIdx = urlText.indexOf('?');
  const base = qIdx === -1 ? urlText : urlText.slice(0, qIdx);
  let query = qIdx === -1 ? '' : urlText.slice(qIdx + 1);
  query = stripQueryParam(query, currentKeyName);
  query = stripQueryParam(query, lastApiKeyQueryName);

  let nextApiKeyHeaderName = null;
  let nextApiKeyQueryName = null;

  if (type === 'basic') {
    const user = basicUsername || '';
    const pass = basicPassword || '';
    headers = appendHeaderLine(headers, `Authorization: Basic ${btoa(`${user}:${pass}`)}`);
  } else if (type === 'bearer') {
    headers = appendHeaderLine(headers, `Authorization: Bearer ${bearerToken || ''}`);
  } else if (type === 'apikey' && currentKeyName) {
    const keyValue = apikeyValue || '';
    if (apikeyLocation === 'query') {
      query = query ? `${query}&${currentKeyName}=${keyValue}` : `${currentKeyName}=${keyValue}`;
      nextApiKeyQueryName = currentKeyName;
    } else {
      headers = appendHeaderLine(headers, `${currentKeyName}: ${keyValue}`);
      nextApiKeyHeaderName = currentKeyName;
    }
  }

  return {
    headersText: headers,
    urlText: query ? `${base}?${query}` : base,
    lastApiKeyHeaderName: nextApiKeyHeaderName,
    lastApiKeyQueryName: nextApiKeyQueryName,
  };
}

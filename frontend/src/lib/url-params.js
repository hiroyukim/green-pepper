// Query-string helpers shared by the Query Params table and the cURL
// import/export tools. Deliberately manual string handling (not URL/
// URLSearchParams) since the URL field may hold double-curly-brace template
// placeholders (e.g. a "base_url" var) and a templated origin.

export function splitUrl(raw) {
  const idx = raw.indexOf('?');
  if (idx === -1) return { base: raw, query: '' };
  return { base: raw.slice(0, idx), query: raw.slice(idx + 1) };
}

export function parseQuery(query) {
  if (!query) return [];
  return query.split('&').filter((part) => part.length > 0).map((part) => {
    const eq = part.indexOf('=');
    if (eq === -1) return { enabled: true, key: part, value: '' };
    return { enabled: true, key: part.slice(0, eq), value: part.slice(eq + 1) };
  });
}

export function buildQuery(rows) {
  return rows.filter((row) => row.enabled && row.key !== '')
    .map((row) => `${row.key}=${row.value}`)
    .join('&');
}

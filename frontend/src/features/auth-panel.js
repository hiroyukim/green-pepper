import { applyAuth } from '../lib/auth-headers.js';

// Authorization section: lets the user pick No Auth / Basic Auth / Bearer
// Token / API Key and have the corresponding header (or query param, for
// API Key) generated automatically instead of hand-writing it into the
// Headers textarea. State lives only in these DOM inputs for the lifetime
// of the page; nothing is persisted server-side.
export function setupAuthPanel() {
  const form = document.getElementById('request-form');
  const headersField = document.getElementById('headers');
  const urlField = document.getElementById('url');
  const authType = document.getElementById('auth-type');
  if (!form || !headersField || !urlField || !authType) return;

  const basicFields = document.getElementById('auth-basic-fields');
  const bearerFields = document.getElementById('auth-bearer-fields');
  const apikeyFields = document.getElementById('auth-apikey-fields');
  const basicUsername = document.getElementById('auth-basic-username');
  const basicPassword = document.getElementById('auth-basic-password');
  const bearerToken = document.getElementById('auth-bearer-token');
  const apikeyKey = document.getElementById('auth-apikey-key');
  const apikeyValue = document.getElementById('auth-apikey-value');
  const apikeyLocation = document.getElementById('auth-apikey-location');

  // Remembers the API key name most recently injected into the headers
  // textarea / URL query string so that renaming the key (while staying on
  // API Key auth) still cleans up the previous line/param on the next
  // submit, not just whatever name is currently typed in.
  let lastApiKeyHeaderName = null;
  let lastApiKeyQueryName = null;

  function updateAuthVisibility() {
    basicFields.hidden = authType.value !== 'basic';
    bearerFields.hidden = authType.value !== 'bearer';
    apikeyFields.hidden = authType.value !== 'apikey';
  }

  authType.addEventListener('change', updateAuthVisibility);
  updateAuthVisibility();

  // Runs on every form submit (send / run / download alike, since they all
  // post the same #headers/#url fields): folds the current Authorization
  // selection into #headers and #url, replacing whatever this same function
  // injected on a previous submit.
  function applyAuthToRequest() {
    const result = applyAuth({
      type: authType.value,
      headersText: headersField.value,
      urlText: urlField.value,
      currentKeyName: apikeyKey.value.trim(),
      lastApiKeyHeaderName,
      lastApiKeyQueryName,
      basicUsername: basicUsername.value,
      basicPassword: basicPassword.value,
      bearerToken: bearerToken.value,
      apikeyValue: apikeyValue.value,
      apikeyLocation: apikeyLocation.value,
    });
    headersField.value = result.headersText;
    urlField.value = result.urlText;
    lastApiKeyHeaderName = result.lastApiKeyHeaderName;
    lastApiKeyQueryName = result.lastApiKeyQueryName;
  }

  form.addEventListener('submit', applyAuthToRequest);
}

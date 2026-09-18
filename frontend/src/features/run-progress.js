import { escapeHtml } from '../lib/dom.js';

export function setupRunProgress() {
  const card = document.getElementById('progress-card');
  if (!card) return;

  const bar = document.getElementById('run-progress');
  const status = document.getElementById('run-progress-status');
  const errorBox = document.getElementById('run-progress-error');
  const runId = card.getAttribute('data-run-id');

  function poll() {
    fetch(`/run-progress/${encodeURIComponent(runId)}`)
      .then((res) => res.json())
      .then((data) => {
        const total = data.total || 0;
        const completed = data.completed || 0;
        bar.max = total > 0 ? total : 1;
        bar.value = completed;
        status.textContent = `${completed} / ${total} 件完了`;

        if (data.done) {
          if (data.error) {
            errorBox.innerHTML = `<div class="error-box"><span class="icon">error</span>${escapeHtml(data.error)}</div>`;
            return;
          }
          window.location.href = `/csv-history/${data.historyId}`;
          return;
        }

        setTimeout(poll, 400);
      })
      .catch(() => {
        // Transient network hiccup: keep polling rather than giving up.
        setTimeout(poll, 400);
      });
  }

  poll();
}

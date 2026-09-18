export function setupResultsDownload() {
  const downloadBtn = document.getElementById('results-download-btn');
  const dataEl = document.getElementById('results-json');
  if (!downloadBtn || !dataEl) return;

  downloadBtn.addEventListener('click', () => {
    const text = dataEl.textContent;
    const blob = new Blob([text], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'results.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    setTimeout(() => URL.revokeObjectURL(url), 0);
  });
}

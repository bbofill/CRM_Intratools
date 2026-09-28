export function measurePageLoadTime({ targetId = null, precision = 2, onComplete = null } = {}) {
    const start = performance.now();
    window.addEventListener('load', () => {
      const end = performance.now();
      const loadSeconds = ((end - start) / 1000).toFixed(precision);
      const message = `Load Time: ${loadSeconds}s`;
      if (targetId) {
        const target = document.getElementById(targetId);
        if (target) target.textContent = message;
      }
      if (typeof onComplete === 'function') {
        onComplete(loadSeconds);
      }
    });
  }
  
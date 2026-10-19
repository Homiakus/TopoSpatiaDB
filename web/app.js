// TopoSpatiaDB — Prometheus Fuels Style Interactivity & KaTeX Auto-Renderer

document.addEventListener('DOMContentLoaded', () => {
  // 1. Render all LaTeX formulas automatically using KaTeX
  if (typeof renderMathInElement === 'function') {
    renderMathInElement(document.body, {
      delimiters: [
        { left: '$$', right: '$$', display: true },
        { left: '$', right: '$', display: false },
        { left: '\\(', right: '\\)', display: false },
        { left: '\\[', right: '\\]', display: true }
      ],
      throwOnError: false,
      ignoredTags: ['script', 'noscript', 'style', 'textarea', 'pre', 'code']
    });
  }

  // 2. Tab switching for Quickstart code showcase
  const tabBtns = document.querySelectorAll('.pf-tab-btn');
  const viewports = document.querySelectorAll('.pf-code-body');

  tabBtns.forEach(btn => {
    btn.addEventListener('click', () => {
      const targetTab = btn.getAttribute('data-tab');

      tabBtns.forEach(b => b.classList.remove('active'));
      viewports.forEach(v => v.classList.add('hidden'));

      btn.classList.add('active');
      const targetViewport = document.getElementById(targetTab);
      if (targetViewport) {
        targetViewport.classList.remove('hidden');
      }
    });
  });
});

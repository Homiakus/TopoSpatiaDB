// TopoSpatiaDB — Prometheus Fuels Style Interactivity, Zero-Radius Button Glow & KaTeX Auto-Renderer

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

  // 3. Dynamic Cursor Torch Effect (Subtle red ambient illumination)
  const torchLight = document.getElementById('pfTorchLight');
  if (torchLight && window.matchMedia('(pointer: fine)').matches) {
    let mouseX = window.innerWidth / 2;
    let mouseY = window.innerHeight / 2;
    let curX = mouseX;
    let curY = mouseY;

    window.addEventListener('mousemove', (e) => {
      mouseX = e.clientX;
      mouseY = e.clientY;
    });

    function updateTorch() {
      curX += (mouseX - curX) * 0.15;
      curY += (mouseY - curY) * 0.15;
      torchLight.style.left = `${curX}px`;
      torchLight.style.top = `${curY}px`;
      requestAnimationFrame(updateTorch);
    }
    updateTorch();
  }

  // 4. Reading Progress Bar (Top laser line)
  const scrollIndicator = document.getElementById('pfScrollIndicator');
  if (scrollIndicator) {
    window.addEventListener('scroll', () => {
      const scrollTop = window.scrollY || document.documentElement.scrollTop;
      const docHeight = document.documentElement.scrollHeight - window.innerHeight;
      const progress = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
      scrollIndicator.style.width = `${progress}%`;
    }, { passive: true });
  }

  // 5. Micro 3D Tilt Effect on Interactive Zero-Radius Cards
  const cards = document.querySelectorAll('.pf-card-interactive');
  if (window.matchMedia('(pointer: fine)').matches) {
    cards.forEach(card => {
      card.addEventListener('mousemove', (e) => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        const centerX = rect.width / 2;
        const centerY = rect.height / 2;
        const rotateX = ((y - centerY) / centerY) * -4;
        const rotateY = ((x - centerX) / centerX) * 4;

        card.style.transform = `perspective(800px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) translateY(-3px)`;
      });

      card.addEventListener('mouseleave', () => {
        card.style.transform = '';
      });
    });
  }

  // 6. Interactive Button Magnet & Click Wave Pulse
  const buttons = document.querySelectorAll('.pf-btn');
  buttons.forEach(btn => {
    btn.addEventListener('click', (e) => {
      const rect = btn.getBoundingClientRect();
      const circle = document.createElement('span');
      const size = Math.max(rect.width, rect.height) * 2;
      circle.style.width = circle.style.height = `${size}px`;
      circle.style.left = `${e.clientX - rect.left - size / 2}px`;
      circle.style.top = `${e.clientY - rect.top - size / 2}px`;
      circle.style.position = 'absolute';
      circle.style.borderRadius = '50%';
      circle.style.background = 'rgba(255, 255, 255, 0.3)';
      circle.style.transform = 'scale(0)';
      circle.style.animation = 'btnPulse 0.5s ease-out forwards';
      circle.style.pointerEvents = 'none';

      btn.appendChild(circle);
      setTimeout(() => circle.remove(), 550);
    });
  });
});

// Inject keyframe for button pulse
const style = document.createElement('style');
style.textContent = `
  @keyframes btnPulse {
    to {
      transform: scale(1);
      opacity: 0;
    }
  }
`;
document.head.appendChild(style);

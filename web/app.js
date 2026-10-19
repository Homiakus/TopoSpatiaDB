// TopoSpatiaDB — High Performance Responsive Engine: Ultra-Crisp Lenis & Hardware-Accelerated GSAP

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

  // 2. High-Performance, Instant-Response Lenis Setup
  let lenis;
  if (typeof Lenis !== 'undefined') {
    lenis = new Lenis({
      duration: 0.8, // Shorter duration = instant crisp feel without floaty lag
      easing: (t) => 1 - Math.pow(1 - t, 3), // Fast-out cubic curve (direct responsive feedback)
      smoothWheel: true,
      syncTouch: false, // Keep native 1:1 responsive touch on mobile/trackpad
      wheelMultiplier: 1.05
    });

    // Directly bind Lenis to requestAnimationFrame for 120Hz/144Hz high-refresh displays
    function raf(time) {
      lenis.raf(time);
      requestAnimationFrame(raf);
    }
    requestAnimationFrame(raf);

    // Sync GSAP ScrollTrigger without ticker lag
    if (typeof ScrollTrigger !== 'undefined') {
      lenis.on('scroll', ScrollTrigger.update);
    }

    // Connect in-page anchor links with immediate scroll
    document.querySelectorAll('a[href^="#"]').forEach((anchor) => {
      anchor.addEventListener('click', function (e) {
        const href = this.getAttribute('href');
        if (href && href !== '#' && document.querySelector(href)) {
          e.preventDefault();
          lenis.scrollTo(href, { offset: -70, duration: 0.9 });
        }
      });
    });
  }

  // 3. Lightweight Hardware-Accelerated GSAP Scroll Trigger Animations (GPU Translated)
  if (typeof gsap !== 'undefined' && typeof ScrollTrigger !== 'undefined') {
    gsap.registerPlugin(ScrollTrigger);

    // Hero Entrance Sequence (One-time, lightweight)
    gsap.from('.pf-badge-row', { opacity: 0, y: -15, duration: 0.6, delay: 0.05, ease: 'power2.out' });
    gsap.from('.pf-hero-title', { opacity: 0, y: 25, duration: 0.7, delay: 0.15, ease: 'power2.out' });
    gsap.from('.pf-hero-lead', { opacity: 0, y: 15, duration: 0.6, delay: 0.3, ease: 'power2.out' });
    gsap.from('.pf-hero-cta .pf-btn', { opacity: 0, y: 15, stagger: 0.1, duration: 0.5, delay: 0.4, ease: 'power2.out' });
    gsap.from('.pf-kpi-bar', { opacity: 0, y: 20, duration: 0.6, delay: 0.5, ease: 'power2.out' });

    // Batch Scroll Trigger (Batching avoids hundreds of listeners, maximizing scroll FPS)
    ScrollTrigger.batch('.pf-card-interactive, .pf-section-title', {
      start: 'top 88%',
      once: true, // Fire once and detach listener to keep scroll completely free
      onEnter: (batch) => {
        gsap.fromTo(
          batch,
          { opacity: 0, y: 28 },
          { opacity: 1, y: 0, stagger: 0.1, duration: 0.6, ease: 'power2.out', clearProps: 'transform' }
        );
      }
    });
  }

  // 4. Tab switching for Quickstart code showcase
  const tabBtns = document.querySelectorAll('.pf-tab-btn');
  const viewports = document.querySelectorAll('.pf-code-body');

  tabBtns.forEach((btn) => {
    btn.addEventListener('click', () => {
      const targetTab = btn.getAttribute('data-tab');

      tabBtns.forEach((b) => b.classList.remove('active'));
      viewports.forEach((v) => v.classList.add('hidden'));

      btn.classList.add('active');
      const targetViewport = document.getElementById(targetTab);
      if (targetViewport) {
        targetViewport.classList.remove('hidden');
        if (typeof ScrollTrigger !== 'undefined') {
          ScrollTrigger.refresh();
        }
      }
    });
  });

  // 5. Throttled GPU Torch Light with will-change
  const torchLight = document.getElementById('pfTorchLight');
  if (torchLight && window.matchMedia('(pointer: fine)').matches) {
    let mouseX = window.innerWidth / 2;
    let mouseY = window.innerHeight / 2;
    let isMoving = false;

    window.addEventListener('mousemove', (e) => {
      mouseX = e.clientX;
      mouseY = e.clientY;
      if (!isMoving) {
        isMoving = true;
        requestAnimationFrame(() => {
          torchLight.style.transform = `translate3d(${mouseX}px, ${mouseY}px, 0) translate(-50%, -50%)`;
          isMoving = false;
        });
      }
    }, { passive: true });
  }

  // 6. Native Passive Scroll Progress Indicator
  const scrollIndicator = document.getElementById('pfScrollIndicator');
  if (scrollIndicator) {
    let scrollTicking = false;
    window.addEventListener('scroll', () => {
      if (!scrollTicking) {
        scrollTicking = true;
        requestAnimationFrame(() => {
          const scrollTop = window.scrollY || document.documentElement.scrollTop;
          const docHeight = document.documentElement.scrollHeight - window.innerHeight;
          const progress = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
          scrollIndicator.style.width = `${progress}%`;
          scrollTicking = false;
        });
      }
    }, { passive: true });
  }

  // 7. Interactive Button Click Pulse Effect
  const buttons = document.querySelectorAll('.pf-btn');
  buttons.forEach((btn) => {
    btn.addEventListener('click', (e) => {
      const rect = btn.getBoundingClientRect();
      const circle = document.createElement('span');
      const size = Math.max(rect.width, rect.height) * 2;
      circle.style.width = circle.style.height = `${size}px`;
      circle.style.left = `${e.clientX - rect.left - size / 2}px`;
      circle.style.top = `${e.clientY - rect.top - size / 2}px`;
      circle.style.position = 'absolute';
      circle.style.borderRadius = '50%';
      circle.style.background = 'rgba(255, 255, 255, 0.25)';
      circle.style.transform = 'scale(0)';
      circle.style.animation = 'btnPulse 0.4s ease-out forwards';
      circle.style.pointerEvents = 'none';

      btn.appendChild(circle);
      setTimeout(() => circle.remove(), 450);
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

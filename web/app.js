// TopoSpatiaDB — Cinematic Experience: Lenis Smooth Scroll, GSAP ScrollTrigger & KaTeX Auto-Renderer

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

  // 2. Lenis Ultra-Smooth Inertial Scroll
  let lenis;
  if (typeof Lenis !== 'undefined') {
    lenis = new Lenis({
      duration: 1.2,
      easing: (t) => Math.min(1, 1.001 - Math.pow(2, -10 * t)), // Exponential deceleration curve
      smoothWheel: true,
      wheelMultiplier: 1.0,
      touchMultiplier: 1.5
    });

    // Synchronize Lenis with GSAP ScrollTrigger
    if (typeof gsap !== 'undefined' && typeof ScrollTrigger !== 'undefined') {
      gsap.registerPlugin(ScrollTrigger);

      lenis.on('scroll', ScrollTrigger.update);

      gsap.ticker.add((time) => {
        lenis.raf(time * 1000);
      });
      gsap.ticker.lagSmoothing(0);
    } else {
      function raf(time) {
        lenis.raf(time);
        requestAnimationFrame(raf);
      }
      requestAnimationFrame(raf);
    }

    // Connect in-page anchor links with Lenis smooth scroll
    document.querySelectorAll('a[href^="#"]').forEach((anchor) => {
      anchor.addEventListener('click', function (e) {
        const href = this.getAttribute('href');
        if (href && href !== '#' && document.querySelector(href)) {
          e.preventDefault();
          lenis.scrollTo(href, { offset: -60, duration: 1.4 });
        }
      });
    });
  }

  // 3. GSAP ScrollTrigger Entrance Animations
  if (typeof gsap !== 'undefined' && typeof ScrollTrigger !== 'undefined') {
    // Hero Entrance Sequence
    const heroTl = gsap.timeline({ defaults: { ease: 'power3.out' } });
    heroTl
      .from('.pf-badge-row', { opacity: 0, y: -20, duration: 0.8, delay: 0.1 })
      .from('.pf-hero-title', { opacity: 0, y: 35, duration: 1.0 }, '-=0.5')
      .from('.pf-hero-lead', { opacity: 0, y: 25, duration: 0.9 }, '-=0.6')
      .from('.pf-hero-cta .pf-btn', { opacity: 0, y: 20, stagger: 0.15, duration: 0.8 }, '-=0.5')
      .from('.pf-kpi-bar', { opacity: 0, y: 30, scale: 0.98, duration: 0.9 }, '-=0.4');

    // Section Titles Reveal
    gsap.utils.toArray('.pf-section-title').forEach((title) => {
      gsap.from(title, {
        scrollTrigger: {
          trigger: title,
          start: 'top 85%',
          toggleActions: 'play none none none'
        },
        opacity: 0,
        y: 40,
        duration: 0.9,
        ease: 'power3.out'
      });
    });

    // Simplicial Cards Stagger Reveal
    gsap.from('.pf-simplex-card', {
      scrollTrigger: {
        trigger: '.pf-simplex-grid',
        start: 'top 80%',
        toggleActions: 'play none none none'
      },
      opacity: 0,
      y: 45,
      stagger: 0.18,
      duration: 0.9,
      ease: 'power3.out'
    });

    // Math Cards Stagger Reveal
    gsap.from('.pf-math-card', {
      scrollTrigger: {
        trigger: '.pf-math-grid',
        start: 'top 80%',
        toggleActions: 'play none none none'
      },
      opacity: 0,
      y: 50,
      stagger: 0.2,
      duration: 1.0,
      ease: 'power3.out'
    });

    // Architecture Cards Reveal
    gsap.from('.pf-arch-card', {
      scrollTrigger: {
        trigger: '.pf-arch-grid',
        start: 'top 80%',
        toggleActions: 'play none none none'
      },
      opacity: 0,
      y: 40,
      stagger: 0.15,
      duration: 0.85,
      ease: 'power3.out'
    });

    // Applications Cards Reveal
    gsap.from('.pf-app-card', {
      scrollTrigger: {
        trigger: '.pf-app-grid',
        start: 'top 80%',
        toggleActions: 'play none none none'
      },
      opacity: 0,
      y: 45,
      stagger: 0.18,
      duration: 0.9,
      ease: 'power3.out'
    });

    // Benchmarks Table Reveal
    gsap.from('.pf-table-wrapper', {
      scrollTrigger: {
        trigger: '.pf-table-wrapper',
        start: 'top 85%',
        toggleActions: 'play none none none'
      },
      opacity: 0,
      y: 40,
      duration: 0.9,
      ease: 'power3.out'
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

  // 5. Dynamic Cursor Torch Effect (Subtle red ambient illumination)
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

  // 6. Reading Progress Bar (Top laser line)
  const scrollIndicator = document.getElementById('pfScrollIndicator');
  if (scrollIndicator) {
    window.addEventListener('scroll', () => {
      const scrollTop = window.scrollY || document.documentElement.scrollTop;
      const docHeight = document.documentElement.scrollHeight - window.innerHeight;
      const progress = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
      scrollIndicator.style.width = `${progress}%`;
    }, { passive: true });
  }

  // 7. Micro 3D Tilt Effect on Interactive Zero-Radius Cards
  const cards = document.querySelectorAll('.pf-card-interactive');
  if (window.matchMedia('(pointer: fine)').matches) {
    cards.forEach((card) => {
      card.addEventListener('mousemove', (e) => {
        const rect = card.getBoundingClientRect();
        const x = e.clientX - rect.left;
        const y = e.clientY - rect.top;
        const centerX = rect.width / 2;
        const centerY = rect.height / 2;
        const rotateX = ((y - centerY) / centerY) * -3.5;
        const rotateY = ((x - centerX) / centerX) * 3.5;

        card.style.transform = `perspective(900px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) translateY(-3px)`;
      });

      card.addEventListener('mouseleave', () => {
        card.style.transform = '';
      });
    });
  }

  // 8. Interactive Button Magnet & Click Wave Pulse
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

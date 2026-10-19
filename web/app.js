// TopoSpatiaDB — High Performance Responsive Engine: Ultra-Crisp Lenis & Hardware-Accelerated GSAP

function initKaTeX() {
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
    return true;
  }
  return false;
}

// Attempt immediate render, and poll briefly if defer is still loading CDN scripts
if (!initKaTeX()) {
  let katexAttempts = 0;
  const katexInterval = setInterval(() => {
    katexAttempts++;
    if (initKaTeX() || katexAttempts > 30) {
      clearInterval(katexInterval);
    }
  }, 100);
}
window.addEventListener('load', initKaTeX);

document.addEventListener('DOMContentLoaded', () => {
  initKaTeX();

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

  // 6. Native Passive Scroll Progress Indicator & 3-Cube Topological Unfolding
  const scrollIndicator = document.getElementById('pfScrollIndicator');
  const cubeSvg = document.getElementById('pfCubeNetSvg');

  // 6. Hypercube (Tesseract 4D) Multi-Projection Engine with Scroll and Interactive Rotation
  const scrollIndicator = document.getElementById('pfScrollIndicator');
  const cubeSvg = document.getElementById('pfCubeNetSvg');

  // 16 Vertices of a 4-Cube (Tesseract) in [-1, 1]^4
  const tesseractVertices = [];
  for (let i = 0; i < 16; i++) {
    tesseractVertices.push([
      (i & 1) ? 1 : -1,
      (i & 2) ? 1 : -1,
      (i & 4) ? 1 : -1,
      (i & 8) ? 1 : -1
    ]);
  }

  // 32 Edges connecting vertices that differ by exactly 1 coordinate
  const tesseractEdges = [];
  for (let i = 0; i < 16; i++) {
    for (let j = i + 1; j < 16; j++) {
      const diff = i ^ j;
      if ((diff & (diff - 1)) === 0) { // power of 2 -> 1 bit difference
        tesseractEdges.push([i, j]);
      }
    }
  }

  // 24 Square 2-Faces for subtle topological cell fills
  // A face is formed by 4 vertices with 2 common fixed coordinates and 2 variable coordinates
  const tesseractFaces = [];
  for (let c1 = 0; c1 < 4; c1++) {
    for (let c2 = c1 + 1; c2 < 4; c2++) {
      for (let s1 = -1; s1 <= 1; s1 += 2) {
        for (let s2 = -1; s2 <= 1; s2 += 2) {
          // Identify 4 vertices
          const fVerts = [];
          for (let v = 0; v < 16; v++) {
            const coords = [
              (v & 1) ? 1 : -1,
              (v & 2) ? 1 : -1,
              (v & 4) ? 1 : -1,
              (v & 8) ? 1 : -1
            ];
            if (coords[c1] === s1 && coords[c2] === s2) {
              fVerts.push(v);
            }
          }
          if (fVerts.length === 4) {
            // Sort to make a clean cyclic quad
            const v0 = fVerts[0];
            const v1 = fVerts.find((idx, i) => i > 0 && ((idx ^ v0) & ((idx ^ v0) - 1)) === 0);
            const remaining = fVerts.filter(idx => idx !== v0 && idx !== v1);
            const v3 = remaining.find(idx => ((idx ^ v0) & ((idx ^ v0) - 1)) === 0);
            const v2 = remaining.find(idx => idx !== v3);
            if (v1 !== undefined && v2 !== undefined && v3 !== undefined) {
              tesseractFaces.push([v0, v1, v2, v3]);
            }
          }
        }
      }
    }
  }

  // Projection modes according to scroll position
  const PROJECTION_MODES = [
    { name: 'SCHLEGEL CELLULAR DUAL', desc: 'Клеточная 3D проекция Шлегеля в R³' },
    { name: 'ISOMETRIC 4D ROTATION', desc: 'Изометрическая гиперсфера Клиффорда' },
    { name: 'ORTHOGONAL DOUBLE ROTATION', desc: 'Двойная ортогональная плоскость XW / YZ' },
    { name: 'STEREOGRAPHIC HOPF FIBRATION', desc: 'Стереографическое расслоение Хопфа' },
    { name: 'HODGE HARMONIC VACUUM', desc: 'Стационарный вакуум Лапласиана Ходжа Δ₀' }
  ];

  let currentRotZW = 0;
  let currentRotXW = 0;
  let currentRotXY = 0;
  let currentRotYZ = 0;

  function renderHypercube(scrollProgress) {
    if (!cubeSvg) return;

    const width = window.innerWidth;
    const height = window.innerHeight;
    cubeSvg.setAttribute('viewBox', `0 0 ${width} ${height}`);

    const isMobile = width < 768;
    const cx = isMobile ? width * 0.5 : width * 0.74;
    const cy = isMobile ? height * 0.48 : height * 0.50;
    const baseScale = Math.min(width, height) * (isMobile ? 0.22 : 0.24);

    // Continuous 4D rotation angles driven by scroll and parametric phase
    // As the user scrolls, the hypercube traverses different 4D projection planes (XW, YW, ZW, XY, YZ)
    const t = scrollProgress * Math.PI * 4; // Multiple full rotations across page
    const thetaXW = t * 0.75 + currentRotXW;
    const thetaZW = t * 1.05 + currentRotZW;
    const thetaXY = t * 0.45 + currentRotXY;
    const thetaYZ = t * 0.60 + currentRotYZ;

    // Determine current active projection mode name
    const modeIndex = Math.min(
      Math.floor(scrollProgress * PROJECTION_MODES.length),
      PROJECTION_MODES.length - 1
    );
    const activeMode = PROJECTION_MODES[modeIndex];

    // Trigonometric caches
    const cosXW = Math.cos(thetaXW), sinXW = Math.sin(thetaXW);
    const cosZW = Math.cos(thetaZW), sinZW = Math.sin(thetaZW);
    const cosXY = Math.cos(thetaXY), sinXY = Math.sin(thetaXY);
    const cosYZ = Math.cos(thetaYZ), sinYZ = Math.sin(thetaYZ);

    // Transform 16 vertices: 4D -> 4D Rotated -> 3D Perspective -> 2D Screen
    const projected2D = [];
    const projected3DZ = [];
    const projectedW = [];

    for (let i = 0; i < 16; i++) {
      let [x, y, z, w] = tesseractVertices[i];

      // 1. Rotate in X-W plane
      let rx1 = x * cosXW - w * sinXW;
      let rw1 = x * sinXW + w * cosXW;
      x = rx1; w = rw1;

      // 2. Rotate in Z-W plane
      let rz1 = z * cosZW - w * sinZW;
      let rw2 = z * sinZW + w * cosZW;
      z = rz1; w = rw2;

      // 3. Rotate in X-Y plane
      let rx2 = x * cosXY - y * sinXY;
      let ry2 = x * sinXY + y * cosXY;
      x = rx2; y = ry2;

      // 4. Rotate in Y-Z plane
      let ry3 = y * cosYZ - z * sinYZ;
      let rz2 = y * sinYZ + z * cosYZ;
      y = ry3; z = rz2;

      // 5. Perspective projection 4D -> 3D
      // Camera distance in W dimension
      const distance4D = 2.6;
      const factor4D = 1 / (distance4D - w);
      const x3 = x * factor4D;
      const y3 = y * factor4D;
      const z3 = z * factor4D;

      // 6. Perspective projection 3D -> 2D
      const distance3D = 3.2;
      const factor3D = 1 / (distance3D - z3);
      const screenX = cx + x3 * factor3D * baseScale * 4.8;
      const screenY = cy + y3 * factor3D * baseScale * 4.8;

      projected2D.push([screenX, screenY]);
      projected3DZ.push(z3);
      projectedW.push(w);
    }

    let svgHtml = '';

    // Defs for gradients & glowing filters
    svgHtml += `
      <defs>
        <radialGradient id="tesseractGlow" cx="50%" cy="50%" r="50%">
          <stop offset="0%" stop-color="#DF1A25" stop-opacity="0.16"/>
          <stop offset="100%" stop-color="#DF1A25" stop-opacity="0"/>
        </radialGradient>
      </defs>
      <circle cx="${cx.toFixed(1)}" cy="${cy.toFixed(1)}" r="${(baseScale * 1.5).toFixed(1)}" fill="url(#tesseractGlow)" />
    `;

    // Draw select 2-faces (semi-transparent topological cells)
    // Select the 8 most prominent faces according to average depth for visual elegance
    const faceDepths = tesseractFaces.map((f, idx) => {
      const avgW = (projectedW[f[0]] + projectedW[f[1]] + projectedW[f[2]] + projectedW[f[3]]) / 4;
      return { idx, avgW };
    });
    faceDepths.sort((a, b) => a.avgW - b.avgW);

    // Draw the 6 back/inner faces with subtle fill
    for (let i = 0; i < 8; i++) {
      const f = tesseractFaces[faceDepths[i].idx];
      const pts = f.map(vi => `${projected2D[vi][0].toFixed(1)},${projected2D[vi][1].toFixed(1)}`).join(' ');
      svgHtml += `<polygon points="${pts}" class="pf-tesseract-face"/>`;
    }

    // Draw 32 4D Edges (1-Simplexes) with depth-modulated styling
    for (let e = 0; e < tesseractEdges.length; e++) {
      const [u, v] = tesseractEdges[e];
      const p1 = projected2D[u];
      const p2 = projected2D[v];
      const avgW = (projectedW[u] + projectedW[v]) / 2;

      // Distinguish inner cube from outer cube in Schlegel projection via W dimension
      const isInnerEdge = avgW < -0.2;
      const isConnectingEdge = (tesseractVertices[u][3] !== tesseractVertices[v][3]);

      let edgeClass = 'pf-tesseract-edge';
      if (isConnectingEdge) edgeClass += ' connecting';
      if (isInnerEdge) edgeClass += ' inner';

      svgHtml += `<line x1="${p1[0].toFixed(1)}" y1="${p1[1].toFixed(1)}" x2="${p2[0].toFixed(1)}" y2="${p2[1].toFixed(1)}" class="${edgeClass}" />`;
    }

    // Draw 16 Vertices (0-Simplexes) with coordinate labels on key corners
    for (let i = 0; i < 16; i++) {
      const p = projected2D[i];
      const w = projectedW[i];
      const radius = w > 0 ? 3.5 : 2.2;
      const isOuter = w > 0.4;
      svgHtml += `<circle cx="${p[0].toFixed(1)}" cy="${p[1].toFixed(1)}" r="${radius.toFixed(1)}" class="pf-tesseract-node ${isOuter ? 'highlight' : ''}"/>`;
    }

    // HUD Status Box rendered directly in SVG: displays active projection, angle & dimension stats
    const hudX = isMobile ? 24 : Math.max(24, cx - baseScale * 1.5);
    const hudY = cy + baseScale * 1.35;
    svgHtml += `
      <g class="pf-tesseract-hud">
        <rect x="${hudX.toFixed(1)}" y="${hudY.toFixed(1)}" width="280" height="64" class="pf-hud-panel" />
        <line x1="${hudX.toFixed(1)}" y1="${hudY.toFixed(1)}" x2="${(hudX + 280).toFixed(1)}" y2="${hudY.toFixed(1)}" stroke="#DF1A25" stroke-width="2" />
        <text x="${(hudX + 14).toFixed(1)}" y="${(hudY + 22).toFixed(1)}" class="pf-hud-tag">4D PROJECTION &bull; TESSERACT [X,Y,Z,W]</text>
        <text x="${(hudX + 14).toFixed(1)}" y="${(hudY + 40).toFixed(1)}" class="pf-hud-title">${activeMode.name}</text>
        <text x="${(hudX + 14).toFixed(1)}" y="${(hudY + 54).toFixed(1)}" class="pf-hud-desc">${activeMode.desc}</text>
      </g>
    `;

    cubeSvg.innerHTML = svgHtml;
  }

  // Initial draw
  renderHypercube(0);

  // Resize handler
  window.addEventListener('resize', () => {
    const scrollTop = window.scrollY || document.documentElement.scrollTop;
    const docHeight = document.documentElement.scrollHeight - window.innerHeight;
    const progress = docHeight > 0 ? (scrollTop / docHeight) : 0;
    renderHypercube(progress);
  }, { passive: true });

  // Interactive Mouse/Touch Rotation
  let isDragging = false;
  let lastMouseX = 0;
  let lastMouseY = 0;

  window.addEventListener('mousedown', (e) => {
    // Only drag when clicking background or right area
    if (e.clientX > window.innerWidth * 0.4) {
      isDragging = true;
      lastMouseX = e.clientX;
      lastMouseY = e.clientY;
    }
  });

  window.addEventListener('mousemove', (e) => {
    if (isDragging) {
      const dx = e.clientX - lastMouseX;
      const dy = e.clientY - lastMouseY;
      lastMouseX = e.clientX;
      lastMouseY = e.clientY;

      currentRotXW += dx * 0.008;
      currentRotZW += dy * 0.008;

      const scrollTop = window.scrollY || document.documentElement.scrollTop;
      const docHeight = document.documentElement.scrollHeight - window.innerHeight;
      const progress = docHeight > 0 ? (scrollTop / docHeight) : 0;
      renderHypercube(progress);
    }
  });

  window.addEventListener('mouseup', () => {
    isDragging = false;
  });

  // Passive Smooth Scroll Tick
  let scrollTicking = false;
  window.addEventListener('scroll', () => {
    if (!scrollTicking) {
      scrollTicking = true;
      requestAnimationFrame(() => {
        const scrollTop = window.scrollY || document.documentElement.scrollTop;
        const docHeight = document.documentElement.scrollHeight - window.innerHeight;
        const progress = docHeight > 0 ? (scrollTop / docHeight) : 0;

        // 1. Update top laser indicator
        if (scrollIndicator) {
          scrollIndicator.style.width = `${progress * 100}%`;
        }

        // 2. Update Hypercube 4D multi-projection
        renderHypercube(progress);

        scrollTicking = false;
      });
    }
  }, { passive: true });

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

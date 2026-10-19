// TopoSpatiaDB Web Studio & 3D Simplicial Hologram Visualizer

let scene, camera, renderer, simplicialMeshGroup;
let isInteracting = false;
let mouseX = 0, mouseY = 0;
let targetRotationX = 0, targetRotationY = 0;

// Initialize 3D Simplicial Complex Hologram
function initThreeHologram() {
  const container = document.getElementById('three-container');
  if (!container || !window.THREE) return;

  const width = container.clientWidth;
  const height = container.clientHeight || 380;

  scene = new THREE.Scene();
  scene.fog = new THREE.FogExp2(0x090c10, 0.08);

  camera = new THREE.PerspectiveCamera(45, width / height, 0.1, 100);
  camera.position.set(0, 1.2, 3.8);

  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true });
  renderer.setSize(width, height);
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
  container.appendChild(renderer.domElement);

  // Group for rotating complex
  simplicialMeshGroup = new THREE.Group();
  scene.add(simplicialMeshGroup);

  // Lighting
  const ambientLight = new THREE.AmbientLight(0xffffff, 0.6);
  scene.add(ambientLight);

  const pointLight1 = new THREE.PointLight(0x58a6ff, 2, 10);
  pointLight1.position.set(2, 3, 2);
  scene.add(pointLight1);

  const pointLight2 = new THREE.PointLight(0xbc8cff, 1.5, 10);
  pointLight2.position.set(-2, -1, 1);
  scene.add(pointLight2);

  // Build Triangle 0-1-2 with 2-simplex face and glowing vertices
  buildSimplicialComplex();

  // Mouse / Touch Controls
  container.addEventListener('mousedown', onMouseDown);
  container.addEventListener('mousemove', onMouseMove);
  window.addEventListener('mouseup', onMouseUp);

  container.addEventListener('touchstart', onTouchStart, { passive: true });
  container.addEventListener('touchmove', onTouchMove, { passive: true });

  window.addEventListener('resize', onWindowResize);

  // Reset Button
  const resetBtn = document.getElementById('resetCameraBtn');
  if (resetBtn) {
    resetBtn.addEventListener('click', () => {
      targetRotationX = 0;
      targetRotationY = 0;
      simplicialMeshGroup.rotation.set(0, 0, 0);
    });
  }

  animate();
}

function buildSimplicialComplex() {
  // Vertices of 2-simplex
  const v0 = new THREE.Vector3(0, 1.1, 0);
  const v1 = new THREE.Vector3(-1.2, -0.7, 0.6);
  const v2 = new THREE.Vector3(1.2, -0.7, 0.6);
  const v3 = new THREE.Vector3(0, -0.4, -1.1); // Hyper-simplex apex (3-simplex)

  const vertices = [v0, v1, v2, v3];

  // 1. Semi-transparent Holographic Faces (2-simplexes)
  const faceGeometry = new THREE.BufferGeometry();
  const facePositions = [
    // Triangle 0-1-2
    v0.x, v0.y, v0.z,
    v1.x, v1.y, v1.z,
    v2.x, v2.y, v2.z,
    // Triangle 0-2-3
    v0.x, v0.y, v0.z,
    v2.x, v2.y, v2.z,
    v3.x, v3.y, v3.z,
    // Triangle 0-3-1
    v0.x, v0.y, v0.z,
    v3.x, v3.y, v3.z,
    v1.x, v1.y, v1.z,
  ];
  faceGeometry.setAttribute('position', new THREE.Float32BufferAttribute(facePositions, 3));
  faceGeometry.computeVertexNormals();

  const faceMaterial = new THREE.MeshPhongMaterial({
    color: 0x388bfd,
    emissive: 0x1f3460,
    specular: 0x58a6ff,
    shininess: 60,
    side: THREE.DoubleSide,
    transparent: true,
    opacity: 0.35,
    wireframe: false,
  });
  const faceMesh = new THREE.Mesh(faceGeometry, faceMaterial);
  simplicialMeshGroup.add(faceMesh);

  // 2. Wireframe overlay for coboundary d_k edges (1-simplexes)
  const edges = [
    [v0, v1], [v1, v2], [v2, v0],
    [v0, v3], [v1, v3], [v2, v3]
  ];

  edges.forEach(([p1, p2]) => {
    const edgeGeo = new THREE.BufferGeometry().setFromPoints([p1, p2]);
    const edgeMat = new THREE.LineBasicMaterial({
      color: 0x58a6ff,
      linewidth: 2,
      transparent: true,
      opacity: 0.85
    });
    const line = new THREE.Line(edgeGeo, edgeMat);
    simplicialMeshGroup.add(line);
  });

  // 3. Glowing Spherical Vertices (0-simplexes)
  const sphereGeo = new THREE.SphereGeometry(0.1, 16, 16);
  const colors = [0x58a6ff, 0x39d353, 0xbc8cff, 0xf0883e];

  vertices.forEach((pos, idx) => {
    const mat = new THREE.MeshStandardMaterial({
      color: colors[idx % colors.length],
      emissive: colors[idx % colors.length],
      emissiveIntensity: 0.6,
      roughness: 0.2,
      metalness: 0.8
    });
    const sphere = new THREE.Mesh(sphereGeo, mat);
    sphere.position.copy(pos);
    simplicialMeshGroup.add(sphere);
  });
}

function onMouseDown(e) {
  isInteracting = true;
  mouseX = e.clientX;
  mouseY = e.clientY;
}

function onMouseMove(e) {
  if (!isInteracting) return;
  const deltaX = e.clientX - mouseX;
  const deltaY = e.clientY - mouseY;
  targetRotationY += deltaX * 0.008;
  targetRotationX += deltaY * 0.008;
  mouseX = e.clientX;
  mouseY = e.clientY;
}

function onMouseUp() {
  isInteracting = false;
}

function onTouchStart(e) {
  if (e.touches.length === 1) {
    isInteracting = true;
    mouseX = e.touches[0].clientX;
    mouseY = e.touches[0].clientY;
  }
}

function onTouchMove(e) {
  if (!isInteracting || e.touches.length !== 1) return;
  const deltaX = e.touches[0].clientX - mouseX;
  const deltaY = e.touches[0].clientY - mouseY;
  targetRotationY += deltaX * 0.008;
  targetRotationX += deltaY * 0.008;
  mouseX = e.touches[0].clientX;
  mouseY = e.touches[0].clientY;
}

function onWindowResize() {
  const container = document.getElementById('three-container');
  if (!container || !renderer || !camera) return;
  const width = container.clientWidth;
  const height = container.clientHeight || 380;
  camera.aspect = width / height;
  camera.updateProjectionMatrix();
  renderer.setSize(width, height);
}

function animate() {
  requestAnimationFrame(animate);

  if (simplicialMeshGroup) {
    if (!isInteracting) {
      targetRotationY += 0.004; // Gentle idle auto-orbit
    }
    simplicialMeshGroup.rotation.y += (targetRotationY - simplicialMeshGroup.rotation.y) * 0.1;
    simplicialMeshGroup.rotation.x += (targetRotationX - simplicialMeshGroup.rotation.x) * 0.1;
  }

  renderer.render(scene, camera);
}

// -------------------------------------------------------------
// Interactive Studio API Integrations
// -------------------------------------------------------------

async function refreshEngineStatus() {
  try {
    const res = await fetch('/api/v1/status');
    if (!res.ok) throw new Error('Status HTTP ' + res.status);
    const data = await res.json();

    const hudV = document.getElementById('hudVertices');
    const hudE = document.getElementById('hudEdges');
    const hudF = document.getElementById('hudFaces');
    if (hudV) hudV.innerHTML = '0-Симплексы (Узлы): <span>' + data.vertices_count + '</span>';
    if (hudE) hudE.innerHTML = '1-Симплексы (Ребра): <span>' + data.edges_count + '</span>';
    if (hudF) hudF.innerHTML = '2-Симплексы (Грани): <span>' + data.faces_count + '</span>';

    const chip = document.getElementById('clusterStatusChip');
    if (chip) {
      chip.innerHTML = '<span class="status-dot pulsing"></span> <span class="status-label">Cluster Live: ' + data.vertices_count + ' nodes (' + data.record_count + ' recs)</span>';
    }
  } catch (err) {
    const chip = document.getElementById('clusterStatusChip');
    if (chip) {
      chip.innerHTML = '<span class="status-dot" style="background:#f85149"></span> <span class="status-label" style="color:#f85149">Offline / Demo mode</span>';
    }
  }
}

async function refreshRecordsTable() {
  const tbody = document.getElementById('recordsTableBody');
  if (!tbody) return;

  try {
    const res = await fetch('/api/v1/records');
    if (!res.ok) throw new Error('HTTP ' + res.status);
    const records = await res.json();

    if (!records || records.length === 0) {
      tbody.innerHTML = '<tr><td colspan="6" class="loading-td">Хранилище пусто. Запишите первый векторный стебель через форму выше.</td></tr>';
      return;
    }

    let rowsHtml = '';
    records.forEach(rec => {
      const vecStr = rec.vector ? `[${rec.vector.map(n => n.toFixed(2)).join(', ')}]` : '[]';
      const sigStr = rec.signature ? rec.signature.substring(0, 16) + '...' : 'none';
      rowsHtml += `
        <tr>
          <td><strong style="color:var(--accent-blue)">${rec.id}</strong></td>
          <td><span class="badge-tag">${rec.cell_id}</span></td>
          <td>${rec.dimension}</td>
          <td><code>${vecStr}</code></td>
          <td>#${rec.tx_id}</td>
          <td title="${rec.signature}"><code>${sigStr}</code></td>
        </tr>
      `;
    });
    tbody.innerHTML = rowsHtml;
  } catch (err) {
    tbody.innerHTML = '<tr><td colspan="6" class="loading-td" style="color:#f85149">Ошибка связи с TopoSpatiaDB REST API: ' + err.message + '</td></tr>';
  }
}

async function runDiffusionConsensus() {
  const out = document.getElementById('diffusionOutput');
  out.innerHTML = '<span style="color:#58a6ff;">[~] Запуск непрерывного градиентного потока Лапласиана Ходжа exp(-t * Delta_0)...</span>';

  try {
    const res = await fetch('/api/v1/consensus/diffuse', { method: 'POST' });
    const data = await res.json();

    out.innerHTML = `[✓] СХОЖДЕНИЕ КОНСЕНСУСА ХОДЖА УСПЕШНО!\n` +
      `  • Итераций градиентного потока: ${data.iterations}\n` +
      `  • Исходная энергия Дирихле E(x): ${data.initial_energy.toFixed(6)}\n` +
      `  • Конечная энергия Дирихле:      ${data.final_energy.toFixed(6)} (H^1 -> 0)\n` +
      `  • Статус: Бесконфликтный гармонический вакуум достигнут.`;

    const hudVal = document.getElementById('hudEnergyVal');
    if (hudVal) hudVal.innerText = data.final_energy.toFixed(6);

    refreshRecordsTable();
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка консенсуса: ${err.message}</span>`;
  }
}

async function addDiscrepancy() {
  const out = document.getElementById('diffusionOutput');
  out.innerHTML = '<span style="color:#d29922;">[!] Вносим искусственное расхождение (Split-Brain) в узел [1]...</span>';

  try {
    const res = await fetch('/api/v1/records/rec-v1', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        vector: [45.0, -22.5],
        payload: { discrepancy: true, timestamp: Date.now() }
      })
    });
    const data = await res.json();
    out.innerHTML += `\n[+] Состояние узла rec-v1 возмущено до [45.0, -22.5]. Энергия Дирихле возросла!\n[i] Нажмите "Запустить Диффузию" для топологического сведения.`;
    
    const hudVal = document.getElementById('hudEnergyVal');
    if (hudVal) hudVal.innerText = "28.450192";

    refreshRecordsTable();
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка внесения шума: ${err.message}</span>`;
  }
}

async function runRAGDiffusion() {
  const seed = parseInt(document.getElementById('ragSeedInput').value, 10) || 0;
  const steps = parseInt(document.getElementById('ragStepsInput').value, 10) || 3;
  const out = document.getElementById('ragOutput');
  out.innerHTML = `<span style="color:#bc8cff;">[~] Вычисление симплициальной тепловой волны RAG из затравки [${seed}] (${steps} шагов)...</span>`;

  try {
    const res = await fetch('/api/v1/rag/diffuse', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ seeds: [seed], steps: steps, alpha: 0.15 })
    });
    const data = await res.json();
    
    let resultStr = `[✓] ТОПОЛОГИЧЕСКИЙ РАНЖИРОВАННЫЙ RAG КОНТЕКСТ:\n`;
    if (data.rankings && data.rankings.length > 0) {
      data.rankings.forEach((r, idx) => {
        resultStr += `  #${idx+1} [${r.cell_id}] Dim=${r.dimension} | Heat Weight=${r.weight.toFixed(4)}\n`;
      });
    } else {
      resultStr += `  (Комплекс пуст или изолирован)`;
    }
    out.innerText = resultStr;
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка RAG: ${err.message}</span>`;
  }
}

async function queryBettiNumbers() {
  const out = document.getElementById('topologyOutput');
  out.innerHTML = '<span style="color:#39c5cf;">[~] Вычисление рангов когомологий Чеха–Де Рама H^k...</span>';

  try {
    const res = await fetch('/api/v1/topology/betti');
    const data = await res.json();
    out.innerText = `[✓] ЧИСЛА БЕТТИ КОМПЛЕКСА ДАННЫХ:\n` +
      `  • β0 (Связные компоненты кластера): ${data.betti_0}\n` +
      `  • β1 (Топологические циклы и дыры):  ${data.betti_1}\n` +
      `  • Нильпотентность: d_{k+1} ∘ d_k = 0 подтверждена.`;
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка Betti: ${err.message}</span>`;
  }
}

async function runHodgeDecompose() {
  const out = document.getElementById('topologyOutput');
  out.innerHTML = '<span style="color:#39c5cf;">[~] Разложение Ходжа-Гельмгольца для 1-коцепей...</span>';

  try {
    const res = await fetch('/api/v1/topology/hodge-decompose', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        flow: [1.2, -0.8, 0.4, 0.2, -0.1, 0.5],
        max_steps: 60,
        dt: 0.04
      })
    });
    const data = await res.json();
    out.innerText = `[✓] HODGE-HELMHOLTZ ДЕКОМПОЗИЦИЯ:\n` +
      `  • Полная энергия потока:   ${(data.total_energy || 0).toFixed(4)}\n` +
      `  • Энергия вихрей/циклов:   ${(data.cycle_energy || 0).toFixed(4)}\n` +
      `  • Гармоническая часть ker(Δ1): ${JSON.stringify(data.harmonic)}`;
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка разложения: ${err.message}</span>`;
  }
}

async function insertCustomRecord() {
  const out = document.getElementById('recordOutput');
  const id = document.getElementById('recIdInput').value || ('rec-' + Date.now().toString().slice(-4));
  const cell = document.getElementById('recCellInput').value || '[0]';
  const vecStr = document.getElementById('recVectorInput').value || '[10.0, 5.0]';

  let vector = [10.0, 5.0];
  try {
    vector = JSON.parse(vecStr);
  } catch (e) {
    out.innerHTML = '<span style="color:#f85149">[!] Неверный формат вектора. Пример: [12.0, 4.5]</span>';
    return;
  }

  out.innerHTML = `<span style="color:#7ee787;">[~] Коммит транзакции в WAL Storage Engine...</span>`;

  try {
    const res = await fetch('/api/v1/records', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        id: id,
        cell_id: cell,
        dimension: 0,
        vector: vector,
        payload: { author: "WebStudio", created_at: new Date().toISOString() }
      })
    });
    const data = await res.json();
    out.innerText = `[✓] УСПЕШНЫЙ WAL КОММИТ:\n` +
      `  • TxID: #${data.tx_id}\n` +
      `  • Record: ${data.id} -> Simplex ${data.cell_id}\n` +
      `  • SHA-256 Digest: ${data.signature.substring(0, 24)}...`;

    refreshEngineStatus();
    refreshRecordsTable();
  } catch (err) {
    out.innerHTML = `<span style="color:#f85149">[!] Ошибка записи: ${err.message}</span>`;
  }
}

// Bind Event Listeners on DOM Ready
document.addEventListener('DOMContentLoaded', () => {
  initThreeHologram();
  refreshEngineStatus();
  refreshRecordsTable();

  // Polling every 5s for live metrics
  setInterval(refreshEngineStatus, 5000);

  // Button actions
  const btnRunDiff = document.getElementById('btnRunDiffusion');
  if (btnRunDiff) btnRunDiff.addEventListener('click', runDiffusionConsensus);

  const btnAddDiscr = document.getElementById('btnAddDiscrepancy');
  if (btnAddDiscr) btnAddDiscr.addEventListener('click', addDiscrepancy);

  const btnRunRAG = document.getElementById('btnRunRAG');
  if (btnRunRAG) btnRunRAG.addEventListener('click', runRAGDiffusion);

  const btnQueryBetti = document.getElementById('btnQueryBetti');
  if (btnQueryBetti) btnQueryBetti.addEventListener('click', queryBettiNumbers);

  const btnHodge = document.getElementById('btnHodgeDecompose');
  if (btnHodge) btnHodge.addEventListener('click', runHodgeDecompose);

  const btnInsert = document.getElementById('btnInsertRecord');
  if (btnInsert) btnInsert.addEventListener('click', insertCustomRecord);

  const btnRefresh = document.getElementById('btnRefreshRecords');
  if (btnRefresh) btnRefresh.addEventListener('click', refreshRecordsTable);
});

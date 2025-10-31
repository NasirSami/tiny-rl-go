const go = new Go();
let wasmReady = false;
let currentView = 'path';
let lastSnapshot = null;
const DEFAULT_PLAYBACK_DELAY = 60;
let snapshotQueue = [];
let isAnimating = false;
let currentTrail = [];
let lastEpisodeId = 0;
let playbackDelayMs = DEFAULT_PLAYBACK_DELAY;
let currentAlgorithm = 'montecarlo';
const canvas = document.getElementById('gridCanvas');
const ctx = canvas.getContext('2d');
const statusEl = document.getElementById('status');
const metricsEl = document.getElementById('metricSummary');
const logEl = document.getElementById('eventLog');

async function loadWasm() {
  if (wasmReady) return;
  const wasmResponse = await fetch('tinyrl.wasm');
  const wasmBytes = await wasmResponse.arrayBuffer();
  const result = await WebAssembly.instantiate(wasmBytes, go.importObject);
  go.run(result.instance);
  wasmReady = true;
}

function registerHandlers() {
  window.tinyrlRegisterSnapshotHandler(handleSnapshot);
}

function handleSnapshot(snapshot) {
  console.log('[snapshot] enqueue', snapshot.status, 'episode', snapshot.episode, 'step', snapshot.step);
  snapshotQueue.push(snapshot);
  if (!isAnimating) {
    isAnimating = true;
    processSnapshotQueue();
  }
}

function processSnapshotQueue() {
  if (snapshotQueue.length === 0) {
    console.log('[queue] empty');
    isAnimating = false;
    return;
  }
  const snapshot = snapshotQueue.shift();
  console.log('[queue] process', snapshot.status, 'episode', snapshot.episode, 'step', snapshot.step);
  updateView(snapshot);
  if (snapshot.status === 'running') {
    if (snapshot.episode !== lastEpisodeId) {
      lastEpisodeId = snapshot.episode;
      currentTrail = [];
    }
    currentTrail.push({ row: snapshot.position.row, col: snapshot.position.col });
  } else if (snapshot.status === 'episode_complete') {
    currentTrail = [];
  }
  if (snapshotQueue.length > 0) {
    const delay = playbackDelayMs > 0 ? playbackDelayMs : DEFAULT_PLAYBACK_DELAY;
    console.log('[queue] schedule next in', delay, 'ms (remaining', snapshotQueue.length, ')');
    setTimeout(processSnapshotQueue, delay);
  } else {
    isAnimating = false;
  }
}

function updateView(snapshot) {
  lastSnapshot = snapshot;
  statusEl.textContent = formatStatus(snapshot);
  if (snapshot.config && typeof snapshot.config.stepDelayMs === 'number') {
    playbackDelayMs = snapshot.config.stepDelayMs;
  }
  if (snapshot.config && typeof snapshot.config.algorithm === 'string') {
    currentAlgorithm = snapshot.config.algorithm;
  }
  const delayLabel = playbackDelayMs > 0 ? `${playbackDelayMs}ms` : '0ms';
  const algoLabel = currentAlgorithm || 'montecarlo';
  metricsEl.textContent = `Episode ${snapshot.episode}/${snapshot.config.episodes} — reward ${snapshot.episodeReward.toFixed(2)} — success ${snapshot.successCount} — grid ${snapshot.config.rows}×${snapshot.config.cols} — algo ${algoLabel} — delay ${delayLabel}`;
  appendLog(snapshot);
  draw();
}

function appendLog(snapshot) {
  if (snapshot.status === 'episode_complete') {
    const entry = document.createElement('div');
    entry.textContent = `Episode ${snapshot.episode}: reward ${snapshot.episodeReward.toFixed(2)} steps ${snapshot.episodeSteps}`;
    logEl.prepend(entry);
  }
  if (snapshot.status === 'done') {
    const entry = document.createElement('div');
    entry.textContent = `Training complete. Total reward ${snapshot.totalReward.toFixed(2)} in ${snapshot.totalSteps} steps.`;
    logEl.prepend(entry);
  }
}

function formatStatus(snapshot) {
  switch (snapshot.status) {
    case 'running':
      return `Running episode ${snapshot.episode}`;
    case 'episode_complete':
      return `Episode ${snapshot.episode} complete`;
    case 'done':
      return 'Training complete';
    case 'cancelled':
      return 'Training cancelled';
    default:
      return `Status: ${snapshot.status}`;
  }
}

function draw() {
  if (!lastSnapshot) return;
  if (currentView === 'heatmap') {
    drawHeatmap(lastSnapshot.valueMap);
  } else {
    drawGrid(lastSnapshot);
  }
}

function drawGrid(snapshot) {
  const rows = snapshot.valueMap.length;
  const cols = snapshot.valueMap[0].length;
  const cellWidth = canvas.width / cols;
  const cellHeight = canvas.height / rows;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  ctx.fillStyle = '#f0f0f0';
  ctx.fillRect(0, 0, canvas.width, canvas.height);

  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      ctx.strokeStyle = '#ccc';
      ctx.strokeRect(c * cellWidth, r * cellHeight, cellWidth, cellHeight);
    }
  }
  const start = { row: rows - 1, col: 0 };
  const goal = { row: 0, col: cols - 1 };
  drawCell(start, cellWidth, cellHeight, '#0d6efd');
  drawCell(goal, cellWidth, cellHeight, '#198754');
  drawTrail(cellWidth, cellHeight);
  drawCell(snapshot.position, cellWidth, cellHeight, '#d63384');
}

function drawCell(pos, w, h, color) {
  ctx.fillStyle = color;
  ctx.fillRect(pos.col * w + 2, pos.row * h + 2, w - 4, h - 4);
}

function drawTrail(cellWidth, cellHeight) {
  if (!currentTrail.length) return;
  const total = currentTrail.length;
  currentTrail.forEach((pos, idx) => {
    const alpha = 0.2 + (idx / total) * 0.6;
    ctx.fillStyle = `rgba(13, 110, 253, ${alpha.toFixed(3)})`;
    ctx.fillRect(pos.col * cellWidth + 4, pos.row * cellHeight + 4, cellWidth - 8, cellHeight - 8);
  });
}

function drawHeatmap(valueMap) {
  const rows = valueMap.length;
  const cols = valueMap[0].length;
  const cellWidth = canvas.width / cols;
  const cellHeight = canvas.height / rows;
  ctx.clearRect(0, 0, canvas.width, canvas.height);
  let max = -Infinity;
  let min = Infinity;
  for (const row of valueMap) {
    for (const v of row) {
      if (v > max) max = v;
      if (v < min) min = v;
    }
  }
  const span = max - min || 1;
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      const t = (valueMap[r][c] - min) / span;
      ctx.fillStyle = heatColor(t);
      ctx.fillRect(c * cellWidth, r * cellHeight, cellWidth, cellHeight);
    }
  }
}

function heatColor(t) {
  const r = Math.floor(255 * t);
  const g = Math.floor(255 * (1 - t));
  return `rgba(${r}, ${g}, 80, 0.8)`;
}

function serializeForm(form) {
  const data = new FormData(form);
  return {
    episodes: Number(data.get('episodes')),
    seed: Number(data.get('seed')),
    epsilon: Number(data.get('epsilon')),
    alpha: Number(data.get('alpha')),
    rows: Number(data.get('rows')),
    cols: Number(data.get('cols')),
    algorithm: String(data.get('algorithm') || 'montecarlo'),
    stepDelayMs: Number(data.get('stepDelayMs')),
  };
}

function attachEventListeners() {
  const form = document.getElementById('controlForm');
  const stopBtn = document.getElementById('stopBtn');
  form.addEventListener('submit', (event) => {
    event.preventDefault();
    if (!wasmReady) {
      statusEl.textContent = 'Loading WASM...';
      return;
    }
  const cfg = serializeForm(form);
  playbackDelayMs = cfg.stepDelayMs || 0;
  currentAlgorithm = cfg.algorithm || 'montecarlo';
  console.log('[form] submitted config', cfg, 'playbackDelay', playbackDelayMs);
  const config = JSON.stringify(cfg);
  resetAnimationState();
  window.tinyrlStartTraining(config);
  statusEl.textContent = 'Training...';
  });
  stopBtn.addEventListener('click', () => {
    if (wasmReady) {
      window.tinyrlStopTraining();
      statusEl.textContent = 'Stopped';
      resetAnimationState();
    }
  });
  document.querySelectorAll('.view-toggle button').forEach((btn) => {
    btn.addEventListener('click', () => {
      document
        .querySelectorAll('.view-toggle button')
        .forEach((b) => b.classList.remove('active'));
      btn.classList.add('active');
      currentView = btn.dataset.view;
      draw();
    });
  });
}

function resetAnimationState() {
  console.log('[state] reset animation state');
  snapshotQueue = [];
  isAnimating = false;
  currentTrail = [];
  lastEpisodeId = 0;
}

(async function init() {
  await loadWasm();
  registerHandlers();
  attachEventListeners();
})();

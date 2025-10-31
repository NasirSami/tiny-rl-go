import './wasm_exec.js';

const go = new Go();
let wasmReady = false;
let currentView = 'path';
let lastSnapshot = null;
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
  lastSnapshot = snapshot;
  statusEl.textContent = `Status: ${snapshot.status}`;
  metricsEl.textContent = `Episode ${snapshot.episode}/${snapshot.config.episodes} — reward ${snapshot.episodeReward.toFixed(2)} — success ${snapshot.successCount}`;
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
  drawCell(snapshot.position, cellWidth, cellHeight, '#d63384');
}

function drawCell(pos, w, h, color) {
  ctx.fillStyle = color;
  ctx.fillRect(pos.col * w + 2, pos.row * h + 2, w - 4, h - 4);
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
  const cfg = {
    episodes: Number(data.get('episodes')),
    seed: Number(data.get('seed')),
    epsilon: Number(data.get('epsilon')),
    alpha: Number(data.get('alpha')),
  };
  return JSON.stringify(cfg);
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
    const config = serializeForm(form);
    window.tinyrlStartTraining(config);
    statusEl.textContent = 'Training...';
  });
  stopBtn.addEventListener('click', () => {
    if (wasmReady) {
      window.tinyrlStopTraining();
      statusEl.textContent = 'Stopped';
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

(async function init() {
  await loadWasm();
  registerHandlers();
  attachEventListeners();
})();

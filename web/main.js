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
let currentGamma = 0.9;
let currentGoals = [];
const canvas = document.getElementById('gridCanvas');
const ctx = canvas.getContext('2d');
const statusEl = document.getElementById('status');
const metricsEl = document.getElementById('metricSummary');
const logEl = document.getElementById('eventLog');
const canvasContainer = document.getElementById('canvasContainer');
const resizeHandle = document.getElementById('resizeHandle');
const form = document.getElementById('controlForm');
const rowSlider = form.querySelector('input[name="rows"]');
const colSlider = form.querySelector('input[name="cols"]');
const sliderInputs = form.querySelectorAll('input[type="range"][data-output-target]');

const CELL_SIZE = 24;
const MIN_ROWS = 1;
const MAX_ROWS = 20;
const MIN_COLS = 1;
const MAX_COLS = 20;
const DEFAULT_GOAL_REWARD = 1;

const state = {
  rows: Number(rowSlider.value),
  cols: Number(colSlider.value),
  goals: [],
};

state.goals = [{ row: 0, col: state.cols - 1, reward: DEFAULT_GOAL_REWARD }];
currentGoals = state.goals.map((goal) => ({ ...goal }));
let currentStepPenalty = Number(form.querySelector('input[name="stepPenalty"]').value);

let isResizingCanvas = false;
let resizeStart = null;

function clamp(value, min, max) {
  return Math.min(max, Math.max(min, value));
}

function updateCanvasSize() {
  canvas.width = state.cols * CELL_SIZE;
  canvas.height = state.rows * CELL_SIZE;
  canvasContainer.style.width = `${canvas.width}px`;
  canvasContainer.style.height = `${canvas.height}px`;
}

function syncSlidersFromState() {
  rowSlider.value = state.rows;
  colSlider.value = state.cols;
  updateSliderOutput(rowSlider);
  updateSliderOutput(colSlider);
}

function updateSliderOutput(input) {
  const targetId = input.dataset.outputTarget;
  if (!targetId) return;
  const output = document.getElementById(targetId);
  if (!output) return;
  const value = Number(input.value);
  if (input.name === 'epsilon' || input.name === 'alpha' || input.name === 'gamma') {
    output.textContent = value.toFixed(2);
  } else if (input.name === 'stepPenalty') {
    output.textContent = value.toFixed(3);
  } else {
    output.textContent = Math.round(value);
  }
}

function initializeSliders() {
  sliderInputs.forEach((input) => {
    updateSliderOutput(input);
    input.addEventListener('input', () => {
      updateSliderOutput(input);
      handleSliderChange(input.name, Number(input.value));
    });
  });
  syncSlidersFromState();
}

function handleSliderChange(name, value) {
  switch (name) {
    case 'rows':
      state.rows = clamp(Math.round(value), MIN_ROWS, MAX_ROWS);
      ensureGoalsWithinBounds();
      updateCanvasSize();
      resetAnimationState();
      draw();
      break;
    case 'cols':
      state.cols = clamp(Math.round(value), MIN_COLS, MAX_COLS);
      ensureGoalsWithinBounds();
      updateCanvasSize();
      resetAnimationState();
      draw();
      break;
    case 'stepDelayMs':
      playbackDelayMs = value;
      break;
    case 'stepPenalty':
      currentStepPenalty = value;
      break;
    default:
      break;
  }
}

function ensureGoalsWithinBounds() {
  state.goals = state.goals.filter((goal) => goal.row >= 0 && goal.row < state.rows && goal.col >= 0 && goal.col < state.cols);
  currentGoals = state.goals.map((goal) => ({ ...goal }));
}

function toggleGoal(row, col) {
  const index = state.goals.findIndex((goal) => goal.row === row && goal.col === col);
  if (index >= 0) {
    state.goals.splice(index, 1);
  } else {
    state.goals.push({ row, col, reward: DEFAULT_GOAL_REWARD });
  }
  currentGoals = state.goals.map((goal) => ({ ...goal }));
  resetAnimationState();
  draw();
}

function handleCanvasClick(event) {
  const rect = canvas.getBoundingClientRect();
  const x = event.clientX - rect.left;
  const y = event.clientY - rect.top;
  const col = clamp(Math.floor(x / CELL_SIZE), 0, state.cols - 1);
  const row = clamp(Math.floor(y / CELL_SIZE), 0, state.rows - 1);
  toggleGoal(row, col);
}

function startResize(event) {
  event.preventDefault();
  isResizingCanvas = true;
  resizeStart = {
    x: event.clientX,
    y: event.clientY,
    rows: state.rows,
    cols: state.cols,
  };
  document.body.style.userSelect = 'none';
  document.addEventListener('mousemove', onResizeMove);
  document.addEventListener('mouseup', stopResize);
}

function onResizeMove(event) {
  if (!isResizingCanvas) return;
  const dx = event.clientX - resizeStart.x;
  const dy = event.clientY - resizeStart.y;
  const newCols = clamp(Math.round((resizeStart.cols * CELL_SIZE + dx) / CELL_SIZE), MIN_COLS, MAX_COLS);
  const newRows = clamp(Math.round((resizeStart.rows * CELL_SIZE + dy) / CELL_SIZE), MIN_ROWS, MAX_ROWS);
  if (newCols !== state.cols || newRows !== state.rows) {
    state.cols = newCols;
    state.rows = newRows;
    ensureGoalsWithinBounds();
    syncSlidersFromState();
    updateCanvasSize();
    resetAnimationState();
    draw();
  }
}

function stopResize() {
  if (!isResizingCanvas) return;
  isResizingCanvas = false;
  document.body.style.userSelect = '';
  document.removeEventListener('mousemove', onResizeMove);
  document.removeEventListener('mouseup', stopResize);
}

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
  snapshotQueue.push(snapshot);
  if (!isAnimating) {
    isAnimating = true;
    processSnapshotQueue();
  }
}

function processSnapshotQueue() {
  if (snapshotQueue.length === 0) {
    isAnimating = false;
    return;
  }
  const snapshot = snapshotQueue.shift();
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
  if (snapshot.config && typeof snapshot.config.gamma === 'number') {
    currentGamma = snapshot.config.gamma;
  }
  if (snapshot.config && typeof snapshot.config.stepPenalty === 'number') {
    currentStepPenalty = snapshot.config.stepPenalty;
    const penaltySlider = form.querySelector('input[name="stepPenalty"]');
    if (penaltySlider) {
      penaltySlider.value = currentStepPenalty;
      updateSliderOutput(penaltySlider);
    }
  }
  if (Array.isArray(snapshot.goals)) {
    currentGoals = snapshot.goals.map((goal) => ({ ...goal }));
  }
  const delayLabel = playbackDelayMs > 0 ? `${playbackDelayMs}ms` : '0ms';
  const algoLabel = currentAlgorithm || 'montecarlo';
  metricsEl.textContent = `Episode ${snapshot.episode}/${snapshot.config.episodes} — reward ${snapshot.episodeReward.toFixed(2)} — success ${snapshot.successCount} — grid ${snapshot.config.rows}×${snapshot.config.cols} — algo ${algoLabel} — gamma ${currentGamma.toFixed(2)} — penalty ${currentStepPenalty.toFixed(3)} — delay ${delayLabel}`;
  appendLog(snapshot);
  if (snapshot.status === 'done' && snapshot.config) {
    state.rows = snapshot.config.rows;
    state.cols = snapshot.config.cols;
    if (Array.isArray(snapshot.config.goals)) {
      state.goals = snapshot.config.goals.map((goal) => ({ ...goal }));
    }
    ensureGoalsWithinBounds();
    syncSlidersFromState();
    updateCanvasSize();
  }
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
  const snapshot = lastSnapshot || createPlaceholderSnapshot();
  if (currentView === 'heatmap') {
    drawHeatmap(snapshot.valueMap);
  } else {
    drawGrid(snapshot);
  }
}

function createPlaceholderSnapshot() {
  const valueMap = Array.from({ length: state.rows }, () => Array(state.cols).fill(0));
  return {
    valueMap,
    position: { row: state.rows - 1, col: 0 },
  };
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
  drawCell(start, cellWidth, cellHeight, '#0d6efd');
  drawGoals(cellWidth, cellHeight);
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

function drawGoals(cellWidth, cellHeight) {
  if (!currentGoals || currentGoals.length === 0) {
    return;
  }
  currentGoals.forEach((goal) => {
    ctx.fillStyle = '#198754';
    ctx.fillRect(goal.col * cellWidth + 4, goal.row * cellHeight + 4, cellWidth - 8, cellHeight - 8);
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
    gamma: Number(data.get('gamma')),
    rows: state.rows,
    cols: state.cols,
    algorithm: String(data.get('algorithm') || 'montecarlo'),
    stepDelayMs: Number(data.get('stepDelayMs')),
    stepPenalty: Number(data.get('stepPenalty')),
    goals: state.goals.map((goal) => ({ ...goal })),
  };
}

function attachEventListeners() {
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
  currentGamma = typeof cfg.gamma === 'number' && !Number.isNaN(cfg.gamma) ? cfg.gamma : currentGamma;
  if (Array.isArray(cfg.goals)) {
    currentGoals = cfg.goals;
  }
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
  canvas.addEventListener('click', handleCanvasClick);
  resizeHandle.addEventListener('mousedown', startResize);
}

function resetAnimationState() {
  snapshotQueue = [];
  isAnimating = false;
  currentTrail = [];
  lastEpisodeId = 0;
  currentGoals = state.goals.map((goal) => ({ ...goal }));
}

(async function init() {
  await loadWasm();
  registerHandlers();
  initializeSliders();
  ensureGoalsWithinBounds();
  updateCanvasSize();
  attachEventListeners();
  draw();
})();

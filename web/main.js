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
let currentWalls = [];
let currentSlips = [];
let currentTool = 'none';
let hoverCell = null;
let recentRewards = [];
let recentSuccessFlags = [];
let lastSuccessCount = 0;
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
const goalCountSlider = form.querySelector('input[name="goalCount"]');
const goalIntervalSlider = form.querySelector('input[name="goalInterval"]');
const sliderInputs = form.querySelectorAll('input[type="range"][data-output-target]');
const wallRowInput = document.getElementById('wallRow');
const wallColInput = document.getElementById('wallCol');
const addWallBtn = document.getElementById('addWallBtn');
const wallList = document.getElementById('wallList');
const slipRowInput = document.getElementById('slipRow');
const slipColInput = document.getElementById('slipCol');
const slipProbInput = document.getElementById('slipProb');
const addSlipBtn = document.getElementById('addSlipBtn');
const slipList = document.getElementById('slipList');
const obstacleToolbar = document.getElementById('obstacleToolbar');
const toolButtons = obstacleToolbar ? Array.from(obstacleToolbar.querySelectorAll('.tool-button')) : [];
const slipProbSlider = document.getElementById('slipProbSlider');
const slipProbValue = document.getElementById('slipProbValue');
const slipProbLabelEl = document.getElementById('slipProbLabel');

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
  walls: [],
  slips: [],
  goalCount: Number(goalCountSlider.value),
  goalInterval: Number(goalIntervalSlider.value),
};

if (state.goalCount === 0) {
  state.goals = [{ row: 0, col: Math.max(state.cols - 1, 0), reward: DEFAULT_GOAL_REWARD }];
}
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
  renderObstacleLists();
  if (slipProbSlider && slipProbValue) {
    slipProbValue.textContent = Number(slipProbSlider.value).toFixed(2);
  }
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
    case 'goalCount':
      state.goalCount = Math.max(0, Math.round(value));
      if (state.goalCount === 0 && state.goals.length === 0) {
        state.goals = [{ row: 0, col: Math.max(state.cols - 1, 0), reward: DEFAULT_GOAL_REWARD }];
      }
      resetAnimationState();
      draw();
      break;
    case 'goalInterval':
      state.goalInterval = Math.max(0, Math.round(value));
      break;
    default:
      break;
  }
}

function ensureGoalsWithinBounds() {
  state.goals = state.goals.filter((goal) => goal.row >= 0 && goal.row < state.rows && goal.col >= 0 && goal.col < state.cols);
  currentGoals = state.goals.map((goal) => ({ ...goal }));
  state.walls = state.walls.filter((wall) => wall.row >= 0 && wall.row < state.rows && wall.col >= 0 && wall.col < state.cols);
  currentWalls = state.walls.map((wall) => ({ ...wall }));
  state.slips = normalizeSlips(state.slips.filter((slip) => slip.row >= 0 && slip.row < state.rows && slip.col >= 0 && slip.col < state.cols));
  currentSlips = state.slips.map((slip) => ({ ...slip }));
  renderObstacleLists();
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
    updateRollingStats(snapshot);
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
  const effectivePenalty =
    typeof snapshot.appliedStepPenalty === 'number' ? snapshot.appliedStepPenalty : currentStepPenalty;
  if (snapshot.config && typeof snapshot.config.goalCount === 'number') {
    state.goalCount = snapshot.config.goalCount;
    if (goalCountSlider) {
      goalCountSlider.value = state.goalCount;
      updateSliderOutput(goalCountSlider);
    }
  }
  if (snapshot.config && typeof snapshot.config.goalInterval === 'number') {
    state.goalInterval = snapshot.config.goalInterval;
    if (goalIntervalSlider) {
      goalIntervalSlider.value = state.goalInterval;
      updateSliderOutput(goalIntervalSlider);
    }
  }
  if (Array.isArray(snapshot.goals)) {
    currentGoals = snapshot.goals.map((goal) => ({ ...goal }));
  }
  if (Array.isArray(snapshot.walls)) {
    currentWalls = snapshot.walls.map((wall) => ({ ...wall }));
    state.walls = currentWalls.map((wall) => ({ ...wall }));
  }
  if (Array.isArray(snapshot.slips)) {
    state.slips = normalizeSlips(snapshot.slips);
    currentSlips = state.slips.map((slip) => ({ ...slip }));
  }
  const delayLabel = playbackDelayMs > 0 ? `${playbackDelayMs}ms` : '0ms';
  const algoLabel = currentAlgorithm || 'montecarlo';
  const goalInfo = snapshot.config.goalCount && snapshot.config.goalCount > 0
    ? `${snapshot.config.goalCount} (interval ${snapshot.config.goalInterval || 0})`
    : 'manual';
  const rolling = getRollingStats();
  renderMetrics(snapshot, {
    effectivePenalty,
    algoLabel,
    delayLabel,
    goalInfo,
    rolling,
  });
  appendLog(snapshot);
  if (snapshot.status === 'done' && snapshot.config) {
    state.rows = snapshot.config.rows;
    state.cols = snapshot.config.cols;
    if (Array.isArray(snapshot.config.goals)) {
      state.goals = snapshot.config.goals.map((goal) => ({ ...goal }));
    }
    if (Array.isArray(snapshot.config.walls)) {
      state.walls = snapshot.config.walls.map((wall) => ({ ...wall }));
    }
    if (Array.isArray(snapshot.config.slips)) {
      state.slips = normalizeSlips(snapshot.config.slips);
    }
    ensureGoalsWithinBounds();
    syncSlidersFromState();
    updateCanvasSize();
  }
  renderObstacleLists();
  draw();
}

function updateRollingStats(snapshot) {
  recentRewards.push(snapshot.episodeReward);
  if (recentRewards.length > 50) {
    recentRewards.shift();
  }
  const successAchieved = snapshot.successCount > lastSuccessCount ? 1 : 0;
  recentSuccessFlags.push(successAchieved);
  if (recentSuccessFlags.length > 50) {
    recentSuccessFlags.shift();
  }
  lastSuccessCount = snapshot.successCount;
}

function averageOfWindow(values, windowSize) {
  if (!values.length) return null;
  const count = Math.min(values.length, windowSize);
  let total = 0;
  for (let i = values.length - count; i < values.length; i++) {
    total += values[i];
  }
  return total / count;
}

function getRollingStats() {
  if (!recentRewards.length) {
    return null;
  }
  return {
    reward10: averageOfWindow(recentRewards, 10),
    reward50: averageOfWindow(recentRewards, 50),
    success10: averageOfWindow(recentSuccessFlags, 10),
    success50: averageOfWindow(recentSuccessFlags, 50),
  };
}

function renderMetrics(snapshot, context) {
  if (!metricsEl) {
    return;
  }
  const completed = snapshot.episodesCompleted || 0;
  const avgReward = completed > 0 ? snapshot.totalReward / completed : 0;
  const avgSteps = completed > 0 ? snapshot.totalSteps / completed : 0;
  const successRate = completed > 0 ? snapshot.successCount / completed : 0;
  const rolling = context.rolling;
  const epsilonDecayDisplay = typeof snapshot.config.epsilonDecay === 'number'
    ? snapshot.config.epsilonDecay.toFixed(3)
    : '--';

  const rollingPills = rolling
    ? [
        { label: 'reward₁₀', value: rolling.reward10 },
        { label: 'reward₅₀', value: rolling.reward50 },
        { label: 'success₁₀', value: rolling.success10 },
        { label: 'success₅₀', value: rolling.success50 },
      ]
        .filter((item) => item.value !== null && item.value !== undefined)
        .map(
          (item) =>
            `<div class="metric-pill"><span>${item.label}</span><strong>${item.value.toFixed(2)}</strong></div>`
        )
        .join('')
    : '';

  metricsEl.innerHTML = `
    <div class="metric-cards">
      <div class="metric-card">
        <span class="metric-label">Episode</span>
        <span class="metric-value">${snapshot.episode}</span>
        <span class="metric-sub">of ${snapshot.config.episodes}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Reward</span>
        <span class="metric-value">${snapshot.episodeReward.toFixed(2)}</span>
        <span class="metric-sub">avg ${avgReward.toFixed(2)}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Success</span>
        <span class="metric-value">${snapshot.successCount}</span>
        <span class="metric-sub">rate ${(successRate * 100).toFixed(0)}%</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Steps</span>
        <span class="metric-value">${snapshot.episodeSteps}</span>
        <span class="metric-sub">avg ${avgSteps.toFixed(2)}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Grid</span>
        <span class="metric-value">${snapshot.config.rows}×${snapshot.config.cols}</span>
        <span class="metric-sub">goals ${context.goalInfo}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Algorithm</span>
        <span class="metric-value">${context.algoLabel}</span>
        <span class="metric-sub">γ=${currentGamma.toFixed(2)}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Penalty</span>
        <span class="metric-value">${currentStepPenalty.toFixed(3)}</span>
        <span class="metric-sub">eff ${context.effectivePenalty.toFixed(3)}</span>
      </div>
      <div class="metric-card">
        <span class="metric-label">Delay</span>
        <span class="metric-value">${context.delayLabel}</span>
        <span class="metric-sub">Eps decay ${epsilonDecayDisplay}</span>
      </div>
    </div>
    ${rollingPills ? `<div class="metric-rolling">${rollingPills}</div>` : ''}
  `;
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

function renderObstacleLists() {
  if (wallList) {
    wallList.innerHTML = '';
    if (state.walls.length > 0) {
      const clearItem = document.createElement('li');
      const clearBtn = document.createElement('button');
      clearBtn.type = 'button';
      clearBtn.textContent = 'clear all';
      clearBtn.addEventListener('click', () => {
        clearWalls();
        renderObstacleLists();
        draw();
      });
      clearItem.append('walls ', clearBtn);
      wallList.appendChild(clearItem);
    }
    if (state.walls.length === 0) {
      const emptyItem = document.createElement('li');
      emptyItem.className = 'obstacle-empty';
      emptyItem.textContent = 'No walls yet — choose Wall tool and click the grid.';
      wallList.appendChild(emptyItem);
    }
    state.walls.forEach((wall, idx) => {
      const item = document.createElement('li');
      const label = document.createElement('span');
      label.className = 'obstacle-chip';
      label.textContent = `(${wall.row}, ${wall.col})`;
      const removeBtn = document.createElement('button');
      removeBtn.type = 'button';
      removeBtn.textContent = 'remove';
      removeBtn.addEventListener('click', () => {
        state.walls.splice(idx, 1);
        currentWalls = state.walls.map((w) => ({ ...w }));
        renderObstacleLists();
        draw();
      });
      item.append(label, removeBtn);
      wallList.appendChild(item);
    });
  }
  if (slipList) {
    slipList.innerHTML = '';
    if (state.slips.length > 0) {
      const clearItem = document.createElement('li');
      const clearBtn = document.createElement('button');
      clearBtn.type = 'button';
      clearBtn.textContent = 'clear all';
      clearBtn.addEventListener('click', () => {
        clearSlips();
        renderObstacleLists();
        draw();
      });
      clearItem.append('slips ', clearBtn);
      slipList.appendChild(clearItem);
    }
    if (state.slips.length === 0) {
      const emptyItem = document.createElement('li');
      emptyItem.className = 'obstacle-empty';
      emptyItem.textContent = 'No slip tiles yet — choose Slip tool and click the grid.';
      slipList.appendChild(emptyItem);
    }
    state.slips.forEach((slip, idx) => {
      const item = document.createElement('li');
      const label = document.createElement('span');
      label.className = 'obstacle-chip';
      label.textContent = `(${slip.row}, ${slip.col})`;
      const probValue = slip.probability ?? slip.Probability;
      const probLabel = typeof probValue === 'number' ? probValue.toFixed(2) : '0.00';
      const badge = document.createElement('span');
      badge.className = 'obstacle-badge';
      badge.textContent = `p=${probLabel}`;
      const removeBtn = document.createElement('button');
      removeBtn.type = 'button';
      removeBtn.textContent = 'remove';
      removeBtn.addEventListener('click', () => {
        state.slips.splice(idx, 1);
        state.slips = normalizeSlips(state.slips);
        currentSlips = state.slips.map((s) => ({ ...s }));
        renderObstacleLists();
        draw();
      });
      item.append(label, badge, removeBtn);
      slipList.appendChild(item);
    });
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
  drawWalls(cellWidth, cellHeight);
  drawSlipTiles(cellWidth, cellHeight);
  drawGoals(cellWidth, cellHeight);
  drawTrail(cellWidth, cellHeight);
  drawHover(cellWidth, cellHeight);
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

function drawWalls(cellWidth, cellHeight) {
  if (!currentWalls || currentWalls.length === 0) {
    return;
  }
  currentWalls.forEach((wall) => {
    ctx.fillStyle = '#495057';
    ctx.fillRect(wall.col * cellWidth + 2, wall.row * cellHeight + 2, cellWidth - 4, cellHeight - 4);
  });
}

function drawSlipTiles(cellWidth, cellHeight) {
  if (!currentSlips || currentSlips.length === 0) {
    return;
  }
  currentSlips.forEach((slip) => {
    ctx.fillStyle = 'rgba(253, 126, 20, 0.6)';
    ctx.fillRect(slip.col * cellWidth + 4, slip.row * cellHeight + 4, cellWidth - 8, cellHeight - 8);
  });
}

function drawHover(cellWidth, cellHeight) {
  if (!hoverCell || currentTool === 'none') {
    return;
  }
  const { row, col } = hoverCell;
  const x = col * cellWidth;
  const y = row * cellHeight;
  ctx.save();
  switch (currentTool) {
    case 'wall':
      ctx.strokeStyle = '#343a40';
      ctx.fillStyle = 'rgba(52, 58, 64, 0.25)';
      break;
    case 'slip':
      ctx.strokeStyle = '#fd7e14';
      ctx.fillStyle = 'rgba(253, 126, 20, 0.25)';
      break;
    case 'erase':
      ctx.strokeStyle = '#d63384';
      ctx.fillStyle = 'rgba(214, 51, 132, 0.2)';
      break;
    default:
      ctx.restore();
      return;
  }
  ctx.lineWidth = 2;
  ctx.fillRect(x + 2, y + 2, cellWidth - 4, cellHeight - 4);
  ctx.strokeRect(x + 2, y + 2, cellWidth - 4, cellHeight - 4);
  ctx.restore();
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
    epsilonMin: 0.05,
    epsilonDecay: 0.998,
    alpha: Number(data.get('alpha')),
    gamma: Number(data.get('gamma')),
    rows: state.rows,
    cols: state.cols,
    algorithm: String(data.get('algorithm') || 'montecarlo'),
    stepDelayMs: Number(data.get('stepDelayMs')),
    stepPenalty: Number(data.get('stepPenalty')),
    goalCount: state.goalCount,
    goalInterval: state.goalInterval,
    goals: state.goalCount > 0 ? [] : state.goals.map((goal) => ({ ...goal })),
    walls: state.walls.map((wall) => ({ ...wall })),
    slips: state.slips.map((slip) => ({ row: slip.row, col: slip.col, probability: slip.probability })),
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
  resizeHandle.addEventListener('mousedown', startResize);
  toolButtons.forEach((btn) => {
    btn.addEventListener('click', () => {
      setTool(btn.dataset.tool || 'none');
    });
  });
  if (slipProbSlider && slipProbValue) {
    slipProbSlider.addEventListener('input', () => {
      slipProbValue.textContent = Number(slipProbSlider.value).toFixed(2);
    });
  }
  if (addWallBtn) {
    addWallBtn.addEventListener('click', () => {
      const row = Number(wallRowInput.value);
      const col = Number(wallColInput.value);
      if (Number.isNaN(row) || Number.isNaN(col)) {
        return;
      }
      const exists = state.walls.some((wall) => wall.row === row && wall.col === col);
      if (!exists) {
        state.walls.push({ row, col });
        removeSlip(row, col);
        ensureGoalsWithinBounds();
        renderObstacleLists();
        draw();
      }
    });
  }
  if (addSlipBtn) {
    addSlipBtn.addEventListener('click', () => {
      const row = Number(slipRowInput.value);
      const col = Number(slipColInput.value);
      let probability = Number(slipProbInput.value);
      if (Number.isNaN(row) || Number.isNaN(col) || Number.isNaN(probability)) {
        return;
      }
      if (probability < 0) probability = 0;
      if (probability > 1) probability = 1;
      placeSlip(row, col, probability);
      renderObstacleLists();
      draw();
    });
  }

  if (canvas) {
    canvas.addEventListener('click', (event) => {
      if (currentTool === 'none') {
        return;
      }
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width / rect.width;
      const scaleY = canvas.height / rect.height;
      const pointerX = (event.clientX - rect.left) * scaleX;
      const pointerY = (event.clientY - rect.top) * scaleY;
      const cellWidth = canvas.width / state.cols;
      const cellHeight = canvas.height / state.rows;
      const col = Math.floor(pointerX / cellWidth);
      const row = Math.floor(pointerY / cellHeight);
      if (col < 0 || col >= state.cols || row < 0 || row >= state.rows) {
        return;
      }
      handleCanvasObstacleClick(row, col);
    });

    canvas.addEventListener('mousemove', (event) => {
      const rect = canvas.getBoundingClientRect();
      const scaleX = canvas.width / rect.width;
      const scaleY = canvas.height / rect.height;
      const pointerX = (event.clientX - rect.left) * scaleX;
      const pointerY = (event.clientY - rect.top) * scaleY;
      const cellWidth = canvas.width / state.cols;
      const cellHeight = canvas.height / state.rows;
      const col = Math.floor(pointerX / cellWidth);
      const row = Math.floor(pointerY / cellHeight);
      if (col < 0 || col >= state.cols || row < 0 || row >= state.rows) {
        if (hoverCell !== null) {
          hoverCell = null;
          draw();
        }
        return;
      }
      if (!hoverCell || hoverCell.row !== row || hoverCell.col !== col) {
        hoverCell = { row, col };
        draw();
      }
    });

    canvas.addEventListener('mouseleave', () => {
      if (hoverCell) {
        hoverCell = null;
        draw();
      }
    });
  }

  window.addEventListener('keydown', (event) => {
    if (event.target && event.target.tagName === 'INPUT') {
      return;
    }
    switch (event.key.toLowerCase()) {
      case 'n':
        setTool('none');
        break;
      case 'w':
        setTool('wall');
        break;
      case 's':
        setTool('slip');
        break;
      case 'e':
        setTool('erase');
        break;
      default:
        break;
    }
  });
}

function setTool(tool) {
  currentTool = tool;
  toolButtons.forEach((btn) => {
    const isActive = (btn.dataset.tool || 'none') === tool;
    btn.classList.toggle('active', isActive);
  });
  if (slipProbSlider) {
    const enabled = tool === 'slip';
    slipProbSlider.disabled = !enabled;
    if (slipProbLabelEl) {
      slipProbLabelEl.classList.toggle('disabled', !enabled);
    }
  }
  if (tool === 'none' && hoverCell) {
    hoverCell = null;
  }
  draw();
}

function handleCanvasObstacleClick(row, col) {
  switch (currentTool) {
    case 'wall':
      toggleWall(row, col);
      break;
    case 'slip':
      placeSlip(row, col, getCurrentSlipProbability());
      break;
    case 'erase':
      eraseObstacle(row, col);
      break;
    default:
      return;
  }
  renderObstacleLists();
  draw();
}

function toggleWall(row, col) {
  const index = state.walls.findIndex((wall) => wall.row === row && wall.col === col);
  if (index >= 0) {
    state.walls.splice(index, 1);
  } else {
    state.walls.push({ row, col });
    removeSlip(row, col);
  }
  currentWalls = state.walls.map((wall) => ({ ...wall }));
}

function placeSlip(row, col, probability) {
  if (probability <= 0) {
    removeSlip(row, col);
    return;
  }
  const payload = { row, col, probability };
  const index = state.slips.findIndex((slip) => slip.row === row && slip.col === col);
  if (index >= 0) {
    state.slips[index] = payload;
  } else {
    state.slips.push(payload);
  }
  removeWall(row, col);
  state.slips = normalizeSlips(state.slips);
  currentSlips = state.slips.map((slip) => ({ ...slip }));
}

function eraseObstacle(row, col) {
  removeWall(row, col);
  removeSlip(row, col);
}

function removeWall(row, col) {
  const index = state.walls.findIndex((wall) => wall.row === row && wall.col === col);
  if (index >= 0) {
    state.walls.splice(index, 1);
    currentWalls = state.walls.map((wall) => ({ ...wall }));
  }
}

function removeSlip(row, col) {
  const index = state.slips.findIndex((slip) => slip.row === row && slip.col === col);
  if (index >= 0) {
    state.slips.splice(index, 1);
    state.slips = normalizeSlips(state.slips);
    currentSlips = state.slips.map((slip) => ({ ...slip }));
  }
}

function getCurrentSlipProbability() {
  if (!slipProbSlider) {
    return 0.5;
  }
  let value = Number(slipProbSlider.value);
  if (Number.isNaN(value)) {
    value = 0.5;
  }
  if (value < 0) value = 0;
  if (value > 1) value = 1;
  return value;
}

function clearWalls() {
  if (state.walls.length === 0) {
    return;
  }
  state.walls = [];
  currentWalls = [];
}

function clearSlips() {
  if (state.slips.length === 0) {
    return;
  }
  state.slips = [];
  currentSlips = [];
}

function normalizeSlips(slips) {
  if (!Array.isArray(slips)) {
    return [];
  }
  return slips.map((slip) => {
    const probabilityRaw = typeof slip.probability === 'number' ? slip.probability : Number(slip.Probability ?? slip.prob ?? 0);
    const probability = clamp(Number.isNaN(probabilityRaw) ? 0 : probabilityRaw, 0, 1);
    return { row: slip.row, col: slip.col, probability };
  });
}

function resetAnimationState() {
  snapshotQueue = [];
  isAnimating = false;
  currentTrail = [];
  lastEpisodeId = 0;
  currentGoals = state.goals.map((goal) => ({ ...goal }));
  currentWalls = state.walls.map((wall) => ({ ...wall }));
  currentSlips = state.slips.map((slip) => ({ ...slip }));
  renderObstacleLists();
}

(async function init() {
  await loadWasm();
  registerHandlers();
  initializeSliders();
  ensureGoalsWithinBounds();
  updateCanvasSize();
  attachEventListeners();
  setTool(currentTool);
  draw();
})();

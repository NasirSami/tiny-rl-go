# tiny-rl-go

CPU-only, **from-scratch** reinforcement-learning lab in Go (stdlib only).  
Goal: learn single-agent and multi-agent systems by building everything yourself—no third-party libs, no GPU.

## Features (v1)
- **Solo GridWorld** (2-D, multi-goal), tabular Monte Carlo, Q-learning, and SARSA.
- **Cooperative grid** with two independent Q-learners, switches that open a door, then each agent reaches its own goal.
- CLI modes, moving-average rewards, greedy rollout with ASCII rendering, and a WASM playground.

### 🧠 Core Engine (Go backend)

* **Algorithms implemented:** Monte Carlo, Q-Learning, SARSA
  – Each shares a common training loop (`internal/engine/trainer.go`) and value-table representation (`value_table.go`).
* **Value table abstraction:**
  Tabular grid of state/action values supporting multiple “distance band” feature mappers for shaping and visualization.
* **Configurable training parameters:**
  Epsilon, gamma, alpha, step penalty, seed, episode count, and exploration decay are all runtime-configurable.
* **Snapshot system:**
  Periodically streams serialized episode snapshots to the JS frontend for live visualization.
* **CLI driver:**
  `cmd/tinyrl/main.go` provides subcommands such as `train`, CSV/JSON export, profiling hooks (`pprof`).

---

### 🌐 WebAssembly Integration

* **Go→JS bridge (`wasm_exec.js`):**
  Custom polyfill for running Go WASM modules in the browser.
* **Exports functions to JS:**
  `tinyrlStartTraining`, `tinyrlStopTraining`, `tinyrlRegisterSnapshotHandler`.
* **Status/error handling:**
  Detects missing crypto/performance APIs, retries on load failure, and surfaces human-readable messages in UI.

---

### 🧩 Frontend / UI (web/)

* **Interactive gridworld canvas:**

  * Resize dynamically by dragging the corner handle
  * Toggle between **Path** and **Heatmap** views
  * Place obstacles, walls, and stochastic “slip” tiles
  * Keyboard shortcuts: **N** (navigate), **W** (wall), **S** (slip), **E** (erase)
* **Parameter controls sidebar:**

  * Algorithm selector (Monte Carlo / Q-learning / SARSA)
  * Sliders for epsilon, alpha, gamma, step delay, step penalty, episodes, etc.
  * Deterministic seed slider for reproducible runs
* **Metrics dashboard:**

  * Rolling statistics (episodes, rewards, success rate)
  * Log panel with filters for episodes / successes / goals and a live event feed
* **Accessibility-friendly UI:**
  Semantic `<details>` sections, focus-visible outlines, ARIA live regions for status/log updates.
* **CSS design:**
  Clean card-style layout with modern shadows, rounded corners, adaptive grid metrics, and responsive behavior below 1024 px.

---

### 🧰 Build & Deployment

* **Makefile targets:**

  * `ensure-go`: installs a pinned Go 1.22.5 if missing
  * `vercel-build`: builds WASM target `web/tinyrl.wasm` and copies `wasm_exec.js`
* **Vercel deployment (`vercel.json`):**

  * Custom headers for `.wasm` files (`Content-Type: application/wasm`)
  * Immutable cache (`max-age=31536000`)
  * Skips npm/yarn install; uses `make vercel-build`
* **Single-page app:**
  `index.html` loads `main.js` and starts the RL engine automatically.

---

### 🧪 Educational Playground Capabilities

* Lets users **experiment interactively** with RL algorithms: adjust hyperparameters, observe live learning behavior, and visualize convergence patterns.
* Designed for **hands-on learning and tinkering**, bridging Go back-end logic and browser-side visualization.

---

## Tutor-First Workflow (for Codex CLI)
- Start in **Tutor Mode**: explanations, plan, and quiz—**no code**.
- To allow code for a tiny scope, type:  
  `CONFIRM: WRITE CODE — <scope>`
- To apply a shown patch, type:  
  `CONFIRM: APPLY PATCH`
- After each patch, the agent returns to Tutor Mode.

## Getting Started
### Live Demo

Explore the playground in your browser: https://tiny-rl-go-mo83.vercel.app

1. **Install Go** and verify with:
   ```bash
   go version
   ```
2. Rebuild and run the CLI playground:
   ```bash
   go run ./cmd/tinyrl train
   ```
3. Rebuild the WebAssembly bundle and launch the web UI:
   ```bash
   ./scripts/dev-web.sh        # default port 8080
   ./scripts/dev-web.sh 8081   # optional custom port
   ```

### CLI Examples

- Monte Carlo with default goal:
  ```bash
  go run ./cmd/tinyrl train --algorithm montecarlo --episodes 10
  ```
- Q-learning with custom goals (row,col,reward):
  ```bash
  go run ./cmd/tinyrl train \
    --algorithm q-learning \
    --alpha 0.5 --gamma 0.9 \
    --goal 0,3,1 --goal 2,2,0.5
  ```
- Include a per-step penalty to discourage loops:
  ```bash
  go run ./cmd/tinyrl train --algorithm montecarlo --step-penalty 0.02
  ```
- Add walls and slip tiles:
  ```bash
  go run ./cmd/tinyrl train \
    --algorithm q-learning --episodes 300 --seed 7 \
    --wall 1,1 --wall 2,2 --slip 1,2,0.2
  ```
- Capture profiles for performance analysis:
  ```bash
  go run ./cmd/tinyrl train \
    --algorithm montecarlo --episodes 500 \
    --pprof-cpu cpu.out --pprof-heap heap.out
  ```

# tiny-rl-go

CPU-only, **from-scratch** reinforcement-learning lab in Go (stdlib only).  
Goal: learn single-agent and multi-agent systems by building everything yourself—no third-party libs, no GPU.

## Features (v1)
- **Solo GridWorld** (2-D, multi-goal), tabular Monte Carlo, Q-learning, and SARSA.
- **Cooperative grid** with two independent Q-learners, switches that open a door, then each agent reaches its own goal.
- CLI modes, moving-average rewards, greedy rollout with ASCII rendering, and a WASM playground.

## Tutor-First Workflow (for Codex CLI)
- Start in **Tutor Mode**: explanations, plan, and quiz—**no code**.
- To allow code for a tiny scope, type:  
  `CONFIRM: WRITE CODE — <scope>`
- To apply a shown patch, type:  
  `CONFIRM: APPLY PATCH`
- After each patch, the agent returns to Tutor Mode.

## Getting Started
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

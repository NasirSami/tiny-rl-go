# tiny-rl-go

CPU-only, **from-scratch** reinforcement-learning lab in Go (stdlib only).  
Goal: learn single-agent and multi-agent systems by building everything yourself—no third-party libs, no GPU.

## Features (v1)
- **Solo GridWorld** (2-D, walls + goal), tabular Q-learning.
- **Cooperative grid** with two independent Q-learners, switches that open a door, then each agent reaches its own goal.
- CLI modes, moving-average rewards, and greedy rollout with ASCII rendering.

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

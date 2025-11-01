package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"syscall/js"

	"tiny-rl-go/internal/engine"
)

var (
	startFnOnce sync.Once
	trainerMu   sync.Mutex
	currentCtx  context.CancelFunc
	onSnapshot  js.Value
)

func main() {
	registerCallbacks()
	// Prevent the program from exiting.
	select {}
}

func registerCallbacks() {
	startFnOnce.Do(func() {
		js.Global().Set("tinyrlRegisterSnapshotHandler", js.FuncOf(registerSnapshotHandler))
		js.Global().Set("tinyrlStartTraining", js.FuncOf(startTraining))
		js.Global().Set("tinyrlStopTraining", js.FuncOf(stopTraining))
	})
}

func registerSnapshotHandler(this js.Value, args []js.Value) interface{} {
	if len(args) != 1 || args[0].Type() != js.TypeFunction {
		fmt.Println("registerSnapshotHandler requires a function argument")
		return nil
	}
	onSnapshot = args[0]
	return nil
}

func startTraining(this js.Value, args []js.Value) interface{} {
	if len(args) == 0 {
		fmt.Println("startTraining requires a JSON config string")
		return nil
	}
	configJSON := args[0].String()
	var cfg engine.Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		fmt.Printf("invalid config: %v\n", err)
		return nil
	}
	if onSnapshot.IsUndefined() || onSnapshot.IsNull() {
		fmt.Println("snapshot handler not registered")
		return nil
	}

	trainerMu.Lock()
	if currentCtx != nil {
		currentCtx()
	}
	ctx, cancel := context.WithCancel(context.Background())
	currentCtx = cancel
	trainerMu.Unlock()

	trainer := engine.NewTrainer(cfg)
	go func() {
		for snapshot := range trainer.Run(ctx) {
			payload := snapshotToJS(snapshot)
			onSnapshot.Invoke(payload)
		}
	}()
	return nil
}

func stopTraining(this js.Value, args []js.Value) interface{} {
	trainerMu.Lock()
	if currentCtx != nil {
		currentCtx()
		currentCtx = nil
	}
	trainerMu.Unlock()
	return nil
}

func snapshotToJS(snapshot engine.Snapshot) js.Value {
	valueMap := make([]interface{}, len(snapshot.ValueMap))
	for i, row := range snapshot.ValueMap {
		rowCopy := make([]interface{}, len(row))
		for j, v := range row {
			rowCopy[j] = v
		}
		valueMap[i] = rowCopy
	}
	position := map[string]interface{}{
		"row": snapshot.Position.Row,
		"col": snapshot.Position.Col,
	}
	goals := make([]interface{}, len(snapshot.Goals))
	for i, goal := range snapshot.Goals {
		goals[i] = map[string]interface{}{
			"row":    goal.Row,
			"col":    goal.Col,
			"reward": goal.Reward,
		}
	}
	config := map[string]interface{}{
		"episodes":    snapshot.Config.Episodes,
		"seed":        snapshot.Config.Seed,
		"epsilon":     snapshot.Config.Epsilon,
		"alpha":       snapshot.Config.Alpha,
		"gamma":       snapshot.Config.Gamma,
		"rows":        snapshot.Config.Rows,
		"cols":        snapshot.Config.Cols,
		"stepDelayMs": snapshot.Config.StepDelayMs,
		"algorithm":   snapshot.Config.Algorithm,
		"goals":       goals,
		"stepPenalty": snapshot.Config.StepPenalty,
	}
	payload := map[string]interface{}{
		"step":              snapshot.Step,
		"episode":           snapshot.Episode,
		"episodeSteps":      snapshot.EpisodeSteps,
		"episodeReward":     snapshot.EpisodeReward,
		"reward":            snapshot.Reward,
		"position":          position,
		"valueMap":          valueMap,
		"goals":             goals,
		"successCount":      snapshot.SuccessCount,
		"episodesCompleted": snapshot.EpisodesCompleted,
		"totalReward":       snapshot.TotalReward,
		"totalSteps":        snapshot.TotalSteps,
		"config":            config,
		"status":            snapshot.Status,
	}
	return js.ValueOf(payload)
}

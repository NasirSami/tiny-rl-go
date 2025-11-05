package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"strconv"
	"strings"

	"tiny-rl-go/internal/engine"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "tiny-rl-go: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("missing subcommand; try 'train'")
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "train":
		return runTrain(os.Args[2:])
	default:
		return fmt.Errorf("unknown subcommand %q", subcommand)
	}
}

func runTrain(args []string) error {
	fs := flag.NewFlagSet("train", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	envName := fs.String("env", "gridworld", "environment to train in")
	episodes := fs.Int("episodes", 1, "number of training episodes")
	seed := fs.Int64("seed", 0, "deterministic seed (0 for default)")
	epsilon := fs.Float64("epsilon", 0.5, "exploration rate (0-1)")
	epsilonMin := fs.Float64("epsilon-min", 0.05, "minimum exploration rate")
	epsilonDecay := fs.Float64("epsilon-decay", 0.998, "per-episode decay multiplier")
	alpha := fs.Float64("alpha", 0.2, "learning rate (0-1)")
	gamma := fs.Float64("gamma", 0.9, "discount factor (0-1)")
	rows := fs.Int("rows", 4, "grid rows")
	cols := fs.Int("cols", 4, "grid columns")
	stepDelay := fs.Int("step-delay", 0, "per-step delay in milliseconds")
	maxSteps := fs.Int("max-steps", 0, "maximum steps per episode (0 uses default)")
	algorithm := fs.String("algorithm", engine.AlgorithmMonteCarlo, "training algorithm (montecarlo, q-learning, sarsa)")
	var goals goalListFlag
	fs.Func("goal", "goal specification row,col,reward (repeatable)", goals.Set)
	stepPenalty := fs.Float64("step-penalty", 0.02, "per-step penalty (non-negative)")
	randomStart := fs.Bool("random-start", false, "randomize start position each episode")
	dumpTrajectory := fs.Bool("dump-trajectory", false, "print first Monte Carlo episode trajectory")
	goalCount := fs.Int("goal-count", 0, "number of auto-placed goals (0 keeps manual goals)")
	goalInterval := fs.Int("goal-interval", 20, "episodes before reshuffling auto goals (0 keeps layout)")
	var wallPositions positionListFlag
	fs.Func("wall", "wall tile at row,col (repeatable)", wallPositions.Set)
	var slipTiles slipListFlag
	fs.Func("slip", "slip tile row,col,probability (repeatable)", slipTiles.Set)
	softmaxTemp := fs.Float64("softmax-temp", 1.0, "initial softmax temperature for Monte Carlo policy")
	softmaxMinTemp := fs.Float64("softmax-min-temp", 0.1, "minimum softmax temperature during an episode")
	lambda := fs.Float64("lambda", 0.9, "eligibility trace decay (0-1)")
	warmupEpisodes := fs.Int("warmup-episodes", 0, "episodes using warmup step penalty (0 disables)")
	warmupPenalty := fs.Float64("warmup-step-penalty", 0, "step penalty during warmup episodes")
	metricsCSV := fs.String("metrics-csv", "", "write per-episode metrics to CSV at path")
	runJSON := fs.String("run-json", "", "write final run summary as JSON at path")
	pprofCPU := fs.String("pprof-cpu", "", "write CPU profile to the given path")
	pprofHeap := fs.String("pprof-heap", "", "write heap profile to the given path at exit")

	if err := fs.Parse(args); err != nil {
		return err
	}

	if *episodes <= 0 {
		return fmt.Errorf("episodes must be positive (got %d)", *episodes)
	}
	if *epsilon < 0 || *epsilon > 1 {
		return fmt.Errorf("epsilon must be between 0 and 1 (got %.2f)", *epsilon)
	}
	if *epsilonMin < 0 || *epsilonMin > *epsilon {
		return fmt.Errorf("epsilon-min must be between 0 and epsilon (got %.2f)", *epsilonMin)
	}
	if *epsilonDecay < 0 {
		return fmt.Errorf("epsilon-decay must be non-negative (got %.2f)", *epsilonDecay)
	}
	if *alpha < 0 || *alpha > 1 {
		return fmt.Errorf("alpha must be between 0 and 1 (got %.2f)", *alpha)
	}
	if *rows <= 0 {
		return fmt.Errorf("rows must be positive (got %d)", *rows)
	}
	if *cols <= 0 {
		return fmt.Errorf("cols must be positive (got %d)", *cols)
	}
	if *stepDelay < 0 {
		return fmt.Errorf("step-delay must be non-negative (got %d)", *stepDelay)
	}
	if *maxSteps < 0 {
		return fmt.Errorf("max-steps must be non-negative (got %d)", *maxSteps)
	}
	if *algorithm != engine.AlgorithmMonteCarlo {
		if *algorithm != engine.AlgorithmQLearning && *algorithm != engine.AlgorithmSARSA {
			return fmt.Errorf("unsupported algorithm %q", *algorithm)
		}
	}
	if *gamma < 0 || *gamma > 1 {
		return fmt.Errorf("gamma must be between 0 and 1 (got %.2f)", *gamma)
	}
	if *stepPenalty < 0 {
		return fmt.Errorf("step-penalty must be non-negative (got %.4f)", *stepPenalty)
	}
	if *goalCount < 0 {
		return fmt.Errorf("goal-count must be non-negative (got %d)", *goalCount)
	}
	if *goalInterval < 0 {
		return fmt.Errorf("goal-interval must be non-negative (got %d)", *goalInterval)
	}
	if *softmaxTemp <= 0 {
		return fmt.Errorf("softmax-temp must be positive (got %.2f)", *softmaxTemp)
	}
	if *softmaxMinTemp < 0 {
		return fmt.Errorf("softmax-min-temp must be non-negative (got %.2f)", *softmaxMinTemp)
	}
	if *softmaxMinTemp > *softmaxTemp {
		return fmt.Errorf("softmax-min-temp must be <= softmax-temp")
	}
	if *lambda < 0 || *lambda > 1 {
		return fmt.Errorf("lambda must be between 0 and 1 (got %.2f)", *lambda)
	}
	if *warmupEpisodes < 0 {
		return fmt.Errorf("warmup-episodes must be non-negative (got %d)", *warmupEpisodes)
	}
	if *warmupPenalty < 0 {
		return fmt.Errorf("warmup-step-penalty must be non-negative (got %.4f)", *warmupPenalty)
	}

	effectivePenalty := engine.ScaledStepPenalty(*rows, *cols, *stepPenalty)

	var (
		metricsFile   *os.File
		metricsWriter *csv.Writer
	)
	if *metricsCSV != "" {
		file, err := os.Create(*metricsCSV)
		if err != nil {
			return fmt.Errorf("create metrics csv: %w", err)
		}
		metricsFile = file
		metricsWriter = csv.NewWriter(file)
		header := []string{"episode", "steps", "episode_reward", "success", "epsilon", "alpha", "gamma", "rows", "cols", "step_penalty", "algorithm", "seed", "goal_count", "goal_interval"}
		if err := metricsWriter.Write(header); err != nil {
			metricsWriter.Flush()
			metricsFile.Close()
			return fmt.Errorf("write metrics header: %w", err)
		}
		metricsWriter.Flush()
		if err := metricsWriter.Error(); err != nil {
			metricsFile.Close()
			return fmt.Errorf("flush metrics header: %w", err)
		}
		defer func() {
			metricsWriter.Flush()
			if err := metricsWriter.Error(); err != nil {
				fmt.Fprintf(os.Stderr, "metrics csv flush error: %v\n", err)
			}
			if err := metricsFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "metrics csv close error: %v\n", err)
			}
		}()
	}

	var (
		runSummaryFile *os.File
		runSummaryEnc  *json.Encoder
	)
	if *runJSON != "" {
		file, err := os.Create(*runJSON)
		if err != nil {
			return fmt.Errorf("create run summary json: %w", err)
		}
		runSummaryFile = file
		runSummaryEnc = json.NewEncoder(file)
		runSummaryEnc.SetIndent("", "  ")
		defer func() {
			if err := runSummaryFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "run summary close error: %v\n", err)
			}
		}()
	}

	var cpuProfileFile *os.File
	if *pprofCPU != "" {
		file, err := os.Create(*pprofCPU)
		if err != nil {
			return fmt.Errorf("create CPU profile: %w", err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			file.Close()
			return fmt.Errorf("start CPU profile: %w", err)
		}
		cpuProfileFile = file
		defer func() {
			pprof.StopCPUProfile()
			if err := cpuProfileFile.Close(); err != nil {
				fmt.Fprintf(os.Stderr, "CPU profile close error: %v\n", err)
			}
		}()
	}

	fmt.Printf("train config => env=%s episodes=%d seed=%d epsilon=%.2f epsilonMin=%.2f epsilonDecay=%.3f alpha=%.2f gamma=%.2f lambda=%.2f rows=%d cols=%d stepDelayMs=%d maxSteps=%d stepPenalty=%.3f warmupEpisodes=%d warmupPenalty=%.3f effectiveStepPenalty=%.3f goalCount=%d goalInterval=%d softmaxTemp=%.2f softmaxMinTemp=%.2f randomStart=%t dumpTrajectory=%t algorithm=%s\n", *envName, *episodes, *seed, *epsilon, *epsilonMin, *epsilonDecay, *alpha, *gamma, *lambda, *rows, *cols, *stepDelay, *maxSteps, *stepPenalty, *warmupEpisodes, *warmupPenalty, effectivePenalty, *goalCount, *goalInterval, *softmaxTemp, *softmaxMinTemp, *randomStart, *dumpTrajectory, *algorithm)

	cfg := engine.Config{
		Episodes:              *episodes,
		Seed:                  *seed,
		Epsilon:               *epsilon,
		EpsilonMin:            *epsilonMin,
		EpsilonDecay:          *epsilonDecay,
		Alpha:                 *alpha,
		Gamma:                 *gamma,
		Rows:                  *rows,
		Cols:                  *cols,
		StepDelayMs:           *stepDelay,
		MaxSteps:              *maxSteps,
		Algorithm:             *algorithm,
		Goals:                 goals.Goals,
		StepPenalty:           *stepPenalty,
		RandomStart:           *randomStart,
		DumpTrajectory:        *dumpTrajectory,
		GoalCount:             *goalCount,
		GoalInterval:          *goalInterval,
		SoftmaxTemperature:    *softmaxTemp,
		SoftmaxMinTemperature: *softmaxMinTemp,
		Lambda:                *lambda,
		WarmupEpisodes:        *warmupEpisodes,
		WarmupStepPenalty:     *warmupPenalty,
		Walls:                 wallPositions.Positions,
		Slips:                 slipTiles.Slips,
	}
	trainer := engine.NewTrainer(cfg)
	ctx := context.Background()
	var (
		cumulativeReward float64
		cumulativeSteps  int
		successCount     int
		valueMap         [][]float64
		finalConfig      = cfg
		lastSuccessCount int
	)
	for snapshot := range trainer.Run(ctx) {
		switch snapshot.Status {
		case engine.StatusRunning:
			// suppress verbose step-level output in CLI mode
		case engine.StatusEpisodeComplete:
			fmt.Printf("episode %d: reward=%.2f steps=%d\n", snapshot.Episode, snapshot.EpisodeReward, snapshot.EpisodeSteps)
			cumulativeReward = snapshot.TotalReward
			cumulativeSteps = snapshot.TotalSteps
			successCount = snapshot.SuccessCount
			valueMap = snapshot.ValueMap
			finalConfig = snapshot.Config
			successDelta := 0
			if snapshot.SuccessCount > lastSuccessCount {
				successDelta = 1
			}
			lastSuccessCount = snapshot.SuccessCount
			if metricsWriter != nil {
				record := []string{
					strconv.Itoa(snapshot.Episode),
					strconv.Itoa(snapshot.EpisodeSteps),
					fmt.Sprintf("%.4f", snapshot.EpisodeReward),
					strconv.Itoa(successDelta),
					fmt.Sprintf("%.6f", snapshot.Config.Epsilon),
					fmt.Sprintf("%.6f", snapshot.Config.Alpha),
					fmt.Sprintf("%.6f", snapshot.Config.Gamma),
					strconv.Itoa(snapshot.Config.Rows),
					strconv.Itoa(snapshot.Config.Cols),
					fmt.Sprintf("%.6f", snapshot.Config.StepPenalty),
					snapshot.Config.Algorithm,
					strconv.FormatInt(snapshot.Config.Seed, 10),
					strconv.Itoa(snapshot.Config.GoalCount),
					strconv.Itoa(snapshot.Config.GoalInterval),
				}
				if err := metricsWriter.Write(record); err != nil {
					return fmt.Errorf("write metrics row: %w", err)
				}
				metricsWriter.Flush()
				if err := metricsWriter.Error(); err != nil {
					return fmt.Errorf("flush metrics row: %w", err)
				}
			}
		case engine.StatusDone:
			cumulativeReward = snapshot.TotalReward
			cumulativeSteps = snapshot.TotalSteps
			successCount = snapshot.SuccessCount
			valueMap = snapshot.ValueMap
			finalConfig = snapshot.Config
		case engine.StatusCancelled:
			fmt.Println("training cancelled")
			return nil
		}
	}

	avgReward := cumulativeReward / float64(*episodes)
	avgSteps := float64(cumulativeSteps) / float64(*episodes)
	successRate := float64(successCount) / float64(*episodes)
	fmt.Printf("summary: avg_reward=%.2f avg_steps=%.2f success_rate=%.2f\n", avgReward, avgSteps, successRate)
	printValueMap(valueMap)
	if runSummaryEnc != nil {
		payload := struct {
			Config  engine.Config `json:"config"`
			Summary struct {
				AvgReward   float64 `json:"avg_reward"`
				AvgSteps    float64 `json:"avg_steps"`
				SuccessRate float64 `json:"success_rate"`
			} `json:"summary"`
		}{
			Config: finalConfig,
		}
		payload.Summary.AvgReward = avgReward
		payload.Summary.AvgSteps = avgSteps
		payload.Summary.SuccessRate = successRate
		if err := runSummaryEnc.Encode(payload); err != nil {
			return fmt.Errorf("write run summary json: %w", err)
		}
	}

	if *pprofHeap != "" {
		heapFile, err := os.Create(*pprofHeap)
		if err != nil {
			return fmt.Errorf("create heap profile: %w", err)
		}
		if err := pprof.WriteHeapProfile(heapFile); err != nil {
			heapFile.Close()
			return fmt.Errorf("write heap profile: %w", err)
		}
		if err := heapFile.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "heap profile close error: %v\n", err)
		}
	}

	return nil
}

func printValueMap(data [][]float64) {
	if len(data) == 0 {
		return
	}
	fmt.Println("value table:")
	for _, row := range data {
		for _, v := range row {
			fmt.Printf("%6.2f ", v)
		}
		fmt.Println()
	}
}

type goalListFlag struct {
	Goals []engine.Goal
}

func (g *goalListFlag) String() string {
	return fmt.Sprintf("%v", g.Goals)
}

func (g *goalListFlag) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return fmt.Errorf("goal must be in row,col,reward format")
	}
	row, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid goal row: %w", err)
	}
	col, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid goal col: %w", err)
	}
	reward, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return fmt.Errorf("invalid goal reward: %w", err)
	}
	g.Goals = append(g.Goals, engine.Goal{Row: row, Col: col, Reward: reward})
	return nil
}

type positionListFlag struct {
	Positions []engine.Position
}

func (p *positionListFlag) String() string {
	return fmt.Sprintf("%v", p.Positions)
}

func (p *positionListFlag) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 2 {
		return fmt.Errorf("wall must be in row,col format")
	}
	row, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid wall row: %w", err)
	}
	col, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid wall col: %w", err)
	}
	p.Positions = append(p.Positions, engine.Position{Row: row, Col: col})
	return nil
}

type slipListFlag struct {
	Slips []engine.SlipTile
}

func (s *slipListFlag) String() string {
	return fmt.Sprintf("%v", s.Slips)
}

func (s *slipListFlag) Set(value string) error {
	parts := strings.Split(value, ",")
	if len(parts) != 3 {
		return fmt.Errorf("slip must be in row,col,probability format")
	}
	row, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return fmt.Errorf("invalid slip row: %w", err)
	}
	col, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return fmt.Errorf("invalid slip col: %w", err)
	}
	prob, err := strconv.ParseFloat(strings.TrimSpace(parts[2]), 64)
	if err != nil {
		return fmt.Errorf("invalid slip probability: %w", err)
	}
	if prob < 0 {
		prob = 0
	}
	if prob > 1 {
		prob = 1
	}
	s.Slips = append(s.Slips, engine.SlipTile{Row: row, Col: col, Probability: prob})
	return nil
}

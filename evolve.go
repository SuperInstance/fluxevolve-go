package fluxevolve

import (
	"math"
	"math/rand"
	"sort"
	"time"
)

type MutationType int

const (
	MutParamAdjust      MutationType = iota
	MutThresholdShift
	MutWeightRebalance
	MutAddBehavior
	MutRemoveBehavior
	MutSwapPriority
	MutRateChange
	MutCapChange
)

type MutationRecord struct {
	Type      MutationType
	Parameter string
	OldValue  float64
	NewValue  float64
	Reason    string
	Timestamp time.Time
	Generation uint32
	Reverted  bool
}

type Behavior struct {
	Name            string
	Value           float64
	Min             float64
	Max             float64
	Default         float64
	MutationRate    float64
	Uses            uint32
	CumulativeScore float64
}

type Genome struct {
	Behaviors        map[string]*Behavior
	Generation       uint32
	Born             time.Time
	MutationsTotal   uint32
	MutationsReverted uint32
}

type Engine struct {
	Genome
	History           []MutationRecord
	FitnessScore      float64
	FitnessThreshold  float64
	MutationProbability float64
	EliteThreshold    float64
}

func NewEngine() *Engine {
	return &Engine{
		Genome: Genome{
			Behaviors: make(map[string]*Behavior),
			Born:      time.Now(),
		},
		FitnessThreshold:    0.3,
		MutationProbability: 0.1,
		EliteThreshold:      0.8,
	}
}

func (e *Engine) AddBehavior(name string, value, min, max, mutRate float64) {
	e.Behaviors[name] = &Behavior{
		Name:         name,
		Value:        math.Max(min, math.Min(max, value)),
		Min:          min,
		Max:          max,
		Default:      value,
		MutationRate: mutRate,
	}
}

func (e *Engine) FindBehavior(name string) *Behavior {
	return e.Behaviors[name]
}

func (e *Engine) Get(name string) float64 {
	b := e.Behaviors[name]
	if b == nil {
		return -1
	}
	return b.Value
}

func (e *Engine) Set(name string, value float64) {
	b := e.Behaviors[name]
	if b == nil {
		return
	}
	b.Value = math.Max(b.Min, math.Min(b.Max, value))
}

func (e *Engine) Cycle(now time.Time, fitness float64) int {
	e.FitnessScore = fitness
	e.Generation++
	mutations := 0

	// Elite: no mutation
	if fitness >= e.EliteThreshold {
		return 0
	}

	rateMultiplier := 1.0
	if fitness < e.FitnessThreshold {
		rateMultiplier = 3.0
	}

	names := make([]string, 0, len(e.Behaviors))
	for name := range e.Behaviors {
		names = append(names, name)
	}

	for _, name := range names {
		b := e.Behaviors[name]
		chance := b.MutationRate * rateMultiplier
		if chance < e.MutationProbability {
			chance = e.MutationProbability
		}
		if chance > 1 {
			chance = 1
		}

		if rand.Float64() > chance {
			continue
		}

		oldVal := b.Value
		// Perturb by up to 10% of range
		rng := b.Max - b.Min
		delta := (rand.Float64()*2 - 1) * 0.1 * rng
		newVal := math.Max(b.Min, math.Min(b.Max, b.Value+delta))

		b.Value = newVal
		e.MutationsTotal++
		mutations++

		e.History = append(e.History, MutationRecord{
			Type:       MutParamAdjust,
			Parameter:  name,
			OldValue:   oldVal,
			NewValue:   newVal,
			Reason:     "cycle mutation",
			Timestamp:  now,
			Generation: e.Generation,
			Reverted:   false,
		})
	}

	return mutations
}

func (e *Engine) Score(behavior string, outcome float64) {
	b := e.Behaviors[behavior]
	if b == nil {
		return
	}
	b.Uses++
	b.CumulativeScore += outcome
}

func (e *Engine) Revert(historyIndex int) bool {
	if historyIndex < 0 || historyIndex >= len(e.History) {
		return false
	}
	rec := &e.History[historyIndex]
	if rec.Reverted {
		return false
	}
	b := e.Behaviors[rec.Parameter]
	if b == nil {
		return false
	}
	b.Value = rec.OldValue
	rec.Reverted = true
	e.MutationsReverted++
	return true
}

func (e *Engine) Rollback(targetGeneration uint32) int {
	reverts := 0
	for i := len(e.History) - 1; i >= 0; i-- {
		rec := &e.History[i]
		if rec.Generation <= targetGeneration || rec.Reverted {
			break
		}
		if e.Revert(i) {
			reverts++
		}
	}
	e.Generation = targetGeneration
	return reverts
}

func (e *Engine) WorstBehaviors(n int) []*Behavior {
	return e.topN(n, false)
}

func (e *Engine) BestBehaviors(n int) []*Behavior {
	return e.topN(n, true)
}

func (e *Engine) topN(n int, best bool) []*Behavior {
	all := make([]*Behavior, 0, len(e.Behaviors))
	for _, b := range e.Behaviors {
		all = append(all, b)
	}
	sort.Slice(all, func(i, j int) bool {
		ai, aj := avgScore(all[i]), avgScore(all[j])
		if best {
			return ai > aj
		}
		return ai < aj
	})
	if n > len(all) {
		n = len(all)
	}
	return all[:n]
}

func avgScore(b *Behavior) float64 {
	if b.Uses == 0 {
		return 0
	}
	return b.CumulativeScore / float64(b.Uses)
}

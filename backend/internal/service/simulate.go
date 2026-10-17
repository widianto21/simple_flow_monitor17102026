package service

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"gorm.io/gorm"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/models"
)

// SimOptions controls demo traffic generation.
type SimOptions struct {
	Hours       int   `json:"hours"`
	RunsPerFlow int   `json:"runsPerFlow"`
	ActiveRuns  int   `json:"activeRuns"` // executions intentionally left running per flow
	Seed        int64 `json:"seed"`
}

// SimStats summarizes generated demo traffic.
type SimStats struct {
	Flows  int `json:"flows"`
	Runs   int `json:"runs"`
	Steps  int `json:"steps"`
	Events int `json:"events"`
	Stuck  int `json:"stuck"`
}

// Simulator generates plausible historical + live traffic by replaying
// synthetic events through the real ingestion path, so all derived state
// (runs, tasks, alerts, events) stays consistent.
type Simulator struct {
	db     *gorm.DB
	cfg    *config.Config
	ingest *Ingest
}

var simFailProbability = map[models.NodeKind]float64{
	models.KindTask:     0.05,
	models.KindExternal: 0.09,
	models.KindDecision: 0.02,
	models.KindGateway:  0.01,
}

// demoNodeFailureTweaks gives a few seeded nodes extra failure flavour.
var demoNodeFailureTweaks = map[string]float64{
	"validate_payment":  0.14,
	"kyc_screening":     0.18,
	"assess_damage":     0.11,
	"reserve_inventory": 0.08,
}

var simErrorMessages = []string{
	"upstream timeout after 30s",
	"validation failed: required field missing",
	"downstream service returned 503",
	"business rule rejected the request",
	"rate limit exceeded",
}

var simInitiatedBy = []string{"checkout-ui", "ops-console", "api-gateway", "cron-scheduler", "partner-api"}

var simRegions = []string{"ap-southeast-1", "eu-west-1", "us-east-1"}

// Run generates demo traffic for every active flow.
func (s *Simulator) Run(ctx context.Context, opts SimOptions) (*SimStats, error) {
	if opts.Hours <= 0 {
		opts.Hours = 48
	}
	if opts.RunsPerFlow <= 0 {
		opts.RunsPerFlow = 40
	}
	if opts.ActiveRuns < 0 {
		opts.ActiveRuns = 0
	}
	if opts.ActiveRuns > opts.RunsPerFlow {
		opts.ActiveRuns = opts.RunsPerFlow
	}
	seed := opts.Seed
	if seed == 0 {
		seed = time.Now().UnixNano()
	}
	rng := rand.New(rand.NewSource(seed))

	var flows []models.Flow
	if err := s.db.WithContext(ctx).Where("active = ?", true).Order("id asc").Find(&flows).Error; err != nil {
		return nil, err
	}

	stats := &SimStats{Flows: len(flows)}
	now := time.Now().UTC()

	for _, flow := range flows {
		graph, ok, err := s.loadGraph(flow.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}

		for i := 0; i < opts.RunsPerFlow; i++ {
			active := i >= opts.RunsPerFlow-opts.ActiveRuns
			businessKey := fmt.Sprintf("%s-%05d", businessKeyPrefix(flow.Key), 1000+i)
			events, stuck := s.simulateRun(graph, flow, businessKey, active, opts.Hours, rng, now)

			err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				for _, ev := range events {
					if _, perr := s.ingest.ProcessTx(tx, ev); perr != nil {
						return perr
					}
				}
				return nil
			})
			if err != nil {
				log.Printf("simulator: run %s skipped: %v", businessKey, err)
				continue
			}

			stats.Runs++
			stats.Events += len(events)
			for _, ev := range events {
				if ev.Status == "started" {
					stats.Steps++
				}
			}
			if stuck {
				stats.Stuck++
			}
		}
	}

	return stats, nil
}

// ---------------------------------------------------------------------------
// graph helpers
// ---------------------------------------------------------------------------

type flowGraph struct {
	nodes map[string]models.FlowNode
	start models.FlowNode
	next  map[string][]models.FlowEdge
}

func (s *Simulator) loadGraph(flowID uint) (*flowGraph, bool, error) {
	var nodes []models.FlowNode
	if err := s.db.Where("flow_id = ?", flowID).Find(&nodes).Error; err != nil {
		return nil, false, err
	}
	if len(nodes) == 0 {
		return nil, false, nil
	}
	var edges []models.FlowEdge
	if err := s.db.Where("flow_id = ?", flowID).Find(&edges).Error; err != nil {
		return nil, false, err
	}

	g := &flowGraph{
		nodes: make(map[string]models.FlowNode, len(nodes)),
		next:  make(map[string][]models.FlowEdge),
	}
	var start *models.FlowNode
	for i := range nodes {
		g.nodes[nodes[i].Key] = nodes[i]
		if nodes[i].Kind == models.KindStart && start == nil {
			start = &nodes[i]
		}
	}
	for _, e := range edges {
		g.next[e.SourceKey] = append(g.next[e.SourceKey], e)
	}
	if start == nil {
		start = &nodes[0]
	}
	g.start = *start
	return g, true, nil
}

// walk picks a plausible path from the start node to a terminal node.
func (g *flowGraph) walk(rng *rand.Rand) []models.FlowNode {
	path := make([]models.FlowNode, 0, 8)
	cur := g.start
	visits := map[string]int{}

	for range 40 {
		path = append(path, cur)
		if cur.Kind == models.KindEnd {
			break
		}
		options := g.next[cur.Key]
		if len(options) == 0 {
			break
		}
		edge := pickEdge(options, rng)
		visits[edge.TargetKey]++
		if visits[edge.TargetKey] > 3 {
			break // do not spin forever on retry loops
		}
		nextNode, ok := g.nodes[edge.TargetKey]
		if !ok {
			break
		}
		cur = nextNode
	}
	return path
}

// pickEdge prefers "happy path" branches so decision outcomes look realistic.
func pickEdge(edges []models.FlowEdge, rng *rand.Rand) models.FlowEdge {
	if len(edges) == 1 {
		return edges[0]
	}
	weights := make([]float64, len(edges))
	total := 0.0
	for i, e := range edges {
		label := strings.ToLower(e.Label + " " + e.Condition)
		w := 1.0
		for _, negative := range []string{"no", "false", "reject", "fail", "decline", "denied"} {
			if strings.Contains(label, negative) {
				w = 0.2
				break
			}
		}
		weights[i] = w
		total += w
	}
	target := rng.Float64() * total
	acc := 0.0
	for i, w := range weights {
		acc += w
		if target <= acc {
			return edges[i]
		}
	}
	return edges[len(edges)-1]
}

func simDuration(kind models.NodeKind, rng *rand.Rand) time.Duration {
	base := map[models.NodeKind]time.Duration{
		models.KindStart:    300 * time.Millisecond,
		models.KindEnd:      400 * time.Millisecond,
		models.KindDecision: 20 * time.Second,
		models.KindGateway:  8 * time.Second,
		models.KindTask:     6 * time.Minute,
		models.KindExternal: 2 * time.Minute,
	}[kind]
	if base == 0 {
		base = 3 * time.Minute
	}
	factor := 0.35 + rng.Float64()*1.3
	if rng.Float64() < 0.07 {
		factor *= 4 // occasional spike → SLA breaches / interesting p95
	}
	return time.Duration(float64(base) * factor)
}

func simFailChance(node models.FlowNode) float64 {
	if p, ok := demoNodeFailureTweaks[node.Key]; ok {
		return p
	}
	return simFailProbability[node.Kind]
}

func pickRandom(values []string, rng *rand.Rand) string {
	if len(values) == 0 {
		return ""
	}
	return values[rng.Intn(len(values))]
}

// simulateRun builds the event stream of one execution.
func (s *Simulator) simulateRun(
	graph *flowGraph,
	flow models.Flow,
	businessKey string,
	active bool,
	hours int,
	rng *rand.Rand,
	now time.Time,
) ([]dto.IngestEvent, bool) {
	path := graph.walk(rng)
	if len(path) == 0 {
		return nil, false
	}

	durations := make([]time.Duration, len(path))
	for i, node := range path {
		durations[i] = simDuration(node.Kind, rng)
	}

	// Index of the first step that is not completed (active runs only).
	cut := len(path)
	if active && len(path) > 1 {
		cut = 1 + rng.Intn(len(path)-1)
	}

	// Point of failure for historical runs (at most one).
	failAt := -1
	if !active {
		for i := range path {
			if rng.Float64() < simFailChance(path[i]) {
				failAt = i
				break
			}
		}
	}

	var start time.Time
	stuck := false
	if active {
		elapsedBefore := time.Duration(0)
		for i := 0; i < cut; i++ {
			elapsedBefore += durations[i]
		}
		sla := time.Duration(resolveTimeout(&path[cut], s.cfg.DefaultSLAMinutes)) * time.Minute
		// Roughly 45% of the open tasks are clearly over SLA (so the stuck view
		// has obvious cases), the rest sit comfortably inside their SLA.
		factor := 0.15 + rng.Float64()*0.7
		if rng.Float64() < 0.45 {
			factor = 1.0 + rng.Float64()*1.2
		}
		openElapsed := time.Duration(factor * float64(sla))
		start = now.Add(-(elapsedBefore + openElapsed))
		stuck = openElapsed > sla
	} else {
		start = now.Add(-time.Duration(rng.Float64() * float64(time.Duration(hours)*time.Hour)))
	}

	initiatedBy := pickRandom(simInitiatedBy, rng)
	region := pickRandom(simRegions, rng)

	events := make([]dto.IngestEvent, 0, len(path)*2)
	t := start

	emitStart := func(node models.FlowNode, at time.Time) {
		ts := at.UTC()
		events = append(events, dto.IngestEvent{
			FlowKey:     flow.Key,
			NodeKey:     node.Key,
			BusinessKey: businessKey,
			Status:      "started",
			OccurredAt:  &ts,
			Attempt:     1,
			Source:      node.System,
			Actor:       "system",
			InitiatedBy: initiatedBy,
			Request:     rawOrNull(node.RequestSchema),
			Metadata:    map[string]any{"region": region, "simulated": true},
		})
	}

	for i := range path {
		node := path[i]

		if active && i >= cut {
			if i == cut {
				emitStart(node, t) // task still running when the demo data is created
			}
			break
		}
		if !active && failAt >= 0 && i > failAt {
			break
		}

		emitStart(node, t)
		done := t.Add(durations[i])

		if !active && i == failAt {
			ts := done.UTC()
			events = append(events, dto.IngestEvent{
				FlowKey:     flow.Key,
				NodeKey:     node.Key,
				BusinessKey: businessKey,
				Status:      "failed",
				OccurredAt:  &ts,
				DurationMs:  durations[i].Milliseconds(),
				Attempt:     1,
				Source:      node.System,
				Actor:       "system",
				Error:       pickRandom(simErrorMessages, rng),
			})
			break
		}

		ts := done.UTC()
		events = append(events, dto.IngestEvent{
			FlowKey:     flow.Key,
			NodeKey:     node.Key,
			BusinessKey: businessKey,
			Status:      "completed",
			OccurredAt:  &ts,
			DurationMs:  durations[i].Milliseconds(),
			Attempt:     1,
			Source:      node.System,
			Actor:       "system",
			Response:    rawOrNull(node.ResponseSample),
		})
		t = done
	}

	return events, stuck
}

package service

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/models"
)

// ErrNotFound is returned when a requested resource does not exist.
var ErrNotFound = errors.New("not found")

// Query answers all read-side questions: definitions, live stats, history.
type Query struct {
	db  *gorm.DB
	cfg *config.Config
}

// RunFilter selects executions.
type RunFilter struct {
	FlowKey     string
	Status      string
	BusinessKey string
	NodeKey     string
	From        *time.Time
	To          *time.Time
	Page        int
	Size        int
}

// StepFilter selects task executions.
type StepFilter struct {
	FlowKey     string
	NodeKey     string
	Status      string
	BusinessKey string
	StuckOnly   bool
	From        *time.Time
	To          *time.Time
	Page        int
	Size        int
}

// NodeDetailResult is the drill-down of one diagram component.
type NodeDetailResult struct {
	Node       dto.FlowNodeDTO                  `json:"node"`
	Tasks      dto.ListResponse[dto.StepRunDTO] `json:"tasks"`
	RecentRuns dto.ListResponse[dto.RunSummary] `json:"recentRuns"`
}

// ---------------------------------------------------------------------------
// indexes & helpers
// ---------------------------------------------------------------------------

func nodeIdxKey(flowID uint, nodeKey string) string {
	return fmt.Sprintf("%d:%s", flowID, nodeKey)
}

func maxTime(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (q *Query) resolveFlow(idOrKey string) (*models.Flow, error) {
	idOrKey = strings.TrimSpace(idOrKey)
	if idOrKey == "" {
		return nil, ErrNotFound
	}
	var flow models.Flow
	if id, err := strconv.ParseUint(idOrKey, 10, 64); err == nil {
		if err := q.db.First(&flow, uint(id)).Error; err == nil {
			return &flow, nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
	}
	if err := q.db.Where("key = ?", idOrKey).First(&flow).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &flow, nil
}

func (q *Query) flowByID(id uint) (*models.Flow, error) {
	var flow models.Flow
	if err := q.db.First(&flow, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &flow, nil
}

func (q *Query) flowIndex() (map[uint]models.Flow, error) {
	var flows []models.Flow
	if err := q.db.Find(&flows).Error; err != nil {
		return nil, err
	}
	idx := make(map[uint]models.Flow, len(flows))
	for _, f := range flows {
		idx[f.ID] = f
	}
	return idx, nil
}

// nodeIndexByFlow loads nodes of the given flows keyed by "flowID:nodeKey".
func (q *Query) nodeIndexByFlow(flowIDs []uint) (map[string]models.FlowNode, error) {
	idx := map[string]models.FlowNode{}
	if len(flowIDs) == 0 {
		return idx, nil
	}
	var nodes []models.FlowNode
	if err := q.db.Where("flow_id IN ?", flowIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	for _, n := range nodes {
		idx[nodeIdxKey(n.FlowID, n.Key)] = n
	}
	return idx, nil
}

func (q *Query) timeoutOf(nodes map[string]models.FlowNode, flowID uint, nodeKey string) int {
	n, ok := nodes[nodeIdxKey(flowID, nodeKey)]
	if !ok {
		return q.cfg.DefaultSLAMinutes
	}
	return resolveTimeout(&n, q.cfg.DefaultSLAMinutes)
}

// timeoutOfNode is the same lookup for maps keyed by the plain node key
// (a single flow's node map, as used by the diagram stats).
func (q *Query) timeoutOfNode(nodes map[string]models.FlowNode, nodeKey string) int {
	n, ok := nodes[nodeKey]
	if !ok {
		return q.cfg.DefaultSLAMinutes
	}
	return resolveTimeout(&n, q.cfg.DefaultSLAMinutes)
}

// ---------------------------------------------------------------------------
// definition reads
// ---------------------------------------------------------------------------

type flowAgg struct {
	nodeCount     int
	runningExec   int
	runningTasks  int
	stuckTasks    int
	completed24h  int
	failed24h     int
	avgDurationMs int64
	lastActivity  *time.Time
}

// flowAggregates computes live counters for the given flows in a handful of queries.
func (q *Query) flowAggregates(flowIDs []uint, now time.Time) (map[uint]*flowAgg, error) {
	out := make(map[uint]*flowAgg, len(flowIDs))
	for _, id := range flowIDs {
		out[id] = &flowAgg{}
	}
	if len(flowIDs) == 0 {
		return out, nil
	}

	var nodeRows []struct {
		FlowID uint
		Total  int64
	}
	if err := q.db.Model(&models.FlowNode{}).Select("flow_id, COUNT(*) AS total").
		Where("flow_id IN ?", flowIDs).Group("flow_id").Scan(&nodeRows).Error; err != nil {
		return nil, err
	}
	for _, row := range nodeRows {
		if agg, ok := out[row.FlowID]; ok {
			agg.nodeCount = int(row.Total)
		}
	}

	nodes, err := q.nodeIndexByFlow(flowIDs)
	if err != nil {
		return nil, err
	}

	var openSteps []models.StepRun
	if err := q.db.Select("id, flow_id, node_key, started_at").
		Where("flow_id IN ? AND status = ?", flowIDs, models.StepRunning).Find(&openSteps).Error; err != nil {
		return nil, err
	}
	for _, s := range openSteps {
		agg, ok := out[s.FlowID]
		if !ok {
			continue
		}
		agg.runningTasks++
		if isOverdue(s.StartedAt, q.timeoutOf(nodes, s.FlowID, s.NodeKey), now) {
			agg.stuckTasks++
		}
		agg.lastActivity = ptrMaxTime(agg.lastActivity, s.StartedAt)
	}

	since := now.Add(-24 * time.Hour)
	var recentRuns []models.Run
	if err := q.db.Select("id, flow_id, status, duration_ms, started_at").
		Where("flow_id IN ? AND started_at >= ?", flowIDs, since).Find(&recentRuns).Error; err != nil {
		return nil, err
	}
	durSum := map[uint]int64{}
	durCnt := map[uint]int{}
	for _, r := range recentRuns {
		agg, ok := out[r.FlowID]
		if !ok {
			continue
		}
		switch r.Status {
		case models.RunCompleted:
			agg.completed24h++
			durSum[r.FlowID] += r.DurationMs
			durCnt[r.FlowID]++
		case models.RunFailed:
			agg.failed24h++
		}
		agg.lastActivity = ptrMaxTime(agg.lastActivity, r.StartedAt)
	}
	for id, agg := range out {
		if durCnt[id] > 0 {
			agg.avgDurationMs = durSum[id] / int64(durCnt[id])
		}
	}

	var runningRuns []models.Run
	if err := q.db.Select("id, flow_id, started_at").
		Where("flow_id IN ? AND status = ?", flowIDs, models.RunRunning).Find(&runningRuns).Error; err != nil {
		return nil, err
	}
	for _, r := range runningRuns {
		if agg, ok := out[r.FlowID]; ok {
			agg.runningExec++
			agg.lastActivity = ptrMaxTime(agg.lastActivity, r.StartedAt)
		}
	}

	return out, nil
}

func ptrMaxTime(current *time.Time, candidate time.Time) *time.Time {
	if current == nil || candidate.After(*current) {
		t := candidate
		return &t
	}
	return current
}

func summaryOf(flow models.Flow, agg *flowAgg) dto.FlowSummary {
	summary := dto.FlowSummary{
		ID:            flow.ID,
		Key:           flow.Key,
		Name:          flow.Name,
		Description:   flow.Description,
		Category:      flow.Category,
		Owner:         flow.Owner,
		Version:       flow.Version,
		Active:        flow.Active,
		UpdatedAt:     flow.UpdatedAt,
		LastActivity:  agg.lastActivity,
		NodeCount:     agg.nodeCount,
		Running:       agg.runningExec,
		RunningTasks:  agg.runningTasks,
		StuckTasks:    agg.stuckTasks,
		Completed24h:  agg.completed24h,
		Failed24h:     agg.failed24h,
		AvgDurationMs: agg.avgDurationMs,
	}
	finished := agg.completed24h + agg.failed24h
	if finished > 0 {
		summary.SuccessRate = float64(agg.completed24h) / float64(finished)
	}
	return summary
}

// ListFlows returns every flow with its live counters.
func (q *Query) ListFlows() ([]dto.FlowSummary, error) {
	now := time.Now().UTC()
	var flows []models.Flow
	if err := q.db.Order("name asc").Find(&flows).Error; err != nil {
		return nil, err
	}
	if len(flows) == 0 {
		return []dto.FlowSummary{}, nil
	}
	ids := make([]uint, 0, len(flows))
	for _, f := range flows {
		ids = append(ids, f.ID)
	}
	aggs, err := q.flowAggregates(ids, now)
	if err != nil {
		return nil, err
	}
	out := make([]dto.FlowSummary, 0, len(flows))
	for _, f := range flows {
		out = append(out, summaryOf(f, aggs[f.ID]))
	}
	return out, nil
}

// nodeStats computes per-node live + 24h aggregated statistics for one flow.
func (q *Query) nodeStats(flowID uint, nodes map[string]models.FlowNode, now time.Time) (map[string]dto.NodeStats, error) {
	out := make(map[string]dto.NodeStats, len(nodes))
	for _, n := range nodes {
		out[n.Key] = dto.NodeStats{NodeKey: n.Key}
	}

	var openSteps []models.StepRun
	if err := q.db.Select("id, node_key, started_at").
		Where("flow_id = ? AND status = ?", flowID, models.StepRunning).Find(&openSteps).Error; err != nil {
		return nil, err
	}
	for _, s := range openSteps {
		stat := out[s.NodeKey]
		stat.Running++
		elapsed := now.Sub(s.StartedAt).Milliseconds()
		if elapsed > stat.MaxElapsedMs {
			stat.MaxElapsedMs = elapsed
		}
		if isOverdue(s.StartedAt, q.timeoutOfNode(nodes, s.NodeKey), now) {
			stat.Stuck++
		}
		stat.LastActivity = ptrMaxTime(stat.LastActivity, s.StartedAt)
		out[s.NodeKey] = stat
	}

	since := now.Add(-24 * time.Hour)
	var closedSteps []models.StepRun
	if err := q.db.Select("id, node_key, status, started_at, duration_ms").
		Where("flow_id = ? AND started_at >= ? AND status <> ?", flowID, since, models.StepRunning).
		Find(&closedSteps).Error; err != nil {
		return nil, err
	}
	durations := map[string][]int64{}
	for _, s := range closedSteps {
		stat := out[s.NodeKey]
		stat.Total24h++
		if s.Status == models.StepFailed {
			stat.Failed24h++
		}
		if s.Status == models.StepCompleted && s.DurationMs > 0 {
			durations[s.NodeKey] = append(durations[s.NodeKey], s.DurationMs)
		}
		stat.LastActivity = ptrMaxTime(stat.LastActivity, s.StartedAt)
		out[s.NodeKey] = stat
	}

	for key, values := range durations {
		stat := out[key]
		sorted := sortedCopy(values)
		var sum int64
		for _, v := range sorted {
			sum += v
		}
		stat.AvgDurationMs = sum / int64(len(sorted))
		stat.P95DurationMs = sorted[p95Index(len(sorted))]
		out[key] = stat
	}

	for key, stat := range out {
		if stat.Total24h > 0 {
			stat.FailureRate = float64(stat.Failed24h) / float64(stat.Total24h)
		}
		out[key] = stat
	}
	return out, nil
}

func toNodeDTO(node models.FlowNode, stats dto.NodeStats) dto.FlowNodeDTO {
	return dto.FlowNodeDTO{
		ID:             node.ID,
		Key:            node.Key,
		Name:           node.Name,
		Kind:           string(node.Kind),
		Description:    node.Description,
		System:         node.System,
		Repository:     node.Repository,
		Team:           node.Team,
		Method:         node.Method,
		Endpoint:       node.Endpoint,
		RequestSchema:  rawOrNull(node.RequestSchema),
		ResponseSample: rawOrNull(node.ResponseSample),
		TimeoutMinutes: node.TimeoutMinutes,
		PosX:           node.PosX,
		PosY:           node.PosY,
		Stats:          stats,
	}
}

func toContract(node models.FlowNode) dto.NodeContract {
	return dto.NodeContract{
		Key:            node.Key,
		Name:           node.Name,
		Kind:           string(node.Kind),
		System:         node.System,
		Repository:     node.Repository,
		Team:           node.Team,
		Method:         node.Method,
		Endpoint:       node.Endpoint,
		RequestSchema:  rawOrNull(node.RequestSchema),
		ResponseSample: rawOrNull(node.ResponseSample),
		TimeoutMinutes: node.TimeoutMinutes,
	}
}

// GetFlow returns the full definition (nodes enriched with live stats) + contracts.
func (q *Query) GetFlow(idOrKey string) (*dto.FlowDetail, error) {
	now := time.Now().UTC()
	flow, err := q.resolveFlow(idOrKey)
	if err != nil {
		return nil, err
	}

	var nodes []models.FlowNode
	if err := q.db.Where("flow_id = ?", flow.ID).Order("id asc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	var edges []models.FlowEdge
	if err := q.db.Where("flow_id = ?", flow.ID).Order("id asc").Find(&edges).Error; err != nil {
		return nil, err
	}

	nodeMap := make(map[string]models.FlowNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.Key] = n
	}
	stats, err := q.nodeStats(flow.ID, nodeMap, now)
	if err != nil {
		return nil, err
	}
	aggs, err := q.flowAggregates([]uint{flow.ID}, now)
	if err != nil {
		return nil, err
	}

	detail := &dto.FlowDetail{
		FlowSummary:   summaryOf(*flow, aggs[flow.ID]),
		Nodes:         make([]dto.FlowNodeDTO, 0, len(nodes)),
		Edges:         make([]dto.FlowEdgeDTO, 0, len(edges)),
		NodeContracts: make([]dto.NodeContract, 0, len(nodes)),
	}
	for _, n := range nodes {
		detail.Nodes = append(detail.Nodes, toNodeDTO(n, stats[n.Key]))
		detail.NodeContracts = append(detail.NodeContracts, toContract(n))
	}
	for _, e := range edges {
		detail.Edges = append(detail.Edges, dto.FlowEdgeDTO{
			ID:        e.ID,
			SourceKey: e.SourceKey,
			TargetKey: e.TargetKey,
			Label:     e.Label,
			Condition: e.Condition,
		})
	}
	return detail, nil
}

func validateFlowInput(in dto.FlowInput) error {
	if len(in.Nodes) == 0 {
		return fmt.Errorf("a flow needs at least one node")
	}
	seen := map[string]bool{}
	for _, n := range in.Nodes {
		key := strings.TrimSpace(n.Key)
		if key == "" {
			return fmt.Errorf("every node needs a key")
		}
		if seen[key] {
			return fmt.Errorf("duplicate node key %q", key)
		}
		seen[key] = true
	}
	for _, e := range in.Edges {
		if !seen[e.SourceKey] {
			return fmt.Errorf("edge source %q is not a node", e.SourceKey)
		}
		if !seen[e.TargetKey] {
			return fmt.Errorf("edge target %q is not a node", e.TargetKey)
		}
	}
	return nil
}

// SaveFlow creates (idOrKey == "") or replaces a flow definition.
func (q *Query) SaveFlow(idOrKey string, in dto.FlowInput) (*dto.FlowDetail, error) {
	in.Key = strings.TrimSpace(in.Key)
	if in.Key == "" || strings.TrimSpace(in.Name) == "" {
		return nil, fmt.Errorf("key and name are required")
	}
	if err := validateFlowInput(in); err != nil {
		return nil, err
	}

	var flowID uint
	if idOrKey != "" {
		existing, err := q.resolveFlow(idOrKey)
		if err != nil {
			return nil, err
		}
		flowID = existing.ID
	}

	active := true
	if in.Active != nil {
		active = *in.Active
	}

	err := q.db.Transaction(func(tx *gorm.DB) error {
		if flowID == 0 {
			flow := models.Flow{
				Key:         in.Key,
				Name:        in.Name,
				Description: in.Description,
				Category:    in.Category,
				Owner:       in.Owner,
				Version:     in.Version,
				Active:      active,
			}
			if err := tx.Create(&flow).Error; err != nil {
				return err
			}
			flowID = flow.ID
		} else if err := tx.Model(&models.Flow{}).Where("id = ?", flowID).Updates(map[string]any{
			"key":         in.Key,
			"name":        in.Name,
			"description": in.Description,
			"category":    in.Category,
			"owner":       in.Owner,
			"version":     in.Version,
			"active":      active,
		}).Error; err != nil {
			return err
		}

		// Edges are cheap to rebuild.
		if err := tx.Where("flow_id = ?", flowID).Delete(&models.FlowEdge{}).Error; err != nil {
			return err
		}

		keep := map[string]bool{}
		for _, n := range in.Nodes {
			key := strings.TrimSpace(n.Key)
			keep[key] = true
			kind := models.NodeKind(n.Kind)
			if kind == "" {
				kind = models.KindTask
			}
			values := map[string]any{
				"name":            n.Name,
				"kind":            kind,
				"description":     n.Description,
				"system":          n.System,
				"repository":      n.Repository,
				"team":            n.Team,
				"method":          n.Method,
				"endpoint":        n.Endpoint,
				"request_schema":  string(n.RequestSchema),
				"response_sample": string(n.ResponseSample),
				"timeout_minutes": n.TimeoutMinutes,
				"pos_x":           n.PosX,
				"pos_y":           n.PosY,
			}

			var existing models.FlowNode
			err := tx.Where("flow_id = ? AND key = ?", flowID, key).First(&existing).Error
			switch {
			case err == nil:
				if err := tx.Model(&models.FlowNode{}).Where("id = ?", existing.ID).Updates(values).Error; err != nil {
					return err
				}
			case errors.Is(err, gorm.ErrRecordNotFound):
				node := models.FlowNode{FlowID: flowID, Key: key}
				if err := tx.Create(&node).Error; err != nil {
					return err
				}
				if err := tx.Model(&models.FlowNode{}).Where("id = ?", node.ID).Updates(values).Error; err != nil {
					return err
				}
			default:
				return err
			}
		}

		// Drop nodes removed in the editor. Telemetry keeps the old node key as
		// history, so nothing is lost by deleting the definition entry.
		var existingNodes []models.FlowNode
		if err := tx.Where("flow_id = ?", flowID).Find(&existingNodes).Error; err != nil {
			return err
		}
		for _, node := range existingNodes {
			if keep[node.Key] {
				continue
			}
			if err := tx.Where("flow_id = ? AND key = ?", flowID, node.Key).Delete(&models.FlowNode{}).Error; err != nil {
				return err
			}
		}

		for _, e := range in.Edges {
			edge := models.FlowEdge{
				FlowID:    flowID,
				SourceKey: e.SourceKey,
				TargetKey: e.TargetKey,
				Label:     e.Label,
				Condition: e.Condition,
			}
			if err := tx.Create(&edge).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return q.GetFlow(strconv.FormatUint(uint64(flowID), 10))
}

// DeleteFlow removes a definition and all of its telemetry.
func (q *Query) DeleteFlow(idOrKey string) error {
	flow, err := q.resolveFlow(idOrKey)
	if err != nil {
		return err
	}
	return q.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("flow_id = ?", flow.ID).Delete(&models.FlowEdge{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", flow.ID).Delete(&models.FlowNode{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", flow.ID).Delete(&models.StepRun{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", flow.ID).Delete(&models.Run{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_id = ?", flow.ID).Delete(&models.Alert{}).Error; err != nil {
			return err
		}
		if err := tx.Where("flow_key = ?", flow.Key).Delete(&models.Event{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Flow{}, flow.ID).Error
	})
}

// NodeDetail returns one diagram component: contract + stats + task history.
func (q *Query) NodeDetail(idOrKey, nodeKey string, limit int) (*NodeDetailResult, error) {
	now := time.Now().UTC()
	flow, err := q.resolveFlow(idOrKey)
	if err != nil {
		return nil, err
	}
	var node models.FlowNode
	if err := q.db.Where("flow_id = ? AND key = ?", flow.ID, nodeKey).First(&node).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	stats, err := q.nodeStats(flow.ID, map[string]models.FlowNode{node.Key: node}, now)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 200 {
		limit = 25
	}
	tasks, err := q.ListSteps(StepFilter{FlowKey: flow.Key, NodeKey: nodeKey, Page: 1, Size: limit})
	if err != nil {
		return nil, err
	}
	runs, err := q.ListRuns(RunFilter{FlowKey: flow.Key, NodeKey: nodeKey, Page: 1, Size: 5})
	if err != nil {
		return nil, err
	}

	return &NodeDetailResult{
		Node:       toNodeDTO(node, stats[node.Key]),
		Tasks:      *tasks,
		RecentRuns: *runs,
	}, nil
}

// FlowNodeStats returns every node of a flow with its live/historic stats —
// the data the diagram uses to colour and badge components.
func (q *Query) FlowNodeStats(idOrKey string) ([]dto.FlowNodeDTO, error) {
	now := time.Now().UTC()
	flow, err := q.resolveFlow(idOrKey)
	if err != nil {
		return nil, err
	}
	var nodes []models.FlowNode
	if err := q.db.Where("flow_id = ?", flow.ID).Order("id asc").Find(&nodes).Error; err != nil {
		return nil, err
	}
	nodeMap := make(map[string]models.FlowNode, len(nodes))
	for _, n := range nodes {
		nodeMap[n.Key] = n
	}
	stats, err := q.nodeStats(flow.ID, nodeMap, now)
	if err != nil {
		return nil, err
	}
	out := make([]dto.FlowNodeDTO, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, toNodeDTO(n, stats[n.Key]))
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// telemetry reads
// ---------------------------------------------------------------------------

func (q *Query) buildStepDTOs(steps []models.StepRun, runIdx map[uint]models.Run, flowIdx map[uint]models.Flow, nodes map[string]models.FlowNode, now time.Time) []dto.StepRunDTO {
	out := make([]dto.StepRunDTO, 0, len(steps))
	for _, s := range steps {
		run := runIdx[s.RunID]
		flow := flowIdx[s.FlowID]
		timeout := q.timeoutOf(nodes, s.FlowID, s.NodeKey)

		item := dto.StepRunDTO{
			ID:              s.ID,
			RunID:           s.RunID,
			FlowID:          s.FlowID,
			FlowKey:         flow.Key,
			FlowName:        flow.Name,
			BusinessKey:     run.BusinessKey,
			RunStatus:       string(run.Status),
			NodeKey:         s.NodeKey,
			NodeName:        s.NodeName,
			NodeKind:        string(s.NodeKind),
			Status:          string(s.Status),
			Sequence:        s.Sequence,
			Attempt:         s.Attempt,
			StartedAt:       s.StartedAt,
			CompletedAt:     s.CompletedAt,
			DurationMs:      s.DurationMs,
			RequestPayload:  rawOrNull(s.RequestPayload),
			ResponsePayload: rawOrNull(s.ResponsePayload),
			Error:           s.Error,
			Source:          s.Source,
			Actor:           s.Actor,
			Metadata:        rawOrNull(s.Metadata),
			TimeoutMinutes:  timeout,
		}
		if node, ok := nodes[nodeIdxKey(s.FlowID, s.NodeKey)]; ok {
			item.System = node.System
		}
		if s.Status == models.StepRunning {
			item.ElapsedMs = now.Sub(s.StartedAt).Milliseconds()
			if isOverdue(s.StartedAt, timeout, now) {
				item.Stuck = true
				item.OverdueMs = item.ElapsedMs - int64(timeout)*60_000
			}
		}
		out = append(out, item)
	}
	return out
}

func buildRunSummary(run models.Run, flow models.Flow, nodes map[string]models.FlowNode, stepCount int, stuck bool, now time.Time) dto.RunSummary {
	summary := dto.RunSummary{
		ID:             run.ID,
		FlowID:         run.FlowID,
		FlowKey:        flow.Key,
		FlowName:       flow.Name,
		BusinessKey:    run.BusinessKey,
		Status:         string(run.Status),
		CurrentNodeKey: run.CurrentNodeKey,
		StartedAt:      run.StartedAt,
		CompletedAt:    run.CompletedAt,
		DurationMs:     run.DurationMs,
		InitiatedBy:    run.InitiatedBy,
		FailureError:   run.FailureError,
		StepCount:      stepCount,
		Stuck:          stuck,
		Metadata:       rawOrNull(run.Metadata),
	}
	if node, ok := nodes[nodeIdxKey(run.FlowID, run.CurrentNodeKey)]; ok {
		summary.CurrentNode = node.Name
	}
	if run.Status == models.RunRunning {
		summary.ElapsedMs = now.Sub(run.StartedAt).Milliseconds()
	}
	return summary
}

// ListRuns lists executions with filters and pagination.
func (q *Query) ListRuns(f RunFilter) (*dto.ListResponse[dto.RunSummary], error) {
	now := time.Now().UTC()
	page, size := normalizePage(f.Page, f.Size)
	flowIdx, err := q.flowIndex()
	if err != nil {
		return nil, err
	}

	if f.Status == "stuck" {
		return q.listStuckRuns(f, flowIdx, now)
	}

	query := q.db.Model(&models.Run{})
	if f.FlowKey != "" {
		flowID, ok := flowIDByKey(flowIdx, f.FlowKey)
		if !ok {
			return emptyRuns(page, size), nil
		}
		query = query.Where("flow_id = ?", flowID)
	}
	if f.BusinessKey != "" {
		query = query.Where("business_key LIKE ?", "%"+f.BusinessKey+"%")
	}
	if f.Status != "" && f.Status != "all" {
		query = query.Where("status = ?", f.Status)
	}
	if f.From != nil {
		query = query.Where("started_at >= ?", *f.From)
	}
	if f.To != nil {
		query = query.Where("started_at <= ?", *f.To)
	}
	if f.NodeKey != "" {
		query = query.Where("id IN (?)", q.db.Model(&models.StepRun{}).Select("run_id").Where("node_key = ?", f.NodeKey))
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var runs []models.Run
	if err := query.Order("started_at desc").Limit(size).Offset((page - 1) * size).Find(&runs).Error; err != nil {
		return nil, err
	}

	items, err := q.enrichRuns(runs, flowIdx, now)
	if err != nil {
		return nil, err
	}
	return &dto.ListResponse[dto.RunSummary]{Items: items, Total: total, Page: page, Size: size}, nil
}

// listStuckRuns filters running executions down to those with at least one
// over-SLA task. Running executions are few, so this filters in memory.
func (q *Query) listStuckRuns(f RunFilter, flowIdx map[uint]models.Flow, now time.Time) (*dto.ListResponse[dto.RunSummary], error) {
	page, size := normalizePage(f.Page, f.Size)

	query := q.db.Model(&models.Run{}).Where("status = ?", models.RunRunning)
	if f.FlowKey != "" {
		flowID, ok := flowIDByKey(flowIdx, f.FlowKey)
		if !ok {
			return emptyRuns(page, size), nil
		}
		query = query.Where("flow_id = ?", flowID)
	}
	if f.BusinessKey != "" {
		query = query.Where("business_key LIKE ?", "%"+f.BusinessKey+"%")
	}

	var runs []models.Run
	if err := query.Order("started_at desc").Find(&runs).Error; err != nil {
		return nil, err
	}
	items, err := q.enrichRuns(runs, flowIdx, now)
	if err != nil {
		return nil, err
	}

	stuck := make([]dto.RunSummary, 0, len(items))
	for _, item := range items {
		if item.Stuck {
			if f.NodeKey != "" && item.CurrentNodeKey != f.NodeKey {
				continue
			}
			stuck = append(stuck, item)
		}
	}
	total := int64(len(stuck))
	start := (page - 1) * size
	if start > len(stuck) {
		start = len(stuck)
	}
	end := start + size
	if end > len(stuck) {
		end = len(stuck)
	}
	return &dto.ListResponse[dto.RunSummary]{Items: stuck[start:end], Total: total, Page: page, Size: size}, nil
}

func (q *Query) enrichRuns(runs []models.Run, flowIdx map[uint]models.Flow, now time.Time) ([]dto.RunSummary, error) {
	items := make([]dto.RunSummary, 0, len(runs))
	if len(runs) == 0 {
		return items, nil
	}

	runIDs := make([]uint, 0, len(runs))
	flowIDs := make([]uint, 0, len(runs))
	for _, r := range runs {
		runIDs = append(runIDs, r.ID)
		flowIDs = append(flowIDs, r.FlowID)
	}
	var steps []models.StepRun
	if err := q.db.Select("id, run_id, flow_id, node_key, status, started_at").
		Where("run_id IN ?", distinctIDs(runIDs)).Find(&steps).Error; err != nil {
		return nil, err
	}
	nodes, err := q.nodeIndexByFlow(distinctIDs(flowIDs))
	if err != nil {
		return nil, err
	}

	stepCount := map[uint]int{}
	stuckRuns := map[uint]bool{}
	for _, s := range steps {
		stepCount[s.RunID]++
		if s.Status == models.StepRunning && isOverdue(s.StartedAt, q.timeoutOf(nodes, s.FlowID, s.NodeKey), now) {
			stuckRuns[s.RunID] = true
		}
	}

	for _, r := range runs {
		flow := flowIdx[r.FlowID]
		items = append(items, buildRunSummary(r, flow, nodes, stepCount[r.ID], stuckRuns[r.ID], now))
	}
	return items, nil
}

func flowIDByKey(flowIdx map[uint]models.Flow, key string) (uint, bool) {
	for id, flow := range flowIdx {
		if flow.Key == key {
			return id, true
		}
	}
	return 0, false
}

func emptyRuns(page, size int) *dto.ListResponse[dto.RunSummary] {
	return &dto.ListResponse[dto.RunSummary]{Items: []dto.RunSummary{}, Total: 0, Page: page, Size: size}
}

// GetRun is the full drill-down of one execution.
func (q *Query) GetRun(id uint) (*dto.RunDetail, error) {
	now := time.Now().UTC()

	var run models.Run
	if err := q.db.First(&run, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	flow, err := q.flowByID(run.FlowID)
	if err != nil {
		return nil, err
	}
	flowIdx := map[uint]models.Flow{flow.ID: *flow}

	var steps []models.StepRun
	if err := q.db.Where("run_id = ?", run.ID).Order("sequence asc, id asc").Find(&steps).Error; err != nil {
		return nil, err
	}
	nodes, err := q.nodeIndexByFlow([]uint{run.FlowID})
	if err != nil {
		return nil, err
	}

	runIdx := map[uint]models.Run{run.ID: run}
	stepDTOs := q.buildStepDTOs(steps, runIdx, flowIdx, nodes, now)

	stuck := false
	for _, s := range stepDTOs {
		if s.Stuck {
			stuck = true
			break
		}
	}
	detail := &dto.RunDetail{
		RunSummary: buildRunSummary(run, *flow, nodes, len(steps), stuck, now),
		Steps:      stepDTOs,
	}

	var events []models.Event
	if err := q.db.Where("run_id = ?", run.ID).Order("occurred_at asc").Find(&events).Error; err != nil {
		return nil, err
	}
	detail.Events = make([]dto.EventDTO, 0, len(events))
	for _, e := range events {
		detail.Events = append(detail.Events, dto.EventDTO{
			ID:         e.ID,
			NodeKey:    e.NodeKey,
			Status:     e.Status,
			OccurredAt: e.OccurredAt,
			ReceivedAt: e.ReceivedAt,
			Source:     e.Source,
			Payload:    rawOrNull(e.Payload),
			Error:      e.Error,
		})
	}

	alerts, err := q.runAlerts(run.ID)
	if err != nil {
		return nil, err
	}
	detail.Alerts = alerts

	var previous []models.Run
	if err := q.db.Where("flow_id = ? AND business_key = ? AND id <> ?", run.FlowID, run.BusinessKey, run.ID).
		Order("started_at desc").Limit(10).Find(&previous).Error; err != nil {
		return nil, err
	}
	prevItems, err := q.enrichRuns(previous, flowIdx, now)
	if err != nil {
		return nil, err
	}
	detail.PreviousRuns = prevItems

	return detail, nil
}

func (q *Query) runAlerts(runID uint) ([]dto.AlertDTO, error) {
	var alerts []models.Alert
	if err := q.db.Where("run_id = ?", runID).Order("created_at desc").Find(&alerts).Error; err != nil {
		return nil, err
	}
	return q.alertDTOs(alerts), nil
}

func (q *Query) alertDTOs(alerts []models.Alert) []dto.AlertDTO {
	out := make([]dto.AlertDTO, 0, len(alerts))
	if len(alerts) == 0 {
		return out
	}
	flowNames := map[uint]string{}
	var flows []models.Flow
	q.db.Select("id, name, key").Find(&flows)
	for _, f := range flows {
		flowNames[f.ID] = f.Name
	}
	businessKeys := map[uint]string{}
	runIDs := make([]uint, 0, len(alerts))
	for _, a := range alerts {
		runIDs = append(runIDs, a.RunID)
	}
	var runs []models.Run
	q.db.Select("id, business_key").Where("id IN ?", distinctIDs(runIDs)).Find(&runs)
	for _, r := range runs {
		businessKeys[r.ID] = r.BusinessKey
	}

	for _, a := range alerts {
		out = append(out, dto.AlertDTO{
			ID:             a.ID,
			FlowID:         a.FlowID,
			FlowKey:        a.FlowKey,
			FlowName:       flowNames[a.FlowID],
			RunID:          a.RunID,
			BusinessKey:    businessKeys[a.RunID],
			NodeKey:        a.NodeKey,
			Type:           string(a.Type),
			Severity:       a.Severity,
			Message:        a.Message,
			CreatedAt:      a.CreatedAt,
			Acknowledged:   a.Acknowledged,
			AcknowledgedAt: a.AcknowledgedAt,
			Resolved:       a.Resolved,
			ResolvedAt:     a.ResolvedAt,
		})
	}
	return out
}

// GetStep is the drill-down of one task execution.
func (q *Query) GetStep(stepID uint) (*dto.StepDetail, error) {
	now := time.Now().UTC()

	var step models.StepRun
	if err := q.db.First(&step, stepID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var run models.Run
	if err := q.db.First(&run, step.RunID).Error; err != nil {
		return nil, err
	}
	flow, err := q.flowByID(step.FlowID)
	if err != nil {
		return nil, err
	}
	flowIdx := map[uint]models.Flow{flow.ID: *flow}
	nodes, err := q.nodeIndexByFlow([]uint{step.FlowID})
	if err != nil {
		return nil, err
	}
	runIdx := map[uint]models.Run{run.ID: run}

	dtoSteps := q.buildStepDTOs([]models.StepRun{step}, runIdx, flowIdx, nodes, now)

	var previous []models.StepRun
	if err := q.db.Where("run_id = ? AND sequence < ?", run.ID, step.Sequence).
		Order("sequence desc").Limit(10).Find(&previous).Error; err != nil {
		return nil, err
	}
	var next []models.StepRun
	if err := q.db.Where("run_id = ? AND sequence > ?", run.ID, step.Sequence).
		Order("sequence asc").Limit(10).Find(&next).Error; err != nil {
		return nil, err
	}
	var history []models.StepRun
	if err := q.db.Where("flow_id = ? AND node_key = ? AND id <> ?", step.FlowID, step.NodeKey, step.ID).
		Order("started_at desc").Limit(10).Find(&history).Error; err != nil {
		return nil, err
	}

	detail := &dto.StepDetail{
		Step:     dtoSteps[0],
		Run:      buildRunSummary(run, *flow, nodes, 0, false, now),
		Previous: q.stepDTOsForSteps(previous, flowIdx, nodes, now),
		Next:     q.stepDTOsForSteps(next, flowIdx, nodes, now),
		History:  q.stepDTOsForSteps(history, flowIdx, nodes, now),
	}
	if node, ok := nodes[nodeIdxKey(step.FlowID, step.NodeKey)]; ok {
		contract := toContract(node)
		detail.Node = &contract
	}
	return detail, nil
}

// stepDTOsForSteps enriches steps that may belong to other runs/flows.
func (q *Query) stepDTOsForSteps(steps []models.StepRun, flowIdx map[uint]models.Flow, nodes map[string]models.FlowNode, now time.Time) []dto.StepRunDTO {
	if len(steps) == 0 {
		return []dto.StepRunDTO{}
	}
	runIDs := make([]uint, 0, len(steps))
	flowIDs := make([]uint, 0, len(steps))
	for _, s := range steps {
		runIDs = append(runIDs, s.RunID)
		flowIDs = append(flowIDs, s.FlowID)
	}
	runIdx := map[uint]models.Run{}
	var runs []models.Run
	q.db.Where("id IN ?", distinctIDs(runIDs)).Find(&runs)
	for _, r := range runs {
		runIdx[r.ID] = r
	}
	// Make sure referenced flows are known, so flowKey/flowName are populated.
	missing := make([]uint, 0)
	for _, id := range distinctIDs(flowIDs) {
		if _, ok := flowIdx[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		var flows []models.Flow
		q.db.Where("id IN ?", missing).Find(&flows)
		for _, f := range flows {
			flowIdx[f.ID] = f
		}
	}
	return q.buildStepDTOs(steps, runIdx, flowIdx, nodes, now)
}

// ListSteps searches task executions across flows.
func (q *Query) ListSteps(f StepFilter) (*dto.ListResponse[dto.StepRunDTO], error) {
	now := time.Now().UTC()
	page, size := normalizePage(f.Page, f.Size)

	flowIdx, err := q.flowIndex()
	if err != nil {
		return nil, err
	}
	empty := func() *dto.ListResponse[dto.StepRunDTO] {
		return &dto.ListResponse[dto.StepRunDTO]{Items: []dto.StepRunDTO{}, Total: 0, Page: page, Size: size}
	}

	if f.StuckOnly || f.Status == "stuck" {
		return q.listStuckSteps(f, flowIdx, now)
	}

	query := q.db.Model(&models.StepRun{})
	if f.FlowKey != "" {
		flowID, ok := flowIDByKey(flowIdx, f.FlowKey)
		if !ok {
			return empty(), nil
		}
		query = query.Where("flow_id = ?", flowID)
	}
	if f.NodeKey != "" {
		query = query.Where("node_key = ?", f.NodeKey)
	}
	if f.Status != "" && f.Status != "all" {
		query = query.Where("status = ?", f.Status)
	}
	if f.BusinessKey != "" {
		query = query.Where("run_id IN (?)", q.db.Model(&models.Run{}).Select("id").Where("business_key LIKE ?", "%"+f.BusinessKey+"%"))
	}
	if f.From != nil {
		query = query.Where("started_at >= ?", *f.From)
	}
	if f.To != nil {
		query = query.Where("started_at <= ?", *f.To)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var steps []models.StepRun
	if err := query.Order("started_at desc").Limit(size).Offset((page - 1) * size).Find(&steps).Error; err != nil {
		return nil, err
	}

	items, err := q.enrichSteps(steps, flowIdx, now)
	if err != nil {
		return nil, err
	}
	return &dto.ListResponse[dto.StepRunDTO]{Items: items, Total: total, Page: page, Size: size}, nil
}

func (q *Query) listStuckSteps(f StepFilter, flowIdx map[uint]models.Flow, now time.Time) (*dto.ListResponse[dto.StepRunDTO], error) {
	page, size := normalizePage(f.Page, f.Size)

	tasks, err := LoadOpenTasks(q.db, q.cfg.DefaultSLAMinutes)
	if err != nil {
		return nil, err
	}
	stuck := DetectStuck(tasks, now)
	if f.FlowKey != "" {
		filtered := stuck[:0]
		for _, t := range stuck {
			if t.FlowKey == f.FlowKey {
				filtered = append(filtered, t)
			}
		}
		stuck = filtered
	}
	if f.NodeKey != "" {
		filtered := stuck[:0]
		for _, t := range stuck {
			if t.NodeKey == f.NodeKey {
				filtered = append(filtered, t)
			}
		}
		stuck = filtered
	}
	sort.Slice(stuck, func(i, j int) bool {
		return stuck[i].Overdue(now) > stuck[j].Overdue(now)
	})

	total := int64(len(stuck))
	start := (page - 1) * size
	if start > len(stuck) {
		start = len(stuck)
	}
	end := start + size
	if end > len(stuck) {
		end = len(stuck)
	}

	ids := make([]uint, 0, end-start)
	for _, t := range stuck[start:end] {
		ids = append(ids, t.StepRunID)
	}
	var steps []models.StepRun
	if len(ids) > 0 {
		if err := q.db.Where("id IN ?", ids).Find(&steps).Error; err != nil {
			return nil, err
		}
	}
	// preserve the overdue ordering
	byID := map[uint]models.StepRun{}
	for _, s := range steps {
		byID[s.ID] = s
	}
	ordered := make([]models.StepRun, 0, len(ids))
	for _, id := range ids {
		if s, ok := byID[id]; ok {
			ordered = append(ordered, s)
		}
	}

	items, err := q.enrichSteps(ordered, flowIdx, now)
	if err != nil {
		return nil, err
	}
	return &dto.ListResponse[dto.StepRunDTO]{Items: items, Total: total, Page: page, Size: size}, nil
}

func (q *Query) enrichSteps(steps []models.StepRun, flowIdx map[uint]models.Flow, now time.Time) ([]dto.StepRunDTO, error) {
	items := []dto.StepRunDTO{}
	if len(steps) == 0 {
		return items, nil
	}
	runIDs := make([]uint, 0, len(steps))
	flowIDs := make([]uint, 0, len(steps))
	for _, s := range steps {
		runIDs = append(runIDs, s.RunID)
		flowIDs = append(flowIDs, s.FlowID)
	}
	runIdx := map[uint]models.Run{}
	var runs []models.Run
	if err := q.db.Where("id IN ?", distinctIDs(runIDs)).Find(&runs).Error; err != nil {
		return nil, err
	}
	for _, r := range runs {
		runIdx[r.ID] = r
	}
	missing := make([]uint, 0)
	for _, id := range distinctIDs(flowIDs) {
		if _, ok := flowIdx[id]; !ok {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 {
		var flows []models.Flow
		q.db.Where("id IN ?", missing).Find(&flows)
		for _, f := range flows {
			flowIdx[f.ID] = f
		}
	}
	nodes, err := q.nodeIndexByFlow(distinctIDs(flowIDs))
	if err != nil {
		return nil, err
	}
	return q.buildStepDTOs(steps, runIdx, flowIdx, nodes, now), nil
}

// ListStuck returns the running tasks that are over their SLA, worst first.
func (q *Query) ListStuck(limit int) ([]dto.StepRunDTO, error) {
	now := time.Now().UTC()
	if limit <= 0 {
		limit = 50
	}
	flowIdx, err := q.flowIndex()
	if err != nil {
		return nil, err
	}
	res, err := q.listStuckSteps(StepFilter{Page: 1, Size: limit}, flowIdx, now)
	if err != nil {
		return nil, err
	}
	return res.Items, nil
}

// ListAlerts returns alerts filtered by lifecycle state.
// status: open (default) | acked | resolved | all
func (q *Query) ListAlerts(status string, limit int) ([]dto.AlertDTO, error) {
	if limit <= 0 {
		limit = 100
	}
	query := q.db.Model(&models.Alert{})
	switch status {
	case "acked", "acknowledged":
		query = query.Where("acknowledged = ?", true)
	case "resolved":
		query = query.Where("resolved = ?", true)
	case "all":
		// no filter
	default: // "open"
		query = query.Where("resolved = ?", false)
	}

	var alerts []models.Alert
	if err := query.Order("created_at desc").Limit(limit).Find(&alerts).Error; err != nil {
		return nil, err
	}
	return q.alertDTOs(alerts), nil
}

// AckAlert acknowledges an alert (it is not acknowledged again).
func (q *Query) AckAlert(id uint) (*dto.AlertDTO, error) {
	var alert models.Alert
	if err := q.db.First(&alert, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if !alert.Acknowledged {
		now := time.Now().UTC()
		alert.Acknowledged = true
		alert.AcknowledgedAt = &now
		if err := q.db.Model(&models.Alert{}).Where("id = ?", id).
			Updates(map[string]any{"acknowledged": true, "acknowledged_at": now}).Error; err != nil {
			return nil, err
		}
	}
	dtos := q.alertDTOs([]models.Alert{alert})
	if len(dtos) == 0 {
		return nil, ErrNotFound
	}
	return &dtos[0], nil
}

// BusinessKeyHistory returns the execution chain of one business key, newest
// first — this is the "what happened before this task" view.
func (q *Query) BusinessKeyHistory(businessKey string, limit int) ([]dto.RunDetail, error) {
	now := time.Now().UTC()
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	if strings.TrimSpace(businessKey) == "" {
		return nil, fmt.Errorf("business key is required")
	}

	var runs []models.Run
	if err := q.db.Where("business_key = ?", businessKey).Order("started_at desc").Limit(limit).Find(&runs).Error; err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return []dto.RunDetail{}, nil
	}

	runIDs := make([]uint, 0, len(runs))
	flowIDs := make([]uint, 0, len(runs))
	for _, r := range runs {
		runIDs = append(runIDs, r.ID)
		flowIDs = append(flowIDs, r.FlowID)
	}
	flowIdx, err := q.flowIndex()
	if err != nil {
		return nil, err
	}
	nodes, err := q.nodeIndexByFlow(distinctIDs(flowIDs))
	if err != nil {
		return nil, err
	}

	var steps []models.StepRun
	if err := q.db.Where("run_id IN ?", distinctIDs(runIDs)).Order("sequence asc, id asc").Find(&steps).Error; err != nil {
		return nil, err
	}
	stepsByRun := map[uint][]models.StepRun{}
	for _, s := range steps {
		stepsByRun[s.RunID] = append(stepsByRun[s.RunID], s)
	}
	runIdx := map[uint]models.Run{}
	for _, r := range runs {
		runIdx[r.ID] = r
	}

	out := make([]dto.RunDetail, 0, len(runs))
	for _, run := range runs {
		runSteps := stepsByRun[run.ID]
		stepDTOs := q.buildStepDTOs(runSteps, runIdx, flowIdx, nodes, now)
		stuck := false
		for _, s := range stepDTOs {
			if s.Stuck {
				stuck = true
				break
			}
		}
		detail := dto.RunDetail{
			RunSummary: buildRunSummary(run, flowIdx[run.FlowID], nodes, len(runSteps), stuck, now),
			Steps:      stepDTOs,
			Events:     []dto.EventDTO{},
			Alerts:     []dto.AlertDTO{},
		}
		var alerts []models.Alert
		if err := q.db.Where("run_id = ?", run.ID).Order("created_at desc").Find(&alerts).Error; err != nil {
			return nil, err
		}
		detail.Alerts = q.alertDTOs(alerts)
		out = append(out, detail)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// dashboard
// ---------------------------------------------------------------------------

func hourBucket(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, time.UTC)
}

func dayBucket(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// Dashboard assembles every headline number and chart in one round-trip.
func (q *Query) Dashboard() (*dto.Dashboard, error) {
	now := time.Now().UTC()

	flows, err := q.ListFlows()
	if err != nil {
		return nil, err
	}

	openTasks, err := LoadOpenTasks(q.db, q.cfg.DefaultSLAMinutes)
	if err != nil {
		return nil, err
	}
	stuckTasks := DetectStuck(openTasks, now)

	var runningExecutions int64
	if err := q.db.Model(&models.Run{}).Where("status = ?", models.RunRunning).Count(&runningExecutions).Error; err != nil {
		return nil, err
	}

	since24h := now.Add(-24 * time.Hour)
	since7d := now.Add(-7 * 24 * time.Hour)

	var runs []models.Run
	if err := q.db.Select("id, flow_id, status, started_at, completed_at, duration_ms").
		Where("started_at >= ?", since7d).Find(&runs).Error; err != nil {
		return nil, err
	}

	// KPIs + status breakdown over the last 24h.
	var completed24h, failed24h, running24h, cancelled24h int
	var durationSum int64
	var durationCount int
	for _, r := range runs {
		switch r.Status {
		case models.RunCompleted:
			if r.StartedAt.After(since24h) {
				completed24h++
				durationSum += r.DurationMs
				durationCount++
			}
		case models.RunFailed:
			if r.StartedAt.After(since24h) {
				failed24h++
			}
		case models.RunRunning:
			if r.StartedAt.After(since24h) {
				running24h++
			}
		case models.RunCancelled:
			if r.StartedAt.After(since24h) {
				cancelled24h++
			}
		}
	}

	kpis := dto.KPIs{
		RunningTasks:      len(openTasks),
		StuckTasks:        len(stuckTasks),
		RunningExecutions: int(runningExecutions),
		Completed24h:      completed24h,
		Failed24h:         failed24h,
		Flows:             len(flows),
	}
	if durationCount > 0 {
		kpis.AvgCycleTimeMs = durationSum / int64(durationCount)
	}
	if finished := completed24h + failed24h; finished > 0 {
		kpis.SuccessRate = float64(completed24h) / float64(finished)
	}

	// 24h throughput, hourly buckets (all hours present so the chart is stable).
	buckets := map[time.Time]*dto.ThroughputPoint{}
	order := make([]time.Time, 0, 25)
	for i := 24; i >= 0; i-- {
		hour := hourBucket(now.Add(-time.Duration(i) * time.Hour))
		if _, ok := buckets[hour]; ok {
			continue
		}
		buckets[hour] = &dto.ThroughputPoint{Hour: hour}
		order = append(order, hour)
	}
	for _, r := range runs {
		if hour := hourBucket(r.StartedAt); hour.After(since24h.Add(-time.Hour)) {
			if point, ok := buckets[hour]; ok {
				point.Started++
			}
		}
		if r.CompletedAt != nil {
			if hour := hourBucket(*r.CompletedAt); hour.After(since24h.Add(-time.Hour)) {
				if point, ok := buckets[hour]; ok {
					if r.Status == models.RunFailed {
						point.Failed++
					} else {
						point.Completed++
					}
				}
			}
		}
	}
	throughput := make([]dto.ThroughputPoint, 0, len(order))
	for _, hour := range order {
		throughput = append(throughput, *buckets[hour])
	}

	// 7d trend, daily buckets.
	trendOrder := make([]time.Time, 0, 7)
	trendBuckets := map[time.Time]*dto.TrendPoint{}
	for i := 6; i >= 0; i-- {
		day := dayBucket(now.Add(-time.Duration(i) * 24 * time.Hour))
		if _, ok := trendBuckets[day]; ok {
			continue
		}
		trendBuckets[day] = &dto.TrendPoint{Day: day}
		trendOrder = append(trendOrder, day)
	}
	trendDurSum := map[time.Time]int64{}
	trendDurCnt := map[time.Time]int{}
	for _, r := range runs {
		if r.CompletedAt == nil {
			continue
		}
		day := dayBucket(*r.CompletedAt)
		point, ok := trendBuckets[day]
		if !ok {
			continue
		}
		if r.Status == models.RunFailed {
			point.Failed++
		} else {
			point.Completed++
			trendDurSum[day] += r.DurationMs
			trendDurCnt[day]++
		}
	}
	trend := make([]dto.TrendPoint, 0, len(trendOrder))
	for _, day := range trendOrder {
		point := trendBuckets[day]
		if total := point.Completed + point.Failed; total > 0 {
			point.SuccessRate = float64(point.Completed) / float64(total)
		}
		if cnt := trendDurCnt[day]; cnt > 0 {
			point.AvgDurationMs = trendDurSum[day] / int64(cnt)
		}
		trend = append(trend, *point)
	}

	statusBreakdown := []dto.StatusCount{
		{Status: string(models.RunRunning), Count: running24h},
		{Status: string(models.RunCompleted), Count: completed24h},
		{Status: string(models.RunFailed), Count: failed24h},
		{Status: string(models.RunCancelled), Count: cancelled24h},
	}

	var openAlerts int64
	if err := q.db.Model(&models.Alert{}).Where("resolved = ?", false).Count(&openAlerts).Error; err != nil {
		return nil, err
	}
	kpis.OpenAlerts = int(openAlerts)

	recentRuns, err := q.ListRuns(RunFilter{Page: 1, Size: 8, Status: "all"})
	if err != nil {
		return nil, err
	}
	stuck, err := q.ListStuck(6)
	if err != nil {
		return nil, err
	}
	alerts, err := q.ListAlerts("open", 6)
	if err != nil {
		return nil, err
	}

	return &dto.Dashboard{
		GeneratedAt:     now,
		KPIs:            kpis,
		Throughput:      throughput,
		Trend:           trend,
		StatusBreakdown: statusBreakdown,
		Flows:           flows,
		RecentRuns:      recentRuns.Items,
		Stuck:           stuck,
		RecentAlerts:    alerts,
	}, nil
}

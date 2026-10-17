package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/dto"
	"flowmonitor/backend/internal/models"
)

// ErrUnknownFlow is returned when a flow is not defined and auto-registration
// is disabled.
var ErrUnknownFlow = errors.New("unknown flow")

// Ingest turns external task events into runs + step runs + alerts.
//
// State machine:
//
//	started   → open (or create) the execution, open a task
//	completed → close the task; an `end` node closes the execution
//	failed    → close the task, fail the execution, raise a failure alert
//	skipped   → close the task without failing the execution
type Ingest struct {
	db  *gorm.DB
	cfg *config.Config
}

// Process applies a single event in its own transaction.
func (s *Ingest) Process(ctx context.Context, ev dto.IngestEvent) (*dto.IngestResult, error) {
	ev.Normalize()
	if err := ev.Validate(); err != nil {
		return nil, err
	}

	var result *dto.IngestResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res, err := s.ProcessTx(tx, ev)
		if err != nil {
			return err
		}
		result = res
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ProcessBatch applies many events, reporting a per-event outcome. Events that
// fail validation/state transitions are reported inline instead of aborting the
// whole batch, so one bad producer cannot block the others.
func (s *Ingest) ProcessBatch(ctx context.Context, events []dto.IngestEvent) []dto.IngestResult {
	results := make([]dto.IngestResult, 0, len(events))
	for i := range events {
		res, err := s.Process(ctx, events[i])
		if err != nil {
			results = append(results, dto.IngestResult{
				FlowKey:     events[i].FlowKey,
				NodeKey:     events[i].NodeKey,
				BusinessKey: events[i].BusinessKey,
				Status:      events[i].Status,
				Accepted:    false,
				Message:     err.Error(),
			})
			continue
		}
		results = append(results, *res)
	}
	return results
}

// ProcessTx applies one event inside a caller-provided transaction. Used by the
// simulator so a whole simulated execution commits atomically.
func (s *Ingest) ProcessTx(tx *gorm.DB, ev dto.IngestEvent) (*dto.IngestResult, error) {
	ev.Normalize()
	if err := ev.Validate(); err != nil {
		return nil, err
	}

	hash := ev.DedupeHash()
	var existing models.Event
	err := tx.Where("dedupe_hash = ?", hash).First(&existing).Error
	if err == nil {
		return &dto.IngestResult{
			FlowKey:     ev.FlowKey,
			NodeKey:     ev.NodeKey,
			BusinessKey: ev.BusinessKey,
			Status:      ev.Status,
			Accepted:    true,
			Duplicate:   true,
			RunID:       existing.RunID,
			StepRunID:   existing.StepRunID,
			Message:     "duplicate delivery ignored",
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	res := &dto.IngestResult{
		FlowKey:     ev.FlowKey,
		NodeKey:     ev.NodeKey,
		BusinessKey: ev.BusinessKey,
		Status:      ev.Status,
		Accepted:    true,
	}
	if err := s.apply(tx, &ev, hash, res); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Ingest) apply(tx *gorm.DB, ev *dto.IngestEvent, hash string, res *dto.IngestResult) error {
	flow, flowCreated, err := s.ensureFlow(tx, ev.FlowKey)
	if err != nil {
		return err
	}
	node, nodeCreated, err := s.ensureNode(tx, flow, ev.NodeKey)
	if err != nil {
		return err
	}
	if flowCreated || nodeCreated {
		res.AutoCreated = true
	}

	run, err := s.activeRun(tx, flow.ID, ev.BusinessKey)
	if err != nil {
		return err
	}

	var step *models.StepRun
	switch ev.Status {
	case "started":
		if run == nil {
			run = &models.Run{
				FlowID:         flow.ID,
				BusinessKey:    ev.BusinessKey,
				Status:         models.RunRunning,
				CurrentNodeKey: node.Key,
				StartedAt:      *ev.OccurredAt,
				InitiatedBy:    ev.InitiatedBy,
				Metadata:       toRawJSON(ev.Metadata),
			}
			if err := tx.Create(run).Error; err != nil {
				return err
			}
		}
		step, err = s.openStep(tx, run, node, ev)
	default: // completed | failed | skipped
		if run == nil {
			return fmt.Errorf("no active execution for business key %q in flow %q "+
				"(send a `started` event for the first step)", ev.BusinessKey, ev.FlowKey)
		}
		step, err = s.closeStep(tx, run, node, ev)
	}
	if err != nil {
		return err
	}

	if err := s.updateRun(tx, run, node, ev, step); err != nil {
		return err
	}

	event := models.Event{
		FlowKey:     ev.FlowKey,
		NodeKey:     node.Key,
		BusinessKey: ev.BusinessKey,
		Status:      ev.Status,
		OccurredAt:  *ev.OccurredAt,
		ReceivedAt:  time.Now().UTC(),
		Source:      ev.Source,
		RunID:       run.ID,
		StepRunID:   step.ID,
		Payload:     toRawJSON(ev),
		Error:       ev.Error,
		DedupeHash:  hash,
	}
	if err := tx.Create(&event).Error; err != nil {
		return err
	}

	res.RunID = run.ID
	res.StepRunID = step.ID
	res.RunStatus = string(run.Status)
	res.StepStatus = string(step.Status)
	if run.Status == models.RunFailed {
		res.Message = "execution marked as failed"
	}
	return nil
}

// ensureFlow fetches a flow by key, auto-registering it when allowed.
func (s *Ingest) ensureFlow(tx *gorm.DB, key string) (*models.Flow, bool, error) {
	var flow models.Flow
	err := tx.Where("key = ?", key).First(&flow).Error
	if err == nil {
		return &flow, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if !s.cfg.AutoRegister {
		return nil, false, fmt.Errorf("%w: %q (create the flow first or enable FLOWMON_AUTO_REGISTER)", ErrUnknownFlow, key)
	}

	flow = models.Flow{
		Key:         key,
		Name:        humanizeKey(key),
		Description: "Auto-registered from ingested events — complete the diagram and node contracts in the editor.",
		Category:    "auto-registered",
		Active:      true,
	}
	if err := tx.Create(&flow).Error; err != nil {
		return nil, false, err
	}
	return &flow, true, nil
}

// ensureNode fetches a node inside a flow, auto-registering it when allowed.
func (s *Ingest) ensureNode(tx *gorm.DB, flow *models.Flow, key string) (*models.FlowNode, bool, error) {
	var node models.FlowNode
	err := tx.Where("flow_id = ? AND key = ?", flow.ID, key).First(&node).Error
	if err == nil {
		return &node, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, err
	}
	if !s.cfg.AutoRegister {
		return nil, false, fmt.Errorf("unknown node %q in flow %q", key, flow.Key)
	}

	var count int64
	if err := tx.Model(&models.FlowNode{}).Where("flow_id = ?", flow.ID).Count(&count).Error; err != nil {
		return nil, false, err
	}
	node = models.FlowNode{
		FlowID:      flow.ID,
		Key:         key,
		Name:        humanizeKey(key),
		Kind:        models.KindTask,
		Description: "Auto-registered from ingested events.",
		PosX:        120 + float64(count%4)*240,
		PosY:        120 + float64(count/4)*180,
	}
	if err := tx.Create(&node).Error; err != nil {
		return nil, false, err
	}
	return &node, true, nil
}

func (s *Ingest) activeRun(tx *gorm.DB, flowID uint, businessKey string) (*models.Run, error) {
	var run models.Run
	err := tx.Where("flow_id = ? AND business_key = ? AND status = ?", flowID, businessKey, models.RunRunning).
		Order("id desc").First(&run).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &run, nil
}

// openStep returns the task to open for a `started` event. A second `started`
// for a task that is already open is treated as an in-flight retry (attempt+1).
func (s *Ingest) openStep(tx *gorm.DB, run *models.Run, node *models.FlowNode, ev *dto.IngestEvent) (*models.StepRun, error) {
	var open models.StepRun
	err := tx.Where("run_id = ? AND node_key = ? AND status = ?", run.ID, node.Key, models.StepRunning).
		Order("id desc").First(&open).Error
	if err == nil {
		updates := map[string]any{
			"attempt":         open.Attempt + 1,
			"request_payload": firstNonEmpty(string(ev.Request), open.RequestPayload),
			"source":          firstNonEmpty(ev.Source, open.Source),
			"actor":           firstNonEmpty(ev.Actor, open.Actor),
		}
		if ev.Metadata != nil {
			updates["metadata"] = toRawJSON(ev.Metadata)
		}
		if err := tx.Model(&models.StepRun{}).Where("id = ?", open.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		open.Attempt++
		return &open, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	seq, err := s.nextSequence(tx, run.ID)
	if err != nil {
		return nil, err
	}
	step := models.StepRun{
		RunID:          run.ID,
		FlowID:         run.FlowID,
		NodeKey:        node.Key,
		NodeName:       node.Name,
		NodeKind:       node.Kind,
		Status:         models.StepRunning,
		Sequence:       seq,
		Attempt:        ev.Attempt,
		StartedAt:      *ev.OccurredAt,
		RequestPayload: string(ev.Request),
		Source:         ev.Source,
		Actor:          ev.Actor,
		Metadata:       toRawJSON(ev.Metadata),
	}
	if err := tx.Create(&step).Error; err != nil {
		return nil, err
	}
	return &step, nil
}

// closeStep finishes the open task of a node. When no task was open (single-shot
// instrumentation that only reports the terminal event) a closed task is created.
func (s *Ingest) closeStep(tx *gorm.DB, run *models.Run, node *models.FlowNode, ev *dto.IngestEvent) (*models.StepRun, error) {
	status := models.StepStatus(ev.Status)

	var step models.StepRun
	err := tx.Where("run_id = ? AND node_key = ? AND status = ?", run.ID, node.Key, models.StepRunning).
		Order("id desc").First(&step).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		seq, seqErr := s.nextSequence(tx, run.ID)
		if seqErr != nil {
			return nil, seqErr
		}
		step = models.StepRun{
			RunID:     run.ID,
			FlowID:    run.FlowID,
			NodeKey:   node.Key,
			NodeName:  node.Name,
			NodeKind:  node.Kind,
			Status:    status,
			Sequence:  seq,
			Attempt:   ev.Attempt,
			StartedAt: ev.OccurredAt.Add(-time.Duration(maxInt64(ev.DurationMs, 0)) * time.Millisecond),
			Source:    ev.Source,
			Actor:     ev.Actor,
		}
	case err != nil:
		return nil, err
	}

	done := *ev.OccurredAt
	durationMs := ev.DurationMs
	if durationMs <= 0 {
		durationMs = done.Sub(step.StartedAt).Milliseconds()
	}
	durationMs = maxInt64(durationMs, 0)

	step.Status = status
	step.CompletedAt = &done
	step.DurationMs = durationMs
	step.ResponsePayload = firstNonEmpty(string(ev.Response), step.ResponsePayload)
	step.Error = ev.Error
	step.Source = firstNonEmpty(ev.Source, step.Source)
	step.Actor = firstNonEmpty(ev.Actor, step.Actor)
	if ev.Metadata != nil {
		step.Metadata = toRawJSON(ev.Metadata)
	}

	if step.ID == 0 {
		if err := tx.Create(&step).Error; err != nil {
			return nil, err
		}
	} else if err := tx.Save(&step).Error; err != nil {
		return nil, err
	}
	return &step, nil
}

// updateRun advances the execution: current node, terminal status, alerts.
func (s *Ingest) updateRun(tx *gorm.DB, run *models.Run, node *models.FlowNode, ev *dto.IngestEvent, step *models.StepRun) error {
	updates := map[string]any{"current_node_key": node.Key}

	switch ev.Status {
	case "failed":
		done := *ev.OccurredAt
		run.Status = models.RunFailed
		run.CompletedAt = &done
		run.DurationMs = maxInt64(done.Sub(run.StartedAt).Milliseconds(), 0)
		run.FailureError = firstNonEmpty(ev.Error, fmt.Sprintf("task %q failed", node.Key))
		updates["status"] = run.Status
		updates["completed_at"] = done
		updates["duration_ms"] = run.DurationMs
		updates["failure_error"] = run.FailureError
		if err := s.raiseFailureAlert(tx, run, node, ev, step); err != nil {
			return err
		}
	case "completed":
		if node.Kind == models.KindEnd {
			done := *ev.OccurredAt
			run.Status = models.RunCompleted
			run.CompletedAt = &done
			run.DurationMs = maxInt64(done.Sub(run.StartedAt).Milliseconds(), 0)
			updates["status"] = run.Status
			updates["completed_at"] = done
			updates["duration_ms"] = run.DurationMs
		}
	}

	return tx.Model(&models.Run{}).Where("id = ?", run.ID).Updates(updates).Error
}

func (s *Ingest) raiseFailureAlert(tx *gorm.DB, run *models.Run, node *models.FlowNode, ev *dto.IngestEvent, step *models.StepRun) error {
	detail := ev.Error
	if detail == "" {
		detail = "no error message provided"
	}
	alert := models.Alert{
		FlowID:    run.FlowID,
		FlowKey:   ev.FlowKey,
		RunID:     run.ID,
		NodeKey:   node.Key,
		Type:      models.AlertFailure,
		Severity:  "critical",
		Message:   fmt.Sprintf("Task %q failed for %s: %s", node.Name, ev.BusinessKey, detail),
		CreatedAt: time.Now().UTC(),
		DedupeKey: fmt.Sprintf("failure:%d", step.ID),
	}
	return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&alert).Error
}

func (s *Ingest) nextSequence(tx *gorm.DB, runID uint) (int, error) {
	var count int64
	if err := tx.Model(&models.StepRun{}).Where("run_id = ?", runID).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count) + 1, nil
}

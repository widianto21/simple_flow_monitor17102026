package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/models"
)

// OpenTask is a currently running task prepared for SLA evaluation.
type OpenTask struct {
	StepRunID   uint
	FlowID      uint
	FlowKey     string
	RunID       uint
	BusinessKey string
	NodeKey     string
	NodeName    string
	StartedAt   time.Time
	Timeout     int // minutes
}

// Elapsed is how long the task has been running.
func (t OpenTask) Elapsed(now time.Time) time.Duration { return now.Sub(t.StartedAt) }

// Overdue is how much longer than its SLA the task has been running.
func (t OpenTask) Overdue(now time.Time) time.Duration {
	return t.Elapsed(now) - time.Duration(t.Timeout)*time.Minute
}

// DetectStuck keeps the tasks whose elapsed time exceeds their SLA. Tasks
// without an SLA (timeout <= 0) are never stuck — setting
// FLOWMON_DEFAULT_SLA_MINUTES=0 therefore disables stuck detection.
// Pure function — the core of the stuck view, unit-tested without a database.
func DetectStuck(tasks []OpenTask, now time.Time) []OpenTask {
	out := make([]OpenTask, 0, len(tasks))
	for _, t := range tasks {
		if t.Timeout <= 0 {
			continue
		}
		if t.Overdue(now) > 0 {
			out = append(out, t)
		}
	}
	return out
}

// LoadOpenTasks returns every running task with its resolved SLA.
func LoadOpenTasks(db *gorm.DB, defaultSLAMinutes int) ([]OpenTask, error) {
	var steps []models.StepRun
	if err := db.Where("status = ?", models.StepRunning).Find(&steps).Error; err != nil {
		return nil, err
	}
	if len(steps) == 0 {
		return nil, nil
	}

	flowIDs := make([]uint, 0, len(steps))
	runIDs := make([]uint, 0, len(steps))
	for _, s := range steps {
		flowIDs = append(flowIDs, s.FlowID)
		runIDs = append(runIDs, s.RunID)
	}
	flowIDs = distinctIDs(flowIDs)
	runIDs = distinctIDs(runIDs)

	flowKeys := map[uint]string{}
	var flows []models.Flow
	if err := db.Where("id IN ?", flowIDs).Find(&flows).Error; err != nil {
		return nil, err
	}
	for _, f := range flows {
		flowKeys[f.ID] = f.Key
	}

	nodeIdx := map[string]models.FlowNode{}
	var nodes []models.FlowNode
	if err := db.Where("flow_id IN ?", flowIDs).Find(&nodes).Error; err != nil {
		return nil, err
	}
	for _, n := range nodes {
		nodeIdx[fmt.Sprintf("%d:%s", n.FlowID, n.Key)] = n
	}

	runIdx := map[uint]models.Run{}
	var runs []models.Run
	if err := db.Where("id IN ?", runIDs).Find(&runs).Error; err != nil {
		return nil, err
	}
	for _, r := range runs {
		runIdx[r.ID] = r
	}

	tasks := make([]OpenTask, 0, len(steps))
	for _, s := range steps {
		node, hasNode := nodeIdx[fmt.Sprintf("%d:%s", s.FlowID, s.NodeKey)]
		timeout := defaultSLAMinutes
		if hasNode {
			timeout = resolveTimeout(&node, defaultSLAMinutes)
		}
		tasks = append(tasks, OpenTask{
			StepRunID:   s.ID,
			FlowID:      s.FlowID,
			FlowKey:     flowKeys[s.FlowID],
			RunID:       s.RunID,
			BusinessKey: runIdx[s.RunID].BusinessKey,
			NodeKey:     s.NodeKey,
			NodeName:    s.NodeName,
			StartedAt:   s.StartedAt,
			Timeout:     timeout,
		})
	}
	return tasks, nil
}

// Watcher periodically materializes stuck alerts and resolves the ones whose
// task has finished (or whose flow disappeared).
type Watcher struct {
	db       *gorm.DB
	cfg      *config.Config
	interval time.Duration
}

// NewWatcher builds the stuck/failure alert watcher.
func NewWatcher(db *gorm.DB, cfg *config.Config) *Watcher {
	interval := time.Duration(cfg.StuckCheckIntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &Watcher{db: db, cfg: cfg, interval: interval}
}

// Start runs the watcher until the context is cancelled.
func (w *Watcher) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	w.Scan(time.Now().UTC())
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.Scan(time.Now().UTC())
		}
	}
}

// Scan raises alerts for tasks over SLA and resolves cleared ones.
func (w *Watcher) Scan(now time.Time) {
	tasks, err := LoadOpenTasks(w.db, w.cfg.DefaultSLAMinutes)
	if err != nil {
		log.Printf("stuck watcher: loading open tasks: %v", err)
		return
	}

	running := make(map[uint]struct{}, len(tasks))
	for _, t := range tasks {
		running[t.StepRunID] = struct{}{}
	}

	raised := 0
	for _, t := range DetectStuck(tasks, now) {
		severity := "warning"
		if t.Overdue(now) > time.Duration(t.Timeout)*time.Minute {
			severity = "critical" // more than 2× the SLA
		}
		alert := models.Alert{
			FlowID:    t.FlowID,
			FlowKey:   t.FlowKey,
			RunID:     t.RunID,
			NodeKey:   t.NodeKey,
			Type:      models.AlertStuck,
			Severity:  severity,
			Message:   fmt.Sprintf("Task %q for %s has been running for %s (SLA %sm)", t.NodeName, t.BusinessKey, humanDuration(t.Elapsed(now)), strconv.Itoa(t.Timeout)),
			CreatedAt: now,
			DedupeKey: stuckKey(t.StepRunID),
		}
		res := w.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&alert)
		if res.Error != nil {
			log.Printf("stuck watcher: raising alert: %v", res.Error)
			continue
		}
		raised += int(res.RowsAffected)
	}

	// Resolve stuck alerts whose task is no longer running.
	var openAlerts []models.Alert
	if err := w.db.Where("type = ? AND resolved = ?", models.AlertStuck, false).Find(&openAlerts).Error; err != nil {
		log.Printf("stuck watcher: loading alerts: %v", err)
		return
	}
	for _, a := range openAlerts {
		stepID, err := parseStuckKey(a.DedupeKey)
		if err != nil {
			continue
		}
		if _, stillRunning := running[stepID]; stillRunning {
			continue
		}
		if err := w.db.Model(&models.Alert{}).Where("id = ?", a.ID).
			Updates(map[string]any{"resolved": true, "resolved_at": now}).Error; err != nil {
			log.Printf("stuck watcher: resolving alert %d: %v", a.ID, err)
		}
	}

	if raised > 0 {
		log.Printf("stuck watcher: raised %d new stuck alert(s)", raised)
	}
}

func stuckKey(stepRunID uint) string {
	return fmt.Sprintf("stuck:%d", stepRunID)
}

func parseStuckKey(key string) (uint, error) {
	raw := strings.TrimPrefix(key, "stuck:")
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(id), nil
}

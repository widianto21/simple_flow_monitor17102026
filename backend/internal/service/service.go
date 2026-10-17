package service

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"

	"flowmonitor/backend/internal/config"
	"flowmonitor/backend/internal/models"
)

// Services bundles the domain services that share the database and config.
type Services struct {
	DB       *gorm.DB
	Cfg      *config.Config
	Ingest   *Ingest
	Query    *Query
	Simulate *Simulator
}

// New wires the services together.
func New(db *gorm.DB, cfg *config.Config) *Services {
	s := &Services{DB: db, Cfg: cfg}
	s.Ingest = &Ingest{db: db, cfg: cfg}
	s.Query = &Query{db: db, cfg: cfg}
	s.Simulate = &Simulator{db: db, cfg: cfg, ingest: s.Ingest}
	return s
}

// ---------------------------------------------------------------------------
// shared helpers
// ---------------------------------------------------------------------------

// rawOrNull turns a stored JSON string into json.RawMessage, or nil when empty.
// Non-JSON text is returned as a JSON string so the API never emits invalid JSON.
func rawOrNull(s string) json.RawMessage {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if json.Valid([]byte(s)) {
		return json.RawMessage(s)
	}
	b, err := json.Marshal(s)
	if err != nil {
		return nil
	}
	return b
}

// toRawJSON marshals a value for storage in a TEXT column.
func toRawJSON(v any) string {
	if v == nil {
		return ""
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// resolveTimeout returns the SLA of a node, falling back to the global default.
func resolveTimeout(node *models.FlowNode, def int) int {
	if node != nil && node.TimeoutMinutes > 0 {
		return node.TimeoutMinutes
	}
	return def
}

func isOverdue(startedAt time.Time, timeoutMinutes int, now time.Time) bool {
	if timeoutMinutes <= 0 {
		return false
	}
	return now.Sub(startedAt) > time.Duration(timeoutMinutes)*time.Minute
}

// humanizeKey turns "order-fulfillment" into "Order Fulfillment".
func humanizeKey(key string) string {
	replaced := strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(key)
	words := strings.Fields(replaced)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}

// businessKeyPrefix builds a readable prefix for simulated business keys.
func businessKeyPrefix(flowKey string) string {
	first := strings.Split(flowKey, "-")[0]
	if len(first) > 4 {
		first = first[:4]
	}
	return strings.ToUpper(first)
}

// humanDuration renders a duration compactly ("3d 4h", "12m 30s", "840ms").
func humanDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd %dh", int(d.Hours())/24, int(d.Hours())%24)
	case d >= time.Hour:
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	case d >= time.Minute:
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	default:
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// p95Index returns the index of the p95 element of a sorted slice.
func p95Index(n int) int {
	if n <= 0 {
		return 0
	}
	i := int(float64(n) * 0.95)
	if i >= n {
		i = n - 1
	}
	return i
}

func sortedCopy(values []int64) []int64 {
	out := make([]int64, len(values))
	copy(out, values)
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// normalizePage clamps page/size into safe bounds.
func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	if size > 200 {
		size = 200
	}
	return page, size
}

// distinctIDs collects unique non-zero ids.
func distinctIDs(ids []uint) []uint {
	seen := make(map[uint]struct{}, len(ids))
	out := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

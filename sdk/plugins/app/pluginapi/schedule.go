package pluginapi

import (
	"fmt"
	"strings"
	"time"
)

// Schedule binds a stable identity to an action and administrator-owned config.
// Multiple schedules may invoke different actions using different cron fields.
type Schedule struct {
	ID           string `json:"id"`
	Label        string `json:"label"`
	Action       string `json:"action"`
	CronField    string `json:"cron_field"`
	EnabledField string `json:"enabled_field"`
	Notify       bool   `json:"notify,omitempty"`
	CatchUp      bool   `json:"catch_up,omitempty"`
}

// ScheduleTrigger survives queue persistence and is available in the WASI SDK.
type ScheduleTrigger struct {
	ID          string `json:"id"`
	Cron        string `json:"cron"`
	ScheduledAt string `json:"scheduled_at"`
}

type ScheduleStatus struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Action    string `json:"action"`
	Cron      string `json:"cron"`
	Enabled   bool   `json:"enabled"`
	CatchUp   bool   `json:"catch_up,omitempty"`
	NextRunAt string `json:"next_run_at,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (d Definition) validateSchedules(fields map[string]Field, actions map[string]bool) error {
	if len(d.Schedules) > 16 {
		return fmt.Errorf("插件定时任务最多注册 16 项")
	}
	permissions := map[string]bool{}
	for _, p := range d.Permissions {
		permissions[p] = true
	}
	seen := map[string]bool{}
	for _, s := range d.Schedules {
		if !permissions["schedule"] || !identifier.MatchString(s.ID) || seen[s.ID] || strings.TrimSpace(s.Label) == "" || len(s.Label) > 128 || !actions[s.Action] || fields[s.CronField].Type != "cron" || fields[s.EnabledField].Type != "boolean" {
			return fmt.Errorf("插件定时任务注册无效：%s", s.ID)
		}
		if s.Notify && !permissions["notification.send"] {
			return fmt.Errorf("定时任务通知需要 notification.send 权限")
		}
		seen[s.ID] = true
	}
	return nil
}

// RegisteredSchedules accepts older daily-checkin manifests without choosing an
// arbitrary first action. New plugins should always declare schedules explicitly.
func (d Definition) RegisteredSchedules() []Schedule {
	if len(d.Schedules) > 0 {
		return d.Schedules
	}
	fields := map[string]string{}
	for _, f := range d.Fields {
		fields[f.Key] = f.Type
	}
	if fields["scheduled"] != "boolean" || fields["cron"] != "cron" {
		return nil
	}
	action := ""
	for _, a := range d.Actions {
		if a.ID == "checkin" {
			action = a.ID
			break
		}
	}
	if action == "" && len(d.Actions) == 1 {
		action = d.Actions[0].ID
	}
	if action == "" {
		return nil
	}
	return []Schedule{{ID: "default", Label: "定时运行", Action: action, CronField: "cron", EnabledField: "scheduled", CatchUp: action == "checkin"}}
}

func ScheduleEnabled(snapshot Snapshot, schedule Schedule) bool {
	return snapshot.State.Enabled && snapshot.State.Configured && snapshot.Config[schedule.EnabledField] == true
}

// ValidateScheduleConfig runs before adapters persist any submitted values.
func ValidateScheduleConfig(d Definition, config map[string]any) error {
	for _, schedule := range d.RegisteredSchedules() {
		if config[schedule.EnabledField] != true {
			continue
		}
		cron, _ := config[schedule.CronField].(string)
		if err := ValidateCron(cron); err != nil {
			return fmt.Errorf("%s：%w", schedule.Label, err)
		}
	}
	return nil
}

// ValidateScheduleTrigger revokes queued schedules when their registration,
// plugin enablement or saved schedule configuration changes before execution.
func ValidateScheduleTrigger(snapshot Snapshot, request Request) (Schedule, error) {
	if request.Schedule == nil {
		return Schedule{}, fmt.Errorf("缺少定时任务来源")
	}
	trigger := request.Schedule
	at, err := time.Parse(time.RFC3339, trigger.ScheduledAt)
	for _, schedule := range snapshot.Definition.RegisteredSchedules() {
		if schedule.ID != trigger.ID {
			continue
		}
		cron, _ := snapshot.Config[schedule.CronField].(string)
		if err == nil && ScheduleEnabled(snapshot, schedule) && schedule.Action == request.ActionID && strings.Join(strings.Fields(cron), " ") == strings.Join(strings.Fields(trigger.Cron), " ") && ValidateCron(cron) == nil && at.Second() == 0 && CronMatches(cron, at) {
			return schedule, nil
		}
		break
	}
	return Schedule{}, fmt.Errorf("定时任务已关闭、配置已变化或注册已移除")
}

func ScheduleStatuses(snapshot Snapshot, now time.Time) []ScheduleStatus {
	var statuses []ScheduleStatus
	for _, s := range snapshot.Definition.RegisteredSchedules() {
		cron, _ := snapshot.Config[s.CronField].(string)
		status := ScheduleStatus{ID: s.ID, Label: s.Label, Action: s.Action, Cron: cron, Enabled: ScheduleEnabled(snapshot, s), CatchUp: s.CatchUp}
		if status.Enabled {
			if err := ValidateCron(cron); err != nil {
				status.Error = err.Error()
			} else if at, ok := NextCronTime(cron, now); ok {
				status.NextRunAt = at.Format(time.RFC3339)
			} else {
				status.Error = "定时规则没有有效执行日期"
			}
		}
		statuses = append(statuses, status)
	}
	return statuses
}

// NextCronTime uses calendar days instead of scanning every minute of years.
// It preserves the existing AND semantics for day-of-month and day-of-week.
func NextCronTime(cron string, now time.Time) (time.Time, bool) {
	if ValidateCron(cron) != nil {
		return time.Time{}, false
	}
	fields := strings.Fields(cron)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	limit := day.AddDate(8, 0, 1)
	for ; day.Before(limit); day = day.AddDate(0, 0, 1) {
		weekday := int(day.Weekday())
		if !cronFieldMatches(fields[4], int(day.Month()), 1, 12) || !cronFieldMatches(fields[3], day.Day(), 1, 31) || !(cronFieldMatches(fields[5], weekday, 0, 7) || weekday == 0 && cronFieldMatches(fields[5], 7, 0, 7)) {
			continue
		}
		for hour := 0; hour < 24; hour++ {
			if !cronFieldMatches(fields[2], hour, 0, 23) {
				continue
			}
			for minute := 0; minute < 60; minute++ {
				if !cronFieldMatches(fields[1], minute, 0, 59) {
					continue
				}
				at := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, now.Location())
				if at.After(now) {
					return at, true
				}
			}
		}
	}
	return time.Time{}, false
}

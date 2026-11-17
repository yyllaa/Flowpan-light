package pluginapi

import (
	"encoding/json"
	"testing"
	"time"
)

func scheduleDefinition() Definition {
	return Definition{SchemaVersion: 2, APIVersion: "1", ID: "future-plugin", Name: "Future", Version: "1", Runtime: "wasm", ConfigVersion: 1,
		Permissions: []string{"schedule", "notification.send"},
		Fields:      []Field{{Key: "on", Label: "On", Type: "boolean"}, {Key: "when", Label: "When", Type: "cron"}},
		Actions:     []Action{{ID: "status", Label: "Status", Kind: "probe"}, {ID: "work", Label: "Work", Kind: "run"}},
		Schedules:   []Schedule{{ID: "daily", Label: "Daily", Action: "work", CronField: "when", EnabledField: "on", Notify: true}}}
}

func TestScheduleRegistrationValidation(t *testing.T) {
	for _, name := range []string{"valid", "duplicate", "missing-action", "wrong-cron", "wrong-switch", "missing-permission", "notification-permission", "too-many"} {
		t.Run(name, func(t *testing.T) {
			d := scheduleDefinition()
			switch name {
			case "duplicate":
				d.Schedules = append(d.Schedules, d.Schedules[0])
			case "missing-action":
				d.Schedules[0].Action = "absent"
			case "wrong-cron":
				d.Schedules[0].CronField = "on"
			case "wrong-switch":
				d.Schedules[0].EnabledField = "when"
			case "missing-permission":
				d.Permissions = []string{"notification.send"}
			case "notification-permission":
				d.Permissions = []string{"schedule"}
			case "too-many":
				for len(d.Schedules) < 17 {
					d.Schedules = append(d.Schedules, d.Schedules[0])
				}
			}
			if err := d.Validate(); (err == nil) != (name == "valid") {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
}

func TestScheduleLegacySelectsCheckinNotFirstAction(t *testing.T) {
	d := scheduleDefinition()
	d.Schedules = nil
	d.Fields = []Field{{Key: "scheduled", Label: "On", Type: "boolean"}, {Key: "cron", Label: "When", Type: "cron"}}
	d.Actions[1].ID = "checkin"
	if s := d.RegisteredSchedules(); len(s) != 1 || s[0].Action != "checkin" || s[0].ID != "default" || !s[0].CatchUp {
		t.Fatal(s)
	}
	d.Actions[1].ID = "unlock"
	if len(d.RegisteredSchedules()) != 0 {
		t.Fatal("ambiguous legacy actions must not run automatically")
	}
	d.Actions = d.Actions[1:]
	if s := d.RegisteredSchedules(); len(s) != 1 || s[0].Action != "unlock" || s[0].CatchUp {
		t.Fatal(s)
	}
}

func TestScheduleConfigAndQueuedRevocation(t *testing.T) {
	d := scheduleDefinition()
	valid := map[string]any{"on": true, "when": "0 0 8 * * *"}
	if err := ValidateScheduleConfig(d, valid); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScheduleConfig(d, map[string]any{"on": true, "when": "not a cron"}); err == nil {
		t.Fatal("invalid enabled cron accepted")
	}
	if err := ValidateScheduleConfig(d, map[string]any{"on": false, "when": ""}); err != nil {
		t.Fatal(err)
	}
	r := Request{PluginID: d.ID, ActionID: "work", Schedule: &ScheduleTrigger{ID: "daily", Cron: "0 0 8 * * *", ScheduledAt: "2026-11-07T08:00:00+08:00"}}
	for _, name := range []string{"valid", "plugin-off", "unconfigured", "schedule-off", "changed-cron", "removed", "wrong-action", "invalid-time", "nonzero-second"} {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(Snapshot{Definition: d, Config: valid, State: State{Enabled: true, Configured: true}})
			var snap Snapshot
			_ = json.Unmarshal(b, &snap)
			b, _ = json.Marshal(r)
			var req Request
			_ = json.Unmarshal(b, &req)
			switch name {
			case "plugin-off":
				snap.State.Enabled = false
			case "unconfigured":
				snap.State.Configured = false
			case "schedule-off":
				snap.Config["on"] = false
			case "changed-cron":
				snap.Config["when"] = "0 1 8 * * *"
			case "removed":
				snap.Definition.Schedules = nil
			case "wrong-action":
				req.ActionID = "status"
			case "invalid-time":
				req.Schedule.ScheduledAt = "bad"
			case "nonzero-second":
				req.Schedule.ScheduledAt = "2026-11-07T08:00:20+08:00"
			}
			if _, err := ValidateScheduleTrigger(snap, req); (err == nil) != (name == "valid") {
				t.Fatalf("%s: %v", name, err)
			}
		})
	}
}

func TestScheduleNextTimeAndStatus(t *testing.T) {
	zone := time.FixedZone("CST", 8*60*60)
	for _, test := range []struct{ cron, from, want string }{
		{"0 0 8 * * *", "2026-11-07 07:59:20", "2026-11-07 08:00:00"},
		{"0 0 8 * * *", "2026-11-07 08:00:20", "2026-11-08 08:00:00"},
		{"0 */15 * * * *", "2026-11-07 08:14:59", "2026-11-07 08:15:00"},
		{"0 0 8 * * 7", "2026-11-07 08:00:20", "2026-11-08 08:00:00"},
		{"0 0 8 * * 0", "2026-11-07 08:00:20", "2026-11-08 08:00:00"},
		{"0 0 8 29 2 *", "2026-11-07 08:00:20", "2028-02-29 08:00:00"},
		{"0 0 8 31 2 *", "2026-11-07 08:00:20", ""},
		{"invalid", "2026-11-07 08:00:20", ""},
	} {
		from, _ := time.ParseInLocation(time.DateTime, test.from, zone)
		got, ok := NextCronTime(test.cron, from)
		if test.want == "" {
			if ok {
				t.Fatal(got)
			}
			continue
		}
		if !ok || got.Format(time.DateTime) != test.want || !CronMatches(test.cron, got) {
			t.Fatalf("%s: %v", test.cron, got)
		}
	}
	snap := Snapshot{Definition: scheduleDefinition(), Config: map[string]any{"on": true, "when": "0 0 8 * * *"}, State: State{Enabled: true, Configured: true}}
	status := ScheduleStatuses(snap, time.Date(2026, 11, 7, 8, 0, 20, 0, zone))
	if len(status) != 1 || status[0].NextRunAt != "2026-11-08T08:00:00+08:00" {
		t.Fatal(status)
	}
}

package main

import (
	"flowpan/app/pluginapi"
	"testing"
)

func TestSigninChoiceModesAndSharedDailyReceipt(t *testing.T) {
	for _, first := range []string{"checkin", "signin-lucky"} {
		t.Run(first, func(t *testing.T) {
			h := newAPIHost(t)
			r := pluginapi.Request{TaskID: "signin-choice", ActionID: first, Confirmed: first == "signin-lucky"}
			if _, err := executeOpenAPI(h, r); err != nil {
				t.Fatal(err)
			}
			want := "normal"
			if first == "signin-lucky" {
				want = "lucky"
			}
			if len(h.requests) != 1 || h.requests[0].Operation != "signin" || h.requests[0].Input["mode"] != want {
				t.Fatal("wrong sign-in mode", h.requests)
			}
			for _, next := range []string{"checkin", "signin-lucky", first} {
				r.ActionID, r.Confirmed = next, next == "signin-lucky"
				if _, err := executeOpenAPI(h, r); err != nil || len(h.requests) != 1 {
					t.Fatal("daily receipt did not prevent another sign-in", err, h.requests)
				}
			}
		})
	}
}

func TestScheduledSigninUsesSavedChoice(t *testing.T) {
	for _, mode := range []string{"", "checkin", "signin-lucky"} {
		t.Run(mode, func(t *testing.T) {
			h := newAPIHost(t)
			r := pluginapi.Request{TaskID: "scheduled-choice", ActionID: "checkin", Confirmed: true,
				Schedule: &pluginapi.ScheduleTrigger{ID: "default"}, Config: map[string]any{"signin_action": mode}}
			if _, err := executeOpenAPI(h, r); err != nil {
				t.Fatal(err)
			}
			want := "normal"
			if mode == "signin-lucky" {
				want = "lucky"
			}
			if len(h.requests) != 1 || h.requests[0].Input["mode"] != want {
				t.Fatal("saved schedule mode lost", h.requests)
			}
			r.Config["signin_action"] = "checkin"
			if _, err := executeOpenAPI(h, r); err != nil || len(h.requests) != 1 {
				t.Fatal("switching modes repeated today's sign-in", err, h.requests)
			}
		})
	}
}

func TestScheduledLuckyStillRequiresConfirmationAndManualChoiceWins(t *testing.T) {
	h := newAPIHost(t)
	r := pluginapi.Request{TaskID: "choice-confirm", ActionID: "checkin", Schedule: &pluginapi.ScheduleTrigger{ID: "default"}, Config: map[string]any{"signin_action": "signin-lucky"}}
	if _, err := executeOpenAPI(h, r); err == nil || len(h.requests) != 0 {
		t.Fatal("unconfirmed lucky sign-in executed", err, h.requests)
	}
	r.Schedule = nil
	if _, err := executeOpenAPI(h, r); err != nil || len(h.requests) != 1 || h.requests[0].Input["mode"] != "normal" {
		t.Fatal("saved schedule overrode explicit manual normal action", err, h.requests)
	}
}

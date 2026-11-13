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

package pluginwasm

import (
	"encoding/json"
	"flowpan/app/pluginapi"
)

// CheckRequirements discovers optional omissions without executing business tools.
// It returns the complete report even when a required interface is unavailable.
func CheckRequirements(required, optional *pluginapi.HostRequirements) (pluginapi.CompatibilityReport, error) {
	caps, err := Capabilities()
	if err != nil {
		return pluginapi.CompatibilityReport{}, err
	}
	c := caps.Contract
	if c.APIVersion == "" {
		c.APIVersion = caps.APIVersion
		c.Limits = map[string]int64{}
		raw, _ := json.Marshal(caps.Limits)
		_ = json.Unmarshal(raw, &c.Limits)
		for _, t := range caps.Tools {
			if t.Reason != "unavailable" {
				c.Tools = append(c.Tools, t.Name)
			}
		}
	}
	report := c.Check(required, optional, "")
	return report, report.Err()
}

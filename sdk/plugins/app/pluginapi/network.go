package pluginapi

import (
	"fmt"
	"strings"
)

// NetworkPolicy selects the host-managed route, never a guest-provided proxy URL.
type NetworkPolicy struct {
	Proxy string `json:"proxy"`
}

const (
	ProxyInherit = "inherit"
	ProxyDirect  = "direct"
	ProxyProject = "project"
)

func (d Definition) ProxyMode() string {
	if d.Network == nil {
		return ProxyInherit
	}
	return d.Network.Proxy
}

func (d Definition) validateNetwork() error {
	switch d.ProxyMode() {
	case ProxyInherit, ProxyDirect, ProxyProject:
		return nil
	default:
		return fmt.Errorf("插件网络代理声明无效")
	}
}

// ProxyURL is shared by HTTP, host SDKs, browser sessions and storage transfers.
// An explicit project declaration requires configuration; legacy plugins inherit
// the previous optional-proxy behavior until their manifests are updated.
func (d Definition) ProxyURL(projectProxy string) (string, error) {
	if err := d.validateNetwork(); err != nil {
		return "", err
	}
	if d.ProxyMode() == ProxyDirect {
		return "", nil
	}
	projectProxy = strings.TrimSpace(projectProxy)
	if d.ProxyMode() == ProxyProject && projectProxy == "" {
		return "", fmt.Errorf("插件需要代理，请先在设置中配置 HTTP 代理")
	}
	return projectProxy, nil
}

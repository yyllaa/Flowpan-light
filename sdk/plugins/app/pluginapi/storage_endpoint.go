package pluginapi

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// StorageEndpoint validates an account-owned base URL. Existing static host
// declarations retain their HTTPS-only policy. A prefix is an access boundary.
func StorageEndpoint(raw string) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(u.Host, "\\ \t\r\n") {
		return nil, fmt.Errorf("网盘服务地址无效")
	}
	if !storageURLPath(u) {
		return nil, fmt.Errorf("网盘服务地址路径无效")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawPath = ""
	return u, nil
}

func storageURLPath(u *url.URL) bool {
	p := u.Path
	for n := 0; n < 4; n++ {
		if strings.ContainsAny(p, "\\\x00\r\n") || (p != "" && path.Clean(p) != strings.TrimSuffix(p, "/") && p != "/") {
			return false
		}
		next, err := url.PathUnescape(p)
		if err != nil {
			return true
		}
		if next == p {
			return true
		}
		p = next
	}
	return false
}

func (d StorageDefinition) AllowsEndpointURL(settings map[string]any, target *url.URL) bool {
	if d.EndpointField == "" || target == nil || target.User != nil || target.Fragment != "" || !storageURLPath(target) {
		return false
	}
	raw, _ := settings[d.EndpointField].(string)
	base, err := StorageEndpoint(raw)
	if err != nil || target.Scheme != base.Scheme || !strings.EqualFold(target.Host, base.Host) {
		return false
	}
	return base.Path == "" || target.Path == base.Path || strings.HasPrefix(target.Path, base.Path+"/")
}

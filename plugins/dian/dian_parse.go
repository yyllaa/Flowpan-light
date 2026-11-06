// Adapted mechanically from 115-Station internal/api/dian.go. See NOTICE.md.
package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

type dianCfg struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func dianResourceKey(mediaType string, tmdbID int) string {
	s := []byte(fmt.Sprintf("1|tmdb|%s|%d|0", mediaType, tmdbID))
	mask := [4]byte{55, 161, 92, 233}
	for i := range s {
		s[i] ^= mask[i%4]
	}
	return base64.RawURLEncoding.EncodeToString(s)
}

// dianShare 一条分享（resource-detail 的 shares[]）。解锁过的才带链接字段
type dianShare struct {
	ID             int            `json:"id"`
	ResourceID     int            `json:"resource_id"`
	ShareKind      string         `json:"share_kind"`   // 115 / offline
	OfflineType    string         `json:"offline_type"` // magnet / ed2k …（share_kind=offline 时）
	Status         string         `json:"status"`
	MediaType      string         `json:"media_type"`
	Season         int            `json:"season"`
	SeasonsCSV     string         `json:"seasons_csv"`
	EpisodeCount   int            `json:"episode_count"`
	Episodes       string         `json:"episodes"` // "1,2,3"
	TitleOverride  string         `json:"title_override"`
	OfflineTitle   string         `json:"offline_title"`
	FileName       string         `json:"file_name"`
	FileExtension  string         `json:"file_extension"`
	FileList       []string       `json:"file_list"`
	HasChineseSub  bool           `json:"has_chinese_subtitle"`
	TagDecoded     map[string]any `json:"tag_decoded"`
	TotalSizeBytes int64          `json:"total_size_bytes"`
	TotalSizeHuman string         `json:"total_size_human"`
	UnlockCost     *int           `json:"unlock_cost"`
	IsUnlocked     bool           `json:"is_unlocked"`
	CreatedAt      string         `json:"created_at"`
	ResourceTitle  string         `json:"resource_title"`
	// 解锁过的才有
	URL         string   `json:"url"`
	URLs        []string `json:"urls"`
	URL115      string   `json:"url_115"`
	ShareCode   string   `json:"share_code"`
	ReceiveCode string   `json:"receive_code"`
}

// offline 离线类（磁力 / ed2k），解锁出来的不是 115 分享
func (s dianShare) offline() bool {
	return s.ShareKind == "offline" || s.ShareKind == "magnet"
}

// owned 已经解锁过（列表里直接带着链接，或者站点标了已解锁）
func (s dianShare) owned() bool {
	return s.IsUnlocked || s.URL115 != "" || s.ShareCode != "" || s.URL != "" || len(s.URLs) > 0
}

// title 展示 / 解析画质用的标题：分享者写的标题，后面补上站点解析出来的画质与季集（画质标签按标题解析）
func (s dianShare) title() string {
	name := firstNonEmpty(strings.TrimSpace(s.TitleOverride), strings.TrimSpace(s.OfflineTitle), strings.TrimSpace(s.FileName), s.ResourceTitle)
	var extra []string
	low := strings.ToLower(name)
	for _, k := range []string{"resolution", "source", "video_codec", "hdr", "audio_codec"} {
		v, _ := s.TagDecoded[k].(string)
		if v = strings.TrimSpace(v); v != "" && !strings.Contains(low, strings.ToLower(v)) {
			extra = append(extra, v)
		}
	}
	// 剧集把站点标的季集放在最前面：覆盖估计（resCoverageOf）取最左边的匹配，分享者常拿第一个文件名当标题，
	// 「绝命毒师 - S01E01 - 4K.WEB-DL…等7个文件」放在后面会被读成 E01-E04（2026-10-15 现场样本）
	if s.MediaType == "tv" {
		if m := s.seasonMark(); m != "" && !strings.Contains(strings.ToUpper(name), m) {
			name = "[" + m + "] " + name
		}
	}
	if len(extra) == 0 {
		return name
	}
	return name + " " + strings.Join(extra, " ")
}

// seasonMark 站点标的季集写成标题里认得出的样子（S05E01-E16 / S01-S03 / S05），给 resCoverageOf 估覆盖用
func (s dianShare) seasonMark() string {
	var seasons []int
	for _, f := range strings.Split(s.SeasonsCSV, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil && n > 0 {
			seasons = append(seasons, n)
		}
	}
	if len(seasons) == 0 && s.Season > 0 {
		seasons = []int{s.Season}
	}
	switch {
	case len(seasons) == 0:
		return ""
	case len(seasons) > 1:
		return fmt.Sprintf("S%02d-S%02d", seasons[0], seasons[len(seasons)-1])
	}
	var eps []int
	for _, f := range strings.Split(s.Episodes, ",") {
		if n, err := strconv.Atoi(strings.TrimSpace(f)); err == nil && n > 0 {
			eps = append(eps, n)
		}
	}
	if len(eps) == 0 {
		return fmt.Sprintf("S%02d", seasons[0])
	}
	lo, hi := eps[0], eps[0]
	for _, e := range eps {
		lo, hi = min(lo, e), max(hi, e)
	}
	// 中间缺集的不写范围：写成 E01-E16 会让订阅以为整段都有
	if hi-lo+1 != len(eps) {
		return fmt.Sprintf("S%02d", seasons[0])
	}
	if lo == hi {
		return fmt.Sprintf("S%02dE%02d", seasons[0], lo)
	}
	return fmt.Sprintf("S%02dE%02d-E%02d", seasons[0], lo, hi)
}

// parseDianDetail resource-detail 的返回 → 有效的分享
func parseDianDetail(raw []byte) ([]dianShare, error) {
	var data struct {
		Resource struct {
			Title     string `json:"title"`
			MediaType string `json:"media_type"`
		} `json:"resource"`
		Shares []dianShare `json:"shares"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("解析癫影资源列表失败: %v", err)
	}
	out := data.Shares[:0]
	for _, s := range data.Shares {
		if s.ID <= 0 || (s.Status != "" && s.Status != "active") {
			continue
		}
		if s.ResourceTitle == "" {
			s.ResourceTitle = data.Resource.Title
		}
		if s.MediaType == "" {
			s.MediaType = data.Resource.MediaType
		}
		out = append(out, s)
	}
	return out, nil
}

// dianListShares 按 TMDB 条目列分享。配了账号就登录后再读（才知道哪些已解锁），没配也能读。
// 缓存在上一层（searchResources 的 resCache），这里每次都真的去问
func dianRef(s dianShare) string { return strconv.Itoa(s.ID) + ":" + strconv.Itoa(s.ResourceID) }

func parseDianRef(ref string) (shareID, resourceID int, ok bool) {
	a, b, _ := strings.Cut(ref, ":")
	shareID, err := strconv.Atoi(a)
	if err != nil || shareID <= 0 {
		return 0, 0, false
	}
	resourceID, _ = strconv.Atoi(b)
	return shareID, resourceID, true
}

// ==================== 解锁 ====================

// dianUnlocked 解锁出来的东西：115 分享（Share + Code）或离线链接（Links，可能好几条）
type dianUnlocked struct {
	Share   string
	Code    string
	Links   []string
	Balance *int // 解锁后的积分余额（站点回 new_balance；不回这次扣了多少）
	Again   bool // 之前解锁过，这次没扣分
}

// parseDianUnlock 解锁返回 → 链接。字段对照站点的 UnlockResultDialog：
// 115 分享取 url_115，没有就用 share_code + receive_code 拼；离线取 urls，没有就把 url 按行拆。
// 2026-10-15 实测返回：{"already":false,"code":"ok","new_balance":353,"payload":{"share_kind":"115","url_115":"https://115.com/s/…?password=…",…}}
func parseDianUnlock(raw []byte) (*dianUnlocked, error) {
	var data struct {
		Already bool      `json:"already"`
		Balance *int      `json:"new_balance"`
		Payload dianShare `json:"payload"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("解析解锁结果失败: %v", err)
	}
	p := data.Payload
	out := &dianUnlocked{Again: data.Already, Balance: data.Balance}
	if !p.offline() {
		out.Share, out.Code = strings.TrimSpace(p.URL115), strings.TrimSpace(p.ReceiveCode)
		if out.Share == "" && is115ShareLink(p.ShareCode) {
			out.Share = strings.TrimSpace(p.ShareCode)
		}
		if out.Share == "" && p.ShareCode != "" {
			out.Share = "https://115cdn.com/s/" + strings.TrimSpace(p.ShareCode)
			if out.Code != "" {
				out.Share += "?password=" + out.Code
			}
		}
		if out.Share == "" && is115ShareLink(p.URL) {
			out.Share = strings.TrimSpace(p.URL)
		}
		if out.Share != "" {
			return out, nil
		}
	}
	links := p.URLs
	if len(links) == 0 {
		links = strings.Split(p.URL, "\n")
	}
	for _, l := range links {
		if l = strings.TrimSpace(l); l != "" {
			out.Links = append(out.Links, l)
		}
	}
	if len(out.Links) == 0 {
		return nil, fmt.Errorf("解锁成功了，但返回里没有链接")
	}
	return out, nil
}

// dianUnlock 解锁一条分享（扣积分；解锁过的再调不扣）。调用方负责积分确认
func dianCheckinText(raw []byte, points int) string {
	var res struct {
		Award int `json:"award"`
	}
	json.Unmarshal(raw, &res)
	msg := "签到完成，本次空签"
	if res.Award > 0 {
		msg = fmt.Sprintf("签到成功，获得 %d 积分", res.Award)
	} else if res.Award < 0 {
		msg = fmt.Sprintf("签到完成，扣了 %d 积分", -res.Award)
	}
	if points >= 0 {
		msg += fmt.Sprintf("，当前积分 %d", points)
	}
	return msg
}

// dianAlreadySigned 当天已签过的报错（站点没公开错误码，按 code 与文案都认）
func dianAlreadySigned(err error) bool {
	var ae *dianAPIErr
	if errors.As(err, &ae) && (strings.Contains(ae.Code, "already") || ae.Code == "conflict") {
		return true
	}
	s := err.Error()
	return strings.Contains(s, "已签") || strings.Contains(s, "已经签") || strings.Contains(strings.ToLower(s), "already")
}

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"

	"flowpan/sdk/pluginwasm"
)

type queryCache struct {
	Until int64       `json:"until"`
	Items []candidate `json:"items"`
}
type throttle struct {
	Next     int64 `json:"next"`
	Failures int   `json:"failures"`
}

func sources(c settings) []string {
	if c.Source == "auto" {
		return []string{"netease", "qq", "itunes"}
	}
	return []string{c.Source}
}
func interval(source string) int64 {
	switch source {
	case "itunes":
		return 4
	case "musicbrainz":
		return 2
	default:
		return 3
	}
}
func (e *engine) fetch(source string, request pluginwasm.HTTPRequest) (pluginwasm.HTTPResponse, error) {
	var gate throttle
	key := "rate:" + source
	if err := e.get(key, &gate); err != nil {
		return pluginwasm.HTTPResponse{}, err
	}
	now := e.now().Unix()
	if gate.Next > now {
		return pluginwasm.HTTPResponse{}, &deferred{gate.Next, "等待音源请求间隔"}
	}
	// Persist before sending, including failed requests and process termination.
	gate.Next = now + interval(source)
	if err := e.set(key, gate); err != nil {
		return pluginwasm.HTTPResponse{}, err
	}
	if request.Headers == nil {
		request.Headers = map[string]string{}
	}
	request.Headers["User-Agent"] = "FlowpanMusic/1.0.0 (https://github.com/yyllaa/Flowpan-light)"
	var response pluginwasm.HTTPResponse
	err := e.call("http.fetch", request, &response)
	if err != nil || response.Status == 403 || response.Status == 429 || response.Status >= 500 || strings.TrimSpace(response.Body) == "" {
		gate.Failures++
		wait := int64(30) * int64(1<<min(4, gate.Failures-1))
		if response.Status == 403 || response.Status == 429 {
			wait = int64(retrySeconds(response.Headers["Retry-After"], e.now()))
		}
		gate.Next = now + wait
		if setErr := e.set(key, gate); setErr != nil {
			return response, setErr
		}
		if gate.Failures >= 3 {
			return response, fmt.Errorf("音源 %s 连续失败，暂停后可手动继续；冷却 %d 秒", source, wait)
		}
		return response, &deferred{gate.Next, fmt.Sprintf("音源 %s 冷却，HTTP %d", source, response.Status)}
	}
	if response.Status != 200 {
		return response, fmt.Errorf("音源 %s 返回 HTTP %d", source, response.Status)
	}
	gate.Failures = 0
	if err = e.set(key, gate); err != nil {
		return response, err
	}
	return response, nil
}
func (e *engine) search(b *batch, w *work) ([]candidate, bool, error) {
	order := sources(b.Config)
	if w.Provider >= len(order) {
		return w.Candidates, false, nil
	}
	source := order[w.Provider]
	if strings.TrimSpace(w.Original.Title) == "" {
		return nil, false, nil
	}
	key := "search:" + digest([]string{source, normalized(w.Original.Title), normalized(w.Original.Artist)})
	var cached queryCache
	if err := e.get(key, &cached); err != nil {
		return nil, false, err
	}
	if cached.Until <= e.now().Unix() {
		req, err := searchRequest(source, w.Original)
		if err != nil {
			return nil, false, err
		}
		response, err := e.fetch(source, req)
		if err != nil {
			return nil, false, err
		}
		items, err := parseSearch(source, response.Body)
		if err != nil {
			return nil, false, err
		}
		ttl := int64(30 * 24 * 3600)
		if len(items) == 0 {
			ttl = 6 * 3600
		}
		cached = queryCache{Until: e.now().Unix() + ttl, Items: items}
		if err = e.set(key, cached); err != nil {
			return nil, false, err
		}
	}
	combined := append(append([]candidate{}, w.Candidates...), cached.Items...)
	ranked, accepted := rank(w.Original, combined)
	if len(ranked) > 12 {
		ranked = ranked[:12]
	}
	w.Candidates = ranked
	w.Provider++
	if !accepted && w.Provider < len(order) {
		return nil, false, &deferred{e.now().Unix() + 1, "当前音源未确认，查询备用音源"}
	}
	return ranked, accepted, nil
}
func searchRequest(source string, t tags) (pluginwasm.HTTPRequest, error) {
	query := strings.TrimSpace(t.Title + " " + t.Artist)
	switch source {
	case "netease":
		body, _ := json.Marshal(map[string]any{"s": query, "type": 1, "limit": 15, "offset": 0, "total": true})
		encrypted, err := weapi(body)
		return pluginwasm.HTTPRequest{URL: "https://music.163.com/weapi/cloudsearch/pc", Method: "POST", Body: encrypted, Headers: map[string]string{"Content-Type": "application/x-www-form-urlencoded", "Referer": "https://music.163.com/"}}, err
	case "qq":
		body, _ := json.Marshal(map[string]any{"req_0": map[string]any{"method": "DoSearchForQQMusicDesktop", "module": "music.search.SearchCgiService", "param": map[string]any{"search_type": 0, "query": query, "page_num": 1, "num_per_page": 15}}})
		return pluginwasm.HTTPRequest{URL: "https://u.y.qq.com/cgi-bin/musicu.fcg", Method: "POST", Body: string(body), Headers: map[string]string{"Content-Type": "application/json", "Referer": "https://i.y.qq.com/"}}, nil
	case "itunes":
		return pluginwasm.HTTPRequest{URL: "https://itunes.apple.com/search?" + url.Values{"term": {query}, "media": {"music"}, "entity": {"song"}, "country": {"CN"}, "limit": {"15"}}.Encode()}, nil
	case "musicbrainz":
		escape := func(s string) string { return strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) }
		q := `recording:"` + escape(t.Title) + `"`
		if t.Artist != "" {
			q += ` AND artist:"` + escape(t.Artist) + `"`
		}
		return pluginwasm.HTTPRequest{URL: "https://musicbrainz.org/ws/2/recording/?" + url.Values{"query": {q}, "fmt": {"json"}, "limit": {"15"}}.Encode()}, nil
	}
	return pluginwasm.HTTPRequest{}, fmt.Errorf("音源无效")
}
func parseSearch(source, body string) ([]candidate, error) {
	out := []candidate{}
	switch source {
	case "netease":
		var response struct {
			Code   int `json:"code"`
			Result struct {
				Songs []struct {
					ID       int64   `json:"id"`
					Name     string  `json:"name"`
					Duration float64 `json:"dt"`
					Track    int     `json:"no"`
					CD       string  `json:"cd"`
					Artists  []struct {
						Name string `json:"name"`
					} `json:"ar"`
					Album struct {
						Name    string `json:"name"`
						Cover   string `json:"picUrl"`
						Publish int64  `json:"publishTime"`
					} `json:"al"`
				} `json:"songs"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return nil, fmt.Errorf("网易响应格式无效")
		}
		if response.Code != 200 {
			return nil, fmt.Errorf("网易响应业务码 %d", response.Code)
		}
		for _, s := range response.Result.Songs {
			c := candidate{Source: source, ID: strconv.FormatInt(s.ID, 10), CoverURL: strings.Replace(s.Album.Cover, "http://", "https://", 1), Tags: tags{Duration: s.Duration / 1000}}
			c.Tags.Title = s.Name
			c.Tags.Album = s.Album.Name
			c.Tags.Disc = s.CD
			if s.Track > 0 {
				c.Tags.Track = strconv.Itoa(s.Track)
			}
			for _, a := range s.Artists {
				if c.Tags.Artist != "" {
					c.Tags.Artist += " / "
				}
				c.Tags.Artist += a.Name
			}
			if s.Album.Publish > 0 {
				c.Tags.Date = time.UnixMilli(s.Album.Publish).Format("2006")
			}
			out = append(out, c)
		}
	case "qq":
		var response struct {
			Code int `json:"code"`
			Req  struct {
				Code int `json:"code"`
				Data struct {
					Body struct {
						Song struct {
							List []struct {
								Mid      string  `json:"mid"`
								Title    string  `json:"title"`
								Name     string  `json:"name"`
								Interval float64 `json:"interval"`
								Track    int     `json:"index_album"`
								CD       int     `json:"index_cd"`
								Date     string  `json:"time_public"`
								Singer   []struct {
									Name string `json:"name"`
								} `json:"singer"`
								Album struct {
									Mid  string `json:"mid"`
									Name string `json:"name"`
								} `json:"album"`
							} `json:"list"`
						} `json:"song"`
					} `json:"body"`
				} `json:"data"`
			} `json:"req_0"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil {
			return nil, fmt.Errorf("QQ 响应格式无效")
		}
		if response.Code != 0 || response.Req.Code != 0 {
			return nil, fmt.Errorf("QQ 响应业务码 %d/%d", response.Code, response.Req.Code)
		}
		if !strings.Contains(body, `"req_0"`) {
			return nil, fmt.Errorf("QQ 响应缺少查询结果")
		}
		for _, s := range response.Req.Data.Body.Song.List {
			c := candidate{Source: source, ID: s.Mid, Tags: tags{Duration: s.Interval}}
			c.Tags.Title = html.UnescapeString(s.Title)
			if c.Tags.Title == "" {
				c.Tags.Title = html.UnescapeString(s.Name)
			}
			c.Tags.Album = html.UnescapeString(s.Album.Name)
			c.Tags.Date = s.Date
			if len(c.Tags.Date) > 4 {
				c.Tags.Date = c.Tags.Date[:4]
			}
			if s.Track > 0 {
				c.Tags.Track = strconv.Itoa(s.Track)
			}
			if s.CD > 0 {
				c.Tags.Disc = strconv.Itoa(s.CD)
			}
			for _, a := range s.Singer {
				if c.Tags.Artist != "" {
					c.Tags.Artist += " / "
				}
				c.Tags.Artist += html.UnescapeString(a.Name)
			}
			if s.Album.Mid != "" {
				c.CoverURL = "https://y.gtimg.cn/music/photo_new/T002R300x300M000" + url.PathEscape(s.Album.Mid) + ".jpg"
			}
			out = append(out, c)
		}
	case "itunes":
		var response struct {
			Count   int `json:"resultCount"`
			Results []struct {
				ID       int64   `json:"trackId"`
				Title    string  `json:"trackName"`
				Artist   string  `json:"artistName"`
				Album    string  `json:"collectionName"`
				Duration float64 `json:"trackTimeMillis"`
				Track    int     `json:"trackNumber"`
				Disc     int     `json:"discNumber"`
				Date     string  `json:"releaseDate"`
				Genre    string  `json:"primaryGenreName"`
				Cover    string  `json:"artworkUrl100"`
			} `json:"results"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil || !strings.Contains(body, `"resultCount"`) {
			return nil, fmt.Errorf("iTunes 响应格式无效")
		}
		for _, s := range response.Results {
			c := candidate{Source: source, ID: strconv.FormatInt(s.ID, 10), CoverURL: s.Cover, Tags: tags{Duration: s.Duration / 1000}}
			c.Tags.Title = s.Title
			c.Tags.Artist = s.Artist
			c.Tags.Album = s.Album
			c.Tags.AlbumArtist = s.Artist
			c.Tags.Genre = s.Genre
			c.Tags.Date = s.Date
			if len(c.Tags.Date) > 4 {
				c.Tags.Date = c.Tags.Date[:4]
			}
			if s.Track > 0 {
				c.Tags.Track = strconv.Itoa(s.Track)
			}
			if s.Disc > 0 {
				c.Tags.Disc = strconv.Itoa(s.Disc)
			}
			out = append(out, c)
		}
	case "musicbrainz":
		var response struct {
			Recordings []struct {
				ID      string  `json:"id"`
				Title   string  `json:"title"`
				Length  float64 `json:"length"`
				Artists []struct {
					Name   string `json:"name"`
					Artist struct {
						Name string `json:"name"`
					} `json:"artist"`
				} `json:"artist-credit"`
				Releases []struct {
					Title string `json:"title"`
					Date  string `json:"date"`
				} `json:"releases"`
			} `json:"recordings"`
		}
		if err := json.Unmarshal([]byte(body), &response); err != nil || !strings.Contains(body, `"recordings"`) {
			return nil, fmt.Errorf("MusicBrainz 响应格式无效")
		}
		for _, s := range response.Recordings {
			c := candidate{Source: source, ID: s.ID, Tags: tags{Duration: s.Length / 1000}}
			c.Tags.Title = s.Title
			for _, a := range s.Artists {
				name := a.Name
				if name == "" {
					name = a.Artist.Name
				}
				if c.Tags.Artist != "" {
					c.Tags.Artist += " / "
				}
				c.Tags.Artist += name
			}
			if len(s.Releases) > 0 {
				c.Tags.Album = s.Releases[0].Title
				c.Tags.Date = s.Releases[0].Date
				if len(c.Tags.Date) > 4 {
					c.Tags.Date = c.Tags.Date[:4]
				}
			}
			out = append(out, c)
		}
	}
	if len(out) > 15 {
		out = out[:15]
	}
	return out, nil
}
func weapi(plain []byte) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := range random {
		random[i] = alphabet[int(random[i])%len(alphabet)]
	}
	encrypt := func(data, key []byte) string {
		block, _ := aes.NewCipher(key)
		padding := aes.BlockSize - len(data)%aes.BlockSize
		data = append(append([]byte{}, data...), bytes.Repeat([]byte{byte(padding)}, padding)...)
		out := make([]byte, len(data))
		cipher.NewCBCEncrypter(block, []byte("0102030405060708")).CryptBlocks(out, data)
		return base64.StdEncoding.EncodeToString(out)
	}
	first := encrypt(plain, []byte("0CoJUm6Qyw8W8jud"))
	params := encrypt([]byte(first), random)
	for i, j := 0, len(random)-1; i < j; i, j = i+1, j-1 {
		random[i], random[j] = random[j], random[i]
	}
	modulus, _ := new(big.Int).SetString("00e0b509f6259df8642dbc35662901477df22677ec152b5ff68ace615bb7b725152b3ab17a876aea8a5aa76d2e417629ec4ee341f56135fccf695280104e0312ecbda92557c93870114af6c9d05c4f7f0c3685b7a46bee255932575cce10b424d813cfe4875d3e82047b97ddef52741d546b8e289dc6935b3ece0462db0a22b8e7", 16)
	value := new(big.Int).Exp(new(big.Int).SetBytes(random), big.NewInt(65537), modulus)
	return url.Values{"params": {params}, "encSecKey": {fmt.Sprintf("%0256x", value)}}.Encode(), nil
}
func (e *engine) lyrics(c candidate) (string, error) {
	if c.Source != "netease" && c.Source != "qq" {
		return "", nil
	}
	key := "lyrics:" + digest([]string{c.Source, c.ID})
	var cached struct {
		Until int64  `json:"until"`
		Text  string `json:"text"`
	}
	if err := e.get(key, &cached); err != nil {
		return "", err
	}
	if cached.Until > e.now().Unix() {
		return cached.Text, nil
	}
	request := pluginwasm.HTTPRequest{Headers: map[string]string{}}
	if c.Source == "netease" {
		request.URL = "https://music.163.com/api/song/lyric?os=pc&id=" + url.QueryEscape(c.ID) + "&lv=-1&kv=-1&tv=-1"
		request.Headers["Referer"] = "https://music.163.com/"
	} else {
		request.URL = "https://c.y.qq.com/lyric/fcgi-bin/fcg_query_lyric_new.fcg?songmid=" + url.QueryEscape(c.ID) + "&format=json&nobase64=1&g_tk=5381"
		request.Headers["Referer"] = "https://y.qq.com/"
	}
	response, err := e.fetch(c.Source, request)
	if err != nil {
		return "", err
	}
	if c.Source == "netease" {
		var raw struct {
			LRC struct {
				Lyric string `json:"lyric"`
			} `json:"lrc"`
		}
		if err = json.Unmarshal([]byte(response.Body), &raw); err != nil {
			return "", err
		}
		cached.Text = raw.LRC.Lyric
	} else {
		var raw struct {
			Lyric string `json:"lyric"`
		}
		if err = json.Unmarshal([]byte(response.Body), &raw); err != nil {
			return "", err
		}
		cached.Text = html.UnescapeString(raw.Lyric)
	}
	if cached.Text == "null" {
		cached.Text = ""
	}
	if len(cached.Text) > 16384 {
		return "", fmt.Errorf("歌词超过嵌入上限")
	}
	cached.Until = e.now().Unix() + 30*24*3600
	if cached.Text == "" {
		cached.Until = e.now().Unix() + 6*3600
	}
	return cached.Text, e.set(key, cached)
}

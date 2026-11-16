package main

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"flowpan/app/pluginapi"
	"flowpan/plugins/music/chineseconv"
	"flowpan/plugins/music/filenamerel"
)

type tags struct {
	pluginapi.MediaMetadata
	Native   pluginapi.MediaMetadata `json:"native,omitempty"`
	Duration float64                 `json:"duration,omitempty"`
	HasCover bool                    `json:"has_cover,omitempty"`
}
type candidate struct {
	Tags     tags    `json:"tags"`
	ID       string  `json:"id"`
	Source   string  `json:"source"`
	CoverURL string  `json:"cover_url,omitempty"`
	Score    float64 `json:"score"`
}

var simplified, _ = chineseconv.T2S()

func normalized(s string) string {
	if simplified != nil {
		s = simplified.Convert(s)
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return unicode.ToLower(r)
		}
		return -1
	}, s)
}
func similar(a, b string) float64 {
	a, b = normalized(a), normalized(b)
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 1
	}
	// Approximate titles are presented for review; never auto-accepted.
	ar, br := []rune(a), []rune(b)
	if len(ar) > 128 || len(br) > 128 {
		return 0
	}
	row := make([]int, len(br)+1)
	for j := range row {
		row[j] = j
	}
	for i, x := range ar {
		previous := row[0]
		row[0] = i + 1
		for j, y := range br {
			old := row[j+1]
			cost := 0
			if x != y {
				cost = 1
			}
			row[j+1] = min(row[j+1]+1, row[j]+1, previous+cost)
			previous = old
		}
	}
	return 1 - float64(row[len(br)])/float64(max(len(ar), len(br)))
}
func rank(input tags, choices []candidate) ([]candidate, bool) {
	unique := map[string]bool{}
	out := []candidate{}
	for _, c := range choices {
		if c.Tags.Title == "" || c.Tags.Artist == "" {
			continue
		}
		key := c.Source + ":" + c.ID
		if unique[key] {
			continue
		}
		unique[key] = true
		title, artist := similar(input.Title, c.Tags.Title), similar(input.Artist, c.Tags.Artist)
		c.Score = .55*title + .35*artist
		if input.Album != "" && normalized(input.Album) == normalized(c.Tags.Album) {
			c.Score += .05
		}
		if input.Duration > 0 && c.Tags.Duration > 0 {
			if math.Abs(input.Duration-c.Tags.Duration) <= 5 {
				c.Score += .05
			} else if math.Abs(input.Duration-c.Tags.Duration) > 10 {
				c.Score -= .25
			}
		}
		out = append(out, c)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) == 0 {
		return out, false
	}
	best := out[0]
	accept := normalized(input.Title) == normalized(best.Tags.Title) && normalized(input.Artist) == normalized(best.Tags.Artist) && best.Score >= .89
	if len(out) > 1 && best.Score-out[1].Score < .04 && normalized(best.Tags.Album) != normalized(out[1].Tags.Album) {
		accept = false
	}
	return out, accept
}
func fromProbe(raw json.RawMessage, name, template string) (tags, error) {
	var p struct {
		Format struct {
			Duration string            `json:"duration"`
			Tags     map[string]string `json:"tags"`
		} `json:"format"`
		Streams []struct {
			Type        string            `json:"codec_type"`
			Tags        map[string]string `json:"tags"`
			Disposition struct {
				Attached int `json:"attached_pic"`
			} `json:"disposition"`
		} `json:"streams"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return tags{}, err
	}
	values := map[string]string{}
	for k, v := range p.Format.Tags {
		values[strings.ToLower(k)] = v
	}
	audio := 0
	cover := false
	for _, s := range p.Streams {
		if s.Type == "audio" {
			audio++
			for k, v := range s.Tags {
				key := strings.ToLower(k)
				if values[key] == "" {
					values[key] = v
				}
			}
		}
		cover = cover || s.Disposition.Attached == 1
	}
	if audio != 1 {
		return tags{}, fmt.Errorf("音频轨道数量为 %d，保留待确认", audio)
	}
	t := tags{MediaMetadata: pluginapi.MediaMetadata{Title: values["title"], Artist: values["artist"], Album: values["album"], AlbumArtist: values["album_artist"], Track: values["track"], Disc: values["disc"], Date: values["date"], Genre: values["genre"], Comment: values["comment"], Lyrics: values["lyrics"]}, HasCover: cover}
	if t.Lyrics == "" {
		t.Lyrics = values["unsyncedlyrics"]
	}
	if t.Lyrics == "" {
		for key, value := range values {
			if strings.HasPrefix(key, "lyrics-") {
				t.Lyrics = value
				break
			}
		}
	}
	t.Duration, _ = strconv.ParseFloat(p.Format.Duration, 64)
	t.Native = t.MediaMetadata
	t.Lyrics = "" // Keep lyrics once in the persisted native snapshot.
	extracted := filenamerel.ParseFilenameToTags(template, strings.TrimSuffix(name, filepath.Ext(name)), func(k string) string {
		switch k {
		case "title":
			return t.Title
		case "artist":
			return t.Artist
		case "album":
			return t.Album
		}
		return ""
	})
	if t.Title == "" {
		t.Title = extracted["title"]
	}
	if t.Artist == "" {
		t.Artist = extracted["artist"]
	}
	if t.Album == "" {
		t.Album = extracted["album"]
	}
	if t.Title == "" {
		t.Title = strings.TrimSuffix(name, filepath.Ext(name))
	}
	return t, nil
}
func desired(original tags, chosen candidate, mode string) pluginapi.MediaMetadata {
	old, new := original.Native, chosen.Tags.MediaMetadata
	take := func(a, b string) string {
		if mode == "missing" && a != "" {
			return a
		}
		if b != "" {
			return b
		}
		return a
	}
	return pluginapi.MediaMetadata{Title: take(old.Title, new.Title), Artist: take(old.Artist, new.Artist), Album: take(old.Album, new.Album), AlbumArtist: take(old.AlbumArtist, new.AlbumArtist), Genre: take(old.Genre, new.Genre), Date: take(old.Date, new.Date), Track: take(old.Track, new.Track), Disc: take(old.Disc, new.Disc), Comment: old.Comment, Lyrics: old.Lyrics}
}

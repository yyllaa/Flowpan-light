package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"flowpan/app/pluginapi"
	"flowpan/sdk/pluginwasm"
)

type cachedCover struct {
	ID    string `json:"id"`
	Until int64  `json:"until"`
}

func (e *engine) cover(b *batch, c candidate) (string, error) {
	key := "cover:" + digest(c.CoverURL)
	var cached cachedCover
	if err := e.get(key, &cached); err != nil {
		return "", err
	}
	if cached.ID != "" && cached.Until > e.now().Unix() {
		var a pluginapi.Artifact
		if err := e.call("artifacts.stat", map[string]any{"id": cached.ID}, &a); err == nil && a.Status == "sealed" {
			return a.ID, nil
		}
	}
	response, err := e.fetch(c.Source, pluginwasm.HTTPRequest{URL: c.CoverURL, ResponseEncoding: "base64"})
	if err != nil {
		return "", err
	}
	data, err := base64.StdEncoding.DecodeString(response.Body)
	if err != nil || len(data) == 0 || len(data) > 512<<10 {
		return "", fmt.Errorf("封面内容无效")
	}
	hash := sha256.Sum256(data)
	var a pluginapi.Artifact
	if err = e.call("artifacts.create", pluginapi.ArtifactCreate{Key: "music-cover:" + b.ID + ":" + hex.EncodeToString(hash[:]), Name: "cover.jpg", Size: int64(len(data)), SHA256: hex.EncodeToString(hash[:])}, &a); err != nil {
		return "", err
	}
	if a.Status != "sealed" {
		for offset := a.Written; offset < int64(len(data)); {
			end := min(offset+32<<10, int64(len(data)))
			if err = e.call("artifacts.write", map[string]any{"id": a.ID, "offset": offset, "base64": base64.StdEncoding.EncodeToString(data[offset:end])}, &a); err != nil {
				return "", err
			}
			if a.Written <= offset {
				return "", fmt.Errorf("封面写入没有推进")
			}
			offset = a.Written
		}
		if err = e.call("artifacts.seal", map[string]any{"id": a.ID}, &a); err != nil {
			return "", err
		}
	}
	if err = e.set(key, cachedCover{ID: a.ID, Until: e.now().Unix() + 7*24*3600}); err != nil {
		return "", err
	}
	var ring []string
	if err = e.get("cover-ring", &ring); err != nil {
		return "", err
	}
	found := false
	for _, k := range ring {
		found = found || k == key
	}
	if !found {
		ring = append(ring, key)
	}
	for len(ring) > 64 {
		var old cachedCover
		if err = e.get(ring[0], &old); err != nil {
			return "", err
		}
		if old.ID != "" {
			if err = e.call("artifacts.release", map[string]any{"id": old.ID}, nil); err != nil {
				return "", err
			}
		}
		if err = e.set(ring[0], cachedCover{}); err != nil {
			return "", err
		}
		ring = ring[1:]
	}
	if err = e.set("cover-ring", ring); err != nil {
		return "", err
	}
	return a.ID, nil
}

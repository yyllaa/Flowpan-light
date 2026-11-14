package pluginapi

// MediaProcess is a structured local operation, never a command line. Outputs
// are generated under the calling plugin's private data directory by the host.
type MediaProcess struct {
	Key             string        `json:"key"`
	Path            string        `json:"path"`
	FileTask        string        `json:"file_task,omitempty"` // completed files.stage input; Path is resolved by the host
	Operation       string        `json:"operation"`           // transcode, remux, frame, cover
	Format          string        `json:"format,omitempty"`    // mp3, m4a, m4b, flac, wav, mp4, mkv, ogg, opus, jpg, png
	AtSeconds       float64       `json:"at_seconds,omitempty"`
	Width           int           `json:"width,omitempty"`
	Bitrate         int           `json:"bitrate_kbps,omitempty"`
	Metadata        MediaMetadata `json:"metadata,omitempty"`
	StartSeconds    float64       `json:"start_seconds,omitempty"`
	DurationSeconds float64       `json:"duration_seconds,omitempty"`
	AudioChannels   int           `json:"audio_channels,omitempty"`
	SampleRate      int           `json:"sample_rate,omitempty"`
	MapAll          bool          `json:"map_all,omitempty"`
}

// Only these text tags are accepted; no FFmpeg option or arbitrary metadata
// key is interpreted from caller input. Values are compared for idempotency.
type MediaMetadata struct {
	Title       string `json:"title,omitempty"`
	Artist      string `json:"artist,omitempty"`
	Album       string `json:"album,omitempty"`
	AlbumArtist string `json:"album_artist,omitempty"`
	Genre       string `json:"genre,omitempty"`
	Date        string `json:"date,omitempty"`
	Track       string `json:"track,omitempty"`
	Disc        string `json:"disc,omitempty"`
	Comment     string `json:"comment,omitempty"`
}

type MediaTask struct {
	ID         string       `json:"id"`
	PluginID   string       `json:"plugin_id"`
	Status     string       `json:"status"`
	Progress   float64      `json:"progress"`
	Message    string       `json:"message,omitempty"`
	Output     string       `json:"output,omitempty"`
	Size       int64        `json:"size,omitempty"`
	CreatedAt  string       `json:"created_at"`
	FinishedAt string       `json:"finished_at,omitempty"`
	Request    MediaProcess `json:"request"`
}

// MediaChunk keeps binary results below the host response limit. Offset/limit
// allow a plugin to read its own completed output without filesystem access.
type MediaChunk struct {
	Offset int64  `json:"offset"`
	Total  int64  `json:"total"`
	Base64 string `json:"base64"`
	EOF    bool   `json:"eof"`
}

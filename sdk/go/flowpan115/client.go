package flowpan115

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// Driver selects the 115 implementation used by Flowpan.
type Driver string

const (
	DriverAuto   Driver = "auto"
	DriverOpen   Driver = "open"
	DriverCookie Driver = "cookie"
)

// Client calls Flowpan's external 115 storage API.
type Client struct {
	BaseURL       string
	Token         string
	DefaultDriver Driver
	HTTPClient    *http.Client
}

// New returns a client that defaults to DriverAuto.
func New(baseURL string, token string) *Client {
	return &Client{
		BaseURL:       strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		Token:         strings.TrimSpace(token),
		DefaultDriver: DriverAuto,
		HTTPClient:    &http.Client{Timeout: defaultTimeout},
	}
}

// SetDefaultDriver changes the driver used when a method receives an empty driver.
func (c *Client) SetDefaultDriver(driver Driver) {
	c.DefaultDriver = normalizeDriver(driver, DriverAuto)
}

// WithDefaultDriver returns a shallow client copy with a different default driver.
func (c *Client) WithDefaultDriver(driver Driver) *Client {
	clone := *c
	clone.DefaultDriver = normalizeDriver(driver, c.DefaultDriver)
	return &clone
}

// StorageResult is the common Flowpan response payload.
type StorageResult[T any] struct {
	Driver Driver
	Result T
}

// APIError is returned for HTTP errors or non-200 Flowpan envelope codes.
type APIError struct {
	StatusCode int
	Code       int
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Message != "" && e.Code != 0:
		return fmt.Sprintf("flowpan115: status=%d code=%d msg=%s", e.StatusCode, e.Code, e.Message)
	case e.Message != "":
		return fmt.Sprintf("flowpan115: status=%d msg=%s", e.StatusCode, e.Message)
	case e.Code != 0:
		return fmt.Sprintf("flowpan115: status=%d code=%d", e.StatusCode, e.Code)
	default:
		return fmt.Sprintf("flowpan115: status=%d", e.StatusCode)
	}
}

type apiEnvelope struct {
	Code    int             `json:"code"`
	Data    json.RawMessage `json:"data"`
	Msg     string          `json:"msg,omitempty"`
	Message string          `json:"message,omitempty"`
	Error   string          `json:"error,omitempty"`
}

type storageEnvelope struct {
	Driver Driver          `json:"driver"`
	Result json.RawMessage `json:"result"`
}

// Bool returns a bool pointer for list/search option fields.
func Bool(v bool) *bool {
	return &v
}

// Int returns an int pointer for list/search option fields.
func Int(v int) *int {
	return &v
}

// Call invokes an arbitrary external 115 storage action and decodes result into out.
func (c *Client) Call(ctx context.Context, driver Driver, action string, payload any, out any) (Driver, error) {
	res, err := call[json.RawMessage](ctx, c, driver, action, payload)
	if err != nil {
		return "", err
	}
	if out != nil && len(res.Result) > 0 && string(res.Result) != "null" {
		if err := json.Unmarshal(res.Result, out); err != nil {
			return res.Driver, fmt.Errorf("flowpan115: decode result: %w", err)
		}
	}
	return res.Driver, nil
}

// Raw invokes an arbitrary action and returns the raw JSON result.
func (c *Client) Raw(ctx context.Context, driver Driver, action string, payload any) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, action, payload)
}

func call[T any](ctx context.Context, c *Client, driver Driver, action string, payload any) (StorageResult[T], error) {
	var zero StorageResult[T]
	endpoint, err := c.endpoint(driver, action)
	if err != nil {
		return zero, err
	}
	if payload == nil {
		payload = map[string]any{}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return zero, fmt.Errorf("flowpan115: encode payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return zero, err
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.Token))

	client := c.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: defaultTimeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	envelope, parseErr := parseEnvelope(respBody)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		apiErr := &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Code,
			Message:    envelope.message(),
			Body:       strings.TrimSpace(string(respBody)),
		}
		if parseErr != nil {
			apiErr.Message = parseErr.Error()
		}
		return zero, apiErr
	}
	if parseErr != nil {
		return zero, parseErr
	}
	if envelope.Code != 200 {
		return zero, &APIError{
			StatusCode: resp.StatusCode,
			Code:       envelope.Code,
			Message:    envelope.message(),
			Body:       strings.TrimSpace(string(respBody)),
		}
	}

	var data storageEnvelope
	if len(envelope.Data) > 0 && string(envelope.Data) != "null" {
		if err := json.Unmarshal(envelope.Data, &data); err != nil {
			return zero, fmt.Errorf("flowpan115: decode data: %w", err)
		}
	}
	var result T
	if len(data.Result) > 0 && string(data.Result) != "null" {
		if err := json.Unmarshal(data.Result, &result); err != nil {
			return zero, fmt.Errorf("flowpan115: decode result: %w", err)
		}
	}
	if data.Driver == "" {
		data.Driver = normalizeDriver(driver, c.DefaultDriver)
	}
	return StorageResult[T]{Driver: data.Driver, Result: result}, nil
}

func (c *Client) endpoint(driver Driver, action string) (string, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if baseURL == "" {
		return "", fmt.Errorf("flowpan115: base URL is empty")
	}
	if strings.TrimSpace(c.Token) == "" {
		return "", fmt.Errorf("flowpan115: token is empty")
	}
	action = strings.Trim(strings.TrimSpace(action), "/")
	if action == "" {
		return "", fmt.Errorf("flowpan115: action is empty")
	}
	driver = normalizeDriver(driver, c.DefaultDriver)
	return baseURL + "/api/v1/storage/115/" + url.PathEscape(string(driver)) + "/" + action, nil
}

func normalizeDriver(driver Driver, fallback Driver) Driver {
	value := Driver(strings.ToLower(strings.TrimSpace(string(driver))))
	if value == "" {
		value = Driver(strings.ToLower(strings.TrimSpace(string(fallback))))
	}
	switch value {
	case DriverOpen, DriverCookie, DriverAuto:
		return value
	default:
		return DriverAuto
	}
}

func parseEnvelope(body []byte) (apiEnvelope, error) {
	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return envelope, fmt.Errorf("flowpan115: decode response: %w", err)
	}
	return envelope, nil
}

func (e apiEnvelope) message() string {
	for _, value := range []string{e.Msg, e.Message, e.Error} {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type DirRequest struct {
	Path    string `json:"path"`
	CID     int64  `json:"cid,omitempty"`
	Type    string `json:"type,omitempty"`
	Storage string `json:"storage,omitempty"`
}

type DirResult struct {
	Path   string `json:"path,omitempty"`
	CID    int64  `json:"cid,omitempty"`
	FileID int64  `json:"file_id,omitempty"`
	ID     int64  `json:"id,omitempty"`
	IDText string `json:"id_text,omitempty"`
}

type ItemResult struct {
	Storage      string `json:"storage,omitempty"`
	FileID       int64  `json:"file_id,omitempty"`
	ID           int64  `json:"id,omitempty"`
	FileIDText   string `json:"file_id_text,omitempty"`
	ParentID     int64  `json:"parent_id,omitempty"`
	ParentIDText string `json:"parent_id_text,omitempty"`
	Name         string `json:"name,omitempty"`
	Path         string `json:"path,omitempty"`
	Type         string `json:"type,omitempty"`
	Star         bool   `json:"star,omitempty"`
	Size         int64  `json:"size,omitempty"`
	ModifyTime   int64  `json:"modify_time,omitempty"`
	Pickcode     string `json:"pickcode,omitempty"`
	SHA1         string `json:"sha1,omitempty"`
}

type PageResult struct {
	Items  []ItemResult    `json:"items"`
	Total  int             `json:"total"`
	Offset int             `json:"offset"`
	Limit  int             `json:"limit"`
	Raw    map[string]any  `json:"raw,omitempty"`
	Extra  json.RawMessage `json:"extra,omitempty"`
}

type ListPageRequest struct {
	CID            int64             `json:"cid,omitempty"`
	Offset         int               `json:"offset,omitempty"`
	Limit          int               `json:"limit,omitempty"`
	Aid            int               `json:"aid,omitempty"`
	Order          string            `json:"order,omitempty"`
	O              string            `json:"o,omitempty"`
	Fields         string            `json:"fields,omitempty"`
	Suffix         string            `json:"suffix,omitempty"`
	Source         string            `json:"source,omitempty"`
	LastUTime      string            `json:"last_utime,omitempty"`
	MinSize        int64             `json:"min_size,omitempty"`
	MaxSize        int64             `json:"max_size,omitempty"`
	Type           int               `json:"type,omitempty"`
	SuffixType     int               `json:"suffix_type,omitempty"`
	Asc            *bool             `json:"asc,omitempty"`
	CountFolders   *bool             `json:"count_folders,omitempty"`
	Cur            *bool             `json:"cur,omitempty"`
	CustomOrder    *int              `json:"custom_order,omitempty"`
	FCMix          *bool             `json:"fc_mix,omitempty"`
	Hidden         *bool             `json:"hidden,omitempty"`
	IsQ            *bool             `json:"is_q,omitempty"`
	IsShare        *bool             `json:"is_share,omitempty"`
	NatSort        *bool             `json:"natsort,omitempty"`
	NF             *bool             `json:"nf,omitempty"`
	RecordOpenTime *bool             `json:"record_open_time,omitempty"`
	ShowDir        *bool             `json:"show_dir,omitempty"`
	Snap           *bool             `json:"snap,omitempty"`
	Star           *bool             `json:"star,omitempty"`
	StdIR          *bool             `json:"stdir,omitempty"`
	Storage        string            `json:"storage,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
}

type SearchRequest struct {
	Keyword      string            `json:"keyword,omitempty"`
	SearchValue  string            `json:"search_value,omitempty"`
	FileLabel    string            `json:"file_label,omitempty"`
	CID          int64             `json:"cid,omitempty"`
	Offset       int               `json:"offset,omitempty"`
	Limit        int               `json:"limit,omitempty"`
	Aid          int               `json:"aid,omitempty"`
	Order        string            `json:"order,omitempty"`
	Date         string            `json:"date,omitempty"`
	Distance     string            `json:"distance,omitempty"`
	GteDay       string            `json:"gte_day,omitempty"`
	LteDay       string            `json:"lte_day,omitempty"`
	Location     string            `json:"location,omitempty"`
	QID          string            `json:"qid,omitempty"`
	RespVer      string            `json:"resp_ver,omitempty"`
	Source       string            `json:"source,omitempty"`
	Suffix       string            `json:"suffix,omitempty"`
	FC           int               `json:"fc,omitempty"`
	Type         int               `json:"type,omitempty"`
	Asc          *bool             `json:"asc,omitempty"`
	CountFolders *bool             `json:"count_folders,omitempty"`
	FCMix        *bool             `json:"fc_mix,omitempty"`
	NatSort      *bool             `json:"natsort,omitempty"`
	NF           *bool             `json:"nf,omitempty"`
	ShowDir      *bool             `json:"show_dir,omitempty"`
	Star         *bool             `json:"star,omitempty"`
	StdIR        *bool             `json:"stdir,omitempty"`
	Storage      string            `json:"storage,omitempty"`
	Extra        map[string]string `json:"extra,omitempty"`
}

type IDsRequest struct {
	IDs      []int64 `json:"ids,omitempty"`
	FileIDs  []int64 `json:"file_ids,omitempty"`
	ParentID int64   `json:"parent_id,omitempty"`
	CID      int64   `json:"cid,omitempty"`
	Starred  bool    `json:"starred,omitempty"`
}

type RenameRequest struct {
	ID     int64  `json:"id,omitempty"`
	FileID int64  `json:"file_id,omitempty"`
	Name   string `json:"name"`
}

type PasteRequest struct {
	IDsRequest
	Action string `json:"action,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

type UsageResult struct {
	Total     int64  `json:"total"`
	Available int64  `json:"available"`
	Used      int64  `json:"used"`
	UserID    int64  `json:"user_id,omitempty"`
	UserName  string `json:"user_name,omitempty"`
}

type DownloadURLRequest struct {
	Pickcode  string `json:"pickcode,omitempty"`
	PickCode  string `json:"pick_code,omitempty"`
	UserAgent string `json:"user_agent,omitempty"`
}

type DownloadURLResult struct {
	URL      string `json:"url"`
	Pickcode string `json:"pickcode"`
}

type UploadFileRequest struct {
	LocalPath string `json:"local_path"`
	Name      string `json:"name,omitempty"`
	TargetCID int64  `json:"target_cid"`
	PartSize  int64  `json:"part_size,omitempty"`
}

type UploadPartURLRequest struct {
	Session    any `json:"session"`
	PartNumber int `json:"part_number"`
}

type UploadCompleteRequest struct {
	Session any `json:"session"`
	Parts   any `json:"parts"`
}

func (c *Client) EnsureDir(ctx context.Context, driver Driver, cid int64, path string) (StorageResult[DirResult], error) {
	return call[DirResult](ctx, c, driver, "dir/ensure", DirRequest{CID: cid, Path: path})
}

func (c *Client) ListDir(ctx context.Context, driver Driver, req DirRequest) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "dir/list", req)
}

func (c *Client) ListPage(ctx context.Context, driver Driver, req ListPageRequest) (StorageResult[PageResult], error) {
	return call[PageResult](ctx, c, driver, "dir/list-page", req)
}

func (c *Client) Search(ctx context.Context, driver Driver, req SearchRequest) (StorageResult[PageResult], error) {
	return call[PageResult](ctx, c, driver, "search", req)
}

func (c *Client) Item(ctx context.Context, driver Driver, req DirRequest) (StorageResult[ItemResult], error) {
	return call[ItemResult](ctx, c, driver, "item", req)
}

func (c *Client) Usage(ctx context.Context, driver Driver) (StorageResult[UsageResult], error) {
	return call[UsageResult](ctx, c, driver, "usage", map[string]any{})
}

func (c *Client) DownloadURL(ctx context.Context, driver Driver, req DownloadURLRequest) (StorageResult[DownloadURLResult], error) {
	return call[DownloadURLResult](ctx, c, driver, "download-url", req)
}

func (c *Client) MoveCheckConflict(ctx context.Context, driver Driver, ids []int64, parentID int64) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "move/check-conflict", IDsRequest{IDs: ids, ParentID: parentID})
}

func (c *Client) Move(ctx context.Context, driver Driver, ids []int64, parentID int64) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "move", IDsRequest{IDs: ids, ParentID: parentID})
}

func (c *Client) Copy(ctx context.Context, driver Driver, ids []int64, parentID int64) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "copy", IDsRequest{IDs: ids, ParentID: parentID})
}

func (c *Client) Paste(ctx context.Context, driver Driver, pasteAction string, ids []int64, parentID int64) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "paste", PasteRequest{IDsRequest: IDsRequest{IDs: ids, ParentID: parentID}, Action: pasteAction})
}

func (c *Client) Rename(ctx context.Context, driver Driver, id int64, name string) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "rename", RenameRequest{ID: id, Name: name})
}

func (c *Client) Star(ctx context.Context, driver Driver, ids []int64, starred bool) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "star", IDsRequest{IDs: ids, Starred: starred})
}

func (c *Client) Delete(ctx context.Context, driver Driver, ids []int64) (StorageResult[map[string]any], error) {
	return call[map[string]any](ctx, c, driver, "delete", IDsRequest{IDs: ids})
}

func (c *Client) UploadFile(ctx context.Context, req UploadFileRequest) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, DriverOpen, "upload/file", req)
}

func (c *Client) UploadInit(ctx context.Context, driver Driver, payload any) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/init", payload)
}

func (c *Client) UploadStart(ctx context.Context, driver Driver, session any) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/start", map[string]any{"session": session})
}

func (c *Client) UploadResume(ctx context.Context, driver Driver, session any) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/resume", map[string]any{"session": session})
}

func (c *Client) UploadListParts(ctx context.Context, driver Driver, session any) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/list-parts", map[string]any{"session": session})
}

func (c *Client) UploadPartURL(ctx context.Context, driver Driver, req UploadPartURLRequest) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/part-url", req)
}

func (c *Client) UploadComplete(ctx context.Context, driver Driver, req UploadCompleteRequest) (StorageResult[json.RawMessage], error) {
	return call[json.RawMessage](ctx, c, driver, "upload/complete", req)
}

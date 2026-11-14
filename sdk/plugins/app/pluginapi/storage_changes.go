package pluginapi

// StorageChanges is the result of storage.operation=changes. Cursor and IDs
// remain opaque; the host supplies the owning account, source and revision.
// Initial cursor="" must return no historical items and a nonempty cursor.
type StorageChanges struct {
	Items   []StorageChange `json:"items"`
	Cursor  string          `json:"cursor"`
	HasMore bool            `json:"has_more"`
}
type StorageChange struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	ParentIDs []string `json:"parent_ids"`
	ObjectIDs []string `json:"object_ids,omitempty"`
}

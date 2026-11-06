package pluginwasm

// Browser is a host-owned page lease. Profiles are isolated by plugin and account.
// The host also releases the page when the guest exits or is cancelled.
type Browser struct {
	Session string `json:"session"`
}

func OpenBrowser(url, profile string) (Browser, error) {
	var page Browser
	err := Call("browser.open", map[string]any{"url": url, "profile": profile}, &page)
	return page, err
}
func (b Browser) Evaluate(script string, args any, secrets map[string]string, result any) error {
	return Call("browser.evaluate", map[string]any{"session": b.Session, "script": script, "args": args, "secrets": secrets}, result)
}
func (b Browser) Close() error {
	return Call("browser.close", map[string]any{"session": b.Session}, nil)
}

// SubmitTransfer uses the account's saved project target and organizer settings.
// The action must be confirmed; reuse key when recovering the same submission.
func SubmitTransfer(account, text, key string, result any) error {
	return Call("transfer.submit", map[string]any{"account": account, "text": text, "key": key}, result)
}
func TransferStatus(account, key string, result any) error {
	return Call("transfer.status", map[string]any{"account": account, "key": key}, result)
}
func RunOrganize(account, key string, result any) error {
	return Call("organize.run", map[string]any{"account": account, "key": key}, result)
}
func OrganizeStatus(account string, result any) error {
	return Call("organize.status", map[string]any{"account": account}, result)
}

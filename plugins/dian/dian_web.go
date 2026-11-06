// Adapted mechanically from 115-Station internal/api/dianweb.go. See NOTICE.md.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
)

const dianSignJS = `() => {
  if (window.__s115dian) return
  const b64u = (buf) => { let s = ''; const a = new Uint8Array(buf); for (let i = 0; i < a.length; i++) s += String.fromCharCode(a[i]); return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '') }
  const DB = 'portal-browser-security-v1', STORE = 'keys', KID = 'request-signing-p256'
  const st = { key: null, proof: null, proofExp: 0, sess: false, skew: 0 }
  const base = { 'Content-Type': 'application/json', 'X-Requested-With': 'XMLHttpRequest' }
  const openDB = () => new Promise((ok, bad) => {
    const r = indexedDB.open(DB, 1)
    r.onupgradeneeded = () => { if (!r.result.objectStoreNames.contains(STORE)) r.result.createObjectStore(STORE, { keyPath: 'id' }) }
    r.onsuccess = () => ok(r.result); r.onerror = () => bad(r.error)
  })
  async function loadKey() {
    if (st.key) return st.key
    try {
      const db = await openDB()
      const rec = await new Promise((ok) => { const q = db.transaction(STORE, 'readonly').objectStore(STORE).get(KID); q.onsuccess = () => ok(q.result || null); q.onerror = () => ok(null) })
      db.close()
      if (rec && rec.privateKey && rec.publicJWK && rec.publicJWK.x) { st.key = rec; return rec }
    } catch (e) {}
    const kp = await crypto.subtle.generateKey({ name: 'ECDSA', namedCurve: 'P-256' }, false, ['sign', 'verify'])
    const jwk = await crypto.subtle.exportKey('jwk', kp.publicKey)
    const rec = { id: KID, privateKey: kp.privateKey, publicJWK: jwk }
    try {
      const db = await openDB()
      await new Promise((ok) => { const t = db.transaction(STORE, 'readwrite'); t.objectStore(STORE).put(rec); t.oncomplete = ok; t.onerror = ok; t.onabort = ok })
      db.close()
    } catch (e) {}
    st.key = rec
    return rec
  }
  async function proof(force) {
    if (!force && st.proof && st.proofExp - Date.now() > 30000) return st.proof
    const r = await fetch('/api/portal/auth/browser-challenge', { credentials: 'include', headers: base })
    const j = await r.json().catch(() => ({}))
    if (j.enabled === false || !j.proof) { st.proof = null; return null }
    st.proof = String(j.proof)
    st.proofExp = j.expires_at ? Date.parse(j.expires_at) : Date.now() + Math.max(Number(j.ttl) || 600, 60) * 1000
    return st.proof
  }
  async function session(pf, force) {
    if (st.sess && !force) return
    const k = await loadKey()
    const h = { ...base }; if (pf) h['X-Portal-Browser-Proof'] = pf
    const { kty, crv, x, y } = k.publicJWK
    const r = await fetch('/api/portal/auth/browser-session', { method: 'POST', credentials: 'include', headers: h, body: JSON.stringify({ public_jwk: { kty, crv, x, y } }) })
    const j = await r.json().catch(() => ({}))
    const srv = Number(j.server_time_ms)
    st.skew = Number.isFinite(srv) ? srv - Date.now() : 0
    st.sess = true
  }
  async function call(method, path, body, retried) {
    const pf = await proof(retried)
    await session(pf, retried)
    const u = '/api/portal' + path
    const pathname = new URL(u, location.origin).pathname
    const ts = String(Math.round(Date.now() + st.skew))
    const nb = new Uint8Array(24); crypto.getRandomValues(nb); const nonce = b64u(nb)
    const msg = 'portal-browser-request/v1\n' + method.toUpperCase() + '\n' + pathname + '\n' + ts + '\n' + nonce
    const k = await loadKey()
    const sig = await crypto.subtle.sign({ name: 'ECDSA', hash: 'SHA-256' }, k.privateKey, new TextEncoder().encode(msg))
    const h = { ...base, 'X-Portal-Browser-TS': ts, 'X-Portal-Browser-Nonce': nonce, 'X-Portal-Browser-Sig': b64u(sig) }
    if (pf) h['X-Portal-Browser-Proof'] = pf
    const r = await fetch(u, { method, credentials: 'include', headers: h, body: body == null ? undefined : JSON.stringify(body) })
    const text = await r.text()
    if (!retried && r.status === 403 && /browser_proof/.test(text)) return call(method, path, body, true)
    return { status: r.status, text }
  }
  window.__s115dian = (method, path, body) => call(method, path, body, false)
}`

// dianCaller 一次网页操作里调接口用
type dianCaller struct {
	p *pluginPage
}

// dianAPIErr 站点接口返回的错误（code 非 ok）
type dianAPIErr struct {
	Status int
	Code   string
	Msg    string
}

func (e *dianAPIErr) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	if e.Code != "" {
		return fmt.Sprintf("%s（HTTP %d）", e.Code, e.Status)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// needLogin 没登录 / 登录失效
func (e *dianAPIErr) needLogin() bool {
	return e.Status == 401 || e.Code == "no_token" || e.Code == "invalid_token" || e.Code == "token_revoked" || e.Code == "unauthorized"
}

// do 调一个接口，返回原始 JSON；code 不是 ok（或 HTTP 非 2xx）时返回 *dianAPIErr
func (c *dianCaller) do(method, path string, body any) ([]byte, error) {
	var arg any = nil
	if body != nil {
		arg = body
	}
	res, err := c.p.request(method, path, arg)
	if err != nil {
		return nil, fmt.Errorf("页面里调用 %s 失败: %v", path, err)
	}
	status := res.Status
	text := res.Text
	var env struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	if json.Unmarshal([]byte(text), &env) != nil {
		// Cloudflare 拦截页之类：不是 JSON
		snippet := strings.TrimSpace(text)
		if strings.Contains(snippet, "Cloudflare") {
			return nil, &dianAPIErr{Status: status, Msg: fmt.Sprintf("被 Cloudflare 拦下（HTTP %d），稍后再试或换个网络", status)}
		}
		return nil, &dianAPIErr{Status: status, Msg: fmt.Sprintf("站点返回了非 JSON 内容（HTTP %d）: %s", status, truncateStr(snippet, 80))}
	}
	if status < 200 || status >= 300 || (env.Code != "" && env.Code != "ok") {
		return nil, &dianAPIErr{Status: status, Code: env.Code, Msg: env.Msg}
	}
	return []byte(text), nil
}

// dianMe /me 里用得到的几项
type dianMe struct {
	ID       int    `json:"id"`
	Nickname string `json:"nickname"`
	Points   int    `json:"points"`
	VIP      bool   `json:"vip"`
}

func (c *dianCaller) me() (*dianMe, error) {
	raw, err := c.do("GET", "/me", nil)
	if err != nil {
		return nil, err
	}
	var out struct {
		User dianMe `json:"user"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("解析账号信息失败: %v", err)
	}
	return &out.User, nil
}

// ensureLogin 读一次 /me，没登录就用账号密码登录（会话 Cookie 存在用户数据目录里，下次不用再登）
func (c *dianCaller) ensureLogin(cfg *dianCfg) (*dianMe, error) {
	me, err := c.me()
	if err == nil {
		log.Printf("[癫影] ○ 登录态有效（%s）", me.Nickname)
		return me, nil
	}
	var ae *dianAPIErr
	if !errors.As(err, &ae) || !ae.needLogin() {
		return nil, err
	}
	re0Step("登录癫影")
	log.Printf("[癫影] ○ 未登录，登录账号 %s", re0MaskUser(cfg.Email))
	if raw, err := c.do("GET", "/auth/policy", nil); err == nil {
		var pol struct {
			Turnstile bool `json:"turnstile_enabled"`
		}
		if json.Unmarshal(raw, &pol) == nil && pol.Turnstile {
			return nil, fmt.Errorf("癫影登录现在要求人机验证，本站暂时没法自动登录")
		}
	}
	if _, err := c.do("POST", "/auth/login", map[string]string{"email": cfg.Email, "password": cfg.Password}); err != nil {
		if errors.As(err, &ae) && ae.Code == "email_not_verified" {
			return nil, fmt.Errorf("登录失败：邮箱还没验证，请先去邮箱点验证链接")
		}
		return nil, fmt.Errorf("登录失败：%v", err)
	}
	me, err = c.me()
	if err != nil {
		return nil, fmt.Errorf("登录后读取账号失败：%v", err)
	}
	log.Printf("[癫影] ✓ 已登录（%s，积分 %d）", me.Nickname, me.Points)
	return me, nil
}

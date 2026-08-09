package telegram

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tgerr"

	"github.com/73ai/openbotkit/store"
)

const authPage = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>OpenBotKit - Link Telegram</title>
<link rel="preconnect" href="https://fonts.googleapis.com">
<link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Inter:wght@400;500;600&display=swap" rel="stylesheet">
<script src="https://cdn.jsdelivr.net/npm/qrcodejs@1.0.0/qrcode.min.js"></script>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:'Inter',system-ui,-apple-system,sans-serif;display:flex;justify-content:center;align-items:center;
  min-height:100vh;background:#f8f9fa;color:#1a1a1a;padding:1.5rem}
.card{background:#fff;border-radius:16px;box-shadow:0 4px 24px rgba(0,0,0,.08);
  max-width:440px;width:100%;padding:2.5rem;text-align:center}
.logo{font-size:1.1rem;font-weight:600;color:#6b7280;letter-spacing:-.02em;margin-bottom:1.5rem}
h1{font-size:1.5rem;font-weight:600;margin-bottom:.5rem}
.subtitle{color:#6b7280;font-size:.95rem;margin-bottom:2rem}
#qr{display:inline-block;padding:12px;background:#fff;border-radius:12px;border:1px solid #e5e7eb;margin-bottom:1.5rem}
#qr canvas,#qr img{display:block;border-radius:4px}
#status{font-size:1rem;font-weight:500;color:#16a34a;min-height:1.5rem;margin-bottom:1.5rem}
.steps{text-align:left;background:#f8f9fa;border-radius:12px;padding:1.25rem 1.5rem}
.steps h3{font-size:.8rem;font-weight:600;text-transform:uppercase;letter-spacing:.05em;color:#9ca3af;margin-bottom:.75rem}
.steps ol{padding-left:1.25rem;font-size:.9rem;color:#4b5563;line-height:1.75}
.steps li{padding-left:.25rem}
.steps strong{color:#1a1a1a}
.success-icon{font-size:3rem;margin-bottom:1rem}
.success-msg{font-size:1.1rem;font-weight:500;color:#16a34a}
.loading{color:#9ca3af}
.error{color:#dc2626;font-size:.9rem;margin-top:.75rem;min-height:1.2rem}
input[type=password]{width:100%;padding:.75rem 1rem;font-size:1rem;font-family:inherit;
  border:1px solid #e5e7eb;border-radius:10px;margin-bottom:1rem}
button{width:100%;padding:.75rem 1rem;font-size:1rem;font-weight:500;font-family:inherit;color:#fff;
  background:#2563eb;border:none;border-radius:10px;cursor:pointer}
button:disabled{background:#9ca3af;cursor:default}
</style></head>
<body>
<div class="card">
  <div class="logo">OpenBotKit</div>
  <div id="main">
    <h1>Link your Telegram</h1>
    <p class="subtitle">Scan the QR code to sync your messages with OpenBotKit</p>
    <div id="qr"></div>
    <p id="status" class="loading">Connecting to Telegram...</p>
    <div class="steps">
      <h3>How to scan</h3>
      <ol>
        <li>Open <strong>Telegram</strong> on your phone</li>
        <li>Go to <strong>Settings</strong></li>
        <li>Tap <strong>Devices</strong></li>
        <li>Tap <strong>Link Desktop Device</strong></li>
        <li>Point your camera at the QR code above</li>
      </ol>
    </div>
  </div>
  <div id="linking" style="display:none">
    <p class="loading" style="font-size:1.1rem;font-weight:500">Linking your device, please wait...</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">This usually takes a few seconds.</p>
  </div>
  <div id="password" style="display:none">
    <h1>Two-step verification</h1>
    <p class="subtitle" id="pwhint">Enter your Telegram cloud password to finish signing in.</p>
    <form id="pwform">
      <input type="password" id="pwinput" placeholder="Cloud password" autocomplete="current-password" autofocus>
      <button type="submit" id="pwsubmit">Continue</button>
    </form>
    <p class="error" id="pwerror"></p>
  </div>
  <div id="syncing" style="display:none">
    <p class="loading" style="font-size:1.1rem;font-weight:500">Loading your chats...</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">This usually takes a few seconds.</p>
  </div>
  <div id="done" style="display:none">
    <div class="success-icon">&#10003;</div>
    <p class="success-msg">Telegram linked successfully!</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">You can close this tab and return to the terminal.</p>
  </div>
</div>
<script>
var qrEl=document.getElementById("qr"),statusEl=document.getElementById("status"),
    mainEl=document.getElementById("main"),linkingEl=document.getElementById("linking"),
    passwordEl=document.getElementById("password"),syncingEl=document.getElementById("syncing"),
    doneEl=document.getElementById("done"),pwErrEl=document.getElementById("pwerror"),
    pwHintEl=document.getElementById("pwhint"),pwInput=document.getElementById("pwinput"),
    pwSubmit=document.getElementById("pwsubmit"),qrCode=null,hasQR=false,submitting=false;
function only(el){var all=[mainEl,linkingEl,passwordEl,syncingEl,doneEl];
  for(var i=0;i<all.length;i++){all[i].style.display=all[i]===el?"block":"none"}}
document.getElementById("pwform").addEventListener("submit",function(e){
  e.preventDefault();
  if(submitting){return}
  submitting=true;pwSubmit.disabled=true;pwErrEl.textContent="";
  fetch("/api/password",{method:"POST",headers:{"Content-Type":"application/json"},
    body:JSON.stringify({password:pwInput.value})}).then(function(){
    pwInput.value="";submitting=false;pwSubmit.disabled=false
  }).catch(function(){submitting=false;pwSubmit.disabled=false;pwErrEl.textContent="Could not reach OpenBotKit."})});
function poll(){fetch("/api/qr").then(function(r){return r.json()}).then(function(d){
  if(d.authenticated){only(doneEl);return}
  if(d.syncing){only(syncingEl);setTimeout(poll,1000);return}
  if(d.password_needed){only(passwordEl);
    if(d.password_hint){pwHintEl.textContent="Password hint: "+d.password_hint}
    if(!submitting){pwErrEl.textContent=d.error||""}
    setTimeout(poll,1000);return}
  if(d.linking){only(linkingEl);setTimeout(poll,1000);return}
  only(mainEl);
  if(d.error){statusEl.textContent=d.error;statusEl.className="loading"}
  if(d.qr){hasQR=true;
    if(!d.error){statusEl.textContent="QR code ready - scan it now";statusEl.className=""}
    // Tokens expire in about 30s, so re-render whenever the payload changes.
    if(!qrCode){qrCode=new QRCode(qrEl,{text:d.qr,width:220,height:220,correctLevel:QRCode.CorrectLevel.L})}
    else if(qrEl.dataset.token!==d.qr){qrCode.clear();qrCode.makeCode(d.qr)}
    qrEl.dataset.token=d.qr}
  setTimeout(poll,hasQR?2000:3000)}).catch(function(){
    statusEl.textContent="Reconnecting...";statusEl.className="loading";setTimeout(poll,3000)})}
poll();
</script>
</body></html>`

// AuthPage returns the HTML for the Telegram QR authentication page.
func AuthPage() string { return authPage }

// authState is the shared state between the auth goroutine and the HTTP
// handlers. Every field is guarded by mu.
type authState struct {
	mu            sync.Mutex
	qr            string
	linking       bool
	passwordNeed  bool
	passwordHint  string
	syncing       bool
	authenticated bool
	errMsg        string

	// passwords carries cloud passwords from the browser to the auth goroutine.
	passwords chan string
}

func newAuthState() *authState {
	return &authState{passwords: make(chan string, 1)}
}

func (s *authState) snapshot() map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return map[string]any{
		"qr":              s.qr,
		"linking":         s.linking,
		"password_needed": s.passwordNeed,
		"password_hint":   s.passwordHint,
		"syncing":         s.syncing,
		"authenticated":   s.authenticated,
		"error":           s.errMsg,
	}
}

func (s *authState) setQR(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qr = token
	s.linking = false
}

func (s *authState) setLinking() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.linking = true
	s.qr = ""
}

func (s *authState) setPasswordNeeded(hint string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.passwordNeed = true
	s.passwordHint = hint
	s.linking = false
	s.qr = ""
}

func (s *authState) setSyncing() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.syncing = true
	s.passwordNeed = false
	s.linking = false
	s.errMsg = ""
}

func (s *authState) setAuthenticated() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.authenticated = true
	s.syncing = false
	s.passwordNeed = false
	s.linking = false
	s.errMsg = ""
}

func (s *authState) setError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.errMsg = msg
}

// newAuthMux wires the auth page and its polling API.
func newAuthMux(st *authState) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, authPage)
	})

	mux.HandleFunc("/api/qr", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st.snapshot())
	})

	mux.HandleFunc("/api/password", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if body.Password == "" {
			http.Error(w, "password is required", http.StatusBadRequest)
			return
		}
		select {
		case st.passwords <- body.Password:
			w.WriteHeader(http.StatusAccepted)
		default:
			http.Error(w, "a password is already being checked", http.StatusConflict)
		}
	})

	return mux
}

// ServeQR runs the browser QR login flow on addr, blocking until the account is
// linked or ctx is cancelled. When db is non-nil the signed-in account and its
// dialogs are recorded so chats are queryable straight after login.
func ServeQR(ctx context.Context, client *Client, addr string, db *store.DB) error {
	if addr == "" {
		addr = ":8086"
	}

	st := newAuthState()

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}
	server := &http.Server{Handler: newAuthMux(st)}

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()
	defer server.Shutdown(context.Background())

	fmt.Printf("Open http://localhost:%d in your browser to scan the QR code\n", ln.Addr().(*net.TCPAddr).Port)

	runErr := client.Run(ctx, func(ctx context.Context) error {
		if err := authenticate(ctx, client, st); err != nil {
			st.setError(err.Error())
			return err
		}

		st.setSyncing()
		if db != nil {
			if err := recordSelf(ctx, client, db); err != nil {
				slog.Warn("telegram: could not record account", "error", err)
			}
			if _, err := SyncDialogs(ctx, NewAPIFetcher(client.API()), db); err != nil {
				slog.Warn("telegram: could not load dialogs after login", "error", err)
			}
		}

		st.setAuthenticated()
		// Give the browser a moment to poll and see the success state.
		time.Sleep(3 * time.Second)
		return nil
	})
	if runErr != nil {
		return runErr
	}

	select {
	case err := <-errCh:
		return err
	default:
	}
	return nil
}

// authenticate drives the QR flow, including the cloud-password step that
// qrlogin does not handle itself.
func authenticate(ctx context.Context, client *Client, st *authState) error {
	loggedIn := qrlogin.OnLoginToken(client.Dispatcher())

	show := func(ctx context.Context, token qrlogin.Token) error {
		st.setQR(token.URL())
		return nil
	}

	_, err := client.TG().QR().Auth(ctx, loggedIn, show)
	if err == nil {
		return nil
	}
	if !tgerr.Is(err, "SESSION_PASSWORD_NEEDED") {
		return fmt.Errorf("qr login: %w", err)
	}

	st.setLinking()
	return authenticatePassword(ctx, client, st)
}

// authenticatePassword collects the cloud password from the auth page and
// retries until it is accepted or ctx is cancelled.
func authenticatePassword(ctx context.Context, client *Client, st *authState) error {
	hint := ""
	if pw, err := client.API().AccountGetPassword(ctx); err == nil {
		hint = pw.Hint
	}
	st.setPasswordNeeded(hint)

	for {
		var password string
		select {
		case <-ctx.Done():
			return ctx.Err()
		case password = <-st.passwords:
		}

		_, err := client.TG().Auth().Password(ctx, password)
		if err == nil {
			return nil
		}
		if tgerr.Is(err, "PASSWORD_HASH_INVALID") {
			st.setError("Incorrect password, please try again.")
			continue
		}
		return fmt.Errorf("cloud password: %w", err)
	}
}

// recordSelf stores the signed-in account so status can be reported without a
// network round trip.
func recordSelf(ctx context.Context, client *Client, db *store.DB) error {
	self, err := client.TG().Self(ctx)
	if err != nil {
		return fmt.Errorf("get self: %w", err)
	}
	if err := SaveSelf(db, self.ID, self.Username); err != nil {
		return err
	}
	return UpsertUser(db, userFromTG(self))
}

package whatsapp

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"go.mau.fi/whatsmeow/types/events"
)

const authPage = `<!DOCTYPE html>
<html lang="en"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>OpenBotKit — Link WhatsApp</title>
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
</style></head>
<body>
<div class="card">
  <div class="logo">OpenBotKit</div>
  <div id="main">
    <h1>Link your WhatsApp</h1>
    <p class="subtitle">Scan the QR code to sync your messages with OpenBotKit</p>
    <div id="qr"></div>
    <p id="status" class="loading">Connecting to WhatsApp...</p>
    <div class="steps">
      <h3>How to scan</h3>
      <ol>
        <li>Open <strong>WhatsApp</strong> on your phone</li>
        <li>Go to <strong>Settings</strong> (or tap the three dots menu)</li>
        <li>Tap <strong>Linked Devices</strong></li>
        <li>Tap <strong>Link a Device</strong></li>
        <li>Point your camera at the QR code above</li>
      </ol>
    </div>
  </div>
  <div id="linking" style="display:none">
    <p class="loading" style="font-size:1.1rem;font-weight:500">Linking your device, please wait...</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">This usually takes 10–15 seconds.</p>
  </div>
  <div id="syncing" style="display:none">
    <p class="loading" style="font-size:1.1rem;font-weight:500">Syncing your message history...</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">This usually takes 15–30 seconds.</p>
  </div>
  <div id="done" style="display:none">
    <div class="success-icon">&#10003;</div>
    <p class="success-msg">WhatsApp linked successfully!</p>
    <p class="subtitle" style="margin-top:.75rem;margin-bottom:0">You can close this tab and return to the terminal.</p>
  </div>
</div>
<script>
var qrEl=document.getElementById("qr"),statusEl=document.getElementById("status"),
    mainEl=document.getElementById("main"),linkingEl=document.getElementById("linking"),
    syncingEl=document.getElementById("syncing"),doneEl=document.getElementById("done"),qrCode=null,hasQR=false;
function poll(){fetch("/api/qr").then(function(r){return r.json()}).then(function(d){
  if(d.authenticated){mainEl.style.display="none";linkingEl.style.display="none";syncingEl.style.display="none";doneEl.style.display="block";return}
  if(d.syncing){mainEl.style.display="none";linkingEl.style.display="none";syncingEl.style.display="block";setTimeout(poll,2000);return}
  if(d.linking){mainEl.style.display="none";linkingEl.style.display="block";setTimeout(poll,2000);return}
  if(d.qr){hasQR=true;statusEl.textContent="QR code ready — scan it now";statusEl.className="";
    if(!qrCode){qrCode=new QRCode(qrEl,{text:d.qr,width:220,height:220,correctLevel:QRCode.CorrectLevel.L})}
    else{qrCode.clear();qrCode.makeCode(d.qr)}}
  setTimeout(poll,hasQR?2000:3000)}).catch(function(){statusEl.textContent="Reconnecting...";statusEl.className="loading";setTimeout(poll,3000)})}
poll();
</script>
</body></html>`

// waitForHistorySync blocks until either the quiet period elapses after the
// last sync signal, or maxWait is reached. This lets the phone finish its
// initial history sync before we declare authentication complete.
func waitForHistorySync(syncSignal <-chan struct{}, maxWait, quietPeriod time.Duration) {
	deadline := time.After(maxWait)
	quietTimer := time.NewTimer(quietPeriod)
	defer quietTimer.Stop()

	select {
	case <-syncSignal:
		quietTimer.Reset(quietPeriod)
	case <-deadline:
		return
	}

	for {
		select {
		case <-syncSignal:
			quietTimer.Reset(quietPeriod)
		case <-quietTimer.C:
			return
		case <-deadline:
			return
		}
	}
}

func ServeQR(ctx context.Context, client *Client, addr string) error {
	if addr == "" {
		addr = ":8085"
	}

	var mu sync.Mutex
	var currentQR string
	linking := false
	syncing := false
	authenticated := false

	qrChan := make(chan string, 5)

	go func() {
		for qr := range qrChan {
			mu.Lock()
			currentQR = qr
			mu.Unlock()
		}
		// QR handshake done; phone still needs time to finish device linking.
		mu.Lock()
		linking = true
		mu.Unlock()
	}()

	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, authPage)
	})

	mux.HandleFunc("/api/qr", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		resp := map[string]any{
			"qr":            currentQR,
			"linking":       linking,
			"syncing":       syncing,
			"authenticated": authenticated,
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	server := &http.Server{Handler: mux}

	errCh := make(chan error, 1)
	go func() {
		if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	port := ln.Addr().(*net.TCPAddr).Port
	fmt.Printf("Open http://localhost:%d in your browser to scan the QR code\n", port)

	syncSignal := make(chan struct{}, 1)
	handlerID := client.WM().AddEventHandler(func(rawEvt any) {
		if _, ok := rawEvt.(*events.HistorySync); ok {
			select {
			case syncSignal <- struct{}{}:
			default:
			}
		}
	})

	connectErr := client.ConnectWithQR(ctx, qrChan)

	// Transition from linking → syncing and wait for initial history sync.
	mu.Lock()
	linking = false
	syncing = true
	mu.Unlock()

	fmt.Println("Syncing message history, please wait...")
	waitForHistorySync(syncSignal, 45*time.Second, 10*time.Second)
	client.WM().RemoveEventHandler(handlerID)

	mu.Lock()
	syncing = false
	authenticated = true
	mu.Unlock()

	// Give the browser a moment to poll and see the success state.
	time.Sleep(3 * time.Second)
	server.Shutdown(context.Background())

	select {
	case err := <-errCh:
		return err
	default:
	}

	return connectErr
}

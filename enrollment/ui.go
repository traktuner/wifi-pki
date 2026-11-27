package main

import (
	"html/template"
	"net/http"
	"strconv"
)

type homePageData struct {
	Authenticated  bool
	Email          string
	SSID           string
	OnboardingSSID string
	CSRF           string
	Notice         string
	Devices        []deviceRow
}

type readyPageData struct {
	Name           string
	SSID           string
	OnboardingSSID string
}

func stylesheet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(appCSS)))
	_, _ = w.Write([]byte(appCSS))
}

func brandingIcon(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(onCloudFavicon)))
	_, _ = w.Write([]byte(onCloudFavicon))
}

var pageTemplate = template.Must(template.New("page").Parse(`<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <meta name="theme-color" content="#17191d">
  <title>OnCloud WiFi</title>
  <link rel="icon" href="/assets/favicon.svg" type="image/svg+xml">
  <link rel="stylesheet" href="/assets/app.css">
</head>
<body>
  <div class="ambient ambient-one" aria-hidden="true"></div>
  <div class="ambient ambient-two" aria-hidden="true"></div>
  <header class="site-header">
    <a class="brand" href="/" aria-label="OnCloud WiFi Startseite">
      <span class="brand-mark" aria-hidden="true">
        <svg class="oncloud-mark" viewBox="0 0 512 512"><path class="oncloud-container" d="M275 53 L176 53 C99.98 53 53 99.98 53 176 L53 336 C53 412.02 99.98 459 176 459 L336 459 C412.02 459 459 412.02 459 336 L459 275 C459 258.98 446.02 246 430 246 C413.98 246 401 258.98 401 275 L401 336 C401 379.98 379.98 401 336 401 L176 401 C132.02 401 111 379.98 111 336 L111 176 C111 132.02 132.02 111 176 111 L275 111 C291.02 111 304 98.02 304 82 C304 65.98 291.02 53 275 53 Z"/><path class="oncloud-module" d="M459 205 L459 176 C459 109.3 409.64 53 345 53 C328.98 53 316 65.98 316 82 C316 98.02 328.98 111 345 111 C375.78 111 401 139.76 401 176 L401 205 C401 221.02 413.98 234 430 234 C446.02 234 459 221.02 459 205 Z"/></svg>
      </span>
      <span>OnCloud <strong>WiFi</strong></span>
    </a>
    <div class="network-status"><span aria-hidden="true"></span>EAP‑TLS geschützt</div>
  </header>

  {{if .Authenticated}}
  <main class="shell dashboard">
    <section class="dashboard-hero">
      <div>
        <p class="eyebrow">Dein privates Netzwerk</p>
        <h1>Sicher verbunden.<br><span>Ganz ohne Passwort.</span></h1>
        <p class="hero-copy">Ein individuelles Zertifikat verbindet dieses Gerät automatisch und sicher mit <strong>{{.SSID}}</strong>.</p>
      </div>
      <div class="identity-card">
        <span class="identity-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24"><path d="M12 3 5 6v5c0 4.6 2.9 8.8 7 10 4.1-1.2 7-5.4 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg>
        </span>
        <span><small>Angemeldet als</small><strong>{{if .Email}}{{.Email}}{{else}}Berechtigtes OnCloud Konto{{end}}</strong></span>
      </div>
    </section>

    {{if .Notice}}
    <div class="notice" role="status">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m8 12 2.5 2.5L16 9"/><circle cx="12" cy="12" r="9"/></svg>
      <span>{{.Notice}}</span>
    </div>
    {{end}}

    <div class="setup-strip" role="note">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0"/><circle cx="12" cy="19" r="1"/></svg>
      <p><strong>Für die Einrichtung:</strong> Bleib bis zum Abschluss mit <strong>{{.OnboardingSSID}}</strong> verbunden. Das neue Zertifikatsprofil verbindet dieses Gerät danach mit <strong>{{.SSID}}</strong>.</p>
    </div>

    <section class="enrollment-grid" aria-labelledby="enroll-title">
      <div class="glass-card enrollment-card">
        <div class="section-heading">
          <span class="step-number">01</span>
          <div>
            <p class="eyebrow">Neues Gerät</p>
            <h2 id="enroll-title">Geräteprofil erstellen</h2>
          </div>
        </div>
        <p>Gib dem Gerät einen Namen, den du später eindeutig wiedererkennst.</p>
        <form method="post" action="/enroll" class="enroll-form">
          <input type="hidden" name="csrf" value="{{.CSRF}}">
          <label for="device-name">Gerätename</label>
          <div class="input-wrap">
            <svg viewBox="0 0 24 24" aria-hidden="true"><rect x="6" y="2.5" width="12" height="19" rx="2.5"/><path d="M10 18.5h4"/></svg>
            <input id="device-name" name="device_name" minlength="2" maxlength="64" required autocomplete="off" placeholder="z. B. iPhone">
          </div>
          <button class="button button-primary" type="submit">
            Profil sicher erstellen
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6"/></svg>
          </button>
        </form>
      </div>

      <aside class="glass-card steps-card" aria-label="So funktioniert es">
        <p class="eyebrow">In drei Schritten</p>
        <ol class="steps">
          <li><span>1</span><div><strong>Profil erstellen</strong><small>Einmalige, kurzlebige Freigabe</small></div></li>
          <li><span>2</span><div><strong>Am Gerät installieren</strong><small>Der private Schlüssel bleibt am iPhone</small></div></li>
          <li><span>3</span><div><strong>Automatisch verbinden</strong><small>Bis zu fünf Jahre ohne WLAN-Passwort</small></div></li>
        </ol>
      </aside>
    </section>

    <section class="devices-section" aria-labelledby="devices-title">
      <div class="devices-heading">
        <div>
          <p class="eyebrow">Deine Identitäten</p>
          <h2 id="devices-title">Eingerichtete Geräte</h2>
        </div>
        <span class="device-count">{{len .Devices}} {{if eq (len .Devices) 1}}Gerät{{else}}Geräte{{end}}</span>
      </div>

      {{if .Devices}}
      <div class="device-list">
        {{range .Devices}}
        <article class="device-card">
          <div class="device-icon" aria-hidden="true">
            <svg viewBox="0 0 24 24"><rect x="6" y="2.5" width="12" height="19" rx="2.5"/><path d="M10 18.5h4"/></svg>
          </div>
          <div class="device-main">
            <div class="device-title-row">
              <h3>{{.Name}}</h3>
              <span class="status {{.StatusClass}}"><i aria-hidden="true"></i>{{.StatusLabel}}</span>
            </div>
            <p class="device-expiry">Gültig bis <time datetime="{{.ExpiresISO}}">{{.Expires}}</time></p>
            <details>
              <summary>Zertifikatsdetails</summary>
              <code>{{.UUID}}</code>
            </details>
          </div>
          <div class="device-actions">
            <form method="post" action="/enroll">
              <input type="hidden" name="csrf" value="{{$.CSRF}}">
              <input type="hidden" name="device_name" value="{{.Name}}">
              <button class="button button-secondary" type="submit">Neues Profil</button>
            </form>
            {{if .CanCancel}}
            <form method="post" action="/device/cancel">
              <input type="hidden" name="csrf" value="{{$.CSRF}}">
              <input type="hidden" name="device_uuid" value="{{.UUID}}">
              <button class="button button-danger" type="submit">Offene Freigabe löschen</button>
            </form>
            {{end}}
          </div>
        </article>
        {{end}}
      </div>
      {{else}}
      <div class="empty-state">
        <span aria-hidden="true"><svg viewBox="0 0 24 24"><rect x="6" y="2.5" width="12" height="19" rx="2.5"/><path d="M10 18.5h4"/></svg></span>
        <h3>Noch kein Gerät eingerichtet</h3>
        <p>Dein erstes Geräteprofil erscheint nach der Ausstellung automatisch hier.</p>
      </div>
      {{end}}
    </section>

    <div class="info-strip">
      <svg viewBox="0 0 24 24" aria-hidden="true"><circle cx="12" cy="12" r="9"/><path d="M12 11v6M12 7.5v.5"/></svg>
      <p><strong>Gut zu wissen:</strong> Jedes Profil erhält eine neue Geräteidentität. Aktive Geräte bleiben als Sicherheitsinventar sichtbar: Das Löschen des Profils am iPhone widerruft das Zertifikat nicht. Geladene Profile und private Schlüssel werden nicht im Portal gespeichert.</p>
    </div>
  </main>
  {{else}}
  <main class="shell landing">
    <section class="hero">
      <div class="hero-content">
        <div class="secure-pill">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 11V8a5 5 0 0 1 10 0v3"/><rect x="4" y="11" width="16" height="10" rx="3"/></svg>
          Privates OnCloud Netzwerk
        </div>
        <h1>WLAN, das einfach<br><span>sicher bleibt.</span></h1>
        <p>Einmal mit deinem OnCloud Konto anmelden. Geräteprofil installieren. Danach verbindet sich dein Gerät automatisch — ohne gemeinsames WLAN-Passwort.</p>
        <a class="button button-primary button-login" href="/login">
          Sicher mit Keycloak anmelden
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m9 18 6-6-6-6"/></svg>
        </a>
        <p class="login-note">
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.6 2.9 8.8 7 10 4.1-1.2 7-5.4 7-10V6l-7-3Z"/></svg>
          Passkey und MFA werden direkt von Keycloak geprüft
        </p>
      </div>
      <div class="signal-visual" aria-hidden="true">
        <div class="signal-orbit orbit-one"></div>
        <div class="signal-orbit orbit-two"></div>
        <div class="signal-orbit orbit-three"></div>
        <div class="signal-core">
          <svg viewBox="0 0 40 40"><path d="M8 18a17 17 0 0 1 24 0"/><path d="M13 24a10 10 0 0 1 14 0"/><circle cx="20" cy="30" r="2"/></svg>
        </div>
        <span class="floating-chip chip-top">EAP‑TLS</span>
        <span class="floating-chip chip-bottom">5 Jahre</span>
      </div>
    </section>

    <div class="setup-strip landing-setup" role="note">
      <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0"/><circle cx="12" cy="19" r="1"/></svg>
      <p><strong>Neues Gerät?</strong> Verbinde es zuerst mit <strong>{{.OnboardingSSID}}</strong> oder nutze Mobilfunk. Nach der Profilinstallation wechselt es zu <strong>{{.SSID}}</strong>.</p>
    </div>

    <section class="benefits" aria-label="Vorteile">
      <article><span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 11V8a5 5 0 0 1 10 0v3"/><rect x="4" y="11" width="16" height="10" rx="3"/></svg></span><div><h2>Schlüssel bleibt privat</h2><p>Er entsteht direkt auf deinem Gerät und verlässt es nie.</p></div></article>
      <article><span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 6v6l4 2"/><circle cx="12" cy="12" r="9"/></svg></span><div><h2>Fünf Jahre Ruhe</h2><p>Im Alltag verbindet sich dein Gerät ohne weitere Eingabe.</p></div></article>
      <article><span><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.6 2.9 8.8 7 10 4.1-1.2 7-5.4 7-10V6l-7-3Z"/><path d="m9 12 2 2 4-4"/></svg></span><div><h2>Individuell sperrbar</h2><p>Verlorene Geräte lassen sich einzeln entfernen.</p></div></article>
    </section>
  </main>
  {{end}}

  <footer>
    <span>OnCloud WiFi</span>
    <span class="footer-dot" aria-hidden="true"></span>
    <span>Private PKI · Keine Passwörter im WLAN</span>
  </footer>
</body>
</html>`))

var readyTemplate = template.Must(template.New("ready").Parse(`<!doctype html>
<html lang="de">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover">
  <meta name="theme-color" content="#17191d">
  <title>OnCloud WiFi Profil</title>
  <link rel="icon" href="/assets/favicon.svg" type="image/svg+xml">
  <link rel="stylesheet" href="/assets/app.css">
</head>
<body>
  <div class="ambient ambient-one" aria-hidden="true"></div>
  <div class="ambient ambient-two" aria-hidden="true"></div>
  <header class="site-header">
    <a class="brand" href="/" aria-label="OnCloud WiFi Startseite">
      <span class="brand-mark" aria-hidden="true"><svg class="oncloud-mark" viewBox="0 0 512 512"><path class="oncloud-container" d="M275 53 L176 53 C99.98 53 53 99.98 53 176 L53 336 C53 412.02 99.98 459 176 459 L336 459 C412.02 459 459 412.02 459 336 L459 275 C459 258.98 446.02 246 430 246 C413.98 246 401 258.98 401 275 L401 336 C401 379.98 379.98 401 336 401 L176 401 C132.02 401 111 379.98 111 336 L111 176 C111 132.02 132.02 111 176 111 L275 111 C291.02 111 304 98.02 304 82 C304 65.98 291.02 53 275 53 Z"/><path class="oncloud-module" d="M459 205 L459 176 C459 109.3 409.64 53 345 53 C328.98 53 316 65.98 316 82 C316 98.02 328.98 111 345 111 C375.78 111 401 139.76 401 176 L401 205 C401 221.02 413.98 234 430 234 C446.02 234 459 221.02 459 205 Z"/></svg></span>
      <span>OnCloud <strong>WiFi</strong></span>
    </a>
    <div class="network-status"><span aria-hidden="true"></span>Profil geschützt</div>
  </header>

  <main class="shell ready-shell">
    <section class="glass-card ready-card">
      <div class="success-mark" aria-hidden="true">
        <svg viewBox="0 0 32 32"><path d="m9 16 5 5 9-10"/></svg>
      </div>
      <p class="eyebrow">Sicher erstellt</p>
      <h1>Dein Profil ist bereit.</h1>
      <p class="ready-lead">Das individuelle Profil für <strong>{{.Name}}</strong> verbindet dieses Gerät mit <strong>{{.SSID}}</strong>.</p>

      <div class="setup-strip ready-setup" role="note">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12.5a10 10 0 0 1 14 0M8.5 16a5 5 0 0 1 7 0"/><circle cx="12" cy="19" r="1"/></svg>
        <p>Bleib während Download und Installation mit <strong>{{.OnboardingSSID}}</strong> verbunden. Erst das fertige Profil wechselt zu <strong>{{.SSID}}</strong>.</p>
      </div>

      <a class="button button-primary download-button" href="/download">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3v12m0 0 5-5m-5 5-5-5M5 20h14"/></svg>
        Profil jetzt laden
      </a>
      <p class="expiry-note">Einmaliger Download · 5 Minuten verfügbar</p>

      <div class="install-guide">
        <h2>Danach am iPhone</h2>
        <ol>
          <li><span>1</span><p>Download in Safari bestätigen</p></li>
          <li><span>2</span><p><strong>Einstellungen</strong> öffnen und auf <strong>Profil geladen</strong> tippen</p></li>
          <li><span>3</span><p>Profil prüfen und oben rechts <strong>Installieren</strong> wählen</p></li>
          <li><span>4</span><p>Das Gerät verbindet sich automatisch mit <strong>{{.SSID}}</strong></p></li>
        </ol>
      </div>

      <div class="privacy-warning">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 3 5 6v5c0 4.6 2.9 8.8 7 10 4.1-1.2 7-5.4 7-10V6l-7-3Z"/></svg>
        <p>Für dieses WLAN wird die private WLAN-Adresse ausgeschaltet. Apple kann deshalb einen Datenschutzhinweis anzeigen; das Gerät verwendet in diesem privaten Netz seine Hardware‑MAC.</p>
      </div>

      <a class="back-link" href="/">
        <svg viewBox="0 0 24 24" aria-hidden="true"><path d="m15 18-6-6 6-6"/></svg>
        Zur Geräteübersicht
      </a>
    </section>
  </main>
</body>
</html>`))

const onCloudFavicon = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
  <rect width="512" height="512" rx="96" fill="#17191d"/>
  <g transform="translate(20.48 20.48) scale(.92)">
    <path fill="#f7f8fa" d="M275 53 L176 53 C99.98 53 53 99.98 53 176 L53 336 C53 412.02 99.98 459 176 459 L336 459 C412.02 459 459 412.02 459 336 L459 275 C459 258.98 446.02 246 430 246 C413.98 246 401 258.98 401 275 L401 336 C401 379.98 379.98 401 336 401 L176 401 C132.02 401 111 379.98 111 336 L111 176 C111 132.02 132.02 111 176 111 L275 111 C291.02 111 304 98.02 304 82 C304 65.98 291.02 53 275 53 Z"/>
    <path fill="#817aa8" d="M459 205 L459 176 C459 109.3 409.64 53 345 53 C328.98 53 316 65.98 316 82 C316 98.02 328.98 111 345 111 C375.78 111 401 139.76 401 176 L401 205 C401 221.02 413.98 234 430 234 C446.02 234 459 221.02 459 205 Z"/>
  </g>
</svg>`

const appCSS = `
:root {
  color-scheme: dark;
  --bg: #07111f;
  --bg-deep: #040a13;
  --surface: rgba(15, 30, 49, .72);
  --surface-strong: #102239;
  --border: rgba(171, 213, 255, .13);
  --border-bright: rgba(116, 203, 255, .32);
  --text: #f7fbff;
  --muted: #9eb0c3;
  --blue: #49a8ff;
  --cyan: #5de4d1;
  --danger: #ff7b89;
  --shadow: 0 28px 80px rgba(0, 0, 0, .35);
  --radius: 24px;
  font-family: -apple-system, BlinkMacSystemFont, "SF Pro Display", "Segoe UI", sans-serif;
  font-synthesis: none;
}

* { box-sizing: border-box; }
html { min-height: 100%; background: var(--bg-deep); }
body {
  min-height: 100vh;
  margin: 0;
  color: var(--text);
  background:
    radial-gradient(circle at 18% -10%, rgba(52, 137, 235, .17), transparent 34rem),
    radial-gradient(circle at 88% 22%, rgba(32, 196, 178, .10), transparent 30rem),
    linear-gradient(145deg, var(--bg-deep), var(--bg) 48%, #081729);
  line-height: 1.55;
  overflow-x: hidden;
}

a { color: inherit; }
button, input { font: inherit; }
svg { display: block; fill: none; stroke: currentColor; stroke-width: 1.8; stroke-linecap: round; stroke-linejoin: round; }
.ambient {
  position: fixed;
  width: 26rem;
  height: 26rem;
  border-radius: 50%;
  filter: blur(90px);
  pointer-events: none;
  opacity: .12;
}
.ambient-one { top: -13rem; left: -8rem; background: var(--blue); }
.ambient-two { right: -13rem; top: 35%; background: var(--cyan); }

.site-header {
  width: min(1180px, calc(100% - 40px));
  height: 88px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: space-between;
  position: relative;
  z-index: 2;
}
.brand {
  display: inline-flex;
  align-items: center;
  gap: 11px;
  color: var(--text);
  text-decoration: none;
  font-size: 1.08rem;
  letter-spacing: -.02em;
}
.brand strong { font-weight: 700; }
.brand-mark {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
}
.brand-mark svg { width: 38px; height: 38px; }
.brand-mark .oncloud-container { fill: #f7f8fa; stroke: none; }
.brand-mark .oncloud-module { fill: #817aa8; stroke: none; }
.network-status {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--muted);
  font-size: .78rem;
  font-weight: 650;
  letter-spacing: .04em;
  text-transform: uppercase;
}
.network-status > span {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--cyan);
  box-shadow: 0 0 0 4px rgba(93, 228, 209, .1), 0 0 16px rgba(93, 228, 209, .8);
}
.shell { width: min(1180px, calc(100% - 40px)); margin: 0 auto; position: relative; z-index: 1; }

.landing { padding: 70px 0 28px; }
.hero {
  min-height: 490px;
  display: grid;
  grid-template-columns: minmax(0, 1.05fr) minmax(340px, .95fr);
  gap: 70px;
  align-items: center;
}
.secure-pill {
  display: inline-flex;
  align-items: center;
  gap: 9px;
  padding: 8px 13px;
  color: #bceeff;
  border: 1px solid rgba(93, 228, 209, .22);
  border-radius: 999px;
  background: rgba(32, 147, 160, .08);
  font-size: .78rem;
  font-weight: 700;
  letter-spacing: .06em;
  text-transform: uppercase;
}
.secure-pill svg { width: 16px; height: 16px; }
.hero h1, .dashboard-hero h1, .ready-card h1 {
  margin: 25px 0 20px;
  font-size: clamp(3.1rem, 6vw, 5.6rem);
  line-height: .98;
  letter-spacing: -.065em;
  font-weight: 760;
}
.hero h1 span, .dashboard-hero h1 span {
  color: transparent;
  background: linear-gradient(100deg, #8ccaff, #70e5d5);
  -webkit-background-clip: text;
  background-clip: text;
}
.hero-content > p {
  max-width: 610px;
  margin: 0;
  color: var(--muted);
  font-size: 1.12rem;
}
.button {
  min-height: 48px;
  padding: 0 18px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  border: 1px solid transparent;
  border-radius: 13px;
  font-weight: 700;
  text-decoration: none;
  cursor: pointer;
  transition: transform .18s ease, border-color .18s ease, background .18s ease, box-shadow .18s ease;
}
.button:hover { transform: translateY(-1px); }
.button:active { transform: translateY(0); }
.button:focus-visible, input:focus-visible, summary:focus-visible, a:focus-visible {
  outline: 3px solid rgba(116, 203, 255, .48);
  outline-offset: 3px;
}
.button svg { width: 19px; height: 19px; stroke-width: 2.2; }
.button-primary {
  color: #04111c;
  background: linear-gradient(135deg, #72e9d8, #59adff);
  box-shadow: 0 14px 36px rgba(45, 157, 219, .22);
}
.button-primary:hover { box-shadow: 0 17px 42px rgba(45, 157, 219, .34); }
.button-secondary { color: #dbeeff; border-color: var(--border); background: rgba(91, 151, 199, .08); }
.button-secondary:hover { border-color: var(--border-bright); background: rgba(91, 151, 199, .13); }
.button-danger { min-height: 40px; color: #ffabb4; border-color: rgba(255, 123, 137, .18); background: transparent; font-size: .82rem; }
.button-danger:hover { border-color: rgba(255, 123, 137, .4); background: rgba(255, 123, 137, .08); }
.button-login { min-height: 57px; margin-top: 31px; padding: 0 23px; }
.login-note {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-top: 18px !important;
  font-size: .8rem !important;
}
.login-note svg { width: 16px; height: 16px; color: var(--cyan); }

.signal-visual { height: 430px; position: relative; display: grid; place-items: center; }
.signal-core {
  width: 124px;
  height: 124px;
  display: grid;
  place-items: center;
  color: #b9fff4;
  border: 1px solid rgba(116, 203, 255, .35);
  border-radius: 38px;
  background: linear-gradient(145deg, rgba(33, 83, 120, .78), rgba(14, 32, 51, .9));
  box-shadow: 0 25px 65px rgba(0, 0, 0, .38), inset 0 1px rgba(255, 255, 255, .12);
  transform: rotate(-3deg);
  z-index: 2;
}
.signal-core svg { width: 68px; height: 68px; stroke-width: 2; }
.signal-orbit { position: absolute; border: 1px solid rgba(96, 187, 242, .12); border-radius: 50%; }
.orbit-one { width: 220px; height: 220px; }
.orbit-two { width: 330px; height: 330px; border-color: rgba(93, 228, 209, .09); }
.orbit-three { width: 430px; height: 430px; border-color: rgba(96, 187, 242, .06); }
.floating-chip {
  position: absolute;
  z-index: 3;
  padding: 9px 13px;
  color: #cde9ff;
  border: 1px solid var(--border);
  border-radius: 11px;
  background: rgba(13, 29, 47, .88);
  box-shadow: 0 14px 30px rgba(0, 0, 0, .22);
  font-size: .76rem;
  font-weight: 750;
  letter-spacing: .06em;
}
.chip-top { top: 72px; right: 54px; }
.chip-bottom { bottom: 72px; left: 50px; }

.benefits {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: rgba(12, 26, 43, .55);
  backdrop-filter: blur(20px);
  overflow: hidden;
}
.benefits article { min-height: 138px; display: flex; gap: 16px; align-items: flex-start; padding: 29px; }
.benefits article + article { border-left: 1px solid var(--border); }
.benefits article > span {
  width: 38px;
  height: 38px;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  color: #8adfd9;
  border-radius: 11px;
  background: rgba(93, 228, 209, .08);
}
.benefits svg { width: 20px; height: 20px; }
.benefits h2 { margin: 0 0 5px; font-size: .94rem; letter-spacing: -.01em; }
.benefits p { margin: 0; color: var(--muted); font-size: .83rem; line-height: 1.5; }

.dashboard { padding: 66px 0 30px; }
.dashboard-hero { display: flex; align-items: flex-end; justify-content: space-between; gap: 30px; margin-bottom: 43px; }
.eyebrow { margin: 0 0 8px; color: #78d9d0; font-size: .71rem; font-weight: 800; letter-spacing: .13em; text-transform: uppercase; }
.dashboard-hero h1 { margin: 0 0 18px; font-size: clamp(2.7rem, 5vw, 4.6rem); }
.hero-copy { max-width: 610px; margin: 0; color: var(--muted); font-size: 1.02rem; }
.identity-card {
  min-width: 250px;
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 13px 16px;
  border: 1px solid var(--border);
  border-radius: 16px;
  background: rgba(12, 27, 44, .62);
}
.identity-icon {
  width: 37px;
  height: 37px;
  display: grid;
  place-items: center;
  color: var(--cyan);
  border-radius: 11px;
  background: rgba(93, 228, 209, .08);
}
.identity-icon svg { width: 21px; height: 21px; }
.identity-card small, .identity-card strong { display: block; }
.identity-card small { color: var(--muted); font-size: .68rem; }
.identity-card strong { max-width: 220px; margin-top: 1px; overflow: hidden; text-overflow: ellipsis; font-size: .83rem; white-space: nowrap; }
.notice {
  display: flex;
  align-items: center;
  gap: 11px;
  margin: -18px 0 28px;
  padding: 13px 15px;
  color: #bdf8e9;
  border: 1px solid rgba(93, 228, 209, .2);
  border-radius: 13px;
  background: rgba(33, 154, 137, .09);
}
.notice svg { width: 20px; height: 20px; flex: 0 0 auto; }
.setup-strip {
  display: flex;
  align-items: flex-start;
  gap: 13px;
  margin: 0 0 24px;
  padding: 16px 18px;
  border: 1px solid rgba(93, 228, 209, .24);
  border-radius: 16px;
  color: #c8f8f0;
  background: rgba(31, 123, 116, .13);
}
.setup-strip svg { width: 22px; height: 22px; flex: 0 0 auto; margin-top: 1px; color: var(--cyan); }
.setup-strip p { margin: 0; }
.landing-setup { max-width: 760px; margin: 0 auto 28px; }
.ready-setup { margin: 24px 0 0; text-align: left; }
.glass-card {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  background: linear-gradient(145deg, rgba(19, 39, 62, .72), rgba(10, 23, 39, .78));
  box-shadow: var(--shadow);
  backdrop-filter: blur(22px);
}
.enrollment-grid { display: grid; grid-template-columns: minmax(0, 1.45fr) minmax(300px, .75fr); gap: 20px; }
.enrollment-card, .steps-card { padding: 29px; }
.section-heading { display: flex; gap: 14px; align-items: center; }
.step-number {
  width: 45px;
  height: 45px;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  color: #83e7dd;
  border: 1px solid rgba(93, 228, 209, .19);
  border-radius: 14px;
  background: rgba(93, 228, 209, .07);
  font-size: .74rem;
  font-weight: 800;
}
.section-heading h2, .devices-heading h2 { margin: 0; font-size: 1.38rem; letter-spacing: -.025em; }
.enrollment-card > p { margin: 21px 0 19px; color: var(--muted); font-size: .9rem; }
.enroll-form label { display: block; margin-bottom: 7px; color: #c1d0df; font-size: .76rem; font-weight: 700; }
.input-wrap { position: relative; margin-bottom: 13px; }
.input-wrap svg { position: absolute; left: 15px; top: 50%; width: 20px; height: 20px; color: #738ba3; transform: translateY(-50%); pointer-events: none; }
.input-wrap input {
  width: 100%;
  min-height: 54px;
  padding: 0 16px 0 46px;
  color: var(--text);
  border: 1px solid rgba(158, 188, 218, .17);
  border-radius: 13px;
  outline: none;
  background: rgba(3, 12, 22, .46);
  transition: border-color .18s ease, box-shadow .18s ease;
}
.input-wrap input::placeholder { color: #60758a; }
.input-wrap input:focus { border-color: rgba(93, 228, 209, .48); box-shadow: 0 0 0 4px rgba(93, 228, 209, .07); }
.enroll-form .button { width: 100%; min-height: 54px; }
.steps-card { box-shadow: none; }
.steps { margin: 22px 0; padding: 0; list-style: none; }
.steps li { display: flex; gap: 12px; position: relative; padding-bottom: 19px; }
.steps li:not(:last-child)::after { content: ""; position: absolute; left: 14px; top: 31px; bottom: 3px; width: 1px; background: var(--border); }
.steps li > span {
  width: 29px;
  height: 29px;
  flex: 0 0 auto;
  display: grid;
  place-items: center;
  color: #8ee9df;
  border: 1px solid rgba(93, 228, 209, .18);
  border-radius: 50%;
  background: rgba(93, 228, 209, .06);
  font-size: .69rem;
  font-weight: 800;
}
.steps strong, .steps small { display: block; }
.steps strong { font-size: .85rem; }
.steps small { margin-top: 2px; color: var(--muted); font-size: .73rem; }
.devices-section { padding-top: 54px; }
.devices-heading { display: flex; align-items: flex-end; justify-content: space-between; margin-bottom: 18px; }
.device-count { padding: 6px 10px; color: var(--muted); border: 1px solid var(--border); border-radius: 999px; font-size: .73rem; }
.device-list { display: grid; gap: 11px; }
.device-card {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr) auto;
  gap: 16px;
  align-items: center;
  padding: 18px;
  border: 1px solid var(--border);
  border-radius: 17px;
  background: rgba(12, 27, 44, .58);
  transition: border-color .18s ease, background .18s ease;
}
.device-card:hover { border-color: rgba(116, 203, 255, .22); background: rgba(15, 33, 53, .72); }
.device-icon {
  width: 43px;
  height: 43px;
  display: grid;
  place-items: center;
  color: #91d9ff;
  border-radius: 13px;
  background: rgba(73, 168, 255, .08);
}
.device-icon svg { width: 22px; height: 22px; }
.device-title-row { display: flex; gap: 10px; align-items: center; flex-wrap: wrap; }
.device-title-row h3 { margin: 0; font-size: .97rem; letter-spacing: -.01em; }
.status { display: inline-flex; align-items: center; gap: 6px; font-size: .66rem; font-weight: 750; letter-spacing: .04em; text-transform: uppercase; }
.status i { width: 6px; height: 6px; border-radius: 50%; background: currentColor; box-shadow: 0 0 10px currentColor; }
.status-good { color: #69dfc9; }
.status-warn { color: #ffd381; }
.status-danger { color: #ff8f9b; }
.status-neutral { color: #9eb0c3; }
.device-expiry { margin: 4px 0 0; color: var(--muted); font-size: .76rem; }
details { margin-top: 5px; color: #728ba2; font-size: .68rem; }
summary { width: max-content; cursor: pointer; }
details code { display: block; max-width: 100%; margin-top: 5px; overflow-wrap: anywhere; color: #829bb2; }
.device-actions { display: flex; align-items: center; gap: 8px; }
.device-actions form { margin: 0; }
.device-actions .button { min-height: 40px; font-size: .79rem; }
.empty-state { padding: 49px 20px; text-align: center; border: 1px dashed rgba(158, 188, 218, .17); border-radius: 18px; background: rgba(12, 27, 44, .34); }
.empty-state > span { width: 50px; height: 50px; display: grid; place-items: center; margin: 0 auto 14px; color: #82d8ef; border-radius: 15px; background: rgba(73, 168, 255, .08); }
.empty-state svg { width: 25px; height: 25px; }
.empty-state h3 { margin: 0 0 6px; font-size: 1rem; }
.empty-state p { margin: 0; color: var(--muted); font-size: .83rem; }
.info-strip { display: flex; gap: 11px; align-items: flex-start; margin-top: 22px; padding: 15px 17px; color: var(--muted); border-radius: 13px; background: rgba(73, 100, 127, .08); font-size: .78rem; }
.info-strip svg { width: 19px; height: 19px; flex: 0 0 auto; color: #75c8ed; }
.info-strip p { margin: 0; }
.info-strip strong { color: #c7d7e7; }

.ready-shell { max-width: 710px; padding: 55px 0 50px; }
.ready-card { padding: 43px; text-align: center; }
.success-mark {
  width: 67px;
  height: 67px;
  display: grid;
  place-items: center;
  margin: 0 auto 19px;
  color: #80ead9;
  border: 1px solid rgba(93, 228, 209, .26);
  border-radius: 21px;
  background: rgba(93, 228, 209, .08);
  box-shadow: 0 12px 40px rgba(34, 193, 173, .10);
}
.success-mark svg { width: 34px; height: 34px; stroke-width: 2.4; }
.ready-card h1 { margin: 4px 0 13px; font-size: clamp(2.4rem, 6vw, 3.8rem); }
.ready-lead { max-width: 500px; margin: 0 auto; color: var(--muted); }
.download-button { min-height: 57px; margin-top: 28px; padding: 0 25px; }
.expiry-note { margin: 9px 0 0; color: #70869b; font-size: .71rem; }
.install-guide { margin-top: 34px; padding: 24px; text-align: left; border: 1px solid var(--border); border-radius: 17px; background: rgba(5, 15, 26, .36); }
.install-guide h2 { margin: 0 0 18px; font-size: .98rem; }
.install-guide ol { display: grid; gap: 14px; margin: 0; padding: 0; list-style: none; }
.install-guide li { display: flex; align-items: center; gap: 12px; }
.install-guide li > span { width: 28px; height: 28px; flex: 0 0 auto; display: grid; place-items: center; color: #79ded3; border: 1px solid rgba(93, 228, 209, .19); border-radius: 50%; font-size: .7rem; font-weight: 800; }
.install-guide p { margin: 0; color: #b6c5d4; font-size: .82rem; }
.install-guide strong { color: var(--text); }
.privacy-warning { display: flex; gap: 11px; align-items: flex-start; margin-top: 14px; padding: 14px; text-align: left; color: #a7bacb; border-radius: 13px; background: rgba(255, 205, 108, .055); font-size: .75rem; }
.privacy-warning svg { width: 19px; height: 19px; flex: 0 0 auto; color: #e7c372; }
.privacy-warning p { margin: 0; }
.back-link { display: inline-flex; align-items: center; gap: 5px; margin-top: 25px; color: #8fa7ba; font-size: .8rem; text-decoration: none; }
.back-link:hover { color: var(--text); }
.back-link svg { width: 16px; height: 16px; }

footer {
  width: min(1180px, calc(100% - 40px));
  min-height: 85px;
  margin: 0 auto;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 10px;
  color: #536a80;
  font-size: .72rem;
}
.footer-dot { width: 3px; height: 3px; border-radius: 50%; background: #536a80; }

@media (max-width: 860px) {
  .landing { padding-top: 38px; }
  .hero { min-height: auto; grid-template-columns: 1fr; gap: 25px; }
  .hero-content { text-align: center; }
  .hero-content > p { margin-inline: auto; }
  .signal-visual { height: 310px; }
  .orbit-three { width: 310px; height: 310px; }
  .orbit-two { width: 245px; height: 245px; }
  .orbit-one { width: 180px; height: 180px; }
  .signal-core { width: 105px; height: 105px; border-radius: 31px; }
  .chip-top { top: 28px; right: 13%; }
  .chip-bottom { bottom: 25px; left: 12%; }
  .benefits { grid-template-columns: 1fr; }
  .benefits article { min-height: auto; }
  .benefits article + article { border-left: 0; border-top: 1px solid var(--border); }
  .dashboard-hero { align-items: flex-start; flex-direction: column; }
  .identity-card { width: 100%; }
  .enrollment-grid { grid-template-columns: 1fr; }
  .device-card { grid-template-columns: auto minmax(0, 1fr); align-items: start; }
  .device-actions { grid-column: 1 / -1; padding-left: 59px; }
}

@media (max-width: 560px) {
  .site-header, .shell, footer { width: min(100% - 28px, 1180px); }
  .site-header { height: 74px; }
  .network-status { font-size: 0; }
  .network-status > span { display: block; }
  .landing { padding-top: 35px; }
  .hero h1 { font-size: clamp(2.75rem, 14vw, 4rem); }
  .hero-content > p { font-size: .98rem; }
  .button-login { width: 100%; }
  .signal-visual { height: 270px; }
  .orbit-three { width: 270px; height: 270px; }
  .orbit-two { width: 215px; height: 215px; }
  .orbit-one { width: 160px; height: 160px; }
  .signal-core { width: 94px; height: 94px; }
  .signal-core svg { width: 55px; height: 55px; }
  .benefits article { padding: 23px; }
  .dashboard { padding-top: 42px; }
  .dashboard-hero { margin-bottom: 32px; }
  .dashboard-hero h1 { font-size: 2.75rem; }
  .enrollment-card, .steps-card { padding: 22px; }
  .glass-card { border-radius: 20px; }
  .devices-section { padding-top: 42px; }
  .device-card { padding: 15px; gap: 12px; }
  .device-actions { padding-left: 55px; align-items: stretch; flex-direction: column; }
  .device-actions form, .device-actions .button { width: 100%; }
  .ready-shell { padding-top: 28px; }
  .ready-card { padding: 28px 20px; }
  .install-guide { padding: 19px; }
  footer { flex-wrap: wrap; text-align: center; padding: 24px 0 calc(24px + env(safe-area-inset-bottom)); }
}

@media (prefers-reduced-motion: reduce) {
  *, *::before, *::after { scroll-behavior: auto !important; transition: none !important; }
}

@media (prefers-contrast: more) {
  :root { --border: rgba(210, 235, 255, .35); --muted: #c1cfdd; }
}
`

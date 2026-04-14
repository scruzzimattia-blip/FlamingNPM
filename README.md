# FlamingNPM — Web Application Firewall

Eine massgeschneiderte Web Application Firewall (WAF), die als Docker-Container und Reverse Proxy vor dem NGINX Proxy Manager betrieben wird. Inklusive Web-Dashboard zur Verwaltung von Regeln, Live-Logs und IP-Sperren.

## Architektur

```
Internet → [FlamingNPM WAF :8080] → [NGINX Proxy Manager :81] → Backend-Services
                  ↕
          [Dashboard :8443]
```

### Komponenten

| Komponente | Technologie | Beschreibung |
|---|---|---|
| **Reverse Proxy** | Go (`net/http/httputil`) | Performanter Proxy mit WAF-Middleware |
| **WAF-Engine** | Go + Regex | Prueft Header, Body, URI und Parameter |
| **REST-API** | Go + Gorilla Mux | CRUD fuer Regeln, Logs, IP-Sperren |
| **Dashboard** | React + Vite | Live-Logs, Regelverwaltung, IP-Blocking |
| **Datenbank** | SQLite (WAL-Modus) | Regeln, Logs, IP-Sperren, Rate-Limiting |
| **WebSocket** | Gorilla WebSocket | Echtzeit-Updates im Dashboard |

## Funktionen

### Integrierte Schutzregeln

Die WAF wird mit folgenden vordefinierten Regeln ausgeliefert:

- **SQL Injection** — Union-Based, Boolean-Based, Comment/Stacked Queries
- **Cross-Site Scripting (XSS)** — Script-Tags, Event-Handler, Data-URIs
- **Path Traversal** — Verzeichnistraversierung, `/etc/passwd`, `/proc/self`
- **Command Injection** — Shell-Befehle via Pipe, Semikolon, Backticks
- **Log4Shell / JNDI** — CVE-2021-44228 Erkennung

### Dashboard

- **Live-Logs**: Blockierte Anfragen in Echtzeit via WebSocket
- **Firewall-Regeln**: Erstellen, bearbeiten, aktivieren/deaktivieren von Blacklist- und Whitelist-Regeln zur Laufzeit
- **IP-Sperren**: Manuelle IP-Sperren (permanent oder zeitlich begrenzt)
- **Statistiken**: Gesamtzahl Blocks, Blocks heute, aktive Regeln, gesperrte IPs

### Rate-Limiting

Automatische temporaere Sperrung bei zu vielen Anfragen pro Zeitfenster. Konfigurierbar ueber Umgebungsvariablen.

## Schnellstart

### Mit Docker Compose

```bash
# Repository klonen
git clone https://github.com/flamingnpm/waf.git
cd waf

# Starten
docker compose up -d

# Dashboard oeffnen
open http://localhost:8443
```

### Umgebungsvariablen

| Variable | Standard | Beschreibung |
|---|---|---|
| `BACKEND_URL` | `http://nginx-proxy-manager:81` | Ziel-URL fuer weitergeleitete Anfragen |
| `LISTEN_ADDR` | `:8080` | Adresse des WAF-Proxy |
| `API_ADDR` | `:8443` | Adresse des Dashboards und der API |
| `DB_PATH` | `/data/waf.db` | Pfad zur SQLite-Datenbank |
| `MAX_BODY_SIZE` | `1048576` | Maximale Body-Groesse in Bytes (Standard: 1 MB) |
| `RATE_LIMIT_MAX` | `100` | Max. Anfragen pro Zeitfenster |
| `RATE_LIMIT_WINDOW` | `60` | Zeitfenster in Sekunden |
| `WAF_SCORE_THRESHOLD` | `50` | Bedrohungs-Score ab dem blockiert wird (Summe der Regel-Gewichte) |

## Projektstruktur

```
FlamingNPM/
├── cmd/waf/
│   └── main.go              # Einstiegspunkt der Applikation
├── internal/
│   ├── api/
│   │   ├── handlers.go      # REST-API-Handler (Regeln, Logs, IPs)
│   │   └── websocket.go     # WebSocket-Hub fuer Live-Updates
│   ├── database/
│   │   └── database.go      # SQLite-Datenschicht mit Migrations
│   ├── models/
│   │   └── models.go        # Datenmodelle
│   ├── proxy/
│   │   └── proxy.go         # Reverse Proxy mit WAF-Integration
│   └── waf/
│       └── engine.go        # WAF-Engine mit Regex-Matching
├── web/frontend/
│   ├── src/
│   │   ├── components/      # React-Komponenten (Dashboard, Logs, etc.)
│   │   ├── hooks/           # Custom Hooks (API, WebSocket)
│   │   ├── App.jsx          # Haupt-App mit Navigation
│   │   └── main.jsx         # React-Einstiegspunkt
│   ├── index.html
│   ├── package.json
│   └── vite.config.js
├── .github/workflows/
│   ├── ci.yml               # Lint/Tests auf Feature-Branches und PRs
│   └── release.yml          # Version, Docker-Image und Release nur auf main
├── Dockerfile               # Multi-Stage-Build (Node + Go + Alpine)
├── docker-compose.yml       # Lokale Entwicklungsumgebung
└── README.md
```

## API-Endpunkte

| Methode | Pfad | Beschreibung |
|---|---|---|
| `GET` | `/api/stats` | Dashboard-Statistiken |
| `GET` | `/api/rules` | Alle Firewall-Regeln auflisten |
| `POST` | `/api/rules` | Neue Regel erstellen |
| `PUT` | `/api/rules/:id` | Regel aktualisieren |
| `DELETE` | `/api/rules/:id` | Regel loeschen |
| `POST` | `/api/rules/reload` | Regeln neu laden |
| `GET` | `/api/logs?limit=100&offset=0` | Blockierte Anfragen abfragen |
| `GET` | `/api/ip-blocks` | Gesperrte IPs auflisten |
| `POST` | `/api/ip-blocks` | IP sperren |
| `DELETE` | `/api/ip-blocks/:id` | IP-Sperre aufheben |
| `WS` | `/api/ws` | WebSocket fuer Live-Updates |

## Versionierung & CI/CD

### Feature-Branches und Pull Requests

Workflow [`.github/workflows/ci.yml`](.github/workflows/ci.yml): bei jedem Push auf einen Branch **ausser** `main` sowie bei **allen** Pull Requests werden ausgefuehrt:

- Go: `gofmt`-Pruefung, `go vet`, `go test` (mit CGO/SQLite)
- Frontend: `npm install` und Produktions-Build (`vite build`)

### Merge nach `main` (Release)

Workflow [`.github/workflows/release.yml`](.github/workflows/release.yml): nur bei **Push auf `main`** (z. B. nach bestaetigtem Merge eines Pull Requests):

1. Go-Tests
2. Versionierung: start bei `v0.0.0` (Datei `VERSION`), sonst automatische Patch-Erhoehung
3. Git-Tag und GitHub Release mit Release Notes
4. Docker-Image wird publiziert mit Tags:
   - `vX.Y.Z`
   - `latest`
   - `main`
   - `git-<sha>`

### Image abrufen

```bash
docker pull ghcr.io/<owner>/<repo>:v0.0.0
docker pull ghcr.io/<owner>/<repo>:latest
```

### Lokale Version setzen

```bash
APP_VERSION=v0.0.0 docker compose up -d
```

## Eigene Regeln erstellen

Ueber das Dashboard oder die API koennen benutzerdefinierte Regeln hinzugefuegt werden:

```bash
curl -X POST http://localhost:8443/api/rules \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Meine Regel",
    "pattern": "(?i)(boeses_wort|anderes_muster)",
    "target": "all",
    "action": "block",
    "enabled": true,
    "description": "Blockiert bekannte schaedliche Muster"
  }'
```

### Regel-Ziele

- `all` — Prueft URI, Body, Header und Parameter
- `uri` — Nur den Request-Pfad und Query-String
- `body` — Nur den Request-Body
- `header` — Nur die HTTP-Header
- `param` — Nur die Query-Parameter

### Regel-Aktionen

- `block` — Erhoeht den Bedrohungs-Score um `score_weight` (Standard 10). Block, wenn die Summe die Schwelle `WAF_SCORE_THRESHOLD` erreicht oder uebersteigt.
- `allow` — Anfrage explizit erlauben (Whitelist, wird vor allen anderen Regeln geprueft)
- `sanitize` — Entfernt Treffer des Regex im gewaehlten Ziel (Parameter, Body, URI, Header, all), ohne sofort zu blockieren

## Lizenz

Siehe [LICENSE](LICENSE).

# Mitwirken an FlamingNPM

Danke, dass du das Projekt verbessern moechtest. Diese Seite beschreibt, wie du lokal entwickelst, Issues einreichst und Pull Requests erstellst — aehnlich wie bei groesseren Open-Source-Projekten (klare Erwartungen, CI, Reviews).

## Verhaltenskodex

Bitte lies den [Code of Conduct](CODE_OF_CONDUCT.md). Mit deiner Teilnahme stimmst du zu, ihn einzuhalten.

## Wo du helfen kannst

- **Bugs:** reproduzierbare Fehler mit Logs und Version melden (Issue-Vorlage „Bug Report“).
- **Ideen:** Feature-Wuensche mit Nutzen und Kontext (Vorlage „Feature Request“).
- **Code:** kleine, fokussierte Pull Requests von einem Feature-Branch gegen `develop`.
- **Dokumentation:** Rechtschreibung in diesem Projekt: **Schweizer Hochdeutsch** (`ss`, kein `ß`).

## Entwicklungsumgebung

### Voraussetzungen

- **Docker** (empfohlen fuer vollen Build) *oder*
- **Go 1.22+** mit **CGO** und **SQLite-Dev-Paket** (z.B. `libsqlite3-dev`) sowie **Node.js 20+** fuer das Dashboard.

### Schnellstart mit Docker (wie in CI)

```bash
git clone https://github.com/scruzzimattia-blip/FlamingNPM.git
cd FlamingNPM
docker compose build
docker compose up -d
```

Dashboard: `http://localhost:8443`, Proxy: `http://localhost:8080`.

### Lokal ohne Compose-Stack

```bash
# Backend (aus Repo-Root)
export CGO_ENABLED=1
go test ./...
go run ./cmd/waf

# Frontend (anderes Terminal)
cd web/frontend
npm install
npm run dev # Vite-Dev-Server, API ggf. per vite.config.js proxy auf :8443
```

### Qualitaetskriterien vor dem PR

- `gofmt` auf alle `.go`-Dateien (wird in CI geprueft).
- `go vet ./...` und `go test ./...` mit `CGO_ENABLED=1`.
- `npm run build` im Ordner `web/frontend`.
- Keine unnoetigen Massenaenderungen (kein „nebenbei alles formatieren“).

## Branch- und Release-Modell

| Branch | Zweck |
|--------|--------|
| `develop` | Integration neuer Features und Fixes; **hierhin** richten sich normale Pull Requests. |
| `main` | Stabiler Stand; Merge nach Review/CI; **Release** (Docker-Image, GitHub Release) laeuft nur bei Push auf `main`. |

**Ablauf fuer externe Beitraege:**

1. Fork erstellen, Feature-Branch von `develop` (`feature/kurze-beschreibung`).
2. Aenderungen committen (sinnvolle, getrennte Commits willkommen).
3. Pull Request **gegen `develop`** oeffnen (nicht gegen `main`, ausser Maintainer vereinbart etwas anderes).
4. CI muss gruen sein; auf Review-Feedback eingehen.

## Commit-Nachrichten

Kurz und im Imperativ, z.B.:

- `fix(proxy): Pfad-Prefix an Segmentgrenze pruefen`
- `feat(api): Endpunkt fuer Metadaten`
- `docs: CONTRIBUTING ergaenzt`

## Sicherheit

Bitte **keine** Sicherheitsluecken oeffentlich im Issue-Tracker posten. Siehe [SECURITY.md](SECURITY.md).

## GitHub fuer Maintainer

Schritt-fuer-Schritt fuer Repository-Einstellungen (Branch-Schutz, CI, Berechtigungen): [docs/GITHUB_SETUP.md](docs/GITHUB_SETUP.md).

# GitHub-Einstellungen fuer professionelles Community-Projekt

Diese Checkliste richtet sich an **Repository-Owner** (du). Ziel: externe
Mitwirkende koennen strukturiert helfen — vergleichbar mit Projekten wie Sonarr
(Issues, CI, Branch-Schutz, klare Erwartungen).

## 1. Allgemein (Settings → General)

- **Features**
  - **Issues:** ein (Pflicht fuer Bugreports/Features).
  - **Discussions:** optional, sinnvoll fuer Fragen ohne Issue (Support, Ideen).
  - **Projects:** optional fuer Roadmaps.
- **Pull Requests**
  - **Allow merge commits** oder **Squash** nach Wunsch; **Allow auto-merge** optional nach CI-Grün.
- **Wiki:** meist aus, wenn alles in `docs/` und README liegt.

## 2. Berechtigungen und Zusammenarbeit

- **Settings → Collaborators and teams**
  - Vertrauenspersonen als **Collaborator** einladen oder Team aus deiner Org hinzufuegen.
- **Forks:** bei oeffentlichem Repo koennen Fremde forken und PRs gegen dein Repo senden — keine Extra-Einstellung noetig.

## 3. Branch-Schutz fuer `main` (empfohlen)

**Settings → Branches → Add branch protection rule** → Branch-Name: `main`.

Empfohlene Optionen:

| Einstellung | Empfehlung |
|-------------|------------|
| **Require a pull request before merging** | Ja (mindestens 1 Genehmigung, wenn du nicht alleine bist). |
| **Require status checks to pass** | Ja — exakte Job-Namen aus `.github/workflows/ci.yml`: **`Go (fmt, vet, test)`** und **`Frontend (Build)`**. Optional zusaetzlich den Job **„Go-Tests vor Release“** aus `release.yml`, wenn du direkte Pushes auf `main` erlaubst. |
| **Require branches to be up to date** | Ja, reduziert Integrationsfehler. |
| **Do not allow bypassing** | Nur fuer Admins ausnahmsweise erlauben. |
| **Require linear history** | Optional (Geschmackssache). |

**Hinweis:** Externe Beitraege laufen typischerweise gegen **`develop`**.
Du kannst fuer `develop` **leichtere** Regeln setzen (nur CI, kein Review-Pflicht)
oder gleiche Regeln — wie du das Team fuehren willst.

## 4. Actions und Packages

- **Settings → Actions → General**
  - **Allow all actions** oder **Allow local + select** nach deiner Sicherheitsrichtlinie.
  - **Read and write permissions** fuer `GITHUB_TOKEN` nur wenn Workflows Releases/Tags schreiben — bei uns braucht `release.yml` Schreibzugriff (bereits in der Workflow-Datei mit `permissions` gesetzt).
- **Packages (ghcr.io):** bei Bedarf Sichtbarkeit des Pakets pruefen (**Public** fuer einfaches `docker pull`).

## 5. Security

- **Settings → Security → Code security and analysis**
  - **Dependency graph** / **Dependabot alerts** aktivieren (Go- und npm-Abhaengigkeiten).
  - **Private vulnerability reporting** aktivieren (siehe [SECURITY.md](../SECURITY.md)).
- Optional: **Dependabot version updates** (`.github/dependabot.yml`) spaeter ergaenzen.

## 6. Community-Profil

Unter **Insights → Community** zeigt GitHub eine Checkliste:

- Beschreibung, Website, Topics (z.B. `go`, `waf`, `reverse-proxy`, `docker`, `security`)
- README, Code of Conduct, Contributing, Security policy, Issue templates

Mit den Dateien in diesem Repo sollten die Punkte nach dem naechsten Push groesstenteils erfuellt sein.

## 7. Labels und Priorisierung (optional)

**Issues → Labels:** z.B. `bug`, `enhancement`, `good first issue`, `help wanted`.
Die Issue-Vorlagen setzen bereits passende Labels, sobald sie existieren.

## 8. Ablauf fuer Freiwillige (kurz)

1. Fork → Branch von `develop` → Aenderungen → PR gegen **`develop`**.
2. CI muss gruen sein.
3. Maintainer mergen nach `develop`; Release nach **`main`** loest Build und Docker aus.

## 9. Diskussionen und Support

- Wenn du **Discussions** aktivierst: Kurzes **README**- oder **CONTRIBUTING**-Update mit Link auf „Q&A“.
- Alternativ: Hinweis im Issue-Template, wo Fragen hingehoren.

---

Wenn du willst, kannst du diese Datei als interne Notiz lassen oder Teile in die
README verlinken — fuer Mitwirkende reicht der Link in [CONTRIBUTING.md](../CONTRIBUTING.md).

# wasaping — note per Claude Code

Scanner IP multipiattaforma in stile LanScan. Go 1.22+, **solo libreria standard** (nessuna dipendenza: tenerlo così se possibile).
Il proprietario parla italiano: messaggi all'utente, README e testi dell'interfaccia in italiano; commenti nel codice in inglese.

## Comandi
- `go test ./...` · `go vet ./...` (anche con `GOOS=windows` e `GOOS=darwin`)
- `go run . app`: finestra (su Mac nativa, serve cgo) · `go run . web`: browser · `go run .`: terminale
- `make oui`: aggiorna il database produttori incorporato · `make release`: tutti gli eseguibili + `dist/wasaping-macos.zip` · `make macapp` (solo su macOS) · `make icon`
- Go sul Mac del proprietario: Homebrew, `/opt/homebrew/bin/go` (aggiungerlo al PATH se manca)

## Architettura
- `scanner/`: motore. `Scan()` lavora in 3 fasi: (1) sonde TCP in parallelo, dove anche ECONNREFUSED vuol dire host vivo, con il ping di sistema come ripiego (RTT letto dall'output del ping); (2) lettura della cache ARP del sistema operativo (`/proc/net/arp` su Linux, `arp -a` altrove); (3) arricchimento: DNS inverso, poi mDNS + NetBIOS (`names.go`, tutti gli host insieme), produttore dal database IEEE incorporato (`scanner/oui.txt.gz`, ~380 KB, rigenerato con `make oui` → `scripts/ouigen`; una copia scaricata con update-oui ha la precedenza), porte.
- macOS 27: la tabella ARP arriva **vuota** ai processi non firmati da Apple/Developer ID (verificato: zsh/perl → `arp -an` dà tutto, wasaping/Python di Homebrew → niente; `02:00:00:00:00:00` è il segnaposto). `MACsHidden()` lo rileva e l'interfaccia lo spiega. Rimedio (`arphelper_darwin.go`): un demone launchd `eu.wasabi.wasaping.arp`, installato una volta con `osascript … with administrator privileges` (contenuti passati in base64 dentro il comando), in `/Library/LaunchDaemons` + `/Library/PrivilegedHelperTools`. KeepAlive/PathState su `/private/var/tmp/wasaping.active`: wasaping lo tocca ogni 10 s, lo script (solo binari Apple) copia `arp -an` in `/var/run/wasaping/arp` ogni secondo e si ferma se il file ha più di 30 s. Da spento: nessun processo. `wasaping helper install|uninstall|status`; nell'interfaccia link in fondo. Se si cambia lo script, `helperInstalled()` vede la differenza e l'app lo reinstalla (nuova password). Verificato funzionante su macOS 27. LanScan invece ha l'entitlement `com.apple.developer.networking.topology-observation` (serve account Apple Developer).
- `devices.go`: archivio dei dispositivi (`devices.json` in UserConfigDir), chiave = MAC oppure `ip:<IP>`; `Begin/View/Finish` segnano nuovi (non alla prima scansione in assoluto) e spariti nel range scansionato; nomi e note via `/api/device`. `scanner.GuessKind` deduce il tipo da gateway, porte, servizi DNS-SD, nome e produttore.
- `main.go`: riga di comando. Sottocomandi: scan (default), web, app, interfaces, update-oui, version.
- `web.go` + `web/index.html`: server su 127.0.0.1 con porta casuale, risultati in streaming via SSE (`/api/scan`). Blocca DNS rebinding (header Host) e richieste da altri siti (Sec-Fetch-Site/Origin). `/api/save` scrive le esportazioni in Download (le finestre app non scaricano blob).
- Modalità `app`, in ordine: finestra nativa Cocoa+WKWebView (`native_darwin.go`, build tag `darwin && cgo`, deve girare sul main thread: `init()` fa LockOSThread) → finestra `--app=` di Edge/Chrome con profilo proprio (`window.go`, si chiude quando il processo esce) → browser predefinito con heartbeat (`/api/ping`) e pulsante Esci. La pagina sa quale da `/api/info` → `ui`.
- Doppio clic: su Windows `launch_windows.go` (GetConsoleProcessList == 1); su macOS `launch_darwin.go` (eseguibile dentro `.app/Contents/MacOS/`). `wasaping.app` contiene un binario universale (lipo arm64+x86_64, cgo, macOS ≥ 13) firmato ad hoc; la release della CI quindi gira su macos-latest.

## Brand
- Nome sempre **minuscolo**: `wasaping`. Slogan: "a spicy little IP scanner".
- Verde **#81d742** (da wasabi.eu, il sito del proprietario) su quasi-nero **#171717**. Font Open Sans / Assistant.
- Nel tema chiaro, il testo verde usa `#3f7a17` per il contrasto; il testo sopra pulsanti verdi è scuro, non bianco.

## Stato e prossimi passi
- v0.1 pubblicata. Provato su Mac reale (macOS 27, arm64): finestra nativa, scansione, nomi mDNS/NetBIOS, esportazione. Windows ancora da provare su hardware reale (finestra Edge, ARP, ping).
- Modulo: `github.com/wasabifilm/wasaping`. Repository pubblico: https://github.com/wasabifilm/wasaping
- Nuova release: `git tag vX.Y.Z && git push --tags` fa partire la CI, che crea la release.
- Idee: confronto tra scansioni, nomi personalizzati, icona Windows (.syso), firma/notarizzazione Apple (anche per i MAC), WebView2 su Windows.

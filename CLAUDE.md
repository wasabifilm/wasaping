# wasaping — note per Claude Code

Scanner IP multipiattaforma in stile LanScan. Go 1.22+, **solo libreria standard** (nessuna dipendenza: tenerlo così se possibile).
Il proprietario parla italiano: messaggi all'utente, README e testi dell'interfaccia in italiano; commenti nel codice in inglese.

## Comandi
- `go test ./...` · `go vet ./...` anche con `GOOS=windows`, `GOOS=darwin` e **`GOOS=linux GOARCH=amd64`** (il Mac è arm64: alcune costanti di `syscall`, es. SO_REUSEPORT, esistono solo su certe architetture)
- `go run . app`: finestra (su Mac nativa, serve cgo) · `go run . web`: browser · `go run .`: terminale
- `make oui`: aggiorna il database produttori incorporato · `make release`: tutti gli eseguibili + `dist/wasaping-macos.zip` · `make macapp` (solo su macOS) · `make icon`
- Go sul Mac del proprietario: Homebrew, `/opt/homebrew/bin/go` (aggiungerlo al PATH se manca)

## Architettura
- `scanner/`: motore. `Scan()` lavora in 3 fasi: (1) sonde TCP in parallelo, dove anche ECONNREFUSED vuol dire host vivo, con il ping di sistema come ripiego (RTT letto dall'output del ping); (2) lettura della cache ARP del sistema operativo (`/proc/net/arp` su Linux, `arp -a` altrove); (3) arricchimento: DNS inverso, poi mDNS + NetBIOS (`names.go`, tutti gli host insieme), produttore dal database IEEE incorporato (`scanner/oui.txt.gz`, ~380 KB, rigenerato con `make oui` → `scripts/ouigen`; una copia scaricata con update-oui ha la precedenza), porte.
- macOS 27: la tabella ARP arriva **vuota** ai processi non firmati da Apple/Developer ID (verificato: zsh/perl → `arp -an` dà tutto, wasaping/Python di Homebrew → niente; `02:00:00:00:00:00` è il segnaposto). `MACsHidden()` lo rileva e l'interfaccia lo spiega. Rimedio (`arphelper_darwin.go`): un demone launchd `eu.wasabi.wasaping.arp`, installato una volta con `osascript … with administrator privileges` (contenuti passati in base64 dentro il comando), in `/Library/LaunchDaemons` + `/Library/PrivilegedHelperTools`. KeepAlive/PathState su `/private/var/tmp/wasaping.active`: wasaping lo tocca ogni 10 s, lo script (solo binari Apple) copia `arp -an` in `/var/run/wasaping/arp` ogni secondo e si ferma se il file ha più di 30 s. Da spento: nessun processo. `wasaping helper install|uninstall|status`; nell'interfaccia link in fondo. Se si cambia lo script, `helperInstalled()` vede la differenza e l'app lo reinstalla (nuova password). Verificato funzionante su macOS 27. LanScan invece ha l'entitlement `com.apple.developer.networking.topology-observation` (serve account Apple Developer).
- `devices.json` in UserConfigDir, chiave dispositivo = MAC oppure `ip:<IP>`; `Begin/View/Finish` segnano nuovi (non alla prima scansione su quella rete) e spariti (sempre i critici). `scanner.GuessKind` deduce il tipo: prima l'attrezzatura show, poi modello, porte, servizi, nome e produttore.
- Show (il proprietario lavora in spettacoli e concerti): `scanner/show.go` fa ArtPoll (porta 6454 condivisa con `listenShared`, SO_REUSEADDR/REUSEPORT, perché grandMA onPC e sACNView la tengono aperta), ascolta sACN (gruppi 239.255.0.1–32 + discovery 239.255.250.214) e PTP (224.0.1.129:319/320) in parallelo alla fase 1, e interroga PJLink (4352). `names.go` legge da DNS-SD i nomi di Dante, NDI, Blackmagic, QLab ecc. `issues.go` dà protocolli, avvisi per dispositivo e problemi di rete. `check.go` è il controllo rapido del Monitor (TCP + ping + ArtPoll).
- `devices.go`: archivio per **rete** (= locale): id `gw:<MAC router>` oppure `net:<sottorete>`; per dispositivo nome, ruolo, note, critico, storico (`seen`); il formato 0.2 (`devices`) viene spostato nella prima rete vista.
- Per provare senza attrezzatura: un simulatore Art-Net/sACN è facile da scrivere (ArtPollReply in broadcast su 6454, pacchetti E1.31 su 239.255.0.x:5568); i test di `scanner/show_test.go` coprono i parser.
- `main.go`: riga di comando. Sottocomandi: scan (default), web, app, interfaces, update-oui, version.
- `web.go` + `web/index.html`: server su 127.0.0.1 con porta casuale, risultati in streaming via SSE (`/api/scan`). Blocca DNS rebinding (header Host) e richieste da altri siti (Sec-Fetch-Site/Origin). `/api/save` scrive le esportazioni in Download (le finestre app non scaricano blob).
- `background.go`: `runCollect` è l'unica via per scansionare (pagina e background): registra nell'archivio e tiene `lastResult` (`/api/last`). `backgroundLoop` ripete ogni `repeat_minutes` (settings.json, `/api/bg`) e notifica nuovi e critici spariti; la pagina fa polling di `/api/last`.
- Password dell'aiuto ARP: **mai all'avvio**, solo dal pulsante «Mostra i MAC» (`/api/helper?do=install`); all'avvio `setupARP(false)` lo attiva se già installato. Apri al login: `login_darwin.go` scrive `~/Library/LaunchAgents/eu.wasabi.wasaping.login.plist` (`open -g -a wasaping.app --args app -hidden`).
- Con `-addr` non di loopback gli endpoint in `localOnly` (save, helper, quit, notify, device, network, bg, login, update-oui) rispondono solo da 127.0.0.1. `devices.json` e `settings.json` sono 0600.
- macOS: `native_darwin.go` ha anche l'NSStatusItem (pallino verde/rosso, menu con ultimo risultato via `setStatus`); chiudere la finestra la nasconde (activation policy Accessory), ⌘Q esce. Menu Scansione: ⌘R/⌘F/⇧⌘M chiamano `window.wasapingScan/Find/Monitor` nella pagina.
- `tools.go`: `/api/pingtest` (SSE, 20 ping, perdite/jitter) e `/api/trace` (traceroute/tracert/tracepath); il bersaglio passa da `toolTarget` (niente opzioni).
- `scanner/upnp.go`: SSDP M-SEARCH + XML di descrizione (solo dall'IP che risponde), titoli delle pagine web (client che accetta certificati self-signed, redirect solo sullo stesso host); scarta MiniDLNA e titoli vuoti.
- Icona Windows: `make winicon` → `scripts/winres` scrive `rsrc_windows_{amd64,arm64}.syso` (COFF .rsrc con RT_ICON PNG + RT_GROUP_ICON) dal file `assets/icon.png`; Go li collega da solo.
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
- Idee: firma/notarizzazione Apple (anche per i MAC), WebView2 su Windows, avvio al login, IPv6.

# wasaping — note per Claude Code

Scanner IP multipiattaforma in stile LanScan. Go 1.22+, **solo libreria standard** (nessuna dipendenza: tenerlo così se possibile).
Il proprietario parla italiano: messaggi all'utente, README e testi dell'interfaccia in italiano; commenti nel codice in inglese.

## Comandi
- `go test ./...` · `go vet ./...` (anche con `GOOS=windows` e `GOOS=darwin`)
- `go run . web`: interfaccia nel browser · `go run .`: scansione da terminale
- `make release`: tutti gli eseguibili + `dist/wasaping-macos.zip` · `make macapp` · `make icon`

## Architettura
- `scanner/`: motore. `Scan()` lavora in 3 fasi: (1) sonde TCP in parallelo, dove anche ECONNREFUSED vuol dire host vivo, con il ping di sistema come ripiego; (2) lettura della cache ARP del sistema operativo (`/proc/net/arp` su Linux, `arp -a` altrove); (3) arricchimento: DNS inverso, produttore dal file IEEE OUI in cache, porte.
- `main.go`: riga di comando. Sottocomandi: scan (default), web, app, interfaces, update-oui, version.
- `web.go` + `web/index.html`: server su 127.0.0.1 con porta casuale, risultati in streaming via SSE (`/api/scan`). Controlla l'header Host per bloccare il DNS rebinding. In modalità `app` riceve un heartbeat dalla pagina (`/api/ping`) e si chiude con `/api/quit` o quando la pagina sparisce.
- Doppio clic: su Windows `launch_windows.go` (GetConsoleProcessList == 1 → modalità app); su macOS c'è il bundle `wasaping.app` (scripts/build_macapp.sh), il cui launcher esegue `wasaping-$(uname -m) app`.

## Brand
- Nome sempre **minuscolo**: `wasaping`. Slogan: "a spicy little IP scanner".
- Verde **#81d742** (da wasabi.eu, il sito del proprietario) su quasi-nero **#171717**. Font Open Sans / Assistant.
- Nel tema chiaro, il testo verde usa `#3f7a17` per il contrasto; il testo sopra pulsanti verdi è scuro, non bianco.

## Stato e prossimi passi
- v0.1 funzionante. Non ancora provato su hardware Mac/Windows reale: verificare per prima cosa (doppio clic, ARP, ping).
- Il modulo si chiama ancora `github.com/USERNAME/wasaping`: sostituire USERNAME con l'utente GitHub reale (go.mod, import in main.go e web.go, commento in main.go).
- Pubblicazione: `gh repo create wasaping --public --source . --push`, poi `git tag v0.1.0 && git push --tags` per far partire la CI, che crea la release.
- Idee: mDNS/NetBIOS per i nomi, confronto tra scansioni, nomi personalizzati, icona Windows (.syso), notarizzazione Apple.

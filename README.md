# wasaping 🟢

**a spicy little IP scanner**: uno scanner di rete per **macOS, Windows e Linux**, pensato anche per chi lavora negli **spettacoli e nei concerti**.
Un solo programma, niente da installare, niente permessi di amministratore.

![wasaping su una rete show di esempio](docs/screenshot.png)

<sub>Rete di esempio: console grandMA3, nodi Art-Net, Dante, ATEM, NDI, proiettore PJLink e un nodo fuori sottorete.</sub>

## Download e primo avvio

Scarica il file per il tuo sistema dalla pagina [Releases](../../releases).

| Sistema | File | Come si apre |
|---|---|---|
| macOS 13 o successivo (Apple Silicon e Intel) | `wasaping-macos.zip` | estrai, sposta **wasaping.app** in Applicazioni, doppio clic |
| Windows 10 e 11 | `wasaping-windows-amd64.exe` (o `-arm64`) | doppio clic |
| Linux | `wasaping-linux-amd64` (o `-arm64`) | `./wasaping-linux-amd64 app` |

Con il doppio clic wasaping si apre in una **finestra sua**, senza passare dal browser:

- **macOS:** finestra nativa con menu e **icona nella barra dei menu**. Se chiudi la finestra, wasaping resta attivo lì: riaprilo dall'icona, e chiudilo del tutto con ⌘Q.
- **Windows:** una finestra di Microsoft Edge in modalità app, senza barra degli indirizzi. Edge c'è già in ogni Windows 10 e 11.
- **Linux:** una finestra di Chrome/Chromium in modalità app, se installato; altrimenti il browser predefinito.

> **macOS, la prima volta:** l'app non è firmata da Apple, quindi macOS la blocca. Apri *Impostazioni di Sistema → Privacy e sicurezza* e premi **Apri comunque**, oppure dal Terminale: `xattr -cr /Applications/wasaping.app`
>
> **macOS, indirizzi MAC:** le versioni recenti di macOS nascondono gli indirizzi MAC ai programmi non firmati da uno sviluppatore Apple registrato. wasaping te lo dice e propone **«Mostra i MAC»**: installa un piccolo aiuto di sistema che legge la tabella ARP. Serve la password di amministratore **una sola volta**, nella finestra standard di macOS (wasaping non la vede). L'aiuto lavora **solo mentre wasaping è aperto**: a wasaping chiuso non usa né CPU né memoria. Lo togli con *Disinstalla* in fondo alla finestra o con `wasaping helper uninstall`. Su Windows e Linux non serve.
>
> **Windows, la prima volta:** se compare SmartScreen, premi *Ulteriori informazioni → Esegui comunque*.

## Per chi lavora negli spettacoli

- **Riconosce l'attrezzatura e la chiama per nome:**
  - **Art-Net**: nodi, dimmer e console, con nome, tipo e universi;
  - **sACN**: sorgenti, universi e priorità;
  - **Dante**, **NDI** (con le sorgenti), **Blackmagic** (ATEM, HyperDeck, Videohub);
  - **QLab**, **OSC**, **grandMA**, **Yamaha**, **Allen & Heath**, **Shure**;
  - **proiettori via PJLink**: modello, ore lampada, stato ed errori.
- **Trova il master del clock PTP**, importante per Dante e AES67.
- **Segnala i problemi che rovinano lo show:**
  - dispositivi fuori sottorete, per esempio un nodo a `2.x` mentre sei a `10.x`;
  - conflitti di IP;
  - più master PTP;
  - lo stesso universo sACN inviato da più sorgenti;
  - rete senza router.
- **Monitor:** segni i dispositivi **critici** (console, switch principale, stagebox, media server…). wasaping li controlla ogni 3–30 secondi su una schermata a riquadri verde/rosso, e suona e avvisa appena uno smette di rispondere.
- **Un elenco per ogni locale:** ogni rete ha i suoi dispositivi, con nome, ruolo e note. La riconosce dal router, o dalla sottorete sulle reti show senza router. Le dai un nome ("Arena Verona") e la volta dopo vedi cosa c'è di nuovo e cosa manca.
- **Elenco IP da stampare** o salvare in PDF per la crew e il rider: dispositivo, ruolo, IP, MAC, modello, protocolli, universi e note.
- **Qualità del collegamento:** 20 ping con perdite, tempi e jitter. Serve per capire se un cavo, uno switch o il Wi-Fi fanno i capricci. C'è anche il traceroute.

## Cosa fa, in generale

- Trova da solo la rete locale e la scansiona in circa 4 secondi.
- Per ogni dispositivo mostra **IP, nome, MAC, produttore, modello e porte aperte**, e ne riconosce il **tipo** con un'icona (router, computer, telefono, TV, stampante, NAS…).
- **Nomi anche senza DNS:**
  - mDNS/Bonjour: Mac, iPhone, Android, stampanti, Dante, NDI;
  - NetBIOS: PC Windows, NAS;
  - UPnP: TV, router, NAS. Per esempio "[TV] Samsung Q90 Series", modello QE55Q95T.
- **Produttori già inclusi:** il database IEEE (oltre 40.000 marche) è dentro il programma e funziona senza internet. Riconosce i **MAC privati/casuali** di iPhone e Android.
- **Si ricorda dei dispositivi:** segna quelli **nuovi** e quelli che **non rispondono più**, con la presenza nelle ultime scansioni.
- **Nomi, ruoli e note personalizzati**, salvati sul tuo computer.
- **Ripetizione automatica** ogni 5, 15 o 60 minuti, anche a finestra chiusa, con una notifica quando arriva un dispositivo nuovo o un dispositivo critico sparisce. Su macOS può **aprirsi al login**, solo nella barra dei menu.
- **Porte cliccabili:**
  - le pagine web si aprono nel browser;
  - SSH nel Terminale;
  - VNC in Condivisione schermo;
  - SMB/AFP nel Finder.

  Col **tasto destro** copi indirizzo e porta.
- **Wake-on-LAN**, controllo di tutte le porte 1–1024, avvisi sulle porte rischiose (Telnet, FTP sul router…).
- Scorciatoie **⌘R** scansiona, **⌘F** cerca, **⇧⌘M** Monitor (Ctrl su Windows e Linux). Tema automatico, chiaro o scuro.
- Esporta in **CSV**, **JSON** o elenco stampabile.

### Come trova i dispositivi senza permessi di amministratore

1. **Sonde TCP** in parallelo su porte comuni. Anche una connessione *rifiutata* dimostra che il dispositivo è acceso.
2. Il **ping di sistema** per chi non risponde via TCP.
3. La **tabella ARP** del sistema operativo: le sonde precedenti la riempiono, così si vedono anche i dispositivi con tutte le porte chiuse.
4. I **protocolli show**: un ArtPoll Art-Net e l'ascolto di sACN e PTP. Così si vedono anche i nodi su un'altra sottorete.

## Riga di comando

```sh
wasaping                     # scansiona la tua rete dal terminale
wasaping 192.168.1.0/24      # una rete precisa
wasaping 10.0.0.1-50 -p none # un intervallo, senza controllare le porte
wasaping -o csv > rete.csv   # esporta in CSV (oppure -o json)
wasaping app                 # apre la finestra
wasaping web                 # apre l'interfaccia nel browser (Ctrl+C per uscire)
wasaping interfaces          # elenca le interfacce di rete
wasaping update-oui          # aggiorna il database dei produttori (è già incluso)
wasaping helper install      # macOS: aiuto per leggere i MAC (anche: uninstall, status)
```

| Opzione | Default | Descrizione |
|---|---|---|
| `-i` | automatica | interfaccia da usare (`en0`, `eth0`, `Wi-Fi`…) |
| `-p` | `common` | porte da controllare: `common`, `none`, `22,80,443`, `1-1024` |
| `-timeout` | `800ms` | timeout per singolo tentativo |
| `-workers` | `256` | indirizzi sondati in parallelo |
| `-no-ping` | | non usa il ping di sistema |
| `-no-dns` | | non cerca i nomi |
| `-o` | `table` | `table`, `json`, `csv` |

`wasaping web -addr 0.0.0.0:8080` rende l'interfaccia raggiungibile dagli altri computer della rete, **senza password**. Da lì si può scansionare, ma non salvare file né cambiare niente su questo computer.

I dispositivi conosciuti sono in `devices.json`, nella cartella delle impostazioni dell'utente:
- Mac: `~/Library/Application Support/wasaping/`
- Windows: `%AppData%\wasaping\`
- Linux: `~/.config/wasaping/`

## Compilare dai sorgenti

Ti serve [Go](https://go.dev/dl/) 1.22 o successivo. Non ci sono dipendenze esterne.
La finestra nativa di macOS usa Cocoa e WebKit tramite cgo, quindi `wasaping.app` si compila solo su un Mac (con gli strumenti a riga di comando di Xcode). Gli altri eseguibili si compilano da qualsiasi sistema.

```sh
go build -o wasaping .
go test ./...
make release        # tutti gli eseguibili + wasaping-macos.zip in dist/
make macapp         # solo dist/wasaping.app (universale, solo su macOS)
make icon           # rigenera le icone in assets/ (serve Python + Pillow)
make winicon        # rigenera l'icona dell'eseguibile Windows
make oui            # aggiorna il database dei produttori incluso
```

## Struttura

```
main.go              riga di comando
web.go               server locale e API per l'interfaccia
background.go        scansioni (anche in background) e impostazioni
devices.go           archivio per rete/locale: nomi, ruoli, note, critici, storico
tools.go             ping con qualità del collegamento, traceroute
window.go            finestra di Edge/Chrome, salvataggio delle esportazioni
native_darwin.go     finestra nativa e barra dei menu macOS (Cocoa + WebKit, cgo)
arphelper_darwin.go  aiuto di sistema per leggere la tabella ARP (macOS)
login_darwin.go      apertura al login (macOS)
web/index.html       interfaccia grafica (incorporata nell'eseguibile)
scanner/             motore: sonde, ARP, nomi (DNS, mDNS, NetBIOS, UPnP),
                     protocolli show (Art-Net, sACN, PTP, PJLink), tipi, avvisi
scripts/             build dell'app macOS, icone, database produttori
```

## Idee per le prossime versioni

- [ ] Firma e notarizzazione Apple (serve un account Apple Developer): toglie l'avviso di Gatekeeper e restituisce i MAC senza aiuto
- [ ] Finestra nativa anche su Windows (WebView2)
- [ ] IPv6

## Licenza

MIT

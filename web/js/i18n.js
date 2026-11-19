// English: translates the Italian interface after every change of the page.
// ---- English: the page is written in Italian; when the language is English
// a translator rewrites interface texts (exact strings and patterns) after
// every update. Device names, notes and other user data are left alone.
const LANG = (() => {
  try { const v = localStorage.getItem("wasaping.lang"); if (v === "it" || v === "en") return v; } catch {}
  return (navigator.language || "it").toLowerCase().startsWith("it") ? "it" : "en";
})();
const EN = new Map(Object.entries({
  "a spicy little IP scanner": "a spicy little IP scanner", "Tema: automatico": "Theme: automatic", "Tema: chiaro": "Theme: light", "Tema: scuro": "Theme: dark",
  "Interfaccia di rete": "Network interface", "Personalizzato…": "Custom…", "Porte comuni": "Common ports", "Nessuna porta (più veloce)": "No ports (faster)",
  "Porte personalizzate…": "Custom ports…", "Porte da controllare": "Ports to check", "22,80,443 o 1-1024": "22,80,443 or 1-1024", "Scansiona": "Scan", "Stop": "Stop",
  "Filtra per IP, nome, produttore…": "Filter by IP, name, maker…", "Ripeti: mai": "Repeat: never", "Ogni 5 min": "Every 5 min", "Ogni 15 min": "Every 15 min", "Ogni ora": "Every hour",
  "Ripeti la scansione da sola e avvisa se arriva un dispositivo nuovo": "Scan again by itself and notify when a new device shows up",
  "Monitor": "Monitor", "Tiene d'occhio i dispositivi critici e suona se uno sparisce": "Watches the critical devices and sounds an alarm if one disappears",
  "Stampa": "Print", "Elenco IP da stampare o salvare in PDF": "IP list to print or save as PDF", "Esci": "Quit", "Chiude wasaping": "Quits wasaping",
  "Pronto.": "Ready.", "Scansione interrotta.": "Scan stopped.", "Connessione interrotta: il target è valido?": "Connection lost: is the target valid?",
  "Scarica il database dei produttori (IEEE) per vedere chi ha fatto ogni dispositivo.": "Download the vendor database (IEEE) to see who made each device.", "Scarica": "Download",
  "Mostra i MAC": "Show MACs", "Non ora": "Not now", "macOS nasconde gli indirizzi MAC": "macOS hides MAC addresses",
  "ai programmi non firmati da uno sviluppatore Apple registrato, quindi MAC e produttori restano vuoti.\n  wasaping può installare un piccolo aiuto di sistema che li legge: serve la password di amministratore":
    "from apps not signed by a registered Apple developer, so MACs and makers stay empty.\n  wasaping can install a small system helper that reads them: it needs the administrator password",
  "una sola volta": "only once",
  ", la inserisci nella finestra di macOS (wasaping non la vede) e l'aiuto lavora solo mentre wasaping è aperto. Lo togli quando vuoi con «Disinstalla» in fondo alla pagina.":
    ", typed in the macOS window (wasaping never sees it), and the helper only works while wasaping is open. Remove it any time with “Uninstall” at the bottom of the page.",
  "Dispositivo": "Device", "IP": "IP", "MAC": "MAC", "Modello / produttore": "Model / maker", "Protocolli": "Protocols", "Porte aperte": "Open ports", "Risposta": "Response",
  "Premi": "Press", "per vedere chi c'è sulla tua rete.": "to see who is on your network.", "Cerco dispositivi…": "Looking for devices…", "Nessun risultato per il filtro.": "Nothing matches the filter.",
  "Nessun dispositivo trovato.": "No devices found.", "tu": "you", "nuovo": "new", "non risponde": "not responding", "critico": "critical", "rete nuova": "new network",
  "Nome personalizzato": "Custom name", "Ruolo": "Role", "Note": "Notes", "es. Console FOH": "e.g. FOH console", "es. luci palco SX, universi 1–4": "e.g. stage left lights, universes 1–4",
  "es. patch su switch 2 porta 14": "e.g. patched on switch 2 port 14", "Critico: tienilo d'occhio nel Monitor": "Critical: keep an eye on it in the Monitor", "Salva": "Save",
  "Sveglia (Wake-on-LAN)": "Wake (Wake-on-LAN)", "Accende il dispositivo, se la sua scheda di rete lo supporta": "Turns the device on, if its network card supports it",
  "Controlla porte 1–1024": "Check ports 1–1024", "Controlla le porte da 1 a 1024": "Checks ports 1 to 1024", "Dimentica": "Forget", "Toglie il dispositivo dall'elenco dei conosciuti": "Removes the device from the known ones",
  "Strumenti di rete": "Network tools", "Qualità del collegamento": "Link quality", "20 ping: perdite, tempi e stabilità del collegamento": "20 pings: loss, times and stability of the link",
  "Traceroute": "Traceroute", "Il percorso dei pacchetti fino al dispositivo": "The path of packets to the device", "IP o nome: anche un indirizzo internet, es. 8.8.8.8": "IP or name, internet addresses too, e.g. 8.8.8.8",
  "Produttore": "Maker", "Modello": "Model", "Nome del dispositivo": "Device name", "Nome in rete": "Network name", "Pagina web": "Web page", "Servizi": "Services",
  "nessuna tra quelle controllate": "none of those checked", "solo tabella ARP": "ARP table only", "Visto la prima volta": "First seen", "Visto l'ultima volta": "Last seen", "Presenza": "Presence",
  "ultime scansioni, la più recente a destra": "latest scans, most recent on the right", "Universi in uscita": "Output universes", "Universi in ingresso": "Input universes", "Stato del nodo": "Node status",
  "Firmware": "Firmware", "Sorgente sACN": "sACN source", "Universi sACN": "sACN universes", "Priorità sACN": "sACN priority", "Clock PTP": "PTP clock", "Sorgenti NDI": "NDI sources",
  "Stato proiettore": "Projector state", "Ore lampada": "Lamp hours", "Errori": "Errors", "protetto da password": "password protected", "(port-address)": "(port-address)", "(Dante)": "(Dante)",
  "Switch": "Switch", "Router": "Router", "Computer": "Computer", "Telefono": "Phone", "Tablet": "Tablet", "Stampante": "Printer", "TV / media": "TV / media", "Altoparlante": "Speaker",
  "NAS / archivio": "NAS / storage", "Videocamera": "Camera", "Domotica / IoT": "Home automation / IoT", "Console luci": "Lighting console", "Luci (nodo / proiettore)": "Lighting (node / fixture)",
  "Audio": "Audio", "Video": "Video", "Proiettore": "Projector", "Switch di rete": "Network switch", "Condivisione file": "File sharing", "Condivisione file (AFP)": "File sharing (AFP)",
  "Condivisione schermo": "Screen sharing", "Dispositivo Apple": "Apple device", "Sincronizzazione Apple": "Apple sync", "Scanner": "Scanner",
  "nodo": "node", "console": "console", "media server": "media server", "router": "router", "backup": "backup", "configuratore": "configurator", "visualizzatore": "visualiser",
  "spento": "off", "acceso": "on", "in raffreddamento": "cooling down", "in accensione": "warming up",
  "TUTTO OK": "ALL OK", "FERMO": "STOPPED", "NON RISPONDE": "NOT RESPONDING", "controllo…": "checking…", "Avvia": "Start", "Ferma": "Stop",
  "Allarme sonoro": "Sound alarm", "Torna all'elenco": "Back to the list", "Il monitor continua a controllare anche a finestra chiusa": "The monitor keeps checking with the window closed",
  "Ogni quanto controllare": "How often to check", "ogni 3 s": "every 3 s", "ogni 5 s": "every 5 s", "ogni 10 s": "every 10 s", "ogni 30 s": "every 30 s",
  "Registro": "Log", "Esporta": "Export", "Salva il registro in CSV": "Save the log as CSV", "Nessun evento.": "No events.", "Accesso da iPad o telefono": "Access from iPad or phone",
  "Mostra questo Monitor su un iPad o un telefono della stessa rete, protetto da un PIN.": "Show this Monitor on an iPad or a phone on the same network, protected by a PIN.",
  "Nuovo PIN": "New PIN", "Chiudi": "Close", "rinomina": "rename", "dai un nome a questo locale": "name this venue", "Piano IP": "IP plan", "IP liberi": "Free IPs",
  "Esporta locale": "Export venue", "Importa locale": "Import venue", "Universi DMX": "DMX universes", "Porte switch": "Switch ports", "Rete:": "Network:",
  "Importa CSV": "Import CSV", "Scarica un modello": "Download a template", "Crea il piano da questa scansione": "Make the plan from this scan", "Importa un altro CSV": "Import another CSV",
  "Usa nomi e ruoli del piano": "Use names and roles from the plan", "Togli il piano": "Remove the plan", "Stato": "Status", "Previsto": "Planned", "Trovato": "Found", "manca": "missing",
  "non previsto": "not planned", "IP doppio nel piano": "duplicate IP in the plan", "✓ ok": "✓ ok", "Fai una scansione per il confronto": "Scan to compare",
  "Dà ai dispositivi trovati il nome e il ruolo del piano, se non ne hanno già uno": "Gives the devices found the plan's name and role, if they have none yet",
  "Universo": "Universe", "Protocollo": "Protocol", "Sorgente": "Source", "Frame/s": "Frames/s", "Canali attivi": "Active channels", "In ascolto…": "Listening…", "Ascolto interrotto.": "Listening stopped.",
  "più sorgenti": "several sources", "Porta": "Port", "Descrizione porta": "Port description", "Cerca": "Search", "Community SNMP di lettura": "Read-only SNMP community",
  "Chiedo agli switch…": "Asking the switches…", "Fai prima una scansione.": "Scan first.", "Nessun evento": "No events", "Non controllare più": "Don't check again",
  "Inserisci la password nella finestra di macOS…": "Type the password in the macOS window…", "Rimuovo l'aiuto…": "Removing the helper…", "Disinstalla": "Uninstall",
  "Indirizzi MAC: aiuto di sistema attivo ·": "MAC addresses: system helper active ·", "Indirizzi MAC nascosti da macOS ·": "MAC addresses hidden by macOS ·", "Installa l'aiuto": "Install the helper",
  "(password una sola volta)": "(password only once)", "✓ Salvato.": "✓ Saved.", "Controllo 1024 porte…": "Checking 1024 ports…", "Copia URL": "Copy URL",
  "✗ Non risponde al ping.": "✗ Does not answer ping.", "✓ Collegamento stabile.": "✓ Stable link.", "Interrotto.": "Stopped.",
  "⚠ Collegamento instabile: pacchetti persi o tempi che variano molto (cavo, Wi-Fi, switch?).": "⚠ Unstable link: lost packets or very variable times (cable, Wi-Fi, switch?).",
  "✓ Pacchetto di risveglio inviato. Rifai la scansione tra qualche secondo.": "✓ Wake-up packet sent. Scan again in a few seconds.",
  "✓ Aiuto installato: rifai la scansione per vedere i MAC.": "✓ Helper installed: scan again to see the MACs.", "Ripetizione automatica spenta.": "Automatic repeat off.",
  "wasaping ripeterà la scansione da solo, anche a finestra chiusa.": "wasaping will scan again by itself, with the window closed too.", "Non controllerò più le nuove versioni.": "I won't check for new versions any more.",
  "Nessun dispositivo critico su questa rete.": "No critical devices on this network.", "Apri un dispositivo dall'elenco e spunta": "Open a device from the list and tick",
  "Critico": "Critical", ": console, switch principale, stagebox Dante, media server…": ": console, main switch, Dante stagebox, media server…",
  "Nessuno switch ha risposto. Controlla che SNMP sia attivo e la community giusta.": "No switch answered. Check that SNMP is on and the community is right.",
  "Nessun universo in arrivo. Sulla rete non passa DMX, oppure la console manda Art-Net diretto ai nodi (unicast) e non a questo computer.": "No universes arriving. There is no DMX on the network, or the console sends Art-Net straight to the nodes (unicast), not to this computer.",
  "Scrivi una rete (es. 10.0.0.0/24) o un intervallo nel campo in alto.": "Type a network (e.g. 10.0.0.0/24) or a range in the field at the top.",
  "Controlla comunque l'intervallo del DHCP, se sulla rete c'è un router.": "Still check the DHCP range, if there is a router on the network.", "Nessuno.": "None.",
  "Sullo switch deve essere attivo SNMP v2c con una community di lettura (spesso": "The switch needs SNMP v2c on, with a read community (often",
  "In ascolto su Art-Net (pacchetti in broadcast o diretti a questo computer) e sACN (universi 1–64). Clic su un universo per vedere i 512 canali.": "Listening to Art-Net (broadcast packets or sent to this computer) and sACN (universes 1–64). Click a universe to see its 512 channels.",
  "wasaping chiede agli switch gestiti (Luminex, Cisco, Netgear, Ubiquiti, MikroTik…) su quale porta si trova ogni dispositivo, via SNMP in sola lettura.": "wasaping asks the managed switches (Luminex, Cisco, Netgear, Ubiquiti, MikroTik…) which port each device is on, over read-only SNMP.",
  "Importa l'elenco dei dispositivi previsti (il patch o il piano IP preparato per lo show) e wasaping lo confronta con la rete: chi manca, chi ha l'IP sbagliato, chi non era previsto.": "Import the list of planned devices (the patch or IP plan made for the show) and wasaping compares it with the network: who is missing, who has the wrong IP, who was not planned.",
  "Un file CSV da Excel, Numbers o Fogli Google, con le colonne": "A CSV file from Excel, Numbers or Google Sheets, with the columns", "e, se vuoi,": "and, if you like,",
  ". Va bene sia la virgola sia il punto e virgola.": ". Both comma and semicolon work.", "Nome": "Name", "Da lì si guarda soltanto, non si cambia niente.": "From there you can only look, not change anything.",
  "Accesso da iPad/telefono non attivo:": "Access from iPad/phone not active:", "fai prima una scansione": "scan first", "fai prima una scansione di questa rete": "scan this network first",
  "dispositivo sconosciuto": "unknown device", "richiesta non valida": "invalid request", "nessuna riga con un indirizzo IP valido": "no rows with a valid IP address",
  "non è un file di locale di wasaping": "not a wasaping venue file", "file non valido": "invalid file", "nome file non valido": "invalid file name", "annullato": "cancelled",
  "Nessuna scansione ancora": "No scans yet", "Tempi di risposta": "Response times", "Confronta con l'ultima volta": "Compare with last time", "Rispetto alla ricerca precedente": "Compared with the previous search",
  "Nessun cambiamento rispetto alla ricerca precedente.": "No changes since the previous search.", "nuovo": "new", "non c'è più": "gone", "IP cambiato": "IP changed", "cambiato": "changed",
  "Nessuna differenza: la rete è come allora.": "No differences: the network is as it was.",
  "Serve almeno un'altra sessione su questa rete: wasaping tiene una fotografia della rete per ogni sessione di lavoro (scansioni a più di un'ora di distanza). La prossima volta che torni in questo locale potrai vedere cosa è cambiato.": "This needs at least one more session on this network: wasaping keeps a snapshot of the network for each working session (scans more than an hour apart). Next time you are back in this venue you can see what changed.", "Mappa della rete": "Network map", "Collegamenti tra switch": "Links between switches", "Tutti i dispositivi": "All devices", "automatico": "automatic", "Tipo": "Type", "Aiuto": "Help", "Salva diagnostica": "Save diagnostics", "Diagnostica": "Diagnostics", "Crea il file di diagnostica": "Create the diagnostics file",
  "Scansione e registrazione in corso…": "Scanning and recording…", "✓ Tipo corretto: grazie, finisce anche nella diagnostica.": "✓ Type corrected: thanks, it also goes into the diagnostics.",
  "Se wasaping l'ha riconosciuto male, correggilo: la correzione resta e finisce nella diagnostica": "If wasaping got it wrong, correct it: the correction is kept and goes into the diagnostics",
  "wasaping fa una scansione registrando le risposte dei dispositivi (Bonjour, UPnP, Art-Net, sACN, PTP, PJLink…) e salva tutto in un file zip nella cartella Download.": "wasaping scans while recording what devices answer (Bonjour, UPnP, Art-Net, sACN, PTP, PJLink…) and saves it all in a zip file in Downloads.",
  "Serve quando un apparecchio non viene riconosciuto o ha un nome sbagliato: manda il file a chi sviluppa wasaping. Non contiene i nomi, i ruoli e le note che hai scritto, né PIN, password o indirizzi degli allarmi.": "Use it when a device is not recognized or has a wrong name: send the file to wasaping's developer. It does not contain the names, roles and notes you wrote, nor PINs, passwords or alarm addresses.", "Identifica": "Identify", "Ferma": "Stop", "Ho capito, identifica": "Understood, identify", "Annulla": "Cancel", "Report di fine show": "End-of-show report", "Mostra": "Show", "Dalle": "From",
  "«Identifica» fa lampeggiare l'apparecchio (o lo muove, se è una testa mobile). Non cambia nessuna impostazione. Durante uno show, usalo solo se sei sicuro.": "“Identify” makes the fixture flash (or move, if it is a moving head). It changes no setting. During a show, use it only if you are sure.",
  "Riassunto di quello che è successo durante lo show, dal registro del Monitor: quante volte è caduto ogni dispositivo, per quanto tempo in tutto, il buco più lungo.": "What happened during the show, from the Monitor log: how many times each device dropped, for how long in total, the longest gap.",
  "Per dispositivo": "Per device", "Cadute": "Drops", "Tempo giù in tutto": "Total time down", "Il più lungo": "Longest", "Cronologia": "Timeline", "ancora giù": "still down", "Nessuna caduta": "No drops",
  "Nessun evento nel registro in questo periodo.": "No events in the log for this period.", "non risponde": "not responding", "di nuovo in linea": "back online", "nessun master del clock": "no clock master", "Controllo in corso: scansione, dispositivi critici, universi, piano IP…": "Checking: scan, critical devices, universes, IP plan…", "Dispositivi critici": "Critical devices", "Universi critici": "Critical universes",
  "Nessun dispositivo segnato come critico.": "No device marked as critical.", "Nessun universo segnato come critico (Show → Universi DMX).": "No universe marked as critical (Show → DMX universes).",
  "Nessun piano IP importato.": "No IP plan imported.", "Problemi di rete": "Network problems", "Nessuno.": "None.", "Avvisi sui dispositivi": "Device warnings", "NON PRONTO": "NOT READY",
  "PRONTO CON AVVISI": "READY WITH WARNINGS", "PRONTO": "READY", "Niente di bloccante, ma qualcosa da controllare.": "Nothing blocking, but something to check.", "Tutto a posto.": "All good.",
  "Stampa o salva il report": "Print or save the report", "Ricontrolla": "Check again", "NON ARRIVA": "NOT ARRIVING", "non arriva": "not arriving", "Il Monitor ora controlla questo universo.": "The Monitor now watches this universe.", "Universo tolto dal Monitor.": "Universe removed from the Monitor.",
  "Il Monitor avvisa se l'universo smette di arrivare": "The Monitor warns if the universe stops arriving", "Clock PTP: master": "PTP clock: master", "Cerca apparecchi": "Find fixtures", "Indirizzo": "Address", "Canali": "Channels", "Etichetta": "Label", "Modalità": "Mode", "UID": "UID", "Nodo": "Node",
  "wasaping chiede ai nodi Art-Net quali apparecchi RDM hanno collegati e a ognuno chi è: produttore, modello, etichetta, indirizzo DMX, canali e modalità. La ricerca non cambia niente sugli apparecchi; per cambiare indirizzo, modalità o etichetta c'è «Modifica», che chiede conferma.": "wasaping asks the Art-Net nodes which RDM fixtures they have connected, and each fixture who it is: maker, model, label, DMX address, channels and mode. The search changes nothing on the fixtures; to change address, mode or label there is “Edit”, which asks for confirmation.",
  "Modifica": "Edit", "Dispositivi": "Devices", "Progetto": "Project", "Sezioni": "Sections", "Fai prima una scansione.": "Scan first.", "Fai prima una scansione di questa rete.": "Scan this network first.",
  "Controlla tutto prima dello show e prepara un report verde/giallo/rosso.": "Checks everything before the show and prepares a green/yellow/red report.",
  "Tiene d'occhio dispositivi e universi critici, e suona se uno sparisce.": "Watches the critical devices and universes, and sounds if one disappears.",
  "Art-Net e sACN dal vivo: sorgenti, canali, patch, due console a confronto.": "Live Art-Net and sACN: sources, channels, patch, two consoles compared.",
  "Gli apparecchi dietro i nodi Art-Net: indirizzi, modalità, Identifica, Modifica.": "The fixtures behind the Art-Net nodes: addresses, modes, Identify, Edit.",
  "Dispositivi, clock e routing in sola lettura.": "Devices, clock and routing, read-only.", "ATEM, HyperDeck, Videohub, vMix, proiettori, NDI.": "ATEM, HyperDeck, Videohub, vMix, projectors, NDI.",
  "Cadute, tempi giù e tempi di risposta della serata.": "Drops, downtime and response times of the night.",
  "Il patch previsto, confrontato con la rete. Si importa o si scrive qui.": "The planned patch, compared with the network. Import it or write it here.",
  "Gli indirizzi liberi, per dare un IP fisso a un apparecchio nuovo.": "Free addresses, to give a new fixture a static IP.",
  "Switch gestiti (SNMP): porte, velocità e chi è collegato dove.": "Managed switches (SNMP): ports, speeds and who is connected where.",
  "Chi c'è di nuovo, chi manca, chi ha cambiato IP rispetto all'altra volta.": "Who is new, who is missing, who changed IP since last time.",
  "Tutto il lavoro su questo locale in un file .wasaping.": "All the work on this venue in a .wasaping file.",
  "Riapre un progetto, anche di un locale dove questo computer non è mai stato.": "Reopens a project, also of a venue this computer has never been to.",
  "Toglie dall'elenco quelli spariti da tempo (i critici restano).": "Removes those gone for a while from the list (critical ones stay).",
  "Per la crew e il rider, da stampare o salvare in PDF.": "For the crew and the rider, to print or save as PDF.", "L'elenco in un foglio di calcolo.": "The list in a spreadsheet.", "L'elenco completo, per altri programmi.": "The full list, for other programs.",
  "Un file da mandare a chi sviluppa wasaping quando qualcosa non va.": "A file to send to wasaping's developer when something is wrong.", "Generali": "General", "Allarmi e notifiche": "Alarms and notifications", "Accesso da altri dispositivi": "Access from other devices", "Sistema": "System", "Altri programmi": "Other programs", "Salva allarmi e notifiche": "Save alarms and notifications", "Manda una prova": "Send a test",
  "Quando un dispositivo o un universo critico cade o torna, wasaping avvisa dove scegli qui.": "When a critical device or universe drops or comes back, wasaping tells the places you choose here.",
  "Un messaggio OSC (a QLab, a una console, a un media server) e/o una chiamata all'indirizzo web con i dettagli in JSON.": "An OSC message (to QLab, a console, a media server) and/or a call to the web address with the details in JSON.", "Esporta CSV": "Export CSV", "Esporta JSON": "Export JSON", "Stampa elenco IP": "Print IP list", "Indirizzo web (webhook): https://…": "Web address (webhook): https://…", "Elenco": "List", "Panoramica": "Overview",
  "In ascolto su Art-Net (pacchetti in broadcast o diretti a questo computer) e sACN (universi 1–64). Solo ascolto: wasaping non manda DMX. Clic su un universo per vedere i 512 canali, clic su un canale per seguirlo nel tempo.": "Listening to Art-Net (packets in broadcast or sent to this computer) and sACN (universes 1–64). Listening only: wasaping sends no DMX. Click a universe to see its 512 channels, click a channel to follow it over time.",
  "Per vedere gli apparecchi sopra i canali, fai una ricerca in Show → Apparecchi RDM.": "To see the fixtures over the channels, run a search in Show → RDM fixtures.", "Scegli un universo dall'elenco.": "Pick a universe from the list.",
  "Sorgenti a confronto": "Sources compared", "Differiscono su": "They differ on", "canali:": "channels:", "Mandano gli stessi valori.": "They send the same values.", "Raccolgo i valori…": "Collecting values…",
  "Stessa priorità: i ricevitori uniscono le sorgenti e tengono il valore più alto (HTP). Di solito non è voluto: controlla le priorità sulle console.": "Same priority: receivers merge the sources and keep the highest value (HTP). Usually not intended: check the priorities on the consoles.",
  "Su Art-Net i nodi uniscono le due sorgenti, di solito tenendo il valore più alto (HTP); alcuni nodi tengono l'ultimo arrivato (LTP).": "On Art-Net the nodes merge the two sources, usually keeping the highest value (HTP); some nodes keep the latest (LTP).",
  "Pacchetti mancanti nell'ultimo secondo (numeri di sequenza saltati): rete congestionata, Wi-Fi o switch in difficoltà": "Packets missing in the last second (skipped sequence numbers): congested network, Wi-Fi or a struggling switch", "Velocità": "Speed", "Video": "Video", "Leggi lo stato": "Read the status", "Uscita": "Output", "Ingresso": "Input", "Uscita bloccata": "Output locked", "Proiettori (PJLink)": "Projectors (PJLink)", "Proiettore": "Projector", "Ore lampada": "Lamp hours", "Errori": "Errors", "serve la password PJLink": "PJLink password needed", "Clock PTPv2 (SMPTE 2110, AES67)": "PTPv2 clock (SMPTE 2110, AES67)", "NDI": "NDI",
  "wasaping chiede lo stato ai dispositivi video trovati nell'ultima scansione: ATEM, HyperDeck, Videohub, vMix, Resolume. Solo lettura. Alcuni accettano pochi collegamenti di controllo: wasaping si collega un attimo e si stacca.": "wasaping asks the video devices of the latest scan for their status: ATEM, HyperDeck, Videohub, vMix, Resolume. Read-only. Some accept few control connections: wasaping connects for a moment and leaves.",
  "Nessun ATEM, HyperDeck, Videohub, vMix o Resolume nell'ultima scansione.": "No ATEM, HyperDeck, Videohub, vMix or Resolume in the latest scan.",
  "Una sorgente NDI a piena qualità in 1080p usa circa 100–150 Mbit/s: va su porte da 1 Gbit, non sul Wi-Fi.": "A full-quality 1080p NDI source uses about 100–150 Mbit/s: it needs 1 Gbit ports, not Wi-Fi.", "Dante": "Dante", "Frequenza": "Sample rate", "Latenza": "Latency", "Canali in trasmissione": "Transmit channels", "Clock": "Clock", "✓ segue": "✓ follows", "master": "master",
  "Chi segue il master del clock PTP: ha chiesto il ritardo al master durante la scansione": "Who follows the PTP clock master: it asked the master for the delay during the scan",
  "Nessun dispositivo Dante nell'ultima scansione.": "No Dante device in the latest scan.", "Collegamenti (routing)": "Connections (routing)", "Leggi il routing": "Read the routing", "un altro IP (facoltativo)": "another IP (optional)",
  "wasaping chiede a ogni dispositivo i suoi canali e a cosa è collegato ogni canale in ricezione, con il protocollo di Dante Controller. Solo lettura: non cambia niente. Il protocollo non è pubblico: se un dispositivo non risponde o i dati sembrano strani, fai fede a Dante Controller e mandaci una diagnostica.": "wasaping asks each device for its channels and what each receive channel is connected to, with Dante Controller's protocol. Read-only: nothing changes. The protocol is not public: if a device does not answer or the data look odd, trust Dante Controller and send us a diagnostic.",
  "Chiedo ai dispositivi…": "Asking the devices…", "Nessun dispositivo da interrogare.": "No device to ask.", "Banda stimata in uscita (unicast):": "Estimated outgoing bandwidth (unicast):", "Canale": "Channel", "Riceve da": "Receives from", "Stato": "Status", "In trasmissione:": "Transmitting:",
  "non collegato": "not connected", "non risolto: il trasmettitore non si trova sulla rete": "unresolved: the transmitter is not on the network", "collegato (unicast)": "connected (unicast)", "collegato (multicast)": "connected (multicast)", "collegato (manuale)": "connected (manual)", "dominio di clock diverso": "different clock domain", "formato del canale incompatibile": "incompatible channel format", "latenza del canale troppo alta per questo ricevitore": "channel latency too high for this receiver", "il trasmettitore non ha più flussi liberi": "the transmitter has no free flows", "il trasmettitore ha raggiunto il limite di ricevitori": "the transmitter reached its receiver limit", "risolto, in attesa": "resolved, waiting", "in corso": "in progress", "inattivo": "idle",
  "il dispositivo non risponde in ARC (porta 4440)": "the device does not answer ARC (port 4440)", "Scrivilo qui": "Write it here", "Modifica il piano": "Edit the plan", "Esporta CSV": "Export CSV", "Modifica il piano IP": "Edit the IP plan", "MAC (facoltativo)": "MAC (optional)", "Togli la riga": "Remove the row", "Aggiungi riga": "Add row", "Ordina per IP": "Sort by IP", "Salva il piano": "Save the plan", "Piano salvato.": "Plan saved.",
  "Una riga per dispositivo previsto. Invio nell'ultima riga ne aggiunge un'altra con l'IP successivo. Le righe vuote non vengono salvate.": "One row per planned device. Enter on the last row adds another with the next IP. Empty rows are not saved.", "Progetto": "Project", "Salva progetto…": "Save project…", "Apri progetto…": "Open project…", "Salva progetto": "Save project", "Apri progetto": "Open project", "Nome": "Name", "Reti": "Networks", "Note dello show": "Show notes", "Salva il file": "Save the file", "Orari, contatti, cose da ricordare…": "Times, contacts, things to remember…",
  "Un file": "A", "con tutto il lavoro su questo locale: dispositivi con nomi, ruoli, note e tipi, critici, piano IP, universi critici, patch RDM, storico e note dello show. Lo riapri qui, lo passi ai colleghi o lo prepari prima di arrivare.": "file with all the work on this venue: devices with names, roles, notes and types, critical ones, IP plan, critical universes, RDM patch, history and show notes. Reopen it here, pass it to colleagues or prepare it before arriving.",
  "Scegli almeno una rete.": "Choose at least one network.", "Il file non è un progetto di wasaping.": "The file is not a wasaping project.", "dispositivi": "devices", "piano IP": "IP plan", "patch RDM": "RDM patch", "è la rete attuale": "it is the current network", "Apri": "Open", "Unisci alla rete attuale": "Merge into the current network",
  "Nomi, ruoli e note del file prendono il posto di quelli che hai già; i dispositivi critici e gli universi critici si aggiungono. Una rete dove questo computer non è mai stato viene preparata: quando arrivi lì e fai la scansione, ritrovi tutto.": "Names, roles and notes from the file replace the ones you have; critical devices and universes add up. A network this computer has never been on is prepared: when you get there and scan, everything is there.", "Pulisci i dispositivi vecchi": "Clean up old devices", "non visti da più di 3 mesi": "not seen for more than 3 months", "non visti da più di 30 giorni": "not seen for more than 30 days", "non visti da più di una settimana": "not seen for more than a week", "tutti quelli che non rispondono ora": "all those not answering now",
  "anche quelli a cui hai dato un nome, un ruolo o una nota": "also those you gave a name, a role or a note",
  "Toglie dall'elenco di questo locale i dispositivi spariti. Se tornano, wasaping li ritrova come nuovi. I dispositivi": "Removes the vanished devices from this venue's list. If they come back, wasaping finds them as new. The", "restano sempre.": "devices always stay.", "critici": "critical", "Controlla ora": "Check now", "Il controllo è spento.": "The check is off.", "Versione di sviluppo: non si aggiorna.": "Development version: it does not update.", "Accesso dalla rete": "Access from the network", "Sei collegato da un altro computer.": "You are connected from another computer.", "Disconnetti": "Sign out",
  "Accesso completo dalla rete": "Full access from the network", "nuova password (facoltativa)": "new password (optional)", "password (almeno 8 caratteri)": "password (at least 8 characters)", "Salva password": "Save password",
  "Tutta l'interfaccia, anche le modifiche, da un altro computer: HTTPS e password.": "The whole interface, changes included, from another computer: HTTPS and password.",
  "Da un altro computer apri": "From another computer open", "e inserisci la password. Il browser avviserà che il certificato non è verificato: è normale, l'ha creato wasaping. Impronta SHA-256:": "and type the password. The browser will warn that the certificate is not verified: that is normal, wasaping made it. SHA-256 fingerprint:",
  "Scrivi la password": "Write the password", "Password salvata.": "Password saved.", "Notifiche sul telefono": "Phone notifications", "ntfy: nome dell'argomento": "ntfy: topic name", "Telegram: token del bot": "Telegram: bot token", "id della chat": "chat id",
  "Il modo più semplice è ntfy: installa l'app ntfy sul telefono, iscriviti a un argomento con un nome difficile da indovinare e scrivi lo stesso nome qui. Per Telegram: crea un bot con @BotFather e scrivi qui il suo token e l'id della chat. Per Slack: un «incoming webhook» del canale.": "The simplest is ntfy: install the ntfy app on the phone, subscribe to a topic with a hard-to-guess name and write the same name here. For Telegram: create a bot with @BotFather and write its token and the chat id here. For Slack: an “incoming webhook” of the channel.",
  "Niente da provare: scrivi almeno una destinazione.": "Nothing to test: write at least one destination.", "Indirizzo DMX": "DMX address", "Avanti": "Next", "Cambia ora": "Change now", "Sto per cambiare:": "About to change:",
  "⚠ Il nuovo indirizzo si sovrappone a:": "⚠ The new address overlaps:", "Ho capito, cambia l'apparecchio": "Understood, change the fixture",
  "L'apparecchio cambia subito: se è in uso nello show, la console non lo comanderà più come prima. wasaping rilegge l'apparecchio per verificare e scrive la modifica nel registro.": "The fixture changes right away: if it is in use in the show, the console will no longer control it as before. wasaping reads the fixture back to verify and writes the change to the log.",
  "✓ Cambiato e verificato": "✓ Changed and verified", "L'indirizzo va da 1 a 512": "The address goes from 1 to 512", "Niente da cambiare": "Nothing to change",
  "Chiedo ai nodi… (può volerci qualche secondo per apparecchio)": "Asking the nodes… (it can take a few seconds per fixture)", "Nessun nodo Art-Net: fai prima una scansione.": "No Art-Net nodes: scan first.",
  "Nessun apparecchio RDM ha risposto. Controlla che il nodo abbia l'RDM attivo sulle uscite.": "No RDM fixture answered. Check that the node has RDM on for its outputs.", "Scegli una rete per dargli un nome, usare il piano IP o esportare il locale.": "Pick a network to name it, use the IP plan or export the venue.",
  "Scegli prima una rete dalle schede.": "First pick a network from the tabs.", "Strumenti ▾": "Tools ▾", "⚙ Impostazioni": "⚙ Settings", "Impostazioni": "Settings",
  "Check pre-show": "Pre-show check", "Esporta": "Export", "Locale": "Venue", "Show": "Show", "Rete": "Network", "Stampa elenco IP": "Print IP list", "Apparecchi RDM": "RDM fixtures",
  "Lingua": "Language", "Tema": "Theme", "automatico": "automatic", "chiaro": "light", "scuro": "dark", "Ripeti la scansione": "Repeat the scan", "mai": "never",
  "ogni 5 minuti": "every 5 minutes", "ogni 15 minuti": "every 15 minutes", "ogni ora": "every hour", "Anche a finestra chiusa, con una notifica quando arriva un dispositivo nuovo.": "With the window closed too, with a notification when a new device shows up.",
  "Allarmi verso altri programmi": "Alarms to other programs", "Prova": "Test", "Monitor su iPad o telefono": "Monitor on iPad or phone", "attivo": "on", "Sola lettura, protetto da un PIN.": "Read-only, protected by a PIN.",
  "Aggiornamenti": "Updates", "controlla le nuove versioni": "check for new versions", "Apri al login": "Open at login", "apri wasaping all'accesso, nella barra dei menu": "open wasaping at login, in the menu bar",
  "Indirizzi MAC (macOS)": "MAC addresses (macOS)", "aiuto di sistema attivo": "system helper active", "nascosti da macOS": "hidden by macOS", "Community SNMP": "SNMP community",
  "Per leggere dagli switch su quale porta è ogni dispositivo.": "To read from the switches which port each device is on.", "Allarmi salvati.": "Alarms saved.", "Inviato.": "Sent.",
  "Quando un dispositivo o un universo critico cade o torna, wasaping manda un messaggio OSC (a QLab, a una console, a un media server) e/o chiama l'indirizzo web con i dettagli in JSON.": "When a critical device or universe drops or comes back, wasaping sends an OSC message (to QLab, a console, a media server) and/or calls the web address with the details in JSON.",
  "Scarico l'aggiornamento…": "Downloading the update…", "Aggiornato: wasaping si riavvia.": "Updated: wasaping restarts.",
  "Controlla tutto prima dello show e prepara un report": "Checks everything before the show and prepares a report", "Piano IP, universi, RDM, switch, esportazioni": "IP plan, universes, RDM, switches, exports", "(MAC privato/casuale)": "(private/random MAC)", "Stampa": "Print",
  "Il monitor è fermo": "The monitor is stopped", "Tema chiaro o scuro": "Light or dark theme",
  // server warnings and network problems
  "Fuori dalla sottorete di questo computer: chi è sulla rete normale non lo raggiunge. Controlla IP e maschera di rete.": "Outside this computer's subnet: devices on the normal network cannot reach it. Check its IP and netmask.",
  "Telnet aperto (porta 23): accesso senza cifratura, spesso con password di fabbrica.": "Telnet open (port 23): unencrypted access, often with the factory password.",
  "FTP aperto sul router (porta 21): meglio disattivarlo.": "FTP open on the router (port 21): better switch it off.",
  "Pannello del router solo in HTTP: la password viaggia in chiaro.": "Router panel on HTTP only: the password travels in clear text.",
  "Desktop remoto di Windows aperto (porta 3389).": "Windows Remote Desktop open (port 3389).",
  "PJLink senza password: chiunque sulla rete può spegnere il proiettore.": "PJLink without password: anyone on the network can switch the projector off.",
  "Nessun router su questa rete: normale per una rete show isolata, ma niente internet e niente DHCP dal router.": "No router on this network: normal for an isolated show network, but no internet and no DHCP from a router.",
}));
const PAT = [
  [/^Priorità diverse: gli apparecchi seguono solo la sorgente con priorità (\d+); le altre sono di riserva\.$/, "Different priorities: fixtures follow only the source with priority $1; the others are backups."],
  [/^(\d+) attivi$/, "$1 active"], [/^(\d+) persi$/, "$1 lost"],
  [/^Stato: (.+), timecode (.*)$/, (m, a, b) => `Status: ${({"fermo": "stopped", "in riproduzione": "playing", "IN REGISTRAZIONE": "RECORDING", "anteprima": "preview", "avanti": "forward", "indietro": "rewind"})[a] || a}, timecode ${b}`],
  [/^Formato: (.+)$/, "Format: $1"], [/^Slot (\d+): vuoto$/, "Slot $1: empty"], [/^Slot (\d+): (.+), (.+) di registrazione liberi$/, "Slot $1: $2, $3 of recording left"],
  [/^Slot (\d+): meno di 30 minuti di registrazione liberi\.$/, "Slot $1: less than 30 minutes of recording left."],
  [/^M\/E (\d+) programma: (.+)$/, "M/E $1 program: $2"], [/^M\/E (\d+) anteprima: (.+)$/, "M/E $1 preview: $2"],
  [/^In onda: (.+)$/, "On air: $1"], [/^Anteprima: (.+)$/, "Preview: $1"], [/^Nome: (.+)$/, "Name: $1"], [/^(\d+) ingressi, (\d+) uscite$/, "$1 inputs, $2 outputs"],
  [/^(\d+) collegati$/, "$1 connected"], [/^✗ (\d+) con problemi$/, "✗ $1 with problems"], [/^(\d+) in attesa$/, "$1 waiting"], [/^· (\d+) in trasmissione · (\d+) in ricezione$/, "· $1 transmitting · $2 receiving"],
  [/^Progetto aperto: (\d+) dispositivi\.( Si attiva quando sei su quella rete\.)?$/, (m, n, x) => `Project opened: ${n} devices.${x ? " It becomes active when you are on that network." : ""}`],
  [/^Hai già l'ultima versione \((.+)\)\.$/, "You already have the latest version ($1)."], [/^Controllo non riuscito: (.+)$/, "Check failed: $1"],
  [/^· universo (\d+)$/, "· universe $1"],
  [/^(\d+) dispositivi trovati in ([\d.,]+) s\.$/, "$1 devices found in $2 s."],
  [/^(\d+) dispositivi · scansione automatica delle (.+)$/, "$1 devices · automatic scan at $2"], [/^(\d+) dispositivi · scansione delle (.+)$/, "$1 devices · scan at $2"],
  [/^Sondaggio (\d+)\/(\d+)… ?(raccolgo i dettagli)?$/, (m, a, b, c) => `Probing ${a}/${b}…${c ? " collecting details" : ""}`],
  [/^Scansione di (.+)…$/, "Scanning $1…"], [/^(\d+) nuov[oi]$/, "$1 new"],
  [/^(\d+) non rispond(?:e|ono) \((nascondi|mostra)\)$/, (m, n, a) => `${n} not responding (${a === "nascondi" ? "hide" : "show"})`],
  [/^visto (.+)$/, (m, w) => "seen " + tr(w)], [/^oggi alle (.+)$/, "today at $1"], [/^ieri alle (.+)$/, "yesterday at $1"], [/^(.+) alle (\d+:\d+)$/, "$1 at $2"],
  [/^(\d+) su (\d+) scansioni$/, "$1 of $2 scans"], [/^(\d+) (NON RISPONDE|NON RISPONDONO)$/, "$1 NOT RESPONDING"], [/^da (.+)$/, "since $1"],
  [/^di nuovo in linea dopo (.+)$/, (m, d) => "back online after " + d.replace(" min ", " min ")],
  [/^Rete (\S+) senza router$/, "Network $1 without router"], [/^Rete (\S+), router (.+)$/, "Network $1, router $2"],
  [/^ · (Rete \S+.*)$/, (m, r) => " · " + tr(r)], [/^ · attivo, anche a finestra chiusa$/, " · on, with the window closed too"], [/^ · fermo$/, " · stopped"],
  [/^(.+) · attivo, anche a finestra chiusa$/, "$1 · on, with the window closed too"], [/^(.+) · fermo$/, "$1 · stopped"],
  [/^(.+) · non risponde$/, (m, k) => tr(k) + " · not responding"],
  [/^Ping (\d+)\/(\d+) · (.+)$/, (m, a, b, c) => `Ping ${a}/${b} · ${c === "perso" ? "lost" : c}`], [/^Ping di (.+)…$/, "Pinging $1…"],
  [/^Ricevuti (\d+)\/(\d+) · persi (\d+)%$/, "Received $1/$2 · lost $3%"], [/^min (.+) · media (.+) · max (.+) ms · jitter (.+) ms$/, "min $1 · avg $2 · max $3 ms · jitter $4 ms"],
  [/^✓ (\d+) porte aperte su (\d+) controllate\.$/, "✓ $1 open ports out of $2 checked."], [/^✓ (\d+) a posto$/, "✓ $1 OK"],
  [/^✗ (\d+) mancanti o in conflitto$/, "✗ $1 missing or in conflict"], [/^⚠ (\d+) da controllare$/, "⚠ $1 to check"], [/^(\d+) non previsti$/, "$1 not planned"],
  [/^IP sbagliato: è a (.+)$/, "wrong IP: it is at $1"], [/^IP occupato da (.+)$/, "IP taken by $1"], [/^(\d+) dispositivi aggiornati dal piano\.$/, "$1 devices updated from the plan."],
  [/^(\d+) indirizzi liberi in (.+): non rispondono ora, non sono di dispositivi conosciuti \(anche spenti\) e non sono nel piano IP\. Clic per copiare\.$/, "$1 free addresses in $2: not answering now, not used by known devices (even switched off) and not in the IP plan. Click to copy."],
  [/^in uso: (.*)$/, "in use: $1"], [/^previsto nel piano: (.*)$/, "planned: $1"], [/^conosciuto, ora spento: (.*)$/, "known, now off: $1"],
  [/^(Art-Net|sACN) universo (\d+)$/, "$1 universe $2"], [/^canale (\d+): (\d+) \((\d+)%\)$/, "channel $1: $2 ($3%)"], [/^prio (\d+)$/, "prio $1"],
  [/^· (\d+) indirizzi$/, "· $1 addresses"], [/^(.+) · (\d+) indirizzi$/, "$1 · $2 addresses"], [/^Su questa porta ci sono (\d+) indirizzi: probabilmente un altro switch o un collegamento tra switch$/, "There are $1 addresses on this port: probably another switch or a link between switches"],
  [/^ · porta $/, " · port "], [/^Apri con (.+)$/, (m, a) => "Open with " + tr(a)], [/^Copia (.+)$/, "Copy $1"], [/^Copiato: (.+)$/, "Copied: $1"], [/^Porta (\d+)(.*)$/, "Port $1$2"],
  [/^(.*)clic: apri con (.+) · tasto destro: copia$/, (m, n, a) => `${n}click: open with ${tr(a)} · right click: copy`], [/^(.*)clic: copia (.+)$/, "$1click: copy $2"],
  [/^Salvato: (.+)$/, "Saved: $1"], [/^Elenco aperto nel browser: (.+)$/, "List opened in the browser: $1"], [/^Locale importato: (\d+) dispositivi\. Rifaccio la scansione\.$/, "Venue imported: $1 devices. Scanning again."],
  [/^Non risponde: (.+)$/, "Not responding: $1"], [/^È uscita (.+) \(hai la (.+)\)\.$/, "$1 is out (you have $2)."],
  [/^(Non salvato|Errore|Non inviato|Non esportato|Non importato|Piano non importato|Non attivato|Aiuto non installato|Aiuto non rimosso|Download non riuscito|Salvataggio non riuscito): (.*)$/,
    (m, p, rest) => ({"Non salvato": "Not saved", "Errore": "Error", "Non inviato": "Not sent", "Non esportato": "Not exported", "Non importato": "Not imported", "Piano non importato": "Plan not imported", "Non attivato": "Not turned on", "Aiuto non installato": "Helper not installed", "Aiuto non rimosso": "Helper not removed", "Download non riuscito": "Download failed", "Salvataggio non riuscito": "Saving failed"}[p]) + ": " + tr(rest)],
  [/^Conflitto di IP: su Art-Net rispondono più dispositivi con questo indirizzo \((.+)\)\.$/, "IP conflict: several devices answer on Art-Net with this address ($1)."],
  [/^Alla scansione precedente questo IP era di (.+) \((.+)\): dispositivo sostituito o conflitto di IP\.$/, "At the previous scan this IP belonged to $1 ($2): device replaced, or an IP conflict."],
  [/^Il proiettore segnala: (.+)\.$/, "The projector reports: $1."], [/^Il nodo Art-Net segnala un problema: (.+)$/, "The Art-Net node reports a problem: $1"],
  [/^Dante a frequenze di campionamento diverse \((.+)\): tra loro l\'audio non passa\.$/, "Dante at different sample rates ($1): audio does not pass between them."],
  [/^Dante: (.+) risponde da (.+) sulla stessa rete\. .*$/, "Dante: $1 answers from $2 on the same network. Are primary and secondary on the same switch? They must be on separate networks."],
  [/^Dispositivi Dante, ma nessun master del clock PTP sentito .*$/, "Dante devices, but no PTP clock master heard during the scan: check the clock (or, on Linux, that wasaping can listen on ports 319 and 320)."],
  [/^Dante: (.+) non risulta agganciato al clock .*$/, "Dante: $1 does not seem locked to the clock (no request to the master during the scan). Check it in Dante Controller → Clock Status."],
  [/^Dispositivo Dante su un\'altra sottorete: .*$/, "Dante device on another subnet: Dante Controller sees it, but audio with the other devices does not pass."],
  [/^Più master di clock (.+): (.+)\. Dante e AES67 vogliono un solo master per dominio\.$/, "Several $1 clock masters: $2. Dante and AES67 want a single master per domain."],
  [/^Universo sACN (\d+) inviato da più sorgenti \((.+)\): controlla le priorità o il merge\.$/, "sACN universe $1 sent by several sources ($2): check priorities or merging."],
  [/^master (.+)$/, "master $1"], [/^media (\d+) ms · massimo (\d+) ms · ([\d.]+)% senza risposta$/, "average $1 ms · maximum $2 ms · $3% without answer"], [/^Oggi \((.+)\) confrontato con$/, "Today ($1) compared with"], [/^(\d+) uguali$/, "$1 unchanged"], [/^(\d+) (nuovo|non c'è più|IP cambiato|cambiato)$/, (m, n, k) => n + " " + tr(k)],
  [/^IP cambiato: (.+)$/, "IP changed: $1"], [/^modello: (.+)$/, "model: $1"], [/^indirizzo (\d+) → (\d+)$/, "address $1 → $2"], [/^modalità (.+)$/, "mode $1"], [/^etichetta (.+)$/, "label $1"], [/^universo (\d+) → (\d+)$/, "universe $1 → $2"], [/^(\d+) indirizzi \(collegamento\)$/, "$1 addresses (link)"], [/^(\d+) indirizzo$/, "$1 address"], [/^automatico \((.+)\)$/, (m, k) => `automatic (${tr(k)})`], [/^wasaping pensava: (.+)$/, (m, k) => "wasaping thought: " + tr(k)], [/^✓ Salvato: (.+)$/, "✓ Saved: $1"], [/^(\d+) cadute$/, "$1 drops"], [/^dalle (.+) alle (.+)$/, "from $1 to $2"], [/^nuovo master (.+) \(prima (.+)\)$/, "new master $1 (was $2)"], [/^master del clock di nuovo presente: (.+)$/, "clock master back: $1"], [/^più sorgenti: (.+)$/, "several sources: $1"], [/^(\d+) problemi da risolvere prima dello show\.$/, "$1 problems to fix before the show."], [/^(\d+) su (\d+) a posto(?: · (\d+) dispositivi non previsti)?\.$/, (m, a, b, c) => `${a} of ${b} OK${c ? ` · ${c} devices not planned` : ""}.`],
  [/^(\d+) frame\/s · più sorgenti$/, "$1 frames/s · several sources"], [/^Report aperto nel browser: (.+)$/, "Report opened in the browser: $1"], [/^(\d+) frame\/s$/, "$1 frames/s"], [/^(Art-Net|sACN) universo (\d+)$/, "$1 universe $2"],
  [/^(\d+) nodi Art-Net nell'ultima scansione\.$/, "$1 Art-Net nodes in the latest scan."], [/^(\d+) apparecchi$/, "$1 fixtures"], [/^⚠ (\d+) con indirizzi sovrapposti$/, "⚠ $1 with overlapping addresses"],
  [/^Dante: (.+) non è sulla rete (.+) \(ridondanza primaria\/secondaria assente\)\.$/, "Dante: $1 is not on network $2 (no primary/secondary redundancy)."],
  [/^Tutte le reti \((\d+)\)$/, "All networks ($1)"], [/^Tutte \((\d+)\)$/, "All ($1)"], [/^Aggiorna a (.+)$/, "Update to $1"],
  [/^(\d+) canali in trasmissione$/, "$1 transmit channels"], [/^Apri (.+) sul dispositivo/, null],
];
function tr(s) {
  if (LANG !== "en" || !s) return s;
  const lead = s.match(/^\s*/)[0], trail = s.match(/\s*$/)[0], core = s.trim();
  if (!core) return s;
  if (EN.has(core)) return lead + EN.get(core) + trail;
  for (const [re, rep] of PAT) {
    if (!rep) continue;
    const m = core.match(re);
    if (m) return lead + (typeof rep === "function" ? rep(...m) : core.replace(re, rep)) + trail;
  }
  // "<network>: <message>" (several networks): translate the message.
  const c = core.indexOf(": ");
  if (c > 0 && c < 80) {
    const rest = tr(core.slice(c + 2));
    if (rest !== core.slice(c + 2)) return lead + core.slice(0, c).replace(/^Rete /, "Network ") + ": " + rest + trail;
  }
  return s;
}
const SKIP = new Set(["SCRIPT", "STYLE", "TEXTAREA", "INPUT"]);
function translateTree(root) {
  if (LANG !== "en") return;
  const w = document.createTreeWalker(root, NodeFilter.SHOW_TEXT | NodeFilter.SHOW_ELEMENT);
  for (let n = root; n; n = w.nextNode()) {
    if (n.nodeType === 3) {
      const p = n.parentElement;
      if (p && !SKIP.has(p.tagName) && !p.closest(".nt")) { const t = tr(n.nodeValue); if (t !== n.nodeValue) n.nodeValue = t; }
    } else if (n.nodeType === 1) {
      for (const a of ["title", "placeholder"]) if (n.hasAttribute && n.hasAttribute(a)) { const v = n.getAttribute(a), t = tr(v); if (t !== v) n.setAttribute(a, t); }
      if (n.tagName === "OPTION") { const t = tr(n.textContent); if (t !== n.textContent) n.textContent = t; }
    }
  }
}
// The remote-access hint has a link inside a sentence: translate it whole.
function translateRemoteHint() {
  const el = $("mRemote");
  if (LANG !== "en" || !el || !el.querySelector("b")) return;
  const b = el.querySelectorAll("b");
  if (b.length >= 2 && el.textContent.startsWith("Apri ")) {
    el.innerHTML = `Open <b>${b[0].textContent}</b> on the device (same network) and type the PIN <b style="font-size:16px;letter-spacing:2px">${b[1].textContent}</b>. From there you can only look, not change anything. <a id="rNew">New PIN</a>`;
    $("rNew").onclick = () => post("api/remote", {on: true, new_pin: true}).then(r => r.json()).then(showRemoteHint).catch(e => toast(e.message));
  }
}
if (LANG === "en") {
  document.documentElement.lang = "en";
  translateTree(document.body);
  let pending = false;
  new MutationObserver(muts => {
    if (pending) return;
    pending = true;
    setTimeout(() => { pending = false; translateTree(document.body); translateRemoteHint(); }, 0); // not rAF: it stops in background windows
  }).observe(document.body, {childList: true, subtree: true, characterData: true});
}
$("langBtn").textContent = LANG === "en" ? "Italiano" : "English";
$("langBtn").onclick = () => { try { localStorage.setItem("wasaping.lang", LANG === "en" ? "it" : "en"); } catch {} location.reload(); };
fetch("api/lang", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({lang: LANG})}).catch(() => {});
const PORT_NAMES = {21:"FTP",22:"SSH",23:"Telnet",53:"DNS",80:"HTTP",139:"NetBIOS",443:"HTTPS",445:"SMB",548:"AFP",554:"RTSP",631:"IPP",1883:"MQTT",3389:"RDP",5000:"UPnP",5900:"VNC",8000:"HTTP",8080:"HTTP",8443:"HTTPS",9100:"Stampante",32400:"Plex",62078:"iPhone"};

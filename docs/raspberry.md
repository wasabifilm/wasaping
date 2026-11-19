# wasaping su Raspberry Pi: il "box" del monitor

Un Raspberry Pi collegato alla rete show con wasaping in **modalità box**:
- controlla sempre i dispositivi e gli universi critici;
- fa una scansione ogni 5 minuti;
- tiene il registro degli eventi;
- mostra il Monitor su un **iPad o un telefono** (con PIN, in sola lettura).

Non serve un computer acceso in regia.

## Cosa serve

- Un **Raspberry Pi 4 o 5** (con la porta Ethernet) e un alimentatore buono.
- Una scheda microSD con **Raspberry Pi OS Lite 64 bit**. Con Raspberry Pi Imager attiva SSH e scegli utente e password.
- Per Dante con primaria e secondaria: un **adattatore USB-Ethernet** per la seconda rete.

## Installazione

Dal tuo computer, collegati al Raspberry:

```sh
ssh UTENTE@IP-DEL-RASPBERRY
```

Sul Raspberry:

```sh
# 1. scarica wasaping (sostituisci la versione con l'ultima)
curl -LO https://github.com/wasabifilm/wasaping/releases/latest/download/wasaping-linux-arm64
sudo install -m 755 wasaping-linux-arm64 /usr/local/bin/wasaping

# 2. un utente apposta, senza login
sudo useradd --system --create-home --shell /usr/sbin/nologin wasaping

# 3. il servizio
curl -LO https://raw.githubusercontent.com/wasabifilm/wasaping/main/deploy/wasaping.service
sudo install -m 644 wasaping.service /etc/systemd/system/wasaping.service
sudo systemctl daemon-reload
sudo systemctl enable --now wasaping

# 4. indirizzo e PIN per l'iPad
sudo journalctl -u wasaping -n 20
```

Nel registro trovi la riga «Monitor su iPad o telefono: http://…:47474/ PIN …». Aprila sull'iPad, nella stessa rete.

## Configurarlo

L'interfaccia completa del box ascolta solo su sé stesso, perché sulla rete non deve essere modificabile da chiunque. Per usarla dal tuo computer, apri un tunnel SSH:

```sh
ssh -L 47480:127.0.0.1:47480 UTENTE@IP-DEL-RASPBERRY
```

e poi, nel browser del tuo computer, apri **http://127.0.0.1:47480/**. Da lì:
- fai una scansione;
- segni i dispositivi **critici**, e in Strumenti → Universi DMX gli universi critici;
- dai un nome al locale;
- importi il piano IP.

In alternativa, prepara il locale sul tuo Mac con wasaping e copialo sul box:

```sh
wasaping venue export arena.wasaping            # sul Mac
scp arena.wasaping UTENTE@IP-DEL-RASPBERRY:/tmp/
sudo -u wasaping wasaping venue import /tmp/arena.wasaping   # sul box
sudo systemctl restart wasaping
```

## Comandi utili sul box

```sh
sudo -u wasaping wasaping preshow            # check pre-show (esce con 1 se non è pronto)
sudo -u wasaping wasaping report -since 18:00   # report di fine show
sudo -u wasaping wasaping dmx -t 5s          # universi dal vivo
sudo journalctl -u wasaping -f               # cosa sta facendo
```

## Consigli per la rete show

- **IP fisso:** dai al Raspberry un IP fisso nella rete show, così l'indirizzo per l'iPad non cambia (in `/etc/dhcpcd.conf` o con `nmtui`).
- **Allarmi:** in Impostazioni → *Allarmi verso altri programmi* puoi mandare gli allarmi in OSC a QLab o alla console, o a un indirizzo web.
- **Aggiornare:** ripeti il punto 1 dell'installazione, poi `sudo systemctl restart wasaping`.

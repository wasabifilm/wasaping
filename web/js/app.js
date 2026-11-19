// The page: scanning, results, settings, the sheet. The tools are in network.js and show.js.
// Ports that the system knows how to open, and the program it uses.
const OPEN = {
  80: ["http", "browser"], 8000: ["http", "browser"], 8080: ["http", "browser"], 5000: ["http", "browser"], 32400: ["http", "browser"],
  443: ["https", "browser"], 8443: ["https", "browser"],
  21: ["ftp", "Finder"], 22: ["ssh", "Terminale"], 23: ["telnet", "Terminale"],
  139: ["smb", "condivisione file"], 445: ["smb", "condivisione file"], 548: ["afp", "condivisione file"],
  554: ["rtsp", "lettore video"], 5900: ["vnc", "Condivisione schermo"], 3389: ["rdp", "Desktop remoto"],
};
function portURL(ip, p) {
  const o = OPEN[p];
  if (!o) return "";
  const [scheme] = o;
  if (scheme === "rdp") return `rdp://full%20address=s:${ip}:${p}`;
  if (scheme === "smb" || scheme === "afp") return `${scheme}://${ip}/`;
  const def = {http: 80, https: 443, ftp: 21, ssh: 22, telnet: 23, rtsp: 554, vnc: 5900}[scheme];
  return `${scheme}://${ip}${p === def ? "" : ":" + p}/`;
}
const KIND = {
  router:   ["Router",        '<path d="M4 15h16v4H4z"/><path d="M8 15l-2-6M16 15l2-6"/><circle cx="8" cy="17" r=".6"/><circle cx="11" cy="17" r=".6"/>'],
  computer: ["Computer",      '<rect x="3" y="4" width="18" height="12" rx="1.5"/><path d="M8 20h8M12 16v4"/>'],
  phone:    ["Telefono",      '<rect x="7" y="2.5" width="10" height="19" rx="2"/><path d="M11 18.5h2"/>'],
  tablet:   ["Tablet",        '<rect x="4.5" y="3" width="15" height="18" rx="2"/><path d="M11 18h2"/>'],
  printer:  ["Stampante",     '<path d="M7 9V3h10v6"/><rect x="3" y="9" width="18" height="8" rx="1.5"/><path d="M7 14h10v7H7z"/>'],
  tv:       ["TV / media",    '<rect x="2.5" y="7" width="19" height="13" rx="2"/><path d="M8 3l4 4 4-4"/>'],
  speaker:  ["Altoparlante",  '<rect x="6" y="2.5" width="12" height="19" rx="2"/><circle cx="12" cy="14" r="3.5"/><circle cx="12" cy="6.5" r="1"/>'],
  nas:      ["NAS / archivio",'<rect x="4" y="3" width="16" height="8" rx="1.5"/><rect x="4" y="13" width="16" height="8" rx="1.5"/><path d="M8 7h.01M8 17h.01"/>'],
  camera:   ["Videocamera",   '<path d="M3 7h12v10H3z"/><path d="M15 10l6-3v10l-6-3"/>'],
  lightdesk:["Console luci",  '<rect x="2.5" y="8" width="19" height="11" rx="1.5"/><path d="M6 11v5M10 11v5M14 11v5M18 11v5"/><path d="M5 14h2M9 12h2M13 15h2M17 13h2"/><path d="M7 8l2-4h6l2 4"/>'],
  lighting: ["Luci (nodo / proiettore)", '<path d="M9 3h6l2 7H7z"/><path d="M12 10v3"/><path d="M8 21l4-8 4 8"/><path d="M5 6l-2-1M19 6l2-1"/>'],
  audio:    ["Audio",         '<path d="M4 10v4M8 7v10M12 4v16M16 8v8M20 11v2"/>'],
  video:    ["Video",         '<rect x="3" y="6" width="13" height="12" rx="2"/><path d="M16 10l5-3v10l-5-3"/>'],
  projector:["Proiettore",    '<rect x="2.5" y="8" width="19" height="9" rx="2"/><circle cx="16" cy="12.5" r="2.5"/><path d="M6 12h4M5 17v2M19 17v2"/>'],
  switch:   ["Switch di rete", '<rect x="2.5" y="8" width="19" height="8" rx="1.5"/><path d="M6 12h.01M9 12h.01M12 12h.01M15 12h.01M18 12h.01"/>'],
  iot:      ["Domotica / IoT",'<path d="M9 18h6M10 21h4"/><path d="M12 3a6 6 0 0 0-3.5 10.9c.6.5 1 1.2 1 2.1h5c0-.9.4-1.6 1-2.1A6 6 0 0 0 12 3z"/>'],
  "":       ["Dispositivo",   '<circle cx="12" cy="12" r="4"/>'],
};
const SERVICES = {"netaudio-arc":"Dante", "netaudio-cmc":"Dante", "netaudio-dbc":"Dante", "netaudio-chan":"Dante", ndi:"NDI", blackmagic:"Blackmagic", videohub:"Videohub", qlab:"QLab", osc:"OSC", workstation:"", airplay:"AirPlay", raop:"AirPlay audio", ipp:"Stampa", ipps:"Stampa", printer:"Stampa", "pdl-datastream":"Stampa", scanner:"Scanner", uscan:"Scanner", smb:"Condivisione file", afpovertcp:"Condivisione file (AFP)", ssh:"SSH", "sftp-ssh":"SFTP", rfb:"Condivisione schermo", googlecast:"Chromecast", http:"Pagina web", https:"Pagina web", hap:"HomeKit", homekit:"HomeKit", matter:"Matter", matterc:"Matter", "companion-link":"Dispositivo Apple", "spotify-connect":"Spotify", sonos:"Sonos", "device-info":"", "sleep-proxy":"Sleep proxy", "apple-mobdev2":"Sincronizzazione Apple"};

let hosts = [], gone = [], es = null, sortKey = "ip", sortDir = 1, ui = "browser", showGone = true;
let repeatTimer = null, selected = null, autoRun = false, network = null, issues = [], snmpCommunity = "public", swPorts = {};
let networks = [], activeNet = ""; // several networks: "" = all tabs

const ipNum = ip => ip.split(".").reduce((a, b) => a * 256 + +b, 0);
const esc = s => String(s ?? "").replace(/[&<>"']/g, c => ({"&":"&amp;","<":"&lt;",">":"&gt;",'"':"&quot;","'":"&#39;"}[c]));
const kindIcon = k => `<span class="kind" title="${esc((KIND[k] || KIND[""])[0])}"><svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round">${(KIND[k] || KIND[""])[1]}</svg></span>`;
const label = h => h.name || h.friendly || h.hostname || "";
// [1,2,3,7] -> "1–3, 7"
const ranges = a => {
  const out = []; const v = [...new Set(a || [])].sort((x, y) => x - y);
  for (let i = 0; i < v.length; i++) {
    let j = i; while (j + 1 < v.length && v[j + 1] === v[j] + 1) j++;
    out.push(j > i ? `${v[i]}–${v[j]}` : `${v[i]}`); i = j;
  }
  return out.join(", ");
};
function protoChips(h) {
  return (h.protocols || []).map(p => {
    let extra = "";
    if (p === "Art-Net" && h.artnet) { const u = [...(h.artnet.out || []), ...(h.artnet.in || [])]; if (u.length) extra = " " + ranges(u); }
    if (p === "sACN" && h.sacn && (h.sacn.universes || []).length) extra = " " + ranges(h.sacn.universes);
    if (p === "PTP master" && h.ptp) extra = " " + h.ptp.replace("PTPv", "v").replace(" dominio ", " d");
    return `<span class="proto" title="${esc(p + extra)}">${esc(p + extra)}</span>`;
  }).join("");
}
const when = t => {
  if (!t || t.startsWith("0001")) return "";
  const d = new Date(t), now = new Date();
  const time = d.toLocaleTimeString("it", {hour: "2-digit", minute: "2-digit"});
  if (d.toDateString() === now.toDateString()) return "oggi alle " + time;
  const y = new Date(now); y.setDate(now.getDate() - 1);
  if (d.toDateString() === y.toDateString()) return "ieri alle " + time;
  return d.toLocaleDateString("it", {day: "numeric", month: "short", year: d.getFullYear() === now.getFullYear() ? undefined : "numeric"}) + " alle " + time;
};

async function init() {
  const info = await fetch("api/info").then(r => r.json());
  document.title = "wasaping " + info.version;
  for (const i of info.interfaces) {
    const o = new Option(`${i.name} — ${i.ip}`, i.network);
    if (i.network === info.defaultTarget) o.selected = true;
    $("iface").add(o);
  }
  if ((info.allTargets || []).length > 1) $("iface").add(new Option(`Tutte le reti (${info.allTargets.length})`, "all"));
  $("iface").add(new Option("Personalizzato…", ""));
  $("target").value = info.defaultTarget;
  if (!info.hasOUI) $("oui").style.display = "flex";
  ui = info.ui || "browser";
  if (info.app && ui === "browser") {
    $("quit").hidden = false;
    const ping = () => fetch("api/ping", {method: "POST"}).catch(() => {});
    ping(); setInterval(ping, 10000);
  }
  try { const b = await fetch("api/bg").then(r => r.json()); $("repeat").value = String(b.repeat_minutes || 0); } catch {}
  try { const last = await fetch("api/last").then(r => r.json()); if (last && !hosts.length) applyResult(last); } catch {}
  setInterval(pollLast, 15000);
  showHelper(info.helper);
  snmpCommunity = info.snmp || "public";
  checkUpdate(); setInterval(checkUpdate, 3600000);
  let dismissed = false;
  try { dismissed = localStorage.getItem("wasaping.helperNo") === "1"; } catch {}
  if (info.helper === "missing" && !dismissed) $("machidden").style.display = "flex";
}

// macOS ARP helper: a system service installed once with the password.
function showHelper(state) {
  const f = $("helperFoot");
  if (state === "installed") {
    f.innerHTML = 'Indirizzi MAC: aiuto di sistema attivo · <a id="helperOff">Disinstalla</a>';
    $("helperOff").onclick = () => helper("uninstall");
  } else if (state === "missing") {
    f.innerHTML = 'Indirizzi MAC nascosti da macOS · <a id="helperOn">Installa l\'aiuto</a> (password una sola volta)';
    $("helperOn").onclick = () => helper("install");
  } else f.innerHTML = "";
}
async function helper(what) {
  $("helperFoot").textContent = what === "install" ? "Inserisci la password nella finestra di macOS…" : "Rimuovo l'aiuto…";
  const r = await fetch(`api/helper?do=${what}`, {method: "POST"});
  if (r.ok) {
    const d = await r.json();
    showHelper(d.helper);
    if (d.helper === "installed") $("machidden").style.display = "none";
    if (what === "install") $("status").textContent = "✓ Aiuto installato: rifai la scansione per vedere i MAC.";
  } else {
    showHelper(what === "install" ? "missing" : "installed");
    $("status").textContent = (what === "install" ? "Aiuto non installato: " : "Aiuto non rimosso: ") + await r.text();
  }
}
$("helperBtn").onclick = () => helper("install");
$("helperNo").onclick = () => {
  $("machidden").style.display = "none";
  try { localStorage.setItem("wasaping.helperNo", "1"); } catch {}
};

$("quit").onclick = async () => {
  if (es) stop("");
  await fetch("api/quit", {method: "POST"}).catch(() => {});
  document.body.innerHTML = '<div class="empty" style="padding-top:120px"><div class="logo" style="margin:0 auto 16px"></div>wasaping è chiuso. Puoi chiudere questa scheda.</div>';
};

$("iface").onchange = e => { if (e.target.value) $("target").value = e.target.value; else $("target").focus(); };
$("ports").onchange = e => {
  const custom = e.target.value === "custom";
  $("custom").hidden = !custom;
  if (custom) { if (!$("custom").value) $("custom").value = "22,80,443"; $("custom").select(); }
};
for (const id of ["target", "custom"]) $(id).onkeydown = e => { if (e.key === "Enter" && !es) start(); };
$("repeat").onchange = () => {
  post("api/bg", {repeat_minutes: +$("repeat").value})
    .then(() => toast(+$("repeat").value ? "wasaping ripeterà la scansione da solo, anche a finestra chiusa." : "Ripetizione automatica spenta."))
    .catch(e => toast("Non salvato: " + e.message));
};

// A background scan finished (menu bar, automatic repeat): show it.
let shownAt = "";
async function pollLast() {
  if (es || $("monitor").classList.contains("open")) return;
  try {
    const last = await fetch("api/last").then(r => r.json());
    if (last && last.at !== shownAt) applyResult(last);
  } catch {}
}
function applyResult(r) {
  hosts = r.hosts || [];
  finish(r, `${r.count} dispositivi · scansione ${r.auto ? "automatica" : ""} delle ${new Date(r.at).toLocaleTimeString("it", {hour: "2-digit", minute: "2-digit"})}`.replace("scansione  delle", "scansione delle"));
}

// Global hooks for the macOS menu (⌘R, ⌘F, ⇧⌘M).
window.wasapingScan = () => { if (!es) start(); };
window.wasapingFind = () => { $("filter").focus(); $("filter").select(); };
window.wasapingMonitor = () => openMonitor();
document.addEventListener("keydown", e => {
  if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
  const k = e.key.toLowerCase();
  if (k === "r" && !e.shiftKey) { e.preventDefault(); wasapingScan(); }
  else if (k === "f") { e.preventDefault(); wasapingFind(); }
});

// Theme: automatic (follows the system), light, dark.
const THEMES = ["", "light", "dark"], THEME_NAMES = {"": "automatico", light: "chiaro", dark: "scuro"};
const themeLabel = t => LANG === "en" ? "Theme: " + {"": "automatic", light: "light", dark: "dark"}[t] : "Tema: " + THEME_NAMES[t];
function applyTheme(t) {
  if (t) document.documentElement.dataset.theme = t; else delete document.documentElement.dataset.theme;
  $("theme").textContent = themeLabel(t);
}
let theme = "";
try { theme = localStorage.getItem("wasaping.theme") || ""; } catch {}
applyTheme(theme);
$("theme").onclick = () => {
  theme = THEMES[(THEMES.indexOf(theme) + 1) % THEMES.length];
  try { localStorage.setItem("wasaping.theme", theme); } catch {}
  applyTheme(theme);
};

$("scan").onclick = () => es ? stop("Scansione interrotta.") : start();

function schedule() {} // repeats are done by the program (backgroundLoop)

function start(auto = false) {
  const target = $("target").value.trim();
  if (!target) return;
  if (!auto && typeof showSection === "function" && section !== "list") showSection("list"); // show the scan
  autoRun = auto;
  hosts = []; gone = [];
  clearTimeout(repeatTimer);
  const ports = $("ports").value === "custom" ? ($("custom").value.trim() || "none") : $("ports").value;
  $("scan").textContent = "Stop"; $("scan").classList.add("stop");
  $("status").textContent = `Scansione di ${target}…`; $("extra").innerHTML = "";
  $("prog").style.width = "0";
  es = new EventSource(`api/scan?target=${encodeURIComponent(target)}&ports=${encodeURIComponent(ports)}${auto ? "&auto=1" : ""}`);
  render();
  es.addEventListener("progress", e => {
    const p = JSON.parse(e.data);
    $("prog").style.width = (100 * p.done / p.total) + "%";
    $("status").textContent = `Sondaggio ${p.done}/${p.total}… ${p.done === p.total ? "raccolgo i dettagli" : ""}`;
  });
  es.addEventListener("host", e => { const h = JSON.parse(e.data); if (h.mac && swPorts[h.mac]) h.swport = swPorts[h.mac]; hosts.push(h); render(); });
  es.addEventListener("done", e => finish(JSON.parse(e.data)));
  es.addEventListener("failure", e => stop("Errore: " + JSON.parse(e.data).error));
  es.onerror = () => { if (es) stop("Connessione interrotta: il target è valido?"); };
}

function finish(d, msg) {
    shownAt = d.at || shownAt;
    gone = d.gone || [];
    network = d.network || null; issues = d.issues || [];
    networks = d.networks && d.networks.length ? d.networks : (network ? [network] : []);
    if (!networks.some(n => n.id === activeNet)) activeNet = "";
    showNetwork();
    fetch("api/info").then(r => r.json()).then(i => showHelper(i.helper)).catch(() => {});
    let no = false; try { no = localStorage.getItem("wasaping.helperNo") === "1"; } catch {}
    $("machidden").style.display = d.macHidden && !no ? "flex" : "none";
    if (d.macHidden) $("oui").style.display = "none"; // no MACs, no vendors to look up
    stop(msg || `${d.count} dispositivi trovati in ${d.seconds.toFixed(1)} s.`);
    lastScanAt = d.at ? new Date(d.at).getTime() : Date.now();
    const bits = [];
    if (d.new) bits.push(`<b>${d.new} ${d.new === 1 ? "nuovo" : "nuovi"}</b>`);
    if (gone.length) bits.push(`<a id="toggleGone">${gone.length} non ${gone.length === 1 ? "risponde" : "rispondono"} (${showGone ? "nascondi" : "mostra"})</a>`);
    $("extra").innerHTML = bits.join(" · ");
    if ($("toggleGone")) $("toggleGone").onclick = () => { showGone = !showGone; $("toggleGone").textContent = `${gone.length} non ${gone.length === 1 ? "risponde" : "rispondono"} (${showGone ? "nascondi" : "mostra"})`; render(); };
    render();
    if (selected) openDetail(selected);
}

function stop(msg) {
  if (es) { es.close(); es = null; }
  $("scan").textContent = "Scansiona"; $("scan").classList.remove("stop");
  $("prog").style.width = "100%";
  $("status").textContent = msg;
  $("csv").disabled = $("json").disabled = $("printBtn").disabled = hosts.length === 0;
  render();
  schedule();
}

function selectNet(id) {
  activeNet = id;
  const n = networks.find(x => x.id === id);
  if (n) { network = n; post("api/network", {select: id}).catch(() => {}); }
  showNetwork(); render();
}
function showNetwork() {
  const nb = $("netbar");
  const tabs = networks.length > 1 ? `<span class="tabs">${[["", `Tutte (${hosts.length})`]].concat(networks.map(n => [n.id, `${n.name || n.subnet || n.auto} (${hosts.filter(h => h.net === n.id).length})`]))
    .map(([id, l]) => `<a class="tab${id === activeNet ? " on" : ""}" data-net="${esc(id)}">${esc(l)}</a>`).join("")}</span>` : "";
  if (networks.length > 1 && !activeNet) {
    nb.innerHTML = tabs + `<span class="muted">Scegli una rete per dargli un nome, usare il piano IP o esportare il locale.</span>`;
  } else if (!network) { nb.innerHTML = ""; } else {
    const name = network.name || network.auto;
    nb.innerHTML = tabs + `Rete: <b>${esc(name)}</b>${network.name ? ` <span class="muted">· ${esc(network.auto)}</span>` : ""}${network.is_new ? ' <span class="badge new">rete nuova</span>' : ""} <a id="netRename">${network.name ? "rinomina" : "dai un nome a questo locale"}</a>
`;
    $("netRename").onclick = async () => {
      nb.innerHTML = `Rete: <input id="netName" maxlength="200" value="${esc(network.name || "")}" placeholder="es. Arena Verona, tour 2027" style="width:260px"> <button id="netSave">Salva</button>`;
      $("netName").focus();
      const save = async () => {
        try { network = await post("api/network", {name: $("netName").value}).then(r => r.json()); } catch (e) { toast("Non salvato: " + e.message); }
        showNetwork();
      };
      $("netSave").onclick = save;
      $("netName").onkeydown = e => { if (e.key === "Enter") save(); if (e.key === "Escape") showNetwork(); };
    };
  }
  nb.querySelectorAll("a.tab").forEach(a => a.onclick = () => selectNet(a.dataset.net));
  const box = $("issues");
  box.style.display = issues.length ? "flex" : "none";
  box.innerHTML = issues.map(i => `<span>⚠️ ${esc(i)}</span>`).join("");
}

// New version on GitHub (the program checks once a day).
async function checkUpdate() {
  try {
    const u = await fetch("api/update").then(r => r.json());
    if (!u.newer) { $("updBar").style.display = "none"; return; }
    $("updText").innerHTML = `È uscita <b>wasaping ${esc(u.latest)}</b> (hai la ${esc(u.current)}).`;
    $("updGet").href = u.url;
    $("updGet").onclick = e => { if (u.can_install) { e.preventDefault(); installUpdate(); } };
    $("updBar").style.display = "flex";
  } catch {}
}
$("updOff").onclick = () => post("api/update", {off: true}).then(() => { $("updBar").style.display = "none"; toast("Non controllerò più le nuove versioni."); });

// ---- Settings: language, theme, repeat, alarms, remote, updates… ----
$("setBtn").onclick = settingsView;
async function settingsView() {
  const get = u => fetch(u).then(r => r.json()).catch(() => ({}));
  const [bg, rem, upd, login, alerts, info, wa] = await Promise.all([get("api/bg"), get("api/remote"), get("api/update"), get("api/login"), get("api/alerts"), get("api/info"), get("api/webaccess")]);
  const waInfo = w => w.on ? `Da un altro computer apri <b class="nt">${esc(w.url)}</b> e inserisci la password. Il browser avviserà che il certificato non è verificato: è normale, l'ha creato wasaping. Impronta SHA-256: <span class="mono nt" style="font-size:11px;word-break:break-all">${esc(w.fingerprint || "")}</span>` : "Tutta l'interfaccia, anche le modifiche, da un altro computer: HTTPS e password.";
  const sel = (id, opts, v) => `<select id="${id}">${opts.map(([k, l]) => `<option value="${k}"${String(k) === String(v) ? " selected" : ""}>${esc(l)}</option>`).join("")}</select>`;
  openSheet("Impostazioni", `<div class="set">
    <h4>Generali</h4>
    <b>Lingua</b><div class="opt">${sel("sLang", [["it", "Italiano"], ["en", "English"]], LANG)}</div>
    <b>Tema</b><div class="opt">${sel("sTheme", [["", "automatico"], ["light", "chiaro"], ["dark", "scuro"]], theme)}</div>
    <b>Ripeti la scansione</b><div class="opt">${sel("sRepeat", [[0, "mai"], [5, "ogni 5 minuti"], [15, "ogni 15 minuti"], [60, "ogni ora"]], bg.repeat_minutes || 0)}<span class="tag">Anche a finestra chiusa, con una notifica quando arriva un dispositivo nuovo.</span></div>
    <h4>Allarmi e notifiche</h4>
    <p class="muted sethint">Quando un dispositivo o un universo critico cade o torna, wasaping avvisa dove scegli qui.</p>
    <b>Altri programmi</b><div class="opt">
      <input id="sOSC" placeholder="OSC: 192.168.1.20:53000" value="${esc(alerts.osc || "")}" style="width:220px" spellcheck="false">
      <input id="sOSCAddr" placeholder="/wasaping" value="${esc(alerts.osc_address || "")}" style="width:140px" spellcheck="false">
      <input id="sHook" placeholder="Indirizzo web (webhook): https://…" value="${esc(alerts.webhook || "")}" style="flex:1 1 260px" spellcheck="false">
      <span class="tag">Un messaggio OSC (a QLab, a una console, a un media server) e/o una chiamata all'indirizzo web con i dettagli in JSON.</span></div>
    <b>Notifiche sul telefono</b><div class="opt">
      <input id="sNtfy" placeholder="ntfy: nome dell'argomento" value="${esc(alerts.ntfy || "")}" style="width:240px" spellcheck="false">
      <input id="sTGToken" placeholder="Telegram: token del bot" value="${esc((alerts.telegram || "").split("|")[0])}" style="width:240px" spellcheck="false">
      <input id="sTGChat" placeholder="id della chat" value="${esc((alerts.telegram || "").split("|")[1] || "")}" style="width:130px" spellcheck="false">
      <input id="sSlack" placeholder="Slack: https://hooks.slack.com/…" value="${esc(alerts.slack || "")}" style="flex:1 1 260px" spellcheck="false">
      <span class="tag">Il modo più semplice è ntfy: installa l'app ntfy sul telefono, iscriviti a un argomento con un nome difficile da indovinare e scrivi lo stesso nome qui. Per Telegram: crea un bot con @BotFather e scrivi qui il suo token e l'id della chat. Per Slack: un «incoming webhook» del canale.</span></div>
    <b></b><div class="opt"><button class="primary" id="sAlertSave">Salva allarmi e notifiche</button><button id="sAlertTest">Manda una prova</button></div>
    <h4>Accesso da altri dispositivi</h4>
    <b>Monitor su iPad o telefono</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sRemote"${rem.on ? " checked" : ""}> attivo</label><span class="tag" id="sRemoteInfo">${rem.on ? `Apri <b>${esc(rem.url)}</b> e inserisci il PIN <b>${esc(rem.pin)}</b>.` : "Sola lettura, protetto da un PIN."}</span></div>
    ${info.remote ? `<b>Accesso dalla rete</b><div class="opt"><span>Sei collegato da un altro computer.</span><form method="post" action="/logout" style="margin:0"><button>Disconnetti</button></form></div>` : `<b>Accesso completo dalla rete</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sWeb"${wa.on ? " checked" : ""}> attivo</label>
      <input type="password" id="sWebPw" placeholder="${wa.has_password ? "nuova password (facoltativa)" : "password (almeno 8 caratteri)"}" autocomplete="new-password" style="width:230px"><button id="sWebPwSave">Salva password</button>
      <span class="tag" id="sWebInfo">${waInfo(wa)}</span></div>`}
    <h4>Sistema</h4>
    <b>Aggiornamenti</b><div class="opt"><span>wasaping ${esc(upd.current || "")}</span>${upd.newer ? `<button class="primary" id="sUpdNow">Aggiorna a ${esc(upd.latest)}</button>` : ""}<label class="check" style="margin:0"><input type="checkbox" id="sUpdCheck"${upd.off ? "" : " checked"}> controlla le nuove versioni</label><button id="sUpdNowCheck">Controlla ora</button><span class="tag" id="sUpdInfo"></span></div>
    ${login.available ? `<b>Apri al login</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sLogin"${login.enabled ? " checked" : ""}> apri wasaping all'accesso, nella barra dei menu</label></div>` : ""}
    ${info.helper && info.helper !== "notneeded" ? `<b>Indirizzi MAC (macOS)</b><div class="opt">${info.helper === "installed" ? '<span>aiuto di sistema attivo</span><button id="sHelperOff">Disinstalla</button>' : '<span>nascosti da macOS</span><button id="sHelperOn">Installa l\'aiuto</button>'}</div>` : ""}
    <b>Community SNMP</b><div class="opt"><input id="sSNMP" value="${esc(snmpCommunity)}" style="width:160px" spellcheck="false"><span class="tag">Per leggere dagli switch su quale porta è ogni dispositivo.</span></div>
  </div>`);
  $("sLang").onchange = () => { try { localStorage.setItem("wasaping.lang", $("sLang").value); } catch {} location.reload(); };
  $("sTheme").onchange = () => { theme = $("sTheme").value; try { localStorage.setItem("wasaping.theme", theme); } catch {} applyTheme(theme); };
  $("sRepeat").onchange = () => { $("repeat").value = $("sRepeat").value; $("repeat").onchange(); };
  const alertForm = () => ({osc: $("sOSC").value.trim(), osc_address: $("sOSCAddr").value.trim(), webhook: $("sHook").value.trim(), ntfy: $("sNtfy").value.trim(), slack: $("sSlack").value.trim(),
    telegram: $("sTGToken").value.trim() || $("sTGChat").value.trim() ? $("sTGToken").value.trim() + "|" + $("sTGChat").value.trim() : ""});
  $("sAlertSave").onclick = () => post("api/alerts", alertForm()).then(r => r.json()).then(d => { $("sNtfy").value = d.ntfy || ""; toast("Allarmi salvati."); }).catch(e => toast("Non salvato: " + e.message));
  $("sAlertTest").onclick = () => post("api/alerts", {...alertForm(), test: true}).then(r => r.json()).then(d => toast(d.result || "Niente da provare: scrivi almeno una destinazione.")).catch(e => toast("Errore: " + e.message));
  $("sRemote").onchange = () => post("api/remote", {on: $("sRemote").checked}).then(r => r.json()).then(r => {
    $("sRemoteInfo").innerHTML = r.on ? `Apri <b>${esc(r.url)}</b> e inserisci il PIN <b>${esc(r.pin)}</b>.` : "Sola lettura, protetto da un PIN.";
  }).catch(e => { toast("Non attivato: " + e.message); $("sRemote").checked = false; });
  const webSet = body => post("api/webaccess", body).then(r => r.json()).then(w => { $("sWeb").checked = w.on; $("sWebInfo").innerHTML = waInfo(w); $("sWebPw").value = ""; return w; });
  if ($("sWeb")) {
    $("sWeb").onchange = () => webSet({on: $("sWeb").checked, password: $("sWebPw").value}).catch(e => { toast("Non attivato: " + e.message); $("sWeb").checked = false; });
    $("sWebPwSave").onclick = () => { if (!$("sWebPw").value) { toast("Scrivi la password"); return; } webSet({on: $("sWeb").checked, password: $("sWebPw").value}).then(() => toast("Password salvata.")).catch(e => toast("Non salvata: " + e.message)); };
  }
  $("sUpdNowCheck").onclick = async () => {
    $("sUpdNowCheck").disabled = true;
    try {
      const u = await post("api/update", {off: !$("sUpdCheck").checked, check: true}).then(r => r.json());
      if (u.off) $("sUpdInfo").textContent = "Il controllo è spento.";
      else if (!u.current.startsWith("v")) $("sUpdInfo").textContent = "Versione di sviluppo: non si aggiorna.";
      else if (u.newer) { settingsView(); checkUpdate(); return; }
      else $("sUpdInfo").textContent = `Hai già l'ultima versione (${u.latest || u.current}).`;
    } catch (e) { $("sUpdInfo").textContent = "Controllo non riuscito: " + e.message; }
    $("sUpdNowCheck").disabled = false;
  };
  $("sUpdCheck").onchange = () => post("api/update", {off: !$("sUpdCheck").checked});
  if ($("sUpdNow")) $("sUpdNow").onclick = installUpdate;
  if ($("sLogin")) $("sLogin").onchange = () => post("api/login", {enabled: $("sLogin").checked}).catch(e => { toast(e.message); $("sLogin").checked = !$("sLogin").checked; });
  if ($("sHelperOff")) $("sHelperOff").onclick = () => { closeSheet(); helper("uninstall"); };
  if ($("sHelperOn")) $("sHelperOn").onclick = () => { closeSheet(); helper("install"); };
  $("sSNMP").onchange = () => { snmpCommunity = $("sSNMP").value.trim() || "public"; };
}
async function installUpdate() {
  toast("Scarico l'aggiornamento…");
  try {
    const d = await post("api/update/install", {}).then(r => r.json());
    toast(d.message || "Aggiornato: wasaping si riavvia.");
  } catch (e) {
    toast("Aggiornamento non riuscito: " + e.message);
    const u = await fetch("api/update").then(r => r.json()).catch(() => null);
    if (u && u.url) openURL(u.url);
  }
}

// ---- Sheet: IP plan, free IPs, venue sharing ----
function openSheet(title, html) {
  $("shTitle").textContent = title; $("shBody").innerHTML = html;
  $("sheet").classList.add("open"); $("sheet").setAttribute("aria-hidden", "false");
}
function closeSheet() {
  $("sheet").classList.remove("open"); $("sheet").setAttribute("aria-hidden", "true");
  if (dmxES) { dmxES.close(); dmxES = null; }
}
$("shClose").onclick = closeSheet;
$("sheet").onclick = e => { if (e.target === $("sheet")) closeSheet(); };
function pickFile(accept, then) {
  const f = $("fileIn"); f.accept = accept; f.value = "";
  f.onchange = () => { const file = f.files[0]; if (!file) return; const r = new FileReader(); r.onload = () => then(String(r.result), file.name); r.readAsText(file); };
  f.click();
}

function needOneNet() {
  if (networks.length > 1 && !activeNet) { toast("Scegli prima una rete dalle schede."); return true; }
  return false;
}
function sortValue(h, k) {
  switch (k) {
    case "protocols": return (h.protocols || []).join(" ").toLowerCase();
    case "model": return (h.model || h.vendor || "").toLowerCase();
    case "ip": return ipNum(h.ip);
    case "ports": return (h.ports || []).length;
    case "rtt_ms": return h.rtt_ms || 0;
    case "label": return label(h).toLowerCase();
    default: return (h[k] || "").toLowerCase();
  }
}

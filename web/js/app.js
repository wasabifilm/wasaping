// Scanning, settings and the tools in sheets (IP plan, DMX, RDM, switches, pre-show, report).
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
    <b>Lingua</b><div class="opt">${sel("sLang", [["it", "Italiano"], ["en", "English"]], LANG)}</div>
    <b>Tema</b><div class="opt">${sel("sTheme", [["", "automatico"], ["light", "chiaro"], ["dark", "scuro"]], theme)}</div>
    <b>Ripeti la scansione</b><div class="opt">${sel("sRepeat", [[0, "mai"], [5, "ogni 5 minuti"], [15, "ogni 15 minuti"], [60, "ogni ora"]], bg.repeat_minutes || 0)}<span class="tag">Anche a finestra chiusa, con una notifica quando arriva un dispositivo nuovo.</span></div>
    <b>Allarmi verso altri programmi</b><div class="opt">
      <input id="sOSC" placeholder="OSC: 192.168.1.20:53000" value="${esc(alerts.osc || "")}" style="width:220px" spellcheck="false">
      <input id="sOSCAddr" placeholder="/wasaping" value="${esc(alerts.osc_address || "")}" style="width:140px" spellcheck="false">
      <input id="sHook" placeholder="Indirizzo web (webhook): https://…" value="${esc(alerts.webhook || "")}" style="flex:1 1 260px" spellcheck="false">
      <span class="tag">Quando un dispositivo o un universo critico cade o torna, wasaping manda un messaggio OSC (a QLab, a una console, a un media server) e/o chiama l'indirizzo web con i dettagli in JSON.</span></div>
    <b>Notifiche sul telefono</b><div class="opt">
      <input id="sNtfy" placeholder="ntfy: nome dell'argomento" value="${esc(alerts.ntfy || "")}" style="width:240px" spellcheck="false">
      <input id="sTGToken" placeholder="Telegram: token del bot" value="${esc((alerts.telegram || "").split("|")[0])}" style="width:240px" spellcheck="false">
      <input id="sTGChat" placeholder="id della chat" value="${esc((alerts.telegram || "").split("|")[1] || "")}" style="width:130px" spellcheck="false">
      <input id="sSlack" placeholder="Slack: https://hooks.slack.com/…" value="${esc(alerts.slack || "")}" style="flex:1 1 260px" spellcheck="false">
      <button id="sAlertSave">Salva</button><button id="sAlertTest">Prova</button>
      <span class="tag">Il modo più semplice è ntfy: installa l'app ntfy sul telefono, iscriviti a un argomento con un nome difficile da indovinare e scrivi lo stesso nome qui. Per Telegram: crea un bot con @BotFather e scrivi qui il suo token e l'id della chat. Per Slack: un «incoming webhook» del canale.</span></div>
    <b>Monitor su iPad o telefono</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sRemote"${rem.on ? " checked" : ""}> attivo</label><span class="tag" id="sRemoteInfo">${rem.on ? `Apri <b>${esc(rem.url)}</b> e inserisci il PIN <b>${esc(rem.pin)}</b>.` : "Sola lettura, protetto da un PIN."}</span></div>
    ${info.remote ? `<b>Accesso dalla rete</b><div class="opt"><span>Sei collegato da un altro computer.</span><form method="post" action="/logout" style="margin:0"><button>Disconnetti</button></form></div>` : `<b>Accesso completo dalla rete</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sWeb"${wa.on ? " checked" : ""}> attivo</label>
      <input type="password" id="sWebPw" placeholder="${wa.has_password ? "nuova password (facoltativa)" : "password (almeno 8 caratteri)"}" autocomplete="new-password" style="width:230px"><button id="sWebPwSave">Salva password</button>
      <span class="tag" id="sWebInfo">${waInfo(wa)}</span></div>`}
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
const normMAC = m => (m || "").toLowerCase().replace(/-/g, ":").split(":").map(x => x.padStart(2, "0")).join(":");

function needOneNet() {
  if (networks.length > 1 && !activeNet) { toast("Scegli prima una rete dalle schede."); return true; }
  return false;
}
async function planView() {
  if (needOneNet()) return;
  let pc = {plan: []};
  try { pc = await fetch(`api/plan?net=${encodeURIComponent(activeNet)}&lang=${LANG}`).then(r => r.json()); } catch {}
  if (!(pc.plan || []).length) {
    openSheet("Piano IP", `<p>Importa l'elenco dei dispositivi previsti (il patch o il piano IP preparato per lo show) e wasaping lo confronta con la rete: chi manca, chi ha l'IP sbagliato, chi non era previsto.</p>
      <p class="muted">Un file CSV da Excel, Numbers o Fogli Google, con le colonne <b>Nome</b>, <b>IP</b> e, se vuoi, <b>Ruolo</b> e <b>MAC</b>. Va bene sia la virgola sia il punto e virgola.</p>
      <div class="row"><button class="primary" id="plEdit">Scrivilo qui</button><button id="plImport">Importa CSV</button><button id="plTemplate">Scarica un modello</button><button id="plFromNet" ${hosts.length ? "" : "disabled"}>Crea il piano da questa scansione</button></div>`);
  } else {
    openSheet("Piano IP", planHTML(pc));
    if ($("plApply")) $("plApply").onclick = () => applyPlanNames(pc);
    $("plClear").onclick = async () => { await post("api/plan", {clear: true}); planView(); };
  }
  if ($("plEdit")) $("plEdit").onclick = () => planEditor(pc.plan || []);
  $("plImport").onclick = () => pickFile(".csv,.txt", async text => {
    try { await post("api/plan", {csv: text}); planView(); } catch (e) { toast("Piano non importato: " + e.message); }
  });
  if ($("plCSV")) $("plCSV").onclick = () => download(`wasaping-piano-${stamp()}.csv`, planCSV(pc.plan || []), "text/csv");
  if ($("plTemplate")) $("plTemplate").onclick = () => download(`wasaping-piano-modello.csv`, "Nome;IP;Ruolo;MAC\ngrandMA3 FOH;10.0.0.10;Console luci;\nLuminode Palco SX;10.0.0.21;Universi 1-4;\n", "text/csv");
  if ($("plFromNet")) $("plFromNet").onclick = async () => {
    const q = v => `"${String(v ?? "").replace(/"/g, '""')}"`;
    const csv = ["Nome;IP;Ruolo;MAC"].concat([...hosts].filter(h => !h.self && (!activeNet || h.net === activeNet)).sort((a, b) => ipNum(a.ip) - ipNum(b.ip)).map(h => [q(label(h) || h.ip), h.ip, q(h.role || ""), h.mac || ""].join(";"))).join("\n");
    try { await post("api/plan", {csv}); planView(); } catch (e) { toast(e.message); }
  };
}
// The comparison is done by the program (preshow.go): same result in the page, the terminal and the tests.
function planHTML(pc) {
  const scanned = hosts.length > 0;
  return `<div class="sum">${scanned ? `<span class="pill ok">✓ ${pc.ok} a posto</span><span class="pill bad">✗ ${pc.bad} mancanti o in conflitto</span><span class="pill warn">⚠ ${pc.warn} da controllare</span><span class="pill">${pc.extra.length} non previsti</span>` : '<span class="pill warn">Fai una scansione per il confronto</span>'}</div>
  <div class="wrap" style="margin:0"><table><thead><tr><th>Stato</th><th>Previsto</th><th>IP</th><th>Ruolo</th><th>Trovato</th></tr></thead><tbody>
  ${pc.rows.map(r => `<tr><td class="st-${r.class}">${scanned ? (r.class === "ok" ? "✓ ok" : esc(r.status)) : "—"}</td><td class="nt"><b>${esc(r.name || "—")}</b></td><td class="mono">${esc(r.ip)}</td><td class="nt">${esc(r.role || "")}</td><td class="nt">${esc(r.found || "")}</td></tr>`).join("")}
  ${scanned ? pc.extra.map(h => `<tr><td class="st-warn">non previsto</td><td>—</td><td class="mono">${esc(h.ip)}</td><td></td><td class="nt">${esc(label(h) || h.mac || "")}</td></tr>`).join("") : ""}
  </tbody></table></div>
  <div class="row"><button class="primary" id="plEdit">Modifica il piano</button><button id="plImport">Importa un altro CSV</button><button id="plCSV">Esporta CSV</button>${scanned ? '<button id="plApply" title="Dà ai dispositivi trovati il nome e il ruolo del piano, se non ne hanno già uno">Usa nomi e ruoli del piano</button>' : ""}<button id="plClear" class="danger">Togli il piano</button></div>`;
}
const planCSV = items => ["Nome;IP;Ruolo;MAC"].concat(items.map(it => [it.name, it.ip, it.role, it.mac].map(v => `"${String(v ?? "").replace(/"/g, '""')}"`).join(";"))).join("\n");
// The plan as an editable table: name, IP, role, MAC.
function planEditor(items) {
  const rows = items.map(it => ({...it}));
  if (!rows.length) rows.push({name: "", ip: "", role: "", mac: ""});
  const validIP = ip => /^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/.test(ip) && ip.split(".").every(x => +x <= 255);
  const nextIP = ip => { if (!validIP(ip)) return ""; const p = ip.split(".").map(Number); p[3] = Math.min(254, p[3] + 1); return p.join("."); };
  const draw = () => {
    openSheet("Modifica il piano IP", `<p class="muted">Una riga per dispositivo previsto. Invio nell'ultima riga ne aggiunge un'altra con l'IP successivo. Le righe vuote non vengono salvate.</p>
      <div class="wrap" style="margin:0"><table class="pledit"><thead><tr><th>Nome</th><th>IP</th><th>Ruolo</th><th>MAC (facoltativo)</th><th></th></tr></thead><tbody>
      ${rows.map((r, i) => `<tr data-i="${i}"><td><input class="nt" data-f="name" value="${esc(r.name || "")}"></td><td><input class="mono" data-f="ip" value="${esc(r.ip || "")}" style="width:120px" spellcheck="false"></td>
        <td><input class="nt" data-f="role" value="${esc(r.role || "")}"></td><td><input class="mono" data-f="mac" value="${esc(r.mac || "")}" style="width:150px" spellcheck="false"></td>
        <td><button class="plDel" title="Togli la riga">✕</button></td></tr>`).join("")}
      </tbody></table></div><div id="plErr"></div>
      <div class="row"><button id="plAdd">Aggiungi riga</button><button id="plSort">Ordina per IP</button><span style="flex:1"></span><button class="primary" id="plSave">Salva il piano</button><button id="plCancel">Annulla</button></div>`);
    const body = $("shBody");
    const check = () => {
      const count = {};
      rows.forEach(r => { if (r.ip) count[r.ip] = (count[r.ip] || 0) + 1; });
      body.querySelectorAll("tr[data-i]").forEach(tr => {
        const r = rows[+tr.dataset.i], ip = tr.querySelector('[data-f="ip"]');
        const empty = !(r.name || r.ip || r.role || r.mac);
        ip.classList.toggle("bad", !empty && (!validIP(r.ip) || count[r.ip] > 1));
        ip.title = !empty && !validIP(r.ip) ? (LANG === "en" ? "invalid IP" : "IP non valido") : count[r.ip] > 1 ? (LANG === "en" ? "same IP as another row" : "stesso IP di un'altra riga") : "";
      });
      const bad = body.querySelectorAll("input.bad").length;
      $("plErr").innerHTML = bad ? `<p class="bad">${LANG === "en" ? `${bad} rows to fix (IP missing, invalid or repeated).` : `${bad} righe da correggere (IP mancante, non valido o ripetuto).`}</p>` : "";
      $("plSave").disabled = !!bad;
    };
    body.querySelectorAll("tr[data-i] input").forEach(inp => {
      inp.oninput = () => { rows[+inp.closest("tr").dataset.i][inp.dataset.f] = inp.value.trim(); check(); };
      inp.onkeydown = e => {
        if (e.key !== "Enter") return;
        const i = +inp.closest("tr").dataset.i;
        if (i === rows.length - 1) { rows.push({name: "", ip: nextIP(rows[i].ip), role: "", mac: ""}); draw(); const ins = $("shBody").querySelectorAll(`tr[data-i="${i + 1}"] input`); ins[0] && ins[0].focus(); }
      };
    });
    body.querySelectorAll(".plDel").forEach(b => b.onclick = () => { rows.splice(+b.closest("tr").dataset.i, 1); if (!rows.length) rows.push({name: "", ip: "", role: "", mac: ""}); draw(); });
    $("plAdd").onclick = () => { rows.push({name: "", ip: nextIP(rows.at(-1)?.ip || ""), role: "", mac: ""}); draw(); const ins = $("shBody").querySelectorAll(`tr[data-i="${rows.length - 1}"] input`); ins[0] && ins[0].focus(); };
    $("plSort").onclick = () => { rows.sort((a, b) => (validIP(a.ip) ? ipNum(a.ip) : 1e12) - (validIP(b.ip) ? ipNum(b.ip) : 1e12)); draw(); };
    $("plCancel").onclick = () => planView();
    $("plSave").onclick = async () => {
      try { await post("api/plan", {items: rows}); toast("Piano salvato."); planView(); }
      catch (e) { $("plErr").innerHTML = `<p class="bad">${esc(e.message)}</p>`; }
    };
    check();
  };
  draw();
}
async function applyPlanNames(pc) {
  let n = 0;
  for (const r of pc.rows) {
    const h = r.found_key && hosts.find(x => x.key === r.found_key && (x.net || "") === (r.found_net || ""));
    if (!h || r.class === "bad" || (h.name && h.role)) continue;
    const name = h.name || r.name || "", role = h.role || r.role || "";
    try { await post("api/device", {key: h.key, net: h.net || "", name, role, note: h.note || "", critical: !!h.critical}); Object.assign(h, {name, role}); n++; } catch {}
  }
  toast(`${n} dispositivi aggiornati dal piano.`); render(); planView();
}

// Free addresses in the scanned range: for a new fixture's static IP.
function rangeOf(spec) {
  const toN = ip => ip.split(".").reduce((a, b) => a * 256 + +b, 0) >>> 0;
  let m = spec.match(/^(\d+\.\d+\.\d+\.\d+)\/(\d+)$/);
  if (m) { const bits = +m[2], size = 2 ** (32 - bits), base = Math.floor(toN(m[1]) / size) * size; return size > 2 ? [base + 1, base + size - 2] : [base, base + size - 1]; }
  m = spec.match(/^(\d+\.\d+\.\d+)\.(\d+)-(\d+)$/);
  if (m) return [toN(m[1] + "." + m[2]), toN(m[1] + "." + m[3])];
  return null;
}
const nToIP = n => [n >>> 24, (n >>> 16) & 255, (n >>> 8) & 255, n & 255].join(".");
async function freeView() {
  const spec = (network && network.subnet && ($("target").value.trim() === "all" || networks.length > 1)) ? network.subnet : $("target").value.trim();
  const r = rangeOf(spec);
  if (!r || r[1] - r[0] > 4096) { openSheet("IP liberi", "<p>Scrivi una rete (es. 10.0.0.0/24) o un intervallo nel campo in alto.</p>"); return; }
  let plan = [];
  try { plan = (await fetch(`api/plan?net=${encodeURIComponent(activeNet)}`).then(x => x.json())).plan || []; } catch {}
  const taken = new Map();
  for (const h of hosts) taken.set(h.ip, "in uso: " + (label(h) || h.mac || ""));
  for (const g of gone) if (!taken.has(g.ip)) taken.set(g.ip, "conosciuto, ora spento: " + (g.name || g.friendly || g.hostname || g.mac || ""));
  for (const p of plan) if (!taken.has(p.ip)) taken.set(p.ip, "previsto nel piano: " + (p.name || ""));
  const ranges = []; let start = null;
  for (let n = r[0]; n <= r[1] + 1; n++) {
    const free = n <= r[1] && !taken.has(nToIP(n));
    if (free && start === null) start = n;
    if (!free && start !== null) { ranges.push([start, n - 1]); start = null; }
  }
  const total = ranges.reduce((a, [x, y]) => a + y - x + 1, 0);
  openSheet("IP liberi", `<p>${total} indirizzi liberi in ${esc(spec)}: non rispondono ora, non sono di dispositivi conosciuti (anche spenti) e non sono nel piano IP. Clic per copiare.</p>
    <p class="muted">Controlla comunque l'intervallo del DHCP, se sulla rete c'è un router.</p>
    <div class="free">${ranges.map(([x, y]) => `<a data-ip="${nToIP(x)}">${x === y ? nToIP(x) : nToIP(x) + " – " + nToIP(y).split(".").pop()}</a>`).join("") || "Nessuno."}</div>`);
  $("shBody").querySelectorAll("a[data-ip]").forEach(a => a.onclick = () => copy(a.dataset.ip));
}

// Live DMX: universes heard on Art-Net and sACN. "Elenco": the table and,
// for the universe picked, its 512 channels with the RDM patch on top, the
// history of one channel and the sources side by side. "Panoramica": all
// universes as strips. Listening only: wasaping never sends DMX.
let dmxES = null, dmxSel = "", dmxSess = "", dmxCh = 0, dmxTab = "list", dmxPatch = [], dmxTableAt = 0, dmxTableKey = "", dmxGridFor = "", critUnis = new Set();
const b64bytes = s => Uint8Array.from(atob(s), c => c.charCodeAt(0));
const dmxQuery = () => `s=${dmxSess}&u=${encodeURIComponent(dmxSel)}&ch=${dmxCh}&all=${dmxTab === "all" ? 1 : 0}`;
const dmxWantPost = () => fetch(`api/dmx?${dmxQuery()}`, {method: "POST"}).catch(() => {});
function dmxView() {
  fetch("api/universe").then(r => r.json()).then(l => { critUnis = new Set(l || []); }).catch(() => {});
  fetch("api/rdm").then(r => r.json()).then(d => { dmxPatch = (d.fixtures || []).filter(f => f.address); dmxGridFor = ""; }).catch(() => {});
  openSheet("Universi DMX", `<p class="muted">In ascolto su Art-Net (pacchetti in broadcast o diretti a questo computer) e sACN (universi 1–64). Solo ascolto: wasaping non manda DMX. Clic su un universo per vedere i 512 canali, clic su un canale per seguirlo nel tempo.</p>
    <div class="row"><button id="dmxTabList" class="primary">Elenco</button><button id="dmxTabAll">Panoramica</button></div>
    <div id="dmxBody"><p>In ascolto…</p></div><div id="dmxAll"></div><div id="dmxDetail"></div>`);
  dmxTableKey = ""; dmxGridFor = "";
  const tab = t => { dmxTab = t; $("dmxTabList").classList.toggle("primary", t === "list"); $("dmxTabAll").classList.toggle("primary", t === "all");
    $("dmxBody").hidden = $("dmxDetail").hidden = t !== "list"; $("dmxAll").hidden = t !== "all"; dmxTableKey = ""; dmxWantPost(); };
  $("dmxTabList").onclick = () => tab("list");
  $("dmxTabAll").onclick = () => tab("all");
  $("dmxAll").hidden = dmxTab !== "all"; $("dmxBody").hidden = $("dmxDetail").hidden = dmxTab !== "list";
  $("dmxTabList").classList.toggle("primary", dmxTab === "list"); $("dmxTabAll").classList.toggle("primary", dmxTab === "all");
  dmxSess = Math.random().toString(36).slice(2);
  dmxES = new EventSource(`api/dmx?${dmxQuery()}`);
  dmxES.addEventListener("snap", e => drawDMX(JSON.parse(e.data)));
  dmxES.onerror = () => { if (dmxES) $("dmxBody").innerHTML = "<p>Ascolto interrotto.</p>"; };
}
// The fixtures of the latest RDM search on this universe (Art-Net port-address).
function dmxFixtures(id) {
  const [proto, n] = id.split(":");
  return proto === "art" ? dmxPatch.filter(f => f.universe === +n) : [];
}
function drawDMX(unis) {
  if (!$("dmxBody")) return;
  if (!unis.length) {
    $("dmxBody").innerHTML = "<p>Nessun universo in arrivo. Sulla rete non passa DMX, oppure la console manda Art-Net diretto ai nodi (unicast) e non a questo computer.</p>";
    $("dmxDetail").innerHTML = $("dmxAll").innerHTML = ""; dmxTableKey = dmxGridFor = ""; return;
  }
  // The table: rebuilt when universes or sources change, else once a second.
  const key = unis.map(u => u.id + u.sources.map(x => x.ip + x.name).join()).join("|") + dmxSel;
  if (key !== dmxTableKey || Date.now() - dmxTableAt > 1000) {
    dmxTableKey = key; dmxTableAt = Date.now();
    $("dmxBody").innerHTML = `<div class="wrap" style="margin:0"><table><thead><tr><th>Universo</th><th>Protocollo</th><th>Sorgente</th><th>Frame/s</th><th>Canali attivi</th><th title="Il Monitor avvisa se l'universo smette di arrivare">Critico</th></tr></thead><tbody>
      ${unis.map(u => `<tr class="pick${u.id === dmxSel ? " sel" : ""}" data-id="${esc(u.id)}"><td><b>${u.number}</b></td><td>${esc(u.protocol)}</td>
        <td>${u.sources.map(x => `<span class="nt">${esc(x.name || x.ip)}</span>${x.name ? ` <span class="muted">${esc(x.ip)}</span>` : ""}${x.priority ? ` <span class="muted">prio ${x.priority}</span>` : ""}`).join("<br>")}${u.sources.length > 1 ? ' <span class="badge warn">più sorgenti</span>' : ""}</td>
        <td>${u.sources.map(x => `${x.fps.toFixed(0)}${x.lost ? ` <span class="badge warn" title="Pacchetti mancanti nell'ultimo secondo (numeri di sequenza saltati): rete congestionata, Wi-Fi o switch in difficoltà">${x.lost} persi</span>` : ""}`).join(" / ")}</td>
        <td>${u.active} / 512</td><td><input type="checkbox" class="ucrit" data-id="${esc(u.id)}"${critUnis.has(u.id) ? " checked" : ""}></td></tr>`).join("")}
    </tbody></table></div>`;
    $("dmxBody").querySelectorAll("input.ucrit").forEach(c => {
      c.onclick = e => e.stopPropagation();
      c.onchange = () => post("api/universe", {id: c.dataset.id, critical: c.checked}).then(r => r.json()).then(l => {
        critUnis = new Set(l || []); toast(c.checked ? "Il Monitor ora controlla questo universo." : "Universo tolto dal Monitor.");
      }).catch(e => toast("Errore: " + e.message));
    });
    $("dmxBody").querySelectorAll("tr.pick").forEach(tr => tr.onclick = () => { dmxSel = tr.dataset.id; dmxCh = 0; dmxGridFor = ""; dmxTableKey = ""; dmxWantPost(); });
  }
  if (dmxTab === "all") drawDMXAll(unis);
  drawDMXDetail(unis.find(u => u.id === dmxSel && u.levels));
}
// Overview: one strip of 512 channels per universe.
function drawDMXAll(unis) {
  const ids = unis.map(u => u.id).join();
  if ($("dmxAll").dataset.ids !== ids) {
    $("dmxAll").dataset.ids = ids;
    $("dmxAll").innerHTML = `<div class="dmxall">${unis.map(u => `<div class="dmxrow" data-id="${esc(u.id)}"><span><b>${esc(u.protocol)} ${u.number}</b></span><canvas width="512" height="1"></canvas><span class="muted act"></span></div>`).join("")}</div>
      <div class="dmxscale"><span>1</span><span>128</span><span>256</span><span>384</span><span>512</span></div>`;
    $("dmxAll").querySelectorAll(".dmxrow").forEach(r => r.onclick = () => { dmxSel = r.dataset.id; dmxCh = 0; dmxGridFor = ""; $("dmxTabList").click(); });
  }
  for (const u of unis) {
    const row = $("dmxAll").querySelector(`.dmxrow[data-id="${CSS.escape(u.id)}"]`);
    if (!row || !u.data) continue;
    const lv = b64bytes(u.data), cv = row.querySelector("canvas"), ctx = cv.getContext("2d"), img = ctx.createImageData(512, 1);
    for (let i = 0; i < 512; i++) { const v = lv[i], o = i * 4; img.data[o] = 129; img.data[o + 1] = 215; img.data[o + 2] = 66; img.data[o + 3] = v ? 40 + Math.round(215 * v / 255) : 18; }
    ctx.putImageData(img, 0, 0);
    row.querySelector(".act").textContent = `${u.active} attivi`;
  }
}
// One universe: grid with patch, channel history, sources compared.
function drawDMXDetail(sel) {
  const box = $("dmxDetail");
  if (!sel) { box.innerHTML = dmxSel ? "" : `<p class="muted">Scegli un universo dall'elenco.</p>`; dmxGridFor = ""; return; }
  const fx = dmxFixtures(sel.id);
  // channel -> fixtures on it
  const at = Array.from({length: 512}, () => []);
  fx.forEach((f, k) => { for (let c = f.address; c < f.address + Math.max(f.footprint, 1) && c <= 512; c++) at[c - 1].push(k); });
  const gridKey = sel.id + "|" + fx.map(f => f.uid + f.address + f.footprint).join();
  if (dmxGridFor !== gridKey) {
    dmxGridFor = gridKey;
    const clash = new Set(at.filter(a => a.length > 1).flat());
    box.innerHTML = `<h3 style="font-size:14px;margin:16px 0 0">${esc(sel.protocol)} universo ${sel.number}</h3>
      ${fx.length ? `<div class="dmxlegend">${fx.map((f, k) => `<span class="${clash.has(k) ? "fxc" : k % 2 ? "fxb" : "fxa"}"><b class="nt">${esc(f.label || f.model || f.uid)}</b> ${f.address}–${f.address + Math.max(f.footprint, 1) - 1}</span>`).join("")}</div>`
        : sel.protocol === "Art-Net" ? `<p class="muted">Per vedere gli apparecchi sopra i canali, fai una ricerca in Strumenti → Apparecchi RDM.</p>` : ""}
      <div class="dmxgrid">${Array.from({length: 512}, (_, i) => { const a = at[i];
        const cls = a.length > 1 ? "fxc" : a.length ? (a[0] % 2 ? "fxb" : "fxa") : "";
        const first = a.some(k => fx[k].address === i + 1) ? " fs" : "";
        const who = a.map(k => `${fx[k].label || fx[k].model || fx[k].uid} · ${i + 2 - fx[k].address}/${fx[k].footprint}`).join(" + ");
        return `<i data-c="${i + 1}" class="${cls}${first}${dmxCh === i + 1 ? " watch" : ""}" data-who="${esc(who)}"></i>`; }).join("")}</div>
      <div id="dmxChan"></div><div id="dmxSrc"></div>`;
    box.querySelectorAll(".dmxgrid i").forEach(c => c.onclick = () => {
      dmxCh = +c.dataset.c; box.querySelectorAll(".dmxgrid i.watch").forEach(x => x.classList.remove("watch")); c.classList.add("watch"); dmxWantPost();
    });
  }
  const cells = box.querySelectorAll(".dmxgrid i");
  sel.levels.forEach((v, i) => {
    const c = cells[i]; if (!c) return;
    c.style.background = v ? `rgba(129,215,66,${0.15 + 0.85 * v / 255})` : "";
    c.title = `${LANG === "en" ? "channel" : "canale"} ${i + 1}: ${v} (${Math.round(v / 2.55)}%)${c.dataset.who ? " · " + c.dataset.who : ""}`;
  });
  // the channel followed
  if (dmxCh) {
    const v = sel.levels[dmxCh - 1], who = cells[dmxCh - 1]?.dataset.who;
    $("dmxChan").innerHTML = `<div class="chanbox"><div><b>${LANG === "en" ? "Channel" : "Canale"} ${dmxCh}</b>${who ? ` <span class="nt muted">${esc(who)}</span>` : ""} · <b>${v}</b> <span class="muted">(${Math.round(v / 2.55)}%)</span></div>${chanChart(sel.history || [])}</div>`;
  } else $("dmxChan").innerHTML = "";
  // more sources: side by side
  const srcs = sel.sources.filter(x => x.data);
  if (srcs.length > 1) {
    const lv = srcs.map(x => b64bytes(x.data));
    const diff = [];
    for (let i = 0; i < 512; i++) if (lv.some(l => l[i] !== lv[0][i])) diff.push(i + 1);
    const prios = [...new Set(srcs.map(x => x.priority))];
    const rule = sel.protocol === "sACN"
      ? (prios.length > 1 ? `Priorità diverse: gli apparecchi seguono solo la sorgente con priorità ${Math.max(...prios)}; le altre sono di riserva.` : "Stessa priorità: i ricevitori uniscono le sorgenti e tengono il valore più alto (HTP). Di solito non è voluto: controlla le priorità sulle console.")
      : "Su Art-Net i nodi uniscono le due sorgenti, di solito tenendo il valore più alto (HTP); alcuni nodi tengono l'ultimo arrivato (LTP).";
    $("dmxSrc").innerHTML = `<h3 style="font-size:14px;margin:16px 0 4px">Sorgenti a confronto</h3><p class="muted">${esc(rule)}</p>
      <div class="dmxall">${srcs.map((x, k) => `<div class="dmxrow"><span class="nt"><b>${esc(x.name || x.ip)}</b>${x.priority ? ` · prio ${x.priority}` : ""}</span><canvas width="512" height="1" data-k="${k}"></canvas><span class="muted">${x.fps.toFixed(0)} fps</span></div>`).join("")}</div>
      <p>${diff.length ? `<span>Differiscono su</span> <b>${diff.length}</b> <span>canali:</span> <span class="mono">${esc(chRanges(diff))}</span>` : "Mandano gli stessi valori."}</p>`;
    $("dmxSrc").querySelectorAll("canvas").forEach(cv => {
      const l = lv[+cv.dataset.k], ctx = cv.getContext("2d"), img = ctx.createImageData(512, 1);
      for (let i = 0; i < 512; i++) { const o = i * 4; img.data[o] = 129; img.data[o + 1] = 215; img.data[o + 2] = 66; img.data[o + 3] = l[i] ? 40 + Math.round(215 * l[i] / 255) : 18; }
      ctx.putImageData(img, 0, 0);
    });
  } else $("dmxSrc").innerHTML = "";
}
// "1-24, 30, 40-48"
function chRanges(list) {
  const out = [];
  for (let i = 0; i < list.length; i++) {
    let j = i; while (j + 1 < list.length && list[j + 1] === list[j] + 1) j++;
    out.push(i === j ? `${list[i]}` : `${list[i]}–${list[j]}`); i = j;
  }
  return out.length > 12 ? out.slice(0, 12).join(", ") + " …" : out.join(", ");
}
// The last 10 seconds of one channel.
function chanChart(h) {
  const W = 600, H = 90, now = Date.now(), t0 = now - 10000;
  if (!h.length) return `<p class="muted">Raccolgo i valori…</p>`;
  let d = "", prev = null;
  for (const [t, v] of h) {
    const x = Math.max(0, (t - t0) / 10000 * W), y = H - 4 - v / 255 * (H - 8);
    d += prev === null ? `M${x.toFixed(1)},${y.toFixed(1)}` : `H${x.toFixed(1)}V${y.toFixed(1)}`; prev = y;
  }
  d += `H${W}`;
  return `<svg class="chansvg" viewBox="0 0 ${W} ${H}" preserveAspectRatio="none"><line x1="0" x2="${W}" y1="${H - 4}" y2="${H - 4}" class="ax"/><line x1="0" x2="${W}" y1="4" y2="4" class="ax"/><path d="${d}"/></svg>
    <div class="dmxscale"><span>−10 s</span><span>−5 s</span><span>${LANG === "en" ? "now" : "ora"}</span></div>`;
}

// What changed since last time on this network (a snapshot per session).
async function compareView(withIdx) {
  if (needOneNet()) return;
  let r;
  try { r = await fetch(`api/compare?net=${encodeURIComponent(activeNet)}&lang=${LANG}${withIdx !== undefined ? "&with=" + withIdx : ""}`).then(x => x.json()); } catch { return; }
  const snaps = r.snapshots || [];
  const fmt = t => new Date(t).toLocaleString(LANG === "en" ? "en-GB" : "it", {day: "numeric", month: "short", year: "numeric", hour: "2-digit", minute: "2-digit"});
  if (snaps.length < 2) {
    openSheet("Confronta con l'ultima volta", "<p>Serve almeno un'altra sessione su questa rete: wasaping tiene una fotografia della rete per ogni sessione di lavoro (scansioni a più di un'ora di distanza). La prossima volta che torni in questo locale potrai vedere cosa è cambiato.</p>");
    return;
  }
  const label = {new: "nuovo", missing: "non c'è più", ip: "IP cambiato", changed: "cambiato"};
  const cls = {new: "warn", missing: "bad", ip: "warn", changed: "warn"};
  const sel = snaps.slice(0, -1).map((t, i) => `<option value="${i}"${new Date(t).getTime() === new Date(r.before).getTime() ? " selected" : ""}>${esc(fmt(t))}</option>`).reverse().join("");
  openSheet("Confronta con l'ultima volta", `<div class="row"><span>Oggi (${esc(fmt(r.now))}) confrontato con</span><select id="cmpWith">${sel}</select></div>
    <div class="sum"><span class="pill ok">${r.same} uguali</span>${["new", "missing", "ip", "changed"].map(k => { const n = r.changes.filter(c => c.kind === k).length; return n ? `<span class="pill ${cls[k]}">${n} ${label[k]}</span>` : ""; }).join("")}</div>
    ${r.changes.length ? `<div class="wrap" style="margin:0"><table><tbody>${r.changes.map(c => { const d = c.kind === "missing" ? c.before : c.now;
      return `<tr><td class="st-${cls[c.kind]}">${esc(label[c.kind])}</td><td>${kindIcon(d.kind)}<b class="nt">${esc(d.name || d.key)}</b></td><td class="mono">${esc(d.ip)}</td><td class="nt">${esc(d.model || "")}</td><td class="nt">${esc(c.detail || "")}</td></tr>`; }).join("")}</tbody></table></div>` : "<p>Nessuna differenza: la rete è come allora.</p>"}`);
  $("cmpWith").onchange = () => compareView(+$("cmpWith").value);
}

// Link speed of a device's switch port; slow links for Dante, NDI and video are a problem.
const needsGigabit = h => !!(h.dante || (h.ndi || []).length || (h.protocols || []).some(p => p === "NDI" || p === "Dante" || p === "Blackmagic"));
function speedCell(h) {
  const m = h.swport && h.swport.mbps;
  if (!m) return '<span class="muted">—</span>';
  const txt = m >= 1000 ? `${m / 1000} Gbit/s` : `${m} Mbit/s`;
  return m < 1000 && needsGigabit(h) ? `<span class="badge warn" title="${LANG === "en" ? "Dante, NDI and video need 1 Gbit/s ports" : "Dante, NDI e video vogliono porte da 1 Gbit/s"}">⚠ ${txt}</span>` : txt;
}
// Network map: each switch with its ports and what is on them.
function switchMap(switches) {
  const byName = new Map(switches.map(s => [(s.name || s.ip).toLowerCase(), s]));
  const links = [];
  const cards = switches.map(sw => {
    const ports = new Map();
    for (const [p, n] of Object.entries(sw.ports || {})) ports.set(p, {macs: n, devs: [], nb: []});
    for (const h of hosts) if (h.swport && h.swport.switch_ip === sw.ip) {
      if (!ports.has(h.swport.port)) ports.set(h.swport.port, {macs: 1, devs: [], nb: []});
      ports.get(h.swport.port).devs.push(h);
    }
    for (const n of sw.neighbors || []) {
      if (!ports.has(n.local_port)) ports.set(n.local_port, {macs: 0, devs: [], nb: []});
      ports.get(n.local_port).nb.push(n);
      if (byName.has((n.name || "").toLowerCase())) links.push(`${sw.name || sw.ip} ${n.local_port} ↔ ${n.name} ${n.remote_port || ""}`);
    }
    const rows = [...ports.entries()].sort((a, b) => a[0].localeCompare(b[0], "it", {numeric: true})).map(([p, v]) => {
      const what = v.nb.length ? v.nb.map(n => `<span class="nt">↔ <b>${esc(n.name || "?")}</b>${n.remote_port ? " " + esc(n.remote_port) : ""}</span>`).join("<br>")
        : v.devs.length ? v.devs.map(h => `<span class="nt">${kindIcon(h.kind)}${esc(label(h) || h.mac)} <span class="muted mono">${esc(h.ip)}</span></span>`).join("<br>")
        : v.macs > 1 ? `<span class="muted">${v.macs} indirizzi (collegamento)</span>` : `<span class="muted">${v.macs} indirizzo</span>`;
      return `<tr><td class="mono">${esc(p)}</td><td>${what}</td></tr>`;
    }).join("");
    return `<div class="swcard"><div class="swh"><b class="nt">${esc(sw.name || sw.ip)}</b> <span class="muted">${esc(sw.ip)}</span><div class="sub nt">${esc((sw.descr || "").slice(0, 80))}</div></div><table><tbody>${rows}</tbody></table></div>`;
  }).join("");
  return `${links.length ? `<h3>Collegamenti tra switch</h3><p class="nt">${[...new Set(links)].map(esc).join("<br>")}</p>` : ""}<div class="swmap">${cards}</div>`;
}

// Switch ports: which port of which managed switch each device is on.
function switchView() {
  openSheet("Mappa della rete", `<p>wasaping chiede agli switch gestiti (Luminex, Cisco, Netgear, Ubiquiti, MikroTik…) su quale porta si trova ogni dispositivo, via SNMP in sola lettura.</p>
    <p class="muted">Sullo switch deve essere attivo SNMP v2c con una community di lettura (spesso <b>public</b>).</p>
    <div class="row"><input id="swComm" value="${esc(snmpCommunity)}" style="width:180px" placeholder="community" title="Community SNMP di lettura"><button class="primary" id="swGo" ${hosts.length ? "" : "disabled"}>Cerca</button>${hosts.length ? "" : '<span class="muted">Fai prima una scansione.</span>'}</div>
    <div id="swOut"></div>`);
  $("swGo").onclick = async () => {
    $("swGo").disabled = true; $("swOut").innerHTML = "<p>Chiedo agli switch…</p>";
    try {
      const d = await post("api/switchports", {community: $("swComm").value}).then(r => r.json());
      snmpCommunity = d.community; swPorts = d.ports || {};
      for (const h of hosts) h.swport = h.mac ? swPorts[h.mac] : undefined;
      const rows = hosts.filter(h => h.swport).sort((a, b) => (a.swport.switch + a.swport.port).localeCompare(b.swport.switch + b.swport.port, "it", {numeric: true}));
      $("swOut").innerHTML = !d.switches || !d.switches.length
        ? "<p>Nessuno switch ha risposto. Controlla che SNMP sia attivo e la community giusta.</p>"
        : switchMap(d.switches) + `<h3>Tutti i dispositivi</h3><p>${d.switches.map(x => `<b>${esc(x.name || x.ip)}</b> <span class="muted">${esc(x.ip)} · ${x.macs} indirizzi</span>`).join("<br>")}</p>
           <div class="wrap" style="margin:0"><table><thead><tr><th>Switch</th><th>Porta</th><th>Velocità</th><th>Descrizione porta</th><th>Dispositivo</th><th>IP</th></tr></thead><tbody>
           ${rows.map(h => `<tr><td>${esc(h.swport.switch)}</td><td class="mono">${esc(h.swport.port)}</td><td>${speedCell(h)}</td><td class="nt">${esc(h.swport.alias || "")}</td><td class="nt"><b>${esc(label(h) || h.mac)}</b>${h.swport.shared > 1 ? ` <span class="muted" title="Su questa porta ci sono ${h.swport.shared} indirizzi: probabilmente un altro switch o un collegamento tra switch">· ${h.swport.shared} indirizzi</span>` : ""}</td><td class="mono">${esc(h.ip)}</td></tr>`).join("")}
           </tbody></table></div>`;
      render();
    } catch (e) { $("swOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("swGo").disabled = false;
  };
}

// RDM: the fixtures behind the Art-Net nodes; identify and, with two
// confirmations, change address, mode and label.
function rdmView() {
  const nodes = hosts.filter(h => h.artnet).length;
  openSheet("Apparecchi RDM", `<p>wasaping chiede ai nodi Art-Net quali apparecchi RDM hanno collegati e a ognuno chi è: produttore, modello, etichetta, indirizzo DMX, canali e modalità. La ricerca non cambia niente sugli apparecchi; per cambiare indirizzo, modalità o etichetta c'è «Modifica», che chiede conferma.</p>
    <div class="row"><button class="primary" id="rdmGo" ${nodes ? "" : "disabled"}>Cerca apparecchi</button><span class="muted">${nodes ? `${nodes} nodi Art-Net nell'ultima scansione.` : "Nessun nodo Art-Net: fai prima una scansione."}</span></div>
    <div id="rdmOut"></div>`);
  let fx = [], chHTML = "";
  const overlaps = (a, b) => a !== b && a.universe === b.universe && a.address && b.address &&
    a.address <= b.address + Math.max(b.footprint, 1) - 1 && b.address <= a.address + Math.max(a.footprint, 1) - 1;
  const nodeName = ip => { const h = hosts.find(x => x.ip === ip); return h ? (label(h) || ip) : ip; };
  const show = () => {
    const clash = new Set();
    for (const a of fx) for (const b of fx) if (overlaps(a, b)) { clash.add(a.uid); clash.add(b.uid); }
    $("rdmOut").innerHTML = chHTML + (!fx.length ? "<p>Nessun apparecchio RDM ha risposto. Controlla che il nodo abbia l'RDM attivo sulle uscite.</p>" :
      `<div class="sum"><span class="pill ok">${fx.length} apparecchi</span>${clash.size ? `<span class="pill bad">⚠ ${clash.size} con indirizzi sovrapposti</span>` : ""}</div>
      <div id="rdmEdit"></div>
      <div class="wrap" style="margin:0"><table><thead><tr><th>Universo</th><th>Indirizzo</th><th>Canali</th><th>Etichetta</th><th>Produttore</th><th>Modello</th><th>Modalità</th><th>UID</th><th>Nodo</th><th></th></tr></thead><tbody>
      ${fx.map((f, i) => `<tr${clash.has(f.uid) ? ' class="st-bad"' : ""}><td>${f.universe}</td><td class="mono">${f.address || "—"}${f.address && f.footprint > 1 ? "–" + (f.address + f.footprint - 1) : ""}</td><td>${f.footprint || ""}</td>
        <td class="nt"><b>${esc(f.label || "")}</b></td><td class="nt">${esc(f.maker || "")}</td><td class="nt">${esc(f.model || "")}</td><td>${esc(f.personality || "")}</td><td class="mono">${esc(f.uid)}</td><td class="nt">${esc(nodeName(f.node))}</td>
        <td style="white-space:nowrap"><button class="ident" data-uid="${esc(f.uid)}" data-node="${esc(f.node)}" data-uni="${f.universe}">Identifica</button> <button class="rdmEditBtn" data-i="${i}">Modifica</button></td></tr>`).join("")}
      </tbody></table></div>`);
    $("rdmOut").querySelectorAll("button.ident").forEach(b => b.onclick = () => identify(b));
    $("rdmOut").querySelectorAll("button.rdmEditBtn").forEach(b => b.onclick = () => edit(+b.dataset.i));
  };
  // The edit box: step 1 fill in, step 2 read the summary and confirm.
  const edit = i => {
    const f = fx[i];
    const [pers, count] = (f.personality || "").split("/").map(Number);
    const box = $("rdmEdit");
    box.innerHTML = `<div class="notice warn" style="display:block;margin:10px 0">
      <p style="margin-top:0"><b>Modifica <span class="nt">${esc(f.label || f.uid)}</span></b> <span class="muted"><span class="nt">${esc([f.maker, f.model].filter(Boolean).join(" "))}</span> · universo ${f.universe}</span></p>
      <div class="row" style="flex-wrap:wrap;gap:12px">
        <label>Indirizzo DMX <input id="reAddr" type="number" min="1" max="512" value="${f.address || ""}" style="width:80px"></label>
        ${count > 1 ? `<label>Modalità <select id="reMode">${Array.from({length: count}, (_, k) => `<option value="${k + 1}"${k + 1 === pers ? " selected" : ""}>${k + 1}</option>`).join("")}</select></label>` : ""}
        <label>Etichetta <input id="reLabel" maxlength="32" value="${esc(f.label || "")}" class="nt"></label>
      </div>
      <div id="reSum"></div>
      <div class="row"><button class="primary" id="reNext">Avanti</button><button id="reNo">Annulla</button></div></div>`;
    box.scrollIntoView({block: "nearest"});
    $("reNo").onclick = () => box.innerHTML = "";
    $("reNext").onclick = () => {
      const want = [];
      const addr = +$("reAddr").value;
      if (addr !== (f.address || 0)) {
        if (!(addr >= 1 && addr <= 512)) { toast("L'indirizzo va da 1 a 512"); return; }
        want.push(["address", String(addr), LANG === "en" ? `address ${f.address || "—"} → ${addr}` : `indirizzo ${f.address || "—"} → ${addr}`]);
      }
      if ($("reMode") && +$("reMode").value !== pers) want.push(["mode", $("reMode").value, LANG === "en" ? `mode ${pers} → ${$("reMode").value} (the channels may change)` : `modalità ${pers} → ${$("reMode").value} (i canali possono cambiare)`]);
      const lab = $("reLabel").value.trim();
      if (lab !== (f.label || "")) want.push(["label", lab, LANG === "en" ? `label “${f.label || ""}” → “${lab}”` : `etichetta «${f.label || ""}» → «${lab}»`]);
      if (!want.length) { toast("Niente da cambiare"); return; }
      const after = {...f, address: addr || f.address};
      const hit = fx.filter(o => overlaps(after, o));
      $("reSum").innerHTML = `<p><b>Sto per cambiare:</b></p><ul>${want.map(w => `<li class="nt">${esc(w[2])}</li>`).join("")}</ul>
        ${hit.length ? `<p class="bad"><span>⚠ Il nuovo indirizzo si sovrappone a:</span> <span class="nt">${hit.map(o => esc(o.label || o.uid)).join(", ")}</span></p>` : ""}
        <p>L'apparecchio cambia subito: se è in uso nello show, la console non lo comanderà più come prima. wasaping rilegge l'apparecchio per verificare e scrive la modifica nel registro.</p>
        <label style="display:flex;gap:8px;align-items:center"><input type="checkbox" id="reOK"> Ho capito, cambia l'apparecchio</label>`;
      $("reNext").textContent = "Cambia ora"; $("reNext").disabled = true;
      $("reOK").onchange = () => $("reNext").disabled = !$("reOK").checked;
      for (const id of ["reAddr", "reMode", "reLabel"]) if ($(id)) $(id).disabled = true;
      $("reNext").onclick = async () => {
        $("reNext").disabled = true; $("reNo").disabled = true;
        try {
          let nf = f;
          for (const [field, value] of want) nf = await post("api/rdm/set", {node: f.node, universe: f.universe, uid: f.uid, field, value}).then(r => r.json());
          fx[i] = {...f, ...nf};
          show(); toast("✓ Cambiato e verificato");
        } catch (e) {
          $("reSum").insertAdjacentHTML("beforeend", `<p class="bad">Errore: ${esc(e.message)}</p>`);
          $("reNo").disabled = false;
        }
      };
    };
  };
  $("rdmGo").onclick = async () => {
    $("rdmGo").disabled = true; $("rdmOut").innerHTML = "<p>Chiedo ai nodi… (può volerci qualche secondo per apparecchio)</p>";
    try {
      const d = await post(`api/rdm?lang=${LANG}`, {}).then(r => r.json());
      fx = d.fixtures || [];
      const changes = d.changes || [];
      chHTML = d.previous ? (changes.length ? `<h3>Rispetto alla ricerca precedente</h3><p class="muted">${esc(new Date(d.previous).toLocaleString(LANG === "en" ? "en-GB" : "it"))}</p><ul class="warnlist">${changes.map(c => `<li><b class="nt">${esc(c.label)}</b> <span class="nt">${esc(c.detail)}</span></li>`).join("")}</ul>` : `<p class="muted">Nessun cambiamento rispetto alla ricerca precedente.</p>`) : "";
      show();
    } catch (e) { $("rdmOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("rdmGo").disabled = false;
  };
}
// Dante: devices, clock, and the routing read over ARC (read-only).
function danteView() {
  const dante = hosts.filter(h => h.dante && !h.self);
  const rows = dante.map(h => `<tr><td class="nt"><b>${esc(h.dante.name || label(h) || h.ip)}</b></td><td class="mono">${esc(h.ip)}</td><td>${h.dante.sample_rate ? h.dante.sample_rate / 1000 + " kHz" : "—"}</td>
    <td>${h.dante.latency_ms ? h.dante.latency_ms + " ms" : "—"}</td><td>${h.dante.tx_channels || "—"}</td>
    <td>${h.ptp === "PTPv1" ? '<span class="badge ok">master</span>' : h.ptp_follower ? "✓ segue" : '<span class="muted">—</span>'}</td></tr>`).join("");
  const danteIssues = issues.filter(i => /Dante|clock/i.test(i));
  openSheet("Dante", `${dante.length ? `<div class="wrap" style="margin:0"><table><thead><tr><th>Dispositivo</th><th>IP</th><th>Frequenza</th><th>Latenza</th><th>Canali in trasmissione</th><th title="Chi segue il master del clock PTP: ha chiesto il ritardo al master durante la scansione">Clock</th></tr></thead><tbody>${rows}</tbody></table></div>` : "<p>Nessun dispositivo Dante nell'ultima scansione.</p>"}
    ${danteIssues.length ? `<ul class="warnlist">${danteIssues.map(i => `<li>${esc(i)}</li>`).join("")}</ul>` : ""}
    <h3>Collegamenti (routing)</h3>
    <p class="muted">wasaping chiede a ogni dispositivo i suoi canali e a cosa è collegato ogni canale in ricezione, con il protocollo di Dante Controller. Solo lettura: non cambia niente. Il protocollo non è pubblico: se un dispositivo non risponde o i dati sembrano strani, fai fede a Dante Controller e mandaci una diagnostica.</p>
    <div class="row"><button class="primary" id="dnGo">Leggi il routing</button><input id="dnIP" placeholder="un altro IP (facoltativo)" style="width:190px" class="mono" spellcheck="false"></div>
    <div id="dnOut"></div>`);
  $("dnGo").onclick = async () => {
    $("dnGo").disabled = true; $("dnOut").innerHTML = "<p>Chiedo ai dispositivi…</p>";
    try {
      const d = await fetch(`api/dante?ip=${encodeURIComponent($("dnIP").value.trim())}`).then(r => r.json());
      const devs = d.devices || [], sm = d.summary || {};
      if (!devs.length) { $("dnOut").innerHTML = "<p>Nessun dispositivo da interrogare.</p>"; }
      else {
        const cls = {ok: "ok", bad: "bad", wait: "warn", none: ""};
        const bw = Object.entries(sm.TxMbps || {}).sort((a, b) => b[1] - a[1]);
        $("dnOut").innerHTML = `<div class="sum"><span class="pill ok">${sm.OK || 0} collegati</span>${sm.Bad ? `<span class="pill bad">✗ ${sm.Bad} con problemi</span>` : ""}${sm.Wait ? `<span class="pill warn">${sm.Wait} in attesa</span>` : ""}</div>
          ${bw.length ? `<p class="muted"><span>Banda stimata in uscita (unicast):</span> ${bw.map(([n, m]) => `<span class="nt">${esc(n)}</span> ${m.toFixed(1)} Mbit/s`).join(" · ")}</p>` : ""}
          ${devs.map(x => x.error ? `<div class="dncard"><b class="nt">${esc(x.name || x.ip)}</b> <span class="muted mono">${esc(x.ip)}</span><p class="bad">${esc(x.error)}</p></div>` :
          `<div class="dncard"><b class="nt">${esc(x.name || x.ip)}</b> <span class="muted mono">${esc(x.ip)}</span> <span class="muted">· ${x.tx_count} in trasmissione · ${x.rx_count} in ricezione</span>
            ${(x.rx || []).length ? `<div class="wrap" style="margin:6px 0 0"><table><thead><tr><th>Canale</th><th>Riceve da</th><th>Stato</th></tr></thead><tbody>
            ${x.rx.map(r => `<tr><td class="nt">${r.num}. ${esc(r.name)}</td><td class="nt">${r.tx_device ? `${esc(r.tx_channel)} <span class="muted">@</span> ${esc(r.tx_device)}` : '<span class="muted">—</span>'}</td><td class="st-${cls[r.state] || ""}">${esc(r.detail)}</td></tr>`).join("")}
            </tbody></table></div>` : ""}
            ${(x.tx || []).length ? `<p class="muted" style="margin:6px 0 0"><span>In trasmissione:</span> <span class="nt">${x.tx.map(t => esc(t.name)).join(", ")}</span></p>` : ""}</div>`).join("")}`;
      }
    } catch (e) { $("dnOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("dnGo").disabled = false;
  };
}
// Video: Blackmagic, vMix, Resolume… asked now; projectors, NDI, PTPv2 from the scan.
function videoView() {
  openSheet("Video", `<p class="muted">wasaping chiede lo stato ai dispositivi video trovati nell'ultima scansione: ATEM, HyperDeck, Videohub, vMix, Resolume. Solo lettura. Alcuni accettano pochi collegamenti di controllo: wasaping si collega un attimo e si stacca.</p>
    <div class="row"><button class="primary" id="vdGo">Leggi lo stato</button><input id="vdIP" placeholder="un altro IP (facoltativo)" style="width:190px" class="mono" spellcheck="false">
      <select id="vdKind">${["ATEM", "HyperDeck", "Videohub", "vMix", "Resolume"].map(k => `<option>${k}</option>`).join("")}</select></div>
    <div id="vdOut"></div>`);
  const go = async () => {
    $("vdGo").disabled = true; $("vdOut").innerHTML = "<p>Chiedo ai dispositivi…</p>";
    try {
      const ip = $("vdIP").value.trim();
      const d = await fetch(`api/video${ip ? `?ip=${encodeURIComponent(ip)}&kind=${encodeURIComponent($("vdKind").value)}` : ""}`).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
      const devs = d.devices || [], pj = d.projectors || [], ndi = d.ndi || [], ptp = d.ptp2 || [];
      const hostName = ip => { const h = hosts.find(x => x.ip === ip); return h ? label(h) : ""; };
      let html = "";
      html += devs.length ? devs.map(v => `<div class="dncard"><b class="nt">${esc(v.model || v.kind)}</b> <span class="muted"><span class="mono">${esc(v.ip)}</span>${hostName(v.ip) ? ` · <span class="nt">${esc(hostName(v.ip))}</span>` : ""}${v.version ? ` · v${esc(v.version)}` : ""}</span>
          ${v.error ? `<p class="bad">${esc(v.error)}</p>` : ""}
          ${(v.lines || []).map(l => `<div class="nt">${esc(l)}</div>`).join("")}
          ${(v.warn || []).map(l => `<div class="bad">⚠ ${esc(l)}</div>`).join("")}
          ${(v.routes || []).length ? `<div class="wrap" style="margin:6px 0 0"><table><thead><tr><th>Uscita</th><th>Ingresso</th></tr></thead><tbody>${v.routes.map(r => `<tr><td class="nt">${r.out}. ${esc(r.out_label)}${r.locked ? ' <span class="muted" title="Uscita bloccata">🔒</span>' : ""}</td><td class="nt">${r.in}. ${esc(r.in_label)}</td></tr>`).join("")}</tbody></table></div>` : ""}
        </div>`).join("") : "<p>Nessun ATEM, HyperDeck, Videohub, vMix o Resolume nell'ultima scansione.</p>";
      if (pj.length) html += `<h3>Proiettori (PJLink)</h3><div class="wrap" style="margin:0"><table><thead><tr><th>Proiettore</th><th>IP</th><th>Stato</th><th>Ore lampada</th><th>Errori</th></tr></thead><tbody>${pj.map(p => `<tr><td class="nt"><b>${esc(p.name)}</b> <span class="muted">${esc([p.pjlink.maker, p.pjlink.model].filter(Boolean).join(" "))}</span></td><td class="mono">${esc(p.ip)}</td><td>${esc(p.pjlink.power || (p.pjlink.needs_login ? "serve la password PJLink" : "—"))}</td><td>${(p.pjlink.lamp_hours || []).join(" / ") || "—"}</td><td class="${p.pjlink.errors ? "st-bad" : ""}">${esc(p.pjlink.errors || "—")}</td></tr>`).join("")}</tbody></table></div>`;
      if (ndi.length) html += `<h3>NDI</h3>${ndi.map(n => `<div><b class="nt">${esc(n.name)}</b> <span class="muted mono">${esc(n.ip)}</span>: <span class="nt">${n.sources.map(esc).join(", ")}</span></div>`).join("")}<p class="muted">Una sorgente NDI a piena qualità in 1080p usa circa 100–150 Mbit/s: va su porte da 1 Gbit, non sul Wi-Fi.</p>`;
      if (ptp.length) html += `<h3>Clock PTPv2 (SMPTE 2110, AES67)</h3>${ptp.map(p => `<div><b class="nt">${esc(p.name)}</b> <span class="muted mono">${esc(p.ip)}</span> · ${esc(p.kind)}</div>`).join("")}`;
      $("vdOut").innerHTML = html;
    } catch (e) { $("vdOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("vdGo").disabled = false;
  };
  $("vdGo").onclick = go;
  go();
}
// RDM Identify: the only command wasaping sends to fixtures, so it asks once.
let identOK = false;
async function identify(b) {
  const on = !b.classList.contains("on");
  if (on && !identOK) {
    const note = document.createElement("div");
    note.className = "notice warn"; note.style.cssText = "display:flex;margin:10px 0";
    note.innerHTML = `<span>«Identifica» fa lampeggiare l'apparecchio (o lo muove, se è una testa mobile). Non cambia nessuna impostazione. Durante uno show, usalo solo se sei sicuro.</span><span style="display:flex;gap:8px"><button class="primary" id="identYes">Ho capito, identifica</button><button id="identNo">Annulla</button></span>`;
    $("rdmOut").prepend(note);
    $("identNo").onclick = () => note.remove();
    $("identYes").onclick = () => { identOK = true; note.remove(); identify(b); };
    return;
  }
  b.disabled = true;
  try {
    await post("api/rdm/identify", {node: b.dataset.node, universe: +b.dataset.uni, uid: b.dataset.uid, on});
    b.classList.toggle("on", on); b.classList.toggle("primary", on);
    b.textContent = on ? "Ferma" : "Identifica";
  } catch (e) { toast("Errore: " + e.message); }
  b.disabled = false;
}

// ---- Pre-show check: done by the program (preshow.go), shown here ----
let lastScanAt = 0;
$("preBtn").onclick = preshow;
async function preshow() {
  if (needOneNet()) return;
  openSheet("Check pre-show", "<p>Controllo in corso: scansione, dispositivi critici, universi, piano IP…</p>");
  let rep;
  try {
    rep = await fetch(`api/preshow?net=${encodeURIComponent(activeNet)}&lang=${LANG}`).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
  } catch (e) { openSheet("Check pre-show", `<p>Errore: ${esc(e.message)}</p>`); return; }
  // The program may have run a fresh scan: show it.
  try { const last = await fetch("api/last").then(r => r.json()); if (last && last.at !== shownAt) applyResult(last); } catch {}
  const icon = c => c === "ok" ? "✓" : c === "bad" ? "✗" : "⚠";
  const body = `<div class="verdict v-${rep.verdict}"><b class="nt">${esc(rep.headline)}</b><span class="nt">${esc(rep.summary)}</span></div>
    <p class="muted nt">${esc(rep.network)} · ${esc(new Date(rep.at).toLocaleString(LANG === "en" ? "en-GB" : "it"))}</p>
    ${rep.sections.map(sec => `<h3 class="nt">${esc(sec.title)}</h3>${sec.note ? `<p class="muted nt">${esc(sec.note)}</p>` : ""}${sec.rows.length ? `<div class="wrap" style="margin:0"><table><tbody>${sec.rows.map(r => `<tr><td class="st-${r.class}">${icon(r.class)}</td><td class="nt"><b>${esc(r.name)}</b></td><td class="nt">${esc(r.role || "")}</td><td class="mono">${esc(r.ip || "")}</td><td class="nt">${esc(r.detail || "")}</td></tr>`).join("")}</tbody></table></div>` : ""}`).join("")}
    <div class="row"><button class="primary" id="preSave">Stampa o salva il report</button><button id="preAgain">Ricontrolla</button></div>`;
  openSheet("Check pre-show", body);
  $("preAgain").onclick = preshow;
  $("preSave").onclick = () => {
    const doc = $("shBody").cloneNode(true);
    doc.querySelector(".row")?.remove();
    const html = `<!DOCTYPE html><html lang="${LANG}"><head><meta charset="utf-8"><title>Check pre-show · ${esc(rep.network)}</title><style>
      body{font-family:-apple-system,"Segoe UI",Roboto,sans-serif;font-size:12px;color:#171717;margin:24px}h1{font-size:20px;margin:0 0 6px}h3{font-size:14px;margin:18px 0 6px}
      table{border-collapse:collapse;width:100%}td{border-bottom:1px solid #ddd;padding:5px 6px;vertical-align:top}.muted{color:#666}.mono{font-family:Menlo,Consolas,monospace}
      .verdict{padding:12px 14px;border-radius:8px;display:flex;gap:12px;align-items:center;font-size:14px}.v-ok{background:#e4f6d6}.v-warn{background:#fdebd3}.v-bad{background:#f9d6d6}
      .st-ok{color:#3f7a17}.st-bad{color:#d63f3f}.st-warn{color:#c46a10}.wrap{border:none}@page{size:A4;margin:14mm}</style></head>
      <body><h1>wasaping · ${LANG === "en" ? "Pre-show check" : "Check pre-show"}</h1>${doc.innerHTML}<script>addEventListener("load",()=>setTimeout(()=>print(),300))<\/script></body></html>`;
    if (ui === "browser") { const w = window.open("", "_blank"); if (w) { w.document.write(html); w.document.close(); return; } }
    post("api/save", {name: `wasaping-preshow-${stamp()}.html`, data: html, open: true}).then(r => r.json()).then(d => toast("Report aperto nel browser: " + d.path)).catch(e => toast("Non salvato: " + e.message));
  };
}

// A device's response times over the whole report period.
function rttChart(s, since, until) {
  const t0 = new Date(since).getTime() / 1000, t1 = new Date(until).getTime() / 1000, W = 760, H = 60;
  const max = Math.max(5, ...s.points.filter(p => p[1] >= 0).map(p => p[1]));
  const x = t => ((t - t0) / Math.max(1, t1 - t0) * W).toFixed(1);
  const pts = s.points.filter(p => p[1] >= 0).map(p => `${x(p[0])},${(H - 4 - (H - 8) * p[1] / max).toFixed(1)}`).join(" ");
  const lost = s.points.filter(p => p[1] < 0).map(p => `<rect x="${x(p[0])}" y="0" width="2" height="${H}" fill="#d63f3f" opacity=".7"/>`).join("");
  return `<div class="rtt"><div><b class="nt">${esc(s.name)}</b> <span class="muted mono">${esc(s.ip || "")}</span> <span class="muted">media ${s.avg.toFixed(0)} ms · massimo ${s.max} ms · ${s.loss.toFixed(1)}% senza risposta</span></div>
    <svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" class="rttsvg"><rect width="${W}" height="${H}" fill="none" stroke="var(--line)"/>${lost}<polyline points="${pts}" fill="none" stroke="#81d742" stroke-width="1.3"/>
    <text x="4" y="12" font-size="10" fill="currentColor" opacity=".6">${max} ms</text></svg></div>`;
}

// End-of-show report, from the monitor log.
async function showReportView() {
  const def = (() => { const d = new Date(Date.now() - 6 * 3600e3); return String(d.getHours()).padStart(2, "0") + ":" + String(d.getMinutes()).padStart(2, "0"); })();
  openSheet("Report di fine show", `<p>Riassunto di quello che è successo durante lo show, dal registro del Monitor: quante volte è caduto ogni dispositivo, per quanto tempo in tutto, il buco più lungo.</p>
    <div class="row"><span>Dalle</span><input id="srSince" value="${def}" style="width:90px" title="18:00, 6h o 2026-11-18 18:00"><button class="primary" id="srGo">Mostra</button></div><div id="srOut"></div>`);
  const go = async () => {
    try {
      const r = await fetch(`api/showreport?since=${encodeURIComponent($("srSince").value)}`).then(x => x.ok ? x.json() : x.text().then(t => { throw new Error(t); }));
      const devs = r.devices || [], ev = r.events || [];
      const t = d => new Date(d).toLocaleString(LANG === "en" ? "en-GB" : "it", {day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", second: "2-digit"});
      $("srOut").innerHTML = `<div class="verdict v-${r.drops ? "warn" : "ok"}" style="margin-top:12px"><b>${r.drops ? `${r.drops} cadute` : "Nessuna caduta"}</b><span>dalle ${esc(t(r.since))} alle ${esc(t(r.until))}</span></div>
        ${devs.length ? `<h3>Per dispositivo</h3><div class="wrap" style="margin:0"><table><thead><tr><th>Dispositivo</th><th>IP</th><th>Cadute</th><th>Tempo giù in tutto</th><th>Il più lungo</th></tr></thead><tbody>
          ${devs.map(d => `<tr><td class="nt"><b>${esc(d.name)}</b>${d.still_down ? ' <span class="badge off">ancora giù</span>' : ""}</td><td class="mono">${esc(d.ip || "")}</td><td>${d.drops}</td><td>${esc(fmtDur(d.down_total))}</td><td>${esc(fmtDur(d.longest))}</td></tr>`).join("")}</tbody></table></div>` : ""}
        ${(r.rtt || []).length ? `<h3>Tempi di risposta</h3>${r.rtt.map(s => rttChart(s, r.since, r.until)).join("")}` : ""}
        ${ev.length ? `<h3>Cronologia</h3><div class="wrap" style="margin:0"><table><tbody>${ev.map(e => `<tr><td class="mono">${esc(t(e.at))}</td><td class="nt"><b>${esc(e.name)}</b></td><td class="${e.down ? "st-bad" : e.up ? "st-ok" : "st-warn"}">${esc(e.event)}${e.up ? " · " + esc(fmtDur(e.down_for)) : ""}</td></tr>`).join("")}</tbody></table></div>` : '<p class="muted">Nessun evento nel registro in questo periodo.</p>'}
        <div class="row"><button id="srPrint">Stampa o salva il report</button></div>`;
      $("srPrint").onclick = () => {
        const html = `<!DOCTYPE html><html lang="${LANG}"><head><meta charset="utf-8"><title>Report di fine show</title><style>body{font-family:-apple-system,"Segoe UI",Roboto,sans-serif;font-size:12px;margin:24px;color:#171717}h1{font-size:20px}h3{font-size:14px;margin:18px 0 6px}table{border-collapse:collapse;width:100%}td,th{border-bottom:1px solid #ddd;padding:5px 6px;text-align:left}.mono{font-family:Menlo,Consolas,monospace}.verdict{padding:10px 12px;border-radius:8px;background:#f2f4ef;display:flex;gap:12px}.st-bad{color:#d63f3f}.st-ok{color:#3f7a17}.badge{font-size:10px;border:1px solid #999;border-radius:4px;padding:0 4px}.row,button{display:none}</style></head><body><h1>wasaping · ${LANG === "en" ? "End-of-show report" : "Report di fine show"}</h1>${$("srOut").innerHTML}<script>addEventListener("load",()=>setTimeout(()=>print(),300))<\/script></body></html>`;
        if (ui === "browser") { const w = window.open("", "_blank"); if (w) { w.document.write(html); w.document.close(); return; } }
        post("api/save", {name: `wasaping-report-${stamp()}.html`, data: html, open: true}).then(x => x.json()).then(d => toast("Report aperto nel browser: " + d.path)).catch(e => toast("Non salvato: " + e.message));
      };
    } catch (e) { $("srOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
  };
  $("srGo").onclick = go; $("srSince").onkeydown = e => { if (e.key === "Enter") go(); };
  go();
}

// The show's project: one file with everything about the venue's networks.
function saveProjectView() {
  const nets = networks.length ? networks : (network ? [network] : []);
  const chosen = activeNet ? [activeNet] : nets.map(n => n.id);
  const cur = nets.find(n => n.id === chosen[0]) || network || {};
  openSheet("Salva progetto", `<p>Un file <b>.wasaping</b> con tutto il lavoro su questo locale: dispositivi con nomi, ruoli, note e tipi, critici, piano IP, universi critici, patch RDM, storico e note dello show. Lo riapri qui, lo passi ai colleghi o lo prepari prima di arrivare.</p>
    <div class="set">
      <b>Nome</b><div class="opt"><input id="pjName" class="nt" value="${esc(cur.name || cur.auto || "")}" style="flex:1 1 260px"></div>
      ${nets.length > 1 ? `<b>Reti</b><div class="opt" style="flex-direction:column;align-items:flex-start">${nets.map(n => `<label class="check" style="margin:0"><input type="checkbox" class="pjNet" value="${esc(n.id)}"${chosen.includes(n.id) ? " checked" : ""}> <span class="nt">${esc(n.name || n.auto)}</span></label>`).join("")}</div>` : ""}
      <b>Note dello show</b><div class="opt"><textarea id="pjNotes" rows="5" class="nt" style="flex:1 1 100%" placeholder="Orari, contatti, cose da ricordare…">${esc(cur.notes || "")}</textarea></div>
    </div>
    <div class="row"><button class="primary" id="pjSave">Salva il file</button></div>`);
  $("pjSave").onclick = async () => {
    const ids = nets.length > 1 ? [...document.querySelectorAll(".pjNet:checked")].map(c => c.value) : [cur.id || ""];
    if (!ids.length) { toast("Scegli almeno una rete."); return; }
    const name = $("pjName").value.trim();
    try {
      await post("api/project", {notes: $("pjNotes").value, net: ids[0]});
      if (network && network.id === ids[0]) network.notes = $("pjNotes").value;
      const p = await fetch(`api/project?nets=${encodeURIComponent(ids.join(","))}&name=${encodeURIComponent(name)}`).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
      const file = (name || "wasaping").replace(/[\\/:*?"<>|]+/g, "-").slice(0, 60) + ` ${new Date().toISOString().slice(0, 10)}.wasaping`;
      await download(file, JSON.stringify(p, null, 2), "application/json");
      closeSheet();
    } catch (e) { toast("Non salvato: " + e.message); }
  };
}
async function openProjectView(text, fileName) {
  let p;
  try { p = JSON.parse(text); } catch { toast("Il file non è un progetto di wasaping."); return; }
  const venues = p.format === "wasaping-project" ? (p.venues || []) : p.format === "wasaping-venue" ? [p] : null;
  if (!venues || !venues.length) { toast("Il file non è un progetto di wasaping."); return; }
  const here = id => (networks.length ? networks : network ? [network] : []).some(n => n.id === id);
  const curID = network ? network.id : "";
  const canMerge = venues.length === 1 && network && venues[0].id !== curID;
  openSheet("Apri progetto", `<p><b class="nt">${esc(p.name || venues[0].name || venues[0].auto || fileName)}</b>${p.saved_at ? ` <span class="muted">· ${esc(new Date(p.saved_at).toLocaleString(LANG === "en" ? "en-GB" : "it"))}</span>` : ""}</p>
    <ul>${venues.map(v => `<li><span class="nt">${esc(v.name || v.auto || v.id || "")}</span> · ${(v.devices || []).length} <span>dispositivi</span>${(v.plan || []).length ? ` · <span>piano IP</span>` : ""}${(v.rdm || []).length ? ` · <span>patch RDM</span>` : ""}${v.id && v.id === curID ? ` · <b>è la rete attuale</b>` : ""}</li>`).join("")}</ul>
    ${p.notes || venues[0].notes ? `<div class="notice nt" style="display:block;white-space:pre-wrap">${esc(p.notes || venues[0].notes)}</div>` : ""}
    <p class="muted">Nomi, ruoli e note del file prendono il posto di quelli che hai già; i dispositivi critici e gli universi critici si aggiungono. Una rete dove questo computer non è mai stato viene preparata: quando arrivi lì e fai la scansione, ritrovi tutto.</p>
    <div class="row"><button class="primary" id="pjOwn">Apri</button>${canMerge ? `<button id="pjMerge">Unisci alla rete attuale</button>` : ""}<button id="pjNo">Annulla</button></div>`);
  $("pjNo").onclick = closeSheet;
  const go = async target => {
    try {
      const d = await post("api/project", {project: p, target}).then(r => r.json());
      closeSheet();
      const onCurrent = target === "" || (d.networks || []).includes(curID);
      toast(LANG === "en" ? `Project opened: ${d.devices} devices.` : `Progetto aperto: ${d.devices} dispositivi.` + (onCurrent ? "" : " Si attiva quando sei su quella rete."));
      if (onCurrent && !es) start();
    } catch (e) { toast("Non aperto: " + e.message); }
  };
  $("pjOwn").onclick = () => go("own");
  if ($("pjMerge")) $("pjMerge").onclick = () => go("");
}
async function exportVenue() {
  if (needOneNet()) return;
  try {
    const v = await fetch("api/venue").then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
    download(`wasaping-locale-${stamp()}.wasaping`, JSON.stringify(v, null, 2), "application/json");
  } catch (e) { toast("Non esportato: " + e.message); }
}
async function importVenue(text) {
  try {
    const d = await post("api/venue", JSON.parse(text)).then(r => r.json());
    toast(`Locale importato: ${d.devices} dispositivi. Rifaccio la scansione.`);
    if (!es) start();
  } catch (e) { toast("Non importato: " + e.message); }
}
document.addEventListener("keydown", e => { if (e.key === "Escape" && $("sheet").classList.contains("open")) closeSheet(); });

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

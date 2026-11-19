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
  const [bg, rem, upd, login, alerts, info] = await Promise.all([get("api/bg"), get("api/remote"), get("api/update"), get("api/login"), get("api/alerts"), get("api/info")]);
  const sel = (id, opts, v) => `<select id="${id}">${opts.map(([k, l]) => `<option value="${k}"${String(k) === String(v) ? " selected" : ""}>${esc(l)}</option>`).join("")}</select>`;
  openSheet("Impostazioni", `<div class="set">
    <b>Lingua</b><div class="opt">${sel("sLang", [["it", "Italiano"], ["en", "English"]], LANG)}</div>
    <b>Tema</b><div class="opt">${sel("sTheme", [["", "automatico"], ["light", "chiaro"], ["dark", "scuro"]], theme)}</div>
    <b>Ripeti la scansione</b><div class="opt">${sel("sRepeat", [[0, "mai"], [5, "ogni 5 minuti"], [15, "ogni 15 minuti"], [60, "ogni ora"]], bg.repeat_minutes || 0)}<span class="tag">Anche a finestra chiusa, con una notifica quando arriva un dispositivo nuovo.</span></div>
    <b>Allarmi verso altri programmi</b><div class="opt">
      <input id="sOSC" placeholder="OSC: 192.168.1.20:53000" value="${esc(alerts.osc || "")}" style="width:220px" spellcheck="false">
      <input id="sOSCAddr" placeholder="/wasaping" value="${esc(alerts.osc_address || "")}" style="width:140px" spellcheck="false">
      <input id="sHook" placeholder="Indirizzo web (webhook): https://…" value="${esc(alerts.webhook || "")}" style="flex:1 1 260px" spellcheck="false">
      <button id="sAlertSave">Salva</button><button id="sAlertTest">Prova</button>
      <span class="tag">Quando un dispositivo o un universo critico cade o torna, wasaping manda un messaggio OSC (a QLab, a una console, a un media server) e/o chiama l'indirizzo web con i dettagli in JSON.</span></div>
    <b>Monitor su iPad o telefono</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sRemote"${rem.on ? " checked" : ""}> attivo</label><span class="tag" id="sRemoteInfo">${rem.on ? `Apri <b>${esc(rem.url)}</b> e inserisci il PIN <b>${esc(rem.pin)}</b>.` : "Sola lettura, protetto da un PIN."}</span></div>
    <b>Aggiornamenti</b><div class="opt"><span>wasaping ${esc(upd.current || "")}</span>${upd.newer ? `<button class="primary" id="sUpdNow">Aggiorna a ${esc(upd.latest)}</button>` : ""}<label class="check" style="margin:0"><input type="checkbox" id="sUpdCheck"${upd.off ? "" : " checked"}> controlla le nuove versioni</label></div>
    ${login.available ? `<b>Apri al login</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sLogin"${login.enabled ? " checked" : ""}> apri wasaping all'accesso, nella barra dei menu</label></div>` : ""}
    ${info.helper && info.helper !== "notneeded" ? `<b>Indirizzi MAC (macOS)</b><div class="opt">${info.helper === "installed" ? '<span>aiuto di sistema attivo</span><button id="sHelperOff">Disinstalla</button>' : '<span>nascosti da macOS</span><button id="sHelperOn">Installa l\'aiuto</button>'}</div>` : ""}
    <b>Community SNMP</b><div class="opt"><input id="sSNMP" value="${esc(snmpCommunity)}" style="width:160px" spellcheck="false"><span class="tag">Per leggere dagli switch su quale porta è ogni dispositivo.</span></div>
  </div>`);
  $("sLang").onchange = () => { try { localStorage.setItem("wasaping.lang", $("sLang").value); } catch {} location.reload(); };
  $("sTheme").onchange = () => { theme = $("sTheme").value; try { localStorage.setItem("wasaping.theme", theme); } catch {} applyTheme(theme); };
  $("sRepeat").onchange = () => { $("repeat").value = $("sRepeat").value; $("repeat").onchange(); };
  $("sAlertSave").onclick = () => post("api/alerts", {osc: $("sOSC").value.trim(), osc_address: $("sOSCAddr").value.trim(), webhook: $("sHook").value.trim()}).then(() => toast("Allarmi salvati.")).catch(e => toast("Non salvato: " + e.message));
  $("sAlertTest").onclick = () => post("api/alerts", {osc: $("sOSC").value.trim(), osc_address: $("sOSCAddr").value.trim(), webhook: $("sHook").value.trim(), test: true}).then(r => r.json()).then(d => toast(d.result || "Inviato.")).catch(e => toast("Errore: " + e.message));
  $("sRemote").onchange = () => post("api/remote", {on: $("sRemote").checked}).then(r => r.json()).then(r => {
    $("sRemoteInfo").innerHTML = r.on ? `Apri <b>${esc(r.url)}</b> e inserisci il PIN <b>${esc(r.pin)}</b>.` : "Sola lettura, protetto da un PIN.";
  }).catch(e => { toast("Non attivato: " + e.message); $("sRemote").checked = false; });
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
      <div class="row"><button class="primary" id="plImport">Importa CSV</button><button id="plTemplate">Scarica un modello</button><button id="plFromNet" ${hosts.length ? "" : "disabled"}>Crea il piano da questa scansione</button></div>`);
  } else {
    openSheet("Piano IP", planHTML(pc));
    if ($("plApply")) $("plApply").onclick = () => applyPlanNames(pc);
    $("plClear").onclick = async () => { await post("api/plan", {clear: true}); planView(); };
  }
  $("plImport").onclick = () => pickFile(".csv,.txt", async text => {
    try { await post("api/plan", {csv: text}); planView(); } catch (e) { toast("Piano non importato: " + e.message); }
  });
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
  <div class="row"><button id="plImport">Importa un altro CSV</button>${scanned ? '<button id="plApply" title="Dà ai dispositivi trovati il nome e il ruolo del piano, se non ne hanno già uno">Usa nomi e ruoli del piano</button>' : ""}<button id="plClear" class="danger">Togli il piano</button></div>`;
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

// Live DMX: universes heard on Art-Net and sACN, with the levels of one.
let dmxES = null, dmxSel = "", dmxSess = "", critUnis = new Set();
function dmxView() {
  fetch("api/universe").then(r => r.json()).then(l => { critUnis = new Set(l || []); }).catch(() => {});
  openSheet("Universi DMX", `<p class="muted">In ascolto su Art-Net (pacchetti in broadcast o diretti a questo computer) e sACN (universi 1–64). Clic su un universo per vedere i 512 canali.</p>
    <div id="dmxBody"><p>In ascolto…</p></div><div id="dmxGrid"></div>`);
  dmxSess = Math.random().toString(36).slice(2);
  dmxES = new EventSource(`api/dmx?s=${dmxSess}&u=${encodeURIComponent(dmxSel)}`);
  dmxES.addEventListener("snap", e => drawDMX(JSON.parse(e.data)));
  dmxES.onerror = () => { if (dmxES) $("dmxBody").innerHTML = "<p>Ascolto interrotto.</p>"; };
}
function drawDMX(unis) {
  if (!$("dmxBody")) return;
  if (!unis.length) { $("dmxBody").innerHTML = "<p>Nessun universo in arrivo. Sulla rete non passa DMX, oppure la console manda Art-Net diretto ai nodi (unicast) e non a questo computer.</p>"; $("dmxGrid").innerHTML = ""; return; }
  const multi = u => u.sources.length > 1;
  $("dmxBody").innerHTML = `<div class="wrap" style="margin:0"><table><thead><tr><th>Universo</th><th>Protocollo</th><th>Sorgente</th><th>Frame/s</th><th>Canali attivi</th><th title="Il Monitor avvisa se l'universo smette di arrivare">Critico</th></tr></thead><tbody>
    ${unis.map(u => `<tr class="pick${u.id === dmxSel ? " sel" : ""}" data-id="${esc(u.id)}"><td><b>${u.number}</b></td><td>${esc(u.protocol)}</td>
      <td>${u.sources.map(x => `${esc(x.name || x.ip)}${x.name ? ` <span class="muted">${esc(x.ip)}</span>` : ""}${x.priority ? ` <span class="muted">prio ${x.priority}</span>` : ""}`).join("<br>")}${multi(u) ? ' <span class="badge warn">più sorgenti</span>' : ""}</td>
      <td>${u.sources.map(x => x.fps.toFixed(0)).join(" / ")}</td><td>${u.active} / 512</td><td><input type="checkbox" class="ucrit" data-id="${esc(u.id)}"${critUnis.has(u.id) ? " checked" : ""}></td></tr>`).join("")}
  </tbody></table></div>`;
  $("dmxBody").querySelectorAll("input.ucrit").forEach(c => {
    c.onclick = e => e.stopPropagation();
    c.onchange = () => post("api/universe", {id: c.dataset.id, critical: c.checked}).then(r => r.json()).then(l => {
      critUnis = new Set(l || []); toast(c.checked ? "Il Monitor ora controlla questo universo." : "Universo tolto dal Monitor.");
    }).catch(e => toast("Errore: " + e.message));
  });
  $("dmxBody").querySelectorAll("tr.pick").forEach(tr => tr.onclick = () => {
    dmxSel = tr.dataset.id;
    fetch(`api/dmx?s=${dmxSess}&u=${encodeURIComponent(dmxSel)}`, {method: "POST"});
  });
  const sel = unis.find(u => u.id === dmxSel && u.levels);
  $("dmxGrid").innerHTML = sel ? `<h3 style="font-size:14px;margin:16px 0 0">${esc(sel.protocol)} universo ${sel.number}</h3><div class="dmxgrid">${sel.levels.map((v, i) =>
    `<i title="canale ${i + 1}: ${v} (${Math.round(v / 2.55)}%)" style="background:${v ? `rgba(129,215,66,${0.15 + 0.85 * v / 255})` : ""}"></i>`).join("")}</div>` : "";
}

// Switch ports: which port of which managed switch each device is on.
function switchView() {
  openSheet("Porte switch", `<p>wasaping chiede agli switch gestiti (Luminex, Cisco, Netgear, Ubiquiti, MikroTik…) su quale porta si trova ogni dispositivo, via SNMP in sola lettura.</p>
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
        : `<p>${d.switches.map(x => `<b>${esc(x.name || x.ip)}</b> <span class="muted">${esc(x.ip)} · ${x.macs} indirizzi</span>`).join("<br>")}</p>
           <div class="wrap" style="margin:0"><table><thead><tr><th>Switch</th><th>Porta</th><th>Descrizione porta</th><th>Dispositivo</th><th>IP</th></tr></thead><tbody>
           ${rows.map(h => `<tr><td>${esc(h.swport.switch)}</td><td class="mono">${esc(h.swport.port)}</td><td class="nt">${esc(h.swport.alias || "")}</td><td class="nt"><b>${esc(label(h) || h.mac)}</b>${h.swport.shared > 1 ? ` <span class="muted" title="Su questa porta ci sono ${h.swport.shared} indirizzi: probabilmente un altro switch o un collegamento tra switch">· ${h.swport.shared} indirizzi</span>` : ""}</td><td class="mono">${esc(h.ip)}</td></tr>`).join("")}
           </tbody></table></div>`;
      render();
    } catch (e) { $("swOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("swGo").disabled = false;
  };
}

// RDM: the fixtures behind the Art-Net nodes, read-only.
function rdmView() {
  const nodes = hosts.filter(h => h.artnet).length;
  openSheet("Apparecchi RDM", `<p>wasaping chiede ai nodi Art-Net quali apparecchi RDM hanno collegati e a ognuno chi è: produttore, modello, etichetta, indirizzo DMX, canali e modalità. Solo lettura: non cambia niente sugli apparecchi.</p>
    <div class="row"><button class="primary" id="rdmGo" ${nodes ? "" : "disabled"}>Cerca apparecchi</button><span class="muted">${nodes ? `${nodes} nodi Art-Net nell'ultima scansione.` : "Nessun nodo Art-Net: fai prima una scansione."}</span></div>
    <div id="rdmOut"></div>`);
  $("rdmGo").onclick = async () => {
    $("rdmGo").disabled = true; $("rdmOut").innerHTML = "<p>Chiedo ai nodi… (può volerci qualche secondo per apparecchio)</p>";
    try {
      const d = await post("api/rdm", {}).then(r => r.json());
      const fx = d.fixtures || [];
      // overlapping DMX addresses on the same universe
      const clash = new Set();
      for (const a of fx) for (const b of fx) if (a !== b && a.universe === b.universe && a.address && b.address &&
        a.address <= b.address + Math.max(b.footprint, 1) - 1 && b.address <= a.address + Math.max(a.footprint, 1) - 1) { clash.add(a.uid); clash.add(b.uid); }
      const nodeName = ip => { const h = hosts.find(x => x.ip === ip); return h ? (label(h) || ip) : ip; };
      $("rdmOut").innerHTML = !fx.length ? "<p>Nessun apparecchio RDM ha risposto. Controlla che il nodo abbia l'RDM attivo sulle uscite.</p>" :
        `<div class="sum"><span class="pill ok">${fx.length} apparecchi</span>${clash.size ? `<span class="pill bad">⚠ ${clash.size} con indirizzi sovrapposti</span>` : ""}</div>
        <div class="wrap" style="margin:0"><table><thead><tr><th>Universo</th><th>Indirizzo</th><th>Canali</th><th>Etichetta</th><th>Produttore</th><th>Modello</th><th>Modalità</th><th>UID</th><th>Nodo</th><th></th></tr></thead><tbody>
        ${fx.map(f => `<tr${clash.has(f.uid) ? ' class="st-bad"' : ""}><td>${f.universe}</td><td class="mono">${f.address || "—"}${f.address && f.footprint > 1 ? "–" + (f.address + f.footprint - 1) : ""}</td><td>${f.footprint || ""}</td>
          <td class="nt"><b>${esc(f.label || "")}</b></td><td class="nt">${esc(f.maker || "")}</td><td class="nt">${esc(f.model || "")}</td><td>${esc(f.personality || "")}</td><td class="mono">${esc(f.uid)}</td><td class="nt">${esc(nodeName(f.node))}</td>
          <td><button class="ident" data-uid="${esc(f.uid)}" data-node="${esc(f.node)}" data-uni="${f.universe}">Identifica</button></td></tr>`).join("")}
        </tbody></table></div>`;
      $("rdmOut").querySelectorAll("button.ident").forEach(b => b.onclick = () => identify(b));
    } catch (e) { $("rdmOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("rdmGo").disabled = false;
  };
}
// RDM Identify: the only command wasaping sends to fixtures, so it asks once.
let identOK = false;
async function identify(b) {
  const on = !b.classList.contains("on");
  if (on && !identOK) {
    const note = document.createElement("div");
    note.className = "notice warn"; note.style.cssText = "display:flex;margin:10px 0";
    note.innerHTML = `<span>«Identifica» fa lampeggiare l'apparecchio (o lo muove, se è una testa mobile). È l'unico comando che wasaping invia agli apparecchi, e non cambia nessuna impostazione. Durante uno show, usalo solo se sei sicuro.</span><span style="display:flex;gap:8px"><button class="primary" id="identYes">Ho capito, identifica</button><button id="identNo">Annulla</button></span>`;
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

// The page: scanning, results, settings, the sheet. The tools are in network.js and show.js.
// Ports that the system knows how to open, and the program it uses.
const OPEN = {
  80: ["http", "browser"], 8000: ["http", "browser"], 8080: ["http", "browser"], 5000: ["http", "browser"], 32400: ["http", "browser"],
  443: ["https", "browser"], 8443: ["https", "browser"], 8006: ["https", "browser"], 8007: ["https", "browser"], 5001: ["https", "browser"], 9443: ["https", "browser"],
  8123: ["http", "browser"], 8096: ["http", "browser"],
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
// Departments of the Monitor (groups.go has the same table).
const DEPTS = {lights: "Luci", audio: "Audio", video: "Video", network: "Rete", other: "Altro"};
const KIND_GROUP = {lightdesk: "lights", lighting: "lights", audio: "audio", speaker: "audio", video: "video", projector: "video", camera: "video", tv: "video", router: "network", switch: "network"};
const groupOf = h => h.group || KIND_GROUP[h.kind] || "other";
const SERVICES = {"netaudio-arc":"Dante", "netaudio-cmc":"Dante", "netaudio-dbc":"Dante", "netaudio-chan":"Dante", ndi:"NDI", blackmagic:"Blackmagic", videohub:"Videohub", qlab:"QLab", osc:"OSC", oca:"AES70 (OCA)", "home-assistant":"Home Assistant", workstation:"", airplay:"AirPlay", raop:"AirPlay audio", ipp:"Stampa", ipps:"Stampa", printer:"Stampa", "pdl-datastream":"Stampa", scanner:"Scanner", uscan:"Scanner", smb:"Condivisione file", afpovertcp:"Condivisione file (AFP)", ssh:"SSH", "sftp-ssh":"SFTP", rfb:"Condivisione schermo", googlecast:"Chromecast", http:"Pagina web", https:"Pagina web", hap:"HomeKit", homekit:"HomeKit", matter:"Matter", matterc:"Matter", "companion-link":"Dispositivo Apple", "spotify-connect":"Spotify", sonos:"Sonos", "device-info":"", "sleep-proxy":"Sleep proxy", "apple-mobdev2":"Sincronizzazione Apple"};

let hosts = [], gone = [], es = null, sortKey = "ip", sortDir = 1, ui = "browser", showGone = true;
try { showGone = localStorage.getItem("wasaping.showGone") !== "0"; } catch {}
let repeatTimer = null, selected = null, autoRun = false, network = null, issues = [], snmpCommunity = "public", swPorts = {};
let networks = [], activeNet = ""; // several networks: "" = all tabs
// Preferences kept by the program (Settings → Scansione, Allarmi).
let prefs = {default_ports: "common", loop_pause: 10, alarm_volume: 60, alarm_sound: "beep", snmp_community: "public"};
// The port menu of the bar starts from the saved choice.
function applyDefaultPorts() {
  const p = prefs.default_ports || "common", custom = !["common", "none"].includes(p);
  $("ports").value = custom ? "custom" : p;
  $("custom").hidden = !custom;
  if (custom) $("custom").value = p;
}

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
// Server programs (databases, Docker, Proxmox…): name and version; red
// when it answers without a password.
function appChips(h) {
  return (h.apps || []).map(a => {
    const t = a.name + (a.version ? " " + a.version : "");
    const tip = t + (a.port ? " · porta " + a.port : "") + (a.no_auth ? " · senza password" : "");
    return `<span class="proto${a.no_auth ? " risky" : ""}" title="${esc(tip)}">${esc(t)}</span>`;
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
  routedNets = info.routed || [];
  if (routedNets.length) { // networks behind a VPN or a router
    const g = document.createElement("optgroup"); g.label = LANG === "en" ? "Reachable through a VPN or router" : "Raggiungibili via VPN o router";
    for (const r of routedNets) g.append(new Option(`${r.network} — ${r.label}`, r.network));
    $("iface").add(g);
  }
  $("iface").add(new Option("Personalizzato…", ""));
  $("target").value = info.defaultTarget;
  if (!info.hasOUI) $("oui").style.display = "flex";
  ui = info.ui || "browser";
  if (info.app && ui === "browser") {
    $("quit").hidden = false;
    const ping = () => fetch("api/ping", {method: "POST"}).catch(() => {});
    ping(); setInterval(ping, 10000);
  }
  try { const last = await fetch("api/last").then(r => r.json()); if (last && !hosts.length) applyResult(last); } catch {}
  setInterval(pollLast, 15000);
  snmpCommunity = info.snmp || "public";
  try { prefs = await fetch("api/prefs").then(r => r.json()); snmpCommunity = prefs.snmp_community; } catch {}
  applyDefaultPorts(); applyLook(); fillRanges(); showPublicIP(); markVia();
  // Back where the page was before a reload (language change).
  let reopen = ""; try { reopen = sessionStorage.getItem("wasaping.reopen") || ""; sessionStorage.removeItem("wasaping.reopen"); } catch {}
  if (reopen) runTool(reopen);
  checkUpdate(); setInterval(checkUpdate, 3600000);
  let dismissed = false;
  try { dismissed = localStorage.getItem("wasaping.helperNo") === "1"; } catch {}
  if (info.helper === "missing" && !dismissed) $("machidden").style.display = "flex";
}

// macOS ARP helper: a system service installed once with the password.
async function helper(what) {
  $("status").textContent = what === "install" ? "Inserisci la password nella finestra di macOS…" : "Rimuovo l'aiuto…";
  const r = await fetch(`api/helper?do=${what}`, {method: "POST"});
  if (r.ok) {
    const d = await r.json();
    if (d.helper === "installed") $("machidden").style.display = "none";
    $("status").textContent = what === "install" ? "✓ Aiuto installato: rifai la scansione per vedere i MAC." : "Aiuto rimosso.";
  } else {
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
const setRepeat = min => post("api/bg", {repeat_minutes: min})
  .then(() => toast(min ? "wasaping ripeterà la scansione da solo, anche a finestra chiusa." : "Ripetizione automatica spenta."))
  .catch(e => toast("Non salvato: " + e.message));

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
  else if (/^[1-9]$/.test(k) && !e.shiftKey) { // ⌘1…⌘9: the pages of the header, in order
    const b = $("quick").querySelectorAll("button")[+k - 1];
    if (b) { e.preventDefault(); b.click(); }
  }
  else if (k === "f") { e.preventDefault(); wasapingFind(); }
});

// Theme: automatic (follows the system), light, dark.
function applyTheme(t) {
  if (t) document.documentElement.dataset.theme = t; else delete document.documentElement.dataset.theme;
}
let theme = "";
try { theme = localStorage.getItem("wasaping.theme") || ""; } catch {}
applyTheme(theme);

$("scan").onclick = () => { if (es) { setLoop(false); stop("Scansione interrotta."); } else start(); };
// Clear the list on screen (the archive keeps names, notes and history).
$("clearBtn").onclick = () => {
  if (es) { setLoop(false); stop(""); }
  setLoop(false);
  hosts = []; gone = []; freshKeys = new Set(); issues = []; network = null; networks = []; activeNet = "";
  closeDetail(); showNetwork();
  $("extra").innerHTML = ""; $("status").textContent = "Pronto."; $("prog").style.width = "0";
  $("rows").innerHTML = `<tr><td colspan="10" class="empty">Premi <b>Scansiona</b> per vedere chi c'è sulla tua rete.</td></tr>`;
  shownKeys = []; $("fcount").textContent = ""; pageTitle(); refreshGroupPage();
};

// Continuous scan: while the window is open, scan again a few seconds after
// each scan ends. The list stays on screen and is swapped when the new
// scan is done; arrivals and departures are announced.
let loop = false, loopTimer = null, loopNext = 0;
function setLoop(on) {
  loop = on;
  clearTimeout(loopTimer); clearInterval(setLoop.tick);
  $("loopBtn").setAttribute("aria-pressed", on ? "true" : "false");
  $("loopBtn").classList.remove("busy");
  if (on && !es) start(false, !!hosts.length);
}
$("loopBtn").onclick = () => {
  unlockSound(); // sound needs a click first
  setLoop(!loop);
  toast(loop ? "Scansione continua: l'elenco si aggiorna da solo." : "Scansione continua spenta.");
};
function loopAgain(msg) {
  if (!loop) return;
  const pause = prefs.loop_pause || 10; // seconds between two scans
  loopNext = Date.now() + pause * 1000;
  const show = () => { const n = Math.max(0, Math.round((loopNext - Date.now()) / 1000)); $("status").textContent = `${msg} · di nuovo tra ${n} s`; };
  show(); setLoop.tick = setInterval(show, 1000);
  const go = () => {
    clearInterval(setLoop.tick);
    if (!loop || es) return;
    if (document.hidden) { loopTimer = setTimeout(go, 2000); return; } // window closed: wait
    start(false, true);
  };
  loopTimer = setTimeout(go, pause * 1000);
}
// What changed since the previous scan, said in a toast and in the title.
function announce(before, d) {
  const name = h => label(h) || h.ip;
  const was = new Set(before.map(h => h.key));
  const came = hosts.filter(h => !was.has(h.key));
  const lost = before.filter(h => !hosts.some(x => x.key === h.key));
  freshKeys = new Set(came.map(h => h.key));
  setTimeout(() => { freshKeys = new Set(); }, 3500); // the highlight is played once
  const back = came.filter(h => !h.new);
  const nu = came.filter(h => h.new);
  if (nu.length) toast(`${nu.length === 1 ? "Nuovo" : "Nuovi"}: ${nu.slice(0, 4).map(name).join(", ")}${nu.length > 4 ? "…" : ""}`);
  else if (back.length) toast(`Tornato: ${back.slice(0, 4).map(name).join(", ")}${back.length > 4 ? "…" : ""}`);
  const critLost = lost.filter(h => h.critical);
  if (critLost.length) alarm(critLost.map(name)); // notification, toast and sound
}
let freshKeys = new Set();

function schedule() {} // repeats are done by the program (backgroundLoop)

// quiet: a repeat of the continuous scan, the current list stays on
// screen until the new one is complete.
function start(auto = false, quiet = false) {
  const target = $("target").value.trim();
  if (!target) return;
  autoRun = auto;
  clearTimeout(loopTimer); clearInterval(setLoop.tick);
  const before = hosts, incoming = [];
  if (!quiet) { hosts = []; gone = []; freshKeys = new Set(); }
  clearTimeout(repeatTimer);
  const ports = $("ports").value === "custom" ? ($("custom").value.trim() || "none") : $("ports").value;
  $("scan").textContent = "Stop"; $("scan").classList.add("stop");
  $("loopBtn").classList.toggle("busy", loop);
  $("status").textContent = quiet ? `Aggiorno ${target}… (scansione continua)` : `Scansione di ${target}…`;
  if (!quiet) $("extra").innerHTML = "";
  $("prog").style.width = "0";
  es = new EventSource(`api/scan?target=${encodeURIComponent(target)}&ports=${encodeURIComponent(ports)}${auto ? "&auto=1" : ""}`);
  es.quiet = quiet;
  render();
  es.addEventListener("progress", e => {
    const p = JSON.parse(e.data);
    $("prog").style.width = (100 * p.done / p.total) + "%";
    if (!quiet) $("status").textContent = `Sondaggio ${p.done}/${p.total}… ${p.done === p.total ? "raccolgo i dettagli" : ""}`;
  });
  es.addEventListener("host", e => {
    const h = JSON.parse(e.data);
    if (h.mac && swPorts[h.mac]) h.swport = swPorts[h.mac];
    if (quiet) incoming.push(h); else { hosts.push(h); renderSoon(); }
  });
  es.addEventListener("done", e => {
    const d = JSON.parse(e.data);
    if (quiet) { hosts = incoming; announce(before, d); }
    finish(d, quiet ? `${d.count} dispositivi · aggiornato alle ${new Date().toLocaleTimeString("it", {hour: "2-digit", minute: "2-digit", second: "2-digit"})}` : "");
  });
  es.addEventListener("failure", e => { const m = "Errore: " + JSON.parse(e.data).error; stop(m); loopAgain(m); });
  es.onerror = () => { if (es) { const q = es.quiet; stop("Connessione interrotta: il target è valido?"); if (q) loopAgain("Connessione interrotta"); else setLoop(false); } };
}

function finish(d, msg) {
    shownAt = d.at || shownAt;
    gone = d.gone || [];
    network = d.network || null; issues = d.issues || [];
    networks = d.networks && d.networks.length ? d.networks : (network ? [network] : []);
    if (!networks.some(n => n.id === activeNet)) activeNet = "";
    showNetwork();
    let no = false; try { no = localStorage.getItem("wasaping.helperNo") === "1"; } catch {}
    $("machidden").style.display = d.macHidden && !no ? "flex" : "none";
    if (d.macHidden) $("oui").style.display = "none"; // no MACs, no vendors to look up
    stop(msg || `${d.count} dispositivi trovati in ${d.seconds.toFixed(1)} s.`);
    lastScanAt = d.at ? new Date(d.at).getTime() : Date.now();
    const bits = [];
    if (d.new) bits.push(`<b>${d.new} ${d.new === 1 ? "nuovo" : "nuovi"}</b>`);
    if (gone.length) bits.push(`<a id="toggleGone">${gone.length} non ${gone.length === 1 ? "risponde" : "rispondono"} (${showGone ? "nascondi" : "mostra"})</a>`);
    $("extra").innerHTML = bits.join(" · ");
    if ($("toggleGone")) $("toggleGone").onclick = () => { showGone = !showGone; try { localStorage.setItem("wasaping.showGone", showGone ? "1" : "0"); } catch {} $("toggleGone").textContent = `${gone.length} non ${gone.length === 1 ? "risponde" : "rispondono"} (${showGone ? "nascondi" : "mostra"})`; render(); };
    render();
    if (selected) openDetail(selected);
    pageTitle(); refreshGroupPage();
    if (!d.auto) loopAgain($("status").textContent);
}
// The window title says how many devices there are, and warns when a
// critical one is missing (useful with the window in the background).
function pageTitle() {
  const down = gone.filter(h => h.critical).length;
  document.title = `${down ? "⚠ " : ""}wasaping · ${hosts.length}${down ? ` · ${down} ✗` : ""}`;
}

function stop(msg) {
  if (es) { es.close(); es = null; }
  $("scan").textContent = "Scansiona"; $("scan").classList.remove("stop");
  $("prog").style.width = "100%";
  $("status").textContent = msg;
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
    nb.innerHTML = tabs + `${/^Rete /.test(name) ? "" : "Rete: "}<b>${esc(name)}</b>${network.name ? ` <span class="muted">· ${esc(network.auto)}</span>` : ""}${network.is_new ? ' <span class="badge new">rete nuova</span>' : ""} <a id="netRename">${network.name ? "rinomina" : "dai un nome a questo locale"}</a>
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
  // Two at most on screen; the others behind "show all".
  const more = issues.length > 2 && !showNetwork.all;
  box.innerHTML = (more ? issues.slice(0, 2) : issues).map(i => `<span>⚠️ ${esc(i)}</span>`).join("") +
    (more ? `<a class="more">${LANG === "en" ? `+${issues.length - 2} more` : `altri ${issues.length - 2}`}</a>` : "");
  const m = box.querySelector("a.more");
  if (m) m.onclick = () => { showNetwork.all = true; showNetwork(); };
}

// New version on GitHub (the program checks every 3 hours): a banner on
// the home page with what is new, until "Later" (one day) or the update.
let updLast = null;
async function checkUpdate() {
  try {
    const u = updLast = await fetch("api/update").then(r => r.json());
    $("setBtn").classList.toggle("dot", !!u.newer);
    $("setBtn").title = u.newer ? `Impostazioni · è uscita wasaping ${u.latest}` : "Impostazioni";
    let later = null;
    try { later = JSON.parse(localStorage.getItem("wasaping.updLater") || "null"); } catch {}
    if (!u.newer || (later && later.v === u.latest && Date.now() < later.until)) { $("updBar").style.display = "none"; return; }
    $("updText").innerHTML = `È uscita <b>wasaping ${esc(String(u.latest).replace(/^v/, ""))}</b> <span class="muted">(hai la ${esc(String(u.current).replace(/^v/, ""))})</span>`;
    $("updNews").innerHTML = (u.news || []).length ? `<span>Novità:</span> <span class="nt">${(u.news || []).map(esc).join(" · ")}</span>` : "";
    $("updNow").textContent = u.can_install ? "Aggiorna ora" : "Scarica";
    $("updNow").title = u.can_install ? "Scarica, controlla e installa la nuova versione, poi riapre wasaping" : "Apre la pagina della versione su GitHub";
    $("updBar").style.display = "flex";
  } catch {}
}
$("updNow").onclick = () => { if (!updLast) return; if (updLast.can_install) installUpdate(); else openURL(updLast.url); };
$("updNotes").onclick = () => { if (updLast && updLast.url) openURL(updLast.url); };
$("updLater").onclick = () => {
  try { localStorage.setItem("wasaping.updLater", JSON.stringify({v: updLast && updLast.latest, until: Date.now() + 86400000})); } catch {}
  $("updBar").style.display = "none"; toast("Te lo ricordo domani. Intanto è in ⚙ Impostazioni → Sistema.");
};
$("updOff").onclick = () => post("api/update", {off: true}).then(() => { $("updBar").style.display = "none"; $("setBtn").classList.remove("dot"); toast("Non controllerò più le nuove versioni."); });

// ---- Settings: language, theme, repeat, alarms, remote, updates… ----
$("setBtn").onclick = () => runTool("settings");
async function settingsView() {
  const get = u => fetch(u).then(r => r.json()).catch(() => ({}));
  const [bg, upd, login, alerts, info, wa, pr, osc] = await Promise.all([get("api/bg"), get("api/update"), get("api/login"), get("api/alerts"), get("api/info"), get("api/webaccess"), get("api/prefs"), get("api/oscin")]);
  const oscInfo = o => o.on ? `In ascolto su <b class="nt">${esc((o.ips || [])[0] || "questo computer")}</b>, porta <b>${o.port}</b> (UDP). Comandi: <span class="mono nt" style="font-size:11px">${(o.commands || []).map(esc).join(" · ")}</span>${o.last ? `<br>Ultimo comando: <span class="mono nt">${esc(o.last)}</span> alle ${esc(new Date(o.last_at).toLocaleTimeString("it"))}` : ""}` : "Da QLab, da una console luci o da uno Stream Deck: avvia il Monitor, segna inizio e fine dello show, silenzia gli allarmi, fa il pre-show. Nessun comando cambia gli apparecchi.";
  if (pr.default_ports) prefs = pr;
  const waInfo = w => w.on ? `Da un altro computer apri <b class="nt">${esc(w.url)}</b> e inserisci la password. Il browser avviserà che il certificato non è verificato: è normale, l'ha creato wasaping. Impronta SHA-256: <span class="mono nt" style="font-size:11px;word-break:break-all">${esc(w.fingerprint || "")}</span>` : "Tutta l'interfaccia, anche le modifiche, da un altro computer: HTTPS e password.";
  const sel = (id, opts, v) => `<select id="${id}">${opts.map(([k, l]) => `<option value="${k}"${String(k) === String(v) ? " selected" : ""}>${esc(l)}</option>`).join("")}</select>`;
  const portMode = ["common", "none"].includes(prefs.default_ports) ? prefs.default_ports : "custom";
  const kb = n => n < 1024 ? `${n} B` : n < 1048576 ? `${Math.round(n / 1024)} KB` : `${(n / 1048576).toFixed(1)} MB`;
  const SECTS = [["sGen", "Generali"], ["sScan", "Scansione"], ["sAl", "Allarmi"], ["sAcc", "Rete e accesso"], ["sSys", "Sistema"]];
  openSheet("Impostazioni", `<nav class="setidx">${SECTS.map(([id, l]) => `<a data-to="${id}">${l}</a>`).join("")}</nav><div class="set">
    <h4 id="sGen">Generali</h4>
    <b>Lingua</b><div class="opt">${sel("sLang", [["it", "Italiano"], ["en", "English"]], LANG)}</div>
    <b>Tema</b><div class="opt">${sel("sTheme", [["", "automatico"], ["light", "chiaro"], ["dark", "scuro"]], theme)}</div>
    <b>Dimensione del testo</b><div class="opt">${sel("sTextSize", [[80, "80%"], [90, "90%"], [100, "normale"], [110, "110%"], [125, "125%"], [150, "150%"], [175, "175%"]], prefs.text_size || 100)}<span class="tag">Anche con ⌘+ e ⌘− (Ctrl su Windows e Linux), ⌘0 per tornare normale.</span></div>
    <b>Colori</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sCB"${prefs.colorblind ? " checked" : ""}> per daltonici</label><span class="tag">Blu per «tutto bene» e arancione per «problema», al posto di verde e rosso.</span></div>
    <b>Intervalli salvati</b><div class="opt"><span id="sRanges">${(prefs.saved_ranges || []).length ? prefs.saved_ranges.map(r => `<span class="pill"><b class="nt">${esc(r.name)}</b> <span class="mono">${esc(r.target)}</span></span>`).join(" ") : "nessuno"}</span><span class="tag">Si salvano con ☆ accanto al campo dell'intervallo e si ritrovano nel menu delle interfacce.</span></div>

    <h4 id="sScan">Scansione</h4>
    <b>Porte da controllare</b><div class="opt">${sel("sPorts", [["common", "Porte comuni"], ["none", "Nessuna porta (più veloce)"], ["custom", "Porte personalizzate…"]], portMode)}
      <input id="sPortsCustom" placeholder="22,80,443 o 1-1024" value="${portMode === "custom" ? esc(prefs.default_ports) : ""}" style="width:200px" spellcheck="false"${portMode === "custom" ? "" : " hidden"}>
      <span class="tag">Quelle scelte all'apertura di wasaping. Per una scansione sola si cambiano nella barra.</span></div>
    <b>Scansione continua</b><div class="opt">${sel("sPause", [[5, "pausa di 5 s"], [10, "pausa di 10 s"], [30, "pausa di 30 s"], [60, "pausa di 1 minuto"]], prefs.loop_pause)}<span class="tag">Quanto aspetta «↻ Continua» tra una scansione e l'altra.</span></div>
    <b>Ripeti in background</b><div class="opt">${sel("sRepeat", [[0, "mai"], [5, "ogni 5 minuti"], [15, "ogni 15 minuti"], [60, "ogni ora"]], bg.repeat_minutes || 0)}<span class="tag">Anche a finestra chiusa, sulla rete predefinita, con una notifica quando arriva un dispositivo nuovo.</span></div>
    <b>Community SNMP</b><div class="opt"><input id="sSNMP" value="${esc(prefs.snmp_community)}" style="width:160px" spellcheck="false"><span class="tag">Per leggere dagli switch su quale porta è ogni dispositivo (Mappa della rete).</span></div>

    <h4 id="sAl">Allarmi</h4>
    <p class="muted sethint">Quando un dispositivo o un universo critico cade o torna, wasaping avvisa dove scegli qui. Si salva da solo.</p>
    <b>Allarme sonoro</b><div class="opt"><input type="range" id="sVol" min="10" max="100" step="10" value="${prefs.alarm_volume}" style="width:180px"><span id="sVolN" class="tag">${prefs.alarm_volume}%</span>${sel("sSound", [["beep", "bip"], ["moo", "mucca 🐄"]], prefs.alarm_sound || "beep")}<button id="sVolTest">Prova il suono</button><span class="tag">Acceso o spento si sceglie nel Monitor.</span></div>
    <b>Altri programmi</b><div class="opt">
      <input id="sOSC" class="al" placeholder="OSC: 192.168.1.20:53000" value="${esc(alerts.osc || "")}" style="width:220px" spellcheck="false">
      <input id="sOSCAddr" class="al" placeholder="/wasaping" value="${esc(alerts.osc_address || "")}" style="width:140px" spellcheck="false">
      <input id="sHook" class="al" placeholder="Indirizzo web (webhook): https://…" value="${esc(alerts.webhook || "")}" style="flex:1 1 260px" spellcheck="false">
      <span class="tag">Un messaggio OSC (a QLab, a una console, a un media server) e/o una chiamata all'indirizzo web con i dettagli in JSON.</span></div>
    <b>Notifiche sul telefono</b><div class="opt">
      <input id="sNtfy" class="al" placeholder="ntfy: nome dell'argomento" value="${esc(alerts.ntfy || "")}" style="width:240px" spellcheck="false">
      <input id="sTGToken" class="al" type="password" autocomplete="off" placeholder="Telegram: token del bot" value="${esc((alerts.telegram || "").split("|")[0])}" style="width:240px" spellcheck="false">
      <input id="sTGChat" class="al" placeholder="id della chat" value="${esc((alerts.telegram || "").split("|")[1] || "")}" style="width:130px" spellcheck="false">
      <input id="sSlack" class="al" placeholder="Slack: https://hooks.slack.com/…" value="${esc(alerts.slack || "")}" style="flex:1 1 260px" spellcheck="false">
      <span class="tag">Il modo più semplice è ntfy: installa l'app ntfy sul telefono, iscriviti a un argomento con un nome difficile da indovinare e scrivi lo stesso nome qui. Per Telegram: crea un bot con @BotFather e scrivi qui il suo token e l'id della chat. Per Slack: un «incoming webhook» del canale.</span></div>
    <b></b><div class="opt"><button id="sAlertTest">Manda una prova</button><span class="tag" id="sAlertMsg"></span></div>

    <h4 id="sAcc">Rete e accesso</h4>
    <b>IP pubblico</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sPub"${prefs.public_ip ? " checked" : ""}> mostra IP pubblico, provider e paese</label><span class="tag">Lo chiede a ipinfo.io (un servizio su internet) ogni 10 minuti: per questo è spento finché non lo accendi.</span></div>
    ${info.remote ? `<b>Accesso dalla rete</b><div class="opt"><span>Sei collegato da un altro computer.</span><form method="post" action="/logout" style="margin:0"><button>Disconnetti</button></form></div>` : `<b>Accesso completo dalla rete</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sWeb"${wa.on ? " checked" : ""}> attivo</label>
      <input type="password" id="sWebPw" placeholder="${wa.has_password ? "nuova password (facoltativa)" : "password (almeno 8 caratteri)"}" autocomplete="new-password" style="width:230px"><button id="sWebPwSave">Salva password</button>
      <span class="tag" id="sWebInfo">${waInfo(wa)}</span><img id="sWebQR" class="qr" alt=""${wa.on ? ` src="api/qr?for=web&t=${Date.now()}"` : " hidden"}></div>
    <b>Comandi OSC in ingresso</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sOSCIn"${osc.on ? " checked" : ""}> attivi</label>
      <input id="sOSCPort" value="${osc.port || 53001}" style="width:90px" inputmode="numeric" title="Porta UDP"><span class="tag" id="sOSCInfo">${oscInfo(osc)}</span></div>`}

    <h4 id="sSys">Sistema</h4>
    <b>Aggiornamenti</b><div class="opt"><span>wasaping ${esc(upd.current || "")}</span>${upd.newer ? `<button class="primary" id="sUpdNow">Aggiorna a ${esc(upd.latest)}</button>` : ""}<label class="check" style="margin:0"><input type="checkbox" id="sUpdCheck"${upd.off ? "" : " checked"}> controlla le nuove versioni</label><button id="sUpdNowCheck">Controlla ora</button><span class="tag" id="sUpdInfo"></span></div>
    ${login.available ? `<b>Apri al login</b><div class="opt"><label class="check" style="margin:0"><input type="checkbox" id="sLogin"${login.enabled ? " checked" : ""}> apri wasaping all'accesso, nella barra dei menu</label></div>` : ""}
    ${info.helper && info.helper !== "notneeded" ? `<b>Indirizzi MAC (macOS)</b><div class="opt">${info.helper === "installed" ? '<span>aiuto di sistema attivo</span><button id="sHelperOff">Disinstalla</button>' : '<span>nascosti da macOS</span><button id="sHelperOn">Installa l\'aiuto</button>'}</div>` : ""}
    <b>Dati di wasaping</b><div class="opt"><span class="mono nt" style="font-size:12px;word-break:break-all">${esc(prefs.data_dir || "")}</span>${info.remote ? "" : '<button id="sReveal">Mostra nella cartella</button>'}
      <span class="tag">${(prefs.files || []).map(f => `<span class="nt">${esc(f.name)}</span> ${kb(f.size)}`).join(" · ")}</span></div>
    <b>Impostazioni</b><div class="opt"><button id="sExport">Esporta…</button><button id="sImport">Importa…</button><span class="tag">Allarmi, notifiche, scansione e community in un file, da portare su un altro computer. Senza PIN e password.</span></div>
  </div>`);
  $("shBody").querySelectorAll(".setidx a").forEach(a => a.onclick = () => $("sheet").scrollTo({top: $(a.dataset.to).getBoundingClientRect().top - $("shBody").getBoundingClientRect().top + $("shBody").offsetTop - 70}));
  const savePrefs = body => post("api/prefs", body).then(r => r.json()).then(p => { prefs = p; snmpCommunity = p.snmp_community; return p; });
  $("sLang").onchange = () => {
    try { localStorage.setItem("wasaping.lang", $("sLang").value); sessionStorage.setItem("wasaping.reopen", "settings"); } catch {}
    location.reload();
  };
  $("sTextSize").onchange = () => setTextSize(+$("sTextSize").value);
  $("sCB").onchange = () => savePrefs({colorblind: $("sCB").checked}).then(() => { applyLook(); toast("Salvato."); }).catch(e => toast("Non salvato: " + e.message));
  $("sPub").onchange = () => savePrefs({public_ip: $("sPub").checked}).then(() => { showPublicIP(); toast("Salvato."); }).catch(e => toast("Non salvato: " + e.message));
  $("sTheme").onchange = () => { theme = $("sTheme").value; try { localStorage.setItem("wasaping.theme", theme); } catch {} applyTheme(theme); };
  const portsChanged = () => {
    const custom = $("sPorts").value === "custom";
    $("sPortsCustom").hidden = !custom;
    const v = custom ? $("sPortsCustom").value.trim() : $("sPorts").value;
    if (custom && !v) { $("sPortsCustom").focus(); return; }
    savePrefs({default_ports: v}).then(() => { applyDefaultPorts(); toast("Porte salvate."); }).catch(e => toast("Non salvato: " + e.message));
  };
  $("sPorts").onchange = portsChanged; $("sPortsCustom").onchange = portsChanged;
  $("sPause").onchange = () => savePrefs({loop_pause: +$("sPause").value}).then(() => toast("Salvato.")).catch(e => toast("Non salvato: " + e.message));
  $("sRepeat").onchange = () => setRepeat(+$("sRepeat").value);
  $("sSNMP").onchange = () => savePrefs({snmp_community: $("sSNMP").value.trim()}).then(() => toast("Salvato.")).catch(e => toast("Non salvato: " + e.message));
  $("sVol").oninput = () => { $("sVolN").textContent = $("sVol").value + "%"; };
  $("sVol").onchange = () => savePrefs({alarm_volume: +$("sVol").value}).then(() => playAlarm()).catch(e => toast("Non salvato: " + e.message));
  $("sVolTest").onclick = () => { unlockSound(); playAlarm(); };
  $("sSound").onchange = () => { unlockSound(); savePrefs({alarm_sound: $("sSound").value}).then(() => playAlarm()).catch(e => toast("Non salvato: " + e.message)); };
  // Alarms save themselves when a field is left.
  const alertForm = () => ({osc: $("sOSC").value.trim(), osc_address: $("sOSCAddr").value.trim(), webhook: $("sHook").value.trim(), ntfy: $("sNtfy").value.trim(), slack: $("sSlack").value.trim(),
    telegram: $("sTGToken").value.trim() || $("sTGChat").value.trim() ? $("sTGToken").value.trim() + "|" + $("sTGChat").value.trim() : ""});
  const alertMsg = (t, bad) => { $("sAlertMsg").textContent = t; $("sAlertMsg").classList.toggle("bad", !!bad); };
  const saveAlerts = () => {
    const tg = [$("sTGToken").value.trim(), $("sTGChat").value.trim()];
    if (!tg[0] !== !tg[1]) { alertMsg("Telegram: servono sia il token sia l'id della chat."); return; }
    post("api/alerts", alertForm()).then(r => r.json()).then(d => { $("sNtfy").value = d.ntfy || ""; alertMsg("✓ Salvato."); }).catch(e => alertMsg("Non salvato: " + e.message, true));
  };
  $("shBody").querySelectorAll("input.al").forEach(i => i.onchange = saveAlerts);
  $("sAlertTest").onclick = () => post("api/alerts", {...alertForm(), test: true}).then(r => r.json()).then(d => alertMsg(d.result || "Niente da provare: scrivi almeno una destinazione.")).catch(e => alertMsg("Errore: " + e.message, true));
  const webSet = body => post("api/webaccess", body).then(r => r.json()).then(w => {
    $("sWeb").checked = w.on; $("sWebInfo").innerHTML = waInfo(w); $("sWebPw").value = "";
    $("sWebQR").hidden = !w.on; if (w.on) $("sWebQR").src = `api/qr?for=web&t=${Date.now()}`;
    return w;
  });
  if ($("sOSCIn")) {
    const oscSet = () => post("api/oscin", {port: $("sOSCIn").checked ? +$("sOSCPort").value : 0}).then(r => r.json()).then(o => { $("sOSCIn").checked = o.on; $("sOSCInfo").innerHTML = oscInfo(o); })
      .catch(e => { toast("Non attivato: " + e.message); $("sOSCIn").checked = false; });
    $("sOSCIn").onchange = oscSet;
    $("sOSCPort").onchange = () => { if ($("sOSCIn").checked) oscSet(); };
  }
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
  if ($("sReveal")) $("sReveal").onclick = () => post("api/prefs/reveal").catch(e => toast("Errore: " + e.message));
  $("sExport").onclick = async () => {
    try { download(`wasaping-impostazioni-${stamp()}.json`, JSON.stringify(await fetch("api/prefs/export").then(r => r.json()), null, 2), "application/json"); }
    catch (e) { toast("Non esportato: " + e.message); }
  };
  $("sImport").onclick = () => pickFile(".json", async text => {
    try {
      await post("api/prefs/import", JSON.parse(text));
      toast("Impostazioni importate."); applyDefaultPorts(); settingsView();
    } catch (e) { toast("Non importato: " + (e instanceof SyntaxError ? "il file non è JSON" : e.message)); }
  });
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
// A tool is a page like the Monitor, above it if the Monitor is open.
function openSheet(title, html) {
  const fresh = !$("sheet").classList.contains("open") || $("shTitle").textContent !== title;
  $("shTitle").textContent = title; $("shBody").innerHTML = html;
  $("sheet").classList.add("open"); $("sheet").setAttribute("aria-hidden", "false");
  if (fresh) $("sheet").scrollTop = 0;
  placeNet(); if (typeof markQuick === "function") markQuick();
}
function closeSheet() {
  $("sheet").classList.remove("open"); $("sheet").setAttribute("aria-hidden", "true");
  if (dmxES) { dmxES.close(); dmxES = null; }
  currentTool = "";
  placeNet(); if (typeof markQuick === "function") markQuick();
}
// The network row and its warnings follow the page on screen: the open
// tool, the Monitor, or the device list. Same elements, links keep working.
function placeNet() {
  const where = $("sheet").classList.contains("open") ? $("shNet") : $("monitor").classList.contains("open") ? $("mNet") : null;
  if (where) where.append($("netbar"), $("issues"));
  else { $("statusRow").append($("netbar")); $("oui").before($("issues")); } // the list: network on the status row
}
let currentTool = "";
function pickFile(accept, then) {
  const f = $("fileIn"); f.accept = accept; f.value = "";
  f.onchange = () => { const file = f.files[0]; if (!file) return; const r = new FileReader(); r.onload = () => then(String(r.result), file.name); r.readAsText(file); };
  f.click();
}

function needOneNet() {
  if (networks.length > 1 && !activeNet) { toast("Scegli prima una rete dalle schede."); return true; }
  return false;
}
// 2 = a problem that breaks things (duplicate IP), 1 = other warnings.
const warnLevel = h => !(h.warnings || []).length ? 0 : h.warnings.some(w => /^(IP doppio|Conflitto di IP)/.test(w)) ? 2 : 1;
function sortValue(h, k) {
  switch (k) {
    case "protocols": return (h.protocols || []).join(" ").toLowerCase();
    case "apps": return (h.apps || []).map(a => a.name).join(" ").toLowerCase();
    case "model": return (h.model || h.vendor || "").toLowerCase();
    case "ip": return ipNum(h.ip);
    case "ports": return (h.ports || []).length;
    case "rtt_ms": return h.rtt_ms || 0;
    case "critical": return h.critical ? 0 : 1;
    case "warn": return -warnLevel(h) * 100 - (h.warnings || []).length;
    case "label": return label(h).toLowerCase();
    default: return (h[k] || "").toLowerCase();
  }
}

// ---- Look: text size and colour-blind colours (Settings → Generali) ----
function applyLook() {
  document.documentElement.style.zoom = (prefs.text_size || 100) === 100 ? "" : String((prefs.text_size || 100) / 100);
  document.documentElement.classList.toggle("cb", !!prefs.colorblind);
}
let textSizeT = 0;
function setTextSize(v) {
  prefs.text_size = Math.min(175, Math.max(70, v)); applyLook();
  toast(`Testo al ${prefs.text_size}%`);
  clearTimeout(textSizeT); textSizeT = setTimeout(() => post("api/prefs", {text_size: prefs.text_size}).catch(() => {}), 600);
  if ($("sTextSize")) $("sTextSize").value = String(prefs.text_size);
}
// ⌘+ / ⌘− / ⌘0 (Ctrl on Windows and Linux): bigger, smaller, normal text.
document.addEventListener("keydown", e => {
  if (!(e.metaKey || e.ctrlKey) || e.altKey) return;
  const steps = [70, 80, 90, 100, 110, 125, 150, 175], cur = prefs.text_size || 100;
  if (e.key === "+" || e.key === "=") { e.preventDefault(); setTextSize(steps.find(s => s > cur) || 175); }
  else if (e.key === "-" || e.key === "_") { e.preventDefault(); setTextSize([...steps].reverse().find(s => s < cur) || 70); }
  else if (e.key === "0") { e.preventDefault(); setTextSize(100); }
});

// ---- Networks reached through a VPN or a router ----
let routedNets = [];
const ipOf = s => { const m = /^(\d+\.\d+\.\d+\.\d+)/.exec((s || "").trim()); return m ? ipNum(m[1]) : null; };
function routeOf(target) {
  const ip = ipOf(target); if (ip === null) return null;
  return routedNets.find(r => { const [a, b] = r.network.split("/"); const size = 2 ** (32 - +b), base = ipNum(a); return ip >= base && ip < base + size; }) || null;
}
// What crosses a VPN: a note beside the status while such a range is chosen.
function markVia() {
  const r = routeOf($("target").value), el = $("viaNote");
  el.hidden = !r;
  if (r) el.textContent = `Attraverso ${r.label}: arrivano IP, porte, ping e nomi DNS o SMB (con il MAC dei PC Windows e di molti NAS); gli altri MAC, Bonjour e i protocolli show in broadcast (Art-Net, Dante) no.`;
}

// ---- Saved ranges: a scan target with a name ("Palco" = 10.0.0.1-50) ----
function fillRanges() {
  const rs = prefs.saved_ranges || [];
  $("rangeList").innerHTML = rs.map(r => `<option value="${esc(r.target)}">${esc(r.name)}</option>`).join("");
  $("iface").querySelector("optgroup.saved")?.remove();
  if (rs.length) {
    const g = document.createElement("optgroup"); g.className = "saved"; g.label = LANG === "en" ? "Saved ranges" : "Intervalli salvati";
    for (const r of rs) g.append(new Option(`${r.name} — ${r.target}`, r.target));
    $("iface").insertBefore(g, [...$("iface").options].find(o => o.value === "") || null);
  }
  markRange();
}
const savedRange = () => (prefs.saved_ranges || []).find(r => r.target === $("target").value.trim());
function markRange() {
  const r = savedRange();
  const opt = r && [...$("iface").querySelectorAll("optgroup.saved option")].find(o => o.value === r.target);
  if (opt) opt.selected = true;
  $("rangeBtn").textContent = r ? "★" : "☆"; $("rangeBtn").classList.toggle("on", !!r);
  $("rangeBtn").title = r ? `Intervallo salvato: «${r.name}». Clic per toglierlo` : "Salva questo intervallo con un nome";
}
$("target").addEventListener("input", () => { markRange(); markVia(); });
$("iface").addEventListener("change", () => { markRange(); markVia(); });
const saveRanges = list => post("api/prefs", {saved_ranges: list}).then(r => r.json()).then(p => { prefs = p; fillRanges(); });
$("rangeBtn").onclick = () => {
  const box = $("rangeBox"), target = $("target").value.trim(), r = savedRange();
  if (!box.hidden) { box.hidden = true; return; }
  if (!target) { $("target").focus(); return; }
  box.innerHTML = r
    ? `<span class="nt">${esc(r.name)}</span> <span class="muted mono">${esc(r.target)}</span><button id="rangeDel" class="danger">Togli</button>`
    : `<input id="rangeName" maxlength="40" placeholder="Nome, es. Palco" spellcheck="false"><button id="rangeSave" class="primary">Salva</button>`;
  box.hidden = false;
  if (r) $("rangeDel").onclick = () => saveRanges(prefs.saved_ranges.filter(x => x !== r)).then(() => { box.hidden = true; toast("Intervallo tolto."); }).catch(e => toast("Errore: " + e.message));
  else {
    const save = () => {
      const name = $("rangeName").value.trim(); if (!name) { $("rangeName").focus(); return; }
      saveRanges([...(prefs.saved_ranges || []), {name, target}]).then(() => { box.hidden = true; toast(`Salvato: «${name}». Lo ritrovi nel menu delle interfacce.`); }).catch(e => toast("Non salvato: " + e.message));
    };
    $("rangeSave").onclick = save;
    $("rangeName").onkeydown = e => { if (e.key === "Enter") save(); if (e.key === "Escape") { e.stopPropagation(); box.hidden = true; } };
    $("rangeName").focus();
  }
};
document.addEventListener("click", e => { if (!e.target.closest(".targetwrap")) $("rangeBox").hidden = true; });

// ---- Public IP, provider and country (off until turned on in Settings) ----
async function showPublicIP() {
  const el = $("pubip");
  if (!prefs.public_ip) { el.hidden = true; return; }
  let d; try { d = await fetch("api/publicip").then(r => r.json()); } catch { return; }
  if (!d.on) { el.hidden = true; return; }
  el.hidden = false;
  if (d.error) { el.textContent = (LANG === "en" ? "Public IP: " : "IP pubblico: ") + d.error; return; }
  let country = d.country || "";
  try { country = new Intl.DisplayNames([LANG], {type: "region"}).of(d.country) || country; } catch {}
  const flag = /^[A-Z]{2}$/.test(d.country || "") ? String.fromCodePoint(...[...d.country].map(c => 0x1F1A5 + c.charCodeAt(0))) + " " : "";
  const org = (d.org || "").replace(/^AS\d+\s+/, "");
  el.innerHTML = `<span class="muted">IP pubblico</span> <span class="mono" data-copy="${esc(d.ip)}">${esc(d.ip)}</span>${org ? ` · <span class="nt">${esc(org)}</span>` : ""}${country ? ` · ${flag}<span class="nt">${esc(country)}</span>` : ""}`;
  el.title = [d.city, country].filter(Boolean).join(", ");
}
setInterval(() => { if (!document.hidden) showPublicIP(); }, 10 * 60 * 1000);

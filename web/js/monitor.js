// Exports (CSV, JSON, print) and the show Monitor.
// ---- Export ----
async function download(name, text, type) {
  if (ui !== "browser") { // app windows: the server saves into Downloads
    const r = await fetch("api/save", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({name, data: text})});
    $("status").textContent = r.ok ? `Salvato: ${(await r.json()).path}` : "Salvataggio non riuscito: " + await r.text();
    return;
  }
  const a = document.createElement("a");
  a.href = URL.createObjectURL(new Blob([text], {type}));
  a.download = name; a.click();
  setTimeout(() => URL.revokeObjectURL(a.href), 1000);
}
const stamp = () => new Date().toISOString().slice(0, 16).replace(/[:T]/g, "-");
const exportJSON = () => download(`wasaping-${stamp()}.json`, JSON.stringify(hosts, null, 2), "application/json");
function exportCSV() {
  const q = v => `"${String(v ?? "").replace(/"/g, '""')}"`;
  const lines = [["ip","nome","ruolo","nome dispositivo","hostname","mac","produttore","modello","tipo","protocolli","universi","porte","rtt_ms","critico","note"].join(",")]
    .concat(hosts.map(h => [h.ip, h.name, h.role, h.friendly, h.hostname, h.mac, h.vendor, h.model, (KIND[h.kind] || KIND[""])[0], (h.protocols || []).join(" "), universeText(h), (h.ports||[]).join(" "), h.rtt_ms || "", h.critical ? "sì" : "", h.note].map(q).join(",")));
  download(`wasaping-${stamp()}.csv`, lines.join("\n"), "text/csv");
}

function universeText(h) {
  const p = [];
  if (h.artnet && (h.artnet.out || []).length) p.push("Art-Net out " + ranges(h.artnet.out));
  if (h.artnet && (h.artnet.in || []).length) p.push("Art-Net in " + ranges(h.artnet.in));
  if (h.sacn && (h.sacn.universes || []).length) p.push("sACN " + ranges(h.sacn.universes));
  return p.join("; ");
}

// Printable IP list for the crew / the venue: saved as a web page and
// opened in the browser, where it can be printed or saved as PDF.
function printList() {
  const title = network ? (network.name || network.auto) : "Rete";
  const list = [...hosts].sort((a, b) => ipNum(a.ip) - ipNum(b.ip));
  const rows = list.map(h => `<tr><td>${esc(h.ip)}</td><td><b>${esc(label(h) || "—")}</b>${h.critical ? " ●" : ""}</td><td>${esc(h.role || "")}</td><td>${esc((KIND[h.kind] || KIND[""])[0])}</td><td>${esc(h.model || h.vendor || "")}</td><td>${esc(h.mac || "")}</td><td>${esc((h.protocols || []).join(", "))}</td><td>${esc(universeText(h))}</td><td>${esc(h.swport ? h.swport.switch + " " + h.swport.port : "")}</td><td>${esc(h.note || "")}</td></tr>`).join("");
  const html = `<!DOCTYPE html><html lang="it"><head><meta charset="utf-8"><title>${esc(title)} — elenco IP</title>
<style>body{font-family:-apple-system,"Segoe UI",Roboto,sans-serif;font-size:11px;color:#171717;margin:24px}h1{font-size:18px;margin:0}p{color:#666;margin:4px 0 14px}
table{border-collapse:collapse;width:100%}th,td{border-bottom:1px solid #ddd;padding:5px 6px;text-align:left;vertical-align:top}th{font-size:10px;text-transform:uppercase;color:#666}
td:first-child,td:nth-child(6){font-family:Menlo,Consolas,monospace}tr:nth-child(even) td{background:#f6f8f3}@page{size:A4 landscape;margin:12mm}</style></head>
<body><h1>${esc(title)}</h1><p>Elenco IP · ${list.length} dispositivi · ${esc(new Date().toLocaleString("it"))} · wasaping · ● = critico</p>
<table><thead><tr><th>IP</th><th>Dispositivo</th><th>Ruolo</th><th>Tipo</th><th>Modello</th><th>MAC</th><th>Protocolli</th><th>Universi</th><th>Switch</th><th>Note</th></tr></thead><tbody>${rows}</tbody></table>
<script>addEventListener("load",()=>setTimeout(()=>print(),300))<\/script></body></html>`;
  let page = html;
  if (LANG === "en") {
    const doc = new DOMParser().parseFromString(html, "text/html");
    translateTree(doc.body);
    for (const [it, en] of [["Elenco IP", "IP list"], ["dispositivi", "devices"], ["critico", "critical"], ["Ruolo", "Role"], ["Tipo", "Type"], ["Universi", "Universes"], ["Note", "Notes"], ["Modello", "Model"], ["Protocolli", "Protocols"], ["Dispositivo", "Device"]])
      doc.body.innerHTML = doc.body.innerHTML.replaceAll(`>${it}<`, `>${en}<`).replace(`Elenco IP ·`, "IP list ·").replace(" dispositivi ·", " devices ·").replace("● = critico", "● = critical");
    page = "<!DOCTYPE html>" + doc.documentElement.outerHTML;
  }
  if (ui === "browser") {
    const w = window.open("", "_blank");
    if (w) { w.document.write(page); w.document.close(); return; }
  }
  post("api/save", {name: `wasaping-elenco-${stamp()}.html`, data: page, open: true})
    .then(r => r.json()).then(d => toast("Elenco aperto nel browser: " + d.path))
    .catch(e => toast("Non salvato: " + e.message));
}

// ---- Show monitor: runs in the program; the page shows its state ----
let monTimer = null, monLast = null, audioCtx = null, monSeenEvent = -1, monMuted = null;
$("mMute").onclick = () => { unlockSound(); post("api/monitor", {mute: monMuted ? 0 : 300}).then(monRefresh).catch(e => toast(e.message)); };
$("mShow").onclick = () => { unlockSound(); const on = $("mShow").classList.contains("on"); post("api/monitor", {show: on ? "end" : "start"}).then(monRefresh).then(() => toast(on ? "Fine show segnata: il report è in Progetto." : "Inizio show segnato.")).catch(e => toast(e.message)); };
$("mSlow").onchange = () => post("api/monitor", {slow_ms: +$("mSlow").value}).then(monRefresh).catch(e => toast(e.message));
$("mEvery").onchange = () => post("api/monitor", {on: !!(monLast && monLast.on), every: +$("mEvery").value}).then(monRefresh);
$("mToggle").onclick = () => post("api/monitor", {on: !(monLast && monLast.on), every: +$("mEvery").value}).then(monRefresh);
$("mExport").onclick = () => {
  const q = v => `"${String(v ?? "").replace(/"/g, '""')}"`;
  const ev = (monLast && monLast.events) || [];
  const lines = ["data,ora,dispositivo,ip,evento,durata_s"].concat(ev.map(e => {
    const d = new Date(e.at);
    return [d.toLocaleDateString("it"), d.toLocaleTimeString("it"), q(e.name), e.ip, e.up ? "di nuovo in linea" : "non risponde", e.up ? Math.round(e.down_for) : ""].join(",");
  }));
  download(`wasaping-registro-${stamp()}.csv`, lines.join("\n"), "text/csv");
};

async function openMonitor() {
  if ($("sheet").classList.contains("open")) closeSheet(); // a tool page would hide it
  $("monitor").classList.add("open"); $("monitor").setAttribute("aria-hidden", "false"); markQuick();
  placeNet(); // the network the Monitor watches, and its warnings
  // Browsers need a user gesture to allow sound later.
  unlockSound();
  const st = await monRefresh();
  // Opening the monitor starts it, if there is something to watch.
  if (st && !st.on) {
    const [crit, unis] = await Promise.all([fetch("api/critical").then(r => r.json()).catch(() => []), fetch("api/universe").then(r => r.json()).catch(() => [])]);
    if ((crit || []).length || (unis || []).length) await post("api/monitor", {on: true, every: +$("mEvery").value}).then(monRefresh);
  }
  fetch("api/remote").then(r => r.json()).then(showRemoteHint).catch(() => {});
  clearInterval(monTimer); monTimer = setInterval(monRefresh, 1000);
}
function closeMonitor() {
  clearInterval(monTimer); monTimer = null;
  $("monitor").classList.remove("open"); $("monitor").setAttribute("aria-hidden", "true"); markQuick();
  placeNet();
}
async function monRefresh() {
  let st;
  try { st = await fetch("api/monitor").then(r => r.json()); } catch { return null; }
  monLast = st;
  $("mEvery").value = String(st.every);
  $("mToggle").textContent = st.on ? "Ferma" : "Avvia";
  $("mToggle").classList.toggle("primary", !st.on);
  $("mSlow").value = String(st.slow_ms || 0);
  monMuted = st.muted_until && !st.muted_until.startsWith("0001") && new Date(st.muted_until) > new Date() ? new Date(st.muted_until) : null;
  $("mMute").textContent = monMuted ? `Silenziato fino alle ${monMuted.toLocaleTimeString("it", {hour: "2-digit", minute: "2-digit"})} · riattiva` : "Silenzia 5 min";
  $("mMute").classList.toggle("on", !!monMuted);
  const showOn = st.show_since && !st.show_since.startsWith("0001");
  $("mShow").textContent = showOn ? `Fine show (dalle ${new Date(st.show_since).toLocaleTimeString("it", {hour: "2-digit", minute: "2-digit"})})` : "Inizio show";
  $("mShow").classList.toggle("on", !!showOn);
  // The network is in the bar below (moved here): repeat it only without one.
  const where = $("netbar").textContent.trim() ? "" : (st.network || "");
  $("mInfo").textContent = [where, st.on ? "attivo, anche a finestra chiusa" : "fermo"].filter(Boolean).join(" · ");
  drawTiles(st);
  const ev = st.events || [];
  $("mEvents").innerHTML = ev.length ? [...ev].reverse().map(e => `<li class="${e.text ? "warn" : e.up ? "up" : "down"}"><span class="t">${new Date(e.at).toLocaleTimeString("it")}</span><b class="nt">${esc(e.name)}</b> ${e.text ? `<span class="nt">${esc(e.text)}</span>` : e.up ? `di nuovo in linea dopo ${esc(fmtDur(e.down_for))}` : (e.kind === "universe" ? "non arriva" : "non risponde")}</li>`).join("") : '<li class="muted">Nessun evento.</li>';
  // Sound for drops that happened since the page last looked.
  const fresh = ev.filter(e => e.id > monSeenEvent && !e.up && !e.text);
  if (monSeenEvent >= 0 && fresh.length && !monMuted) alarm(fresh.map(e => e.name), true);
  if (ev.length) monSeenEvent = Math.max(monSeenEvent, ev[ev.length - 1].id); else if (monSeenEvent < 0) monSeenEvent = 0;
  return st;
}
const fmtDur = s => s < 60 ? `${Math.round(s)} s` : s < 3600 ? `${Math.floor(s / 60)} min ${Math.round(s % 60)} s` : `${Math.floor(s / 3600)} h ${Math.floor(s % 3600 / 60)} min`;
function spark(rtts) {
  if (!rtts || rtts.length < 2) return "";
  const w = 180, h = 26, max = Math.max(5, ...rtts.filter(x => x >= 0));
  const pts = rtts.map((v, i) => `${(i * w / (rtts.length - 1)).toFixed(1)},${v < 0 ? h : (h - 2 - (h - 4) * v / max).toFixed(1)}`);
  const lost = rtts.map((v, i) => v < 0 ? `<rect x="${(i * w / (rtts.length - 1) - 1.5).toFixed(1)}" y="0" width="3" height="${h}" fill="var(--warn)" opacity=".6"/>` : "").join("");
  return `<svg class="spark" viewBox="0 0 ${w} ${h}" preserveAspectRatio="none">${lost}<polyline points="${pts.join(" ")}" fill="none" stroke="var(--wasabi)" stroke-width="1.5"/></svg>`;
}
function drawTiles(st) {
  const devs = st.devices || [], unis = st.universes || [];
  $("mPTP").innerHTML = (st.ptp || []).map(p => `Clock PTP: master <b>${esc(p.ip)}</b> <span class="muted">${esc(p.kind)} · ${esc(new Date(p.since).toLocaleTimeString("it"))}</span>`).join("<br>");
  if (!devs.length && !unis.length) {
    $("tiles").innerHTML = '<div class="mhint">Nessun dispositivo critico su questa rete.<br>Apri un dispositivo dall\'elenco e spunta <b>Critico</b>: console, switch principale, stagebox Dante, media server…</div>';
    $("mState").className = "state ok"; $("mState").textContent = "—";
    return;
  }
  $("mState").className = "state " + (st.down ? "bad" : "ok");
  $("mState").className = "state " + (st.down ? "bad" : st.slow ? "slow" : "ok");
  $("mState").textContent = !st.on ? "FERMO" : st.down ? `${st.down} ${st.down === 1 ? "NON RISPONDE" : "NON RISPONDONO"}` : st.slow ? `${st.slow} ${st.slow === 1 ? "LENTO" : "LENTI"}` : "TUTTO OK";
  $("tiles").innerHTML = devs.map(d => {
    const isDown = d.known && !d.alive;
    const status = !d.known ? "controllo…" : d.alive ? (d.rtt_ms ? d.rtt_ms + " ms" : "ok") + (d.slow ? " · lento" : "") : "NON RISPONDE";
    const since = d.known && d.since ? `da ${new Date(d.since).toLocaleTimeString("it")}` : "";
    return `<div class="tile${isDown ? " down" : d.slow ? " slow" : ""}"><div class="t nt">${kindIcon(d.kind)}${esc(d.name)}</div>
      <div class="r nt">${esc([d.role, d.ip].filter(Boolean).join(" · "))}</div>
      <div class="s"><b>${esc(status)}</b><span class="muted">${esc(since)}</span></div>${spark(d.rtts)}</div>`;
  }).join("") + unis.map(u => {
    const isDown = u.known && !u.alive;
    const status = !u.known ? "controllo…" : u.alive ? `${Math.round(u.fps)} frame/s` : "NON ARRIVA";
    return `<div class="tile${isDown ? " down" : ""}"><div class="t">${kindIcon("lighting")}${esc(u.name)}</div>
      <div class="r nt">${esc((u.sources || []).join(", "))}</div>
      <div class="s"><b>${esc(status)}</b><span class="muted">${u.known && u.since ? esc((LANG === "en" ? "since " : "da ") + new Date(u.since).toLocaleTimeString("it")) : ""}</span></div></div>`;
  }).join("");
}
function showRemoteHint(r) {
  $("rOn").checked = !!(r && r.on);
  $("mRemote").innerHTML = r && r.on
    ? `Apri <b>${esc(r.url)}</b> sul dispositivo (stessa rete) e inserisci il PIN <b style="font-size:16px;letter-spacing:2px">${esc(r.pin)}</b>. Da lì si guarda soltanto, non si cambia niente. <a id="rNew">Nuovo PIN</a>`
    : "Mostra questo Monitor su un iPad o un telefono della stessa rete, protetto da un PIN.";
  $("mQR").hidden = !(r && r.on);
  if (r && r.on) $("mQR").src = `api/qr?for=remote&t=${Date.now()}`;
  if ($("rNew")) $("rNew").onclick = () => post("api/remote", {on: true, new_pin: true}).then(r => r.json()).then(showRemoteHint).catch(e => toast(e.message));
}
$("rOn").onchange = () => post("api/remote", {on: $("rOn").checked}).then(r => r.json()).then(showRemoteHint)
  .catch(e => { toast("Non attivato: " + e.message); $("rOn").checked = false; });
function alarm(names, quiet) {
  // The program sends the system notification; the page adds sound and a toast.
  if (!quiet) post("api/notify", {title: "⚠️ wasaping: non risponde", body: names.join(", ")}).catch(() => {});
  toast("Non risponde: " + names.join(", "));
  if ($("mSound").checked) playAlarm();
}
// Browsers allow sound only after a click: every button that may lead to
// an alarm calls this.
function unlockSound() {
  try { audioCtx = audioCtx || new (window.AudioContext || window.webkitAudioContext)(); audioCtx.resume(); } catch {}
}
// Three beeps, at the volume of Settings → Allarmi.
function playAlarm() {
  if (!audioCtx) return;
  const vol = 0.25 * (prefs.alarm_volume || 60) / 100;
  try {
    const t = audioCtx.currentTime;
    for (let i = 0; i < 3; i++) {
      const o = audioCtx.createOscillator(), g = audioCtx.createGain();
      o.type = "square"; o.frequency.value = i % 2 ? 660 : 880;
      g.gain.setValueAtTime(vol, t + i * 0.25); g.gain.setValueAtTime(0, t + i * 0.25 + 0.18);
      o.connect(g).connect(audioCtx.destination); o.start(t + i * 0.25); o.stop(t + i * 0.25 + 0.2);
    }
  } catch {}
}

$("ouiBtn").onclick = async () => {
  $("ouiBtn").disabled = true; $("ouiBtn").textContent = "Scarico…";
  const r = await fetch("api/update-oui", {method: "POST"});
  if (r.ok) {
    const d = await r.json();
    $("oui").innerHTML = `<span>✓ ${d.vendors.toLocaleString("it")} produttori scaricati. Rifai la scansione per vederli.</span>`;
  } else {
    $("ouiBtn").disabled = false; $("ouiBtn").textContent = "Riprova";
    $("status").textContent = "Download non riuscito: " + await r.text();
  }
};

// The device list, context menu, Tools menu and the detail panel.
function render() {
  const f = $("filter").value.toLowerCase();
  const match = h => !f || [h.ip, h.name, h.role, h.note, h.friendly, h.hostname, h.mac, h.vendor, h.model, (h.protocols || []).join(" "), (KIND[h.kind] || KIND[""])[0], (h.ports || []).join(" ")].join(" ").toLowerCase().includes(f);
  const inNet = h => !activeNet || h.net === activeNet;
  const list = hosts.filter(h => inNet(h) && match(h));
  list.sort((a, b) => {
    const x = sortValue(a, sortKey), y = sortValue(b, sortKey);
    if (typeof x === "string") { if (!x && y) return 1; if (!y && x) return -1; }
    return (x > y ? 1 : x < y ? -1 : 0) * sortDir;
  });
  const off = es || !showGone ? [] : gone.filter(h => inNet(h) && match(h));
  if (!list.length && !off.length) {
    $("rows").innerHTML = `<tr><td colspan="7" class="empty">${es ? "Cerco dispositivi…" : (hosts.length ? "Nessun risultato per il filtro." : "Nessun dispositivo trovato.")}</td></tr>`;
    return;
  }
  const row = (h, isGone) => {
    const main = label(h);
    const sub = [h.role, h.name ? (h.friendly || h.hostname) : (h.friendly && h.hostname && h.friendly !== h.hostname ? h.hostname : "")].filter(Boolean).join(" · ");
    const warns = (h.warnings || []).length;
    return `<tr data-key="${esc(h.key)}" class="${isGone ? "gone" : ""}${selected === h.key ? " sel" : ""}">
    <td class="name"><div class="namecell">${kindIcon(h.kind)}<div>${main ? `<span class="nt ${h.name ? "label" : ""}">${esc(main)}</span>` : '<span class="muted">—</span>'}${h.self ? '<span class="badge me">tu</span>' : ""}${h.new ? '<span class="badge new">nuovo</span>' : ""}${isGone ? '<span class="badge off">non risponde</span>' : ""}${h.critical ? '<span class="badge crit">critico</span>' : ""}${warns ? `<span class="badge warn" title="${esc(h.warnings.join("\n"))}">⚠ ${warns}</span>` : ""}${sub ? `<span class="sub nt">${esc(sub)}</span>` : ""}${isGone ? `<span class="sub">visto ${esc(when(h.last_seen))}</span>` : ""}</div></div></td>
    <td class="mono" data-copy="${esc(h.ip)}">${esc(h.ip)}</td>
    <td class="mono" data-copy="${esc(h.mac || "")}">${h.mac ? esc(h.mac) : '<span class="muted">—</span>'}</td>
    <td>${h.model ? `${esc(h.model)}${h.vendor && !h.vendor.startsWith("(") ? `<span class="sub">${esc(h.vendor)}</span>` : ""}` : h.vendor ? esc(h.vendor) : '<span class="muted">—</span>'}</td>
    <td>${isGone ? "" : protoChips(h)}</td>
    <td class="ports">${isGone ? "" : portLinks(h)}</td>
    <td class="muted">${isGone ? "" : h.rtt_ms ? h.rtt_ms + " ms" : h.method === "arp" ? "ARP" : ""}</td>
  </tr>`;
  };
  $("rows").innerHTML = list.map(h => row(h, false)).join("") + off.map(h => row(h, true)).join("");
}

function portLinks(h, ports = h.ports) {
  return (ports || []).map(p => {
    const url = portURL(h.ip, p), name = PORT_NAMES[p] ? PORT_NAMES[p] + " · " : "";
    const tip = url ? `${name}clic: apri con ${OPEN[p][1]} · tasto destro: copia` : `${name}clic: copia ${h.ip}:${p}`;
    return `<a class="port" data-ip="${esc(h.ip)}" data-port="${p}"${url ? ` href="${esc(url)}" target="_blank" rel="noopener"` : ""} title="${esc(tip)}">${p}</a>`;
  }).join("");
}

// ---- Copy and context menu ----
function toast(msg) {
  $("toast").textContent = msg; $("toast").classList.add("show");
  clearTimeout(toast.t); toast.t = setTimeout(() => $("toast").classList.remove("show"), 1600);
}
async function copy(text) {
  try { await navigator.clipboard.writeText(text); }
  catch {
    const t = document.createElement("textarea");
    t.value = text; t.style.position = "fixed"; t.style.opacity = "0";
    document.body.appendChild(t); t.select(); document.execCommand("copy"); t.remove();
  }
  toast("Copiato: " + text);
}
function openURL(url) {
  const a = document.createElement("a");
  a.href = url; a.target = "_blank"; a.rel = "noopener"; a.click();
}
function showMenu(x, y, hint, items) {
  const m = $("ctx");
  m.innerHTML = (hint ? `<div class="hint">${esc(hint)}</div>` : "") + items.map((it, i) => `<div data-i="${i}">${esc(it[0])}</div>`).join("");
  m.querySelectorAll("[data-i]").forEach(d => d.onclick = () => { hideMenu(); items[+d.dataset.i][1](); });
  m.style.display = "block";
  const r = m.getBoundingClientRect();
  m.style.left = Math.min(x, innerWidth - r.width - 8) + "px";
  m.style.top = Math.min(y, innerHeight - r.height - 8) + "px";
}
function hideMenu() { $("ctx").style.display = "none"; }

// "Strumenti" menu: everything that is not scanning, grouped.
$("toolsBtn").onclick = e => {
  e.stopPropagation();
  const r = $("toolsBtn").getBoundingClientRect();
  const none = !hosts.length;
  const groups = [
    ["Rete", [["Piano IP", planView], ["IP liberi", freeView], ["Porte switch", switchView, none]]],
    ["Show", [["Universi DMX", dmxView], ["Apparecchi RDM", rdmView], ["Report di fine show", showReportView]]],
    ["Locale", [["Esporta locale", exportVenue, !network], ["Importa locale", () => pickFile(".wasaping,.json", importVenue), !network]]],
    ["Esporta", [["Stampa elenco IP", () => $("printBtn").click(), none], ["CSV", () => $("csv").click(), none], ["JSON", () => $("json").click(), none]]],
  ];
  const m = $("ctx");
  m.innerHTML = groups.map(([g, items]) => `<div class="hint">${esc(g)}</div>` + items.map(([l, , dis], i) => `<div data-g="${esc(g)}" data-i="${i}"${dis ? ' class="dis"' : ""}>${esc(l)}</div>`).join("")).join("");
  m.querySelectorAll("[data-i]").forEach(d => d.onclick = () => {
    if (d.classList.contains("dis")) return;
    hideMenu();
    groups.find(x => x[0] === d.dataset.g)[1][+d.dataset.i][1]();
  });
  m.style.display = "block";
  const w = m.getBoundingClientRect().width;
  m.style.left = Math.max(8, Math.min(r.right - w, innerWidth - w - 8)) + "px";
  m.style.top = (r.bottom + 6) + "px";
  translateTree(m);
};
document.addEventListener("click", e => { if (!e.target.closest("#ctx")) hideMenu(); });
document.addEventListener("scroll", hideMenu, true);
window.addEventListener("blur", hideMenu);

// Left click on a port: open it, or copy it when there is nothing to open.
document.addEventListener("click", e => {
  const a = e.target.closest("a.port");
  if (a && !a.getAttribute("href")) { e.preventDefault(); copy(`${a.dataset.ip}:${a.dataset.port}`); }
});
// Right click: port menu, or copy an IP / MAC.
document.addEventListener("contextmenu", e => {
  const a = e.target.closest("a.port");
  if (a) {
    e.preventDefault();
    const ip = a.dataset.ip, p = +a.dataset.port, url = portURL(ip, p), items = [];
    if (url) items.push([`Apri con ${OPEN[p][1]}`, () => openURL(url)]);
    items.push([`Copia ${ip}:${p}`, () => copy(`${ip}:${p}`)]);
    if (url) items.push(["Copia URL", () => copy(url)]);
    showMenu(e.clientX, e.clientY, `Porta ${p}${PORT_NAMES[p] ? " · " + PORT_NAMES[p] : ""}`, items);
    return;
  }
  const cell = e.target.closest("[data-copy]");
  if (cell && cell.dataset.copy) {
    e.preventDefault();
    showMenu(e.clientX, e.clientY, "", [[`Copia ${cell.dataset.copy}`, () => copy(cell.dataset.copy)]]);
  }
});

document.querySelectorAll("th").forEach(th => th.onclick = () => {
  const k = th.dataset.k;
  sortDir = sortKey === k ? -sortDir : 1; sortKey = k; render();
});
$("filter").oninput = render;
$("rows").onclick = e => {
  if (e.target.closest("a")) return; // port links open the device page
  const tr = e.target.closest("tr[data-key]");
  if (tr) openDetail(tr.dataset.key);
};

// ---- Detail panel ----
const findHost = key => hosts.find(h => h.key === key) || gone.find(h => h.key === key);

function openDetail(key) {
  const h = findHost(key);
  if (!h) { closeDetail(); return; }
  const changed = selected !== key;
  selected = key;
  const isGone = !hosts.includes(h);
  $("dTitle").innerHTML = kindIcon(h.kind) + `<span class="nt">${esc(label(h) || h.ip)}</span>`;
  $("dSub").textContent = (KIND[h.kind] || KIND[""])[0] + (isGone ? " · non risponde" : "");
  if (changed) { stopTool(); $("dToolHost").value = h.ip; $("dPings").hidden = $("dToolOut").hidden = true; }
  if (changed) { $("dName").value = h.name || ""; $("dRole").value = h.role || ""; $("dNote").value = h.note || ""; $("dCrit").checked = !!h.critical; $("dMsg").textContent = ""; }
  $("dName").placeholder = (h.friendly || h.hostname) ? `vuoto = ${h.friendly || h.hostname}` : "es. Console FOH";
  $("dWarn").innerHTML = (h.warnings || []).map(w => `<li>${esc(w)}</li>`).join("");
  const svcs = [...new Set((h.services || []).map(s => SERVICES[s] ?? s).filter(Boolean))];
  const rows = [
    ["IP", `<span class="mono" data-copy="${esc(h.ip)}">${esc(h.ip)}</span>`],
    ["MAC", h.mac ? `<span class="mono" data-copy="${esc(h.mac)}">${esc(h.mac)}</span>` : "—"],
    ["Produttore", esc(h.vendor) || "—"],
    ...(h.model ? [["Modello", esc(h.model)]] : []),
    ...(h.friendly ? [["Nome del dispositivo", esc(h.friendly)]] : []),
    ...(h.web_title ? [["Pagina web", esc(h.web_title)]] : []),
    ...(h.swport ? [["Switch", `${esc(h.swport.switch)} · porta <b>${esc(h.swport.port)}</b>${h.swport.alias ? ` <span class="muted">(${esc(h.swport.alias)})</span>` : ""}`]] : []),
    ["Nome in rete", esc(h.hostname) || "—"],
    ...(h.protocols && h.protocols.length ? [["Protocolli", protoChips(h)]] : []),
    ...showFacts(h),
    ["Servizi", svcs.length ? svcs.map(s => `<span class="svc">${esc(s)}</span>`).join("") : "—"],
    ["Porte aperte", isGone ? "—" : portLinks(h) || "nessuna tra quelle controllate"],
    ["Risposta", isGone ? "—" : h.rtt_ms ? h.rtt_ms + " ms" : h.method === "arp" ? "solo tabella ARP" : "—"],
    ["Visto la prima volta", esc(when(h.first_seen)) || "—"],
  ];
  if (isGone) rows.push(["Visto l'ultima volta", esc(when(h.last_seen))]);
  if (h.history) {
    const cells = [...h.history].map(c => `<i class="${c === "1" ? "y" : c === "0" ? "n" : ""}"></i>`).join("");
    const n = [...h.history].filter(c => c !== "-"), y = n.filter(c => c === "1").length;
    rows.push(["Presenza", `${y} su ${n.length} scansioni<div class="hist" title="ultime scansioni, la più recente a destra">${cells}</div>`]);
  }
  $("dInfo").innerHTML = rows.map(([k, v]) => `<dt>${k}</dt><dd>${v}</dd>`).join("");
  $("dWol").disabled = !h.mac || h.self;
  $("dPorts").disabled = isGone;
  $("dForget").hidden = !isGone;
  $("detail").classList.add("open"); $("detail").setAttribute("aria-hidden", "false");
  document.body.classList.add("detail-open");
  render();
}
function showFacts(h) {
  const f = [];
  if (h.artnet) {
    const a = h.artnet;
    f.push(["Art-Net", esc([a.short_name, tr(a.style)].filter(Boolean).join(" · "))]);
    if ((a.out || []).length) f.push(["Universi in uscita", esc(ranges(a.out)) + ' <span class="muted">(port-address)</span>']);
    if ((a.in || []).length) f.push(["Universi in ingresso", esc(ranges(a.in))]);
    if (a.report) f.push(["Stato del nodo", esc(a.report)]);
    if (a.firmware) f.push(["Firmware", esc(a.firmware)]);
  }
  if (h.sacn) {
    f.push(["Sorgente sACN", esc(h.sacn.name || "—")]);
    if ((h.sacn.universes || []).length) f.push(["Universi sACN", esc(ranges(h.sacn.universes))]);
    if (h.sacn.priority) f.push(["Priorità sACN", esc(h.sacn.priority)]);
  }
  if (h.ptp) f.push(["Clock PTP", `master ${esc(h.ptp)}${h.ptp === "PTPv1" ? ' <span class="muted">(Dante)</span>' : ""}`]);
  if (h.dante) {
    const d = h.dante, bits = [];
    if (d.tx_channels) bits.push(LANG === "en" ? `${d.tx_channels} transmit channels` : `${d.tx_channels} canali in trasmissione`);
    if (d.sample_rate) bits.push(`${d.sample_rate / 1000} kHz`);
    if (d.latency_ms) bits.push((LANG === "en" ? "latency " : "latenza ") + d.latency_ms + " ms");
    if (bits.length) f.push(["Dante", esc(bits.join(" · "))]);
  }
  if ((h.ndi || []).length) f.push(["Sorgenti NDI", h.ndi.map(n => `<span class="svc">${esc(n)}</span>`).join("")]);
  if (h.pjlink) {
    const p = h.pjlink;
    if (p.needs_login) f.push(["PJLink", "protetto da password"]);
    else {
      if (p.power) f.push(["Stato proiettore", esc(tr(p.power))]);
      if ((p.lamp_hours || []).length) f.push(["Ore lampada", esc(p.lamp_hours.join(" / "))]);
      if (p.errors) f.push(["Errori", esc(p.errors)]);
    }
  }
  return f;
}

function closeDetail() {
  selected = null;
  $("detail").classList.remove("open"); $("detail").setAttribute("aria-hidden", "true");
  document.body.classList.remove("detail-open");
  render();
}
$("dClose").onclick = closeDetail;
document.addEventListener("keydown", e => {
  if (e.key !== "Escape") return;
  if ($("ctx").style.display === "block") hideMenu(); else if (selected) closeDetail();
});

async function post(url, body) {
  const r = await fetch(url, {method: "POST", headers: {"Content-Type": "application/json"}, body: body ? JSON.stringify(body) : undefined});
  if (!r.ok) throw new Error(await r.text());
  return r;
}

$("dSave").onclick = async () => {
  const h = findHost(selected); if (!h) return;
  const name = $("dName").value.trim(), role = $("dRole").value.trim(), note = $("dNote").value.trim(), critical = $("dCrit").checked;
  try {
    await post("api/device", {key: h.key, net: h.net || "", name, role, note, critical});
    Object.assign(h, {name, role, note, critical});
    $("dMsg").textContent = "✓ Salvato.";
    openDetail(h.key);
  } catch (e) { $("dMsg").textContent = "Non salvato: " + e.message; }
};
for (const id of ["dName", "dRole"]) $(id).onkeydown = e => { if (e.key === "Enter") $("dSave").click(); };

$("dWol").onclick = async () => {
  const h = findHost(selected); if (!h) return;
  try {
    await post(`api/wol?mac=${encodeURIComponent(h.mac)}`);
    $("dMsg").textContent = "✓ Pacchetto di risveglio inviato. Rifai la scansione tra qualche secondo.";
  } catch (e) { $("dMsg").textContent = "Non inviato: " + e.message; }
};

$("dPorts").onclick = async () => {
  const h = findHost(selected); if (!h) return;
  $("dPorts").disabled = true; $("dMsg").textContent = "Controllo 1024 porte…";
  try {
    const d = await fetch(`api/ports?ip=${encodeURIComponent(h.ip)}&ports=1-1024`).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
    h.ports = [...new Set([...(h.ports || []), ...d.open])].sort((a, b) => a - b);
    $("dMsg").textContent = `✓ ${d.open.length} porte aperte su ${d.checked} controllate.`;
    openDetail(h.key);
  } catch (e) { $("dMsg").textContent = "Errore: " + e.message; }
  $("dPorts").disabled = false;
};

let toolES = null;
function stopTool() { if (toolES) { toolES.close(); toolES = null; } $("dPing").disabled = $("dTrace").disabled = false; }
$("dPing").onclick = () => {
  stopTool();
  const host = $("dToolHost").value.trim(); if (!host) return;
  const bars = $("dPings"), out = $("dToolOut");
  bars.hidden = false; bars.innerHTML = ""; out.hidden = false; out.textContent = `Ping di ${host}…`;
  $("dPing").disabled = $("dTrace").disabled = true;
  toolES = new EventSource(`api/pingtest?host=${encodeURIComponent(host)}&count=20`);
  let max = 1;
  toolES.addEventListener("ping", e => {
    const p = JSON.parse(e.data);
    if (p.ok) max = Math.max(max, p.ms);
    bars.insertAdjacentHTML("beforeend", p.ok ? `<i data-ms="${p.ms}" title="${p.ms.toFixed(1)} ms"></i>` : '<i class="lost" title="perso"></i>');
    bars.querySelectorAll("i[data-ms]").forEach(i => i.style.height = Math.max(3, 34 * i.dataset.ms / max) + "px");
    out.textContent = `Ping ${p.seq}/20 · ${p.ok ? p.ms.toFixed(1) + " ms" : "perso"}`;
  });
  toolES.addEventListener("done", e => {
    const d = JSON.parse(e.data);
    const f = v => v === undefined ? "—" : v.toFixed(1);
    const verdict = d.received === 0 ? "✗ Non risponde al ping."
      : d.loss > 0 || d.jitter > 5 ? "⚠ Collegamento instabile: pacchetti persi o tempi che variano molto (cavo, Wi-Fi, switch?)."
      : "✓ Collegamento stabile.";
    out.textContent = `${verdict}\nRicevuti ${d.received}/${d.sent} · persi ${d.loss.toFixed(0)}%\nmin ${f(d.min)} · media ${f(d.avg)} · max ${f(d.max)} ms · jitter ${f(d.jitter)} ms`;
    stopTool();
  });
  toolES.onerror = () => { out.textContent += "\nInterrotto."; stopTool(); };
};
$("dTrace").onclick = () => {
  stopTool();
  const host = $("dToolHost").value.trim(); if (!host) return;
  const out = $("dToolOut");
  $("dPings").hidden = true; out.hidden = false; out.textContent = "";
  $("dPing").disabled = $("dTrace").disabled = true;
  toolES = new EventSource(`api/trace?host=${encodeURIComponent(host)}`);
  toolES.addEventListener("line", e => { out.textContent += JSON.parse(e.data) + "\n"; out.scrollTop = out.scrollHeight; });
  toolES.addEventListener("done", stopTool);
  toolES.onerror = stopTool;
};

$("dForget").onclick = async () => {
  const h = findHost(selected); if (!h) return;
  try {
    await post("api/device", {key: h.key, net: h.net || "", forget: true});
    gone = gone.filter(x => x.key !== h.key);
    closeDetail();
  } catch (e) { $("dMsg").textContent = "Errore: " + e.message; }
};

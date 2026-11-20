// The device list, context menu and the detail panel.
// Shown for devices found only in the system's ARP table.
const NOPING_TIP = "Non risponde al ping né alle porte, nemmeno al secondo tentativo. C'è perché ha risposto alla richiesta ARP («chi ha questo IP?»), quindi era sulla rete negli ultimi minuti. Succede con telefoni in standby, PC con il firewall e dispositivi smart.";

// During a scan hosts arrive one by one: draw the list at most every 150 ms
// instead of once per host (with hundreds of hosts that was slow).
let renderTimer = 0;
function renderSoon() { if (!renderTimer) renderTimer = setTimeout(render, 150); }

// What the filter looks into: the fields, plus the words of the badges
// ("critico", "nuovo", "avviso", "non risponde") in both languages.
function haystack(h, isGone) {
  return [h.ip, (h.ipv6 || []).join(" "), Object.values(h.names || {}).join(" "), h.name, h.role, h.note, h.friendly, h.hostname, h.mac, h.vendor, h.model, (h.protocols || []).join(" "),
    (KIND[h.kind] || KIND[""])[0], tr((KIND[h.kind] || KIND[""])[0]), (h.ports || []).join(" "),
    h.critical ? "critico critical" : "", h.new ? "nuovo new" : "", h.self ? "tu you" : "",
    (h.warnings || []).length ? "avviso avvisi warning " + h.warnings.join(" ") : "", isGone ? "non risponde offline spento" : ""].join(" ").toLowerCase();
}
// Every word must match (in any field): "dante critico", "artnet 10.0.0".
const filterMatch = (h, words, isGone) => { if (!words.length) return true; const t = haystack(h, isGone); return words.every(w => t.includes(w)); };
let shownKeys = []; // the rows on screen, in order (keyboard navigation)

function render() {
  clearTimeout(renderTimer); renderTimer = 0;
  const words = $("filter").value.toLowerCase().split(/\s+/).filter(Boolean);
  const match = (h, isGone) => filterMatch(h, words, isGone);
  const inNet = h => !activeNet || h.net === activeNet;
  const list = hosts.filter(h => inNet(h) && match(h, false));
  list.sort((a, b) => {
    const x = sortValue(a, sortKey), y = sortValue(b, sortKey);
    if (typeof x === "string") { if (!x && y) return 1; if (!y && x) return -1; }
    return (x > y ? 1 : x < y ? -1 : 0) * sortDir;
  });
  const hideGone = (es && !es.quiet) || !showGone; // a continuous scan keeps them on screen
  const off = hideGone ? [] : gone.filter(h => inNet(h) && match(h, true));
  const all = hosts.filter(inNet).length + (hideGone ? 0 : gone.filter(inNet).length);
  $("fcount").textContent = words.length && all ? `${list.length + off.length} di ${all}` : "";
  shownKeys = list.concat(off).map(h => h.key);
  if (!list.length && !off.length) {
    $("rows").innerHTML = `<tr><td colspan="9" class="empty">${es ? "Cerco dispositivi…" : (hosts.length ? "Nessun risultato per il filtro." : "Nessun dispositivo trovato.")}</td></tr>`;
    return;
  }
  const row = (h, isGone) => {
    const main = label(h);
    const sub = [h.role, h.name ? (h.friendly || h.hostname) : (h.friendly && h.hostname && h.friendly !== h.hostname ? h.hostname : "")].filter(Boolean).join(" · ");
    const warns = (h.warnings || []).length;
    const wl = isGone ? 0 : warnLevel(h);
    return `<tr data-key="${esc(h.key)}" class="${isGone ? "gone" : ""}${wl === 2 ? " bad" : wl ? " warned" : ""}${selected === h.key ? " sel" : ""}${!isGone && freshKeys.has(h.key) ? " fresh" : ""}">
    <td class="ccell"><input type="checkbox" class="mcrit" aria-label="Monitor"${h.critical ? " checked" : ""} title="${h.critical ? "Critico: il Monitor lo tiene d'occhio. Togli la spunta per smettere" : "Spunta per tenerlo d'occhio nel Monitor"}"></td>
    <td class="name"><div class="namecell">${kindIcon(h.kind)}<div>${main ? `<span class="nt ${h.name ? "label" : ""}">${esc(main)}</span>` : '<span class="muted">—</span>'}${h.self ? '<span class="badge me">tu</span>' : ""}${h.new ? '<span class="badge new">nuovo</span>' : ""}${isGone ? '<span class="badge off">non risponde</span>' : ""}${sub ? `<span class="sub nt">${esc(sub)}</span>` : ""}${isGone ? `<span class="sub">visto ${esc(when(h.last_seen))}</span>` : ""}</div></div></td>
    <td class="mono" data-copy="${esc(h.ip)}">${esc(h.ip)}</td>
    <td class="mono" data-copy="${esc(h.mac || "")}">${h.mac ? esc(h.mac) : '<span class="muted">—</span>'}</td>
    <td>${h.model ? `${esc(h.model)}${h.vendor && !h.vendor.startsWith("(") ? `<span class="sub">${esc(h.vendor)}</span>` : ""}` : h.vendor ? esc(h.vendor) : '<span class="muted">—</span>'}</td>
    <td class="protos">${isGone ? "" : protoChips(h)}</td>
    <td class="ports">${isGone ? "" : portLinks(h)}</td>
    <td class="muted">${isGone ? "" : h.rtt_ms ? h.rtt_ms + " ms" : h.method === "arp" ? `<span class="noping" title="${esc(NOPING_TIP)}">nessun ping</span>` : ""}</td>
    <td class="wcell">${warns && !isGone ? `<span class="wico" title="${esc(h.warnings.map(tr).join("\n"))}">⚠ ${warns}</span>` : ""}</td>
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
async function copy(text, msg) {
  try { await navigator.clipboard.writeText(text); }
  catch {
    const t = document.createElement("textarea");
    t.value = text; t.style.position = "fixed"; t.style.opacity = "0";
    document.body.appendChild(t); t.select(); document.execCommand("copy"); t.remove();
  }
  toast(msg || "Copiato: " + text);
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
function hideMenu() { $("ctx").style.display = "none"; $("ctx").style.maxHeight = ""; }

document.addEventListener("click", e => { if (!e.target.closest("#ctx")) hideMenu(); });
document.addEventListener("scroll", hideMenu, true);
// Not on "blur": the macOS app window sends one right after the click that
// opens the menu, which closed it at once. When the page is really hidden
// (window closed or minimized), close it.
document.addEventListener("visibilitychange", () => { if (document.hidden) hideMenu(); });

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
  const row = e.target.closest("#rows tr[data-key]");
  if (row && !e.target.closest("[data-copy]")) { e.preventDefault(); rowMenu(row.dataset.key, e.clientX, e.clientY); return; }
  const cell = e.target.closest("[data-copy]");
  if (cell && cell.dataset.copy) {
    e.preventDefault();
    showMenu(e.clientX, e.clientY, "", [[`Copia ${cell.dataset.copy}`, () => copy(cell.dataset.copy)]]);
  }
});

// Right click on a row: the things done most often, without opening the panel.
function rowMenu(key, x, y) {
  const h = findHost(key);
  if (!h) return;
  const isGone = !hosts.includes(h), items = [];
  const web = (h.ports || []).find(p => OPEN[p] && OPEN[p][1] === "browser");
  if (web && !isGone) items.push(["Apri la pagina web", () => openURL(portURL(h.ip, web))]);
  items.push([`Copia ${h.ip}`, () => copy(h.ip)]);
  if (h.mac) items.push([`Copia ${h.mac}`, () => copy(h.mac)]);
  if (label(h)) items.push(["Copia il nome", () => copy(label(h))]);
  items.push(["Rinomina…", () => { openDetail(key); $("dName").focus(); $("dName").select(); }]);
  items.push([h.critical ? "Non è più critico" : "Segna come critico", () => setCritical(h, !h.critical)]);
  if (!isGone) items.push(["Qualità del collegamento", () => { openDetail(key); $("dTools").open = true; $("dPing").click(); }]);
  showMenu(x, y, label(h) || h.ip, items);
}
async function setCritical(h, critical) {
  try {
    await post("api/device", {key: h.key, net: h.net || "", name: h.name || "", role: h.role || "", note: h.note || "", critical});
    h.critical = critical;
    toast(critical ? "Critico: il Monitor lo tiene d'occhio." : "Non è più critico.");
    if (selected === h.key) $("dCrit").checked = critical;
    render();
  } catch (e) { toast("Non salvato: " + e.message); }
}

// Keyboard: / or ⌘F filter, ↑ ↓ move between devices, Esc closes or clears.
const typing = e => e.target.closest && e.target.closest("input, textarea, select, [contenteditable]");
document.addEventListener("keydown", e => {
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  if ($("sheet").classList.contains("open") || $("monitor").classList.contains("open")) return;
  if (e.key === "/" && !typing(e)) { e.preventDefault(); wasapingFind(); return; }
  if ((e.key === "ArrowDown" || e.key === "ArrowUp") && !typing(e) && shownKeys.length) {
    e.preventDefault();
    const i = shownKeys.indexOf(selected), n = e.key === "ArrowDown" ? 1 : -1;
    const next = shownKeys[i < 0 ? (n > 0 ? 0 : shownKeys.length - 1) : Math.max(0, Math.min(shownKeys.length - 1, i + n))];
    openDetail(next);
    const row = document.querySelector(`#rows tr[data-key="${CSS.escape(next)}"]`);
    if (row) row.scrollIntoView({block: "nearest"});
  }
});
$("filter").addEventListener("keydown", e => {
  if (e.key === "Escape" && $("filter").value) { e.stopPropagation(); $("filter").value = ""; render(); }
  if (e.key === "ArrowDown" && shownKeys.length) { e.preventDefault(); $("filter").blur(); openDetail(shownKeys[0]); }
});

document.querySelectorAll("th").forEach(th => th.onclick = () => {
  const k = th.dataset.k;
  sortDir = sortKey === k ? -sortDir : 1; sortKey = k;
  try { localStorage.setItem("wasaping.sort", `${sortKey} ${sortDir}`); } catch {}
  render();
});
// The list box ends at the bottom of the window (see #listWrap in app.css).
(() => {
  const fit = () => {
    const top = $("listWrap").getBoundingClientRect().top + scrollY;
    document.documentElement.style.setProperty("--listTop", Math.round(top) + "px");
  };
  new ResizeObserver(fit).observe(document.body);
  addEventListener("resize", fit);
  fit();
})();
// Columns always fit the list box. IP, MAC, response, Monitor and ⚠ have
// the width of what they hold; the text columns share the rest by weight
// (never below a minimum: on a very narrow window the box scrolls
// sideways). Dragging the edge of a text column moves width between it and
// the next text column; double-click goes back to the default.
// Remembered in wasaping.colpct.
const COL_FIXED = {critical: 84, ip: 130, mac: 158, rtt_ms: 96, warn: 46};
const COL_PCT = {label: 34, model: 28, protocols: 17, ports: 21};
const COL_MIN = {label: 130, model: 100, protocols: 86, ports: 90};
(() => {
  const wrap = $("listWrap"), table = wrap.querySelector("table");
  const all = [...table.querySelectorAll("thead th")];
  const flex = all.filter(t => COL_PCT[t.dataset.k]);
  let pct = {...COL_PCT};
  try { const v = JSON.parse(localStorage.getItem("wasaping.colpct") || "null"); if (v && flex.every(t => v[t.dataset.k] > 0)) pct = v; } catch {}
  const fixedPx = all.reduce((a, t) => a + (COL_FIXED[t.dataset.k] || 0), 0);
  const room = () => Math.max(wrap.clientWidth - fixedPx, 0);
  // Share the room by weight; a column held at its minimum takes its
  // share from the others (still above theirs).
  const widths = () => {
    const r = room(), w = {};
    let free = flex.map(t => t.dataset.k), left = r;
    for (let pass = 0; pass < flex.length; pass++) {
      const sum = free.reduce((a, k) => a + pct[k], 0);
      const low = free.filter(k => left * pct[k] / sum < COL_MIN[k]);
      if (!low.length) { free.forEach(k => w[k] = Math.floor(left * pct[k] / sum)); break; }
      low.forEach(k => { w[k] = COL_MIN[k]; left -= COL_MIN[k]; });
      free = free.filter(k => !low.includes(k));
      if (!free.length) break;
    }
    return w;
  };
  const apply = () => {
    const w = widths();
    let total = fixedPx;
    all.forEach(t => {
      const k = t.dataset.k, px = COL_FIXED[k] || w[k];
      t.style.width = px + "px"; total += COL_FIXED[k] ? 0 : px;
    });
    table.style.width = total + "px";
  };
  new ResizeObserver(apply).observe(wrap);
  apply();
  flex.slice(0, -1).forEach((th, i) => {
    const next = flex[i + 1];
    const g = document.createElement("span"); g.className = "colgrip"; g.title = "Trascina per allargare · doppio clic: larghezze iniziali"; th.append(g);
    g.onclick = e => e.stopPropagation();
    g.ondblclick = e => { e.stopPropagation(); pct = {...COL_PCT}; apply(); try { localStorage.removeItem("wasaping.colpct"); } catch {} };
    g.onpointerdown = e => {
      e.preventDefault(); e.stopPropagation(); g.setPointerCapture(e.pointerId);
      const k = th.dataset.k, n = next.dataset.k, a0 = pct[k], b0 = pct[n], x0 = e.clientX;
      const perPx = flex.reduce((a, t) => a + pct[t.dataset.k], 0) / Math.max(room(), 1);
      g.onpointermove = m => {
        const d = Math.max(5 - a0, Math.min(b0 - 5, (m.clientX - x0) * perPx));
        pct[k] = a0 + d; pct[n] = b0 - d; apply();
      };
      g.onpointerup = () => { g.onpointermove = g.onpointerup = null; try { localStorage.setItem("wasaping.colpct", JSON.stringify(pct)); } catch {} };
    };
  });
  try { localStorage.removeItem("wasaping.colw"); } catch {} // widths in pixels, before 0.8.7
})();
try { const [k, d] = (localStorage.getItem("wasaping.sort") || "").split(" "); if (document.querySelector(`th[data-k="${k}"]`)) { sortKey = k; sortDir = d === "-1" ? -1 : 1; } } catch {}
$("filter").oninput = render;
$("rows").onclick = e => {
  if (e.target.closest("a")) return; // port links open the device page
  if (e.target.closest("td.ccell")) { // the Monitor checkbox, not the detail
    const box = e.target.closest("td.ccell").querySelector("input"), key = e.target.closest("tr[data-key]")?.dataset.key;
    if (e.target !== box) box.checked = !box.checked;
    const h = hosts.find(x => x.key === key) || gone.find(x => x.key === key);
    if (h) setCritical(h, box.checked);
    return;
  }
  const tr = e.target.closest("tr[data-key]");
  if (tr) openDetail(tr.dataset.key);
};

// ---- Detail panel ----
const findHost = key => hosts.find(h => h.key === key) || gone.find(h => h.key === key);

// Every name the device gave, by protocol (only the ones that say
// something more than "Nome in rete").
function netNames(h) {
  const n = h.names || {}, out = [];
  const add = (label, v, tip) => { if (v) out.push([`<span title="${esc(tip)}">${label}</span>`, `<span class="nt">${esc(v)}</span>`]); };
  if (n.dns && n.dns !== h.hostname) add("Nome DNS", n.dns, "Dal DNS (ricerca inversa)");
  if (n.mdns && n.mdns !== h.hostname) add("Nome Bonjour", n.mdns, "Da mDNS / Bonjour");
  if (n.netbios && n.netbios !== h.hostname) add("Nome SMB", n.netbios, "Da NetBIOS: il nome del computer per la condivisione file (Windows, NAS)");
  add("Gruppo SMB", n.workgroup, "Gruppo di lavoro o dominio Windows (NetBIOS)");
  if (n.dns && n.dns === h.hostname && (n.mdns || n.netbios)) add("Nome DNS", n.dns, "Dal DNS (ricerca inversa)");
  return out;
}
function openDetail(key) {
  const h = findHost(key);
  if (!h) { closeDetail(); return; }
  const changed = selected !== key;
  selected = key;
  const isGone = !hosts.includes(h);
  $("dTitle").innerHTML = kindIcon(h.kind) + `<span class="nt">${esc(label(h) || h.ip)}</span>`;
  $("dSub").textContent = (KIND[h.kind] || KIND[""])[0] + (isGone ? " · non risponde" : "");
  if (changed) { stopTool(); $("dToolHost").value = h.ip; $("dPings").hidden = $("dToolOut").hidden = true; }
  if (changed) {
    $("dKind").innerHTML = `<option value="">automatico${h.auto_kind ? "" : ` (${esc((KIND[h.kind] || KIND[""])[0])})`}</option>` +
      Object.entries(KIND).filter(([k]) => k).map(([k, v]) => `<option value="${k}">${esc(v[0])}</option>`).join("");
    $("dKind").value = h.auto_kind ? h.kind : "";
    $("dKindAuto").textContent = h.auto_kind ? `wasaping pensava: ${(KIND[h.auto_kind] || KIND[""])[0]}` : "";
    translateTree($("detail"));
  }
  if (changed) { $("dName").value = h.name || ""; $("dRole").value = h.role || ""; $("dNote").value = h.note || ""; $("dCrit").checked = !!h.critical; $("dMsg").textContent = ""; }
  $("dName").placeholder = (h.friendly || h.hostname) ? `vuoto = ${h.friendly || h.hostname}` : "es. Console FOH";
  $("dWarn").innerHTML = (h.warnings || []).map(w => `<li>${esc(w)}</li>`).join("");
  const svcs = [...new Set((h.services || []).map(s => SERVICES[s] ?? s).filter(Boolean))];
  const rows = [
    ["IP", `<span class="mono" data-copy="${esc(h.ip)}">${esc(h.ip)}</span>`],
    ["MAC", h.mac ? `<span class="mono" data-copy="${esc(h.mac)}">${esc(h.mac)}</span>` : "—"],
    ...((h.ipv6 || []).length ? [["IPv6", h.ipv6.map(a => `<span class="mono" data-copy="${esc(a)}" style="display:block;font-size:12px">${esc(a)}</span>`).join("")]] : []),
    ["Produttore", esc(h.vendor) || "—"],
    ...(h.model ? [["Modello", esc(h.model)]] : []),
    ...(h.friendly ? [["Nome del dispositivo", esc(h.friendly)]] : []),
    ...(h.web_title ? [["Pagina web", esc(h.web_title)]] : []),
    ...(h.swport ? [["Switch", `${esc(h.swport.switch)} · porta <b>${esc(h.swport.port)}</b>${h.swport.alias ? ` <span class="muted">(${esc(h.swport.alias)})</span>` : ""}`]] : []),
    ["Nome in rete", esc(h.hostname) || "—"],
    ...netNames(h),
    ...(h.protocols && h.protocols.length ? [["Protocolli", protoChips(h)]] : []),
    ...showFacts(h),
    ["Servizi", svcs.length ? svcs.map(s => `<span class="svc">${esc(s)}</span>`).join("") : "—"],
    ["Porte aperte", isGone ? "—" : portLinks(h) || "nessuna tra quelle controllate"],
    ["Risposta", isGone ? "—" : h.rtt_ms ? h.rtt_ms + " ms" : h.method === "arp" ? NOPING_TIP : "—"],
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
  $("dPorts").disabled = isGone && !portES; $("dPortMode").disabled = isGone;
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
  if ($("ctx").style.display === "block") hideMenu();
  else if ($("sheet").classList.contains("open")) closeSheet();
  else if (selected) closeDetail();
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

// Deep port scan of one device (like the detail panel of LanScan): common
// ports, all 65,535 or a list, with progress; a second click stops it.
let portES = null;
const stopPortScan = () => { if (portES) { portES.close(); portES = null; } $("dPorts").textContent = "Controlla porte"; $("dPortBar").hidden = true; };
$("dPortMode").onchange = () => { $("dPortList").hidden = $("dPortMode").value !== "list"; if (!$("dPortList").hidden) $("dPortList").focus(); };
$("dPorts").onclick = () => {
  if (portES) { stopPortScan(); $("dMsg").textContent = "Controllo delle porte fermato."; return; }
  const h = findHost(selected); if (!h) return;
  const mode = $("dPortMode").value, list = $("dPortList").value.trim();
  if (mode === "list" && !list) { $("dPortList").focus(); return; }
  const key = h.key;
  $("dPorts").textContent = "Ferma"; $("dPortBar").hidden = false; $("dPortBar").firstElementChild.style.width = "0%";
  $("dMsg").textContent = mode === "all" ? "Controllo tutte le 65.535 porte: circa un minuto…" : "Controllo le porte…";
  portES = new EventSource(`api/portscan?ip=${encodeURIComponent(h.ip)}&mode=${mode}&ports=${encodeURIComponent(list)}`);
  const found = open => open.length ? `aperte: ${open.join(", ")}` : "nessuna aperta finora";
  portES.addEventListener("progress", e => {
    const d = JSON.parse(e.data);
    $("dPortBar").firstElementChild.style.width = (100 * d.done / d.total).toFixed(1) + "%"; $("dPortBar").hidden = selected !== key;
    if (selected === key) $("dMsg").textContent = `${d.done.toLocaleString("it")} di ${d.total.toLocaleString("it")} · ${found(d.open)}`;
  });
  portES.addEventListener("done", e => {
    const d = JSON.parse(e.data), hh = findHost(key);
    stopPortScan();
    if (hh) hh.ports = [...new Set([...(hh.ports || []), ...d.open])].sort((a, b) => a - b);
    if (selected === key) {
      openDetail(key);
      $("dMsg").textContent = `✓ ${d.open.length} ${d.open.length === 1 ? "porta aperta" : "porte aperte"} su ${d.total.toLocaleString("it")} controllate, in ${Math.round(d.seconds)} s.`;
    }
    render();
  });
  portES.onerror = () => { if (portES) { stopPortScan(); $("dMsg").textContent = "Controllo interrotto."; } };
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

// Correct the type wasaping guessed: saved, and included in the diagnostics.
$("dKind").onchange = async () => {
  const h = findHost(selected); if (!h) return;
  const kind = $("dKind").value;
  try {
    await post("api/device", {key: h.key, net: h.net || "", kind});
    if (kind) { if (!h.auto_kind) h.auto_kind = h.kind; h.kind = kind; }
    else if (h.auto_kind) { h.kind = h.auto_kind; h.auto_kind = ""; }
    if (h.auto_kind === h.kind) h.auto_kind = "";
    selected = null; openDetail(h.key); render();
    $("dMsg").textContent = "✓ Tipo corretto: grazie, finisce anche nella diagnostica.";
  } catch (e) { $("dMsg").textContent = "Non salvato: " + e.message; }
};

// The devices on screen (filter and network included) as a table to paste
// into a spreadsheet, a chat or the rider.
function copyList() {
  const rows = shownKeys.map(findHost).filter(Boolean);
  if (!rows.length) { toast("Nessun dispositivo da copiare."); return; }
  const head = ["IP", "Nome", "Ruolo", "Tipo", "Modello", "MAC", "Protocolli", "Universi"].map(tr);
  const cell = v => String(v ?? "").replace(/[\t\n]+/g, " ");
  const text = [head].concat(rows.map(h => [h.ip, label(h), h.role, tr((KIND[h.kind] || KIND[""])[0]), h.model || h.vendor, h.mac, (h.protocols || []).join(", "), universeText(h)].map(cell)))
    .map(r => r.join("\t")).join("\n");
  copy(text, LANG === "en" ? `${rows.length} devices copied: paste them in a spreadsheet.` : `${rows.length} dispositivi copiati: incollali in un foglio di calcolo.`);
}

// Diagnostic bundle: a scan that records what devices answer, saved as a zip.
async function saveDiag() {
  openSheet("Diagnostica", `<p>wasaping fa una scansione registrando le risposte dei dispositivi (Bonjour, UPnP, Art-Net, sACN, PTP, PJLink…) e salva tutto in un file zip nella cartella Download.</p>
    <p class="muted">Serve quando un apparecchio non viene riconosciuto o ha un nome sbagliato: manda il file a chi sviluppa wasaping. Non contiene i nomi, i ruoli e le note che hai scritto, né PIN, password o indirizzi degli allarmi.</p>
    <div class="row"><button class="primary" id="diagGo">Crea il file di diagnostica</button></div><div id="diagOut"></div>`);
  $("diagGo").onclick = async () => {
    $("diagGo").disabled = true; $("diagOut").innerHTML = "<p>Scansione e registrazione in corso…</p>";
    try {
      const d = await post("api/diag", {}).then(r => r.json());
      $("diagOut").innerHTML = `<p>✓ Salvato: <b class="nt">${esc(d.path)}</b> (${Math.round(d.size / 1024)} KB)</p>`;
    } catch (e) { $("diagOut").innerHTML = `<p>Errore: ${esc(e.message)}</p>`; }
    $("diagGo").disabled = false;
  };
}

// Remove in one go the devices not seen for a while (never the critical
// ones). The numbers come from the program: it knows every device of the
// venue, also those outside the range scanned last.
function cleanupView() {
  if (needOneNet()) return;
  const opts = [[90, "non visti da più di 3 mesi"], [30, "non visti da più di 30 giorni"], [7, "non visti da più di una settimana"], [0, "tutti quelli che non rispondono ora"]];
  openSheet("Pulisci i dispositivi vecchi", `<p>Toglie dall'elenco di questo locale i dispositivi spariti. Se tornano, wasaping li ritrova come nuovi. I dispositivi <b>critici</b> restano sempre.</p>
    <div class="set">${opts.map(([d, l], i) => `<label class="check"><input type="radio" name="clDays" value="${d}"${i === 1 ? " checked" : ""}> <span>${esc(l)}</span> <span class="muted cl-n" data-d="${d}"></span></label>`).join("")}
    <label class="check"><input type="checkbox" id="clNamed"> anche quelli a cui hai dato un nome, un ruolo o una nota</label></div>
    <div class="row"><button class="primary" id="clGo" disabled></button></div>`);
  const ask = (days, named, dry) => post("api/forget-old", {net: activeNet, days, named, dry}).then(r => r.json()).then(r => r.removed);
  const days = () => +document.querySelector("input[name=clDays]:checked").value;
  const upd = async () => {
    const named = $("clNamed").checked;
    const counts = await Promise.all(opts.map(([d]) => ask(d, named, true).catch(() => 0)));
    if (!$("clGo")) return;
    document.querySelectorAll(".cl-n").forEach((x, i) => x.textContent = `(${counts[i]})`);
    const n = counts[opts.findIndex(([d]) => d === days())];
    $("clGo").textContent = LANG === "en" ? `Remove ${n} devices` : `Togli ${n} dispositivi`;
    $("clGo").disabled = !n;
  };
  document.querySelectorAll("input[name=clDays], #clNamed").forEach(x => x.onchange = upd);
  upd();
  $("clGo").onclick = async () => {
    $("clGo").disabled = true;
    try {
      const n = await ask(days(), $("clNamed").checked, false);
      toast(LANG === "en" ? `${n} devices removed.` : `${n} dispositivi tolti.`);
      const d = days(), named = $("clNamed").checked, limit = Date.now() - d * 86400000;
      gone = gone.filter(h => (activeNet && h.net !== activeNet) || h.critical || (!named && (h.name || h.role || h.note)) ||
        (d > 0 && new Date(h.last_seen).getTime() >= limit));
      closeSheet(); render();
    } catch (e) { toast("Errore: " + e.message); $("clGo").disabled = false; }
  };
}

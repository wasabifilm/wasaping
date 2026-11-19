// Show tools in sheets: live DMX, RDM, Dante, video, pre-show check,
// end-of-show report.
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
  openSheet("DMX", `<p class="muted">In ascolto su Art-Net (pacchetti in broadcast o diretti a questo computer) e sACN (universi 1–64). Solo ascolto: wasaping non manda DMX. Clic su un universo per vedere i 512 canali, clic su un canale per seguirlo nel tempo.</p>
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
// 512 levels as a strip of green, brighter where higher.
function drawStrip(cv, lv) {
  const ctx = cv.getContext("2d"), img = ctx.createImageData(512, 1);
  for (let i = 0; i < 512; i++) { const o = i * 4; img.data[o] = 129; img.data[o + 1] = 215; img.data[o + 2] = 66; img.data[o + 3] = lv[i] ? 40 + Math.round(215 * lv[i] / 255) : 18; }
  ctx.putImageData(img, 0, 0);
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
    drawStrip(row.querySelector("canvas"), b64bytes(u.data));
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
        : sel.protocol === "Art-Net" ? `<p class="muted">Per vedere gli apparecchi sopra i canali, fai una ricerca in Show → Apparecchi RDM.</p>` : ""}
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
      <p>${diff.length ? `<span>Differiscono su</span> <b>${diff.length}</b> <span>canali:</span> <span class="mono">${esc(ranges(diff))}</span>` : "Mandano gli stessi valori."}</p>`;
    $("dmxSrc").querySelectorAll("canvas").forEach(cv => drawStrip(cv, lv[+cv.dataset.k]));
  } else $("dmxSrc").innerHTML = "";
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

// RDM: the fixtures behind the Art-Net nodes; identify and, with two
// confirmations, change address, mode and label.
function rdmView() {
  const nodes = hosts.filter(h => h.artnet).length;
  openSheet("RDM", `<p>wasaping chiede ai nodi Art-Net quali apparecchi RDM hanno collegati e a ognuno chi è: produttore, modello, etichetta, indirizzo DMX, canali e modalità. La ricerca non cambia niente sugli apparecchi; per cambiare indirizzo, modalità o etichetta c'è «Modifica», che chiede conferma.</p>
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
          ${routeChanges(d)}
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
// What changed in the patch since the last session in this venue (the
// program saves it every time it is read).
function routeChanges(d) {
  const ch = d.changes || [];
  if (!d.changes_since || d.changes_since.startsWith("0001")) return "";
  const when = new Date(d.changes_since).toLocaleString(LANG === "en" ? "en-GB" : "it", {day: "numeric", month: "short", hour: "2-digit", minute: "2-digit"});
  if (!ch.length) return `<p class="st-ok">✓ ${LANG === "en" ? `Same patch as last time (${esc(when)}).` : `Stesso patch dell'ultima volta (${esc(when)}).`}</p>`;
  return `<div class="dncard"><b>${LANG === "en" ? `${ch.length} connections changed since ${esc(when)}` : `${ch.length} collegamenti cambiati dal ${esc(when)}`}</b>
    <div class="wrap" style="margin:6px 0 0"><table><thead><tr><th>Dispositivo</th><th>Destinazione</th><th>Prima</th><th>Ora</th></tr></thead><tbody>
    ${ch.map(c => `<tr><td class="nt">${esc(c.device)}</td><td class="nt">${esc(c.dest)}</td><td class="nt">${c.was ? esc(c.was) : '<span class="muted">—</span>'}</td><td class="nt ${c.now ? "" : "st-bad"}">${c.now ? esc(c.now) : (LANG === "en" ? "disconnected" : "scollegato")}</td></tr>`).join("")}
    </tbody></table></div></div>`;
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
      let html = routeChanges(d);
      html += devs.length ? devs.map(v => `<div class="dncard"><b class="nt">${esc(v.model || v.kind)}</b> <span class="muted"><span class="mono">${esc(v.ip)}</span>${hostName(v.ip) ? ` · <span class="nt">${esc(hostName(v.ip))}</span>` : ""}${v.version ? ` · v${esc(v.version)}` : ""}</span>
          ${v.error ? `<p class="bad">${esc(v.error)}</p>` : ""}
          ${(v.lines || []).map(l => `<div class="nt">${esc(l)}</div>`).join("")}
          ${(v.warn || []).map(l => `<div class="bad">⚠ <span>${esc(l)}</span></div>`).join("")}
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
async function preshow() {
  if (needOneNet()) return;
  openSheet("Pre-show", "<p>Controllo in corso: scansione, dispositivi critici, universi, piano IP…</p>");
  let rep;
  try {
    rep = await fetch(`api/preshow?net=${encodeURIComponent(activeNet)}&lang=${LANG}`).then(r => r.ok ? r.json() : r.text().then(t => { throw new Error(t); }));
  } catch (e) { openSheet("Pre-show", `<p>Errore: ${esc(e.message)}</p>`); return; }
  // The program may have run a fresh scan: show it.
  try { const last = await fetch("api/last").then(r => r.json()); if (last && last.at !== shownAt) applyResult(last); } catch {}
  const icon = c => c === "ok" ? "✓" : c === "bad" ? "✗" : "⚠";
  const body = `<div class="verdict v-${rep.verdict}"><b class="nt">${esc(rep.headline)}</b><span class="nt">${esc(rep.summary)}</span></div>
    <p class="muted nt">${esc(rep.network)} · ${esc(new Date(rep.at).toLocaleString(LANG === "en" ? "en-GB" : "it"))}</p>
    ${rep.sections.map(sec => `<h3 class="nt">${esc(sec.title)}</h3>${sec.note ? `<p class="muted nt">${esc(sec.note)}</p>` : ""}${sec.rows.length ? `<div class="wrap" style="margin:0"><table><tbody>${sec.rows.map(r => `<tr><td class="st-${r.class}">${icon(r.class)}</td><td class="nt"><b>${esc(r.name)}</b></td><td class="nt">${esc(r.role || "")}</td><td class="mono">${esc(r.ip || "")}</td><td class="nt">${esc(r.detail || "")}</td></tr>`).join("")}</tbody></table></div>` : ""}`).join("")}
    <div class="row"><button class="primary" id="preSave">Stampa o salva il report</button><button id="preAgain">Ricontrolla</button></div>`;
  openSheet("Pre-show", body);
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
    <div class="row"><button id="srShow" hidden>Ultimo show segnato</button><span>Dalle</span><input id="srSince" value="${def}" style="width:90px" title="18:00, 6h o 2026-11-18 18:00"><button class="primary" id="srGo">Mostra</button></div><div id="srOut"></div>`);
  const go = async (show) => {
    try {
      const r = await fetch(show ? "api/showreport?show=last" : `api/showreport?since=${encodeURIComponent($("srSince").value)}`).then(x => x.ok ? x.json() : x.text().then(t => { throw new Error(t); }));
      const devs = r.devices || [], ev = r.events || [];
      const t = d => new Date(d).toLocaleString(LANG === "en" ? "en-GB" : "it", {day: "numeric", month: "short", hour: "2-digit", minute: "2-digit", second: "2-digit"});
      $("srOut").innerHTML = `<div class="verdict v-${r.drops ? "warn" : "ok"}" style="margin-top:12px"><b>${r.drops ? `${r.drops} cadute` : "Nessuna caduta"}</b><span>${r.show ? (LANG === "en" ? "Show: " : "Show: ") : ""}dalle ${esc(t(r.since))} alle ${esc(t(r.until))}</span></div>
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
  $("srGo").onclick = () => go(false); $("srSince").onkeydown = e => { if (e.key === "Enter") go(false); };
  $("srShow").onclick = () => go(true);
  // A show marked with "Inizio show" (Monitor or OSC): report that one.
  fetch("api/showreport?show=info").then(r => r.json()).then(i => { if ($("srShow")) { $("srShow").hidden = !i.ok; } go(!!i.ok); }).catch(() => go(false));
}


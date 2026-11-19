// Network tools in sheets: IP plan (and its editor), free IPs, comparison
// with last time, network map, the show's project file.
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
document.addEventListener("keydown", e => { if (e.key === "Escape" && $("sheet").classList.contains("open")) closeSheet(); });


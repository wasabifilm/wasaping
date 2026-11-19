// The tools, in one list: the sections of the page (Show, Rete, Progetto)
// are drawn from it, and the macOS menu bar calls them by id
// (window.wasapingTool). Ids must match toolMenu in native_darwin.go.
// needs: "scan" (a scan done in this window) or "network" (a known network).
const TOOLS = [
  {id: "preshow", group: "show", label: "Pre-show", desc: "Controlla tutto prima dello show e prepara un report verde/giallo/rosso.", fn: () => preshow()},
  {id: "monitor", group: "show", label: "Monitor", desc: "Tiene d'occhio dispositivi e universi critici, e suona se uno sparisce.", fn: () => openMonitor()},
  {id: "dmx", group: "show", label: "DMX", desc: "Art-Net e sACN dal vivo: sorgenti, canali, patch, due console a confronto.", fn: () => dmxView()},
  {id: "rdm", group: "show", label: "RDM", desc: "Gli apparecchi dietro i nodi Art-Net: indirizzi, modalità, Identifica, Modifica.", fn: () => rdmView()},
  {id: "dante", group: "show", label: "Dante", desc: "Dispositivi, clock e routing in sola lettura.", fn: () => danteView()},
  {id: "video", group: "show", label: "Video", desc: "ATEM, HyperDeck, Videohub, vMix, proiettori, NDI.", fn: () => videoView()},
  {id: "report", group: "project", label: "Report di fine show", desc: "Cadute, tempi giù e tempi di risposta della serata.", fn: () => showReportView()},
  {id: "plan", group: "net", label: "Piano IP", desc: "Il patch previsto, confrontato con la rete. Si importa o si scrive qui.", fn: () => planView()},
  {id: "free", group: "net", label: "IP liberi", desc: "Gli indirizzi liberi, per dare un IP fisso a un apparecchio nuovo.", fn: () => freeView()},
  {id: "map", group: "net", label: "Mappa della rete", desc: "Switch gestiti (SNMP): porte, velocità e chi è collegato dove.", fn: () => switchView(), needs: "scan"},
  {id: "compare", group: "net", label: "Confronta con l'ultima volta", desc: "Chi c'è di nuovo, chi manca, chi ha cambiato IP rispetto all'altra volta.", fn: () => compareView(), needs: "network"},
  {id: "save", group: "project", label: "Salva progetto…", desc: "Tutto il lavoro su questo locale in un file .wasaping.", fn: () => saveProjectView(), needs: "network"},
  {id: "open", group: "project", label: "Apri progetto…", desc: "Riapre un progetto, anche di un locale dove questo computer non è mai stato.", fn: () => pickFile(".wasaping,.json", openProjectView)},
  {id: "cleanup", group: "project", label: "Pulisci i dispositivi vecchi", desc: "Toglie dall'elenco quelli spariti da tempo (i critici restano).", fn: () => cleanupView(), needs: "network"},
  {id: "copy", group: "project", label: "Copia elenco", desc: "I dispositivi mostrati (con il filtro), da incollare in un foglio di calcolo o in una chat.", fn: () => copyList(), needs: "scan"},
  {id: "labels", group: "project", label: "Etichette con QR", desc: "Adesivi per gli apparecchi: nome, IP, ruolo e un QR che apre la loro pagina web.", fn: () => labelsView(), needs: "scan"},
  {id: "print", group: "project", label: "Stampa elenco IP", desc: "Per la crew e il rider, da stampare o salvare in PDF.", fn: () => printList(), needs: "scan"},
  {id: "csv", group: "project", label: "Esporta CSV", desc: "L'elenco in un foglio di calcolo.", fn: () => exportCSV(), needs: "scan"},
  {id: "json", group: "project", label: "Esporta JSON", desc: "L'elenco completo, per altri programmi.", fn: () => exportJSON(), needs: "scan"},
  {id: "diag", group: "project", label: "Salva diagnostica", desc: "Un file da mandare a chi sviluppa wasaping quando qualcosa non va.", fn: () => saveDiag()},
  {id: "settings", group: "", label: "Impostazioni", fn: () => settingsView()},
];

// Why a tool cannot run now ("" = it can).
function toolBlocked(t) {
  if (t.needs === "scan" && !hosts.length) return "Fai prima una scansione.";
  if (t.needs === "network" && !network) return "Fai prima una scansione di questa rete.";
  return "";
}
function runTool(id) {
  const t = TOOLS.find(x => x.id === id);
  if (!t) return;
  const why = toolBlocked(t);
  if (why) { toast(why); return; }
  hideMenu();
  currentTool = id;
  t.fn();
}
window.wasapingTool = runTool;

// Rete and Progetto: a page with their tools, like the other pages.
const GROUPS = {net: "Rete", project: "Progetto"};
function groupPage(g) {
  currentTool = g;
  openSheet(GROUPS[g], `<div class="toolgrid">${TOOLS.filter(t => t.group === g).map(t => {
    const why = toolBlocked(t);
    return `<button class="tool${why ? " dis" : ""}" data-id="${t.id}"><b>${esc(t.label)}</b><span>${esc(t.desc)}</span>${why ? `<em>${esc(why)}</em>` : ""}</button>`;
  }).join("")}</div>`);
  $("shBody").querySelectorAll("button.tool").forEach(b => b.onclick = () => runTool(b.dataset.id));
}
// After a scan, a group page on screen shows what it unlocked.
function refreshGroupPage() { if (GROUPS[currentTool] && $("sheet").classList.contains("open")) groupPage(currentTool); }
// The header: every page of wasaping, always at hand (also from the
// Monitor: a tool page opens above it).
const QUICK = [["list", "Dispositivi"], ["preshow", "Pre-show"], ["monitor", "Monitor"], ["dmx", "DMX"], ["rdm", "RDM"], ["dante", "Dante"], ["video", "Video"], ["net", "Rete"], ["project", "Progetto"]];
const quickTip = id => id === "list" ? "L'elenco dei dispositivi" : id === "net" ? "Piano IP, IP liberi, mappa della rete, confronto" : id === "project" ? "Salva e apri, report, stampa, esportazioni, diagnostica" : TOOLS.find(t => t.id === id).desc;
const mod = /Mac/.test(navigator.platform) ? "⌘" : "Ctrl+";
$("quick").innerHTML = QUICK.map(([id, l], i) => `<button data-id="${id}" title="${esc(tr(quickTip(id)))} · ${mod}${i + 1}">${esc(l)}</button>`).join("");
$("quick").querySelectorAll("button").forEach(b => b.onclick = () => {
  const id = b.dataset.id;
  if (id === "list") goHome(); else if (GROUPS[id]) groupPage(id); else runTool(id);
});
// Green = the page on screen: a tool, the Monitor or the device list.
function markQuick() {
  const sheet = $("sheet").classList.contains("open"), mon = $("monitor").classList.contains("open");
  const t = TOOLS.find(x => x.id === currentTool);
  const on = !sheet ? (mon ? "monitor" : "list") : t && GROUPS[t.group] ? t.group : currentTool; // a Rete tool lights Rete
  $("quick").querySelectorAll("button").forEach(b => b.classList.toggle("on", b.dataset.id === on));
  $("setBtn").classList.toggle("on", on === "settings");
}
markQuick();

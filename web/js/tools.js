// The tools, in one list: the sections of the page (Show, Rete, Progetto)
// are drawn from it, and the macOS menu bar calls them by id
// (window.wasapingTool). Ids must match toolMenu in native_darwin.go.
// needs: "scan" (a scan done in this window) or "network" (a known network).
const TOOLS = [
  {id: "preshow", group: "show", label: "Check pre-show", desc: "Controlla tutto prima dello show e prepara un report verde/giallo/rosso.", fn: () => preshow()},
  {id: "monitor", group: "show", label: "Monitor", desc: "Tiene d'occhio dispositivi e universi critici, e suona se uno sparisce.", fn: () => openMonitor()},
  {id: "dmx", group: "show", label: "Universi DMX", desc: "Art-Net e sACN dal vivo: sorgenti, canali, patch, due console a confronto.", fn: () => dmxView()},
  {id: "rdm", group: "show", label: "Apparecchi RDM", desc: "Gli apparecchi dietro i nodi Art-Net: indirizzi, modalità, Identifica, Modifica.", fn: () => rdmView()},
  {id: "dante", group: "show", label: "Dante", desc: "Dispositivi, clock e routing in sola lettura.", fn: () => danteView()},
  {id: "video", group: "show", label: "Video", desc: "ATEM, HyperDeck, Videohub, vMix, proiettori, NDI.", fn: () => videoView()},
  {id: "report", group: "show", label: "Report di fine show", desc: "Cadute, tempi giù e tempi di risposta della serata.", fn: () => showReportView()},
  {id: "plan", group: "net", label: "Piano IP", desc: "Il patch previsto, confrontato con la rete. Si importa o si scrive qui.", fn: () => planView()},
  {id: "free", group: "net", label: "IP liberi", desc: "Gli indirizzi liberi, per dare un IP fisso a un apparecchio nuovo.", fn: () => freeView()},
  {id: "map", group: "net", label: "Mappa della rete", desc: "Switch gestiti (SNMP): porte, velocità e chi è collegato dove.", fn: () => switchView(), needs: "scan"},
  {id: "compare", group: "net", label: "Confronta con l'ultima volta", desc: "Chi c'è di nuovo, chi manca, chi ha cambiato IP rispetto all'altra volta.", fn: () => compareView(), needs: "network"},
  {id: "save", group: "project", label: "Salva progetto…", desc: "Tutto il lavoro su questo locale in un file .wasaping.", fn: () => saveProjectView(), needs: "network"},
  {id: "open", group: "project", label: "Apri progetto…", desc: "Riapre un progetto, anche di un locale dove questo computer non è mai stato.", fn: () => pickFile(".wasaping,.json", openProjectView)},
  {id: "cleanup", group: "project", label: "Pulisci i dispositivi vecchi", desc: "Toglie dall'elenco quelli spariti da tempo (i critici restano).", fn: () => cleanupView(), needs: "network"},
  {id: "print", group: "project", label: "Stampa elenco IP", desc: "Per la crew e il rider, da stampare o salvare in PDF.", fn: () => $("printBtn").click(), needs: "scan"},
  {id: "csv", group: "project", label: "Esporta CSV", desc: "L'elenco in un foglio di calcolo.", fn: () => $("csv").click(), needs: "scan"},
  {id: "json", group: "project", label: "Esporta JSON", desc: "L'elenco completo, per altri programmi.", fn: () => $("json").click(), needs: "scan"},
  {id: "diag", group: "project", label: "Salva diagnostica", desc: "Un file da mandare a chi sviluppa wasaping quando qualcosa non va.", fn: () => saveDiag()},
  {id: "settings", group: "", label: "Impostazioni", fn: () => settingsView()},
];
const SECTIONS = [["list", "Dispositivi"], ["show", "Show"], ["net", "Rete"], ["project", "Progetto"]];

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
  t.fn();
}
window.wasapingTool = runTool;

let section = "list";
try { section = localStorage.getItem("wasaping.section") || "list"; } catch {}
if (!SECTIONS.some(s => s[0] === section)) section = "list";
function showSection(id) {
  section = id;
  try { localStorage.setItem("wasaping.section", id); } catch {}
  document.querySelectorAll("#tabs button").forEach(b => b.classList.toggle("on", b.dataset.s === id));
  document.body.classList.toggle("in-tools", id !== "list");
  drawTools();
}
function drawTools() {
  const box = $("toolsView");
  if (section === "list") { box.innerHTML = ""; return; }
  box.innerHTML = `<div class="toolgrid">${TOOLS.filter(t => t.group === section).map(t => {
    const why = toolBlocked(t);
    return `<button class="tool${why ? " dis" : ""}" data-id="${t.id}"><b>${esc(t.label)}</b><span>${esc(t.desc)}</span>${why ? `<em>${esc(why)}</em>` : ""}</button>`;
  }).join("")}</div>`;
  box.querySelectorAll("button.tool").forEach(b => b.onclick = () => runTool(b.dataset.id));
}
$("tabs").innerHTML = SECTIONS.map(([id, l]) => `<button data-s="${id}">${esc(l)}</button>`).join("");
$("tabs").querySelectorAll("button").forEach(b => b.onclick = () => showSection(b.dataset.s));
showSection(section);

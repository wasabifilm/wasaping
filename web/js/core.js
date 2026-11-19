// wasaping: helpers shared by the other scripts.
const $ = id => document.getElementById(id);
// Errors of the page go to the program: they end up in the diagnostic file
// (the app window has no console to look at).
const pageErrors = [];
const reportError = (msg, where) => {
  if (pageErrors.length > 20) return;
  pageErrors.push(msg);
  try { fetch("api/clientlog", {method: "POST", headers: {"Content-Type": "application/json"}, body: JSON.stringify({msg: String(msg).slice(0, 500), where: String(where || "").slice(0, 200)})}); } catch {}
};
window.addEventListener("error", e => reportError(e.message, `${e.filename || ""}:${e.lineno || ""}:${e.colno || ""}`));
window.addEventListener("unhandledrejection", e => reportError(e.reason && (e.reason.stack || e.reason.message) || e.reason, "promise"));

// The main header stays visible: Monitor, sheets and the detail panel start
// below it (CSS --head).
const mainHeader = document.querySelector("body > header");
const setHead = () => document.documentElement.style.setProperty("--head", mainHeader.offsetHeight + "px");
setHead();
if (window.ResizeObserver) new ResizeObserver(setHead).observe(mainHeader); else addEventListener("resize", setHead);

// Development (WASAPING_WEB_DIR): reload the page when a file of it changes.
fetch("api/dev").then(r => r.ok ? r.json() : null).then(d => {
  if (!d) return;
  let seen = d.changed;
  setInterval(() => fetch("api/dev").then(r => r.json()).then(x => { if (x.changed !== seen) location.reload(); }).catch(() => {}), 1000);
}).catch(() => {});

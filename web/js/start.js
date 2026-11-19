// Logo and name: back to the device list, from any page.
const goHome = () => {
  if ($("monitor").classList.contains("open")) closeMonitor();
  if ($("sheet").classList.contains("open")) closeSheet();
};
document.querySelectorAll("body > header .logo, body > header h1").forEach(el => el.onclick = goHome);
// Start.
init();

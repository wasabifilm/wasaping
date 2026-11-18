package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

// startAppWindow opens url in a chromeless window of Edge or Chrome (an "app"
// window, no address bar), with its own profile so the process ends when the
// window is closed. It returns nil when no such browser is installed.
func startAppWindow(url string) *exec.Cmd {
	exe := findAppBrowser()
	if exe == "" {
		return nil
	}
	profile := filepath.Join(os.TempDir(), "wasaping-window")
	if dir, err := os.UserCacheDir(); err == nil {
		profile = filepath.Join(dir, "wasaping", "window")
	}
	cmd := exec.Command(exe,
		"--app="+url,
		"--user-data-dir="+profile,
		"--no-first-run",
		"--no-default-browser-check",
		"--window-size=1120,720",
	)
	if err := cmd.Start(); err != nil {
		return nil
	}
	return cmd
}

func findAppBrowser() string {
	var candidates []string
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LocalAppData"} {
			base := os.Getenv(env)
			if base == "" {
				continue
			}
			candidates = append(candidates,
				filepath.Join(base, `Microsoft\Edge\Application\msedge.exe`),
				filepath.Join(base, `Google\Chrome\Application\chrome.exe`))
		}
	case "darwin":
		for _, app := range []string{"Google Chrome", "Microsoft Edge", "Chromium", "Brave Browser"} {
			candidates = append(candidates, "/Applications/"+app+".app/Contents/MacOS/"+app)
		}
	default:
		for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
			if p, err := exec.LookPath(name); err == nil {
				candidates = append(candidates, p)
			}
		}
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

var reExportName = regexp.MustCompile(`^wasaping-[0-9A-Za-z-]+\.(csv|json|html)$`)

// handleSave writes an export into the Downloads folder and shows it. App
// windows cannot always download files from the page, so the server does it.
func handleSave(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "usa POST", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
		Data string `json:"data"`
		Open bool   `json:"open"` // open it (printable list) instead of showing it in the folder
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<20)).Decode(&req); err != nil {
		http.Error(w, "richiesta non valida", http.StatusBadRequest)
		return
	}
	if !reExportName.MatchString(req.Name) {
		http.Error(w, "nome file non valido", http.StatusBadRequest)
		return
	}
	dir := downloadsDir()
	path := filepath.Join(dir, req.Name)
	ext := filepath.Ext(req.Name)
	for i := 2; ; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", req.Name[:len(req.Name)-len(ext)], i, ext))
	}
	if err := os.WriteFile(path, []byte(req.Data), 0o644); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if req.Open && ext == ".html" {
		openFile(path)
	} else {
		revealFile(path)
	}
	writeJSON(w, map[string]string{"path": path})
}

func downloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return os.TempDir()
	}
	if d := filepath.Join(home, "Downloads"); dirExists(d) {
		return d
	}
	return home
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// openFile opens a file with its default program (the browser for .html).
func openFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	cmd.Start()
}

// revealFile shows the file in Finder / Explorer / the file manager.
func revealFile(path string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", "-R", path)
	case "windows":
		cmd = exec.Command("explorer", "/select,", path)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(path))
	}
	cmd.Start()
}

// Command debgen builds a .deb package of wasaping (Debian, Ubuntu,
// Raspberry Pi OS) from a Linux binary, without dpkg-deb: a .deb is an ar
// archive with debian-binary, control.tar.gz and data.tar.gz.
//
//	go run ./scripts/debgen -bin dist/wasaping-linux-arm64 -arch arm64 -version v0.7.0 -out dist
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const service = `# wasaping in box mode: monitor of the show network, always on.
[Unit]
Description=wasaping box (monitor della rete show)
Wants=network-online.target
After=network-online.target

[Service]
User=wasaping
ExecStart=/usr/bin/wasaping box
Restart=always
RestartSec=3
# Listen to PTP (ports 319/320) and ping without being root.
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_RAW
CapabilityBoundingSet=CAP_NET_BIND_SERVICE CAP_NET_RAW
NoNewPrivileges=true
Environment=LANG=it_IT.UTF-8

[Install]
WantedBy=multi-user.target
`

const postinst = `#!/bin/sh
set -e
if [ "$1" = configure ]; then
	if ! getent passwd wasaping >/dev/null; then
		useradd --system --create-home --home-dir /var/lib/wasaping --shell /usr/sbin/nologin wasaping
	fi
	if [ -d /run/systemd/system ]; then
		systemctl daemon-reload || true
		if systemctl is-enabled --quiet wasaping 2>/dev/null; then
			systemctl restart wasaping || true
		else
			echo "wasaping: per il box sempre acceso:  sudo systemctl enable --now wasaping"
			echo "wasaping: PIN per l'iPad:            sudo journalctl -u wasaping | grep PIN"
		fi
	fi
fi
exit 0
`

const prerm = `#!/bin/sh
set -e
if [ "$1" = remove ] && [ -d /run/systemd/system ]; then
	systemctl disable --now wasaping >/dev/null 2>&1 || true
fi
exit 0
`

const postrm = `#!/bin/sh
set -e
if [ -d /run/systemd/system ]; then systemctl daemon-reload || true; fi
if [ "$1" = purge ]; then
	userdel wasaping >/dev/null 2>&1 || true
	rm -rf /var/lib/wasaping
fi
exit 0
`

type file struct {
	name string
	mode int64
	data []byte
}

func tgz(files []file, dirs []string, mtime time.Time) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, d := range dirs {
		tw.WriteHeader(&tar.Header{Name: "./" + d + "/", Mode: 0o755, Typeflag: tar.TypeDir, ModTime: mtime, Uname: "root", Gname: "root", Format: tar.FormatGNU})
	}
	for _, f := range files {
		tw.WriteHeader(&tar.Header{Name: "./" + f.name, Mode: f.mode, Size: int64(len(f.data)), Typeflag: tar.TypeReg, ModTime: mtime, Uname: "root", Gname: "root", Format: tar.FormatGNU})
		tw.Write(f.data)
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func arMember(w *bytes.Buffer, name string, data []byte, mtime time.Time) {
	fmt.Fprintf(w, "%-16s%-12d%-6d%-6d%-8o%-10d`\n", name, mtime.Unix(), 0, 0, 0o100644, len(data))
	w.Write(data)
	if len(data)%2 == 1 {
		w.WriteByte('\n')
	}
}

// debVersion turns a git version (v0.7.0, v0.7.0-3-gabc-dirty, abc123) into
// one dpkg accepts.
func debVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	v = regexp.MustCompile(`[^A-Za-z0-9.+~-]`).ReplaceAllString(v, "")
	if v == "" || v[0] < '0' || v[0] > '9' {
		v = "0.0.0+" + v
	}
	return strings.ReplaceAll(v, "-", "+") // no Debian revision: keep it all upstream
}

func main() {
	bin := flag.String("bin", "", "Linux executable")
	arch := flag.String("arch", "arm64", "Debian architecture (amd64, arm64, armhf)")
	version := flag.String("version", "dev", "version")
	out := flag.String("out", "dist", "output folder")
	flag.Parse()
	exe, err := os.ReadFile(*bin)
	if err != nil {
		log.Fatal(err)
	}
	ver := debVersion(*version)
	mtime := time.Now().Truncate(time.Second)
	if st, err := os.Stat(*bin); err == nil {
		mtime = st.ModTime().Truncate(time.Second)
	}
	data := tgz([]file{
		{"usr/bin/wasaping", 0o755, exe},
		{"usr/lib/systemd/system/wasaping.service", 0o644, []byte(service)},
		{"usr/share/doc/wasaping/README", 0o644, []byte("wasaping — a spicy little IP scanner\nhttps://github.com/wasabifilm/wasaping\n\nBox sempre acceso:  sudo systemctl enable --now wasaping\nGuida:              https://github.com/wasabifilm/wasaping/blob/main/docs/raspberry.md\n")},
	}, []string{"usr", "usr/bin", "usr/lib", "usr/lib/systemd", "usr/lib/systemd/system", "usr/share", "usr/share/doc", "usr/share/doc/wasaping"}, mtime)
	control := fmt.Sprintf(`Package: wasaping
Version: %s
Architecture: %s
Maintainer: wasaping <wasaping@users.noreply.github.com>
Installed-Size: %d
Recommends: iputils-ping
Section: net
Priority: optional
Homepage: https://github.com/wasabifilm/wasaping
Description: a spicy little IP scanner, for show networks
 Finds the devices on the network (with the show gear: Art-Net, sACN,
 Dante, NDI, PTP, RDM), monitors the critical ones and, as a box on a
 Raspberry Pi, shows the monitor on an iPad.
`, ver, *arch, (len(exe)+len(service)+1023)/1024)
	ctl := tgz([]file{
		{"control", 0o644, []byte(control)},
		{"postinst", 0o755, []byte(postinst)},
		{"prerm", 0o755, []byte(prerm)},
		{"postrm", 0o755, []byte(postrm)},
	}, nil, mtime)
	var deb bytes.Buffer
	deb.WriteString("!<arch>\n")
	arMember(&deb, "debian-binary", []byte("2.0\n"), mtime)
	arMember(&deb, "control.tar.gz", ctl, mtime)
	arMember(&deb, "data.tar.gz", data, mtime)
	name := filepath.Join(*out, fmt.Sprintf("wasaping-linux-%s.deb", *arch)) // fixed name: releases/latest/download works
	if err := os.WriteFile(name, deb.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Println("→", name)
}

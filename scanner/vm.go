package scanner

import "strings"

// vmPrefixes are the MAC prefixes that hypervisors give their virtual
// machines' network cards.
var vmPrefixes = map[string]string{
	"bc2411": "Proxmox",
	"525400": "QEMU/KVM",
	"005056": "VMware", "000c29": "VMware", "000569": "VMware", "001c14": "VMware",
	"080027": "VirtualBox",
	"00155d": "Hyper-V",
	"00163e": "Xen",
	"001c42": "Parallels",
}

// VirtualMachine names the hypervisor when the MAC is one of a virtual
// machine, "" otherwise.
func VirtualMachine(mac string) string {
	h := strings.ToLower(strings.NewReplacer(":", "", "-", "", ".", "").Replace(mac))
	if len(h) < 6 {
		return ""
	}
	return vmPrefixes[h[:6]]
}

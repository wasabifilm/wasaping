package main

import (
	"os"
	"reflect"
	"testing"
)

func TestReleaseHighlights(t *testing.T) {
	body := "**a spicy little IP scanner** 🟢: intro.\r\n\r\n## Novità\r\n\r\n- **Reti via VPN o router:** wasaping legge…\r\n- **Programmi show da Bonjour:** Millumin…\r\n  - sotto-punto\r\n- **IP pubblico** (facoltativo): indirizzo.\r\n- Elenco più comodo: colonne.\r\n\r\n## Download\r\n\r\n- non questo\r\n"
	got := releaseHighlights(body)
	want := []string{"Reti via VPN o router", "Programmi show da Bonjour", "IP pubblico", "Elenco più comodo"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%q", got)
	}
	// The real notes of 0.8.9, when the scratch copy is around.
	if b, err := os.ReadFile(os.Getenv("WASAPING_NOTES")); err == nil {
		t.Logf("%q", releaseHighlights(string(b)))
	}
}

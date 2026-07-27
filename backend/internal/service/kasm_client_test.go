package service

import "testing"

func TestKasmUsernameIsScopedByAccount(t *testing.T) {
	c := &KasmClient{seedNamespace: "prod"}

	got := c.kasmUsername(103, 146)
	if want := "surplus-prod-u103-a146@kasm.local"; got != want {
		t.Fatalf("kasm username = %q, want %q", got, want)
	}
	if got == c.kasmUsername(103, 102) {
		t.Fatal("different accounts must not share a Kasm username")
	}
}

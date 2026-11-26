package server

import "testing"

func FuzzValidID(f *testing.F) {
	f.Add("ABCDEFGHIJKLMNOPQRSTUVWXYZ")
	f.Add("short")
	f.Add("")
	f.Fuzz(func(t *testing.T, id string) {
		_ = validID(id)
	})
}

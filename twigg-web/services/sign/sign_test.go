package sign

import "testing"

// Run with: go test -fuzz=FuzzSignAndCheck -fuzztime=10s

func FuzzSignAndCheck(f *testing.F) {

	f.Add("my message")
	f.Add("")
	f.Add("hello world")
	f.Add("test@example.com")
	f.Add("a.b.c")

	signer := NewSigner([]byte("my-secret-key"))

	f.Fuzz(func(t *testing.T, msg string) {
		signed := signer.SignAndAppend(msg)
		gotMsg, ok := signer.VerifyAndExtract(signed)
		if !ok {
			t.Fatalf("expected valid signature")
		}
		if gotMsg != msg {
			t.Fatalf("expected %q, got %q", msg, gotMsg)
		}

		brokenSigned := signed + " "
		_, ok = signer.VerifyAndExtract(brokenSigned)
		if ok {
			t.Fatal("expected wrong sig")
		}
	})
}

// Run with: go test -fuzz=FuzzTamperedSig -fuzztime=10s
func FuzzTamperedSig(f *testing.F) {

	f.Add("my message", "tampered sig")
	signer := NewSigner([]byte("my-super-secret-key"))

	f.Fuzz(func(t *testing.T, msg string, tamperedSig string) {
		signed := signer.SignAndAppend(msg)
		if signed == tamperedSig {
			return
		}
		_, ok := signer.VerifyAndExtract(tamperedSig)
		if ok {
			t.Fatalf("expected invalid signature")
		}
	})
}
package tse

import (
	"bytes"
	"fmt"
	"testing"
)

func TestScrubber_NoSecrets(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, nil)
	if err != nil {
		t.Fatal(err)
	}

	input := "hello world"

	_, err = scrubber.Write([]byte(input))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	if out.String() != input {
		t.Fatalf("expected %q got %q", input, out.String())
	}
}

func TestScrubber_SimpleSecret(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = scrubber.Write([]byte("token=SECRET"))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "token=" + string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_MultipleSecrets(t *testing.T) {
	var out bytes.Buffer

	w, close, err := NewTseScrubber(&out, []string{"AAA", "BBB"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = w.Write([]byte("AAA A B BBB"))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := string(MaskPlaceholder) + " A B " + string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_SplitSecretAcrossWrites(t *testing.T) {
	var out bytes.Buffer

	w, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	w.Write([]byte("SE"))
	w.Write([]byte("CRET"))

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_ByteByByteWrites(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, []string{"ABC"})
	if err != nil {
		t.Fatal(err)
	}

	scrubber.Write([]byte("A"))
	scrubber.Write([]byte("B"))
	scrubber.Write([]byte("C"))

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_SecretAtBeginningAndEnd(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, []string{"KEY"})
	if err != nil {
		t.Fatal(err)
	}

	scrubber.Write([]byte("KEY middle KEY"))

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := string(MaskPlaceholder) + " middle " + string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_InvalidSecretAndNoSecrets(t *testing.T) {
	var out bytes.Buffer

	// Invalid secret
	_, _, err := NewTseScrubber(&out, []string{""})

	if err == nil {
		t.Fatal("expected error for empty secret")
	}

	// No secrets
	scrubberWithNoSecrets, close, err := NewTseScrubber(&out, []string{})
	if err != nil {
		t.Fatal("expected no error for empty secrets")
	}

	input := "scrubber with no secrets works!"
	_, err = scrubberWithNoSecrets.Write([]byte(input))
	if err != nil {
		t.Fatal(err)
	}
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	if out.String() != input {
		t.Fatalf("expected %q got %q", input, out.String())
	}
}

func TestScrubber_OverlappingSecrets(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, []string{"token_123", "token_123456"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = scrubber.Write([]byte("token_123456"))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}
func TestScrubber_LargeWriteSecretsOnEdges(t *testing.T) {
	var out bytes.Buffer

	scrubber, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	// Start with the beginning of a secret split across writes.
	_, err = scrubber.Write([]byte("---SEC"))
	if err != nil {
		t.Fatal(err)
	}

	// Large body to simulate a realistic log line.
	largeMiddle := bytes.Repeat([]byte("A"), 1_000)

	// Finish the first secret and append a large body.
	input := append([]byte("RET"), largeMiddle...)

	// End the chunk with the beginning of another secret.
	input = append(input, []byte("---SEC")...)

	_, err = scrubber.Write(input)
	if err != nil {
		t.Fatal(err)
	}

	// Finish the second secret.
	_, err = scrubber.Write([]byte("RET"))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "---" + string(MaskPlaceholder) + string(largeMiddle) + "---" + string(MaskPlaceholder)

	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_WriteFailureAndRecovery(t *testing.T) {
	fakeW := &fakeWriter{t: t}

	scrubber, close, err := NewTseScrubber(fakeW, []string{"SHH"})
	if err != nil {
		t.Fatal(err)
	}

	// Write will fail internally.
	fakeW.failNextWrite = true
	n, err := scrubber.Write([]byte("12345678"))
	if err != nil {
		t.Fatal("expected not write error")
	}
	if n != 8 {
		t.Fatal("expected n to be 8")
	}

	// Second write should not err
	fakeW.failNextWrite = false
	n, err = scrubber.Write([]byte("910"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("expected n to be 3")
	}

	// Final close should flush remaining data
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "12345678910"

	if fakeW.String() != expected {
		t.Fatalf("unexpected output: %q", fakeW.String())
	}
}

func TestScrubber_WriteAndCleanupFailure(t *testing.T) {
	fakeW := &fakeWriter{t: t}

	scrubber, close, err := NewTseScrubber(fakeW, []string{"SHH"})
	if err != nil {
		t.Fatal(err)
	}

	// Write will fail internally.
	fakeW.failNextWrite = true
	n, err := scrubber.Write([]byte("12345678"))
	if err != nil {
		t.Fatal("expected not write error")
	}
	if n != 8 {
		t.Fatal("expected n to be 8")
	}

	// Cleanup write should err
	fakeW.failNextWrite = true
	n, err = scrubber.Write([]byte("910"))
	if err == nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatal("expected n to be 0")
	}

	// Since last write failed and n == 0, we should call write with "910"
	// and now make write not fail.
	fakeW.failNextWrite = false
	n, err = scrubber.Write([]byte("910"))
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatal("expected n to be 3")
	}

	// Final close should flush remaining data
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "12345678910"

	if fakeW.String() != expected {
		t.Fatalf("unexpected output: %q", fakeW.String())
	}
}
func TestScrubber_Flush_FlushesOnlySafe(t *testing.T) {
	var out bytes.Buffer

	s, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Write([]byte("123SECRE"))
	if err != nil {
		t.Fatal(err)
	}

	err = s.Flush()
	if err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}

	expected := "123"
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}

	// Call flush multiple times should not change
	if err = s.Flush(); err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}
	if err = s.Flush(); err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}
	if err = s.Flush(); err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}
	if err = s.Flush(); err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}
	if err = s.Flush(); err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}

	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}
}

func TestScrubber_Flush_BoundaryThenComplete(t *testing.T) {
	var out bytes.Buffer

	s, close, err := NewTseScrubber(&out, []string{"XYZ"})
	if err != nil {
		t.Fatal(err)
	}

	s.Write([]byte("abcXY"))
	err = s.Flush()
	if err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}

	// "abc" flushed, "XY" kept
	if out.String() != "abc" {
		t.Fatalf("expected %q got %q", "abc", out.String())
	}

	_, err = s.Write([]byte("Z"))
	if err != nil {
		t.Fatal(err)
	}
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "abc" + string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_Flush_EmptyBuffer(t *testing.T) {
	var out bytes.Buffer

	s, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	err = s.Flush()
	if err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}

	if out.Len() != 0 {
		t.Fatalf("expected empty output, got %q", out.String())
	}
	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}
}

type fakeWriter struct {
	buf           bytes.Buffer
	failNextWrite bool // if true writes only first half of bytes and fails
	t             *testing.T
}

func (w *fakeWriter) Write(p []byte) (int, error) {
	if !w.failNextWrite {
		n, err := w.buf.Write(p)
		if err != nil { // Just if it fails we now it was not the intended one.
			w.t.Fatal("unintentionally failed")
		}
		return n, err
	}

	bytesToWrite := p[:(len(p)/2)+1]
	n, err := w.buf.Write(bytesToWrite)
	if err != nil { // Just if it fails we now it was not the intended one.
		w.t.Fatal("unintentionally failed")
	}
	return n, fmt.Errorf("forced write failure")
}

func (w *fakeWriter) String() string {
	return w.buf.String()
}
func TestScrubber_UnsafeFlush_FlushesEverything(t *testing.T) {
	var out bytes.Buffer

	// Longer than anything written below, so a regular Flush never emits
	s, close, err := NewTseScrubber(&out, []string{"VERY-VERY-LONG-SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Write([]byte("hello!"))
	if err != nil {
		t.Fatal(err)
	}

	err = s.Flush()
	if err != nil {
		t.Fatalf("got while flushing scrubber err: %s", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected empty output, got %q", out.String())
	}

	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}
	expected := "hello!"
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}

	// Force flushing again should not duplicate anything
	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_UnsafeFlush_RedactsSecretsAlreadyInBuffer(t *testing.T) {
	var out bytes.Buffer

	s, closeS, err := NewTseScrubber(&out, []string{"ABC"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Write([]byte("ABC"))
	if err != nil {
		t.Fatal(err)
	}

	// UnsafeFlush is only unsafe against secrets split between Writes
	// like: Write("SE"), UnsafeFlush(), Write("CRET")
	// it should still work ok for everything that was already written
	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}
	expected := string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}

	err = closeS()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

func TestScrubber_UnsafeFlush_SplitSecretIsNotRedacted(t *testing.T) {
	var out bytes.Buffer

	s, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Write([]byte("SE"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}
	_, err = s.Write([]byte("CRET"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	// Accepted trade-off of UnsafeFlush: the split secret is not redacted
	if out.String() != "SECRET" {
		t.Fatalf("expected %q got %q", "SECRET", out.String())
	}
}

func TestScrubber_UnsafeFlush_KeepsScrubbingAfterwards(t *testing.T) {
	var out bytes.Buffer

	s, close, err := NewTseScrubber(&out, []string{"SECRET"})
	if err != nil {
		t.Fatal(err)
	}

	_, err = s.Write([]byte("hello "))
	if err != nil {
		t.Fatal(err)
	}
	err = s.UnsafeFlush()
	if err != nil {
		t.Fatalf("got while force flushing scrubber err: %s", err)
	}

	_, err = s.Write([]byte("token=SECRET"))
	if err != nil {
		t.Fatal(err)
	}

	err = close()
	if err != nil {
		t.Fatalf("got while closing scrubber err: %s", err)
	}

	expected := "hello token=" + string(MaskPlaceholder)
	if out.String() != expected {
		t.Fatalf("expected %q got %q", expected, out.String())
	}
}

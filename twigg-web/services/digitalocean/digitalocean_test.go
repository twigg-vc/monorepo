package digitalocean

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"testing"
)

// This test actually makes requests to digital ocean, so it requires
// proper keys. To run it, set RunDigitalOceanTest=true and populate the key
const (
	RunDigitalOceanTest = false // set to true to run the test
	Access_Key_ID       = "DO..."
	Secret_key          = "hy..."
)

func Test(t *testing.T) {
	if !RunDigitalOceanTest {
		return
	}
	c := NewSpacesClient(
		"https://twigg-debug.sfo3.digitaloceanspaces.com",
		/*folder */ "test",
		Access_Key_ID,
		Secret_key,
	)
	err := c.Put("digital-ocean-test", "key", 6, bytes.NewBufferString("012345"))
	if err != nil {
		t.Fatal(err)
	}

	r, closeR, err := c.Get("digital-ocean-test", "key", 2)
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 2)
	_, err = r.Read(b)
	defer closeR()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(b, []byte("23")) {
		t.Fatalf("got: %v", b)
	}
}

func FuzzPutGet(f *testing.F) {
	if !RunDigitalOceanTest {
		f.Skip("RunDigitalOceanTest is false")
	}
	c := NewSpacesClient(
		"https://twigg-debug.sfo3.digitaloceanspaces.com",
		/*folder */ "test",
		Access_Key_ID,
		Secret_key,
	)

	f.Add("digital-ocean-test", "key", []byte("012345"), int64(2))
	f.Add("", "", []byte{0}, int64(0))
	f.Add("prefix/with/slashes", "key with spaces & symbols?#%", []byte("payload"), int64(0))
	f.Add("unicode-préfixo", "chave-ção", bytes.Repeat([]byte{0xff, 0x00}, 1000), int64(1999))

	f.Fuzz(func(t *testing.T, keyPrefix, key string, data []byte, offset int64) {
		// Empty puts are rejected by the server (the client sends them
		if len(data) == 0 {
			t.Skip("empty payload")
		}
		// Map the fuzzed offset into [0, size) so the read is always valid.
		size := int64(len(data))
		offset = offset % size // now in (-size, size)
		if offset < 0 {
			offset += size
		}
		// hash the data and use as key so this is safe to run in parallel
		key = fmt.Sprintf("%s-%x", key, sha256.Sum256(data))
		err := c.Put(keyPrefix, key, size, bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}

		r, closeR, err := c.Get(keyPrefix, key, offset)
		if err != nil {
			t.Fatal(err)
		}
		defer closeR()
		got, err := io.ReadAll(r)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, data[offset:]) {
			t.Fatalf("prefix=%q key=%q offset=%d: got %d bytes, want %d; got=%x want=%x",
				keyPrefix, key, offset, len(got), len(data[offset:]), got, data[offset:])
		}
	})
}

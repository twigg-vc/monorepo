package xchange

import (
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"monorepo/twigg/commit"
)

func newIdWriter(w io.Writer) (cw CommitIdWriter, close func() error, err error) {
	zipW, err := flate.NewWriter(w, flateLevel)
	if err != nil {
		// Flate only returns err for invalid compression
		panic("crating flate failed: " + err.Error())
	}
	protocolVersion := CurrentProtocol
	if UseMockProtocolVersionSentByWriters {
		protocolVersion = MockProtocolVersionSentByWriters
	}
	close = zipW.Close
	cw = idWriter{
		zipW:     zipW,
		protocol: protocolVersion,
		buff:     make([]byte, 8)}
	err = writeProtocol(protocolVersion, zipW)
	return
}

type idWriter struct {
	zipW     io.Writer
	protocol uint8
	buff     []byte
}

func (iw idWriter) WriteEof() error {
	iw.buff[0] = 1
	_, err := iw.zipW.Write(iw.buff[:1])
	return err
}

func (iw idWriter) WriteErrMsg(errMsg string) error {
	// First write 0 to indicate not an EOF
	iw.buff[0] = 0
	_, err := iw.zipW.Write(iw.buff[:1])
	if err != nil {
		return err
	}
	// Now write 1 to indicate this is an err message
	iw.buff[0] = 1
	_, err = iw.zipW.Write(iw.buff[:1])
	if err != nil {
		return err
	}
	// Now write the msg len
	for len(iw.buff) < len(errMsg) {
		iw.buff = append(iw.buff, 0)
	}
	binary.BigEndian.PutUint64(iw.buff[:8], uint64(len(errMsg)))
	_, err = iw.zipW.Write(iw.buff[:8])
	if err != nil {
		return err
	}
	// And the msg itself
	_, err = io.WriteString(iw.zipW, errMsg)
	return err
}

func (iw idWriter) Write(L commit.LocalId, V uint64) error {
	// First write 0 to indicate not an EOF
	iw.buff[0] = 0
	_, err := iw.zipW.Write(iw.buff[:1])
	if err != nil {
		return err
	}
	// Then write 0 to indicate we're writing IDs (not an err msg)
	iw.buff[0] = 0
	_, err = iw.zipW.Write(iw.buff[:1])
	if err != nil {
		return err
	}
	// Then L
	binary.BigEndian.PutUint64(iw.buff[:8], L)
	_, err = iw.zipW.Write(iw.buff[:8])
	if err != nil {
		return err
	}
	// Then V
	binary.BigEndian.PutUint64(iw.buff[:8], V)
	_, err = iw.zipW.Write(iw.buff[:8])
	if err != nil {
		return err
	}
	return nil
}

func newIdReader(r io.Reader) (cr CommitIdReader, close func() error, err error) {
	zipR := flate.NewReader(r)
	close = zipR.Close
	p, err := readProtocol(zipR)
	if err != nil {
		_ = close()
		close = func() error { return nil }
		return
	}
	err = checkProtocol(p)
	if err != nil {
		_ = close()
		close = func() error { return nil }
		return
	}
	return &idReader{zipR: zipR, protocol: p, buff: make([]byte, 8)},
		close, err
}

type idReader struct {
	zipR     io.Reader
	buff     []byte
	protocol uint8
}

func (ir idReader) Read() (L commit.LocalId, V uint64, err error) {
	// First check EOF
	_, err = io.ReadFull(ir.zipR, ir.buff[:1])
	if err != nil {
		return
	}
	if ir.buff[0] == 1 {
		err = io.EOF
		return
	}
	// Now check if this is an err msg or IDs
	_, err = io.ReadFull(ir.zipR, ir.buff[:1])
	if err != nil {
		return
	}
	// 1 indicates errMsg, 0 indicates IDs
	if ir.buff[0] == 1 {
		err = ir.readErrMsg()
		return
	}
	L, V, err = ir.readIds()
	return
}

func (ir idReader) readErrMsg() (err error) {
	// Read msg size
	_, err = io.ReadFull(ir.zipR, ir.buff[:8])
	if err != nil {
		return
	}
	msgSize := binary.BigEndian.Uint64(ir.buff[:8])
	for uint64(len(ir.buff)) < msgSize {
		ir.buff = append(ir.buff, 0)
	}
	// Now the actual msg
	_, err = io.ReadFull(ir.zipR, ir.buff[:msgSize])
	if err != nil {
		return
	}
	err = errors.New(string(ir.buff[:msgSize]))
	return
}

func (ir idReader) readIds() (L commit.LocalId, V uint64, err error) {
	// Read L
	_, err = io.ReadFull(ir.zipR, ir.buff[:8])
	if err != nil {
		return
	}
	L = binary.BigEndian.Uint64(ir.buff[:8])
	// Then V
	_, err = io.ReadFull(ir.zipR, ir.buff[:8])
	if err != nil {
		return
	}
	V = binary.BigEndian.Uint64(ir.buff[:8])
	return
}

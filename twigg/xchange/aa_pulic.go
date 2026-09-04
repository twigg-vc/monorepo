// Package related to sharing commits and files
package xchange

import (
	"errors"
	"io"
	"monorepo/twigg/commit"
	"monorepo/twigg/repo"
	"net/http"
)

// Returns an interface that can be used to send a commit somewhere
// (to a server, for example).
func NewCommitWriter(w io.Writer) (cw CommitWriter, close func() error, err error) {
	return newWriter(w)
}

type CommitWriter interface {
	// Writes an error message.
	// Doesn't write EOF.
	WriteErrMsg(msg string) error
	// Write a commit. `base` must be provided as that will determine
	// which trees must be fully written: only trees in the commit that
	// changed with respect to base will be written. The target (server for
	// example) that will read what's written must know the specified
	// commit version, else it'll interpret the diff incorrectly.
	// You can always resort to using the
	// root commit, but to reduce the data you want to use the commit "closest"
	// to the one you're writing that you still know the server definitely has
	// in the same state that you do.
	// Submitted commits can always be used for these, as they are immutable.
	// This function can be called many times to write many commits, just make
	// sure to call WriteEof when done.
	Write(c commit.Commit,
		baseServerL commit.LocalId, baseServerV uint64,
		baseTreeVersion repo.TreeVersion,
		r repo.Repo, l repo.Read) error
	// Writes EOF to the writer.
	// This should be called after you're done writing all the commits you want.
	WriteEof() error
	// Writes UnexpectedEOF to the writer.
	// This should be called after you've written some commits, but not all
	// the ones you wanted to. This will indicate that the reader should read
	// again from the start.
	WriteUnexpectedEof() error
}

// Returns ErrOldProtocol if the writer uses a newer version
func NewCommitReader(r io.Reader) (cr CommitReader, close func() error, err error) {
	return newReader(r)
}

type CommitReader interface {
	// Read what's been written by the Write function.
	// If WriteEofTo/WriteUnexpectedEofTo was called (which it should be),
	// will return io.EOF or io.UnexpectedEOF.
	Read() (c commit.Commit, baseServerL commit.LocalId, baseServerV uint64,
		it repo.DeltaIter, err error)
}

// Writes the ids of commits and error messages.
// Make sure to call Eof when done writing.
type CommitIdWriter interface {
	// Write IDs
	Write(L commit.LocalId, V uint64) error
	// Write an error message. This does not write EOF.
	WriteErrMsg(err string) error
	// Write EOF after done writing everything.
	WriteEof() error
}

func NewCommitIdWriter(w io.Writer) (cr CommitIdWriter, close func() error, err error) {
	return newIdWriter(w)
}

// Reads the ids/versions of commits.
type CommitIdReader interface {
	// Returns io.EOF when done reading (L/V should not be used then).
	// Returns the error message if WriteErrMsg was used.
	Read() (L commit.LocalId, V uint64, err error)
}

// Returns ErrOldProtocol if the writer uses a newer version
func NewCommitIdReader(r io.Reader) (cr CommitIdReader, close func() error, err error) {
	return newIdReader(r)
}

// Sets the api key header
func SetApiKeyHeader(apiKey string, r *http.Request) {
	r.Header.Set(apiKeyHeader, apiKey)
}

// Read the api key header
func GetApiKeyHeader(r *http.Request) string {
	return r.Header.Get(apiKeyHeader)
}

// Returns true if the response might be from from a twigg server.
// Notice "Might": this is easily spoofable (it's just a header), but all
// Twigg requests will use this header so that we can easily check if a client
// is not trying to talk with a random http server
func MightBeTwiggResponse(r *http.Response) bool {
	return r.Header.Get(twiggHeader) != ""
}

// See `MightBeTwiggResponse`
func MightBeTwiggRequest(r *http.Request) bool {
	return r.Header.Get(twiggHeader) != ""
}

// Sets a header that identifies this request as being from a twigg server
func SetTwiggHeaderInRequest(r *http.Request) {
	r.Header.Set(twiggHeader, "1")
}

// Sets a header that identifies this response as being from a twigg server
func SetTwiggHeaderInResponse(w http.ResponseWriter) {
	w.Header().Set(twiggHeader, "1")
}

// Header used to identify requests made by a twigg client/server
const twiggHeader = "X-Twigg-Hello"

// Header used to carry a twigg api key
const apiKeyHeader = "X-Twigg-Api-Key"

// Msg used to signal bad/missing api keys
const BadApiKeyErrMsg = "bad/missing API Key"

// Msg used to signal bad/missing twigg token
const BadTwTokenErrMsg = "bad/missing Twigg Token"

// Variables used to mock (for testing) which version of the protocol the reader
// will receive
var (
	UseMockProtocolVersionSentByWriters = false
	MockProtocolVersionSentByWriters    = uint8(0)
)

const CurrentProtocol = uint8(1)

var (
	ErrOldProtocol = errors.New("old protocol version - update is required")
)

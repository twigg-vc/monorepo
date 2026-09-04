package client

import (
	"errors"
	"fmt"
	"io"
	"monorepo/base/stack"
	"monorepo/twigg/commit"
	"monorepo/twigg/xchange"
	"net/http"
)

func (a tw) Push(input *commit.Commit, url string, apiKey string, l Write) (
	isObsParentErr bool, isBadApiKeyErr bool, err error) {
	if input.Status == commit.StatusObsolete {
		panic("tried to push obsolete")
	}
	if input.IsOnServer() {
		panic("commit was already pushed")
	}

	// The commits are pushed in a stack order
	pushStack := stack.New[*commit.Commit]()
	// We first add the input commit. This way, it'll be pushed last.
	pushStack.Push(input)
	// Keep adding the the parents on top
	for !(*pushStack.Peek()).IsOnServer() {
		var parent commit.Commit
		parent, err = a.GetVersion(
			(*pushStack.Peek()).ParentL,
			(*pushStack.Peek()).ParentV, l)
		if err != nil {
			return
		}
		if parent.Status == commit.StatusObsolete && !parent.IsOnServer() {
			err = fmt.Errorf(
				"#%d-v%d has the parent #%d-v%d, which is obsolete and has never been pushed",
				input.L, input.Version, parent.L, parent.Version)
			isObsParentErr = true
			return
		}
		pushStack.Push(&parent)
	}
	httpClient := &http.Client{}
	parent := pushStack.Pop()
	for !pushStack.IsEmpty() {
		pushed := pushStack.Pop()
		if pushed.ParentL != parent.L {
			panic("expected to use parent as pushBase")
		}
		if !parent.IsOnServer() {
			panic("expected to only see parents pushed before")
		}
		pushed.SetParentServerData(*parent)
		err = a.writeOneCommit(httpClient, url, apiKey, pushed, *parent, l)
		if err != nil {
			isBadApiKeyErr = err.Error() == xchange.BadApiKeyErrMsg
			return
		}
		if pushStack.IsEmpty() {
			break
		}
		parent = pushed
	}

	// Set the parent server IDs on all children of this commit
	for i, childId := range input.Children {
		var child commit.Commit
		child, err = a.GetVersion(childId, input.ChildrenVersions[i], l)
		if err != nil {
			return
		}
		if child.SetParentServerData(*input) {
			err = l.SetCommit(a.QuotaOwner, a.RepoId, child)
			if err != nil {
				return
			}
		}
	}

	return
}

func (a tw) writeOneCommit(httpClient *http.Client, url string, apiKey string,
	c *commit.Commit, base commit.Commit, l Write) (err error) {

	r, w := io.Pipe()
	req, err := http.NewRequest(PushMethod, url+PushEndpoint, r)
	if err != nil {
		w.Close()
		return
	}
	xchange.SetTwiggHeaderInRequest(req)
	xchange.SetApiKeyHeader(apiKey, req)
	req.Header.Set("Content-Type", "application/octet-stream")

	errCh := make(chan error, 1)
	go func() {
		resp, err_ := httpClient.Do(req)
		if err_ != nil {
			errCh <- err_
			return
		}
		defer resp.Body.Close()
		if !xchange.MightBeTwiggResponse(resp) {
			errCh <- ErrNotTwiggServer
			return
		}
		idReader, closeIdReader, err_ := xchange.NewCommitIdReader(resp.Body)
		defer closeIdReader()
		if err_ != nil {
			errCh <- err_
			return
		}
		// We expect two chunks of data: the first is the commit ids,
		// than an EOF indicator.
		// First read the IDs
		var ServerL commit.LocalId
		var ServerV uint64
		ServerL, ServerV, err_ = idReader.Read()
		if err_ != nil {
			errCh <- err_
			return
		}
		if c.HasServerL {
			if c.ServerL != ServerL {
				panic("commit changed L when pushed")
			}
		}

		c.HasServerL = true
		c.ServerL = ServerL
		c.HasServerV = true
		c.ServerV = ServerV
		err_ = l.SetCommit(a.QuotaOwner, a.RepoId, *c)
		if err_ != nil {
			errCh <- err_
			return
		}
		// Read again to get an EOF
		_, _, err_ = idReader.Read()
		if !errors.Is(err_, io.EOF) {
			errCh <- fmt.Errorf("expected EOF, got %s", err_)
			return
		}
		errCh <- nil
	}()
	defer func() {
		w.Close()
		chErr := <-errCh
		err = errors.Join(err, chErr)
	}()

	cw, cl, err := xchange.NewCommitWriter(w)
	defer cl()
	if err != nil {
		return
	}
	err = cw.Write(*c, base.ServerL, base.ServerV, base.TreeVersion, a.repo, l)
	if err != nil {
		return
	}
	err = cw.WriteEof()
	return
}
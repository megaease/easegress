/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package proxy

import (
	"bytes"
	"io"

	"github.com/megaease/easegress/pkg/util/callbackreader"
)

type (
	// primaryReader reads bytes from reader and synchronize them to secondary reader
	primaryReader struct {
		r        io.Reader
		buffChan chan []byte
		sawEOF   bool
	}

	// secondaryReader receives the bytes from primary
	secondaryReader struct {
		unreadBuff *bytes.Buffer
		buffChan   chan []byte
	}
)

func newPrimarySecondaryReader(r io.Reader) (io.ReadCloser, io.Reader) {
	var realReader io.Reader
	if cbReader, ok := r.(*callbackreader.CallbackReader); ok {
		realReader = cbReader.GetReader()
	} else {
		realReader = r
	}
	buffChan := make(chan []byte, 10)
	mr := &primaryReader{
		r:        realReader,
		buffChan: buffChan,
		sawEOF:   false,
	}
	sr := &secondaryReader{
		unreadBuff: bytes.NewBuffer(nil),
		buffChan:   buffChan,
	}

	return mr, sr
}

func (mr *primaryReader) Read(p []byte) (n int, err error) {
	if mr.sawEOF {
		return 0, io.EOF
	}
	buff := bytes.NewBuffer(nil)
	tee := io.TeeReader(mr.r, buff)
	n, err = tee.Read(p)

	if n != 0 {
		mr.buffChan <- buff.Bytes()
	}

	if err == io.EOF {
		close(mr.buffChan)
		mr.sawEOF = true
	}

	return n, err
}

func (mr *primaryReader) Close() error {
	if closer, ok := mr.r.(io.ReadCloser); ok {
		return closer.Close()
	}

	return nil
}

func (sr *secondaryReader) Read(p []byte) (int, error) {
	buff, ok := <-sr.buffChan

	if !ok {
		return 0, io.EOF
	}

	var n int
	// NOTE: This if-branch is defensive programming,
	// Because the callers of Read of both master and slave
	// are the same, so it never happens that len(p) < len(buff).
	// else-branch is faster because it is one less copy operation than if-branch.
	if sr.unreadBuff.Len() > 0 || len(p) < len(buff) {
		sr.unreadBuff.Write(buff)
		n, _ = sr.unreadBuff.Read(p)
	} else {
		n = copy(p, buff)
	}

	return n, nil
}

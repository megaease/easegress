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

package callbackreader

import (
	"io"
)

type (
	// CallbackReader is counter for io.Reader
	CallbackReader struct {
		beforeFuncs []BeforeFunc
		afterFuncs  []AfterFunc

		num    int
		reader io.Reader
	}

	// BeforeFunc runs before each Read.
	// num means the number of calling Read, starts from 1.
	BeforeFunc func(num int, p []byte) []byte

	// AfterFunc runs after each read.
	// num means the number of calling Read, starts from 1.
	AfterFunc func(num int, p []byte, n int, err error) ([]byte, int, error)
)

// New creates CallbackReader.
func New(r io.Reader) *CallbackReader {
	return &CallbackReader{
		// pre-allocate slices for performance
		beforeFuncs: make([]BeforeFunc, 0, 1),
		afterFuncs:  make([]AfterFunc, 0, 1),
		reader:      r,
	}
}

func (cr *CallbackReader) Read(p []byte) (int, error) {
	cr.num++

	for _, fn := range cr.beforeFuncs {
		p = fn(cr.num, p)
	}

	n, err := 0, io.EOF
	if cr.reader != nil {
		n, err = cr.reader.Read(p)
	}

	for _, fn := range cr.afterFuncs {
		p, n, err = fn(cr.num, p, n, err)
	}

	return n, err
}

// OnBefore registers callback function running before the first read.
func (cr *CallbackReader) OnBefore(fn BeforeFunc) {
	cr.beforeFuncs = append(cr.beforeFuncs, fn)
}

// OnAfter registers callback function running after the last read.
func (cr *CallbackReader) OnAfter(fn AfterFunc) {
	cr.afterFuncs = append(cr.afterFuncs, fn)
}

// Close wraps Close if existed
func (cr *CallbackReader) Close() error {
	closer, ok := cr.reader.(io.Closer)
	if ok {
		return closer.Close()
	}

	return nil
}

func (cr *CallbackReader) GetReader() io.Reader {
	return cr.reader
}

// SetReader replace previous reader with new reader. If closePreviousReader set to true, it will close
// previous reader.
func (cr *CallbackReader) SetReader(reader io.Reader, closePreviousReader bool) {
	if closePreviousReader {
		closer, ok := cr.reader.(io.Closer)
		if ok {
			closer.Close()
		}
	}
	cr.reader = reader
}

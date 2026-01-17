// Copyright 2022 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package buffer provides a pool-allocated byte buffer.
package buffer

import "sync"

// Buffer is a byte buffer.
//
// This implementation is adapted from the Go standard library
// in go/src/log/slog/internal/buffer/buffer.go.
type Buffer []byte

// Having an initial size gives a dramatic speedup.
var bufPool = sync.Pool{
	New: func() any {
		b := make([]byte, 0, 1024)
		return (*Buffer)(&b)
	},
}

// New returns a buffer from the pool.
func New() *Buffer {
	return bufPool.Get().(*Buffer)
}

// Free returns the buffer to the pool.
// To reduce peak allocation, return only smaller buffers to the pool.
func (b *Buffer) Free() {
	const maxBufferSize = 16 << 10 // 16KB
	if cap(*b) <= maxBufferSize {
		*b = (*b)[:0]
		bufPool.Put(b)
	}
}

// Reset resets the buffer to be empty.
func (b *Buffer) Reset() {
	b.SetLen(0)
}

// Write appends the contents of p to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	*b = append(*b, p...)
	return len(p), nil
}

// WriteString appends the contents of s to the buffer.
func (b *Buffer) WriteString(s string) (int, error) {
	*b = append(*b, s...)
	return len(s), nil
}

// WriteByte appends the byte c to the buffer.
func (b *Buffer) WriteByte(c byte) error {
	*b = append(*b, c)
	return nil
}

// String returns the contents of the buffer as a string.
func (b *Buffer) String() string {
	return string(*b)
}

// Len returns the number of bytes in the buffer.
func (b *Buffer) Len() int {
	return len(*b)
}

// SetLen sets the length of the buffer.
func (b *Buffer) SetLen(n int) {
	*b = (*b)[:n]
}

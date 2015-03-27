package smtp

import (
	"bufio"
	"bytes"
	"errors"
	"io"
)

type bufferedReader struct {
	reader io.Reader
	buffer []byte
	r, w   int
}

func (b *bufferedReader) Fill() error {
	if b.r != 0 {
		copy(b.buffer, b.buffer[b.r:b.w])
		b.r, b.w = 0, b.w-b.r
	}

	if b.w == len(b.buffer) {
		return bufio.ErrBufferFull
	}

	n, err := b.reader.Read(b.buffer[b.w:])
	b.w += n
	return err
}

func (b *bufferedReader) Read(data []byte) (n int, err error) {
	for b.w-b.r == 0 {
		if err := b.Fill(); err != nil {
			return 0, err
		}
	}
	n = len(data)
	if b.w-b.r < n {
		n = b.w - b.r
	}
	copy(data, b.buffer[b.r:b.r+n])
	b.r += n
	return n, nil
}

func (b *bufferedReader) Buffered() []byte {
	return b.buffer[b.r:b.w]
}

func (b *bufferedReader) ReadLine() (string, error) {
	idx := 0

	for {
		line := b.Buffered()

		foundAt := bytes.Index(line[idx:], []byte("\r\n"))
		if foundAt != -1 {
			foundAt += idx
			buf := make([]byte, foundAt+2)
			b.Read(buf)
			return string(buf[:foundAt]), nil
		} else if len(line) >= 2 {
			idx = len(line) - 2
		}

		if err := b.Fill(); err == io.EOF || err == bufio.ErrBufferFull {
			return "", errors.New("line too long")
		}
	}
}

func newBufferedReader(reader io.Reader, n int) *bufferedReader {
	return &bufferedReader{
		reader: reader,
		buffer: make([]byte, n),
	}
}

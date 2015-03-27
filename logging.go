package smtp

import (
	"encoding/json"
	"fmt"
	"io"
)

type loggingReadWriteCloser struct {
	conn io.ReadWriteCloser
}

func (l *loggingReadWriteCloser) Close() error {
	fmt.Printf("Close: ")
	err := l.conn.Close()
	fmt.Printf("%v\n", err)
	return err
}

func (l *loggingReadWriteCloser) Write(b []byte) (n int, err error) {
	s, _ := json.Marshal(string(b))
	fmt.Printf("Write(%v): ", string(s))
	n, err = l.conn.Write(b)
	fmt.Printf("(%v, %v)\n", n, err)
	return n, err
}

func (l *loggingReadWriteCloser) Read(b []byte) (n int, err error) {
	fmt.Printf("Read(length=%v): ", len(b))
	n, err = l.conn.Read(b)
	s, _ := json.Marshal(string(b[:n]))
	fmt.Printf("(%v, %v) data=%v\n", n, err, string(s))
	return n, err
}

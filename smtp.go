// Package smtp is a barebones, pure Go SMTP server.
//
// The server supports UTF8 and chunked e-mails.
package smtp

import (
	"bytes"
	"io"
	"net"
	"strconv"
	"strings"
)

// A Mail holds a received e-mail. From and To are SMTP protocol-level fields
// (and not parsed from the e-mail headers).
type Mail struct {
	From, To string
	Mail     string
}

// A Handler processes received e-mails. Should be thread-safe.
type Handler func(*Mail)

// SizeLimit is the maximum e-mail in bytes. Currently, package smtp does not support large e-mails.
const SizeLimit = 32 * 1024

// MaxLineLength is the maximum length of a SMTP protocol line. Currently,
// package smtp does not support long lines.
const MaxLineLength = SizeLimit

type conn struct {
	domain string

	conn   io.ReadWriteCloser
	reader *bufferedReader

	handler Handler

	state    state
	from, to string
}

func (c *conn) greeting() {
	c.conn.Write([]byte("220 " + c.domain + " jellevandenhooff/smtp ready!\r\n"))
}

func (c *conn) ehlo() {
	c.conn.Write([]byte("250-" + c.domain + "\r\n250-PIPELINING\r\n250-8BITMIME\r\n250-SMTPUTF8\r\n250-CHUNKING\r\n250 SIZE " + strconv.Itoa(sizeLimit) + "\r\n"))
}

func (c *conn) helo() {
	c.conn.Write([]byte("250 " + c.domain + "\r\n"))
}

func (c *conn) syntaxError(message string) {
	c.conn.Write([]byte("500 " + message + "\r\n"))
}

func (c *conn) tooManyRecipients() {
	c.conn.Write([]byte("451 only one recipient per mail, please\r\n"))
}

func (c *conn) tooMuchMail() {
	c.conn.Write([]byte("552 too much data\r\n"))
}

func (c *conn) unexpectedCommand() {
	c.conn.Write([]byte("503 did not expect that command\r\n"))
}

func (c *conn) ok() {
	c.conn.Write([]byte("250 ok\r\n"))
}

func (c *conn) quitOk() {
	c.conn.Write([]byte("221 ok\r\n"))
}

func (c *conn) weDontVerify() {
	c.conn.Write([]byte("252 vrfy is so 90s\r\n"))
}

func (c *conn) startMail() {
	c.conn.Write([]byte("354 here we go\r\n"))
}

func (c *conn) readData() (string, bool) {
	c.startMail()

	var lines []string
	length := 0

	for {
		line, err := c.reader.ReadLine()
		if err != nil {
			return "", false
		}
		if line == "." {
			break
		}
		line = strings.TrimPrefix(line, ".")

		length += len(line) + 2
		if length > sizeLimit {
			c.tooMuchMail()
			return "", false
		}
		lines = append(lines, line)
	}

	lines = append(lines, "") // include final CRLF
	email := strings.Join(lines, "\r\n")
	c.ok()

	return email, true
}

func (c *conn) readNextBdat() (*bdatCmd, bool) {
	for {
		line, err := c.reader.ReadLine()
		if err != nil {
			return nil, false
		}
		cmd, err := parseCommand(line)
		if err != nil {
			c.syntaxError(err.Error())
			continue
		}
		switch cmd := cmd.(type) {
		case *bdatCmd:
			return cmd, true
		default:
			c.unexpectedCommand()
		}
	}
}

func (c *conn) readBdat(cmd *bdatCmd) (string, bool) {
	var data [][]byte
	length := 0

	for {
		length += cmd.length
		if length > sizeLimit {
			c.tooMuchMail()
			return "", false
		}

		slice := make([]byte, cmd.length)
		if _, err := io.ReadFull(c.reader, slice); err != nil {
			return "", false
		}
		data = append(data, slice)

		if cmd.last {
			break
		}

		var ok bool
		cmd, ok = c.readNextBdat()
		if !ok {
			return "", false
		}
	}

	c.ok()
	return string(bytes.Join(data, nil)), true
}

type state int

const (
	initial state = iota
	gotFrom
	gotTo
	gotData
)

func (c *conn) processCommand(cmd interface{}) bool {
	switch cmd := cmd.(type) {
	case *heloCmd:
		if c.state != initial {
			c.unexpectedCommand()
			return true
		}
		if cmd.isEhlo {
			c.ehlo()
		} else {
			c.helo()
		}
		return true

	case *mailFromCmd:
		if c.state != initial {
			c.unexpectedCommand()
			return true
		}
		c.state, c.from = gotFrom, cmd.from
		c.ok()
		return true

	case *rcptToCmd:
		if c.state != gotFrom {
			c.unexpectedCommand()
			return true
		}
		c.state, c.to = gotTo, cmd.to
		c.ok()
		return true

	case *bdatCmd:
		if c.state != gotTo {
			c.unexpectedCommand()
			return true
		}
		mail, ok := c.readBdat(cmd)
		if !ok {
			return false
		}
		c.handler(&Mail{From: c.from, To: c.to, Mail: mail})
		c.state, c.from, c.to = initial, "", ""
		return true

	case *dataCmd:
		if c.state != gotTo {
			c.unexpectedCommand()
			return true
		}
		mail, ok := c.readData()
		if !ok {
			return false
		}
		c.handler(&Mail{From: c.from, To: c.to, Mail: mail})
		c.state, c.from, c.to = initial, "", ""
		return true

	case *rsetCmd:
		c.state, c.from, c.to = initial, "", ""
		c.ok()
		return true

	case *noopCmd:
		c.ok()
		return true

	case *quitCmd:
		c.quitOk()
		return false

	case *vrfyCmd:
		c.weDontVerify()
		return true

	default:
		c.unexpectedCommand()
		return true
	}
}

func (c *conn) handle() {
	c.greeting()
	defer c.conn.Close()

	c.state = initial

	for {
		line, err := c.reader.ReadLine()
		if err != nil {
			break
		}
		cmd, err := parseCommand(line)
		if err != nil {
			c.syntaxError(err.Error())
			continue
		}

		if ok := c.processCommand(cmd); !ok {
			break
		}
	}
}

// Serve runs an SMTP server. Prints domain on connection. Returns an error if
// the listener fails.
func Serve(domain string, listener net.Listener, handler Handler) error {
	for {
		var c io.ReadWriteCloser
		c, err := listener.Accept()
		if err != nil {
			return err
		}

		conn := &conn{
			domain:  domain,
			conn:    c,
			reader:  newBufferedReader(c, maxLineLength),
			handler: handler,
		}
		go conn.handle()
	}
}

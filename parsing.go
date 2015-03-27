package smtp

import (
	"errors"
	"strconv"
	"strings"
)

func parseDomain(line string) (string, error) {
	if len(line) < 1 {
		return "", errors.New("missing domain")
	}
	return line, nil
}

func parseEmail(line string) (string, error) {
	if len(line) < 2 || line[0] != '<' || line[len(line)-1] != '>' {
		return "", errors.New("missing outer <> around email")
	}

	line = line[1 : len(line)-1]
	return line, nil
}

func extractWord(in string) (string, string) {
	idx := strings.Index(in, " ")
	if idx == -1 {
		return in, ""
	}
	return in[:idx], in[idx+1:]
}

func parseCommand(line string) (interface{}, error) {
	command, args := extractWord(line)

	switch strings.ToLower(command) {
	case "helo":
		domain, err := parseDomain(args)
		if err != nil {
			return nil, err
		}
		return &heloCmd{
			domain: domain,
			isEhlo: false,
		}, nil
	case "ehlo":
		domain, err := parseDomain(args)
		if err != nil {
			return nil, err
		}
		return &heloCmd{
			domain: domain,
			isEhlo: true,
		}, nil
	case "mail":
		// eat all args to handle extensions
		from, _ := extractWord(args)

		if !strings.HasPrefix(strings.ToLower(from), "from:") {
			return nil, errors.New("expected from: after mail")
		}
		from, err := parseEmail(from[5:])
		if err != nil {
			return nil, err
		}
		return &mailFromCmd{
			from: from,
		}, nil
	case "rcpt":
		if !strings.HasPrefix(strings.ToLower(args), "to:") {
			return nil, errors.New("expected to: after rcpt")
		}
		to, err := parseEmail(args[3:])
		if err != nil {
			return nil, err
		}
		return &rcptToCmd{
			to: to,
		}, nil
	case "bdat":
		length, args := extractWord(args)
		n, err := strconv.Atoi(length)
		if err != nil || n < 0 {
			return nil, errors.New("bad length")
		}
		var last bool
		if args == "" {
			last = false
		} else if strings.ToLower(args) == "last" {
			last = true
		} else {
			return nil, errors.New("unexpected bdat args")
		}
		return &bdatCmd{
			length: n,
			last:   last,
		}, nil
	case "data":
		if args != "" {
			return nil, errors.New("unexpected data args")
		}
		return &dataCmd{}, nil
	case "rset":
		if args != "" {
			return nil, errors.New("unexpected rset args")
		}
		return &rsetCmd{}, nil
	case "noop":
		return &noopCmd{}, nil
	case "quit":
		if args != "" {
			return nil, errors.New("unexpected quit args")
		}
		return &quitCmd{}, nil
	case "vrfy":
		return &vrfyCmd{}, nil
	default:
		return nil, errors.New("unknown command")
	}
}

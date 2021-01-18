package parser

import (
	"net"
)

// parse IP address, but don't trigger error on special value "unknown"
func parseIP(b []byte) (net.IP, error) {
	if len(b) == 0 {
		return nil, nil
	}

	if string(b) == "unknown" {
		return nil, nil
	}

	ip := net.ParseIP(string(b))

	if ip == nil {
		return nil, &net.ParseError{Type: "IP Address", Text: "Invalid IP"}
	}

	return ip, nil
}

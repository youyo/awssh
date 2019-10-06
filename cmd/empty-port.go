package cmd

import "net"

func fetchEmptyPort() (port string, err error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err == nil {
		defer l.Close()

		addr := l.Addr().String()
		_, port, err := net.SplitHostPort(addr)

		if err == nil {
			return port, nil
		} else {
			return "", err
		}
	} else {
		return "", err
	}
}

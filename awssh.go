package awssh

import (
	"io/ioutil"
	"net"
	"time"

	homedir "github.com/mitchellh/go-homedir"
)

const (
	ConnectHost string = "127.0.0.1"
)

func fetchEmptyPort(host string) (port string, err error) {
	l, err := net.Listen("tcp", host+":0")
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

func waitOpenPort(host, localPort string) {
	for {
		conn, _ := net.DialTimeout("tcp", net.JoinHostPort(host, localPort), 200000*time.Nanosecond)
		if conn != nil {
			conn.Close()
			break
		}
	}
}

func guessPublickey(identityFile, publickey string) (guessedPublickey string) {
	guessedPublickey = publickey
	if publickey == "identity-file+'.pub'" {
		guessedPublickey = identityFile + ".pub"
	}
	return guessedPublickey
}

func readPublicKey(filePath string) (publicKey string, err error) {
	fullPath, err := homedir.Expand(filePath)
	if err != nil {
		return "", err
	}

	publicKeyBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	publicKey = string(publicKeyBytes)
	return publicKey, nil
}

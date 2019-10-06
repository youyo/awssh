package cmd

import (
	"io/ioutil"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

const (
	ConnectHost string = "127.0.0.1"
)

func ReadIdentityFile(filePath string) (sshSigner ssh.Signer, err error) {
	fullPath, err := homedir.Expand(filePath)
	if err != nil {
		return nil, err
	}

	privateKeyBytes, err := ioutil.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}
	sshSigner, err = ssh.ParsePrivateKey(privateKeyBytes)
	return sshSigner, err
}

func BuildSshClientConfig(username string, sshSigner ssh.Signer) (sshConfig *ssh.ClientConfig, err error) {
	sshConfig = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshSigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return sshConfig, nil
}

func NewSshClient(port string, sshConfig *ssh.ClientConfig) (sshClient *ssh.Client, err error) {
	sshClient, err = ssh.Dial("tcp", ConnectHost+":"+port, sshConfig)
	return sshClient, err
}

func NewSshSession(sshClient *ssh.Client) (sshSession *ssh.Session, err error) {
	sshSession, err = sshClient.NewSession()
	return sshSession, err
}

func GetFileDescriptor() (fd int) {
	fd = int(os.Stdin.Fd())
	return fd
}

// Put the terminal connected to the given file descriptor into raw mode.
func MakeFdIntoRawMode(fd int) (state *terminal.State, err error) {
	state, err = terminal.MakeRaw(fd)
	return state, err
}

// Get dimensions of the given terminal
func GetTerminalSize(fd int) (width, height int, err error) {
	width, height, err = terminal.GetSize(fd)
	return width, height, err
}

func SetPty(sshSession *ssh.Session, term string, width, height int) (err error) {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = sshSession.RequestPty(term, height, width, modes)
	return err
}

func SetInputOutput(sshSession *ssh.Session, stdout *os.File, stdin *os.File, stderr *os.File) {
	sshSession.Stdout = stdout
	sshSession.Stdin = stdin
	sshSession.Stderr = stderr
}

func GetTerm() (term string) {
	//term = os.Getenv("TERM")
	term = "xterm"
	return term
}

func ExecShell(sshSession *ssh.Session) (err error) {
	if err = sshSession.Shell(); err != nil {
		return err
	}
	err = sshSession.Wait()
	return err
}

func ExecSshLogin(username, port, identityFilePath string) (err error) {
	sshSigner, err := ReadIdentityFile(identityFilePath)
	if err != nil {
		return err
	}

	sshConfig, err := BuildSshClientConfig(username, sshSigner)
	if err != nil {
		return err
	}

	sshClient, err := NewSshClient(port, sshConfig)
	if err != nil {
		return err
	}

	sshSession, err := NewSshSession(sshClient)
	if err != nil {
		return err
	}
	defer sshSession.Close()

	fd := GetFileDescriptor()
	state, err := MakeFdIntoRawMode(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, state)

	width, height, err := GetTerminalSize(fd)
	if err != nil {
		return err
	}

	term := GetTerm()
	if err = SetPty(sshSession, term, width, height); err != nil {
		return err
	}

	SetInputOutput(sshSession, os.Stdout, os.Stdin, os.Stderr)
	err = ExecShell(sshSession)
	return err
}

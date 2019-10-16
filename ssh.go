package awssh

import (
	"io/ioutil"
	"os"

	homedir "github.com/mitchellh/go-homedir"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"
)

func readIdentityFile(filePath string) (sshSigner ssh.Signer, err error) {
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

func buildSshClientConfig(username string, sshSigner ssh.Signer) (sshConfig *ssh.ClientConfig, err error) {
	sshConfig = &ssh.ClientConfig{
		User: username,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(sshSigner),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
	return sshConfig, nil
}

func newSshClient(host, port string, sshConfig *ssh.ClientConfig) (sshClient *ssh.Client, err error) {
	sshClient, err = ssh.Dial("tcp", host+":"+port, sshConfig)
	return sshClient, err
}

func newSshSession(sshClient *ssh.Client) (sshSession *ssh.Session, err error) {
	sshSession, err = sshClient.NewSession()
	return sshSession, err
}

func getFileDescriptor() (fd int) {
	fd = int(os.Stdin.Fd())
	return fd
}

// Put the terminal connected to the given file descriptor into raw mode.
func makeFdIntoRawMode(fd int) (state *terminal.State, err error) {
	state, err = terminal.MakeRaw(fd)
	return state, err
}

// Get dimensions of the given terminal
func getTerminalSize(fd int) (width, height int, err error) {
	width, height, err = terminal.GetSize(fd)
	return width, height, err
}

func setPty(sshSession *ssh.Session, term string, width, height int) (err error) {
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	err = sshSession.RequestPty(term, height, width, modes)
	return err
}

func setInputOutput(sshSession *ssh.Session, stdout *os.File, stdin *os.File, stderr *os.File) {
	sshSession.Stdout = stdout
	sshSession.Stdin = stdin
	sshSession.Stderr = stderr
}

func getTerm() (term string) {
	term = os.Getenv("TERM")
	return term
}

func execShell(sshSession *ssh.Session) (err error) {
	if err = sshSession.Shell(); err != nil {
		return err
	}
	err = sshSession.Wait()
	return err
}

func ExecSshLogin(username, host, port, identityFilePath string) (err error) {
	sshSigner, err := readIdentityFile(identityFilePath)
	if err != nil {
		return err
	}

	sshConfig, err := buildSshClientConfig(username, sshSigner)
	if err != nil {
		return err
	}

	sshClient, err := newSshClient(host, port, sshConfig)
	if err != nil {
		return err
	}

	sshSession, err := newSshSession(sshClient)
	if err != nil {
		return err
	}
	defer sshSession.Close()

	fd := getFileDescriptor()
	state, err := makeFdIntoRawMode(fd)
	if err != nil {
		return err
	}
	defer terminal.Restore(fd, state)

	width, height, err := getTerminalSize(fd)
	if err != nil {
		return err
	}

	term := getTerm()
	if err = setPty(sshSession, term, width, height); err != nil {
		return err
	}

	setInputOutput(sshSession, os.Stdout, os.Stdin, os.Stderr)
	err = execShell(sshSession)
	return err
}

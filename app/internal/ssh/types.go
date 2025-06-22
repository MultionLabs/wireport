package ssh

// Credentials represents different types of SSH authentication
type Credentials struct {
	Host     string
	Port     uint
	Username string
	// Password authentication
	Password string
	// Key-based authentication
	PrivateKeyPath string
	PrivateKeyData []byte
	// Passphrase for private key (if encrypted)
	Passphrase string
}

type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

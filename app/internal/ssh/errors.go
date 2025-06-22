package ssh

import "errors"

// SSH connection errors
var (
	ErrNoAuthMethodProvided        = errors.New("no valid authentication method provided")
	ErrPrivateKeyDataNotSupported  = errors.New("private key data not supported, please use PrivateKeyPath instead")
	ErrSSHConnectionNotEstablished = errors.New("SSH connection not established")
	ErrFailedToCreateAuth          = errors.New("failed to create auth")
	ErrFailedToCreateSSHClient     = errors.New("failed to create SSH client")
	ErrFailedToTestSSHConnection   = errors.New("failed to test SSH connection")
)

// Command execution errors
var (
	ErrFailedToCheckWireportInstallation      = errors.New("failed to check wireport installation")
	ErrFailedToCheckInstallationPermissions   = errors.New("failed to check installation permissions")
	ErrInsufficientPermissionsForInstallation = errors.New("insufficient permissions for installation")
	ErrUnsupportedInstallationMethod          = errors.New("unsupported installation method")
	ErrFailedToGetDockerVersion               = errors.New("failed to get Docker version")
	ErrFailedToGetContainerStatus             = errors.New("failed to get container status")
	ErrFailedToGetNetworkStatus               = errors.New("failed to get network status")
)

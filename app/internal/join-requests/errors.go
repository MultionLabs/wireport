package join_requests

import "errors"

var (
	ErrFailedToParseJoinToken    = errors.New("failed to parse join token")
	ErrFailedToSendJoinRequest   = errors.New("failed to send join request")
	ErrFailedToGetPublicIP       = errors.New("failed to get public IP")
	ErrFailedToReadPublicIP      = errors.New("failed to read public IP response")
	ErrInvalidJoinRequest        = errors.New("join request is invalid")
	ErrInvalidJoinRequestRole    = errors.New("invalid join request role")
	ErrFailedToCreateServerNode  = errors.New("failed to create server node")
	ErrFailedToGetHostNode       = errors.New("failed to get host node")
	ErrFailedToSaveHostConfigs   = errors.New("failed to save host configs")
	ErrFailedToEncryptResponse   = errors.New("failed to encrypt response")
	ErrFailedToDeleteJoinRequest = errors.New("failed to delete join request")
)

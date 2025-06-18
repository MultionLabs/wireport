package nodes

import "errors"

var (
	ErrHostNodeNotFound             = errors.New("host node not found")
	ErrHostNodePublicIPPortNotFound = errors.New("host node public ip or port not found")
	ErrHostNodeAlreadyExists        = errors.New("host node already exists")
	ErrFailedToParseIP              = errors.New("failed to parse ip")
	ErrNoAvailableDockerSubnets     = errors.New("no available docker subnets found in 172.16.0.0/12 range")
	ErrNoAvailableWGPrivateIPs      = errors.New("no available wg public ips found in 10.0.0.0/24 range")
)

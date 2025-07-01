package nodes

import "errors"

var (
	ErrGatewayNodeNotFound             = errors.New("gateway node not found")
	ErrGatewayNodePublicIPPortNotFound = errors.New("gateway node public ip or port not found")
	ErrGatewayNodeAlreadyExists        = errors.New("gateway node already exists")
	ErrFailedToParseIP                 = errors.New("failed to parse ip")
	ErrNoAvailableDockerSubnets        = errors.New("no available docker subnets found in 172.16.0.0/12 range")
	ErrNoAvailableWGPrivateIPs         = errors.New("no available wg public ips found in 10.0.0.0/24 range")
)

package types

import (
	"net"
)

type IPMarshable struct {
	net.IP
}

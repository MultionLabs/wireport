package types

import (
	"fmt"
	"net"
)

func IPToString(ip net.IP) string {
	if len(ip) < 16 {
		return ""
	}

	return fmt.Sprintf("%d.%d.%d.%d", ip[12], ip[13], ip[14], ip[15])
}

type UDPAddrMarshable struct {
	net.UDPAddr
}

func (udpaddr UDPAddrMarshable) String() string {
	if udpaddr.IP == nil {
		return ""
	}

	if udpaddr.Port == 0 {
		return IPToString(udpaddr.IP)
	}

	return fmt.Sprintf("%s:%d", IPToString(udpaddr.IP), udpaddr.Port)
}

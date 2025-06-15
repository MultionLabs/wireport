package types

import (
	"errors"
	"net"
	"strconv"
	"strings"
)

type IPNetMarshable struct {
	net.IPNet
}

func ParseIPNetMarshable(s string, maskMustBeSpecified bool) (*IPNetMarshable, error) {
	if s == "" {
		return nil, errors.New("invalid IP network format, expected 'IP/CIDR'")
	}

	parts := strings.Split(s, "/")

	if len(parts) == 0 {
		return nil, errors.New("invalid IP network format, expected 'IP/CIDR'")
	}

	ip := net.ParseIP(parts[0])

	if ip == nil {
		return nil, errors.New("invalid IP address")
	}

	if len(parts) == 1 {
		if maskMustBeSpecified {
			return nil, errors.New("network mask must be specified")
		}

		return &IPNetMarshable{IPNet: net.IPNet{IP: ip, Mask: net.CIDRMask(32, 32)}}, nil
	}

	ones, err := strconv.Atoi(parts[1])

	if err != nil {
		return nil, errors.New("invalid CIDR value for network mask")
	}

	if ones < 0 || ones > 32 {
		return nil, errors.New("CIDR value for network mask must be between 0 and 32")
	}

	return &IPNetMarshable{IPNet: net.IPNet{IP: ip, Mask: net.CIDRMask(ones, 32)}}, nil
}

func MapIPNetMarshablesToStrings(ipnets []IPNetMarshable, includeMask bool) []string {
	result := make([]string, len(ipnets))

	for i, ipnet := range ipnets {
		if includeMask {
			result[i] = ipnet.String()
		} else {
			result[i] = IPToString(ipnet.IP)
		}
	}

	return result
}

func MapStringsToIPNetMarshables(strs []string) []IPNetMarshable {
	result := make([]IPNetMarshable, 0, len(strs))

	for _, s := range strs {
		s = strings.TrimSpace(s)

		if s == "" {
			continue
		}

		ipnet, err := ParseIPNetMarshable(s, false)

		if err != nil {
			continue
		}

		result = append(result, *ipnet)
	}

	return result
}

package commands

import (
	"fmt"
	"io"
	"strings"
	"wireport/internal/networkapps"
	"wireport/internal/publicservices"
)

func (s *LocalCommandsService) ServicePublish(stdOut io.Writer, errOut io.Writer, requestFromNodeID *string,
	localProtocol string, localHost string, localPort uint16, publicProtocol string, publicHost string, publicPort uint16) {
	err := s.PublicServicesRepository.Save(&publicservices.PublicService{
		PublishedByNodeID: requestFromNodeID,
		LocalProtocol:     localProtocol,
		LocalHost:         localHost,
		LocalPort:         localPort,
		PublicProtocol:    publicProtocol,
		PublicHost:        publicHost,
		PublicPort:        publicPort,
	})

	if err != nil {
		fmt.Fprintf(errOut, "Error creating public service: %v\n", err)
		return
	}

	gatewayNode, err := s.NodesRepository.GetGatewayNode()

	if err != nil {
		fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
		return
	}

	publicServices, err := s.PublicServicesRepository.GetAll()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
		return
	}

	err = gatewayNode.SaveConfigs(publicServices, false)

	if err != nil {
		fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
		return
	}

	err = networkapps.RestartNetworkApps(false, false, true)

	if err != nil {
		fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "✅ Service %s://%s:%d is now published on\n\n\t\t%s://%s:%d\n\n\n", localProtocol, localHost, localPort, publicProtocol, publicHost, publicPort)
}

func (s *LocalCommandsService) ServiceUnpublish(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	serviceDeleted := s.PublicServicesRepository.Delete(publicProtocol, publicHost, publicPort)

	if serviceDeleted {
		gatewayNode, err := s.NodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		publicServices, err := s.PublicServicesRepository.GetAll()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServices, false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "✅ Service %s://%s:%d is now unpublished\n", publicProtocol, publicHost, publicPort)
	} else {
		fmt.Fprintf(stdOut, "❌ Service %s://%s:%d was not found or was already unpublished earliner\n", publicProtocol, publicHost, publicPort)
	}
}

func (s *LocalCommandsService) ServiceList(stdOut io.Writer, errOut io.Writer) {
	services, err := s.PublicServicesRepository.GetAll()

	if err != nil {
		fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "PUBLIC\t->\tLOCAL\n")
	fmt.Fprintf(stdOut, "%s\n", strings.Repeat("=", 80))

	if len(services) > 0 {
		for _, service := range services {
			fmt.Fprintf(stdOut, "%s://%s:%d\t->\t%s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort, service.LocalProtocol, service.LocalHost, service.LocalPort)
		}
	} else {
		fmt.Fprintf(stdOut, "No services are published on the gateway.\nUse 'wireport service publish' to publish a new service.\n")
	}
}

func (s *LocalCommandsService) ServiceParamNew(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	added := s.PublicServicesRepository.AddParam(publicProtocol, publicHost, publicPort, paramType, paramValue)

	if added {
		gatewayNode, err := s.NodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		publicServices, err := s.PublicServicesRepository.GetAll()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServices, false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "✅ Parameter '%s' successfully added to service %s://%s:%d\n", paramValue, publicProtocol, publicHost, publicPort)
	} else {
		fmt.Fprintf(stdOut, "❌ Parameter '%s' was not added to service %s://%s:%d (probably already exists)\n", paramValue, publicProtocol, publicHost, publicPort)
	}
}

func (s *LocalCommandsService) ServiceParamRemove(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16, paramType publicservices.PublicServiceParamType, paramValue string) {
	removed := s.PublicServicesRepository.RemoveParam(publicProtocol, publicHost, publicPort, paramType, paramValue)

	if removed {
		gatewayNode, err := s.NodesRepository.GetGatewayNode()

		if err != nil {
			fmt.Fprintf(errOut, "Error getting gateway node: %v\n", err)
			return
		}

		publicServices, err := s.PublicServicesRepository.GetAll()

		if err != nil {
			fmt.Fprintf(errOut, "Failed to list services: %v\n", err)
			return
		}

		err = gatewayNode.SaveConfigs(publicServices, false)

		if err != nil {
			fmt.Fprintf(errOut, "Error saving gateway node configs: %v\n", err)
			return
		}

		err = networkapps.RestartNetworkApps(false, false, true)

		if err != nil {
			fmt.Fprintf(errOut, "Error restarting services: %v\n", err)
			return
		}

		fmt.Fprintf(stdOut, "✅ Parameter '%s' successfully removed from service %s://%s:%d\n", paramValue, publicProtocol, publicHost, publicPort)
	} else {
		fmt.Fprintf(stdOut, "❌ Parameter '%s' was not removed from service %s://%s:%d (probably not found)\n", paramValue, publicProtocol, publicHost, publicPort)
	}
}

func (s *LocalCommandsService) ServiceParamList(stdOut io.Writer, errOut io.Writer, publicProtocol string, publicHost string, publicPort uint16) {
	service, err := s.PublicServicesRepository.Get(publicProtocol, publicHost, publicPort)

	if err != nil {
		fmt.Fprintf(errOut, "Error getting service: %v\n", err)
		return
	}

	fmt.Fprintf(stdOut, "SERVICE PARAMS: %s://%s:%d\n", service.PublicProtocol, service.PublicHost, service.PublicPort)
	fmt.Fprintf(stdOut, "%s\n", strings.Repeat("=", 80))

	if len(service.Params) > 0 {
		for _, param := range service.Params {
			fmt.Fprintf(stdOut, "%s\n", param.ParamValue)
		}
	} else {
		fmt.Fprintf(stdOut, "No params are set for this service\nUse 'wireport service param new' to add a new param.\n")
	}

	fmt.Fprintf(stdOut, "\n")
}

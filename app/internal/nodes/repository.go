package nodes

import (
	"errors"
	"fmt"
	"net"
	"slices"

	"wireport/cmd/server/config"
	docker_utils "wireport/internal/dockerutils"
	"wireport/internal/encryption/mtls"
	"wireport/internal/logger"

	"wireport/internal/nodes/types"
	"wireport/internal/wg"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	// In 172.16.0.0/12 range, we can use networks from 172.16.0.0 to 172.31.0.0
	// That's 16 possible networks (16-31 in the second octet); we only use 20-31
	dockerSubnetStart = 20
	dockerSubnetEnd   = 31
	// wg node range (10.0.0.0/24)
	wgPrivateIPStart = 1
	wgPrivateIPEnd   = 254
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{
		db: db,
	}
}

func (r *Repository) filterNodes(allNodes *[]types.Node, roles []types.NodeRole) []types.Node {
	var clientServerNodes []types.Node

	for _, node := range *allNodes {
		if slices.Contains(roles, node.Role) {
			clientServerNodes = append(clientServerNodes, node)
		}
	}

	return clientServerNodes
}

func (r *Repository) updateNodes() error {
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var hostNode types.Node
		tx.First(&hostNode, "role = ?", types.NodeRoleHost)

		if hostNode.ID == "" {
			return ErrHostNodeNotFound
		}

		if hostNode.WGPublicIP == nil || hostNode.WGPublicPort == nil {
			return ErrHostNodePublicIPPortNotFound
		}

		var hostEndpoint = types.UDPAddrMarshable{
			UDPAddr: net.UDPAddr{
				IP:   net.ParseIP(*hostNode.WGPublicIP),
				Port: int(*hostNode.WGPublicPort),
			},
		}

		var oldNodes []types.Node
		tx.Find(&oldNodes)

		//

		clientServerNodes := r.filterNodes(&oldNodes, []types.NodeRole{types.NodeRoleClient, types.NodeRoleServer})

		dockerDNS := "127.0.0.11"
		persistentKeepalive := 15
		serverPeerAllowedIps := "10.0.0.0/24"
		dockerAllAllowedSubnets := "172.16.0.0/12"
		precisePeerIPTemplate := "%s/32"
		imprecisePeerIPTemplate := "%s/24"
		for _, node := range oldNodes {
			// HOST - list of all client and server nodes as peers
			if node.Role == types.NodeRoleHost {
				// DNS
				serverNodes := r.filterNodes(&oldNodes, []types.NodeRole{types.NodeRoleServer})
				dnsServerAddresses := []string{dockerDNS}

				for _, serverNode := range serverNodes {
					dnsServerAddresses = append(dnsServerAddresses, types.IPToString(serverNode.WGConfig.Interface.Address.IP))
				}

				node.WGConfig.Interface.DNS = types.MapStringsToIPNetMarshables(dnsServerAddresses)

				// PEERS
				node.WGConfig.Peers = []types.WGConfigPeer{}

				for _, clientServerNode := range clientServerNodes {
					var allowedIPs []string

					switch clientServerNode.Role {
					case types.NodeRoleServer:
						allowedIPs = []string{
							fmt.Sprintf(precisePeerIPTemplate, types.IPToString(clientServerNode.WGConfig.Interface.Address.IP)),
							clientServerNode.DockerSubnet.String(),
						}
					case types.NodeRoleClient:
						allowedIPs = []string{fmt.Sprintf(precisePeerIPTemplate, types.IPToString(clientServerNode.WGConfig.Interface.Address.IP))}
					}

					node.WGConfig.Peers = append(node.WGConfig.Peers, types.WGConfigPeer{
						PublicKey:  clientServerNode.WGPublicKey,
						AllowedIPs: types.MapStringsToIPNetMarshables(allowedIPs),
					})
				}
			}

			// SERVERS & CLIENTS

			if node.Role == types.NodeRoleServer || node.Role == types.NodeRoleClient {
				// DNS
				dnsServerAddresses := []string{types.IPToString(hostNode.WGConfig.Interface.Address.IP)}

				// PEERS
				allowedIPs := []string{}

				switch node.Role {
				case types.NodeRoleServer:
					dnsServerAddresses = append(dnsServerAddresses, dockerDNS)
					allowedIPs = []string{serverPeerAllowedIps}
				case types.NodeRoleClient:
					allowedIPs = []string{
						dockerAllAllowedSubnets,
						fmt.Sprintf(imprecisePeerIPTemplate, types.IPToString(hostNode.WGConfig.Interface.Address.IP)),
					}
				}

				node.WGConfig.Interface.DNS = types.MapStringsToIPNetMarshables(dnsServerAddresses)

				node.WGConfig.Peers = []types.WGConfigPeer{
					{
						PublicKey:           hostNode.WGPublicKey,
						Endpoint:            &hostEndpoint,
						AllowedIPs:          types.MapStringsToIPNetMarshables(allowedIPs),
						PersistentKeepalive: &persistentKeepalive,
					},
				}
			}

			tx.Save(&node)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func (r *Repository) CreateHost(WGPublicIP types.IPMarshable, WGPublicPort uint16, hostPublicIP string, hostPublicPort uint16) (*types.Node, error) {
	logger.Info("Creating host node")

	if r.db.First(&types.Node{}, "role = ?", types.NodeRoleHost).RowsAffected > 0 {
		// only one host node is allowed
		return nil, ErrHostNodeAlreadyExists
	}

	wgPrivateIP, err := r.GetNextAssignableWGPrivateIP()

	if err != nil {
		return nil, err
	}

	var hostInterfaceAddress = types.IPNetMarshable{
		IPNet: net.IPNet{
			IP:   wgPrivateIP.IP,
			Mask: net.CIDRMask(24, 32),
		},
	}
	var hostInterfacePostUp = "iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth1 -j MASQUERADE"
	var hostInterfacePostDown = "iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth1 -j MASQUERADE"
	hostInterfaceWGPrivateKey, hostInterfaceWGPublicKey, err := wg.GenerateKeyPair()

	if err != nil {
		return nil, err
	}

	nodeID := uuid.New().String()

	hostCertBundle, err := mtls.Generate(mtls.Options{
		CommonName:  nodeID,
		Expiry:      config.Config.CertExpiry,
		IPAddresses: []string{hostPublicIP},
	}, config.Config.CertExpiry)

	if err != nil {
		return nil, err
	}

	var node *types.Node

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var clientServerNodes []types.Node

		tx.Find(&clientServerNodes, "role = ? OR role = ?", types.NodeRoleClient, types.NodeRoleServer)

		var hostPeers []types.WGConfigPeer = []types.WGConfigPeer{}

		var hostWGPublicIP = WGPublicIP.String()
		var dockerSubnet *types.IPNetMarshable

		dockerSubnet, err = r.GetNextAssignableDockerSubnet()

		if err != nil {
			return err
		}

		// ensure docker network exists and is attached to the container

		if err = docker_utils.EnsureDockerNetworkExistsAndAttached(dockerSubnet); err != nil {
			logger.Error("Failed to ensure docker network exists and is attached to the container: %v", err)
			return err
		}

		node = &types.Node{
			ID:           nodeID,
			Role:         types.NodeRoleHost,
			WGPrivateKey: hostInterfaceWGPrivateKey,
			WGPublicKey:  hostInterfaceWGPublicKey,
			WGConfig: types.WGConfig{
				Interface: types.WGConfigInterface{
					Address:    hostInterfaceAddress,
					ListenPort: &WGPublicPort,
					PrivateKey: hostInterfaceWGPrivateKey,
					DNS:        []types.IPNetMarshable{}, // refreshed in updateNodes
					PostUp:     hostInterfacePostUp,
					PostDown:   hostInterfacePostDown,
				},
				Peers: hostPeers, // refreshed in updateNodes
			},
			WGPublicIP:       &hostWGPublicIP,
			WGPublicPort:     &WGPublicPort,
			HostPublicIP:     hostPublicIP,
			HostPublicPort:   hostPublicPort,
			HostCertBundle:   hostCertBundle,
			ClientCertBundle: nil,
			DockerSubnet:     dockerSubnet,
			IsCurrentNode:    true, // only create on host node
		}

		result := tx.Create(node)

		if result.Error != nil {
			return result.Error
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to create host node")
		return nil, err
	}

	err = r.updateNodes()

	if err != nil {
		return nil, err
	}

	node, err = r.GetByID(node.ID)

	if err != nil {
		return nil, err
	}

	return node, nil
}

func (r *Repository) CreateServer(forceDockerSubnetStr *string) (*types.Node, error) {
	var serverInterfaceWGPrivateKey, serverInterfaceWGPublicKey, err = wg.GenerateKeyPair()

	if err != nil {
		return nil, err
	}

	var node *types.Node

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var hostNode types.Node
		tx.First(&hostNode, "role = ?", types.NodeRoleHost)

		if hostNode.ID == "" {
			return errors.New("host node not found")
		}

		nodeID := uuid.New().String()

		err = hostNode.HostCertBundle.AddClient(mtls.Options{
			CommonName: nodeID,
			Expiry:     config.Config.CertExpiry,
		})

		if err != nil {
			return err
		}

		tx.Save(&hostNode)

		var clientCertBundle *mtls.FullClientBundle

		clientCertBundle, err = hostNode.HostCertBundle.GetClientBundlePublic(nodeID)

		if err != nil {
			return err
		}

		if hostNode.WGPublicIP == nil || hostNode.WGPublicPort == nil {
			return errors.New("host node public ip or port not found")
		}

		var wgPrivateIP *types.IPMarshable
		wgPrivateIP, err = r.GetNextAssignableWGPrivateIP()

		if err != nil {
			return err
		}

		var serverInterfaceAddress = types.IPNetMarshable{
			IPNet: net.IPNet{
				IP:   wgPrivateIP.IP,
				Mask: net.CIDRMask(24, 32),
			},
		}

		var dockerSubnet *types.IPNetMarshable

		if forceDockerSubnetStr != nil {
			dockerSubnet, err = types.ParseIPNetMarshable(*forceDockerSubnetStr, true)

			if err != nil {
				return err
			}

			if !r.IsDockerSubnetAvailable(dockerSubnet) {
				return errors.New("docker subnet already in use")
			}
		} else {
			dockerSubnet, err = r.GetNextAssignableDockerSubnet()
		}

		if err != nil {
			return err
		}

		node = &types.Node{
			ID:           nodeID,
			Role:         types.NodeRoleServer,
			WGPrivateKey: serverInterfaceWGPrivateKey,
			WGPublicKey:  serverInterfaceWGPublicKey,
			WGConfig: types.WGConfig{
				Interface: types.WGConfigInterface{
					Address:    serverInterfaceAddress,
					PrivateKey: serverInterfaceWGPrivateKey,
					DNS:        []types.IPNetMarshable{}, // refreshed in updateNodes
				},
				Peers: []types.WGConfigPeer{
					{
						PublicKey: hostNode.WGPublicKey,
					},
				},
			},
			HostPublicIP:     hostNode.HostPublicIP,
			HostPublicPort:   hostNode.HostPublicPort,
			HostCertBundle:   nil,
			ClientCertBundle: clientCertBundle,
			DockerSubnet:     dockerSubnet,
		}

		result := tx.Create(node)

		if result.Error != nil {
			return result.Error
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	err = r.updateNodes()

	if err != nil {
		return nil, err
	}

	// return the freshly updated node
	node, err = r.GetByID(node.ID)

	if err != nil {
		return nil, err
	}

	return node, nil
}

func (r *Repository) CreateClient() (*types.Node, error) {
	var clientInterfaceWGPrivateKey, clientInterfaceWGPublicKey, err = wg.GenerateKeyPair()

	if err != nil {
		return nil, err
	}

	var node *types.Node

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var allNodes []types.Node

		tx.Find(&allNodes)

		var wgPrivateIP *types.IPMarshable
		wgPrivateIP, err = r.GetNextAssignableWGPrivateIP()

		if err != nil {
			return err
		}

		clientInterfaceAddressIP := types.IPNetMarshable{
			IPNet: net.IPNet{
				IP:   wgPrivateIP.IP,
				Mask: net.CIDRMask(24, 32),
			},
		}

		var hostNode types.Node

		tx.Find(&hostNode, "role = ?", types.NodeRoleHost)

		if hostNode.ID == "" {
			return errors.New("host node not found")
		}

		nodeID := uuid.New().String()

		err = hostNode.HostCertBundle.AddClient(mtls.Options{
			CommonName: nodeID,
			Expiry:     config.Config.CertExpiry,
		})

		if err != nil {
			return err
		}

		tx.Save(&hostNode)

		var clientCertBundle *mtls.FullClientBundle

		clientCertBundle, err = hostNode.HostCertBundle.GetClientBundlePublic(nodeID)

		if err != nil {
			return err
		}

		node = &types.Node{
			ID:           nodeID,
			Role:         types.NodeRoleClient,
			WGPrivateKey: clientInterfaceWGPrivateKey,
			WGPublicKey:  clientInterfaceWGPublicKey,
			WGConfig: types.WGConfig{
				Interface: types.WGConfigInterface{
					Address:    clientInterfaceAddressIP,
					PrivateKey: clientInterfaceWGPrivateKey,
					DNS:        []types.IPNetMarshable{}, // refreshed in updateNodes
				},
				Peers: []types.WGConfigPeer{
					{
						PublicKey: hostNode.WGPublicKey,
					},
				},
			},
			HostPublicIP:     hostNode.HostPublicIP,
			HostPublicPort:   hostNode.HostPublicPort,
			HostCertBundle:   nil,
			ClientCertBundle: clientCertBundle,
			DockerSubnet:     nil,
		}

		result := tx.Create(node)

		if result.Error != nil {
			return result.Error
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	err = r.updateNodes()

	if err != nil {
		return nil, err
	}

	// return the freshly updated node
	node, err = r.GetByID(node.ID)

	if err != nil {
		return nil, err
	}

	return node, nil
}

// GetByID retrieves a node by its ID
func (r *Repository) GetByID(id string) (*types.Node, error) {
	var node types.Node

	result := r.db.First(&node, "id = ?", id)

	if result.Error != nil {
		return nil, result.Error
	}

	return &node, nil
}

func (r *Repository) GetCurrentNode() (*types.Node, error) {
	var node types.Node

	result := r.db.First(&node, "is_current_node = ?", true)

	if result.Error != nil {
		return nil, result.Error
	}

	return &node, nil
}

func (r *Repository) GetHostNode() (*types.Node, error) {
	var hostNode types.Node

	result := r.db.First(&hostNode, "role = ?", types.NodeRoleHost)

	if result.Error != nil {
		if result.Error.Error() == "record not found" {
			return nil, nil
		}
		return nil, result.Error
	}

	return &hostNode, nil
}

func (r *Repository) IsDockerSubnetAvailable(dockerSubnet *types.IPNetMarshable) bool {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleHost)

	if result.Error != nil {
		return false
	}

	for _, node := range nodes {
		if node.DockerSubnet != nil && node.DockerSubnet.String() == dockerSubnet.String() {
			return false
		}
	}

	return true
}

func (r *Repository) IsWGPrivateIPAvailable(WGPrivateIP types.IPMarshable) bool {
	var nodes []types.Node

	result := r.db.Find(&nodes)

	if result.Error != nil {
		return false
	}

	wgPrivateIPStr := WGPrivateIP.String()

	for _, node := range nodes {
		if types.IPToString(node.WGConfig.Interface.Address.IP) == wgPrivateIPStr {
			return false
		}
	}

	return true
}

func (r *Repository) TotalAndAvailableDockerSubnets() (int, int, error) {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleHost)

	if result.Error != nil {
		return 0, 0, result.Error
	}

	return len(nodes), (dockerSubnetEnd - dockerSubnetStart + 1) - len(nodes), nil
}

func (r *Repository) TotalAvailableWireguardClients() (int, int, error) {
	var count int64

	if err := r.db.Model(&types.Node{}).Count(&count).Error; err != nil {
		return 0, 0, err
	}

	return int(count), (wgPrivateIPEnd - wgPrivateIPStart + 1) - int(count), nil
}

func (r *Repository) GetNextAssignableDockerSubnet() (*types.IPNetMarshable, error) {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleHost)

	if result.Error != nil {
		return nil, result.Error
	}

	for networkNum := dockerSubnetStart; networkNum <= dockerSubnetEnd; networkNum++ {
		ip := net.ParseIP(fmt.Sprintf("172.%d.0.0", networkNum))

		if ip == nil {
			return nil, ErrFailedToParseIP
		}

		proposedSubnet := &types.IPNetMarshable{
			IPNet: net.IPNet{
				IP:   ip,
				Mask: net.CIDRMask(16, 32),
			},
		}

		// Check if this subnet is already in use
		subnetExists := false
		for _, node := range nodes {
			if node.DockerSubnet != nil && node.DockerSubnet.String() == proposedSubnet.String() {
				subnetExists = true
				break
			}
		}

		if !subnetExists {
			return proposedSubnet, nil
		}
	}

	return nil, ErrNoAvailableDockerSubnets
}

func (r *Repository) GetNextAssignableWGPrivateIP() (*types.IPMarshable, error) {
	var nodes []types.Node

	result := r.db.Find(&nodes)

	if result.Error != nil {
		return nil, result.Error
	}

	for networkNum := wgPrivateIPStart; networkNum <= wgPrivateIPEnd; networkNum++ {
		ip := net.ParseIP(fmt.Sprintf("10.0.0.%d", networkNum))

		if ip == nil {
			return nil, ErrFailedToParseIP
		}

		proposedIPStr := types.IPToString(ip)

		// Check if this ip is already in use
		ipExists := false

		for _, node := range nodes {
			if types.IPToString(node.WGConfig.Interface.Address.IP) == proposedIPStr {
				ipExists = true
				break
			}
		}

		if !ipExists {
			return &types.IPMarshable{
				IP: ip,
			}, nil
		}
	}

	return nil, ErrNoAvailableWGPrivateIPs
}

func (r *Repository) EnsureHostNode(WGPublicIP types.IPMarshable, WGPublicPort uint16, hostPublicIP string, hostPublicPort uint16) (*types.Node, error) {
	hostNode, err := r.GetHostNode()

	if err != nil {
		return nil, err
	}

	if hostNode == nil {
		logger.Info("Host node not found, creating host node")

		hostNode, err = r.CreateHost(WGPublicIP, WGPublicPort, hostPublicIP, hostPublicPort)

		if err != nil {
			logger.Error("Failed to create host node: %v", err)
			return nil, err
		}
	} else {
		err := docker_utils.EnsureDockerNetworkExistsAndAttached(hostNode.DockerSubnet)

		if err != nil {
			logger.Error("Failed to ensure docker network exists and is attached to the container: %v", err)
			return nil, err
		}
	}

	return hostNode, nil
}

func (r *Repository) SaveNode(node *types.Node) error {
	result := r.db.Save(node)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (r *Repository) IsCurrentNodeHost() bool {
	var node types.Node

	result := r.db.First(&node, "is_current_node = ?", true)

	if result.Error != nil {
		return false
	}

	return node.Role == types.NodeRoleHost
}

func (r *Repository) GetNodesByRole(role types.NodeRole) ([]types.Node, error) {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ?", role)

	if result.Error != nil {
		return nil, result.Error
	}

	return nodes, nil
}

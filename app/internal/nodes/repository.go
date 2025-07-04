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
		var gatewayNode types.Node
		tx.First(&gatewayNode, "role = ?", types.NodeRoleGateway)

		if gatewayNode.ID == "" {
			return ErrGatewayNodeNotFound
		}

		if gatewayNode.WGPublicIP == nil || gatewayNode.WGPublicPort == nil {
			return ErrGatewayNodePublicIPPortNotFound
		}

		var gatewayEndpoint = types.UDPAddrMarshable{
			UDPAddr: net.UDPAddr{
				IP:   net.ParseIP(*gatewayNode.WGPublicIP),
				Port: int(*gatewayNode.WGPublicPort),
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
			// GATEWAY - list of all client and server nodes as peers
			if node.Role == types.NodeRoleGateway {
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
				dnsServerAddresses := []string{types.IPToString(gatewayNode.WGConfig.Interface.Address.IP)}

				// PEERS
				allowedIPs := []string{}

				switch node.Role {
				case types.NodeRoleServer:
					dnsServerAddresses = append(dnsServerAddresses, dockerDNS)
					allowedIPs = []string{serverPeerAllowedIps}
				case types.NodeRoleClient:
					allowedIPs = []string{
						dockerAllAllowedSubnets,
						fmt.Sprintf(imprecisePeerIPTemplate, types.IPToString(gatewayNode.WGConfig.Interface.Address.IP)),
					}
				}

				node.WGConfig.Interface.DNS = types.MapStringsToIPNetMarshables(dnsServerAddresses)

				node.WGConfig.Peers = []types.WGConfigPeer{
					{
						PublicKey:           gatewayNode.WGPublicKey,
						Endpoint:            &gatewayEndpoint,
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

func (r *Repository) CreateGateway(WGPublicIP types.IPMarshable, WGPublicPort uint16, gatewayPublicIP string, gatewayPublicPort uint16) (*types.Node, error) {
	logger.Info("Creating gateway node")

	if r.db.First(&types.Node{}, "role = ?", types.NodeRoleGateway).RowsAffected > 0 {
		// only one gateway node is allowed
		return nil, ErrGatewayNodeAlreadyExists
	}

	wgPrivateIP, err := r.GetNextAssignableWGPrivateIP()

	if err != nil {
		return nil, err
	}

	var gatewayInterfaceAddress = types.IPNetMarshable{
		IPNet: net.IPNet{
			IP:   wgPrivateIP.IP,
			Mask: net.CIDRMask(24, 32),
		},
	}
	var gatewayInterfacePostUp = "iptables -A FORWARD -i wg0 -j ACCEPT; iptables -t nat -A POSTROUTING -o eth1 -j MASQUERADE"
	var gatewayInterfacePostDown = "iptables -D FORWARD -i wg0 -j ACCEPT; iptables -t nat -D POSTROUTING -o eth1 -j MASQUERADE"
	gatewayInterfaceWGPrivateKey, gatewayInterfaceWGPublicKey, err := wg.GenerateKeyPair()

	if err != nil {
		return nil, err
	}

	nodeID := uuid.New().String()

	gatewayCertBundle, err := mtls.Generate(mtls.Options{
		CommonName:  nodeID,
		Expiry:      config.Config.CertExpiry,
		IPAddresses: []string{gatewayPublicIP},
	}, config.Config.CertExpiry)

	if err != nil {
		return nil, err
	}

	var node *types.Node

	err = r.db.Transaction(func(tx *gorm.DB) error {
		var clientServerNodes []types.Node

		tx.Find(&clientServerNodes, "role = ? OR role = ?", types.NodeRoleClient, types.NodeRoleServer)

		var gatewayPeers []types.WGConfigPeer = []types.WGConfigPeer{}

		var gatewayWGPublicIP = WGPublicIP.String()
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
			Role:         types.NodeRoleGateway,
			WGPrivateKey: gatewayInterfaceWGPrivateKey,
			WGPublicKey:  gatewayInterfaceWGPublicKey,
			WGConfig: types.WGConfig{
				Interface: types.WGConfigInterface{
					Address:    gatewayInterfaceAddress,
					ListenPort: &WGPublicPort,
					PrivateKey: gatewayInterfaceWGPrivateKey,
					DNS:        []types.IPNetMarshable{}, // refreshed in updateNodes
					PostUp:     gatewayInterfacePostUp,
					PostDown:   gatewayInterfacePostDown,
				},
				Peers: gatewayPeers, // refreshed in updateNodes
			},
			WGPublicIP:        &gatewayWGPublicIP,
			WGPublicPort:      &WGPublicPort,
			GatewayPublicIP:   gatewayPublicIP,
			GatewayPublicPort: gatewayPublicPort,
			GatewayCertBundle: gatewayCertBundle,
			ClientCertBundle:  nil,
			DockerSubnet:      dockerSubnet,
			IsCurrentNode:     true, // only create on gateway node
		}

		result := tx.Create(node)

		if result.Error != nil {
			return result.Error
		}

		return nil
	})

	if err != nil {
		logger.Error("Failed to create gateway node")
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
		var gatewaytNode types.Node
		tx.First(&gatewaytNode, "role = ?", types.NodeRoleGateway)

		if gatewaytNode.ID == "" {
			return errors.New("gateway node not found")
		}

		nodeID := uuid.New().String()

		err = gatewaytNode.GatewayCertBundle.AddClient(mtls.Options{
			CommonName: nodeID,
			Expiry:     config.Config.CertExpiry,
		})

		if err != nil {
			return err
		}

		tx.Save(&gatewaytNode)

		var clientCertBundle *mtls.FullClientBundle

		clientCertBundle, err = gatewaytNode.GatewayCertBundle.GetClientBundlePublic(nodeID)

		if err != nil {
			return err
		}

		if gatewaytNode.WGPublicIP == nil || gatewaytNode.WGPublicPort == nil {
			return errors.New("gateway node public ip or port not found")
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
						PublicKey: gatewaytNode.WGPublicKey,
					},
				},
			},
			GatewayPublicIP:   gatewaytNode.GatewayPublicIP,
			GatewayPublicPort: gatewaytNode.GatewayPublicPort,
			GatewayCertBundle: nil,
			ClientCertBundle:  clientCertBundle,
			DockerSubnet:      dockerSubnet,
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

		var gatewayNode types.Node

		tx.Find(&gatewayNode, "role = ?", types.NodeRoleGateway)

		if gatewayNode.ID == "" {
			return errors.New("gateway node not found")
		}

		nodeID := uuid.New().String()

		err = gatewayNode.GatewayCertBundle.AddClient(mtls.Options{
			CommonName: nodeID,
			Expiry:     config.Config.CertExpiry,
		})

		if err != nil {
			return err
		}

		tx.Save(&gatewayNode)

		var clientCertBundle *mtls.FullClientBundle

		clientCertBundle, err = gatewayNode.GatewayCertBundle.GetClientBundlePublic(nodeID)

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
						PublicKey: gatewayNode.WGPublicKey,
					},
				},
			},
			GatewayPublicIP:   gatewayNode.GatewayPublicIP,
			GatewayPublicPort: gatewayNode.GatewayPublicPort,
			GatewayCertBundle: nil,
			ClientCertBundle:  clientCertBundle,
			DockerSubnet:      nil,
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

func (r *Repository) GetGatewayNode() (*types.Node, error) {
	var gatewayNode types.Node

	result := r.db.First(&gatewayNode, "role = ?", types.NodeRoleGateway)

	if result.Error != nil {
		if result.Error.Error() == "record not found" {
			return nil, nil
		}
		return nil, result.Error
	}

	return &gatewayNode, nil
}

func (r *Repository) IsDockerSubnetAvailable(dockerSubnet *types.IPNetMarshable) bool {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleGateway)

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

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleGateway)

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

	result := r.db.Find(&nodes, "role = ? OR role = ?", types.NodeRoleServer, types.NodeRoleGateway)

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

func (r *Repository) EnsureGatewayNode(WGPublicIP types.IPMarshable, WGPublicPort uint16, gatewayPublicIP string, gatewayPublicPort uint16) (*types.Node, error) {
	gatewayNode, err := r.GetGatewayNode()

	if err != nil {
		return nil, err
	}

	if gatewayNode == nil {
		logger.Info("Gateway node not found, initiating a new gateway node")

		gatewayNode, err = r.CreateGateway(WGPublicIP, WGPublicPort, gatewayPublicIP, gatewayPublicPort)

		if err != nil {
			logger.Error("Failed to create gateway node: %v", err)
			return nil, err
		}
	} else {
		err := docker_utils.EnsureDockerNetworkExistsAndAttached(gatewayNode.DockerSubnet)

		if err != nil {
			logger.Error("Failed to ensure docker network exists and is attached to the container: %v", err)
			return nil, err
		}
	}

	return gatewayNode, nil
}

func (r *Repository) SaveNode(node *types.Node) error {
	result := r.db.Save(node)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (r *Repository) IsCurrentNodeGateway() bool {
	var node types.Node

	result := r.db.First(&node, "is_current_node = ?", true)

	if result.Error != nil {
		return false
	}

	return node.Role == types.NodeRoleGateway
}

func (r *Repository) GetNodesByRole(role types.NodeRole) ([]types.Node, error) {
	var nodes []types.Node

	result := r.db.Find(&nodes, "role = ?", role)

	if result.Error != nil {
		return nil, result.Error
	}

	return nodes, nil
}

func (r *Repository) CountNodesByRole(role types.NodeRole) (int, error) {
	var count int64

	result := r.db.Model(&types.Node{}).Where("role = ?", role).Count(&count)

	if result.Error != nil {
		return 0, result.Error
	}

	return int(count), nil
}

func (r *Repository) DeleteServer(nodeID string) error {
	var node types.Node

	result := r.db.Delete(&node, "id = ? AND role = ?", nodeID, types.NodeRoleServer)

	if result.Error != nil {
		return result.Error
	}

	return nil
}

func (r *Repository) DeleteAll() error {
	result := r.db.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&types.Node{})

	if result.Error != nil {
		return result.Error
	}

	return nil
}

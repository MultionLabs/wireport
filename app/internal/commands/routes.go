package commands

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"time"
	"wireport/internal/commands/types"
	"wireport/internal/joinrequests"
	joinrequeststypes "wireport/internal/joinrequests/types"
	"wireport/internal/logger"
	"wireport/internal/networkapps"
	nodes "wireport/internal/nodes"
	node_types "wireport/internal/nodes/types"
	"wireport/internal/publicservices"

	"gorm.io/gorm"
)

// a function that handles a specific command
type RouteHandler func(w http.ResponseWriter, r *http.Request, services *Services)
type Services struct {
	NodesRepository          *nodes.Repository
	PublicServicesRepository *publicservices.Repository
	JoinRequestsRepository   *joinrequests.Repository
	CommandsService          Service
}

// a command execution function
type CommandHandler func(stdOut, errOut *bytes.Buffer) error

// common request validation
func validateRequest(w http.ResponseWriter, r *http.Request, operation string) bool {
	if r.TLS == nil {
		logger.Error("[%s] %s request is not over TLS; dropping request", r.Method, operation)
		http.Error(w, "", http.StatusBadRequest)
		return false
	}

	if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
		logger.Error("[%s] Invalid method or content type: %v", r.Method, r.Method)
		http.Error(w, "", http.StatusBadRequest)
		return false
	}

	return true
}

// a generic handler for requests (request body validation and parsing)
func handleRequestWithBody[T any](w http.ResponseWriter, r *http.Request, handler func(*T, *bytes.Buffer, *bytes.Buffer) error, customResponsePacker func(stdOut, errOut *bytes.Buffer) (any, error)) {
	operation := r.URL.Path
	if !validateRequest(w, r, operation) {
		return
	}

	var requestDTO T
	err := json.NewDecoder(r.Body).Decode(&requestDTO)
	if err != nil {
		logger.Error("[%s] Failed to parse %s request: %v", r.Method, operation, err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	stdOut := bytes.NewBufferString("")
	errOut := bytes.NewBufferString("")

	err = handler(&requestDTO, stdOut, errOut)
	if err != nil {
		logger.Error("[%s] Failed to execute %s: %v", r.Method, operation, err)
		http.Error(w, "", http.StatusBadRequest)
		return
	}

	var response any // JSON-serializable type

	if customResponsePacker != nil {
		response, err = customResponsePacker(stdOut, errOut)

		if err != nil {
			logger.Error("[%s] Failed to pack %s response: %v", r.Method, operation, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}
	} else {
		response = types.ExecResponseDTO{
			Stdout: strings.TrimSpace(stdOut.String()),
			Stderr: strings.TrimSpace(errOut.String()),
		}
	}

	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		logger.Error("[%s] Failed to encode %s response: %v", r.Method, operation, err)
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	if execResp, ok := response.(types.ExecResponseDTO); ok {
		logger.Info("[%s] %s response: stdout: %v, stderr: %v", r.Method, operation, execResp.Stdout[:min(len(execResp.Stdout), 20)], execResp.Stderr[:min(len(execResp.Stderr), 20)])
	} else {
		logger.Info("[%s] %s response: custom response type", r.Method, operation)
	}
}

func RegisterRoutes(mux *http.ServeMux, db *gorm.DB) {
	nodesRepository := nodes.NewRepository(db)
	publicServicesRepository := publicservices.NewRepository(db)
	joinRequestsRepository := joinrequests.NewRepository(db)

	services := &Services{
		NodesRepository:          nodesRepository,
		PublicServicesRepository: publicServicesRepository,
		JoinRequestsRepository:   joinRequestsRepository,
		CommandsService: Service{
			LocalCommandsService: LocalCommandsService{
				NodesRepository:          nodesRepository,
				PublicServicesRepository: publicServicesRepository,
				JoinRequestsRepository:   joinRequestsRepository,
			},
			NodesRepository:          nodesRepository,
			PublicServicesRepository: publicServicesRepository,
			JoinRequestsRepository:   joinRequestsRepository,
		},
	}

	// Server routes
	mux.HandleFunc("/commands/server/new", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServerNewRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServerNew(stdOut, errOut, req.Force, req.Quiet, req.DockerSubnet)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/server/remove", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServerRemoveRequestDTO, stdOut, errOut *bytes.Buffer) error {
			if req.NodeID != r.TLS.PeerCertificates[0].Subject.CommonName {
				logger.Error("[%s] Server can only remove itself; node removal request came from a different node: requested node ID: %s, current node ID: %s", r.Method, req.NodeID, r.TLS.PeerCertificates[0].Subject.CommonName)
				return ErrFailedToCreateServerNode
			}
			services.CommandsService.ServerRemove(stdOut, errOut, req.NodeID)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/server/list", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(_ *types.ServerListRequestDTO, stdOut, errOut *bytes.Buffer) error {
			requestFromNodeID := r.TLS.PeerCertificates[0].Subject.CommonName
			services.CommandsService.ServerList(&requestFromNodeID, stdOut, errOut)
			return nil
		}, func(stdOut, errOut *bytes.Buffer) (any, error) {
			serversCount, err := services.NodesRepository.CountNodesByRole(node_types.NodeRoleServer)

			if err != nil {
				return nil, err
			}

			return types.ServerListResponseDTO{
				ExecResponseDTO: types.ExecResponseDTO{
					Stdout: strings.TrimSpace(stdOut.String()),
					Stderr: strings.TrimSpace(errOut.String()),
				},
				ServerNodesCount: serversCount,
			}, nil
		})
	})

	// Client routes
	mux.HandleFunc("/commands/client/new", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ClientNewRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ClientNew(stdOut, errOut, req.JoinRequest, req.Quiet, req.Wait)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/client/list", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(_ *types.ClientListRequestDTO, stdOut, errOut *bytes.Buffer) error {
			requestFromNodeID := r.TLS.PeerCertificates[0].Subject.CommonName
			services.CommandsService.ClientList(&requestFromNodeID, stdOut, errOut)
			return nil
		}, nil)
	})

	// Service routes
	mux.HandleFunc("/commands/service/publish", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServicePublishRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServicePublish(stdOut, errOut, req.LocalProtocol, req.LocalHost, req.LocalPort, req.PublicProtocol, req.PublicHost, req.PublicPort)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/service/unpublish", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServiceUnpublishRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServiceUnpublish(stdOut, errOut, req.PublicProtocol, req.PublicHost, req.PublicPort)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/service/list", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(_ *types.ServiceListRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServiceList(stdOut, errOut)
			return nil
		}, nil)
	})

	// Service parameter routes
	mux.HandleFunc("/commands/service/params/new", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServiceParamNewRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServiceParamNew(stdOut, errOut, req.PublicProtocol, req.PublicHost, req.PublicPort, req.ParamType, req.ParamValue)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/service/params/remove", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServiceParamRemoveRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServiceParamRemove(stdOut, errOut, req.PublicProtocol, req.PublicHost, req.PublicPort, req.ParamType, req.ParamValue)
			return nil
		}, nil)
	})

	mux.HandleFunc("/commands/service/params/list", func(w http.ResponseWriter, r *http.Request) {
		handleRequestWithBody(w, r, func(req *types.ServiceParamListRequestDTO, stdOut, errOut *bytes.Buffer) error {
			services.CommandsService.ServiceParamList(stdOut, errOut, req.PublicProtocol, req.PublicHost, req.PublicPort)
			return nil
		}, nil)
	})

	// special case with different response format
	mux.HandleFunc("/commands/join", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Join request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		logger.Info("[%s] Join request from %s", r.Method, r.RemoteAddr)

		switch r.Method {
		case http.MethodPost:
			// 1. Parse & validate request
			var joinRequestDto = types.JoinRequestDTO{}

			if err := json.NewDecoder(r.Body).Decode(&joinRequestDto); err != nil {
				logger.Error("[%s] Failed to parse request: %v", r.Method, err)
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			// Decode the join token from base64 to get the join request ID
			decryptedJoinRequest := &joinrequeststypes.JoinRequest{}

			err := decryptedJoinRequest.FromBase64(joinRequestDto.JoinToken)

			if err != nil {
				logger.Error("[%s] Failed to decode join token: %v", r.Method, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			joinRequestFromDB, err := services.JoinRequestsRepository.Get(decryptedJoinRequest.ID)

			if err != nil {
				logger.Error("[%s] Failed to get join request from DB: %v", r.Method, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			joinRequestFromDBBase64, err := joinRequestFromDB.ToBase64()

			if err != nil {
				logger.Error("[%s] Failed to encode join request from DB: %v", r.Method, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			if joinRequestDto.JoinToken != *joinRequestFromDBBase64 {
				// must be identical, otherwise it's a man-in-the-middle attack
				logger.Error("[%s] Join request is invalid: %v", r.Method, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			// 2. Create the node & pack the configs into a response object

			responsePayload := types.JoinResponseDTO{}

			var serverNode, clientNode, gatewayNode *node_types.Node

			switch joinRequestFromDB.Role {
			case node_types.NodeRoleServer:
				serverNode, err = services.NodesRepository.CreateServer(decryptedJoinRequest.DockerSubnet)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateServerNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				gatewayNode, err = services.NodesRepository.GetGatewayNode()

				if err != nil || gatewayNode == nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetGatewayNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				publicServices := services.PublicServicesRepository.GetAll()

				err = gatewayNode.SaveConfigs(publicServices, false)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveGatewayConfigs, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = gatewayNode.GatewayCertBundle.RemoveClient(joinRequestFromDB.ID)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to remove client from gateway cert bundle", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = services.NodesRepository.SaveNode(gatewayNode)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to save gateway node", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				_ = networkapps.RestartNetworkApps(true, false, false)

				// Schedule service restart for 10 seconds later to ensure coredns picks up the new server node
				networkapps.ScheduleNetworkAppsRestart(10*time.Second, false, true, true)

				responsePayload.NodeConfig = serverNode
			case node_types.NodeRoleClient:
				clientNode, err = services.NodesRepository.CreateClient()

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateClientNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				gatewayNode, err = services.NodesRepository.GetGatewayNode()

				if err != nil || gatewayNode == nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetGatewayNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				logger.Info("[%s] Client node created from join request", r.Method)

				publicServices := services.PublicServicesRepository.GetAll()

				err = gatewayNode.SaveConfigs(publicServices, false)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveGatewayConfigs, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = gatewayNode.GatewayCertBundle.RemoveClient(joinRequestFromDB.ID)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to remove client from gateway cert bundle", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = services.NodesRepository.SaveNode(gatewayNode)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to save gateway node", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = networkapps.RestartNetworkApps(true, false, false)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToRestartServices, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				responsePayload.NodeConfig = clientNode
			default:
				logger.Error("[%s] %v: %v", r.Method, ErrInvalidJoinRequestRole, joinRequestFromDB.Role)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			err = services.JoinRequestsRepository.Delete(decryptedJoinRequest.ID)

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToDeleteJoinRequest, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			logger.Info("[%s] Join request processed: %v", r.Method, decryptedJoinRequest.ID)

			// 3. Send the response directly (no encryption)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			err = json.NewEncoder(w).Encode(responsePayload)

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, "Failed to encode response", err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}
		default:
			logger.Error("[%s] Invalid method: %v", r.Method, r.Method)
			http.Error(w, "", http.StatusBadRequest)
		}
	})
}

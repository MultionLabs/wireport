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

func RegisterRoutes(mux *http.ServeMux, db *gorm.DB) {
	nodesRepository := nodes.NewRepository(db)
	publicServicesRepository := publicservices.NewRepository(db)
	joinRequestsRepository := joinrequests.NewRepository(db)
	commandsService := Service{}

	mux.HandleFunc("/commands/server/new", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Server new request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			logger.Error("[%s] Invalid method or content type: %v", r.Method, r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		var serverNewRequestDTO types.ServerNewRequestDTO

		err := json.NewDecoder(r.Body).Decode(&serverNewRequestDTO)

		if err != nil {
			logger.Error("[%s] Failed to parse server new request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		stdOut := bytes.NewBufferString("")
		errOut := bytes.NewBufferString("")

		commandsService.ServerNew(nodesRepository, joinRequestsRepository, stdOut, errOut, serverNewRequestDTO.Force, serverNewRequestDTO.Quiet, serverNewRequestDTO.DockerSubnet)

		exitCode := 0

		if len(errOut.String()) > 0 {
			exitCode = 1
		}

		response := types.ExecResponseDTO{
			Stdout:   strings.TrimSpace(stdOut.String()),
			Stderr:   strings.TrimSpace(errOut.String()),
			ExitCode: exitCode,
		}

		err = json.NewEncoder(w).Encode(response)

		if err != nil {
			logger.Error("[%s] Failed to encode server new response: %v", r.Method, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		logger.Info("[%s] Server new response: %v", r.Method, response)
	})

	mux.HandleFunc("/commands/server/list", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Server list request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			logger.Error("[%s] Invalid method or content type: %v", r.Method, r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		requestFromNodeID := r.TLS.PeerCertificates[0].Subject.CommonName

		var serverListRequestDTO types.ServerListRequestDTO

		err := json.NewDecoder(r.Body).Decode(&serverListRequestDTO)

		if err != nil {
			logger.Error("[%s] Failed to parse server list request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		stdOut := bytes.NewBufferString("")
		errOut := bytes.NewBufferString("")

		commandsService.ServerList(nodesRepository, &requestFromNodeID, stdOut, errOut)

		exitCode := 0

		if len(errOut.String()) > 0 {
			exitCode = 1
		}

		response := types.ExecResponseDTO{
			Stdout:   strings.TrimSpace(stdOut.String()),
			Stderr:   strings.TrimSpace(errOut.String()),
			ExitCode: exitCode,
		}

		err = json.NewEncoder(w).Encode(response)

		if err != nil {
			logger.Error("[%s] Failed to encode server list response: %v", r.Method, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		logger.Info("[%s] Server list response: %v", r.Method, response)
	})

	mux.HandleFunc("/commands/client/new", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Client new request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			logger.Error("[%s] Invalid method or content type: %v", r.Method, r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		var clientNewRequestDTO types.ClientNewRequestDTO

		err := json.NewDecoder(r.Body).Decode(&clientNewRequestDTO)

		if err != nil {
			logger.Error("[%s] Failed to parse client new request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		stdOut := bytes.NewBufferString("")
		errOut := bytes.NewBufferString("")

		commandsService.ClientNew(nodesRepository, joinRequestsRepository, publicServicesRepository, stdOut, errOut, clientNewRequestDTO.JoinRequest, clientNewRequestDTO.Quiet, clientNewRequestDTO.Wait)

		exitCode := 0

		if len(errOut.String()) > 0 {
			exitCode = 1
		}

		response := types.ExecResponseDTO{
			Stdout:   strings.TrimSpace(stdOut.String()),
			Stderr:   strings.TrimSpace(errOut.String()),
			ExitCode: exitCode,
		}

		err = json.NewEncoder(w).Encode(response)

		if err != nil {
			logger.Error("[%s] Failed to encode server new response: %v", r.Method, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		logger.Info("[%s] Client new response: %v", r.Method, response)
	})

	mux.HandleFunc("/commands/client/list", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Client list request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
			logger.Error("[%s] Invalid method or content type: %v", r.Method, r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		requestFromNodeID := r.TLS.PeerCertificates[0].Subject.CommonName

		var clientListRequestDTO types.ClientListRequestDTO

		err := json.NewDecoder(r.Body).Decode(&clientListRequestDTO)

		if err != nil {
			logger.Error("[%s] Failed to parse client list request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		stdOut := bytes.NewBufferString("")
		errOut := bytes.NewBufferString("")

		commandsService.ClientList(nodesRepository, &requestFromNodeID, stdOut, errOut)

		exitCode := 0

		if len(errOut.String()) > 0 {
			exitCode = 1
		}

		response := types.ExecResponseDTO{
			Stdout:   strings.TrimSpace(stdOut.String()),
			Stderr:   strings.TrimSpace(errOut.String()),
			ExitCode: exitCode,
		}

		err = json.NewEncoder(w).Encode(response)

		if err != nil {
			logger.Error("[%s] Failed to encode client list response: %v", r.Method, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		logger.Info("[%s] Client list response: %v", r.Method, response)
	})

	mux.HandleFunc("/commands/join", func(w http.ResponseWriter, r *http.Request) {
		if r.TLS == nil {
			logger.Error("[%s] Join request is not over TLS; dropping request", r.Method)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

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

			joinRequestFromDB, err := joinRequestsRepository.Get(decryptedJoinRequest.ID)

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

			var serverNode, clientNode, hostNode *node_types.Node

			switch joinRequestFromDB.Role {
			case node_types.NodeRoleServer:
				serverNode, err = nodesRepository.CreateServer(decryptedJoinRequest.DockerSubnet)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateServerNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				hostNode, err = nodesRepository.GetHostNode()

				if err != nil || hostNode == nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetHostNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				publicServices := publicServicesRepository.GetAll()

				err = hostNode.SaveConfigs(publicServices, false)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveHostConfigs, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = hostNode.HostCertBundle.RemoveClient(joinRequestFromDB.ID)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to remove client from host cert bundle", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = nodesRepository.SaveNode(hostNode)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to save host node", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				_ = networkapps.RestartNetworkApps(true, false, false)

				// Schedule service restart for 10 seconds later to ensure coredns picks up the new server node
				networkapps.ScheduleNetworkAppsRestart(10*time.Second, false, true, true)

				responsePayload.NodeConfig = serverNode
			case node_types.NodeRoleClient:
				clientNode, err = nodesRepository.CreateClient()

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateClientNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				hostNode, err = nodesRepository.GetHostNode()

				if err != nil || hostNode == nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetHostNode, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				logger.Info("[%s] Client node created from join request", r.Method)

				publicServices := publicServicesRepository.GetAll()

				err = hostNode.SaveConfigs(publicServices, false)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveHostConfigs, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = hostNode.HostCertBundle.RemoveClient(joinRequestFromDB.ID)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to remove client from host cert bundle", err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = nodesRepository.SaveNode(hostNode)

				if err != nil {
					logger.Error("[%s] %v: %v", r.Method, "Failed to save host node", err)
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

			err = joinRequestsRepository.Delete(decryptedJoinRequest.ID)

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

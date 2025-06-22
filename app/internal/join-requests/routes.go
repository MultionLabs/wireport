package join_requests

import (
	"encoding/json"
	"net/http"
	"time"
	"wireport/internal/encryption"
	join_requests_types "wireport/internal/join-requests/types"
	"wireport/internal/logger"
	network_apps "wireport/internal/network-apps"
	nodes "wireport/internal/nodes"
	node_types "wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"

	"gorm.io/gorm"
)

func handleJoinRequest(w http.ResponseWriter, r *http.Request, join_requests_repository *Repository, nodes_repository *nodes.Repository, public_services_repository *public_services.Repository) {
	logger.Info("[%s] Join request from %s", r.Method, r.RemoteAddr)

	switch r.Method {
	case http.MethodPost:
		// 1. Parse & validate request
		var encryptedRequest = encryption.EncryptedRequestDTO{}

		if err := json.NewDecoder(r.Body).Decode(&encryptedRequest); err != nil {
			logger.Error("[%s] Failed to parse request: %v", r.Method, err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		joinRequestId := encryptedRequest.SyncID

		joinRequestFromDB, err := join_requests_repository.Get(joinRequestId)

		if err != nil {
			logger.Error("[%s] Failed to get join request from DB: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		decryptedJoinRequestDto, err := encryption.DecryptRequest[join_requests_types.JoinRequestDTO](encryptedRequest, joinRequestFromDB.EncryptionKeyBase64)

		if err != nil {
			logger.Error("[%s] Failed to decrypt join request: %v", r.Method, err)
			http.Error(w, "", http.StatusInternalServerError)
			return
		}

		decryptedJoinRequest := &join_requests_types.JoinRequest{}

		err = decryptedJoinRequest.FromBase64(decryptedJoinRequestDto.JoinToken)

		if err != nil {
			logger.Error("[%s] Failed to decrypt join request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		decryptedJoinRequestBase64, err := decryptedJoinRequest.ToBase64()

		if err != nil {
			logger.Error("[%s] Failed to encrypt join request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		joinRequestFromDBBase64, err := joinRequestFromDB.ToBase64()

		if err != nil {
			logger.Error("[%s] Failed to encrypt join request: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		if *decryptedJoinRequestBase64 != *joinRequestFromDBBase64 {
			// must be identical, otherwise it's a man-in-the-middle attack
			logger.Error("[%s] Join request is invalid: %v", r.Method, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		// 2. Create the node & pack the configs into a response object

		responsePayload := join_requests_types.JoinResponseDTO{}

		switch joinRequestFromDB.Role {
		case node_types.NodeRoleServer:
			serverNode, err := nodes_repository.CreateServer(decryptedJoinRequest.DockerSubnet)

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateServerNode, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			hostNode, err := nodes_repository.GetHostNode()

			if err != nil || hostNode == nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetHostNode, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			publicServices := public_services_repository.GetAll()

			err = hostNode.SaveConfigs(publicServices, false)

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveHostConfigs, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			network_apps.RestartNetworkApps(true, false, false)

			// Schedule service restart for 10 seconds later to ensure coredns picks up the new server node
			network_apps.ScheduleNetworkAppsRestart(10*time.Second, false, true, true)

			responsePayload.NodeConfig = serverNode
		case node_types.NodeRoleClient:
			clientNode, err := nodes_repository.CreateClient()

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToCreateClientNode, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			hostNode, err := nodes_repository.GetHostNode()

			if err != nil || hostNode == nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToGetHostNode, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			logger.Info("[%s] Client node created from join request", r.Method)

			publicServices := public_services_repository.GetAll()

			err = hostNode.SaveConfigs(publicServices, false)

			if err != nil {
				logger.Error("[%s] %v: %v", r.Method, ErrFailedToSaveHostConfigs, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			err = network_apps.RestartNetworkApps(true, false, false)

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

		// 3. Encrypt the response
		encryptedResponse, err := encryption.EncryptResponse(responsePayload, joinRequestId, joinRequestFromDB.EncryptionKeyBase64)

		if err != nil {
			logger.Error("[%s] %v: %v", r.Method, ErrFailedToEncryptResponse, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		err = join_requests_repository.Delete(joinRequestId)

		if err != nil {
			logger.Error("[%s] %v: %v", r.Method, ErrFailedToDeleteJoinRequest, err)
			http.Error(w, "", http.StatusBadRequest)
			return
		}

		logger.Info("[%s] Join request processed: %v", r.Method, joinRequestId)

		// 4. Send the response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(encryptedResponse)
	default:
		logger.Error("[%s] Invalid method: %v", r.Method, r.Method)
		http.Error(w, "", http.StatusBadRequest)
	}
}

func RegisterRoutes(mux *http.ServeMux, db *gorm.DB) {
	nodes_repository := nodes.NewRepository(db)
	join_requests_repository := NewRepository(db)
	public_services_repository := public_services.NewRepository(db)

	mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
		handleJoinRequest(w, r, join_requests_repository, nodes_repository, public_services_repository)
	})
}

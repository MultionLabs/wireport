package join_requests

import (
	"encoding/json"
	"net/http"
	"wireport/internal/encryption"
	join_requests_types "wireport/internal/join-requests/types"
	"wireport/internal/logger"
	nodes "wireport/internal/nodes"
	node_types "wireport/internal/nodes/types"
	public_services "wireport/internal/public-services"
	"wireport/internal/terminal"

	"gorm.io/gorm"
)

func RegisterRoutes(mux *http.ServeMux, db *gorm.DB) {
	nodes_repository := nodes.NewRepository(db)
	join_requests_repository := NewRepository(db)
	public_services_repository := public_services.NewRepository(db)

	mux.HandleFunc("/join", func(w http.ResponseWriter, r *http.Request) {
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
				serverNode, err := nodes_repository.CreateServer()

				if err != nil {
					logger.Error("[%s] Failed to create server node: %v", r.Method, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				hostNode, err := nodes_repository.GetHostNode()

				if err != nil || hostNode == nil {
					logger.Error("[%s] Failed to get host: %v", r.Method, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				publicServices := public_services_repository.GetAll()

				err = hostNode.SaveConfigs(publicServices)

				if err != nil {
					logger.Error("[%s] Failed to save host configs: %v", r.Method, err)
					http.Error(w, "", http.StatusBadRequest)
					return
				}

				err = terminal.RestartServices()

				if err != nil {
					logger.Error("[%s] Failed to restart services: %v", r.Method, err)
				}

				wireguardConfig, _ := serverNode.GetFormattedWireguardConfig()
				resolvConfig, _ := serverNode.GetFormattedResolvConfig()
				coreDNSConfig, _ := serverNode.GetFormattedCoreDNSConfig()

				dockerSubnet := serverNode.DockerSubnet.String()

				responsePayload.JoinConfigs = &join_requests_types.JoinConfigs{
					WireguardConfig: wireguardConfig,
					ResolvConfig:    resolvConfig,
					CoreDNSConfig:   coreDNSConfig,
					DockerSubnet:    &dockerSubnet,
				}
			default:
				logger.Error("[%s] Invalid join request role: %v", r.Method, joinRequestFromDB.Role)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			// 3. Encrypt the response
			encryptedResponse, err := encryption.EncryptResponse(responsePayload, joinRequestId, joinRequestFromDB.EncryptionKeyBase64)

			if err != nil {
				logger.Error("[%s] Failed to encrypt response: %v", r.Method, err)
				http.Error(w, "", http.StatusBadRequest)
				return
			}

			err = join_requests_repository.Delete(joinRequestId)

			if err != nil {
				logger.Error("[%s] Failed to delete join request: %v", r.Method, err)
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
	})
}

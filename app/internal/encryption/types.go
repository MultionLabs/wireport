package encryption

type EncryptedRequestDTO struct {
	SyncID  string `json:"syncId"`
	Payload string `json:"payload"`
}

type EncryptedResponseDTO struct {
	SyncID  string `json:"syncId"`
	Payload string `json:"payload"`
}

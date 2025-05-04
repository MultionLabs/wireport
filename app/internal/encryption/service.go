package encryption

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

func pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)

	return append(data, padtext...)
}

func unpad(data []byte) ([]byte, error) {
	length := len(data)

	if length == 0 {
		return nil, errors.New("invalid padding size")
	}

	padding := int(data[length-1])

	if padding > length || padding == 0 {
		return nil, errors.New("invalid padding")
	}

	return data[:length-padding], nil
}

func EncryptAES(plainText []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	plainText = pad(plainText, block.BlockSize())

	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]

	// Random IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], plainText)

	return cipherText, nil
}

func DecryptAES(cipherText []byte, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)

	if err != nil {
		return nil, err
	}

	if len(cipherText) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	if len(cipherText)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)

	return unpad(plainText)
}

func GenerateKey() ([]byte, error) {
	key := make([]byte, 32)

	_, err := rand.Read(key)

	if err != nil {
		return nil, err
	}

	return key, nil
}

func EncryptedAPIRequest[RT any](client *http.Client, url string, payload interface{}, syncID string, encryptionKeyBase64 string) (*RT, error) {
	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	requestPayloadJSON, err := json.Marshal(payload)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	encryptedRequestPayload, err := EncryptAES(requestPayloadJSON, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("failed to encrypt payload: %w", err)
	}

	encryptedRequestPayloadBase64 := base64.StdEncoding.EncodeToString(encryptedRequestPayload)

	requestBody := EncryptedRequestDTO{
		SyncID:  syncID,
		Payload: encryptedRequestPayloadBase64,
	}

	requestBodyJSON, err := json.Marshal(requestBody)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	resp, err := client.Post(
		url,
		"application/json",
		bytes.NewBuffer(requestBodyJSON),
	)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response EncryptedResponseDTO

	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response body: %w", err)
	}

	encryptedResponsePayloadBytes, err := base64.StdEncoding.DecodeString(response.Payload)

	if err != nil {
		return nil, fmt.Errorf("failed to decode decrypted response payload: %w", err)
	}

	decryptedResponsePayloadBytes, err := DecryptAES(encryptedResponsePayloadBytes, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("failed to decrypt response: %w", err)
	}

	var decryptedResponsePayload RT

	err = json.Unmarshal(decryptedResponsePayloadBytes, &decryptedResponsePayload)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted response: %w", err)
	}

	return &decryptedResponsePayload, nil
}

func DecryptRequest[RT any](encryptedRequest EncryptedRequestDTO, encryptionKeyBase64 string) (*RT, error) {
	encryptedRequestPayload, err := base64.StdEncoding.DecodeString(encryptedRequest.Payload)

	if err != nil {
		return nil, fmt.Errorf("failed to decode encrypted request payload: %w", err)
	}

	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	decryptedRequestPayloadJSON, err := DecryptAES(encryptedRequestPayload, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("failed to decrypt request: %w", err)
	}

	var decryptedRequestPayload RT

	err = json.Unmarshal(decryptedRequestPayloadJSON, &decryptedRequestPayload)

	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal decrypted request payload: %w", err)
	}

	return &decryptedRequestPayload, nil
}

func EncryptResponse(response interface{}, syncID string, encryptionKeyBase64 string) (*EncryptedResponseDTO, error) {
	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("failed to decode encryption key: %w", err)
	}

	responsePayloadJSON, err := json.Marshal(response)

	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}

	encryptedResponsePayload, err := EncryptAES(responsePayloadJSON, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("failed to encrypt response: %w", err)
	}

	encryptedResponsePayloadBase64 := base64.StdEncoding.EncodeToString(encryptedResponsePayload)

	return &EncryptedResponseDTO{
		SyncID:  syncID,
		Payload: encryptedResponsePayloadBase64,
	}, nil
}

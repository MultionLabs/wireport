package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
		return nil, ErrInvalidPaddingSize
	}

	padding := int(data[length-1])

	if padding > length || padding == 0 {
		return nil, ErrInvalidPadding
	}

	return data[:length-padding], nil
}

func EncryptAES(plainText []byte, aesKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)

	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	plainText = pad(plainText, block.BlockSize())

	cipherText := make([]byte, aes.BlockSize+len(plainText))
	iv := cipherText[:aes.BlockSize]

	// Random IV
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("generate IV: %w", err)
	}

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(cipherText[aes.BlockSize:], plainText)

	return cipherText, nil
}

func DecryptAES(cipherText []byte, aesKey []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey)

	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	if len(cipherText) < aes.BlockSize {
		return nil, ErrCiphertextTooShort
	}

	iv := cipherText[:aes.BlockSize]
	cipherText = cipherText[aes.BlockSize:]

	if len(cipherText)%aes.BlockSize != 0 {
		return nil, ErrInvalidBlockSize
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	plainText := make([]byte, len(cipherText))
	mode.CryptBlocks(plainText, cipherText)

	return unpad(plainText)
}

func GenerateAESKey() ([]byte, error) {
	key := make([]byte, 32)

	_, err := rand.Read(key)

	if err != nil {
		return nil, fmt.Errorf("generate random aes key: %w", err)
	}

	return key, nil
}

func EncryptedAPIRequest[RT any](client *http.Client, url string, payload interface{}, syncID string, encryptionKeyBase64 string) (*RT, error) {
	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeEncryptionKey, err)
	}

	requestPayloadJSON, err := json.Marshal(payload)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMarshalPayload, err)
	}

	encryptedRequestPayload, err := EncryptAES(requestPayloadJSON, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncryptPayload, err)
	}

	encryptedRequestPayloadBase64 := base64.StdEncoding.EncodeToString(encryptedRequestPayload)

	requestBody := EncryptedRequestDTO{
		SyncID:  syncID,
		Payload: encryptedRequestPayloadBase64,
	}

	requestBodyJSON, err := json.Marshal(requestBody)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMarshalRequestBody, err)
	}

	resp, err := client.Post(
		url,
		"application/json",
		bytes.NewBuffer(requestBodyJSON),
	)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSendRequest, err)
	}

	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadResponse, err)
	}

	var response EncryptedResponseDTO

	err = json.Unmarshal(responseBody, &response)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshalResponse, err)
	}

	encryptedResponsePayloadBytes, err := base64.StdEncoding.DecodeString(response.Payload)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	decryptedResponsePayloadBytes, err := DecryptAES(encryptedResponsePayloadBytes, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecryptResponse, err)
	}

	var decryptedResponsePayload RT

	err = json.Unmarshal(decryptedResponsePayloadBytes, &decryptedResponsePayload)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshalDecrypted, err)
	}

	return &decryptedResponsePayload, nil
}

func DecryptRequest[RT any](encryptedRequest EncryptedRequestDTO, encryptionKeyBase64 string) (*RT, error) {
	encryptedRequestPayload, err := base64.StdEncoding.DecodeString(encryptedRequest.Payload)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeResponse, err)
	}

	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeEncryptionKey, err)
	}

	decryptedRequestPayloadJSON, err := DecryptAES(encryptedRequestPayload, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecryptResponse, err)
	}

	var decryptedRequestPayload RT

	err = json.Unmarshal(decryptedRequestPayloadJSON, &decryptedRequestPayload)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrUnmarshalDecrypted, err)
	}

	return &decryptedRequestPayload, nil
}

func EncryptResponse(response interface{}, syncID string, encryptionKeyBase64 string) (*EncryptedResponseDTO, error) {
	encryptionKey, err := base64.StdEncoding.DecodeString(encryptionKeyBase64)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrDecodeEncryptionKey, err)
	}

	responsePayloadJSON, err := json.Marshal(response)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMarshalPayload, err)
	}

	encryptedResponsePayload, err := EncryptAES(responsePayloadJSON, encryptionKey)

	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEncryptPayload, err)
	}

	encryptedResponsePayloadBase64 := base64.StdEncoding.EncodeToString(encryptedResponsePayload)

	return &EncryptedResponseDTO{
		SyncID:  syncID,
		Payload: encryptedResponsePayloadBase64,
	}, nil
}

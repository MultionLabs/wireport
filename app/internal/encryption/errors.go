package encryption

import "fmt"

// Common encryption errors
var (
	ErrInvalidPadding     = fmt.Errorf("invalid padding")
	ErrInvalidPaddingSize = fmt.Errorf("invalid padding size")
	ErrCiphertextTooShort = fmt.Errorf("ciphertext too short")
	ErrInvalidBlockSize   = fmt.Errorf("ciphertext is not a multiple of block size")
)

// Request/Response errors
var (
	ErrDecodeEncryptionKey = fmt.Errorf("failed to decode encryption key")
	ErrMarshalPayload      = fmt.Errorf("failed to marshal payload")
	ErrEncryptPayload      = fmt.Errorf("failed to encrypt payload")
	ErrMarshalRequestBody  = fmt.Errorf("failed to marshal request body")
	ErrSendRequest         = fmt.Errorf("failed to send request")
	ErrReadResponse        = fmt.Errorf("failed to read response body")
	ErrUnmarshalResponse   = fmt.Errorf("failed to unmarshal response body")
	ErrDecodeResponse      = fmt.Errorf("failed to decode decrypted response payload")
	ErrDecryptResponse     = fmt.Errorf("failed to decrypt response")
	ErrUnmarshalDecrypted  = fmt.Errorf("failed to unmarshal decrypted response")
)

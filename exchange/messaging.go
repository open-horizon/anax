package exchange

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/config"
	"golang.org/x/crypto/sha3"
	"io/ioutil"
	"os"
	"path"
	"sync"
)

// This module is used to construct a message that can be sent over an insecure transport
// from one Horizon Exchange registered party to another. Since the Exchange is a man in the
// middle and could be perceived as untrusted, it is necessary for both parties to be able to
// exchange messages securely.
//
// The ExchangeMessage is the in memory form of a secure message that can be sent through the
// Horizon Exchange Message Broker. The APIs in this module are used to create and deconstruct
// ExchangeMessages.

type EncryptedWrappedMessage []byte
type EncryptedSymmetricValues []byte

type ExchangeMessage struct {
	WrappedMessage  EncryptedWrappedMessage  `json:"wrappedMessage"`
	SymmetricValues EncryptedSymmetricValues `json:"symmetricValues"`
}

func newExchangeMessage(wMsg EncryptedWrappedMessage, sVal EncryptedSymmetricValues) *ExchangeMessage {
	return &ExchangeMessage{
		WrappedMessage:  wMsg,
		SymmetricValues: sVal,
	}
}

func (self ExchangeMessage) String() string {
	res := ""
	res += fmt.Sprintf("Wrapped Message: %v\n SymmetricValues: %v\n", self.WrappedMessage, self.SymmetricValues)
	return res
}

type WrappedMessage struct {
	Msg          []byte `json:"msg"`
	Signature    []byte `json:"signature"`
	SignerPubKey []byte `json:"signerPubkey"`
}

type SymmetricValues struct {
	Key   []byte `json:"key"`
	Nonce []byte `json:"nonce"`
}

// Here is an overview of what happens in order to construct a secure ExchangeMessage
// 1. create hash of the original message
// 2. digitally sign the hash
// 3. construct a WrappedMessage object including the original message, the signature, and the signer's public key
// 4. symmetrically encrypt the WrappedMessage with a random symmetric key and nonce
// 5. construct a SymmetricValues object including the symmetric key and nonce
// 6. encrypt the SymmetricValues using the public key of the intended receiver
// 7. construct an ExchangeMessage from the encrypted WrappedMessage and the encrypted SymmetricValues

func ConstructExchangeMessage(message []byte, senderPublicKey *rsa.PublicKey, senderPrivateKey *rsa.PrivateKey, receiverPublicKey *rsa.PublicKey) (*ExchangeMessage, error) {

	// Up front sanity checks
	if len(message) == 0 {
		return nil, errors.New(fmt.Sprintf("Error message has length zero"))
	} else if senderPublicKey == nil || senderPrivateKey == nil || receiverPublicKey == nil {
		return nil, errors.New(fmt.Sprintf("Error one of sender public key %v, sender private key %v, or receiver public key %v is nil", senderPublicKey, senderPrivateKey, receiverPublicKey))
	} else if err := senderPrivateKey.Validate(); err != nil {
		return nil, errors.New(fmt.Sprintf("Private key is not valid"))
	}

	// Start constructing the message
	err := error(nil)
	glog.V(6).Infof("Creating ExchangeMessage for %s", message)

	// 1. create a sha3 hash of the original message, called the message digest.
	// Digital signing can be an expensive operation, so we will be signing the hash because
	// it is significantly shorter than the original message.
	digest := sha3.Sum256(message)

	// 2. Sign the hash (digest).
	// Signing the message gives the sender the assurance that its message cannot be altered
	// by a third party.
	var signature []byte
	if signature, err = rsa.SignPSS(rand.Reader, senderPrivateKey, crypto.SHA3_256, digest[:], &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto}); err != nil {
		return nil, errors.New(fmt.Sprintf("Error signing the message, error: %v", err))
	} else if len(signature) == 0 {
		return nil, errors.New(fmt.Sprintf("Error signing the message, signature is empty byte array"))
	} else {
		glog.V(6).Infof("Created message digest %x", digest)
	}

	// 3. construct a WrappedMessage object including the original message, the signature, and the signer's public key.
	// All of thes parts are needed to ensure message integrity.

	var pubKey []byte
	if pubKey, err = MarshalPublicKey(senderPublicKey); err != nil {
		return nil, errors.New(fmt.Sprintf("Error marshalling sender public key, error %v", err))
	} else if len(pubKey) == 0 {
		return nil, errors.New(fmt.Sprintf("Error marshalling sender public key, returned empty byte array"))
	}

	wrappedMessage := &WrappedMessage{
		Msg:          message,
		Signature:    signature,
		SignerPubKey: pubKey,
	}

	// 4. symmetrically encrypt the WrappedMessage with a random symmetric key and nonce.
	// We need to encrypt the original message, digital signature and the public key to make them unreadable to
	// 3rd parties. Symmetric encryption is faster than public/private key encryption, so we will
	// use that for this segment of the ExchangeMessage. The downside is that the message receiver
	// needs the symmetric key and nonce in order to do the encryption, so we will send those to the
	// receiver in the second segment of the message.

	var wmBytes []byte
	if wmBytes, err = json.Marshal(wrappedMessage); err != nil {
		return nil, errors.New(fmt.Sprintf("Error marshalling wrapped message, error %v", err))
	} else if len(wmBytes) == 0 {
		return nil, errors.New(fmt.Sprintf("Error marshalling wrapped message, returned empty byte array"))
	} else {
		glog.V(6).Infof("Created Wrapped Message %s", wmBytes)
	}

	var encryptedMessage EncryptedWrappedMessage
	var symmetricKey, nonce []byte
	if encryptedMessage, symmetricKey, nonce, err = symmetricallyEncrypt(wmBytes); err != nil {
		return nil, errors.New(fmt.Sprintf("Error symmetrically encrypting %v", err))
	} else if len(encryptedMessage) == 0 || len(symmetricKey) == 0 || len(nonce) == 0 {
		return nil, errors.New(fmt.Sprintf("Error symmetrically encrypting, one of encrypted message %v, symmetric key %v, or nonce %v is an empty byte array", encryptedMessage, symmetricKey, nonce))
	} else {
		glog.V(6).Infof("Encrypted wrapped message  %x", encryptedMessage)
	}

	// 5. construct a SymmetricValues object including the symmetric key and nonce.
	// Now that the first part of the Exchange message is secured, we need to construct the second
	// half which contains the symmetric key and nonce needed to decrypt the first half.

	var encryptedSymmetricValues EncryptedSymmetricValues

	symmetricValues := &SymmetricValues{
		Key:   symmetricKey,
		Nonce: nonce,
	}

	var svBytes []byte
	if svBytes, err = json.Marshal(symmetricValues); err != nil {
		return nil, errors.New(fmt.Sprintf("Error marshalling symmetric values, error %v", err))
	} else if len(svBytes) == 0 {
		return nil, errors.New(fmt.Sprintf("Error marshalling symmetric values, returned empty byte array"))
	}

	// 6. encrypt the SymmetricValues using the public key of the intended receiver.
	// Since this data is small, we can use public/private key encryption on it. We will encrypt
	// using the receiver's public key so that only the receiver can decrypt.

	// What's the purpose of the label?
	label := []byte("")

	if encryptedSymmetricValues, err = rsa.EncryptOAEP(sha3.New256(), rand.Reader, receiverPublicKey, svBytes, label); err != nil {
		return nil, errors.New(fmt.Sprintf("Error encrypting symmetric values, error %v", err))
	} else if len(encryptedSymmetricValues) == 0 {
		return nil, errors.New(fmt.Sprintf("Error encrypting symmetric values, returned empty byte array"))
	} else {
		glog.V(6).Infof("Encrypted SymmetricValues %x", encryptedSymmetricValues)
	}

	// 7. construct an ExchangeMessage from the encrypted WrappedMessage and the encrypted SymmetricValues.

	return newExchangeMessage(encryptedMessage, encryptedSymmetricValues), nil

}

// Here is an overview of what happens in order to deconstruct a secure ExchangeMessage. To more deeply
// understand message security implemented here, read the ConstructExchangeMessage function. The
// Deconstruct function is just the reverse operations. Here is the sequence:
// 1. receive the encrypted WrappedMessage and SymmetricValues
// 2. decrypt the symmetric values using the receiver's private key
// 3. use the symmetric key and nonce to decrypt the WrappedMessage
// 4. verify the signature of the hash of the message
// 5. extract the plain text message

func DeconstructExchangeMessage(encryptedMessage []byte, receiverPrivateKey *rsa.PrivateKey) ([]byte, *rsa.PublicKey, error) {

	// Up front sanity checks
	if len(encryptedMessage) == 0 {
		return nil, nil, errors.New(fmt.Sprintf("Error message has length zero"))
	} else if receiverPrivateKey == nil {
		return nil, nil, errors.New(fmt.Sprintf("Error Private key is nil"))
	} else if err := receiverPrivateKey.Validate(); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error Private key is not valid"))
	}

	err := error(nil)

	// 1. receive the encrypted WrappedMessage and SymmetricValues.
	// The encrypted values of these two fields in the message are assumed to have been base64 encoded
	// when placed into the message that is put "on the wire".

	em := new(ExchangeMessage)
	if err = json.Unmarshal(encryptedMessage, &em); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling exchange message %s, error %v", encryptedMessage, err))
	} else if len(em.WrappedMessage) == 0 || len(em.SymmetricValues) == 0 {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling exchange message, one of wrapped message %v or symmetric values %v has length zero.", em.WrappedMessage, em.SymmetricValues))
	}

	glog.V(6).Infof("Encrypted Wrapped Message  %x", em.WrappedMessage)
	glog.V(6).Infof("Encrypted Symmetric Values %x", em.SymmetricValues)

	// 2. decrypt the symmetric values using the receiver's private key.
	// The SymmetricValues section includes the key and nonce needed to decrypt the wrapped message
	// section where the business logic message resides.

	// Decrypt symmetric values
	// What's the purpose of the label?
	label := []byte("")
	var receivedSymValues []byte
	if receivedSymValues, err = rsa.DecryptOAEP(sha3.New256(), rand.Reader, receiverPrivateKey, em.SymmetricValues, label); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error decrypting Symmetric values from message, error %v", err))
	}

	sv := new(SymmetricValues)
	if err = json.Unmarshal(receivedSymValues, &sv); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling symmetric values, error %v", err))
	} else if len(sv.Key) == 0 || len(sv.Nonce) == 0 {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling symmetric values, one of key %v or nonce %v has length zero.", sv.Key, sv.Nonce))
	}

	// 3. use the symmetric key and nonce to decrypt the WrappedMessage.
	// The WrappedMessage section is very long, so it was symmetrically encrypted because it's faster.

	var receivedDecryptedMessage []byte
	if receivedDecryptedMessage, err = symmetricallyDecrypt(em.WrappedMessage, sv.Key, sv.Nonce); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error decrypting message: %v", err))
	} else {
		glog.V(6).Infof("Decrypted Wrapped Message %s", receivedDecryptedMessage)
	}

	wm := new(WrappedMessage)
	if err = json.Unmarshal(receivedDecryptedMessage, &wm); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling wrapped message, error %v", err))
	} else if len(wm.Signature) == 0 || len(wm.SignerPubKey) == 0 {
		return nil, nil, errors.New(fmt.Sprintf("Error unmarshalling wrapped message, one of signature %v or signer public key %v has length zero.", wm.Signature, wm.SignerPubKey))
	} else {
		glog.V(6).Infof("Decrypted Wrapped Signature  %x", wm.Signature)
		glog.V(6).Infof("Decrypted Wrapped Public Key %x", wm.SignerPubKey)
	}

	// 4. verify the signature of the hash of the message

	var receivedPubKey *rsa.PublicKey
	if receivedPubKey, err = DemarshalPublicKey(wm.SignerPubKey); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error demarshalling sender public key, %v", err))
	} else if receivedPubKey == nil {
		return nil, nil, errors.New(fmt.Sprintf("Error demarshalling sender public key, returned public key is nil"))
	}

	//Verify Signature
	receivedDigest := sha3.Sum256(wm.Msg)
	glog.V(6).Infof("Digest %x", receivedDigest)

	if err = rsa.VerifyPSS(receivedPubKey, crypto.SHA3_256, receivedDigest[:], wm.Signature, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthAuto}); err != nil {
		return nil, nil, errors.New(fmt.Sprintf("Error verifying signature, error %v", err))
	} else {
		glog.V(6).Infof("Signature verification successful")
	}

	// 5. extract the plain text message
	return wm.Msg, receivedPubKey, nil
}

// Helper function that uses the PKI X.509 library to serialize an RSA key.
func MarshalPublicKey(key *rsa.PublicKey) ([]byte, error) {

	if key == nil {
		return nil, errors.New(fmt.Sprintf("key must not be nil"))
	} else if pubKey, err := x509.MarshalPKIXPublicKey(key); err != nil {
		return nil, err
	} else {
		return pubKey, nil
	}
}

// Helper function that uses the PKI X.509 library to deserialize an RSA key.
func DemarshalPublicKey(serializedKey []byte) (*rsa.PublicKey, error) {

	var receivedPubKey *rsa.PublicKey
	if receivedKey, err := x509.ParsePKIXPublicKey(serializedKey); err != nil {
		return nil, err
	} else {
		switch receivedKey.(type) {
		case *rsa.PublicKey:
			receivedPubKey = receivedKey.(*rsa.PublicKey)
		default:
			return nil, errors.New(fmt.Sprintf("returned type %T is not *rsa.PublicKey", receivedKey))
		}
	}
	return receivedPubKey, nil
}

// Helper function to symmetrically encrypt a hunk of data using a given key and nonce with the
// GCM block mode cipher.
func symmetricallyEncrypt(data []byte) ([]byte, []byte, []byte, error) {

	// Generate 1 time use symmetric key for encryption of the message
	symmetricKey := make([]byte, 32) // 256 bit symmetric key
	if _, err := rand.Read(symmetricKey); err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("Error getting random symmetric key, error %v", err))
	}

	// Generate 1 time use nonce (that's redundant) for GCM cipher
	nonce := make([]byte, 12) // 96 bit number
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("Error getting random nonce, error %v", err))
	}

	// Now do the encryption
	var encryptedMessage []byte
	// Create a GCM block cipher object and then encrypt the message
	if blockCipher, err := aes.NewCipher(symmetricKey); err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("Error getting AES block cipher object, error %v", err))
	} else if gcmCipher, err := cipher.NewGCM(blockCipher); err != nil {
		return nil, nil, nil, errors.New(fmt.Sprintf("Error getting GCM block cipher object, error %v", err))
	} else {
		// Encrypt the message. We are not using the additional data feature of the GCM
		// algorithm becuse we dont think we need it. This is because we are wrapping this whole
		// symmetrically encrypted message inside a public key encryption that also includes a digital
		// signature.
		encryptedMessage = gcmCipher.Seal(nil, nonce, data, nil)
	}
	return encryptedMessage, symmetricKey, nonce, nil
}

// Helper function to symmetrically decrypt a hunk of data using a given key and nonce with the
// GCM block mode cipher.
func symmetricallyDecrypt(data []byte, key []byte, nonce []byte) ([]byte, error) {

	if len(nonce) != 12 {
		return nil, errors.New(fmt.Sprintf("Error nonce must be 12 bytes long"))
	}

	var receivedDecryptedMessage []byte

	// Create a GCM block cipher object and then encrypt the message
	if blockCipher, err := aes.NewCipher(key); err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting AES block cipher object, error %v", err))
	} else if gcmCipher, err := cipher.NewGCM(blockCipher); err != nil {
		return nil, errors.New(fmt.Sprintf("Error getting GCM block cipher object, error %v", err))
	} else {

		// Decrypt the message
		if receivedDecryptedMessage, err = gcmCipher.Open(nil, nonce, data, nil); err != nil {
			return nil, errors.New(fmt.Sprintf("Error decrypting message, error %v", err))
		}
	}
	return receivedDecryptedMessage, nil

}

// Get the public and private RSA keys being used by this runtime. If the keys dont exist in the
// filesystem, they will be created and written to the filesystem. If they already exist in the
// filesystem then they will be demarshalled and returned to the caller.

var gPublicKey *rsa.PublicKey
var gPrivateKey *rsa.PrivateKey

func HasKeys() bool {
	if gPublicKey != nil {
		return true
	}
	return false
}

var privFileName = "privateMessagingKey.pem"
var pubFileName = "publicMessagingKey.pem"

var KeyLock sync.Mutex

func GetKeys(keyPath string) (*rsa.PublicKey, *rsa.PrivateKey, error) {
	KeyLock.Lock()
	defer KeyLock.Unlock()

	if gPublicKey != nil {
		return gPublicKey, gPrivateKey, nil
	}

	snap_common := os.Getenv("HZN_VAR_BASE")
	if len(snap_common) == 0 {
		snap_common = config.HZN_VAR_BASE_DEFAULT
	}

	privFilepath := path.Join(snap_common, keyPath, privFileName)
	pubFilepath := path.Join(snap_common, keyPath, pubFileName)
	if _, ferr := os.Stat(privFilepath); os.IsNotExist(ferr) {

		if privateKey, err := rsa.GenerateKey(rand.Reader, 2048); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Could not generate private key, error %v", err))
		} else if privFile, err := os.Create(privFilepath); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Could not create private key file %v, error %v", privFilepath, err))
		} else if err := privFile.Chmod(0600); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Could not chmod private key file %v, error %v", privFilepath, err))
		} else if pubFile, err := os.Create(pubFilepath); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Could not create public key file %v, error %v", pubFilepath, err))
		} else if err := pubFile.Chmod(0600); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Could not chmod public key file %v, error %v", pubFilepath, err))
		} else {
			publicKey := &privateKey.PublicKey

			if pubKeyBytes, err := x509.MarshalPKIXPublicKey(publicKey); err != nil {
				return nil, nil, errors.New(fmt.Sprintf("Could not marshal public key, error %v", err))
			} else {
				pubEnc := &pem.Block{
					Type:    "PUBLIC KEY",
					Headers: nil,
					Bytes:   pubKeyBytes}
				if err := pem.Encode(pubFile, pubEnc); err != nil {
					return nil, nil, errors.New(fmt.Sprintf("Could not encode public key to file, error %v", err))
				} else if err := pubFile.Close(); err != nil {
					return nil, nil, errors.New(fmt.Sprintf("Could not close public key file %v, error %v", pubFilepath, err))
				}
			}

			privEnc := &pem.Block{
				Type:    "RSA PRIVATE KEY",
				Headers: nil,
				Bytes:   x509.MarshalPKCS1PrivateKey(privateKey)}
			if err := pem.Encode(privFile, privEnc); err != nil {
				return nil, nil, errors.New(fmt.Sprintf("Could not encode private key to file, error %v", err))
			} else if err := privFile.Close(); err != nil {
				return nil, nil, errors.New(fmt.Sprintf("Could not close private key file %v, error %v", privFilepath, err))
			}

			gPublicKey = publicKey
			gPrivateKey = privateKey
		}
	} else {
		if _, ferr := os.Stat(pubFilepath); os.IsNotExist(ferr) {
			return nil, nil, errors.New(fmt.Sprintf("Could not find public key file %v, error %v", privFilepath, ferr))
		} else if privFile, err := os.Open(privFilepath); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to open private key file %v, error: %v", privFilepath, err))
		} else if privBytes, err := ioutil.ReadAll(privFile); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to read private key file %v, error: %v", privFilepath, err))
		} else if privBlock, _ := pem.Decode(privBytes); privBlock == nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to extract pem block from private key file %v, error: %v", privFilepath, err))
		} else if privateKey, err := x509.ParsePKCS1PrivateKey(privBlock.Bytes); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to parse private key %x, error: %v", privBytes, err))
		} else if pubFile, err := os.Open(pubFilepath); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to open public key file %v, error: %v", pubFilepath, err))
		} else if pubBytes, err := ioutil.ReadAll(pubFile); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to read public key file %v, error: %v", pubFilepath, err))
		} else if pubBlock, _ := pem.Decode(pubBytes); pubBlock == nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to extract pem block from public key file %v, error: %v", pubFilepath, err))
		} else if publicKey, err := x509.ParsePKIXPublicKey(pubBlock.Bytes); err != nil {
			return nil, nil, errors.New(fmt.Sprintf("Unable to parse public key %x, error: %v", pubBytes, err))
		} else {
			gPublicKey = publicKey.(*rsa.PublicKey)
			gPrivateKey = privateKey
		}
	}

	return gPublicKey, gPrivateKey, nil
}

func DeleteKeys(keyPath string) error {
	// Construct the full file path name
	snap_common := os.Getenv("HZN_VAR_BASE")
	if len(snap_common) == 0 {
		snap_common = config.HZN_VAR_BASE_DEFAULT
	}

	privFilepath := path.Join(snap_common, keyPath, privFileName)
	pubFilepath := path.Join(snap_common, keyPath, pubFileName)

	glog.V(5).Infof("Removing private key path %v, and public key path %v", privFilepath, pubFilepath)

	// Delete both the private and public key files
	if _, ferr := os.Stat(privFilepath); !os.IsNotExist(ferr) {
		if err := os.Remove(privFilepath); err != nil {
			return err
		}
	}

	if _, ferr := os.Stat(pubFilepath); !os.IsNotExist(ferr) {
		if err := os.Remove(pubFilepath); err != nil {
			return err
		}
	}

	return nil
}

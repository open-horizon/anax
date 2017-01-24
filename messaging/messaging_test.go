package messaging

import (
    "bytes"
    "crypto"
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "encoding/json"
    "fmt"
    "golang.org/x/crypto/sha3"
    "testing"
)

func TestEncryptedMessagingExample(t *testing.T) {

    testLoops := 10

    // Overview: Sending a message from consumer to producer
    // 1. create hash of the original message
    // 2. digitally sign the hash
    // 3. add the signers public key 
    // 4. symmetrically encrypt the whole thing
    // 5. encrypt the symmetric encryption key and nonce using the public key of the receiver
    // 6. send the output of 4 and 5 to the recipient

    // The message to be sent. Messages will all be similar to it is important that repeated
    // encryption of the same original message will produce drastically different outputs.
    message := []byte(`{"type":"proposal","protocol":"citizen scientist","version":1}`)
    fmt.Printf("Original Message : %s\n", message)

    // Generate RSA key pair for producer and consumer
    var prodPrivateKey *rsa.PrivateKey
    var consumerPrivateKey *rsa.PrivateKey
    err := error(nil)

    if prodPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
        t.Errorf("Could not generate producer private key, error %v\n", err)
    }
    prodPublicKey := &prodPrivateKey.PublicKey

    if consumerPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048); err != nil {
        t.Errorf("Could not generate consumer private key, error %v\n", err)
    }
    consumerPublicKey := &consumerPrivateKey.PublicKey


    // Prepare for looping
    allSigs := make(map[string]bool)
    allMsgEnc := make(map[string]bool)

    for i := 0; i < testLoops; i++ {

        // 1. create a sha3 hash of the original message, called the message digest
        digest := sha3.Sum256(message)
        fmt.Printf("Digest : %x\n", digest)

        // 2. Sign the hash (digest)
        var signature []byte
        if signature, err = rsa.SignPSS(rand.Reader, consumerPrivateKey, crypto.SHA3_256, digest[:], &rsa.PSSOptions{SaltLength:rsa.PSSSaltLengthAuto}); err != nil {
            t.Errorf("Error signing the message, error %v", err)
        } else {
            fmt.Printf("Signature of digest : %x\n", signature)
        }

        // testcase - track signatures to make sure we don't get any duplicates
        if _, exists := allSigs[string(signature)]; exists {
            t.Errorf("Error, duplicate signature created: %v", allSigs)
        } else {
            allSigs[string(signature)] = true
        }

        // 3. Add signer's public key to the message
        type WrappedMessage struct {
            Msg       []byte `json:"msg"`
            Signature []byte `json:"signature"`
            PubKey    []byte `json:"pubkey"`
        }

        var pubKey []byte
        if pubKey, err = x509.MarshalPKIXPublicKey(consumerPublicKey); err != nil {
            t.Errorf("Error marshalling consumer public key, error %v", err)
        }
        fmt.Printf("Consumer Public Key : %x\n", pubKey)

        wrappedMessage := &WrappedMessage{
            Msg:       message,
            Signature: signature,
            PubKey:    pubKey,
        }

        // 4. Symmetrically encrypt the whole thing

        var wmBytes []byte
        if wmBytes, err = json.Marshal(wrappedMessage); err != nil {
            t.Errorf("Error marshalling wrapped message, error %v", err)
        }
        fmt.Printf("Wrapped Message : %s\n", wmBytes)

        // Generate 1 time use symmetric key for encryption of the message
        symmetricKey := make([]byte, 32)     // 256 bit symmetric key
        if _, err := rand.Read(symmetricKey); err != nil {
            t.Errorf("Error getting random symmetric key, %v\n", err)
        }
        fmt.Printf("Symmetric key : %x\n", symmetricKey)

        // Generate 1 time use nonce (that's redundant) for GCM cipher
        nonce := make([]byte, 12)     // 96 bit number
        if _, err := rand.Read(nonce); err != nil {
            t.Errorf("Error getting random nonce, %v\n", err)
        }
        fmt.Printf("Nonce : %x\n", nonce)

        var encryptedMessage []byte
        // Create a GCM block cipher object and then encrypt the message
        if blockCipher, err := aes.NewCipher(symmetricKey); err != nil {
            t.Errorf("Error getting AES block cipher object, %v\n", err)
        } else  if gcmCipher, err := cipher.NewGCM(blockCipher); err != nil {
            t.Errorf("Error getting GCM block cipher object, %v\n", err)
        } else {
            // Encrypt the message. We are not using the additional data feature of the GCM
            // algorithm becuse we dont think we need it. This is because we are wrapping this whole
            // symmetrically encrypted message inside a public key encryption that also includes a digital
            // signature.
            fmt.Printf("Nonce size : %v\n", gcmCipher.NonceSize())
            encryptedMessage = gcmCipher.Seal(nil, nonce, wmBytes, nil)
            fmt.Printf("Encrypted Wrapped message : %x\n", encryptedMessage)

        }

        // testcase - track encryption to make sure we don't get any duplicates
        if _, exists := allMsgEnc[string(encryptedMessage)]; exists {
            t.Errorf("Error, duplicate encrypted message: %v", allMsgEnc)
        } else {
            allMsgEnc[string(encryptedMessage)] = true
        }

        // Wrapped message is encrypted with symmetric encryption at this point

        // 5. encrypt the symmetric encryption key and nonce using the public key of the receiver

        var encryptedSymmetricValues []byte

        type SymmetricValues struct {
            Key    []byte `json:"key"`
            Nonce  []byte `json:"nonce"`
        }

        symmetricValues := &SymmetricValues{
            Key:    symmetricKey,
            Nonce:  nonce,
        }

        var svBytes []byte
        if svBytes, err = json.Marshal(symmetricValues); err != nil {
            t.Errorf("Error marshalling symmetric values, error %v", err)
        }
        fmt.Printf("Symmetric Values : %s\n", svBytes)

        // What's the purpose of the label?
        label := []byte("")
        //hash := sha256.New()

        if encryptedSymmetricValues, err = rsa.EncryptOAEP(sha3.New256(), rand.Reader, prodPublicKey, svBytes, label); err != nil {
            t.Errorf("Error encrypting message, error %v", err)
        } else {
            fmt.Printf("Encrypted Symmetric Values : %x\n", encryptedSymmetricValues)
        }

        // 6. send the output of 4 and 5 to the recipient

        type HorizonMessage struct {
            Part1  []byte `json:"part1"`
            Part2  []byte `json:"part2"`
        }

        horizonMessage := &HorizonMessage{
            Part1: encryptedMessage,
            Part2: encryptedSymmetricValues,
        }

        var msgBody []byte
        if msgBody, err = json.Marshal(horizonMessage); err != nil {
            t.Errorf("Error marshalling symmetric values, error %v", err)
        } else {
            fmt.Printf("\nSend this Message Body to the producer : %s\n\n", msgBody)
        }

        // Overview: Producer receiving a message from a consumer
        // 1. receive the message parts
        // 2. decrypt the symmetric values from part2 using the receiver's private key
        // 3. use the symmetric key and nonce to decrypt the message in part 1
        // 4. verify the signature of the hash of the message
        // 5. extract the plain text message

        // 1. receive the message parts
        receivedMsgBody := msgBody

        hm := new(HorizonMessage)
        if err = json.Unmarshal(receivedMsgBody, &hm); err != nil {
            t.Errorf("Error unmarshalling message body, error %v", err)
        }

        fmt.Printf("Message Part    : %x\n", hm.Part1)
        fmt.Printf("Sym Values Part : %x\n", hm.Part2)

        if bytes.Compare(hm.Part1, encryptedMessage) != 0 {
            t.Errorf("Received message part1 %x is not the same as the encrypted message %x.", hm.Part1, encryptedMessage)
        } else if bytes.Compare(hm.Part2, encryptedSymmetricValues) != 0 {
            t.Errorf("Received message part2 %x is not the same as the encrypted symmetric values %x.", hm.Part1, encryptedMessage)
        }

        // 2. decrypt the symmetric values from part2 using the receiver's private key

        // Decrypt symmetric values
        var receivedSymValues []byte
        if receivedSymValues, err = rsa.DecryptOAEP(sha3.New256(), rand.Reader, prodPrivateKey, hm.Part2, label); err != nil {
            t.Errorf("Error decrypting Symmatric values from message, error %v\n", err)
        } else {
            fmt.Printf("Decrypted Symmetric Values %x\n", receivedSymValues)
        }

        sv := new(SymmetricValues)
        if err = json.Unmarshal(receivedSymValues, &sv); err != nil {
            t.Errorf("Error unmarshalling symmetric values, error %v", err)
        }

        fmt.Printf("Decrypted Symmetric Key : %x\n", sv.Key)
        fmt.Printf("Decrypted Nonce         : %x\n", sv.Nonce)

        if bytes.Compare(sv.Key, symmetricKey) != 0 {
            t.Errorf("Received message symmetric key %x is not the same as the original symmetric key %x.", sv.Key, symmetricKey)
        } else if bytes.Compare(sv.Nonce, nonce) != 0 {
            t.Errorf("Received message nonce %x is not the same as the original nonce %x.", sv.Nonce, nonce)
        }

        // 3. use the symmetric key and nonce to decrypt the message in part 1

        var receivedDecryptedMessage [] byte

        // Create a GCM block cipher object and then encrypt the message
        if blockCipher, err := aes.NewCipher(sv.Key); err != nil {
            t.Errorf("Error getting AES block cipher object, %v\n", err)
        } else  if gcmCipher, err := cipher.NewGCM(blockCipher); err != nil {
            t.Errorf("Error getting GCM block cipher object, %v\n", err)
        } else {

            // Decrypt the message
            if receivedDecryptedMessage, err = gcmCipher.Open(nil, sv.Nonce, hm.Part1, nil); err != nil {
                t.Errorf("Error decrypting message, %v\n", err)
            } else {
                fmt.Printf("Decrypted Part1 : %s\n", receivedDecryptedMessage)
            }
        }

        wm := new(WrappedMessage)
        if err = json.Unmarshal(receivedDecryptedMessage, &wm); err != nil {
            t.Errorf("Error unmarshalling wrapped message, error %v", err)
        }

        fmt.Printf("Decrypted Wrapped Signature  : %x\n", wm.Signature)
        fmt.Printf("Decrypted Wrapped Public Key : %x\n", wm.PubKey)

        // 4. verify the signature of the hash of the message

        var receivedKey interface{}
        var receivedPubKey *rsa.PublicKey
        if receivedKey, err = x509.ParsePKIXPublicKey(wm.PubKey); err != nil {
            t.Errorf("Error demarshalling consumer public key, error %v", err)
        } else {
            switch receivedKey.(type) {
            case *rsa.PublicKey:
                receivedPubKey = receivedKey.(*rsa.PublicKey)
            default:
                t.Errorf("Error demarshalling consumer public key, returned type %T is not *rsa.PublicKey", receivedKey)
            }
        }

        //Verify Signature
        receivedDigest := sha3.Sum256(wm.Msg)
        fmt.Printf("Digest : %x\n", receivedDigest)

        if err = rsa.VerifyPSS(receivedPubKey, crypto.SHA3_256, receivedDigest[:], wm.Signature, &rsa.PSSOptions{SaltLength:rsa.PSSSaltLengthAuto}); err != nil {
            t.Errorf("Error verifying signature, error %v\n", err)
        } else {
            fmt.Println("Verify Signature successful")
        }

        // 5. extract the plain text message
        fmt.Printf("Decrypted and Verified Original Message    : %s\n", wm.Msg)
        if bytes.Compare(message, wm.Msg) != 0 {
            t.Errorf("Received message %x is not the same as the original message %x.", wm.Msg, message)
        }
    }

    if len(allSigs) != testLoops {
        t.Errorf("test did not create 10 unique signatures of the digest: %v", allSigs)
    }

    if len(allMsgEnc) != testLoops {
        t.Errorf("test did not create 10 unique message encryptions: %v", allMsgEnc)
    }


}
package metering

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/golang/glog"
	"github.com/open-horizon/anax/persistence"
	"github.com/open-horizon/anax/policy"
	"golang.org/x/crypto/sha3"
	"time"
)

// The purpose of this module is to encapsulate the metering notification object as much as possible. This
// object is sent as the body of metering notification messages from the consumer to the producer. The Producer
// can use this object to form a blockchain transaction that will permanently record the meter notification
// on the blockchain.

type MeteringNotification struct {
	Amount                 uint64 `json:"amount"`       // The number of tokens granted by this notification, rounded to the nearest minute
	StartTime              uint64 `json:"start_time"`   // The time when the agreement started, in seconds since 1970.
	CurrentTime            uint64 `json:"current_time"` // The time when the notification was sent, in seconds since 1970.
	MissedTime             uint64 `json:"missed_time"`  // The amount of time in seconds that the consumer detected missing data
	AgreementId            string `json:"agreement_id"` // Hex encoded string of a 32 byte number indicating the agreement that this notification is related to.
	meterHash              string // Not serialized in the message, just saved for efficiency
	ConsumerMeterSignature string `json:"consumer_meter_signature"`     // The consumer's signature of the meter (amount, current time, agreement Id)
	AgreementHash          string `json:"agreement_hash"`               // The 32 byte SHA3 FIPS 202 hash of the proposal for the agreement.
	ConsumerSignature      string `json:"consumer_agreement_signature"` // The consumer's signature of the agreement hash.
	ConsumerAddress        string `json:"consumer_address"`             // The consumer's blockchain account/address.
	ProducerSignature      string `json:"producer_agreement_signature"` // The producer's signature of the agreement
	BlockchainType         string `json:"blockchain_type"`              // The type of the blockchain that this notification is intended to work with
}

func (m MeteringNotification) String() string {
	return fmt.Sprintf("Amount: %v, "+
		"StartTime: %v, "+
		"CurrentTime: %v, "+
		"Missed Time: %v, "+
		"AgreementId: %v, "+
		"Meter Hash: %v, "+
		"ConsumerMeterSignature: %v, "+
		"AgreementHash: %v, "+
		"ConsumerSignature: %v, "+
		"ConsumerAddress: %v, "+
		"ProducerSignature: %v, "+
		"BlockchainType: %v",
		m.Amount, m.StartTime, m.CurrentTime, m.MissedTime, m.AgreementId, m.meterHash, m.ConsumerMeterSignature,
		m.AgreementHash, m.ConsumerSignature, m.ConsumerAddress, m.ProducerSignature,
		m.BlockchainType)
}

func (m MeteringNotification) IsValid() (bool, error) {
	if m.StartTime == 0 {
		return false, errors.New(fmt.Sprintf("start time must be non-zero"))
	} else if m.CurrentTime == 0 {
		return false, errors.New(fmt.Sprintf("current time must be non-zero"))
	} else if len(m.AgreementId) == 0 {
		return false, errors.New(fmt.Sprintf("agreement id is empty"))
	} else if len(m.ConsumerMeterSignature) == 0 {
		return false, errors.New(fmt.Sprintf("consumer signature of meter is empty"))
	} else if len(m.AgreementHash) == 0 {
		return false, errors.New(fmt.Sprintf("agreement hash is empty"))
	} else if len(m.ConsumerSignature) == 0 {
		return false, errors.New(fmt.Sprintf("consumer signature of agreement is empty"))
	} else if len(m.ConsumerAddress) == 0 {
		return false, errors.New(fmt.Sprintf("consumer address is empty"))
	} else if len(m.ProducerSignature) == 0 {
		return false, errors.New(fmt.Sprintf("producer signature of agreement is empty"))
	} else if len(m.BlockchainType) == 0 {
		return false, errors.New(fmt.Sprintf("block chain type is empty"))
	}
	return true, nil

}

// This function creates a Metering notification object. Everything but the meter signature is created at this time.
// Once this object is created, the meter hash can be signed and set into the object.
func NewMeteringNotification(meterPolicy policy.Meter, startTime uint64, checkRate uint64, missedChecks uint64, agId string, agHash string, cSig string, cAddr string, pSig string, bcType string) (*MeteringNotification, error) {

	m := new(MeteringNotification)
	m.CurrentTime = uint64(time.Now().Unix())
	m.StartTime = startTime
	m.AgreementId = agId
	m.AgreementHash = agHash
	m.ConsumerSignature = cSig
	m.ConsumerAddress = cAddr
	m.ProducerSignature = pSig
	m.BlockchainType = bcType
	if err := m.calculateAmount(meterPolicy, startTime, checkRate, missedChecks); err != nil {
		return nil, err
	}
	m.meterHash = m.GetMeterHash()

	glog.V(5).Infof("Created Metering Notification: %v", m)

	return m, nil

}

func (m *MeteringNotification) SetConsumerMeterSignature(sig string) {
	m.ConsumerMeterSignature = sig
}

// The persistent form of the Metering notification is nearly identical to the wire form. In fact,
// it is a subset of the wire form, so it should be possible to just unmarshal the wire form
// directly to the persistent form.
func ConvertToPersistent(mnString string) (*persistence.MeteringNotification, error) {
	m := new(persistence.MeteringNotification)
	if err := json.Unmarshal([]byte(mnString), m); err != nil {
		return nil, err
	}
	return m, nil
}

// The persistent form of the metering notification is a subset of the wire form, so the conversion
// to wire form only requires the addition of a few fields.
func ConvertFromPersistent(mn persistence.MeteringNotification, agId string) *MeteringNotification {
	m := new(MeteringNotification)
	m.Amount = mn.Amount
	m.StartTime = mn.StartTime
	m.CurrentTime = mn.CurrentTime
	m.MissedTime = mn.MissedTime
	m.AgreementId = agId
	m.ConsumerMeterSignature = mn.ConsumerMeterSignature
	m.AgreementHash = mn.AgreementHash
	m.ConsumerSignature = mn.ConsumerSignature
	m.ConsumerAddress = mn.ConsumerAddress
	m.ProducerSignature = mn.ProducerSignature
	m.BlockchainType = mn.BlockchainType
	return m
}

// This function returns the stringified hash of the meter notification. This is the thing that
// the consumer signs to produce the consumer meter signature.
func (m *MeteringNotification) GetMeterHash() string {

	if len(m.meterHash) != 0 {
		return m.meterHash
	} else {
		binaryAgreementId, _ := hex.DecodeString(m.AgreementId)

		theMeter := make([]byte, 0, 96)
		theMeter = append(theMeter, toBuffer(m.Amount)...)
		theMeter = append(theMeter, toBuffer(m.CurrentTime)...)
		theMeter = append(theMeter, binaryAgreementId...)

		hash := sha3.Sum256(theMeter)
		return "0x" + hex.EncodeToString(hash[:])
	}
}

// This is an internal function used to calculate the amount of tokens to send in the notification message. The
// results of the calculation are set directly onto the object.
//
// The amount is the full amount earned since the agreement started.
func (m *MeteringNotification) calculateAmount(meterPolicy policy.Meter, startTime uint64, checkRate uint64, missedChecks uint64) error {

	normalize := map[string]float64{"min": 1.0, "hour": 60.0, "day": 1440.0}

	// Find the number of mins per check for data
	minPerCheck := float64(checkRate) / 60.0

	// Find the amount of missed time
	missedDataMin := float64(missedChecks) * minPerCheck

	// Find the number of mins since the agreement was made minus the missed data time
	agreementDurationMins := (float64(m.CurrentTime-startTime) / 60.0) - missedDataMin

	// Find the payment rate per minute
	paymentPerMin := float64(meterPolicy.Tokens) / normalize[meterPolicy.PerTimeUnit]

	// Find the total number of tokens to be given
	m.Amount = uint64(agreementDurationMins * paymentPerMin)
	m.MissedTime = uint64(missedDataMin * 60)

	return nil

}

func toBuffer(seq uint64) []byte {
	buf := make([]byte, 32)
	for i := len(buf) - 1; seq != 0; i-- {
		buf[i] = byte(seq & 0xff)
		seq >>= 8
	}
	return buf
}

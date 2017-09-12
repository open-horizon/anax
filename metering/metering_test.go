// +build unit

package metering

import (
	"fmt"
	"github.com/open-horizon/anax/policy"
	"testing"
	"time"
)

// AgreementProtocolList Tests
// First, some tests where the lists intersect
func Test_IsValid(t *testing.T) {

	m1 := MeteringNotification{
		Amount: 0, StartTime: 1, CurrentTime: 1, AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "",
	}

	if ok, _ := m1.IsValid(); ok {
		t.Errorf("Metering Notification %v is not valid.", m1)
	}

	m2 := MeteringNotification{
		Amount: 1, StartTime: 1, CurrentTime: 1, AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}

	if ok, _ := m2.IsValid(); !ok {
		t.Errorf("Metering Notification %v is valid.", m2)
	}

}

func Test_GetMeterHash(t *testing.T) {

	m1 := MeteringNotification{
		Amount: 6, StartTime: 1, CurrentTime: 1, AgreementId: "00000000000000000000000000000000", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "",
	}
	if hash := m1.GetMeterHash(); hash != "0x6b431419f352bf9646f469f0afe927cfe0df5b224a0e508c5426b168b2b755f9" {
		t.Errorf("Metering Notification hash %v is incorrect, should be %v.", hash, "0x6b431419f352bf9646f469f0afe927cfe0df5b224a0e508c5426b168b2b755f9")
	}

}

func Test_newMeteringNotification(t *testing.T) {

	m := policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	now := uint64(time.Now().Unix())
	past := now - 300
	if mn, err := NewMeteringNotification(m, past, 0, 0, "00000000000000000000000000000000", "agHash", "cSig", "cAddr", "pSig", "ethereum"); err != nil {
		t.Errorf("error creating new metering notification: %v", err)
	} else if mn.Amount != 15 {
		t.Errorf("should have calculated 15, but calculated %v", mn.Amount)
	} else if mn.MissedTime != 0 {
		t.Errorf("should have calculated 0, missed time but calculated %v", mn.MissedTime)
	} else if mn, err := NewMeteringNotification(m, past, 15, 4, "agId", "agHash", "cSig", "cAddr", "pSig", "ethereum"); err != nil {
		t.Errorf("error creating new metering notification: %v", err)
	} else if mn.Amount != 12 {
		t.Errorf("should have calculated 12, but calculated %v", mn.Amount)
	} else if mn.MissedTime != 60 {
		t.Errorf("should have calculated 60, missed time but calculated %v", mn.MissedTime)
	} else if mn, err := NewMeteringNotification(m, past, 15, 3, "0000000000000000000000000000000000000000000000000000000000000000", "agHash", "cSig", "cAddr", "pSig", "ethereum"); err != nil {
		t.Errorf("error creating new metering notification: %v", err)
	} else if mn.Amount != 12 {
		t.Errorf("should have calculated 12, but calculated %v", mn.Amount)
	} else if mn.MissedTime != 45 {
		t.Errorf("should have calculated 45, missed time but calculated %v", mn.MissedTime)
	}

}

func Test_calcAmount(t *testing.T) {

	// start with an easy test
	m := policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past := uint64(time.Now().Unix()) - 300
	m1 := MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans := uint64(15)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// less than 1 token in mins
	m = policy.Meter{Tokens: 2, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (15)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(0)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// less than 1 token in hours
	m = policy.Meter{Tokens: 2, PerTimeUnit: "hour", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 25)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(0)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// less than 1 token in days
	m = policy.Meter{Tokens: 2, PerTimeUnit: "day", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 1)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(0)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 1 token in mins
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (25)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(1)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 1 token in hours
	m = policy.Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 25)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(1)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 1 token in days
	m = policy.Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 9)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(1)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 2 token in mins
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (45)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(2)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 2 token in hours
	m = policy.Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 45)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(2)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// 2 token in days
	m = policy.Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 17)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(2)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// many tokens in mins
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 4) - 25
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(13)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// many tokens in hours
	m = policy.Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 3) - (60 * 25)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(10)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// many tokens in days
	m = policy.Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 24) - (60 * 60 * 9)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(4)
	if err := m1.calculateAmount(m, past, 0, 0); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

}

func Test_calcAmount_withmisses(t *testing.T) {

	// miss 1 min of data
	m := policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past := uint64(time.Now().Unix()) - 300
	m1 := MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans := uint64(12)
	if err := m1.calculateAmount(m, past, 15, 4); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// miss 30 secs of data
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - 300
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(13)
	if err := m1.calculateAmount(m, past, 15, 2); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// miss 2:30 secs of data
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - 300
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(7)
	if err := m1.calculateAmount(m, past, 15, 10); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// miss 2:30 secs of data
	m = policy.Meter{Tokens: 3, PerTimeUnit: "hour", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(2)
	if err := m1.calculateAmount(m, past, 15, 10); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// miss 2:30 secs of data
	m = policy.Meter{Tokens: 3, PerTimeUnit: "day", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - (60 * 60 * 24)
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(2)
	if err := m1.calculateAmount(m, past, 15, 10); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

	// miss 2:30 secs of data
	m = policy.Meter{Tokens: 3, PerTimeUnit: "min", NotificationIntervalS: 300}
	past = uint64(time.Now().Unix()) - 300
	m1 = MeteringNotification{
		Amount: 0, StartTime: past, CurrentTime: uint64(time.Now().Unix()), AgreementId: "abc", ConsumerMeterSignature: "1234",
		AgreementHash: "abcdef", ConsumerSignature: "fdebca", ConsumerAddress: "1234", ProducerSignature: "bcdefa",
		BlockchainType: "ethereum",
	}
	ans = uint64(8)
	if err := m1.calculateAmount(m, past, 25, 5); err != nil {
		t.Errorf(err.Error())
	} else if m1.Amount != ans {
		fmt.Println(m1)
		t.Errorf("Token calculation was incorrect, calculated %v should have been %v", m1.Amount, ans)
	}

}

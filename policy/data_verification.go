package policy

import (
	"errors"
	"fmt"
)

type Meter struct {
	Tokens                uint64 `json:"tokens,omitempty"`                // The number of tokens per time_unit
	PerTimeUnit           string `json:"per_time_unit,omitempty"`         // The per time units: min, hour and day are supported
	NotificationIntervalS int    `json:"notification_interval,omitempty"` // The number of seconds between metering notifications
}

func (m Meter) String() string {
	return fmt.Sprintf("Tokens: %v, PerTimeUnits: %v, Notification Interval: %v", m.Tokens, m.PerTimeUnit, m.NotificationIntervalS)
}

func (m Meter) IsValid() bool {
	// Token and PerTimeUnit must be specified together
	if (m.Tokens != 0 && m.PerTimeUnit == "") || (m.Tokens == 0 && m.PerTimeUnit != "") {
		return false

		// Notification interval requires both Token and PerTimeUnit to be specified
	} else if m.NotificationIntervalS != 0 && (m.Tokens == 0 || m.PerTimeUnit == "") {
		return false

		// PerTimeUnit must be a valid value
	} else if m.PerTimeUnit != "min" && m.PerTimeUnit != "hour" && m.PerTimeUnit != "day" && m.PerTimeUnit != "" {
		return false
	}

	return true
}

func (m Meter) IsEmpty() bool {
	if m.Tokens == 0 && m.PerTimeUnit == "" && m.NotificationIntervalS == 0 {
		return true
	}
	return false
}

func (m Meter) IsSame(otherMeter Meter) bool {
	return m.Tokens == otherMeter.Tokens && m.PerTimeUnit == otherMeter.PerTimeUnit && m.NotificationIntervalS == otherMeter.NotificationIntervalS
}

// Producer requirement satisfied by consumer offer. Both policies are assumed to be valid.
func (m Meter) IsSatisfiedBy(otherMeter Meter) bool {

	// When this meter is empty, it is always compatible with the other policy. Only when
	// both policies are not empty do we have to do an in-depth compatibility check.

	if m.IsEmpty() {
		return true
	} else {
		if otherMeter.IsEmpty() {
			return true
		} else {
			// Need to perform a compatibility check now

			// Normalize the token values to a per day total
			reqTokens := normalizeTokens(m.Tokens, m.PerTimeUnit)
			offerTokens := normalizeTokens(otherMeter.Tokens, otherMeter.PerTimeUnit)

			// If the offer doesnt satisfy the requirements, then return not satisfied
			if offerTokens < reqTokens {
				return false
			}

		}
	}

	return true
}

// This function returns the normalized meter policy to be used in the proposal. Both policies are
// assumed to be compatible.
func (m Meter) MergeWith(otherMeter Meter, dvCheckRate int) Meter {
	ret := Meter{}

	if m.IsEmpty() && otherMeter.IsEmpty() {
		return ret
	}

	// Pick the time unit
	chosenTimeUnit := "min"
	if m.PerTimeUnit == "day" || otherMeter.PerTimeUnit == "day" {
		chosenTimeUnit = "day"
	} else if m.PerTimeUnit == "hour" || otherMeter.PerTimeUnit == "hour" {
		chosenTimeUnit = "hour"
	}
	ret.PerTimeUnit = chosenTimeUnit

	// Normalize the token values to a per day total. Assume that the two policies
	// are already compatible.
	if !otherMeter.IsEmpty() {
		ret.Tokens = normalizeTokens(otherMeter.Tokens, otherMeter.PerTimeUnit)
	} else {
		ret.Tokens = normalizeTokens(m.Tokens, m.PerTimeUnit)
	}

	// Convert tokens back to the chosen time unit.
	divisors := map[string]uint64{"min": 1440, "hour": 24, "day": 1}
	ret.Tokens = ret.Tokens / divisors[chosenTimeUnit]

	// Choose the notification interval, shorter of both or the default
	min := minOf(m.NotificationIntervalS, otherMeter.NotificationIntervalS)
	max := maxOf(m.NotificationIntervalS, otherMeter.NotificationIntervalS)

	if min == 0 && max == 0 {
		// choose default based on other policy
		if dvCheckRate == 0 {
			ret.NotificationIntervalS = 10
		} else if dvCheckRate <= 10 {
			ret.NotificationIntervalS = 10
		} else {
			ret.NotificationIntervalS = dvCheckRate
		}

	} else if min != 0 && max != 0 {
		ret.NotificationIntervalS = min
	} else if min == 0 {
		ret.NotificationIntervalS = max
	}

	return ret
}

// Merge 2 producer meter policies. This function does not do the same thing as the MergeWith function.
func (m *Meter) ProducerMergeWith(otherMeter *Meter, dvCheckRate int) Meter {
	ret := Meter{}

	if m.IsEmpty() && otherMeter.IsEmpty() {
		return ret
	}

	// Pick the time unit
	chosenTimeUnit := "min"
	if m.PerTimeUnit == "day" || otherMeter.PerTimeUnit == "day" {
		chosenTimeUnit = "day"
	} else if m.PerTimeUnit == "hour" || otherMeter.PerTimeUnit == "hour" {
		chosenTimeUnit = "hour"
	}
	ret.PerTimeUnit = chosenTimeUnit

	// Normalize the token values to a per day total.
	var myTokens, otherTokens uint64
	if !m.IsEmpty() {
		myTokens = normalizeTokens(m.Tokens, m.PerTimeUnit)
	}
	if !otherMeter.IsEmpty() {
		otherTokens = normalizeTokens(otherMeter.Tokens, otherMeter.PerTimeUnit)
	}

	// Choose the greater of the 2 token amounts
	ret.Tokens = maxOfUint64(myTokens, otherTokens)

	// Convert tokens back to the chosen time unit.
	divisors := map[string]uint64{"min": 1440, "hour": 24, "day": 1}
	ret.Tokens = ret.Tokens / divisors[chosenTimeUnit]

	// Choose the notification interval, shorter of both or the default
	min := minOf(m.NotificationIntervalS, otherMeter.NotificationIntervalS)
	max := maxOf(m.NotificationIntervalS, otherMeter.NotificationIntervalS)

	if min == 0 && max == 0 {
		// choose default based on other policy
		if dvCheckRate == 0 {
			ret.NotificationIntervalS = 10
		} else if dvCheckRate <= 10 {
			ret.NotificationIntervalS = 10
		} else {
			ret.NotificationIntervalS = dvCheckRate
		}

	} else if min != 0 && max != 0 {
		ret.NotificationIntervalS = min
	} else if min == 0 {
		ret.NotificationIntervalS = max
	}

	return ret
}

// This function determines if 2 producer policies can be reconciled.
func (m Meter) IsCompatibleWith(otherMeter Meter) bool {

	// There are no aspects of Metering policy that cant be reconciled between
	// 2 producers.
	return true

}

// Convert tokens per x time into tokens per day
func normalizeTokens(amt uint64, timeUnit string) uint64 {
	multipliers := map[string]uint64{"min": 1440, "hour": 24, "day": 1}
	return amt * multipliers[timeUnit]
}

func minOf(x int, y int) int {
	if x < y {
		return x
	} else {
		return y
	}
}

func maxOf(x int, y int) int {
	if x > y {
		return x
	} else {
		return y
	}
}

func maxOfUint64(x uint64, y uint64) uint64 {
	if x > y {
		return x
	} else {
		return y
	}
}

type DataVerification struct {
	Enabled     bool   `json:"enabled,omitempty"`     // Whether or not data verification is enabled
	URL         string `json:"URL,omitempty"`         // The URL to be used for data receipt verification
	URLUser     string `json:"URLUser,omitempty"`     // The user id to use when calling the verification URL
	URLPassword string `json:"URLPassword,omitempty"` // The password to use when calling the verification URL
	Interval    int    `json:"interval,omitempty"`    // The number of seconds to check for data before deciding there isnt any data
	CheckRate   int    `json:"check_rate,omitempty"`  // The number of seconds between checks for valid data being received
	Metering    Meter  `json:"metering,omitempty"`    // The metering configuration
}

func DataVerification_Factory(url string, urluser string, urlpw string, interval int, checkRate int, meterPolicy Meter) *DataVerification {
	d := new(DataVerification)
	d.Enabled = true
	d.URL = url
	d.URLUser = urluser
	d.URLPassword = urlpw
	d.Interval = interval
	d.CheckRate = checkRate
	d.Metering = meterPolicy

	return d
}

func (d DataVerification) IsValid() (bool, error) {
	if !d.Metering.IsValid() {
		return false, errors.New(fmt.Sprintf("Metering is not valid"))
	} else if d.Interval != 0 && d.CheckRate != 0 && d.Interval < d.CheckRate {
		return false, errors.New(fmt.Sprintf("Interval is shorter than check rate"))
	}
	return true, nil
}

func (d DataVerification) IsSame(compare DataVerification) bool {
	return d.Enabled == compare.Enabled &&
		d.URL == compare.URL &&
		d.URLUser == compare.URLUser &&
		d.Interval == compare.Interval &&
		d.CheckRate == compare.CheckRate &&
		d.Metering.IsSame(compare.Metering)
}

func (d DataVerification) String() string {
	return fmt.Sprintf("Enabled: %v, URL: %v, URL User: %v, Interval: %v, CheckRate: %v, Metering: %v", d.Enabled, d.URL, d.URLUser, d.Interval, d.CheckRate, d.Metering)
}

func (d *DataVerification) Obscure() {
	if d.URLPassword != "" {
		d.URLPassword = "********"
	}
}

// Two policies are compatible if they dont conflict with each other such that the
// compare policy satisfies the requirements of the d or "self" policy.
func (d DataVerification) IsCompatibleWith(compare DataVerification) bool {
	// If the DV sections are compatible then check the metering section.
	if (&d).internalCompatibleWith(&compare) {
		return d.Metering.IsSatisfiedBy(compare.Metering)
	}
	return false
}

func (d *DataVerification) internalCompatibleWith(compare *DataVerification) bool {
	// single out the case where 2 DV sections are not compatible; both sections are
	// enabled they want to use different URLs and/or Users to verify. That difference
	// cannot be reconciled and therefore the sections are incompatible.
	if (d.Enabled && compare.Enabled && d.URL != "" && compare.URL != "" && d.URL != compare.URL) ||
		(d.Enabled && compare.Enabled && d.URLUser != "" && compare.URLUser != "" && d.URLUser != compare.URLUser) {
		return false
	}
	return true
}

// Two producer policies are compatible with each other if the difference are reconcileable
// as producers.
func (d DataVerification) IsProducerCompatible(compare DataVerification) bool {
	// If the DV sections are compatible then check the metering section.
	if (&d).internalCompatibleWith(&compare) {
		return d.Metering.IsCompatibleWith(compare.Metering)
	}
	return false
}

// Common logic for merging the interval value of 2 DV sections.
func (ret *DataVerification) internalMergeInterval(d *DataVerification, other *DataVerification, configInterval uint64) {

	// Choose the no data interval, shorter of both or the default
	thisInterval := 0
	if d.Enabled {
		thisInterval = d.Interval
	}
	otherInterval := 0
	if other.Enabled {
		otherInterval = other.Interval
	}
	min := minOf(thisInterval, otherInterval)
	max := maxOf(thisInterval, otherInterval)

	if min == 0 && max == 0 {
		if d.Enabled || other.Enabled {
			ret.Interval = int(configInterval)
		} else {
			ret.Interval = 0
		}
	} else if min != 0 && max != 0 {
		ret.Interval = min
	} else if min == 0 {
		ret.Interval = max
	}
}

// Common logic for merging the CheckRate value of 2 DV sections.
func (ret *DataVerification) internalMergeCheckRate(d *DataVerification, other *DataVerification) {

	// Choose the check rate, shorter of both or the default
	thisCheckRate := 0
	if d.Enabled {
		thisCheckRate = d.CheckRate
	}
	otherCheckRate := 0
	if other.Enabled {
		otherCheckRate = other.CheckRate
	}
	min := minOf(thisCheckRate, otherCheckRate)
	max := maxOf(thisCheckRate, otherCheckRate)

	if min == 0 && max == 0 {
		ret.CheckRate = 0
	} else if min != 0 && max != 0 {
		ret.CheckRate = min
	} else if min == 0 {
		ret.CheckRate = max
	}
}

// Merge 2 producer DV sections. This is different from merging sections from a producer and
// consumer because 2 producer sections just have to be reconciled, one does not have to
// satisfy the other as in the consumer/producer relationship. Further this function
// assumes that IsProducerCompatible() has already been called for these 2 sections so they
// are already compatible.
func (d DataVerification) ProducerMergeWith(other DataVerification, configInterval uint64) DataVerification {
	ret := DataVerification{}

	// If one policy is enabled then the merge is enabled
	if d.Enabled || other.Enabled {
		ret.Enabled = true
	}

	// If there is a URL and User in one of the policies, use it. If there is a URL or User
	// in both, they will be the same because a previous compat check is assumed.
	if d.Enabled && d.URL != "" {
		ret.URL = d.URL
	} else if other.Enabled && other.URL != "" {
		ret.URL = other.URL
	}

	if d.Enabled && d.URLUser != "" {
		ret.URLUser = d.URLUser
		ret.URLPassword = d.URLPassword
	} else if other.Enabled && other.URLUser != "" {
		ret.URLUser = other.URLUser
		ret.URLPassword = other.URLPassword
	}

	(&ret).internalMergeInterval(&d, &other, configInterval)

	(&ret).internalMergeCheckRate(&d, &other)

	// Merge the metering policy
	ret.Metering = (&d.Metering).ProducerMergeWith(&other.Metering, ret.CheckRate)

	return ret
}

// Produce a merged data verification section. Both policies are assumed to be valid and compatible.
func (d DataVerification) MergeWith(other DataVerification, configInterval uint64) DataVerification {

	ret := DataVerification{}

	// Obscure password if set
	if other.Enabled && other.URLPassword != "" {
		ret.Obscure()
	}

	// If one policy is enabled then the merge is enabled
	if d.Enabled || other.Enabled {
		ret.Enabled = true
	}

	// If there is a URL and User in one of the policies, use it. If there is a URL or User
	// in both, they will be the same because a previous compat check is assumed.
	if d.Enabled && d.URL != "" {
		ret.URL = d.URL
	} else if other.Enabled && other.URL != "" {
		ret.URL = other.URL
	}

	if d.Enabled && d.URLUser != "" {
		ret.URLUser = d.URLUser
	} else if other.Enabled && other.URLUser != "" {
		ret.URLUser = other.URLUser
	}

	(&ret).internalMergeInterval(&d, &other, configInterval)

	(&ret).internalMergeCheckRate(&d, &other)

	// Merge the metering policy
	if d.Enabled && other.Enabled {
		ret.Metering = d.Metering.MergeWith(other.Metering, ret.CheckRate)
	} else if d.Enabled {
		ret.Metering = d.Metering.MergeWith(Meter{}, ret.CheckRate)
	} else if other.Enabled {
		ret.Metering = ret.Metering.MergeWith(other.Metering, ret.CheckRate)
	} else {
		ret.Metering = ret.Metering.MergeWith(Meter{}, ret.CheckRate)
	}

	return ret
}

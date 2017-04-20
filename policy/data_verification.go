package policy

import (
	"errors"
	"fmt"
)

type Meter struct {
	Tokens                uint64 `json:"tokens"`                // The number of tokens per time_unit
	PerTimeUnit           string `json:"per_time_unit"`         // The per time units: min, hour and day are supported
	NotificationIntervalS int    `json:"notification_interval"` // The number of seconds between metering notifications
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
	divisors := map[string]uint64{"min":1440, "hour":24, "day":1}
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

// Convert tokens per x time into tokens per day
func normalizeTokens(amt uint64, timeUnit string) uint64 {
	multipliers := map[string]uint64{"min":1440, "hour":24, "day":1}
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

type DataVerification struct {
	Enabled     bool   `json:"enabled"`            // Whether or not data verification is enabled
	URL         string `json:"URL"`                // The URL to be used for data receipt verification
	URLUser     string `json:"URLUser"`            // The user id to use when calling the verification URL
	URLPassword string `json:"URLPassword"`        // The password to use when calling the verification URL
	Interval    int    `json:"interval"`           // The number of seconds to check for data before deciding there isnt any data
	CheckRate   int    `json:"check_rate"`         // The number of seconds between checks for valid data being received
	Metering    Meter  `json:"metering,omitempty"` // The metering configuration
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

// Two policies are compatible if they dont conflict with each other
func (d DataVerification) IsCompatibleWith(compare DataVerification) bool {
	if d.IsSame(compare) {
		return true
	} else {
		if (d.Enabled && compare.Enabled && d.URL != "" && compare.URL != "" && d.URL != compare.URL) ||
		   (d.Enabled && compare.Enabled && d.URLUser != "" && compare.URLUser != "" && d.URLUser != compare.URLUser) {
			return false
		} else if d.Enabled && compare.Enabled {
			return d.Metering.IsSatisfiedBy(compare.Metering)
		}
	}
	return true
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

	// Choose the check rate, shorter of both or the default
	thisCheckRate := 0
	if d.Enabled {
		thisCheckRate = d.CheckRate
	}
	otherCheckRate := 0
	if other.Enabled {
		otherCheckRate = other.CheckRate
	}
	min = minOf(thisCheckRate, otherCheckRate)
	max = maxOf(thisCheckRate, otherCheckRate)

	if min == 0 && max == 0 {
		ret.CheckRate = 0
	} else if min != 0 && max != 0 {
		ret.CheckRate = min
	} else if min == 0 {
		ret.CheckRate = max
	}

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

package policy

import (
	"fmt"
)

type DataVerification struct {
	Enabled          bool   `json:"enabled"`          // Whether or not data verification is enabled
	URL              string `json:"URL"`              // The URL to be used for data receipt verification
	URLUser          string `json:"URLUser"`          // The user id to use when calling the verification URL
	URLPassword      string `json:"URLPassword"`      // The password to use when calling the verification URL
	Interval         int    `json:"interval"`         // The number of seconds between data receipt checks
}


// Two sections are identical if the field values are the same.
func (d DataVerification) IsSame(compare DataVerification) bool {
	return d.Enabled == compare.Enabled && d.URL == compare.URL && d.URLUser == compare.URLUser && d.Interval == compare.Interval
}

func (d DataVerification) String() string {
	return fmt.Sprintf("Enabled: %v, URL: %v, URL User: %v, Interval: %v", d.Enabled, d.URL, d.URLUser, d.Interval)
}

func (d *DataVerification) Obscure() {
	if d.URLPassword != "" {
		d.URLPassword = "********"
	}
}

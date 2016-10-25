package ethblockchain

import (
	"bytes"
)

// take a padded 32-byte array and trun it into a string
func byte32string(str string) string {
     byteArr := []byte(str)
     return string(bytes.Trim(byteArr, "\x00"))
}


// exract the value by given key.
// The input is a string array with each element is a byte array padded up to 32 bytes.
func extractAttr(attributes []string, key string) string {

	for ix, bAttr := range attributes {

		// bAttr is a byte array that is padded up to 32 bytes
		attr := byte32string(bAttr)

		if key == attr {
			if len(attributes) > ix+1 {
				return byte32string(attributes[ix+1])
			}
		}
	}

	return ""
}


// extract the key-value pairs from a byte array.
// The input is a string array with each element is a byte array padded up to 32 bytes.
// Each odd element is a key and each even element is a value.
func extractAll(attributes []string) map[string]string {

    m := make(map[string]string)
    var key,val string
    for i := 0; i < len(attributes); i = i+2 {
        key = byte32string(attributes[i])
        if len(attributes) > i+1 {
           val = byte32string(attributes[i+1])
        } else {
           val = ""
        }
        m[key] = val
    }

	return m
}
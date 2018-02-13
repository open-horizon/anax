package config

type ArchSynonyms map[string]string // the key is the arch string and the value is GOARCH representation of the arch which we call canonical arch.

func NewArchSynonyms() ArchSynonyms {
	var a ArchSynonyms
	a = make(map[string]string)
	return a
}

// return the arch GOARCH representaion of the given arch.
// It returns an empty string if the given arch is not defined in the ArchSynonyms attribute of the configuration file.
func (c ArchSynonyms) GetCanonicalArch(arch string) string {
	if arch == "" {
		return ""
	}

	if v, ok := c[arch]; ok {
		return v
	} else {
		return ""
	}
}

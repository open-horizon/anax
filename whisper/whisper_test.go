package whisper

import (
	"encoding/json"
	"net/url"
	gwhisper "github.com/open-horizon/go-whisper"
	"testing"
)

func Test_MarshalWhisperConfigureMsg(t *testing.T) {
	url, _ := url.Parse("http://foo.com")

	msg := gwhisper.NewConfigure("nonce", *url, map[string]string{}, map[string]string{}, "", "", "")

	b, err := json.Marshal(msg)
	if err != nil {
		t.Error(err)
	}

	t.Logf("Raw marshaled configure msg: %v", string(b))
}

func Test_UnmarshalWhisperProviderMsg(t *testing.T) {

	raw := `{"type":"CONFIGURE","configure_nonce":"nonce","torrent_url":{"Scheme":"http","Opaque":"","User":null,"Host":"foo.com","Path":"","RawPath":"","RawQuery":"","Fragment":""},"image_hashes":{},"image_signatures":{},"deployment":"","deployment_signature":"","deployment_user_info":""}`

	var whisperProviderMsg gwhisper.WhisperProviderMsg

	if err := json.Unmarshal([]byte(raw), &whisperProviderMsg); err != nil {
		t.Errorf("Error deserializing whisperprovidermsg. Error: %v", err)
	} else {
		switch whisperProviderMsg.Type {

		case gwhisper.T_CONFIGURE:
			var configure gwhisper.Configure
			configureRaw := []byte(raw)

			if err := json.Unmarshal(configureRaw, &configure); err != nil {
				t.Error("Unmarshal failure")
			}

			if configure.ConfigureNonce != "nonce" {
				t.Errorf("Unable to locate expected nonce value")
			}

		default:
			t.Errorf("Unknown type: %v", whisperProviderMsg.Type)

		}
	}
}

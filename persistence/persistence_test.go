package persistence

import (
	"encoding/json"
	gwhisper "github.com/open-horizon/go-whisper"
	"net/url"
	"testing"
)

// AND between terms
func Test_provider_maps_json_scheme(t *testing.T) {
	nonce := "nonce"

	expected := `{"type":"CONFIGURE","configure_nonce":"` + nonce + `","torrent_url":{"Scheme":"http","Opaque":"","User":null,"Host":"workloads.horizon.engineering","Path":"/torrents/2089b12bf257a0150175a7e633645e252b2d6aad.torrent","RawPath":"","ForceQuery":false,"RawQuery":"","Fragment":""},"image_hashes":{"3383694dc857f73b805cb6a5b7ff8904a221505e.tar.gz":"3383694dc857f73b805cb6a5b7ff8904a221505e"},"image_signatures":{"3383694dc857f73b805cb6a5b7ff8904a221505e.tar.gz":"YVXkeKpuTymyBtsnVracCJgOQerfUX/pHSmC9UrN48lC91pktcJWfwYQl0okB/vHMHc5u1ZJap/+T6yjra9hz4LJm9V1eeKenNVgYFpQRCCHbHQTY4mQzD4e02jXzsRaEUYNX90RDooU3w7fEFjzQOOFUYKi06uPnhlZEA7gh0x/L15jKjZ4egdbiBzP17wgDJ+TkesxVdSV3DxV7Et465kSGajAUVOcrmoNQSLVK41zHUsO+hBOFMLS3BZDYV8IBuYVqdHUZ3BGCl8jNimW4RXJerd07nOamKfZ4MWHmx6JsOWtqD0vWDyVNmxl5kDi/uFyQfG3/wVQVxF+4zTC+73FBMO6ttrkvfXzWPaplL9j2zjWCBVXPJgy8pFeAstsmyJ4gViN9ECjOiF0j6/XQxJDfICWRtd//IKMbF9ZRt3VEfkIYTJnIbHBlZA6PK6zMyxLxtz3vaJhZlM6x+xphjncjnmDiP2Hxex6WfearvmSAWlCdDECSO5ss1ctP6dXAya6+BYvFDgZWqSxVoO/DTTY5and9wOtS6ItvFxr3HUm+Yc6z3b+IhO5gliKan374VcR2M+cu/JRXf8wPVmsdewC0T5kCBLCf/f65ywvBX4XyLoSZXtvsJUlpgOExoEJCGDp5enAd346rfx3aY1yAPWc/s/O8HA2dVr+EQ94+Bc="},"deployment":"{\"rtlsdr\":{\"image\":\"summit.hovitos.engineering/armhf/rtlsdr:volcano\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"command\":\"/start.sh\",\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/armhf/pitcherd:volcano\",\"command\":\"/tmp/pitcherd --conf /tmp/pitcherd-conf.json\",\"environment\":[\"MTN_CATCHERS=https://catcher.bluehorizon.network:443\"],\"restart\":\"always\"},\"apollo\":{\"image\":\"summit.hovitos.engineering/armhf/apollo:volcano\",\"links\":[\"rtlsdr\",\"pitcherd\"],\"command\":\"/start.sh\",\"environment\":[\"CATCHER_HOST=catcher.bluehorizon.network\",\"CATCHER_PORT=443\",\"DUMP1090_HOST=bluehorizon.network\",\"JOBS_HOST=bluehorizon.network\"],\"restart\":\"always\"}}","deployment_signature":"fooo","deployment_user_info":"goo"}`

	url, _ := url.Parse("http://workloads.horizon.engineering/torrents/2089b12bf257a0150175a7e633645e252b2d6aad.torrent")

	hashes := make(map[string]string, 0)
	hashes["3383694dc857f73b805cb6a5b7ff8904a221505e.tar.gz"] = "3383694dc857f73b805cb6a5b7ff8904a221505e"

	signatures := make(map[string]string, 0)
	signatures["3383694dc857f73b805cb6a5b7ff8904a221505e.tar.gz"] = "YVXkeKpuTymyBtsnVracCJgOQerfUX/pHSmC9UrN48lC91pktcJWfwYQl0okB/vHMHc5u1ZJap/+T6yjra9hz4LJm9V1eeKenNVgYFpQRCCHbHQTY4mQzD4e02jXzsRaEUYNX90RDooU3w7fEFjzQOOFUYKi06uPnhlZEA7gh0x/L15jKjZ4egdbiBzP17wgDJ+TkesxVdSV3DxV7Et465kSGajAUVOcrmoNQSLVK41zHUsO+hBOFMLS3BZDYV8IBuYVqdHUZ3BGCl8jNimW4RXJerd07nOamKfZ4MWHmx6JsOWtqD0vWDyVNmxl5kDi/uFyQfG3/wVQVxF+4zTC+73FBMO6ttrkvfXzWPaplL9j2zjWCBVXPJgy8pFeAstsmyJ4gViN9ECjOiF0j6/XQxJDfICWRtd//IKMbF9ZRt3VEfkIYTJnIbHBlZA6PK6zMyxLxtz3vaJhZlM6x+xphjncjnmDiP2Hxex6WfearvmSAWlCdDECSO5ss1ctP6dXAya6+BYvFDgZWqSxVoO/DTTY5and9wOtS6ItvFxr3HUm+Yc6z3b+IhO5gliKan374VcR2M+cu/JRXf8wPVmsdewC0T5kCBLCf/f65ywvBX4XyLoSZXtvsJUlpgOExoEJCGDp5enAd346rfx3aY1yAPWc/s/O8HA2dVr+EQ94+Bc="

	deployment := "{\"rtlsdr\":{\"image\":\"summit.hovitos.engineering/armhf/rtlsdr:volcano\",\"privileged\":true,\"devices\":[\"/dev/bus/usb/001/001:/dev/bus/usb/001/001\"],\"command\":\"/start.sh\",\"restart\":\"always\"},\"pitcherd\":{\"image\":\"summit.hovitos.engineering/armhf/pitcherd:volcano\",\"command\":\"/tmp/pitcherd --conf /tmp/pitcherd-conf.json\",\"environment\":[\"MTN_CATCHERS=https://catcher.bluehorizon.network:443\"],\"restart\":\"always\"},\"apollo\":{\"image\":\"summit.hovitos.engineering/armhf/apollo:volcano\",\"links\":[\"rtlsdr\",\"pitcherd\"],\"command\":\"/start.sh\",\"environment\":[\"CATCHER_HOST=catcher.bluehorizon.network\",\"CATCHER_PORT=443\",\"DUMP1090_HOST=bluehorizon.network\",\"JOBS_HOST=bluehorizon.network\"],\"restart\":\"always\"}}"

	configure := gwhisper.NewConfigure(nonce, *url, hashes, signatures, deployment, "fooo", "goo")

	if serial, err := json.Marshal(configure); err != nil {
		t.Error(err)
	} else {
		if string(serial) != expected {
			t.Errorf("Expected serial %v but got: %v", expected, string(serial))
		}
	}
}

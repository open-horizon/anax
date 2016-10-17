package container

import (
	"encoding/json"
	docker "github.com/fsouza/go-dockerclient"
	"net/url"
	gwhisper "github.com/open-horizon/go-whisper"
	"testing"
)

func Test_UnmarshalNetworkIsolation(t *testing.T) {
	s := `
		{
			"outbound_permit_only": ["foo", "new", {"dd_key": "goo", "encoding": "JSON", "path": "too.zoo"}]
		}
	`

	var n NetworkIsolation

	if err := json.Unmarshal([]byte(s), &n); err != nil {
		t.Error(err)
	} else {
		obj := n.OutboundPermitOnly[2].(DynamicOutboundPermitValue)

		if obj.DdKey != "goo" || obj.Encoding != JSON {
			t.Error("Ill-formed network isolation member")
		}
	}
}

func Test_generatePermittedStringDynamic(t *testing.T) {
	isolation := &NetworkIsolation{
		OutboundPermitOnly: []OutboundPermitValue{
			StaticOutboundPermitValue("198.60.81.209/28"),
			StaticOutboundPermitValue("4.2.2.2"),
			DynamicOutboundPermitValue{
				DdKey:    "deployment_user_info",
				Encoding: "JSON",
				Path:     "quarks.externalBrokerHost",
			},
		},
	}

	containerNetwork := docker.ContainerNetwork{
		IPAddress:   "10.55.24.100",
		IPPrefixLen: 24,
	}

	deploymentUserInfo := `
		{
			"quarks": {
				"externalBrokerHost": "8.8.8.8"
			}
		}
	`

	url, _ := url.Parse("http://goo.foo")

	configure := gwhisper.NewConfigure("", *url, map[string]string{}, map[string]string{}, "", "", deploymentUserInfo)

	bytes, _ := json.Marshal(configure)

	permitted, err := generatePermittedString(isolation, containerNetwork, bytes)
	if err != nil {
		t.Error(err)
	} else if permitted != "198.60.81.209/28,4.2.2.2,8.8.8.8,10.55.24.100/24" {
		t.Errorf("Expected permitted string %v, but found %v", "gooo", permitted)
	}
}

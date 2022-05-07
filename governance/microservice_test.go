//go:build unit
// +build unit

package governance

import (
	"github.com/open-horizon/anax/exchange"
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_composeNewRegisteredServices(t *testing.T) {
	activeServices := []exchange.Microservice{
		exchange.Microservice{Url: "org1/url1", ConfigState: "active"},
		exchange.Microservice{Url: "org2/url2", ConfigState: "active"},
		exchange.Microservice{Url: "org3/url3", ConfigState: "active"},
		exchange.Microservice{Url: "org4/url4", ConfigState: "active"},
	}
	oldRegisteredServices := []exchange.Microservice{
		exchange.Microservice{Url: "org1/url1", ConfigState: "active"},
		exchange.Microservice{Url: "org2/url2", ConfigState: "active"},
		exchange.Microservice{Url: "org3/url3", ConfigState: "active"},
		exchange.Microservice{Url: "org6/url6", ConfigState: "suspended"},
		exchange.Microservice{Url: "org4/url4", ConfigState: "active"},
		exchange.Microservice{Url: "org5/url5", ConfigState: "suspended"},
	}
	newRS, isSame := composeNewRegisteredServices(activeServices, oldRegisteredServices)
	assert.True(t, isSame, "The elements should be the same.")
	assert.True(t, len(newRS) == 6, "The number of the elements should be 6")
	assert.True(t, newRS[4].ConfigState == "suspended", "There should be suspended element.")
	assert.True(t, newRS[5].ConfigState == "suspended", "There should be suspended element.")

	oldRegisteredServices = []exchange.Microservice{
		exchange.Microservice{Url: "org1/url1", ConfigState: "active"},
		exchange.Microservice{Url: "org5/url5", ConfigState: "active"},
		exchange.Microservice{Url: "org2/url2", ConfigState: "suspended"},
		exchange.Microservice{Url: "org3/url3", ConfigState: "active"},
		exchange.Microservice{Url: "org6/url6", ConfigState: "suspended"},
		exchange.Microservice{Url: "org4/url4", ConfigState: "active"},
	}
	newRS, isSame = composeNewRegisteredServices(activeServices, oldRegisteredServices)
	assert.False(t, isSame, "The elements should not be the same.")
	assert.True(t, len(newRS) == 5, "The number of the elements should be 5")
	assert.True(t, newRS[1].ConfigState == "suspended" && newRS[1].Url == "org2/url2", "There should be suspended element.")
	assert.True(t, newRS[4].ConfigState == "suspended" && newRS[4].Url == "org6/url6", "There should be suspended element.")

	oldRegisteredServices = []exchange.Microservice{
		exchange.Microservice{Url: "org1/url1", ConfigState: "active"},
		exchange.Microservice{Url: "org5/url5", ConfigState: "active"},
		exchange.Microservice{Url: "org2/url2", ConfigState: "active"},
		exchange.Microservice{Url: "org3/url3", ConfigState: "active"},
		exchange.Microservice{Url: "org6/url6", ConfigState: "active"},
		exchange.Microservice{Url: "org4/url4", ConfigState: "active"},
	}
	newRS, isSame = composeNewRegisteredServices(activeServices, oldRegisteredServices)
	assert.False(t, isSame, "The elements should not be the same.")
	assert.True(t, len(newRS) == 4, "The number of the elements should be 5")
	assert.True(t, newRS[1].ConfigState == "active" && newRS[1].Url == "org2/url2", "There should not be suspended element.")
	assert.True(t, newRS[3].ConfigState == "active" && newRS[3].Url == "org4/url4", "There should not be suspended element.")

	oldRegisteredServices = []exchange.Microservice{
		exchange.Microservice{Url: "org1/url1", ConfigState: "active"},
		exchange.Microservice{Url: "org5/url5", ConfigState: "active"},
		exchange.Microservice{Url: "org2/url2", ConfigState: "active"},
		exchange.Microservice{Url: "org3/url3", ConfigState: "active"},
		exchange.Microservice{Url: "org6/url6", ConfigState: "active"},
		exchange.Microservice{Url: "org4/url4", ConfigState: "active"},
	}
	newRS, isSame = composeNewRegisteredServices([]exchange.Microservice{}, oldRegisteredServices)
	assert.False(t, isSame, "The elements should not be the same.")
	assert.True(t, len(newRS) == 0, "The number of the elements should be 0")

	newRS, isSame = composeNewRegisteredServices(activeServices, nil)
	assert.False(t, isSame, "The elements should not be the same.")
	assert.True(t, len(newRS) == 4, "The number of the elements should be 4")
}

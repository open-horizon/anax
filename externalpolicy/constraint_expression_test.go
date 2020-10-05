// +build unit

package externalpolicy

import (
	_ "github.com/open-horizon/anax/externalpolicy/text_language"
	"testing"
)

// ================================================================================================================
// Verify the function that converts external policy constraint expressions to the internal JSON format, for simple
// constraint expressions.
//
func Test_simple_conversion(t *testing.T) {

	ce := new(ConstraintExpression)

	(*ce) = append((*ce), "prop == value")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be a top level array element")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 2 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3 || prop4 == value4")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement len(tle): %v tle: %v", len(tle), tle)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2 || prop3 == value3 || prop4 == value4 && prop5 == value5")
	if rp, err := RequiredPropertyFromConstraint(ce); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	} else if tle := rp.TopLevelElements(); tle == nil {
		t.Errorf("Error: There should be 3 top level array elements")
	} else if len(tle) != 1 {
		t.Errorf("Error: Should be 1 top level array alement")
	}
}

func Test_succeed_IsSatisfiedBy(t *testing.T) {
	ce := new(ConstraintExpression)
	(*ce) = append((*ce), "prop == true && prop2 == \"a b\"")
	props := new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", true)), *(Property_Factory("prop2", "a b")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == onefishtwofish && prop2 == value2")
	props = new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", "onefishtwofish")), *(Property_Factory("prop2", "value2")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce), "prop == value && prop2 == value2", "prop3 == value3 || prop4 == value4 && prop5 <= 5", "property6 >= 6")
	props = new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", "value")), *(Property_Factory("prop2", "value2")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", 5)), *(Property_Factory("property6", 7.0)))
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"iame2edev == true && cpu == 3 || memory <= 32",
		"hello == \"world\"",
		//"hello in \"'hiworld', 'test'\"",
		"eggs == \"truckload\" AND certification in \"USDA,Organic\"",
		"version == 1.1.1 OR USDA == true",
		"version in [1.1.1,INFINITY) OR cert == USDA")
	prop_list := `[{"name":"iame2edev", "value":true},{"name":"cpu", "value":3},{"name":"memory", "value":32},{"name":"hello", "value":"world"},{"name":"eggs","value":"truckload"},{"name":"USDA","value":true},{"name":"certification","value":"USDA"},{"name":"version","value":"1.2.1","type":"version"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	prop_list = `[{"name":"version", "value":"2.1.5", "type":"version"},{"name":"USDA", "value":"Department"},{"name":"color", "value":"orange","type":"string"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"purpose==network-testing")
	prop_list = `[{"name":"purpose", "value":"network-testing"},{"name":"group", "value":"bluenode"},{"name":"openhorizon.cpu", "value":"1"},{"name":"openhorizon.arch", "value":"amd64"},{"name":"openhorizon.memory", "value":"3918"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"iame2edev == true",
		"NONS==false || NOGPS == true || NOLOC == false || NOPWS == false || NOHELLO == false")
	prop_list = `[{"name":"iame2edev", "value":"true"},{"name":"NONS", "value":"false"},{"name":"number", "value":"12"},{"name":"foo", "value":"bar"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"color == gray || (colour =\"grey\" && countryin\"england,australia,canada\")")
	prop_list = `[{"name":"color", "value":"gray"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"(species == t-rex && ((feathers =hopefully && scales=maybe || (talonsin\"sharp,very sharp\" || teeth =1.3.8))))")
	prop_list = `[{"name":"species", "value":"t-rex"},{"name":"feathers", "value": "hopefully"},{"name":"talons", "value": "sharp    "}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"species == cat")
	prop_list = `[{"name":"species", "value":"toad,cat,dog"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"version in [0.0.1,9.3.2)")
	prop_list = `[{"name":"version", "value":"0.0.2"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"inVersion in [0.0.1,9.3.2) && intValue > 4")
	prop_list = `[{"name":"inVersion", "value":"0.0.2"},{"name":"intValue","value":5}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"ininString in \"cat, dog, mouse\" || intValue > 4")
	prop_list = `[{"name":"ininString", "value":"dog"},{"name":"intValue","value":2}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err != nil {
		t.Errorf("Error: unable to convert simple expression: %v", err)
	}
}

func Test_fail_IsSatisfiedBy(t *testing.T) {
	ce := new(ConstraintExpression)
	(*ce) = append((*ce), "prop == true && prop2 == \"value2, value3, value4\"")
	props := new([]Property)
	(*props) = append((*props), *(Property_Factory("prop", true)), *(Property_Factory("prop2", "value3")), *(Property_Factory("prop3", "value3")), *(Property_Factory("prop4", "value4")), *(Property_Factory("prop5", "value5")))
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"((versionin[1.3.1,3.1.2) && server=false) || (os!=windows && foo !=bar) )")
	prop_list := `[{"name":"version", "value":"2.3.4"},{"name":"foo", "value": "chunk"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		" (sasquatch=real && locationin\"British Columbia,Alberta,Montana\") OR (nessie  ==   real && location=\"Scotland\")")
	prop_list = `[{"name":"sasquatch", "value":"real"},{"name":"location", "value": "Scotland"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		" purpose == testing AND ( nodenum > 4 || nodenum <= 2)")
	prop_list = `[{"name":"purpose", "value":"testing"},{"name":"nodenum", "value": "3"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		" purpose == testing AND ( nodenum > 4 || nodenum <= 2 || (test2 in \"a,b,c\" && openhorizon.cpu ==2) || propx = false)")
	prop_list = `[{"name":"purpose", "value":"testing"},{"name":"nodenum", "value": "3"},{"name":"test2", "value": "b"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"( purpose == testing) AND ( nodenum > 4 || nodenum <= 2 || (test2 in \"a,b,c\" && openhorizon.cpu ==2) || propx = false)")
	prop_list = `[{"name":"openhorizon.cpu", "value":"2"},{"name":"nodenum", "value": "3"},{"name":"test2", "value": "b"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"device=kiosk && (station=kyoto OR station=hunai)")
	prop_list = `[{"name":"device", "value":"kiosk"},{"name":"station", "value": "tokyo"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"device=kiosk && (station=kyoto OR station=hunai) && stationtype=\"train\"")
	prop_list = `[{"name":"device", "value":"kiosk"},{"name":"station", "value": "kyoto"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"device=kiosk && station=kyoto  && stationtype=\"train\" AND schedule=on-time")
	prop_list = `[{"name":"device", "value":"kiosk"},{"name":"schedule", "value": "on-time"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"device=kiosk || station=kyoto  || stationtype=\"train\" OR schedule=on-time")
	prop_list = `[{"name":"device", "value":"scanner"},{"name":"schedule", "value": "delayed"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"((device=kiosk || station=macau) && (station=kyoto OR station=hunai))")
	prop_list = `[{"name":"device", "value":"kiosk"},{"name":"station", "value": "tokyo"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}

	ce = new(ConstraintExpression)
	(*ce) = append((*ce),
		"purpose == network-testing1", "group == bluenode")
	prop_list = `[{"name":"purpose", "value":"network-testing"},{"name":"group", "value": "bluenode"}]`
	props = create_property_list(prop_list, t)
	if err := ce.IsSatisfiedBy(*props); err == nil {
		t.Errorf("Error: constraints not satisfied but no error occured %v %v", ce, props)
	}
}

func Test_MergeWith(t *testing.T) {
	ce1 := new(ConstraintExpression)
	ce2 := new(ConstraintExpression)
	(*ce1) = append((*ce1), "prop == true")
	(*ce2) = append((*ce2), "prop == true")
	ce1.MergeWith(ce2)
	if len(*ce1) != 1 {
		t.Errorf("Error: constraints %v should have 1 element but got %v", ce1, len(*ce1))
	}

	ce1 = new(ConstraintExpression)
	ce2 = new(ConstraintExpression)
	(*ce1) = append((*ce1),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	(*ce2) = append((*ce2),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	ce1.MergeWith(ce2)
	if len(*ce1) != 3 {
		t.Errorf("Error: constraints %v should have 3 elements but got %v", ce1, len(*ce1))
	}

	ce1 = new(ConstraintExpression)
	ce2 = new(ConstraintExpression)
	(*ce1) = append((*ce1),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA",
		"color == \"orange\"")
	(*ce2) = append((*ce2),
		"version == 1.1.1 OR USDA in \"United,States,Department,of,Agriculture\"",
		"version in [1.1.1,INFINITY) OR cert == USDA_PART1")
	ce1.MergeWith(ce2)
	if len(*ce1) != 4 {
		t.Errorf("Error: constraints %v should have 4 elements but got %v", ce1, len(*ce1))
	}
}

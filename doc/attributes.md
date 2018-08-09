## Horizon Attributes

This document contains the definition for each attribute that can be set on the [POST /attribute](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--attribute) API or the [POST /microservice/config](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--microserviceconfig) API.
Attributes are used to condition the behavior of the Horizon agent and/or microservices running on the agent.

The body of the attribute section always follows the following form:
```
    {
        "type": "<attribute_type>",
        "label": "a string that could be displayed in a user interface",
        "publishable": boolean,
        "host_only": boolean,
        "mappings": {
          "variable": value
        }
    }
```

The `type` field is a string that indicates which attribute you would like to set.
Valid values are:
* [ComputeAttributes](#compa)
* [LocationAttributes](#loca)
* [UserInputAttributes](#uia)
* [HTTPSBasicAuthAttributes](#httpsa)
* [DockerRegistryAuthAttributes](#bxa)
* [HAAttributes](#haa)
* [MeteringAttributes](#ma)
* [AgreementProtocolAttributes](#agpa)
* [PropertyAttributes](#pa)
* [CounterPartyPropertyAttributes](#cpa)

Each attrinbute type is described in it's own section below.

The `label` field is a string that is displayed in the Horizon user interface when working with this attribute.

The `publishable` field is a boolean that indicates whether or not the variables in the mapping section are published for the workload to see.

The `host_only` field is a boolean that indicates whether or not this attribute should be applied to the Horizon agent runtime, not to a specific microservice.

The `mappings` field is a map of variables and values that are specific to the type of the attribute.
If an attribute type has any specific variables to be set, they are described in the type's section below.

### <a name="compa"></a>ComputeAttributes
This attribute is used to define the number of CPUs and the amount of memory to allocate to the microservice.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The variables you can set are:
* `cpu` - An integer describing the number of CPUs to allocate to the microservice. The default is 1.
* `ram` - An integer in MB describing how much memory to allocate to the microservice. The default is 128.

These values will be applied to microservice docker containers, but are not currently included in agreement negotiation.

For example; 2 CPUS and 256 MB of memory:
```
    {
        "type": "ComputeAttributes",
        "label": "Compute Resources",
        "publishable": true,
        "host_only": false,
        "mappings": {
            "ram": 256,
            "cpus": 2
        }
    }
```

### <a name="loca"></a>LocationAttributes
This attribute is used to define the GPS coordinates of the Horizon agent.

The value for `host_only` should be `false`.

The variables you can set are:
* `lat` - A float describing the GPS latitude of the node.
* `lon` - A float describing the GPS longitude of the node.
* `location_accuracy_km` - A float describing a random radius within which the specific `lat` and `lon` are located.
This allows the microservice to broadcast its location publicly without giving away its exact coordinates.
* `use_gps` - A boolean that enables the microservice to use a GPS chip (ignoring the `lat` and `lon`) to determine coordinates.

For example;
```
    {
        "type": "LocationAttributes",
        "label": "Registered Location Facts",
        "publishable": false,
        "host_only": false,
        "mappings": {
            "lat": 1.234567,
            "lon": -1.234567,
            "location_accuracy_km": 0.5,
            "use_gps": false
        }
    }
```

### <a name="uia"></a>UserInputAttributes
This attribute is used to set user input variables from a microservice definition.
Every microservice can define variables that the node user can configure.
Only microservice variables that don't have default values in the microservice definition must be set through the UserInputAttributes attribute.
The variables are typed, which can also be found in the microservice definition.
The supported types are: `string`, `int`, `float`, `boolean`, `list of strings`.
These variables are converted to environment variables (and the value is converted to a string) so they can be passed into the microservice implementation container.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The variables you can set are defined by the microservice definition.
Suppose the microservice definition contained the following userInputs section:
```
    "userInput":[
        {
            "name":"test",
            "label":"a label description",
            "type":"string"
        },
        {
            "name":"testdefault",
            "label":"a label description",
            "type":"string",
            "defaultValue":"default"
        }
    ]
```

The `test` variable has no default so it needs to be set through a UserInputAttributes attribute.
The `testDefault` variable has a default, so it can be optionally set by the same attribute.
For example:
```
    {
        "type": "UserInputAttributes",
        "label": "variables",
        "publishable": true,
        "host_only": false,
        "mappings": {
            "test": "aValue"
        }
    }
```

### <a name="httpsa"></a>HTTPSBasicAuthAttributes
This attribute is used to set a host wide basic auth user and password for HTTPS communication.
The `sensor_urls` variable sets the HTTP network domain and path to which this attribute applies.
HTTPS communication to other URLs will not use this basic auth configuration.

The value for `publishable` should be `false`.

The value for `host_only` should be `true`.

The `username` variable is a string containing the basic auth user name.

The `password` variable is a string containing the basic auth user's password.

For example:
```
    {
        "type": "HTTPSBasicAuthAttributes",
        "label": "HTTPS Basic auth",
        "sensor_urls": [
            "https://us.internetofthings.ibmcloud.com/api/v0002/horizon-image/common"
        ],
        "publishable": false,
        "host_only": true,
        "mappings": {
            "username": "me",
            "password": "myPassword"
        }
    }
```

### <a name="bxa"></a>DockerRegistryAuthAttributes
This attribute is used to set a docker authentication user name and password or token that enables the Horizon agent to access a docker repository when downloading images for microservices and workloads.

The value for `publishable` should be `false`.

The value for `host_only` should be `true`.

The value for `token` can be a token, an API key or a password. 

/* use this if your docker images are in the IBM Cloud container registry, you can use either token or Identity and Access Management (IAM) API key. */


For example:
```
    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "sensor_urls": [
            "mydockerrepo"
        ],
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {"user": "user1", "token": "myDockerhubPassword"}
            ]
        }
    }

    /* Use this if your docker images are in the IBM Cloud container registry. The `myDockerToken` variable is a string containing the docker token used to access the repository. */

    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "sensor_urls": [
            "registry.ng.bluemix.net"
        ],
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {"user": "token", "token": "myDockerToken"}
            ]
        }
    }

    /* Use this if your docker images are in the IBM Cloud container registry. The `myIAMApiKey` variable is a string containing the IBM Cloud Identity and Access Management (IAM) API key. The user is `iamapikey`. */
    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "sensor_urls": [
            "registry.ng.bluemix.net"
        ],
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {"user": "iamapikey", "token": "myIAMApiKey"}
            ]
        }
    }


```

### <a name="haa"></a>HAAttributes
This attribute is used to declare the node as an HA partner with some other node(s).
HA nodes all have the same microservices and workloads running on them.
Workload and microservice upgrades will happen sequentially (i.e. not concurrently) on each HA partner so that there is always at least 1 node running.
This attribute is used in conjunction with the `ha` field on the [POST /node](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--node) API.
If that `ha` field is set to true, then this attribute is used to specify the device ID of the partner node(s).
The 'partnerID' variable is used to declare the partner(s) for this node, the value is the `id` field of the [POST /node](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--node) API that the partner node(s) used.
Each node that is a partner must name all its partners.

The value for `publishable` should be `false`.

The value for `host_only` should be `false`.

For example:
```
    {
        "type": "HAAttributes",
        "label": "HA Partner",
        "publishable": false,
        "host_only": false,
        "mappings": {
            "partnerID": ["otherNode"]
        }
    }
```

### <a name="ma"></a>MeteringAttributes
This attribute is used to configure how the microservice wants to be metered as part of an agreement.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The variables that can be configured are:
* `tokens` - The number of tokens to be granted per unit time as specified below.
* `perTimeUnit` - The unit of time over which the tokens are granted. Valid values are: `min`, `hour`, `day`.
* `notificationInterval` - An integer indication how often, in seconds, the agbot should notify the node that tokens are being granted.

If the agbot also specifies a metering policy, the metering attributes specified by the node must be satisfied by the agbot's policy.
If a nodes wants more token per unit time than the agbot is willing to provide, then an agreement cannot be made.
If an agbot is able to satisfy the node, then the tokens per unit time specified by the node willbe used.

For example, the microservice wants the agbot to grant 2 tokens per hour, and notify the mode that the agreement is still valid every hour (3600 seconds).
```
{
    "type": "MeteringAttributes",
    "label": "Metering Policy",
    "publishable": true,
    "host_only": false,
    "mappings": {
        "tokens": 2,
        "perTimeUnit": "hour",
        "notificationInterval": 3600
    }
}
```

### <a name="agpa"></a>AgreementProtocolAttributes
This attribute is used when a microservice has a specific requirement for an agreement protocol.
An agreement protocol is a pre-defined mechanism for enabling 2 entities (a node and an agbot) to agree on which microservices and workloads to run.
The Horizon system supports 2 protocols; "Citizen Scientist" and "Basic".
The "Citizen Scientist" protocol is based on and requires an Ethereum blockchain.
By default, the Horizon system uses the "Basic" protocol (which requires nothing more than a TCP network) and therefore this attribute should only be used in advanced situations where more than 1 protocol is available.

Agreement protocols are chosen by the agbot based on the order they appear in the node's microservice's attributes.
For the "Citizen Scientist" protocol, a specific blockchain instance can be chosen.
Blockchain instances must be registeres in the exchange and refered to by name and org in this attribute.
It is recommended that this attribute is defined once for all microservices on the node so that all microservice attempt to use the same blockchain instance.

For example, the microservice wants to prefer the "Basic" protocol, but is willing to use "Citizen Scientist" with either of the blockchain instances shown:
```
    {
        "type": "AgreementProtocolAttributes",
        "label": "Agreement Protocols",
        "publishable": true,
        "host_only": false,
        "mappings": {
            "protocols": [
                {
                    "Basic": []
                },
                {
                    "Citizen Scientist": [
                        {
                            "name": "privatebc",
                            "organization": "e2edev"
                        },
                        {
                            "name": "bluehorizon",
                            "organization": "e2edev"
                        }
                    ]
                }
            ]
        }
    }
```

### <a name="pa"></a>PropertyAttributes
This attribute is used to configure arbitrary properties on the microservice such that an agbot can select (or ignore) nodes with given property values.
These properties are ignored when a node uses the [POST /node](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--node) API with a non-empty `pattern` field.
Think of these properties as the means by which a node can advertise anything about its microservice(s).
An agbot policy file that wants to select a node based on these properties would do so using the "counterPartyProperties" section of its policy file.
When a property is advertised, it's value is automaticaly determined to be 1 of the following types: `string`, `int`, `float`, `boolean`, `list of strings`.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

For example, `myProp` is a `boolean` property:
```
    {
        "type": "PropertyAttributes",
        "label": "Property",
        "publishable": true,
        "host_only": false,
        "mappings": {
            "myProp": true
        }
    }
```

### <a name="cpa"></a>CounterPartyPropertyAttributes
This attribute is used to indicate that a microservice will only be part of an agreement with an agbot that advertises properties which satisfy the specified expression.
Agbots can advertise properties in their policy files similarly to how nodes advertise properties.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The counter-party property expression syntax is as follows:
```
    "expression": {
        _control_operator_ : [_expression_] || _property_
    }
```
Where:
* `_control_operator_` is one of `and`, `or`, `not`
* `_expression_` = `_control_operator_`: [`_expression_`] or `_property_`
* `_property_` = "name": "property_name", "value": "property_value", "op": `_comparison_operator_`
* `_comparison_operator_` is one of `<`, `=`, `>`, `<=`, `>=`, `!=`.
The `=` and `!=` comparison operators can be applied to strings and integers.
If the "op" key is missing, then `=` is assumed.

For example, only make agreements with an agbot that advertises property p1="1" and p2="2", OR p3>3:
```
    {
        "type": "CounterPartyPropertyAttributes",
        "label": "CounterParty Property",
        "publishable": true,
        "host_only": false,
        "mappings": {
            "expression": {
                "or": [
                    "and": [
                        {
                            "name":"p1",
                            "op":"=",
                            "value":"1"
                        },
                        {
                            "name":"p2",
                            "op":"=",
                            "value":"2"
                        }
                    ],
                    "and": [
                        {
                            "name":"p3",
                            "op":">",
                            "value":3
                        }
                    ]
                ]
            }
        }
    }
```

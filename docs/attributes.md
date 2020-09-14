## Horizon Attributes

This document contains the definition for each attribute that can be set on the [POST /attribute](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--attribute) API or the [POST /service/config](https://github.com/open-horizon/anax/blob/master/doc/api.md#api-post--serviceconfig) API.
Attributes are used to condition the behavior of the Horizon agent and/or services running on the agent.

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
* [UserInputAttributes](#uia)
* [HTTPSBasicAuthAttributes](#httpsa)
* [DockerRegistryAuthAttributes](#bxa)
* [HAAttributes](#haa)
* [MeteringAttributes](#ma)
* [AgreementProtocolAttributes](#agpa)

Each attrinbute type is described in it's own section below.

The `label` field is a string that is displayed in the Horizon user interface when working with this attribute.

The `publishable` field is a boolean that indicates whether or not the variables in the mapping section are published for the workload to see.

The `host_only` field is a boolean that indicates whether or not this attribute should be applied to the Horizon agent runtime, not to a specific service.

The `mappings` field is a map of variables and values that are specific to the type of the attribute.
If an attribute type has any specific variables to be set, they are described in the type's section below.

### <a name="uia"></a>UserInputAttributes
This attribute is used to set user input variables from a service definition.
Every service can define variables that the node user can configure.
Only service variables that don't have default values in the service definition must be set through the UserInputAttributes attribute.
The variables are typed, which can also be found in the service definition.
The supported types are: `string`, `int`, `float`, `boolean`, `list of strings`.
These variables are converted to environment variables (and the value is converted to a string) so they can be passed into the service implementation container.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The `service_specs` specifies what services the attribue applies to. If the `url` is an empty string, it applies to all the services. If you set the UserInputAttributes through the `/service/config` api, you do not need to specify the `service_specs` becuase the serivce is specified in other fields. However, if you use `/attribute` api to set the UserInputAttributes, you must specify the `service_specs`. 

The variables you can set are defined by the service definition.
Suppose the service definition contained the following userInputs section:
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
        "service_specs": [
            {
                "url": "https://bluehorizon.network/services/netspeed",
                "organization": "myorg"
            }
        ],
        "mappings": {
            "test": "aValue"
        }
    }
```

### <a name="httpsa"></a>HTTPSBasicAuthAttributes
This attribute is used to set a host wide basic auth user and password for HTTPS communication.
The `url` variable sets the HTTP network domain and path to which this attribute applies.
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
        "publishable": false,
        "host_only": true,
        "mappings": {
            "url":      "https://us.internetofthings.ibmcloud.com/api/v0002/horizon-image/common"
            "username": "me",
            "password": "myPassword"
        }
    }
```

### <a name="bxa"></a>DockerRegistryAuthAttributes
This attribute is used to set a docker authentication user name and password or token that enables the Horizon agent to access a docker repository when downloading images for services and workloads.

The value for `publishable` should be `false`.

The value for `host_only` should be `true`.

The value for `token` can be a token, an API key or a password. 

/* use this if your docker images are in the IBM Cloud container registry, you can use either token or Identity and Access Management (IAM) API key. */


For example:
```
    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {
                    "registry": "mydockerrepo", 
                    "username": "user1", 
                    "token": "myDockerhubPassword"
                }
            ]
        }
    }

    /* Use this if your docker images are in the IBM Cloud container registry. The `myDockerToken` variable is a string containing the docker token used to access the repository. */

    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {
                    "registry": "registry.ng.bluemix.net",
                    "username": "token", 
                    "token": "myDockerToken"
                }
            ]
        }
    }

    /* Use this if your docker images are in the IBM Cloud container registry. The `myIAMApiKey` variable is a string containing the IBM Cloud Identity and Access Management (IAM) API key. The user is `iamapikey`. */
    {
        "type": "DockerRegistryAuthAttributes",
        "label": "Docker auth",
        "publishable": false,
        "host_only": true,
        "mappings": {
            "auths": [
                {
                    "registry": "registry.ng.bluemix.net", 
                    "username": "iamapikey", 
                    "token": "myIAMApiKey"}
            ]
        }
    }


```

### <a name="haa"></a>HAAttributes
This attribute is used to declare the node as an HA partner with some other node(s).
HA nodes all have the same services and workloads running on them.
Workload and service upgrades will happen sequentially (i.e. not concurrently) on each HA partner so that there is always at least 1 node running.
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
This attribute is used to configure how the service wants to be metered as part of an agreement.

The value for `publishable` should be `true`.

The value for `host_only` should be `false`.

The variables that can be configured are:
* `tokens` - The number of tokens to be granted per unit time as specified below.
* `perTimeUnit` - The unit of time over which the tokens are granted. Valid values are: `min`, `hour`, `day`.
* `notificationInterval` - An integer indication how often, in seconds, the agbot should notify the node that tokens are being granted.
* `service_specs` - An array specifies what services the attribue applies to. If the `url` is an empty string, it applies to all the services. If you set the MeteringAttributes through the `/service/config` api, you do not need to specify the `service_specs` becuase the serivce is specified in other fields. However, if you use `/attribute` api to set the MeteringAttributes, you must specify the `service_specs`. 

If the agbot also specifies a metering policy, the metering attributes specified by the node must be satisfied by the agbot's policy.
If a nodes wants more token per unit time than the agbot is willing to provide, then an agreement cannot be made.
If an agbot is able to satisfy the node, then the tokens per unit time specified by the node willbe used.

For example, the service wants the agbot to grant 2 tokens per hour, and notify the mode that the agreement is still valid every hour (3600 seconds).
```
{
    "type": "MeteringAttributes",
    "label": "Metering Policy",
    "publishable": true,
    "host_only": false,
    "service_specs": [
        {
            "url": "https://bluehorizon.network/services/netspeed",
            "organization": "myorg"
        }
    ],
    "mappings": {
        "tokens": 2,
        "perTimeUnit": "hour",
        "notificationInterval": 3600
    }
}
```

### <a name="agpa"></a>AgreementProtocolAttributes
This attribute is used when service has a specific requirement for an agreement protocol.
An agreement protocol is a pre-defined mechanism for enabling 2 entities (a node and an agbot) to agree on which services and workloads to run.
The Horizon system supports 1 protocol; "Basic".
By default, the Horizon system uses the "Basic" protocol (which requires nothing more than a TCP network) and therefore this attribute should only be used in advanced situations where more than 1 protocol is available.

Agreement protocols are chosen by the agbot based on the order they appear in the node's service's attributes.

The `service_specs` specifies what services the attribue applies to. If the `url` is an empty string, it applies to all the services. If you set the AgreementProtocolAttributes through the `/service/config` api, you do not need to specify the `service_specs` becuase the serivce is specified in other fields. However, if you use `/attribute` api to set the AgreementProtocolAttributes, you must specify the `service_specs`. 

```
    {
        "type": "AgreementProtocolAttributes",
        "label": "Agreement Protocols",
        "publishable": true,
        "host_only": false,
        "service_specs": [
            {
                "url": "https://bluehorizon.network/services/netspeed",
                "organization": "myorg"
            }
        ],
        "mappings": {
            "protocols": [
                {
                    "Basic": []
                }
            ]
        }
    }
```



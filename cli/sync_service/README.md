# Model Management System

## Introduction

The model management system (MMS) is one of the most important parts in Edge Fabric(EF). It is leveraged to ease the burden of AI model management of cognitive services running on an edge node. It can be extended to distribute broader types of data files to edge nodes. The MMS will facilitate the storage, delivery and security of models/data and other metadata packages needed by edge and cloud services. Edge nodes can send and receive models and metadata to and from the cloud.

The MMS is implemented by embedding the sync service into the EF. The cloud sync service (CSS) delivers models/metadata/data to specific nodes or groups of nodes within a given organization. Once the objects are delivered, a service is able to obtain the model/data from edge sync service (ESS) by calling ESS API.

Objects are populated in the CSS by service developers, devops administrators, and model authors. The CSS facilitates an integration between AI model tools and the cognitive services running on the edge. As models are completed by model authors, they are published to the CSS, making them immediately available to edge nodes.

EF provides a set of hzn CLIs to use MMS to manipulate the model objects and their metadata.
 
### MMS Concepts

MMS is leveraging Sync Service to manage the objects. This section is about some sync service concepts and how they are related to EdgeFabric.

#### CSS and ESS

Sync service is consist of two components: cloud sync service (CSS) and edge sync service (ESS). The CSS is deployed in EF cluster as part of the IBM EF cloud service. CSS connects to database (mongoDB) to store and retrieve objects on the cloud.

ESS is embedded in the EF agent and running on the edge node. ESS constantly polls object updates from CSS, and stores the object in database (bolt) on the edge node. ESS APIs can be used by user defined services on the edge node to access the metadata and model/data object.

#### object: metadata and data

Metadata is the file to describe your data/models. Object is published by MMS with metadata and data, or only with metadata. In the metadata, objectType and objectID as a whole are defined as identity of the object within the same organization. Destination related fields (`destinationOrgID`, `destinationType`, `destinationID`, `destinationList`, `destinationPolicy`) are used to determine the ESS/nodes to send the object to. Other object information including description, version, etc., can be specified in the metadata as well. The version value has no semantic meaning to the sync service, therefore only 1 copy of the object exists in the CSS.

Data file is the file to distribute. Example of the data files are AI model file, configuration file, binary data, etc.

#### AI model

AI model is not sync service concept. It is a major use case of MMS. AI (Artificial Intelligence) model is a mathematical representation of a real world process that related to Artificial Intelligence. Cognitive services which mimic the cognitive functions of human will use and consume the AI model. To generate an AI model, you will need to apply AI algorithms on training data. In summary, the AI model in EF is distributed by MMS and used by cognitive user defined services running on the edge node.

### MMS Concept with EF

What is the relationship between those MMS concepts and EF? How are they appied in EF system? This section will describe the concept mapping between MMS concepts and EF concepts.

EF currently have two ways to register a node: register with pattern or policy. For the node that is registered with pattern, the `destinationType` is mapping to pattern name. All EF nodes using the same pattern can be thought of as being in the same group. Therefore this mapping makes it possible to target objects to all nodes of a given type. `destinationID` is set same as EF nodeID of your target edge node. When `destinationID` is not set, the object will be broadcast to all nodes with given pattern(`destinationType`)

For the node that is registered with policy. `destinationType` and `destinationID` should leave blank in the metadata because the destinations of object are calculated and set automatically by agbot. In this case, `destinationPolicy` is the field that holds policy property, constrain and service. The destination policy will be compared with other EF policies (service policy, business policy and node policy) and then generate a list of nodes that will receive the object. Then those nodes will be added to the object destination list by agbot.

There is no explicit relateionship between services deployed by the EF. and the ESS hosted objects. Each EF node will have 1 ESS, but may have several services running on it. To control the object access, EF and ESS have authentication layer to control the access of object for services running on the EF node. Objects are only visible to the services that have authentication. EF service definitions, patterns, etc do not explicitly refer to objects managed by the MMS. Lkewise, MMS object metadata does not explicitily refer to EF services, pattern etc. This allows the service life cycle to be independent from the object/model life cycle.

## MMS CLI

This section will describe a MMS example and the usage of some MMS commands. As a user, I want to publish an object to a EF node, so that my helloMMS service running on that node can use the object.
 
### Check MMS status

Before publishing the object, we need to check MMS status using `hzn mms status` command, to make sure MMS is running properly. Check `heathStatus` under `general` and `dbStatus` under `dbHealth`. The values of these two fields should be “green”, which indicate that CSS (cloud sync service part of MMS) and database are both running.

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms status
    {
      "general": {
        "nodeType": "CSS",
        "healthStatus": "green",
        "upTime": 21896
      },
      "dbHealth": {
        "dbStatus": "green",
        "disconnectedFromDB": false,
        "dbReadFailures": 0,
        "dbWriteFailures": 0
      }
    }
 
### Create and publish MMS object

#### Create MMS Object

In MMS, model/data file is not published by its own. MMS requires a metadata file along with the model/data file when publishing and distributing. Metadata file configures a set of attributes of your data/model. MMS will store, distribute, and retrieve the model objects based on those attributes defined in metadata. 

Metadata file is a json file. You can use `hzn mms object new` to view a template of metadata file. Use `hzn mms object new >> my_metadata.json` to copy the template to a file named `my_metadata.json`.  (Or you can just copy the template from terminal and paste to a file).

We will talk about meaning of some important fields before showing the metadata example.

`objectID` and `objectType` are required fields. `objectID` is unique identifier of the object. `objectType` is the type of the object.

`destinationOrgID` is a required field no matter to distribute object to nodes regiestered with pattern or with policy. 

When distributing object to nodes registered with pattern, `destinationType` with `destinationID`, or `destinationsList` will be used. `destinationType` refers to the pattern in use by nodes that should receive this object. `destinationID` will be the node id (without org prefix) where the object should be placed. If `destinationID` omit, (and if destinationsList is omitted too), the object is broadcast to all known nodes. The alternative way to define destinations is to use `destinationsList`. It is easier to give more than 1 destination pairs in destinationsList in format of `pattern:nodeId`.

`destinationPolicy` field is used when distributing object to nodes registered with policy. Delete `destinationType`, `destinationID` and `destinationsList` field in this situation. `properties`, `constraints` and `services` will describe the condition, and constrains of the destinations to receive this object. This field will be populated to EF agot for destination generation.

`version` and `description` can be given as string within metadata. The value of `version` is not semantically interpreted. The MMS does not keep multiple version of an object.

`expiration` and `activationTime` is optional. `expiration` indicates when object will be expirated and removed from MMS. `activationTime` is a timestamp/date as to when this object should automatically be activated. Both should be provided in RFC3339 format. 

After filling out the fields in `my_metadata.json`, you can save the file.

##### Send MMS to node running with policy

My edge node `an12345` is using policy. `helloMMS` is one of the services running on it. 
I would like my data file (input.json) to be used by `helloMMS` (service script can be found here: https://github.com/open-horizon/examples/blob/master/edge/services/helloMMS/service.sh). Since the target node is registered with policy, I only use `destinationOrgID` and `destinationPolicy`. I filled out the metadata file like the following:

    {
      "objectID": "input.json",
      "objectType": "json",
      "destinationOrgID": "$HZN_ORG_ID",
      "destinationPolicy": {
        "properties": [],
        "constraints": [],
        "services": [
          {
            "orgID": "e2edev@somecomp.com",
            "arch": "amd64",
            "serviceName": "my.company.com.services.hellomms",
            "version": "1.0.0"
          }
        ]
      },
      "version": "0.0.1",
      "description": "helloMMS"
    }
    
##### Send MMS to node running with pattern

Now we have the same node (nodeID: an12345), but this time it is registered with pattern `pattern-ibm.hello-mms`, which has `helloMMS` as one of the services.

If I want to send object to this node with pattern, I will make some changes to my metadata file. In this case, I will:

1. specify node pattern name as the `destinationType`. 
1. specify node id as the `destinationID`.  
1. remove destinationPolicy field

The metadat file looks like:

    {
      "objectID": "input.json",
      "objectType": "json",
      "destinationOrgID": "$HZN_ORG_ID",
      "destinationID": "an12345",
      "destinationType": "pattern-ibm.hello-mms",
      "destinationsList": null,
      "version": "0.0.1",
      "description": "helloMMS"
    }


Please refer helloMMS as an example (https://github.com/open-horizon/examples/blob/master/edge/services/helloMMS/object.json)

Now, I have both my data file(input.json) and metadata file(my_metadata.json) ready. Next step is to publish my MMS object those files.

#### Publish MMS object

To publish my data/model with its metadata, you will use `hzn mms object publish [<flags>]` command

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object publish -m my_metadata.json -f input.json
    Object input.json added to org userdev in the Model Management Service
    

#### List MMS object

hzn mms CLI provides a command to list MMS objects with given flags. This command will list all objects with its objectID and objectType within the given org:

    hzn mms object list [<flags>]
    
result of the command:

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object list
    Listing objects in org userdev:
    [
      {
        "objectID": "policy-basicres.tgz",
        "objectType": "model"
      },
      {
        "objectID": "policy-multires.tgz",
        "objectType": "model"
      },
      {
        "objectID": "input.json",
        "objectType": "json"
      }
    ]
    
We can specify `objectType` and `objectID` to check the object we just published:

    hzn mms object list --objectType=json --objectId=input.json
    
result of the command:

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object list --objectType=json --objectId=input.json
    Listing objects in org userdev:
    [
      {
        "objectID": "input.json",
        "objectType": "json"
      }
    ]
    
To show the full information of MMS object metadata, you can add `-l` to the command:

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object list --objectType=model --objectId=lily1 -l

To show object status and destinations along with the object, just add `-d` to the command. The following destination result indicates my object is delivered to my edge node `an12345`

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object list --objectType=json --objectId=input.json -d
    Listing objects in org userdev:
    [
      {
        "objectID": "input.json",
        "objectType": "json",
        "destinations": [
          {
            "destinationType": "openhorizon.edgenode",
            "destinationID": "an12345",
            "status": "delivered",
            "message": ""
          }
        ],
        "objectStatus": "ready"
      }
    ]
    
There are more advanced filtering options available to narrow down the MMS object list. Full list of flags can be obtained by `hzn mms object list --help`

#### Delete MMS object
If you want to delete your MMS object, simply use this hzn command:

    hzn mms object delete --type=TYPE --id=ID

Result of delete command:

    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object delete --type=json --id=input.json
    Object input.json deleted from org userdev in the Model Management Service
    
The object will be removed from MMS.

## Appendix

Examples of some hzn mms CLI output

#### Get template of MMS Object metadata
    root@lily-test:~/go/src/github.com/open-horizon/anax# hzn mms object new
    {
      "objectID": "",            /* Required: A unique identifier of the object. */
      "objectType": "",          /* Required: The type of the object. */
      "destinationOrgID": "$HZN_ORG_ID", /* Required: The organization ID of the object (an object belongs to exactly one organization). */
      "destinationID": "",       /* The node id (without org prefix) where the object should be placed. */
                                 /* If omitted the object is sent to all nodes with the same destinationType. */
                                 /* Delete this field when you are using destinationPolicy. */
      "destinationType": "",     /* The pattern in use by nodes that should receive this object. */
                                 /* If omitted (and if destinationsList is omitted too) the object is broadcast to all known nodes. */
                                 /* Delete this field when you are using policy. */
      "destinationsList": null,  /* The list of destinations as an array of pattern:nodeId pairs that should receive this object. */
                                 /* If provided, destinationType and destinationID must be omitted. */
                                 /* Delete this field when you are using policy. */
      "destinationPolicy": {     /* The policy specification that should be used to distribute this object. */
                                 /* Delete these fields if the target node is using a pattern. */
        "properties": [          /* A list of policy properties that describe the object. */
          {
            "name": "",
            "value": null,
            "type": ""           /* Valid types are string, bool, int, float, list of string (comma separated), version. */
                                 /* Type can be omitted if the type is discernable from the value, e.g. unquoted true is boolean. */
          }
        ],
        "constraints": [         /* A list of constraint expressions of the form <property name> <operator> <property value>, separated by boolean operators AND (&&) or OR (||). */
          ""
        ],
        "services": [            /* The service(s) that will use this object. */
          {
            "orgID": "",         /* The org of the service. */
            "serviceName": "",   /* The name of the service. */
            "arch": "",          /* Set to '*' to indcate services of any hardware architecture. */
            "version": ""        /* A version range. */
          }
        ]
      },
      "expiration": "",          /* A timestamp/date indicating when the object expires (it is automatically deleted). The timestamp should be provided in RFC3339 format.  */
      "version": "",             /* Arbitrary string value. The value is not semantically interpreted. The Model Management System does not keep multiple version of an object. */
      "description": "",         /* An arbitrary description. */
      "activationTime": ""       /* A timestamp/date as to when this object should automatically be activated. The timestamp should be provided in RFC3339 format. */
    }


---
copyright:
years: 2022 - 2023
lastupdated: "2023-01-24"
description: Model Policies deploy application metadata objects to edge nodes
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Model object
{: #model-objects}

Model objects in {{site.data.keyword.edge_notm}} are the metadata representation of application metadata objects.
Applications are often written such that external metadata can be injected into the application in order to alter the behavior of the logic.
This is notably true in the case of machine learning and AI inferencing, where the logic which analyzes a data stream is instructed via a machine learning model how to process the data to derive an analysis result.
{{site.data.keyword.edge_notm}} allows application to be structured using this metadata driven approach, by supporting the ability to deploy the application's metadata on different lifecycle boundaries than the service (inferencing logic) which consumes the metadata.
In {{site.data.keyword.edge_notm}} the application's metadata is called a model, and it is represented by a model object with the JSON serialization shown below.

One aspect of the model object is the policy expressions it contains. The {{site.data.keyword.edge_notm}} policy based, autonomous deployment capability is described [here](./policy.md).
The key to understanding model policy is remember that models are associated with services, and thus follow those services in terms of where the services are deployed.
Model policy expressions are used to further constrain the deployment targets for a given model.

Use the `hzn mms object new` command to generate a skeleton model object file.

Following are the fields in the JSON representation of a model object:

- `objectID`: A unique name for a model object, created by the author of the model object. It must be unique within an {{site.data.keyword.edge_notm}} organization. This field is required.
- `objectType`: A user defined name representing the type of the object. This is used to help the model object author differentiate different kinds of objects. There are no builtin types, nor does the {{site.data.keyword.edge_notm}} runtime use this type for anything other than identifying a model object instance or group of instances. This field is required.
- `destinationOrgID`: The {{site.data.keyword.edge_notm}} organization where the model object will reside. This field is required.
- `destinationID`: The node id (without organization prefix) where the object will be deployed. This field is optional and not recommended. You should delete this field when using `destinationPolicy`.
- `destinationType`: The {{site.data.keyword.edge_notm}} Pattern in use by nodes where this object should be deployed. This field is optional and mutually exclusive with `destinationPolicy`. You should delete this field when using `destinationPolicy`.
- `destinationList`: The list of destinations as an array of pattern:nodeId pairs where the object will be deployed. This field is mutually exclusive with `desinationID` and `destinationPolicy`. You should delete this field when using `destinationPolicy`.
- `destinationPolicy`:
  - `properties`: Policy properties as described [here](./properties_and_constraints.md) which a node policy constraint can refer to.
  - `constraints`: Policy constraints as described [here](./properties_and_constraints.md) which refer to node policy properties.
  - `services`: A list of services which can consume the associated model object.
    - `serviceName`: The name of the service that will consume the model object. This is the same value as found in the `url` field [here](./service_def.md).
    - `orgID` : The organization in which the service in `serviceName` is defined.
    - `version`: A version range indicating the set of service versions with which this model object is associated.
    - `arch`: The hardware architecture of the service in `serviceName`, or `*` to indicate any compatible architecture. This is the same value as found in the `arch` field [here](./service_def.md).
- `expiration`: A timestamp/date indicating when the object expires (it is automatically deleted). The timestamp should be provided in RFC3339 format. This field is optional.
- `version`: An arbitrary string value. The value is not semantically interpreted. The {{site.data.keyword.edge_notm}} Model Management System does not keep multiple versions of an object.
- `description`: A long description of the model object.
- `activationTime`: A timestamp/date as to when this object should automatically be activated. The timestamp should be provided in RFC3339 format. This field is optional.

The following is an example of a model policy.
The model object associated with this policy will be deployed on nodes where the `my.company.com.services.usemodel` service from `serviceOrg` is deployed, as long as the node properties are compatible with the model policy constraint expression. And further, that any node constraint expressions are compatible with this model's properties:

```json
{
  "description": "a long description of the object",
  "objectID": "uniqueString",
  "objectType": "userDefinedTypeName",
  "destinationPolicy": {
    "properties": [
      {
        "name": "prop1",
        "value": "value1",
        "type": "string"
      }
    ],
    "constraints": ["aNodeProperty == someValue"],
    "services": [
      {
        "serviceName": "my.company.com.services.usemodel",
        "orgID": "serviceOrg",
        "version": "1.0.0",
        "arch": "*",
      }
    ]
  },
  "version": "1.0.0"
}
```
{: codeblock}

---
copyright: Contributors to the Open Horizon project
years: 2022 - 2025
title: Policy Properties and Constraints
description: Policy Properties and constraints
lastupdated: 2025-05-03
nav_order: 17
parent: Agent (anax)
---

{:new_window: target="blank"}
{:shortdesc: .shortdesc}
{:screen: .screen}
{:codeblock: .codeblock}
{:pre: .pre}
{:child: .link .ulchildlink}
{:childlinks: .ullinks}

# Policy properties and constraints
{: #policy-props}

Services are deployed to edge nodes by either:

* Assigning a pattern (which defines the services that should be deployed) to a node when the node registers with the Hub, or
* Assigning policy expressions to a node, which can be done at any point in the node's lifecycle. This is where policy properties and constraints become important.

For an overview of the {{site.data.keyword.edge_notm}} policy based deployment system, see this [article](./policy.md).

Properties and constraints are the foundation of the policy expressions used to direct {{site.data.keyword.edge_notm}} workload deployment engine. Remember, workloads in {{site.data.keyword.edge_notm}} are containerized services wrapped in an {{site.data.keyword.edge_notm}} [service definition](./service_def.md). Careful thought should be given to the way in which the edge computing environment will be described by properties and constraints, taking into account various factors such kinds, purpose, and possibly the location of equipment where services will be deployed.

{{site.data.keyword.edge_notm}} provides some `built-in` properties.
For more information, refer to the [built-in properties](./built_in_policy.md) documentation.

## Properties
{: #properties}

Properties are statements of fact about a node, a service implementation, a model, a deployment, and so on.
They are most commonly attached to nodes, enabling constraints in a deployment policy to select the nodes where services should be deployed.

Properties are simple name value pairs, for example, color = red.
In that example, the name of the property is "color" and the value is "red".
Property names can be any valid string value, there are no requirements imposed by {{site.data.keyword.edge_notm}}.
However, in order to avoid name collisions, {{site.data.keyword.edge_notm}} suggests that policy names are created based on a convention that enables the property names to be unique, such as using your domain name or other organizational mechanism, for example mydomain.mycomponent.propertyName.
Notice that the {{site.data.keyword.edge_notm}} [built-in property](./built_in_policy.md) names are all prefixed with `openhorizon`, to distinguish them from user defined properties.

Properties are typed; `string`, `int`, `boolean`, `float`, `version` and `list of strings`, but the type can be omitted from a property definition if the type can be determined by inspecting the specified property value.
When specifying a property value, do so with the property type in mind.
For example, to specify an `int` typed property value, just set the number without quotes.
The `version` type corresponds to the semantic versions used to describe service definitions, for example 1.0.0. Version values are always quoted strings.
The `version` type is distinguished from a `string` because it enables constraints to be expressed on a version that would not be possible if the property type was a string.
The `list of strings` type is a comma separated list of strings, essentially enabling a string typed property to have multiple values.
There is currently no support for custom property types, and there are currently no complex property types.

The JSON representation of a property is:

```json
{
 "name": "property-name-here",
 "type": "property-type-here",
 "value": <property-value here>
}
```
{: codeblock}

The following is an example using each of the property types:

```json
{
 "name": "stringProperty",       /* type is omitted to demonstrate that {{site.data.keyword.edge_notm}} will interpret this property as a string type */
 "value": "string-value"
},
{
 "name": "intProperty",          /* type is omitted to demonstrate that {{site.data.keyword.edge_notm}} will interpret this property as an int type */
 "value": 10
},
{
 "name": "booleanProperty",      /* type is omitted to demonstrate that {{site.data.keyword.edge_notm}} will interpret this property as a string type */
 "value": true
},
{
 "name": "floatProperty",        /* type is omitted to demonstrate that {{site.data.keyword.edge_notm}} will interpret this property as a string type */
 "value": 3.4
},
{
 "name": "versionProperty",      /* type is specified to demonstrate that {{site.data.keyword.edge_notm}} would otherwise interpret this property as a string */
 "type": "version",
 "value": "1.2.7"
},
{
 "name": "losProperty",          /* type is specified to demonstrate that {{site.data.keyword.edge_notm}} would otherwise interpret this property as a string */
 "type": "list of strings",
 "value": "value1,value2"
}
```
{: codeblock}

## Constraints
{: #constraints}

Constraints are the other half of the policy expression language in {{site.data.keyword.edge_notm}}.
Think of constraints as selection predicates used to select nodes based on the properties defined and set on those nodes.
Constraints are most commonly found in deployment policies but can be specified on node, service and model policy.

Constraints are specified using a simple text based language.
The lexical parser used to interpret the language is described by an extended Backus-Naur form (EBNF) in a function called getLexer() in [the code ](https://github.com/open-horizon/anax/blob/master/externalpolicy/text_language/text_language.go){:target="_blank"}{: .externalLink}.
The language allows property name references and their expected values to be strung together with Boolean operators `AND` and `OR` into Boolean expressions.
The more golang-like Boolean operators (`&&` and `||`) are also supported.
Parentheses are supported in order to create evaluation precedence.
The Boolean operator `NOT` is not currently supported in constraint expressions.

When a constraint expression is evaluated against a list of properties, the result will be either true or false.
True means that the constraint is compatible with the property list, false means it is not compatible.

For example, using the example properties defined in the properties section above, the constraint:
`"booleanProperty = true AND floatProperty < 1.0"` will evaluate to false.

Each property type has operators that can be used to evaluate property values:

* `string` - the operators `==` or `=` denote equals to and `!=` denotes not equal to.
* `int` - supports the operators `==, <, >, <=, >=, =, !=`.
* `boolean` - supports `==, =`
* `float` - supports the operators `==, <, >, <=, >=, =, !=`.
* `version` - supports `==, =, in` where `in` is used to indicate that a version is within a given range, for example any version 1 service is specified as: "[1.0.0,2.0.0)".
* `list of strings` - supports `in` where the property has one of the values specified in the constraint.

The JSON representation of a constraint is:

```json
[
 "<constraint-expression-one>",
 "<constraint-expression-two>", ...
]
```
{: codeblock}

Constraint expressions that appears in a list are logically ANDed together to produce a single true or false result.

#!/bin/bash

ANAX_API=http://localhost
PROP_NAME=$1
PROP_VALUE=$2

# If there is already a node level Property object, then just update it with our property.
ATTRS=$(curl -sS -X GET -H "Content-Type: application/json" "$ANAX_API/attribute")

PROP=$(echo $ATTRS | jq -r '.attributes[] | select (.type == "CounterPartyPropertyAttributes")')

# If there is no counter party property attribute, create one.
if [ "$PROP" == "" ]; then

    # Then set a node level counter party property
    read -d '' propattribute <<EOF
{
  "type": "CounterPartyPropertyAttributes",
  "label": "A required property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "expression": {
      "and": [
        {"name":"$PROP_NAME", "op":"==", "value":"$PROP_VALUE"}
      ]
    }
  }
}
EOF

    echo -e "\n\n[D] counter party property payload: $propattribute"

    echo "Creating node counter party property attribute with $PROP_NAME=$PROP_VALUE"

    ERR=$(echo "$propattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

# Otherwise we need to update the existing counter party property object.
else

    ID=$(echo $PROP | jq -r '.id')

    # If the property we want to add is already there, just update it.
    existing=$(echo $PROP | jq '(.mappings.expression.and[] | select (.name == "'$PROP_NAME'") | .value)')
    if [ "$existing" != "" ]; then

        NEW_MAPPINGS=$(echo $PROP | jq --arg val "$PROP_VALUE" '(.mappings.expression.and[] | select (.name == "'$PROP_NAME'") | .value) |= $val')

        read -d '' propattribute <<EOF
$NEW_MAPPINGS
EOF

        echo -e "\n\n[D] counter party property payload: $propattribute"

        echo "Updating node counter party property $PROP_NAME=$PROP_VALUE"

        ERR=$(echo "$propattribute" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/attribute/$ID" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

    else
        # Add a new property to the expression
        echo "Add a new property"

        NEW_MAPPINGS=$(echo $PROP | jq '.mappings.expression.and[.mappings.expression.and| length] |= . + {"name":"'$PROP_NAME'","op":"=","value":"'$PROP_VALUE'"}')
        read -d '' propattribute <<EOF
$NEW_MAPPINGS
EOF

        echo -e "\n\n[D] counter party property payload: $propattribute"

        echo "Adding node counter party property $PROP_NAME=$PROP_VALUE"

        ERR=$(echo "$propattribute" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/attribute/$ID" | jq -r '.error')
        if [ "$ERR" != "null" ]; then
            echo -e "error occured: $ERR"
            exit 2
        fi

    fi

fi
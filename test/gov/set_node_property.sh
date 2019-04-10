#!/bin/bash

ANAX_API=http://localhost
PROP_NAME=$1
PROP_VALUE=$2

# If there is already a node level Property object, then just update it with our property.
ATTRS=$(curl -sS -X GET -H "Content-Type: application/json" "$ANAX_API/attribute")

PROP=$(echo $ATTRS | jq -r '.attributes[] | select (.type == "PropertyAttributes")')

# If there is no property attribute, create one.
if [ "$PROP" == "" ]; then

    # Then set a node level property
    read -d '' propattribute <<EOF
{
  "type": "PropertyAttributes",
  "label": "A property",
  "publishable": true,
  "host_only": false,
  "mappings": {
    "$PROP_NAME": "$PROP_VALUE"
  }
}
EOF

    echo -e "\n\n[D] property payload: $propattribute"

    echo "Creating node property attribute with $PROP_NAME=$PROP_VALUE"

    ERR=$(echo "$propattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
      echo -e "error occured: $ERR"
      exit 2
    fi

# Otherwise we need to update the existing property object.
else

	  ID=$(echo $PROP | jq -r '.id')

    # The specific property we're updating might already be present

    NEW_MAPPINGS=$(echo $PROP | jq --arg val "$PROP_VALUE" '.mappings + {'$PROP_NAME': $val}')

    read -d '' propattribute <<EOF
{
  "type": "PropertyAttributes",
  "label": "A property",
  "publishable": true,
  "host_only": false,
  "mappings": $NEW_MAPPINGS
}
EOF

    echo -e "\n\n[D] property payload: $propattribute"

    echo "Updating node property $PROP_NAME=$PROP_VALUE"

    ERR=$(echo "$propattribute" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/attribute/$ID" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

fi
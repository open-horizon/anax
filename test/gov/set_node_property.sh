#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

ANAX_API=http://localhost:8510
PROP_NAME=$1
PROP_VALUE=$2

# If there is already a node level Property object, then just update it with our property.
ATTRS=$(curl -sS -X GET -H "Content-Type: application/json" "$ANAX_API/attribute")

PROP=$(echo "$ATTRS" | jq -r '.attributes[] | select (.type == "PropertyAttributes")')

# If there is no property attribute, create one.
if [ "$PROP" = "" ]; then

    # Then set a node level property
    cat > /tmp/propattribute.tmp <<'EOF'
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
    propattribute=$(cat /tmp/propattribute.tmp)

    echo -e "\n\n[D] property payload: $propattribute"

    echo "Creating node property attribute with $PROP_NAME=$PROP_VALUE"

    ERR=$(echo "$propattribute" | curl -sS -X POST -H "Content-Type: application/json" --data @- "$ANAX_API/attribute" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
      echo -e "error occured: $ERR"
      exit 2
    fi

# Otherwise we need to update the existing property object.
else

	  ID=$(echo "$PROP" | jq -r '.id')

    # The specific property we're updating might already be present

    NEW_MAPPINGS=$(echo "$PROP" | jq --arg key "$PROP_NAME" --arg val "$PROP_VALUE" '.mappings + {($key): $val}')

    cat > /tmp/propattribute.tmp <<'EOF'
{
  "type": "PropertyAttributes",
  "label": "A property",
  "publishable": true,
  "host_only": false,
  "mappings": $NEW_MAPPINGS
}
EOF
    propattribute=$(cat /tmp/propattribute.tmp)

    echo -e "\n\n[D] property payload: $propattribute"

    echo "Updating node property $PROP_NAME=$PROP_VALUE"

    ERR=$(echo "$propattribute" | curl -sS -X PUT -H "Content-Type: application/json" --data @- "$ANAX_API/attribute/$ID" | jq -r '.error')
    if [ "$ERR" != "null" ]; then
        echo -e "error occured: $ERR"
        exit 2
    fi

fi
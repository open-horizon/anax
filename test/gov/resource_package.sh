#!/bin/bash

# tar up the raw resource files used in the test services.
echo "Building resource packages."

cd /root/resources

RESOURCE_ORG=e2edev
RESOURCE_TYPE=model

# For each directory in the resources folder, make zipped tarball of directory contents and then register the resources in the Cloud side sync service (CSS).
for dir in */; do
	# Remove the trailing slash from the directory name
	justDirName=${dir%"/"}
	echo "Making resource tarball for ${justDirName}"
	cd $justDirName
	# Find all the files in the directory, excluding the . directory and any gzipped tarball
	# that might be there. This allows us to run the script over and over without causing damaged tarballs.
	res=$(find . -not -name "*.tgz" -not -path ".")
	tar -czvf $justDirName.tgz $res

	echo "Installing resource package ${justDirName}.tgz."

read -d '' resmeta <<EOF
{
  "data": [],
  "meta": {
  	"objectID": "${justDirName}",
  	"objectType": "$RESOURCE_TYPE",
  	"version": "1.0.0",
  	"description": "A JSON configuration file tarball."
  }
}
EOF

	ADDM=$(echo "$resmeta" | curl -sLX PUT "http://css-api:8500/api/v1/objects/${RESOURCE_ORG}/${RESOURCE_TYPE}/${justDirName}" --data @-)

	if [ "$ADDM" == "" ]
	then
		echo -e "$resmeta \nadded successfully"
	else
		echo -e "$resmeta \nPUT returned:"
	 	echo $ADDM
	fi

	ADDF=$(curl -sLX PUT --header 'Content-Type:application/octet-stream' "http://css-api:8500/api/v1/objects/${RESOURCE_ORG}/${RESOURCE_TYPE}/${justDirName}/data" --data-binary @${justDirName}.tgz)

	if [ "$ADDF" == "" ]
	then
		echo -e "Data file ${justDirName}.tgz added successfully"
	else
		echo -e "Data file PUT returned:"
	 	echo $ADDF
	fi


	cd ..
done
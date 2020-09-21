#!/bin/bash

# tar up the raw resource files used in the test services.
echo "Building resource packages."

EXEC_DIR=$PWD
cd /root/resources/private

RESOURCE_ORG1=e2edev@somecomp.com
RESOURCE_ORG2=userdev
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

	if [ "${TEST_PATTERNS}" != "" ]
	then
		$EXEC_DIR/deploy_file.sh /root/resources/${dir}${justDirName}.tgz 1.0.0 ${RESOURCE_ORG1} ${RESOURCE_TYPE} none none none false
		if [ $? -ne 0 ]
		then
			exit -1
		fi

		$EXEC_DIR/deploy_file.sh /root/resources/${dir}${justDirName}.tgz 1.0.0 ${RESOURCE_ORG2} ${RESOURCE_TYPE} none none none false
		if [ $? -ne 0 ]
		then
			exit -1
		fi
	else

		$EXEC_DIR/deploy_file.sh /root/resources/${dir}${justDirName}.tgz 1.0.0 ${RESOURCE_ORG1} ${RESOURCE_TYPE} none none "$(cat /root/objects/${justDirName}.policy)" false
		if [ $? -ne 0 ]
		then
			exit -1
		fi

		$EXEC_DIR/deploy_file.sh /root/resources/${dir}${justDirName}.tgz 1.0.0 ${RESOURCE_ORG2} ${RESOURCE_TYPE} none none "$(cat /root/objects/${justDirName}.policy)" false
		if [ $? -ne 0 ]
		then
			exit -1
		fi
	fi

	cd ..
done

echo "Making resource tarball for public resource"
cd /root/resources/public
RESOURCE_ORG=IBM
RESOURCE_TYPE=public

res=$(find . -not -name "*.tgz" -not -path ".")
tar -czvf public.tgz $res

echo "Installing resource package public.tgz. in ${RESOURCE_ORG} org"
ls /root/resources/public
$EXEC_DIR/deploy_file.sh /root/resources/public/public.tgz 1.0.0 ${RESOURCE_ORG} ${RESOURCE_TYPE} none none none true
if [ $? -ne 0 ]
then
	exit -1
fi

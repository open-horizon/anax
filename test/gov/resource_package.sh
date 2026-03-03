#!/bin/bash

# Enable debug tracing when DEBUG=1 or RUNNER_DEBUG=1 (GitHub Actions debug mode).
if [ "${DEBUG:-0}" = "1" ] || [ "${RUNNER_DEBUG:-0}" = "1" ]; then
    set -x
fi

# tar up the raw resource files used in the test services.
echo "Building resource packages."

EXEC_DIR=$PWD
cd "${EXEC_DIR}/docker/fs/resources/private" || { echo "Error: resource_package.sh - ln 7 - Failure to change directories"; error 1; }

RESOURCE_ORG1="e2edev@somecomp.com"
RESOURCE_ORG2="userdev"
RESOURCE_TYPE="model"

# For each directory in the resources folder, make zipped tarball of directory contents and then register the resources in the Cloud side sync service (CSS).
for dir in */; do
	# Remove the trailing slash from the directory name
	justDirName=${dir%"/"}
	echo "Making resource tarball for ${justDirName}"
	cd "$justDirName" || { echo "Error: resource_package.sh - ln 18 - Failure to change directories"; error 1; }
	# Find all the files in the directory, excluding the . directory and any gzipped tarball
	# that might be there. This allows us to run the script over and over without causing damaged tarballs.
	res=$(find . -not -name "*.tgz" -not -path ".")
	tar -czvf "$justDirName".tgz "$res"

	echo "Installing resource package ${justDirName}.tgz."

	if [ "${TEST_PATTERNS}" != "" ]
	then
		if ! "${EXEC_DIR}/gov/deploy_file.sh" "${EXEC_DIR}/docker/fs/resources/private/${dir}${justDirName}.tgz" 1.0.0 "${RESOURCE_ORG1}" "${RESOURCE_TYPE}" none none none false
		then
			exit 255
		fi

		if ! "${EXEC_DIR}/gov/deploy_file.sh" "${EXEC_DIR}/docker/fs/resources/private/${dir}${justDirName}.tgz" 1.0.0 "${RESOURCE_ORG2}" "${RESOURCE_TYPE}" none none none false
		then
			exit 255
		fi
	else
		# Create policy files from templates by passing current ARCH to them
		for in_file in "${EXEC_DIR}"/docker/fs/objects/*.policy
		do
			if ! sed -i -e "s#__ARCH__#${ARCH}#g" "$in_file"
			then
				exit 255
			fi
		done

		if ! "${EXEC_DIR}/gov/deploy_file.sh" "${EXEC_DIR}/docker/fs/resources/private/${dir}${justDirName}.tgz" 1.0.0 "${RESOURCE_ORG1}" "${RESOURCE_TYPE}" none none "$(cat "${EXEC_DIR}/docker/fs/objects/${justDirName}".policy)" false
		then
			exit 255
		fi

		if ! "${EXEC_DIR}/gov/deploy_file.sh" "${EXEC_DIR}/docker/fs/resources/private/${dir}${justDirName}.tgz" 1.0.0 "${RESOURCE_ORG2}" "${RESOURCE_TYPE}" none none "$(cat "${EXEC_DIR}/docker/fs/objects/${justDirName}".policy)" false
		then
			exit 255
		fi
	fi

	cd .. || exit
done

echo "Making resource tarball for public resource"
cd "${EXEC_DIR}/docker/fs/resources/public" || { echo "Error: resource_package.sh - ln 64 - Failure to change directories"; error 1; }
RESOURCE_ORG=IBM
RESOURCE_TYPE=public

res=$(find . -not -name "*.tgz" -not -path ".")
tar -czvf public.tgz "$res"

echo "Installing resource package public.tgz. in ${RESOURCE_ORG} org"
ls "${EXEC_DIR}/docker/fs/resources/public"
if ! "${EXEC_DIR}/gov/deploy_file.sh" "${EXEC_DIR}/docker/fs/resources/public/public.tgz" 1.0.0 "${RESOURCE_ORG}" "${RESOURCE_TYPE}" none none none true
then
	exit 255
fi

cd "${EXEC_DIR}" || { echo "Error: resource_package.sh - ln 78 - Failure to change directories"; error 1; }

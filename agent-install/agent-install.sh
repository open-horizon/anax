#!/bin/bash

# The script installs Horizon agent on an edge node

set -e

SUPPORTED_OS=( "macos" "linux" )
SUPPORTED_LINUX_DISTRO=( "ubuntu" "raspbian" )
SUPPORTED_LINUX_VERSION=( "bionic" "xenial" "stretch" )
SUPPORTED_ARCH=( "amd64" "arm64" "armhf" "ppc64el" )

# Defaults
PKG_PATH="."
PKG_TREE_IGNORE=false
SKIP_REGISTRATION=false
CFG="agent-install.cfg"
OVERWRITE=false

# required parameters and their defaults
REQUIRED_PARAMS=( "CERTIFICATE" "HZN_EXCHANGE_URL" "HZN_FSS_CSSURL" "HZN_ORG_ID" )
REQUIRED_VALUE_FLAG="REQUIRED_FROM_USER"
DEFAULTS=( "agent-install.crt" "${REQUIRED_VALUE_FLAG}" "${REQUIRED_VALUE_FLAG}" "${REQUIRED_VALUE_FLAG}" )

# certificate for the CLI package on MacOS
MAC_PACKAGE_CERT="horizon-cli.crt"

# Script help
function help() {
     cat << EndOfMessage
$(basename "$0") <options> -- installing Horizon software
where:
    \$HZN_EXCHANGE_URL, \$HZN_FSS_CSSURL, \$HZN_ORG_ID, \$HZN_EXCHANGE_USER_AUTH variables must be defined either in a config file or environment, 

    -c          - path to a certificate file
    -k          - path to a configuration file (if not specified, uses agent-install.cfg in current directory, if present)
    -p          - pattern name to register with (if not specified, registers node w/o pattern)
    -i          - installation packages location (if not specified, uses current directory)
    -s          - skip registration

Example: ./$(basename "$0") -i <path_to_package(s)>
	
EndOfMessage

quit 1
}

# Exit handling
function quit(){
  case $1 in
    1) echo -e "Exiting..."; exit 1
    ;;
    2) echo -e "Input error, exiting..."; exit 2
    ;;
    *) exit
    ;;
  esac
}

function now() {
	echo `date '+%Y-%m-%d %H:%M:%S'`
}

# get variables for the script
# if the env variable is defined uses it, if not checks it in the config file
function get_variable() {
	
	if ! [ -z "${!1}" ]; then
		# if env variable is defined, using it
		if [[ $1 == *"AUTH"* ]]; then
			echo `now` "Using variable from environment, ${1}"
		else
			echo `now` "Using variable from environment, ${1} is ${!1}"
		fi
	else
		echo `now` "The ${1} is missed in environment, looking for it in the config file ${2} ..."
		# the env variable not defined, using config file
		# check if it exists
		echo `now` "Checking if the config file ${2} exists..."
		if [[ -f "$2" ]] ; then
			echo `now` "The config file ${2} exists"
			if [ -z "$(grep ${1} ${2} | grep "^#")" ] && ! [ -z "$(grep ${1} ${2} | cut -d'=' -f2 | cut -d'"' -f2)" ]; then
				# found variable in the config file
				ref=${1}
				IFS= read -r "$ref" <<<"$(grep ${1} ${2} | cut -d'=' -f2 | cut -d'"' -f2)"
                if [[ $1 == *"AUTH"* ]]; then
                    echo `now` "Using variable from the config file ${2}, ${1}"
                else
				    echo `now` "Using variable from the config file ${2}, ${1} is ${!1}"
                fi
			else
				# found neither in env nor in config file. check if the missed var is in required parameters
				if [[ " ${REQUIRED_PARAMS[*]} " == *" ${1} "* ]]; then
    				# if found neither in the env nor in the env, try to use its default value, if any
    				echo `now` "The required variable ${1} found neither in environment nor in the config file ${2}, checking if it has defaults..."

    				for i in "${!REQUIRED_PARAMS[@]}"; do
   						if [[ "${REQUIRED_PARAMS[$i]}" = "${1}" ]]; then
       							echo `now` "Found ${1} in required params with index ${i}, using it for looking up its default value...";
       							echo `now` "Found ${1} default, it is ${DEFAULTS[i]}"
       							ref=${1}
								IFS= read -r "$ref" <<<"${DEFAULTS[i]}"
   						fi
					done
					if [ ${!1}  = "$REQUIRED_VALUE_FLAG" ]; then
						echo `now` "The ${1} is required and needs to be set either in the config file or environment, exiting..."
						exit 1
					fi
    			else
    				echo `now` "The variable ${1} found neither in environment nor in the config file ${2}, but it's not required, continuing..."
				fi
			fi
		else
			echo `now` "The config file ${2} doesn't exist, exiting..."
			exit 1
		fi
	fi
}

# checks input arguments and env variables specified
function validate_args(){
    echo `now` "Checking script arguments..."

    # preliminary check for script arguments
    check_empty "$PKG_PATH" "path to installation packages"
    check_exist d "$PKG_PATH" "The package installation"
    check_empty "$SKIP_REGISTRATION" "registration flag"
    echo `now` "Check finished successfully"

    echo `now` "Checking configuration..."
    # read and validate configuration
    get_variable HZN_EXCHANGE_URL $CFG
    check_empty HZN_EXCHANGE_URL "Exchange URL"
    get_variable HZN_FSS_CSSURL $CFG
    check_empty HZN_FSS_CSSURL "FSS_CSS URL"
    get_variable HZN_ORG_ID $CFG
    check_empty HZN_ORG_ID "ORG ID"
    get_variable HZN_EXCHANGE_USER_AUTH $CFG
    check_empty HZN_EXCHANGE_USER_AUTH "Exchange User Auth"
    get_variable HZN_EXCHANGE_PATTERN $CFG

    get_variable CERTIFICATE $CFG
    check_exist f "$CERTIFICATE" "The certificate"

    echo `now` "Check finished successfully"
}

function show_config() {
    echo "Current configuration:"
    echo "Certification file: ${CERTIFICATE}"
    echo "Configuration file: ${CFG}"
    echo "Installation packages location: ${PKG_PATH}"
    echo "Ignore package tree: ${PKG_TREE_IGNORE}"
    echo "Pattern name: ${HZN_EXCHANGE_PATTERN}"
    echo "Skip registration: ${SKIP_REGISTRATION}"
    echo "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}"
    echo "HZN_FSS_CSSURL=${HZN_FSS_CSSURL}"
    echo "HZN_ORG_ID=${HZN_ORG_ID}"
    echo "HZN_EXCHANGE_USER_AUTH=<specified>"
}

function check_installed() {
    if command -v "$1" >/dev/null 2>&1; then
        echo `now` "${2} is installed"
    else
        echo `now` "${2} not found, please install it"
        quit 1
    fi
}

# compare versions
function version_gt() {
	test "$(printf '%s\n' "$@" | sort -V | head -n 1)" != "$1"; 
}

function install_macos() {
    echo `now` "install_macos() begin"
    
    echo `now` "Installing agent on ${OS}..."
    
    echo `now` "Checking ${OS} specific prerequisites..."
    check_installed "socat" "socat"
    check_installed "docker" "Docker"
    check_installed "wget" "wget"
    check_installed "jq" "jq"
    
    # Setting up a certificate
    echo `now` "Importing the horizon-cli package certificate into Mac OS keychain..."
    set -x
    #wget http://pkg.bluehorizon.network/macos/certs/horizon-cli.crt
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain ${PACKAGES}/${MAC_PACKAGE_CERT}
    #rm -f horizon-cli.crt
    set +x
    echo `now` "Configuring an edge node to trust the ICP certificate..."
    set -x
    sudo security add-trusted-cert -d -r trustRoot -k /Library/Keychains/System.keychain "$CERTIFICATE"
    set +x

	echo `now` "Detecting packages version..."
	PACKAGE_VERSION=$(ls ${PACKAGES} | grep horizon-cli | cut -d'-' -f3 | cut -d'.' -f1-3)
	echo `now` "The packages version is ${PACKAGE_VERSION}"
	echo `now` "Setting up the agent container tag on Mac..."
    export HC_DOCKER_TAG=$PACKAGE_VERSION
    echo `now` "HC_DOCKER_TAG is ${HC_DOCKER_TAG}"

    echo `now` "Checking if hzn is installed..."
    if command -v hzn >/dev/null 2>&1; then
    	# if hzn is installed, need to check the current setup
		echo `now` "hzn found, checking setup..."
		AGENT_VERSION=$(hzn version | grep "^Horizon Agent" | sed 's/^.*: //')
		echo `now` "Found Agent version is ${AGENT_VERSION}"
		re='^[0-9]+([.][0-9]+)+([.][0-9]+)'
		if ! [[ $AGENT_VERSION =~ $re ]] ; then
			echo `now` "Something's wrong. Can't get the agent verison, installing it..."
			set -x
	        sudo installer -pkg ${PACKAGES}/horizon-cli-*.pkg -target /
	        set +x
		else
			# compare version for installing and what we have
			echo `now` "Comparing agent and packages versions..."
			if [ "$AGENT_VERSION" = "$PACKAGE_VERSION" ]; then
				echo `now` "Versions are equal: agent is ${AGENT_VERSION} and packages are ${PACKAGE_VERSION}. Don't need to install"
			else
				if version_gt $AGENT_VERSION $PACKAGE_VERSION; then
					echo `now` "Installed agent ${AGENT_VERSION} is newer than the packages ${PACKAGE_VERSION}"
					if [ ! "$OVERWRITE" = true ] ; then
						echo "The installed agent is newer than one you're trying to install, continue?[y/N]:"
						read RESPONSE
						if [ ! "$RESPONSE" == 'y' ]; then
							echo "Exiting at users request"
							exit
						fi
					fi
					echo `now` "Installing older packages ${PACKAGE_VERSION}..."
					set -x
        			sudo installer -pkg ${PACKAGES}/horizon-cli-*.pkg -target /
        			set +x
				else
					echo `now` "Installed agent is ${AGENT_VERSION}, package is ${PACKAGE_VERSION}"
					echo `now` "Installing newer package (${PACKAGE_VERSION}) ..."
					set -x
        			sudo installer -pkg ${PACKAGES}/horizon-cli-*.pkg -target /
        			set +x
				fi
			fi
		fi		
	else
        echo `now` "hzn not found, installing it..." 
        set -x
        sudo installer -pkg ${PACKAGES}/horizon-cli-*.pkg -target /
        set +x
	fi

    # check if the horizon-container script exists
    if command -v horizon-container >/dev/null 2>&1; then
		# horizon-container script is installed
        if ! [[ -z $(docker ps -q --filter name=horizon1) ]]; then
			# there is a running container
			if [ ! "$OVERWRITE" = true ] ; then
				echo "Do you want to stop the Horizon services container (the stop unregisters the node)?[y/N]:"
				read RESPONSE
				if [ ! "$RESPONSE" == 'y' ]; then
					echo `now` "Continue without stopping the Horizon services container..."
				else
					echo `now` "Stopping the Horizon services container...."
					set -x
            		horizon-container stop
            		set +x
				fi
			else
				# overwriting current configuration
				echo `now` "Stopping the Horizon services container...."
				set -x
            	horizon-container stop
            	set +x
			fi
           
        fi       
	else
        echo `now` "horizon-container not found, hzn is not installed or its installation is broken, exiting..." 
        exit 1
	fi

    # configuring agent inside the container
    HZN_CONFIG=/etc/default/horizon
    echo `now` "Configuring ${HZN_CONFIG} file for the agent container..."
    HZN_CONFIG_DIR=$(dirname "${HZN_CONFIG}")
    if ! [[ -f "$HZN_CONFIG" ]] ; then
	    echo `now` "$HZN_CONFIG file doesn't exist, creating..."
	    # check if the directory exists
	    if ! [[ -d "$(dirname "${HZN_CONFIG}")" ]] ; then
		    echo `now` "The directory ${HZN_CONFIG_DIR} doesn't exist, creating..."
            set -x
		    sudo mkdir -p "$HZN_CONFIG_DIR"
            set +x
	    fi
	    echo `now` "Creating ${HZN_CONFIG} file..."
        set -x
	    echo -e "HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL} \nHZN_FSS_CSSURL=${HZN_FSS_CSSURL} \
			\nHZN_DEVICE_ID=${HOSTNAME}" | sudo tee "$HZN_CONFIG"
        set +x
        echo `now` "Config created"
    else
        if [[ ! -z "${HZN_EXCHANGE_URL}" ]] && [[ ! -z "${HZN_FSS_CSSURL}" ]]; then
                echo `now` "Found environment variables HZN_EXCHANGE_URL and HZN_FSS_CSSURL, updating horizon config..."
                set -x
                sudo sed -i.bak -e "s~^HZN_EXCHANGE_URL=[^ ]*~HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}~g" \
                    -e "s~^HZN_FSS_CSSURL=[^ ]*~HZN_FSS_CSSURL=${HZN_FSS_CSSURL}~g" "$HZN_CONFIG"
                set +x
                echo `now` "Config updated"
        fi
    fi

    CONFIG_MAC=~/.hzn/hzn.json
    echo `now` "Configuring hzn..."
    if [[ ! -z "${HZN_EXCHANGE_URL}" ]] && [[ ! -z "${HZN_FSS_CSSURL}" ]]; then
        if [[ -f "$CONFIG_MAC" ]]; then
	        echo `now` "${CONFIG_MAC} config file exists, updating..."
            set -x
	        sed -i.bak -e "s|\"HZN_EXCHANGE_URL\": \"[^ ]*\",|\"HZN_EXCHANGE_URL\": \""$HZN_EXCHANGE_URL"\",|" \
			   -e "s|\"HZN_FSS_CSSURL\": \"[^ ]*\"|\"HZN_FSS_CSSURL\": \""$HZN_FSS_CSSURL"\"|" \
	            "$CONFIG_MAC"
            set +x
            echo `now` "Config updated"
        else
	        echo "${CONFIG_MAC} file doesn't exist, creating..."
            set -x
	        echo -e "{\n  \"HZN_EXCHANGE_URL\": \""$HZN_EXCHANGE_URL"\",\n  \"HZN_FSS_CSSURL\": \""$HZN_FSS_CSSURL"\"\n}" > "$CONFIG_MAC"
            set +x
            echo `now` "Config created"
        fi
    fi

	if [[ -z $(docker ps -q --filter name=horizon1) ]]; then
		# container is not running, starting it
	    echo `now` "Making ICP certificate available to the agent container..."
	    export icpCertMount="-v $(pwd)/${CERTIFICATE}:/etc/ssl/certs/icp.crt"
	    echo `now` "ICP Cert Mount is ${icpCertMount}"
	    echo `now` "Starting horizon services..."
	    set -x
	    horizon-container start
	    set +x

	   	start_horizon_container_check=`date +%s`
	    
	    while [ -z "$(docker exec -ti horizon1 curl http://localhost/status | jq -r .configuration.exchange_version)" ] ; do
	    	current_horizon_container_check=`date +%s`
			echo `now` "the horizon-container with anax is not ready, retry in 10 seconds"
			if (( current_horizon_container_check - start_horizon_container_check > 60 )); then
				echo `now` "horizon container timeout of 60 seconds occured"
				exit 1
			fi
			sleep 10
		done

		echo `now` "The horizon-container is ready"
	fi

	process_node

    echo `now` "install_macos() end"
}

function install_linux(){
    echo `now` "install_linux() begin"
    echo `now` "Installing agent on ${DISTRO}, version ${CODENAME}, architecture ${ARCH}"

    # Configure certificates 
    echo `now` "Configuring an edge node to trust the certificate..."
    set -x
    sudo cp "$CERTIFICATE" /usr/local/share/ca-certificates && sudo update-ca-certificates
    set +x

    ANAX_PORT=8510

    if [[ "$OS" == "linux" ]]; then
        if [ -f /etc/default/horizon ]; then
            echo `now` "Getting agent port from /etc/default/horizon file..."
            anaxPort=$(grep HZN_AGENT_PORT /etc/default/horizon |cut -d'=' -f2)
            if [[ "$anaxPort" == "" ]]; then
                echo `now` "Cannot detect agent port as /etc/default/horizon does not contain HZN_AGENT_PORT, using ${ANAX_PORT} instead"
            else
                ANAX_PORT=$anaxPort
            fi
        else
            echo `now` "Cannot detect agent port as /etc/default/horizon cannot be found, using ${ANAX_PORT} instead"
        fi
    fi

	echo `now` "Checking if the agent port ${ANAX_PORT} is free..."
	if [ ! -z "$(netstat -nlp | grep :$ANAX_PORT)" ]; then
		echo `now` "Something is running on ${ANAX_PORT}..."
		if [ -z "$(netstat -nlp | grep :$ANAX_PORT | grep anax)" ]; then
			echo `now` "It's not anax, please free the port in order to install horizon, exiting..."
			netstat -nlp | grep :$ANAX_PORT
			exit 1
		else
			echo "It's anax, continuing..."
			netstat -nlp | grep :$ANAX_PORT
		fi
	else
		echo "Anax port ${ANAX_PORT} is free, continuing..."
	fi
    
    echo `now` "Updating OS..."
    set -x
    apt update
    set +x
    echo `now` "Checking if curl is installed..."
    if command -v curl >/dev/null 2>&1; then
		echo `now` "curl found"
	else
        echo `now` "curl not found, installing it..." 
        set -x
        apt install -y curl
        set +x
        echo `now` "curl installed"
	fi

	if command -v jq >/dev/null 2>&1; then
		echo `now` "jq found"
	else
        echo `now` "jq not found, installing it..."
        set -x
        apt install -y jq
        set +x
        echo `now` "jq installed"
	fi

    echo `now` "Checking if Docker is installed..."
    if command -v docker >/dev/null 2>&1; then
		echo `now` "Docker found"
	else
        echo `now` "Docker not found, installing it..." 
        set -x
        curl -fsSL get.docker.com | sh
        set +x
	fi

    echo `now` "Checking if hzn is installed..."
    if command -v hzn >/dev/null 2>&1; then
    	# if hzn is installed, need to check the current setup
		echo `now` "hzn found, checking setup..."
		AGENT_VERSION=$(hzn version | grep "^Horizon Agent" | sed 's/^.*: //')
		echo `now` "Found Agent version is ${AGENT_VERSION}"
		re='^[0-9]+([.][0-9]+)+([.][0-9]+)'
		if ! [[ $AGENT_VERSION =~ $re ]] ; then
			echo `now` "Something's wrong. Can't get the agent verison, installing it..."
			set -x
	        set +e
	        dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
	        set -e
	        set +x
        	echo `now` "Resolving any dependency errors..."
        	set -x
        	apt update && apt-get install -y -f
        	set +x
		else
			# compare version for installing and what we have
			PACKAGE_VERSION=$(ls ${PACKAGES} | grep horizon-cli | cut -d'_' -f2 | cut -d'~' -f1)
			echo `now` "The packages version is ${PACKAGE_VERSION}"
			echo `now` "Comparing agent and packages versions..."
			if [ "$AGENT_VERSION" = "$PACKAGE_VERSION" ]; then
				echo `now` "Versions are equal: agent is ${AGENT_VERSION} and packages are ${PACKAGE_VERSION}. Don't need to install"
			else
				if version_gt $AGENT_VERSION $PACKAGE_VERSION; then
					echo `now` "Installed agent ${AGENT_VERSION} is newer than the packages ${PACKAGE_VERSION}"
					if [ ! "$OVERWRITE" = true ] ; then
						echo "The installed agent is newer than one you're trying to install, continue?[y/N]:"
						read RESPONSE
						if [ ! "$RESPONSE" == 'y' ]; then
							echo "Exiting at users request"
							exit
						fi
					fi
					echo `now` "Installing older packages ${PACKAGE_VERSION}..."
					set -x
		        	set +e
		        	dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
		        	set -e
		        	set +x
		        	echo `now` "Resolving any dependency errors..."
		        	set -x
		        	apt update && apt-get install -y -f
		        	set +x
				else
					echo `now` "Installed agent is ${AGENT_VERSION}, package is ${PACKAGE_VERSION}"
					echo `now` "Installing newer package (${PACKAGE_VERSION}) ..."
					set -x
		        	set +e
		        	dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
		        	set -e
		        	set +x
		        	echo `now` "Resolving any dependency errors..."
		        	set -x
		        	apt update && apt-get install -y -f
		        	set +x
				fi
			fi
		fi
	else
        echo `now` "hzn not found, installing it..." 
        set -x
        set +e
        dpkg -i ${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb
        set -e
        set +x
        echo `now` "Resolving any dependency errors..."
        set -x
        apt update && apt-get install -y -f
        set +x
	fi
   
    check_exist f "/etc/default/horizon" "horizon configuration"
    # The /etc/default/horizon creates upon horizon deb packages installation
    if [[ ! -z "${HZN_EXCHANGE_URL}" ]] && [[ ! -z "${HZN_FSS_CSSURL}" ]]; then
        echo `now` "Found variables HZN_EXCHANGE_URL and HZN_FSS_CSSURL, updating horizon config..."
        set -x
        sed -i.bak -e "s~^HZN_EXCHANGE_URL=[^ ]*~HZN_EXCHANGE_URL=${HZN_EXCHANGE_URL}~g" \
	        -e "s~^HZN_FSS_CSSURL=[^ ]*~HZN_FSS_CSSURL=${HZN_FSS_CSSURL}~g" /etc/default/horizon
        set +x
        echo `now` "Config updated"
    fi

    echo `now` "Restarting the service..."
    set -x
    systemctl restart horizon.service
    set +x
    
    start_anax_service_check=`date +%s`

   	while [ -z "$(curl -sm 10 http://localhost/status | jq -r .configuration.exchange_version)" ] ; do
   		current_anax_service_check=`date +%s`
		echo `now` "the service is not ready, will retry in 1 second"
		if (( current_anax_service_check - start_anax_service_check > 60 )); then
			echo `now` "anax service timeout of 60 seconds occured"
			exit 1
		fi
		sleep 1
	done
    
    echo `now` "The service is ready"

    process_node

    echo `now` "install_linux() end"
}

function process_node(){
	echo `now` "process_node() begin"

	# Checking node state
	NODE_STATE=$(hzn node list | jq -r .configstate.state)
	WORKLOADS=$(hzn agreement list | jq -r .[])

	if [ $NODE_STATE = "configured" ]; then
		# node is registered
		echo `now` "Node is registered, state is ${NODE_STATE}"
		if [ -z "$WORKLOADS" ]; then
		 	# w/o pattern currently
		 	# if there's a pattern, then unregister and register again
		 	if [[ -z "$HZN_EXCHANGE_PATTERN" ]]; then
		 		echo `now` "Pattern has not been specified, skipping registration..."
		 	else
		 		echo `now` "There's no workloads running, but ${HZN_EXCHANGE_PATTERN} pattern has been specified"
		 		echo `now` "Unregistering the node and register it again with the new ${HZN_EXCHANGE_PATTERN} pattern..."
		 		set -x
    			hzn unregister -rf
    			set +x
    			registration $SKIP_REGISTRATION $HZN_EXCHANGE_PATTERN
    		fi
		else
			# with a pattern currently
			echo `now` "The node currently has workload(s) (check them with hzn agreement list)"
			if [[ -z "$HZN_EXCHANGE_PATTERN" ]]; then
				echo `now` "Pattern has not been specified"
				if [ ! "$OVERWRITE" = true ] ; then
					echo "Do you want to unregister node  and register it without pattern, continue?[y/N]:"
					read RESPONSE
					if [ ! "$RESPONSE" == 'y' ]; then
						echo "Exiting at users request"
						exit
					fi
				fi
				echo `now` "Unregistering the node and register it again without pattern..."
			else
				echo `now` "${HZN_EXCHANGE_PATTERN} has been specified"
				if [ ! "$OVERWRITE" = true ] ; then
					echo "Do you want to unregister and register it with a new ${HZN_EXCHANGE_PATTERN} pattern, continue?[y/N]:"
					read RESPONSE
					if [ ! "$RESPONSE" == 'y' ]; then
						echo "Exiting at users request"
						exit
					fi
				fi
				echo `now` "Unregistering the node and register it again with the new ${HZN_EXCHANGE_PATTERN} pattern..."
			fi
		 	set -x
    		hzn unregister -rf
    		set +x
    		registration $SKIP_REGISTRATION $HZN_EXCHANGE_PATTERN
		fi
	else
		# register, if not specified to skip
		echo `now` "Node is not registered, state is ${NODE_STATE}"

		create_node

    	registration $SKIP_REGISTRATION $HZN_EXCHANGE_PATTERN
	fi

	echo `now` "process_node() end"

}

# creates node
function create_node(){
	echo `now` "create_node() begin"

    NODE_NAME=$HOSTNAME
    echo `now` "Node name is $NODE_NAME"
    if [ -z "$HZN_EXCHANGE_NODE_AUTH" ]; then
        echo `now` "HZN_EXCHANGE_NODE_AUTH is not defined, creating it..."
        if [[ "$OS" == "linux" ]]; then
            if [ -f /etc/default/horizon ]; then
                echo `now` "Getting node id from /etc/default/horizon file..."
                NODE_ID=$(grep HZN_DEVICE_ID /etc/default/horizon |cut -d'=' -f2)
            else
                echo `now` "Cannot detect node id as /etc/default/horizon cannot be found, using ${NODE_NAME} hostname instead"
                NODE_ID=$NODE_NAME
            fi
        elif [[ "$OS" == "macos" ]]; then
            echo `now` "Using hostname as node id..."
            NODE_ID=$NODE_NAME
        fi
        echo `now` "Node id is $NODE_ID"

        echo `now` "Generating node token..."
        HZN_NODE_TOKEN=$(cat /dev/urandom | env LC_CTYPE=C tr -dc 'a-zA-Z0-9' | fold -w 45 | head -n 1)
        echo `now` "Generated node token is ${HZN_NODE_TOKEN}"
        HZN_EXCHANGE_NODE_AUTH="${NODE_ID}:${HZN_NODE_TOKEN}"
        echo `now` "HZN_EXCHANGE_NODE_AUTH for a node is ${HZN_EXCHANGE_NODE_AUTH}"
    else
        echo "Found HZN_EXCHANGE_NODE_AUTH variable, using it..."
    fi

    echo `now` "Creating a node..."

    set -x
    hzn exchange node create -n "$HZN_EXCHANGE_NODE_AUTH" -m "$NODE_NAME" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH"
    set +x

    echo `now` "Verifying a node..."
    set -x
    hzn exchange node confirm -n "$HZN_EXCHANGE_NODE_AUTH" -o "$HZN_ORG_ID"
    set +x

    echo `now` "create_node() end"
}

# register node depending on if registration's requested and pattern name
function registration() {
	echo `now` "registration() begin"

    NODE_NAME=$HOSTNAME
    echo `now` "Node name is $NODE_NAME"
    if [ "$1" = true ] ; then
        echo `now` "Skipping registration as it was specified with -s"
    else
        echo `now` "Registering node..."
        if [[ -z "${2}" ]]; then
            echo `now` "Pattern name was not specified, registering without it..."
            set -x
            hzn register -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH"
            set +x
        else
            echo `now` "Registering node with ${2} pattern"
            set -x
            hzn register -p "$2" -m "${NODE_NAME}" -o "$HZN_ORG_ID" -u "$HZN_EXCHANGE_USER_AUTH" -n "$HZN_EXCHANGE_NODE_AUTH"
            set +x
        fi
    fi

    echo `now` "registration() end"
}

function check_empty() {
    if [ -z "$1" ]; then
        echo `now` "The ${2} value is empty, exiting..."
        exit 1
    fi
}

# checks if file or directory exists
function check_exist() {
    case $1 in
	f) if ! [[ -f "$2" ]] ; then
			echo `now` "${3} file ${2} doesn't exist"
		    exit 1
		fi
	;;
	d) if ! [[ -d "$2" ]] ; then
			echo `now` "${3} directory ${2} doesn't exist"
	        exit 1
		fi
    ;;
    w) if ! ls ${2} 1> /dev/null 2>&1 ; then
			echo `now` "${3} files ${2} do not exist"
	        exit 1
	    fi
	;;
	*) echo "not supported"
        exit 1
	;;
	esac
}

# detects operating system. 
function detect_os() {
    echo `now` "detect_os begin"
    if [[ "$OSTYPE" == "linux"* ]]; then
        OS="linux"
    elif [[ "$OSTYPE" == "darwin"* ]]; then
        OS="macos"
    else
        OS="unknown"
    fi
    echo `now` "Detected OS is ${OS}"
    echo `now` "detect_os end"
}

# detects linux distributive name, version, and codename
function detect_distro() {
    echo `now` "detect_distro begin"

    if [ -f /etc/os-release ]; then
            . /etc/os-release
            DISTRO=$ID
            VER=$VERSION_ID
            CODENAME=$VERSION_CODENAME
    elif type lsb_release >/dev/null 2>&1; then
            DISTRO=$(lsb_release -si)
            VER=$(lsb_release -sr)
            CODENAME=$(lsb_release -sc)
    elif [ -f /etc/lsb-release ]; then
            . /etc/lsb-release
            DISTRO=$DISTRIB_ID
            VER=$DISTRIB_RELEASE
            CODENAME=$DISTRIB_CODENAME
    else
            echo `now` "Cannot detect Linux version, exiting..."
            exit 1
    fi

    # Raspbian has a codename embedded in a version
    if [[ "$DISTRO" == "raspbian" ]]; then
        CODENAME=$(echo ${VERSION} | sed -e 's/.*(\(.*\))/\1/')
    fi

    echo `now` "Detected distributive is ${DISTRO}, verison is ${VER}, codename is ${CODENAME}"
         
    echo `now` "detect_distro end"
}

# detects hardware architecture on linux
function detect_arch() {
    echo `now` "detect_arch() begin"

    # detecting architecture
    uname="$(uname -m)"
    if [[ "$uname" =~ "aarch64" ]]; then
        ARCH="arm64"
    elif [[ "$uname" =~ "arm" ]]; then
        ARCH="armhf"
    elif [[ "$uname" == "x86_64" ]]; then
        ARCH="amd64"
    elif [[ "$uname" == "ppc64le" ]]; then
        ARCH="ppc64el"
    else
        (>&2 echo "Unknown architecture $uname")
        exit 1
    fi

    echo `now` "Detected architecture is ${ARCH}"

    echo `now` "detect_arch() end"
}

# checks if OS/distributive/codename/arch is supported
function check_support() {
    echo `now` "check_support begin"
    
    # checks if OS, distro or arch is supported
    
    if [[ ! "${1}" = *"${2}"* ]]; then
        echo "Supported components are: "
        for i in "${1}"; do echo -n "${i} "; done
        echo ""
        echo `now` "The detected ${2} is not supported, exiting..."
        exit 1
    else
        echo `now` "The detected ${2} is supported"
    fi
    
    echo `now` "check_support end"
}

# checks if requirements are met
function check_requirements() {
    echo `now` "check_requirements begin"
    
    detect_os
    
    echo `now` "Checking support of detected OS..."
    check_support "${SUPPORTED_OS[*]}" "$OS"

    if [ "$OS" = "linux" ]; then
        detect_distro
        echo `now` "Checking support of detected Linux distributive..."
        check_support "${SUPPORTED_LINUX_DISTRO[*]}" "$DISTRO"
        echo `now` "Checking support of detected Linux version/codename..."
        check_support "${SUPPORTED_LINUX_VERSION[*]}" "$CODENAME"
        detect_arch
        echo `now` "Checking support of detected architecture..."
        check_support "${SUPPORTED_ARCH[*]}" "$ARCH"
        
        echo `now` "Checking the path with packages..."
        
        if [ "$PKG_TREE_IGNORE" = true ] ; then
        	# ignoring the package tree, checking the current dir
        	PACKAGES="${PKG_PATH}"
      	else
        	# checking the package tree for linux
        	PACKAGES="${PKG_PATH}/${OS}/${DISTRO}/${CODENAME}/${ARCH}"
        fi

        echo `now` "Checking path with packages ${PACKAGES}"
        check_exist w "${PACKAGES}/*horizon*${DISTRO}.${CODENAME}*.deb" "Linux installation"

        if [ $(id -u) -ne 0 ]; then
	        echo `now` "Please run script with the root priveledges by running 'sudo -s' command first"
            quit 1
        fi
        
    elif [ "$OS" = "macos" ]; then
    	
    	echo `now` "Checking the path with packages..."
    	
    	if [ "$PKG_TREE_IGNORE" = true ] ; then
        	# ignoring the package tree, checking the current dir
        	PACKAGES="${PKG_PATH}"
      	else
        	# checking the package tree for macos
        	PACKAGES="${PKG_PATH}/${OS}"
        fi

        echo `now` "Checking path with packages ${PACKAGES}"
        check_exist w "${PACKAGES}/horizon-cli-*.pkg" "MacOS installation"
        check_exist f "${PACKAGES}/${MAC_PACKAGE_CERT}" "The CLI package certificate"
    fi

    echo `now` "check_requirements end"
}

function check_node_state() {
	echo `now` "check_node_state() begin"

	if command -v hzn >/dev/null 2>&1; then
		local NODE_STATE=$(hzn node list | jq -r .configstate.state)
		echo `now` "Current node state is: ${NODE_STATE}"

		if [ "$NODE_STATE" = "configured" ]; then
			# node is configured need to ask what to do
			echo `now` "You node is registered"
			echo "Do you want to overwrite the current node configuration?[y/N]:"
			read RESPONSE
			if [ "$RESPONSE" == 'y' ]; then
				OVERWRITE=true
				echo `now` "The configuration will be overwritten..."
			else
				echo `now` "You might be asked for overwrite confirmations later..."
			fi
		elif [[ "$NODE_STATE" = "unconfigured" ]]; then
			# node is unconfigured
			echo `now` "The node is in unconfigured state, continuing..."
		fi
	else
		echo `now` "The hzn doesn't seem to be installed, continuing..."
	fi

	echo `now` "check_node_state() end"
}

# Accept the parameters from command line
while getopts "c:i:p:k:hs" opt; do
    case $opt in
        c) CERTIFICATE="$OPTARG"
        ;;
        i) PKG_PATH="$OPTARG" PKG_TREE_IGNORE=true
        ;;
        p) HZN_EXCHANGE_PATTERN="$OPTARG"
        ;;
        k) CFG="$OPTARG"
        ;;
        h) help
        ;;
        s) SKIP_REGISTRATION=true
        ;;
        \?) echo "Invalid option: -$OPTARG"; help
        ;;
        :) echo "Option -$OPTARG requires an argument"; help
        ;;
    esac
done

# checking the supplied arguments
validate_args "$*" "$#"
# showing current configuration
show_config
# checking if the requirements are met
check_requirements

check_node_state

if [[ "$OS" == "linux" ]]; then
    echo `now` "Detection results: OS is ${OS}, distributive is ${DISTRO}, release is ${CODENAME}, architecture is ${ARCH}"
    install_${OS} ${OS} ${DISTRO} ${CODENAME} ${ARCH}
elif [[ "$OS" == "macos" ]]; then
    echo `now` "Detection results: OS is ${OS}"
    install_${OS}
fi

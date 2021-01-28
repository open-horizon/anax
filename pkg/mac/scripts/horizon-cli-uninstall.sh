#!/bin/sh

# Remove all of the files and sym links that the mac horizon-cli pkg installed (including the sym links that postinstall created)

# They will not be able to remove the files unless they run this script via sudo
if [[ $(whoami) != 'root' ]]; then
    echo "Error: must be root to run ${0##*/}."
    exit 2
fi

# Make sure they want to do this
if [[ "$1" != '-y' ]]; then
    printf "Are you sure you want to remove all of the files the horizon-cli package installed and stop the horizon agent container? [y/N] "
    read RESPONSE
    if [ "$RESPONSE" != 'y' ]; then
        echo "Aborting ${0##*/}"
        exit
    fi
fi

SRCDIR=/Users/Shared
DESTDIR=/usr/local

isContainerRunning() {
	docker ps --filter "name=$1" | tail -n +2 | grep -q -w "$1"
	return $?
}

# The agent container "requires" the horizon-cli, so stop/remove the container (but not the image) before removing horizon-cli
if isContainerRunning horizon1; then
    $DESTDIR/bin/horizon-container stop   # this will echo what it is doing
fi

# Remove the sym links that postinstall created
echo "Removing horizon-cli files from $DESTDIR ..."
rm -f $DESTDIR/bin/hzn 
rm -f $DESTDIR/bin/horizon-container
rm -f $DESTDIR/bin/{agent-install.sh,agent-uninstall.sh,edgeNodeFiles.sh}
rm -f $DESTDIR/bin/horizon-cli-uninstall.sh
# hzn_bash_autocomplete.sh is in share/horizon, so linking the dir takes care of it
rm -f $DESTDIR/share/horizon
rm -f $DESTDIR/share/man/man1/hzn.1
rm -f $DESTDIR/etc/horizon/hzn.json
rm -f $DESTDIR/share/man/*/man1/hzn.1

# Remove the files the pkg directly installed
echo "Removing $SRCDIR/horizon-cli ..."
rm -rf $SRCDIR/horizon-cli


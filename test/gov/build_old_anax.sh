#!/bin/bash

  echo "Building old anax."
  chown -R root:root /root/.ssh
  mkdir -p /tmp/oldanax/anax-gopath/src/github.com/open-horizon
  mkdir -p /tmp/oldanax/anax-gopath/bin

  export GOPATH="/tmp/oldanax/anax-gopath"
  export TMPDIR="/tmp/oldanax/"

  cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon
  git clone https://github.com/open-horizon/anax.git
  if [ $? -ne 0 ]; then
  	echo "Failed to clone the anax repository."
  	exit 2
  fi
  cd /tmp/oldanax/anax-gopath/src/github.com/open-horizon/anax
  make anax
  if [ $? -ne 0 ]; then
  	echo "Failed to build anax."
  	exit 2
  fi
  cp anax /usr/bin/old-anax

  export GOPATH="/tmp"
  unset TMPDIR
  cd /tmp

pipeline {
    agent {
    	node {
        	label 'ubuntu2204-builder-aws-1c-2g'
    	}
    }
    stages {
	stage('Install Dependencies'){
	    steps{
		sh 'echo "Installing dependencies"'
		sh '''
			#!/usr/bin/env bash
			sudo usermod -aG sudo $USER
		        sudo su - $USER
                        apt-get update -qq && apt-get install -y \
                            wget \
                            gnupg2 \
                            software-properties-common
                        # install golang
                        export GO_VERSION=1.14.1
                        wget https://storage.googleapis.com/golang/go${GO_VERSION}.linux-amd64.tar.gz
                        tar -C /usr/local -xzf go${GO_VERSION}.linux-amd64.tar.gz
                        mkdir -p $HOME/go/src/github.com/open-horizon/anax
                        export GOPATH=$HOME/go
                        export PATH=$PATH:/usr/local/go/bin
                        ln -fs $WORKSPACE $GOPATH/src/github.com/open-horizon/anax
		'''
	    }
	}
        stage('Build Anax'){
            steps {
                sh 'echo "Building anax binaries"'
		sh '''
			#!/usr/bin/env bash
			sudo usermod -aG sudo $USER
		        sudo su - $USER
			make 
		'''
            }
        }
    }
}


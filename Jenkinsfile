pipeline {
    agent {
    	node {
        	label 'ubuntu2004-docker-aws-1c-2g'
    	}
    }
    stages {
	stage('Install Dependencies'){
	    steps{
		sh 'echo "Installing dependencies"'
		sh '''
			#!/usr/bin/env bash
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
			make 
		'''
            }
        }
    }
}


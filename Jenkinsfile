pipeline {
    agent {
    	node {
        	label 'ubuntu18.04-docker-8c-8g'
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


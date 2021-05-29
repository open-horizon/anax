pipeline {
    agent {
    	node {
        	label 'ubuntu2004-docker-aws-1c-2g'
    	}
    }
	
    stages {
		stage('Prepare environment'){
	    		steps{
				sh '''
		       		#!/usr/bin/env bash
		       		export PATH=$PATH:/usr/local/go/bin
				mkdir -p $HOME/go/src/github.com/open-horizon/anax
		       		export GOPATH=$HOME/go
		       		ln -fs $WORKSPACE $GOPATH/src/github.com/open-horizon/anax
				go version
				'''
	    		}
		}
        	stage('Build anax'){
			matrix {
				axes {
					axis {
						name "tests"
						values "NOLOOP=1", "NOLOOP=1 TEST_PATTERNS=sloc"
					}
				}
				stages {
					stage('Conduct e2e-dev-test') {
						steps {
							sh '''
							#!/usr/bin/env bash
							export GOPATH=$HOME/go
							export PATH=$PATH:/usr/local/go/bin
							make
							make -C test build-remote
							make -C test clean 
							make -C test test TEST_VARS=${tests}
							'''
            					}
					}
				}
			}
        	}
    	}
}


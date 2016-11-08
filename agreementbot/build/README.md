The build subcomponent of the agreementbot modules is used to create a docker container of the agreementbot function of anax, so that the container can be deployed.

To run the build;
1. make clean docker-push

If you get a Permission Denied error during the clean step, run sudo make clean and then run make docker-push

The results will be a docker container that can run the agbot on an x86 machine.

How does it really work? It's a little complex in order to provide for the ability to run on any platform, a "build" container is created which holds the source and compiled output of the anax build.
The build container is a mechanism for ensuring that the system on which you are building the container, doesn't have to have all the tools necessary to compile anax.
Once anax is compiled, the output is copied to a temporary build directory called 'docker-exec' on the building machine's file system.
The last step creates a docker container and pushes the compiled output into it, and then saves the container.
This becomes the deployable container, which is susequently tagged and made ready for upload to the docker registry.

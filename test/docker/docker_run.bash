#!/usr/bin/env bash
# docker run helper, depending on inputs and status of container will either start, or re-create a non-running container

container_name="$1"

base_cmd="docker ps -qaf name=$container_name"
running_cmd="$base_cmd -f 'status=running'"

if [[ "$(eval "$running_cmd")" != "" ]]; then
  exit 0
elif [[ "$(eval "$base_cmd")" != "" ]]; then
  # exists, not running, we didn't pass in a run cmd, then just want to start it back up
  if [ -z "$2" ]; then
    docker start "$container_name"
    exit 0
  else
    # exists but isn't running we want to remove it before sending the run cmd
    docker rm -f "$container_name"
  fi
fi

eval "${@:2}"

# check that container is now running, if not exit with > 1
if [[ "$(eval "$running_cmd")" == "" ]]; then
  (>&2 echo "Failure to ensure $container_name is running or to start it")
  exit 2
fi

exit 0

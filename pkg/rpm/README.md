# Building, Installing, and Using Horizon Agent RPMs

## Building the Horizon RPMs

Before manually invoking RPM build scripts, make sure you have passed `VERSION`
and `BUILD_NUMBER` environment variables. `BUILD_NUMBER` may be blank if you're
providing optional `DISTRO` tag. For example, this set of variables gives `horizon` and `horizon-cli` packages of version 2.26.12 relevant for using at RHEL8.x platforms:

```bash
export VERSION=2.26.12
export BUILD_NUMBER=
export DISTRO=el8
```

> NOTE: When you'll build RPM on host with different architecture (cross-compilation), make sure you're also exporting variables below along with version and build number (example for `ppc64le` arch of destination platform):

```bash
export arch=ppc64el
export rpm_arch=ppc64le
```

You can build RPMs on a Linux or Mac host. This command executes build process for both `horizon` and `horizon-cli` packages:

```bash
make all
```

If you would like to create RPMs for Open Horizon packages separately, try these commands:

```bash
# For `horizon` package
make -C horizon rpmbuild
# For `horizon-cli` package
make -C horizon-cli rpmbuild
```

Note where it wrote the rpm files. Usually the location is `$HOME/rpmbuild/RPMS/`.

## Installing the Horizon RPMs

### Installing the RPMs on RHEL/CentOS 8.x

- Copy the rpm files to RHEL/CentOS 8.x host
- `docker-ce` does not automatically install cleanly. Follow https://linuxconfig.org/how-to-install-docker-in-rhel-8 to install and configure it.
- Verify docker is working: `docker ps`
- Install the horizon agent and CLI: `dnf install horizon-*.x86_64.rpm`
- Fill in variable values in `/etc/default/horizon`
- Restart the agent: `systemctl restart horizon`
- Verify the agent is working and the `configuration.exchange_version` field is filled in: `hzn node list`

### Installing the RPMs on Fedora

TBD

## Using the Horizon RPMs

Once you have installed and configured the Horizon RPMS by following one of the sections above, and have confirmed that both docker and the Horizon agent are running, use the agent like this:

- export `HZN_ORG_ID` and `HZN_EXCHANGE_USER_AUTH`
- Register the node with the helloworld pattern:

  ```bash
  hzn register -n <nodeid:nodetoken> -p IBM/pattern-ibm.helloworld -s ibm.helloworld --serviceorg IBM
  ```

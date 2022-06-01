# Building, Installing, and Using Horizon Agent RPMs

## Building the Horizon RPMs

You can build them on a Linux or Mac host. From this directory run:

```bash
make -C horizon
make -C horizon-cli
```

Note where it wrote the rpm files.

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

- Copy the rpm files to Fedora host
- Install podman `dnf install podman`
- Verify podman is working: `podman ps`
- Install the horizon agent and CLI: `dnf install horizon-*.x86_64.rpm`
- Fill in variable values in `/etc/default/horizon`
- Restart the agent: `systemctl restart horizon`
- Verify the agent is working and the `configuration.exchange_version` field is filled in: `hzn node list`

## Using the Horizon RPMs

Once you have installed and configured the Horizon RPMS by following one of the sections above, and have confirmed that both docker and the Horizon agent are running, use the agent like this:

- export `HZN_ORG_ID` and `HZN_EXCHANGE_USER_AUTH`
- Register the node with the helloworld pattern:

  ```bash
  hzn register -n <nodeid:nodetoken> -p IBM/pattern-ibm.helloworld -s ibm.helloworld --serviceorg IBM
  ```

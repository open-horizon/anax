# Spec file for the horizon RPM. For info about writing spec files, see:
#	http://ftp.rpm.org/max-rpm/s1-rpm-build-creating-spec-file.html
#	https://rpm-packaging-guide.github.io/

Summary: Open-horizon edge agent
Name: horizon
Version: %{getenv:VERSION}
Release: %{getenv:BUILD_NUMBER}
Epoch: 1
License: Apache License Version 2.0
Source: horizon-%{version}.tar.gz
Packager: Open-horizon
BuildArch: %{_arch}
Provides: horizon = %{version}

# Note: in RHEL/CentOS 8.x, docker-ce does not automatically install cleanly.
#	Must do this manually *before* installing this horizon pkg: https://linuxconfig.org/how-to-install-docker-in-rhel-8
Requires: horizon-cli docker-ce iptables jq

#Prefix: /usr/horizon
#Vendor: ?
#Distribution: ?
#BuildRoot: ?

%description
Open-horizon edge node agent

%prep
%setup -c

%build
# This phase is done in ~/rpmbuild/BUILD/horizon-<version> . All of the tarball source has been unpacked there and
# is in the same file structure as it is in the git repo. $RPM_BUILD_DIR has a value like ~/rpmbuild/BUILD
#env | grep -i build
# Need to play some games to get our src dir under a GOPATH
#rm -f ../src; ln -s . ../src
#mkdir -p ../github.com/open-horizon
#rm -f ../github.com/open-horizon/anax; ln -s ../../anax-%{version} ../github.com/open-horizon/anax
#GOPATH=$RPM_BUILD_DIR make anax

%install
# The install phase puts all of the files in the paths they should be in when the rpm is installed on a system.
# The $RPM_BUILD_ROOT is a simulated root file system and usually has a value like: ~/rpmbuild/BUILDROOT/horizon-1.0.0-1.x86_64
# Following the LSB Filesystem Hierarchy Standard: https://refspecs.linuxfoundation.org/FHS_3.0/fhs-3.0.pdf
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/usr/horizon/{bin,samples} $RPM_BUILD_ROOT/etc/default $RPM_BUILD_ROOT/etc/horizon/trust/
cp -a fs/* $RPM_BUILD_ROOT/

%files
#%defattr(-, root, root)
%license /usr/horizon/LICENSE.txt
/usr/horizon
/lib/systemd/system/horizon.service
/etc/horizon

%post
# Runs after the pkg is installed
if [[ ! -f /etc/default/horizon ]]; then
    # Only create an empty/template file if they do not already have a real one
    mkdir -p /etc/default
    echo -e "HZN_EXCHANGE_URL=\nHZN_FSS_CSSURL=\nHZN_MGMT_HUB_CERT_PATH=\nHZN_DEVICE_ID=\nHZN_AGENT_PORT=8510" > /etc/default/horizon
    # Note: postun deletes this file in the complete removal case
fi

#if systemctl > /dev/null 2>&1; then  # for testing installation in docker container
systemctl daemon-reload
systemctl enable horizon.service
if systemctl --quiet is-active horizon.service; then
	systemctl stop horizon.service   # in case this was an update
fi
systemctl start horizon.service
#fi
mkdir -p /var/{horizon/,run/horizon/}

%preun
# This runs before the pkg is removed. But the way rpm updates work is the newer rpm is installed 1st (with reference counting on the files),
# and then the old rpm is removed (and any files whose reference count is 1), so we have to be able to tell the difference.
# The $1 arg is set to the number of rpms that will be left when this rpm is removed, so 0 means this is a complete removal, not an update.
if [ "$1" = "0" ]; then
  # Complete removal of the rpm
  # Save off container, agreements, etc. resources anax knows about for later removal
  IMAGES_OUT=/var/horizon/prerm.images
  BRIDGES_OUT=/var/horizon/prerm.bridges
  touch $IMAGES_OUT $BRIDGES_OUT

  agreements_fetch="$(curl -s http://localhost/agreement)"
  if [ $? == 0 ]; then
    anax_agreements=$(echo $agreements_fetch | jq -r '(.agreements.active[], .agreements.archived[])')

    # get all image names
    echo "$anax_agreements" | jq -r '.proposal' | jq -r '.tsandcs'  | jq -r '.workloads[].deployment' | jq -r '.services[].image' | sort | uniq > $IMAGES_OUT

    # get all network bridge names (same as agreement right now)
    echo $anax_agreements | jq -r '.current_agreement_id' | sort | uniq > $BRIDGES_OUT
  fi

  # Now shutdown the daemon
  #if systemctl > /dev/null 2>&1; then  # for testing installation in docker container
  if systemctl --quiet is-enabled horizon.service; then
	systemctl disable horizon.service
  fi
  systemctl daemon-reload
  systemctl reset-failed
  #fi
fi

%postun
# Runs after the pkg is uninstalled. Same deal as for preun above for updates.
if [ "$1" = "0" ]; then
  # Complete removal of the rpm
  # Now that the daemon is stopped, we can delete all the resources we collected in preun above
  # remove all running containers with horizon tags
  containers="$(docker ps -aq 2> /dev/null)"
  if [ "$containers" != "" ]; then
    # TODO: add infrastructure labels too
    # reassign containers variable after doing some filtering
    containers=$(echo $containers | xargs docker inspect | jq -r '.[] | select ((.Config.Labels | length != 0) and (.Config.Labels["openhorizon.anax.service_name"] !="" or .Config.Labels["openhorizon.anax.infrastructure"] != ""))')
  fi

  # remove running containers
  if [ "$containers" != "" ]; then
    echo $containers | jq -r '.Id' | xargs docker rm -f
  fi

  # remove networks; some errors are expected b/c we're issuing remove command for even networks that should have already been removed by anax
  cat /var/horizon/prerm.bridges <<< $(echo $containers | jq -r '.NetworkSettings.Networks | keys[]') | sort | uniq | grep -v 'bridge' | xargs docker network rm 2> /dev/null

  # remove container images; TODO: use labels to remove infrastructure container images too once they are tagged properly upon
  cat /var/horizon/prerm.images <<< $(echo $containers | jq -r '.Config.Image') | sort | uniq | xargs docker rmi 2> /dev/null

  # Note: in the debian pkg, these cmds only run for the purge option. There doesn't seem to be an rpm equivalent
  rm -Rf /etc/horizon /var/cache/horizon /etc/default/horizon /var/tmp/horizon /var/run/horizon
  # remove all content from /var/horizon that isn't related to the dedicated SBC images
  find /var/horizon -mindepth 1 ! \( -name '.firstboot' -or -name 'image_version' \) -exec rm -rf {} +
  if [ "$(ls -A /var/horizon)" ]; then
    rmdir /var/horizon
  fi
fi

%clean
# This step happens *after* the %files packaging
rm -rf $RPM_BUILD_ROOT

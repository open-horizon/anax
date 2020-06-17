# Spec file for the horizon-cli RPM. For info about writing spec files, see:
#	http://ftp.rpm.org/max-rpm/s1-rpm-build-creating-spec-file.html
#	https://rpm-packaging-guide.github.io/

Summary: Open-horizon CLI
Name: horizon-cli
Version: %{getenv:VERSION}
Release: %{getenv:RELEASE}
Epoch: 1
License: Apache License Version 2.0
Source: horizon-cli-%{version}.tar.gz
Packager: Open-horizon
BuildArch: x86_64
Provides: horizon-cli = %{version}
#todo: restore: Requires: docker

#Prefix: /usr/horizon
#Vendor: ?
#Distribution: ?
#BuildRoot: ?

%description
Open-horizon command line interface

%prep
%setup -q

%build
#todo: the rpm should really build the executables itself, but we have a ways to go before getting there...
# This phase is done in ~/rpmbuild/BUILD/horizon-cli-<version> . All of the tarball source has been unpacked there and
# is in the same file structure as it is in the git repo. $RPM_BUILD_DIR has a value like ~/rpmbuild/BUILD
#env | grep -i build
# Need to play some games to get our src dir under a GOPATH
#rm -f ../src; ln -s . ../src
#mkdir -p ../github.com/open-horizon
#rm -f ../github.com/open-horizon/hzn; ln -s ../../hzn-%{version} ../github.com/open-horizon/hzn
#GOPATH=$RPM_BUILD_DIR make cli/hzn

%install
# The install phase puts all of the files in the paths they should be in when the rpm is installed on a system.
# The $RPM_BUILD_ROOT is a simulated root file system and usually has a value like: ~/rpmbuild/BUILDROOT/horizon-cli-1.0.0-1.x86_64
# Following the LSB Filesystem Hierarchy Standard: https://refspecs.linuxfoundation.org/FHS_3.0/fhs-3.0.pdf
rm -rf $RPM_BUILD_ROOT
mkdir -p $RPM_BUILD_ROOT/usr/horizon/bin $RPM_BUILD_ROOT/etc/horizon $RPM_BUILD_ROOT/etc/bash_completion.d
cp -a fs/* $RPM_BUILD_ROOT/

%files
#%defattr(-, root, root)
/usr/horizon
/etc/horizon
#todo: cmd completion for hzn is not working. Have not figured out why yet.
/etc/bash_completion.d/hzn_bash_autocomplete.sh

%post
# Runs after the pkg is installed
if [[ ! -e "/usr/bin/hzn" ]]; then
	ln -s /usr/horizon/bin/hzn /usr/bin/hzn
fi
if [[ ! -e "/usr/bin/horizon-container" ]]; then
	ln -s /usr/horizon/bin/horizon-container /usr/bin/horizon-container
fi

%postun
# Runs after the pkg is uninstalled. $1 == 0 means this is a complete removal, not an update.
if [ "$1" = "0" ]; then
	# Remove the sym links we created during install of this pkg
	rm -f /usr/bin/hzn
	rm -f /usr/bin/horizon-container
fi

%clean
# This step happens *after* the %files packaging
rm -rf $RPM_BUILD_ROOT

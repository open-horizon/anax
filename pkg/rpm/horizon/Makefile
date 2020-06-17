SHELL ?= /bin/bash -e
#todo: not sure where/how to get this automatically
export VERSION ?= 2.26.12
export RELEASE ?= 1
# rpmbuild does not give us a good way to set topdir, so use the default location
RPMROOT ?= $(HOME)/rpmbuild
RPMNAME ?= horizon


# Remember to up VERSION above. If building the rpm on mac, first: brew install rpm
# Note: during rpmbuild on mac, you may get this benign msg: error: Couldn't exec /usr/local/Cellar/rpm/4.14.1_1/lib/rpm/elfdeps: No such file or directory
rpmbuild:
	opsys=Linux arch=amd64 $(MAKE) -C ../../.. anax
	cp ../../../anax fs/usr/horizon/bin
	cp -a ../../../cli/samples fs/usr/horizon
	cp -a ../../../LICENSE.txt fs/usr/horizon
	cp -a ../README.md fs/usr/horizon
	mkdir -p $(RPMROOT)/{SOURCES,SRPMS,SRPMS}
	rm -f $(RPMNAME)-$(VERSION); ln -s . $(RPMNAME)-$(VERSION)  # so the tar file files can have this prefix
	rm -f $(RPMROOT)/SOURCES/$(RPMNAME)-*.tar.gz
	#tar --exclude '.git*' -X .tarignore -H -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz $(RPMNAME)-$(VERSION)
	tar -H -czf $(RPMROOT)/SOURCES/$(RPMNAME)-$(VERSION).tar.gz $(RPMNAME)-$(VERSION)
	rm -rf $(RPMROOT)/BUILD/horizon-*
	rm -f $(RPMROOT)/SRPMS/$(RPMNAME)-$${VERSION%%.*}*rpm $(RPMROOT)/RPMS/x86_64/$(RPMNAME)-$${VERSION%%.*}*rpm $(RPMROOT)/RPMS/x86_64/$(RPMNAME)-$${VERSION%%.*}*rpm.gz
	rpmbuild --target x86_64-linux -ba $(RPMNAME).spec
	gzip --keep $(RPMROOT)/RPMS/x86_64/$(RPMNAME)-$(VERSION)-$(RELEASE).x86_64.rpm
	rm -f $(RPMNAME)-$(VERSION)   # remove the sym link

.PHONY: rpmbuild

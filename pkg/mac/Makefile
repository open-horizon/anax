SHELL ?= /bin/bash -e

# When calling this Makefile you must pass in at least VERSION for the artifact build targets
export VERSION
export BUILD_NUMBER

ifdef BUILD_NUMBER
BUILD_NUMBER := -$(BUILD_NUMBER:-%=%)
endif


# these are normally inherited from the top-level Makefile, but defaults are provided here, in case running this Makefile directly
CLI_EXECUTABLE ?= cli/hzn
LICENSE_FILE ?= LICENSE.txt
CLI_MAN_DIR ?= cli/man1
SUPPORTED_LOCALES ?= de  es  fr  it  ja  ko  pt_BR  zh_CN  zh_TW
CLI_COMPLETION_DIR ?= cli/bash_completion
CLI_CONFIG_FILE ?= cli/hzn.json

MAC_PKG = build/horizon-cli-$(VERSION)$(BUILD_NUMBER).pkg
MAC_PKG_IDENTIFIER ?= com.github.open-horizon.pkg.horizon-cli
MAC_PKG_INSTALL_DIR ?= /Users/Shared/horizon-cli

# This is a 1-time step to create the private signing key and public cert for the mac pkg.
# You must first set HORIZON_CLI_PRIV_KEY_PW to the passphrase to use the private key.
gen-mac-key:
	: $${HORIZON_CLI_PRIV_KEY_PW:?}
	@echo "Generating the horizon-cli mac pkg private key and public certificate, and putting them in the p12 archive"
	openssl genrsa -out build/horizon-cli.key 2048  # create private key
	openssl req -x509 -days 3650 -new -config key-gen/horizon-cli-key.conf -nodes -key build/horizon-cli.key -extensions extensions -sha256 -out build/horizon-cli.crt  # create self-signed cert
	openssl pkcs12 -export -inkey build/horizon-cli.key -in build/horizon-cli.crt -out build/horizon-cli.p12 -password env:HORIZON_CLI_PRIV_KEY_PW  # wrap the key and certificate into PKCS#12 archive
	rm -f build/horizon-cli.key  # clean up intermediate files
	@echo "Created pkg/mac/build/horizon-cli.crt and pkg/mac/build/horizon-cli.p12. Once you are sure that this the key/cert that should be used for all new staging mac packages:"
	@echo "  - Copy pkg/mac/build/horizon-cli.p12 to the horizon dev team's private key location."
	@echo "  - Make pkg/mac/build/horizon-cli.crt available to users via 'make macuploadcert' ."

# This is a 1-time step to install the mac pkg signing key on your mac so it can be used to sign the pkg.
# You must first set HORIZON_CLI_PRIV_KEY_PW to the passphrase to use the private key.
# If you did not just create the build/horizon-cli.p12 file using the make target above, download it and put it there.
install-mac-key:
	: $${HORIZON_CLI_PRIV_KEY_PW:?}
	@echo "Importing the key/cert into your keychain. When prompted, enter your Mac admin password:"
	sudo security import build/horizon-cli.p12 -k /Library/Keychains/System.keychain -P "$$HORIZON_CLI_PRIV_KEY_PW" -f pkcs12  # import key/cert into keychain
	#todo: the cmd above does not automatically set the cert to "Always Trust", tried the cmd below but it does not work.
	#sudo security add-trusted-cert -d -r trustAsRoot -p pkgSign -k /Library/Keychains/System.keychain build/horizon-cli.p12
	@echo "pkg/mac/build/horizon-cli.p12 installed in the System keychain. Now set it to 'Always Trust' by doing:"
	@echo "Open Finder, click on Applications and then Utilities, and open the Keychain Access app."
	@echo "Click on the System keychain, find horizon-cli-installer in list, and open it."
	@echo "Expand the Trust section and for 'When using this certificate' select 'Always Trust'."

# The rest of the targets will fail oddly if VERSION is not set, so provide a target to check for it
require-version:
ifndef VERSION
	@echo "Error: VERSION must be passed into this Makefile for this target"
	@false
else
	@true
endif

# Build the pkg and put it in build/
macpkg: $(MAC_PKG)

# Build the mac pkg. Assumes version.go has already be modified appropriately
$(MAC_PKG): require-version
	@echo "Producing Mac pkg horizon-cli"
	$(MAKE) -C ../.. cli/hzn
	mkdir -p build horizon-cli/bin horizon-cli/share/horizon/cluster horizon-cli/share/man/man1 horizon-cli/etc/horizon
	@# If you add files copied, check if they need to be added to postinstall too
	cp ../../$(CLI_EXECUTABLE) horizon-cli/bin
	cp -Rapv ../../cli/samples horizon-cli/share/horizon
	cp ../../anax-in-container/horizon-container horizon-cli/bin
	cp ../../agent-install/agent-install.sh horizon-cli/bin
	cp ../../agent-install/agent-uninstall.sh horizon-cli/bin
	cp ../../agent-install/edgeNodeFiles.sh horizon-cli/bin
	cp ../../agent-install/k8s/deployment-template.yml horizon-cli/share/horizon/cluster
	cp ../../agent-install/k8s/persistentClaim-template.yml horizon-cli/share/horizon/cluster
	cp scripts/horizon-cli-uninstall.sh horizon-cli/bin
	cp ../../$(LICENSE_FILE) horizon-cli/share/horizon
	cp ../../$(CLI_MAN_DIR)/hzn.1 horizon-cli/share/man/man1
	for loc in $(SUPPORTED_LOCALES) ; do \
		mkdir -p horizon-cli/share/man/$$loc/man1 && \
		cp ../../$(CLI_MAN_DIR)/hzn.1.$$loc horizon-cli/share/man/$$loc/man1/hzn.1; \
	done
	cp ../../$(CLI_COMPLETION_DIR)/hzn_bash_autocomplete.sh horizon-cli/share/horizon
	cp ../../$(CLI_CONFIG_FILE) horizon-cli/etc/horizon
	pkgbuild --sign "horizon-cli-installer" --root horizon-cli --scripts scripts --identifier $(MAC_PKG_IDENTIFIER) --version $(VERSION)$(BUILD_NUMBER) --install-location $(MAC_PKG_INSTALL_DIR) $@

# Convenience targets to install and view the built mac pkg locally
macinstall: $(MAC_PKG)
	sudo installer -pkg $< -target '/Volumes/Macintosh HD'

# This only works on the installed package
macpkginfo: require-version
	pkgutil --pkg-info $(MAC_PKG_IDENTIFIER)
	pkgutil --only-files --files $(MAC_PKG_IDENTIFIER)

.PHONY: gen-mac-key install-mac-key require-version macpkg macinstall macpkginfo

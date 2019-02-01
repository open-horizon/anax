# Building the horizon-cli Mac Package

## Precondition

Set/export `HORIZON_CLI_PRIV_KEY_PW` to the passphrase the signing private key has (if it has already been created) or will have (if you are going to create it now).

## Generate and Install the Signing Key

If the signing private key and public certificate have never been created, or you need to recreate them:

```
# at the top level of the anax git repo:
make gen-mac-key
```

To install the signing private key on your Mac (this only needs to be done once) so you can sign the mac pkg you are building:

```
# at the top level of the anax git repo:
make install-mac-key
```

## Build the Mac Package

```
# at the top level of the anax git repo:
# set MAC_PKG_VERSION to the desired version number in the Makefile, and then:
make macpkg
```

## Install the Mac Package Locally For Testing

If you are trying to verify that the package is signed properly:
- Find `pkg/mac/build/horizon-cli-<version>.pkg` in Finder and open it
- Verify you do not get any unnerving messages (like "invalid certificate" or "unknown package")

Otherwise, you can install the package via the command line:

```
# at the top level of the anax git repo:
make macinstall
```

## Upload the Mac Package (and New Certificate, If Necessary)

Once the package has been verified (including the signature):

Upload the package to the staging download spot, so other dev team members can test:

```
# at the top level of the anax git repo:
make macupload
```

If you created a new private signing key and public cert, upload the cert to the staging download spot, so other dev team members can test:

```
# at the top level of the anax git repo:
make macuploadcert
```

Then they can be downloaded by others from http://pkg.bluehorizon.network/macos/testing/ .

## Promote the Mac Package (and New Certificate, If Necessary)

Once it has been verified in staging, and this version of horizon is being promoted:

```
# at the top level of the anax git repo:
make promote-mac-pkg
```

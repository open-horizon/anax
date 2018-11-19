// Command line interface to the horizon agent. Provide sub-commands to register an edge node, display info about the node, etc.
package main

import (
	"flag"
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/agreementbot"
	"github.com/open-horizon/anax/cli/attribute"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/eventlog"
	"github.com/open-horizon/anax/cli/exchange"
	_ "github.com/open-horizon/anax/cli/helm_deployment"
	"github.com/open-horizon/anax/cli/key"
	"github.com/open-horizon/anax/cli/metering"
	_ "github.com/open-horizon/anax/cli/native_deployment"
	"github.com/open-horizon/anax/cli/node"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cli/service"
	"github.com/open-horizon/anax/cli/status"
	"github.com/open-horizon/anax/cli/unregister"
	"github.com/open-horizon/anax/cli/utilcmds"
	"github.com/open-horizon/anax/cutil"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"strings"
)

func main() {
	// Shut off the Anax runtime logging, so functions reused from anax don't fight with the kingpin parsing of args/flags.
	// Also, in the reused code need to change any calls like glog.Infof("some string") to glog.V(3).Infof("some string")
	flag.Set("v", "0")

	// Command flags and args - see https://github.com/alecthomas/kingpin
	app := kingpin.New("hzn", `Command line interface for Horizon agent. Most of the sub-commands use the Horizon Agent API at the default location http://localhost (see environment Environment Variables section to override this).

Environment Variables:
  HORIZON_URL:  Override the URL at which hzn contacts the Horizon Agent API.
      This can facilitate using a remote Horizon Agent via an ssh tunnel.
  HZN_EXCHANGE_URL:  Override the URL that the 'hzn exchange' sub-commands use
      to communicate with the Horizon Exchange, for example
      https://exchange.bluehorizon.network/api/v1. (By default hzn will ask the
      Horizon Agent for the URL.)
  HZN_ORG_ID:  Default value for the 'hzn exchange -o' flag,
      to specify the organization ID'.
  HZN_EXCHANGE_USER_AUTH:  Default value for the 'hzn exchange -u' or 'hzn
      register -u' flag, in the form '[org/]user:pw'.
  HZN_DONT_SUBST_ENV_VARS:  Set this to "1" to indicate that input json files
      should *not* be processed to replace environment variable references with
      their values.
`)
	app.HelpFlag.Short('h')
	app.UsageTemplate(kingpin.CompactUsageTemplate)
	cliutils.Opts.Verbose = app.Flag("verbose", "Verbose output.").Short('v').Bool()
	cliutils.Opts.IsDryRun = app.Flag("dry-run", "When calling the Horizon or Exchange API, do GETs, but don't do PUTs, POSTs, or DELETEs.").Bool()

	versionCmd := app.Command("version", "Show the Horizon version.") // using a cmd for this instead of --version flag, because kingpin takes over the latter and can't get version only when it is needed

	exchangeCmd := app.Command("exchange", "List and manage Horizon Exchange resources.")
	exOrg := exchangeCmd.Flag("org", "The Horizon exchange organization ID. If not specified, HZN_ORG_ID will be used as a default.").Short('o').String()
	exUserPw := exchangeCmd.Flag("user-pw", "Horizon Exchange user credentials to query and create exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.").Short('u').PlaceHolder("USER:PW").String()

	exVersionCmd := exchangeCmd.Command("version", "Display the version of the Horizon Exchange.")
	exStatusCmd := exchangeCmd.Command("status", "Display the status of the Horizon Exchange.")

	exUserCmd := exchangeCmd.Command("user", "List and manage users in the Horizon Exchange.")
	exUserListCmd := exUserCmd.Command("list", "Display the user resource from the Horizon Exchange. (Normally you can only display your own user. If the user does not exist, you will get an invalid credentials error.)")
	exUserListAll := exUserListCmd.Flag("all", "List all users in the org. Will only do this if you are a user with admin privilege.").Short('a').Bool()
	exUserCreateCmd := exUserCmd.Command("create", "Create the user resource in the Horizon Exchange.")
	exUserCreateUser := exUserCreateCmd.Arg("user", "Your username for this user account when creating it in the Horizon exchange.").Required().String()
	exUserCreatePw := exUserCreateCmd.Arg("pw", "Your password for this user account when creating it in the Horizon exchange.").Required().String()
	exUserCreateEmail := exUserCreateCmd.Arg("email", "Your email address that should be associated with this user account when creating it in the Horizon exchange. If your username is an email address, this argument can be omitted.").String()
	exUserCreateIsAdmin := exUserCreateCmd.Flag("admin", "This user should be an administrator, capable of managing all resources in this org of the exchange.").Short('A').Bool()
	exUserSetAdminCmd := exUserCmd.Command("setadmin", "Change the existing user to be an admin user (like root in his/her org) or to no longer be an admin user. Can only be run by exchange root or another admin user.")
	exUserSetAdminUser := exUserSetAdminCmd.Arg("user", "The user to be modified.").Required().String()
	exUserSetAdminBool := exUserSetAdminCmd.Arg("isadmin", "True if they should be an admin user, otherwise false.").Required().Bool()
	exUserDelCmd := exUserCmd.Command("remove", "Remove a user resource from the Horizon Exchange. Warning: this will cause all exchange resources owned by this user to also be deleted (nodes, services, patterns, etc).")
	exDelUser := exUserDelCmd.Arg("user", "The user to remove.").Required().String()
	exUserDelForce := exUserDelCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()

	exNodeCmd := exchangeCmd.Command("node", "List and manage nodes in the Horizon Exchange")
	exNodeListCmd := exNodeCmd.Command("list", "Display the node resources from the Horizon Exchange.")
	exNode := exNodeListCmd.Arg("node", "List just this one node.").String()
	exNodeLong := exNodeListCmd.Flag("long", "When listing all of the nodes, show the entire resource of each nodes, instead of just the name.").Short('l').Bool()
	exNodeCreateCmd := exNodeCmd.Command("create", "Create the node resource in the Horizon Exchange.")
	exNodeCreateNodeIdTok := exNodeCreateCmd.Flag("node-id-tok", "The Horizon Exchange node ID and token to be created. The node ID must be unique within the organization.").Short('n').PlaceHolder("ID:TOK").String()
	exNodeCreateNodeEmail := exNodeCreateCmd.Flag("email", "Your email address. Only needs to be specified if: the user specified in the -u flag does not exist, and you specified the 'public' org. If these things are true we will create the user and include this value as the email attribute.").Short('e').String()
	exNodeCreateNode := exNodeCreateCmd.Arg("node", "The node to be created.").String()
	exNodeCreateToken := exNodeCreateCmd.Arg("token", "The token the new node should have.").String()
	exNodeSetTokCmd := exNodeCmd.Command("settoken", "Change the token of a node resource in the Horizon Exchange.")
	exNodeSetTokNode := exNodeSetTokCmd.Arg("node", "The node to be changed.").Required().String()
	exNodeSetTokToken := exNodeSetTokCmd.Arg("token", "The new token for the node.").Required().String()
	exNodeConfirmCmd := exNodeCmd.Command("confirm", "Check to see if the specified node and token are valid in the Horizon Exchange.")
	exNodeConfirmNode := exNodeConfirmCmd.Arg("node", "The node id to be checked.").Required().String()
	exNodeConfirmToken := exNodeConfirmCmd.Arg("token", "The token for the node.").Required().String()
	exNodeDelCmd := exNodeCmd.Command("remove", "Remove a node resource from the Horizon Exchange. Do NOT do this when an edge node is registered with this node id.")
	exDelNode := exNodeDelCmd.Arg("node", "The node to remove.").Required().String()
	exNodeDelForce := exNodeDelCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()

	exAgbotCmd := exchangeCmd.Command("agbot", "List and manage agbots in the Horizon Exchange")
	exAgbotListCmd := exAgbotCmd.Command("list", "Display the agbot resources from the Horizon Exchange.")
	exAgbot := exAgbotListCmd.Arg("agbot", "List just this one agbot.").String()
	exAgbotLong := exAgbotListCmd.Flag("long", "When listing all of the agbots, show the entire resource of each agbots, instead of just the name.").Short('l').Bool()
	exAgbotListPatsCmd := exAgbotCmd.Command("listpattern", "Display the patterns that this agbot is serving.")
	exAgbotLP := exAgbotListPatsCmd.Arg("agbot", "The agbot to list the patterns for.").Required().String()
	exAgbotLPPatOrg := exAgbotListPatsCmd.Arg("patternorg", "The organization of the 1 pattern to list.").String()
	exAgbotLPPat := exAgbotListPatsCmd.Arg("pattern", "The name of the 1 pattern to list.").String()
	exAgbotLPNodeOrg := exAgbotListPatsCmd.Arg("nodeorg", "The organization of the nodes that should be searched. Defaults to patternorg.").String()
	exAgbotAddPatCmd := exAgbotCmd.Command("addpattern", "Add this pattern to the list of patterns this agbot is serving.")
	exAgbotAP := exAgbotAddPatCmd.Arg("agbot", "The agbot to add the pattern to.").Required().String()
	exAgbotAPPatOrg := exAgbotAddPatCmd.Arg("patternorg", "The organization of the pattern to add.").Required().String()
	exAgbotAPPat := exAgbotAddPatCmd.Arg("pattern", "The name of the pattern to add.").Required().String()
	exAgbotAPNodeOrg := exAgbotAddPatCmd.Arg("nodeorg", "The organization of the nodes that should be searched. Defaults to patternorg.").String()
	exAgbotDelPatCmd := exAgbotCmd.Command("removepattern", "Remove this pattern from the list of patterns this agbot is serving.")
	exAgbotDP := exAgbotDelPatCmd.Arg("agbot", "The agbot to remove the pattern from.").Required().String()
	exAgbotDPPatOrg := exAgbotDelPatCmd.Arg("patternorg", "The organization of the pattern to remove.").Required().String()
	exAgbotDPPat := exAgbotDelPatCmd.Arg("pattern", "The name of the pattern to remove.").Required().String()
	exAgbotDPNodeOrg := exAgbotDelPatCmd.Arg("nodeorg", "The organization of the nodes that should be searched. Defaults to patternorg.").String()

	exPatternCmd := exchangeCmd.Command("pattern", "List and manage patterns in the Horizon Exchange")
	exPatternListCmd := exPatternCmd.Command("list", "Display the pattern resources from the Horizon Exchange.")
	exPattern := exPatternListCmd.Arg("pattern", "List just this one pattern. Use <org>/<pat> to specify a public pattern in another org, or <org>/ to list all of the public patterns in another org.").String()
	exPatternLong := exPatternListCmd.Flag("long", "When listing all of the patterns, show the entire resource of each pattern, instead of just the name.").Short('l').Bool()
	exPatternPublishCmd := exPatternCmd.Command("publish", "Sign and create/update the pattern resource in the Horizon Exchange.")
	exPatJsonFile := exPatternPublishCmd.Flag("json-file", "The path of a JSON file containing the metadata necessary to create/update the pattern in the Horizon exchange. See /usr/horizon/samples/pattern.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exPatKeyFile := exPatternPublishCmd.Flag("private-key-file", "The path of a private key file to be used to sign the pattern.").Short('k').ExistingFile()
	exPatPubPubKeyFile := exPatternPublishCmd.Flag("public-key-file", "The path of public key file (that corresponds to the private key) that should be stored with the pattern, to be used by the Horizon Agent to verify the signature.").Short('K').ExistingFile()
	exPatName := exPatternPublishCmd.Flag("pattern-name", "The name to use for this pattern in the Horizon exchange. If not specified, will default to the base name of the file path specified in -f.").Short('p').String()
	exPatternVerifyCmd := exPatternCmd.Command("verify", "Verify the signatures of a pattern resource in the Horizon Exchange.")
	exVerPattern := exPatternVerifyCmd.Arg("pattern", "The pattern to verify.").Required().String()
	exPatPubKeyFile := exPatternVerifyCmd.Flag("public-key-file", "The path of a pem public key file to be used to verify the pattern. ").Short('k').Required().ExistingFile()
	exPatDelCmd := exPatternCmd.Command("remove", "Remove a pattern resource from the Horizon Exchange.")
	exDelPat := exPatDelCmd.Arg("pattern", "The pattern to remove.").Required().String()
	exPatDelForce := exPatDelCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()
	exPatternListKeyCmd := exPatternCmd.Command("listkey", "List the signing public keys/certs for this pattern resource in the Horizon Exchange.")
	exPatListKeyPat := exPatternListKeyCmd.Arg("pattern", "The existing pattern to list the keys for.").Required().String()
	exPatListKeyKey := exPatternListKeyCmd.Arg("key-name", "The existing key name to see the contents of.").String()
	exPatternRemKeyCmd := exPatternCmd.Command("removekey", "Remove a signing public key/cert for this pattern resource in the Horizon Exchange.")
	exPatRemKeyPat := exPatternRemKeyCmd.Arg("pattern", "The existing pattern to remove the key from.").Required().String()
	exPatRemKeyKey := exPatternRemKeyCmd.Arg("key-name", "The existing key name to remove.").Required().String()

	exServiceCmd := exchangeCmd.Command("service", "List and manage services in the Horizon Exchange")
	exServiceListCmd := exServiceCmd.Command("list", "Display the service resources from the Horizon Exchange.")
	exService := exServiceListCmd.Arg("service", "List just this one service. Use <org>/<svc> to specify a public service in another org, or <org>/ to list all of the public services in another org.").String()
	exServiceLong := exServiceListCmd.Flag("long", "When listing all of the services, show the entire resource of each services, instead of just the name.").Short('l').Bool()
	exServicePublishCmd := exServiceCmd.Command("publish", "Sign and create/update the service resource in the Horizon Exchange.")
	exSvcJsonFile := exServicePublishCmd.Flag("json-file", "The path of a JSON file containing the metadata necessary to create/update the service in the Horizon exchange. See /usr/horizon/samples/service.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exSvcPrivKeyFile := exServicePublishCmd.Flag("private-key-file", "The path of a private key file to be used to sign the service. ").Short('k').ExistingFile()
	exSvcPubPubKeyFile := exServicePublishCmd.Flag("public-key-file", "The path of public key file (that corresponds to the private key) that should be stored with the service, to be used by the Horizon Agent to verify the signature.").Short('K').ExistingFile()
	exSvcPubDontTouchImage := exServicePublishCmd.Flag("dont-change-image-tag", "The image paths in the deployment field have regular tags and should not be changed to sha256 digest values. This should only be used during development when testing new versions often.").Short('I').Bool()
	exSvcRegistryTokens := exServicePublishCmd.Flag("registry-token", "Docker registry domain and auth that should be stored with the service, to enable the Horizon edge node to access the service's docker images. This flag can be repeated, and each flag should be in the format: registry:user:token").Short('r').Strings()
	exServiceVerifyCmd := exServiceCmd.Command("verify", "Verify the signatures of a service resource in the Horizon Exchange.")
	exVerService := exServiceVerifyCmd.Arg("service", "The service to verify.").Required().String()
	exSvcPubKeyFile := exServiceVerifyCmd.Flag("public-key-file", "The path of a pem public key file to be used to verify the service. ").Short('k').Required().ExistingFile()
	exSvcDelCmd := exServiceCmd.Command("remove", "Remove a service resource from the Horizon Exchange.")
	exDelSvc := exSvcDelCmd.Arg("service", "The service to remove.").Required().String()
	exSvcDelForce := exSvcDelCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()
	exServiceListKeyCmd := exServiceCmd.Command("listkey", "List the signing public keys/certs for this service resource in the Horizon Exchange.")
	exSvcListKeySvc := exServiceListKeyCmd.Arg("service", "The existing service to list the keys for.").Required().String()
	exSvcListKeyKey := exServiceListKeyCmd.Arg("key-name", "The existing key name to see the contents of.").String()
	exServiceRemKeyCmd := exServiceCmd.Command("removekey", "Remove a signing public key/cert for this service resource in the Horizon Exchange.")
	exSvcRemKeySvc := exServiceRemKeyCmd.Arg("service", "The existing service to remove the key from.").Required().String()
	exSvcRemKeyKey := exServiceRemKeyCmd.Arg("key-name", "The existing key name to remove.").Required().String()
	exServiceListAuthCmd := exServiceCmd.Command("listauth", "List the docker auth tokens for this service resource in the Horizon Exchange.")
	exSvcListAuthSvc := exServiceListAuthCmd.Arg("service", "The existing service to list the docker auths for.").Required().String()
	exSvcListAuthId := exServiceListAuthCmd.Arg("auth-name", "The existing docker auth id to see the contents of.").Uint()
	exServiceRemAuthCmd := exServiceCmd.Command("removeauth", "Remove a docker auth token for this service resource in the Horizon Exchange.")
	exSvcRemAuthSvc := exServiceRemAuthCmd.Arg("service", "The existing service to remove the docker auth from.").Required().String()
	exSvcRemAuthId := exServiceRemAuthCmd.Arg("auth-name", "The existing docker auth id to remove.").Required().Uint()

	regInputCmd := app.Command("reginput", "Create an input file template for this pattern that can be used for the 'hzn register' command (once filled in). This examines the services that the specified pattern uses, and determines the node owner input that is required for them.")
	regInputNodeIdTok := regInputCmd.Flag("node-id-tok", "The Horizon exchange node ID and token (it must already exist).").Short('n').PlaceHolder("ID:TOK").Required().String()
	regInputInputFile := regInputCmd.Flag("input-file", "The JSON input template file name that should be created. This file will contain placeholders for you to fill in user input values.").Short('f').Required().String()
	regInputOrg := regInputCmd.Arg("nodeorg", "The Horizon exchange organization ID that the node will be registered in.").Required().String()
	regInputPattern := regInputCmd.Arg("pattern", "The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format.").Required().String()
	regInputArch := regInputCmd.Arg("arch", "The architecture to write the template file for. (Horizon ignores services in patterns whose architecture is different from the target system.) The architecture must be what is returned by 'hzn node list' on the target system.").Default(cutil.ArchString()).String()

	registerCmd := app.Command("register", "Register this edge node with Horizon.")
	nodeIdTok := registerCmd.Flag("node-id-tok", "The Horizon exchange node ID and token. The node ID must be unique within the organization. If not specified, the node ID will be created by Horizon from the machine serial number or fully qualified hostname. If the token is not specified, Horizon will create a random token. If node resource in the exchange identified by the ID and token does not yet exist, you must also specify the -u flag so it can be created.").Short('n').PlaceHolder("ID:TOK").String()
	userPw := registerCmd.Flag("user-pw", "User credentials to create the node resource in the Horizon exchange if it does not already exist.").Short('u').PlaceHolder("USER:PW").String()
	email := registerCmd.Flag("email", "Your email address. Only needs to be specified if: the node resource does not yet exist in the Horizon exchange, and the user specified in the -u flag does not exist, and you specified the 'public' org. If all of these things are true we will create the user and include this value as the email attribute.").Short('e').String()
	inputFile := registerCmd.Flag("input-file", "A JSON file that sets or overrides variables needed by the node, workloads, and microservices that are part of this pattern. See /usr/horizon/samples/input.json and /usr/horizon/samples/more-examples.json. Specify -f- to read from stdin.").Short('f').String() // not using ExistingFile() because it can be - for stdin
	org := registerCmd.Arg("nodeorg", "The Horizon exchange organization ID that the node should be registered in.").Required().String()
	pattern := registerCmd.Arg("pattern", "The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format.").Required().String()

	keyCmd := app.Command("key", "List and manage keys for signing and verifying services.")
	keyListCmd := keyCmd.Command("list", "List the signing keys that have been imported into this Horizon agent.")
	keyName := keyListCmd.Arg("key-name", "The name of a specific key to show.").String()
	keyListAll := keyListCmd.Flag("all", "List the names of all signing keys, even the older public keys not wrapped in a certificate.").Short('a').Bool()
	keyCreateCmd := keyCmd.Command("create", "Generate a signing key pair.")
	keyX509Org := keyCreateCmd.Arg("x509-org", "x509 certificate Organization (O) field (preferably a company name or other organization's name).").Required().String()
	keyX509CN := keyCreateCmd.Arg("x509-cn", "x509 certificate Common Name (CN) field (preferably an email address issued by x509org).").Required().String()
	keyOutputDir := keyCreateCmd.Flag("output-dir", "The directory to put the key pair files in. Defaults to the current directory.").Short('d').Default(".").ExistingDir()
	keyLength := keyCreateCmd.Flag("length", "The length of the key to create.").Short('l').Default("4096").Int()
	keyDaysValid := keyCreateCmd.Flag("days-valid", "x509 certificate validity (Validity > Not After) expressed in days from the day of generation.").Default("1461").Int()
	keyImportFlag := keyCreateCmd.Flag("import", "Automatically import the created public key into the local Horizon agent.").Short('i').Bool()
	keyImportCmd := keyCmd.Command("import", "Imports a signing public key into the Horizon agent.")
	keyImportPubKeyFile := keyImportCmd.Flag("public-key-file", "The path of a pem public key file to be imported. The base name in the path is also used as the key name in the Horizon agent. ").Short('k').Required().ExistingFile()
	keyDelCmd := keyCmd.Command("remove", "Remove the specified signing key from this Horizon agent.")
	keyDelName := keyDelCmd.Arg("key-name", "The name of a specific key to remove.").Required().String()

	nodeCmd := app.Command("node", "List and manage general information about this Horizon edge node.")
	nodeListCmd := nodeCmd.Command("list", "Display general information about this Horizon edge node.")

	agreementCmd := app.Command("agreement", "List or manage the active or archived agreements this edge node has made with a Horizon agreement bot.")
	agreementListCmd := agreementCmd.Command("list", "List the active or archived agreements this edge node has made with a Horizon agreement bot.")
	listAgreementId := agreementListCmd.Arg("agreement-id", "Show the details of this active or archived agreement.").String()
	listArchivedAgreements := agreementListCmd.Flag("archived", "List archived agreements instead of the active agreements.").Short('r').Bool()
	agreementCancelCmd := agreementCmd.Command("cancel", "Cancel 1 or all of the active agreements this edge node has made with a Horizon agreement bot. Usually an agbot will immediately negotiated a new agreement. If you want to cancel all agreements and not have this edge accept new agreements, run 'hzn unregister'.")
	cancelAllAgreements := agreementCancelCmd.Flag("all", "Cancel all of the current agreements.").Short('a').Bool()
	cancelAgreementId := agreementCancelCmd.Arg("agreement-id", "The active agreement to cancel.").String()

	meteringCmd := app.Command("metering", "List or manage the metering (payment) information for the active or archived agreements.")
	meteringListCmd := meteringCmd.Command("list", "List the metering (payment) information for the active or archived agreements.")
	listArchivedMetering := meteringListCmd.Flag("archived", "List archived agreement metering information instead of metering for the active agreements.").Short('r').Bool()

	attributeCmd := app.Command("attribute", "List or manage the global attributes that are currently registered on this Horizon edge node.")
	attributeListCmd := attributeCmd.Command("list", "List the global attributes that are currently registered on this Horizon edge node.")

	serviceCmd := app.Command("service", "List or manage the microservices that are currently registered on this Horizon edge node.")
	serviceListCmd := serviceCmd.Command("list", "List the microservices variable configuration that has been done on this Horizon edge node.")
	serviceRegisteredCmd := serviceCmd.Command("registered", "List the microservices that are currently registered on this Horizon edge node.")

	unregisterCmd := app.Command("unregister", "Unregister and reset this Horizon edge node so that it is ready to be registered again. Warning: this will stop all the Horizon services running on this edge node, and restart the Horizon agent.")

	forceUnregister := unregisterCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()
	removeNodeUnregister := unregisterCmd.Flag("remove", "Also remove this node resource from the Horizon exchange (because you no longer want to use this node with Horizon).").Short('r').Bool()

	statusCmd := app.Command("status", "Display the current horizon internal status for the node.")
	statusLong := statusCmd.Flag("long", "Show detailed status").Short('l').Bool()

	eventlogCmd := app.Command("eventlog", "List the event logs for the current or all registrations.")
	eventlogListCmd := eventlogCmd.Command("list", "List the event logs for the current or all registrations.")
	listAllEventlogs := eventlogListCmd.Flag("all", "List all the event logs including the previous registrations.").Short('a').Bool()
	listDetailedEventlogs := eventlogListCmd.Flag("long", "List event logs with details.").Short('l').Bool()
	listSelectedEventlogs := eventlogListCmd.Flag("select", "Selection string. This flag can be repeated which means 'AND'. Each flag should be in the format of attribute=value, attribute~value, \"attribute>value\" or \"attribute<value\", where '~' means contains. The common attribute names are timestamp, severity, message, event_code, source_type, agreement_id, service_url etc. Use the '-l' flag to see all the attribute names.").Short('s').Strings()

	devCmd := app.Command("dev", "Developmnt tools for creation of workloads and microservices.")
	devHomeDirectory := devCmd.Flag("directory", "Directory containing Horizon project metadata.").Short('d').String()

	devServiceCmd := devCmd.Command("service", "For working with a service project.")
	devServiceNewCmd := devServiceCmd.Command("new", "Create a new service project.")
	devServiceNewCmdOrg := devServiceNewCmd.Flag("org", "The Org id that the service is defined within. If this flag is omitted, the HZN_ORG_ID environment variable is used.").Short('o').String()
	devServiceNewCmdCfg := devServiceNewCmd.Flag("dconfig", "Indicates the type of deployment that will be used, e.g. native (the default), or helm.").Short('c').Default("native").String()
	devServiceStartTestCmd := devServiceCmd.Command("start", "Run a service in a mocked Horizon Agent environment.")
	devServiceUserInputFile := devServiceStartTestCmd.Flag("userInputFile", "File containing user input values for running a test.").Short('f').String()
	devServiceStopTestCmd := devServiceCmd.Command("stop", "Stop a service that is running in a mocked Horizon Agent environment.")
	devServiceValidateCmd := devServiceCmd.Command("verify", "Validate the project for completeness and schema compliance.")
	devServiceVerifyUserInputFile := devServiceValidateCmd.Flag("userInputFile", "File containing user input values for verification of a project.").Short('f').String()

	devDependencyCmd := devCmd.Command("dependency", "For working with project dependencies.")
	devDependencyCmdSpecRef := devDependencyCmd.Flag("specRef", "The URL of the microservice dependency in the exchange. Mutually exclusive with -p and --url.").Short('s').String()
	devDependencyCmdURL := devDependencyCmd.Flag("url", "The URL of the service dependency in the exchange. Mutually exclusive with -p and --specRef.").String()
	devDependencyCmdOrg := devDependencyCmd.Flag("org", "The Org of the service or microservice dependency in the exchange. Mutually exclusive with -p.").Short('o').String()
	devDependencyCmdVersion := devDependencyCmd.Flag("ver", "(optional) The Version of the microservice dependency in the exchange. Mutually exclusive with -p.").String()
	devDependencyCmdArch := devDependencyCmd.Flag("arch", "(optional) The hardware Architecture of the service or microservice dependency in the exchange. Mutually exclusive with -p.").Short('a').String()
	devDependencyFetchCmd := devDependencyCmd.Command("fetch", "Retrieving Horizon metadata for a new dependency.")
	devDependencyFetchCmdProject := devDependencyFetchCmd.Flag("project", "Horizon project containing the definition of a dependency. Mutually exclusive with -s -o --ver -a and --url.").Short('p').ExistingDir()
	devDependencyFetchCmdUserPw := devDependencyFetchCmd.Flag("user-pw", "Horizon Exchange user credentials to query exchange resources. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.").Short('u').PlaceHolder("USER:PW").String()
	devDependencyFetchCmdKeyFiles := devDependencyFetchCmd.Flag("public-key-file", "The path of a public key file to be used to verify a signature.").Short('k').ExistingFiles()
	devDependencyFetchCmdUserInputFile := devDependencyFetchCmd.Flag("userInputFile", "File containing user input values for configuring the new dependency.").Short('f').ExistingFile()
	devDependencyListCmd := devDependencyCmd.Command("list", "List all dependencies.")
	devDependencyRemoveCmd := devDependencyCmd.Command("remove", "Remove a project dependency.")

	agbotCmd := app.Command("agbot", "List and manage Horizon agreement bot resources.")
	agbotListCmd := agbotCmd.Command("list", "Display general information about this Horizon agbot node.")
	agbotAgreementCmd := agbotCmd.Command("agreement", "List or manage the active or archived agreements this Horizon agreement bot has with edge nodes.")
	agbotAgreementListCmd := agbotAgreementCmd.Command("list", "List the active or archived agreements this Horizon agreement bot has with edge nodes.")
	agbotlistArchivedAgreements := agbotAgreementListCmd.Flag("archived", "List archived agreements instead of the active agreements.").Short('r').Bool()
	agbotAgreement := agbotAgreementListCmd.Arg("agreement", "List just this one agreement.").String()
	agbotAgreementCancelCmd := agbotAgreementCmd.Command("cancel", "Cancel 1 or all of the active agreements this Horizon agreement bot has with edge nodes. Usually an agbot will immediately negotiated a new agreement. ")
	agbotCancelAllAgreements := agbotAgreementCancelCmd.Flag("all", "Cancel all of the current agreements.").Short('a').Bool()
	agbotCancelAgreementId := agbotAgreementCancelCmd.Arg("agreement", "The active agreement to cancel.").String()
	agbotPolicyCmd := agbotCmd.Command("policy", "List the policies this Horizon agreement bot hosts.")
	agbotPolicyListCmd := agbotPolicyCmd.Command("list", "List policies this Horizon agreement bot hosts.")
	agbotPolicyOrg := agbotPolicyListCmd.Arg("org", "The organization the policy belongs to.").String()
	agbotPolicyName := agbotPolicyListCmd.Arg("name", "The policy name.").String()
	agbotStatusCmd := agbotCmd.Command("status", "Display the current horizon internal status for the Horizon agreement bot.")
	agbotStatusLong := agbotStatusCmd.Flag("long", "Show detailed status").Short('l').Bool()

	utilCmd := app.Command("util", "Utility commands.")
	utilSignCmd := utilCmd.Command("sign", "Sign the text in stdin. The signature is sent to stdout.")
	utilSignPrivKeyFile := utilSignCmd.Flag("private-key-file", "The path of a private key file to be used to sign the stdin. ").Short('k').Required().ExistingFile()
	utilVerifyCmd := utilCmd.Command("verify", "Verify that the signature specified via -s is a valid signature for the text in stdin.")
	utilVerifyPubKeyFile := utilVerifyCmd.Flag("public-key-file", "The path of public key file (that corresponds to the private key that was used to sign) to verify the signature of stdin.").Short('K').Required().ExistingFile()
	utilVerifySig := utilVerifyCmd.Flag("signature", "The supposed signature of stdin.").Short('s').Required().String()

	app.Version("Run 'hzn version' to see the Horizon version.")
	/* trying to override the base --version behavior does not work....
	fmt.Printf("version: %v\n", *version)
	if *version {
		node.Version()
		os.Exit(0)
	}
	*/

	// Parse cmd and apply env var defaults
	fullCmd := kingpin.MustParse(app.Parse(os.Args[1:]))
	//cliutils.Verbose("Full command: %s", fullCmd)
	if strings.HasPrefix(fullCmd, "exchange") {
		exOrg = cliutils.RequiredWithDefaultEnvVar(exOrg, "HZN_ORG_ID", "organization ID must be specified with either the -o flag or HZN_ORG_ID")
		exUserPw = cliutils.RequiredWithDefaultEnvVar(exUserPw, "HZN_EXCHANGE_USER_AUTH", "exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH")
	}
	if strings.HasPrefix(fullCmd, "register") {
		userPw = cliutils.WithDefaultEnvVar(userPw, "HZN_EXCHANGE_USER_AUTH")
	}

	// Decide which command to run
	switch fullCmd {
	case versionCmd.FullCommand():
		node.Version()
	case exVersionCmd.FullCommand():
		exchange.Version(*exOrg, *exUserPw)
	case exStatusCmd.FullCommand():
		exchange.Status(*exOrg, *exUserPw)
	case exUserListCmd.FullCommand():
		exchange.UserList(*exOrg, *exUserPw, *exUserListAll)
	case exUserCreateCmd.FullCommand():
		exchange.UserCreate(*exOrg, *exUserPw, *exUserCreateUser, *exUserCreatePw, *exUserCreateEmail, *exUserCreateIsAdmin)
	case exUserSetAdminCmd.FullCommand():
		exchange.UserSetAdmin(*exOrg, *exUserPw, *exUserSetAdminUser, *exUserSetAdminBool)
	case exUserDelCmd.FullCommand():
		exchange.UserRemove(*exOrg, *exUserPw, *exDelUser, *exUserDelForce)
	case exNodeListCmd.FullCommand():
		exchange.NodeList(*exOrg, *exUserPw, *exNode, !*exNodeLong)
	case exNodeCreateCmd.FullCommand():
		exchange.NodeCreate(*exOrg, *exNodeCreateNodeIdTok, *exNodeCreateNode, *exNodeCreateToken, *exUserPw, *exNodeCreateNodeEmail)
	case exNodeSetTokCmd.FullCommand():
		exchange.NodeSetToken(*exOrg, *exUserPw, *exNodeSetTokNode, *exNodeSetTokToken)
	case exNodeConfirmCmd.FullCommand():
		exchange.NodeConfirm(*exOrg, *exNodeConfirmNode, *exNodeConfirmToken)
	case exNodeDelCmd.FullCommand():
		exchange.NodeRemove(*exOrg, *exUserPw, *exDelNode, *exNodeDelForce)
	case exAgbotListCmd.FullCommand():
		exchange.AgbotList(*exOrg, *exUserPw, *exAgbot, !*exAgbotLong)
	case exAgbotListPatsCmd.FullCommand():
		exchange.AgbotListPatterns(*exOrg, *exUserPw, *exAgbotLP, *exAgbotLPPatOrg, *exAgbotLPPat, *exAgbotLPNodeOrg)
	case exAgbotAddPatCmd.FullCommand():
		exchange.AgbotAddPattern(*exOrg, *exUserPw, *exAgbotAP, *exAgbotAPPatOrg, *exAgbotAPPat, *exAgbotAPNodeOrg)
	case exAgbotDelPatCmd.FullCommand():
		exchange.AgbotRemovePattern(*exOrg, *exUserPw, *exAgbotDP, *exAgbotDPPatOrg, *exAgbotDPPat, *exAgbotDPNodeOrg)
	case exPatternListCmd.FullCommand():
		exchange.PatternList(*exOrg, *exUserPw, *exPattern, !*exPatternLong)
	case exPatternPublishCmd.FullCommand():
		exchange.PatternPublish(*exOrg, *exUserPw, *exPatJsonFile, *exPatKeyFile, *exPatPubPubKeyFile, *exPatName)
	case exPatternVerifyCmd.FullCommand():
		exchange.PatternVerify(*exOrg, *exUserPw, *exVerPattern, *exPatPubKeyFile)
	case exPatDelCmd.FullCommand():
		exchange.PatternRemove(*exOrg, *exUserPw, *exDelPat, *exPatDelForce)
	case exPatternListKeyCmd.FullCommand():
		exchange.PatternListKey(*exOrg, *exUserPw, *exPatListKeyPat, *exPatListKeyKey)
	case exPatternRemKeyCmd.FullCommand():
		exchange.PatternRemoveKey(*exOrg, *exUserPw, *exPatRemKeyPat, *exPatRemKeyKey)
	case exServiceListCmd.FullCommand():
		exchange.ServiceList(*exOrg, *exUserPw, *exService, !*exServiceLong)
	case exServicePublishCmd.FullCommand():
		exchange.ServicePublish(*exOrg, *exUserPw, *exSvcJsonFile, *exSvcPrivKeyFile, *exSvcPubPubKeyFile, *exSvcPubDontTouchImage, *exSvcRegistryTokens)
	case exServiceVerifyCmd.FullCommand():
		exchange.ServiceVerify(*exOrg, *exUserPw, *exVerService, *exSvcPubKeyFile)
	case exSvcDelCmd.FullCommand():
		exchange.ServiceRemove(*exOrg, *exUserPw, *exDelSvc, *exSvcDelForce)
	case exServiceListKeyCmd.FullCommand():
		exchange.ServiceListKey(*exOrg, *exUserPw, *exSvcListKeySvc, *exSvcListKeyKey)
	case exServiceRemKeyCmd.FullCommand():
		exchange.ServiceRemoveKey(*exOrg, *exUserPw, *exSvcRemKeySvc, *exSvcRemKeyKey)
	case exServiceListAuthCmd.FullCommand():
		exchange.ServiceListAuth(*exOrg, *exUserPw, *exSvcListAuthSvc, *exSvcListAuthId)
	case exServiceRemAuthCmd.FullCommand():
		exchange.ServiceRemoveAuth(*exOrg, *exUserPw, *exSvcRemAuthSvc, *exSvcRemAuthId)
	case regInputCmd.FullCommand():
		register.CreateInputFile(*regInputOrg, *regInputPattern, *regInputArch, *regInputNodeIdTok, *regInputInputFile)
	case registerCmd.FullCommand():
		register.DoIt(*org, *pattern, *nodeIdTok, *userPw, *email, *inputFile)
	case keyListCmd.FullCommand():
		key.List(*keyName, *keyListAll)
	case keyCreateCmd.FullCommand():
		key.Create(*keyX509Org, *keyX509CN, *keyOutputDir, *keyLength, *keyDaysValid, *keyImportFlag)
	case keyImportCmd.FullCommand():
		key.Import(*keyImportPubKeyFile)
	case keyDelCmd.FullCommand():
		key.Remove(*keyDelName)
	case nodeListCmd.FullCommand():
		node.List()
	case agreementListCmd.FullCommand():
		agreement.List(*listArchivedAgreements, *listAgreementId)
	case agreementCancelCmd.FullCommand():
		agreement.Cancel(*cancelAgreementId, *cancelAllAgreements)
	case meteringListCmd.FullCommand():
		metering.List(*listArchivedMetering)
	case attributeListCmd.FullCommand():
		attribute.List()
	case serviceListCmd.FullCommand():
		service.List()
	case serviceRegisteredCmd.FullCommand():
		service.Registered()
	case unregisterCmd.FullCommand():
		unregister.DoIt(*forceUnregister, *removeNodeUnregister)
	case statusCmd.FullCommand():
		status.DisplayStatus(*statusLong, false)
	case eventlogListCmd.FullCommand():
		eventlog.List(*listAllEventlogs, *listDetailedEventlogs, *listSelectedEventlogs)
	case devServiceNewCmd.FullCommand():
		dev.ServiceNew(*devHomeDirectory, *devServiceNewCmdOrg, *devServiceNewCmdCfg)
	case devServiceStartTestCmd.FullCommand():
		dev.ServiceStartTest(*devHomeDirectory, *devServiceUserInputFile)
	case devServiceStopTestCmd.FullCommand():
		dev.ServiceStopTest(*devHomeDirectory)
	case devServiceValidateCmd.FullCommand():
		dev.ServiceValidate(*devHomeDirectory, *devServiceVerifyUserInputFile)
	case devDependencyFetchCmd.FullCommand():
		dev.DependencyFetch(*devHomeDirectory, *devDependencyFetchCmdProject, *devDependencyCmdSpecRef, *devDependencyCmdURL, *devDependencyCmdOrg, *devDependencyCmdVersion, *devDependencyCmdArch, *devDependencyFetchCmdUserPw, *devDependencyFetchCmdKeyFiles, *devDependencyFetchCmdUserInputFile)
	case devDependencyListCmd.FullCommand():
		dev.DependencyList(*devHomeDirectory)
	case devDependencyRemoveCmd.FullCommand():
		dev.DependencyRemove(*devHomeDirectory, *devDependencyCmdSpecRef, *devDependencyCmdURL, *devDependencyCmdVersion, *devDependencyCmdArch)
	case agbotAgreementListCmd.FullCommand():
		agreementbot.AgreementList(*agbotlistArchivedAgreements, *agbotAgreement)
	case agbotAgreementCancelCmd.FullCommand():
		agreementbot.AgreementCancel(*agbotCancelAgreementId, *agbotCancelAllAgreements)
	case agbotListCmd.FullCommand():
		agreementbot.List()
	case agbotPolicyListCmd.FullCommand():
		agreementbot.PolicyList(*agbotPolicyOrg, *agbotPolicyName)
	case utilSignCmd.FullCommand():
		utilcmds.Sign(*utilSignPrivKeyFile)
	case utilVerifyCmd.FullCommand():
		utilcmds.Verify(*utilVerifyPubKeyFile, *utilVerifySig)
	case agbotStatusCmd.FullCommand():
		status.DisplayStatus(*agbotStatusLong, true)
	}
}

// Command line interface to the horizon agent. Provide sub-commands to register an edge node, display info about the node, etc.
package main

import (
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/agreementbot"
	"github.com/open-horizon/anax/cli/attribute"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/key"
	"github.com/open-horizon/anax/cli/metering"
	"github.com/open-horizon/anax/cli/node"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cli/service"
	"github.com/open-horizon/anax/cli/unregister"
	"github.com/open-horizon/anax/cli/workload"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
)

func main() {
	// Command flags and args - see https://github.com/alecthomas/kingpin
	app := kingpin.New("hzn", `Command line interface for Horizon agent. Most of the sub-commands use the Horizon Agent API at the default location http://localhost (see environment Environment Variables section to override this).

Environment Variables:
  HORIZON_URL_BASE: Override the URL at which hzn contacts the Horizon Agent API. This can facilitate using a remote Horizon Agent via an ssh tunnel.
  HORIZON_EXCHANGE_URL_BASE:  Override the URL that the 'hzn exchange' sub-commands use to communicate with the Horizon Exchange, for example https://exchange.bluehorizon.network/api/v1. (By default hzn will ask the Horizon Agent for the URL.)
  USING_API_KEY:  Set this to "1" to indicate that the credential passed into the 'hzn exchange -u' flag is an WIoTP API key/token, not a Horizon Exchange user/password.
`)
	app.HelpFlag.Short('h')
	cliutils.Opts.Verbose = app.Flag("verbose", "Verbose output.").Short('v').Bool()

	exchangeCmd := app.Command("exchange", "List and manage Horizon Exchange resources.")
	exOrg := exchangeCmd.Flag("org", "The Horizon exchange organization ID.").Short('o').Default("public").String()
	exUserPw := exchangeCmd.Flag("user-pw", "Horizon Exchange user credentials to query and create exchange resources. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.").Short('u').PlaceHolder("USER:PW").Required().String()

	userCmd := exchangeCmd.Command("user", "List and manage users in the Horizon Exchange")
	//exUserPw := userCmd.Flag("user-pw", "User credentials in the Horizon exchange.").Short('u').PlaceHolder("USER:PW").Required().String()
	userListCmd := userCmd.Command("list", "Display the user resource from the Horizon Exchange.")
	userCreateCmd := userCmd.Command("create", "Create the user resource in the Horizon Exchange.")
	userCreateEmail := userCreateCmd.Flag("email", "Your email address that should be associated with this user account when creating it in the Horizon exchange.").Short('e').Required().String()

	exNodeCmd := exchangeCmd.Command("node", "List and manage nodes in the Horizon Exchange")
	exNodeListCmd := exNodeCmd.Command("list", "Display the node resources from the Horizon Exchange.")
	exNode := exNodeListCmd.Arg("node", "List just this one node.").String()
	exNodeNames := exNodeListCmd.Flag("names-only", "Only list the names (IDs) of the nodes.").Short('N').Bool()
	exNodeCreateCmd := exNodeCmd.Command("create", "Create the node resource in the Horizon Exchange.")
	exNodeIdTok := exNodeCreateCmd.Flag("node-id-tok", "The Horizon Exchange node ID and token. The node ID must be unique within the organization.").Short('n').PlaceHolder("ID:TOK").Required().String()
	//exNodeUserPw := exNodeCreateCmd.Flag("user-pw", "User credentials to create the node resource in the Horizon exchange.").Short('u').PlaceHolder("USER:PW").Required().String()
	exNodeEmail := exNodeCreateCmd.Flag("email", "Your email address. Only needs to be specified if: the user specified in the -u flag does not exist, and you specified is the 'public' org. If these things are true we will create the user and include this value as the email attribute.").Short('e').String()

	exAgbotCmd := exchangeCmd.Command("agbot", "List and manage agbots in the Horizon Exchange")
	exAgbotListCmd := exAgbotCmd.Command("list", "Display the agbot resources from the Horizon Exchange.")
	exAgbot := exAgbotListCmd.Arg("agbot", "List just this one agbot.").String()
	exAgbotNames := exAgbotListCmd.Flag("names-only", "Only list the names (IDs) of the agbots.").Short('N').Bool()
	exAgbotListPatsCmd := exAgbotCmd.Command("listpattern", "Display the patterns that this agbot is serving.")
	exAgbotLP := exAgbotListPatsCmd.Arg("agbot", "The agbot to list the patterns for.").Required().String()
	exAgbotLPPatOrg := exAgbotListPatsCmd.Arg("patternorg", "The organization of the 1 pattern to list.").String()
	exAgbotLPPat := exAgbotListPatsCmd.Arg("pattern", "The name of the 1 pattern to list.").String()
	exAgbotAddPatCmd := exAgbotCmd.Command("addpattern", "Add this pattern to the list of patterns this agbot is serving.")
	exAgbotAP := exAgbotAddPatCmd.Arg("agbot", "The agbot to add the pattern to.").Required().String()
	exAgbotAPPatOrg := exAgbotAddPatCmd.Arg("patternorg", "The organization of the pattern to add.").Required().String()
	exAgbotAPPat := exAgbotAddPatCmd.Arg("pattern", "The name of the pattern to add.").Required().String()
	exAgbotDelPatCmd := exAgbotCmd.Command("removepattern", "Remove this pattern from the list of patterns this agbot is serving.")
	exAgbotDP := exAgbotDelPatCmd.Arg("agbot", "The agbot to remove the pattern from.").Required().String()
	exAgbotDPPatOrg := exAgbotDelPatCmd.Arg("patternorg", "The organization of the pattern to remove.").Required().String()
	exAgbotDPPat := exAgbotDelPatCmd.Arg("pattern", "The name of the pattern to remove.").Required().String()

	exPatternCmd := exchangeCmd.Command("pattern", "List and manage patterns in the Horizon Exchange")
	//exPatNodeIdTok := exPatternCmd.Flag("node-id-tok", "The Horizon Exchange node ID and token to use to query the exchange. Create with 'hzn exchange node create'.").Short('n').PlaceHolder("ID:TOK").Required().String()
	exPatternListCmd := exPatternCmd.Command("list", "Display the pattern resources from the Horizon Exchange.")
	exPattern := exPatternListCmd.Arg("pattern", "List just this one pattern.").String()
	exPatternNames := exPatternListCmd.Flag("names-only", "Only list the names (IDs) of the patterns.").Short('N').Bool()
	exPatternPublishCmd := exPatternCmd.Command("publish", "Sign and create/update the pattern resource in the Horizon Exchange.")
	exPatJsonFile := exPatternPublishCmd.Flag("json-file", "The path of a JSON file containing the metadata necessary to create/update the pattern in the Horizon exchange. See /usr/horizon/samples/pattern.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exPatKeyFile := exPatternPublishCmd.Flag("private-key-file", "The path of a private key file to be used to sign the pattern. ").Short('k').Required().String()
	exPatternAddWorkCmd := exPatternCmd.Command("insertworkload", "Add or replace a workload in an existing pattern resource in the Horizon Exchange.")
	exPatAddWork := exPatternAddWorkCmd.Arg("pattern", "The existing pattern that the workload should be inserted into.").Required().String()
	exPatAddWorkJsonFile := exPatternAddWorkCmd.Flag("json-file", "The path of a JSON file containing the additional workload metadata. See /usr/horizon/samples/insert-workload-into-pattern.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exPatAddWorkKeyFile := exPatternAddWorkCmd.Flag("private-key-file", "The path of a private key file to be used to sign the inserted workload. ").Short('k').Required().String()

	exWorkloadCmd := exchangeCmd.Command("workload", "List and manage workloads in the Horizon Exchange")
	//exWorkNodeIdTok := exWorkloadCmd.Flag("node-id-tok", "The Horizon Exchange node ID and token to use to query the exchange. Create with 'hzn exchange node create'.").Short('n').PlaceHolder("ID:TOK").Required().String()
	exWorkloadListCmd := exWorkloadCmd.Command("list", "Display the workload resources from the Horizon Exchange.")
	exWorkload := exWorkloadListCmd.Arg("workload", "List just this one workload.").String()
	exWorkloadNames := exWorkloadListCmd.Flag("names-only", "Only list the names (IDs) of the workloads.").Short('N').Bool()
	exWorkloadPublishCmd := exWorkloadCmd.Command("publish", "Sign and create/update the workload resource in the Horizon Exchange.")
	exWorkJsonFile := exWorkloadPublishCmd.Flag("json-file", "The path of a JSON file containing the metadata necessary to create/update the workload in the Horizon exchange. See /usr/horizon/samples/workload.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exWorkPrivKeyFile := exWorkloadPublishCmd.Flag("private-key-file", "The path of a private key file to be used to sign the workload. ").Short('k').Required().ExistingFile()
	//todo: add remove workload from exchange
	exWorkloadVerifyCmd := exWorkloadCmd.Command("verify", "Verify the signatures of the workload resource in the Horizon Exchange.")
	exVerWorkload := exWorkloadVerifyCmd.Arg("workload", "The workload to verify.").Required().String()
	exWorkPubKeyFile := exWorkloadVerifyCmd.Flag("public-key-file", "The path of a pem public key file to be used to verify the workload. ").Short('k').Required().ExistingFile()

	exMicroserviceCmd := exchangeCmd.Command("microservice", "List and manage microservices in the Horizon Exchange")
	//exMicroNodeIdTok := exMicroserviceCmd.Flag("node-id-tok", "The Horizon Exchange node ID and token to use to query the exchange. Create with 'hzn exchange node create'.").Short('n').PlaceHolder("ID:TOK").Required().String()
	exMicroserviceListCmd := exMicroserviceCmd.Command("list", "Display the microservice resources from the Horizon Exchange.")
	exMicroservice := exMicroserviceListCmd.Arg("microservice", "List just this one microservice.").String()
	exMicroserviceNames := exMicroserviceListCmd.Flag("names-only", "Only list the names (IDs) of the microservices.").Short('N').Bool()
	exMicroservicePublishCmd := exMicroserviceCmd.Command("publish", "Sign and create/update the microservice resource in the Horizon Exchange.")
	exMicroJsonFile := exMicroservicePublishCmd.Flag("json-file", "The path of a JSON file containing the metadata necessary to create/update the microservice in the Horizon exchange. See /usr/horizon/samples/microservice.json. Specify -f- to read from stdin.").Short('f').Required().String()
	exMicroKeyFile := exMicroservicePublishCmd.Flag("private-key-file", "The path of a private key file to be used to sign the microservice. ").Short('k').Required().String()

	registerCmd := app.Command("register", "Register this edge node with Horizon.")
	nodeIdTok := registerCmd.Flag("node-id-tok", "The Horizon exchange node ID and token. The node ID must be unique within the organization. If not specified, the node ID will be created by Horizon from the machine serial number or fully qualified hostname. If the token is not specified, Horizon will create a random token. If node resource in the exchange identified by the ID and token does not yet exist, you must also specify the -u flag so it can be created.").Short('n').PlaceHolder("ID:TOK").String()
	userPw := registerCmd.Flag("user-pw", "User credentials to create the node resource in the Horizon exchange if it does not already exist.").Short('u').PlaceHolder("USER:PW").String()
	email := registerCmd.Flag("email", "Your email address. Only needs to be specified if: the node resource does not yet exist in the Horizon exchange, and the user specified in the -u flag does not exist, and you specified is the 'public' org. If all of these things are true we will create the user and include this value as the email attribute.").Short('e').String()
	inputFile := registerCmd.Flag("input-file", "A JSON file that sets or overrides variables needed by the node, workloads, and microservices that are part of this pattern. See /usr/horizon/samples/input.json and /usr/horizon/samples/more-examples.json. Specify -f- to read from stdin.").Short('f').String()
	org := registerCmd.Arg("organization", "The Horizon exchange organization ID.").Required().String()
	pattern := registerCmd.Arg("pattern", "The Horizon exchange pattern that describes what workloads that should be deployed to this node.").Required().String()

	keyCmd := app.Command("key", "List and manage keys for signing and verifying services.")
	keyListCmd := keyCmd.Command("list", "List the signing keys that have been imported into this Horizon agent.")
	keyCreateCmd := keyCmd.Command("create", "Generate a signing key pair.")
	keyX509Org := keyCreateCmd.Arg("x509-org", "x509 certificate Organization (O) field (preferrably a company name or other organization's name).").Required().String()
	keyX509CN := keyCreateCmd.Arg("x509-cn", "x509 certificate Common Name (CN) field (preferrably an email address issued by x509org).").Required().String()
	keyOutputDir := keyCreateCmd.Flag("output-dir", "The directory to put the key pair files in. Defaults to the current directory.").Short('d').Default(".").ExistingDir()
	keyLength := keyCreateCmd.Flag("length", "The directory to put the key pair files in. Defaults to the current directory.").Short('l').Default("8192").Int()
	keyDaysValid := keyCreateCmd.Flag("days-valid", "x509 certificate validity (Validity > Not After) expressed in days from the day of generation.").Default("1461").Int()
	//todo: add import option

	nodeCmd := app.Command("node", "List and manage general information about this Horizon edge node.")
	nodeListCmd := nodeCmd.Command("list", "Display general information about this Horizon edge node.")

	agreementCmd := app.Command("agreement", "List or manage the active or archived agreements this edge node has made with a Horizon agreement bot.")
	agreementListCmd := agreementCmd.Command("list", "List the active or archived agreements this edge node has made with a Horizon agreement bot.")
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
	serviceListCmd := serviceCmd.Command("list", "List the microservices that are currently registered on this Horizon edge node.")

	workloadCmd := app.Command("workload", "List or manage the workloads that are currently registered on this Horizon edge node.")
	workloadListCmd := workloadCmd.Command("list", "List the workloads that are currently registered on this Horizon edge node.")

	unregisterCmd := app.Command("unregister", "Unregister and reset this Horizon edge node so that it is ready to be registered again. Warning: this will stop all the Horizon workloads running on this edge node, and restart the Horizon agent.")
	forceUnregister := unregisterCmd.Flag("force", "Skip the 'are you sure?' prompt.").Short('f').Bool()
	removeNodeUnregister := unregisterCmd.Flag("remove", "Also remove this node resource from the Horizon exchange (because you no longer want to use this node with Horizon).").Short('r').Bool()

	devCmd := app.Command("dev", "Developmnt tools for creation of workloads and microservices.")
	devHomeDirectory := devCmd.Flag("directory", "Directory containing Horizon project metadata.").Short('d').String()

	devWorkloadCmd := devCmd.Command("workload", "For working with a workload project.")
	devWorkloadNewCmd := devWorkloadCmd.Command("new", "Create a new workload project.")
	devWorkloadStartTestCmd := devWorkloadCmd.Command("start", "Run a workload in a mocked Horizon Agent environment.")
	devWorkloadUserInputFile := devWorkloadStartTestCmd.Flag("userInputFile", "File containing user input values for running a test.").Short('f').String()
	devWorkloadStopTestCmd := devWorkloadCmd.Command("stop", "Stop a workload that is running in a mocked Horizon Agent environment.")
	devWorkloadDeployCmd := devWorkloadCmd.Command("deploy", "Deploy a workload to a Horizon Exchange.")
	devWorkloadValidateCmd := devWorkloadCmd.Command("verify", "Validate the project for completeness and schema compliance.")

	devMicroserviceCmd := devCmd.Command("microservice", "For working with a microservice project.")
	devMicroserviceNewCmd := devMicroserviceCmd.Command("new", "Create a new microservice project.")
	devMicroserviceStartTestCmd := devMicroserviceCmd.Command("start", "Run a microservice in a mocked Horizon Agent environment.")
	devMicroserviceUserInputFile := devMicroserviceStartTestCmd.Flag("userInputFile", "File containing user input values for running a test.").Short('f').String()
	devMicroserviceStopTestCmd := devMicroserviceCmd.Command("stop", "Stop a microservice that is running in a mocked Horizon Agent environment.")
	devMicroserviceDeployCmd := devMicroserviceCmd.Command("deploy", "Deploy a microservice to a Horizon Exchange.")
	devMicroserviceValidateCmd := devMicroserviceCmd.Command("verify", "Validate the project for completeness and schema compliance.")

	devDependencyCmd := devCmd.Command("dependency", "For working with project dependencies.")
	devDependencyFetchCmd := devDependencyCmd.Command("fetch", "Retrieving Horizon metadata for a new dependency.")
	devDependencyListCmd := devDependencyCmd.Command("list", "List all dependencies.")

	agbotCmd := app.Command("agbot", "List and manage Horizon agreement bot resources.")

	agbotAgreementCmd := agbotCmd.Command("agreement", "List or manage the active or archived agreements this Horizon agreement bot has with edge nodes.")
	agbotAgreementListCmd := agbotAgreementCmd.Command("list", "List the active or archived agreements this Horizon agreement bot has with edge nodes.")
	agbotlistArchivedAgreements := agbotAgreementListCmd.Flag("archived", "List archived agreements instead of the active agreements.").Short('r').Bool()
	agbotAgreement := agbotAgreementListCmd.Arg("agreement", "List just this one agreement.").String()
	agbotAgreementCancelCmd := agbotAgreementCmd.Command("cancel", "Cancel 1 or all of the active agreements this Horizon agreement bot has with edge nodes. Usually an agbot will immediately negotiated a new agreement. ")
	agbotCancelAllAgreements := agbotAgreementCancelCmd.Flag("all", "Cancel all of the current agreements.").Short('a').Bool()
	agbotCancelAgreementId := agbotAgreementCancelCmd.Arg("agreement", "The active agreement to cancel.").String()

	app.Version("0.0.3") //todo: get the real version of anax

	// Decide which command to run
	switch kingpin.MustParse(app.Parse(os.Args[1:])) {
	case userListCmd.FullCommand():
		exchange.UserList(*exOrg, *exUserPw)
	case userCreateCmd.FullCommand():
		exchange.UserCreate(*exOrg, *exUserPw, *userCreateEmail)
	case exNodeListCmd.FullCommand():
		exchange.NodeList(*exOrg, *exUserPw, *exNode, *exNodeNames)
	case exNodeCreateCmd.FullCommand():
		exchange.NodeCreate(*exOrg, *exNodeIdTok, *exUserPw, *exNodeEmail)
	case exAgbotListCmd.FullCommand():
		exchange.AgbotList(*exOrg, *exUserPw, *exAgbot, *exAgbotNames)
	case exAgbotListPatsCmd.FullCommand():
		exchange.AgbotListPatterns(*exOrg, *exUserPw, *exAgbotLP, *exAgbotLPPatOrg, *exAgbotLPPat)
	case exAgbotAddPatCmd.FullCommand():
		exchange.AgbotAddPattern(*exOrg, *exUserPw, *exAgbotAP, *exAgbotAPPatOrg, *exAgbotAPPat)
	case exAgbotDelPatCmd.FullCommand():
		exchange.AgbotDeletePattern(*exOrg, *exUserPw, *exAgbotDP, *exAgbotDPPatOrg, *exAgbotDPPat)
	case exPatternListCmd.FullCommand():
		exchange.PatternList(*exOrg, *exUserPw, *exPattern, *exPatternNames)
	case exPatternPublishCmd.FullCommand():
		exchange.PatternPublish(*exOrg, *exUserPw, *exPatJsonFile, *exPatKeyFile)
	case exPatternAddWorkCmd.FullCommand():
		exchange.PatternAddWorkload(*exOrg, *exUserPw, *exPatAddWork, *exPatAddWorkJsonFile, *exPatAddWorkKeyFile)
	case exWorkloadListCmd.FullCommand():
		exchange.WorkloadList(*exOrg, *exUserPw, *exWorkload, *exWorkloadNames)
	case exWorkloadPublishCmd.FullCommand():
		exchange.WorkloadPublish(*exOrg, *exUserPw, *exWorkJsonFile, *exWorkPrivKeyFile)
	case exWorkloadVerifyCmd.FullCommand():
		exchange.WorkloadVerify(*exOrg, *exUserPw, *exVerWorkload, *exWorkPubKeyFile)
	case exMicroserviceListCmd.FullCommand():
		exchange.MicroserviceList(*exOrg, *exUserPw, *exMicroservice, *exMicroserviceNames)
	case exMicroservicePublishCmd.FullCommand():
		exchange.MicroservicePublish(*exOrg, *exUserPw, *exMicroJsonFile, *exMicroKeyFile)
	case registerCmd.FullCommand():
		register.DoIt(*org, *pattern, *nodeIdTok, *userPw, *email, *inputFile)
	case keyListCmd.FullCommand():
		key.List()
	//case keyCmd.FullCommand():   // <- I'd like to just default to list in this case, but don't know how to do that yet
	//	keyCmd.List()
	case keyCreateCmd.FullCommand():
		key.Create(*keyX509Org, *keyX509CN, *keyOutputDir, *keyLength, *keyDaysValid)
	case nodeListCmd.FullCommand():
		node.List()
	case agreementListCmd.FullCommand():
		agreement.List(*listArchivedAgreements)
	case agreementCancelCmd.FullCommand():
		agreement.Cancel(*cancelAgreementId, *cancelAllAgreements)
	case meteringListCmd.FullCommand():
		metering.List(*listArchivedMetering)
	case attributeListCmd.FullCommand():
		attribute.List()
	case serviceListCmd.FullCommand():
		service.List()
	case workloadListCmd.FullCommand():
		workload.List()
	case unregisterCmd.FullCommand():
		unregister.DoIt(*forceUnregister, *removeNodeUnregister)
	case devWorkloadNewCmd.FullCommand():
		dev.WorkloadNew(*devHomeDirectory)
	case devWorkloadStartTestCmd.FullCommand():
		dev.WorkloadStartTest(*devHomeDirectory, *devWorkloadUserInputFile)
	case devWorkloadStopTestCmd.FullCommand():
		dev.WorkloadStopTest(*devHomeDirectory)
	case devWorkloadValidateCmd.FullCommand():
		dev.WorkloadValidate(*devHomeDirectory)
	case devWorkloadDeployCmd.FullCommand():
		dev.WorkloadDeploy(*devHomeDirectory)
	case devMicroserviceNewCmd.FullCommand():
		dev.MicroserviceNew(*devHomeDirectory)
	case devMicroserviceStartTestCmd.FullCommand():
		dev.MicroserviceStartTest(*devHomeDirectory, *devMicroserviceUserInputFile)
	case devMicroserviceStopTestCmd.FullCommand():
		dev.MicroserviceStopTest(*devHomeDirectory)
	case devMicroserviceValidateCmd.FullCommand():
		dev.MicroserviceValidate(*devHomeDirectory)
	case devMicroserviceDeployCmd.FullCommand():
		dev.MicroserviceDeploy(*devHomeDirectory)
	case devDependencyFetchCmd.FullCommand():
		dev.DependencyFetch(*devHomeDirectory)
	case devDependencyListCmd.FullCommand():
		dev.DependencyList(*devHomeDirectory)
	case agbotAgreementListCmd.FullCommand():
		agreementbot.AgreementList(*agbotlistArchivedAgreements, *agbotAgreement)
	case agbotAgreementCancelCmd.FullCommand():
		agreementbot.AgreementCancel(*agbotCancelAgreementId, *agbotCancelAllAgreements)
	}
}

// Command line interface to the horizon agent. Provide sub-commands to register an edge node, display info about the node, etc.
package main

import (
	"flag"
	"os"
	"strings"

	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/agreementbot"
	"github.com/open-horizon/anax/cli/attribute"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/deploycheck"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/eventlog"
	"github.com/open-horizon/anax/cli/exchange"
	_ "github.com/open-horizon/anax/cli/helm_deployment"
	_ "github.com/open-horizon/anax/cli/i18n_messages"
	"github.com/open-horizon/anax/cli/key"
	"github.com/open-horizon/anax/cli/metering"
	_ "github.com/open-horizon/anax/cli/native_deployment"
	"github.com/open-horizon/anax/cli/node"
	"github.com/open-horizon/anax/cli/policy"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cli/service"
	"github.com/open-horizon/anax/cli/status"
	"github.com/open-horizon/anax/cli/sync_service"
	"github.com/open-horizon/anax/cli/unregister"
	"github.com/open-horizon/anax/cli/userinput"
	"github.com/open-horizon/anax/cli/utilcmds"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"gopkg.in/alecthomas/kingpin.v2"
)

func main() {
	// Shut off the Anax runtime logging, so functions reused from anax don't fight with the kingpin parsing of args/flags.
	// Also, in the reused code need to change any calls like glog.Infof("some string") to glog.V(3).Infof("some string")
	flag.Set("v", "0")

	// initialize the message printer for globalization for the cliconfig.SetEnvVarsFromConfigFiles("") call
	if err := i18n.InitMessagePrinter(false); err != nil {
		cliutils.Verbose("%v. The messages will be displayed in English.", err)
		i18n.InitMessagePrinter(true)
	}

	// set up environment variables from the cli package configuration file and user configuration file.
	cliconfig.SetEnvVarsFromConfigFiles("")

	// initialize the message printer for globalization again because HZN_LANG could have changed from the above call.
	if err := i18n.InitMessagePrinter(false); err != nil {
		cliutils.Verbose("%v. The messages will be displayed in English.", err)
		i18n.InitMessagePrinter(true)
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Command flags and args - see https://github.com/alecthomas/kingpin
	app := kingpin.New("hzn", msgPrinter.Sprintf(`Command line interface for Horizon agent. Most of the sub-commands use the Horizon Agent API at the default location http://localhost (see environment Environment Variables section to override this).

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
	  register -u' flag, in the form '[org/]user:pw'. Notice that HZN_ORG_ID can be set
	  if org is omitted when HZN_EXCHANGE_USER_AUTH is set.
  HZN_FSS_CSSURL:  Override the URL that the 'hzn mms' sub-commands use
      to communicate with the Horizon Model Management Service, for example
      https://exchange.bluehorizon.network/css/. (By default hzn will ask the
      Horizon Agent for the URL.)

  All these environment variables and ones mentioned in the command help can be
  specified in user's configuration file: ~/.hzn/hzn.json with JSON format.
  For example:
  %s
  `, `{
    "HZN_ORG_ID": "me@mycomp.com"
  }
`))
	app.HelpFlag.Short('h')
	app.UsageTemplate(kingpin.CompactUsageTemplate)
	cliutils.Opts.Verbose = app.Flag("verbose", msgPrinter.Sprintf("Verbose output.")).Short('v').Bool()
	cliutils.Opts.IsDryRun = app.Flag("dry-run", msgPrinter.Sprintf("When calling the Horizon or Exchange API, do GETs, but don't do PUTs, POSTs, or DELETEs.")).Bool()

	envCmd := app.Command("env", msgPrinter.Sprintf("Show the Horizon Environment Variables."))
	envOrg := envCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	envUserPw := envCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query and create exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value. As an alternative to using -o, you can set HZN_ORG_ID with the Horizon exchange organization ID")).Short('u').PlaceHolder("USER:PW").String()
	envExchUrl := cliutils.GetExchangeUrl()
	envCcsUrl := cliutils.GetMMSUrl()

	versionCmd := app.Command("version", msgPrinter.Sprintf("Show the Horizon version.")) // using a cmd for this instead of --version flag, because kingpin takes over the latter and can't get version only when it is needed
	archCmd := app.Command("architecture", msgPrinter.Sprintf("Show the architecture of this machine (as defined by Horizon and golang)."))

	exchangeCmd := app.Command("exchange", msgPrinter.Sprintf("List and manage Horizon Exchange resources."))
	exOrg := exchangeCmd.Flag("org", msgPrinter.Sprintf("The Horizon exchange organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	exUserPw := exchangeCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query and create exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value. As an alternative to using -o, you can set HZN_ORG_ID with the Horizon exchange organization ID")).Short('u').PlaceHolder("USER:PW").String()

	exVersionCmd := exchangeCmd.Command("version", msgPrinter.Sprintf("Display the version of the Horizon Exchange."))
	exStatusCmd := exchangeCmd.Command("status", msgPrinter.Sprintf("Display the status of the Horizon Exchange."))

	exUserCmd := exchangeCmd.Command("user", msgPrinter.Sprintf("List and manage users in the Horizon Exchange."))
	exUserListCmd := exUserCmd.Command("list", msgPrinter.Sprintf("Display the user resource from the Horizon Exchange. (Normally you can only display your own user. If the user does not exist, you will get an invalid credentials error.)"))
	exUserListUser := exUserListCmd.Arg("user", msgPrinter.Sprintf("List this one user. Default is your own user. Only admin users can list other users.")).String()
	exUserListAll := exUserListCmd.Flag("all", msgPrinter.Sprintf("List all users in the org. Will only do this if you are a user with admin privilege.")).Short('a').Bool()
	exUserListNamesOnly := exUserListCmd.Flag("names", msgPrinter.Sprintf("When listing all of the users, show only the usernames, instead of each entire resource.")).Short('N').Bool()
	exUserCreateCmd := exUserCmd.Command("create", msgPrinter.Sprintf("Create the user resource in the Horizon Exchange."))
	exUserCreateUser := exUserCreateCmd.Arg("user", msgPrinter.Sprintf("Your username for this user account when creating it in the Horizon exchange.")).Required().String()
	exUserCreatePw := exUserCreateCmd.Arg("pw", msgPrinter.Sprintf("Your password for this user account when creating it in the Horizon exchange.")).Required().String()
	exUserCreateEmail := exUserCreateCmd.Arg("email", msgPrinter.Sprintf("Your email address that should be associated with this user account when creating it in the Horizon exchange. If your username is an email address, this argument can be omitted.")).String()
	exUserCreateIsAdmin := exUserCreateCmd.Flag("admin", msgPrinter.Sprintf("This user should be an administrator, capable of managing all resources in this org of the exchange.")).Short('A').Bool()
	exUserSetAdminCmd := exUserCmd.Command("setadmin", msgPrinter.Sprintf("Change the existing user to be an admin user (like root in his/her org) or to no longer be an admin user. Can only be run by exchange root or another admin user."))
	exUserSetAdminUser := exUserSetAdminCmd.Arg("user", msgPrinter.Sprintf("The user to be modified.")).Required().String()
	exUserSetAdminBool := exUserSetAdminCmd.Arg("isadmin", msgPrinter.Sprintf("True if they should be an admin user, otherwise false.")).Required().Bool()
	exUserDelCmd := exUserCmd.Command("remove", msgPrinter.Sprintf("Remove a user resource from the Horizon Exchange. Warning: this will cause all exchange resources owned by this user to also be deleted (nodes, services, patterns, etc)."))
	exDelUser := exUserDelCmd.Arg("user", msgPrinter.Sprintf("The user to remove.")).Required().String()
	exUserDelForce := exUserDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()

	exNodeCmd := exchangeCmd.Command("node", msgPrinter.Sprintf("List and manage nodes in the Horizon Exchange"))
	exNodeListCmd := exNodeCmd.Command("list", msgPrinter.Sprintf("Display the node resources from the Horizon Exchange."))
	exNode := exNodeListCmd.Arg("node", msgPrinter.Sprintf("List just this one node.")).String()
	exNodeListNodeIdTok := exNodeListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeLong := exNodeListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the nodes, show the entire resource of each node, instead of just the name.")).Short('l').Bool()
	exNodeCreateCmd := exNodeCmd.Command("create", msgPrinter.Sprintf("Create the node resource in the Horizon Exchange."))
	exNodeCreateNodeIdTok := exNodeCreateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be created. The node ID must be unique within the organization.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeCreateNodeEmail := exNodeCreateCmd.Flag("email", msgPrinter.Sprintf("Your email address. Only needs to be specified if: the user specified in the -u flag does not exist, and you specified the 'public' org. If these things are true we will create the user and include this value as the email attribute.")).Short('e').String()
	exNodeCreateNodeArch := exNodeCreateCmd.Flag("arch", msgPrinter.Sprintf("Your node architecture. If not specified, arch will leave blank.")).Short('a').String()
	exNodeCreateNodeName := exNodeCreateCmd.Flag("name", msgPrinter.Sprintf("The name of your node")).Short('m').String()
	exNodeCreateNode := exNodeCreateCmd.Arg("node", msgPrinter.Sprintf("The node to be created.")).String()
	exNodeCreateToken := exNodeCreateCmd.Arg("token", msgPrinter.Sprintf("The token the new node should have.")).String()
	exNodeUpdateCmd := exNodeCmd.Command("update", msgPrinter.Sprintf("Update an attribute of the node in the Horizon Exchange."))
	exNodeUpdateNode := exNodeUpdateCmd.Arg("node", msgPrinter.Sprintf("The node to be updated.")).Required().String()
	exNodeUpdateIdTok := exNodeUpdateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeUpdateJsonFile := exNodeUpdateCmd.Flag("json-file", msgPrinter.Sprintf("The path to a json file containing the changed attribute to be updated in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exNodeSetTokCmd := exNodeCmd.Command("settoken", msgPrinter.Sprintf("Change the token of a node resource in the Horizon Exchange."))
	exNodeSetTokNode := exNodeSetTokCmd.Arg("node", msgPrinter.Sprintf("The node to be changed.")).Required().String()
	exNodeSetTokToken := exNodeSetTokCmd.Arg("token", msgPrinter.Sprintf("The new token for the node.")).Required().String()
	exNodeSetTokNodeIdTok := exNodeSetTokCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeConfirmCmd := exNodeCmd.Command("confirm", msgPrinter.Sprintf("Check to see if the specified node and token are valid in the Horizon Exchange."))
	exNodeConfirmNodeIdTok := exNodeConfirmCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token to be checked. If not specified, HZN_EXCHANGE_NODE_AUTH will be used as a default. Mutually exclusive with <node> and <token> arguments.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeConfirmNode := exNodeConfirmCmd.Arg("node", msgPrinter.Sprintf("The node id to be checked. Mutually exclusive with -n flag.")).String()
	exNodeConfirmToken := exNodeConfirmCmd.Arg("token", msgPrinter.Sprintf("The token for the node. Mutually exclusive with -n flag.")).String()
	exNodeDelCmd := exNodeCmd.Command("remove", msgPrinter.Sprintf("Remove a node resource from the Horizon Exchange. Do NOT do this when an edge node is registered with this node id."))
	exNodeRemoveNodeIdTok := exNodeDelCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modfy the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exDelNode := exNodeDelCmd.Arg("node", msgPrinter.Sprintf("The node to remove.")).Required().String()
	exNodeDelForce := exNodeDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exNodeListPolicyCmd := exNodeCmd.Command("listpolicy", msgPrinter.Sprintf("Display the node policy from the Horizon Exchange."))
	exNodeListPolicyIdTok := exNodeListPolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeListPolicyNode := exNodeListPolicyCmd.Arg("node", msgPrinter.Sprintf("List policy for this node.")).Required().String()
	exNodeAddPolicyCmd := exNodeCmd.Command("addpolicy", msgPrinter.Sprintf("Add or replace the node policy in the Horizon Exchange."))
	exNodeAddPolicyIdTok := exNodeAddPolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeAddPolicyNode := exNodeAddPolicyCmd.Arg("node", msgPrinter.Sprintf("Add or replace policy for this node.")).Required().String()
	exNodeAddPolicyJsonFile := exNodeAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the node policy in the Horizon exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exNodeUpdatePolicyCmd := exNodeCmd.Command("updatepolicy", msgPrinter.Sprintf("Update an attribute of the policy for this node in the Horizon Exchange."))
	exNodeUpdatePolicyNode := exNodeUpdatePolicyCmd.Arg("node", msgPrinter.Sprintf("Update the policy for this node.")).Required().String()
	exNodeUpdatePolicyIdTok := exNodeUpdatePolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeUpdatePolicyJsonFile := exNodeUpdatePolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the attribute to be updated in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exNodeRemovePolicyCmd := exNodeCmd.Command("removepolicy", msgPrinter.Sprintf("Remove the node policy in the Horizon Exchange."))
	exNodeRemovePolicyIdTok := exNodeRemovePolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeRemovePolicyNode := exNodeRemovePolicyCmd.Arg("node", msgPrinter.Sprintf("Remove policy for this node.")).Required().String()
	exNodeRemovePolicyForce := exNodeRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exNodeErrorsList := exNodeCmd.Command("listerrors", msgPrinter.Sprintf("List the node errors currently surfaced to the exchange."))
	exNodeErrorsListIdTok := exNodeErrorsList.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeErrorsListNode := exNodeErrorsList.Arg("node", msgPrinter.Sprintf("List surfaced errors for this node.")).Required().String()
	exNodeErrorsListLong := exNodeErrorsList.Flag("long", msgPrinter.Sprintf("Show the full eventlog object of the errors currently surfaced to the exchange.")).Short('l').Bool()

	exAgbotCmd := exchangeCmd.Command("agbot", msgPrinter.Sprintf("List and manage agbots in the Horizon Exchange"))
	exAgbotListCmd := exAgbotCmd.Command("list", msgPrinter.Sprintf("Display the agbot resources from the Horizon Exchange."))
	exAgbot := exAgbotListCmd.Arg("agbot", msgPrinter.Sprintf("List just this one agbot.")).String()
	exAgbotLong := exAgbotListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the agbots, show the entire resource of each agbots, instead of just the name.")).Short('l').Bool()
	exAgbotListPatsCmd := exAgbotCmd.Command("listpattern", msgPrinter.Sprintf("Display the patterns that this agbot is serving."))
	exAgbotLP := exAgbotListPatsCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to list the patterns for.")).Required().String()
	exAgbotLPPatOrg := exAgbotListPatsCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the 1 pattern to list.")).String()
	exAgbotLPPat := exAgbotListPatsCmd.Arg("pattern", msgPrinter.Sprintf("The name of the 1 pattern to list.")).String()
	exAgbotLPNodeOrg := exAgbotListPatsCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()
	exAgbotAddPatCmd := exAgbotCmd.Command("addpattern", msgPrinter.Sprintf("Add this pattern to the list of patterns this agbot is serving."))
	exAgbotAP := exAgbotAddPatCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to add the pattern to.")).Required().String()
	exAgbotAPPatOrg := exAgbotAddPatCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the pattern to add.")).Required().String()
	exAgbotAPPat := exAgbotAddPatCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern to add.")).Required().String()
	exAgbotAPNodeOrg := exAgbotAddPatCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()
	exAgbotDelPatCmd := exAgbotCmd.Command("removepattern", msgPrinter.Sprintf("Remove this pattern from the list of patterns this agbot is serving."))
	exAgbotDP := exAgbotDelPatCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to remove the pattern from.")).Required().String()
	exAgbotDPPatOrg := exAgbotDelPatCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the pattern to remove.")).Required().String()
	exAgbotDPPat := exAgbotDelPatCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern to remove.")).Required().String()
	exAgbotDPNodeOrg := exAgbotDelPatCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()
	exAgbotListPolicyCmd := exAgbotCmd.Command("listbusinesspol", msgPrinter.Sprintf("Display the business policies that this agbot is serving."))
	exAgbotPol := exAgbotListPolicyCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to list serving business policies for.")).Required().String()
	exAgbotAddPolCmd := exAgbotCmd.Command("addbusinesspol", msgPrinter.Sprintf("Add this business policy to the list of policies this agbot is serving. Currently only support adding all the business polycies from an organization."))
	exAgbotAPolAg := exAgbotAddPolCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to add the business policy to.")).Required().String()
	exAgbotAPPolOrg := exAgbotAddPolCmd.Arg("policyorg", msgPrinter.Sprintf("The organization of the business policy to add.")).Required().String()
	exAgbotDelPolCmd := exAgbotCmd.Command("removebusinesspol", msgPrinter.Sprintf("Remove this business policy from the list of policies this agbot is serving. Currently only support removing all the business polycies from an organization."))
	exAgbotDPolAg := exAgbotDelPolCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to remove the business policy from.")).Required().String()
	exAgbotDPPolOrg := exAgbotDelPolCmd.Arg("policyorg", msgPrinter.Sprintf("The organization of the business policy to remove.")).Required().String()

	exPatternCmd := exchangeCmd.Command("pattern", msgPrinter.Sprintf("List and manage patterns in the Horizon Exchange"))
	exPatternListCmd := exPatternCmd.Command("list", msgPrinter.Sprintf("Display the pattern resources from the Horizon Exchange."))
	exPatternListNodeIdTok := exPatternListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPattern := exPatternListCmd.Arg("pattern", msgPrinter.Sprintf("List just this one pattern. Use <org>/<pat> to specify a public pattern in another org, or <org>/ to list all of the public patterns in another org.")).String()
	exPatternLong := exPatternListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the patterns, show the entire resource of each pattern, instead of just the name.")).Short('l').Bool()
	exPatternPublishCmd := exPatternCmd.Command("publish", msgPrinter.Sprintf("Sign and create/update the pattern resource in the Horizon Exchange."))
	exPatJsonFile := exPatternPublishCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the pattern in the Horizon exchange. See /usr/horizon/samples/pattern.json. Specify -f- to read from stdin.")).Short('f').Required().String()
	exPatKeyFile := exPatternPublishCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the pattern. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.private.key is the default.")).Short('k').ExistingFile()
	exPatPubPubKeyFile := exPatternPublishCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of public key file (that corresponds to the private key) that should be stored with the pattern, to be used by the Horizon Agent to verify the signature. If both this and -k flags are not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If HZN_PUBLIC_KEY_FILE is not set, ~/.hzn/keys/service.public.pem is the default. If -k is specified and this flag is not specified, then no public key file will be stored with the pattern. The Horizon Agent needs to import the public key to verify the signature.")).Short('K').ExistingFile()
	exPatName := exPatternPublishCmd.Flag("pattern-name", msgPrinter.Sprintf("The name to use for this pattern in the Horizon exchange. If not specified, will default to the base name of the file path specified in -f.")).Short('p').String()
	exPatternVerifyCmd := exPatternCmd.Command("verify", msgPrinter.Sprintf("Verify the signatures of a pattern resource in the Horizon Exchange."))
	exVerPattern := exPatternVerifyCmd.Arg("pattern", msgPrinter.Sprintf("The pattern to verify.")).Required().String()
	exPatternVerifyNodeIdTok := exPatternVerifyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatPubKeyFile := exPatternVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be used to verify the pattern. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()
	exPatUpdateCmd := exPatternCmd.Command("update", msgPrinter.Sprintf("Update an attribute of the pattern in the Horizon Exchange."))
	exPatUpdateNodeIdTok := exPatUpdateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatUpdatePattern := exPatUpdateCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern in the Horizon Exchange to publish.")).Required().String()
	exPatUpdateJsonFile := exPatUpdateCmd.Flag("json-file", msgPrinter.Sprintf("The path to a json file containing the updated attribute of the pattern to be put in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exPatDelCmd := exPatternCmd.Command("remove", msgPrinter.Sprintf("Remove a pattern resource from the Horizon Exchange."))
	exDelPat := exPatDelCmd.Arg("pattern", msgPrinter.Sprintf("The pattern to remove.")).Required().String()
	exPatDelForce := exPatDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exPatternListKeyCmd := exPatternCmd.Command("listkey", msgPrinter.Sprintf("List the signing public keys/certs for this pattern resource in the Horizon Exchange."))
	exPatternListKeyNodeIdTok := exPatternListKeyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatListKeyPat := exPatternListKeyCmd.Arg("pattern", msgPrinter.Sprintf("The existing pattern to list the keys for.")).Required().String()
	exPatListKeyKey := exPatternListKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to see the contents of.")).String()
	exPatternRemKeyCmd := exPatternCmd.Command("removekey", msgPrinter.Sprintf("Remove a signing public key/cert for this pattern resource in the Horizon Exchange."))
	exPatRemKeyPat := exPatternRemKeyCmd.Arg("pattern", msgPrinter.Sprintf("The existing pattern to remove the key from.")).Required().String()
	exPatRemKeyKey := exPatternRemKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to remove.")).Required().String()

	exServiceCmd := exchangeCmd.Command("service", msgPrinter.Sprintf("List and manage services in the Horizon Exchange"))
	exServiceListCmd := exServiceCmd.Command("list", msgPrinter.Sprintf("Display the service resources from the Horizon Exchange."))
	exService := exServiceListCmd.Arg("service", msgPrinter.Sprintf("List just this one service. Use <org>/<svc> to specify a public service in another org, or <org>/ to list all of the public services in another org.")).String()
	exServiceListNodeIdTok := exServiceListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceLong := exServiceListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the services, show the entire resource of each services, instead of just the name.")).Short('l').Bool()
	exServicePublishCmd := exServiceCmd.Command("publish", msgPrinter.Sprintf("Sign and create/update the service resource in the Horizon Exchange."))
	exSvcJsonFile := exServicePublishCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service in the Horizon exchange. See /usr/horizon/samples/service.json. Specify -f- to read from stdin.")).Short('f').Required().String()
	exSvcPrivKeyFile := exServicePublishCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the service. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.private.key is the default.")).Short('k').ExistingFile()
	exSvcPubPubKeyFile := exServicePublishCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of public key file (that corresponds to the private key) that should be stored with the service, to be used by the Horizon Agent to verify the signature. If both this and -k flags are not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If HZN_PUBLIC_KEY_FILE is not set, ~/.hzn/keys/service.public.pem is the default. If -k is specified and this flag is not specified, then no public key file will be stored with the service. The Horizon Agent needs to import the public key to verify the signature.")).Short('K').ExistingFile()
	exSvcPubDontTouchImage := exServicePublishCmd.Flag("dont-change-image-tag", msgPrinter.Sprintf("The image paths in the deployment field have regular tags and should not be changed to sha256 digest values. The image will not get automatically uploaded to the repository. This should only be used during development when testing new versions often.")).Short('I').Bool()
	exSvcPubPullImage := exServicePublishCmd.Flag("pull-image", msgPrinter.Sprintf("Use the image from the image repository. It will pull the image from the image repository and overwrite the local image if exists. This flag is mutually exclusive with -I.")).Short('P').Bool()
	exSvcRegistryTokens := exServicePublishCmd.Flag("registry-token", msgPrinter.Sprintf("Docker registry domain and auth that should be stored with the service, to enable the Horizon edge node to access the service's docker images. This flag can be repeated, and each flag should be in the format: registry:user:token")).Short('r').Strings()
	exSvcOverwrite := exServicePublishCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing version if the service exists in the exchange. It will skip the 'do you want to overwrite' prompt.")).Short('O').Bool()
	exSvcPolicyFile := exServicePublishCmd.Flag("service-policy-file", msgPrinter.Sprintf("The path of the service policy JSON file to be used for the service to be published. This flag is optional")).Short('p').String()
	exServiceVerifyCmd := exServiceCmd.Command("verify", msgPrinter.Sprintf("Verify the signatures of a service resource in the Horizon Exchange."))
	exVerService := exServiceVerifyCmd.Arg("service", msgPrinter.Sprintf("The service to verify.")).Required().String()
	exServiceVerifyNodeIdTok := exServiceVerifyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exSvcPubKeyFile := exServiceVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be used to verify the service. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()
	exSvcDelCmd := exServiceCmd.Command("remove", msgPrinter.Sprintf("Remove a service resource from the Horizon Exchange."))
	exDelSvc := exSvcDelCmd.Arg("service", msgPrinter.Sprintf("The service to remove.")).Required().String()
	exSvcDelForce := exSvcDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exServiceListKeyCmd := exServiceCmd.Command("listkey", msgPrinter.Sprintf("List the signing public keys/certs for this service resource in the Horizon Exchange."))
	exSvcListKeySvc := exServiceListKeyCmd.Arg("service", msgPrinter.Sprintf("The existing service to list the keys for.")).Required().String()
	exSvcListKeyKey := exServiceListKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to see the contents of.")).String()
	exServiceListKeyNodeIdTok := exServiceListKeyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceRemKeyCmd := exServiceCmd.Command("removekey", msgPrinter.Sprintf("Remove a signing public key/cert for this service resource in the Horizon Exchange."))
	exSvcRemKeySvc := exServiceRemKeyCmd.Arg("service", msgPrinter.Sprintf("The existing service to remove the key from.")).Required().String()
	exSvcRemKeyKey := exServiceRemKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to remove.")).Required().String()
	exServiceListAuthCmd := exServiceCmd.Command("listauth", msgPrinter.Sprintf("List the docker auth tokens for this service resource in the Horizon Exchange."))
	exSvcListAuthSvc := exServiceListAuthCmd.Arg("service", msgPrinter.Sprintf("The existing service to list the docker auths for.")).Required().String()
	exSvcListAuthId := exServiceListAuthCmd.Arg("auth-name", msgPrinter.Sprintf("The existing docker auth id to see the contents of.")).Uint()
	exServiceRemAuthCmd := exServiceCmd.Command("removeauth", msgPrinter.Sprintf("Remove a docker auth token for this service resource in the Horizon Exchange."))
	exServiceListAuthNodeIdTok := exServiceListAuthCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exSvcRemAuthSvc := exServiceRemAuthCmd.Arg("service", msgPrinter.Sprintf("The existing service to remove the docker auth from.")).Required().String()
	exSvcRemAuthId := exServiceRemAuthCmd.Arg("auth-name", msgPrinter.Sprintf("The existing docker auth id to remove.")).Required().Uint()
	exServiceListPolicyCmd := exServiceCmd.Command("listpolicy", msgPrinter.Sprintf("Display the service policy from the Horizon Exchange."))
	exServiceListPolicyIdTok := exServiceListPolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange id and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceListPolicyService := exServiceListPolicyCmd.Arg("service", msgPrinter.Sprintf("List policy for this service.")).Required().String()
	exServiceNewPolicyCmd := exServiceCmd.Command("newpolicy", msgPrinter.Sprintf("Display an empty service policy template that can be filled in."))
	exServiceAddPolicyCmd := exServiceCmd.Command("addpolicy", msgPrinter.Sprintf("Add or replace the service policy in the Horizon Exchange."))
	exServiceAddPolicyIdTok := exServiceAddPolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange ID and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceAddPolicyService := exServiceAddPolicyCmd.Arg("service", msgPrinter.Sprintf("Add or replace policy for this service.")).Required().String()
	exServiceAddPolicyJsonFile := exServiceAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exServiceRemovePolicyCmd := exServiceCmd.Command("removepolicy", msgPrinter.Sprintf("Remove the service policy in the Horizon Exchange."))
	exServiceRemovePolicyIdTok := exServiceRemovePolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange ID and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceRemovePolicyService := exServiceRemovePolicyCmd.Arg("service", msgPrinter.Sprintf("Remove policy for this service.")).Required().String()
	exServiceRemovePolicyForce := exServiceRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()

	exBusinessCmd := exchangeCmd.Command("business", msgPrinter.Sprintf("List and manage business policies in the Horizon Exchange."))
	exBusinessListPolicyCmd := exBusinessCmd.Command("listpolicy", msgPrinter.Sprintf("Display the business policies from the Horizon Exchange."))
	exBusinessListPolicyIdTok := exBusinessListPolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessListPolicyLong := exBusinessListPolicyCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about the business policies.")).Short('l').Bool()
	exBusinessListPolicyPolicy := exBusinessListPolicyCmd.Arg("policy", msgPrinter.Sprintf("List just this one policy. Use <org>/<policy> to specify a public policy in another org, or <org>/ to list all of the public policies in another org.")).String()
	exBusinessNewPolicyCmd := exBusinessCmd.Command("new", msgPrinter.Sprintf("Display an empty business policy template that can be filled in."))
	exBusinessAddPolicyCmd := exBusinessCmd.Command("addpolicy", msgPrinter.Sprintf("Add or replace a business policy in the Horizon Exchange. Use 'hzn exchange business new' for an empty business policy template."))
	exBusinessAddPolicyIdTok := exBusinessAddPolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessAddPolicyPolicy := exBusinessAddPolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the policy to add or overwrite.")).Required().String()
	exBusinessAddPolicyJsonFile := exBusinessAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exBusinessUpdatePolicyCmd := exBusinessCmd.Command("updatepolicy", msgPrinter.Sprintf("Update one attribute of an existing policy in the Horizon Exchange. The supported attributes are the top level attributes in the policy definition as shown by the command 'hzn exchange business new'."))
	exBusinessUpdatePolicyIdTok := exBusinessUpdatePolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessUpdatePolicyPolicy := exBusinessUpdatePolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the policy to be updated in the Horizon Exchange.")).Required().String()
	exBusinessUpdatePolicyJsonFile := exBusinessUpdatePolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path to the json file containing the updated business policy attribute to be changed in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exBusinessRemovePolicyCmd := exBusinessCmd.Command("removepolicy", msgPrinter.Sprintf("Remove the business policy in the Horizon Exchange."))
	exBusinessRemovePolicyIdTok := exBusinessRemovePolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessRemovePolicyForce := exBusinessRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exBusinessRemovePolicyPolicy := exBusinessRemovePolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the business policy to be removed.")).Required().String()

	exCatalogCmd := exchangeCmd.Command("catalog", msgPrinter.Sprintf("List all public services/patterns in all orgs that have orgType: IBM."))
	exCatalogServiceListCmd := exCatalogCmd.Command("servicelist", msgPrinter.Sprintf("Display all public services in all orgs that have orgType: IBM."))
	exCatalogServiceListShort := exCatalogServiceListCmd.Flag("short", msgPrinter.Sprintf("Only display org (IBM) and service names.")).Short('s').Bool()
	exCatalogServiceListLong := exCatalogServiceListCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about public services in all orgs that have orgType: IBM.")).Short('l').Bool()
	exCatalogPatternListCmd := exCatalogCmd.Command("patternlist", msgPrinter.Sprintf("Display all public patterns in all orgs that have orgType: IBM. "))
	exCatalogPatternListShort := exCatalogPatternListCmd.Flag("short", msgPrinter.Sprintf("Only display org (IBM) and pattern names.")).Short('s').Bool()
	exCatalogPatternListLong := exCatalogPatternListCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about public patterns in all orgs that have orgType: IBM.")).Short('l').Bool()

	regInputCmd := app.Command("reginput", msgPrinter.Sprintf("Create an input file template for this pattern that can be used for the 'hzn register' command (once filled in). This examines the services that the specified pattern uses, and determines the node owner input that is required for them."))
	regInputNodeIdTok := regInputCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token (it must already exist).")).Short('n').PlaceHolder("ID:TOK").Required().String()
	regInputInputFile := regInputCmd.Flag("input-file", msgPrinter.Sprintf("The JSON input template file name that should be created. This file will contain placeholders for you to fill in user input values.")).Short('f').Required().String()
	regInputOrg := regInputCmd.Arg("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node will be registered in.")).Required().String()
	regInputPattern := regInputCmd.Arg("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format.")).Required().String()
	regInputArch := regInputCmd.Arg("arch", msgPrinter.Sprintf("The architecture to write the template file for. (Horizon ignores services in patterns whose architecture is different from the target system.) The architecture must be what is returned by 'hzn node list' on the target system.")).Default(cutil.ArchString()).String()

	registerCmd := app.Command("register", msgPrinter.Sprintf("Register this edge node with Horizon."))
	nodeIdTok := registerCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token. The node ID must be unique within the organization. If not specified, HZN_EXCHANGE_NODE_AUTH will be used as a default. If both -n and HZN_EXCHANGE_NODE_AUTH are not specified, the node ID will be created by Horizon from the machine serial number or fully qualified hostname. If the token is not specified, Horizon will create a random token. If node resource in the exchange identified by the ID and token does not yet exist, you must also specify the -u flag so it can be created.")).Short('n').PlaceHolder("ID:TOK").String()
	nodeName := registerCmd.Flag("name", msgPrinter.Sprintf("The name of the node. If not specified, it will be the same as the node id.")).Short('m').String()
	userPw := registerCmd.Flag("user-pw", msgPrinter.Sprintf("User credentials to create the node resource in the Horizon exchange if it does not already exist. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default.")).Short('u').PlaceHolder("USER:PW").String()
	email := registerCmd.Flag("email", msgPrinter.Sprintf("Your email address. Only needs to be specified if: the node resource does not yet exist in the Horizon exchange, and the user specified in the -u flag does not exist, and you specified the 'public' org. If all of these things are true we will create the user and include this value as the email attribute.")).Short('e').String()
	inputFile := registerCmd.Flag("input-file", msgPrinter.Sprintf("A JSON file that sets or overrides variables needed by the node and services that are part of this pattern. See /usr/horizon/samples/node_reg_input.json and /usr/horizon/samples/more-examples.json. Specify -f- to read from stdin.")).Short('f').String() // not using ExistingFile() because it can be - for stdin

	nodeOrgFlag := registerCmd.Flag("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node should be registered in. The default is the HZN_ORG_ID environment variable. Mutually exclusive with <nodeorg> and <pattern> arguments.")).Short('o').String()
	patternFlag := registerCmd.Flag("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. Mutually exclusive with <nodeorg> and <pattern> arguments and --policy flag. ")).Short('p').String()
	nodepolicyFlag := registerCmd.Flag("policy", msgPrinter.Sprintf("A JSON file that sets or overrides the node policy for this node that will be used for policy based agreement negotiation. Mutually exclusive with -p argument.")).String()
	org := registerCmd.Arg("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node should be registered in. Mutually exclusive with -o and -p.")).String()
	pattern := registerCmd.Arg("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. Mutually exclusive with -o, -p and --policy.")).String()

	keyCmd := app.Command("key", msgPrinter.Sprintf("List and manage keys for signing and verifying services."))
	keyListCmd := keyCmd.Command("list", msgPrinter.Sprintf("List the signing keys that have been imported into this Horizon agent."))
	keyName := keyListCmd.Arg("key-name", msgPrinter.Sprintf("The name of a specific key to show.")).String()
	keyListAll := keyListCmd.Flag("all", msgPrinter.Sprintf("List the names of all signing keys, even the older public keys not wrapped in a certificate.")).Short('a').Bool()
	keyCreateCmd := keyCmd.Command("create", msgPrinter.Sprintf("Generate a signing key pair."))
	keyX509Org := keyCreateCmd.Arg("x509-org", msgPrinter.Sprintf("x509 certificate Organization (O) field (preferably a company name or other organization's name).")).Required().String()
	keyX509CN := keyCreateCmd.Arg("x509-cn", msgPrinter.Sprintf("x509 certificate Common Name (CN) field (preferably an email address issued by x509org).")).Required().String()
	keyOutputDir := keyCreateCmd.Flag("output-dir", msgPrinter.Sprintf("The directory to put the key pair files in. Mutually exclusive with -k and -K. The file names will be randomly generated.")).Short('d').ExistingDir()
	keyCreatePrivKey := keyCreateCmd.Flag("private-key-file", msgPrinter.Sprintf("The full path of the private key file. Mutually exclusive with -d. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.private.key is the default.")).Short('k').String()
	keyCreatePubKey := keyCreateCmd.Flag("pubic-key-file", msgPrinter.Sprintf("The full path of the public key file. Mutually exclusive with -d. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('K').String()
	keyCreateOverwrite := keyCreateCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing files. It will skip the 'do you want to overwrite' prompt.")).Short('f').Bool()
	keyLength := keyCreateCmd.Flag("length", msgPrinter.Sprintf("The length of the key to create.")).Short('l').Default("4096").Int()
	keyDaysValid := keyCreateCmd.Flag("days-valid", msgPrinter.Sprintf("x509 certificate validity (Validity > Not After) expressed in days from the day of generation.")).Default("1461").Int()
	keyImportFlag := keyCreateCmd.Flag("import", msgPrinter.Sprintf("Automatically import the created public key into the local Horizon agent.")).Short('i').Bool()
	keyImportCmd := keyCmd.Command("import", msgPrinter.Sprintf("Imports a signing public key into the Horizon agent."))
	keyImportPubKeyFile := keyImportCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be imported. The base name in the path is also used as the key name in the Horizon agent. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()
	keyDelCmd := keyCmd.Command("remove", msgPrinter.Sprintf("Remove the specified signing key from this Horizon agent."))
	keyDelName := keyDelCmd.Arg("key-name", msgPrinter.Sprintf("The name of a specific key to remove.")).Required().String()

	nodeCmd := app.Command("node", msgPrinter.Sprintf("List and manage general information about this Horizon edge node."))
	nodeListCmd := nodeCmd.Command("list", msgPrinter.Sprintf("Display general information about this Horizon edge node."))

	policyCmd := app.Command("policy", msgPrinter.Sprintf("List and manage policy for this Horizon edge node."))
	policyListCmd := policyCmd.Command("list", msgPrinter.Sprintf("Display this edge node's policy."))
	policyNewCmd := policyCmd.Command("new", msgPrinter.Sprintf("Display an empty policy template that can be filled in."))
	policyUpdateCmd := policyCmd.Command("update", msgPrinter.Sprintf("Update the node's policy. The node's built-in properties will be automatically added if the input policy does not contain them."))
	policyUpdateInputFile := policyUpdateCmd.Flag("input-file", msgPrinter.Sprintf("The JSON input file name containing the node policy.")).Short('f').Required().String()
	policyPatchCmd := policyCmd.Command("patch", msgPrinter.Sprintf("Add or update node policy properties or overwrite the node policy constraint expression."))
	policyPatchInput := policyPatchCmd.Arg("patch", msgPrinter.Sprintf("The new constraints or properties in the format '%s' or '%s'.", "{\"constraints\":[<constraint list>]}", "{\"properties\":[<property list>]}")).Required().String()
	policyRemoveCmd := policyCmd.Command("remove", msgPrinter.Sprintf("Remove the node's policy."))
	policyRemoveForce := policyRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()

	deploycheckCmd := app.Command("deploycheck", msgPrinter.Sprintf("Check deployment compatibility."))
	deploycheckOrg := deploycheckCmd.Flag("org", msgPrinter.Sprintf("The Horizon exchange organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	deploycheckUserPw := deploycheckCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon exchange user credential to query exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH will be used as a default. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()
	deploycheckCheckAll := deploycheckCmd.Flag("check-all", msgPrinter.Sprintf("Show the compatibility status of all the service versions referenced in the business policy.")).Short('c').Bool()
	deploycheckLong := deploycheckCmd.Flag("long", msgPrinter.Sprintf("Show policies and userinput used for the compatibility checking.")).Short('l').Bool()
	policyCompCmd := deploycheckCmd.Command("policy", msgPrinter.Sprintf("Check policy compatibility."))
	policyCompNodeId := policyCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --node-pol. If omitted, the node ID that the current device is registered with will be used. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('n').String()
	policyCompBPolId := policyCompCmd.Flag("business-pol-id", msgPrinter.Sprintf("The Horizon exchange business policy ID. Mutually exclusive with --business-pol. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	policyCompNodeArch := policyCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the business policy will be checked for compatibility.")).Short('a').String()
	policyCompNodePolFile := policyCompCmd.Flag("node-pol", msgPrinter.Sprintf("The JSON input file name containing the node policy. Mutually exclusive with -n.")).String()
	policyCompBPolFile := policyCompCmd.Flag("business-pol", msgPrinter.Sprintf("The JSON input file name containing the business policy. Mutually exclusive with -b.")).String()
	policyCompSPolFile := policyCompCmd.Flag("service-pol", msgPrinter.Sprintf("(optional) The JSON input file name containing the service policy. If omitted, the service policy will be retrieved from the exchange for the service defined in the business policy.")).String()
	userinputCompCmd := deploycheckCmd.Command("userinput", msgPrinter.Sprintf("Check user input compatibility."))
	userinputCompNodeId := userinputCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --node-ui. If omitted, the node ID that the current device is registered with will be used. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('n').String()
	userinputCompBPolId := userinputCompCmd.Flag("business-pol-id", msgPrinter.Sprintf("The Horizon exchange business policy ID. Mutually exclusive with --business-pol. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	userinputCompNodeArch := userinputCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the business policy will be checked for compatibility.")).Short('a').String()
	userinputCompNodeUIFile := userinputCompCmd.Flag("node-ui", msgPrinter.Sprintf("The JSON input file name containing the node user input. Mutually exclusive with -n.")).String()
	userinputCompBPolFile := userinputCompCmd.Flag("business-pol", msgPrinter.Sprintf("The JSON input file name containing the business policy. Mutually exclusive with -b.")).String()
	userinputCompSvcFile := userinputCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. If omitted, the service defined in the business policy will be retrieved from the exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	allCompCmd := deploycheckCmd.Command("all", msgPrinter.Sprintf("Check all compatibilities for a deployment."))
	allCompNodeId := allCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --node-pol and --node-ui. If omitted, the node ID that the current device is registered with will be used. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('n').String()
	allCompBPolId := allCompCmd.Flag("business-pol-id", msgPrinter.Sprintf("The Horizon exchange business policy ID. Mutually exclusive with --business-pol. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	allCompNodeArch := allCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the business policy will be checked for compatibility.")).Short('a').String()
	allCompNodePolFile := allCompCmd.Flag("node-pol", msgPrinter.Sprintf("The JSON input file name containing the node policy. Mutually exclusive with -n.")).String()
	allCompNodeUIFile := allCompCmd.Flag("node-ui", msgPrinter.Sprintf("The JSON input file name containing the node user input. Mutually exclusive with -n.")).String()
	allCompBPolFile := allCompCmd.Flag("business-pol", msgPrinter.Sprintf("The JSON input file name containing the business policy. Mutually exclusive with -b.")).String()
	allCompSvcFile := allCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. If omitted, the service defined in the business policy will be retrieved from the exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	allCompSPolFile := allCompCmd.Flag("service-pol", msgPrinter.Sprintf("(optional) The JSON input file name containing the service policy. If omitted, the service policy will be retrieved from the exchange for the service defined in the business policy.")).String()

	agreementCmd := app.Command("agreement", msgPrinter.Sprintf("List or manage the active or archived agreements this edge node has made with a Horizon agreement bot."))
	agreementListCmd := agreementCmd.Command("list", msgPrinter.Sprintf("List the active or archived agreements this edge node has made with a Horizon agreement bot."))
	listAgreementId := agreementListCmd.Arg("agreement-id", msgPrinter.Sprintf("Show the details of this active or archived agreement.")).String()
	listArchivedAgreements := agreementListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreements instead of the active agreements.")).Short('r').Bool()
	agreementCancelCmd := agreementCmd.Command("cancel", msgPrinter.Sprintf("Cancel 1 or all of the active agreements this edge node has made with a Horizon agreement bot. Usually an agbot will immediately negotiated a new agreement. If you want to cancel all agreements and not have this edge accept new agreements, run 'hzn unregister'."))
	cancelAllAgreements := agreementCancelCmd.Flag("all", msgPrinter.Sprintf("Cancel all of the current agreements.")).Short('a').Bool()
	cancelAgreementId := agreementCancelCmd.Arg("agreement-id", msgPrinter.Sprintf("The active agreement to cancel.")).String()

	meteringCmd := app.Command("metering", msgPrinter.Sprintf("List or manage the metering (payment) information for the active or archived agreements."))
	meteringListCmd := meteringCmd.Command("list", msgPrinter.Sprintf("List the metering (payment) information for the active or archived agreements."))
	listArchivedMetering := meteringListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreement metering information instead of metering for the active agreements.")).Short('r').Bool()

	attributeCmd := app.Command("attribute", msgPrinter.Sprintf("List or manage the global attributes that are currently registered on this Horizon edge node."))
	attributeListCmd := attributeCmd.Command("list", msgPrinter.Sprintf("List the global attributes that are currently registered on this Horizon edge node."))

	userinputCmd := app.Command("userinput", msgPrinter.Sprintf("List or manage the service user inputs that are currently registered on this Horizon edge node."))
	userinputListCmd := userinputCmd.Command("list", msgPrinter.Sprintf("List the service user inputs currently registered on this Horizon edge node."))
	userinputNewCmd := userinputCmd.Command("new", msgPrinter.Sprintf("Display an empty userinput template."))
	userinputAddCmd := userinputCmd.Command("add", msgPrinter.Sprintf("Add a new user input object or overwrite the current user input object for this Horizon edge node."))
	userinputAddFilePath := userinputAddCmd.Flag("file-path", msgPrinter.Sprintf("The file path to the json file with the user input object. Specify -f- to read from stdin.")).Short('f').Required().String()
	userinputUpdateCmd := userinputCmd.Command("update", msgPrinter.Sprintf("Update an existing user input object for this Horizon edge node."))
	userinputUpdateFilePath := userinputUpdateCmd.Flag("file-path", msgPrinter.Sprintf("The file path to the json file with the updated user input object. Specify -f- to read from stdin.")).Short('f').Required().String()
	userinputRemoveCmd := userinputCmd.Command("remove", msgPrinter.Sprintf("Remove the user inputs that are currently registered on this Horizon edge node."))
	userinputRemoveForce := userinputRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'Are you sure?' prompt.")).Short('f').Bool()

	serviceCmd := app.Command("service", msgPrinter.Sprintf("List or manage the services that are currently registered on this Horizon edge node."))
	serviceLogCmd := serviceCmd.Command("log", msgPrinter.Sprintf("Show the container logs for a service."))
	logServiceName := serviceLogCmd.Arg("service", msgPrinter.Sprintf("The name of the service whose log records should be displayed. The service name is the same as the url field of a service definition. Displays log records similar to tail behavior and returns .")).Required().String()
	logTail := serviceLogCmd.Flag("tail", msgPrinter.Sprintf("Continuously polls the service's logs to display the most recent records, similar to tail -F behavior.")).Short('f').Bool()
	serviceListCmd := serviceCmd.Command("list", msgPrinter.Sprintf("List the services variable configuration that has been done on this Horizon edge node."))
	serviceRegisteredCmd := serviceCmd.Command("registered", msgPrinter.Sprintf("List the services that are currently registered on this Horizon edge node."))
	serviceConfigStateCmd := serviceCmd.Command("configstate", msgPrinter.Sprintf("List or manage the configuration state for the services that are currently registered on this Horizon edge node."))
	serviceConfigStateListCmd := serviceConfigStateCmd.Command("list", msgPrinter.Sprintf("List the configuration state for the services that are currently registered on this Horizon edge node."))
	serviceConfigStateSuspendCmd := serviceConfigStateCmd.Command("suspend", msgPrinter.Sprintf("Change the configuration state to 'suspend' for a service."))
	serviceConfigStateActiveCmd := serviceConfigStateCmd.Command("resume", msgPrinter.Sprintf("Change the configuration state to 'active' for a service."))
	suspendAllServices := serviceConfigStateSuspendCmd.Flag("all", msgPrinter.Sprintf("Suspend all registerd services.")).Short('a').Bool()
	suspendServiceOrg := serviceConfigStateSuspendCmd.Arg("serviceorg", msgPrinter.Sprintf("The organization of the service that should be suspended.")).String()
	suspendServiceName := serviceConfigStateSuspendCmd.Arg("service", msgPrinter.Sprintf("The name of the service that should be suspended.")).String()
	forceSuspendService := serviceConfigStateSuspendCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	resumeAllServices := serviceConfigStateActiveCmd.Flag("all", msgPrinter.Sprintf("Resume all registerd services.")).Short('a').Bool()
	resumeServiceOrg := serviceConfigStateActiveCmd.Arg("serviceorg", msgPrinter.Sprintf("The organization of the service that should be resumed.")).String()
	resumeServiceName := serviceConfigStateActiveCmd.Arg("service", msgPrinter.Sprintf("The name of the service that should be resumed.")).String()

	unregisterCmd := app.Command("unregister", msgPrinter.Sprintf("Unregister and reset this Horizon edge node so that it is ready to be registered again. Warning: this will stop all the Horizon services running on this edge node, and restart the Horizon agent."))

	forceUnregister := unregisterCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	removeNodeUnregister := unregisterCmd.Flag("remove", msgPrinter.Sprintf("Also remove this node resource from the Horizon exchange (because you no longer want to use this node with Horizon).")).Short('r').Bool()
	deepCleanUnregister := unregisterCmd.Flag("deep-clean", msgPrinter.Sprintf("Also remove all the previous registration information. Use it only after the 'hzn unregister' command failed. Please capture the logs by running 'hzn eventlog list -a -l' command before using this flag.")).Short('D').Bool()
	timeoutUnregister := unregisterCmd.Flag("timeout", msgPrinter.Sprintf("The number of minutes to wait for unregistration to complete. The default is zero which will wait forever.")).Short('t').Default("0").Int()

	statusCmd := app.Command("status", msgPrinter.Sprintf("Display the current horizon internal status for the node."))
	statusLong := statusCmd.Flag("long", msgPrinter.Sprintf("Show detailed status")).Short('l').Bool()

	eventlogCmd := app.Command("eventlog", msgPrinter.Sprintf("List the event logs for the current or all registrations."))
	eventlogListCmd := eventlogCmd.Command("list", msgPrinter.Sprintf("List the event logs for the current or all registrations."))
	listAllEventlogs := eventlogListCmd.Flag("all", msgPrinter.Sprintf("List all the event logs including the previous registrations.")).Short('a').Bool()
	listDetailedEventlogs := eventlogListCmd.Flag("long", msgPrinter.Sprintf("List event logs with details.")).Short('l').Bool()
	listSelectedEventlogs := eventlogListCmd.Flag("select", msgPrinter.Sprintf("Selection string. This flag can be repeated which means 'AND'. Each flag should be in the format of attribute=value, attribute~value, \"attribute>value\" or \"attribute<value\", where '~' means contains. The common attribute names are timestamp, severity, message, event_code, source_type, agreement_id, service_url etc. Use the '-l' flag to see all the attribute names.")).Short('s').Strings()
	surfaceErrorsEventlogs := eventlogCmd.Command("surface", msgPrinter.Sprintf("List all the active errors that will be shared with the exchange if the node is online."))
	surfaceErrorsEventlogsLong := surfaceErrorsEventlogs.Flag("long", msgPrinter.Sprintf("List the full event logs of the surface errors.")).Short('l').Bool()

	devCmd := app.Command("dev", msgPrinter.Sprintf("Development tools for creation of services."))
	devHomeDirectory := devCmd.Flag("directory", msgPrinter.Sprintf("Directory containing Horizon project metadata. If omitted, a subdirectory called 'horizon' under current directory will be used.")).Short('d').String()

	devServiceCmd := devCmd.Command("service", msgPrinter.Sprintf("For working with a service project."))
	devServiceNewCmd := devServiceCmd.Command("new", msgPrinter.Sprintf("Create a new service project."))
	devServiceNewCmdOrg := devServiceNewCmd.Flag("org", msgPrinter.Sprintf("The Org id that the service is defined within. If this flag is omitted, the HZN_ORG_ID environment variable is used.")).Short('o').String()
	devServiceNewCmdName := devServiceNewCmd.Flag("specRef", msgPrinter.Sprintf("The name of the service. If this flag and the -i flag are omitted, only the skeletal horizon metadata files will be generated.")).Short('s').String()
	devServiceNewCmdVer := devServiceNewCmd.Flag("ver", msgPrinter.Sprintf("The version of the service. If this flag is omitted, '0.0.1' is used.")).Short('V').String()
	devServiceNewCmdImage := devServiceNewCmd.Flag("image", msgPrinter.Sprintf("The docker container image base name without the version tag for the service. This command will add arch and version to the base name to form the final image name. The format is 'basename_arch:serviceversion'. This flag can be repeated to specify multiple images when '--noImageGen' flag is specified.")).Short('i').Strings()
	devServiceNewCmdNoImageGen := devServiceNewCmd.Flag("noImageGen", msgPrinter.Sprintf("Indicates that the image is built somewhere else. No image sample code will be created by this command. If this flag is not specified, files for generating a simple service image will be created under current directory.")).Bool()
	devServiceNewCmdNoPattern := devServiceNewCmd.Flag("noPattern", msgPrinter.Sprintf("Indicates no pattern definition file will be created.")).Bool()
	devServiceNewCmdNoPolicy := devServiceNewCmd.Flag("noPolicy", msgPrinter.Sprintf("Indicate no policy file will be created.")).Bool()
	devServiceNewCmdCfg := devServiceNewCmd.Flag("dconfig", msgPrinter.Sprintf("Indicates the type of deployment that will be used, e.g. native (the default), or helm.")).Short('c').Default("native").String()
	devServiceStartTestCmd := devServiceCmd.Command("start", msgPrinter.Sprintf("Run a service in a mocked Horizon Agent environment."))
	devServiceUserInputFile := devServiceStartTestCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for running a test. If omitted, the userinput file for the project will be used.")).Short('f').String()
	devServiceConfigFile := devServiceStartTestCmd.Flag("configFile", msgPrinter.Sprintf("File to be made available through the sync service APIs. This flag can be repeated to populate multiple files.")).Short('m').Strings()
	devServiceConfigType := devServiceStartTestCmd.Flag("type", msgPrinter.Sprintf("The type of file to be made available through the sync service APIs. All config files are presumed to be of the same type. This flag is required if any configFiles are specified.")).Short('t').String()
	devServiceNoFSS := devServiceStartTestCmd.Flag("noFSS", msgPrinter.Sprintf("Do not bring up file sync service (FSS) containers. They are brought up by default.")).Short('S').Bool()
	devServiceStartCmdUserPw := devServiceStartTestCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. Specify it when you want to automatically fetch the missing dependent services from the exchange. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()
	devServiceStopTestCmd := devServiceCmd.Command("stop", msgPrinter.Sprintf("Stop a service that is running in a mocked Horizon Agent environment."))
	devServiceValidateCmd := devServiceCmd.Command("verify", msgPrinter.Sprintf("Validate the project for completeness and schema compliance."))
	devServiceVerifyUserInputFile := devServiceValidateCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for verification of a project. If omitted, the userinput file for the project will be used.")).Short('f').String()
	devServiceValidateCmdUserPw := devServiceValidateCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. Specify it when you want to automatically fetch the missing dependent services from the exchange. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()

	devDependencyCmd := devCmd.Command("dependency", msgPrinter.Sprintf("For working with project dependencies."))
	devDependencyCmdSpecRef := devDependencyCmd.Flag("specRef", msgPrinter.Sprintf("The URL of the service dependency in the exchange. Mutually exclusive with -p and --url.")).Short('s').String()
	devDependencyCmdURL := devDependencyCmd.Flag("url", msgPrinter.Sprintf("The URL of the service dependency in the exchange. Mutually exclusive with -p and --specRef.")).String()
	devDependencyCmdOrg := devDependencyCmd.Flag("org", msgPrinter.Sprintf("The Org of the service dependency in the exchange. Mutually exclusive with -p.")).Short('o').String()
	devDependencyCmdVersion := devDependencyCmd.Flag("ver", msgPrinter.Sprintf("(optional) The Version of the service dependency in the exchange. Mutually exclusive with -p.")).String()
	devDependencyCmdArch := devDependencyCmd.Flag("arch", msgPrinter.Sprintf("(optional) The hardware Architecture of the service dependency in the exchange. Mutually exclusive with -p.")).Short('a').String()
	devDependencyFetchCmd := devDependencyCmd.Command("fetch", msgPrinter.Sprintf("Retrieving Horizon metadata for a new dependency."))
	devDependencyFetchCmdProject := devDependencyFetchCmd.Flag("project", msgPrinter.Sprintf("Horizon project containing the definition of a dependency. Mutually exclusive with -s -o --ver -a and --url.")).Short('p').ExistingDir()
	devDependencyFetchCmdUserPw := devDependencyFetchCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()
	devDependencyFetchCmdUserInputFile := devDependencyFetchCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for configuring the new dependency. If omitted, the userinput file in the dependency project will be used.")).Short('f').ExistingFile()
	devDependencyListCmd := devDependencyCmd.Command("list", msgPrinter.Sprintf("List all dependencies."))
	devDependencyRemoveCmd := devDependencyCmd.Command("remove", msgPrinter.Sprintf("Remove a project dependency."))

	agbotCmd := app.Command("agbot", msgPrinter.Sprintf("List and manage Horizon agreement bot resources."))
	agbotListCmd := agbotCmd.Command("list", msgPrinter.Sprintf("Display general information about this Horizon agbot node."))
	agbotAgreementCmd := agbotCmd.Command("agreement", msgPrinter.Sprintf("List or manage the active or archived agreements this Horizon agreement bot has with edge nodes."))
	agbotAgreementListCmd := agbotAgreementCmd.Command("list", msgPrinter.Sprintf("List the active or archived agreements this Horizon agreement bot has with edge nodes."))
	agbotlistArchivedAgreements := agbotAgreementListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreements instead of the active agreements.")).Short('r').Bool()
	agbotAgreement := agbotAgreementListCmd.Arg("agreement", msgPrinter.Sprintf("List just this one agreement.")).String()
	agbotAgreementCancelCmd := agbotAgreementCmd.Command("cancel", msgPrinter.Sprintf("Cancel 1 or all of the active agreements this Horizon agreement bot has with edge nodes. Usually an agbot will immediately negotiated a new agreement. "))
	agbotCancelAllAgreements := agbotAgreementCancelCmd.Flag("all", msgPrinter.Sprintf("Cancel all of the current agreements.")).Short('a').Bool()
	agbotCancelAgreementId := agbotAgreementCancelCmd.Arg("agreement", msgPrinter.Sprintf("The active agreement to cancel.")).String()
	agbotPolicyCmd := agbotCmd.Command("policy", msgPrinter.Sprintf("List the policies this Horizon agreement bot hosts."))
	agbotPolicyListCmd := agbotPolicyCmd.Command("list", msgPrinter.Sprintf("List policies this Horizon agreement bot hosts."))
	agbotPolicyOrg := agbotPolicyListCmd.Arg("org", msgPrinter.Sprintf("The organization the policy belongs to.")).String()
	agbotPolicyName := agbotPolicyListCmd.Arg("name", msgPrinter.Sprintf("The policy name.")).String()
	agbotStatusCmd := agbotCmd.Command("status", msgPrinter.Sprintf("Display the current horizon internal status for the Horizon agreement bot."))
	agbotStatusLong := agbotStatusCmd.Flag("long", msgPrinter.Sprintf("Show detailed status")).Short('l').Bool()

	utilCmd := app.Command("util", msgPrinter.Sprintf("Utility commands."))
	utilSignCmd := utilCmd.Command("sign", msgPrinter.Sprintf("Sign the text in stdin. The signature is sent to stdout."))
	utilSignPrivKeyFile := utilSignCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the stdin. ")).Short('k').Required().ExistingFile()
	utilVerifyCmd := utilCmd.Command("verify", msgPrinter.Sprintf("Verify that the signature specified via -s is a valid signature for the text in stdin."))
	utilVerifyPubKeyFile := utilVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of public key file (that corresponds to the private key that was used to sign) to verify the signature of stdin.")).Short('K').Required().ExistingFile()
	utilVerifySig := utilVerifyCmd.Flag("signature", msgPrinter.Sprintf("The supposed signature of stdin.")).Short('s').Required().String()
	utilConfigConvCmd := utilCmd.Command("configconv", msgPrinter.Sprintf("Convert the configuration file from JSON format to a shell script."))
	utilConfigConvFile := utilConfigConvCmd.Flag("config-file", msgPrinter.Sprintf("The path of a configuration file to be converted. ")).Short('f').Required().ExistingFile()

	mmsCmd := app.Command("mms", msgPrinter.Sprintf("List and manage Horizon Model Management Service resources."))
	mmsOrg := mmsCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	mmsUserPw := mmsCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon user credentials to query and create Model Management Service resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()

	mmsStatusCmd := mmsCmd.Command("status", msgPrinter.Sprintf("Display the status of the Horizon Model Management Service."))
	mmsObjectCmd := mmsCmd.Command("object", msgPrinter.Sprintf("List and manage objects in the Horizon Model Management Service."))
	mmsObjectListCmd := mmsObjectCmd.Command("list", msgPrinter.Sprintf("List objects in the Horizon Model Management Service."))
	mmsObjectListType := mmsObjectListCmd.Flag("objectType", msgPrinter.Sprintf("The type of the object to list.")).Short('t').String()
	mmsObjectListId := mmsObjectListCmd.Flag("objectId", msgPrinter.Sprintf("The id of the object to list. This flag is optional. Omit this flag to list all objects of a given object type.")).Short('i').String()
	mmsObjectListDestinationPolicy := mmsObjectListCmd.Flag("policy", msgPrinter.Sprintf("Specify true to show only objects using policy. Specify false to show only objects not using policy. If this flag is omitted, both kinds of objects are shown.")).Short('p').String()
	mmsObjectListDPService := mmsObjectListCmd.Flag("service", msgPrinter.Sprintf("List mms objects using policy that are targetted for the given service. Service specified in the format service-org/service-name.")).Short('s').String()
	mmsObjectListDPProperty := mmsObjectListCmd.Flag("property", msgPrinter.Sprintf("List mms objects using policy that reference the given property name.")).String()
	mmsObjectListDPUpdateTime := mmsObjectListCmd.Flag("updateTime", msgPrinter.Sprintf("List mms objects using policy that has been updated since the given time. The time value is spefified in RFC3339 format: yyyy-MM-ddTHH:mm:ssZ. The time of day may be omitted.")).String()
	mmsObjectListDestinationType := mmsObjectListCmd.Flag("destinationType", msgPrinter.Sprintf("List mms objects with given destination type")).String()
	mmsObjectListDestinationId := mmsObjectListCmd.Flag("destinationId", msgPrinter.Sprintf("List mms objects with given destination id. Must specify --destinationType to use this flag")).String()
	mmsObjectListWithData := mmsObjectListCmd.Flag("data", msgPrinter.Sprintf("Specify true to show objects that have data. Specify false to show objects that have no data. If this flag is omitted, both kinds of objects are shown.")).String()
	mmsObjectListExpirationTime := mmsObjectListCmd.Flag("expirationTime", msgPrinter.Sprintf("List mms objects that expired before the given time. The time value is spefified in RFC3339 format: yyyy-MM-ddTHH:mm:ssZ. Specify now to show objects that are currently expired.")).Short('e').String()
	mmsObjectListLong := mmsObjectListCmd.Flag("long", msgPrinter.Sprintf("Show detailed object metadata information")).Short('l').Bool()
	mmsObjectListDetail := mmsObjectListCmd.Flag("detail", msgPrinter.Sprintf("Provides additional detail about the deployment of the object on edge nodes.")).Short('d').Bool()

	mmsObjectNewCmd := mmsObjectCmd.Command("new", msgPrinter.Sprintf("Display an empty object metadata template that can be filled in and passed as the -m option on the 'hzn mms object publish' command."))
	mmsObjectPublishCmd := mmsObjectCmd.Command("publish", msgPrinter.Sprintf("Publish an object in the Horizon Model Management Service, making it available for services deployed on nodes."))
	mmsObjectPublishType := mmsObjectPublishCmd.Flag("type", msgPrinter.Sprintf("The type of the object to publish. This flag must be used with -i. It is mutually exclusive with -m")).Short('t').String()
	mmsObjectPublishId := mmsObjectPublishCmd.Flag("id", msgPrinter.Sprintf("The id of the object to publish. This flag must be used with -t. It is mutually exclusive with -m")).Short('i').String()
	mmsObjectPublishPat := mmsObjectPublishCmd.Flag("pattern", msgPrinter.Sprintf("If you want the object to be deployed on nodes using a given pattern, specify it using this flag. This flag is optional and can only be used with --type and --id. It is mutually exclusive with -m")).Short('p').String()
	mmsObjectPublishDef := mmsObjectPublishCmd.Flag("def", msgPrinter.Sprintf("The definition of the object to publish. A blank template can be obtained from the 'hzn mss object new' command.")).Short('m').String()
	mmsObjectPublishObj := mmsObjectPublishCmd.Flag("object", msgPrinter.Sprintf("The object (in the form of a file) to publish. This flag is optional so that you can update only the object's definition.")).Short('f').String()
	mmsObjectDeleteCmd := mmsObjectCmd.Command("delete", msgPrinter.Sprintf("Delete an object in the Horizon Model Management Service, making it unavailable for services deployed on nodes."))
	mmsObjectDeleteType := mmsObjectDeleteCmd.Flag("type", msgPrinter.Sprintf("The type of the object to delete.")).Short('t').Required().String()
	mmsObjectDeleteId := mmsObjectDeleteCmd.Flag("id", msgPrinter.Sprintf("The id of the object to delete.")).Short('i').Required().String()

	app.VersionFlag = nil

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

	// setup the environment variables from the project config file
	project_dir := ""
	if strings.HasPrefix(fullCmd, "dev ") {
		project_dir = *devHomeDirectory
	}
	cliconfig.SetEnvVarsFromProjectConfigFile(project_dir)

	credToUse := ""
	if strings.HasPrefix(fullCmd, "exchange") {
		exOrg = cliutils.RequiredWithDefaultEnvVar(exOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))

		// some hzn exchange commands can take either -u user:pw or -n nodeid:token as credentials.
		switch subCmd := strings.TrimPrefix(fullCmd, "exchange "); subCmd {
		case "node list":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeListNodeIdTok)
		case "node update":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeUpdateIdTok)
		case "node settoken":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeSetTokNodeIdTok)
		case "node remove":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeRemoveNodeIdTok)
		case "node confirm":
			//do nothing because it uses the node id and token given in the argument as the credential
		case "node listpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeListPolicyIdTok)
		case "node addpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeAddPolicyIdTok)
		case "node updatepolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeUpdatePolicyIdTok)
		case "node removepolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeRemovePolicyIdTok)
		case "node listerrors":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeErrorsListIdTok)
		case "service list":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListNodeIdTok)
		case "service verify":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceVerifyNodeIdTok)
		case "service listkey":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListKeyNodeIdTok)
		case "service listauth":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListAuthNodeIdTok)
		case "pattern list":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternListNodeIdTok)
		case "pattern update":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatUpdateNodeIdTok)
		case "pattern verify":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternVerifyNodeIdTok)
		case "pattern listkey":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternListKeyNodeIdTok)
		case "service listpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListPolicyIdTok)
		case "service addpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceAddPolicyIdTok)
		case "service removepolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceRemovePolicyIdTok)
		case "business listpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessListPolicyIdTok)
		case "business updatepolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessUpdatePolicyIdTok)
		case "business addpolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessAddPolicyIdTok)
		case "business removepolicy":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessRemovePolicyIdTok)
		case "version":
			credToUse = cliutils.GetExchangeAuthVersion(*exUserPw)
		default:
			// get HZN_EXCHANGE_USER_AUTH as default if exUserPw is empty
			exUserPw = cliutils.RequiredWithDefaultEnvVar(exUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
		}
	}

	if strings.HasPrefix(fullCmd, "register") {
		// use HZN_EXCHANGE_USER_AUTH for -u
		userPw = cliutils.WithDefaultEnvVar(userPw, "HZN_EXCHANGE_USER_AUTH")

		// use HZN_EXCHANGE_NODE_AUTH for -n and trim the org
		nodeIdTok = cliutils.WithDefaultEnvVar(nodeIdTok, "HZN_EXCHANGE_NODE_AUTH")
	}

	if strings.HasPrefix(fullCmd, "deploycheck") {
		deploycheckOrg = cliutils.WithDefaultEnvVar(deploycheckOrg, "HZN_ORG_ID")
		deploycheckUserPw = cliutils.WithDefaultEnvVar(deploycheckUserPw, "HZN_EXCHANGE_USER_AUTH")
	}

	// For the mms command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "mms") {
		mmsOrg = cliutils.RequiredWithDefaultEnvVar(mmsOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		mmsUserPw = cliutils.RequiredWithDefaultEnvVar(mmsUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}

	// For the ENV command , get org and exchange credentials
	if strings.HasPrefix(fullCmd, "env") {
		envOrg = cliutils.WithDefaultEnvVar(envOrg, "HZN_ORG_ID")
		envUserPw = cliutils.WithDefaultEnvVar(envUserPw, "HZN_EXCHANGE_USER_AUTH")
	}

	// key file defaults
	switch fullCmd {
	case "key create":
		if *keyOutputDir == "" {
			keyCreatePrivKey = cliutils.WithDefaultEnvVar(keyCreatePrivKey, "HZN_PRIVATE_KEY_FILE")
			keyCreatePubKey = cliutils.WithDefaultEnvVar(keyCreatePubKey, "HZN_PUBLIC_KEY_FILE")
		}
	case "exchange pattern verify":
		exPatPubKeyFile = cliutils.WithDefaultEnvVar(exPatPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	case "exchange service verify":
		exSvcPubKeyFile = cliutils.WithDefaultEnvVar(exSvcPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	case "key import":
		keyImportPubKeyFile = cliutils.WithDefaultEnvVar(keyImportPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	}

	// set env variable ARCH if it is not set
	cliutils.SetDefaultArch()

	// Decide which command to run
	switch fullCmd {
	case envCmd.FullCommand():
		node.Env(*envOrg, *envUserPw, envExchUrl, envCcsUrl)
	case versionCmd.FullCommand():
		node.Version()
	case archCmd.FullCommand():
		node.Architecture()
	case exVersionCmd.FullCommand():
		exchange.Version(*exOrg, credToUse)
	case exStatusCmd.FullCommand():
		exchange.Status(*exOrg, *exUserPw)
	case exUserListCmd.FullCommand():
		exchange.UserList(*exOrg, *exUserPw, *exUserListUser, *exUserListAll, *exUserListNamesOnly)
	case exUserCreateCmd.FullCommand():
		exchange.UserCreate(*exOrg, *exUserPw, *exUserCreateUser, *exUserCreatePw, *exUserCreateEmail, *exUserCreateIsAdmin)
	case exUserSetAdminCmd.FullCommand():
		exchange.UserSetAdmin(*exOrg, *exUserPw, *exUserSetAdminUser, *exUserSetAdminBool)
	case exUserDelCmd.FullCommand():
		exchange.UserRemove(*exOrg, *exUserPw, *exDelUser, *exUserDelForce)
	case exNodeListCmd.FullCommand():
		exchange.NodeList(*exOrg, credToUse, *exNode, !*exNodeLong)
	case exNodeUpdateCmd.FullCommand():
		exchange.NodeUpdate(*exOrg, credToUse, *exNodeUpdateNode, *exNodeUpdateJsonFile)
	case exNodeCreateCmd.FullCommand():
		exchange.NodeCreate(*exOrg, *exNodeCreateNodeIdTok, *exNodeCreateNode, *exNodeCreateToken, *exUserPw, *exNodeCreateNodeEmail, *exNodeCreateNodeArch, *exNodeCreateNodeName)
	case exNodeSetTokCmd.FullCommand():
		exchange.NodeSetToken(*exOrg, credToUse, *exNodeSetTokNode, *exNodeSetTokToken)
	case exNodeConfirmCmd.FullCommand():
		exchange.NodeConfirm(*exOrg, *exNodeConfirmNode, *exNodeConfirmToken, *exNodeConfirmNodeIdTok)
	case exNodeDelCmd.FullCommand():
		exchange.NodeRemove(*exOrg, credToUse, *exDelNode, *exNodeDelForce)
	case exNodeListPolicyCmd.FullCommand():
		exchange.NodeListPolicy(*exOrg, credToUse, *exNodeListPolicyNode)
	case exNodeAddPolicyCmd.FullCommand():
		exchange.NodeAddPolicy(*exOrg, credToUse, *exNodeAddPolicyNode, *exNodeAddPolicyJsonFile)
	case exNodeUpdatePolicyCmd.FullCommand():
		exchange.NodeUpdatePolicy(*exOrg, credToUse, *exNodeUpdatePolicyNode, *exNodeUpdatePolicyJsonFile)
	case exNodeRemovePolicyCmd.FullCommand():
		exchange.NodeRemovePolicy(*exOrg, credToUse, *exNodeRemovePolicyNode, *exNodeRemovePolicyForce)
	case exNodeErrorsList.FullCommand():
		exchange.NodeListErrors(*exOrg, credToUse, *exNodeErrorsListNode, *exNodeErrorsListLong)
	case exAgbotListCmd.FullCommand():
		exchange.AgbotList(*exOrg, *exUserPw, *exAgbot, !*exAgbotLong)
	case exAgbotListPatsCmd.FullCommand():
		exchange.AgbotListPatterns(*exOrg, *exUserPw, *exAgbotLP, *exAgbotLPPatOrg, *exAgbotLPPat, *exAgbotLPNodeOrg)
	case exAgbotAddPatCmd.FullCommand():
		exchange.AgbotAddPattern(*exOrg, *exUserPw, *exAgbotAP, *exAgbotAPPatOrg, *exAgbotAPPat, *exAgbotAPNodeOrg)
	case exAgbotDelPatCmd.FullCommand():
		exchange.AgbotRemovePattern(*exOrg, *exUserPw, *exAgbotDP, *exAgbotDPPatOrg, *exAgbotDPPat, *exAgbotDPNodeOrg)
	case exAgbotListPolicyCmd.FullCommand():
		exchange.AgbotListBusinessPolicy(*exOrg, *exUserPw, *exAgbotPol)
	case exAgbotAddPolCmd.FullCommand():
		exchange.AgbotAddBusinessPolicy(*exOrg, *exUserPw, *exAgbotAPolAg, *exAgbotAPPolOrg)
	case exAgbotDelPolCmd.FullCommand():
		exchange.AgbotRemoveBusinessPolicy(*exOrg, *exUserPw, *exAgbotDPolAg, *exAgbotDPPolOrg)
	case exPatternListCmd.FullCommand():
		exchange.PatternList(*exOrg, credToUse, *exPattern, !*exPatternLong)
	case exPatternPublishCmd.FullCommand():
		exchange.PatternPublish(*exOrg, *exUserPw, *exPatJsonFile, *exPatKeyFile, *exPatPubPubKeyFile, *exPatName)
	case exPatternVerifyCmd.FullCommand():
		exchange.PatternVerify(*exOrg, credToUse, *exVerPattern, *exPatPubKeyFile)
	case exPatDelCmd.FullCommand():
		exchange.PatternRemove(*exOrg, *exUserPw, *exDelPat, *exPatDelForce)
	case exPatternListKeyCmd.FullCommand():
		exchange.PatternListKey(*exOrg, credToUse, *exPatListKeyPat, *exPatListKeyKey)
	case exPatUpdateCmd.FullCommand():
		exchange.PatternUpdate(*exOrg, credToUse, *exPatUpdatePattern, *exPatUpdateJsonFile)
	case exPatternRemKeyCmd.FullCommand():
		exchange.PatternRemoveKey(*exOrg, *exUserPw, *exPatRemKeyPat, *exPatRemKeyKey)
	case exServiceListCmd.FullCommand():
		exchange.ServiceList(*exOrg, credToUse, *exService, !*exServiceLong)
	case exServicePublishCmd.FullCommand():
		exchange.ServicePublish(*exOrg, *exUserPw, *exSvcJsonFile, *exSvcPrivKeyFile, *exSvcPubPubKeyFile, *exSvcPubDontTouchImage, *exSvcPubPullImage, *exSvcRegistryTokens, *exSvcOverwrite, *exSvcPolicyFile)
	case exServiceVerifyCmd.FullCommand():
		exchange.ServiceVerify(*exOrg, credToUse, *exVerService, *exSvcPubKeyFile)
	case exSvcDelCmd.FullCommand():
		exchange.ServiceRemove(*exOrg, *exUserPw, *exDelSvc, *exSvcDelForce)
	case exServiceListKeyCmd.FullCommand():
		exchange.ServiceListKey(*exOrg, credToUse, *exSvcListKeySvc, *exSvcListKeyKey)
	case exServiceRemKeyCmd.FullCommand():
		exchange.ServiceRemoveKey(*exOrg, *exUserPw, *exSvcRemKeySvc, *exSvcRemKeyKey)
	case exServiceListAuthCmd.FullCommand():
		exchange.ServiceListAuth(*exOrg, credToUse, *exSvcListAuthSvc, *exSvcListAuthId)
	case exServiceRemAuthCmd.FullCommand():
		exchange.ServiceRemoveAuth(*exOrg, *exUserPw, *exSvcRemAuthSvc, *exSvcRemAuthId)
	case exServiceListPolicyCmd.FullCommand():
		exchange.ServiceListPolicy(*exOrg, credToUse, *exServiceListPolicyService)
	case exServiceNewPolicyCmd.FullCommand():
		exchange.ServiceNewPolicy()
	case exServiceAddPolicyCmd.FullCommand():
		exchange.ServiceAddPolicy(*exOrg, credToUse, *exServiceAddPolicyService, *exServiceAddPolicyJsonFile)
	case exServiceRemovePolicyCmd.FullCommand():
		exchange.ServiceRemovePolicy(*exOrg, credToUse, *exServiceRemovePolicyService, *exServiceRemovePolicyForce)
	case exBusinessListPolicyCmd.FullCommand():
		exchange.BusinessListPolicy(*exOrg, credToUse, *exBusinessListPolicyPolicy, !*exBusinessListPolicyLong)
	case exBusinessNewPolicyCmd.FullCommand():
		exchange.BusinessNewPolicy()
	case exBusinessAddPolicyCmd.FullCommand():
		exchange.BusinessAddPolicy(*exOrg, credToUse, *exBusinessAddPolicyPolicy, *exBusinessAddPolicyJsonFile)
	case exBusinessUpdatePolicyCmd.FullCommand():
		exchange.BusinessUpdatePolicy(*exOrg, credToUse, *exBusinessUpdatePolicyPolicy, *exBusinessUpdatePolicyJsonFile)
	case exBusinessRemovePolicyCmd.FullCommand():
		exchange.BusinessRemovePolicy(*exOrg, credToUse, *exBusinessRemovePolicyPolicy, *exBusinessRemovePolicyForce)
	case exCatalogServiceListCmd.FullCommand():
		exchange.CatalogServiceList(*exOrg, *exUserPw, *exCatalogServiceListShort, *exCatalogServiceListLong)
	case exCatalogPatternListCmd.FullCommand():
		exchange.CatalogPatternList(*exOrg, *exUserPw, *exCatalogPatternListShort, *exCatalogPatternListLong)
	case regInputCmd.FullCommand():
		register.CreateInputFile(*regInputOrg, *regInputPattern, *regInputArch, *regInputNodeIdTok, *regInputInputFile)
	case registerCmd.FullCommand():
		register.DoIt(*org, *pattern, *nodeIdTok, *userPw, *email, *inputFile, *nodeOrgFlag, *patternFlag, *nodeName, *nodepolicyFlag)
	case keyListCmd.FullCommand():
		key.List(*keyName, *keyListAll)
	case keyCreateCmd.FullCommand():
		key.Create(*keyX509Org, *keyX509CN, *keyOutputDir, *keyLength, *keyDaysValid, *keyImportFlag, *keyCreatePrivKey, *keyCreatePubKey, *keyCreateOverwrite)
	case keyImportCmd.FullCommand():
		key.Import(*keyImportPubKeyFile)
	case keyDelCmd.FullCommand():
		key.Remove(*keyDelName)
	case nodeListCmd.FullCommand():
		node.List()
	case policyListCmd.FullCommand():
		policy.List()
	case policyNewCmd.FullCommand():
		policy.New()
	case policyUpdateCmd.FullCommand():
		policy.Update(*policyUpdateInputFile)
	case policyPatchCmd.FullCommand():
		policy.Patch(*policyPatchInput)
	case policyRemoveCmd.FullCommand():
		policy.Remove(*policyRemoveForce)
	case policyCompCmd.FullCommand():
		deploycheck.PolicyCompatible(*deploycheckOrg, *deploycheckUserPw, *policyCompNodeId, *policyCompNodeArch, *policyCompNodePolFile, *policyCompBPolId, *policyCompBPolFile, *policyCompSPolFile, *deploycheckCheckAll, *deploycheckLong)
	case userinputCompCmd.FullCommand():
		deploycheck.UserInputCompatible(*deploycheckOrg, *deploycheckUserPw, *userinputCompNodeId, *userinputCompNodeArch, *userinputCompNodeUIFile, *userinputCompBPolId, *userinputCompBPolFile, *userinputCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
	case allCompCmd.FullCommand():
		deploycheck.AllCompatible(*deploycheckOrg, *deploycheckUserPw, *allCompNodeId, *allCompNodeArch, *allCompNodePolFile, *allCompNodeUIFile, *allCompBPolId, *allCompBPolFile, *allCompSPolFile, *allCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
	case agreementListCmd.FullCommand():
		agreement.List(*listArchivedAgreements, *listAgreementId)
	case agreementCancelCmd.FullCommand():
		agreement.Cancel(*cancelAgreementId, *cancelAllAgreements)
	case meteringListCmd.FullCommand():
		metering.List(*listArchivedMetering)
	case attributeListCmd.FullCommand():
		attribute.List()
	case userinputListCmd.FullCommand():
		userinput.List()
	case userinputNewCmd.FullCommand():
		userinput.New()
	case userinputAddCmd.FullCommand():
		userinput.Add(*userinputAddFilePath)
	case userinputUpdateCmd.FullCommand():
		userinput.Update(*userinputUpdateFilePath)
	case userinputRemoveCmd.FullCommand():
		userinput.Remove(*userinputRemoveForce)
	case serviceListCmd.FullCommand():
		service.List()
	case serviceLogCmd.FullCommand():
		service.Log(*logServiceName, *logTail)
	case serviceRegisteredCmd.FullCommand():
		service.Registered()
	case serviceConfigStateListCmd.FullCommand():
		service.ListConfigState()
	case serviceConfigStateSuspendCmd.FullCommand():
		service.Suspend(*forceSuspendService, *suspendAllServices, *suspendServiceOrg, *suspendServiceName)
	case serviceConfigStateActiveCmd.FullCommand():
		service.Resume(*resumeAllServices, *resumeServiceOrg, *resumeServiceName)
	case unregisterCmd.FullCommand():
		unregister.DoIt(*forceUnregister, *removeNodeUnregister, *deepCleanUnregister, *timeoutUnregister)
	case statusCmd.FullCommand():
		status.DisplayStatus(*statusLong, false)
	case eventlogListCmd.FullCommand():
		eventlog.List(*listAllEventlogs, *listDetailedEventlogs, *listSelectedEventlogs)
	case surfaceErrorsEventlogs.FullCommand():
		eventlog.ListSurfaced(*surfaceErrorsEventlogsLong)
	case devServiceNewCmd.FullCommand():
		dev.ServiceNew(*devHomeDirectory, *devServiceNewCmdOrg, *devServiceNewCmdName, *devServiceNewCmdVer, *devServiceNewCmdImage, *devServiceNewCmdNoImageGen, *devServiceNewCmdCfg, *devServiceNewCmdNoPattern, *devServiceNewCmdNoPolicy)
	case devServiceStartTestCmd.FullCommand():
		dev.ServiceStartTest(*devHomeDirectory, *devServiceUserInputFile, *devServiceConfigFile, *devServiceConfigType, *devServiceNoFSS, *devServiceStartCmdUserPw)
	case devServiceStopTestCmd.FullCommand():
		dev.ServiceStopTest(*devHomeDirectory)
	case devServiceValidateCmd.FullCommand():
		dev.ServiceValidate(*devHomeDirectory, *devServiceVerifyUserInputFile, []string{}, "", *devServiceValidateCmdUserPw)
	case devDependencyFetchCmd.FullCommand():
		dev.DependencyFetch(*devHomeDirectory, *devDependencyFetchCmdProject, *devDependencyCmdSpecRef, *devDependencyCmdURL, *devDependencyCmdOrg, *devDependencyCmdVersion, *devDependencyCmdArch, *devDependencyFetchCmdUserPw, *devDependencyFetchCmdUserInputFile)
	case devDependencyListCmd.FullCommand():
		dev.DependencyList(*devHomeDirectory)
	case devDependencyRemoveCmd.FullCommand():
		dev.DependencyRemove(*devHomeDirectory, *devDependencyCmdSpecRef, *devDependencyCmdURL, *devDependencyCmdVersion, *devDependencyCmdArch, *devDependencyCmdOrg)
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
	case utilConfigConvCmd.FullCommand():
		utilcmds.ConvertConfig(*utilConfigConvFile)
	case mmsStatusCmd.FullCommand():
		sync_service.Status(*mmsOrg, *mmsUserPw)
	case mmsObjectListCmd.FullCommand():
		sync_service.ObjectList(*mmsOrg, *mmsUserPw, *mmsObjectListType, *mmsObjectListId, *mmsObjectListDestinationPolicy, *mmsObjectListDPService, *mmsObjectListDPProperty, *mmsObjectListDPUpdateTime, *mmsObjectListDestinationType, *mmsObjectListDestinationId, *mmsObjectListWithData, *mmsObjectListExpirationTime, *mmsObjectListLong, *mmsObjectListDetail)
	case mmsObjectNewCmd.FullCommand():
		sync_service.ObjectNew(*mmsOrg)
	case mmsObjectPublishCmd.FullCommand():
		sync_service.ObjectPublish(*mmsOrg, *mmsUserPw, *mmsObjectPublishType, *mmsObjectPublishId, *mmsObjectPublishPat, *mmsObjectPublishDef, *mmsObjectPublishObj)
	case mmsObjectDeleteCmd.FullCommand():
		sync_service.ObjectDelete(*mmsOrg, *mmsUserPw, *mmsObjectDeleteType, *mmsObjectDeleteId)
	}
}

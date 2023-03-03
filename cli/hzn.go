// Command line interface to the horizon agent. Provide sub-commands to register an edge node, display info about the node, etc.
package main

import (
	"flag"
	"github.com/open-horizon/anax/cli/agreement"
	"github.com/open-horizon/anax/cli/agreementbot"
	"github.com/open-horizon/anax/cli/attribute"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/deploycheck"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/eventlog"
	"github.com/open-horizon/anax/cli/exchange"
	"github.com/open-horizon/anax/cli/fdo"
	_ "github.com/open-horizon/anax/cli/i18n_messages"
	"github.com/open-horizon/anax/cli/key"
	"github.com/open-horizon/anax/cli/kube_deployment"
	"github.com/open-horizon/anax/cli/metering"
	_ "github.com/open-horizon/anax/cli/native_deployment"
	"github.com/open-horizon/anax/cli/nm_status"
	"github.com/open-horizon/anax/cli/node"
	"github.com/open-horizon/anax/cli/node_management"
	"github.com/open-horizon/anax/cli/policy"
	"github.com/open-horizon/anax/cli/register"
	"github.com/open-horizon/anax/cli/sdo"
	secret_manager "github.com/open-horizon/anax/cli/secrets_manager"
	"github.com/open-horizon/anax/cli/service"
	"github.com/open-horizon/anax/cli/status"
	"github.com/open-horizon/anax/cli/sync_service"
	"github.com/open-horizon/anax/cli/unregister"
	"github.com/open-horizon/anax/cli/userinput"
	"github.com/open-horizon/anax/cli/utilcmds"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/anax/version"
	"gopkg.in/alecthomas/kingpin.v2"
	"os"
	"runtime"
	"strings"
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

	// the sample file direcory is different between Liunx and mac
	sample_dir := "/usr/horizon/samples"
	if runtime.GOOS == "darwin" {
		sample_dir = "/Users/Shared/horizon-cli/samples"
	}

	// Command flags and args - see https://github.com/alecthomas/kingpin
	app := kingpin.New("hzn", msgPrinter.Sprintf(`Command line interface for Horizon agent. Most of the sub-commands use the Horizon Agent API at the default location http://localhost (see environment Environment Variables section to override this).

Subcommands Description:
  agbot: List and manage Horizon agreement bot resources.
  agreement: List or manage the agreements this edge node has made with a Horizon agreement bot.
  architecture: Show the architecture of this machine. 
  attribute: List or manage the global attributes that are currently registered on this Horizon edge node.
  deploycheck: Check deployment compatibility.
  dev: Development tools for creation of services.
  env: Show the Horizon Environment Variables.
  eventlog: List the event logs for the current or all registrations.
  exchange: List and manage Horizon Exchange resources.
  key: List and manage keys for signing and verifying services. 
  mms: List and manage Horizon Model Management Service resources.
  nmstatus: List and manage node management status for the local node.
  node: List and manage general information about this Horizon edge node.
  nodemanagement: List and manage manifests and agent files for node management.
  policy: List and manage policy for this Horizon edge node. 
  reginput: Create an input file template for this pattern that can be used for the 'hzn register' command. 
  register: Register this edge node with the management hub.
  secretsmanager: List and manage secrets in the secrets manager.
  service: List or manage the services that are currently registered on this Horizon edge node.
  status: Display the current horizon internal status for the node.
  unregister: Unregister and reset this Horizon edge node.
  userinput: List or manager the service user inputs that are currently registered on this Horizon edge node.
  util: Utility commands.
  version: Show the Horizon version.
  sdo: List and manage Horizon SDO ownership vouchers and keys.
  fdo: List and manage Horizon FDO ownership vouchers and keys.

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
  HZN_SDO_SVC_URL:  Override the URL that the 'hzn sdo' sub-commands use
	  to communicate with SDO owner services. (By default hzn will ask the
		Horizon Agent for the URL.)
  HZN_FDO_SVC_URL:  Override the URL that the 'hzn fdo' sub-commands use
	  to communicate with FDO owner services. (By default hzn will ask the
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

	agbotCmd := app.Command("agbot", msgPrinter.Sprintf("List and manage Horizon agreement bot resources."))

	agbotAgreementCmd := agbotCmd.Command("agreement | ag", msgPrinter.Sprintf("List or manage the active or archived agreements this Horizon agreement bot has with edge nodes.")).Alias("ag").Alias("agreement")
	agbotAgreementCancelCmd := agbotAgreementCmd.Command("cancel | can", msgPrinter.Sprintf("Cancel 1 or all of the active agreements this Horizon agreement bot has with edge nodes. Usually an agbot will immediately negotiated a new agreement. ")).Alias("can").Alias("cancel")
	agbotCancelAllAgreements := agbotAgreementCancelCmd.Flag("all", msgPrinter.Sprintf("Cancel all of the current agreements.")).Short('a').Bool()
	agbotCancelAgreementId := agbotAgreementCancelCmd.Arg("agreement", msgPrinter.Sprintf("The active agreement to cancel.")).String()
	agbotAgreementListCmd := agbotAgreementCmd.Command("list | ls", msgPrinter.Sprintf("List the active or archived agreements this Horizon agreement bot has with edge nodes.")).Alias("ls").Alias("list")
	agbotlistArchivedAgreements := agbotAgreementListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreements instead of the active agreements.")).Short('r').Bool()
	agbotAgreement := agbotAgreementListCmd.Arg("agreement-id", msgPrinter.Sprintf("Show the details of this active or archived agreement.")).String()

	agbotCacheCmd := agbotCmd.Command("cache", msgPrinter.Sprintf("Manage cached agbot-serving organizations, patterns, and deployment policies."))
	agbotCacheDeployPol := agbotCacheCmd.Command("deploymentpol | dep", msgPrinter.Sprintf("List served deployment policies cached in the agbot.")).Alias("dep").Alias("deploymentpol")
	agbotCacheDeployPolList := agbotCacheDeployPol.Command("list | ls", msgPrinter.Sprintf("Display served deployment policies cached in the agbot.")).Alias("ls").Alias("list")
	agbotCacheDeployPolListOrg := agbotCacheDeployPolList.Flag("org", msgPrinter.Sprintf("Display policies under this org.")).Short('o').String()
	agbotCacheDeployPolListName := agbotCacheDeployPolList.Arg("name", msgPrinter.Sprintf("Display this policy.")).String()
	agbotCacheDeployPolListLong := agbotCacheDeployPolList.Flag("long", msgPrinter.Sprintf("Display detailed info.")).Short('l').Bool()
	agbotCachePattern := agbotCacheCmd.Command("pattern | pat", msgPrinter.Sprintf("List patterns cached in the agbot.")).Alias("pat").Alias("pattern")
	agbotCachePatternList := agbotCachePattern.Command("list | ls", msgPrinter.Sprintf("Display served patterns cached in the agbot.")).Alias("ls").Alias("list")
	agbotCachePatternListOrg := agbotCachePatternList.Flag("org", msgPrinter.Sprintf("Display patterns under this org.")).Short('o').String()
	agbotCachePatternListName := agbotCachePatternList.Arg("name", msgPrinter.Sprintf("Display this pattern.")).String()
	agbotCachePatternListLong := agbotCachePatternList.Flag("long", msgPrinter.Sprintf("Display detailed info.")).Short('l').Bool()
	agbotCacheServedOrg := agbotCacheCmd.Command("servedorg | sorg", msgPrinter.Sprintf("List served pattern orgs and deployment policy orgs.")).Alias("sorg").Alias("servedorg")
	agbotCacheServedOrgList := agbotCacheServedOrg.Command("list | ls", msgPrinter.Sprintf("Display served pattern orgs and deployment policy orgs.")).Alias("ls").Alias("list")

	agbotListCmd := agbotCmd.Command("list | ls", msgPrinter.Sprintf("Display general information about this Horizon agbot node.")).Alias("ls").Alias("list")
	agbotPolicyCmd := agbotCmd.Command("policy | pol", msgPrinter.Sprintf("List the policies this Horizon agreement bot hosts.")).Alias("pol").Alias("policy")
	agbotPolicyListCmd := agbotPolicyCmd.Command("list | ls", msgPrinter.Sprintf("List policies this Horizon agreement bot hosts.")).Alias("ls").Alias("list")
	agbotPolicyOrg := agbotPolicyListCmd.Arg("org", msgPrinter.Sprintf("The organization the policy belongs to.")).String()
	agbotPolicyName := agbotPolicyListCmd.Arg("name", msgPrinter.Sprintf("The policy name.")).String()
	agbotStatusCmd := agbotCmd.Command("status", msgPrinter.Sprintf("Display the current horizon internal status for the Horizon agreement bot."))
	agbotStatusLong := agbotStatusCmd.Flag("long", msgPrinter.Sprintf("Show detailed status")).Short('l').Bool()

	agreementCmd := app.Command("agreement | ag", msgPrinter.Sprintf("List or manage the active or archived agreements this edge node has made with a Horizon agreement bot.")).Alias("ag").Alias("agreement")
	agreementListCmd := agreementCmd.Command("list | ls", msgPrinter.Sprintf("List the active or archived agreements this edge node has made with a Horizon agreement bot.")).Alias("ls").Alias("list")
	listAgreementId := agreementListCmd.Arg("agreement-id", msgPrinter.Sprintf("Show the details of this active or archived agreement.")).String()
	listArchivedAgreements := agreementListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreements instead of the active agreements.")).Short('r').Bool()
	agreementCancelCmd := agreementCmd.Command("cancel | can", msgPrinter.Sprintf("Cancel 1 or all of the active agreements this edge node has made with a Horizon agreement bot. Usually an agbot will immediately negotiated a new agreement. If you want to cancel all agreements and not have this edge accept new agreements, run 'hzn unregister'.")).Alias("can").Alias("cancel")
	cancelAllAgreements := agreementCancelCmd.Flag("all", msgPrinter.Sprintf("Cancel all of the current agreements.")).Short('a').Bool()
	cancelAgreementId := agreementCancelCmd.Arg("agreement-id", msgPrinter.Sprintf("The active agreement to cancel.")).String()

	archCmd := app.Command("architecture", msgPrinter.Sprintf("Show the architecture of this machine (as defined by Horizon and golang)."))

	attributeCmd := app.Command("attribute | attr", msgPrinter.Sprintf("List or manage the global attributes that are currently registered on this Horizon edge node.")).Alias("attr").Alias("attribute")
	attributeListCmd := attributeCmd.Command("list | ls", msgPrinter.Sprintf("List the global attributes that are currently registered on this Horizon edge node.")).Alias("ls").Alias("list")

	deploycheckCmd := app.Command("deploycheck | dc", msgPrinter.Sprintf("Check deployment compatibility.")).Alias("dc").Alias("deploycheck")
	deploycheckOrg := deploycheckCmd.Flag("org", msgPrinter.Sprintf("The Horizon exchange organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	deploycheckUserPw := deploycheckCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon exchange user credential to query exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH or HZN_EXCHANGE_NODE_AUTH will be used as a default. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()
	deploycheckCheckAll := deploycheckCmd.Flag("check-all", msgPrinter.Sprintf("Show the compatibility status of all the service versions referenced in the deployment policy.")).Short('c').Bool()
	deploycheckLong := deploycheckCmd.Flag("long", msgPrinter.Sprintf("Show policies and userinput used for the compatibility checking.")).Short('l').Bool()
	allCompCmd := deploycheckCmd.Command("all", msgPrinter.Sprintf("Check all compatibilities for a deployment."))
	allCompNodeArch := allCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the deployment policy or pattern will be checked for compatibility.")).Short('a').String()
	allCompNodeType := allCompCmd.Flag("node-type", msgPrinter.Sprintf("The node type. The valid values are 'device' and 'cluster'. The default value is the type of the node provided by -n or current registered device, if omitted.")).Short('t').String()
	allCompNodeOrg := allCompCmd.Flag("node-org", msgPrinter.Sprintf("The organization of the node. The default value is the organization of the node provided by -n or current registered device, if omitted.")).Short('O').String()
	allCompNodeId := allCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --ha-group, --node-pol and --node-ui. If omitted, the node ID that the current device is registered with will be used. This flag can be repeated to specify more than one nodes. If you don't prepend a node id with the organization id, it will automatically be prepended with the -o value.")).Short('n').Strings()
	allCompHAGroup := allCompCmd.Flag("ha-group", msgPrinter.Sprintf("The name of an HA group. Mutually exclusive with -n, --node-pol and --node-ui.")).String()
	allCompNodePolFile := allCompCmd.Flag("node-pol", msgPrinter.Sprintf("The JSON input file name containing the node policy. Mutually exclusive with -n, --ha-group, -p and -P.")).String()
	allCompNodeUIFile := allCompCmd.Flag("node-ui", msgPrinter.Sprintf("The JSON input file name containing the node user input. Mutually exclusive with -n, --ha-group.")).String()
	allCompBPolId := allCompCmd.Flag("business-pol-id", "").Hidden().String()
	allCompDepPolId := allCompCmd.Flag("deployment-pol-id", msgPrinter.Sprintf("The Horizon exchange deployment policy ID. Mutually exclusive with -B, -p and -P. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	allCompBPolFile := allCompCmd.Flag("business-pol", "").Hidden().String()
	allCompDepPolFile := allCompCmd.Flag("deployment-pol", msgPrinter.Sprintf("The JSON input file name containing the deployment policy. Mutually exclusive with -b, -p and -P.")).Short('B').String()
	allCompSPolFile := allCompCmd.Flag("service-pol", msgPrinter.Sprintf("(optional) The JSON input file name containing the service policy. Mutually exclusive with -p and -P. If omitted, the service policy will be retrieved from the Exchange for the service defined in the deployment policy.")).String()
	allCompSvcFile := allCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. If omitted, the service defined in the deployment policy or pattern will be retrieved from the Exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	allCompPatternId := allCompCmd.Flag("pattern-id", msgPrinter.Sprintf("The Horizon exchange pattern ID. Mutually exclusive with -P, -b, -B --node-pol and --service-pol. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('p').String()
	allCompPatternFile := allCompCmd.Flag("pattern", msgPrinter.Sprintf("The JSON input file name containing the pattern. Mutually exclusive with -p, -b and -B, --node-pol and --service-pol.")).Short('P').String()
	policyCompCmd := deploycheckCmd.Command("policy | pol", msgPrinter.Sprintf("Check policy compatibility.")).Alias("pol").Alias("policy")
	policyCompNodeArch := policyCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the deployment policy will be checked for compatibility.")).Short('a').String()
	policyCompNodeType := policyCompCmd.Flag("node-type", msgPrinter.Sprintf("The node type. The valid values are 'device' and 'cluster'. The default value is the type of the node provided by -n or current registered device, if omitted.")).Short('t').String()
	policyCompNodeId := policyCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --ha-group and --node-pol. If omitted, the node ID that the current device is registered with will be used. This flag can be repeated to specify more than one nodes. If you don't prepend a node id with the organization id, it will automatically be prepended with the -o value.")).Short('n').Strings()
	policyCompHAGroup := policyCompCmd.Flag("ha-group", msgPrinter.Sprintf("The name of an HA group. Mutually exclusive with -n and --node-pol.")).String()
	policyCompNodePolFile := policyCompCmd.Flag("node-pol", msgPrinter.Sprintf("The JSON input file name containing the node policy. Mutually exclusive with -n, --ha-group.")).String()
	policyCompBPolId := policyCompCmd.Flag("business-pol-id", "").Hidden().String()
	policyCompDepPolId := policyCompCmd.Flag("deployment-pol-id", msgPrinter.Sprintf("The Horizon exchange deployment policy ID. Mutually exclusive with -B. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	policyCompBPolFile := policyCompCmd.Flag("business-pol", "").Hidden().String()
	policyCompDepPolFile := policyCompCmd.Flag("deployment-pol", msgPrinter.Sprintf("The JSON input file name containing the Deployment policy. Mutually exclusive with -b.")).Short('B').String()
	policyCompSPolFile := policyCompCmd.Flag("service-pol", msgPrinter.Sprintf("(optional) The JSON input file name containing the service policy. If omitted, the service policy will be retrieved from the Exchange for the service defined in the deployment policy.")).String()
	policyCompSvcFile := policyCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. Mutually exclusive with -b. If omitted, the service referenced in the deployment policy is retrieved from the Exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	secretCompCmd := deploycheckCmd.Command("secretbinding | sb", msgPrinter.Sprintf("Check secret bindings.")).Alias("sb").Alias("secretbinding")
	secretCompNodeArch := secretCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the deployment policy or pattern will be checked for compatibility.")).Short('a').String()
	secretCompNodeOrg := secretCompCmd.Flag("node-org", msgPrinter.Sprintf("The organization of the node. The default value is the organization of the node provided by -n or current registered device, if omitted.")).Short('O').String()
	secretCompNodeType := secretCompCmd.Flag("node-type", msgPrinter.Sprintf("The node type. The valid values are 'device' and 'cluster'. The default value is the type of the node provided by -n or current registered device, if omitted.")).Short('t').String()
	secretCompNodeId := secretCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. If omitted, the node ID that the current device is registered with will be used. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('n').String()
	secretCompDepPolId := secretCompCmd.Flag("deployment-pol-id", msgPrinter.Sprintf("The Horizon exchange deployment policy ID. Mutually exclusive with -B, -p and -P. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	secretCompDepPolFile := secretCompCmd.Flag("deployment-pol", msgPrinter.Sprintf("The JSON input file name containing the deployment policy. Mutually exclusive with -b, -p and -P.")).Short('B').String()
	secretCompSvcFile := secretCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. If omitted, the service defined in the deployment policy or pattern will be retrieved from the Exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	secretCompPatternId := secretCompCmd.Flag("pattern-id", msgPrinter.Sprintf("The Horizon exchange pattern ID. Mutually exclusive with -P, -b and -B. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('p').String()
	secretCompPatternFile := secretCompCmd.Flag("pattern", msgPrinter.Sprintf("The JSON input file name containing the pattern. Mutually exclusive with -p, -b and -B.")).Short('P').String()
	userinputCompCmd := deploycheckCmd.Command("userinput | u", msgPrinter.Sprintf("Check user input compatibility.")).Alias("u").Alias("userinput")
	userinputCompNodeArch := userinputCompCmd.Flag("arch", msgPrinter.Sprintf("The architecture of the node. It is required when -n is not specified. If omitted, the service of all the architectures referenced in the deployment policy or pattern will be checked for compatibility.")).Short('a').String()
	userinputCompNodeType := userinputCompCmd.Flag("node-type", msgPrinter.Sprintf("The node type. The valid values are 'device' and 'cluster'. The default value is the type of the node provided by -n or current registered device, if omitted.")).Short('t').String()
	userinputCompNodeId := userinputCompCmd.Flag("node-id", msgPrinter.Sprintf("The Horizon exchange node ID. Mutually exclusive with --node-ui. If omitted, the node ID that the current device is registered with will be used. If you don't prepend it with the organization id, it will automatically be prepended with the -o value.")).Short('n').String()
	userinputCompNodeUIFile := userinputCompCmd.Flag("node-ui", msgPrinter.Sprintf("The JSON input file name containing the node user input. Mutually exclusive with -n.")).String()
	userinputCompBPolId := userinputCompCmd.Flag("business-pol-id", "").Hidden().String()
	userinputCompDepPolId := userinputCompCmd.Flag("deployment-pol-id", msgPrinter.Sprintf("The Horizon exchange deployment policy ID. Mutually exclusive with -B, -p and -P. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('b').String()
	userinputCompBPolFile := userinputCompCmd.Flag("business-pol", "").Hidden().String()
	userinputCompDepPolFile := userinputCompCmd.Flag("deployment-pol", msgPrinter.Sprintf("The JSON input file name containing the deployment policy. Mutually exclusive with -b, -p and -P.")).Short('B').String()
	userinputCompSvcFile := userinputCompCmd.Flag("service", msgPrinter.Sprintf("(optional) The JSON input file name containing the service definition. If omitted, the service defined in the deployment policy or pattern will be retrieved from the Exchange. This flag can be repeated to specify different versions of the service.")).Strings()
	userinputCompPatternId := userinputCompCmd.Flag("pattern-id", msgPrinter.Sprintf("The Horizon exchange pattern ID. Mutually exclusive with -P, -b and -B. If you don't prepend it with the organization id, it will automatically be prepended with the node's organization id.")).Short('p').String()
	userinputCompPatternFile := userinputCompCmd.Flag("pattern", msgPrinter.Sprintf("The JSON input file name containing the pattern. Mutually exclusive with -p, -b and -B.")).Short('P').String()

	devCmd := app.Command("dev", msgPrinter.Sprintf("Development tools for creation of services."))
	devHomeDirectory := devCmd.Flag("directory", msgPrinter.Sprintf("Directory containing Horizon project metadata. If omitted, a subdirectory called 'horizon' under current directory will be used.")).Short('d').String()

	devDependencyCmd := devCmd.Command("dependency | dep", msgPrinter.Sprintf("For working with project dependencies.")).Alias("dep").Alias("dependency")
	devDependencyCmdSpecRef := devDependencyCmd.Flag("specRef", msgPrinter.Sprintf("The URL of the service dependency in the Exchange. Mutually exclusive with -p and --url.")).Short('s').String()
	devDependencyCmdURL := devDependencyCmd.Flag("url", msgPrinter.Sprintf("The URL of the service dependency in the Exchange. Mutually exclusive with -p and --specRef.")).String()
	devDependencyCmdOrg := devDependencyCmd.Flag("org", msgPrinter.Sprintf("The Org of the service dependency in the Exchange. Mutually exclusive with -p.")).Short('o').String()
	devDependencyCmdVersion := devDependencyCmd.Flag("ver", msgPrinter.Sprintf("(optional) The Version of the service dependency in the Exchange. Mutually exclusive with -p.")).String()
	devDependencyCmdArch := devDependencyCmd.Flag("arch", msgPrinter.Sprintf("(optional) The hardware Architecture of the service dependency in the Exchange. Mutually exclusive with -p.")).Short('a').String()
	devDependencyFetchCmd := devDependencyCmd.Command("fetch | f", msgPrinter.Sprintf("Retrieving Horizon metadata for a new dependency.")).Alias("f").Alias("fetch")
	devDependencyFetchCmdProject := devDependencyFetchCmd.Flag("project", msgPrinter.Sprintf("Horizon project containing the definition of a dependency. Mutually exclusive with -s -o --ver -a and --url.")).Short('p').ExistingDir()
	devDependencyFetchCmdUserPw := devDependencyFetchCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()
	devDependencyFetchCmdUserInputFile := devDependencyFetchCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for configuring the new dependency. If omitted, the userinput file in the dependency project will be used.")).Short('f').ExistingFile()
	devDependencyListCmd := devDependencyCmd.Command("list | ls", msgPrinter.Sprintf("List all dependencies.")).Alias("ls").Alias("list")
	devDependencyRemoveCmd := devDependencyCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a project dependency.")).Alias("rm").Alias("remove")

	devServiceCmd := devCmd.Command("service | serv", msgPrinter.Sprintf("For working with a service project.")).Alias("serv").Alias("service")
	devServiceLogCmd := devServiceCmd.Command("log", msgPrinter.Sprintf("Show the container/system logs for a service."))
	devServiceLogCmdServiceName := devServiceLogCmd.Arg("service", msgPrinter.Sprintf("The name of the service whose log records should be displayed. The service name is the same as the url field of a service definition.")).String()
	devServiceLogCmd.Flag("service", msgPrinter.Sprintf("(DEPRECATED) This flag is deprecated and is replaced by -c.")).Short('s').String()
	devServiceLogCmdContainerName := devServiceLogCmd.Flag("container", msgPrinter.Sprintf("The name of the service container whose log records should be displayed. Can be omitted if the service definition has only one container in its deployment config.")).Default(*devServiceLogCmdServiceName).Short('c').String()
	devServiceLogCmdTail := devServiceLogCmd.Flag("tail", msgPrinter.Sprintf("Continuously polls the service's logs to display the most recent records, similar to tail -F behavior.")).Short('f').Bool()
	devServiceNewCmd := devServiceCmd.Command("new", msgPrinter.Sprintf("Create a new service project."))
	devServiceNewCmdOrg := devServiceNewCmd.Flag("org", msgPrinter.Sprintf("The Org id that the service is defined within. If this flag is omitted, the HZN_ORG_ID environment variable is used.")).Short('o').String()
	devServiceNewCmdName := devServiceNewCmd.Flag("specRef", msgPrinter.Sprintf("The name of the service. If this flag and the -i flag are omitted, only the skeletal horizon metadata files will be generated.")).Short('s').String()
	devServiceNewCmdVer := devServiceNewCmd.Flag("ver", msgPrinter.Sprintf("The version of the service. If this flag is omitted, '0.0.1' is used.")).Short('V').String()
	devServiceNewCmdImage := devServiceNewCmd.Flag("image", msgPrinter.Sprintf("The docker container image base name without the version tag for the service. This command will add arch and version to the base name to form the final image name. The format is 'basename_arch:serviceversion'. This flag can be repeated to specify multiple images when '--noImageGen' flag is specified. This flag is ignored for the '--dconfig %v' deployment configuration.", kube_deployment.KUBE_DEPLOYMENT_CONFIG_TYPE)).Short('i').Strings()
	devServiceNewCmdNoImageGen := devServiceNewCmd.Flag("noImageGen", msgPrinter.Sprintf("Indicates that the image is built somewhere else. No image sample code will be created by this command. If this flag is not specified, files for generating a simple service image will be created under current directory.")).Bool()
	devServiceNewCmdNoPattern := devServiceNewCmd.Flag("noPattern", msgPrinter.Sprintf("Indicates no pattern definition file will be created.")).Bool()
	devServiceNewCmdNoPolicy := devServiceNewCmd.Flag("noPolicy", msgPrinter.Sprintf("Indicate no policy file will be created.")).Bool()
	devServiceNewCmdCfg := devServiceNewCmd.Flag("dconfig", msgPrinter.Sprintf("Indicates the type of deployment configuration that will be used, native (the default), or %v. This flag can be specified more than once to create a service with more than 1 kind of deployment configuration.", kube_deployment.KUBE_DEPLOYMENT_CONFIG_TYPE)).Short('c').Default("native").Strings()
	devServiceStartTestCmd := devServiceCmd.Command("start", msgPrinter.Sprintf("Run a service in a mocked Horizon Agent environment. This command is not supported for services using the %v deployment configuration.", kube_deployment.KUBE_DEPLOYMENT_CONFIG_TYPE))
	devServiceUserInputFile := devServiceStartTestCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for running a test. If omitted, the userinput file for the project will be used.")).Short('f').String()
	devServiceConfigFile := devServiceStartTestCmd.Flag("configFile", msgPrinter.Sprintf("File to be made available through the sync service APIs. This flag can be repeated to populate multiple files.")).Short('m').Strings()
	devServiceConfigType := devServiceStartTestCmd.Flag("type", msgPrinter.Sprintf("The type of file to be made available through the sync service APIs. All config files are presumed to be of the same type. This flag is required if any configFiles are specified.")).Short('t').String()
	devServiceNoFSS := devServiceStartTestCmd.Flag("noFSS", msgPrinter.Sprintf("Do not bring up file sync service (FSS) containers. They are brought up by default.")).Short('S').Bool()
	devServiceStartCmdUserPw := devServiceStartTestCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. Specify it when you want to automatically fetch the missing dependent services from the Exchange. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()
	devServiceStartSecretsFiles := devServiceStartTestCmd.Flag("secret", msgPrinter.Sprintf("Filepath of a file containing a secret that is required by the service or one of its dependent services. The filename must match a secret name in the service definition. The file is encoded in JSON as an object containing two keys both typed as a string; \"key\" is used to indicate the kind of secret, and \"value\" is the string form of the secret. This flag can be repeated.")).Strings()
	devServiceStopTestCmd := devServiceCmd.Command("stop", msgPrinter.Sprintf("Stop a service that is running in a mocked Horizon Agent environment. This command is not supported for services using the %v deployment configuration.", kube_deployment.KUBE_DEPLOYMENT_CONFIG_TYPE))
	devServiceValidateCmd := devServiceCmd.Command("verify | vf", msgPrinter.Sprintf("Validate the project for completeness and schema compliance.")).Alias("vf").Alias("verify")
	devServiceVerifyUserInputFile := devServiceValidateCmd.Flag("userInputFile", msgPrinter.Sprintf("File containing user input values for verification of a project. If omitted, the userinput file for the project will be used.")).Short('f').String()
	devServiceValidateCmdUserPw := devServiceValidateCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query exchange resources. Specify it when you want to automatically fetch the missing dependent services from the Exchange. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()

	envCmd := app.Command("env", msgPrinter.Sprintf("Show the Horizon Environment Variables."))

	eventlogCmd := app.Command("eventlog | ev", msgPrinter.Sprintf("List the event logs for the current or all registrations.")).Alias("ev").Alias("eventlog")
	eventlogListCmd := eventlogCmd.Command("list | ls", msgPrinter.Sprintf("List the event logs for the current or all registrations.")).Alias("ls").Alias("list")
	listTail := eventlogListCmd.Flag("tail", msgPrinter.Sprintf("Continuously polls the event log to display the most recent records, similar to tail -F behavior.")).Short('f').Bool()
	listAllEventlogs := eventlogListCmd.Flag("all", msgPrinter.Sprintf("List all the event logs including the previous registrations.")).Short('a').Bool()
	listDetailedEventlogs := eventlogListCmd.Flag("long", msgPrinter.Sprintf("List event logs with details.")).Short('l').Bool()
	listSelectedEventlogs := eventlogListCmd.Flag("select", msgPrinter.Sprintf("Selection string. This flag can be repeated which means 'AND'. Each flag should be in the format of attribute=value, attribute~value, \"attribute>value\" or \"attribute<value\", where '~' means contains. The common attribute names are timestamp, severity, message, event_code, source_type, agreement_id, service_url etc. Use the '-l' flag to see all the attribute names.")).Short('s').Strings()
	surfaceErrorsEventlogs := eventlogCmd.Command("surface | sf", msgPrinter.Sprintf("List all the active errors that will be shared with the Exchange if the node is online.")).Alias("sf").Alias("surface")
	surfaceErrorsEventlogsLong := surfaceErrorsEventlogs.Flag("long", msgPrinter.Sprintf("List the full event logs of the surface errors.")).Short('l').Bool()

	exchangeCmd := app.Command("exchange | ex", msgPrinter.Sprintf("List and manage Horizon Exchange resources.")).Alias("ex").Alias("exchange")
	exOrg := exchangeCmd.Flag("org", msgPrinter.Sprintf("The Horizon exchange organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	exUserPw := exchangeCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange user credentials to query and create exchange resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value. As an alternative to using -o, you can set HZN_ORG_ID with the Horizon exchange organization ID")).Short('u').PlaceHolder("USER:PW").String()

	exAgbotCmd := exchangeCmd.Command("agbot", msgPrinter.Sprintf("List and manage agbots in the Horizon Exchange"))
	exAgbotAddPolCmd := exAgbotCmd.Command("adddeploymentpol | addpo", msgPrinter.Sprintf("Add this deployment policy to the list of policies this agbot is serving. Currently only support adding all the deployment policies from an organization.")).Alias("addbusinesspol").Alias("addpo").Alias("adddeploymentpol")
	exAgbotAPolAg := exAgbotAddPolCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to add the deployment policy to.")).Required().String()
	exAgbotAPPolOrg := exAgbotAddPolCmd.Arg("policyorg", msgPrinter.Sprintf("The organization of the deployment policy to add.")).Required().String()
	exAgbotAddPatCmd := exAgbotCmd.Command("addpattern | addpa", msgPrinter.Sprintf("Add this pattern to the list of patterns this agbot is serving.")).Alias("addpa").Alias("addpattern")
	exAgbotAP := exAgbotAddPatCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to add the pattern to.")).Required().String()
	exAgbotAPPatOrg := exAgbotAddPatCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the pattern to add.")).Required().String()
	exAgbotAPPat := exAgbotAddPatCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern to add.")).Required().String()
	exAgbotAPNodeOrg := exAgbotAddPatCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()
	exAgbotListCmd := exAgbotCmd.Command("list | ls", msgPrinter.Sprintf("Display the agbot resources from the Horizon Exchange.")).Alias("ls").Alias("list")
	exAgbot := exAgbotListCmd.Arg("agbot", msgPrinter.Sprintf("List just this one agbot.")).String()
	exAgbotLong := exAgbotListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the agbots, show the entire resource of each agbots, instead of just the name.")).Short('l').Bool()
	exAgbotListPolicyCmd := exAgbotCmd.Command("listdeploymentpol | lspo", msgPrinter.Sprintf("Display the deployment policies that this agbot is serving.")).Alias("listbusinesspol").Alias("lspo").Alias("listdeploymentpol")
	exAgbotPol := exAgbotListPolicyCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to list serving deployment policies for.")).Required().String()
	exAgbotListPatsCmd := exAgbotCmd.Command("listpattern | lspa", msgPrinter.Sprintf("Display the patterns that this agbot is serving.")).Alias("lspa").Alias("listpattern")
	exAgbotLP := exAgbotListPatsCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to list the patterns for.")).Required().String()
	exAgbotLPPatOrg := exAgbotListPatsCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the 1 pattern to list.")).String()
	exAgbotLPPat := exAgbotListPatsCmd.Arg("pattern", msgPrinter.Sprintf("The name of the 1 pattern to list.")).String()
	exAgbotLPNodeOrg := exAgbotListPatsCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()
	exAgbotDelPolCmd := exAgbotCmd.Command("removedeploymentpol | rmpo", msgPrinter.Sprintf("Remove this deployment policy from the list of policies this agbot is serving. Currently only support removing all the deployment policies from an organization.")).Alias("removebusinesspol").Alias("rmpo").Alias("removedeploymentpol")
	exAgbotDPolAg := exAgbotDelPolCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to remove the deployment policy from.")).Required().String()
	exAgbotDPPolOrg := exAgbotDelPolCmd.Arg("policyorg", msgPrinter.Sprintf("The organization of the deployment policy to remove.")).Required().String()
	exAgbotDelPatCmd := exAgbotCmd.Command("removepattern | rmpa", msgPrinter.Sprintf("Remove this pattern from the list of patterns this agbot is serving.")).Alias("rmpa").Alias("removepattern")
	exAgbotDP := exAgbotDelPatCmd.Arg("agbot", msgPrinter.Sprintf("The agbot to remove the pattern from.")).Required().String()
	exAgbotDPPatOrg := exAgbotDelPatCmd.Arg("patternorg", msgPrinter.Sprintf("The organization of the pattern to remove.")).Required().String()
	exAgbotDPPat := exAgbotDelPatCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern to remove.")).Required().String()
	exAgbotDPNodeOrg := exAgbotDelPatCmd.Arg("nodeorg", msgPrinter.Sprintf("The organization of the nodes that should be searched. Defaults to patternorg.")).String()

	exCatalogCmd := exchangeCmd.Command("catalog | cat", msgPrinter.Sprintf("List all public services/patterns in all orgs that have orgType: IBM.")).Alias("cat").Alias("catalog")
	exCatalogPatternListCmd := exCatalogCmd.Command("patternlist | pat", msgPrinter.Sprintf("Display all public patterns in all orgs that have orgType: IBM. ")).Alias("pat").Alias("patternlist")
	exCatalogPatternListShort := exCatalogPatternListCmd.Flag("short", msgPrinter.Sprintf("Only display org (IBM) and pattern names.")).Short('s').Bool()
	exCatalogPatternListLong := exCatalogPatternListCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about public patterns in all orgs that have orgType: IBM.")).Short('l').Bool()
	exCatalogServiceListCmd := exCatalogCmd.Command("servicelist | serv", msgPrinter.Sprintf("Display all public services in all orgs that have orgType: IBM.")).Alias("serv").Alias("servicelist")
	exCatalogServiceListShort := exCatalogServiceListCmd.Flag("short", msgPrinter.Sprintf("Only display org (IBM) and service names.")).Short('s').Bool()
	exCatalogServiceListLong := exCatalogServiceListCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about public services in all orgs that have orgType: IBM.")).Short('l').Bool()

	exBusinessCmd := exchangeCmd.Command("deployment | dep", msgPrinter.Sprintf("List and manage deployment policies in the Horizon Exchange.")).Alias("business").Alias("dep").Alias("deployment")
	exBusinessAddPolicyCmd := exBusinessCmd.Command("addpolicy | addp", msgPrinter.Sprintf("Add or replace a deployment policy in the Horizon Exchange. Use 'hzn exchange deployment new' for an empty deployment policy template.")).Alias("addp").Alias("addpolicy")
	exBusinessAddPolicyIdTok := exBusinessAddPolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessAddPolicyPolicy := exBusinessAddPolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the deployment policy to add or overwrite.")).Required().String()
	exBusinessAddPolicyJsonFile := exBusinessAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exBusinessAddPolNoConstraint := exBusinessAddPolicyCmd.Flag("no-constraints", msgPrinter.Sprintf("Allow this deployment policy to be published even though it does not have any constraints.")).Bool()
	exBusinessListPolicyCmd := exBusinessCmd.Command("listpolicy | ls", msgPrinter.Sprintf("Display the deployment policies from the Horizon Exchange.")).Alias("ls").Alias("listpolicy")
	exBusinessListPolicyIdTok := exBusinessListPolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessListPolicyLong := exBusinessListPolicyCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about the deployment policies.")).Short('l').Bool()
	exBusinessListPolicyPolicy := exBusinessListPolicyCmd.Arg("policy", msgPrinter.Sprintf("List just this one deployment policy. Use <org>/<policy> to specify a public policy in another org, or <org>/ to list all of the public policies in another org.")).String()
	exBusinessNewPolicyCmd := exBusinessCmd.Command("new", msgPrinter.Sprintf("Display an empty deployment policy template that can be filled in."))
	exBusinessRemovePolicyCmd := exBusinessCmd.Command("removepolicy | rmp", msgPrinter.Sprintf("Remove the deployment policy in the Horizon Exchange.")).Alias("rmp").Alias("removepolicy")
	exBusinessRemovePolicyIdTok := exBusinessRemovePolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessRemovePolicyForce := exBusinessRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exBusinessRemovePolicyPolicy := exBusinessRemovePolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the deployment policy to be removed.")).Required().String()
	exBusinessUpdatePolicyCmd := exBusinessCmd.Command("updatepolicy | upp", msgPrinter.Sprintf("Update one attribute of an existing deployment policy in the Horizon Exchange. The supported attributes are the top level attributes in the policy definition as shown by the command 'hzn exchange deployment new'.")).Alias("upp").Alias("updatepolicy")
	exBusinessUpdatePolicyIdTok := exBusinessUpdatePolicyCmd.Flag("id-token", msgPrinter.Sprintf("The Horizon ID and password of the user.")).Short('n').PlaceHolder("ID:TOK").String()
	exBusinessUpdatePolicyPolicy := exBusinessUpdatePolicyCmd.Arg("policy", msgPrinter.Sprintf("The name of the policy to be updated in the Horizon Exchange.")).Required().String()
	exBusinessUpdatePolicyJsonFile := exBusinessUpdatePolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path to the json file containing the updated deployment policy attribute to be changed in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()

	exNMPCmd := exchangeCmd.Command("nmp", msgPrinter.Sprintf("List and manage node management policies in the Horizon Exchange."))
	exNMPListCmd := exNMPCmd.Command("list | ls", msgPrinter.Sprintf("Display the node management policies from the Horizon Exchange.")).Alias("ls").Alias("list")
	exNMPListName := exNMPListCmd.Arg("nmp-name", msgPrinter.Sprintf("List just this one node management policy.")).String()
	exNMPListIdTok := exNMPListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNMPListLong := exNMPListCmd.Flag("long", msgPrinter.Sprintf("Display detailed output about the node management policies.")).Short('l').Bool()
	exNMPListNodes := exNMPListCmd.Flag("nodes", msgPrinter.Sprintf("List all the nodes that apply for the given node management policy.")).Bool()
	exNMPAddCmd := exNMPCmd.Command("add", msgPrinter.Sprintf("Add or replace a node management policy in the Horizon Exchange. Use 'hzn exchange nmp new' for an empty node management policy template."))
	exNMPAddAppliesTo := exNMPAddCmd.Flag("applies-to", msgPrinter.Sprintf("List all the nodes that will be compatible with this node management policy. Use this flag with --dry-run to list nodes without publishing the policy to the Exchange.")).Bool()
	exNMPAddName := exNMPAddCmd.Arg("nmp-name", msgPrinter.Sprintf("The name of the node management policy to add or overwrite.")).Required().String()
	exNMPAddJsonFile := exNMPAddCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the node management policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exNMPAddNoConstraint := exNMPAddCmd.Flag("no-constraints", msgPrinter.Sprintf("Allow this node management policy to be published even though it does not have any constraints.")).Bool()
	exNMPNewCmd := exNMPCmd.Command("new", msgPrinter.Sprintf("Display an empty node management policy template that can be filled in."))
	exNMPRemoveCmd := exNMPCmd.Command("remove | rm", msgPrinter.Sprintf("Remove the node management policy in the Horizon Exchange.")).Alias("rm").Alias("remove")
	exNMPRemoveName := exNMPRemoveCmd.Arg("nmp-name", msgPrinter.Sprintf("The name of the node management policy to be removed.")).Required().String()
	exNMPRemoveForce := exNMPRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exNMPStatusCmd := exNMPCmd.Command("status", msgPrinter.Sprintf("List the status of a given node management policy across all nodes in given org."))
	exNMPStatusName := exNMPStatusCmd.Arg("nmp-name", msgPrinter.Sprintf("The name of the node management policy status to check.")).Required().String()
	exNMPStatusIdTok := exNMPStatusCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNMPStatusNode := exNMPStatusCmd.Flag("node", msgPrinter.Sprintf("Filter output to include just this one node. Use with --long flag to display entire content of a single node management policy status object.")).Short('N').String()
	exNMPStatusLong := exNMPStatusCmd.Flag("long", msgPrinter.Sprintf("Show the entire contents of each node management policy status object.")).Short('l').Bool()
	exNMPEnableCmd := exNMPCmd.Command("enable", msgPrinter.Sprintf("Enable a node management policy in the Horizon Exchange."))
	exNMPEnableName := exNMPEnableCmd.Arg("nmp-name", msgPrinter.Sprintf("The name of the node management policy to enable.")).String()
	exNMPEnableStartTime := exNMPEnableCmd.Flag("start-time", msgPrinter.Sprintf("The start time of the enabled node management policy. Start time should be RFC3339 timestamp or \"now\"")).Short('s').String()
	exNMPEnableStartWindow := exNMPEnableCmd.Flag("start-window", msgPrinter.Sprintf("The start window of the enabled node management policy.")).Short('w').String()
	exNMPDisableCmd := exNMPCmd.Command("disable", msgPrinter.Sprintf("Disable a node management policy in the Horizon Exchange."))
	exNMPDisableName := exNMPDisableCmd.Arg("nmp-name", msgPrinter.Sprintf("The name of the node management policy to disable.")).String()

	exNodeCmd := exchangeCmd.Command("node", msgPrinter.Sprintf("List and manage nodes in the Horizon Exchange"))
	exNodeAddPolicyCmd := exNodeCmd.Command("addpolicy | addp", msgPrinter.Sprintf("Add or replace the node policy in the Horizon Exchange.")).Alias("addp").Alias("addpolicy")
	exNodeAddPolicyIdTok := exNodeAddPolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeAddPolicyNode := exNodeAddPolicyCmd.Arg("node", msgPrinter.Sprintf("Add or replace policy for this node.")).Required().String()
	exNodeAddPolicyJsonFile := exNodeAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the node policy in the Horizon exchange. Specify -f- to read from stdin. A node policy contains the 'deployment' and 'management' attributes. Please use 'hzn policy new' to see the node policy format.")).Short('f').Required().String()
	exNodeCreateCmd := exNodeCmd.Command("create | cr", msgPrinter.Sprintf("Create the node resource in the Horizon Exchange.")).Alias("cr").Alias("create")
	exNodeCreateNodeIdTok := exNodeCreateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be created. The node ID must be unique within the organization.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeCreateNodeArch := exNodeCreateCmd.Flag("arch", msgPrinter.Sprintf("Your node architecture. If not specified, architecture will be left blank.")).Short('a').String()
	exNodeCreateNodeName := exNodeCreateCmd.Flag("name", msgPrinter.Sprintf("The name of your node")).Short('m').String()
	exNodeCreateNodeType := exNodeCreateCmd.Flag("node-type", msgPrinter.Sprintf("The type of your node. The valid values are: device, cluster. If omitted, the default is device. However, the node type stays unchanged if the node already exists, only the node token will be updated.")).Short('T').Default("device").String()
	exNodeCreateNode := exNodeCreateCmd.Arg("node", msgPrinter.Sprintf("The node to be created.")).String()
	exNodeCreateToken := exNodeCreateCmd.Arg("token", msgPrinter.Sprintf("The token the new node should have.")).String()
	exNodeConfirmCmd := exNodeCmd.Command("confirm | con", msgPrinter.Sprintf("Check to see if the specified node and token are valid in the Horizon Exchange.")).Alias("con").Alias("confirm")
	exNodeConfirmNodeIdTok := exNodeConfirmCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token to be checked. If not specified, HZN_EXCHANGE_NODE_AUTH will be used as a default. Mutually exclusive with <node> and <token> arguments.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeConfirmNode := exNodeConfirmCmd.Arg("node", msgPrinter.Sprintf("The node id to be checked. Mutually exclusive with -n flag.")).String()
	exNodeConfirmToken := exNodeConfirmCmd.Arg("token", msgPrinter.Sprintf("The token for the node. Mutually exclusive with -n flag.")).String()
	exNodeListCmd := exNodeCmd.Command("list | ls", msgPrinter.Sprintf("Display the node resources from the Horizon Exchange.")).Alias("ls").Alias("list")
	exNode := exNodeListCmd.Arg("node", msgPrinter.Sprintf("List just this one node.")).String()
	exNodeListNodeIdTok := exNodeListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeLong := exNodeListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the nodes, show the entire resource of each node, instead of just the name.")).Short('l').Bool()
	exNodeErrorsList := exNodeCmd.Command("listerrors | lse", msgPrinter.Sprintf("List the node errors currently surfaced to the Exchange.")).Alias("lse").Alias("listerrors")
	exNodeErrorsListIdTok := exNodeErrorsList.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeErrorsListNode := exNodeErrorsList.Arg("node", msgPrinter.Sprintf("List surfaced errors for this node.")).Required().String()
	exNodeErrorsListLong := exNodeErrorsList.Flag("long", msgPrinter.Sprintf("Show the full eventlog object of the errors currently surfaced to the Exchange.")).Short('l').Bool()
	exNodeListPolicyCmd := exNodeCmd.Command("listpolicy | lsp", msgPrinter.Sprintf("Display the node policy from the Horizon Exchange.")).Alias("lsp").Alias("listpolicy")
	exNodeListPolicyIdTok := exNodeListPolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeListPolicyNode := exNodeListPolicyCmd.Arg("node", msgPrinter.Sprintf("List policy for this node.")).Required().String()

	exNodeManagementCmd := exNodeCmd.Command("management | mgmt", msgPrinter.Sprintf("List and manage node management resources in the Horizon Exchange")).Alias("mgmt").Alias("management")
	exNodeManagementListCmd := exNodeManagementCmd.Command("list | ls", msgPrinter.Sprintf("List the compatible node management policies for the node. Only policies that are enabled will be displayed unless the -a flag is specified.")).Alias("ls").Alias("list")
	exNodeManagementListName := exNodeManagementListCmd.Arg("node", msgPrinter.Sprintf("List node management policies for this node")).Required().String()
	exNodeManagementListNodeIdTok := exNodeManagementListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modfy the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeManagementListAll := exNodeManagementListCmd.Flag("all", msgPrinter.Sprintf("Include disabled NMP's.")).Short('a').Bool()
	exNodeManagementStatusCmd := exNodeManagementCmd.Command("status", msgPrinter.Sprintf("List the node management policy statuses for this node."))
	exNodeManagementStatusName := exNodeManagementStatusCmd.Arg("node", msgPrinter.Sprintf("List node management policy statuses for this node.")).Required().String()
	exNodeManagementStatusNodeIdTok := exNodeManagementStatusCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modfy the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeManagementStatusPol := exNodeManagementStatusCmd.Flag("policy", msgPrinter.Sprintf("Filter output to include just this one node managment policy. Use with --long flag to display entire content of a single node management policy status object.")).Short('p').String()
	exNodeManagementStatusLong := exNodeManagementStatusCmd.Flag("long", msgPrinter.Sprintf("Show the entire contents of each node management policy status object.")).Short('l').Bool()
	exNodeManagementResetCmd := exNodeManagementCmd.Command("reset", msgPrinter.Sprintf("Re-evaluate the node management policy (nmp) for this node. Run this command to retry a nmp when the upgrade failed and the problem is fixed. Do not run this command when the node is still in the middle of an upgrade."))
	exNodeManagementResetName := exNodeManagementResetCmd.Arg("node", msgPrinter.Sprintf("Re-evaluate node management policy for this node.")).Required().String()
	exNodeManagementResetNodeIdTok := exNodeManagementResetCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modfy the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeManagementResetPol := exNodeManagementResetCmd.Flag("policy", msgPrinter.Sprintf("The name of the node managment policy to be re-evaluated. If omitted, all of the node management policies will be re-evaluated for this node.")).Short('p').String()
	exNodeStatusList := exNodeCmd.Command("liststatus | lst", msgPrinter.Sprintf("List the run-time status of the node.")).Alias("lst").Alias("liststatus")
	exNodeStatusIdTok := exNodeStatusList.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeStatusListNode := exNodeStatusList.Arg("node", msgPrinter.Sprintf("List status for this node")).Required().String()
	exNodeDelCmd := exNodeCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a node resource from the Horizon Exchange. Do NOT do this when an edge node is registered with this node id.")).Alias("rm").Alias("remove")
	exNodeRemoveNodeIdTok := exNodeDelCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modfy the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exDelNode := exNodeDelCmd.Arg("node", msgPrinter.Sprintf("The node to remove.")).Required().String()
	exNodeDelForce := exNodeDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exNodeRemovePolicyCmd := exNodeCmd.Command("removepolicy | rmp", msgPrinter.Sprintf("Remove the node policy in the Horizon Exchange.")).Alias("rmp").Alias("removepolicy")
	exNodeRemovePolicyIdTok := exNodeRemovePolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeRemovePolicyNode := exNodeRemovePolicyCmd.Arg("node", msgPrinter.Sprintf("Remove policy for this node.")).Required().String()
	exNodeRemovePolicyForce := exNodeRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exNodeSetTokCmd := exNodeCmd.Command("settoken", msgPrinter.Sprintf("Change the token of a node resource in the Horizon Exchange."))
	exNodeSetTokNode := exNodeSetTokCmd.Arg("node", msgPrinter.Sprintf("The node to be changed.")).Required().String()
	exNodeSetTokToken := exNodeSetTokCmd.Arg("token", msgPrinter.Sprintf("The new token for the node.")).Required().String()
	exNodeSetTokNodeIdTok := exNodeSetTokCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeUpdateCmd := exNodeCmd.Command("update | up", msgPrinter.Sprintf("Update an attribute of the node in the Horizon Exchange.")).Alias("up").Alias("update")
	exNodeUpdateNode := exNodeUpdateCmd.Arg("node", msgPrinter.Sprintf("The node to be updated.")).Required().String()
	exNodeUpdateIdTok := exNodeUpdateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeUpdateJsonFile := exNodeUpdateCmd.Flag("json-file", msgPrinter.Sprintf("The path to a json file containing the changed attribute to be updated in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exNodeUpdatePolicyCmd := exNodeCmd.Command("updatepolicy | upp", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn exchange node addpolicy' to update the node policy. This command is used to update either the node policy properties or the constraints, but not both.")).Alias("upp").Alias("updatepolicy")
	exNodeUpdatePolicyNode := exNodeUpdatePolicyCmd.Arg("node", msgPrinter.Sprintf("Update the policy for this node.")).Required().String()
	exNodeUpdatePolicyIdTok := exNodeUpdatePolicyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exNodeUpdatePolicyJsonFile := exNodeUpdatePolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the new constraints or properties (not both) for the node policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()

	exOrgCmd := exchangeCmd.Command("org", msgPrinter.Sprintf("List and manage organizations in the Horizon Exchange."))
	exOrgCreateCmd := exOrgCmd.Command("create | cr", msgPrinter.Sprintf("Create the organization resource in the Horizon Exchange.")).Alias("cr").Alias("create")
	exOrgCreateOrg := exOrgCreateCmd.Arg("org", msgPrinter.Sprintf("Create this organization and assign it to an agbot.")).Required().String()
	exOrgCreateLabel := exOrgCreateCmd.Flag("label", msgPrinter.Sprintf("Label for new organization.")).Short('l').String()
	exOrgCreateDesc := exOrgCreateCmd.Flag("description", msgPrinter.Sprintf("Description for new organization.")).Short('d').Required().String()
	exOrgCreateTags := exOrgCreateCmd.Flag("tag", msgPrinter.Sprintf("Tag for new organization. The format is mytag1=myvalue1. This flag can be repeated to specify multiple tags.")).Short('t').Strings()
	exOrgCreateHBMin := exOrgCreateCmd.Flag("heartbeatmin", msgPrinter.Sprintf("The minimum number of seconds between agent heartbeats to the Exchange.")).Int()
	exOrgCreateHBMax := exOrgCreateCmd.Flag("heartbeatmax", msgPrinter.Sprintf("The maximum number of seconds between agent heartbeats to the Exchange. During periods of inactivity, the agent will increase the interval between heartbeats by increments of --heartbeatadjust.")).Int()
	exOrgCreateHBAdjust := exOrgCreateCmd.Flag("heartbeatadjust", msgPrinter.Sprintf("The number of seconds to increment the agent's heartbeat interval.")).Int()
	exOrgCreateMaxNodes := exOrgCreateCmd.Flag("max-nodes", msgPrinter.Sprintf("The maximum number of nodes this organization is allowed to have. The value cannot exceed the Exchange global limit. The default is 0 which means no organization limit.")).Int()
	exOrgCreateAddToAgbot := exOrgCreateCmd.Flag("agbot", msgPrinter.Sprintf("Add the organization to this agbot so that it will be responsible for deploying services in this org. The agbot will deploy services to nodes in this org, using the patterns and deployment policies in this org. If omitted, the first agbot found in the exchange will become responsible for this org. The format is 'agbot_org/agbot_id'.")).Short('a').String()
	exOrgListCmd := exOrgCmd.Command("list | ls", msgPrinter.Sprintf("Display the organization resource from the Horizon Exchange. (Normally you can only display your own organiztion. If the org does not exist, you will get an invalid credentials error.)")).Alias("ls").Alias("list")
	exOrgListOrg := exOrgListCmd.Arg("org", msgPrinter.Sprintf("List this one organization.")).String()
	exOrgListLong := exOrgListCmd.Flag("long", msgPrinter.Sprintf("Display detailed info of orgs")).Short('l').Bool()
	exOrgDelCmd := exOrgCmd.Command("remove | rm", msgPrinter.Sprintf("Remove an organization resource from the Horizon Exchange.")).Alias("rm").Alias("remove")
	exOrgDelOrg := exOrgDelCmd.Arg("org", msgPrinter.Sprintf("Remove this organization.")).Required().String()
	exOrgDelFromAgbot := exOrgDelCmd.Flag("agbot", msgPrinter.Sprintf("The agbot to remove the deployment policy from. If omitted, the first agbot found in the exchange will be used. The format is 'agbot_org/agbot_id'.")).Short('a').String()
	exOrgDelForce := exOrgDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exOrgUpdateCmd := exOrgCmd.Command("update | up", msgPrinter.Sprintf("Update the organization resource in the Horizon Exchange.")).Alias("up").Alias("update")
	exOrgUpdateOrg := exOrgUpdateCmd.Arg("org", msgPrinter.Sprintf("Update this organization.")).Required().String()
	exOrgUpdateLabel := exOrgUpdateCmd.Flag("label", msgPrinter.Sprintf("New label for organization.")).Short('l').String()
	exOrgUpdateDesc := exOrgUpdateCmd.Flag("description", msgPrinter.Sprintf("New description for organization.")).Short('d').String()
	exOrgUpdateTags := exOrgUpdateCmd.Flag("tag", msgPrinter.Sprintf("New tag for organization. The format is mytag1=myvalue1. This flag can be repeated to specify multiple tags. Use '-t \"\"' once to remove all the tags.")).Short('t').Strings()
	exOrgUpdateHBMin := exOrgUpdateCmd.Flag("heartbeatmin", msgPrinter.Sprintf("New minimum number of seconds the between agent heartbeats to the Exchange. The default negative integer -1 means no change to this attribute.")).Default("-1").Int()
	exOrgUpdateHBMax := exOrgUpdateCmd.Flag("heartbeatmax", msgPrinter.Sprintf("New maximum number of seconds between agent heartbeats to the Exchange. The default negative integer -1 means no change to this attribute.")).Default("-1").Int()
	exOrgUpdateHBAdjust := exOrgUpdateCmd.Flag("heartbeatadjust", msgPrinter.Sprintf("New value for the number of seconds to increment the agent's heartbeat interval. The default negative integer -1 means no change to this attribute.")).Default("-1").Int()
	exOrgUpdateMaxNodes := exOrgUpdateCmd.Flag("max-nodes", msgPrinter.Sprintf("The new maximum number of nodes this organization is allowed to have. The value cannot exceed the Exchange global limit. The default negative integer -1 means no change.")).Default("-1").Int()

	exPatternCmd := exchangeCmd.Command("pattern | pat", msgPrinter.Sprintf("List and manage patterns in the Horizon Exchange")).Alias("pat").Alias("pattern")
	exPatternListCmd := exPatternCmd.Command("list | ls", msgPrinter.Sprintf("Display the pattern resources from the Horizon Exchange.")).Alias("ls").Alias("list")
	exPatternListNodeIdTok := exPatternListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPattern := exPatternListCmd.Arg("pattern", msgPrinter.Sprintf("List just this one pattern. Use <org>/<pat> to specify a public pattern in another org, or <org>/ to list all of the public patterns in another org.")).String()
	exPatternLong := exPatternListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the patterns, show the entire resource of each pattern, instead of just the name.")).Short('l').Bool()
	exPatternListKeyCmd := exPatternCmd.Command("listkey | lsk", msgPrinter.Sprintf("List the signing public keys/certs for this pattern resource in the Horizon Exchange.")).Alias("lsk").Alias("listkey")
	exPatternListKeyNodeIdTok := exPatternListKeyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatListKeyPat := exPatternListKeyCmd.Arg("pattern", msgPrinter.Sprintf("The existing pattern to list the keys for.")).Required().String()
	exPatListKeyKey := exPatternListKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to see the contents of.")).String()
	exPatternPublishCmd := exPatternCmd.Command("publish | pub", msgPrinter.Sprintf("Sign and create/update the pattern resource in the Horizon Exchange.")).Alias("pub").Alias("publish")
	exPatJsonFile := exPatternPublishCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the pattern in the Horizon exchange. See %v/pattern.json. Specify -f- to read from stdin.", sample_dir)).Short('f').Required().String()
	exPatKeyFile := exPatternPublishCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the pattern. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If HZN_PRIVATE_KEY_FILE not specified, ~/.hzn/keys/service.private.key will be used. If none are specified, a random key pair will be generated and the public key will be stored with the pattern.")).Short('k').ExistingFile()
	exPatPubPubKeyFile := exPatternPublishCmd.Flag("public-key-file", msgPrinter.Sprintf("(DEPRECATED) The path of public key file (that corresponds to the private key) that should be stored with the pattern, to be used by the Horizon Agent to verify the signature. If this flag is not specified, the public key will be calculated from the private key.")).Short('K').ExistingFile()
	exPatName := exPatternPublishCmd.Flag("pattern-name", msgPrinter.Sprintf("The name to use for this pattern in the Horizon exchange. If not specified, will default to the base name of the file path specified in -f.")).Short('p').String()
	exPatDelCmd := exPatternCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a pattern resource from the Horizon Exchange.")).Alias("rm").Alias("remove")
	exDelPat := exPatDelCmd.Arg("pattern", msgPrinter.Sprintf("The pattern to remove.")).Required().String()
	exPatDelForce := exPatDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exPatternRemKeyCmd := exPatternCmd.Command("removekey | rmk", msgPrinter.Sprintf("Remove a signing public key/cert for this pattern resource in the Horizon Exchange.")).Alias("rmk").Alias("removekey")
	exPatRemKeyPat := exPatternRemKeyCmd.Arg("pattern", msgPrinter.Sprintf("The existing pattern to remove the key from.")).Required().String()
	exPatRemKeyKey := exPatternRemKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to remove.")).Required().String()
	exPatUpdateCmd := exPatternCmd.Command("update | up", msgPrinter.Sprintf("Update an attribute of the pattern in the Horizon Exchange.")).Alias("up").Alias("update")
	exPatUpdateNodeIdTok := exPatUpdateCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatUpdatePattern := exPatUpdateCmd.Arg("pattern", msgPrinter.Sprintf("The name of the pattern in the Horizon Exchange to publish.")).Required().String()
	exPatUpdateJsonFile := exPatUpdateCmd.Flag("json-file", msgPrinter.Sprintf("The path to a json file containing the updated attribute of the pattern to be put in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exPatternVerifyCmd := exPatternCmd.Command("verify | vf", msgPrinter.Sprintf("Verify the signatures of a pattern resource in the Horizon Exchange.")).Alias("vf").Alias("verify")
	exVerPattern := exPatternVerifyCmd.Arg("pattern", msgPrinter.Sprintf("The pattern to verify.")).Required().String()
	exPatternVerifyNodeIdTok := exPatternVerifyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exPatPubKeyFile := exPatternVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be used to verify the pattern. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()

	exServiceCmd := exchangeCmd.Command("service | serv", msgPrinter.Sprintf("List and manage services in the Horizon Exchange")).Alias("serv").Alias("service")
	exServiceAddPolicyCmd := exServiceCmd.Command("addpolicy | addp", msgPrinter.Sprintf("Add or replace the service policy in the Horizon Exchange.")).Alias("addp").Alias("addpolicy")
	exServiceAddPolicyIdTok := exServiceAddPolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange ID and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceAddPolicyService := exServiceAddPolicyCmd.Arg("service", msgPrinter.Sprintf("Add or replace policy for this service.")).Required().String()
	exServiceAddPolicyJsonFile := exServiceAddPolicyCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service policy in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exServiceListCmd := exServiceCmd.Command("list | ls", msgPrinter.Sprintf("Display the service resources from the Horizon Exchange.")).Alias("ls").Alias("list")
	exService := exServiceListCmd.Arg("service", msgPrinter.Sprintf("List just this one service. Use <org>/<svc> to specify a public service in another org, or <org>/ to list all of the public services in another org.")).String()
	exServiceListNodeIdTok := exServiceListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceLong := exServiceListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the services, show the entire service definition, instead of just the name. When listing a specific service, show more details.")).Short('l').Bool()
	exSvcOpYamlFilePath := exServiceListCmd.Flag("op-yaml-file", msgPrinter.Sprintf("The name of the file where the cluster deployment operator yaml archive will be saved. This flag is only used when listing a specific service. This flag is ignored when the service does not have a clusterDeployment attribute.")).Short('f').String()
	exSvcOpYamlForce := exServiceListCmd.Flag("force", msgPrinter.Sprintf("Skip the 'do you want to overwrite?' prompt when -f is specified and the file exists.")).Short('F').Bool()
	exServiceListAuthCmd := exServiceCmd.Command("listauth | lsau", msgPrinter.Sprintf("List the docker auth tokens for this service resource in the Horizon Exchange.")).Alias("lsau").Alias("listauth")
	exSvcListAuthSvc := exServiceListAuthCmd.Arg("service", msgPrinter.Sprintf("The existing service to list the docker auths for.")).Required().String()
	exSvcListAuthId := exServiceListAuthCmd.Arg("auth-name", msgPrinter.Sprintf("The existing docker auth id to see the contents of.")).Uint()
	exServiceListAuthNodeIdTok := exServiceListAuthCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceListKeyCmd := exServiceCmd.Command("listkey | lsk", msgPrinter.Sprintf("List the signing public keys/certs for this service resource in the Horizon Exchange.")).Alias("lsk").Alias("listkey")
	exSvcListKeySvc := exServiceListKeyCmd.Arg("service", msgPrinter.Sprintf("The existing service to list the keys for.")).Required().String()
	exSvcListKeyKey := exServiceListKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to see the contents of.")).String()
	exServiceListKeyNodeIdTok := exServiceListKeyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceListnode := exServiceCmd.Command("listnode | lsn", msgPrinter.Sprintf("Display the nodes that the service is running on.")).Alias("lsn").Alias("listnode")
	exServiceListnodeService := exServiceListnode.Arg("service", msgPrinter.Sprintf("The service id. Use <org>/<svc> to specify a service from a different org.")).Required().String()
	exServiceListnodeNodeOrg := exServiceListnode.Flag("node-org", msgPrinter.Sprintf("The node's organization. If omitted, it will be same as the org specified by -o or HZN_ORG_ID.")).Short('O').String()
	exServiceListPolicyCmd := exServiceCmd.Command("listpolicy | lsp", msgPrinter.Sprintf("Display the service policy from the Horizon Exchange.")).Alias("lsp").Alias("listpolicy")
	exServiceListPolicyIdTok := exServiceListPolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange id and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceListPolicyService := exServiceListPolicyCmd.Arg("service", msgPrinter.Sprintf("List policy for this service.")).Required().String()
	exServiceNewPolicyCmd := exServiceCmd.Command("newpolicy | newp", msgPrinter.Sprintf("Display an empty service policy template that can be filled in.")).Alias("newp").Alias("newpolicy")
	exServicePublishCmd := exServiceCmd.Command("publish | pub", msgPrinter.Sprintf("Sign and create/update the service resource in the Horizon Exchange.")).Alias("pub").Alias("publish")
	exSvcJsonFile := exServicePublishCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the service in the Horizon exchange. See %v/service.json and %v/service_cluster.json. Specify -f- to read from stdin.", sample_dir, sample_dir)).Short('f').Required().String()
	exSvcPrivKeyFile := exServicePublishCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the service. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If HZN_PRIVATE_KEY_FILE not specified, ~/.hzn/keys/service.private.key will be used. If none are specified, a random key pair will be generated and the public key will be stored with the service.")).Short('k').ExistingFile()
	exSvcPubPubKeyFile := exServicePublishCmd.Flag("public-key-file", msgPrinter.Sprintf("(DEPRECATED) The path of public key file (that corresponds to the private key) that should be stored with the service, to be used by the Horizon Agent to verify the signature. If this flag is not specified, the public key will be calculated from the private key.")).Short('K').ExistingFile()
	exSvcPubDontTouchImage := exServicePublishCmd.Flag("dont-change-image-tag", msgPrinter.Sprintf("The image paths in the deployment field have regular tags and should not be changed to sha256 digest values. The image will not get automatically uploaded to the repository. This should only be used during development when testing new versions often.")).Short('I').Bool()
	exSvcPubPullImage := exServicePublishCmd.Flag("pull-image", msgPrinter.Sprintf("Use the image from the image repository. It will pull the image from the image repository and overwrite the local image if exists. This flag is mutually exclusive with -I.")).Short('P').Bool()
	exSvcRegistryTokens := exServicePublishCmd.Flag("registry-token", msgPrinter.Sprintf("Docker registry domain and auth that should be stored with the service, to enable the Horizon edge node to access the service's docker images. This flag can be repeated, and each flag should be in the format: registry:user:token")).Short('r').Strings()
	exSvcOverwrite := exServicePublishCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing version if the service exists in the Exchange. It will skip the 'do you want to overwrite' prompt.")).Short('O').Bool()
	exSvcPolicyFile := exServicePublishCmd.Flag("service-policy-file", msgPrinter.Sprintf("The path of the service policy JSON file to be used for the service to be published. This flag is optional")).Short('p').String()
	exSvcPublic := exServicePublishCmd.Flag("public", msgPrinter.Sprintf("Whether the service is visible to users outside of the organization. This flag is optional. If left unset, the service will default to whatever the metadata has set. If the service definition has also not set the public field, then the service will by default not be public.")).String()
	exSvcDelCmd := exServiceCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a service resource from the Horizon Exchange.")).Alias("rm").Alias("remove")
	exDelSvc := exSvcDelCmd.Arg("service", msgPrinter.Sprintf("The service to remove.")).Required().String()
	exSvcDelForce := exSvcDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exServiceRemAuthCmd := exServiceCmd.Command("removeauth | rmau", msgPrinter.Sprintf("Remove a docker auth token for this service resource in the Horizon Exchange.")).Alias("rmau").Alias("removeauth")
	exSvcRemAuthSvc := exServiceRemAuthCmd.Arg("service", msgPrinter.Sprintf("The existing service to remove the docker auth from.")).Required().String()
	exSvcRemAuthId := exServiceRemAuthCmd.Arg("auth-name", msgPrinter.Sprintf("The existing docker auth id to remove.")).Required().Uint()
	exServiceRemKeyCmd := exServiceCmd.Command("removekey | rmk", msgPrinter.Sprintf("Remove a signing public key/cert for this service resource in the Horizon Exchange.")).Alias("rmk").Alias("removekey")
	exSvcRemKeySvc := exServiceRemKeyCmd.Arg("service", msgPrinter.Sprintf("The existing service to remove the key from.")).Required().String()
	exSvcRemKeyKey := exServiceRemKeyCmd.Arg("key-name", msgPrinter.Sprintf("The existing key name to remove.")).Required().String()
	exServiceRemovePolicyCmd := exServiceCmd.Command("removepolicy | rmp", msgPrinter.Sprintf("Remove the service policy in the Horizon Exchange.")).Alias("rmp").Alias("removepolicy")
	exServiceRemovePolicyIdTok := exServiceRemovePolicyCmd.Flag("service-id-tok", msgPrinter.Sprintf("The Horizon Exchange ID and password of the user")).Short('n').PlaceHolder("ID:TOK").String()
	exServiceRemovePolicyService := exServiceRemovePolicyCmd.Arg("service", msgPrinter.Sprintf("Remove policy for this service.")).Required().String()
	exServiceRemovePolicyForce := exServiceRemovePolicyCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exServiceVerifyCmd := exServiceCmd.Command("verify | vf", msgPrinter.Sprintf("Verify the signatures of a service resource in the Horizon Exchange.")).Alias("vf").Alias("verify")
	exVerService := exServiceVerifyCmd.Arg("service", msgPrinter.Sprintf("The service to verify.")).Required().String()
	exServiceVerifyNodeIdTok := exServiceVerifyCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exSvcPubKeyFile := exServiceVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be used to verify the service. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()

	exHAGroupCmd := exchangeCmd.Command("hagroup | hagr", msgPrinter.Sprintf("List and manage high availability (HA) groups in the Horizon Exchange")).Alias("hagroup").Alias("hagr")
	exHAGroupListCmd := exHAGroupCmd.Command("list | ls", msgPrinter.Sprintf("Display the HA group resources from the Horizon Exchange.")).Alias("ls").Alias("list")
	exHAGroupListName := exHAGroupListCmd.Arg("group-name", msgPrinter.Sprintf("List just this one HA group.")).String()
	exHAGroupListNodeIdTok := exHAGroupListCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon Exchange node ID and token to be used as credentials to query and modify the node resources if -u flag is not specified. HZN_EXCHANGE_NODE_AUTH will be used as a default for -n. If you don't prepend it with the node's org, it will automatically be prepended with the -o value.")).Short('n').PlaceHolder("ID:TOK").String()
	exHAGroupListLong := exHAGroupListCmd.Flag("long", msgPrinter.Sprintf("When listing all of the HA groups, show the entire resource of each group, instead of just the name.")).Short('l').Bool()
	exHAGroupNewCmd := exHAGroupCmd.Command("new", msgPrinter.Sprintf("Display an empty HA group template that can be filled in."))
	exHAGroupAddCmd := exHAGroupCmd.Command("add", msgPrinter.Sprintf("Add or replace an HA group in the Horizon Exchange. Use 'hzn exchange hagroup new' for an empty HA group template."))
	exHAGroupAddName := exHAGroupAddCmd.Arg("group-name", msgPrinter.Sprintf("The name of the HA group to add or overwrite. If omitted, the name attribute in the input file will be used.")).String()
	exHAGroupAddJsonFile := exHAGroupAddCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the metadata necessary to create/update the HA group in the Horizon Exchange. Specify -f- to read from stdin.")).Short('f').Required().String()
	exHAGroupRemoveCmd := exHAGroupCmd.Command("remove | rm", msgPrinter.Sprintf("Remove the HA group in the Horizon Exchange.")).Alias("rm").Alias("remove")
	exHAGroupRemoveName := exHAGroupRemoveCmd.Arg("group-name", msgPrinter.Sprintf("The name of the HA group to be removed.")).Required().String()
	exHAGroupRemoveForce := exHAGroupRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exHAGroupMemberCmd := exHAGroupCmd.Command("member | mb", msgPrinter.Sprintf("Manage HA group members in the Horizon Exchange")).Alias("mb").Alias("member")
	exHAGroupMemberAddCmd := exHAGroupMemberCmd.Command("add", msgPrinter.Sprintf("Add nodes to the HA group in the Horizon Exchange."))
	exHAGroupMemberAddName := exHAGroupMemberAddCmd.Arg("group-name", msgPrinter.Sprintf("The name of the HA group.")).Required().String()
	exHAGroupMemberAddNodes := exHAGroupMemberAddCmd.Flag("node", msgPrinter.Sprintf("Node to be added to the HA group. This flag can be repeated to specify different nodes.")).Short('m').Required().Strings()
	exHAGroupMemberRemoveCmd := exHAGroupMemberCmd.Command("remove | rm", msgPrinter.Sprintf("Remove nodes from the HA group in the Horizon Exchange.")).Alias("rm").Alias("remove")
	exHAGroupMemberRemoveName := exHAGroupMemberRemoveCmd.Arg("group-name", msgPrinter.Sprintf("The name of the HA group.")).Required().String()
	exHAGroupMemberRemoveNodes := exHAGroupMemberRemoveCmd.Flag("node", msgPrinter.Sprintf("Node to be removed from the HA group. This flag can be repeated to specify different nodes.")).Short('m').Required().Strings()
	exHAGroupMemberRemoveForce := exHAGroupMemberRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()

	exStatusCmd := exchangeCmd.Command("status", msgPrinter.Sprintf("Display the status of the Horizon Exchange."))

	exUserCmd := exchangeCmd.Command("user", msgPrinter.Sprintf("List and manage users in the Horizon Exchange."))
	exUserCreateCmd := exUserCmd.Command("create | cr", msgPrinter.Sprintf("Create the user resource in the Horizon Exchange.")).Alias("cr").Alias("create")
	exUserCreateUser := exUserCreateCmd.Arg("user", msgPrinter.Sprintf("Your username for this user account when creating it in the Horizon exchange.")).Required().String()
	exUserCreatePw := exUserCreateCmd.Arg("pw", msgPrinter.Sprintf("Your password for this user account when creating it in the Horizon exchange.")).Required().String()
	exUserCreateEmail := exUserCreateCmd.Arg("email", msgPrinter.Sprintf("Your email address that should be associated with this user account when creating it in the Horizon exchange. This argument is optional")).String()
	exUserCreateIsAdmin := exUserCreateCmd.Flag("admin", msgPrinter.Sprintf("This user should be an administrator, capable of managing all resources in this org of the Exchange.")).Short('A').Bool()
	exUserCreateIsHubAdmin := exUserCreateCmd.Flag("hubadmin", msgPrinter.Sprintf("This user should be a hub administrator, capable of managing orgs in this administration hub.")).Short('H').Bool()
	exUserListCmd := exUserCmd.Command("list | ls", msgPrinter.Sprintf("Display the user resource from the Horizon Exchange. (Normally you can only display your own user. If the user does not exist, you will get an invalid credentials error.)")).Alias("ls").Alias("list")
	exUserListUser := exUserListCmd.Arg("user", msgPrinter.Sprintf("List this one user. Default is your own user. Only admin users can list other users.")).String()
	exUserListAll := exUserListCmd.Flag("all", msgPrinter.Sprintf("List all users in the org. Will only do this if you are a user with admin privilege.")).Short('a').Bool()
	exUserListNamesOnly := exUserListCmd.Flag("names", msgPrinter.Sprintf("When listing all of the users, show only the usernames, instead of each entire resource.")).Short('N').Bool()
	exUserDelCmd := exUserCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a user resource from the Horizon Exchange. Warning: this will cause all exchange resources owned by this user to also be deleted (nodes, services, patterns, etc).")).Alias("rm").Alias("remove")
	exDelUser := exUserDelCmd.Arg("user", msgPrinter.Sprintf("The user to remove.")).Required().String()
	exUserDelForce := exUserDelCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	exUserSetAdminCmd := exUserCmd.Command("setadmin | sa", msgPrinter.Sprintf("Change the existing user to be an admin user (like root in his/her org) or to no longer be an admin user. Can only be run by exchange root or another admin user.")).Alias("sa").Alias("setadmin")
	exUserSetAdminUser := exUserSetAdminCmd.Arg("user", msgPrinter.Sprintf("The user to be modified.")).Required().String()
	exUserSetAdminBool := exUserSetAdminCmd.Arg("isadmin", msgPrinter.Sprintf("True if they should be an admin user, otherwise false.")).Required().Bool()

	exVersionCmd := exchangeCmd.Command("version", msgPrinter.Sprintf("Display the version of the Horizon Exchange."))

	keyCmd := app.Command("key", msgPrinter.Sprintf("List and manage keys for signing and verifying services."))
	keyCreateCmd := keyCmd.Command("create | cr", msgPrinter.Sprintf("Generate a signing key pair.")).Alias("cr").Alias("create")
	keyX509Org := keyCreateCmd.Arg("x509-org", msgPrinter.Sprintf("x509 certificate Organization (O) field (preferably a company name or other organization's name).")).Required().String()
	keyX509CN := keyCreateCmd.Arg("x509-cn", msgPrinter.Sprintf("x509 certificate Common Name (CN) field (preferably an email address issued by x509org).")).Required().String()
	keyOutputDir := keyCreateCmd.Flag("output-dir", msgPrinter.Sprintf("The directory to put the key pair files in. Mutually exclusive with -k and -K. The file names will be randomly generated.")).Short('d').ExistingDir()
	keyCreatePrivKey := keyCreateCmd.Flag("private-key-file", msgPrinter.Sprintf("The full path of the private key file. Mutually exclusive with -d. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.private.key is the default.")).Short('k').String()
	keyCreatePubKey := keyCreateCmd.Flag("pubic-key-file", msgPrinter.Sprintf("The full path of the public key file. Mutually exclusive with -d. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('K').String()
	keyCreateOverwrite := keyCreateCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing files. It will skip the 'do you want to overwrite' prompt.")).Short('f').Bool()
	keyLength := keyCreateCmd.Flag("length", msgPrinter.Sprintf("The length of the key to create.")).Short('l').Default("4096").Int()
	keyDaysValid := keyCreateCmd.Flag("days-valid", msgPrinter.Sprintf("x509 certificate validity (Validity > Not After) expressed in days from the day of generation.")).Default("1461").Int()
	keyImportFlag := keyCreateCmd.Flag("import", msgPrinter.Sprintf("Automatically import the created public key into the local Horizon agent.")).Short('i').Bool()
	keyImportCmd := keyCmd.Command("import | imp", msgPrinter.Sprintf("Imports a signing public key into the Horizon agent.")).Alias("imp").Alias("import")
	keyImportPubKeyFile := keyImportCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of a pem public key file to be imported. The base name in the path is also used as the key name in the Horizon agent. If not specified, the environment variable HZN_PUBLIC_KEY_FILE will be used. If none of them are set, ~/.hzn/keys/service.public.pem is the default.")).Short('k').String()
	keyListCmd := keyCmd.Command("list | ls", msgPrinter.Sprintf("List the signing keys that have been imported into this Horizon agent.")).Alias("list").Alias("ls")
	keyName := keyListCmd.Arg("key-name", msgPrinter.Sprintf("The name of a specific key to show.")).String()
	keyListAll := keyListCmd.Flag("all", msgPrinter.Sprintf("List the names of all signing keys, even the older public keys not wrapped in a certificate.")).Short('a').Bool()
	keyDelCmd := keyCmd.Command("remove | rm", msgPrinter.Sprintf("Remove the specified signing key from this Horizon agent.")).Alias("remove").Alias("rm")
	keyDelName := keyDelCmd.Arg("key-name", msgPrinter.Sprintf("The name of a specific key to remove.")).Required().String()

	meteringCmd := app.Command("metering | mt", msgPrinter.Sprintf("List or manage the metering (payment) information for the active or archived agreements.")).Alias("mt").Alias("metering")
	meteringListCmd := meteringCmd.Command("list | ls", msgPrinter.Sprintf("List the metering (payment) information for the active or archived agreements.")).Alias("ls").Alias("list")
	listArchivedMetering := meteringListCmd.Flag("archived", msgPrinter.Sprintf("List archived agreement metering information instead of metering for the active agreements.")).Short('r').Bool()

	mmsCmd := app.Command("mms", msgPrinter.Sprintf("List and manage Horizon Model Management Service resources."))
	mmsOrg := mmsCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	mmsUserPw := mmsCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon user credentials to query and create Model Management Service resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()

	mmsObjectCmd := mmsCmd.Command("object | obj", msgPrinter.Sprintf("List and manage objects in the Horizon Model Management Service.")).Alias("obj").Alias("object")
	mmsObjectDeleteCmd := mmsObjectCmd.Command("delete | del", msgPrinter.Sprintf("Delete an object in the Horizon Model Management Service, making it unavailable for services deployed on nodes.")).Alias("delete").Alias("del")
	mmsObjectDeleteType := mmsObjectDeleteCmd.Flag("type", msgPrinter.Sprintf("The type of the object to delete.")).Short('t').Required().String()
	mmsObjectDeleteId := mmsObjectDeleteCmd.Flag("id", msgPrinter.Sprintf("The id of the object to delete.")).Short('i').Required().String()
	mmsObjectDownloadCmd := mmsObjectCmd.Command("download | dl", msgPrinter.Sprintf("Download data of the given object in the Horizon Model Management Service.")).Alias("dl").Alias("download")
	mmsObjectDownloadType := mmsObjectDownloadCmd.Flag("type", msgPrinter.Sprintf("The type of the object to download data. This flag must be used with -i.")).Short('t').Required().String()
	mmsObjectDownloadId := mmsObjectDownloadCmd.Flag("id", msgPrinter.Sprintf("The id of the object to download data. This flag must be used with -t.")).Short('i').Required().String()
	mmsObjectDownloadFile := mmsObjectDownloadCmd.Flag("file", msgPrinter.Sprintf("The file that the data of downloaded object is written to. This flag must be used with -f. If omit, will use default file name in format of objectType_objectID and save in current directory")).Short('f').String()
	mmsObjectDownloadOverwrite := mmsObjectDownloadCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing file if it exists in the file system.")).Short('O').Bool()
	mmsObjectDownloadSkipIntegrityCheck := mmsObjectDownloadCmd.Flag("noIntegrity", msgPrinter.Sprintf("The download command will not perform a data integrity check on the downloaded object data")).Bool()

	mmsObjectListCmd := mmsObjectCmd.Command("list | ls", msgPrinter.Sprintf("List objects in the Horizon Model Management Service.")).Alias("ls").Alias("list")
	mmsObjectListType := mmsObjectListCmd.Flag("type", msgPrinter.Sprintf("The type of the object to list.")).Short('t').String()
	mmsObjectListObjType := mmsObjectListCmd.Flag("objectType", "").Hidden().String()
	mmsObjectListId := mmsObjectListCmd.Flag("id", msgPrinter.Sprintf("The id of the object to list. This flag is optional. Omit this flag to list all objects of a given object type.")).Short('i').String()
	mmsObjectListObjId := mmsObjectListCmd.Flag("objectId", "").Hidden().String()
	mmsObjectListDestinationPolicy := mmsObjectListCmd.Flag("policy", msgPrinter.Sprintf("Specify true to show only objects using policy. Specify false to show only objects not using policy. If this flag is omitted, both kinds of objects are shown.")).Short('p').String()
	mmsObjectListDPService := mmsObjectListCmd.Flag("service", msgPrinter.Sprintf("List mms objects using policy that are targetted for the given service. Service specified in the format service-org/service-name.")).Short('s').String()
	mmsObjectListDPProperty := mmsObjectListCmd.Flag("property", msgPrinter.Sprintf("List mms objects using policy that reference the given property name.")).String()
	mmsObjectListDPUpdateTime := mmsObjectListCmd.Flag("updateTime", msgPrinter.Sprintf("List mms objects using policy that has been updated since the given time. The time value is spefified in RFC3339 format: yyyy-MM-ddTHH:mm:ssZ. The time of day may be omitted.")).String()
	mmsObjectListDestinationType := mmsObjectListCmd.Flag("destinationType", msgPrinter.Sprintf("List mms objects with given destination type")).String()
	mmsObjectListDestinationId := mmsObjectListCmd.Flag("destinationId", msgPrinter.Sprintf("List mms objects with given destination id. Must specify --destinationType to use this flag")).String()
	mmsObjectListWithData := mmsObjectListCmd.Flag("data", msgPrinter.Sprintf("Specify true to show objects that have data. Specify false to show objects that have no data. If this flag is omitted, both kinds of objects are shown.")).String()
	mmsObjectListExpirationTime := mmsObjectListCmd.Flag("expirationTime", msgPrinter.Sprintf("List mms objects that expired before the given time. The time value is spefified in RFC3339 format: yyyy-MM-ddTHH:mm:ssZ. Specify now to show objects that are currently expired.")).Short('e').String()
	mmsObjectListDeleted := mmsObjectListCmd.Flag("deleted", msgPrinter.Sprintf("Specify true to show objects that are marked deleted. Specify false to show objects that are not marked deleted. If this flag is omitted, both kinds of objects are shown. Object will be marked deleted if object is deleted in CSS but it doesn't receive notifications from all the destinations")).String()
	mmsObjectListLong := mmsObjectListCmd.Flag("long", msgPrinter.Sprintf("Show detailed object metadata information")).Short('l').Bool()
	mmsObjectListDetail := mmsObjectListCmd.Flag("detail", msgPrinter.Sprintf("Provides additional detail about the deployment of the object on edge nodes.")).Short('d').Bool()

	mmsObjectNewCmd := mmsObjectCmd.Command("new", msgPrinter.Sprintf("Display an empty object metadata template that can be filled in and passed as the -m option on the 'hzn mms object publish' command."))
	mmsObjectPublishCmd := mmsObjectCmd.Command("publish | pub", msgPrinter.Sprintf("Publish an object in the Horizon Model Management Service, making it available for services deployed on nodes.")).Alias("pub").Alias("publish")
	mmsObjectPublishType := mmsObjectPublishCmd.Flag("type", msgPrinter.Sprintf("The type of the object to publish. This flag must be used with -i. It is mutually exclusive with -m")).Short('t').String()
	mmsObjectPublishId := mmsObjectPublishCmd.Flag("id", msgPrinter.Sprintf("The id of the object to publish. This flag must be used with -t. It is mutually exclusive with -m")).Short('i').String()
	mmsObjectPublishPat := mmsObjectPublishCmd.Flag("pattern", msgPrinter.Sprintf("If you want the object to be deployed on nodes using a given pattern, specify it using this flag. This flag is optional and can only be used with --type and --id. It is mutually exclusive with -m")).Short('p').String()
	mmsObjectPublishDef := mmsObjectPublishCmd.Flag("def", msgPrinter.Sprintf("The definition of the object to publish. A blank template can be obtained from the 'hzn mms object new' command. Specify -m- to read from stdin.")).Short('m').String()
	mmsObjectPublishObj := mmsObjectPublishCmd.Flag("object", msgPrinter.Sprintf("The object (in the form of a file) to publish. This flag is optional so that you can update only the object's definition.")).Short('f').String()
	mmsObjectPublishNoChunkUpload := mmsObjectPublishCmd.Flag("disableChunkUpload", msgPrinter.Sprintf("The publish command will disable chunk upload. Data will stream to CSS.")).Bool()
	mmsObjectPublishChunkUploadDataSize := mmsObjectPublishCmd.Flag("chunkSize", msgPrinter.Sprintf("The size of data chunk that will be published with. Ignored if --disableChunkUpload is specified.")).Default("52428800").Int()
	mmsObjectPublishSkipIntegrityCheck := mmsObjectPublishCmd.Flag("noIntegrity", msgPrinter.Sprintf("The publish command will not perform a data integrity check on the uploaded object data. It is mutually exclusive with --hashAlgo and --hash")).Bool()
	mmsObjectPublishDSHashAlgo := mmsObjectPublishCmd.Flag("hashAlgo", msgPrinter.Sprintf("The hash algorithm used to hash the object data before signing it, ensuring data integrity during upload and download. Supported hash algorithms are SHA1 or SHA256, the default is SHA1. It is mutually exclusive with the --noIntegrity flag")).Short('a').String()
	mmsObjectPublishDSHash := mmsObjectPublishCmd.Flag("hash", msgPrinter.Sprintf("The hash of the object data being uploaded or downloaded. Use this flag if you want to provide the hash instead of allowing the command to automatically calculate the hash. The hash must be generated using either the SHA1 or SHA256 algorithm. The -a flag must be specified if the hash was generated using SHA256. This flag is mutually exclusive with --noIntegrity.")).String()
	mmsObjectPublishPrivKeyFile := mmsObjectPublishCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the object. The corresponding public key will be stored in the MMS to ensure integrity of the object. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used to find a private key. If not set, ~/.hzn/keys/service.private.key will be used. If it does not exist, an RSA key pair is generated only for this publish operation and then the private key is discarded.")).Short('k').ExistingFile()
	mmsObjectTypesCmd := mmsObjectCmd.Command("types", msgPrinter.Sprintf("Display a list of object types stored in the Horizon Model Management Service."))
	mmsStatusCmd := mmsCmd.Command("status", msgPrinter.Sprintf("Display the status of the Horizon Model Management Service."))

	nodeCmd := app.Command("node", msgPrinter.Sprintf("List and manage general information about this Horizon edge node."))
	nodeListCmd := nodeCmd.Command("list | ls", msgPrinter.Sprintf("Display general information about this Horizon edge node.")).Alias("list").Alias("ls")

	nodeManagementCmd := app.Command("nodemanagement | nm", msgPrinter.Sprintf("List and manage manifests and agent files for node management.")).Alias("nm").Alias("nodemanagement")
	nmOrg := nodeManagementCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	nmUserPw := nodeManagementCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon user credentials to query and create Node Management resources. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()

	nmManifestCmd := nodeManagementCmd.Command("manifest | man", msgPrinter.Sprintf("List and manage manifest files stored in the management hub.")).Alias("man").Alias("manifest")
	nmManifestAddCmd := nmManifestCmd.Command("add", msgPrinter.Sprintf("Add or replace a manifest file in the management hub. Use 'hzn nodemanagement manifest new' for an empty manifest template."))
	nmManifestAddType := nmManifestAddCmd.Flag("type", msgPrinter.Sprintf("The type of manifest to add. Valid values include 'agent_upgrade_manifests'.")).Required().Short('t').String()
	nmManifestAddId := nmManifestAddCmd.Flag("id", msgPrinter.Sprintf("The id of the manifest to add.")).Required().Short('i').String()
	nmManifestAddFile := nmManifestAddCmd.Flag("json-file", msgPrinter.Sprintf("The path of a JSON file containing the manifest data. Specify -f- to read from stdin.")).Short('f').Required().String()
	nmManifestAddDSHashAlgo := nmManifestAddCmd.Flag("hashAlgo", msgPrinter.Sprintf("The hash algorithm used to hash the manifest data before signing it, ensuring data integrity during upload and download. Supported hash algorithms are SHA1 or SHA256, the default is SHA1. It is mutually exclusive with the --noIntegrity flag")).Short('a').String()
	nmManifestAddDSHash := nmManifestAddCmd.Flag("hash", msgPrinter.Sprintf("The hash of the manifest data being uploaded or downloaded. Use this flag if you want to provide the hash instead of allowing the command to automatically calculate the hash. The hash must be generated using either the SHA1 or SHA256 algorithm. The -a flag must be specified if the hash was generated using SHA256. This flag is mutually exclusive with --noIntegrity.")).String()
	nmManifestAddPrivKeyFile := nmManifestAddCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the manifest. The corresponding public key will be stored in the MMS to ensure integrity of the manifest. If not specified, the environment variable HZN_PRIVATE_KEY_FILE will be used to find a private key. If not set, ~/.hzn/keys/service.private.key will be used. If it does not exist, an RSA key pair is generated only for this publish operation and then the private key is discarded.")).Short('k').ExistingFile()
	nmManifestAddSkipIntegrityCheck := nmManifestAddCmd.Flag("noIntegrity", msgPrinter.Sprintf("The publish command will not perform a data integrity check on the uploaded manifest data. It is mutually exclusive with --hashAlgo and --hash")).Bool()
	nmManifestListCmd := nmManifestCmd.Command("list | ls", msgPrinter.Sprintf("Display a list of manifest files stored in the management hub.")).Alias("ls").Alias("list")
	nmManifestListType := nmManifestListCmd.Flag("type", msgPrinter.Sprintf("The type of manifest to list. Valid values include 'agent_upgrade_manifests'.")).Short('t').String()
	nmManifestListId := nmManifestListCmd.Flag("id", msgPrinter.Sprintf("The id of the manifest to list. Must specify --type flag.")).Short('i').String()
	nmManifestListLong := nmManifestListCmd.Flag("long", msgPrinter.Sprintf("Display the contents of the manifest file. Must specify --type and --id flags.")).Short('l').Bool()
	nmManifestNewCmd := nmManifestCmd.Command("new", msgPrinter.Sprintf("Display an empty manifest template that can be filled in."))
	nmManifestRemoveCmd := nmManifestCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a manifest file from the management hub. Use 'hzn nodemanagement manifest new' for an empty manifest template.")).Alias("rm").Alias("remove")
	nmManifestRemoveType := nmManifestRemoveCmd.Flag("type", msgPrinter.Sprintf("The type of manifest to remove. Valid values include 'agent_upgrade_manifests'.")).Required().Short('t').String()
	nmManifestRemoveId := nmManifestRemoveCmd.Flag("id", msgPrinter.Sprintf("The id of the manifest to remove.")).Required().Short('i').String()
	nmManifestRemoveForce := nmManifestRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	nmAgentFilesCmd := nodeManagementCmd.Command("agentfiles | af", msgPrinter.Sprintf("List agent files and types stored in the management hub.")).Alias("af").Alias("agentfiles")
	nmAgentFilesListCmd := nmAgentFilesCmd.Command("list | ls", msgPrinter.Sprintf("Display a list of agent files stored in the management hub.")).Alias("ls").Alias("list")
	nmAgentFilesListType := nmAgentFilesListCmd.Flag("type", msgPrinter.Sprintf("Filter the list of agent upgrade files by the specified type. Valid values include 'agent_software_files', 'agent_cert_files' and 'agent_config_files'.")).Short('t').String()
	nmAgentFilesListVersion := nmAgentFilesListCmd.Flag("version", msgPrinter.Sprintf("Filter the list of agent upgrade files by the specified version range or version string. Version can be a version range, a single version string or 'latest'.")).Short('V').String()
	nmAgentFilesVersionsCmd := nmAgentFilesCmd.Command("versions | ver", msgPrinter.Sprintf("Display a list of agent file types with their corresponding versions.")).Alias("ver").Alias("versions")
	nmAgentFilesVersionsType := nmAgentFilesVersionsCmd.Flag("type", msgPrinter.Sprintf("The type of agent files to list versions for. Valid values include 'agent_software_files', 'agent_cert_files' and 'agent_config_files'.")).Short('t').String()
	nmAgentFilesVersionsVersionOnly := nmAgentFilesVersionsCmd.Flag("version-only", msgPrinter.Sprintf("Show only a list of versions for a given file type. Must also specify a file type with the --type flag.")).Short('V').Bool()

	policyCmd := app.Command("policy | pol", msgPrinter.Sprintf("List and manage policy for this Horizon edge node.")).Alias("pol").Alias("policy")
	policyListCmd := policyCmd.Command("list | ls", msgPrinter.Sprintf("Display this edge node's policy.")).Alias("ls").Alias("list")
	policyNewCmd := policyCmd.Command("new", msgPrinter.Sprintf("Display an empty policy template that can be filled in."))
	policyPatchCmd := policyCmd.Command("patch", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn policy update' to update the node policy. This command is used to update either the node policy properties or the constraints, but not both."))
	policyPatchInput := policyPatchCmd.Arg("patch", msgPrinter.Sprintf("The new constraints or properties in the format '%s' or '%s'.", "{\"constraints\":[<constraint list>]}", "{\"properties\":[<property list>]}")).Required().String()
	policyRemoveCmd := policyCmd.Command("remove | rm", msgPrinter.Sprintf("Remove the node's policy.")).Alias("rm").Alias("remove")
	policyRemoveForce := policyRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	policyUpdateCmd := policyCmd.Command("update | up", msgPrinter.Sprintf("Create or replace the node's policy. The node's built-in properties cannot be modified or deleted by this command, with the exception of openhorizon.allowPrivileged.")).Alias("up").Alias("update")
	policyUpdateInputFile := policyUpdateCmd.Flag("input-file", msgPrinter.Sprintf("The JSON input file name containing the node policy. Specify -f- to read from stdin. A node policy contains the 'deployment' and 'management' attributes. Please use 'hzn policy new' to see the node policy format.")).Short('f').Required().String()

	regInputCmd := app.Command("reginput", msgPrinter.Sprintf("Create an input file template for this pattern that can be used for the 'hzn register' command (once filled in). This examines the services that the specified pattern uses, and determines the node owner input that is required for them."))
	regInputNodeIdTok := regInputCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token (it must already exist).")).Short('n').PlaceHolder("ID:TOK").Required().String()
	regInputInputFile := regInputCmd.Flag("input-file", msgPrinter.Sprintf("The JSON input template file name that should be created. This file will contain placeholders for you to fill in user input values.")).Short('f').Required().String()
	regInputOrg := regInputCmd.Arg("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node will be registered in.")).Required().String()
	regInputPattern := regInputCmd.Arg("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format.")).Required().String()
	regInputArch := regInputCmd.Arg("arch", msgPrinter.Sprintf("The architecture to write the template file for. (Horizon ignores services in patterns whose architecture is different from the target system.) The architecture must be what is returned by 'hzn node list' on the target system.")).Default(cutil.ArchString()).String()

	registerCmd := app.Command("register | reg", msgPrinter.Sprintf("Register this edge node with Horizon.")).Alias("reg").Alias("register")
	nodeIdTok := registerCmd.Flag("node-id-tok", msgPrinter.Sprintf("The Horizon exchange node ID and token. The node ID must be unique within the organization. If not specified, HZN_EXCHANGE_NODE_AUTH will be used as a default. If both -n and HZN_EXCHANGE_NODE_AUTH are not specified, the node ID will be created by Horizon from the machine serial number or fully qualified hostname. If the token is not specified, Horizon will create a random token. If node resource in the Exchange identified by the ID and token does not yet exist, you must also specify the -u flag so it can be created.")).Short('n').PlaceHolder("ID:TOK").String()
	nodeName := registerCmd.Flag("name", msgPrinter.Sprintf("The name of the node. If not specified, it will be the same as the node id.")).Short('m').String()
	userPw := registerCmd.Flag("user-pw", msgPrinter.Sprintf("User credentials to create the node resource in the Horizon exchange if it does not already exist. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default.")).Short('u').PlaceHolder("USER:PW").String()
	inputFile := registerCmd.Flag("input-file", msgPrinter.Sprintf("A JSON file that sets or overrides user input variables needed by the services that will be deployed to this node. See %v/user_input.json. Specify -f- to read from stdin.", sample_dir)).Short('f').String() // not using ExistingFile() because it can be - for stdin
	nodeOrgFlag := registerCmd.Flag("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node should be registered in. The default is the HZN_ORG_ID environment variable. Mutually exclusive with <nodeorg> and <pattern> arguments.")).Short('o').String()
	patternFlag := registerCmd.Flag("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. Mutually exclusive with <nodeorg> and <pattern> arguments.")).Short('p').String()
	nodepolicyFlag := registerCmd.Flag("policy", msgPrinter.Sprintf("A JSON file that sets or overrides the node policy for this node. A node policy contains the 'deployment' and 'management' attributes. Please use 'hzn policy new' to see the node policy format.")).String()
	org := registerCmd.Arg("nodeorg", msgPrinter.Sprintf("The Horizon exchange organization ID that the node should be registered in. Mutually exclusive with -o and -p.")).String()
	pattern := registerCmd.Arg("pattern", msgPrinter.Sprintf("The Horizon exchange pattern that describes what workloads that should be deployed to this node. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. Mutually exclusive with -o and -p.")).String()
	haGroupName := registerCmd.Flag("ha-group", msgPrinter.Sprintf("The name of the HA group that this node will be added to.")).String()
	waitServiceFlag := registerCmd.Flag("service", msgPrinter.Sprintf("Wait for the named service to start executing on this node. When registering with a pattern, use '*' to watch all the services in the pattern. When registering with a policy, '*' is not a valid value for -s. This flag is not supported for edge cluster nodes.")).Short('s').String()
	waitServiceOrgFlag := registerCmd.Flag("serviceorg", msgPrinter.Sprintf("The org of the service to wait for on this node. If '-s *' is specified, then --serviceorg must be omitted.")).String()
	waitTimeoutFlag := registerCmd.Flag("timeout", msgPrinter.Sprintf("The number of seconds for the --service to start. The default is 60 seconds, beginning when registration is successful. Ignored if --service is not specified.")).Short('t').Default("60").Int()

	serviceCmd := app.Command("service | serv", msgPrinter.Sprintf("List or manage the services that are currently registered on this Horizon edge node.")).Alias("serv").Alias("service")
	serviceConfigStateCmd := serviceCmd.Command("configstate | cfg", msgPrinter.Sprintf("List or manage the configuration state for the services that are currently registered on this Horizon edge node.")).Alias("cfg").Alias("configstate")
	serviceConfigStateListCmd := serviceConfigStateCmd.Command("list | ls", msgPrinter.Sprintf("List the configuration state for the services that are currently registered on this Horizon edge node.")).Alias("ls").Alias("list")
	serviceConfigStateActiveCmd := serviceConfigStateCmd.Command("resume | r", msgPrinter.Sprintf("Change the configuration state to 'active' for a service.")).Alias("r").Alias("resume")
	resumeAllServices := serviceConfigStateActiveCmd.Flag("all", msgPrinter.Sprintf("Resume all registerd services.")).Short('a').Bool()
	resumeServiceOrg := serviceConfigStateActiveCmd.Arg("serviceorg", msgPrinter.Sprintf("The organization of the service that should be resumed.")).String()
	resumeServiceName := serviceConfigStateActiveCmd.Arg("service", msgPrinter.Sprintf("The name of the service that should be resumed. If omitted, all the services for the organization will be resumed.")).String()
	resumeServiceVersion := serviceConfigStateActiveCmd.Arg("version", msgPrinter.Sprintf("The version of the service that should be resumed. If omitted, all the versions for this service will be resumed.")).String()
	serviceConfigStateSuspendCmd := serviceConfigStateCmd.Command("suspend | s", msgPrinter.Sprintf("Change the configuration state to 'suspend' for a service. Parent and child dependencies of the suspended service will be stopped until the service is resumed.")).Alias("s").Alias("suspend")
	suspendAllServices := serviceConfigStateSuspendCmd.Flag("all", msgPrinter.Sprintf("Suspend all registerd services.")).Short('a').Bool()
	suspendServiceOrg := serviceConfigStateSuspendCmd.Arg("serviceorg", msgPrinter.Sprintf("The organization of the service that should be suspended.")).String()
	suspendServiceName := serviceConfigStateSuspendCmd.Arg("service", msgPrinter.Sprintf("The name of the service that should be suspended. If omitted, all the services for the organization will be suspended.")).String()
	suspendServiceVersion := serviceConfigStateSuspendCmd.Arg("version", msgPrinter.Sprintf("The version of the service that should be suspended. If omitted, all the versions for this service will be suspended.")).String()
	forceSuspendService := serviceConfigStateSuspendCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	serviceLogCmd := serviceCmd.Command("log", msgPrinter.Sprintf("Show the container logs for a service."))
	logServiceName := serviceLogCmd.Arg("service", msgPrinter.Sprintf("The name of the service whose log records should be displayed. The service name is the same as the url field of a service definition. Displays log records similar to tail behavior and returns .")).Required().String()
	logServiceVersion := serviceLogCmd.Flag("version", msgPrinter.Sprintf("The version of the service.")).Short('V').String()
	logServiceContainerName := serviceLogCmd.Flag("container", msgPrinter.Sprintf("The name of the container within the service whose log records should be displayed.")).Short('c').String()
	logTail := serviceLogCmd.Flag("tail", msgPrinter.Sprintf("Continuously polls the service's logs to display the most recent records, similar to tail -F behavior.")).Short('f').Bool()
	serviceListCmd := serviceCmd.Command("list | ls", msgPrinter.Sprintf("List the services variable configuration that has been done on this Horizon edge node.")).Alias("ls").Alias("list")
	serviceRegisteredCmd := serviceCmd.Command("registered | reg", msgPrinter.Sprintf("List the services that are currently registered on this Horizon edge node.")).Alias("reg").Alias("registered")

	statusCmd := app.Command("status", msgPrinter.Sprintf("Display the current horizon internal status for the node."))
	statusLong := statusCmd.Flag("long", msgPrinter.Sprintf("Show detailed status")).Short('l').Bool()

	unregisterCmd := app.Command("unregister | unreg", msgPrinter.Sprintf("Unregister and reset this Horizon edge node so that it is ready to be registered again. Warning: this will stop all the Horizon services running on this edge node, and restart the Horizon agent.")).Alias("unreg").Alias("unregister")
	forceUnregister := unregisterCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	removeNodeUnregister := unregisterCmd.Flag("remove", msgPrinter.Sprintf("Also remove this node resource from the Horizon exchange (because you no longer want to use this node with Horizon).")).Short('r').Bool()
	deepCleanUnregister := unregisterCmd.Flag("deep-clean", msgPrinter.Sprintf("Also remove all the previous registration information. Use it only after the 'hzn unregister' command failed. Please capture the logs by running 'hzn eventlog list -a -l' command before using this flag.")).Short('D').Bool()
	timeoutUnregister := unregisterCmd.Flag("timeout", msgPrinter.Sprintf("The number of minutes to wait for unregistration to complete. The default is zero which will wait forever.")).Short('t').Default("0").Int()
	containerUnregister := unregisterCmd.Flag("container", msgPrinter.Sprintf("Perform a deep clean on a node running in a container. This flag  must be used with -D and only if the agent was installed as anax-in-container.")).Short('C').Bool()

	userinputCmd := app.Command("userinput | u", msgPrinter.Sprintf("List or manage the service user inputs that are currently registered on this Horizon edge node.")).Alias("u").Alias("userinput")
	userinputAddCmd := userinputCmd.Command("add", msgPrinter.Sprintf("Add a new user input object or overwrite the current user input object for this Horizon edge node."))
	userinputAddFilePath := userinputAddCmd.Flag("file-path", msgPrinter.Sprintf("The file path to the json file with the user input object. Specify -f- to read from stdin.")).Short('f').Required().String()
	userinputListCmd := userinputCmd.Command("list | ls", msgPrinter.Sprintf("List the service user inputs currently registered on this Horizon edge node.")).Alias("ls").Alias("list")
	userinputNewCmd := userinputCmd.Command("new", msgPrinter.Sprintf("Display an empty userinput template."))
	userinputRemoveCmd := userinputCmd.Command("remove | rm", msgPrinter.Sprintf("Remove the user inputs that are currently registered on this Horizon edge node.")).Alias("rm").Alias("remove")
	userinputRemoveForce := userinputRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'Are you sure?' prompt.")).Short('f').Bool()
	userinputUpdateCmd := userinputCmd.Command("update | up", msgPrinter.Sprintf("Update an existing user input object for this Horizon edge node.")).Alias("up").Alias("update")
	userinputUpdateFilePath := userinputUpdateCmd.Flag("file-path", msgPrinter.Sprintf("The file path to the json file with the updated user input object. Specify -f- to read from stdin.")).Short('f').Required().String()

	nmstatusCmd := app.Command("nmstatus", msgPrinter.Sprintf("List and manage node management status for the local node."))
	nmstatusListCmd := nmstatusCmd.Command("list", msgPrinter.Sprintf("Display the node managment status for the local node."))
	nmstatusListName := nmstatusListCmd.Arg("name", msgPrinter.Sprintf("The name of the node management policy. If omitted the status of all management policies for this node will be displayed.")).String()
	nmstatusListLong := nmstatusListCmd.Flag("long", msgPrinter.Sprintf("Show the entire contents of each node management status object.")).Short('l').Bool()
	nmstatusResetCmd := nmstatusCmd.Command("reset", msgPrinter.Sprintf("Re-evaluate the node management policy (nmp). Run this command to retry a nmp when the upgrade failed and the problem is fixed. Do not run this command when the node is still in the middle of an upgrade."))
	nmstatusResetName := nmstatusResetCmd.Arg("name", msgPrinter.Sprintf("The name of the node management policy. If omitted all management policies for this node will be re-evaluated.")).String()

	utilCmd := app.Command("util", msgPrinter.Sprintf("Utility commands."))
	utilConfigConvCmd := utilCmd.Command("configconv | cfg", msgPrinter.Sprintf("Convert the configuration file from JSON format to a shell script.")).Alias("cfg").Alias("configconv")
	utilConfigConvFile := utilConfigConvCmd.Flag("config-file", msgPrinter.Sprintf("The path of a configuration file to be converted. ")).Short('f').Required().ExistingFile()
	utilSignCmd := utilCmd.Command("sign", msgPrinter.Sprintf("Sign the text in stdin. The signature is sent to stdout."))
	utilSignPrivKeyFile := utilSignCmd.Flag("private-key-file", msgPrinter.Sprintf("The path of a private key file to be used to sign the stdin. ")).Short('k').Required().ExistingFile()
	utilVerifyCmd := utilCmd.Command("verify | vf", msgPrinter.Sprintf("Verify that the signature specified via -s is a valid signature for the text in stdin.")).Alias("vf").Alias("verify")
	utilVerifyPubKeyFile := utilVerifyCmd.Flag("public-key-file", msgPrinter.Sprintf("The path of public key file (that corresponds to the private key that was used to sign) to verify the signature of stdin.")).Short('K').Required().ExistingFile()
	utilVerifySig := utilVerifyCmd.Flag("signature", msgPrinter.Sprintf("The supposed signature of stdin.")).Short('s').Required().String()

	smCmd := app.Command("secretsmanager | sm", msgPrinter.Sprintf("List and manage secrets in the secrets manager. NOTE: You must authenticate as an administrator to list secrets available to the entire organization. Secrets are not supported on cluster agents.")).Alias("sm").Alias("secretsmanager")
	smOrg := smCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	smUserPw := smCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange credentials to query secrets manager resources. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()
	smSecretCmd := smCmd.Command("secret", msgPrinter.Sprintf("List and manage secrets in the secrets manager."))
	smSecretListCmd := smSecretCmd.Command("list | ls", msgPrinter.Sprintf("Display the names of the secrets in the secrets manager.")).Alias("ls").Alias("list")
	smSecretListName := smSecretListCmd.Arg("secretName", msgPrinter.Sprintf("List just this one secret. Returns a boolean indicating the existence of the secret. This is the name of the secret used in the secrets manager. If the secret does not exist, returns with exit code 1.")).String()
	smSecretAddCmd := smSecretCmd.Command("add", msgPrinter.Sprintf("Add a secret to the secrets manager."))
	smSecretAddName := smSecretAddCmd.Arg("secretName", msgPrinter.Sprintf("The name of the secret. It must be unique within your organization. This name is used in deployment policies and patterns to bind this secret to a secret name in a service definition.")).Required().String()
	smSecretAddFile := smSecretAddCmd.Flag("secretFile", msgPrinter.Sprintf("Filepath to a file containing the secret details. Mutually exclusive with --secretDetail. Specify -f- to read from stdin.")).Short('f').String()
	smSecretAddKey := smSecretAddCmd.Flag("secretKey", msgPrinter.Sprintf("A key for the secret.")).Required().String()
	smSecretAddDetail := smSecretAddCmd.Flag("secretDetail", msgPrinter.Sprintf("The secret details as a string. Secret details are the actual secret itself, not the name of the secret. For example, a password, a private key, etc. are examples of secret details. Mutually exclusive with --secretFile.")).Short('d').String()
	smSecretAddOverwrite := smSecretAddCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing secret if it exists in the secrets manager. It will skip the 'do you want to overwrite' prompt.")).Short('O').Bool()
	smSecretRemoveCmd := smSecretCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a secret in the secrets manager.")).Alias("rm").Alias("remove")
	smSecretRemoveForce := smSecretRemoveCmd.Flag("force", msgPrinter.Sprintf("Skip the 'are you sure?' prompt.")).Short('f').Bool()
	smSecretRemoveName := smSecretRemoveCmd.Arg("secretName", msgPrinter.Sprintf("The name of the secret to be removed from the secrets manager.")).Required().String()
	smSecretReadCmd := smSecretCmd.Command("read", msgPrinter.Sprintf("Read the details of a secret stored in the secrets manager. This consists of the key and value pair provided on secret creation."))
	smSecretReadName := smSecretReadCmd.Arg("secretName", msgPrinter.Sprintf("The name of the secret to read in the secrets manager.")).Required().String()

	versionCmd := app.Command("version", msgPrinter.Sprintf("Show the Horizon version.")) // using a cmd for this instead of --version flag, because kingpin takes over the latter and can't get version only when it is needed

	sdoCmd := app.Command("sdo", msgPrinter.Sprintf("List and manage resources in SDO owner services"))
	sdoOrg := sdoCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	sdoUserPw := sdoCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange credentials to SDO owner service resources. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()

	sdoKeyCmd := sdoCmd.Command("key", msgPrinter.Sprintf("List and manage Horizon SDO ownership keys."))

	sdoKeyListCmd := sdoKeyCmd.Command("list | ls", msgPrinter.Sprintf("List the SDO ownership keys stored in SDO owner services.")).Alias("ls").Alias("list")
	sdoKeyToList := sdoKeyListCmd.Arg("keyName", msgPrinter.Sprintf("List the full details of this SDO ownership key.")).String()
	sdoKeyCreateCmd := sdoKeyCmd.Command("create | cr", msgPrinter.Sprintf("Create a new key in SDO owner services.")).Alias("cr").Alias("create")
	sdoKeyCreateInputFile := sdoKeyCreateCmd.Arg("key-meta-file", msgPrinter.Sprintf("The file containing metadata for the key to be created in SDO owner services. Must be JSON file type extension.")).Required().File()
	sdoKeyCreateFile := sdoKeyCreateCmd.Flag("file-path", msgPrinter.Sprintf("The file that the returned public key is written to. If omit, the key will be printed to the console.")).Short('f').String()
	sdoKeyCreateOverwrite := sdoKeyCreateCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing output public key file if it exists.")).Short('O').Bool()
	sdoKeyDownloadCmd := sdoKeyCmd.Command("download | dl", msgPrinter.Sprintf("Download the specified key from SDO owner services.")).Alias("dl").Alias("download")
	sdoKeyToDownload := sdoKeyDownloadCmd.Arg("keyName", msgPrinter.Sprintf("The name of the key to be downloaded from SDO owner services.")).Required().String()
	sdoKeyDownloadFile := sdoKeyDownloadCmd.Flag("file-path", msgPrinter.Sprintf("The file that the data of downloaded key is written to. If omit, the key will be printed to the console.")).Short('f').String()
	sdoKeyDownloadOverwrite := sdoKeyDownloadCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing file if it exists.")).Short('O').Bool()
	sdoKeyRemoveCmd := sdoKeyCmd.Command("remove | rm", msgPrinter.Sprintf("Remove a key from SDO owner services.")).Alias("rm").Alias("remove")
	sdoKeyToRemove := sdoKeyRemoveCmd.Arg("keyName", msgPrinter.Sprintf("The name of the key to be removed from SDO owner services.")).Required().String()
	sdoKeyNewCmd := sdoKeyCmd.Command("new", msgPrinter.Sprintf("Create a new SDO key metadata template file. All fields must be filled before adding to SDO owner services."))
	sdoKeyNewFile := sdoKeyNewCmd.Flag("file-path", msgPrinter.Sprintf("The file that the SDO key template will be written to in JSON format. If omit, the key metadata will be printed to the console.")).Short('f').String()
	sdoKeyNewOverwrite := sdoKeyNewCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing file if it exists.")).Short('O').Bool()

	sdoVoucherCmd := sdoCmd.Command("voucher", msgPrinter.Sprintf("List and manage Horizon SDO ownership vouchers."))

	sdoVoucherListCmd := sdoVoucherCmd.Command("list | ls", msgPrinter.Sprintf("List the imported SDO ownership vouchers.")).Alias("ls").Alias("list")
	sdoVoucherToList := sdoVoucherListCmd.Arg("voucher", msgPrinter.Sprintf("List the full details of this SDO ownership voucher.")).String()
	sdoVoucherListLong := sdoVoucherListCmd.Flag("long", msgPrinter.Sprintf("When a voucher uuid is specified the full contents of the voucher will be listed, otherwise the full contents of all the imported vouchers will be listed.")).Short('l').Bool()
	sdoVoucherInspectCmd := sdoVoucherCmd.Command("inspect | ins", msgPrinter.Sprintf("Display properties of the SDO ownership voucher.")).Alias("ins").Alias("inspect")
	sdoVoucherInspectFile := sdoVoucherInspectCmd.Arg("voucher-file", msgPrinter.Sprintf("The SDO ownership voucher file.")).Required().File() // returns the file descriptor
	sdoVoucherImportCmd := sdoVoucherCmd.Command("import | imp", msgPrinter.Sprintf("Imports the SDO ownership voucher so that the corresponding device can be booted, configured, and registered.")).Alias("import").Alias("imp")
	sdoVoucherImportFile := sdoVoucherImportCmd.Arg("voucher-file", msgPrinter.Sprintf("The SDO ownership voucher file. Must be file type extension: json, tar, tar.gz, tgz, or zip. If it is any of the tar/zip formats, all json files within it will be imported (other files/dirs will be silently ignored).")).Required().File() // returns the file descriptor
	sdoVoucherImportExample := sdoVoucherImportCmd.Flag("example", msgPrinter.Sprintf("Automatically create a node policy that will result in the specified example edge service (for example 'helloworld') being deployed to the edge device associated with this voucher. It is mutually exclusive with --policy and -p.")).Short('e').String()
	sdoVoucherImportPolicy := sdoVoucherImportCmd.Flag("policy", msgPrinter.Sprintf("The node policy file to use for the edge device associated with this voucher. It is mutually exclusive with -e and -p. A node policy contains the 'deployment' and 'management' attributes. Please use 'hzn policy new' to see the node policy format.")).String()
	sdoVoucherImportPattern := sdoVoucherImportCmd.Flag("pattern", msgPrinter.Sprintf("The deployment pattern name to use for the edge device associated with this voucher. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. It is mutually exclusive with -e and --policy.")).Short('p').String()
	sdoVoucherImportUI := sdoVoucherImportCmd.Flag("user-input", msgPrinter.Sprintf("A JSON file that sets or overrides user input variables needed by the services that will be deployed to the edge device associated with this voucher. Please use 'hzn userinput new' to see the format.")).Short('f').String()
	sdoVoucherImportHAGroup := sdoVoucherImportCmd.Flag("ha-group", msgPrinter.Sprintf("The name of the HA group that the edge device associated with this voucher will be added to.")).String()
	sdoVoucherDownloadCmd := sdoVoucherCmd.Command("download | dl", msgPrinter.Sprintf("Download the specified SDO ownership voucher from SDO owner services.")).Alias("dl").Alias("download")
	sdoVoucherDownloadDevice := sdoVoucherDownloadCmd.Arg("device-id", msgPrinter.Sprintf("The SDO ownership voucher to download.")).Required().String()
	sdoVoucherDownloadFile := sdoVoucherDownloadCmd.Flag("file-path", msgPrinter.Sprintf("The file that the data of downloaded voucher is written to in JSON format. This flag must be used with -f. If omit, will use default file name in format of <deviceID>.json and save in current directory.")).Short('f').String()
	sdoVoucherDownloadOverwrite := sdoVoucherDownloadCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing file if it exists.")).Short('O').Bool()

	voucherCmd := app.Command("voucher", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn sdo voucher' to list and manage Horizon SDO ownership vouchers."))

	voucherListCmd := voucherCmd.Command("list | ls", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn sdo voucher list' to list the imported SDO ownership vouchers.")).Alias("ls").Alias("list")
	voucherToList := voucherListCmd.Arg("voucher", msgPrinter.Sprintf("List the full details of this SDO ownership voucher.")).String()
	voucherListLong := voucherListCmd.Flag("long", msgPrinter.Sprintf("When a voucher uuid is specified the full contents of the voucher will be listed, otherwise the full contents of all the imported vouchers will be listed.")).Short('l').Bool()
	voucherInspectCmd := voucherCmd.Command("inspect | ins", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn sdo voucher inspect' to display properties of the SDO ownership voucher.")).Alias("ins").Alias("inspect")
	voucherInspectFile := voucherInspectCmd.Arg("voucher-file", msgPrinter.Sprintf("The SDO ownership voucher file.")).Required().File() // returns the file descriptor
	voucherImportCmd := voucherCmd.Command("import | imp", msgPrinter.Sprintf("(DEPRECATED) This command is deprecated. Please use 'hzn sdo voucher import' to import the SDO ownership voucher")).Alias("import").Alias("imp")
	voucherImportFile := voucherImportCmd.Arg("voucher-file", msgPrinter.Sprintf("The SDO ownership voucher file. Must be file type extension: json, tar, tar.gz, tgz, or zip. If it is any of the tar/zip formats, all json files within it will be imported (other files/dirs will be silently ignored).")).Required().File() // returns the file descriptor
	voucherOrg := voucherImportCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	voucherUserPw := voucherImportCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon user credentials to import a voucher. If not specified, HZN_EXCHANGE_USER_AUTH will be used as a default. If you don't prepend it with the user's org, it will automatically be prepended with the -o value.")).Short('u').PlaceHolder("USER:PW").String()
	voucherImportExample := voucherImportCmd.Flag("example", msgPrinter.Sprintf("Automatically create a node policy that will result in the specified example edge service (for example 'helloworld') being deployed to the edge device associated with this voucher. It is mutually exclusive with --policy and -p.")).Short('e').String()
	voucherImportPolicy := voucherImportCmd.Flag("policy", msgPrinter.Sprintf("The node policy file to use for the edge device associated with this voucher. It is mutually exclusive with -e and -p.")).String()
	voucherImportPattern := voucherImportCmd.Flag("pattern", msgPrinter.Sprintf("The deployment pattern name to use for the edge device associated with this voucher. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. It is mutually exclusive with -e and --policy.")).Short('p').String()

	fdoCmd := app.Command("fdo", msgPrinter.Sprintf("List and manage resources in FDO owner services"))
	fdoOrg := fdoCmd.Flag("org", msgPrinter.Sprintf("The Horizon organization ID. If not specified, HZN_ORG_ID will be used as a default.")).Short('o').String()
	fdoUserPw := fdoCmd.Flag("user-pw", msgPrinter.Sprintf("Horizon Exchange credentials to query FDO Owner service resources. The default is HZN_EXCHANGE_USER_AUTH environment variable. If you don't prepend it with the user's org, it will automatically be prepended with the value of the HZN_ORG_ID environment variable.")).Short('u').PlaceHolder("USER:PW").String()

	fdoKeyCmd := fdoCmd.Command("key", msgPrinter.Sprintf("List FDO ownership public keys."))

	fdoKeyListCmd := fdoKeyCmd.Command("list | ls", msgPrinter.Sprintf("List a public key from the FDO Owner services using the device alias from the manufacturer.")).Alias("ls").Alias("list")
	fdoKeyToList := fdoKeyListCmd.Arg("alias", msgPrinter.Sprintf("The device alias received from the manufacturer. It can be one of the following: SECP256R1, SECP384R1, RSAPKCS3072, RSAPKCS2048, RSA2048RESTR which are all cryptography standards.")).String()

	fdoVoucherCmd := fdoCmd.Command("voucher", msgPrinter.Sprintf("List and manage Horizon FDO ownership vouchers."))

	fdoVoucherListCmd := fdoVoucherCmd.Command("list | ls", msgPrinter.Sprintf("List the imported FDO ownership vouchers.")).Alias("ls").Alias("list")
	fdoVoucherToList := fdoVoucherListCmd.Arg("voucher", msgPrinter.Sprintf("List the full details of this FDO ownership voucher.")).String()
	fdoVoucherImportCmd := fdoVoucherCmd.Command("import | imp", msgPrinter.Sprintf("Imports the FDO ownership voucher so that the corresponding device can be booted, configured, and registered.")).Alias("import").Alias("imp")
	fdoVoucherImportFile := fdoVoucherImportCmd.Arg("voucher-file", msgPrinter.Sprintf("The FDO ownership voucher file. Must be file type extension: txt, tar, tar.gz, tgz, or zip. If it is any of the tar/zip formats, all .txt files within it will be imported (other files/dirs will be silently ignored).")).Required().File() // returns the file descriptor
	fdoVoucherImportExample := fdoVoucherImportCmd.Flag("example", msgPrinter.Sprintf("Automatically create a node policy that will result in the specified example edge service (for example 'helloworld') being deployed to the edge device associated with this voucher. It is mutually exclusive with --policy and -p.")).Short('e').String()
	fdoVoucherImportPolicy := fdoVoucherImportCmd.Flag("policy", msgPrinter.Sprintf("The node policy file to use for the edge device associated with this voucher. It is mutually exclusive with -e and -p. A node policy contains the 'deployment' and 'management' attributes. Please use 'hzn policy new' to see the node policy format.")).String()
	fdoVoucherImportPattern := fdoVoucherImportCmd.Flag("pattern", msgPrinter.Sprintf("The deployment pattern name to use for the edge device associated with this voucher. If the pattern is from a different organization than the node, use the 'other_org/pattern' format. It is mutually exclusive with -e and --policy.")).Short('p').String()
	fdoVoucherImportUI := fdoVoucherImportCmd.Flag("user-input", msgPrinter.Sprintf("A JSON file that sets or overrides user input variables needed by the services that will be deployed to the edge device associated with this voucher. Please use 'hzn userinput new' to see the format.")).Short('f').String()
	fdoVoucherImportHAGroup := fdoVoucherImportCmd.Flag("ha-group", msgPrinter.Sprintf("The name of the HA group that the edge device associated with this voucher will be added to.")).String()
	fdoVoucherDownloadCmd := fdoVoucherCmd.Command("download | dl", msgPrinter.Sprintf("Download the specified FDO ownership voucher from FDO owner services.")).Alias("dl").Alias("download")
	fdoVoucherDownloadDevice := fdoVoucherDownloadCmd.Arg("device-id", msgPrinter.Sprintf("The FDO ownership voucher to download.")).Required().String()
	fdoVoucherDownloadFile := fdoVoucherDownloadCmd.Flag("file-path", msgPrinter.Sprintf("The file that the data of downloaded voucher is written to in JSON format. This flag must be used with -f. If omit, will use default file name in format of <deviceID>.json and save in current directory.")).Short('f').String()
	fdoVoucherDownloadOverwrite := fdoVoucherDownloadCmd.Flag("overwrite", msgPrinter.Sprintf("Overwrite the existing file if it exists.")).Short('O').Bool()

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
	if strings.HasPrefix(fullCmd, "exchange ") {
		exOrg = cliutils.WithDefaultEnvVar(exOrg, "HZN_ORG_ID")

		// Allow undefined org for 'exchange org' commands and 'new' commands
		if *exOrg == "" && !(strings.HasPrefix(fullCmd, "exchange | ex org") ||
			strings.HasPrefix(fullCmd, "exchange | ex deployment | dep new") ||
			strings.HasPrefix(fullCmd, "exchange | ex service | serv newpolicy | newp") ||
			strings.HasPrefix(fullCmd, "exchange | ex nmp new")) {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		}

		// some hzn exchange commands can take either -u user:pw or -n nodeid:token as credentials.
		switch subCmd := strings.TrimPrefix(fullCmd, "exchange | ex "); subCmd {
		case "nmp add":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", true)
		case "nmp list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNMPListIdTok, false)
		case "nmp new":
			// does not require exchange credentials
		case "nmp remove | rm":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", true)
		case "nmp status":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNMPStatusIdTok, false)
		case "nmp enable":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", true)
		case "nmp disable":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", true)
		case "node list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeListNodeIdTok, false)
		case "node update | up":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeUpdateIdTok, false)
		case "node settoken":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeSetTokNodeIdTok, false)
		case "node remove | rm":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeRemoveNodeIdTok, false)
		case "node confirm | con":
			//do nothing because it uses the node id and token given in the argument as the credential
		case "node listpolicy | lsp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeListPolicyIdTok, false)
		case "node addpolicy | addp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeAddPolicyIdTok, false)
		case "node updatepolicy | upp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeUpdatePolicyIdTok, false)
		case "node removepolicy | rmp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeRemovePolicyIdTok, false)
		case "node listerrors | lse":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeErrorsListIdTok, false)
		case "node liststatus | lst":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeStatusIdTok, false)
		case "node management | mgmt list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeManagementListNodeIdTok, false)
		case "node management | mgmt status":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeManagementStatusNodeIdTok, false)
		case "node management | mgmt reset":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exNodeManagementResetNodeIdTok, false)
		case "service | serv list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListNodeIdTok, false)
		case "service | serv verify | vf":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceVerifyNodeIdTok, false)
		case "service | serv listkey | lsk":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListKeyNodeIdTok, false)
		case "service | serv listauth | lsau":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListAuthNodeIdTok, false)
		case "pattern | pat list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternListNodeIdTok, false)
		case "pattern | pat update | up":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatUpdateNodeIdTok, false)
		case "pattern | pat verify | vf":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternVerifyNodeIdTok, false)
		case "pattern | pat listkey | lsk":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exPatternListKeyNodeIdTok, false)
		case "service | serv listpolicy | lsp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceListPolicyIdTok, false)
		case "service | serv addpolicy | addp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceAddPolicyIdTok, false)
		case "service | serv removepolicy | rmp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exServiceRemovePolicyIdTok, false)
		case "service | serv newpolicy | newp":
			// does not require exchange credentials
		case "hagroup | hagr list | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exHAGroupListNodeIdTok, false)
		case "hagroup | hagr add":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", false)
		case "hagroup | hagr remove | rm":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", false)
		case "hagroup | hagr new":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", false)
		case "hagroup | hagr member | mb add":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", false)
		case "hagroup | hagr member | mb remove | rm":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, "", false)
		case "deployment | dep listpolicy | ls":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessListPolicyIdTok, false)
		case "deployment | dep updatepolicy | upp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessUpdatePolicyIdTok, false)
		case "deployment | dep addpolicy | addp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessAddPolicyIdTok, false)
		case "deployment | dep removepolicy | rmp":
			credToUse = cliutils.GetExchangeAuth(*exUserPw, *exBusinessRemovePolicyIdTok, false)
		case "deployment | dep new":
			// does not require exchange credentials
		case "version":
			credToUse = cliutils.GetExchangeAuthVersion(*exUserPw)
		default:
			// get HZN_EXCHANGE_USER_AUTH as default if exUserPw is empty
			exUserPw = cliutils.RequiredWithDefaultEnvVar(exUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
		}

		if exVersion := exchange.LoadExchangeVersion(false, *exOrg, credToUse, *exUserPw); exVersion != "" {
			if err := version.VerifyExchangeVersion1(exVersion, false); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			}
		}
	}

	if strings.HasPrefix(fullCmd, "register") {
		// use HZN_EXCHANGE_USER_AUTH for -u
		userPw = cliutils.WithDefaultEnvVar(userPw, "HZN_EXCHANGE_USER_AUTH")

		// use HZN_EXCHANGE_NODE_AUTH for -n and trim the org
		nodeIdTok = cliutils.WithDefaultEnvVar(nodeIdTok, "HZN_EXCHANGE_NODE_AUTH")

		// use HZN_ORG_ID or org provided by -o for version check
		verCheckOrg := cliutils.WithDefaultEnvVar(org, "HZN_ORG_ID")

		if exVersion := exchange.LoadExchangeVersion(false, *verCheckOrg, *userPw, *nodeIdTok); exVersion != "" {
			if err := version.VerifyExchangeVersion1(exVersion, false); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			}
		}
	}

	if strings.HasPrefix(fullCmd, "deploycheck") {
		deploycheckOrg = cliutils.WithDefaultEnvVar(deploycheckOrg, "HZN_ORG_ID")
		deploycheckUserPw = cliutils.WithDefaultEnvVar(deploycheckUserPw, "HZN_EXCHANGE_USER_AUTH")
		if *policyCompBPolId == "" {
			policyCompBPolId = policyCompDepPolId
		}
		if *policyCompBPolFile == "" {
			policyCompBPolFile = policyCompDepPolFile
		}
		if *userinputCompBPolId == "" {
			userinputCompBPolId = userinputCompDepPolId
		}
		if *userinputCompBPolFile == "" {
			userinputCompBPolFile = userinputCompDepPolFile
		}
		if *allCompBPolId == "" {
			allCompBPolId = allCompDepPolId
		}
		if *allCompBPolFile == "" {
			allCompBPolFile = allCompDepPolFile
		}

		if exVersion := exchange.LoadExchangeVersion(false, *deploycheckOrg, *deploycheckUserPw); exVersion != "" {
			if err := version.VerifyExchangeVersion1(exVersion, false); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			}
		}
	}

	// For the mms command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "mms") {
		if !(strings.HasPrefix(fullCmd, "mms object | obj new")) {
			mmsOrg = cliutils.RequiredWithDefaultEnvVar(mmsOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
			mmsUserPw = cliutils.RequiredWithDefaultEnvVar(mmsUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))

		}

		if *mmsObjectListId == "" {
			mmsObjectListId = mmsObjectListObjId
		}
		if *mmsObjectListType == "" {
			mmsObjectListType = mmsObjectListObjType
		}
	}

	// For the nodemanagement command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "nodemanagement | nm") {
		if !(strings.HasPrefix(fullCmd, "nodemanagement | nm manifest | man new")) {
			nmOrg = cliutils.RequiredWithDefaultEnvVar(nmOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
			nmUserPw = cliutils.RequiredWithDefaultEnvVar(nmUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))

		}
	}

	// For the sdo command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "sdo voucher") && !strings.HasPrefix(fullCmd, "sdo voucher inspect") {
		sdoOrg = cliutils.RequiredWithDefaultEnvVar(sdoOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		sdoUserPw = cliutils.RequiredWithDefaultEnvVar(sdoUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}
	if strings.HasPrefix(fullCmd, "sdo key") && !strings.HasPrefix(fullCmd, "sdo key new") {
		sdoOrg = cliutils.RequiredWithDefaultEnvVar(sdoOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		sdoUserPw = cliutils.RequiredWithDefaultEnvVar(sdoUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}

	// For the fdo command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "fdo voucher") {
		fdoOrg = cliutils.RequiredWithDefaultEnvVar(fdoOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		fdoUserPw = cliutils.RequiredWithDefaultEnvVar(fdoUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}
	if strings.HasPrefix(fullCmd, "fdo key") {
		fdoOrg = cliutils.RequiredWithDefaultEnvVar(fdoOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		fdoUserPw = cliutils.RequiredWithDefaultEnvVar(fdoUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}

	// DEPRECATED
	// For the voucher import command family, make sure that org and exchange credentials are specified in some way.
	if strings.HasPrefix(fullCmd, "voucher import") {
		voucherOrg = cliutils.RequiredWithDefaultEnvVar(voucherOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		voucherUserPw = cliutils.RequiredWithDefaultEnvVar(voucherUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}
	if strings.HasPrefix(fullCmd, "voucher list") {
		voucherOrg = cliutils.RequiredWithDefaultEnvVar(voucherOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		voucherUserPw = cliutils.RequiredWithDefaultEnvVar(voucherUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}

	// For the secret manager command family, make sure that org is specified in some way.
	if strings.HasPrefix(fullCmd, "secretsmanager") {
		smOrg = cliutils.RequiredWithDefaultEnvVar(smOrg, "HZN_ORG_ID", msgPrinter.Sprintf("organization ID must be specified with either the -o flag or HZN_ORG_ID"))
		smUserPw = cliutils.RequiredWithDefaultEnvVar(smUserPw, "HZN_EXCHANGE_USER_AUTH", msgPrinter.Sprintf("exchange user authentication must be specified with either the -u flag or HZN_EXCHANGE_USER_AUTH"))
	}

	// key file defaults
	switch fullCmd {
	case "key create":
		if *keyOutputDir == "" {
			keyCreatePrivKey = cliutils.WithDefaultEnvVar(keyCreatePrivKey, "HZN_PRIVATE_KEY_FILE")
			keyCreatePubKey = cliutils.WithDefaultEnvVar(keyCreatePubKey, "HZN_PUBLIC_KEY_FILE")
		}
	case "exchange | ex pattern | pat verify":
		exPatPubKeyFile = cliutils.WithDefaultEnvVar(exPatPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	case "exchange | ex service | serv verify":
		exSvcPubKeyFile = cliutils.WithDefaultEnvVar(exSvcPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	case "key import":
		keyImportPubKeyFile = cliutils.WithDefaultEnvVar(keyImportPubKeyFile, "HZN_PUBLIC_KEY_FILE")
	}

	// set env variable ARCH if it is not set
	cliutils.SetDefaultArch()

	// Decide which command to run
	switch fullCmd {
	case envCmd.FullCommand():
		envOrg := os.Getenv("HZN_ORG_ID")
		envUserPw := os.Getenv("HZN_EXCHANGE_USER_AUTH")
		envExchUrl := cliutils.GetExchangeUrl()
		envCcsUrl := cliutils.GetMMSUrl()
		envAgbotUrl := cliutils.GetAgbotSecureAPIUrlBase()
		node.Env(envOrg, envUserPw, envExchUrl, envCcsUrl, envAgbotUrl)
	case versionCmd.FullCommand():
		node.Version()
	case archCmd.FullCommand():
		node.Architecture()
	case exVersionCmd.FullCommand():
		exchange.Version(*exOrg, credToUse)
	case exStatusCmd.FullCommand():
		exchange.Status(*exOrg, *exUserPw)

	case exOrgListCmd.FullCommand():
		exchange.OrgList(*exOrg, *exUserPw, *exOrgListOrg, *exOrgListLong)
	case exOrgCreateCmd.FullCommand():
		exchange.OrgCreate(*exOrg, *exUserPw, *exOrgCreateOrg, *exOrgCreateLabel, *exOrgCreateDesc, *exOrgCreateTags, *exOrgCreateHBMin, *exOrgCreateHBMax, *exOrgCreateHBAdjust, *exOrgCreateMaxNodes, *exOrgCreateAddToAgbot)
	case exOrgUpdateCmd.FullCommand():
		exchange.OrgUpdate(*exOrg, *exUserPw, *exOrgUpdateOrg, *exOrgUpdateLabel, *exOrgUpdateDesc, *exOrgUpdateTags, *exOrgUpdateHBMin, *exOrgUpdateHBMax, *exOrgUpdateHBAdjust, *exOrgUpdateMaxNodes)
	case exOrgDelCmd.FullCommand():
		exchange.OrgDel(*exOrg, *exUserPw, *exOrgDelOrg, *exOrgDelFromAgbot, *exOrgDelForce)

	case exUserListCmd.FullCommand():
		exchange.UserList(*exOrg, *exUserPw, *exUserListUser, *exUserListAll, *exUserListNamesOnly)
	case exUserCreateCmd.FullCommand():
		exchange.UserCreate(*exOrg, *exUserPw, *exUserCreateUser, *exUserCreatePw, *exUserCreateEmail, *exUserCreateIsAdmin, *exUserCreateIsHubAdmin)
	case exUserSetAdminCmd.FullCommand():
		exchange.UserSetAdmin(*exOrg, *exUserPw, *exUserSetAdminUser, *exUserSetAdminBool)
	case exUserDelCmd.FullCommand():
		exchange.UserRemove(*exOrg, *exUserPw, *exDelUser, *exUserDelForce)

	case exNMPListCmd.FullCommand():
		exchange.NMPList(*exOrg, credToUse, *exNMPListName, !*exNMPListLong, *exNMPListNodes)
	case exNMPAddCmd.FullCommand():
		exchange.NMPAdd(*exOrg, credToUse, *exNMPAddName, *exNMPAddJsonFile, *exNMPAddAppliesTo, *exNMPAddNoConstraint)
	case exNMPNewCmd.FullCommand():
		exchange.NMPNew()
	case exNMPRemoveCmd.FullCommand():
		exchange.NMPRemove(*exOrg, credToUse, *exNMPRemoveName, *exNMPRemoveForce)
	case exNMPStatusCmd.FullCommand():
		exchange.NMPStatus(*exOrg, credToUse, *exNMPStatusName, *exNMPStatusNode, !*exNMPStatusLong)
	case exNMPEnableCmd.FullCommand():
		exchange.NMPEnable(*exOrg, credToUse, *exNMPEnableName, *exNMPEnableStartTime, *exNMPEnableStartWindow)
	case exNMPDisableCmd.FullCommand():
		exchange.NMPDisable(*exOrg, credToUse, *exNMPDisableName)

	case exHAGroupNewCmd.FullCommand():
		exchange.HAGroupNew()
	case exHAGroupListCmd.FullCommand():
		exchange.HAGroupList(*exOrg, credToUse, *exHAGroupListName, !*exHAGroupListLong)
	case exHAGroupAddCmd.FullCommand():
		exchange.HAGroupAdd(*exOrg, credToUse, *exHAGroupAddName, *exHAGroupAddJsonFile)
	case exHAGroupRemoveCmd.FullCommand():
		exchange.HAGroupRemove(*exOrg, credToUse, *exHAGroupRemoveName, *exHAGroupRemoveForce)
	case exHAGroupMemberAddCmd.FullCommand():
		exchange.HAGroupMemberAdd(*exOrg, credToUse, *exHAGroupMemberAddName, *exHAGroupMemberAddNodes)
	case exHAGroupMemberRemoveCmd.FullCommand():
		exchange.HAGroupMemberRemove(*exOrg, credToUse, *exHAGroupMemberRemoveName, *exHAGroupMemberRemoveNodes, *exHAGroupMemberRemoveForce)

	case exNodeListCmd.FullCommand():
		exchange.NodeList(*exOrg, credToUse, *exNode, !*exNodeLong)
	case exNodeUpdateCmd.FullCommand():
		exchange.NodeUpdate(*exOrg, credToUse, *exNodeUpdateNode, *exNodeUpdateJsonFile)
	case exNodeCreateCmd.FullCommand():
		exchange.NodeCreate(*exOrg, *exNodeCreateNodeIdTok, *exNodeCreateNode, *exNodeCreateToken, *exUserPw, *exNodeCreateNodeArch, *exNodeCreateNodeName, *exNodeCreateNodeType, true)
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
	case exNodeStatusList.FullCommand():
		exchange.NodeListStatus(*exOrg, credToUse, *exNodeStatusListNode)
	case exNodeManagementListCmd.FullCommand():
		exchange.NodeManagementList(*exOrg, credToUse, *exNodeManagementListName, *exNodeManagementListAll)
	case exNodeManagementStatusCmd.FullCommand():
		exchange.NodeManagementStatus(*exOrg, credToUse, *exNodeManagementStatusName, *exNodeManagementStatusPol, !*exNodeManagementStatusLong)
	case exNodeManagementResetCmd.FullCommand():
		exchange.NodeManagementReset(*exOrg, credToUse, *exNodeManagementResetName, *exNodeManagementResetPol)

	case agbotCacheServedOrgList.FullCommand():
		agreementbot.GetServedOrgs()
	case agbotCachePatternList.FullCommand():
		agreementbot.GetPatterns(*agbotCachePatternListOrg, *agbotCachePatternListName, *agbotCachePatternListLong)
	case agbotCacheDeployPolList.FullCommand():
		agreementbot.GetPolicies(*agbotCacheDeployPolListOrg, *agbotCacheDeployPolListName, *agbotCacheDeployPolListLong)

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
		exchange.ServiceList(*exOrg, credToUse, *exService, !*exServiceLong, *exSvcOpYamlFilePath, *exSvcOpYamlForce)
	case exServicePublishCmd.FullCommand():
		exchange.ServicePublish(*exOrg, *exUserPw, *exSvcJsonFile, *exSvcPrivKeyFile, *exSvcPubPubKeyFile, *exSvcPubDontTouchImage, *exSvcPubPullImage, *exSvcRegistryTokens, *exSvcOverwrite, *exSvcPolicyFile, *exSvcPublic)
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
	case exServiceListnode.FullCommand():
		exchange.ListServiceNodes(*exOrg, *exUserPw, *exServiceListnodeService, *exServiceListnodeNodeOrg)
	case exBusinessListPolicyCmd.FullCommand():
		exchange.BusinessListPolicy(*exOrg, credToUse, *exBusinessListPolicyPolicy, !*exBusinessListPolicyLong)
	case exBusinessNewPolicyCmd.FullCommand():
		exchange.BusinessNewPolicy()
	case exBusinessAddPolicyCmd.FullCommand():
		exchange.BusinessAddPolicy(*exOrg, credToUse, *exBusinessAddPolicyPolicy, *exBusinessAddPolicyJsonFile, *exBusinessAddPolNoConstraint)
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
		register.DoIt(*org, *pattern, *nodeIdTok, *userPw, *inputFile, *nodeOrgFlag, *patternFlag, *nodeName, *haGroupName, *nodepolicyFlag, *waitServiceFlag, *waitServiceOrgFlag, *waitTimeoutFlag)
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
		deploycheck.PolicyCompatible(*deploycheckOrg, *deploycheckUserPw, *policyCompNodeId, *policyCompHAGroup, *policyCompNodeArch, *policyCompNodeType, *policyCompNodePolFile, *policyCompBPolId, *policyCompBPolFile, *policyCompSPolFile, *policyCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
	case userinputCompCmd.FullCommand():
		deploycheck.UserInputCompatible(*deploycheckOrg, *deploycheckUserPw, *userinputCompNodeId, *userinputCompNodeArch, *userinputCompNodeType, *userinputCompNodeUIFile, *userinputCompBPolId, *userinputCompBPolFile, *userinputCompPatternId, *userinputCompPatternFile, *userinputCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
	case secretCompCmd.FullCommand():
		deploycheck.SecretBindingCompatible(*deploycheckOrg, *deploycheckUserPw, *secretCompNodeId, *secretCompNodeArch, *secretCompNodeType, *secretCompNodeOrg, *secretCompDepPolId, *secretCompDepPolFile, *secretCompPatternId, *secretCompPatternFile, *secretCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
	case allCompCmd.FullCommand():
		deploycheck.AllCompatible(*deploycheckOrg, *deploycheckUserPw, *allCompNodeId, *allCompHAGroup, *allCompNodeArch, *allCompNodeType, *allCompNodeOrg, *allCompNodePolFile, *allCompNodeUIFile, *allCompBPolId, *allCompBPolFile, *allCompPatternId, *allCompPatternFile, *allCompSPolFile, *allCompSvcFile, *deploycheckCheckAll, *deploycheckLong)
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
		service.Log(*logServiceName, *logServiceVersion, *logServiceContainerName, *logTail)
	case serviceRegisteredCmd.FullCommand():
		service.Registered()
	case serviceConfigStateListCmd.FullCommand():
		service.ListConfigState()
	case serviceConfigStateSuspendCmd.FullCommand():
		service.Suspend(*forceSuspendService, *suspendAllServices, *suspendServiceOrg, *suspendServiceName, *suspendServiceVersion)
	case serviceConfigStateActiveCmd.FullCommand():
		service.Resume(*resumeAllServices, *resumeServiceOrg, *resumeServiceName, *resumeServiceVersion)
	case unregisterCmd.FullCommand():
		unregister.DoIt(*forceUnregister, *removeNodeUnregister, *deepCleanUnregister, *timeoutUnregister, *containerUnregister)
	case statusCmd.FullCommand():
		status.DisplayStatus(*statusLong, false)
	case eventlogListCmd.FullCommand():
		eventlog.List(*listAllEventlogs, *listDetailedEventlogs, *listSelectedEventlogs, *listTail)
	case surfaceErrorsEventlogs.FullCommand():
		eventlog.ListSurfaced(*surfaceErrorsEventlogsLong)
	case devServiceNewCmd.FullCommand():
		dev.ServiceNew(*devHomeDirectory, *devServiceNewCmdOrg, *devServiceNewCmdName, *devServiceNewCmdVer, *devServiceNewCmdImage, *devServiceNewCmdNoImageGen, *devServiceNewCmdCfg, *devServiceNewCmdNoPattern, *devServiceNewCmdNoPolicy)
	case devServiceStartTestCmd.FullCommand():
		dev.ServiceStartTest(*devHomeDirectory, *devServiceUserInputFile, *devServiceConfigFile, *devServiceConfigType, *devServiceNoFSS, *devServiceStartCmdUserPw, *devServiceStartSecretsFiles)
	case devServiceStopTestCmd.FullCommand():
		dev.ServiceStopTest(*devHomeDirectory)
	case devServiceValidateCmd.FullCommand():
		dev.ServiceValidate(*devHomeDirectory, *devServiceVerifyUserInputFile, []string{}, "", *devServiceValidateCmdUserPw)
	case devServiceLogCmd.FullCommand():
		dev.ServiceLog(*devHomeDirectory, *devServiceLogCmdServiceName, *devServiceLogCmdContainerName, *devServiceLogCmdTail)
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
		sync_service.ObjectList(*mmsOrg, *mmsUserPw, *mmsObjectListType, *mmsObjectListId, *mmsObjectListDestinationPolicy, *mmsObjectListDPService, *mmsObjectListDPProperty, *mmsObjectListDPUpdateTime, *mmsObjectListDestinationType, *mmsObjectListDestinationId, *mmsObjectListWithData, *mmsObjectListExpirationTime, *mmsObjectListDeleted, *mmsObjectListLong, *mmsObjectListDetail)
	case mmsObjectNewCmd.FullCommand():
		sync_service.ObjectNew(*mmsOrg)
	case mmsObjectPublishCmd.FullCommand():
		sync_service.ObjectPublish(*mmsOrg, *mmsUserPw, *mmsObjectPublishType, *mmsObjectPublishId, *mmsObjectPublishPat, *mmsObjectPublishDef, *mmsObjectPublishObj, *mmsObjectPublishNoChunkUpload, *mmsObjectPublishChunkUploadDataSize, *mmsObjectPublishSkipIntegrityCheck, *mmsObjectPublishDSHashAlgo, *mmsObjectPublishDSHash, *mmsObjectPublishPrivKeyFile)
	case mmsObjectDeleteCmd.FullCommand():
		sync_service.ObjectDelete(*mmsOrg, *mmsUserPw, *mmsObjectDeleteType, *mmsObjectDeleteId)
	case mmsObjectDownloadCmd.FullCommand():
		sync_service.ObjectDownLoad(*mmsOrg, *mmsUserPw, *mmsObjectDownloadType, *mmsObjectDownloadId, *mmsObjectDownloadFile, *mmsObjectDownloadOverwrite, *mmsObjectDownloadSkipIntegrityCheck)
	case mmsObjectTypesCmd.FullCommand():
		sync_service.ObjectTypes(*mmsOrg, *mmsUserPw)

	case nmAgentFilesListCmd.FullCommand():
		node_management.AgentFilesList(*nmOrg, *nmUserPw, *nmAgentFilesListType, *nmAgentFilesListVersion)
	case nmAgentFilesVersionsCmd.FullCommand():
		node_management.AgentFilesVersions(*nmOrg, *nmUserPw, *nmAgentFilesVersionsType, *nmAgentFilesVersionsVersionOnly)

	case nmManifestListCmd.FullCommand():
		node_management.ManifestList(*nmOrg, *nmUserPw, *nmManifestListId, *nmManifestListType, *nmManifestListLong)
	case nmManifestAddCmd.FullCommand():
		node_management.ManifestAdd(*nmOrg, *nmUserPw, *nmManifestAddFile, *nmManifestAddId, *nmManifestAddType, *nmManifestAddDSHashAlgo, *nmManifestAddDSHash, *nmManifestAddPrivKeyFile, *nmManifestAddSkipIntegrityCheck)
	case nmManifestNewCmd.FullCommand():
		node_management.ManifestNew()
	case nmManifestRemoveCmd.FullCommand():
		node_management.ManifestRemove(*nmOrg, *nmUserPw, *nmManifestRemoveId, *nmManifestRemoveType, *nmManifestRemoveForce)

	// node managment for local node
	case nmstatusListCmd.FullCommand():
		nm_status.List(*nmstatusListName, *nmstatusListLong)
	case nmstatusResetCmd.FullCommand():
		nm_status.Reset(*nmstatusResetName)

	// DEPRECATED (voucherInspectCmd, voucherImportCmd, voucherListCmd are deprecated commands)
	case voucherInspectCmd.FullCommand():
		sdo.DeprecatedVoucherInspect(*voucherInspectFile)
	case voucherImportCmd.FullCommand():
		sdo.DeprecatedVoucherImport(*voucherOrg, *voucherUserPw, *voucherImportFile, *voucherImportExample, *voucherImportPolicy, *voucherImportPattern)
	case voucherListCmd.FullCommand():
		sdo.DeprecatedVoucherList(*voucherOrg, *voucherUserPw, *voucherToList, !*voucherListLong)

	case sdoKeyCreateCmd.FullCommand():
		sdo.KeyCreate(*sdoOrg, *sdoUserPw, *sdoKeyCreateInputFile, *sdoKeyCreateFile, *sdoKeyCreateOverwrite)
	case sdoKeyListCmd.FullCommand():
		sdo.KeyList(*sdoOrg, *sdoUserPw, *sdoKeyToList)
	case sdoKeyDownloadCmd.FullCommand():
		sdo.KeyDownload(*sdoOrg, *sdoUserPw, *sdoKeyToDownload, *sdoKeyDownloadFile, *sdoKeyDownloadOverwrite)
	case sdoKeyRemoveCmd.FullCommand():
		sdo.KeyRemove(*sdoOrg, *sdoUserPw, *sdoKeyToRemove)
	case sdoKeyNewCmd.FullCommand():
		sdo.KeyNew(*sdoKeyNewFile, *sdoKeyNewOverwrite)
	case sdoVoucherInspectCmd.FullCommand():
		sdo.VoucherInspect(*sdoVoucherInspectFile)
	case sdoVoucherImportCmd.FullCommand():
		sdo.VoucherImport(*sdoOrg, *sdoUserPw, *sdoVoucherImportFile, *sdoVoucherImportExample, *sdoVoucherImportPolicy, *sdoVoucherImportPattern, *sdoVoucherImportUI, *sdoVoucherImportHAGroup)
	case sdoVoucherListCmd.FullCommand():
		sdo.VoucherList(*sdoOrg, *sdoUserPw, *sdoVoucherToList, !*sdoVoucherListLong)
	case sdoVoucherDownloadCmd.FullCommand():
		sdo.VoucherDownload(*sdoOrg, *sdoUserPw, *sdoVoucherDownloadDevice, *sdoVoucherDownloadFile, *sdoVoucherDownloadOverwrite)

	case fdoKeyListCmd.FullCommand():
		fdo.KeyList(*fdoOrg, *fdoUserPw, *fdoKeyToList)
	case fdoVoucherImportCmd.FullCommand():
		fdo.VoucherImport(*fdoOrg, *fdoUserPw, *fdoVoucherImportFile, *fdoVoucherImportExample, *fdoVoucherImportPolicy, *fdoVoucherImportPattern, *fdoVoucherImportUI, *fdoVoucherImportHAGroup)
	case fdoVoucherListCmd.FullCommand():
		fdo.VoucherList(*fdoOrg, *fdoUserPw, *fdoVoucherToList)
	case fdoVoucherDownloadCmd.FullCommand():
		fdo.VoucherDownload(*fdoOrg, *fdoUserPw, *fdoVoucherDownloadDevice, *fdoVoucherDownloadFile, *fdoVoucherDownloadOverwrite)

	case smSecretListCmd.FullCommand():
		secret_manager.SecretList(*smOrg, *smUserPw, *smSecretListName)
	case smSecretAddCmd.FullCommand():
		secret_manager.SecretAdd(*smOrg, *smUserPw, *smSecretAddName, *smSecretAddFile, *smSecretAddKey, *smSecretAddDetail, *smSecretAddOverwrite)
	case smSecretRemoveCmd.FullCommand():
		secret_manager.SecretRemove(*smOrg, *smUserPw, *smSecretRemoveName, *smSecretRemoveForce)
	case smSecretReadCmd.FullCommand():
		secret_manager.SecretRead(*smOrg, *smUserPw, *smSecretReadName)
	}
}

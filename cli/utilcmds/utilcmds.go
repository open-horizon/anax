package utilcmds

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"os"
)

func Sign(privKeyFilePath string) {
	stdinBytes := cliutils.ReadStdin()
	signature, err := sign.Input(privKeyFilePath, stdinBytes)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("problem signing stdin with %s: %v", privKeyFilePath, err))
	}
	fmt.Println(signature)
}

func Verify(pubKeyFilePath, signature string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	stdinBytes := cliutils.ReadStdin()
	verified, err := verify.Input(pubKeyFilePath, signature, stdinBytes)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("problem verifying deployment string with %s: %v", pubKeyFilePath, err))
	} else if !verified {
		msgPrinter.Printf("This is not a valid signature for stdin.")
		msgPrinter.Println()
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		msgPrinter.Printf("Signature is valid.")
		msgPrinter.Println()
	}
}

// convert the given json file to shell export commands and output it to stdout
func ConvertConfig(cofigFile string) {
	// get the env vars from the file
	hzn_vars, metadata_vars, err := cliconfig.GetVarsFromFile(cofigFile)
	if err != nil && !os.IsNotExist(err) {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, i18n.GetMessagePrinter().Sprintf("Failed to get the variables from configuration file %v. Error: %v", cofigFile, err))
	}

	// convert it to shell commands
	for k, v := range hzn_vars {
		fmt.Printf("export %v=%v\n", k, v)
	}
	for k, v := range metadata_vars {
		fmt.Printf("export %v=%v\n", k, v)
	}
}

package utilcmds

import (
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/rsapss-tool/sign"
	"github.com/open-horizon/rsapss-tool/verify"
	"os"
)

func Sign(privKeyFilePath string) {
	stdinBytes := cliutils.ReadStdin()
	signature, err := sign.Input(privKeyFilePath, stdinBytes)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem signing stdin with %s: %v", privKeyFilePath, err)
	}
	fmt.Println(signature)
}

func Verify(pubKeyFilePath, signature string) {
	stdinBytes := cliutils.ReadStdin()
	verified, err := verify.Input(pubKeyFilePath, signature, stdinBytes)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "problem verifying deployment string with %s: %v", pubKeyFilePath, err)
	} else if !verified {
		fmt.Println("This is not a valid signature for stdin.")
		os.Exit(cliutils.SIGNATURE_INVALID)
	} else {
		fmt.Println("Signature is valid.")
	}
}

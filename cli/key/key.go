package key

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/api"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/rsapss-tool/generatekeys"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type KeyPairSimpleOutput struct {
	ID               string `json:"id"`
	CommonName       string `json:"common_name"`
	OrganizationName string `json:"organization_name"`
	SerialNumber     string `json:"serial_number"`
	NotValidBefore   string `json:"not_valid_before"`
	NotValidAfter    string `json:"not_valid_after"`
}

type KeyList struct {
	Pem []string `json:"pem"`
}

func List(keyName string, listAll bool) {
	if keyName == "" && listAll {
		var apiOutput KeyList
		cliutils.HorizonGet("trust", []int{200}, &apiOutput, false)
		jsonBytes, err := json.MarshalIndent(apiOutput.Pem, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'key list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else if keyName == "" {
		// Getting all of the keys only returns the names
		var apiOutput map[string][]api.KeyPairSimpleRecord
		// Note: it is allowed to get /trust before post /node is called, so we don't have to check for that error
		cliutils.HorizonGet("trust?verbose=true", []int{200}, &apiOutput, false)
		cliutils.Verbose("apiOutput: %v", apiOutput)

		var output []api.KeyPairSimpleRecord
		var ok bool
		if output, ok = apiOutput["pem"]; !ok {
			cliutils.Fatal(cliutils.HTTP_ERROR, "horizon api trust output did not include 'pem' key")
		}

		certsSimpleOutput := []KeyPairSimpleOutput{}
		for _, kps := range output {
			certsSimpleOutput = append(certsSimpleOutput, KeyPairSimpleOutput{
				ID:               kps.ID,
				SerialNumber:     kps.SerialNumber,
				CommonName:       kps.SubjectNames["commonName (CN)"].(string),
				OrganizationName: kps.SubjectNames["organizationName (O)"].(string),
				NotValidBefore:   kps.NotValidBefore.String(),
				NotValidAfter:    kps.NotValidAfter.String(),
			})
		}

		jsonBytes, err := json.MarshalIndent(certsSimpleOutput, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, "failed to marshal 'key list' output: %v", err)
		}
		fmt.Printf("%s\n", jsonBytes)
	} else {
		// Get the content of 1 key, which is not json
		var apiOutput string
		cliutils.HorizonGet("trust/"+keyName, []int{200}, &apiOutput, false)
		fmt.Printf("%s", apiOutput)
	}
}

// Create generates a private/public key pair
func Create(x509Org, x509CN, outputDir string, keyLength, daysValid int, importKey bool, privKeyFile string, pubKeyFile string, overwrite bool) {

	// verify input, confirm overwrites, remove existing files, create dirs
	genDir, privKeyFile, pubKeyFile := verifyAndPrepareKeyCreateInput(outputDir, privKeyFile, pubKeyFile, overwrite)

	fmt.Println("Creating RSA PSS private and public keys, and an x509 certificate for distribution. This is a CPU-intensive operation and, depending on key length and platform, may take a while. Key generation on an amd64 or ppc64 system using the default key length will complete in less than 1 minute.")
	newKeys, err := generatekeys.Write(genDir, keyLength, x509CN, x509Org, time.Now().AddDate(0, 0, daysValid))
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to create a new key pair: %v", err)
	}

	// the created names are randomly generated, need to move them to the files if they are specified in the input
	var pubKeyName, privKeyName string
	for _, key := range newKeys {
		if strings.Contains(key, "public") { // this seems like a better check than blindly getting the 2nd key in the list
			pubKeyName = key
		} else {
			privKeyName = key
		}
	}

	// move the file to the given location.
	if outputDir == "" {
		cliutils.Verbose("Move private key file from %v to %v", privKeyName, privKeyFile)
		if err := os.Rename(privKeyName, privKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to move private key file from %v to %v. %v", privKeyName, privKeyFile, err)
		}
		cliutils.Verbose("Move public key file from %v to %v.", pubKeyName, pubKeyFile)
		if err := os.Rename(pubKeyName, pubKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "failed to move public key file from %v to %v. %v", pubKeyName, pubKeyFile, err)
		}
	} else {
		privKeyFile = privKeyName
		pubKeyFile = pubKeyName
	}
	fmt.Printf("Created keys:\n \t%v\n\t%v\n", privKeyFile, pubKeyFile)

	// Import the key to anax if they requested that
	if importKey {
		if pubKeyFile == "" {
			cliutils.Fatal(cliutils.INTERNAL_ERROR, "asked to import the created public key, but can not determine the name.")
		}
		cliutils.Verbose("Importing public key file %v to the Horizon agent.", pubKeyFile)
		Import(pubKeyFile)
		fmt.Printf("%s imported to the Horizon agent\n", pubKeyFile)
	}
}

func Import(pubKeyFile string) {
	// Note: the CLI framework already verified the file exists
	bodyBytes := cliutils.ReadFile(pubKeyFile)
	baseName := filepath.Base(pubKeyFile)
	cliutils.HorizonPutPost(http.MethodPut, "trust/"+baseName, []int{201, 200}, bodyBytes)
}

func Remove(keyName string) {
	cliutils.HorizonDelete("trust/"+keyName, []int{200, 204})
	fmt.Printf("Public key '%s' removed from the Horizon agent.\n", keyName)
}

// verify the inputs, prompt for overwrite if files exist, create direcories if not exist.
func verifyAndPrepareKeyCreateInput(outputDir string, privKeyFile string, pubKeyFile string, overwrite bool) (string, string, string) {
	if outputDir != "" {
		if privKeyFile != "" || pubKeyFile != "" {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "-d is mutually exclusive with -k and -K")
		}
	} else {
		var err error

		// get default file names if input is empty
		if privKeyFile == "" {
			if privKeyFile, err = cliutils.GetDefaultSigningKeyFile(false); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			}
		}
		if pubKeyFile == "" {
			if pubKeyFile, err = cliutils.GetDefaultSigningKeyFile(true); err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
			}
		}

		// convert to absolute path
		if privKeyFile, err = filepath.Abs(privKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "Failed to get absolute path for file %v. %v", privKeyFile, err)
		}
		if pubKeyFile, err = filepath.Abs(pubKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "Failed to get absolute path for file %v. %v", pubKeyFile, err)
		}

		// confirm overwrite
		confirmOverwrite(privKeyFile, pubKeyFile, overwrite)

		outputDir = filepath.Dir(privKeyFile)

		// create the public key directory if it does not exist and it is not the same as the private key directory
		if outputDirPub := filepath.Dir(pubKeyFile); outputDirPub != outputDir {
			if _, err := os.Stat(outputDirPub); os.IsNotExist(err) {
				cliutils.Verbose("Creating directory %v.", outputDirPub)
				if err := os.MkdirAll(outputDirPub, os.ModePerm); err != nil {
					cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
				}
			}
		}
	}

	// create the directory if it does not exist
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		cliutils.Verbose("Creating directory %v.", outputDir)
		if err := os.MkdirAll(outputDir, os.ModePerm); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		}
	}

	return outputDir, privKeyFile, pubKeyFile
}

// check if the given files exists and ask the user if they should be overwritten.
// if overwrite, delete the files.
func confirmOverwrite(privKeyFile string, pubKeyFile string, overwrite bool) {
	priveExists := false
	pubExists := false
	if fi, err := os.Stat(privKeyFile); err == nil {
		if fi.Mode().IsDir() {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v is a directory. Please specify a file name.", privKeyFile)
		}
		priveExists = true
	}
	if fi, err := os.Stat(pubKeyFile); err == nil {
		if fi.Mode().IsDir() {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v is a directory. Please specify a file name.", pubKeyFile)
		}
		pubExists = true
	}

	if priveExists && pubExists {
		if !overwrite {
			cliutils.ConfirmRemove(fmt.Sprintf("File %v and %v exist, do you want to overwrite?", privKeyFile, pubKeyFile))
		}
	} else {
		if priveExists && !overwrite {
			cliutils.ConfirmRemove(fmt.Sprintf("File %v exists, do you want to overwrite?", privKeyFile))
		}
		if pubExists && !overwrite {
			cliutils.ConfirmRemove(fmt.Sprintf("File %v exists, do you want to overwrite?", pubKeyFile))
		}
	}

	// remove the files for overwrite
	if priveExists {
		cliutils.Verbose("Deleting file %v.", privKeyFile)
		if err := os.Remove(privKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		}
	}
	if pubExists {
		cliutils.Verbose("Deleting file %v.", pubKeyFile)
		if err := os.Remove(pubKeyFile); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
		}
	}
}

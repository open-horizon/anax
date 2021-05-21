package cliconfig

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/i18n"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
)

const DEFAULT_CONFIG_FILE = "hzn.json"
const DEV_SERVICE_DEFINITION_FILE = "service.definition.json"
const DEV_DEFAULT_WORKING_DIR = "horizon"
const DEV_DEFAULT_DEPENDENCY_DIR = "dependencies"
const CONFIG_FILE_NAME = "hzn.json"

var PROJECT_CONFIG_FILE string
var PACKAGE_CONFIG_FILE string
var USER_CONFIG_FILE string

// configuration for hzn command
type HorizonCliConfig struct {

	// user usually needs to setup these, or input them with the flags for a hzn command
	HZN_EXCHANGE_USER_AUTH string `json:"HZN_EXCHANGE_USER_AUTH,omitempty"`
	HZN_EXCHANGE_NODE_AUTH string `json:"HZN_EXCHANGE_NODE_AUTH,omitempty"`
	HZN_ORG_ID             string `json:"HZN_ORG_ID,omitempty"`

	// the locale that the hzn cli will run under, for example pt-BR, es, fr, de, it, ja, ko, zh-CN, zh-TW.
	HZN_LANG string `json:"HZN_LANG,omitempty"`

	// the url to the horizon agent, the default is "http://localhost:8510" for linux and "http://localhost:8081" for mac
	HORIZON_URL string `json:"HORIZON_URL,omitempty"`

	// exchange url, the default is shipped with the horizon-cli package
	HZN_EXCHANGE_URL string `json:"HZN_EXCHANGE_URL,omitempty"`

	// http max retries and retry interval (in second) when transport error occurs.
	HZN_HTTP_RETRIES        string `json:"HZN_HTTP_RETRIES,omitempty"`
	HZN_HTTP_RETRY_INTERVAL string `json:"HZN_HTTP_RETRY_INTERVAL,omitempty"`

	// the CSS url, the default is shipped with the horizon-cli package
	HZN_FSS_CSSURL string `json:"HZN_FSS_CSSURL,omitempty"`

	// the agbot secure api url, the default is shipped with the horizon-cli package
	HZN_AGBOT_URL string `json:"HZN_AGBOT_URL,omitempty"`

	// keys to sign the service or pattern, the default is ~/.hzn/keys/service.private.key and ~/.hzn/keys/service.public.pem
	HZN_PRIVATE_KEY_FILE string `json:"HZN_PRIVATE_KEY_FILE,omitempty"`
	HZN_PUBLIC_KEY_FILE  string `json:"HZN_PUBLIC_KEY_FILE,omitempty"`

	// if the env variables in a file will be substituted or not
	//HZN_DONT_SUBST_ENV_VARS string `json:"HZN_DONT_SUBST_ENV_VARS,omitempty"`

	// if the user auth or the node auth is an api key or not
	//USING_API_KEY string `json:"USING_API_KEY,omitempty"`

	// the following are only used by 'hzn dev' commands
	HZN_DEVICE_ID           string `json:"HZN_DEVICE_ID,omitempty"`
	HZN_PATTERN             string `json:"HZN_PATTERN,omitempty"`
	HZN_DEV_FSS_IMAGE_REPO  string `json:"HZN_DEV_FSS_IMAGE_REPO,omitempty"`
	HZN_DEV_FSS_IMAGE_TAG   string `json:"HZN_DEV_FSS_IMAGE_TAG,omitempty"`
	HZN_DEV_FSS_CSS_PORT    string `json:"HZN_DEV_FSS_CSS_PORT,omitempty"`
	HZN_DEV_FSS_MONGO_IMAGE string `json:"HZN_DEV_FSS_MONGO_IMAGE,omitempty"`
	HZN_DEV_FSS_WORKING_DIR string `json:"HZN_DEV_FSS_WORKING_DIR,omitempty"`

	// the timeout variable for calls to the node that occur during registration
	HZN_REGISTER_HTTP_TIMEOUT string `json:"HZN_REGISTER_HTTP_TIMEOUT,omitempty"`

	// used to substitute the env variables in a file
	MetadataVars map[string]string `json:"MetadataVars,omitempty"`
}

// get the config from the given file. Assume file exists.
func GetConfig(configFile string) (*HorizonCliConfig, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	cliutils.Verbose(msgPrinter.Sprintf("Reading configuration file: %v", configFile))

	fileBytes, err := ioutil.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Unable to read config file: %v. %v", configFile, err))
	}

	// Remove /* */ comments
	re := regexp.MustCompile(`(?s)/\*.*?\*/`)
	newBytes := re.ReplaceAll(fileBytes, nil)

	config := HorizonCliConfig{}
	if err := json.Unmarshal(newBytes, &config); err != nil {
		return nil, fmt.Errorf(msgPrinter.Sprintf("Unable to decode content of config file %v. %v", configFile, err))
	} else {
		return &config, nil
	}
}

// returns the hzn level env vars and metadata vars from the config in the map format
func GetVarsFromConfig(config *HorizonCliConfig) (map[string]string, map[string]string) {
	hzn_vars := map[string]string{}
	metadata_vars := map[string]string{}

	fields := reflect.TypeOf(*config)
	values := reflect.ValueOf(*config)

	for i := 0; i < fields.NumField(); i++ {
		field := fields.Field(i)
		value := values.Field(i)
		if field.Name == "MetadataVars" {
			for _, k := range value.MapKeys() {
				metadata_vars[k.String()] = value.MapIndex(k).String()
			}
		} else {
			if value.String() != "" {
				hzn_vars[field.Name] = value.String()
			}
		}
	}

	return hzn_vars, metadata_vars
}

// returns the hzn level env vars and metadata vars from the config file
func GetVarsFromFile(configFile string) (map[string]string, map[string]string, error) {
	hzn_vars := map[string]string{}
	metadata_vars := map[string]string{}

	if _, err := os.Stat(configFile); err != nil {
		cliutils.Verbose(i18n.GetMessagePrinter().Sprintf("Config file does not exist: %v.", configFile))

		// return no error here because the file does not exists.
		return hzn_vars, metadata_vars, nil
	}
	if config, err := GetConfig(configFile); err != nil {
		return hzn_vars, metadata_vars, err
	} else {
		hzn_vars, metadata_vars = GetVarsFromConfig(config)
	}

	return hzn_vars, metadata_vars, nil
}

// set up the environment variables from the given config file.
// skip the ones that's alrady there originally if override_env is false
// it returns the hzn env vars and metadata env vars from the given file
func SetEnvVarsFromConfigFile(configFile string, orig_env_vars map[string]string, override_env bool) (map[string]string, map[string]string, error) {
	hzn_vars := map[string]string{}
	metadata_vars := map[string]string{}

	hzn_vars, metadata_vars, err := GetVarsFromFile(configFile)
	if err != nil {
		return hzn_vars, metadata_vars, err
	}

	if err := SetEnvVars(metadata_vars, orig_env_vars, override_env); err != nil {
		return hzn_vars, metadata_vars, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Failed to set the environment variable defined in the MetadataVars attribute in file %v. %v", configFile, err))
	}
	if err := SetEnvVars(hzn_vars, orig_env_vars, override_env); err != nil {
		return hzn_vars, metadata_vars, fmt.Errorf(i18n.GetMessagePrinter().Sprintf("Failed to set the environment variable in top level in file %v. %v", configFile, err))
	}
	return hzn_vars, metadata_vars, nil
}

// set up the environment variables from the given non-jsonfile.
// skip the ones that's alrady there originally if override_env is false
// it returns the hzn env vars from the given file
func SetEnvVarsFromNonJsonFile(configFile string, orig_env_vars map[string]string, override_env bool) (map[string]string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	hzn_vars := map[string]string{}

	if _, err := os.Stat(configFile); err != nil {
		cliutils.Verbose(msgPrinter.Sprintf("Config file does not exist: %v.", configFile))

		// return no error here because the file does not exists.
		return hzn_vars, nil
	}

	cliutils.Verbose(msgPrinter.Sprintf("Reading configuration file: %v", configFile))

	// read the configuration from the file
	hzn_vars, err := GetConfigFromNonJsonFile(configFile)
	if err != nil {
		return nil, err
	}

	// set the env variables
	if err := SetEnvVars(hzn_vars, orig_env_vars, override_env); err != nil {
		return hzn_vars, fmt.Errorf(msgPrinter.Sprintf("Failed to set the environment variable defined in file %v. %v", configFile, err))
	}

	return hzn_vars, nil
}

func GetConfigFromNonJsonFile(configFile string) (map[string]string, error) {
	hzn_vars := map[string]string{}

	fileHandle, err := os.Open(configFile)
	if err != nil {
		return hzn_vars, err
	}
	defer fileHandle.Close()

	// regex for comment line
	r, _ := regexp.Compile(`^(\s)*#(.*)*`)

	scanner := bufio.NewScanner(fileHandle)
	for scanner.Scan() {
		// skip the comment line
		if r.MatchString(scanner.Text()) {
			continue
		}

		// now handle the normal line
		a := strings.Split(scanner.Text(), "=")
		if len(a) > 0 {
			key := strings.TrimSpace(a[0])
			value := ""
			if len(a) > 1 {
				value = strings.TrimSpace(a[1])
			}
			if key != "" {
				hzn_vars[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return hzn_vars, err
	}

	return hzn_vars, nil
}

func SetEnvVars(new_env_vars, orig_env_vars map[string]string, override bool) error {
	for k, v := range new_env_vars {
		if override {
			if err := os.Setenv(k, v); err != nil {
				return err
			}
		} else {
			if _, found := orig_env_vars[k]; !found {
				if err := os.Setenv(k, v); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// get all the env vars and convert them to a map for easy referencing later.
func GetEnvVars() map[string]string {
	ret_map := map[string]string{}
	for _, pairs := range os.Environ() {
		parts := strings.SplitN(pairs, "=", 2)
		if len(parts) == 2 {
			ret_map[parts[0]] = parts[1]
		}
	}

	return ret_map
}

// Check for file existence and return any errors.
// Treat error as file does not exists because os.Stat() only returns PathError.
func FileExists(directory string, fileName string) bool {
	filePath := filepath.Join(directory, fileName)
	if _, err := os.Stat(filePath); err != nil {
		return false
	} else {
		return true
	}
}

// check the project's configuration file, it could be under current directory or the ./horizon directory
// or ../ directory if current directory is dependecies.
// it is usually setup by 'hzn dev service create' command
func GetProjectConfigFile(dir string) (string, error) {

	// look for service.definition.json file under current dir
	if found := FileExists(dir, DEV_SERVICE_DEFINITION_FILE); found {
		return filepath.Join(dir, DEFAULT_CONFIG_FILE), nil
	}

	// look service.definition.json file under horizon dir
	if found := FileExists(filepath.Join(dir, DEV_DEFAULT_WORKING_DIR), DEV_SERVICE_DEFINITION_FILE); found {
		return filepath.Join(dir, DEV_DEFAULT_WORKING_DIR, DEFAULT_CONFIG_FILE), nil
	}

	// look service.definition.json file under dir above if current dir is dependencies
	path := filepath.Clean(dir)
	if filepath.Base(path) == DEV_DEFAULT_DEPENDENCY_DIR {
		if found := FileExists(filepath.Dir(path), DEV_SERVICE_DEFINITION_FILE); found {
			return filepath.Join(filepath.Dir(path), DEFAULT_CONFIG_FILE), nil
		}
	}

	return "", nil
}

// restore the env vars to its original state after setting up hzn_vars and metadata_vars
func RestoreEnvVars(orig_env_vars, hzn_vars, metadata_vars map[string]string) error {
	for k, _ := range hzn_vars {
		if v_orig, found := orig_env_vars[k]; found {
			if err := os.Setenv(k, v_orig); err != nil {
				return err
			}
		} else {
			if err := os.Unsetenv(k); err != nil {
				return err
			}

		}
	}
	for k, _ := range metadata_vars {
		if v_orig, found := orig_env_vars[k]; found {
			if err := os.Setenv(k, v_orig); err != nil {
				return err
			}
		} else {
			if err := os.Unsetenv(k); err != nil {
				return err
			}
		}
	}

	return nil
}

// ReadJsonFile reads json from a file or stdin, eliminates comments,
// setup env vars from local hzn.json configuration file if any, substitutes env vars, and returns it.
func ReadJsonFileWithLocalConfig(filePath string) []byte {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	inputFile, err := filepath.Abs(filePath)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "%v", err)
	}
	localConfigFile := filepath.Join(filepath.Dir(inputFile), "hzn.json")
	localConfigFile = filepath.Clean(localConfigFile)

	// no local configuration file
	if _, err := os.Stat(localConfigFile); err == nil {
		return cliutils.ReadJsonFile(filePath)
	}

	// check if the local configuration file has been used already
	useLocalConfig := true
	if localConfigFile == PROJECT_CONFIG_FILE || localConfigFile == PACKAGE_CONFIG_FILE || localConfigFile == USER_CONFIG_FILE {
		useLocalConfig = false
		cliutils.Verbose(msgPrinter.Sprintf("Local configuration %v has been setup at the beginning of this command. Will not setup twice.", localConfigFile))
	}

	orig_env_vars := map[string]string{}
	hzn_vars := map[string]string{}
	metadata_vars := map[string]string{}
	if useLocalConfig {
		orig_env_vars = GetEnvVars()
		hzn_vars, metadata_vars, err = SetEnvVarsFromConfigFile(localConfigFile, orig_env_vars, false)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to set the environment variable from the local configuration file %v for file %v. Error: %v", localConfigFile, filePath, err))
		}
	}

	contents := cliutils.ReadJsonFile(filePath)

	if useLocalConfig {
		// restore the env vars
		err = RestoreEnvVars(orig_env_vars, hzn_vars, metadata_vars)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to restore the environment variable after using local configuration file %v. %v", useLocalConfig, err))
		}
	}

	return contents
}

// set up the environment variables from the config files.
// the precedence order is: environmental variables, user config file, package config file
func SetEnvVarsFromConfigFiles(project_dir string) error {
	var err error

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get original env vars
	orig_env_vars := GetEnvVars()

	// check the /etc/horiozn/hzn.json file that ships with the horizon-cli package
	default_config_file_dir := "/etc/horizon"
	if runtime.GOOS == "darwin" {
		default_config_file_dir = "/usr/local/etc/horizon"
	}
	configFile_pkg := filepath.Join(default_config_file_dir, DEFAULT_CONFIG_FILE)
	if configFile_pkg, err = filepath.Abs(configFile_pkg); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to get the absolute path for file %v. %v", configFile_pkg, err))
	}
	_, _, err = SetEnvVarsFromConfigFile(configFile_pkg, orig_env_vars, false)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Error reading environment variables from file %v. %v", configFile_pkg, err))
	} else {
		PACKAGE_CONFIG_FILE = filepath.Clean(configFile_pkg)
	}

	// check /etc/default/horizon file that ships with horizon package
	_, err = SetEnvVarsFromNonJsonFile("/etc/default/horizon", orig_env_vars, false)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Error reading environment variables from file /etc/default/horizon. %v", err))
	}

	// check the user's configuration file  ~/.hzn/hzn.json
	configFile_user := filepath.Join(os.Getenv("HOME"), ".hzn", DEFAULT_CONFIG_FILE)
	if configFile_user, err = filepath.Abs(configFile_user); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to get the absolute path for file ~/.hzn/hzn.json. %v", err))
	}
	if configFile_user != configFile_pkg {
		_, _, err = SetEnvVarsFromConfigFile(configFile_user, orig_env_vars, false)
		if err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Error reading environment variables from file %v. %v", configFile_user, err))
		} else {
			USER_CONFIG_FILE = filepath.Clean(configFile_user)
		}
	}

	return nil

}

// set up the environment variables from the project config files.
// the precedence order is: environmental variables, project config file
func SetEnvVarsFromProjectConfigFile(project_dir string) error {
	var err error

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// get original env vars
	orig_env_vars := GetEnvVars()

	// check the project's configuration file, it could be under current directory or the ./horizon directory.
	// it is usually setup by 'hzn dev service create' command
	configFile_project, err := GetProjectConfigFile(project_dir)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Error getting project level configuration file name. %v", err))
	}
	if configFile_project == "" {
		cliutils.Verbose(msgPrinter.Sprintf("No project level configuration file found."))
	} else {
		if configFile_project, err = filepath.Abs(configFile_project); err != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Failed to get the absolute path for file %v. %v", configFile_project, err))
		}
		if configFile_project != PACKAGE_CONFIG_FILE && configFile_project != USER_CONFIG_FILE {
			_, _, err = SetEnvVarsFromConfigFile(configFile_project, orig_env_vars, false)
			if err != nil {
				cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("Error reading environment variables from file %v. %v", configFile_project, err))
			} else {
				PROJECT_CONFIG_FILE = filepath.Clean(configFile_project)
			}
		}
	}

	return nil
}

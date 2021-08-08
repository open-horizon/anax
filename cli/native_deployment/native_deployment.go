package native_deployment

import (
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cli/dev"
	"github.com/open-horizon/anax/cli/plugin_registry"
	"github.com/open-horizon/anax/cli/sync_service"
	"github.com/open-horizon/anax/common"
	"github.com/open-horizon/anax/config"
	"github.com/open-horizon/anax/containermessage"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/i18n"
	"github.com/open-horizon/rsapss-tool/sign"
)

func init() {
	plugin_registry.Register("native", NewNativeDeploymentConfigPlugin())
}

type NativeDeploymentConfigPlugin struct {
}

func NewNativeDeploymentConfigPlugin() plugin_registry.DeploymentConfigPlugin {
	return new(NativeDeploymentConfigPlugin)
}

func (p *NativeDeploymentConfigPlugin) Sign(dep map[string]interface{}, privKey *rsa.PrivateKey, ctx plugin_registry.PluginContext) (bool, string, string, error) {

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if owned, err := p.Validate(dep, nil); !owned || err != nil {
		return owned, "", "", err
	}

	// Since the deployment config has been validated as ours, we can assume it is structured correctly.
	services := dep["services"].(map[string]interface{})
	var dontTouchImage, pullImage, ok bool
	dontTouchImage, ok = (ctx.Get("dontTouchImage")).(bool)
	if !ok {
		dontTouchImage = false
	}
	pullImage, ok = (ctx.Get("pullImage")).(bool)
	if !ok {
		pullImage = false
	}

	for _, svc := range services {
		service := svc.(map[string]interface{})
		image := service["image"].(string)

		newImage := cliutils.GetNewDockerImageName(image, dontTouchImage, pullImage)
		if newImage != image {
			msgPrinter.Printf("Using '%s' in 'deployment' field instead of '%s'", newImage, image)
			msgPrinter.Println()
			service["image"] = newImage
		}
	}

	// Now that we have uploaded images and possibly modified the deployment config, we can stringify it and sign it.
	// Convert the deployment field from map[string]interface{} to []byte (i think treating it as type DeploymentConfig is too inflexible for future additions)
	deployment, err := json.Marshal(dep)
	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("failed to marshal deployment string %v, error %v", dep, err))
	}
	depStr := string(deployment)

	hasher := sha256.New()
	_, err = hasher.Write(deployment)
	if err != nil {
		return true, "", "", err
	}
	sig, err := sign.Sha256HashOfInput(privKey, hasher)

	if err != nil {
		return true, "", "", errors.New(msgPrinter.Sprintf("problem signing deployment string: %v", err))
	}

	return true, depStr, sig, nil
}

func (p *NativeDeploymentConfigPlugin) GetContainerImages(dep interface{}) (bool, []string, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var imageList []string
	if owned, err := p.Validate(dep, nil); !owned || err != nil {
		return owned, imageList, err
	}

	depConfig, err := common.ConvertToDeploymentConfig(dep, msgPrinter)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err.Error())
	}

	for _, svc := range depConfig.Services {
		imageList = append(imageList, svc.Image)
	}

	return true, imageList, nil
}

// Given a map of image name and image pairs, the function returns a very simple deployment configuration to be used in the service definition.
func (p *NativeDeploymentConfigPlugin) DefaultConfig(imageInfo interface{}) interface{} {
	imageList := make(map[string]string, 0)
	if imageInfo != nil {
		imageList = imageInfo.(map[string]string)
	}

	if len(imageList) == 0 {
		return map[string]interface{}{
			"services": map[string]*containermessage.Service{
				"": &containermessage.Service{
					Image:   "",
					Secrets: map[string]containermessage.Secret{},
				},
			},
		}
	} else {
		serviceDep := make(map[string]*containermessage.Service, len(imageList))
		for image_name, image := range imageList {
			serviceDep[image_name] = &containermessage.Service{
				Image:   image,
				Secrets: map[string]containermessage.Secret{},
			}
		}
		return map[string]interface{}{"services": serviceDep}
	}
}

// Return the default cluster config object, which is nil in this case.
func (p *NativeDeploymentConfigPlugin) DefaultClusterConfig() interface{} {
	return nil
}

func (p *NativeDeploymentConfigPlugin) Validate(dep interface{}, cdep interface{}) (bool, error) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if dc, ok := dep.(map[string]interface{}); !ok {
		return false, nil
	} else if s, ok := dc["services"]; !ok {
		return false, nil
	} else if services, ok := s.(map[string]interface{}); !ok {
		return false, nil
	} else {
		depConfig, err1 := common.ConvertToDeploymentConfig(dep, msgPrinter)
		if err1 != nil {
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, err1.Error())
		}

		if err := depConfig.CanStartStop(); err != nil {
			return true, err
		}
		for k, svc := range services {
			switch s := svc.(type) {
			case map[string]interface{}:
				if err := CheckDeploymentService(k, s); err != nil {
					return true, err
				}
			default:
				return true, errors.New(i18n.GetMessagePrinter().Sprintf("each service defined under 'deployment.services' must be a json object (with strings as the keys)"))
			}
		}
		return true, nil
	}

}

// This can't be a const because a map literal isn't a const in go
var VALID_DEPLOYMENT_FIELDS = map[string]int8{"image": 1, "privileged": 1, "cap_add": 1, "environment": 1, "devices": 1, "binds": 1, "specific_ports": 1, "command": 1, "ports": 1, "ephemeral_ports": 1, "tmpfs": 1, "network": 1, "entrypoint": 1, "max_memory_mb": 1, "max_cpus": 1, "log_driver": 1, "secrets": 1}

// CheckDeploymentService verifies it has the required 'image' key, and checks for keys we don't recognize.
// For now it only prints a warning for unrecognized keys, in case we recently added a key to anax and haven't updated hzn yet.
// It also checks for invalid use of the default anax port, and puts out a warning message.
func CheckDeploymentService(svcName string, depSvc map[string]interface{}) error {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if _, ok := depSvc["image"]; !ok {
		return errors.New(msgPrinter.Sprintf("service '%s' defined under 'deployment.services' does not have mandatory 'image' field", svcName))
	}

	// Check the rest of the keys for unrecognized ones
	for k := range depSvc {
		if _, ok := VALID_DEPLOYMENT_FIELDS[k]; !ok {
			cliutils.Warning(msgPrinter.Sprintf("service '%s' defined under 'deployment.services' has unrecognized field '%s'. See https://github.com/open-horizon/anax/blob/master/doc/deployment_string.md", svcName, k))
		}

		// Check for the use of the default agent API port, which will cause a port conflict at runtime.
		if k == "ports" {
			// Marshal and unmarshal the ports deployment config so that we can reuse typed APIs for parsing the host port
			// references out of the ports config value.
			var pbs []dockerclient.PortBinding
			if bytes, err := json.Marshal(depSvc[k]); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("service '%s' defined under 'deployment.services' has a malformed ports value %v, error %v", svcName, depSvc[k], err))
			} else if err := json.Unmarshal(bytes, &pbs); err != nil {
				cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("service '%s' defined under 'deployment.services' has a malformed ports value %v, error %v", svcName, string(bytes), err))
			} else {
				for _, pb := range pbs {
					hostPort := containermessage.GetSpecificHostPort(pb.HostPort)
					if hostPort != "" && hostPort == config.AnaxAPIPortDefault {
						cliutils.Warning(msgPrinter.Sprintf("service '%s' defined under 'deployment.services' is mapping a port %v to the default Horizon API port %v. This service will fail when deployed on a node which has not changed the default Horizon API port.", svcName, pb, config.AnaxAPIPortDefault))
					}
				}
			}
		}
	}
	return nil
}

// SignImagesFromDeploymentMap finds the images in this deployment structure (if any) and appends them to the imageList
func SignImagesFromDeploymentMap(deployment map[string]interface{}, dontTouchImage bool) (imageList []string) {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// The deployment string should include: {"services":{"cpu2wiotp":{"image":"openhorizon/example_wl_x86_cpu2wiotp:1.1.2",...}}}
	// Since we have to parse the deployment structure anyway, we do some validity checking while we are at it
	// Note: in the code below we are exploiting the golang map feature that it returns the zero value when a key does not exist in the map.
	if len(deployment) == 0 {
		return imageList // an empty deployment structure is valid
	}
	var client *dockerclient.Client

	if _, ok := deployment["chart_archive"]; ok {
		// TODO: come back and fill this in.
		return

	} else if _, ok := deployment["services"]; ok {
		switch services := deployment["services"].(type) {
		case map[string]interface{}:
			for k, svc := range services {
				switch s := svc.(type) {
				case map[string]interface{}:
					if err := CheckDeploymentService(k, s); err != nil {
						cliutils.Fatal(cliutils.CLI_INPUT_ERROR, "%v", err)
					}
					switch image := s["image"].(type) {
					case string:
						domain, path, tag, digest := cutil.ParseDockerImagePath(image)
						cliutils.Verbose(msgPrinter.Sprintf("%s parsed into: domain=%s, path=%s, tag=%s", image, domain, path, tag))
						if path == "" {
							msgPrinter.Printf("Warning: could not parse image path '%v'. Not pushing it to a docker registry, just including it in the 'deployment' field as-is.", image)
							msgPrinter.Println()
						} else if digest == "" {
							// This image has a tag, or default tag
							if dontTouchImage {
								imageList = append(imageList, image)
							} else {
								// Push it, get the repo digest, and modify the imagePath to use the digest
								if client == nil {
									client = cliutils.NewDockerClient()
								}
								digest := cliutils.PushDockerImage(client, domain, path, tag) // this will error out if the push fails or can't get the digest
								if domain != "" {
									domain = domain + "/"
								}
								newImage := domain + path + "@" + digest
								msgPrinter.Printf("Using '%s' in 'deployment' field instead of '%s'", newImage, image)
								msgPrinter.Println()
								s["image"] = newImage
							}
						}
					}
				default:
					cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("each service defined under 'deployment.services' must be a json object (with strings as the keys)"))
				}
			}
		default:
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the 'deployment' field must contain the 'services' field, whose value must be a json object (with strings as the keys)"))
		}
	} else {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("the 'deployment' field must contain either the native Horizon deployment config or the Helm deployment config, whose value must be a json object (with strings as the keys)"))
	}
	return
}

// Start the native deployment config in test mode. Only services are supported.
func (p *NativeDeploymentConfigPlugin) StartTest(homeDirectory string, userInputFile string, configFiles []string, configType string, noFSS bool, userCreds string, secretsFiles map[string]string) bool {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Run verification before trying to start anything.
	absConfigFiles := dev.ServiceValidate(homeDirectory, userInputFile, configFiles, configType, userCreds)

	// Perform the common execution setup.
	dir, userInputs, cw := dev.CommonExecutionSetup(homeDirectory, userInputFile, dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND)

	// Get the service definition, so that we can look at the user input variable definitions.
	serviceDef, sderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if sderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sderr)
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.Deployment, nil); !owned || err != nil {
		return false
	}

	if !noFSS {
		// Start the file sync service infrastructure containers so the services can use it in test mode.
		sserr := sync_service.Start(cw, serviceDef.Org, absConfigFiles, configType)
		if sserr != nil {
			sync_service.Stop(cw.GetClient())
			cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to start file sync service, %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sserr))
		}
	}

	// Get the metadata for each dependency. The metadata is returned as a list of service definition files from
	// the project's dependency directory.
	deps, derr := dev.GetServiceDependencies(dir, serviceDef.RequiredServices)
	if derr != nil {
		if !noFSS {
			sync_service.Stop(cw.GetClient())
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to get service dependencies, %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, derr))
	}

	dev.AddDependentServiceSecretBinds(deps, secretsFiles)

	// Log the starting of dependencies if there are any.
	if len(deps) != 0 {
		cliutils.Verbose(msgPrinter.Sprintf("Starting dependencies."))
	}

	// Generate an agreement id for testing purposes.
	agreementId, aerr := cutil.GenerateAgreementId()
	if aerr != nil {
		if !noFSS {
			sync_service.Stop(cw.GetClient())
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to generate test agreementId, %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, aerr))
	}

	// If the service has dependencies, get them started first.
	msNetworks, perr := dev.ProcessStartDependencies(dir, deps, userInputs.Global, userInputs.Services, cw, agreementId)
	if perr != nil {
		if !noFSS {
			sync_service.Stop(cw.GetClient())
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to start service dependencies, %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, perr))
	}

	// Get the service's deployment description from the deployment config in the definition.
	dc, deployment, cerr := serviceDef.ConvertToDeploymentDescription(true, msgPrinter)
	if cerr != nil {
		if !noFSS {
			sync_service.Stop(cw.GetClient())
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, cerr)
	}

	dev.AddTopLevelServiceSecretBinds(deployment, secretsFiles)

	// Now we can start the service container.
	_, err := dev.StartContainers(deployment, serviceDef.URL, userInputs.Global, serviceDef.UserInputs, userInputs.Services, serviceDef.Org, dc, cw, msNetworks, true, true, agreementId)
	if err != nil {
		if !noFSS {
			sync_service.Stop(cw.GetClient())
		}
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v.", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, err)
	}

	return true
}

// Stop the native deployment config in test mode. Only services are supported.
func (p *NativeDeploymentConfigPlugin) StopTest(homeDirectory string) bool {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// Perform the common execution setup.
	dir, _, cw := dev.CommonExecutionSetup(homeDirectory, "", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND)

	// Get the service definition for this project.
	serviceDef, wderr := dev.GetServiceDefinition(dir, dev.SERVICE_DEFINITION_FILE)
	if wderr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND, wderr)
	}

	// Now that we have the service def, we can check if we own the deployment config object.
	if owned, err := p.Validate(serviceDef.Deployment, nil); !owned || err != nil {
		return false
	}

	// Get the deployment config. This is a top-level service because it's the one being launched, so it is treated as
	// if it is managed by an agreement.
	dc, _, cerr := serviceDef.ConvertToDeploymentDescription(true, msgPrinter)
	if cerr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND, cerr)
	}

	// Stop the service.
	err := dev.StopService(dc, cw)
	if err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, "'%v %v' %v", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND, err)
	}

	// Get the metadata for each dependency. The metadata is returned as a list of service definition files from
	// the project's dependency directory.
	deps, derr := dev.GetServiceDependencies(dir, serviceDef.RequiredServices)
	if derr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to get service dependencies, %v", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND, derr))
	}

	// If the service has dependencies, stop them.
	if err := dev.ProcessStopDependencies(dir, deps, cw); err != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to stop service dependencies, %v", dev.SERVICE_COMMAND, dev.SERVICE_STOP_COMMAND, err))
	}

	// Perform the execution teardown.
	dev.ExecutionTearDown(cw)

	// Stop the file sync service infrastructure containers if any now that the service(s) are stopped.
	sserr := sync_service.Stop(cw.GetClient())
	if sserr != nil {
		cliutils.Fatal(cliutils.CLI_GENERAL_ERROR, msgPrinter.Sprintf("'%v %v' unable to stop file sync service, %v", dev.SERVICE_COMMAND, dev.SERVICE_START_COMMAND, sserr))
	}

	msgPrinter.Printf("Stopped service.")
	msgPrinter.Println()
	return true
}

package validations

import (
	"fmt"
	"os/exec"
	"regexp"

	"github.com/Azure/draft/pkg/config"
	"github.com/Azure/draft/pkg/providers"

	log "github.com/sirupsen/logrus"
)

func Validate(name string, variable config.BuilderVar, value string) error {
	switch variable.ValidateType {
	case "azContainerRegistry":
		return validateAzureContainerRegistry(value)
	case "azClusterName":
		return validateAzureClusterName(value)
	case "azResourceGroup":
		return validateAzureResourceGroup(value)
	case "containerName":
		return validateContainerName(value)
	case "dir":
		return validateDir(value)
	case "ghBranch":
		return validateGitHubBranch(value)
	}
	log.Debugf("no validation rule found for %s with ValidateType of %s", name, variable.ValidateType)
	return nil
}

func validateAzureContainerRegistry(value string) error {
	providers.CheckAzCliInstalled()
	if !providers.IsLoggedInToAz() {
		if err := providers.LogInToAz(); err != nil {
			return fmt.Errorf("failed to log in to Azure CLI: %v", err)
		}
	}

	// Regex to check Azure Container Registry naming requirements
	regexPattern := `^[a-z0-9]{5,50}$`
	matched, err := regexp.MatchString(regexPattern, value)
	if err != nil {
		return fmt.Errorf("regex error: %v", err)
	}
	if !matched {
		return fmt.Errorf("registry name '%s' does not meet Azure Container Registry naming requirements", value)
	}

	getAccountCmd := exec.Command("az", "acr", "show", "--name", value)
	_, err = getAccountCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to find Azure Container Registry %s: %v", value, err)
	}

	return nil
}

func validateAzureClusterName(value string) error {
	return nil
}

func validateAzureResourceGroup(value string) error {
	return nil
}

func validateContainerName(value string) error {
	return nil
}

func validateDir(value string) error {
	return nil
}

func validateGitHubBranch(value string) error {
	return nil
}

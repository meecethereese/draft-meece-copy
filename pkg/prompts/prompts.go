package prompts

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"

	"github.com/Azure/draft/pkg/config"
	"github.com/Azure/draft/pkg/providers"
	"github.com/Azure/draft/pkg/validations"
)

func RunPromptsFromConfig(config *config.DraftConfig) (map[string]string, error) {
	return RunPromptsFromConfigWithSkips(config, []string{})
}

func RunPromptsFromConfigWithSkips(config *config.DraftConfig, varsToSkip []string) (map[string]string, error) {
	return RunPromptsFromConfigWithSkipsIO(config, varsToSkip, nil, nil)
}

// RunPromptsFromConfigWithSkipsIO runs the prompts for the given config
// skipping any variables in varsToSkip or where the BuilderVar.IsPromptDisabled is true.
// If Stdin or Stdout are nil, the default values will be used.
func RunPromptsFromConfigWithSkipsIO(config *config.DraftConfig, varsToSkip []string, Stdin io.ReadCloser, Stdout io.WriteCloser) (map[string]string, error) {
	skipMap := make(map[string]interface{})
	for _, v := range varsToSkip {
		skipMap[v] = interface{}(nil)
	}

	inputs := make(map[string]string)

	for name, variable := range config.Variables {
		if val, ok := skipMap[name]; ok && val != "" {
			log.Debugf("Skipping prompt for %s", name)
			continue
		}

		if variable.IsPromptDisabled {
			log.Debugf("Skipping prompt for %s as it has IsPromptDisabled=true", name)
			noPromptDefaultValue := GetVariableDefaultValue(name, variable, inputs)
			if noPromptDefaultValue == "" {
				return nil, fmt.Errorf("IsPromptDisabled is true for %s but no default value was found", name)
			}
			log.Debugf("Using default value %s for %s", noPromptDefaultValue, name)
			inputs[name] = noPromptDefaultValue
			continue
		}

		log.Debugf("constructing prompt for: %s", name)
		if variable.Type == "bool" {
			input, err := RunBoolPrompt(variable, Stdin, Stdout)
			if err != nil {
				return nil, err
			}
			inputs[name] = input
		} else {
			defaultValue := GetVariableDefaultValue(name, variable, inputs)

			stringInput, err := RunDefaultableStringPrompt(variable, defaultValue, nil, Stdin, Stdout)
			if err != nil {
				return nil, err
			}
			inputs[name] = stringInput
		}

		err := validations.Validate(name, variable, inputs[name])
	}

	return inputs, nil
}

// GetVariableDefaultValue returns the default value for a variable, if one is set in variableDefaults from a ReferenceVar or literal VariableDefault.Value in that order.
func GetVariableDefaultValue(variableName string, variable config.BuilderVar, inputs map[string]string) string {
	defaultValue := ""

	defaultValue = variable.Value
	log.Debugf("setting default value for %s to %s from variable default rule", variableName, defaultValue)
	if variable.ReferenceVar != "" && inputs[variable.ReferenceVar] != "" {
		defaultValue = inputs[variable.ReferenceVar]
		log.Debugf("setting default value for %s to %s from referenceVar %s", variableName, defaultValue, variable.ReferenceVar)
	}

	return defaultValue
}

func RunBoolPrompt(customPrompt config.BuilderVar, Stdin io.ReadCloser, Stdout io.WriteCloser) (string, error) {
	newSelect := &promptui.Select{
		Label:  "Please select " + customPrompt.Description,
		Items:  []bool{true, false},
		Stdin:  Stdin,
		Stdout: Stdout,
	}

	_, input, err := newSelect.Run()
	if err != nil {
		return "", err
	}
	return input, nil
}

// AllowAllStringValidator is a string validator that allows any string
func AllowAllStringValidator(_ string) error {
	return nil
}

// NoBlankStringValidator is a string validator that does not allow blank strings
func NoBlankStringValidator(s string) error {
	if len(s) <= 0 {
		return fmt.Errorf("input must be greater than 0")
	}
	return nil
}

// RunDefaultableStringPrompt runs a prompt for a string variable, returning the user string input for the prompt
func RunDefaultableStringPrompt(customPrompt config.BuilderVar, defaultValue string, validate func(string) error, Stdin io.ReadCloser, Stdout io.WriteCloser) (string, error) {
	var validatorFunc func(string) error
	if validate == nil {
		validatorFunc = NoBlankStringValidator
	}

	defaultString := ""
	if defaultValue != "" {
		validatorFunc = AllowAllStringValidator
		defaultString = " (default: " + defaultValue + ")"
	}

	prompt := &promptui.Prompt{
		Label:    "Please enter " + customPrompt.Description + defaultString,
		Validate: validatorFunc,
		Stdin:    Stdin,
		Stdout:   Stdout,
	}

	input, err := prompt.Run()
	if err != nil {
		return "", err
	}
	// Variable-level substitution, we need to get defaults so later references can be resolved in this loop
	if input == "" && defaultString != "" {
		input = defaultValue
	}
	return input, nil
}

func GetInputFromPrompt(desiredInput string) string {
	prompt := &promptui.Prompt{
		Label: "Please enter " + desiredInput,
		Validate: func(s string) error {
			if len(s) <= 0 {
				return fmt.Errorf("input must be greater than 0")
			}
			return nil
		},
	}

	input, err := prompt.Run()
	if err != nil {
		log.Fatal(err)
	}

	return input
}

type SelectOpt[T any] struct {
	// Field returns the name to use for each select item.
	Field func(t T) string
	// Default is the default selection. If Field is used this should be the result of calling Field on the default.
	Default *T
}

func Select[T any](label string, items []T, opt *SelectOpt[T]) (T, error) {
	selections := make([]interface{}, len(items))
	for i, item := range items {
		selections[i] = item
	}

	if opt != nil && opt.Field != nil {
		for i, item := range items {
			selections[i] = opt.Field(item)
		}
	}

	if len(selections) == 0 {
		return *new(T), errors.New("no selection options")
	}

	if _, ok := selections[0].(string); !ok {
		return *new(T), errors.New("selections must be of type string or use opt.Field")
	}

	searcher := func(search string, i int) bool {
		str, _ := selections[i].(string) // no need to check if okay, we guard earlier

		selection := strings.ToLower(str)
		search = strings.ToLower(search)

		return strings.Contains(selection, search)
	}

	// sort the default selection to top if exists
	if opt != nil && opt.Default != nil {
		defaultStr := opt.Field(*opt.Default)
		for i, selection := range selections {
			if defaultStr == selection {
				selections[0], selections[i] = selections[i], selections[0]
				items[0], items[i] = items[i], items[0]
				break
			}
		}
	}

	p := promptui.Select{
		Label:    label,
		Items:    selections,
		Searcher: searcher,
	}

	i, _, err := p.Run()
	if err != nil {
		return *new(T), fmt.Errorf("running select: %w", err)
	}

	if i >= len(items) {
		return *new(T), errors.New("items index out of range")
	}

	return items[i], nil
}

func PromptByResource(config *config.DraftConfig, varsToSkip []string) (map[string]string, error) {
	skipMap := make(map[string]interface{})
	for _, v := range varsToSkip {
		skipMap[v] = interface{}(nil)
	}

	inputs := make(map[string]string)

	for name, variable := range config.Variables {
		if val, ok := skipMap[name]; ok && val != "" {
			log.Debugf("Skipping prompt for %s", name)
			continue
		}

		if variable.IsPromptDisabled {
			log.Debugf("Skipping prompt for %s as it has IsPromptDisabled=true", name)
			noPromptDefaultValue := GetVariableDefaultValue(name, variable, inputs)
			if noPromptDefaultValue == "" {
				return nil, fmt.Errorf("IsPromptDisabled is true for %s but no default value was found", name)
			}
			log.Debugf("Using default value %s for %s", noPromptDefaultValue, name)
			inputs[name] = noPromptDefaultValue
			continue
		}

		var err error

		switch variable.Resource {
		case "azContainerRegistry":
			inputs[name], err = promptForAcr()
			if err != nil {
				return nil, fmt.Errorf("prompting for Azure Container Registry: %v", err)
			}
		case "azClusterName":
			inputs[name], err = promptForAzureClusterName()

		case "azResourceGroup":

		case "containerName":

		case "dir":

		case "ghBranch":

		}
	}
}

func promptForAcr() (string, error) {
	providers.CheckAzCliInstalled()
	if !providers.IsLoggedInToAz() {
		if err := providers.LogInToAz(); err != nil {
			return "", fmt.Errorf("failed to log in to Azure CLI: %v", err)
		}
	}

	getAccountCmd := exec.Command("az", "acr", "list", "--query", "[].name")
	out, err := getAccountCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to find Azure Container Registry %s: %v", value, err)
	}

	var acrNames []string
	json.Unmarshal(out, &acrNames)

	acr, err := Select("Please select the Azure Container Registry you would like to use", acrNames, nil)
	if err != nil {
		return "", fmt.Errorf("failed to select Azure Container Registry: %v", err)
	}

	return acr, nil
}

func promptForAzureClusterName() (string, error) {
	providers.CheckAzCliInstalled()
	if !providers.IsLoggedInToAz() {
		if err := providers.LogInToAz(); err != nil {
			return "", fmt.Errorf("failed to log in to Azure CLI: %v", err)
		}
	}

	getAccountCmd := exec.Command("az", "acr", "list", "--query", "[].name")
	out, err := getAccountCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to find Azure Container Registry %s: %v", value, err)
	}
}

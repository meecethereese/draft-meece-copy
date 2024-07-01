package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v3"

	"github.com/Azure/draft/pkg/reporeader"
	"github.com/Azure/draft/pkg/reporeader/readers"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/Azure/draft/pkg/config"
	"github.com/Azure/draft/pkg/deployments"
	dryrunpkg "github.com/Azure/draft/pkg/dryrun"
	"github.com/Azure/draft/pkg/filematches"
	"github.com/Azure/draft/pkg/languages"
	"github.com/Azure/draft/pkg/linguist"
	"github.com/Azure/draft/pkg/prompts"
	"github.com/Azure/draft/pkg/templatewriter"
	"github.com/Azure/draft/pkg/templatewriter/writers"
	"github.com/Azure/draft/template"
)

// ErrNoLanguageDetected is raised when `draft create` does not detect source
// code for linguist to classify, or if there are no packs available for the detected languages.
var ErrNoLanguageDetected = errors.New("no supported languages were detected")
var flagVariablesMap = make(map[string]string)

const LANGUAGE_VARIABLE = "LANGUAGE"
const TWO_SPACES = "  "

// Flag defaults
const emptyDefaultFlagValue = ""
const currentDirDefaultFlagValue = "."

type createCmd struct {
	lang       string
	dest       string
	deployType string

	dockerfileOnly    bool
	deploymentOnly    bool
	skipFileDetection bool
	flagVariables     []string

	createConfigPath string
	createConfig     *CreateConfig

	supportedLangs *languages.Languages

	templateWriter           templatewriter.TemplateWriter
	templateVariableRecorder config.TemplateVariableRecorder
	repoReader               reporeader.RepoReader
}

func newCreateCmd() *cobra.Command {
	cc := &createCmd{}

	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Add minimum required files to the directory",
		Long:  "This command will add the minimum required files to the local directory for your Kubernetes deployment.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cc.initConfig(); err != nil {
				return err
			}
			return cc.run()
		},
	}

	f := cmd.Flags()

	f.StringVarP(&cc.createConfigPath, "create-config", "c", emptyDefaultFlagValue, "specify the path to the configuration file")
	f.StringVarP(&cc.lang, "language", "l", emptyDefaultFlagValue, "specify the language used to create the Kubernetes deployment")
	f.StringVarP(&cc.dest, "destination", "d", currentDirDefaultFlagValue, "specify the path to the project directory")
	f.StringVarP(&cc.deployType, "deploy-type", "", emptyDefaultFlagValue, "specify deployment type (eg. helm, kustomize, manifests)")
	f.BoolVar(&cc.dockerfileOnly, "dockerfile-only", false, "only create Dockerfile in the project directory")
	f.BoolVar(&cc.deploymentOnly, "deployment-only", false, "only create deployment files in the project directory")
	f.BoolVar(&cc.skipFileDetection, "skip-file-detection", false, "skip file detection step")
	f.StringArrayVarP(&cc.flagVariables, "variable", "", []string{}, "pass additional variables using repeated --variable flag")

	return cmd
}

func (cc *createCmd) initConfig() error {
	if cc.createConfigPath != "" {
		log.Debug("loading config")
		configBytes, err := os.ReadFile(cc.createConfigPath)
		if err != nil {
			return err
		}

		var cfg CreateConfig
		if err = yaml.Unmarshal(configBytes, &cfg); err != nil {
			return err
		}
		cc.createConfig = &cfg
		return nil
	}

	//TODO: create a config for the user and save it for subsequent uses
	cc.createConfig = &CreateConfig{}

	return nil
}

func (cc *createCmd) run() error {
	log.Debugf("config: %s", cc.createConfigPath)

	for _, flagVar := range cc.flagVariables {
		flagVarName, flagVarValue, ok := strings.Cut(flagVar, "=")
		if !ok {
			return fmt.Errorf("invalid variable format: %s", flagVar)
		}
		flagVariablesMap[flagVarName] = flagVarValue
		log.Debugf("flag variable %s=%s", flagVarName, flagVarValue)
	}

	var dryRunRecorder *dryrunpkg.DryRunRecorder
	if dryRun {
		dryRunRecorder = dryrunpkg.NewDryRunRecorder()
		cc.templateVariableRecorder = dryRunRecorder
		cc.templateWriter = dryRunRecorder
	} else {
		cc.templateWriter = &writers.LocalFSWriter{}
	}
	cc.repoReader = &readers.LocalFSReader{}

	detectedLangDraftConfig, languageName, err := cc.detectLanguage()
	if err != nil {
		return err
	}

	err = cc.createFiles(detectedLangDraftConfig, languageName)
	if dryRun {
		cc.templateVariableRecorder.Record(LANGUAGE_VARIABLE, languageName)
		dryRunText, err := json.MarshalIndent(dryRunRecorder.DryRunInfo, "", TWO_SPACES)
		if err != nil {
			return err
		}
		fmt.Println(string(dryRunText))
		if dryRunFile != "" {
			log.Printf("writing dry run info to file %s", dryRunFile)
			err = os.WriteFile(dryRunFile, dryRunText, 0644)
			if err != nil {
				return err
			}
		}
	}
	return err
}

// detectLanguage detects the language used in a project destination directory
// It returns the DraftConfig for that language and the name of the language
func (cc *createCmd) detectLanguage() (*config.DraftConfig, string, error) {
	hasGo := false
	hasGoMod := false
	var langs []*linguist.Language
	var err error
	if cc.createConfig.LanguageType == "" {
		if cc.lang != "" {
			cc.createConfig.LanguageType = cc.lang
		} else {
			log.Info("--- Detecting Language ---")
			langs, err = linguist.ProcessDir(cc.dest)
			log.Debugf("linguist.ProcessDir(%v) result:\n\nError: %v", cc.dest, err)
			if err != nil {
				return nil, "", fmt.Errorf("there was an error detecting the language: %s", err)
			}
			for _, lang := range langs {
				log.Debugf("%s:\t%f (%s)", lang.Language, lang.Percent, lang.Color)
				// For now let's check here for weird stuff like go module support
				if lang.Language == "Go" {
					hasGo = true

					selection := &promptui.Select{
						Label: "Linguist detected Go, do you use Go Modules?",
						Items: []string{"yes", "no"},
					}

					_, selectResponse, err := selection.Run()
					if err != nil {
						return nil, "", err
					}

					hasGoMod = strings.EqualFold(selectResponse, "yes")
				}

				if lang.Language == "Java" {

					selection := &promptui.Select{
						Label: "Linguist detected Java, are you using maven or gradle?",
						Items: []string{"gradle", "maven", "gradlew"},
					}

					_, selectResponse, err := selection.Run()
					if err != nil {
						return nil, "", err
					}

					if selectResponse == "gradle" {
						lang.Language = "Gradle"
					} else if selectResponse == "gradlew" {
						lang.Language = "Gradlew"
					}
				}
			}

			log.Debugf("detected %d langs", len(langs))

			if len(langs) == 0 {
				return nil, "", ErrNoLanguageDetected
			}
		}
	}

	cc.supportedLangs = languages.CreateLanguagesFromEmbedFS(template.Dockerfiles, cc.dest)

	if cc.createConfig.LanguageType != "" {
		log.Debug("using configuration language")
		lowerLang := strings.ToLower(cc.createConfig.LanguageType)
		langConfig := cc.supportedLangs.GetConfig(lowerLang)
		if langConfig == nil {
			return nil, "", ErrNoLanguageDetected
		}

		return langConfig, lowerLang, nil
	}

	for _, lang := range langs {
		detectedLang := linguist.Alias(lang)
		log.Infof("--> Draft detected %s (%f%%)\n", detectedLang.Language, detectedLang.Percent)
		lowerLang := strings.ToLower(detectedLang.Language)
		if cc.supportedLangs.ContainsLanguage(lowerLang) {
			if lowerLang == "go" && hasGo && hasGoMod {
				log.Debug("detected go and go module")
				lowerLang = "gomodule"
			}
			langConfig := cc.supportedLangs.GetConfig(lowerLang)
			return langConfig, lowerLang, nil
		}
		log.Infof("--> Could not find a pack for %s. Trying to find the next likely language match...", detectedLang.Language)
	}
	return nil, "", ErrNoLanguageDetected
}

func (cc *createCmd) generateDockerfile(langConfig *config.DraftConfig, lowerLang string) error {
	log.Info("--- Dockerfile Creation ---")
	if cc.supportedLangs == nil {
		return errors.New("supported languages were loaded incorrectly")
	}

	// Extract language-specific defaults from repo
	extractedValues, err := cc.supportedLangs.ExtractDefaults(lowerLang, cc.repoReader)
	if err != nil {
		return err
	}

	// Check for existing duplicate defaults
	for k, v := range extractedValues {
		variableExists := false
		for i, variable := range langConfig.Variables {
			if k == variable.Name {
				variableExists = true
				langConfig.Variables[i].Default.Value = v
				break
			}
		}
		if !variableExists {
			langConfig.Variables = append(langConfig.Variables, config.BuilderVar{
				Name: k,
				Default: config.BuilderVarDefault{
					Value: v,
				},
			})
		}
	}

	if cc.createConfig.LanguageVariables == nil {
		if err = prompts.RunPromptsFromConfigWithSkips(langConfig); err != nil {
			return err
		}
	} else {
		inputs, err = validateConfigInputsToPrompts(langConfig, cc.createConfig.LanguageVariables)
		if err != nil {
			return err
		}
	}

	if cc.templateVariableRecorder != nil {
		for k, v := range inputs {
			cc.templateVariableRecorder.Record(k, v)
		}
	}

	maps.Copy(inputs, flagVariablesMap)

	if err = cc.supportedLangs.CreateDockerfileForLanguage(lowerLang, inputs, cc.templateWriter); err != nil {
		return fmt.Errorf("there was an error when creating the Dockerfile for language %s: %w", cc.createConfig.LanguageType, err)
	}

	log.Info("--> Creating Dockerfile...\n")
	return err
}

func (cc *createCmd) createDeployment() error {
	log.Info("--- Deployment File Creation ---")
	d := deployments.CreateDeploymentsFromEmbedFS(template.Deployments, cc.dest)
	var deployType string
	var customInputs map[string]string
	var err error

	if cc.createConfig.DeployType != "" {
		deployType = strings.ToLower(cc.createConfig.DeployType)
		deployConfig, err := d.GetConfig(deployType)
		if err != nil {
			return err
		}
		if deployConfig == nil {
			return errors.New("invalid deployment type")
		}
		customInputs, err = validateConfigInputsToPrompts(deployConfig, cc.createConfig.DeployVariables)
		if err != nil {
			return err
		}

	} else {
		if cc.deployType == "" {
			selection := &promptui.Select{
				Label: "Select k8s Deployment Type",
				Items: []string{"helm", "kustomize", "manifests"},
			}

			_, deployType, err = selection.Run()
			if err != nil {
				return err
			}
		} else {
			deployType = cc.deployType
		}

		deployConfig, err := d.GetConfig(deployType)
		if err != nil {
			return err
		}
		customInputs, err = prompts.RunPromptsFromConfigWithSkips(deployConfig, maps.Keys(flagVariablesMap))
		if err != nil {
			return err
		}
	}

	maps.Copy(customInputs, flagVariablesMap)

	if cc.templateVariableRecorder != nil {
		for k, v := range customInputs {
			cc.templateVariableRecorder.Record(k, v)
		}
	}

	log.Infof("--> Creating %s Kubernetes resources...\n", deployType)

	return d.CopyDeploymentFiles(deployType, customInputs, cc.templateWriter)
}

func (cc *createCmd) createFiles(detectedLang *config.DraftConfig, lowerLang string) error {
	// does no further checks without file detection

	if cc.dockerfileOnly && cc.deploymentOnly {
		return errors.New("can only pass in one of --dockerfile-only and --deployment-only")
	}

	if cc.skipFileDetection {
		if !cc.deploymentOnly {
			err := cc.generateDockerfile(detectedLang, lowerLang)
			if err != nil {
				return err
			}
		}
		if !cc.dockerfileOnly {
			err := cc.createDeployment()
			if err != nil {
				return err
			}
		}
		return nil
	}

	// check if the local directory has dockerfile or charts
	hasDockerFile, hasDeploymentFiles, err := filematches.SearchDirectory(cc.dest)
	if err != nil {
		return err
	}

	// prompts user for dockerfile re-creation
	if hasDockerFile && !cc.deploymentOnly {
		selection := &promptui.Select{
			Label: "We found Dockerfile in the directory, would you like to recreate the Dockerfile?",
			Items: []string{"yes", "no"},
		}

		_, selectResponse, err := selection.Run()
		if err != nil {
			return err
		}

		hasDockerFile = strings.EqualFold(selectResponse, "no")
	}

	if cc.deploymentOnly {
		log.Info("--> --deployment-only=true, skipping Dockerfile creation...")
	} else if hasDockerFile {
		log.Info("--> Found Dockerfile in local directory, skipping Dockerfile creation...")
	} else if !cc.deploymentOnly {
		err := cc.generateDockerfile(detectedLang, lowerLang)
		if err != nil {
			return err
		}
	}

	// prompts user for deployment re-creation
	if hasDeploymentFiles && !cc.dockerfileOnly {
		selection := &promptui.Select{
			Label: "We found deployment files in the directory, would you like to create new deployment files?",
			Items: []string{"yes", "no"},
		}

		_, selectResponse, err := selection.Run()
		if err != nil {
			return err
		}

		hasDeploymentFiles = strings.EqualFold(selectResponse, "no")
	}

	if cc.dockerfileOnly {
		log.Info("--> --dockerfile-only=true, skipping deployment file creation...")
	} else if hasDeploymentFiles {
		log.Info("--> Found deployment directory in local directory, skipping deployment file creation...")
	} else if !cc.dockerfileOnly {
		err := cc.createDeployment()
		if err != nil {
			return err
		}
	}

	log.Info("Draft has successfully created deployment resources for your project 😃")
	log.Info("Use 'draft setup-gh' to set up Github OIDC.")

	return nil
}

func init() {
	rootCmd.AddCommand(newCreateCmd())
}

func validateConfigInputsToPrompts(draftConfig *config.DraftConfig, provided []UserInputs) error {
	// set inputs to provided values
	for _, providedVar := range provided {
		variable, err := draftConfig.GetVariable(providedVar.Name)
		if err != nil {
			return fmt.Errorf("validate config inputs to prompts: %w", providedVar.Name, err)
		}
		variable.Value = providedVar.Value
	}

	if err := draftConfig.ApplyDefaultVariables(customInputs); err != nil {
		return nil, fmt.Errorf("validate config inputs to prompts: %w", err)
	}

	for _, variable := range draftConfig.Variables {
		value, ok := customInputs[variable.Name]
		if !ok {
			return nil, fmt.Errorf("config missing required variable: %s with description: %s", variable.Name, variable.Description)
		} else if value == "" {
			return nil, fmt.Errorf("value for variable %s is empty", variable.Name)
		}
	}

	return customInputs, nil
}

func FlagVariablesToMap(flagVariables []string) map[string]string {
	flagValuesMap := make(map[string]string)
	for _, flagVar := range flagVariables {
		flagVarName, flagVarValue, ok := strings.Cut(flagVar, "=")
		if !ok {
			log.Fatalf("invalid variable format: %s", flagVar)
		}
		flagValuesMap[flagVarName] = flagVarValue
	}
	return flagValuesMap
}

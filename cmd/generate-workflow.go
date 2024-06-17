package cmd

import (
	"errors"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"

	"github.com/Azure/draft/pkg/config"
	"github.com/Azure/draft/pkg/osutil"
	"github.com/Azure/draft/pkg/prompts"
	"github.com/Azure/draft/pkg/templatewriter"
	"github.com/Azure/draft/pkg/templatewriter/writers"
	"github.com/Azure/draft/pkg/workflows"
	"github.com/Azure/draft/template"
)

type generateWorkflowCmd struct {
	workflowConfig workflows.WorkflowConfig
	dest           string
	deployType     string
	flagVariables  []string
	templateWriter templatewriter.TemplateWriter
}

var flagValuesMap map[string]string

func newGenerateWorkflowCmd() *cobra.Command {

	gwCmd := &generateWorkflowCmd{}
	gwCmd.dest = ""
	var cmd = &cobra.Command{
		Use:   "generate-workflow [flags]",
		Short: "Generates a Github workflow for automatic build and deploy to AKS",
		Long: `This command will generate a Github workflow to build and deploy an application containerized 
with draft on AKS. This command assumes the 'setup-gh' command has been run properly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			flagValuesMap = make(map[string]string)
			if cmd.Flags().NFlag() != 0 {
				flagValuesMap = gwCmd.workflowConfig.SetFlagValuesToMap()
			}
			log.Info("--> Generating Github workflow")
			if err := gwCmd.generateWorkflows(gwCmd.dest, gwCmd.deployType, gwCmd.flagVariables, gwCmd.templateWriter, flagValuesMap); err != nil {
				return err
			}

			log.Info("Draft has successfully generated a Github workflow for your project 😃")

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&gwCmd.workflowConfig.AksClusterName, "cluster-name", "c", emptyDefaultFlagValue, "specify the AKS cluster name")
	f.StringVarP(&gwCmd.workflowConfig.AcrName, "registry-name", "r", emptyDefaultFlagValue, "specify the Azure container registry name")
	f.StringVar(&gwCmd.workflowConfig.ContainerName, "container-name", emptyDefaultFlagValue, "specify the container image name")
	f.StringVarP(&gwCmd.workflowConfig.ResourceGroupName, "resource-group", "g", emptyDefaultFlagValue, "specify the Azure resource group of your AKS cluster")
	f.StringVarP(&gwCmd.dest, "destination", "d", currentDirDefaultFlagValue, "specify the path to the project directory")
	f.StringVarP(&gwCmd.workflowConfig.BranchName, "branch", "b", emptyDefaultFlagValue, "specify the Github branch to automatically deploy from")
	f.StringVar(&gwCmd.deployType, "deploy-type", emptyDefaultFlagValue, "specify the type of deployment")
	f.StringArrayVarP(&gwCmd.flagVariables, "variable", "", []string{}, "pass additional variables")
	f.StringVarP(&gwCmd.workflowConfig.BuildContextPath, "build-context-path", "x", emptyDefaultFlagValue, "specify the docker build context path")
	gwCmd.templateWriter = &writers.LocalFSWriter{}
	return cmd
}

func init() {
	rootCmd.AddCommand(newGenerateWorkflowCmd())
}

func (gwc *generateWorkflowCmd) generateWorkflows(dest string, deployType string, flagVariables []string, templateWriter templatewriter.TemplateWriter, flagValuesMap map[string]string) error {
	if flagValuesMap == nil {
		return fmt.Errorf("flagValuesMap is nil")
	}
	var err error
	for _, flagVar := range flagVariables {
		flagVarName, flagVarValue, ok := strings.Cut(flagVar, "=")
		if !ok {
			return fmt.Errorf("invalid variable format: %s", flagVar)
		}
		flagValuesMap[flagVarName] = flagVarValue
		log.Debugf("flag variable %s=%s", flagVarName, flagVarValue)
	}

	if deployType == "" {
		selection := &promptui.Select{
			Label: "Select k8s Deployment Type",
			Items: []string{"helm", "kustomize", "manifests"},
		}

		_, deployType, err = selection.Run()
		if err != nil {
			return err
		}
	}

	workflow := workflows.CreateWorkflowsFromEmbedFS(template.Workflows, dest)
	workflowConfig, err := workflow.GetConfig(deployType)
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}

	customInputs, err := prompts.RunPromptsFromConfigWithSkips(workflowConfig, maps.Keys(flagValuesMap))
	if err != nil {
		return err
	}

	maps.Copy(customInputs, flagValuesMap)

	return workflow.CreateWorkflowFiles(deployType, customInputs, templateWriter)
}

func GenerateWorkflowBytes(draftConfig *config.DraftConfig, deployType string) ([]byte, error) {
	envArgs := draftConfig.BuilderVarMap()
	var srcPath string

	switch deployType {
	case "helm":
		srcPath = "workflow/helm/.github/workflows/azure-kubernetes-service-helm.yml"
	case "manifests":
		srcPath = "workflow/manifests/.github/workflows/azure-kubernetes-service.yml"
	default:
		return nil, errors.New("unsupported deploy type")
	}

	workflowBytes, err := osutil.ReplaceTemplateVariables(template.Workflows, srcPath, envArgs)
	if err != nil {
		return nil, fmt.Errorf("replace template variables: %w", err)
	}

	if err = osutil.CheckAllVariablesSubstituted(string(workflowBytes)); err != nil {
		return nil, fmt.Errorf("check all variables substituted: %w", err)
	}

	return workflowBytes, nil
}

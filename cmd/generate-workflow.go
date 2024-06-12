package cmd

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/maps"

	"github.com/Azure/draft/pkg/osutil"
	"github.com/Azure/draft/pkg/prompts"
	"github.com/Azure/draft/pkg/templatewriter"
	"github.com/Azure/draft/pkg/templatewriter/writers"
	"github.com/Azure/draft/pkg/workflows"
	"github.com/Azure/draft/template"
)

type generateWorkflowCmd struct {
	workflowEnv    workflows.WorkflowEnv
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
			gwCmd.workflowEnv.FillWorkflowEnv()

			flagValuesMap, err := createFlagMap(*cmd, gwCmd.workflowEnv, gwCmd.flagVariables)
			if err != nil {
				return fmt.Errorf("create flag map: %w", err)
			}

			gwCmd.deployType, err = promptDeployType(gwCmd.deployType)
			if err != nil {
				return fmt.Errorf("prompt deploy type: %w", err)
			}

			customInputs, err := prompts.RunPromptsForDeployTypeWithSkipsIO(gwCmd.workflowEnv, gwCmd.deployType, maps.Keys(flagValuesMap), nil, nil)
			if err != nil {
				return err
			}
			maps.Copy(customInputs, flagValuesMap)

			log.Info("--> Generating Github workflow")
			workflowBytes, err := GenerateWorkflowBytes(gwCmd.deployType, flagValuesMap)
			if err != nil {
				return fmt.Errorf("generate workflow bytes: %w", err)
			}

			writeFile(gwCmd.deployType, gwCmd.dest, workflowBytes, gwCmd.templateWriter)

			log.Info("Draft has successfully generated a Github workflow for your project 😃")

			return nil
		},
	}

	f := cmd.Flags()
	f.StringVarP(&gwCmd.workflowEnv.ClusterName.Value, "cluster-name", "c", emptyDefaultFlagValue, "specify the AKS cluster name")
	f.StringVarP(&gwCmd.workflowEnv.PrivateCluster.Value, "private-cluster", "p", emptyDefaultFlagValue, "specify if the AKS cluster is private")
	f.StringVarP(&gwCmd.workflowEnv.AzureContainerRegistry.Value, "registry-name", "r", emptyDefaultFlagValue, "specify the Azure container registry name")
	f.StringVar(&gwCmd.workflowEnv.ContainerName.Value, "container-name", emptyDefaultFlagValue, "specify the container image name")
	f.StringVarP(&gwCmd.workflowEnv.AcrResourceGroup.Value, "resource-group", "g", emptyDefaultFlagValue, "specify the Azure resource group of your ACR")
	f.StringVarP(&gwCmd.workflowEnv.ClusterResourceGroup.Value, "cluster-resource-group", "l", emptyDefaultFlagValue, "specify the Azure resource group of your AKS cluster")
	f.StringVarP(&gwCmd.dest, "destination", "d", currentDirDefaultFlagValue, "specify the path to the project directory")
	f.StringVarP(&gwCmd.workflowEnv.BranchName.Value, "branch", "b", emptyDefaultFlagValue, "specify the Github branch to automatically deploy from")
	f.StringVar(&gwCmd.deployType, "deploy-type", emptyDefaultFlagValue, "specify the type of deployment")
	f.StringArrayVarP(&gwCmd.flagVariables, "variable", "", []string{}, "pass additional variables")
	f.StringVarP(&gwCmd.workflowEnv.BuildContextPath.Value, "build-context-path", "x", emptyDefaultFlagValue, "specify the docker build context path")
	gwCmd.templateWriter = &writers.LocalFSWriter{}
	return cmd
}

func init() {
	rootCmd.AddCommand(newGenerateWorkflowCmd())
}

func createFlagMap(cmd cobra.Command, workflowEnv workflows.WorkflowEnv, flagVariables []string) (map[string]string, error) {
	flagValuesMap = make(map[string]string)
	if cmd.Flags().NFlag() != 0 {
		flagValuesMap = workflowEnv.BuildMap()
	}

	if flagValuesMap == nil {
		return nil, fmt.Errorf("flagValuesMap is nil")
	}

	for _, flagVar := range flagVariables {
		flagVarName, flagVarValue, ok := strings.Cut(flagVar, "=")
		if !ok {
			return nil, fmt.Errorf("invalid variable format: %s", flagVar)
		}
		flagValuesMap[flagVarName] = flagVarValue
		log.Debugf("flag variable %s=%s", flagVarName, flagVarValue)
	}

	return flagValuesMap, nil
}

func promptDeployType(deployType string) (string, error) {
	var err error

	if deployType == "" {
		selection := &promptui.Select{
			Label: "Select k8s Deployment Type",
			Items: []string{"helm", "kustomize", "manifests"},
		}

		_, deployType, err = selection.Run()
		if err != nil {
			return "", err
		}
	}

	return deployType, nil
}

func writeFile(deployType string, dest string, workflowBytes []byte, templateWriter templatewriter.TemplateWriter) error {
	var destPath string

	switch deployType {
	case "helm":
		destPath = path.Join(dest, ".github/workflows/azure-kubernetes-service-helm.yml")
	case "kustomize":
		destPath = path.Join(dest, ".github/workflows/azure-kubernetes-service-kustomize.yml")
	case "manifests":
		destPath = path.Join(dest, ".github/workflows/azure-kubernetes-service.yml")
	default:
		return errors.New("unsupported deployment type")
	}

	if err := templateWriter.EnsureDirectory(destPath); err != nil {
		return fmt.Errorf("ensure directory: %w", err)
	}

	if err := templateWriter.WriteFile(destPath, workflowBytes); err != nil {
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}

func GenerateWorkflowBytes(deployType string, envArgsMap map[string]string) ([]byte, error) {
	switch deployType {
	case "helm":
		workflowBytes, err := osutil.ReplaceTemplateVariables(template.Workflows, "workflow/helm/.github/workflows/azure-kubernetes-service-helm.yml", envArgsMap)
		if err != nil {
			return nil, fmt.Errorf("replace template variables: %w", err)
		}

		if err = osutil.CheckAllVariablesSubstituted(string(workflowBytes)); err != nil {
			return nil, fmt.Errorf("check all variables substituted: %w", err)
		}

		return workflowBytes, nil

	case "kustomize":
		workflowBytes, err := osutil.ReplaceTemplateVariables(template.Workflows, "workflow/kustomize/.github/workflows/azure-kubernetes-service-kustomize.yml", envArgsMap)
		if err != nil {
			return nil, fmt.Errorf("replace template variables: %w", err)
		}

		if err = osutil.CheckAllVariablesSubstituted(string(workflowBytes)); err != nil {
			return nil, fmt.Errorf("check all variables substituted: %w", err)
		}

		return workflowBytes, nil
	case "manifests":
		workflowBytes, err := osutil.ReplaceTemplateVariables(template.Workflows, "workflow/manifests/.github/workflows/azure-kubernetes-service.yml", envArgsMap)
		if err != nil {
			return nil, fmt.Errorf("replace template variables: %w", err)
		}

		if err = osutil.CheckAllVariablesSubstituted(string(workflowBytes)); err != nil {
			return nil, fmt.Errorf("check all variables substituted: %w", err)
		}

		return workflowBytes, nil
	}

	return nil, errors.New("unsupported deployment type")
}

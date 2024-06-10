package workflows

type WorkflowConfig struct {
	AcrName                  string
	AcrResourceGroupName     string
	AksClusterName           string
	BranchName               string
	BuildContextPath         string
	ChartPath                string
	ChartOverridePath        string
	ClusterResourceGroupName string
	ContainerName            string
}

func (config *WorkflowConfig) SetFlagValuesToMap() map[string]string {
	flagValuesMap := make(map[string]string)
	if config.AcrName != "" {
		flagValuesMap["AZURECONTAINERREGISTRY"] = config.AcrName
	}

	if config.AcrResourceGroupName != "" {
		flagValuesMap["RESOURCEGROUP"] = config.AcrResourceGroupName
	}

	if config.AksClusterName != "" {
		flagValuesMap["CLUSTERNAME"] = config.AksClusterName
	}

	if config.BranchName != "" {
		flagValuesMap["BRANCHNAME"] = config.BranchName
	}

	if config.BuildContextPath != "" {
		flagValuesMap["BUILDCONTEXTPATH"] = config.BuildContextPath
	}

	if config.ChartPath != "" {
		flagValuesMap["CHARTPATH"] = config.ChartPath
	}

	if config.ChartOverridePath != "" {
		flagValuesMap["CHARTOVERRIDEPATH"] = config.ChartOverridePath
	}

	if config.ClusterResourceGroupName != "" {
		flagValuesMap["CLUSTERRESOURCEGROUP"] = config.ClusterResourceGroupName
	}

	if config.ContainerName != "" {
		flagValuesMap["CONTAINERNAME"] = config.ContainerName
	}

	return flagValuesMap
}

type EnvVar struct {
	Name          string
	Description   string
	DisablePrompt bool
	Value         string
	Type          string
}

type WorkflowEnv struct {
	AcrResourceGroup       EnvVar
	AzureContainerRegistry EnvVar
	BranchName             EnvVar
	BuildContextPath       EnvVar
	ClusterName            EnvVar
	ClusterResourceGroup   EnvVar
	ContainerName          EnvVar

	HelmEnvStruct      HelmEnv
	KustomizeEnvStruct KustomizeEnv
	ManifestEnvStruct  ManifestEnv
}

type HelmEnv struct {
	ChartPath         EnvVar
	ChartOverridePath EnvVar
	ChartOverrides    EnvVar
}

type ManifestEnv struct {
	DeploymentManifestPath EnvVar
}

type KustomizeEnv struct {
	KustomizePath EnvVar
}

func (we *WorkflowEnv) FillWorkflowEnv() {
	we.AcrResourceGroup.Name = "ACR_RESOURCE_GROUP"
	we.AcrResourceGroup.Description = "The Azure Resource Group of the Azure Container Registry"
	we.AcrResourceGroup.DisablePrompt = false

	we.AzureContainerRegistry.Name = "ACR_NAME"
	we.AzureContainerRegistry.Description = "The name of the Azure Container Registry"
	we.AzureContainerRegistry.DisablePrompt = false

	we.BranchName.Name = "BRANCH_NAME"
	we.BranchName.Description = "The name of the branch to deploy from"
	we.BranchName.DisablePrompt = false

	we.BuildContextPath.Name = "BUILD_CONTEXT_PATH"
	we.BuildContextPath.Description = "The path to the Docker build context"
	if we.BuildContextPath.Value == "" {
		we.BuildContextPath.Value = "."
	}
	we.BuildContextPath.DisablePrompt = false

	we.ClusterName.Name = "CLUSTER_NAME"
	we.ClusterName.Description = "The name of the AKS cluster"
	we.ClusterName.DisablePrompt = false

	we.ClusterResourceGroup.Name = "CLUSTER_RESOURCE_GROUP"
	we.ClusterResourceGroup.Description = "The Azure Resource Group of the AKS cluster"
	we.ClusterResourceGroup.DisablePrompt = false

	we.ContainerName.Name = "CONTAINER_NAME"
	we.ContainerName.Description = "The name of the container image"
	we.ContainerName.DisablePrompt = false

	we.HelmEnvStruct.ChartPath.Name = "CHART_PATH"
	we.HelmEnvStruct.ChartPath.Description = "The path to the Helm chart"
	if we.HelmEnvStruct.ChartPath.Value == "" {
		we.HelmEnvStruct.ChartPath.Value = "./charts"
	}
	we.HelmEnvStruct.ChartPath.DisablePrompt = true

	we.HelmEnvStruct.ChartOverridePath.Name = "CHART_OVERRIDE_PATH"
	we.HelmEnvStruct.ChartOverridePath.Description = "The path to the Helm chart overrides"
	if we.HelmEnvStruct.ChartOverridePath.Value == "" {
		we.HelmEnvStruct.ChartOverridePath.Value = "./charts/production.yaml"
	}
	we.HelmEnvStruct.ChartOverridePath.DisablePrompt = true

	we.HelmEnvStruct.ChartOverrides.Name = "CHART_OVERRIDES"
	we.HelmEnvStruct.ChartOverrides.Description = "The Helm chart overrides"
	we.HelmEnvStruct.ChartOverrides.DisablePrompt = true

	we.KustomizeEnvStruct.KustomizePath.Name = "KUSTOMIZE_PATH"
	we.KustomizeEnvStruct.KustomizePath.Description = "The path to the Kustomize directory"
	if we.KustomizeEnvStruct.KustomizePath.Value == "" {
		we.KustomizeEnvStruct.KustomizePath.Value = "./overlays/production"
	}
	we.KustomizeEnvStruct.KustomizePath.DisablePrompt = true

	we.ManifestEnvStruct.DeploymentManifestPath.Name = "DEPLOYMENT_MANIFEST_PATH"
	we.ManifestEnvStruct.DeploymentManifestPath.Description = "The path to the deployment manifest"
	if we.ManifestEnvStruct.DeploymentManifestPath.Value == "" {
		we.ManifestEnvStruct.DeploymentManifestPath.Value = "./manifests"
	}
	we.ManifestEnvStruct.DeploymentManifestPath.DisablePrompt = true
}

func (we *WorkflowEnv) BuildMap() map[string]string {
	envMap := make(map[string]string)

	checkForVal := func(envVar EnvVar) {
		if envVar.Value != "" {
			envMap[envVar.Name] = envVar.Value
		}
	}

	checkForVal(we.AcrResourceGroup)
	checkForVal(we.AzureContainerRegistry)
	checkForVal(we.BranchName)
	checkForVal(we.BuildContextPath)
	checkForVal(we.ClusterName)
	checkForVal(we.ClusterResourceGroup)
	checkForVal(we.ContainerName)
	checkForVal(we.HelmEnvStruct.ChartPath)
	checkForVal(we.HelmEnvStruct.ChartOverridePath)
	checkForVal(we.HelmEnvStruct.ChartOverrides)
	checkForVal(we.KustomizeEnvStruct.KustomizePath)
	checkForVal(we.ManifestEnvStruct.DeploymentManifestPath)

	return envMap
}

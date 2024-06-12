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
	PrivateCluster         EnvVar

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
	we.AcrResourceGroup.Description = "the Azure Resource Group of the Azure Container Registry"
	we.AcrResourceGroup.DisablePrompt = false

	we.AzureContainerRegistry.Name = "ACR_NAME"
	we.AzureContainerRegistry.Description = "the name of the Azure Container Registry"
	we.AzureContainerRegistry.DisablePrompt = false

	we.BranchName.Name = "BRANCH_NAME"
	we.BranchName.Description = "the name of the branch to deploy from"
	we.BranchName.DisablePrompt = false

	we.BuildContextPath.Name = "BUILD_CONTEXT_PATH"
	we.BuildContextPath.Description = "the path to the Docker build context"
	we.BuildContextPath.DisablePrompt = false
	if we.BuildContextPath.Value == "" {
		we.BuildContextPath.Value = "."
	}

	we.ClusterName.Name = "CLUSTER_NAME"
	we.ClusterName.Description = "the name of the AKS cluster"
	we.ClusterName.DisablePrompt = false

	we.ClusterResourceGroup.Name = "CLUSTER_RESOURCE_GROUP"
	we.ClusterResourceGroup.Description = "the Azure Resource Group of the AKS cluster"
	we.ClusterResourceGroup.DisablePrompt = false

	we.ContainerName.Name = "CONTAINER_NAME"
	we.ContainerName.Description = "the name of the container image"
	we.ContainerName.DisablePrompt = false

	we.PrivateCluster.Name = "PRIVATE_CLUSTER"
	we.PrivateCluster.Description = "whether the AKS cluster is private or not"
	we.PrivateCluster.DisablePrompt = false
	we.PrivateCluster.Type = "bool"

	we.HelmEnvStruct.ChartPath.Name = "CHART_PATH"
	we.HelmEnvStruct.ChartPath.Description = "the path to the Helm chart"
	we.HelmEnvStruct.ChartPath.DisablePrompt = true
	if we.HelmEnvStruct.ChartPath.Value == "" {
		we.HelmEnvStruct.ChartPath.Value = "./charts"
	}

	we.HelmEnvStruct.ChartOverridePath.Name = "CHART_OVERRIDE_PATH"
	we.HelmEnvStruct.ChartOverridePath.Description = "the path to the Helm chart overrides"
	we.HelmEnvStruct.ChartOverridePath.DisablePrompt = true
	if we.HelmEnvStruct.ChartOverridePath.Value == "" {
		we.HelmEnvStruct.ChartOverridePath.Value = "./charts/production.yaml"
	}

	we.HelmEnvStruct.ChartOverrides.Name = "CHART_OVERRIDES"
	we.HelmEnvStruct.ChartOverrides.Description = "the Helm chart overrides"
	we.HelmEnvStruct.ChartOverrides.DisablePrompt = true
	if we.HelmEnvStruct.ChartOverrides.Value == "" {
		we.HelmEnvStruct.ChartOverrides.Value = "replicas:2"
	}

	we.KustomizeEnvStruct.KustomizePath.Name = "KUSTOMIZE_PATH"
	we.KustomizeEnvStruct.KustomizePath.Description = "the path to the Kustomize directory"
	we.KustomizeEnvStruct.KustomizePath.DisablePrompt = true
	if we.KustomizeEnvStruct.KustomizePath.Value == "" {
		we.KustomizeEnvStruct.KustomizePath.Value = "./overlays/production"
	}

	we.ManifestEnvStruct.DeploymentManifestPath.Name = "DEPLOYMENT_MANIFEST_PATH"
	we.ManifestEnvStruct.DeploymentManifestPath.Description = "the path to the deployment manifest"
	we.ManifestEnvStruct.DeploymentManifestPath.DisablePrompt = true
	if we.ManifestEnvStruct.DeploymentManifestPath.Value == "" {
		we.ManifestEnvStruct.DeploymentManifestPath.Value = "./manifests"
	}
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

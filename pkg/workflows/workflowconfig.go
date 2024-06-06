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

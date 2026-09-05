package provider

// Read models returned by the Coolify v4 API. Fields that the API does not
// return for every resource use pointer types so that they can be distinguished
// from plain zero values.
type CoolifyProject struct {
	ID          int    `json:"id"`
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type CoolifyEnvironment struct {
	ID   int    `json:"id"`
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type CoolifyDatabase struct {
	UUID          string  `json:"uuid"`
	Name          string  `json:"name"`
	Description   *string `json:"description"`
	DatabaseType  string  `json:"database_type"`
	Image         string  `json:"image"`
	IsPublic      bool    `json:"is_public"`
	PublicPort    *int    `json:"public_port"`
	EnvironmentID int     `json:"environment_id"`
	PostgresUser  *string `json:"postgres_user"`
	PostgresPass  *string `json:"postgres_password"`
	PostgresDB    *string `json:"postgres_db"`
	InternalDBURL *string `json:"internal_db_url"`
	ExternalDBURL *string `json:"external_db_url"`
}

type CoolifyPrivateKey struct {
	ID           int     `json:"id"`
	UUID         string  `json:"uuid"`
	Name         string  `json:"name"`
	Description  *string `json:"description"`
	PrivateKey   string  `json:"private_key"`
	PublicKey    *string `json:"public_key"`
	FingerPrint  *string `json:"fingerprint"`
	IsGitRelated bool    `json:"is_git_related"`
}

type CoolifyServer struct {
	ID          int     `json:"id"`
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	IP          string  `json:"ip"`
	User        string  `json:"user"`
	Port        int     `json:"port"`
	ProxyType   *string `json:"proxy_type"`
}

type CoolifyDestination struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	ServerID int    `json:"server_id"`
	Network  string `json:"network"`
	Type     string `json:"type"`
}

type CoolifyS3Storage struct {
	UUID        string  `json:"uuid"`
	Name        string  `json:"name"`
	Description *string `json:"description"`
	Endpoint    string  `json:"endpoint"`
	Bucket      string  `json:"bucket"`
	Region      string  `json:"region"`
	IsUsable    bool    `json:"is_usable"`
}

type CoolifyDeployment struct {
	DeploymentUUID string `json:"deployment_uuid"`
	ApplicationID  string `json:"application_id"`
	Status         string `json:"status"`
	Commit         string `json:"commit"`
	CommitMessage  string `json:"commit_message"`
	Logs           string `json:"logs"`
	DeploymentURL  string `json:"deployment_url"`
}

// CoolifyEnvironmentVariable is an environment variable attached to an
// application or service. Hidden variables (is_shown_once) may have their
// value masked by the API on subsequent reads.
type CoolifyEnvironmentVariable struct {
	UUID      string `json:"uuid"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	IsPreview bool   `json:"is_preview"`
}

type CoolifyApplication struct {
	ID                      int                  `json:"id"`
	UUID                    string               `json:"uuid"`
	Name                    string               `json:"name"`
	Description             string               `json:"description"`
	FQDN                    string               `json:"fqdn"`
	BuildPack               string               `json:"build_pack"`
	GitRepository           string               `json:"git_repository"`
	GitBranch               string               `json:"git_branch"`
	GitCommitSHA            string               `json:"git_commit_sha"`
	DockerRegistryImageName string               `json:"docker_registry_image_name"`
	DockerRegistryImageTag  string               `json:"docker_registry_image_tag"`
	PortsExposes            string               `json:"ports_exposes"`
	PortsMappings           string               `json:"ports_mappings"`
	BaseDirectory           string               `json:"base_directory"`
	PublishDirectory        string               `json:"publish_directory"`
	InstallCommand          string               `json:"install_command"`
	BuildCommand            string               `json:"build_command"`
	StartCommand            string               `json:"start_command"`
	HealthCheckEnabled      bool                 `json:"health_check_enabled"`
	HealthCheckPath         string               `json:"health_check_path"`
	Dockerfile              string               `json:"dockerfile"`
	DockerfileLocation      string               `json:"dockerfile_location"`
	CustomLabels            string               `json:"custom_labels"`
	CustomDockerRunOptions  string               `json:"custom_docker_run_options"`
	PreDeploymentCommand    string               `json:"pre_deployment_command"`
	PostDeploymentCommand   string               `json:"post_deployment_command"`
	Redirect                *string              `json:"redirect"`
	Status                  string               `json:"status"`
	SourceID                *int                 `json:"source_id"`
	PrivateKeyID            *int                 `json:"private_key_id"`
	EnvironmentID           int                  `json:"environment_id"`
	Settings                *ApplicationSettings `json:"settings"`
}

type ApplicationSettings struct {
	IsAutoDeployEnabled         bool `json:"is_auto_deploy_enabled"`
	IsForceHttpsEnabled         bool `json:"is_force_https_enabled"`
	IsPreviewDeploymentsEnabled bool `json:"is_preview_deployments_enabled"`
	IsStatic                    bool `json:"is_static"`
	IsSPA                       bool `json:"is_spa"`
}

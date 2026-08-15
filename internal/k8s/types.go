package k8s

const NAMESPACE = "ofan-dev"

type ServerOpts struct {
	Name        string
	Namespace   string
	Replicas    int32
	NodePort    int32
	StorageSize string
	Config      ValheimConfig
}

type ValheimConfig struct {
	CoreSettings   CoreSettings
	AccessControl  AccessControl
	Maintenance    Maintenance
	Mods           Mods
	SystemSettings SystemSettings
}

type CoreSettings struct {
	ServerName   string
	WorldName    string
	ServerPass   string
	ServerPort   int32
	ServerPublic bool
}

type AccessControl struct {
	AdminListIDs     string
	BannedListIDs    string
	PermittedListIDs string
}

type Maintenance struct {
	UpdateCron      string
	UpdateIfIdle    bool
	RestartCron     string
	RestartIfIdle   bool
	Backups         bool
	BackupsIfIdle   bool
	BackupsCron     string
	BackupsMaxAge   int
	BackupsMaxCount int
}

type Mods struct {
	ValheimPlus bool
	BepInEx     bool
}

type SystemSettings struct {
	TimeZone string
	PUID     int
	PGID     int
}

func DefaultValheimConfig(name, password string) ValheimConfig {
	return ValheimConfig{
		CoreSettings: CoreSettings{
			ServerName:   name,
			WorldName:    "Dedicated",
			ServerPass:   password,
			ServerPort:   2456,
			ServerPublic: false,
		},
		Maintenance: Maintenance{
			UpdateCron:    "*/15 * * * *",
			UpdateIfIdle:  true,
			Backups:       true,
			BackupsIfIdle: true,
			BackupsCron:   "0 * * * *",
			BackupsMaxAge: 3,
		},
		SystemSettings: SystemSettings{
			TimeZone: "Etc/UTC",
			PUID:     0,
			PGID:     0,
		},
	}
}

func NewServerOpts(name, password string, config *ValheimConfig) ServerOpts {
	srvOpts := ServerOpts{
		Name:        name,
		Namespace:   NAMESPACE,
		Replicas:    1,
		NodePort:    0,
		StorageSize: "10Gi",
	}

	if config == nil {
		srvOpts.Config = DefaultValheimConfig(name, password)
		return srvOpts
	}

	srvOpts.Config = *config
	return srvOpts
}

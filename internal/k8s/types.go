package k8s

var maxReplicas int32 = 1

type ServerOpts struct {
	Name        string        `json:"name"`
	Namespace   string        `json:"namespace,omitempty"`
	Replicas    int32         `json:"replicas"`
	StorageSize string        `json:"storage_size,omitempty"`
	Config      ValheimConfig `json:"config"`
}

type ValheimConfig struct {
	CoreSettings   CoreSettings   `json:"core_settings"`
	AccessControl  AccessControl  `json:"access_control"`
	Maintenance    Maintenance    `json:"maintenance"`
	Mods           Mods           `json:"mods"`
	SystemSettings SystemSettings `json:"system_settings"`
}

type CoreSettings struct {
	ServerName   string `json:"server_name"`
	WorldName    string `json:"world_name"`
	ServerPass   string `json:"server_pass"`
	ServerPort   int32  `json:"server_port"`
	ServerPublic bool   `json:"server_public"`
}

type AccessControl struct {
	AdminListIDs     string `json:"admin_list_ids,omitempty"`
	BannedListIDs    string `json:"banned_list_ids,omitempty"`
	PermittedListIDs string `json:"permitted_list_ids,omitempty"`
}

type Maintenance struct {
	UpdateCron      string `json:"update_cron,omitempty"`
	UpdateIfIdle    bool   `json:"update_if_idle"`
	RestartCron     string `json:"restart_cron,omitempty"`
	RestartIfIdle   bool   `json:"restart_if_idle"`
	Backups         bool   `json:"backups"`
	BackupsIfIdle   bool   `json:"backups_if_idle"`
	BackupsCron     string `json:"backups_cron,omitempty"`
	BackupsMaxAge   int    `json:"backups_max_age,omitempty"`
	BackupsMaxCount int    `json:"backups_max_count,omitempty"`
}

type Mods struct {
	ValheimPlus bool `json:"valheim_plus"`
	BepInEx     bool `json:"bepinex"`
}

type SystemSettings struct {
	TimeZone string `json:"time_zone,omitempty"`
	PUID     int    `json:"puid,omitempty"`
	PGID     int    `json:"pgid,omitempty"`
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
	opts := ServerOpts{
		Name:        name,
		Replicas:    maxReplicas,
		StorageSize: "10Gi",
	}

	if config == nil {
		opts.Config = DefaultValheimConfig(name, password)
		return opts
	}

	opts.Config = *config
	return opts
}

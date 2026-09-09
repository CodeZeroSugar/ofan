package k8s

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strconv"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const PVC_STORAGE_AMOUNT = "10Gi"

func hashCfg(vCfg ValheimConfig) (string, error) {
	b, err := json.Marshal(vCfg)
	if err != nil {
		return "", fmt.Errorf("failed to marshal config struct back to json: %w", err)
	}

	h := sha256.New()
	h.Write(b)

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func (m *ServerManager) BuildDeployment() (*appsv1.Deployment, error) {
	labels := serverLabels(m.opts.Name)
	hash, err := hashCfg(m.opts.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to generate config hash for server '%s': %w", m.opts.Name, err)
	}
	annotations := annotations(hash)
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.opts.Name,
			Namespace: m.opts.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RecreateDeploymentStrategyType},
			Replicas: &m.opts.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      labels,
					Annotations: annotations,
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:  "valheim-server",
							Image: "ghcr.io/lloesche/valheim-server:latest",
							VolumeMounts: []v1.VolumeMount{
								{
									Name:      m.opts.Name + "-volume",
									MountPath: "/config",
								},
							},
							EnvFrom: []v1.EnvFromSource{
								{
									ConfigMapRef: &v1.ConfigMapEnvSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: m.opts.Name + "-configmap",
										},
									},
								},
								{
									SecretRef: &v1.SecretEnvSource{
										LocalObjectReference: v1.LocalObjectReference{
											Name: m.opts.Name + "-secret",
										},
									},
								},
							},
						},
					},
					Volumes: []v1.Volume{
						{
							Name: m.opts.Name + "-volume",
							VolumeSource: v1.VolumeSource{
								PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
									ClaimName: m.opts.Name + "-pvc",
								},
							},
						},
					},
				},
			},
		},
	}, nil
}

func cfgMapper(cfg ValheimConfig) map[string]string {
	m := make(map[string]string, 0)
	m["SERVER_NAME"] = cfg.CoreSettings.ServerName
	m["WORLD_NAME"] = cfg.CoreSettings.WorldName
	m["SERVER_PUBLIC"] = strconv.FormatBool(cfg.CoreSettings.ServerPublic)
	m["SERVER_PORT"] = strconv.Itoa(int(cfg.CoreSettings.ServerPort))
	m["ADMINLIST_IDS"] = cfg.AccessControl.AdminListIDs
	m["BANNEDLIST_IDS"] = cfg.AccessControl.BannedListIDs
	m["PERMITTEDLIST_IDS"] = cfg.AccessControl.PermittedListIDs
	m["UPDATE_CRON"] = cfg.Maintenance.UpdateCron
	m["RESTART_CRON"] = cfg.Maintenance.RestartCron
	m["BACKUPS_CRON"] = cfg.Maintenance.BackupsCron
	m["UPDATE_IF_IDLE"] = strconv.FormatBool(cfg.Maintenance.UpdateIfIdle)
	m["RESTART_IF_IDLE"] = strconv.FormatBool(cfg.Maintenance.RestartIfIdle)
	m["BACKUPS"] = strconv.FormatBool(cfg.Maintenance.Backups)
	m["BACKUPS_IF_IDLE"] = strconv.FormatBool(cfg.Maintenance.BackupsIfIdle)
	m["BACKUPS_MAX_AGE"] = strconv.Itoa(cfg.Maintenance.BackupsMaxAge)
	m["BACKUPS_MAX_COUNT"] = strconv.Itoa(cfg.Maintenance.BackupsMaxCount)
	m["VALHEIM_PLUS"] = strconv.FormatBool(cfg.Mods.ValheimPlus)
	m["BEPINEX"] = strconv.FormatBool(cfg.Mods.BepInEx)
	m["TZ"] = cfg.SystemSettings.TimeZone
	m["PUID"] = strconv.Itoa(cfg.SystemSettings.PUID)
	m["PGID"] = strconv.Itoa(cfg.SystemSettings.PGID)
	return m
}

func (m *ServerManager) BuildConfigMap() *v1.ConfigMap {
	labels := serverLabels(m.opts.Name)
	return &v1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.opts.Name + "-configmap",
			Namespace: m.opts.Namespace,
			Labels:    labels,
		},
		Data: cfgMapper(m.opts.Config),
	}
}

func (m *ServerManager) BuildSecret() *v1.Secret {
	labels := serverLabels(m.opts.Name)
	return &v1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.opts.Name + "-secret",
			Namespace: m.opts.Namespace,
			Labels:    labels,
		},
		Type: v1.SecretTypeOpaque,
		StringData: map[string]string{
			"SERVER_PASS": m.opts.Config.CoreSettings.ServerPass,
		},
	}
}

func (m *ServerManager) BuildService() *v1.Service {
	labels := serverLabels(m.opts.Name)
	return &v1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.opts.Name + "-service",
			Namespace: m.opts.Namespace,
			Labels:    labels,
		},
		Spec: v1.ServiceSpec{
			Type: v1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": m.opts.Name,
			},
			Ports: []v1.ServicePort{
				{
					Name:       "valheim-udp",
					Protocol:   v1.ProtocolUDP,
					Port:       m.opts.Config.CoreSettings.ServerPort,
					TargetPort: intstr.FromInt32(m.opts.Config.CoreSettings.ServerPort),
					NodePort:   0,
				},
				{
					Name:       "valheim-query",
					Protocol:   v1.ProtocolUDP,
					Port:       m.opts.Config.CoreSettings.ServerPort + 1,
					TargetPort: intstr.FromInt32(m.opts.Config.CoreSettings.ServerPort + 1),
					NodePort:   0,
				},
			},
		},
	}
}

func (m *ServerManager) BuildPersistentVolumeClaim() *v1.PersistentVolumeClaim {
	labels := serverLabels(m.opts.Name)
	return &v1.PersistentVolumeClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "PersistentVolumeClaim",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      m.opts.Name + "-pvc",
			Namespace: m.opts.Namespace,
			Labels:    labels,
		},
		Spec: v1.PersistentVolumeClaimSpec{
			AccessModes: []v1.PersistentVolumeAccessMode{
				v1.ReadWriteOnce,
			},
			Resources: v1.VolumeResourceRequirements{
				Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse(PVC_STORAGE_AMOUNT)},
			},
		},
	}
}

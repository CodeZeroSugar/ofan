package k8s

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func (m *ServerManager) BuildDeployment() *appsv1.Deployment {
	labels := serverLabels(m.opts.Name)
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
			Replicas: &m.opts.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
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
	}
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
		Data: map[string]string{
			"SERVER_NAME":   m.opts.Config.CoreSettings.ServerName,
			"WORLD_NAME":    m.opts.Config.CoreSettings.WorldName,
			"SERVER_PUBLIC": fmt.Sprintf("%t", m.opts.Config.CoreSettings.ServerPublic),
		},
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
	var gameNodePort, queryNodePort int32
	if m.opts.NodePort > 0 {
		gameNodePort = m.opts.NodePort
		queryNodePort = gameNodePort + 1
	}
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
					NodePort:   gameNodePort,
				},
				{
					Name:       "valheim-query",
					Protocol:   v1.ProtocolUDP,
					Port:       m.opts.Config.CoreSettings.ServerPort + 1,
					TargetPort: intstr.FromInt32(m.opts.Config.CoreSettings.ServerPort + 1),
					NodePort:   queryNodePort,
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
				Requests: v1.ResourceList{v1.ResourceStorage: resource.MustParse("10Gi")},
			},
		},
	}
}

package k8s

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func TestBuildDeployment(t *testing.T) {
	mgr := NewServerManager(nil, ServerOpts{
		Name:      "alpha",
		Namespace: "ofan-test",
		Replicas:  int32(2),
	})

	name := mgr.opts.Name
	labels := serverLabels(name)
	dep := mgr.BuildDeployment()
	assert.Equal(t, "alpha", dep.Name)
	assert.Equal(t, "ofan-test", dep.Namespace)
	assert.Equal(t, int32(2), *dep.Spec.Replicas)
	assert.Equal(t, labels, dep.Labels)
	assert.Equal(t, labels, dep.Spec.Selector.MatchLabels)
	assert.Equal(t, labels, dep.Spec.Template.Labels)
	for _, c := range dep.Spec.Template.Spec.Containers {
		assert.Equal(t, "valheim-server", c.Name)
		assert.Equal(t, "ghcr.io/lloesche/valheim-server:latest", c.Image)
		for _, m := range c.VolumeMounts {
			assert.Equal(t, name+"-volume", m.Name)
		}
		for _, e := range c.EnvFrom {
			if e.ConfigMapRef != nil {
				assert.Equal(t, name+"-configmap", e.ConfigMapRef.Name)
			}
			if e.SecretRef != nil {
				assert.Equal(t, name+"-secret", e.SecretRef.Name)
			}
		}
	}
	for _, v := range dep.Spec.Template.Spec.Volumes {
		assert.Equal(t, name+"-volume", v.Name)
		assert.Equal(t, name+"-pvc", v.PersistentVolumeClaim.ClaimName)
	}
}

func TestBuildService(t *testing.T) {
	tests := []struct {
		name                string
		inputServerPort     int32
		expectedQueryPort   int32
		expectedServerPort  int32
		expectedServerQuery int32
	}{
		{
			name:                "node port is 0",
			inputServerPort:     2456,
			expectedQueryPort:   0,
			expectedServerPort:  2456,
			expectedServerQuery: 2457,
		},
	}

	for _, tt := range tests {
		mgr := NewServerManager(nil, ServerOpts{
			Name:      "alpha",
			Namespace: "ofan-test",
			Config: ValheimConfig{
				CoreSettings: CoreSettings{
					ServerPort: tt.inputServerPort,
				},
				AccessControl:  AccessControl{},
				Maintenance:    Maintenance{},
				Mods:           Mods{},
				SystemSettings: SystemSettings{},
			},
		})
		svc := mgr.BuildService()
		assert.True(t, len(svc.Spec.Ports) == 2)
		sawUdp := false
		sawQuery := false
		for _, p := range svc.Spec.Ports {
			switch p.Name {
			case "valheim-udp":
				sawUdp = true
				assert.Equal(t, tt.expectedServerPort, p.Port)
				assert.Equal(t, intstr.FromInt32(tt.expectedServerPort), p.TargetPort)
				assert.Equal(t, v1.ProtocolUDP, p.Protocol)
			case "valheim-query":
				sawQuery = true
				assert.Equal(t, tt.expectedServerQuery, p.Port)
				assert.Equal(t, intstr.FromInt32(tt.expectedServerQuery), p.TargetPort)
				assert.Equal(t, v1.ProtocolUDP, p.Protocol)
			}
		}
		assert.True(t, sawUdp)
		assert.True(t, sawQuery)
		assert.Equal(t, map[string]string{"app": mgr.opts.Name}, svc.Spec.Selector)
		assert.Equal(t, serverLabels("alpha"), svc.Labels)
	}
}

func TestMapper(t *testing.T) {
	tests := []struct {
		name string
		cfg  ValheimConfig
	}{
		{
			name: "mapper full surface",
			cfg: ValheimConfig{
				CoreSettings: CoreSettings{
					ServerName:   "alpha",
					WorldName:    "alpha-world",
					ServerPass:   "secret123",
					ServerPort:   3024,
					ServerPublic: false,
				},
				AccessControl: AccessControl{
					AdminListIDs:     "1,2,3,4,5",
					BannedListIDs:    "20,13",
					PermittedListIDs: "bob,jill,kevin",
				},
				Maintenance: Maintenance{
					UpdateCron:      "5 * * * *",
					UpdateIfIdle:    true,
					RestartCron:     "10 5 * * *",
					RestartIfIdle:   true,
					Backups:         true,
					BackupsIfIdle:   true,
					BackupsCron:     "5 * * * *",
					BackupsMaxAge:   5,
					BackupsMaxCount: 3,
				},
				Mods: Mods{
					BepInEx:     false,
					ValheimPlus: true,
				},
				SystemSettings: SystemSettings{
					TimeZone: "ct",
					PUID:     123456,
					PGID:     123456,
				},
			},
		},
		{
			name: "mapper defaults",
			cfg:  DefaultValheimConfig("alpha", "secret123"),
		},
		{
			name: "mapper empty optionals emitted",
			cfg: ValheimConfig{
				CoreSettings: CoreSettings{
					ServerName:   "alpha",
					WorldName:    "alpha-world",
					ServerPass:   "secret123",
					ServerPort:   3024,
					ServerPublic: false,
				},
				AccessControl: AccessControl{},
				Maintenance: Maintenance{
					UpdateIfIdle:  true,
					RestartIfIdle: true,
					Backups:       true,
					BackupsIfIdle: true,
				},
				Mods: Mods{
					BepInEx:     false,
					ValheimPlus: true,
				},
				SystemSettings: SystemSettings{},
			},
		},
		{
			name: "mapper excludes password",
			cfg:  DefaultValheimConfig("alpha", "secret123"),
		},
		{
			name: "mapper includes port",
			cfg: ValheimConfig{
				CoreSettings: CoreSettings{
					ServerName:   "alpha",
					WorldName:    "alpha-world",
					ServerPass:   "secret123",
					ServerPort:   2457,
					ServerPublic: false,
				},
				AccessControl: AccessControl{},
				Maintenance: Maintenance{
					UpdateIfIdle:  true,
					RestartIfIdle: true,
					Backups:       true,
					BackupsIfIdle: true,
				},
			},
		},
		{
			name: "mapper backups toggle",
			cfg:  DefaultValheimConfig("alpha", "secret123"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := cfgMapper(tt.cfg)
			assert.Len(t, m, 21)
			switch tt.name {
			case "mapper full surface":
				for _, v := range m {
					assert.IsType(t, string(""), v)
				}
				_, ok := m["ADMINLIST_IDS"]
				assert.True(t, ok)
				_, ok = m["BANNEDLIST_IDS"]
				assert.True(t, ok)
				_, ok = m["PERMITTEDLIST_IDS"]
				assert.True(t, ok)
			case "mapper defaults":
				v, ok := m["RESTART_CRON"]
				require.True(t, ok)
				assert.Equal(t, "10 5 * * *", v)
				v, ok = m["BACKUPS_CRON"]
				require.True(t, ok)
				assert.Equal(t, "5 * * * *", v)
				v, ok = m["RESTART_IF_IDLE"]
				require.True(t, ok)
				assert.Equal(t, "true", v)
				v, ok = m["VALHEIM_PLUS"]
				require.True(t, ok)
				assert.Equal(t, "false", v)
				v, ok = m["BEPINEX"]
				require.True(t, ok)
				assert.Equal(t, "false", v)
				v, ok = m["PUID"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
				v, ok = m["PGID"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
			case "mapper empty optionals emitted":
				v, ok := m["ADMINLIST_IDS"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["BANNEDLIST_IDS"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["PERMITTEDLIST_IDS"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["UPDATE_CRON"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["RESTART_CRON"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["BACKUPS_CRON"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["BACKUPS_MAX_AGE"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
				v, ok = m["BACKUPS_MAX_COUNT"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
				v, ok = m["TZ"]
				require.True(t, ok)
				assert.Equal(t, "", v)
				v, ok = m["PUID"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
				v, ok = m["PGID"]
				require.True(t, ok)
				assert.Equal(t, "0", v)
			case "mapper excludes password":
				_, ok := m["SERVER_PASS"]
				assert.False(t, ok)
			case "mapper includes port":
				v, ok := m["SERVER_PORT"]
				require.True(t, ok)
				assert.Equal(t, "2457", v)
			case "mapper backups toggle":
				v, ok := m["BACKUPS"]
				require.True(t, ok)
				assert.Equal(t, "true", v)
				tt.cfg.Maintenance.Backups = false
				m := cfgMapper(tt.cfg)
				v, ok = m["BACKUPS"]
				require.True(t, ok)
				assert.Equal(t, "false", v)
			default:
				t.Fatalf("no matching case for %s", tt.name)
			}
		})
	}
}

func TestBuildConfigMap(t *testing.T) {
	cfg := DefaultValheimConfig("alpha", "secret123")
	opts := NewServerOpts("alpha", "secret123", &cfg)
	fc := fake.NewSimpleClientset()
	mgr := NewServerManager(fc, opts)

	m := cfgMapper(cfg)
	cm := mgr.BuildConfigMap().Data
	assert.Equal(t, m, cm)
}

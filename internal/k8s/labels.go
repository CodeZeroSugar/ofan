package k8s

const (
	LabelManagedBy  = "ofan.io/managed-by"
	LabelServerName = "ofan.io/server-name"
	ManagedByOfan   = "ofan"
)

func serverLabels(name string) map[string]string {
	return map[string]string{
		"app":           name,
		LabelManagedBy:  ManagedByOfan,
		LabelServerName: name,
	}
}

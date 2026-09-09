package k8s

const (
	LabelManagedBy       = "ofan.io/managed-by"
	LabelServerName      = "ofan.io/server-name"
	ManagedByOfan        = "ofan"
	AnnotationConfigHash = "ofan.io/config-hash"
)

func serverLabels(name string) map[string]string {
	return map[string]string{
		"app":           name,
		LabelManagedBy:  ManagedByOfan,
		LabelServerName: name,
	}
}

func annotations(hash string) map[string]string {
	return map[string]string{
		AnnotationConfigHash: hash,
	}
}

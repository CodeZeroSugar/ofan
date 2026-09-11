package api

func deriveHealth(status, desired, podWaiting string, failures int) string {
	switch {
	case failures >= 5 || podWaiting == "CrashLoopBackOff":
		return "failed"
	case desired == "deleting" || desired == status:
		return "healthy"
	case desired == "running" && (status == "provisioning" || status == "starting"):
		return "healthy"
	default:
		return "degraded"
	}
}

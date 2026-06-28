package shared

func Short(commit string) string {
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

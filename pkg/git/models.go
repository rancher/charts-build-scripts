package git

// Git struct holds necessary data to work with the current git repository
type Git struct {
	Dir     string
	Branch  string
	Remotes map[string]string
}

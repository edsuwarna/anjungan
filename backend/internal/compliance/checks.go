package compliance

// DefaultChecks returns all default checks (CIS Level 1 + Level 2).
// Used by the check registry on initialization.
func DefaultChecks() []CheckDefinition {
	var all []CheckDefinition
	all = append(all, SSHChecks()...)
	all = append(all, KernelChecks()...)
	all = append(all, FilesystemChecks()...)
	all = append(all, UserChecks()...)
	all = append(all, ServiceChecks()...)
	all = append(all, NetworkChecks()...)
	all = append(all, LoggingChecks()...)
	all = append(all, DockerChecks()...)
	return all
}

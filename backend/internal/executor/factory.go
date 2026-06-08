package executor

// NewDockerExecutor creates a Docker socket executor.
// This is used for connection_type='docker-socket' servers.
func NewDockerExecutor(socketPath string) (ServerExecutor, error) {
	return NewDockerSocketExecutor(socketPath)
}

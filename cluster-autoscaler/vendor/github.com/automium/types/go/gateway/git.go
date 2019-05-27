package gateway

type GitConfig struct {
	RepositoryURL      string `json:"url"`
	RepositoryUsername string `json:"username"`
	RepositoryKey      string `json:"key"`
}

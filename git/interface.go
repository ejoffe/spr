package git

type GitInterface interface {
	Git(args string, output *string) error
	RootDir() string
}

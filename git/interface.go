package git

type Cmd func(args string, output *string) error
type RootDir func() string

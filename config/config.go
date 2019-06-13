package config

// Config represents command configuration.
type Config struct {
	Image       string
	Owner       string
	Asgname     string
	Clustername string
	Dryrun      bool
	Debug       bool
	Generation  int
}

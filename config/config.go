package config

import (
	"github.com/urfave/cli"
)

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

//SetConfig set current args to config
func SetConfig(ctx *cli.Context) *Config {
	conf := &Config{
		Asgname:     ctx.String("asgname"),
		Image:       ctx.String("image"),
		Clustername: ctx.String("clustername"),
		Owner:       ctx.String("owner"),
		Dryrun:      ctx.Bool("dry-run"),
		Debug:       ctx.Bool("verbose"),
		Generation:  ctx.Int("gen"),
	}
	return conf
}

package main

import "git.sr.ht/~jakintosh/command-go/pkg/args"

var Root = &args.Command{
	Name:    "assistant",
	Help:    "Assistant application",
	Subcommands: []*args.Command{
		Notes,
	},
}

func main() {
	Root.Parse()
}

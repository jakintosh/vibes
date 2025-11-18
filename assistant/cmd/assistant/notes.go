package main

import "git.sr.ht/~jakintosh/command-go/pkg/args"

var Notes = &args.Command{
	Name:    "notes",
	Help:    "Manage notes and extract insights",
	Subcommands: []*args.Command{
		Add,
	},
}

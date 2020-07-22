// Copyright (c) 2020 Tailscale Inc & AUTHORS All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// The tailscale command is the Tailscale command-line client. It interacts
// with the tailscaled node agent.
package main // import "tailscale.com/cmd/tailscale"

import (
	"fmt"
	"log"
	"os"

	"github.com/apenwarr/fixconsole"
	"tailscale.com/cmd/tailscale/cli"
)

func main() {
	err := fixconsole.FixConsoleIfNeeded()
	if err != nil {
		log.Printf("fixConsoleOutput: %v\n", err)
	}

	if err := cli.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

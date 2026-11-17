package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"kvdb/internal/client"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("kvdb", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)

	addr := flags.String("addr", "127.0.0.1:9000", "KVDB server address")
	timeout := flags.Duration("timeout", client.DefaultTimeout, "UDP request timeout")

	flags.Usage = printUsage

	if err := flags.Parse(args); err != nil {
		return err
	}

	commandArgs := flags.Args()
	if len(commandArgs) == 0 {
		printUsage()

		return errors.New("command is required")
	}

	req, err := parseRequest(commandArgs)
	if err != nil {
		return err
	}

	c, err := client.New(*addr, *timeout)
	if err != nil {
		return err
	}
	defer c.Close()

	resp, err := c.Do(req)
	if err != nil {
		return err
	}

	return printResponse(resp)
}

func printUsage() {
	fmt.Fprintln(os.Stderr, `KVDB UDP CLI

Usage:
  kvdb [options] <command> [arguments]

Options:
  -addr string       Server address (default 127.0.0.1:9000)
  -timeout duration  Request timeout (default 2s)

Commands:
  ping
  get <key>
  set <key> <value> [ttl_ms]
  setnx <key> <value> [ttl_ms]
  del <key>
  ttl <key>
  expire <key> <ttl_ms>
  keys <regexp>

Examples:
  kvdb ping
  kvdb set username Evgeny
  kvdb get username
  kvdb set session abc 60000
  kvdb keys "^user"
`)
}

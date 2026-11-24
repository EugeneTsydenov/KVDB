package main

import (
	"fmt"
	"unicode"
	"unicode/utf8"

	"kvdb/internal/protocol"
)

// printResponse displays the command result.
//
//nolint:cyclop
func printResponse(resp protocol.Response) error {
	switch resp.Status {
	case protocol.StatusNotFound, protocol.StatusNotSet:
		fmt.Println(resp.Status)

		return nil

	case protocol.StatusOK:

	default:
		if resp.Message != "" {
			return fmt.Errorf("%s: %s", resp.Status, resp.Message)
		}

		return fmt.Errorf("server returned %s", resp.Status)
	}

	switch resp.Header.Op {
	case protocol.OpPing:
		fmt.Println("PONG")

	case protocol.OpSet, protocol.OpSetNX, protocol.OpExpire:
		fmt.Println("OK")

	case protocol.OpGet:
		printValue(resp.Value)

	case protocol.OpDel:
		fmt.Printf("(integer) %d\n", resp.Deleted)

	case protocol.OpTTL:
		fmt.Printf("(integer) %d\n", resp.TTLMs)

	case protocol.OpKeys:
		if len(resp.Keys) == 0 {
			fmt.Println("(empty list)")

			return nil
		}

		for i, key := range resp.Keys {
			fmt.Printf("%d) %q\n", i+1, key)
		}

	default:
		return fmt.Errorf("unexpected opcode: %s", resp.Header.Op)
	}

	return nil
}

// printValue prints text directly and escapes binary values.
func printValue(value []byte) {
	if len(value) == 0 {
		fmt.Println()

		return
	}

	if utf8.Valid(value) {
		printable := true

		for _, r := range string(value) {
			if !unicode.IsPrint(r) {
				printable = false

				break
			}
		}

		if printable {
			fmt.Println(string(value))

			return
		}
	}

	fmt.Printf("%q\n", value)
}

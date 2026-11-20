package main

import (
	"fmt"
	"log"
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
		log.Println(resp.Status)

		return nil

	case protocol.StatusOK:

	default:
		if resp.Message != "" {
			return fmt.Errorf("%s: %s", resp.Status, resp.Message)
		}

		return fmt.Errorf("server returned %s", resp.Status)
	}

	switch resp.Op {
	case protocol.OpPing:
		log.Println("PONG")

	case protocol.OpSet, protocol.OpSetNX, protocol.OpExpire:
		log.Println("OK")

	case protocol.OpGet:
		printValue(resp.Value)

	case protocol.OpDel:
		log.Printf("(integer) %d\n", resp.Deleted)

	case protocol.OpTTL:
		log.Printf("(integer) %d\n", resp.TTLMs)

	case protocol.OpKeys:
		if len(resp.Keys) == 0 {
			log.Println("(empty list)")

			return nil
		}

		for i, key := range resp.Keys {
			log.Printf("%d) %q\n", i+1, key)
		}

	default:
		return fmt.Errorf("unexpected opcode: %s", resp.Op)
	}

	return nil
}

// printValue prints text directly and escapes binary values.
func printValue(value []byte) {
	if len(value) == 0 {
		log.Println()

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
			log.Println(string(value))

			return
		}
	}

	log.Printf("%q\n", value)
}

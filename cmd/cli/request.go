package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// parseRequest converts CLI arguments into a protocol request.
//
//nolint:cyclop,funlen
func parseRequest(args []string) (protocol.Request, error) {
	var req protocol.Request

	if len(args) == 0 {
		return req, errors.New("command is required")
	}

	command := strings.ToLower(args[0])
	params := args[1:]

	switch command {
	case "ping":
		if err := checkArgs(params, 0, 0); err != nil {
			return req, err
		}
		req.Op = protocol.OpPing

	case "get":
		if err := checkArgs(params, 1, 1); err != nil {
			return req, err
		}
		req.Op = protocol.OpGet
		req.Key = []byte(params[0])

	case "set", "setnx":
		//nolint:mnd
		if err := checkArgs(params, 2, 3); err != nil {
			return req, err
		}

		if command == "set" {
			req.Op = protocol.OpSet
		} else {
			req.Op = protocol.OpSetNX
		}

		req.Key = []byte(params[0])
		req.Value = []byte(params[1])

		//nolint:mnd
		if len(params) == 3 {
			ttl, err := parseTTL(params[2])
			if err != nil {
				return req, err
			}
			req.TTLMs = ttl
		}

	case "del":
		if err := checkArgs(params, 1, 1); err != nil {
			return req, err
		}
		req.Op = protocol.OpDel
		req.Key = []byte(params[0])

	case "ttl":
		if err := checkArgs(params, 1, 1); err != nil {
			return req, err
		}
		req.Op = protocol.OpTTL
		req.Key = []byte(params[0])

	case "expire":
		//nolint:mnd
		if err := checkArgs(params, 2, 2); err != nil {
			return req, err
		}

		ttl, err := parseTTL(params[1])
		if err != nil {
			return req, err
		}
		if ttl == 0 {
			return req, errors.New("EXPIRE requires a positive TTL")
		}

		req.Op = protocol.OpExpire
		req.Key = []byte(params[0])
		req.TTLMs = ttl

	case "keys":
		if err := checkArgs(params, 1, 1); err != nil {
			return req, err
		}

		req.Op = protocol.OpKeys
		req.Pattern = params[0]

	default:
		return req, fmt.Errorf("unknown command: %s", command)
	}

	return req, nil
}

func checkArgs(args []string, minim, maxim int) error {
	if len(args) < minim || len(args) > maxim {
		return fmt.Errorf(
			"invalid number of arguments: expected %d to %d, got %d",
			minim, maxim, len(args),
		)
	}

	return nil
}

func parseTTL(value string) (uint64, error) {
	ttl, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid TTL %q: %w", value, err)
	}

	if ttl > protocol.MaxTTLMs {
		return 0, fmt.Errorf(
			"TTL exceeds maximum %d ms",
			protocol.MaxTTLMs,
		)
	}

	return ttl, nil
}

package storage

import "context"

type Storage interface {
	Set(ctx context.Context, key string, value Value) error

	Get(ctx context.Context, key string) (Value, bool, error)

	SetNX(ctx context.Context, key string, value Value) (bool, error)

	Del(ctx context.Context, keys ...string) (int, error)
}

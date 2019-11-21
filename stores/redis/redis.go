// Package redis implements a Redis cache storage backend for fastcache.
// The internal structure looks like this where
// XX1234 = namespace, marketwach = group
// ```
// CACHE:XX1234:marketwatch {
//     "/user/marketwatch_ctype" -> []byte
//     "/user/marketwatch_etag" -> []byte
//     "/user/marketwatch_blob" -> []byte
//     "/user/marketwatch/123_ctype" -> []byte
//     "/user/marketwatch/123_etag" -> []byte
//     "/user/marketwatch/123_blob" -> []byte
// }
// ```
package redis

import (
	"time"

	"github.com/gomodule/redigo/redis"
	"REDACTED/commons/fastcache"
)

const (
	// Store keys.
	keyEtag  = "_etag"
	keyCtype = "_ctype"
	keyBlob  = "_blob"

	sep = ":"
)

// Store is a Redis cache store implementation for fastcache.
type Store struct {
	prefix string
	pool   *redis.Pool
}

// New creates a new Redis instance. prefix is the prefix to apply to all
// cache keys.
func New(prefix string, pool *redis.Pool) *Store {
	return &Store{
		prefix: prefix,
		pool:   pool,
	}
}

// Get gets the fastcache.Item for a single cached URI.
func (s *Store) Get(namespace, group, uri string) (fastcache.Item, error) {
	cn := s.pool.Get()
	defer cn.Close()

	var out fastcache.Item
	// Get content_type, etag, blob in that order.
	resp, err := redis.ByteSlices(cn.Do("HMGET", s.key(namespace, group), keyCtype, keyEtag, keyBlob))
	if err != nil {
		return out, err
	}

	out = fastcache.Item{
		ContentType: resp[0],
		ETag:        resp[1],
		Blob:        resp[2],
	}
	return out, err
}

// Put sets a value to given session but stored only on commit
func (s *Store) Put(namespace, group, uri string, b fastcache.Item, ttl time.Duration) error {
	cn := s.pool.Get()
	defer cn.Close()

	key := s.key(namespace, group)
	cn.Send("HMSET", key,
		keyCtype, b.ContentType,
		keyEtag, b.ETag,
		keyBlob, b.Blob)
	if ttl.Seconds() > 0 {
		exp := ttl.Nanoseconds() / int64(time.Millisecond)
		cn.Send("PEXPIRE", key, exp)
	}
	return cn.Flush()
}

// Del deletes a single cached URI.
func (s *Store) Del(namespace, group, uri string) error {
	cn := s.pool.Get()
	defer cn.Close()

	cn.Send("HDEL", s.key(namespace, group), uri+keyCtype, uri+keyEtag, uri+keyBlob)
	return cn.Flush()
}

// DelGroup deletes a whole group.
func (s *Store) DelGroup(namespace string, groups ...string) error {
	cn := s.pool.Get()
	defer cn.Close()

	for _, group := range groups {
		cn.Send("DEL", s.key(namespace, group))
	}
	return cn.Flush()
}

func (s *Store) key(namespace, group string) string {
	return s.prefix + namespace + sep + group
}
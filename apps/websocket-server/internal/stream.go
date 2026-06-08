package internal

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// fanoutStream manages a Redis pub/sub channel fan-out to multiple users.
type fanoutStream struct {
	ctx       context.Context
	redis     *redis.Client
	name      string
	channelFn func(key string) string
	makeFn    func(key string, data []byte) (*Event, error)

	mu       sync.RWMutex
	streams  map[string]*redis.PubSub
	users    map[string]map[*User]bool // key → set of subscribed users
	userKeys map[*User]map[string]bool // user → set of subscribed keys (for bulk removal)
}

func newFanoutStream(
	ctx context.Context,
	redisClient *redis.Client,
	name string,
	channelFn func(string) string,
	makeFn func(string, []byte) (*Event, error),
) *fanoutStream {
	return &fanoutStream{
		ctx:       ctx,
		redis:     redisClient,
		name:      name,
		channelFn: channelFn,
		makeFn:    makeFn,
		streams:   make(map[string]*redis.PubSub),
		users:     make(map[string]map[*User]bool),
		userKeys:  make(map[*User]map[string]bool),
	}
}

func (s *fanoutStream) subscribe(user *User, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.userKeys[user] != nil && s.userKeys[user][key] {
		return
	}

	if s.userKeys[user] == nil {
		s.userKeys[user] = make(map[string]bool)
	}
	s.userKeys[user][key] = true

	if s.users[key] == nil {
		s.users[key] = make(map[*User]bool)
	}
	alreadyStreaming := len(s.users[key]) > 0
	s.users[key][user] = true

	if alreadyStreaming {
		slog.Info("user joined stream", "stream", s.name, "user", user.ID, "key", key)
		return
	}

	pubsub := s.redis.Subscribe(s.ctx, s.channelFn(key))
	s.streams[key] = pubsub
	slog.Info("user started stream", "stream", s.name, "user", user.ID, "key", key)
	go s.receive(key, pubsub)
}

func (s *fanoutStream) unsubscribe(user *User, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.removeFromKey(user, key)
}

func (s *fanoutStream) removeUser(user *User) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.userKeys[user] {
		s.removeFromKey(user, key)
	}
}

// removeFromKey must be called with mu write-locked.
func (s *fanoutStream) removeFromKey(user *User, key string) {
	if s.userKeys[user] != nil {
		delete(s.userKeys[user], key)
		if len(s.userKeys[user]) == 0 {
			delete(s.userKeys, user)
		}
	}

	if s.users[key] == nil {
		return
	}
	delete(s.users[key], user)
	if len(s.users[key]) > 0 {
		return
	}

	if pubsub, ok := s.streams[key]; ok {
		pubsub.Close()
		delete(s.streams, key)
	}
	delete(s.users, key)
	slog.Info("closed stream (no subscribers)", "stream", s.name, "key", key)
}

func (s *fanoutStream) receive(key string, pubsub *redis.PubSub) {
	defer func() {
		s.mu.Lock()
		delete(s.streams, key)
		hasUsers := len(s.users[key]) > 0
		s.mu.Unlock()

		if hasUsers && s.ctx.Err() == nil {
			slog.Warn("stream dropped, reconnecting", "stream", s.name, "key", key)
			time.Sleep(reconnectDelay)
			go s.reconnect(key)
		}
	}()

	for msg := range pubsub.Channel() {
		event, err := s.makeFn(key, []byte(msg.Payload))
		if err != nil {
			slog.Error("stream event build failed", "stream", s.name, "key", key, "err", err)
			continue
		}

		s.mu.RLock()
		snapshot := make([]*User, 0, len(s.users[key]))
		for u := range s.users[key] {
			snapshot = append(snapshot, u)
		}
		s.mu.RUnlock()

		for _, u := range snapshot {
			u.emit(event)
		}
	}
}

func (s *fanoutStream) reconnect(key string) {
	s.mu.RLock()
	hasUsers := len(s.users[key]) > 0
	s.mu.RUnlock()

	if !hasUsers || s.ctx.Err() != nil {
		return
	}

	pubsub := s.redis.Subscribe(s.ctx, s.channelFn(key))

	s.mu.Lock()
	s.streams[key] = pubsub
	s.mu.Unlock()

	go s.receive(key, pubsub)
}

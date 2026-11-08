package replica

import (
	"context"
	"errors"
	"fmt"

	"distkv/internal/kv"
)

// REFERENCE SOLUTION for Step 03. Try it yourself first!

func NewCluster(names ...string) *Cluster {
	if len(names) == 0 {
		names = []string{"node-0"} // zero names: a single default node, so nil never has to happen
	}
	all := make([]*Replica, 0, len(names))
	for i, name := range names {
		role := RoleFollower
		if i == 0 {
			role = RoleLeader
		}
		all = append(all, &Replica{Name: name, Role: role, store: kv.NewStore(), healthy: true})
	}
	return &Cluster{leader: all[0], followers: all[1:], majority: len(all)/2 + 1}
}

func (c *Cluster) Leader() *Replica { return c.leader }

func (c *Cluster) Followers() []*Replica {
	out := make([]*Replica, len(c.followers))
	copy(out, c.followers)
	return out
}

func (r *Replica) Get(ctx context.Context, key string) (string, error) {
	if !r.healthy {
		return "", ErrReplicaDown
	}
	value, err := r.store.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("replica %s: %w", r.Name, err)
	}
	return value, nil
}

func (r *Replica) getRaw(ctx context.Context, key string) (string, bool, error) {
	if !r.healthy {
		return "", false, ErrReplicaDown
	}
	value, err := r.store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return value, true, nil
}

func (r *Replica) Set(ctx context.Context, key, value string) error {
	if !r.healthy {
		return ErrReplicaDown
	}
	return r.store.Put(ctx, key, value)
}

func (r *Replica) Delete(ctx context.Context, key string) error {
	if !r.healthy {
		return ErrReplicaDown
	}
	return r.store.Delete(ctx, key)
}

func (c *Cluster) Put(ctx context.Context, key, value string) error {
	// Read the pre-write state from the leader to know how to roll back.
	old, found, err := c.leader.getRaw(ctx, key)
	if err != nil {
		return err
	}
	if err := c.leader.Set(ctx, key, value); err != nil {
		return err
	}

	acks := 1 // the leader
	touched := []*Replica{c.leader}
	for _, f := range c.followers {
		if err := f.Set(ctx, key, value); err == nil {
			acks++
			touched = append(touched, f)
		}
	}

	if acks < c.majority {
		for _, r := range touched {
			if found {
				_ = r.Set(ctx, key, old)
			} else {
				_ = r.Delete(ctx, key)
			}
		}
		return ErrNoQuorum
	}
	return nil
}

func (c *Cluster) Get(ctx context.Context, key string) (string, error) {
	return c.leader.Get(ctx, key)
}

func (c *Cluster) FollowerGet(ctx context.Context, i int, key string) (string, error) {
	if i < 0 || i >= len(c.followers) {
		return "", ErrNoFollower
	}
	return c.followers[i].Get(ctx, key)
}

func (c *Cluster) Crash(i int) {
	if i < 0 || i >= len(c.followers) {
		return
	}
	c.followers[i].healthy = false
}

func (c *Cluster) Heal(i int) {
	if i < 0 || i >= len(c.followers) {
		return
	}
	c.followers[i].healthy = true
}

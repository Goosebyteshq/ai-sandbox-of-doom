package engine

import (
	"crypto/sha1"
	"encoding/binary"
	"strings"
)

type CanaryPolicy struct {
	Enabled bool   `json:"enabled"`
	Percent int    `json:"percent"`
	Salt    string `json:"salt,omitempty"`
}

func ShouldUseCanary(runID string, policy CanaryPolicy) bool {
	if !policy.Enabled {
		return false
	}
	if policy.Percent <= 0 {
		return false
	}
	if policy.Percent >= 100 {
		return true
	}
	v := strings.TrimSpace(runID)
	if v == "" {
		return false
	}
	h := sha1.Sum([]byte(policy.Salt + ":" + v))
	n := binary.BigEndian.Uint32(h[:4]) % 100
	return int(n) < policy.Percent
}

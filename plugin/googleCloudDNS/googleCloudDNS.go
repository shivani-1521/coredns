package route53

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/file"
	"github.com/coredns/coredns/plugin/pkg/fall"
	"github.com/coredns/coredns/plugin/pkg/upstream"
	"github.com/coredns/coredns/request"
	)

type GcloudDNS struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    dns.Service
	upstream  *upstream.Upstream

	zMu   sync.RWMutex
	zones zones
}

type zone struct {
	id  string
	z   *file.Zone
	dns string
}

type zones map[string][]*zone



func New(ctx context.Context, c *GoogleDNS, keys map[string][]uint64, up *upstream.Upstream) (*GcloudDNS, error) {
	zones := make(map[string][]*zone, len(keys))
	zoneNames := make([]string, 0, len(keys))
	for dns, hostedZoneIDs := range keys {
		for _, hostedZoneID := range hostedZoneIDs {
			// working here
			if err != nil {
				return nil, err
			}
			if _, ok := zones[dns]; !ok {
				zoneNames = append(zoneNames, dns)
			}
			zones[dns] = append(zones[dns], &zone{id: hostedZoneID, dns: dns, z: file.NewZone(dns, "")})
		}
	}
	return &GcloudDNS{
		client:    c,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}
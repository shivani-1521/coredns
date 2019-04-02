package googleCloudDNS

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

	"golang.org/x/oauth2/google"
	googledns "google.golang.org/api/dns/v1"
	"github.com/miekg/dns"
	)

type GcloudDNS struct {
	Next plugin.Handler
	Fall fall.F

	zoneNames []string
	client    GoogleDNS
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
	for dns, managedZoneIDs := range keys {
		for _, managedZoneID := range managedZoneIDs {
			z := dns.ManagedZone{
				DnsName : dns.(string),
				Id : managedZoneID.(uint64),
			}
			
			_, err := c.dnsClient.ManagedZones.List(c.projectID).Context(ctx).DnsName(dns).Do()

			if _, ok := zones[dns]; !ok {
				zoneNames = append(zoneNames, dns)
			}
			zones[dns] = append(zones[dns], &zone{id: managedZoneID, dns: dns, z: file.NewZone(dns, "")})
		}
	}
	return &GcloudDNS{
		client:    c,
		zoneNames: zoneNames,
		zones:     zones,
		upstream:  up,
	}, nil
}

func (h *GcloudDNS) Run(ctx context.Context) error {
	if err := h.updateZones(ctx); err != nil {
		return err
	}
	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Infof("Breaking out of GcloudDNS update loop: %v", ctx.Err())
				return
			case <-time.After(1 * time.Minute):
				if err := h.updateZones(ctx); err != nil && ctx.Err() == nil {
					log.Errorf("Failed to update zones: %v", err)
				}
			}
		}
	}()
	return nil
}

func (h *GcloudDNS) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}
	qname := state.Name()

	zName := plugin.Zones(h.zoneNames).Matches(qname)
	if zName == "" {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}
	z, ok := h.zones[zName]
	if !ok || z == nil {
		return dns.RcodeServerFailure, nil
	}

	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true
	var result file.Result
	for _, managedZone := range z {
		h.zMu.RLock()
		m.Answer, m.Ns, m.Extra, result = managedZone.z.Lookup(state, qname)
		h.zMu.RUnlock()
		if len(m.Answer) != 0 {
			break
		}
	}

	if len(m.Answer) == 0 && h.Fall.Through(qname) {
		return plugin.NextOrFailure(h.Name(), h.Next, ctx, w, r)
	}

	switch result {
	case file.Success:
	case file.NoData:
	case file.NameError:
		m.Rcode = dns.RcodeNameError
	case file.Delegation:
		m.Authoritative = false
	case file.ServerFailure:
		return dns.RcodeServerFailure, nil
	}

	w.WriteMsg(m)
	return dns.RcodeSuccess, nil
}

func updateZoneFromRRS(rrs *googledns.ResourceRecordSet, z *file.Zone) error {
	rfc1035 := fmt.Sprintf("%s %d IN %s %s", rrs.Name, rrs.Ttl, rrs.Kind, rrs.Rrdatas)
	r, err := dns.NewRR(rfc1035)
	if err != nil {
		return fmt.Errorf("failed to parse resource record: %v", err)
	}
	z.Insert(r)
	return nil
}

func (h *GcloudDNS) updateZones(ctx context.Context) error {
	errc := make(chan error)
	defer close(errc)
	for zName, z := range h.zones {
		go func(zName string, z []*zone) {
			var err error
			defer func() {
				errc <- err
			}()

			for i, managedZone := range z {
				newZ := file.NewZone(zName, "")
				newZ.Upstream = h.upstream
				
				err = h.client.dnsClient.ResourceRecordSetsService.List(client.projectID, managedZone).Pages(ctx,
					func(out *googledns.ResourceRecordSetsListResponse) error {
						for _, rrs := range out.Rrsets {
							if err := updateZoneFromRRS(rrs, newZ); err != nil {
								log.Warningf("Failed to process resource record set: %v", err)
							}
						}
						return nil
					})
				if err != nil {
					err = fmt.Errorf("failed to list resource records for %v:%v from Google cloud DNS: %v", zName, managedZone.id, err)
					return
				}
				h.zMu.Lock()
				(*z[i]).z = newZ
				h.zMu.Unlock()
			}

		}(zName, z)
	}

	var errs []string
	for i := 0; i < len(h.zones); i++ {
		err := <-errc
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) != 0 {
		return fmt.Errorf("errors updating zones: %v", errs)
	}
	return nil
}

func (h *GoogleDNS) Name() string { return "googleCloudDNS" }

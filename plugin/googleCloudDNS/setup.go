package googleCloudDNS

import (
	"context"
	"strings"
	"strconv"

	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/plugin/pkg/fall"
	//clog "github.com/coredns/coredns/plugin/pkg/log"
	"github.com/coredns/coredns/plugin/pkg/upstream"

	"golang.org/x/oauth2/google"
	googledns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	"io/ioutil"

	"github.com/mholt/caddy"
)


type GoogleDNS struct {
	projectID string
	dnsClient *googledns.Service
}

const scopes = googledns.NdevClouddnsReadonlyScope

func init() {
	caddy.RegisterPlugin("googleCloudDNS", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(creds *google.Credentials)  (*GoogleDNS, error){
				ctx := context.Background()
				dnsService, err := googledns.NewService(ctx, option.WithCredentials(creds), option.WithScopes(scopes))
				if err != nil {
					return nil,err
				}
				return &GoogleDNS{
					projectID : creds.ProjectID,
					dnsClient : dnsService,
				}, nil
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(creds *google.Credentials) (*GoogleDNS, error)) error{
	keyPairs := map[string]struct{}{}
	keys := map[string][]uint64{}	
	
	var fall fall.F
 
	var data []byte
	var creds *google.Credentials
	var client *GoogleDNS
	var err, jsonErr error

	up := upstream.New()
	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			dns, managedZoneid := parts[0], parts[1]
			managedZoneID, err := strconv.ParseUint(managedZoneid, 10, 64)
			if dns == "" || managedZoneID == 0 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			if _, ok := keyPairs[args[i]]; ok {
				return c.Errf("conflict zone '%s'", args[i])
			}

			keyPairs[args[i]] = struct{}{}
			keys[dns] = append(keys[dns], managedZoneID)
		}

		for c.NextBlock() {
			switch c.Val() {
			case "json_key":
				v := c.RemainingArgs()
				if len(v) < 1 {
					return c.Errf("invalid json key '%v'", v)
				}
				data, jsonErr = ioutil.ReadFile(v[0])
				if jsonErr != nil {
					return jsonErr
				}
				ctx := context.Background()
				creds, err = google.CredentialsFromJSON(ctx, data, scopes)
				if err != nil {
				    return err
				}

			case "upstream":
				c.RemainingArgs() 

			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())

			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	
	if data != nil{
		client, err = f(creds) 
	} else {
		ctx := context.Background()
		creds, err = google.FindDefaultCredentials(ctx , scopes)
		if err != nil {
			return err
		}
		client, err = f(creds)
	}
	
	ctx := context.Background()
	h, err := New(ctx, client, keys, up)
	if err != nil {
		return c.Errf("failed to create googleCloudDNS plugin: %v", err)
	}
	h.Fall = fall
	if err := h.Run(ctx); err != nil {
		return c.Errf("failed to initialize googleCloudDNS plugin: %v", err)
	}
	dnsserver.GetConfig(c).AddPlugin(func(next plugin.Handler) plugin.Handler {
		h.Next = next
		return h
	})
	return nil
}

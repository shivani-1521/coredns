package googleCloudDNS

import (
	"net/http"
	"context"
	//"strings"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googledns "google.golang.org/api/dns/v1"
)


type GoogleDNS struct {
	projectID string
	dnsClient *googledns.Service
}

func init() {
	caddy.RegisterPlugin("googleCloudDNS", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(creds *google.Credentials)  (*GoogleDNS, error){
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
	ctx := context.Background()
	scopes := googledns.NdevClouddnsReadonlyScope
	var fall fall.F
 
	data := ""
	jsonErr := nil
	var creds *google.Credentials
	err := nil

	up := upstream.New()
	for c.Next() {
		args := c.RemainingArgs()

		for i := 0; i < len(args); i++ {
			parts := strings.SplitN(args[i], ":", 2)
			if len(parts) != 2 {
				return c.Errf("invalid zone '%s'", args[i])
			}
			dns, hostedZoneID := parts[0], parts[1]
			if dns == "" || hostedZoneID == "" {
				return c.Errf("invalid zone '%s'", args[i])
			}
			if _, ok := keyPairs[args[i]]; ok {
				return c.Errf("conflict zone '%s'", args[i])
			}

			keyPairs[args[i]] = struct{}{}
			keys[dns] = append(keys[dns], hostedZoneID)
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
	
	if data != ""{
		client, err := f(creds) 
	} else {
		creds, err = google.FindDefaultCredentials(ctx , scopes)
		if err != nil {
			return err
		}
		client, err := f(creds)
	}
	
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


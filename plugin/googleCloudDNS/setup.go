package googleCloudDNS

import (
	"net/http"
	"context"
	//"strings"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)


type GoogleDNS struct {
	projectID string
	dnsClient *dns.Service
}

func init() {
	caddy.RegisterPlugin("googleCloudDNS", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(serviceAccount []byte, projectID string)  (*GoogleDNS, error){

				jwtConfig, err := google.JWTConfigFromJSON(serviceAccount, dns.NdevClouddnsReadonlyScope)
				if err != nil {
					return nil, err
				}
				ctx := context.Background()
				ts := jwtConfig.TokenSource(ctx)
				client := oauth2.NewClient(ctx, ts)

				d, err := dns.New(client)
				return &GoogleDNS{
					projectID : projectID,
					dnsClient : d,
				}, nil
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(serviceAccount []byte) (*dns.Service, error)) error{
	keyPairs := map[string]struct{}{}
	keys := map[string][]uint64{}

	
	ctx := context.Background()
	scopes := dns.NdevClouddnsReadonlyScope
	var fall fall.F

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
				jsonKey, err := ioutil.ReadFile(v[0])
				if err != nil {
					return nil, err
				}

				var jwt map[string]string

				err = json.Unmarshal(jsonKey, &jwt)
				if err != nil {
					return nil, err
				}

				projectID, ok := jwt["project_id"]
				if !ok {
					return nil, fmt.Errorf("Unable to get project_id from jwt")
				}
			case "upstream":
				c.RemainingArgs() 

			case "credentials":
				projectID = c.Val()

			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())

			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}
	
	client, err := f(jsonKey, projectID) 
	
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


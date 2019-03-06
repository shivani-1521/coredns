package googleCloudDNS

import (
	"net/http"
	"context"
	//"strings"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
)



func init() {
	caddy.RegisterPlugin("googleCloudDNS", caddy.Plugin{
		ServerType: "dns",
		Action: func(c *caddy.Controller) error {
			f := func(serviceAccount []byte)  (dns.Service, error){

				jwtConfig, err := google.JWTConfigFromJSON(serviceAccount, dns.NdevClouddnsReadonlyScope)
				if err != nil {
					return nil, err
				}
				ctx := context.Background()
				jwtHTTPClient := jwtConfig.Client(ctx)

				return dns.New(jwtHTTPClient)
			}
			return setup(c, f)
		},
	})
}

func setup(c *caddy.Controller, f func(serviceAccount []byte) (*dns.Service, error)) error{
	keyPairs := map[string]struct{}{}
	keys := map[string][]string{}

	var credentials
	var token
	var err
	var flag

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
				credentials, err := google.CredentialsFromJSON(ctx, v[0], scopes)
				flag = credentials.TokenSource(oauth2.NoContext)
				token, err = flag.Token()
				if err != nil{
					//we didn't get token from json so check in env var and other places
					credentials, err = google.FindDefaultCredentials(ctx, scopes)
					flag = credentials.TokenSource(oauth2.NoContext)
					token, err = flag.Token()
					if err != nil {
						return c.Errf("invalid json key '%v'", v)
					}
				}


			case "upstream":
				c.RemainingArgs() 

			case "credentials":
				credentials.ProjectID = c.Val()

			case "fallthrough":
				fall.SetZonesFromArgs(c.RemainingArgs())

			default:
				return c.Errf("unknown property '%s'", c.Val())
			}
		}
	}

	
	
	/* FindDefaultCredentials looks for credentials in the following places, preferring the first location found:

		1. A JSON file whose path is specified by the
		   GOOGLE_APPLICATION_CREDENTIALS environment variable.
		2. A JSON file in a location known to the gcloud command-line tool.
		   On Windows, this is %APPDATA%/gcloud/application_default_credentials.json.
		   On other systems, $HOME/.config/gcloud/application_default_credentials.json.
		3. On Google App Engine standard first generation runtimes (<= Go 1.9) it uses
		   the appengine.AccessToken function.
		4. On Google Compute Engine, Google App Engine standard second generation runtimes
		   (>= Go 1.11), and Google App Engine flexible environment, it fetches
		   credentials from the metadata server.
		   (In this final case any provided scopes are ignored.)
	*/

	
	

	client, err := f(credentials.JSON) 
	
	//something like new of route53
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


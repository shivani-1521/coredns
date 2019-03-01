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
package googleCloudDNS

import (
	"testing"

	"golang.org/x/oauth2/google"
	googledns "google.golang.org/api/dns/v1"
	"github.com/mholt/caddy"
)

func TestSetupgoogleCloudDNS(t *testing.T) {
	f := func(creds *google.Credentials)  (*GoogleDNS, error){
		return fakeGoogleCloudDNS{}
	}

	tests := []struct {
		body          string
		expectedError bool
	}{
		{`googleCloudDNS`, false},
		{`googleCloudDNS :`, true},
		{`googleCloudDNS example.org:12345678`, false},
		{`googleCloudDNS example.org:12345678 {
    aws_access_key
}`, true},
		{`googleCloudDNS example.org:12345678 {
    upstream 10.0.0.1
}`, false},

		{`googleCloudDNS example.org:12345678 {
    upstream
}`, false},
		{`googleCloudDNS example.org:12345678 {
    wat
}`, true},
		{`googleCloudDNS example.org:12345678 {
    aws_access_key ACCESS_KEY_ID SEKRIT_ACCESS_KEY
    upstream 1.2.3.4
}`, false},

		{`googleCloudDNS example.org:12345678 {
    fallthrough
}`, false},
		{`googleCloudDNS example.org:12345678 {
 		upstream 1.2.3.4
	}`, true},

		{`googleCloudDNS example.org:12345678 {
 		upstream 1.2.3.4
	}`, false},
		{`googleCloudDNS example.org:12345678 {
 		upstream 1.2.3.4
	}`, false},
		{`googleCloudDNS example.org:12345678 {
 		upstream 1.2.3.4
	}`, true},
		{`googleCloudDNS example.org:12345678 example.org:12345678 {
 		upstream 1.2.3.4
	}`, true},

		{`googleCloudDNS example.org {
 		upstream 1.2.3.4
	}`, true},
	}

	for _, test := range tests {
		c := caddy.NewTestController("dns", test.body)
		if err := setup(c, f); (err == nil) == test.expectedError {
			t.Errorf("Unexpected errors: %v", err)
		}
	}
}

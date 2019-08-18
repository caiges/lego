package liquidweb

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-acme/lego/v3/platform/tester"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var envTest = tester.NewEnvTest("LW_URL", "LW_USERNAME", "LW_PASSWORD", "LW_TIMEOUT")

func setupTest() (*DNSProvider, *http.ServeMux, func()) {
	handler := http.NewServeMux()
	server := httptest.NewServer(handler)
	config := NewDefaultConfig()
	config.Username = "blars"
	config.Password = "tacoman"
	config.URL = server.URL
	config.Zone = "tacoman.com"

	provider, err := NewDNSProviderConfig(config)
	if err != nil {
		panic(err)
	}

	return provider, handler, server.Close
}

func TestNewDNSProvider(t *testing.T) {
	testCases := []struct {
		desc     string
		envVars  map[string]string
		expected string
	}{
		{
			desc: "success",
			envVars: map[string]string{
				"LW_URL":      "https://storm.com",
				"LW_USERNAME": "blars",
				"LW_PASSWORD": "tacoman",
			},
		},
		{
			desc:     "missing url",
			envVars:  map[string]string{},
			expected: "liquidweb: url is missing",
		},
		{
			desc: "missing username",
			envVars: map[string]string{
				"LW_URL": "https://storm.com",
			},
			expected: "liquidweb: username is missing",
		},
		{
			desc: "missing password",
			envVars: map[string]string{
				"LW_URL":      "https://storm.com",
				"LW_USERNAME": "blars",
			}, expected: "liquidweb: password is missing",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			defer envTest.RestoreEnv()
			envTest.ClearEnv()

			envTest.Apply(test.envVars)

			p, err := NewDNSProvider()

			if len(test.expected) == 0 {
				require.NoError(t, err)
				require.NotNil(t, p)
				require.NotNil(t, p.config)
				require.NotNil(t, p.recordIDs)
			} else {
				require.EqualError(t, err, test.expected)
			}
		})
	}
}

func TestNewDNSProviderConfig(t *testing.T) {
	testCases := []struct {
		desc     string
		envVars  map[string]string
		expected string
	}{
		{
			desc: "success",
			envVars: map[string]string{
				"LW_URL":      "https://storm.com",
				"LW_USERNAME": "blars",
				"LW_PASSWORD": "tacoman",
			},
		},
		{
			desc:     "missing url",
			envVars:  map[string]string{},
			expected: "liquidweb: url is missing",
		},
		{
			desc: "missing username",
			envVars: map[string]string{
				"LW_URL": "https://storm.com",
			},
			expected: "liquidweb: username is missing",
		},
		{
			desc: "missing password",
			envVars: map[string]string{
				"LW_URL":      "https://storm.com",
				"LW_USERNAME": "blars",
			}, expected: "liquidweb: password is missing",
		},
	}

	for _, test := range testCases {
		t.Run(test.desc, func(t *testing.T) {
			defer envTest.RestoreEnv()
			envTest.ClearEnv()

			envTest.Apply(test.envVars)

			config := NewDefaultConfig()

			p, err := NewDNSProviderConfig(config)

			if len(test.expected) == 0 {
				require.NoError(t, err)
				require.NotNil(t, p)
				require.NotNil(t, p.config)
				require.NotNil(t, p.recordIDs)
			} else {
				require.EqualError(t, err, test.expected)
			}
		})
	}
}

func TestDNSProvider_Present(t *testing.T) {
	provider, mux, tearDown := setupTest()
	defer tearDown()

	mux.HandleFunc("/v1/Network/DNS/Record/create", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method, "method")

		//assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type")
		assert.Equal(t, "Basic YmxhcnM6dGFjb21hbg==", r.Header.Get("Authorization"), "Authorization")

		reqBody, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		expectedReqBody := `{"params":{"name":"_acme-challenge.tacoman.com.","rdata":"\"47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU\"","type":"TXT","zone":"tacoman.com"}}`
		assert.Equal(t, expectedReqBody, string(reqBody))

		w.WriteHeader(http.StatusOK)
		_, err = fmt.Fprintf(w, `{
			"type": "TXT",
			"name": "_acme-challenge.tacoman.com",
			"rdata": "47DEQpj8HBSa-_TImW-5JCeuQeRkm5NMpJWZG3hSuFU",
			"id": 1234567,
			"prio": null
		}`)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	err := provider.Present("tacoman.com", "", "")
	fmt.Printf("%+v", err)
	require.NoError(t, err)
}

func TestDNSProvider_CleanUp(t *testing.T) {
	provider, mux, tearDown := setupTest()
	defer tearDown()

	mux.HandleFunc("/v2/domains/example.com/records/1234567", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodDelete, r.Method, "method")

		assert.Equal(t, "/v2/domains/example.com/records/1234567", r.URL.Path, "Path")

		// NOTE: Even though the body is empty, DigitalOcean API docs still show setting this Content-Type...
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"), "Content-Type")
		assert.Equal(t, "Bearer asdf1234", r.Header.Get("Authorization"), "Authorization")

		w.WriteHeader(http.StatusNoContent)
	})

	provider.recordIDsMu.Lock()
	provider.recordIDs["_acme-challenge.example.com."] = 1234567
	provider.recordIDsMu.Unlock()

	err := provider.CleanUp("example.com", "", "")
	require.NoError(t, err, "fail to remove TXT record")
}

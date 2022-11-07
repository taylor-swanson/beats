package ea_azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	authLoginURL = "https://login.microsoftonline.com"
)

var (
	authScopes = []string{"https://graph.microsoft.com/.default"}
)

type authResponse struct {
	TokenType    string `json:"token_type"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	ExtExpiresIn int    `json:"ext_expires_in"`
}

func (a *azure) renewToken(ctx context.Context) error {
	if a.conf.LoginURL == "" {
		a.conf.LoginURL = authLoginURL
	}
	if len(a.conf.LoginScopes) == 0 {
		a.conf.LoginScopes = authScopes
	}

	c, err := a.conf.Transport.Client()
	if err != nil {
		return fmt.Errorf("unable to create HTTP client: %w", err)
	}

	endpointURL, err := url.Parse(authLoginURL + "/" + a.conf.TenantID + "/oauth2/v2.0/token")
	if err != nil {
		return fmt.Errorf("unable to parse URL: %w", err)
	}
	reqValues := url.Values{
		"client_id":     []string{a.conf.ClientID},
		"scope":         authScopes,
		"client_secret": []string{url.QueryEscape(a.conf.Secret)},
		"grant_type":    []string{"client_credentials"},
	}
	reqEncoded := reqValues.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpointURL.String(), strings.NewReader(reqEncoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	res, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("auth token request failed: %w", err)
	}
	defer res.Body.Close()
	resData, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("unable to read token response body: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("token request returned unexpected status code: %d body: %s", res.StatusCode, string(resData))
	}

	var authRes authResponse
	if err = json.Unmarshal(resData, &authRes); err != nil {
		return fmt.Errorf("unable to unmarshal token reseponse: %w", err)
	}

	a._authToken = authRes.AccessToken
	a.tokenExpires = time.Now().Add(time.Duration(authRes.ExpiresIn) * time.Second)
	a.logger.Debugf("Renewed bearer token, expires at: %v", a.tokenExpires)

	return nil
}

func (a *azure) getToken(ctx context.Context) (string, error) {
	if time.Now().Before(a.tokenExpires) && a._authToken != "" {
		a.logger.Debug("Retrieving cached token")
		return a._authToken, nil
	}

	a.logger.Debugf("Existing token has expired or not set, renewing token")
	if err := a.renewToken(ctx); err != nil {
		return "", fmt.Errorf("failed to renew token: %w", err)
	}

	return a._authToken, nil
}

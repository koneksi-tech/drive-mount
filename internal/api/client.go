package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/koneksi/koneksi-drive/internal/config"
)

type Client struct {
	baseURL      string
	clientID     string
	clientSecret string
	directoryID  string
	httpClient   *http.Client
	token        string
	tokenExpiry  time.Time
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type FileInfo struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
	Path     string    `json:"path"`
}

type ListResponse struct {
	Files []FileInfo `json:"files"`
}

func NewClient(cfg *config.APIConfig) (*Client, error) {
	return &Client{
		baseURL:      cfg.BaseURL,
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		directoryID:  cfg.DirectoryID,
		httpClient: &http.Client{
			Timeout: cfg.Timeout,
		},
	}, nil
}

func (c *Client) authenticate() error {
	authURL := fmt.Sprintf("%s/oauth/token", c.baseURL)
	
	payload := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     c.clientID,
		"client_secret": c.clientSecret,
	}
	
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	req, err := http.NewRequest("POST", authURL, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("authentication failed: %s", resp.Status)
	}
	
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return err
	}
	
	c.token = tokenResp.AccessToken
	c.tokenExpiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	
	return nil
}

func (c *Client) ensureAuthenticated() error {
	if c.token == "" || time.Now().After(c.tokenExpiry) {
		return c.authenticate()
	}
	return nil
}

func (c *Client) doRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	if err := c.ensureAuthenticated(); err != nil {
		return nil, err
	}
	
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, err
	}
	u.Path = path.Join(u.Path, endpoint)
	
	req, err := http.NewRequest(method, u.String(), body)
	if err != nil {
		return nil, err
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	return c.httpClient.Do(req)
}

func (c *Client) List(dirPath string) ([]FileInfo, error) {
	endpoint := fmt.Sprintf("/api/v1/directories/%s/files", c.directoryID)
	if dirPath != "" && dirPath != "/" {
		endpoint += "?path=" + url.QueryEscape(dirPath)
	}
	
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("list failed: %s", resp.Status)
	}
	
	var listResp ListResponse
	if err := json.NewDecoder(resp.Body).Decode(&listResp); err != nil {
		return nil, err
	}
	
	return listResp.Files, nil
}

func (c *Client) Read(filePath string) (io.ReadCloser, error) {
	endpoint := fmt.Sprintf("/api/v1/directories/%s/files/%s/content", 
		c.directoryID, url.QueryEscape(filePath))
	
	resp, err := c.doRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("read failed: %s", resp.Status)
	}
	
	return resp.Body, nil
}

func (c *Client) Write(filePath string, data io.Reader) error {
	endpoint := fmt.Sprintf("/api/v1/directories/%s/files/%s/content", 
		c.directoryID, url.QueryEscape(filePath))
	
	resp, err := c.doRequest("PUT", endpoint, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("write failed: %s", resp.Status)
	}
	
	return nil
}

func (c *Client) Delete(filePath string) error {
	endpoint := fmt.Sprintf("/api/v1/directories/%s/files/%s", 
		c.directoryID, url.QueryEscape(filePath))
	
	resp, err := c.doRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("delete failed: %s", resp.Status)
	}
	
	return nil
}

func (c *Client) Mkdir(dirPath string) error {
	endpoint := fmt.Sprintf("/api/v1/directories/%s/folders", c.directoryID)
	
	payload := map[string]string{
		"path": dirPath,
	}
	
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	
	resp, err := c.doRequest("POST", endpoint, bytes.NewBuffer(data))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("mkdir failed: %s", resp.Status)
	}
	
	return nil
}
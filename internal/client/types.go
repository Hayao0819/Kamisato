package client

import "github.com/Hayao0819/Kamisato/internal/protocol"

// BuildRequest is the build request wire type.
type BuildRequest = protocol.BuildRequest
type GitSource = protocol.GitSource

// Job is the command-line job projection.
type Job = protocol.BuildJob
type Stats = protocol.BuildStats

type Admin struct {
	ID    int64  `json:"id"`
	Login string `json:"login"`
}

// TokenPair is returned by login and refresh endpoints.
type TokenPair struct {
	AccessToken  string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Login        string `json:"login"`
	ID           int64  `json:"id"`
}

type DeviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

type DeviceTokenResult struct {
	Token     string
	Refresh   string
	ExpiresIn int
	Login     string
	ID        int64
	Status    string
}

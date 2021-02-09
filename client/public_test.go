package client

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOAuthRequestToParams(t *testing.T) {
	o := OAuthRequest{
		Username: "jlebowski",
		Password: "abides!",
		ClientID: "the-rug",
	}
	p := o.toParams()

	assert.Equal(t, o.Username, p.Get("username"))
	assert.Equal(t, o.Password, p.Get("password"))
	assert.Equal(t, o.ClientID, p.Get("client_id"))
}

package checker

import (
	"net/http"
)

var (
	client *http.Client
	ipinfo IPInfo

	endpoint = "https://security.bilibili.com/412"
)

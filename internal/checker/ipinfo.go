package checker

type IPInfo struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ttl     int    `json:"ttl"`
	Data    struct {
		IpAddr string `json:"ip_addr"`
		Mid    string `json:"mid"`
	} `json:"data"`
}

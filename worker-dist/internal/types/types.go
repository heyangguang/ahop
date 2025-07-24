package types

// HostInfo 主机连接信息
type HostInfo struct {
	ID         uint            `json:"id"`
	IP         string          `json:"ip"`
	Port       int             `json:"port"`
	Hostname   string          `json:"hostname"`
	Credential *CredentialInfo `json:"credential"`
}

// CredentialInfo 凭证信息（解密后）
type CredentialInfo struct {
	Type       string `json:"type"`
	Username   string `json:"username"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"private_key,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}
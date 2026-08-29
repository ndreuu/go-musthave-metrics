package models

// AuditEvent представляет событие аудита для логирования запросов.
type AuditEvent struct {
	IPAddress string   `json:"ip_address"`
	Metrics   []string `json:"metrics"`
	Timestamp int64    `json:"ts"`
}
